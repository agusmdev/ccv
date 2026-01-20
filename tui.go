package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the Bubble Tea application state
type Model struct {
	// Runner integration
	runner   *ClaudeRunner
	messages []interface{} // History of all messages received

	// UI state
	width  int
	height int
	ready  bool

	// Current display state
	currentMessage string
	systemInfo     *SystemInit
	thinking       string
	toolCalls      []ToolCall

	// Error state
	err error

	// Styles
	styles Styles
}

// Styles contains all Lip Gloss styles for the UI
type Styles struct {
	// Layout styles
	container     lipgloss.Style
	header        lipgloss.Style
	content       lipgloss.Style
	footer        lipgloss.Style

	// Content styles
	systemInfo    lipgloss.Style
	thinking      lipgloss.Style
	assistantText lipgloss.Style
	toolCall      lipgloss.Style
	toolResult    lipgloss.Style
	error         lipgloss.Style

	// UI elements
	border        lipgloss.Border
	primaryColor  lipgloss.Color
	secondaryColor lipgloss.Color
	accentColor   lipgloss.Color
	errorColor    lipgloss.Color
}

// NewModel creates a new Bubble Tea model
func NewModel(runner *ClaudeRunner) Model {
	return Model{
		runner:   runner,
		messages: make([]interface{}, 0),
		styles:   NewStyles(),
	}
}

// NewStyles creates and configures all UI styles
func NewStyles() Styles {
	// Define color palette
	primaryColor := lipgloss.Color("#7C3AED")   // Purple
	secondaryColor := lipgloss.Color("#A78BFA") // Light purple
	accentColor := lipgloss.Color("#10B981")    // Green
	errorColor := lipgloss.Color("#EF4444")     // Red

	border := lipgloss.RoundedBorder()

	return Styles{
		border:         border,
		primaryColor:   primaryColor,
		secondaryColor: secondaryColor,
		accentColor:    accentColor,
		errorColor:     errorColor,

		container: lipgloss.NewStyle().
			Padding(1, 2),

		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			BorderStyle(border).
			BorderBottom(true).
			BorderForeground(secondaryColor).
			Padding(0, 1).
			MarginBottom(1),

		content: lipgloss.NewStyle().
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1),

		footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			BorderStyle(border).
			BorderTop(true).
			BorderForeground(secondaryColor).
			Padding(0, 1).
			MarginTop(1),

		systemInfo: lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true),

		thinking: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Italic(true).
			Padding(0, 1),

		assistantText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")),

		toolCall: lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(0, 1).
			MarginTop(1),

		toolResult: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Padding(0, 1).
			MarginLeft(2),

		error: lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Padding(0, 1),
	}
}

// Messages for Bubble Tea message passing

// MessageReceived wraps a parsed message from the runner
type MessageReceived struct {
	msg interface{}
}

// ErrorOccurred wraps an error from the runner
type ErrorOccurred struct {
	err error
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	// Start listening for messages from the runner
	return tea.Batch(
		waitForMessage(m.runner),
		waitForError(m.runner),
	)
}

// waitForMessage waits for the next message from the runner
func waitForMessage(runner *ClaudeRunner) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-runner.Messages()
		if !ok {
			// Channel closed
			return nil
		}
		return MessageReceived{msg: msg}
	}
}

// waitForError waits for the next error from the runner
func waitForError(runner *ClaudeRunner) tea.Cmd {
	return func() tea.Msg {
		err, ok := <-runner.Errors()
		if !ok {
			// Channel closed
			return nil
		}
		return ErrorOccurred{err: err}
	}
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Stop the runner and quit
			m.runner.Stop()
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		// Handle terminal resize
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update style widths
		m.styles.container = m.styles.container.Width(m.width - 4)
		m.styles.content = m.styles.content.Width(m.width - 8)

	case MessageReceived:
		// Store the message
		m.messages = append(m.messages, msg.msg)

		// Update current state based on message type
		switch typedMsg := msg.msg.(type) {
		case *SystemInit:
			m.systemInfo = typedMsg

		case *AssistantMessage:
			// Process assistant message content
			for _, block := range typedMsg.Message.Content {
				switch block.Type {
				case ContentBlockTypeText:
					m.currentMessage = block.Text
				case ContentBlockTypeThinking:
					m.thinking = block.Thinking
				case ContentBlockTypeToolUse:
					// Add tool call
					m.toolCalls = append(m.toolCalls, ToolCall{
						ID:     block.ID,
						Name:   block.Name,
						Input:  block.Input,
						Status: ToolCallStatusRunning,
					})
				}
			}

		case *StreamEvent:
			// Handle streaming updates
			if typedMsg.Delta != nil {
				if typedMsg.Delta.Text != "" {
					m.currentMessage += typedMsg.Delta.Text
				}
				if typedMsg.Delta.Thinking != "" {
					m.thinking += typedMsg.Delta.Thinking
				}
			}

		case *Result:
			// Final result received - we could mark completion
			m.currentMessage = fmt.Sprintf("Completed: %s (Cost: $%.4f)", typedMsg.Result, typedMsg.TotalCost)
		}

		// Continue waiting for next message
		return m, waitForMessage(m.runner)

	case ErrorOccurred:
		m.err = msg.err
		// Continue waiting for next error
		return m, waitForError(m.runner)
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sections []string

	// Header with system info
	if m.systemInfo != nil {
		header := fmt.Sprintf("Claude Code Viewer - Session: %s | Model: %s",
			m.systemInfo.SessionID[:8], m.systemInfo.Model)
		sections = append(sections, m.styles.header.Render(header))
	} else {
		sections = append(sections, m.styles.header.Render("Claude Code Viewer"))
	}

	// Content area
	var content []string

	// Show thinking if available
	if m.thinking != "" {
		thinkingText := m.styles.thinking.Render(fmt.Sprintf("💭 Thinking: %s", truncate(m.thinking, 100)))
		content = append(content, thinkingText)
	}

	// Show current message
	if m.currentMessage != "" {
		messageText := m.styles.assistantText.Render(m.currentMessage)
		content = append(content, messageText)
	}

	// Show active tool calls
	if len(m.toolCalls) > 0 {
		content = append(content, "")
		content = append(content, m.styles.toolCall.Render("Tool Calls:"))
		for _, tc := range m.toolCalls {
			toolLine := fmt.Sprintf("  → %s (%s)", tc.Name, tc.Status)
			content = append(content, m.styles.toolResult.Render(toolLine))
		}
	}

	// Show errors
	if m.err != nil {
		content = append(content, "")
		content = append(content, m.styles.error.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	contentStr := strings.Join(content, "\n")
	sections = append(sections, m.styles.content.Render(contentStr))

	// Footer with controls
	footer := "Press q or Ctrl+C to quit"
	sections = append(sections, m.styles.footer.Render(footer))

	// Combine all sections
	return m.styles.container.Render(strings.Join(sections, "\n"))
}

// Helper functions

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
