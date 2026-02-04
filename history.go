package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HistoryOptions holds the command line options for history mode
type HistoryOptions struct {
	Path    string
	Since   string
	Last    int
	Project string
}

// HistoryMessage represents a parsed message from a session JSONL
type HistoryMessage struct {
	Type      string    // "user", "assistant", "tool_result"
	Timestamp time.Time
	UUID      string
	ParentUUID string

	// For user messages
	UserText string

	// For assistant messages
	AssistantText string
	ToolCalls     map[string]int // tool name -> count
	InputTokens   int
	OutputTokens  int

	// For identifying tool results
	IsToolResult bool
}

// ConversationTurn represents a single turn in the conversation
type ConversationTurn struct {
	Timestamp     time.Time
	UserPrompt    string
	AssistantText string
	ToolCounts    map[string]int
	InputTokens   int
	OutputTokens  int
}

// HistoryReader reads and processes session JSONL files
type HistoryReader struct {
	colors *ColorScheme
}

// NewHistoryReader creates a new history reader
func NewHistoryReader() *HistoryReader {
	return &HistoryReader{
		colors: GetScheme(),
	}
}

// Run executes the history mode with the given options
func (h *HistoryReader) Run(opts HistoryOptions) error {
	// Determine what we're reading
	var sessions []SessionIndexEntry
	var err error

	if opts.Project != "" {
		// Find project by name in ~/.claude/projects/
		sessions, err = h.findProjectSessions(opts.Project)
		if err != nil {
			return err
		}
	} else if opts.Path != "" {
		if strings.HasSuffix(opts.Path, ".jsonl") {
			// Direct session file
			sessions = []SessionIndexEntry{{FullPath: opts.Path}}
		} else {
			// Directory - read sessions-index.json
			sessions, err = h.readSessionIndex(opts.Path)
			if err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("no path or project specified")
	}

	if len(sessions) == 0 {
		return fmt.Errorf("no sessions found")
	}

	// Apply filters
	sessions = h.filterSessions(sessions, opts)

	if len(sessions) == 0 {
		return fmt.Errorf("no sessions match the specified filters")
	}

	// Sort by modified time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, sessions[i].Modified)
		tj, _ := time.Parse(time.RFC3339, sessions[j].Modified)
		return ti.After(tj)
	})

	// Process and display each session
	for i, session := range sessions {
		if i > 0 {
			fmt.Println()
			fmt.Println(strings.Repeat("=", 60))
			fmt.Println()
		}
		if err := h.processSession(session); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing session %s: %v\n", session.SessionID, err)
		}
	}

	return nil
}

// findProjectSessions searches ~/.claude/projects/ for a project by name
func (h *HistoryReader) findProjectSessions(projectName string) ([]SessionIndexEntry, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	// Search for matching project directories
	var matches []string
	lowerName := strings.ToLower(projectName)
	for _, entry := range entries {
		if entry.IsDir() {
			// Convert directory name to lowercase and check if it contains the project name
			dirLower := strings.ToLower(entry.Name())
			if strings.Contains(dirLower, lowerName) {
				matches = append(matches, filepath.Join(projectsDir, entry.Name()))
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no project found matching '%s'", projectName)
	}

	// Use the first match (or could prompt user if multiple)
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "Found %d matching projects, using: %s\n", len(matches), matches[0])
	}

	return h.readSessionIndex(matches[0])
}

// readSessionIndex reads the sessions-index.json file from a directory
func (h *HistoryReader) readSessionIndex(dirPath string) ([]SessionIndexEntry, error) {
	indexPath := filepath.Join(dirPath, "sessions-index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		// If no index file, try to list .jsonl files directly
		if os.IsNotExist(err) {
			return h.listJSONLFiles(dirPath)
		}
		return nil, fmt.Errorf("failed to read sessions-index.json: %w", err)
	}

	var index SessionIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse sessions-index.json: %w", err)
	}

	return index.Entries, nil
}

// listJSONLFiles lists .jsonl files in a directory as fallback
func (h *HistoryReader) listJSONLFiles(dirPath string) ([]SessionIndexEntry, error) {
	pattern := filepath.Join(dirPath, "*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var entries []SessionIndexEntry
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		entries = append(entries, SessionIndexEntry{
			SessionID: strings.TrimSuffix(filepath.Base(path), ".jsonl"),
			FullPath:  path,
			Modified:  info.ModTime().Format(time.RFC3339),
		})
	}

	return entries, nil
}

// filterSessions applies --since and --last filters
func (h *HistoryReader) filterSessions(sessions []SessionIndexEntry, opts HistoryOptions) []SessionIndexEntry {
	var filtered []SessionIndexEntry

	// Parse --since date if provided
	var sinceTime time.Time
	if opts.Since != "" {
		// Try multiple formats
		formats := []string{
			"2006-01-02",
			"2006-01-02T15:04:05",
			time.RFC3339,
		}
		for _, format := range formats {
			if t, err := time.Parse(format, opts.Since); err == nil {
				sinceTime = t
				break
			}
		}
	}

	for _, session := range sessions {
		// Apply --since filter
		if !sinceTime.IsZero() {
			modified, err := time.Parse(time.RFC3339, session.Modified)
			if err != nil {
				continue
			}
			if modified.Before(sinceTime) {
				continue
			}
		}

		filtered = append(filtered, session)
	}

	// Apply --last filter
	if opts.Last > 0 && len(filtered) > opts.Last {
		// Sort by modified time first
		sort.Slice(filtered, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, filtered[i].Modified)
			tj, _ := time.Parse(time.RFC3339, filtered[j].Modified)
			return ti.After(tj)
		})
		filtered = filtered[:opts.Last]
	}

	return filtered
}

// processSession reads and formats a single session file
func (h *HistoryReader) processSession(entry SessionIndexEntry) error {
	file, err := os.Open(entry.FullPath)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	// Parse all messages
	messages, err := h.parseSessionFile(file)
	if err != nil {
		return fmt.Errorf("failed to parse session: %w", err)
	}

	// Build conversation turns
	turns := h.buildConversationTurns(messages)

	// Calculate session time range
	var startTime, endTime time.Time
	if len(turns) > 0 {
		startTime = turns[0].Timestamp
		endTime = turns[len(turns)-1].Timestamp
	}

	// Print session header
	h.printSessionHeader(entry, startTime, endTime)

	// Print each turn
	var totalIn, totalOut int
	for _, turn := range turns {
		h.printTurn(turn)
		totalIn += turn.InputTokens
		totalOut += turn.OutputTokens
	}

	// Print session footer with totals
	h.printSessionFooter(totalIn, totalOut)

	return nil
}

// parseSessionFile parses a JSONL session file
func (h *HistoryReader) parseSessionFile(reader io.Reader) ([]HistoryMessage, error) {
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var messages []HistoryMessage

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		msg, err := h.parseHistoryLine(line)
		if err != nil {
			// Skip lines we can't parse
			continue
		}

		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	return messages, scanner.Err()
}

// parseHistoryLine parses a single line from a session JSONL file
func (h *HistoryReader) parseHistoryLine(line []byte) (*HistoryMessage, error) {
	// Parse outer envelope
	var envelope struct {
		Type       string          `json:"type"`
		Timestamp  string          `json:"timestamp"`
		UUID       string          `json:"uuid"`
		ParentUUID *string         `json:"parentUuid"`
		Message    json.RawMessage `json:"message"`
	}

	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, err
	}

	// Skip non-message types
	if envelope.Type != "user" && envelope.Type != "assistant" {
		return nil, nil
	}

	timestamp, _ := time.Parse(time.RFC3339, envelope.Timestamp)

	parentUUID := ""
	if envelope.ParentUUID != nil {
		parentUUID = *envelope.ParentUUID
	}

	msg := &HistoryMessage{
		Type:       envelope.Type,
		Timestamp:  timestamp,
		UUID:       envelope.UUID,
		ParentUUID: parentUUID,
	}

	if envelope.Type == "user" {
		return h.parseUserMessage(msg, envelope.Message)
	}

	if envelope.Type == "assistant" {
		return h.parseAssistantMessage(msg, envelope.Message)
	}

	return nil, nil
}

// parseUserMessage parses a user message
func (h *HistoryReader) parseUserMessage(msg *HistoryMessage, rawMessage json.RawMessage) (*HistoryMessage, error) {
	// User message content can be a string or an array (for tool results)
	var userMsg struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}

	if err := json.Unmarshal(rawMessage, &userMsg); err != nil {
		return nil, err
	}

	// Try to parse as string first (actual user prompt)
	var contentStr string
	if err := json.Unmarshal(userMsg.Content, &contentStr); err == nil {
		msg.UserText = contentStr
		return msg, nil
	}

	// Try to parse as array (tool_result)
	var contentArr []struct {
		Type      string `json:"type"`
		ToolUseID string `json:"tool_use_id,omitempty"`
	}
	if err := json.Unmarshal(userMsg.Content, &contentArr); err == nil {
		if len(contentArr) > 0 && contentArr[0].Type == "tool_result" {
			msg.IsToolResult = true
			return msg, nil
		}
	}

	return msg, nil
}

// parseAssistantMessage parses an assistant message
func (h *HistoryReader) parseAssistantMessage(msg *HistoryMessage, rawMessage json.RawMessage) (*HistoryMessage, error) {
	var assistantMsg struct {
		Content []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text,omitempty"`
			Thinking string          `json:"thinking,omitempty"`
			Name     string          `json:"name,omitempty"`
			Input    json.RawMessage `json:"input,omitempty"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(rawMessage, &assistantMsg); err != nil {
		return nil, err
	}

	msg.ToolCalls = make(map[string]int)
	var textParts []string

	for _, block := range assistantMsg.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "tool_use":
			msg.ToolCalls[block.Name]++
		// Skip thinking blocks as per plan
		}
	}

	msg.AssistantText = strings.Join(textParts, "\n")
	msg.InputTokens = assistantMsg.Usage.InputTokens
	msg.OutputTokens = assistantMsg.Usage.OutputTokens

	return msg, nil
}

// buildConversationTurns groups messages into conversation turns
func (h *HistoryReader) buildConversationTurns(messages []HistoryMessage) []ConversationTurn {
	var turns []ConversationTurn

	for i := 0; i < len(messages); i++ {
		msg := messages[i]

		// Skip tool results - they're not user prompts
		if msg.IsToolResult {
			continue
		}

		// Look for user prompt
		if msg.Type == "user" && msg.UserText != "" {
			turn := ConversationTurn{
				Timestamp:  msg.Timestamp,
				UserPrompt: msg.UserText,
				ToolCounts: make(map[string]int),
			}

			// Find all following assistant messages until next user prompt
			for j := i + 1; j < len(messages); j++ {
				nextMsg := messages[j]

				// Stop at next user prompt (that's not a tool result)
				if nextMsg.Type == "user" && !nextMsg.IsToolResult && nextMsg.UserText != "" {
					break
				}

				// Accumulate assistant responses
				if nextMsg.Type == "assistant" {
					if turn.AssistantText != "" && nextMsg.AssistantText != "" {
						turn.AssistantText += "\n\n"
					}
					turn.AssistantText += nextMsg.AssistantText

					// Accumulate tool counts
					for tool, count := range nextMsg.ToolCalls {
						turn.ToolCounts[tool] += count
					}

					// Accumulate tokens
					turn.InputTokens += nextMsg.InputTokens
					turn.OutputTokens += nextMsg.OutputTokens
				}
			}

			turns = append(turns, turn)
		}
	}

	return turns
}

// printSessionHeader prints the session header
func (h *HistoryReader) printSessionHeader(entry SessionIndexEntry, start, end time.Time) {
	c := h.colors

	// Format session time range
	var timeRange string
	if !start.IsZero() && !end.IsZero() {
		duration := end.Sub(start)
		startStr := start.Format("2006-01-02 15:04:05")
		endStr := end.Format("15:04:05")

		if duration >= time.Hour {
			hours := int(duration.Hours())
			mins := int(duration.Minutes()) % 60
			timeRange = fmt.Sprintf("%s - %s (%dh %dm)", startStr, endStr, hours, mins)
		} else if duration >= time.Minute {
			mins := int(duration.Minutes())
			timeRange = fmt.Sprintf("%s - %s (%d min)", startStr, endStr, mins)
		} else {
			timeRange = fmt.Sprintf("%s - %s (<1 min)", startStr, endStr)
		}
	} else {
		timeRange = "unknown"
	}

	fmt.Printf("%sSession:%s %s\n", c.LabelDim, c.Reset, timeRange)
	if entry.ProjectPath != "" {
		fmt.Printf("%sProject:%s %s\n", c.LabelDim, c.Reset, entry.ProjectPath)
	}
	fmt.Println()
	fmt.Println("---")
	fmt.Println()
}

// printTurn prints a single conversation turn
func (h *HistoryReader) printTurn(turn ConversationTurn) {
	c := h.colors

	// Timestamp
	timestamp := turn.Timestamp.Format("15:04:05")
	fmt.Printf("%s[%s]%s\n", c.LabelDim, timestamp, c.Reset)

	// User prompt
	fmt.Printf("%sUser:%s %s\n", c.ValueBright, c.Reset, turn.UserPrompt)
	fmt.Println()

	// Assistant response
	if turn.AssistantText != "" {
		fmt.Printf("%sAssistant:%s %s\n", c.ValueBright, c.Reset, turn.AssistantText)
		fmt.Println()
	}

	// Tool summary
	if len(turn.ToolCounts) > 0 {
		var toolParts []string
		for tool, count := range turn.ToolCounts {
			toolParts = append(toolParts, fmt.Sprintf("%s(%d)", tool, count))
		}
		// Sort for consistent output
		sort.Strings(toolParts)
		fmt.Printf("%sTools:%s %s\n", c.LabelDim, c.Reset, strings.Join(toolParts, ", "))
	}

	// Token usage
	if turn.InputTokens > 0 || turn.OutputTokens > 0 {
		fmt.Printf("%sTokens:%s %s%d%s in / %s%d%s out\n",
			c.LabelDim, c.Reset,
			c.ValueBright, turn.InputTokens, c.Reset,
			c.ValueBright, turn.OutputTokens, c.Reset)
	}

	fmt.Println()
	fmt.Println("---")
	fmt.Println()
}

// printSessionFooter prints the session footer with totals
func (h *HistoryReader) printSessionFooter(totalIn, totalOut int) {
	c := h.colors

	if totalIn > 0 || totalOut > 0 {
		fmt.Printf("%sTotal:%s %s%d%s in / %s%d%s out\n",
			c.LabelDim, c.Reset,
			c.ValueBright, totalIn, c.Reset,
			c.ValueBright, totalOut, c.Reset)
	}
}
