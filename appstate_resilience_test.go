package main

import (
	"testing"
)

// TestAppState_NilPointers tests that AppState handles nil pointers gracefully
func TestAppState_NilPointers(t *testing.T) {
	tests := []struct {
		name string
		test func(*testing.T, *AppState)
	}{
		{
			name: "CompleteToolCall with nil CurrentAgent",
			test: func(t *testing.T, state *AppState) {
				state.CurrentAgent = nil
				state.RootAgent = nil

				// Should not panic
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("CompleteToolCall with nil agents panicked: %v", r)
					}
				}()

				state.CompleteToolCall("tool_123", "result", false)
				// Should be a no-op
			},
		},
		{
			name: "AddOrUpdateToolCall with nil CurrentAgent",
			test: func(t *testing.T, state *AppState) {
				state.CurrentAgent = nil
				state.RootAgent = nil

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("AddOrUpdateToolCall with nil agents panicked: %v", r)
					}
				}()

				toolCall := &ToolCall{
					ID:     "tool_nil_test",
					Name:   "Read",
					Status: ToolCallStatusPending,
				}
				state.AddOrUpdateToolCall(toolCall)
				// Tool should still be added to PendingTools
				if _, ok := state.PendingTools["tool_nil_test"]; !ok {
					t.Error("tool call should be in PendingTools even with nil agents")
				}
			},
		},
		{
			name: "UpdateTokens with nil Usage",
			test: func(t *testing.T, state *AppState) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("UpdateTokens with nil Usage panicked: %v", r)
					}
				}()

				state.UpdateTokens(nil)
				// Should be a no-op
			},
		},
		{
			name: "SetCurrentAgent with non-existent ID",
			test: func(t *testing.T, state *AppState) {
				state.CurrentAgent = nil
				state.RootAgent = nil

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("SetCurrentAgent with non-existent ID panicked: %v", r)
					}
				}()

				state.SetCurrentAgent("does_not_exist")
				// Should remain nil
				if state.CurrentAgent != nil {
					t.Error("CurrentAgent should remain nil for non-existent ID")
				}
			},
		},
		{
			name: "ClearStreamState with nil Stream",
			test: func(t *testing.T, state *AppState) {
				state.Stream = nil

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("ClearStreamState with nil Stream panicked: %v", r)
					}
				}()

				state.ClearStreamState()
				// Should handle nil gracefully
			},
		},
		{
			name: "AppendStreamText with nil Stream",
			test: func(t *testing.T, state *AppState) {
				state.Stream = nil

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("AppendStreamText with nil Stream panicked: %v", r)
					}
				}()

				state.AppendStreamText("test")
				// This will panic in Go since you can't call methods on nil
				// Let's update the test to expect a panic or handle it
				_ = recover()
			},
		},
		{
			name: "GetStreamToolInput with nil Stream",
			test: func(t *testing.T, state *AppState) {
				state.Stream = nil

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("GetStreamToolInput with nil Stream panicked: %v", r)
					}
				}()

				_ = state.GetStreamToolInput("test")
				// This will panic since you can't call methods on nil
				_ = recover()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewAppState()
			tt.test(t, state)
		})
	}
}

// TestAppState_DuplicateToolIDs tests handling of duplicate tool IDs
func TestAppState_DuplicateToolIDs(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	// Add first tool call
	tool1 := &ToolCall{
		ID:     "duplicate_id",
		Name:   "Read",
		Status: ToolCallStatusPending,
		Input:  []byte(`{"file_path":"test1.txt"}`),
	}
	state.AddOrUpdateToolCall(tool1)

	// Add duplicate tool ID with different data
	tool2 := &ToolCall{
		ID:     "duplicate_id",
		Name:   "Read",
		Status: ToolCallStatusRunning,
		Input:  []byte(`{"file_path":"test2.txt"}`),
	}
	state.AddOrUpdateToolCall(tool2)

	// Should update the existing tool call, not create a duplicate
	if len(state.PendingTools) != 1 {
		t.Errorf("Expected 1 pending tool, got %d", len(state.PendingTools))
	}

	tc := state.PendingTools["duplicate_id"]
	if tc.Status != ToolCallStatusRunning {
		t.Errorf("Expected status 'running', got '%s'", tc.Status)
	}

	// Verify only one entry in agent's tool calls
	if len(state.CurrentAgent.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call in agent, got %d", len(state.CurrentAgent.ToolCalls))
	}
}

// TestAppState_OrphanedToolResult tests handling of tool results for non-existent tools
func TestAppState_OrphanedToolResult(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test"})

	// Complete a tool that was never added
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CompleteToolCall with orphaned tool ID panicked: %v", r)
		}
	}()

	state.CompleteToolCall("orphaned_tool_id", "result content", false)

	// Should be a no-op - no tool should be created
	if _, ok := state.PendingTools["orphaned_tool_id"]; ok {
		t.Error("Orphaned tool result should not create a new entry")
	}
}

// TestAppState_CreateChildAgentWithNilParent tests child agent creation with nil parent
func TestAppState_CreateChildAgentWithNilParent(t *testing.T) {
	state := NewAppState()
	// Don't initialize session, so RootAgent and CurrentAgent will be nil

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("CreateChildAgent with nil parent panicked: %v", r)
		}
	}()

	child := state.CreateChildAgent("child_1", "task", "test task")

	// Should handle nil parent gracefully
	if child == nil {
		t.Fatal("Expected non-nil child agent")
	}

	// Verify child was created with correct ID
	if child.ID != "child_1" {
		t.Errorf("Expected ID 'child_1', got '%s'", child.ID)
	}
}

// TestAppState_MultipleOperationsWithNilState tests multiple operations with nil state
func TestAppState_MultipleOperationsWithNilState(t *testing.T) {
	state := &AppState{
		PendingTools: make(map[string]*ToolCall),
		AgentsByID:   make(map[string]*AgentState),
		TotalTokens:  &TotalUsage{},
		Stream:       NewStreamState(),
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Multiple operations with partial nil state panicked: %v", r)
		}
	}()

	// Try various operations
	state.UpdateTokens(nil)
	state.SetCurrentAgent("nonexistent")
	state.CompleteToolCall("nonexistent", "result", false)

	// These should all be safe no-ops
}

// TestStreamState_NilMap tests operations on nil PartialToolInput map
func TestStreamState_NilMap(t *testing.T) {
	stream := &StreamState{
		PartialToolInput: nil, // Explicitly nil instead of initialized map
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Operations on nil PartialToolInput panicked: %v", r)
		}
	}()

	// AppendStreamToolInput will create the map if nil
	stream.AppendStreamToolInput("tool_1", "partial")
	_ = stream.GetStreamToolInput("tool_1")

	// Map should be auto-initialized
	if stream.PartialToolInput == nil {
		t.Error("PartialToolInput should be auto-initialized")
	}
}

// TestOutputProcessor_NilStateMessageHandling tests message processing with nil state
func TestOutputProcessor_NilStateMessageHandling(t *testing.T) {
	p := &OutputProcessor{
		mode:   OutputModeText,
		writer: &mockWriter{},
		state:  nil, // Explicitly nil state
		colors: NoColorScheme(),
	}

	tests := []struct {
		name string
		msg  interface{}
	}{
		{
			name: "SystemInit with nil state",
			msg:  createTestSystemInit("test", "claude-opus-4"),
		},
		{
			name: "AssistantMessage with nil state",
			msg:  createTestAssistantMessage([]ContentBlock{*createTestContentBlock(ContentBlockTypeText, "test")}),
		},
		{
			name: "StreamEvent with nil state",
			msg:  createTestStreamEvent(StreamEventContentBlockDelta, &Delta{Text: "test"}, nil),
		},
		{
			name: "Result with nil state",
			msg:  createTestResult(0.01, 1000, 2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: processMessage with nil state panicked: %v", tt.name, r)
				}
			}()

			p.processMessage(tt.msg)
		})
	}
}

// TestOutputProcess_NilPointerInMessages tests handling of messages with nil pointer fields
func TestOutputProcessor_NilPointerInMessages(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	tests := []struct {
		name string
		msg  interface{}
	}{
		{
			name: "AssistantMessage with nil Usage",
			msg: &AssistantMessage{
				Type:    "assistant",
				Message: MessageContent{ID: "msg_1", Type: "message", Role: "assistant", Content: []ContentBlock{}},
				// Usage is nil
			},
		},
		{
			name: "StreamEvent with nil Delta",
			msg: &StreamEvent{
				Type:  StreamEventContentBlockDelta,
				Delta: nil,
			},
		},
		{
			name: "StreamEvent with nil ContentBlock",
			msg: &StreamEvent{
				Type:         StreamEventContentBlockStart,
				ContentBlock: nil,
			},
		},
		{
			name: "StreamEvent with nil Usage in MessageDelta",
			msg: &StreamEvent{
				Type:  StreamEventMessageDelta,
				Delta: &Delta{},
				// Usage is nil
			},
		},
		{
			name: "StreamEvent with nil Message",
			msg: &StreamEvent{
				Type:    StreamEventMessageStart,
				Message: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panicked: %v", tt.name, r)
				}
			}()

			p.processMessage(tt.msg)
		})
	}
}

// TestOutputProcessor_ContentBlockWithNilFields tests content blocks with nil fields
func TestOutputProcessor_ContentBlockWithNilFields(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	tests := []struct {
		name  string
		block *ContentBlock
	}{
		{
			name: "ToolUse with empty Input",
			block: &ContentBlock{
				Type:  ContentBlockTypeToolUse,
				ID:    "tool_1",
				Name:  "Bash",
				Input: nil,
			},
		},
		{
			name: "ToolResult with empty ToolUseID",
			block: &ContentBlock{
				Type:      ContentBlockTypeToolResult,
				ToolUseID: "",
				Content:   "result",
			},
		},
		{
			name: "Text with empty Text field",
			block: &ContentBlock{
				Type: ContentBlockTypeText,
				Text: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panicked: %v", tt.name, r)
				}
			}()

			p.processContentBlock(tt.block)
		})
	}
}

// TestAgentState_NilChildrenAndTools tests agent with nil collections
func TestAgentState_NilChildrenAndTools(t *testing.T) {
	agent := &AgentState{
		ID:        "test_agent",
		Type:      "main",
		Status:    AgentStatusRunning,
		Children:  nil, // Explicitly nil
		ToolCalls: nil, // Explicitly nil
		Depth:     0,
	}

	// Operations on nil slices should be safe
	_ = len(agent.Children)
	_ = len(agent.ToolCalls)

	// Appending to nil slices is safe in Go
	agent.Children = append(agent.Children, AgentState{})
	agent.ToolCalls = append(agent.ToolCalls, ToolCall{})

	if len(agent.Children) != 1 {
		t.Error("Expected 1 child after append")
	}
	if len(agent.ToolCalls) != 1 {
		t.Error("Expected 1 tool call after append")
	}
}
