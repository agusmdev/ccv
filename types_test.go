package main

import (
	"fmt"
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

func TestAppState_AddOrUpdateToolCall_NilCurrentAgent(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	// Set CurrentAgent to nil
	state.CurrentAgent = nil

	toolCall := &ToolCall{
		ID:     "tool_123",
		Name:   "Read",
		Status: ToolCallStatusPending,
	}

	// Should not panic when CurrentAgent is nil
	state.AddOrUpdateToolCall(toolCall)

	// Tool should still be added to PendingTools
	if _, ok := state.PendingTools["tool_123"]; !ok {
		t.Error("expected tool call to be in pending tools even with nil CurrentAgent")
	}
}

func TestAppState_AddOrUpdateToolCall_UpdateExisting(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	// Add initial tool call
	toolCall := &ToolCall{
		ID:     "tool_123",
		Name:   "Read",
		Status: ToolCallStatusPending,
	}
	state.AddOrUpdateToolCall(toolCall)

	// Update the same tool call
	updatedCall := &ToolCall{
		ID:     "tool_123",
		Name:   "Read",
		Status: ToolCallStatusRunning,
	}
	state.AddOrUpdateToolCall(updatedCall)

	// Verify it was updated in PendingTools
	tc := state.PendingTools["tool_123"]
	if tc.Status != ToolCallStatusRunning {
		t.Errorf("expected status 'running', got '%s'", tc.Status)
	}

	// Verify it was updated in CurrentAgent's ToolCalls
	if len(state.CurrentAgent.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call in current agent, got %d", len(state.CurrentAgent.ToolCalls))
	}
	if state.CurrentAgent.ToolCalls[0].Status != ToolCallStatusRunning {
		t.Errorf("expected agent tool call status 'running', got '%s'", state.CurrentAgent.ToolCalls[0].Status)
	}
}

func TestAppState_CreateChildAgent_NilRootAgent(t *testing.T) {
	state := NewAppState()
	// Don't initialize session, so RootAgent will be nil

	child := state.CreateChildAgent("tool_1", "Explore", "Exploring codebase")

	// Should not panic when RootAgent is nil
	if child == nil {
		t.Fatal("expected non-nil child agent even with nil RootAgent")
	}
	// When RootAgent is nil, CreateChildAgent will use RootAgent as parent
	// which defaults to nil, so child should be created with nil parent
	if child.ID != "tool_1" {
		t.Errorf("expected ID 'tool_1', got '%s'", child.ID)
	}
}

func TestParseMessage_StreamEventTypes(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "message_start event",
			json:    `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`,
			wantErr: false,
		},
		{
			name:    "content_block_start event",
			json:    `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			wantErr: false,
		},
		{
			name:    "content_block_stop event",
			json:    `{"type":"content_block_stop","index":0}`,
			wantErr: false,
		},
		{
			name:    "message_delta event",
			json:    `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10,"output_tokens":5}}`,
			wantErr: false,
		},
		{
			name:    "message_stop event",
			json:    `{"type":"message_stop"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				event, ok := msg.(*StreamEvent)
				if !ok {
					t.Errorf("ParseMessage() returned %T, want *StreamEvent", msg)
				} else {
					// Verify the Type field matches expected
					if event.Type == "" {
						t.Error("ParseMessage() StreamEvent has empty Type")
					}
				}
			}
		})
	}
}

func TestParseMessage_UnmarshalError(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantTypeIn string // The type field that should be parsed even if inner unmarshal fails
	}{
		{
			name:       "system message with invalid inner structure",
			json:       `{"type":"system","subtype":"init","session_id":123}`,
			wantTypeIn: "system",
		},
		{
			name:       "assistant message with invalid content",
			json:       `{"type":"assistant","message":"invalid"}`,
			wantTypeIn: "assistant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should error during unmarshal
			_, err := ParseMessage([]byte(tt.json))
			if err == nil {
				t.Error("ParseMessage() expected error for invalid JSON structure, got nil")
			}
		})
	}
}

func TestParseMessage_StreamEventWrapperWithNilEvent(t *testing.T) {
	// Test stream_event wrapper with nil event
	data := []byte(`{"type":"stream_event","event":null,"session_id":"test"}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	// Should return the wrapper itself when event is nil
	wrapper, ok := msg.(*StreamEventWrapper)
	if !ok {
		t.Fatalf("expected *StreamEventWrapper with nil event, got %T", msg)
	}
	if wrapper.Event != nil {
		t.Error("expected nil Event")
	}
}

func TestParseMessage_UserMessage(t *testing.T) {
	data := []byte(`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"user input"}]},"session_id":"sess-1"}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	user, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}
	if user.Type != "user" {
		t.Errorf("expected type 'user', got '%s'", user.Type)
	}
	if user.SessionID != "sess-1" {
		t.Errorf("expected session_id 'sess-1', got '%s'", user.SessionID)
	}
	if len(user.Message.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(user.Message.Content))
	}
}

func TestParseMessage_SystemInit_AllOptionalFields(t *testing.T) {
	data := []byte(`{
		"type": "system",
		"subtype": "init",
		"session_id": "sess-full",
		"model": "claude-opus-4-5-20251101",
		"cwd": "/home/user/project",
		"tools": ["Read", "Bash", "Write"],
		"mcp_servers": [
			{"name": "server1", "status": "running"},
			{"name": "server2", "status": "stopped"}
		],
		"permissionMode": "auto",
		"slash_commands": ["/commit", "/review"],
		"apiKeySource": "env",
		"claude_code_version": "1.0.0",
		"output_style": "compact",
		"agents": ["explore", "plan"],
		"skills": ["fastapi-auth"],
		"plugins": [
			{"name": "plugin1", "path": "/path/to/plugin1"}
		],
		"uuid": "uuid-12345"
	}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	sysInit, ok := msg.(*SystemInit)
	if !ok {
		t.Fatalf("expected *SystemInit, got %T", msg)
	}

	if sysInit.SessionID != "sess-full" {
		t.Errorf("expected session_id 'sess-full', got '%s'", sysInit.SessionID)
	}
	if sysInit.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected model 'claude-opus-4-5-20251101', got '%s'", sysInit.Model)
	}
	if sysInit.CwdPath != "/home/user/project" {
		t.Errorf("expected cwd '/home/user/project', got '%s'", sysInit.CwdPath)
	}
	if len(sysInit.Tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(sysInit.Tools))
	}
	if len(sysInit.McpServers) != 2 {
		t.Errorf("expected 2 mcp_servers, got %d", len(sysInit.McpServers))
	}
	if sysInit.McpServers[0].Name != "server1" {
		t.Errorf("expected first mcp_server name 'server1', got '%s'", sysInit.McpServers[0].Name)
	}
	if sysInit.PermissionMode != "auto" {
		t.Errorf("expected permissionMode 'auto', got '%s'", sysInit.PermissionMode)
	}
	if len(sysInit.SlashCommands) != 2 {
		t.Errorf("expected 2 slash_commands, got %d", len(sysInit.SlashCommands))
	}
	if sysInit.APIKeySource != "env" {
		t.Errorf("expected apiKeySource 'env', got '%s'", sysInit.APIKeySource)
	}
	if sysInit.ClaudeCodeVersion != "1.0.0" {
		t.Errorf("expected claude_code_version '1.0.0', got '%s'", sysInit.ClaudeCodeVersion)
	}
	if sysInit.OutputStyle != "compact" {
		t.Errorf("expected output_style 'compact', got '%s'", sysInit.OutputStyle)
	}
	if len(sysInit.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(sysInit.Agents))
	}
	if len(sysInit.Skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(sysInit.Skills))
	}
	if len(sysInit.Plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(sysInit.Plugins))
	}
	if sysInit.Plugins[0].Name != "plugin1" {
		t.Errorf("expected plugin name 'plugin1', got '%s'", sysInit.Plugins[0].Name)
	}
	if sysInit.UUID != "uuid-12345" {
		t.Errorf("expected uuid 'uuid-12345', got '%s'", sysInit.UUID)
	}
}

func TestParseMessage_AssistantMessage_AllContentBlockTypes(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		validateFn func(t *testing.T, block ContentBlock)
	}{
		{
			name: "text content block",
			content: `{"type":"assistant","message":{
				"id":"msg-1",
				"type":"message",
				"role":"assistant",
				"content":[
					{"type":"text","text":"Hello, world!"}
				]
			}}`,
			validateFn: func(t *testing.T, block ContentBlock) {
				if block.Type != ContentBlockTypeText {
					t.Errorf("expected content type 'text', got '%s'", block.Type)
				}
				if block.Text != "Hello, world!" {
					t.Errorf("expected text 'Hello, world!', got '%s'", block.Text)
				}
			},
		},
		{
			name: "thinking content block",
			content: `{"type":"assistant","message":{
				"id":"msg-2",
				"type":"message",
				"role":"assistant",
				"content":[
					{"type":"thinking","thinking":"Let me analyze this..."}
				]
			}}`,
			validateFn: func(t *testing.T, block ContentBlock) {
				if block.Type != ContentBlockTypeThinking {
					t.Errorf("expected content type 'thinking', got '%s'", block.Type)
				}
				if block.Thinking != "Let me analyze this..." {
					t.Errorf("expected thinking 'Let me analyze this...', got '%s'", block.Thinking)
				}
			},
		},
		{
			name: "redacted_thinking content block",
			content: `{"type":"assistant","message":{
				"id":"msg-3",
				"type":"message",
				"role":"assistant",
				"content":[
					{"type":"redacted_thinking","thinking":"[REDACTED]"}
				]
			}}`,
			validateFn: func(t *testing.T, block ContentBlock) {
				if block.Type != ContentBlockTypeRedactedThinking {
					t.Errorf("expected content type 'redacted_thinking', got '%s'", block.Type)
				}
			},
		},
		{
			name: "tool_use content block",
			content: `{"type":"assistant","message":{
				"id":"msg-4",
				"type":"message",
				"role":"assistant",
				"content":[
					{
						"type":"tool_use",
						"id":"tool_123",
						"name":"Read",
						"input":{"file_path":"./test.go"}
					}
				]
			}}`,
			validateFn: func(t *testing.T, block ContentBlock) {
				if block.Type != ContentBlockTypeToolUse {
					t.Errorf("expected content type 'tool_use', got '%s'", block.Type)
				}
				if block.ID != "tool_123" {
					t.Errorf("expected tool id 'tool_123', got '%s'", block.ID)
				}
				if block.Name != "Read" {
					t.Errorf("expected tool name 'Read', got '%s'", block.Name)
				}
				if len(block.Input) == 0 {
					t.Error("expected tool input to be non-empty")
				}
			},
		},
		{
			name: "tool_result content block",
			content: `{"type":"assistant","message":{
				"id":"msg-5",
				"type":"message",
				"role":"assistant",
				"content":[
					{
						"type":"tool_result",
						"tool_use_id":"tool_123",
						"content":"file contents here",
						"is_error":false
					}
				]
			}}`,
			validateFn: func(t *testing.T, block ContentBlock) {
				if block.Type != ContentBlockTypeToolResult {
					t.Errorf("expected content type 'tool_result', got '%s'", block.Type)
				}
				if block.ToolUseID != "tool_123" {
					t.Errorf("expected tool_use_id 'tool_123', got '%s'", block.ToolUseID)
				}
				if block.Content != "file contents here" {
					t.Errorf("expected content 'file contents here', got '%s'", block.Content)
				}
				if block.IsError {
					t.Error("expected is_error to be false")
				}
			},
		},
		{
			name: "mixed content blocks",
			content: `{"type":"assistant","message":{
				"id":"msg-6",
				"type":"message",
				"role":"assistant",
				"content":[
					{"type":"thinking","thinking":"I'll help with that"},
					{"type":"text","text":"Here's what I found:"},
					{
						"type":"tool_use",
						"id":"tool_456",
						"name":"Bash",
						"input":{"command":"ls -la"}
					}
				]
			}}`,
			validateFn: func(t *testing.T, block ContentBlock) {
				// Just verify we got all 3 blocks
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage([]byte(tt.content))
			if err != nil {
				t.Fatalf("ParseMessage failed: %v", err)
			}

			assistant, ok := msg.(*AssistantMessage)
			if !ok {
				t.Fatalf("expected *AssistantMessage, got %T", msg)
			}

			if len(assistant.Message.Content) == 0 {
				t.Fatal("expected at least one content block")
			}

			// Validate first content block
			tt.validateFn(t, assistant.Message.Content[0])

			// For mixed content, verify count
			if tt.name == "mixed content blocks" {
				if len(assistant.Message.Content) != 3 {
					t.Errorf("expected 3 content blocks in mixed content, got %d", len(assistant.Message.Content))
				}
			}
		})
	}
}

func TestParseMessage_UserMessage_WithToolResult(t *testing.T) {
	data := []byte(`{
		"type": "user",
		"message": {
			"role": "user",
			"content": [
				{
					"type": "tool_result",
					"tool_use_id": "tool_123",
					"content": "Command output",
					"is_error": false
				}
			]
		},
		"session_id": "sess-user",
		"tool_use_result": {
			"stdout": "stdout content",
			"stderr": "stderr content"
		}
	}`)

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

	block := user.Message.Content[0]
	if block.Type != ContentBlockTypeToolResult {
		t.Errorf("expected content type 'tool_result', got '%s'", block.Type)
	}
	if block.ToolUseID != "tool_123" {
		t.Errorf("expected tool_use_id 'tool_123', got '%s'", block.ToolUseID)
	}
	if block.Content != "Command output" {
		t.Errorf("expected content 'Command output', got '%s'", block.Content)
	}

	if user.ToolUseResult == nil {
		t.Fatal("expected tool_use_result to be non-nil")
	}
	if user.ToolUseResult.Stdout != "stdout content" {
		t.Errorf("expected stdout 'stdout content', got '%s'", user.ToolUseResult.Stdout)
	}
	if user.ToolUseResult.Stderr != "stderr content" {
		t.Errorf("expected stderr 'stderr content', got '%s'", user.ToolUseResult.Stderr)
	}
}

func TestParseMessage_Result_AllOptionalFields(t *testing.T) {
	data := []byte(`{
		"type": "result",
		"subtype": "success",
		"result": "Task completed successfully",
		"is_error": false,
		"total_cost_usd": 0.12345,
		"duration_ms": 15000,
		"duration_api_ms": 12000,
		"session_id": "sess-result",
		"num_turns": 5,
		"usage": {
			"input_tokens": 1000,
			"output_tokens": 500,
			"cache_creation_input_tokens": 100,
			"cache_read_input_tokens": 50,
			"total_tokens": 1500,
			"service_tier": "default"
		},
		"modelUsage": {
			"claude-opus-4-5-20251101": {
				"inputTokens": 800,
				"outputTokens": 400,
				"cacheReadInputTokens": 30,
				"cacheCreationInputTokens": 20,
				"webSearchRequests": 2,
				"webFetchRequests": 5,
				"costUSD": 0.1,
				"contextWindow": 200000,
				"maxOutputTokens": 8192
			}
		},
		"permission_denials": [
			{"tool_name": "DangerousTool", "reason": "security policy"}
		],
		"uuid": "result-uuid-67890"
	}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	result, ok := msg.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", msg)
	}

	if result.Result != "Task completed successfully" {
		t.Errorf("expected result 'Task completed successfully', got '%s'", result.Result)
	}
	if result.TotalCost != 0.12345 {
		t.Errorf("expected total_cost 0.12345, got %f", result.TotalCost)
	}
	if result.DurationMS != 15000 {
		t.Errorf("expected duration_ms 15000, got %d", result.DurationMS)
	}
	if result.DurationAPIMS != 12000 {
		t.Errorf("expected duration_api_ms 12000, got %d", result.DurationAPIMS)
	}
	if result.NumTurns != 5 {
		t.Errorf("expected num_turns 5, got %d", result.NumTurns)
	}
	if result.Usage == nil {
		t.Fatal("expected usage to be non-nil")
	}
	if result.Usage.InputTokens != 1000 {
		t.Errorf("expected input_tokens 1000, got %d", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 500 {
		t.Errorf("expected output_tokens 500, got %d", result.Usage.OutputTokens)
	}
	if result.Usage.TotalTokens != 1500 {
		t.Errorf("expected total_tokens 1500, got %d", result.Usage.TotalTokens)
	}
	if result.ModelUsage == nil {
		t.Fatal("expected modelUsage to be non-nil")
	}
	if len(result.ModelUsage) != 1 {
		t.Errorf("expected 1 model in modelUsage, got %d", len(result.ModelUsage))
	}
	modelEntry := result.ModelUsage["claude-opus-4-5-20251101"]
	if modelEntry == nil {
		t.Fatal("expected model entry for claude-opus-4-5-20251101")
	}
	if modelEntry.InputTokens != 800 {
		t.Errorf("expected model inputTokens 800, got %d", modelEntry.InputTokens)
	}
	if modelEntry.CostUSD != 0.1 {
		t.Errorf("expected model costUSD 0.1, got %f", modelEntry.CostUSD)
	}
	if len(result.PermissionDenials) != 1 {
		t.Errorf("expected 1 permission denial, got %d", len(result.PermissionDenials))
	}
	if result.PermissionDenials[0].ToolName != "DangerousTool" {
		t.Errorf("expected tool_name 'DangerousTool', got '%s'", result.PermissionDenials[0].ToolName)
	}
	if result.UUID != "result-uuid-67890" {
		t.Errorf("expected uuid 'result-uuid-67890', got '%s'", result.UUID)
	}
}

func TestToolUseResult_UnmarshalJSON_PartialFields(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		validate func(t *testing.T, result ToolUseResult)
	}{
		{
			name: "only stdout field",
			json: `{"stdout":"only stdout"}`,
			validate: func(t *testing.T, result ToolUseResult) {
				if result.Stdout != "only stdout" {
					t.Errorf("expected stdout 'only stdout', got '%s'", result.Stdout)
				}
				if result.Stderr != "" {
					t.Errorf("expected stderr to be empty, got '%s'", result.Stderr)
				}
				if result.Interrupted {
					t.Error("expected Interrupted to be false")
				}
			},
		},
		{
			name: "only stderr field",
			json: `{"stderr":"error message"}`,
			validate: func(t *testing.T, result ToolUseResult) {
				if result.Stdout != "" {
					t.Errorf("expected stdout to be empty, got '%s'", result.Stdout)
				}
				if result.Stderr != "error message" {
					t.Errorf("expected stderr 'error message', got '%s'", result.Stderr)
				}
			},
		},
		{
			name: "stdout and interrupted only",
			json: `{"stdout":"output","interrupted":true}`,
			validate: func(t *testing.T, result ToolUseResult) {
				if result.Stdout != "output" {
					t.Errorf("expected stdout 'output', got '%s'", result.Stdout)
				}
				if !result.Interrupted {
					t.Error("expected Interrupted to be true")
				}
			},
		},
		{
			name: "isImage only",
			json: `{"isImage":true}`,
			validate: func(t *testing.T, result ToolUseResult) {
				if !result.IsImage {
					t.Error("expected IsImage to be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result ToolUseResult
			err := result.UnmarshalJSON([]byte(tt.json))
			if err != nil {
				t.Fatalf("UnmarshalJSON failed: %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestAppState_StreamToolInput_MultipleTools(t *testing.T) {
	state := NewAppState()

	// Append partial input for multiple tools concurrently
	state.AppendStreamToolInput("tool_1", `{"file_`)
	state.AppendStreamToolInput("tool_2", `{"command":`)
	state.AppendStreamToolInput("tool_1", `path":"./a.go"}`)
	state.AppendStreamToolInput("tool_3", `{"url":"http://ex`)
	state.AppendStreamToolInput("tool_2", `"ls -la"}`)
	state.AppendStreamToolInput("tool_3", `ample.com"}`)

	// Verify each tool's accumulated input
	result1 := state.GetStreamToolInput("tool_1")
	if result1 != `{"file_path":"./a.go"}` {
		t.Errorf("tool_1: expected '%s', got '%s'", `{"file_path":"./a.go"}`, result1)
	}

	result2 := state.GetStreamToolInput("tool_2")
	if result2 != `{"command":"ls -la"}` {
		t.Errorf("tool_2: expected '%s', got '%s'", `{"command":"ls -la"}`, result2)
	}

	result3 := state.GetStreamToolInput("tool_3")
	if result3 != `{"url":"http://example.com"}` {
		t.Errorf("tool_3: expected '%s', got '%s'", `{"url":"http://example.com"}`, result3)
	}

	// Verify map has exactly 3 entries
	if len(state.Stream.PartialToolInput) != 3 {
		t.Errorf("expected 3 tools in PartialToolInput, got %d", len(state.Stream.PartialToolInput))
	}
}

func TestParseMessage_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantType   interface{}
		validateFn func(t *testing.T, msg interface{})
	}{
		{
			name: "special characters in text content",
			json: `{"type":"assistant","message":{"id":"msg-1","type":"message","role":"assistant","content":[{"type":"text","text":"Hello \"world\"!\nNew line\tTab\n\u00E9 Unicode \U0001F600 backslash \\ slash /"}]}}`,
			validateFn: func(t *testing.T, msg interface{}) {
				assistant := msg.(*AssistantMessage)
				expected := `Hello "world"!
New line	Tab
é Unicode 😀 backslash \ slash /`
				if assistant.Message.Content[0].Text != expected {
					t.Errorf("expected text with special chars, got '%s'", assistant.Message.Content[0].Text)
				}
			},
		},
		{
			name: "special characters in tool input JSON",
			json: `{"type":"assistant","message":{"id":"msg-1","type":"message","role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"Bash","input":{"command":"echo \"test\" && ls -la | grep .go"}}]}}`,
			validateFn: func(t *testing.T, msg interface{}) {
				assistant := msg.(*AssistantMessage)
				if len(assistant.Message.Content[0].Input) == 0 {
					t.Error("expected tool input to be non-empty")
				}
			},
		},
		{
			name: "unicode in session_id",
			json: `{"type":"system","subtype":"init","session_id":"sess-测试-🚀","model":"claude-opus-4-5-20251101"}`,
			validateFn: func(t *testing.T, msg interface{}) {
				sysInit := msg.(*SystemInit)
				if sysInit.SessionID != "sess-测试-🚀" {
					t.Errorf("expected session_id with unicode, got '%s'", sysInit.SessionID)
				}
			},
		},
		{
			name: "null characters and escaped chars",
			json: `{"type":"assistant","message":{"id":"msg-1","type":"message","role":"assistant","content":[{"type":"text","text":"Path: C:\\Users\\test\\file.txt"}]}}`,
			validateFn: func(t *testing.T, msg interface{}) {
				assistant := msg.(*AssistantMessage)
				if assistant.Message.Content[0].Text != `Path: C:\Users\test\file.txt` {
					t.Errorf("expected path with backslashes, got '%s'", assistant.Message.Content[0].Text)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage([]byte(tt.json))
			if err != nil {
				t.Fatalf("ParseMessage failed: %v", err)
			}
			tt.validateFn(t, msg)
		})
	}
}

func TestAppState_VeryLongToolInput(t *testing.T) {
	state := NewAppState()

	// Simulate streaming a very long JSON input (like a large file content)
	longContent := string(make([]byte, 100*1024)) // 100KB
	for i := range longContent {
		longContent = longContent[:i] + "a" + longContent[i+1:]
	}

	// Append in chunks to simulate streaming
	chunkSize := 1024
	for i := 0; i < len(longContent); i += chunkSize {
		end := i + chunkSize
		if end > len(longContent) {
			end = len(longContent)
		}
		state.AppendStreamToolInput("tool_large", longContent[i:end])
	}

	result := state.GetStreamToolInput("tool_large")
	if len(result) != len(longContent) {
		t.Errorf("expected length %d, got %d", len(longContent), len(result))
	}

	// Verify the content is correct
	if result != longContent {
		t.Error("long tool input content mismatch")
	}
}

func TestToolUseResult_UnmarshalJSON_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		check func(t *testing.T, result ToolUseResult)
	}{
		{
			name: "newlines and quotes in stdout",
			json: `{"stdout":"Line 1\nLine \"quoted\"\nLine 3"}`,
			check: func(t *testing.T, result ToolUseResult) {
				expected := "Line 1\nLine \"quoted\"\nLine 3"
				if result.Stdout != expected {
					t.Errorf("expected '%s', got '%s'", expected, result.Stdout)
				}
			},
		},
		{
			name: "unicode in stderr",
			json: `{"stderr":"Error: 错误 \U0001F4A3"}`,
			check: func(t *testing.T, result ToolUseResult) {
				if result.Stderr != "Error: 错误 💣" {
					t.Errorf("expected unicode stderr, got '%s'", result.Stderr)
				}
			},
		},
		{
			name: "backslashes in string form",
			json: `"C:\\Users\\test\\file.txt"`,
			check: func(t *testing.T, result ToolUseResult) {
				if result.RawString != `C:\Users\test\file.txt` {
					t.Errorf("expected '%s', got '%s'", `C:\Users\test\file.txt`, result.RawString)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result ToolUseResult
			err := result.UnmarshalJSON([]byte(tt.json))
			if err != nil {
				t.Fatalf("UnmarshalJSON failed: %v", err)
			}
			tt.check(t, result)
		})
	}
}

func TestParseMessage_UsageWithCacheCreation(t *testing.T) {
	data := []byte(`{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"usage": {
			"input_tokens": 1000,
			"output_tokens": 500,
			"cache_creation_input_tokens": 200,
			"cache_read_input_tokens": 100,
			"total_tokens": 1500,
			"cache_creation": {
				"ephemeral_5m_input_tokens": 150,
				"ephemeral_1h_input_tokens": 50
			},
			"service_tier": "default"
		}
	}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	result, ok := msg.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", msg)
	}

	if result.Usage == nil {
		t.Fatal("expected usage to be non-nil")
	}
	if result.Usage.CacheCreation == nil {
		t.Fatal("expected cache_creation to be non-nil")
	}
	if result.Usage.CacheCreation.Ephemeral5mInputTokens != 150 {
		t.Errorf("expected ephemeral_5m_input_tokens 150, got %d", result.Usage.CacheCreation.Ephemeral5mInputTokens)
	}
	if result.Usage.CacheCreation.Ephemeral1hInputTokens != 50 {
		t.Errorf("expected ephemeral_1h_input_tokens 50, got %d", result.Usage.CacheCreation.Ephemeral1hInputTokens)
	}
}

func TestParseMessage_AssistantMessage_WithUsage(t *testing.T) {
	data := []byte(`{
		"type": "assistant",
		"message": {
			"id": "msg-1",
			"type": "message",
			"role": "assistant",
			"model": "claude-opus-4-5-20251101",
			"content": [{"type":"text","text":"Response"}],
			"stop_reason": "end_turn",
			"stop_sequence": null,
			"usage": {
				"input_tokens": 500,
				"output_tokens": 100,
				"cache_creation_input_tokens": 50,
				"cache_read_input_tokens": 25
			}
		},
		"cost_usd": 0.01,
		"duration_ms": 2000,
		"is_error": false
	}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	assistant, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}

	if assistant.Message.Model != "claude-opus-4-5-20251101" {
		t.Errorf("expected model 'claude-opus-4-5-20251101', got '%s'", assistant.Message.Model)
	}
	if assistant.Message.StopReason != "end_turn" {
		t.Errorf("expected stop_reason 'end_turn', got '%s'", assistant.Message.StopReason)
	}
	if assistant.Message.Usage == nil {
		t.Fatal("expected usage to be non-nil")
	}
	if assistant.Message.Usage.InputTokens != 500 {
		t.Errorf("expected input_tokens 500, got %d", assistant.Message.Usage.InputTokens)
	}
	if assistant.CostUSD != 0.01 {
		t.Errorf("expected cost_usd 0.01, got %f", assistant.CostUSD)
	}
	if assistant.DurationMS != 2000 {
		t.Errorf("expected duration_ms 2000, got %d", assistant.DurationMS)
	}
}

func TestStreamState_CurrentIndex(t *testing.T) {
	stream := NewStreamState()

	if stream.CurrentIndex != 0 {
		t.Errorf("expected initial CurrentIndex 0, got %d", stream.CurrentIndex)
	}

	stream.CurrentIndex = 5
	if stream.CurrentIndex != 5 {
		t.Errorf("expected CurrentIndex 5, got %d", stream.CurrentIndex)
	}

	stream.Reset()
	if stream.CurrentIndex != 0 {
		t.Errorf("expected CurrentIndex 0 after Reset, got %d", stream.CurrentIndex)
	}
}

// FuzzParseMessage tests ParseMessage with random byte input
func FuzzParseMessage(f *testing.F) {
	// Seed corpus from existing valid test cases
	f.Add([]byte(`{"type":"system","subtype":"init","session_id":"test"}`))
	f.Add([]byte(`{"type":"assistant","message":{"id":"msg_01","type":"message","role":"assistant","content":[{"type":"text","text":"Hello"}]}}`))
	f.Add([]byte(`{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.05}`))
	f.Add([]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`))
	f.Add([]byte(`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"input"}]}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"type":"unknown_type","field":"value"}`))
	f.Add([]byte(`"simple string"`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"type":"compact_boundary","subtype":"context_trim","session_id":"test-123"}`))
	f.Add([]byte(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}`))

	// Add edge cases
	f.Add([]byte(""))
	f.Add([]byte("{"))
	f.Add([]byte("}"))
	f.Add([]byte(`{"type":"`))
	f.Add([]byte(`{"type":"system","subtype":"init","session_id":"`)) // incomplete

	// Add special characters
	f.Add([]byte(`{"type":"system","subtype":"init","session_id":"test\n\x00\x1f"}`))
	f.Add([]byte(`{"type":"system","subtype":"init","session_id":"test\"quotes\""}`))

	// Add deeply nested JSON (potential stack overflow)
	deepJSON := []byte(`{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"nested":{`)
	for i := 0; i < 100; i++ {
		deepJSON = append(deepJSON, `"field`...)
		deepJSON = append(deepJSON, []byte(fmt.Sprintf("%d", i))...)
		deepJSON = append(deepJSON, `":{`...)
	}
	for i := 0; i < 100; i++ {
		deepJSON = append(deepJSON, `}}`...)
	}
	deepJSON = append(deepJSON, `}}`...)
	f.Add(deepJSON)

	// Add extremely long line (1MB+)
	longJSON := make([]byte, 0, 1024*1024+100)
	longJSON = append(longJSON, `{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"`...)
	for i := 0; i < 1024*1024; i++ {
		longJSON = append(longJSON, 'a')
	}
	longJSON = append(longJSON, `"}]}}`...)
	f.Add(longJSON)

	f.Fuzz(func(t *testing.T, data []byte) {
		// ParseMessage should never panic, even with completely invalid input
		// The panic recovery inside ParseMessage should catch any panics
		result, err := ParseMessage(data)

		// If we got a result without error, verify it's not nil
		if err == nil && result == nil {
			t.Error("ParseMessage returned nil result with nil error")
		}

		// The result should be one of the known types or BaseMessage
		if result != nil {
			switch result.(type) {
			case *SystemInit, *AssistantMessage, *UserMessage, *Result, *CompactBoundary,
				*StreamEvent, *StreamEventWrapper, *BaseMessage:
				// Valid types
			default:
				// Unknown type - should be BaseMessage at minimum
				if _, ok := result.(*BaseMessage); !ok {
					t.Errorf("ParseMessage returned unexpected type: %T", result)
				}
			}
		}
	})
}

// FuzzToolUseResult_UnmarshalJSON tests ToolUseResult unmarshaling with random input
func FuzzToolUseResult_UnmarshalJSON(f *testing.F) {
	// Seed corpus with valid cases
	f.Add([]byte(`"simple string result"`))
	f.Add([]byte(`{"stdout":"output text","stderr":"error text","interrupted":true,"isImage":false}`))
	f.Add([]byte(`{"stdout":"only stdout"}`))
	f.Add([]byte(`{"stderr":"error message"}`))
	f.Add([]byte(`""`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))

	// Add special characters
	f.Add([]byte(`{"stdout":"Line 1\nLine \"quoted\"\nLine 3"}`))
	f.Add([]byte(`"C:\\Users\\test\\file.txt"`))
	f.Add([]byte(`{"stderr":"Error: 错误 \U0001F4A3"}`))

	// Add malformed JSON
	f.Add([]byte(`{invalid}`))
	f.Add([]byte(`{`))
	f.Add([]byte(`}`))
	f.Add([]byte(`"`)) // incomplete string

	// Add long strings
	longStr := make([]byte, 0, 100*1024)
	longStr = append(longStr, `{"stdout":"`...)
	for i := 0; i < 100*1024; i++ {
		longStr = append(longStr, 'a')
	}
	longStr = append(longStr, `"}`...)
	f.Add(longStr)

	f.Fuzz(func(t *testing.T, data []byte) {
		var result ToolUseResult
		err := result.UnmarshalJSON(data)

		// UnmarshalJSON may fail with invalid JSON, but should never panic
		// Even with errors, the result should be in a consistent state
		if err == nil {
			// If no error, we should have valid data in either RawString or the struct fields
			if result.RawString == "" && result.Stdout == "" && result.Stderr == "" &&
				!result.Interrupted && !result.IsImage {
				// Empty but valid (e.g., {} or "")
				// This is acceptable
			}
		}
	})
}
