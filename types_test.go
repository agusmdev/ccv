package main

import (
	"testing"
)

func TestParseMessage_SystemInit(t *testing.T) {
	data := []byte(`{"type":"system","subtype":"init","session_id":"test-session","model":"claude-opus-4-5-20251101"}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	sysInit, ok := msg.(*SystemInit)
	if !ok {
		t.Fatalf("expected *SystemInit, got %T", msg)
	}

	if sysInit.SessionID != "test-session" {
		t.Errorf("expected session_id 'test-session', got '%s'", sysInit.SessionID)
	}
	if sysInit.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected model 'claude-opus-4-5-20251101', got '%s'", sysInit.Model)
	}
}

func TestParseMessage_AssistantMessage(t *testing.T) {
	data := []byte(`{"type":"assistant","message":{"id":"msg_01","type":"message","role":"assistant","content":[{"type":"text","text":"Hello!"}]}}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	assistant, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}

	if len(assistant.Message.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(assistant.Message.Content))
	}

	if assistant.Message.Content[0].Type != ContentBlockTypeText {
		t.Errorf("expected content type 'text', got '%s'", assistant.Message.Content[0].Type)
	}
	if assistant.Message.Content[0].Text != "Hello!" {
		t.Errorf("expected text 'Hello!', got '%s'", assistant.Message.Content[0].Text)
	}
}

func TestParseMessage_Result(t *testing.T) {
	data := []byte(`{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.05,"duration_ms":5000,"num_turns":2}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	result, ok := msg.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", msg)
	}

	if result.IsError {
		t.Error("expected is_error to be false")
	}
	if result.TotalCost != 0.05 {
		t.Errorf("expected total_cost 0.05, got %f", result.TotalCost)
	}
	if result.NumTurns != 2 {
		t.Errorf("expected num_turns 2, got %d", result.NumTurns)
	}
}

func TestParseMessage_UserMessage(t *testing.T) {
	data := []byte(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool_123","content":"output"}]}}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	user, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}

	if len(user.Message.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(user.Message.Content))
	}

	if user.Message.Content[0].ToolUseID != "tool_123" {
		t.Errorf("expected tool_use_id 'tool_123', got '%s'", user.Message.Content[0].ToolUseID)
	}
}

func TestParseMessage_StreamEvent(t *testing.T) {
	data := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	event, ok := msg.(*StreamEvent)
	if !ok {
		t.Fatalf("expected *StreamEvent, got %T", msg)
	}

	if event.Type != StreamEventContentBlockDelta {
		t.Errorf("expected type 'content_block_delta', got '%s'", event.Type)
	}
	if event.Delta == nil {
		t.Fatal("expected delta to be non-nil")
	}
	if event.Delta.Text != "Hello" {
		t.Errorf("expected delta text 'Hello', got '%s'", event.Delta.Text)
	}
}

func TestParseMessage_UnknownType(t *testing.T) {
	data := []byte(`{"type":"unknown_type","field":"value"}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	base, ok := msg.(*BaseMessage)
	if !ok {
		t.Fatalf("expected *BaseMessage, got %T", msg)
	}

	if base.Type != "unknown_type" {
		t.Errorf("expected type 'unknown_type', got '%s'", base.Type)
	}
}

func TestAppState_InitializeSession(t *testing.T) {
	state := NewAppState()

	sysInit := &SystemInit{
		SessionID: "test-session",
		Model:     "claude-opus-4-5-20251101",
	}

	state.InitializeSession(sysInit)

	if state.SessionID != "test-session" {
		t.Errorf("expected session_id 'test-session', got '%s'", state.SessionID)
	}
	if state.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected model 'claude-opus-4-5-20251101', got '%s'", state.Model)
	}
	if state.RootAgent == nil {
		t.Fatal("expected root agent to be initialized")
	}
	if state.RootAgent.Type != "main" {
		t.Errorf("expected root agent type 'main', got '%s'", state.RootAgent.Type)
	}
}

func TestAppState_AddAndCompleteToolCall(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	toolCall := &ToolCall{
		ID:     "tool_123",
		Name:   "Read",
		Status: ToolCallStatusPending,
	}

	state.AddOrUpdateToolCall(toolCall)

	if _, ok := state.PendingTools["tool_123"]; !ok {
		t.Error("expected tool call to be in pending tools")
	}

	state.CompleteToolCall("tool_123", "file contents", false)

	tc := state.PendingTools["tool_123"]
	if tc.Status != ToolCallStatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", tc.Status)
	}
	if tc.Result != "file contents" {
		t.Errorf("expected result 'file contents', got '%s'", tc.Result)
	}
}

func TestAppState_StreamState(t *testing.T) {
	state := NewAppState()

	state.AppendStreamText("Hello ")
	state.AppendStreamText("World")

	if state.Stream.PartialText != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", state.Stream.PartialText)
	}

	state.AppendStreamThinking("Let me think...")
	if state.Stream.PartialThinking != "Let me think..." {
		t.Errorf("expected 'Let me think...', got '%s'", state.Stream.PartialThinking)
	}

	state.ClearStreamState()
	if state.Stream.PartialText != "" {
		t.Error("expected partial text to be cleared")
	}
	if state.Stream.PartialThinking != "" {
		t.Error("expected partial thinking to be cleared")
	}
}
