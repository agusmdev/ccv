package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// OutputMode represents the output formatting mode
type OutputMode string

const (
	OutputModeText    OutputMode = "text"
	OutputModeJSON    OutputMode = "json"
	OutputModeVerbose OutputMode = "verbose"
	OutputModeQuiet   OutputMode = "quiet"
)

// OutputProcessor processes and formats messages from the Claude runner
type OutputProcessor struct {
	mode   OutputMode
	writer io.Writer
	state  *AppState
	result *Result       // Final result with cost, duration, turns
	colors *ColorScheme  // Terminal color scheme
}

// NewOutputProcessor creates a new output processor
func NewOutputProcessor(format string, verbose bool, quiet bool) *OutputProcessor {
	mode := OutputModeText

	if quiet {
		mode = OutputModeQuiet
	} else if verbose {
		mode = OutputModeVerbose
	} else if format == "json" {
		mode = OutputModeJSON
	}

	return &OutputProcessor{
		mode:   mode,
		writer: os.Stdout,
		state:  NewAppState(),
		colors: GetScheme(),
	}
}

// ProcessMessages consumes messages from the channel and outputs them
func (p *OutputProcessor) ProcessMessages(messages <-chan interface{}, errors <-chan error) {
	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				// Channel closed, processing complete
				p.printFinalSummary()
				return
			}
			p.processMessage(msg)

		case err := <-errors:
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}
}

// processMessage handles a single message based on its type
func (p *OutputProcessor) processMessage(msg interface{}) {
	// JSON mode: just output the raw message
	if p.mode == OutputModeJSON {
		data, err := json.Marshal(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling message: %v\n", err)
			return
		}
		fmt.Fprintln(p.writer, string(data))
		return
	}

	// Process by type
	switch m := msg.(type) {
	case *SystemInit:
		p.handleSystemInit(m)
	case *AssistantMessage:
		p.handleAssistantMessage(m)
	case *StreamEvent:
		p.handleStreamEvent(m)
	case *Result:
		p.handleResult(m)
	case *CompactBoundary:
		// Skip compact boundaries in text mode
	}
}

// handleSystemInit processes system initialization messages
func (p *OutputProcessor) handleSystemInit(msg *SystemInit) {
	p.state.InitializeSession(msg)

	if p.mode == OutputModeQuiet {
		return
	}

	c := p.colors
	fmt.Fprintf(p.writer, "%s[Session started: %s]%s\n", c.SessionInfo, msg.Model, c.Reset)
	// Show initial agent state
	p.printAgentContext()
	// Add spacing after session info
	fmt.Fprintln(p.writer)
}

// handleAssistantMessage processes complete assistant messages
func (p *OutputProcessor) handleAssistantMessage(msg *AssistantMessage) {
	// Update tokens
	if msg.Message.Usage != nil {
		p.state.UpdateTokens(msg.Message.Usage)
	}

	// Clear streaming state
	p.state.ClearStreamState()

	// Process content blocks
	for _, block := range msg.Message.Content {
		p.processContentBlock(&block)
	}
}

// handleStreamEvent processes streaming events
func (p *OutputProcessor) handleStreamEvent(event *StreamEvent) {
	switch event.Type {
	case StreamEventContentBlockStart:
		p.handleContentBlockStart(event)

	case StreamEventContentBlockDelta:
		p.handleContentBlockDelta(event)

	case StreamEventContentBlockStop:
		// Content block finished streaming
		// Reset colors after thinking blocks
		if p.state.Stream.PartialThinking != "" {
			fmt.Fprint(p.writer, p.colors.Reset)
			fmt.Fprintln(p.writer)
		}

	case StreamEventMessageDelta:
		// Update usage if provided
		if event.Usage != nil {
			p.state.UpdateTokens(event.Usage)
		}

	case StreamEventMessageStop:
		// Message streaming complete
	}
}

// handleContentBlockStart handles the start of a new content block
func (p *OutputProcessor) handleContentBlockStart(event *StreamEvent) {
	if event.ContentBlock == nil {
		return
	}

	block := event.ContentBlock

	// Handle tool_use blocks
	if block.Type == ContentBlockTypeToolUse {
		toolCall := &ToolCall{
			ID:     block.ID,
			Name:   block.Name,
			Input:  block.Input,
			Status: ToolCallStatusPending,
		}
		p.state.AddOrUpdateToolCall(toolCall)

		// If this is a Task tool call, it spawns a child agent
		if block.Name == "Task" {
			// Parse input to extract subagent_type and description
			var inputMap map[string]interface{}
			if len(block.Input) > 0 {
				// Ignore error - continue with empty map on failure (graceful degradation)
				_ = json.Unmarshal(block.Input, &inputMap)
			}

			agentType := "task"
			if subtype, ok := inputMap["subagent_type"].(string); ok {
				agentType = subtype
			}

			description := ""
			if desc, ok := inputMap["description"].(string); ok {
				description = desc
			}

			// Create child agent
			p.state.CreateChildAgent(block.ID, agentType, description)
		}

		// Don't print yet - input may be incomplete during streaming
		// Will print when complete message arrives in processContentBlock
	}
}

// handleContentBlockDelta handles streaming deltas
func (p *OutputProcessor) handleContentBlockDelta(event *StreamEvent) {
	if event.Delta == nil {
		return
	}

	delta := event.Delta

	// Stream text content
	if delta.Text != "" {
		p.state.AppendStreamText(delta.Text)
		// Output text in real-time
		fmt.Fprint(p.writer, delta.Text)
	}

	// Stream thinking content
	if delta.Thinking != "" {
		p.state.AppendStreamThinking(delta.Thinking)

		if p.mode != OutputModeQuiet {
			c := p.colors
			// First thinking chunk - print prefix
			if p.state.Stream.PartialThinking == delta.Thinking {
				fmt.Fprintf(p.writer, "%s[THINKING]%s %s", c.ThinkingPrefix, c.Reset, c.ThinkingText)
			}
			fmt.Fprint(p.writer, delta.Thinking)
		}
	}

	// Accumulate partial tool input JSON
	if delta.PartialJSON != "" && event.ContentBlock != nil {
		toolID := event.ContentBlock.ID
		p.state.AppendStreamToolInput(toolID, delta.PartialJSON)
	}
}

// processContentBlock processes a complete content block
func (p *OutputProcessor) processContentBlock(block *ContentBlock) {
	switch block.Type {
	case ContentBlockTypeText:
		// Text already streamed, just ensure newline and spacing
		if block.Text != "" && p.mode != OutputModeQuiet {
			fmt.Fprintln(p.writer)
			fmt.Fprintln(p.writer) // Add extra blank line after text blocks
		}

	case ContentBlockTypeThinking:
		if p.mode != OutputModeQuiet {
			c := p.colors
			fmt.Fprintf(p.writer, "%s[THINKING]%s %s%s%s\n", c.ThinkingPrefix, c.Reset, c.ThinkingText, block.Thinking, c.Reset)
			fmt.Fprintln(p.writer) // Add spacing after thinking blocks
		}

	case ContentBlockTypeToolUse:
		// Print tool use now that we have complete input
		if p.mode != OutputModeQuiet {
			// Update the tool call with complete input
			if tc, ok := p.state.PendingTools[block.ID]; ok {
				tc.Input = block.Input

				// If this is a Task tool, switch to child agent and show context
				if tc.Name == "Task" {
					p.state.SetCurrentAgent(block.ID)
					p.printAgentContext()
				}

				p.printToolCall(tc)
			}
		}

	case ContentBlockTypeToolResult:
		p.processToolResult(block)
	}
}

// toolResultHandler handles the output for a specific tool result type
type toolResultHandler func(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock)

// toolResultHandlers maps tool names to their result handlers
var toolResultHandlers = map[string]toolResultHandler{
	"Bash":       handleBashResult,
	"Glob":       handleGlobResult,
	"Grep":       handleGrepResult,
	"WebSearch":  handleWebSearchResult,
	"KillShell":  handleKillShellResult,
	"TaskOutput": handleTaskOutputResult,
}

// processToolResult processes a tool result
func (p *OutputProcessor) processToolResult(block *ContentBlock) {
	p.state.CompleteToolCall(block.ToolUseID, block.Content, block.IsError)

	if p.mode == OutputModeQuiet {
		return
	}

	// Find the tool call to get its name
	toolCall, ok := p.state.PendingTools[block.ToolUseID]
	if !ok {
		return
	}

	// If this is a Task tool result, switch back to parent agent
	if toolCall.Name == "Task" {
		// Find the child agent
		if childAgent, ok := p.state.AgentsByID[block.ToolUseID]; ok {
			// Mark child as completed
			childAgent.Status = AgentStatusCompleted

			// Switch back to parent
			if childAgent.ParentID != "" {
				if parentAgent, ok := p.state.AgentsByID[childAgent.ParentID]; ok {
					p.state.CurrentAgent = parentAgent
					p.printAgentContext()
				}
			}
		}
	}

	// Dispatch to tool-specific handler if available
	if handler, exists := toolResultHandlers[toolCall.Name]; exists {
		handler(p, toolCall, block)
		return
	}

	// Default handling for other tools (including Read/Write)
	// Note: tool_result blocks are not streamed by claude CLI, so this
	// is mainly for any future tools that do expose results
	handleDefaultResult(p, toolCall, block)
}

// handleBashResult handles Bash tool results - always show output
func handleBashResult(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock) {
	c := p.colors
	if block.Content != "" {
		// Indent and display output, preserving ANSI colors
		lines := strings.Split(block.Content, "\n")
		for _, line := range lines {
			// Show all lines, even empty ones, to preserve output structure
			fmt.Fprintf(p.writer, "  %s\n", line)
		}
	}

	// Show error indicator if it failed
	if block.IsError {
		fmt.Fprintf(p.writer, "  %s✗ Command failed%s\n", c.Error, c.Reset)
	}
}

// handleGlobResult handles Glob tool results - show file paths found
func handleGlobResult(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock) {
	c := p.colors
	if block.Content != "" {
		lines := strings.Split(block.Content, "\n")
		fileCount := 0
		for _, line := range lines {
			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(p.writer, "  %s%s%s\n", c.FilePath, line, c.Reset)
			fileCount++
		}

		// Show count summary if no files or error
		if fileCount == 0 && !block.IsError {
			fmt.Fprintf(p.writer, "  %s(no matches)%s\n", c.LabelDim, c.Reset)
		}
	} else if !block.IsError {
		fmt.Fprintf(p.writer, "  %s(no matches)%s\n", c.LabelDim, c.Reset)
	}

	if block.IsError {
		fmt.Fprintf(p.writer, "  %s✗ Search failed%s\n", c.Error, c.Reset)
	}
}

// handleGrepResult handles Grep tool results - show matches with file:line format
func handleGrepResult(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock) {
	c := p.colors
	if block.Content != "" {
		lines := strings.Split(block.Content, "\n")
		matchCount := 0
		for _, line := range lines {
			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(p.writer, "  %s\n", line)
			matchCount++
		}

		// Show count summary if no matches or error
		if matchCount == 0 && !block.IsError {
			fmt.Fprintf(p.writer, "  %s(no matches)%s\n", c.LabelDim, c.Reset)
		}
	} else if !block.IsError {
		fmt.Fprintf(p.writer, "  %s(no matches)%s\n", c.LabelDim, c.Reset)
	}

	if block.IsError {
		fmt.Fprintf(p.writer, "  %s✗ Search failed%s\n", c.Error, c.Reset)
	}
}

// handleWebSearchResult handles WebSearch tool results - show search results summary
func handleWebSearchResult(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock) {
	c := p.colors
	if block.Content != "" {
		lines := strings.Split(block.Content, "\n")
		for _, line := range lines {
			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(p.writer, "  %s\n", line)
		}
	} else if !block.IsError {
		fmt.Fprintf(p.writer, "  %s(no results)%s\n", c.LabelDim, c.Reset)
	}

	if block.IsError {
		fmt.Fprintf(p.writer, "  %s✗ Search failed%s\n", c.Error, c.Reset)
	}
}

// handleKillShellResult handles KillShell tool results - show termination status
func handleKillShellResult(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock) {
	c := p.colors
	if block.IsError {
		fmt.Fprintf(p.writer, "  %s✗ Shell termination failed%s\n", c.Error, c.Reset)
	} else {
		fmt.Fprintf(p.writer, "  %s✓ Shell terminated%s\n", c.Success, c.Reset)
	}

	// Show output content if present (typically includes success/failure details)
	if p.mode == OutputModeVerbose && block.Content != "" {
		lines := strings.Split(block.Content, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				fmt.Fprintf(p.writer, "  %s\n", line)
			}
		}
	}
}

// handleTaskOutputResult handles TaskOutput tool results - show task output or status
func handleTaskOutputResult(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock) {
	c := p.colors
	if block.Content != "" {
		// Display the task output
		lines := strings.Split(block.Content, "\n")
		for _, line := range lines {
			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(p.writer, "  %s\n", line)
		}
	} else if block.IsError {
		fmt.Fprintf(p.writer, "  %s✗ Failed to retrieve task output%s\n", c.Error, c.Reset)
	}
	// No output case is silent - the tool just returns nothing useful to display
}

// handleDefaultResult handles tool results for tools without specific handlers
func handleDefaultResult(p *OutputProcessor, toolCall *ToolCall, block *ContentBlock) {
	c := p.colors
	statusColor := c.Success
	status := "✓"
	if block.IsError {
		statusColor = c.Error
		status = "✗"
	}

	fmt.Fprintf(p.writer, "  %s%s%s %s completed\n", statusColor, status, c.Reset, toolCall.Name)

	// Show result in verbose mode
	if p.mode == OutputModeVerbose && block.Content != "" {
		// Indent result content
		lines := strings.Split(block.Content, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Fprintf(p.writer, "    %s\n", line)
			}
		}
	}
}

// printDiff prints a unified diff-style output for Edit operations
func (p *OutputProcessor) printDiff(oldStr, newStr string) {
	c := p.colors

	// Split into lines for diff display
	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	// Remove trailing empty line if present (from final \n)
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	// Print removed lines (old)
	for _, line := range oldLines {
		fmt.Fprintf(p.writer, "  %s- %s%s\n", c.DiffRemove, line, c.Reset)
	}

	// Print added lines (new)
	for _, line := range newLines {
		fmt.Fprintf(p.writer, "  %s+ %s%s\n", c.DiffAdd, line, c.Reset)
	}
}

// printToolCall prints a tool call
func (p *OutputProcessor) printToolCall(toolCall *ToolCall) {
	c := p.colors

	// Parse input to extract parameters
	var inputMap map[string]interface{}
	if len(toolCall.Input) > 0 {
		// Ignore error - continue with empty map on failure (graceful degradation)
		_ = json.Unmarshal(toolCall.Input, &inputMap)
	}

	// Handle Bash tool specially
	if toolCall.Name == "Bash" {
		if command, ok := inputMap["command"].(string); ok {
			// Show command as the primary info
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, command)

			// In verbose mode, also show description if available
			if p.mode == OutputModeVerbose {
				if desc, ok := inputMap["description"].(string); ok && desc != "" {
					fmt.Fprintf(p.writer, "  %sDescription:%s %s\n", c.LabelDim, c.Reset, desc)
				}
			}
			return
		}
	}

	// Handle Read tool specially
	if toolCall.Name == "Read" {
		if filePath, ok := inputMap["file_path"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, filePath, c.Reset)
			return
		}
	}

	// Handle Write tool specially
	if toolCall.Name == "Write" {
		if filePath, ok := inputMap["file_path"].(string); ok {
			// Count lines in the content to be written
			lineCount := 0
			if content, ok := inputMap["content"].(string); ok {
				lineCount = strings.Count(content, "\n")
				// If content doesn't end with newline, add 1 for the last line
				if !strings.HasSuffix(content, "\n") && len(content) > 0 {
					lineCount++
				}
			}

			if lineCount > 0 {
				fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s %s(%d lines)%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, filePath, c.Reset, c.LabelDim, lineCount, c.Reset)
			} else {
				fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, filePath, c.Reset)
			}
			return
		}
	}

	// Handle Edit tool specially
	if toolCall.Name == "Edit" {
		if filePath, ok := inputMap["file_path"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, filePath, c.Reset)

			// Show diff if we have old and new strings
			oldStr, hasOld := inputMap["old_string"].(string)
			newStr, hasNew := inputMap["new_string"].(string)

			if hasOld && hasNew {
				p.printDiff(oldStr, newStr)
			}
			return
		}
	}

	// Handle MultiEdit tool specially
	if toolCall.Name == "MultiEdit" {
		if filePath, ok := inputMap["file_path"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, filePath, c.Reset)

			// Show diff for each edit if we have edits array
			if edits, ok := inputMap["edits"].([]interface{}); ok {
				for i, edit := range edits {
					if editMap, ok := edit.(map[string]interface{}); ok {
						oldStr, hasOld := editMap["old_string"].(string)
						newStr, hasNew := editMap["new_string"].(string)

						if hasOld && hasNew {
							if i > 0 {
								fmt.Fprintf(p.writer, "  %s---%s\n", c.LabelDim, c.Reset)
							}
							p.printDiff(oldStr, newStr)
						}
					}
				}
			}
			return
		}
	}

	// Handle Glob tool specially
	if toolCall.Name == "Glob" {
		if pattern, ok := inputMap["pattern"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, pattern)
			return
		}
	}

	// Handle Grep tool specially
	if toolCall.Name == "Grep" {
		if pattern, ok := inputMap["pattern"].(string); ok {
			// Also include any filters or options if present
			filters := ""
			if glob, ok := inputMap["glob"].(string); ok && glob != "" {
				filters += fmt.Sprintf(" %s[glob: %s]%s", c.LabelDim, glob, c.Reset)
			}
			if fileType, ok := inputMap["type"].(string); ok && fileType != "" {
				filters += fmt.Sprintf(" %s[type: %s]%s", c.LabelDim, fileType, c.Reset)
			}
			if path, ok := inputMap["path"].(string); ok && path != "" && path != "." {
				filters += fmt.Sprintf(" %s[path: %s]%s", c.LabelDim, path, c.Reset)
			}

			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, pattern, filters)
			return
		}
	}

	// Handle LS tool specially - display path
	if toolCall.Name == "LS" {
		if path, ok := inputMap["path"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, path, c.Reset)

			// In verbose mode, show stat details if present
			if p.mode == OutputModeVerbose {
				if stat, ok := inputMap["stat"].(bool); ok && stat {
					fmt.Fprintf(p.writer, "  %s[with file stats]%s\n", c.LabelDim, c.Reset)
				}
			}
			return
		}
	}

	// Handle NotebookRead tool specially - display notebook path
	if toolCall.Name == "NotebookRead" {
		if notebookPath, ok := inputMap["notebook_path"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, notebookPath, c.Reset)

			// Show offset and limit if present (for partial reads)
			offset, hasOffset := inputMap["offset"].(float64)
			limit, hasLimit := inputMap["limit"].(float64)
			if hasOffset || hasLimit {
				rangeInfo := ""
				if hasOffset {
					rangeInfo += fmt.Sprintf("offset: %.0f", offset)
				}
				if hasLimit {
					if rangeInfo != "" {
						rangeInfo += ", "
					}
					rangeInfo += fmt.Sprintf("limit: %.0f", limit)
				}
				fmt.Fprintf(p.writer, "  %s[%s]%s\n", c.LabelDim, rangeInfo, c.Reset)
			}
			return
		}
	}

	// Handle NotebookEdit tool specially - display notebook path, cell ID, and edit mode
	if toolCall.Name == "NotebookEdit" {
		if notebookPath, ok := inputMap["notebook_path"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.FilePath, notebookPath, c.Reset)

			// Show cell ID and edit mode
			cellID, hasCellID := inputMap["cell_id"].(string)
			editMode, hasEditMode := inputMap["edit_mode"].(string)

			if hasCellID {
				fmt.Fprintf(p.writer, "  %sCell:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, cellID, c.Reset)
			}

			if hasEditMode {
				// Map edit mode to display values
				modeDisplay := editMode
				switch editMode {
				case "replace":
					modeDisplay = "replace"
				case "insert":
					modeDisplay = "insert"
				case "delete":
					modeDisplay = "delete"
				}
				fmt.Fprintf(p.writer, "  %sMode:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, modeDisplay, c.Reset)
			}

			// In verbose mode, show cell type if present
			if p.mode == OutputModeVerbose {
				if cellType, ok := inputMap["cell_type"].(string); ok && cellType != "" {
					fmt.Fprintf(p.writer, "  %sType:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, cellType, c.Reset)
				}
			}
			return
		}
	}

	// Handle Skill tool specially - display skill name (e.g., /commit, /review-pr)
	if toolCall.Name == "Skill" {
		if skillName, ok := inputMap["skill"].(string); ok {
			// Format as /skill-name for consistency with how users invoke skills
			displayName := fmt.Sprintf("/%s", skillName)

			// Include args if provided
			if args, ok := inputMap["args"].(string); ok && args != "" {
				fmt.Fprintf(p.writer, "%s→%s %s%s%s %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, displayName, c.Reset, c.LabelDim, args, c.Reset)
			} else {
				fmt.Fprintf(p.writer, "%s→%s %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, displayName, c.Reset)
			}
			return
		}
	}

	// Handle WebFetch tool specially - display URL and prompt in readable format
	if toolCall.Name == "WebFetch" {
		url, hasURL := inputMap["url"].(string)
		prompt, hasPrompt := inputMap["prompt"].(string)

		fmt.Fprintf(p.writer, "%s→%s %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset)

		if hasURL {
			fmt.Fprintf(p.writer, "  %sURL:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, url, c.Reset)
		}

		if hasPrompt {
			// Truncate prompt if too long (show first 120 chars)
			displayPrompt := prompt
			if len(prompt) > 120 {
				displayPrompt = prompt[:117] + "..."
			}
			fmt.Fprintf(p.writer, "  %sPrompt:%s %s\n", c.LabelDim, c.Reset, displayPrompt)

			// In verbose mode, show full prompt if it was truncated
			if p.mode == OutputModeVerbose && len(prompt) > 120 {
				// Wrap long prompts to 80 chars per line
				words := strings.Fields(prompt)
				var lines []string
				var currentLine strings.Builder

				for _, word := range words {
					if currentLine.Len() > 0 && currentLine.Len()+len(word)+1 > 80 {
						lines = append(lines, currentLine.String())
						currentLine.Reset()
					}
					if currentLine.Len() > 0 {
						currentLine.WriteString(" ")
					}
					currentLine.WriteString(word)
				}
				if currentLine.Len() > 0 {
					lines = append(lines, currentLine.String())
				}

				fmt.Fprintf(p.writer, "  %sFull prompt:%s\n", c.LabelDim, c.Reset)
				for _, line := range lines {
					fmt.Fprintf(p.writer, "    %s\n", line)
				}
			}
		}
		return
	}

	// Handle WebSearch tool specially - display query and filters
	if toolCall.Name == "WebSearch" {
		query, hasQuery := inputMap["query"].(string)

		fmt.Fprintf(p.writer, "%s→%s %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset)

		if hasQuery {
			fmt.Fprintf(p.writer, "  %sQuery:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, query, c.Reset)
		}

		// Show allowed domains if present
		if allowedDomains, ok := inputMap["allowed_domains"].([]interface{}); ok && len(allowedDomains) > 0 {
			domains := make([]string, 0, len(allowedDomains))
			for _, d := range allowedDomains {
				if domain, ok := d.(string); ok {
					domains = append(domains, domain)
				}
			}
			if len(domains) > 0 {
				fmt.Fprintf(p.writer, "  %sAllowed:%s %s\n", c.LabelDim, c.Reset, strings.Join(domains, ", "))
			}
		}

		// Show blocked domains if present
		if blockedDomains, ok := inputMap["blocked_domains"].([]interface{}); ok && len(blockedDomains) > 0 {
			domains := make([]string, 0, len(blockedDomains))
			for _, d := range blockedDomains {
				if domain, ok := d.(string); ok {
					domains = append(domains, domain)
				}
			}
			if len(domains) > 0 {
				fmt.Fprintf(p.writer, "  %sBlocked:%s %s\n", c.LabelDim, c.Reset, strings.Join(domains, ", "))
			}
		}
		return
	}

	// Handle AskUserQuestion tool specially - display questions with numbered options
	if toolCall.Name == "AskUserQuestion" {
		if questionsRaw, ok := inputMap["questions"].([]interface{}); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset)

			// Print each question with its options
			for i, questionRaw := range questionsRaw {
				if questionMap, ok := questionRaw.(map[string]interface{}); ok {
					question, hasQuestion := questionMap["question"].(string)
					header, hasHeader := questionMap["header"].(string)
					multiSelect, _ := questionMap["multiSelect"].(bool)

					if hasQuestion {
						// Print question header with index if multiple questions
						if len(questionsRaw) > 1 {
							if hasHeader {
								fmt.Fprintf(p.writer, "  %s[%s]%s %s\n", c.LabelDim, header, c.Reset, question)
							} else {
								fmt.Fprintf(p.writer, "  %s[Q%d]%s %s\n", c.LabelDim, i+1, c.Reset, question)
							}
						} else {
							if hasHeader {
								fmt.Fprintf(p.writer, "  %s[%s]%s %s\n", c.LabelDim, header, c.Reset, question)
							} else {
								fmt.Fprintf(p.writer, "  %s\n", question)
							}
						}

						// Show multi-select indicator if enabled
						if multiSelect {
							fmt.Fprintf(p.writer, "  %s(multiple selections allowed)%s\n", c.LabelDim, c.Reset)
						}

						// Print options with numbers
						if optionsRaw, ok := questionMap["options"].([]interface{}); ok {
							for j, optionRaw := range optionsRaw {
								if optionMap, ok := optionRaw.(map[string]interface{}); ok {
									label, hasLabel := optionMap["label"].(string)
									description, hasDesc := optionMap["description"].(string)

									if hasLabel {
										fmt.Fprintf(p.writer, "    %s%d.%s %s\n", c.LabelDim, j+1, c.Reset, label)

										// Show description in verbose mode
										if p.mode == OutputModeVerbose && hasDesc && description != "" {
											fmt.Fprintf(p.writer, "       %s%s%s\n", c.LabelDim, description, c.Reset)
										}
									}
								}
							}
						}

						// Add spacing between questions if there are multiple
						if i < len(questionsRaw)-1 {
							fmt.Fprintln(p.writer)
						}
					}
				}
			}
			return
		}
	}

	// Handle TodoWrite tool specially - display todos in a prettified list format
	if toolCall.Name == "TodoWrite" {
		if todosRaw, ok := inputMap["todos"].([]interface{}); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset)

			// Print each todo item with status indicator
			for _, todoRaw := range todosRaw {
				if todoMap, ok := todoRaw.(map[string]interface{}); ok {
					content, hasContent := todoMap["content"].(string)
					status, hasStatus := todoMap["status"].(string)

					if hasContent && hasStatus {
						// Choose status indicator
						statusIcon := "○" // pending
						statusColor := c.LabelDim
						switch status {
						case "in_progress":
							statusIcon = "◐"
							statusColor = c.ValueBright
						case "completed":
							statusIcon = "●"
							statusColor = c.Success
						}

						// Print the todo item
						fmt.Fprintf(p.writer, "  %s%s%s %s\n", statusColor, statusIcon, c.Reset, content)
					}
				}
			}
			return
		}
	}

	// Handle Task tool specially - display subagent type, description, model, and background flag
	if toolCall.Name == "Task" {
		subagentType := ""
		if st, ok := inputMap["subagent_type"].(string); ok {
			subagentType = st
		}

		description := ""
		if desc, ok := inputMap["description"].(string); ok {
			description = desc
		}

		// Build the output line
		fmt.Fprintf(p.writer, "%s→%s %s%s%s", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset)

		// Show subagent type if present
		if subagentType != "" {
			fmt.Fprintf(p.writer, " [%s]", subagentType)
		}

		// Show description if present
		if description != "" {
			fmt.Fprintf(p.writer, ": %s", description)
		}

		fmt.Fprintln(p.writer)

		// Show model override if present
		if model, ok := inputMap["model"].(string); ok && model != "" {
			fmt.Fprintf(p.writer, "  %sModel:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, model, c.Reset)
		}

		// Show background flag if true
		if runInBackground, ok := inputMap["run_in_background"].(bool); ok && runInBackground {
			fmt.Fprintf(p.writer, "  %sBackground:%s %srunning%s\n", c.LabelDim, c.Reset, c.LabelDim, c.Reset)
		}

		// Show max_turns if present
		if maxTurns, ok := inputMap["max_turns"].(float64); ok && maxTurns > 0 {
			fmt.Fprintf(p.writer, "  %sMax turns:%s %s%.0f%s\n", c.LabelDim, c.Reset, c.ValueBright, maxTurns, c.Reset)
		}

		return
	}

	// Handle KillShell tool specially - display shell ID being terminated
	if toolCall.Name == "KillShell" {
		if shellID, ok := inputMap["shell_id"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.ValueBright, shellID, c.Reset)
			return
		}
	}

	// Handle TaskOutput tool specially - display task ID and blocking mode
	if toolCall.Name == "TaskOutput" {
		if taskID, ok := inputMap["task_id"].(string); ok {
			fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s%s%s", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.ValueBright, taskID, c.Reset)

			// Show blocking mode if present
			if block, ok := inputMap["block"].(bool); ok && block {
				fmt.Fprintf(p.writer, " %s[blocking]%s", c.LabelDim, c.Reset)
			} else if block, ok := inputMap["block"].(string); ok && block == "true" {
				fmt.Fprintf(p.writer, " %s[blocking]%s", c.LabelDim, c.Reset)
			}
			fmt.Fprintln(p.writer)

			// Show timeout if present (in verbose mode only)
			if p.mode == OutputModeVerbose {
				if timeout, ok := inputMap["timeout"].(float64); ok && timeout > 0 {
					fmt.Fprintf(p.writer, "  %sTimeout:%s %s%.0fms%s\n", c.LabelDim, c.Reset, c.ValueBright, timeout, c.Reset)
				}
			}
			return
		}
	}

	// Handle EnterPlanMode tool specially - display plan mode entry
	if toolCall.Name == "EnterPlanMode" {
		fmt.Fprintf(p.writer, "%s→%s %s[PLAN MODE]%s Entering plan mode\n", c.ToolArrow, c.Reset, c.ValueBright, c.Reset)
		return
	}

	// Handle ExitPlanMode tool specially - display plan status and requested permissions
	if toolCall.Name == "ExitPlanMode" {
		fmt.Fprintf(p.writer, "%s→%s %s[PLAN MODE]%s Exiting plan mode\n", c.ToolArrow, c.Reset, c.ValueBright, c.Reset)

		// Show requested permissions if present
		if allowedPrompts, ok := inputMap["allowedPrompts"].([]interface{}); ok && len(allowedPrompts) > 0 {
			fmt.Fprintf(p.writer, "  %sRequested permissions:%s\n", c.LabelDim, c.Reset)
			for _, promptRaw := range allowedPrompts {
				if promptMap, ok := promptRaw.(map[string]interface{}); ok {
					if tool, ok := promptMap["tool"].(string); ok {
						if prompt, ok := promptMap["prompt"].(string); ok {
							fmt.Fprintf(p.writer, "    %s%s:%s %s\n", c.ToolName, tool, c.Reset, prompt)
						}
					}
				}
			}
		}

		// Show remote push info if present
		if pushToRemote, ok := inputMap["pushToRemote"].(bool); ok && pushToRemote {
			fmt.Fprintf(p.writer, "  %sRemote sync:%s enabled\n", c.LabelDim, c.Reset)
			if sessionID, ok := inputMap["remoteSessionId"].(string); ok && sessionID != "" {
				fmt.Fprintf(p.writer, "    %sSession ID:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, sessionID, c.Reset)
			}
			if sessionURL, ok := inputMap["remoteSessionUrl"].(string); ok && sessionURL != "" {
				fmt.Fprintf(p.writer, "    %sSession URL:%s %s%s%s\n", c.LabelDim, c.Reset, c.ValueBright, sessionURL, c.Reset)
			}
		}

		return
	}

	// Default rendering for other tools
	description := ""
	if desc, ok := inputMap["description"].(string); ok {
		description = desc
	}

	// Format status
	statusStr := string(toolCall.Status)

	if description != "" {
		fmt.Fprintf(p.writer, "%s→%s %s%s%s: %s %s[%s]%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, description, c.ToolStatus, statusStr, c.Reset)
	} else {
		fmt.Fprintf(p.writer, "%s→%s %s%s%s %s[%s]%s\n", c.ToolArrow, c.Reset, c.ToolName, toolCall.Name, c.Reset, c.ToolStatus, statusStr, c.Reset)
	}

	// Show full input in verbose mode
	if p.mode == OutputModeVerbose && len(toolCall.Input) > 0 {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, toolCall.Input, "  ", "  "); err == nil {
			fmt.Fprintf(p.writer, "  Input:\n%s\n", prettyJSON.String())
		}
	}
}

// handleResult processes the final result message
func (p *OutputProcessor) handleResult(msg *Result) {
	// Store the result for final summary
	p.result = msg

	if msg.Usage != nil {
		p.state.TotalTokens = msg.Usage
	}
}

// printAgentContext prints the current agent context with indentation
func (p *OutputProcessor) printAgentContext() {
	if p.mode == OutputModeQuiet {
		return
	}

	agent := p.state.CurrentAgent
	if agent == nil {
		return
	}

	c := p.colors

	// Create indentation based on depth
	indent := strings.Repeat("  ", agent.Depth)

	// Format agent context
	fmt.Fprintf(p.writer, "%s%s[%s%s%s: %s%s%s]%s\n", indent, c.AgentBrackets, c.AgentType, agent.Type, c.AgentBrackets, c.AgentStatus, agent.Status, c.AgentBrackets, c.Reset)

	// Show description if present and in verbose mode
	if p.mode == OutputModeVerbose && agent.Description != "" {
		fmt.Fprintf(p.writer, "%s  %s%s%s\n", indent, c.LabelDim, agent.Description, c.Reset)
	}
}

// printFinalSummary prints the final result summary with tokens, cost, duration, and turns
func (p *OutputProcessor) printFinalSummary() {
	if p.mode == OutputModeQuiet {
		return
	}

	// Check if we have any data to display
	tokens := p.state.TotalTokens
	hasTokens := tokens != nil && (tokens.InputTokens > 0 || tokens.OutputTokens > 0)
	hasResult := p.result != nil && (p.result.TotalCost > 0 || p.result.DurationMS > 0 || p.result.NumTurns > 0)

	if !hasTokens && !hasResult {
		return
	}

	c := p.colors

	fmt.Fprintln(p.writer)
	fmt.Fprintf(p.writer, "%s───────────────────────────────────────%s\n", c.Separator, c.Reset)

	// Token summary
	if hasTokens {
		// Calculate total if not provided
		if tokens.TotalTokens == 0 {
			tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens
		}

		fmt.Fprintf(p.writer, "%sTokens:%s %s%d%s total %s(%d in, %d out)%s\n",
			c.LabelDim, c.Reset, c.ValueBright, tokens.TotalTokens, c.Reset, c.LabelDim, tokens.InputTokens, tokens.OutputTokens, c.Reset)

		// Cache info
		if tokens.CacheReadInputTokens > 0 || tokens.CacheCreationInputTokens > 0 {
			fmt.Fprintf(p.writer, "%sCache:%s ", c.LabelDim, c.Reset)
			if tokens.CacheReadInputTokens > 0 {
				fmt.Fprintf(p.writer, "%s%d%s read", c.ValueBright, tokens.CacheReadInputTokens, c.Reset)
				if tokens.CacheCreationInputTokens > 0 {
					fmt.Fprintf(p.writer, ", %s%d%s created", c.ValueBright, tokens.CacheCreationInputTokens, c.Reset)
				}
			} else if tokens.CacheCreationInputTokens > 0 {
				fmt.Fprintf(p.writer, "%s%d%s created", c.ValueBright, tokens.CacheCreationInputTokens, c.Reset)
			}
			fmt.Fprintln(p.writer)
		}
	}

	// Cost, duration, and turns from Result
	if p.result != nil {
		// Cost
		if p.result.TotalCost > 0 {
			fmt.Fprintf(p.writer, "%sCost:%s %s$%.4f%s\n", c.LabelDim, c.Reset, c.ValueBright, p.result.TotalCost, c.Reset)
		}

		// Duration
		if p.result.DurationMS > 0 {
			duration := p.result.DurationMS
			if duration >= 60000 {
				// Show as minutes:seconds for durations >= 1 minute
				mins := duration / 60000
				secs := (duration % 60000) / 1000
				fmt.Fprintf(p.writer, "%sDuration:%s %s%dm %ds%s\n", c.LabelDim, c.Reset, c.ValueBright, mins, secs, c.Reset)
			} else if duration >= 1000 {
				// Show as seconds for durations >= 1 second
				fmt.Fprintf(p.writer, "%sDuration:%s %s%.1fs%s\n", c.LabelDim, c.Reset, c.ValueBright, float64(duration)/1000, c.Reset)
			} else {
				// Show as milliseconds for very short durations
				fmt.Fprintf(p.writer, "%sDuration:%s %s%dms%s\n", c.LabelDim, c.Reset, c.ValueBright, duration, c.Reset)
			}
		}

		// Turns
		if p.result.NumTurns > 0 {
			fmt.Fprintf(p.writer, "%sTurns:%s %s%d%s\n", c.LabelDim, c.Reset, c.ValueBright, p.result.NumTurns, c.Reset)
		}
	}
}
