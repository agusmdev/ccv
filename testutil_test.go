package main

import (
	"bytes"
	"encoding/json"
	"io"
)

// mockWriter captures output written to it for testing
type mockWriter struct {
	buf bytes.Buffer
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockWriter) String() string {
	return m.buf.String()
}

func (m *mockWriter) Reset() {
	m.buf.Reset()
}

// newTestOutputProcessor creates an OutputProcessor with a mock writer for testing
func newTestOutputProcessor(mode OutputMode) (*OutputProcessor, *mockWriter) {
	w := &mockWriter{}
	p := &OutputProcessor{
		mode:   mode,
		writer: w,
		state:  NewAppState(),
		colors: NoColorScheme(), // No colors for predictable test output
	}
	return p, w
}

// createTestToolCall creates a ToolCall with the given parameters for testing
func createTestToolCall(id, name string, input map[string]interface{}) *ToolCall {
	var inputJSON json.RawMessage
	if input != nil {
		inputJSON, _ = json.Marshal(input)
	}
	return &ToolCall{
		ID:     id,
		Name:   name,
		Input:  inputJSON,
		Status: ToolCallStatusPending,
	}
}

// createTestStreamEvent creates a StreamEvent for testing
func createTestStreamEvent(eventType StreamEventType, delta *Delta, contentBlock *ContentBlock) *StreamEvent {
	return &StreamEvent{
		Type:         eventType,
		Delta:        delta,
		ContentBlock: contentBlock,
	}
}

// createTestContentBlock creates a ContentBlock for testing
func createTestContentBlock(blockType ContentBlockType, text string) *ContentBlock {
	return &ContentBlock{
		Type: blockType,
		Text: text,
	}
}

// createTestToolUseBlock creates a tool_use ContentBlock for testing
func createTestToolUseBlock(id, name string, input map[string]interface{}) *ContentBlock {
	var inputJSON json.RawMessage
	if input != nil {
		inputJSON, _ = json.Marshal(input)
	}
	return &ContentBlock{
		Type:  ContentBlockTypeToolUse,
		ID:    id,
		Name:  name,
		Input: inputJSON,
	}
}

// createTestToolResultBlock creates a tool_result ContentBlock for testing
func createTestToolResultBlock(toolUseID, content string, isError bool) *ContentBlock {
	return &ContentBlock{
		Type:      ContentBlockTypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

// createTestSystemInit creates a SystemInit message for testing
func createTestSystemInit(sessionID, model string) *SystemInit {
	return &SystemInit{
		Type:      "system",
		Subtype:   "init",
		SessionID: sessionID,
		Model:     model,
	}
}

// createTestAssistantMessage creates an AssistantMessage for testing
func createTestAssistantMessage(content []ContentBlock) *AssistantMessage {
	return &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_test",
			Type:    "message",
			Role:    "assistant",
			Content: content,
		},
	}
}

// createTestResult creates a Result message for testing
func createTestResult(cost float64, duration int64, turns int) *Result {
	return &Result{
		Type:       "result",
		Subtype:    "success",
		TotalCost:  cost,
		DurationMS: duration,
		NumTurns:   turns,
		Usage: &TotalUsage{
			InputTokens:  1000,
			OutputTokens: 500,
		},
	}
}

// discardWriter is an io.Writer that discards all output
type discardWriter struct{}

func (d discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

var _ io.Writer = (*mockWriter)(nil)
var _ io.Writer = discardWriter{}
