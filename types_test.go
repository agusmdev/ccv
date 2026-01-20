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

func TestAppState_CreateChildAgent(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	// Create first child agent
	child := state.CreateChildAgent("tool_1", "Explore", "Exploring codebase")

	if child == nil {
		t.Fatal("expected non-nil child agent")
	}
	if child.ID != "tool_1" {
		t.Errorf("expected ID 'tool_1', got '%s'", child.ID)
	}
	if child.Type != "Explore" {
		t.Errorf("expected type 'Explore', got '%s'", child.Type)
	}
	if child.Description != "Exploring codebase" {
		t.Errorf("expected description, got '%s'", child.Description)
	}
	if child.ParentID != "main" {
		t.Errorf("expected parent ID 'main', got '%s'", child.ParentID)
	}
	if child.Depth != 1 {
		t.Errorf("expected depth 1, got %d", child.Depth)
	}
	if child.Status != AgentStatusRunning {
		t.Errorf("expected status 'running', got '%s'", child.Status)
	}

	// Verify it's registered in lookup map
	if _, ok := state.AgentsByID["tool_1"]; !ok {
		t.Error("expected child to be in AgentsByID map")
	}

	// Create nested child (grandchild)
	state.SetCurrentAgent("tool_1")
	grandchild := state.CreateChildAgent("tool_2", "task", "Sub-task")

	if grandchild.Depth != 2 {
		t.Errorf("expected depth 2 for grandchild, got %d", grandchild.Depth)
	}
	if grandchild.ParentID != "tool_1" {
		t.Errorf("expected parent ID 'tool_1', got '%s'", grandchild.ParentID)
	}
}

func TestAppState_SetCurrentAgent(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	// Create a child agent
	child := state.CreateChildAgent("tool_1", "task", "Test task")

	// Current should still be root
	if state.CurrentAgent.ID != "main" {
		t.Errorf("expected current agent to be 'main', got '%s'", state.CurrentAgent.ID)
	}

	// Set current to child
	state.SetCurrentAgent("tool_1")

	if state.CurrentAgent.ID != "tool_1" {
		t.Errorf("expected current agent to be 'tool_1', got '%s'", state.CurrentAgent.ID)
	}
	if state.CurrentAgent.Status != AgentStatusRunning {
		t.Errorf("expected status 'running', got '%s'", state.CurrentAgent.Status)
	}

	// Set to non-existent agent (should be a no-op)
	state.SetCurrentAgent("nonexistent")
	if state.CurrentAgent.ID != "tool_1" {
		t.Errorf("setting non-existent agent should not change current, got '%s'", state.CurrentAgent.ID)
	}

	// Set back to main
	state.SetCurrentAgent("main")
	if state.CurrentAgent.ID != "main" {
		t.Errorf("expected current agent to be 'main', got '%s'", state.CurrentAgent.ID)
	}

	// Verify child is still accessible
	if child.Type != "task" {
		t.Errorf("child agent should still exist, got type '%s'", child.Type)
	}
}

func TestAppState_UpdateTokens(t *testing.T) {
	state := NewAppState()

	// Test nil usage (should be no-op)
	state.UpdateTokens(nil)
	if state.TotalTokens.InputTokens != 0 {
		t.Error("nil usage should not affect tokens")
	}

	// First update
	usage1 := &Usage{
		InputTokens:              100,
		OutputTokens:             50,
		CacheReadInputTokens:     20,
		CacheCreationInputTokens: 10,
	}
	state.UpdateTokens(usage1)

	if state.TotalTokens.InputTokens != 100 {
		t.Errorf("expected input tokens 100, got %d", state.TotalTokens.InputTokens)
	}
	if state.TotalTokens.OutputTokens != 50 {
		t.Errorf("expected output tokens 50, got %d", state.TotalTokens.OutputTokens)
	}
	if state.TotalTokens.CacheReadInputTokens != 20 {
		t.Errorf("expected cache read tokens 20, got %d", state.TotalTokens.CacheReadInputTokens)
	}
	if state.TotalTokens.CacheCreationInputTokens != 10 {
		t.Errorf("expected cache creation tokens 10, got %d", state.TotalTokens.CacheCreationInputTokens)
	}
	if state.TotalTokens.TotalTokens != 150 {
		t.Errorf("expected total tokens 150, got %d", state.TotalTokens.TotalTokens)
	}

	// Second update (should accumulate)
	usage2 := &Usage{
		InputTokens:  200,
		OutputTokens: 100,
	}
	state.UpdateTokens(usage2)

	if state.TotalTokens.InputTokens != 300 {
		t.Errorf("expected accumulated input tokens 300, got %d", state.TotalTokens.InputTokens)
	}
	if state.TotalTokens.OutputTokens != 150 {
		t.Errorf("expected accumulated output tokens 150, got %d", state.TotalTokens.OutputTokens)
	}
	if state.TotalTokens.TotalTokens != 450 {
		t.Errorf("expected accumulated total tokens 450, got %d", state.TotalTokens.TotalTokens)
	}
}

func TestToolUseResult_UnmarshalJSON_String(t *testing.T) {
	jsonData := []byte(`"simple string result"`)

	var result ToolUseResult
	err := result.UnmarshalJSON(jsonData)
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if result.RawString != "simple string result" {
		t.Errorf("expected RawString 'simple string result', got '%s'", result.RawString)
	}
	if result.Stdout != "" {
		t.Error("expected Stdout to be empty for string form")
	}
}

func TestToolUseResult_UnmarshalJSON_Object(t *testing.T) {
	jsonData := []byte(`{"stdout":"output text","stderr":"error text","interrupted":true,"isImage":false}`)

	var result ToolUseResult
	err := result.UnmarshalJSON(jsonData)
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if result.Stdout != "output text" {
		t.Errorf("expected Stdout 'output text', got '%s'", result.Stdout)
	}
	if result.Stderr != "error text" {
		t.Errorf("expected Stderr 'error text', got '%s'", result.Stderr)
	}
	if !result.Interrupted {
		t.Error("expected Interrupted to be true")
	}
	if result.IsImage {
		t.Error("expected IsImage to be false")
	}
	if result.RawString != "" {
		t.Error("expected RawString to be empty for object form")
	}
}

func TestToolUseResult_UnmarshalJSON_InvalidJSON(t *testing.T) {
	jsonData := []byte(`{invalid json}`)

	var result ToolUseResult
	err := result.UnmarshalJSON(jsonData)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseMessage_InvalidJSON(t *testing.T) {
	data := []byte(`{not valid json}`)

	_, err := ParseMessage(data)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseMessage_EmptyJSON(t *testing.T) {
	data := []byte(`{}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	// Should return BaseMessage for empty/unknown type
	base, ok := msg.(*BaseMessage)
	if !ok {
		t.Fatalf("expected *BaseMessage, got %T", msg)
	}
	if base.Type != "" {
		t.Errorf("expected empty type, got '%s'", base.Type)
	}
}

func TestParseMessage_StreamEventWrapper(t *testing.T) {
	data := []byte(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	// Should unwrap to StreamEvent
	event, ok := msg.(*StreamEvent)
	if !ok {
		t.Fatalf("expected *StreamEvent, got %T", msg)
	}
	if event.Type != StreamEventContentBlockDelta {
		t.Errorf("expected content_block_delta, got '%s'", event.Type)
	}
}

func TestParseMessage_CompactBoundary(t *testing.T) {
	data := []byte(`{"type":"compact_boundary","subtype":"context_trim","session_id":"test-123"}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	boundary, ok := msg.(*CompactBoundary)
	if !ok {
		t.Fatalf("expected *CompactBoundary, got %T", msg)
	}
	if boundary.Subtype != "context_trim" {
		t.Errorf("expected subtype 'context_trim', got '%s'", boundary.Subtype)
	}
}

func TestAppState_StreamToolInput(t *testing.T) {
	state := NewAppState()

	// Append partial tool input
	state.AppendStreamToolInput("tool_1", `{"command":`)
	state.AppendStreamToolInput("tool_1", `"ls -la"}`)

	result := state.GetStreamToolInput("tool_1")
	if result != `{"command":"ls -la"}` {
		t.Errorf("expected concatenated input, got '%s'", result)
	}

	// Non-existent tool should return empty string
	result = state.GetStreamToolInput("nonexistent")
	if result != "" {
		t.Errorf("expected empty string for non-existent tool, got '%s'", result)
	}
}

func TestStreamState_Reset(t *testing.T) {
	stream := NewStreamState()

	stream.PartialText = "some text"
	stream.PartialThinking = "some thinking"
	stream.PartialToolInput["tool_1"] = "some input"
	stream.CurrentIndex = 5

	stream.Reset()

	if stream.PartialText != "" {
		t.Error("expected PartialText to be cleared")
	}
	if stream.PartialThinking != "" {
		t.Error("expected PartialThinking to be cleared")
	}
	if len(stream.PartialToolInput) != 0 {
		t.Error("expected PartialToolInput to be cleared")
	}
	if stream.CurrentIndex != 0 {
		t.Error("expected CurrentIndex to be reset")
	}
}

func TestNewStreamState(t *testing.T) {
	stream := NewStreamState()

	if stream == nil {
		t.Fatal("expected non-nil stream state")
	}
	if stream.PartialToolInput == nil {
		t.Error("expected PartialToolInput map to be initialized")
	}
}

func TestNewAppState(t *testing.T) {
	state := NewAppState()

	if state == nil {
		t.Fatal("expected non-nil app state")
	}
	if state.PendingTools == nil {
		t.Error("expected PendingTools map to be initialized")
	}
	if state.AgentsByID == nil {
		t.Error("expected AgentsByID map to be initialized")
	}
	if state.TotalTokens == nil {
		t.Error("expected TotalTokens to be initialized")
	}
	if state.Stream == nil {
		t.Error("expected Stream to be initialized")
	}
}

func TestAppState_CompleteToolCall_WithError(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	toolCall := &ToolCall{
		ID:     "tool_err",
		Name:   "Bash",
		Status: ToolCallStatusPending,
	}
	state.AddOrUpdateToolCall(toolCall)

	// Complete with error
	state.CompleteToolCall("tool_err", "command not found", true)

	tc := state.PendingTools["tool_err"]
	if tc.Status != ToolCallStatusFailed {
		t.Errorf("expected status 'failed', got '%s'", tc.Status)
	}
	if !tc.IsError {
		t.Error("expected IsError to be true")
	}
	if tc.Result != "command not found" {
		t.Errorf("expected error result, got '%s'", tc.Result)
	}
}

func TestAppState_CompleteToolCall_NonExistent(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	// Complete a tool that doesn't exist - should be a no-op
	state.CompleteToolCall("nonexistent", "result", false)

	// Should not panic or create an entry
	if _, ok := state.PendingTools["nonexistent"]; ok {
		t.Error("completing non-existent tool should not create entry")
	}
}
