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
		data, _ := json.Marshal(msg)
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

	fmt.Fprintf(p.writer, "[Session started: %s]\n", msg.Model)
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
			// First thinking chunk - print prefix
			if p.state.Stream.PartialThinking == delta.Thinking {
				fmt.Fprint(p.writer, "[THINKING] ")
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
		// Text already streamed, just ensure newline
		if block.Text != "" && p.mode != OutputModeQuiet {
			fmt.Fprintln(p.writer)
		}

	case ContentBlockTypeThinking:
		if p.mode != OutputModeQuiet {
			fmt.Fprintf(p.writer, "[THINKING] %s\n", block.Thinking)
		}

	case ContentBlockTypeToolUse:
		// Print tool use now that we have complete input
		if p.mode != OutputModeQuiet {
			// Update the tool call with complete input
			if tc, ok := p.state.PendingTools[block.ID]; ok {
				tc.Input = block.Input
				p.printToolCall(tc)
			}
		}

	case ContentBlockTypeToolResult:
		p.processToolResult(block)
	}
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

	// Handle Bash tool results specially - always show output
	if toolCall.Name == "Bash" {
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
			fmt.Fprintf(p.writer, "  ✗ Command failed\n")
		}
		return
	}

	// Default handling for other tools (including Read/Write)
	// Note: tool_result blocks are not streamed by claude CLI, so this
	// is mainly for any future tools that do expose results
	status := "✓"
	if block.IsError {
		status = "✗"
	}

	fmt.Fprintf(p.writer, "  %s %s completed\n", status, toolCall.Name)

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
		fmt.Fprintf(p.writer, "  - %s\n", line)
	}

	// Print added lines (new)
	for _, line := range newLines {
		fmt.Fprintf(p.writer, "  + %s\n", line)
	}
}

// printToolCall prints a tool call
func (p *OutputProcessor) printToolCall(toolCall *ToolCall) {
	// Parse input to extract parameters
	var inputMap map[string]interface{}
	if len(toolCall.Input) > 0 {
		json.Unmarshal(toolCall.Input, &inputMap)
	}

	// Handle Bash tool specially
	if toolCall.Name == "Bash" {
		if command, ok := inputMap["command"].(string); ok {
			// Show command as the primary info
			fmt.Fprintf(p.writer, "→ Bash: %s\n", command)

			// In verbose mode, also show description if available
			if p.mode == OutputModeVerbose {
				if desc, ok := inputMap["description"].(string); ok && desc != "" {
					fmt.Fprintf(p.writer, "  Description: %s\n", desc)
				}
			}
			return
		}
	}

	// Handle Read tool specially
	if toolCall.Name == "Read" {
		if filePath, ok := inputMap["file_path"].(string); ok {
			fmt.Fprintf(p.writer, "→ Read: %s\n", filePath)
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
				fmt.Fprintf(p.writer, "→ Write: %s (%d lines)\n", filePath, lineCount)
			} else {
				fmt.Fprintf(p.writer, "→ Write: %s\n", filePath)
			}
			return
		}
	}

	// Handle Edit tool specially
	if toolCall.Name == "Edit" {
		if filePath, ok := inputMap["file_path"].(string); ok {
			fmt.Fprintf(p.writer, "→ Edit: %s\n", filePath)

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
			fmt.Fprintf(p.writer, "→ MultiEdit: %s\n", filePath)

			// Show diff for each edit if we have edits array
			if edits, ok := inputMap["edits"].([]interface{}); ok {
				for i, edit := range edits {
					if editMap, ok := edit.(map[string]interface{}); ok {
						oldStr, hasOld := editMap["old_string"].(string)
						newStr, hasNew := editMap["new_string"].(string)

						if hasOld && hasNew {
							if i > 0 {
								fmt.Fprintf(p.writer, "  ---\n")
							}
							p.printDiff(oldStr, newStr)
						}
					}
				}
			}
			return
		}
	}

	// Default rendering for other tools
	description := ""
	if desc, ok := inputMap["description"].(string); ok {
		description = desc
	}

	// Format status
	statusStr := string(toolCall.Status)

	if description != "" {
		fmt.Fprintf(p.writer, "→ %s: %s [%s]\n", toolCall.Name, description, statusStr)
	} else {
		fmt.Fprintf(p.writer, "→ %s [%s]\n", toolCall.Name, statusStr)
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
	if msg.Usage != nil {
		p.state.TotalTokens = msg.Usage
	}

	if p.mode == OutputModeQuiet {
		return
	}

	// Don't print summary here - will be done in printFinalSummary
}

// printFinalSummary prints the final token usage and cost summary
func (p *OutputProcessor) printFinalSummary() {
	if p.mode == OutputModeQuiet {
		return
	}

	tokens := p.state.TotalTokens
	if tokens == nil {
		return
	}

	// Calculate total if not provided
	if tokens.TotalTokens == 0 {
		tokens.TotalTokens = tokens.InputTokens + tokens.OutputTokens
	}

	if tokens.TotalTokens == 0 {
		return
	}

	fmt.Fprintln(p.writer)
	fmt.Fprintf(p.writer, "───────────────────────────────────────\n")
	fmt.Fprintf(p.writer, "Tokens: %d total (%d in, %d out)\n",
		tokens.TotalTokens, tokens.InputTokens, tokens.OutputTokens)

	if tokens.CacheReadInputTokens > 0 {
		fmt.Fprintf(p.writer, "Cache: %d read", tokens.CacheReadInputTokens)
		if tokens.CacheCreationInputTokens > 0 {
			fmt.Fprintf(p.writer, ", %d created", tokens.CacheCreationInputTokens)
		}
		fmt.Fprintln(p.writer)
	}
}
