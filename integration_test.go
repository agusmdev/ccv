package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestIntegration_FullMessageFlow tests a complete message flow from start to finish
func TestIntegration_FullMessageFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up channels
	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	// Create output processor
	p, w := newTestOutputProcessor(OutputModeText)

	// Start processing in background
	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// Simulate a full Claude session

	// 1. SystemInit
	messages <- createTestSystemInit("test-session-123", "claude-opus-4-5-20251101")

	// 2. StreamEvent: message_start
	messages <- &StreamEvent{
		Type: StreamEventMessageStart,
		Message: &MessageContent{
			ID:      "msg_1",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{},
		},
	}

	// 3. StreamEvent: content_block_start (text)
	messages <- &StreamEvent{
		Type: StreamEventContentBlockStart,
		ContentBlock: &ContentBlock{
			Type: ContentBlockTypeText,
			Text: "",
		},
	}

	// 4. StreamEvent: content_block_delta (text streaming)
	messages <- &StreamEvent{
		Type: StreamEventContentBlockDelta,
		Delta: &Delta{
			Type: "text_delta",
			Text: "Hello",
		},
	}

	messages <- &StreamEvent{
		Type: StreamEventContentBlockDelta,
		Delta: &Delta{
			Type: "text_delta",
			Text: " world!",
		},
	}

	// 5. StreamEvent: content_block_stop
	messages <- &StreamEvent{
		Type: StreamEventContentBlockStop,
	}

	// 6. StreamEvent: message_delta with usage
	messages <- &StreamEvent{
		Type: StreamEventMessageDelta,
		Usage: &Usage{
			InputTokens:  100,
			OutputTokens: 10,
		},
	}

	// 7. StreamEvent: message_stop
	messages <- &StreamEvent{
		Type: StreamEventMessageStop,
	}

	// 8. Result
	messages <- createTestResult(0.002, 1500, 1)

	// Close channels
	close(messages)
	close(errors)

	// Wait for processing to complete
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	// Verify output
	output := w.String()

	// Should contain session info
	if !strings.Contains(output, "Session started") {
		t.Error("expected 'Session started' in output")
	}

	// Should contain streamed text
	if !strings.Contains(output, "Hello") {
		t.Error("expected 'Hello' in output")
	}
	if !strings.Contains(output, "world!") {
		t.Error("expected 'world!' in output")
	}

	// Should contain summary
	if !strings.Contains(output, "Tokens") {
		t.Error("expected 'Tokens' in summary")
	}
}

// TestIntegration_WithToolCalls tests a session with tool calls
func TestIntegration_WithToolCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-with-tools", "claude-opus-4-5-20251101")

	// AssistantMessage with tool use
	toolInput, _ := json.Marshal(map[string]interface{}{
		"file_path": "/path/to/file.go",
	})
	assistantMsg := &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_tools",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{
					Type: ContentBlockTypeText,
					Text: "Let me read that file.",
				},
				{
					Type:  ContentBlockTypeToolUse,
					ID:    "tool_1",
					Name:  "Read",
					Input: toolInput,
				},
			},
		},
	}
	messages <- assistantMsg

	// Result
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Should show tool call
	if !strings.Contains(output, "Read") {
		t.Error("expected 'Read' tool in output")
	}
	if !strings.Contains(output, "/path/to/file.go") {
		t.Error("expected file path in output")
	}
}

// TestIntegration_WithThinking tests a session with thinking blocks
func TestIntegration_WithThinking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-thinking", "claude-opus-4-5-20251101")

	// StreamEvent: thinking content block
	messages <- &StreamEvent{
		Type: StreamEventContentBlockStart,
		ContentBlock: &ContentBlock{
			Type: ContentBlockTypeThinking,
		},
	}

	// Stream thinking delta
	messages <- &StreamEvent{
		Type: StreamEventContentBlockDelta,
		Delta: &Delta{
			Type:     "thinking_delta",
			Thinking: "Let me analyze this problem...",
		},
	}

	messages <- &StreamEvent{
		Type: StreamEventContentBlockStop,
	}

	// AssistantMessage with complete thinking
	assistantMsg := &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_think",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{
					Type:     ContentBlockTypeThinking,
					Thinking: "Let me analyze this problem...",
				},
			},
		},
	}
	messages <- assistantMsg

	// Result
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Should show thinking
	if !strings.Contains(output, "[THINKING]") {
		t.Error("expected '[THINKING]' in output")
	}
}

// TestIntegration_TaskAgentFlow tests Task agent creation and completion
func TestIntegration_TaskAgentFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-task", "claude-opus-4-5-20251101")

	// Task tool start
	taskInput, _ := json.Marshal(map[string]interface{}{
		"subagent_type": "Explore",
		"description":   "Find all Go files",
	})
	messages <- &StreamEvent{
		Type: StreamEventContentBlockStart,
		ContentBlock: &ContentBlock{
			Type:  ContentBlockTypeToolUse,
			ID:    "task_1",
			Name:  "Task",
			Input: taskInput,
		},
	}

	// AssistantMessage with complete Task tool
	assistantMsg := &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_task",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{
					Type:  ContentBlockTypeToolUse,
					ID:    "task_1",
					Name:  "Task",
					Input: taskInput,
				},
			},
		},
	}
	messages <- assistantMsg

	// Task result (completion)
	messages <- &UserMessage{
		Type: "user",
		Message: UserMessageContent{
			Role: "user",
			Content: []ContentBlock{
				{
					Type:      ContentBlockTypeToolResult,
					ToolUseID: "task_1",
					Content:   "Task completed successfully",
					IsError:   false,
				},
			},
		},
	}

	// Final result
	messages <- createTestResult(0.01, 10000, 3)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Should show Task tool
	if !strings.Contains(output, "Task") {
		t.Error("expected 'Task' tool in output")
	}
	// Should show agent context
	if !strings.Contains(output, "[Explore") {
		t.Error("expected '[Explore' agent context in output")
	}
}

// TestIntegration_MultipleTurns tests multiple conversation turns
func TestIntegration_MultipleTurns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// Turn 1: User -> Assistant
	messages <- createTestSystemInit("session-multi", "claude-opus-4-5-20251101")

	messages <- &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_1",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "First response"},
			},
			Usage: &Usage{
				InputTokens:  50,
				OutputTokens: 10,
			},
		},
	}

	// Turn 2: More assistant response
	messages <- &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_2",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Second response"},
			},
			Usage: &Usage{
				InputTokens:  30,
				OutputTokens: 15,
			},
		},
	}

	// Final result
	result := &Result{
		Type:       "result",
		Subtype:    "success",
		IsError:    false,
		TotalCost:  0.02,
		DurationMS: 5000,
		NumTurns:   2,
		Usage: &TotalUsage{
			InputTokens:  80,
			OutputTokens: 25,
			TotalTokens: 105,
		},
	}
	messages <- result

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Should show both responses
	if !strings.Contains(output, "First response") {
		t.Error("expected 'First response' in output")
	}
	if !strings.Contains(output, "Second response") {
		t.Error("expected 'Second response' in output")
	}

	// Verify token accumulation
	if p.state.TotalTokens.InputTokens != 80 {
		t.Errorf("expected input tokens 80, got %d", p.state.TotalTokens.InputTokens)
	}
}

// TestIntegration_ErrorHandling tests error handling throughout the flow
func TestIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-errors", "claude-opus-4-5-20251101")

	// Send parse error
	errors <- &testError{message: "line 5: failed to parse message"}

	// Continue with valid message
	messages <- &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_1",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Response after error"},
			},
		},
	}

	// Result
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Should show response despite error
	if !strings.Contains(output, "Response after error") {
		t.Error("expected response after error in output")
	}
}

// TestIntegration_JSONMode tests complete flow in JSON mode
func TestIntegration_JSONMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeJSON)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-json", "claude-opus-4-5-20251101")

	// Assistant message
	messages <- &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_1",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "JSON response"},
			},
		},
	}

	// Result
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Each line should be valid JSON
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i, err, line)
		}
	}
}

// TestIntegration_QuietMode tests quiet mode output
func TestIntegration_QuietMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeQuiet)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-quiet", "claude-opus-4-5-20251101")

	// Thinking
	messages <- &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_1",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{Type: ContentBlockTypeThinking, Thinking: "Secret thinking"},
			},
		},
	}

	// Text
	messages <- &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_2",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Visible response"},
			},
		},
	}

	// Result
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Should have the visible response
	if !strings.Contains(output, "Visible response") {
		t.Error("expected visible response in quiet mode")
	}

	// Should NOT have session info or thinking
	if strings.Contains(output, "Session started") {
		t.Error("quiet mode should not show session info")
	}
	if strings.Contains(output, "[THINKING]") {
		t.Error("quiet mode should not show thinking")
	}
	if strings.Contains(output, "Secret thinking") {
		t.Error("quiet mode should not show thinking content")
	}
}

// TestIntegration_VerboseMode tests verbose mode output
func TestIntegration_VerboseMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeVerbose)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-verbose", "claude-opus-4-5-20251101")

	// Tool with verbose details
	toolInput, _ := json.Marshal(map[string]interface{}{
		"command":     "go test ./...",
		"description": "Run all tests",
	})
	messages <- &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_1",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{
				{
					Type:  ContentBlockTypeToolUse,
					ID:    "tool_1",
					Name:  "Bash",
					Input: toolInput,
				},
			},
		},
	}

	// Result
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	output := w.String()

	// Should show tool details in verbose mode
	if !strings.Contains(output, "go test ./...") {
		t.Error("verbose mode should show command")
	}
	if strings.Contains(output, "Description") {
		// Description might be shown in verbose mode
	}
}

// TestIntegration_UserMessages tests user message handling
func TestIntegration_UserMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		p.ProcessMessages(messages, errors)
		close(done)
	}()

	// SystemInit
	messages <- createTestSystemInit("session-user", "claude-opus-4-5-20251101")

	// User message (should be handled gracefully)
	messages <- &UserMessage{
		Type: "user",
		Message: UserMessageContent{
			Role: "user",
			Content: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "user input"},
			},
		},
	}

	// Result
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}

	// User message should be handled without panic
}

// TestIntegration_NilHandlers tests with nil handlers
func TestIntegration_NilHandlers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p := &OutputProcessor{
		mode:   OutputModeText,
		writer: &mockWriter{},
		state:  NewAppState(),
		colors: NoColorScheme(),
	}

	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ProcessMessages with nil state panicked: %v", r)
			}
			close(done)
		}()
		p.ProcessMessages(messages, errors)
	}()

	// Send various messages
	messages <- createTestSystemInit("test", "model")
	messages <- createTestResult(0.001, 1000, 1)

	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Processing did not complete within timeout")
	}
}
