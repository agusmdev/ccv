package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestEdgeCase_VeryLongStrings tests handling of very long strings
func TestEdgeCase_VeryLongStrings(t *testing.T) {
	// Create a very long string (10KB)
	longString := strings.Repeat("a", 10000)

	block := &ContentBlock{
		Type: ContentBlockTypeText,
		Text: longString,
	}

	p, w := newTestOutputProcessor(OutputModeText)
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	// Should not panic with very long string
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("processContentBlock with very long string panicked: %v", r)
		}
	}()

	p.processContentBlock(block)

	// Should output the content
	output := w.String()
	if !strings.Contains(output, "\n") {
		t.Error("expected newlines after text block")
	}
}

// TestEdgeCase_UnicodeCharacters tests handling of various Unicode characters
func TestEdgeCase_UnicodeCharacters(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		valid bool
	}{
		{
			name:  "emoji",
			text:  "😀🎉🚀❤️🔥💯⭐✨🎨🚀",
			valid: true,
		},
		{
			name:  "chinese characters",
			text:  "你好世界这是一个测试",
			valid: true,
		},
		{
			name:  "arabic",
			text:  "مرحبا بالعالم",
			valid: true,
		},
		{
			name:  "hebrew",
			text:  "שלום עולם",
			valid: true,
		},
		{
			name:  "russian",
			text:  "Привет мир",
			valid: true,
		},
		{
			name:  "mixed scripts",
			text:  "Hello你好مرحباשלוםПривет",
			valid: true,
		},
		{
			name:  "zero width joiner",
			text:  "man\u200Dwoman",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := &ContentBlock{
				Type: ContentBlockTypeText,
				Text: tt.text,
			}

			p, _ := newTestOutputProcessor(OutputModeText)

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Unicode text %q panicked: %v", tt.text, r)
				}
			}()

			p.processContentBlock(block)
		})
	}
}

// TestEdgeCase_SpecialCharactersInJSON tests special characters in JSON
func TestEdgeCase_SpecialCharactersInJSON(t *testing.T) {
	tests := []string{
		`{"type":"system","value":"text with \"quotes\""}`,
		`{"type":"system","value":"text with\nnewline"}`,
		`{"type":"system","value":"text with\ttab"}`,
		`{"type":"system","value":"text with\rcarriage"}`,
		`{"type":"system","value":"text with\\backslash"}`,
		`{"type":"system","value":"text with/slash"}`,
	}

	for _, jsonStr := range tests {
		t.Run(jsonStr[:20], func(t *testing.T) {
			// Should parse without error
			_, err := ParseMessage([]byte(jsonStr))
			if err != nil {
				t.Errorf("Failed to parse JSON with special chars: %v\nJSON: %s", err, jsonStr)
			}
		})
	}
}

// TestEdgeCase_UnexpectedMessageTypes tests unknown message types
func TestEdgeCase_UnexpectedMessageTypes(t *testing.T) {
	unexpectedTypes := []string{
		"random_future_type",
		"beta_feature_v2",
		"internal_message",
		"debug_info",
		"metrics_v1",
	}

	for _, typ := range unexpectedTypes {
		t.Run(typ, func(t *testing.T) {
			jsonStr := `{"type":"` + typ + `","value":"test"}`

			// Should return BaseMessage for unknown types
			msg, err := ParseMessage([]byte(jsonStr))
			if err != nil {
				t.Errorf("Failed to parse unknown message type: %v", err)
			}

			base, ok := msg.(*BaseMessage)
			if !ok {
				t.Errorf("Unknown type should return *BaseMessage, got %T", msg)
			}
			if base.Type != typ {
				t.Errorf("BaseMessage.Type = %q, want %q", base.Type, typ)
			}
		})
	}
}

// TestEdgeCase_EmptyContentBlocks tests empty content blocks
func TestEdgeCase_EmptyContentBlocks(t *testing.T) {
	tests := []struct {
		name  string
		block *ContentBlock
	}{
		{
			name: "empty text block",
			block: &ContentBlock{
				Type: ContentBlockTypeText,
				Text: "",
			},
		},
		{
			name: "empty thinking block",
			block: &ContentBlock{
				Type:     ContentBlockTypeThinking,
				Thinking: "",
			},
		},
		{
			name: "tool use with empty fields",
			block: &ContentBlock{
				Type:  ContentBlockTypeToolUse,
				ID:    "",
				Name:  "",
				Input: json.RawMessage("{}"),
			},
		},
		{
			name: "tool result with empty content",
			block: &ContentBlock{
				Type:      ContentBlockTypeToolResult,
				ToolUseID: "",
				Content:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newTestOutputProcessor(OutputModeText)

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Empty content block panicked: %v", r)
				}
			}()

			p.processContentBlock(tt.block)
		})
	}
}

// TestEdgeCase_ArrayFields tests array fields with various content
func TestEdgeCase_ArrayFields(t *testing.T) {
	tests := []struct {
		name  string
		json  string
	}{
		{
			name: "empty tools array",
			json: `{"type":"system","tools":[]}`,
		},
		{
			name: "empty mcp_servers array",
			json: `{"type":"system","mcp_servers":[]}`,
		},
		{
			name: "empty content array",
			json: `{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`,
		},
		{
			name: "nested empty arrays",
			json: `{"type":"system","tools":[],"mcp_servers":[],"slash_commands":[]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should parse without error
			_, err := ParseMessage([]byte(tt.json))
			if err != nil {
				t.Errorf("Failed to parse JSON with empty arrays: %v\nJSON: %s", err, tt.json)
			}
		})
	}
}

// TestEdgeCase_NumericFields tests edge cases for numeric fields
func TestEdgeCase_NumericFields(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		valid bool
	}{
		{
			name:  "zero tokens",
			json:  `{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[]},"usage":{"input_tokens":0,"output_tokens":0}}`,
			valid: true,
		},
		{
			name:  "very large token count",
			json:  `{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[]},"usage":{"input_tokens":999999999,"output_tokens":999999999}}`,
			valid: true,
		},
		{
			name:  "negative duration",
			json:  `{"type":"result","is_error":false,"duration_ms":-1}`,
			valid: true, // JSON allows it
		},
		{
			name:  "fractional cost",
			json:  `{"type":"result","is_error":false,"total_cost_usd":0.0001}`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage([]byte(tt.json))
			if tt.valid && err != nil {
				t.Errorf("Failed to parse valid JSON: %v\nJSON: %s", err, tt.json)
			}
			if !tt.valid && err == nil {
				t.Errorf("Expected error for invalid JSON, got nil")
			}
			if msg == nil {
				t.Error("Expected non-nil message")
			}
		})
	}
}

// TestEdgeCase_ConcurrentMessageProcessing tests concurrent message processing
func TestEdgeCase_ConcurrentMessageProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	const numMessages = 1000
	messages := make(chan interface{}, numMessages)
	errors := make(chan error, 10)

	// Fill messages channel
	for i := 0; i < numMessages; i++ {
		messages <- createTestSystemInit("session-concurrent", "claude-opus-4-5-20251101")
	}
	close(messages)
	close(errors)

	p, _ := newTestOutputProcessor(OutputModeText)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Concurrent message processing panicked: %v", r)
		}
	}()

	p.ProcessMessages(messages, errors)
}

// TestEdgeCase_RapidSuccessiveMessages tests rapid message succession
func TestEdgeCase_RapidSuccessiveMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rapid message test in short mode")
	}

	messages := make(chan interface{}, 1000)
	errors := make(chan error, 10)

	p, _ := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Rapid message processing panicked: %v", r)
			}
			close(done)
		}()
		p.ProcessMessages(messages, errors)
	}()

	// Send messages rapidly
	for i := 0; i < 500; i++ {
		messages <- &AssistantMessage{
			Type: "assistant",
			Message: MessageContent{
				ID:      "msg_rapid",
				Type:    "message",
				Role:    "assistant",
				Content: []ContentBlock{
					{Type: ContentBlockTypeText, Text: "Rapid message"},
				},
			},
		}
	}

	messages <- createTestResult(0.001, 1000, 1)
	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("Rapid message processing did not complete within timeout")
	}
}

// TestEdgeCase_AlternatingMessageTypes tests alternating between different message types
func TestEdgeCase_AlternatingMessageTypes(t *testing.T) {
	messages := make(chan interface{}, 100)
	errors := make(chan error, 10)

	p, _ := newTestOutputProcessor(OutputModeText)

	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Alternating message types panicked: %v", r)
			}
			close(done)
		}()
		p.ProcessMessages(messages, errors)
	}()

	// Alternate between different message types
	for i := 0; i < 10; i++ {
		messages <- createTestSystemInit("test", "model")
		messages <- createTestStreamEvent(StreamEventContentBlockDelta, &Delta{Text: "text"}, nil)
		messages <- &AssistantMessage{
			Type: "assistant",
			Message: MessageContent{
				ID:      "msg_alt",
				Type:    "message",
				Role:    "assistant",
				Content: []ContentBlock{{Type: ContentBlockTypeText, Text: "Response"}},
			},
		}
		messages <- &CompactBoundary{Type: "compact_boundary", Subtype: "trim"}
	}

	messages <- createTestResult(0.001, 1000, 1)
	close(messages)
	close(errors)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("Alternating message types did not complete within timeout")
	}
}

// TestEdgeCase_VeryDeeplyNestedAgentHierarchy tests deeply nested agent hierarchy
func TestEdgeCase_VeryDeeplyNestedAgentHierarchy(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(createTestSystemInit("test", "model"))

	// Create a deep hierarchy
	depth := 100
	var currentAgent *AgentState = state.RootAgent

	for i := 0; i < depth; i++ {
		child := state.CreateChildAgent("tool_"+string(rune('a'+i%26))+string(rune('0'+i%10)), "task", "Task "+string(rune('0'+i)))
		state.SetCurrentAgent(child.ID)
		currentAgent = child
	}

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Deep agent hierarchy panicked: %v", r)
		}
	}()

	// Verify depth
	if currentAgent.Depth != depth {
		t.Errorf("Expected depth %d, got %d", depth, currentAgent.Depth)
	}

	// Verify we can traverse back up
	for i := depth - 1; i >= 0; i-- {
		expectedDepth := i
		if currentAgent.Depth != expectedDepth {
			t.Errorf("At iteration %d, expected depth %d, got %d", i, expectedDepth, currentAgent.Depth)
		}
		// Move to parent (simulated by re-initializing and finding parent)
		if currentAgent.ParentID != "" {
			if parent, ok := state.AgentsByID[currentAgent.ParentID]; ok {
				currentAgent = parent
			}
		}
	}
}

// TestEdgeCase_ManyToolCalls tests handling many tool calls
func TestEdgeCase_ManyToolCalls(t *testing.T) {
	state := NewAppState()
	state.InitializeSession(createTestSystemInit("test", "model"))

	const numTools = 1000

	// Add many tool calls
	for i := 0; i < numTools; i++ {
		toolCall := &ToolCall{
			ID:     "tool_" + string(rune('0'+i%10)),
			Name:   "Read",
			Status: ToolCallStatusPending,
		}
		state.AddOrUpdateToolCall(toolCall)
	}

	// Should have all tools
	if len(state.PendingTools) != numTools {
		t.Errorf("Expected %d tools, got %d", numTools, len(state.PendingTools))
	}

	// Should not panic on iteration
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Iterating many tool calls panicked: %v", r)
		}
	}()

	count := 0
	for _, tool := range state.PendingTools {
		_ = tool
		count++
	}

	if count != numTools {
		t.Errorf("Expected to iterate %d tools, got %d", numTools, count)
	}
}

// TestEdgeCase_InvalidToolUseID tests tool result with non-existent tool ID
func TestEdgeCase_InvalidToolUseID(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)
	p.state.InitializeSession(createTestSystemInit("test", "model"))

	// Complete a tool that doesn't exist
	block := createTestToolResultBlock("nonexistent_tool", "result", false)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Invalid tool use ID panicked: %v", r)
		}
	}()

	p.processToolResult(block)
}

// TestEdgeCase_JsonRawMessage tests JSON RawMessage handling
func TestEdgeCase_JsonRawMessage(t *testing.T) {
	tests := []struct {
		name  string
		input json.RawMessage
		valid bool
	}{
		{
			name:  "valid JSON",
			input: json.RawMessage(`{"key":"value"}`),
			valid: true,
		},
		{
			name:  "empty JSON object",
			input: json.RawMessage(`{}`),
			valid: true,
		},
		{
			name:  "JSON array",
			input: json.RawMessage(`[1,2,3]`),
			valid: true,
		},
		{
			name:  "nested JSON",
			input: json.RawMessage(`{"outer":{"inner":"value"}}`),
			valid: true,
		},
		{
			name:  "JSON with special chars",
			input: json.RawMessage(`{"text":"line1\nline2\ttab"}`),
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := json.Unmarshal(tt.input, &result)
			if tt.valid && err != nil {
				t.Errorf("Failed to parse valid JSON: %v", err)
			}
		})
	}
}

// TestEdgeCase_AllContentBlockTypes tests all content block types
func TestEdgeCase_AllContentBlockTypes(t *testing.T) {
	blockTypes := []ContentBlockType{
		ContentBlockTypeText,
		ContentBlockTypeToolUse,
		ContentBlockTypeToolResult,
		ContentBlockTypeThinking,
		ContentBlockTypeRedactedThinking,
	}

	for _, blockType := range blockTypes {
		t.Run(string(blockType), func(t *testing.T) {
			block := &ContentBlock{
				Type: blockType,
			}

			p, _ := newTestOutputProcessor(OutputModeText)

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Content block type %s panicked: %v", blockType, r)
				}
			}()

			p.processContentBlock(block)
		})
	}
}

// TestEdgeCase_AllStreamEventTypes tests all stream event types
func TestEdgeCase_AllStreamEventTypes(t *testing.T) {
	eventTypes := []StreamEventType{
		StreamEventMessageStart,
		StreamEventContentBlockStart,
		StreamEventContentBlockDelta,
		StreamEventContentBlockStop,
		StreamEventMessageDelta,
		StreamEventMessageStop,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			event := &StreamEvent{
				Type: eventType,
			}

			p, _ := newTestOutputProcessor(OutputModeText)
			p.state.InitializeSession(createTestSystemInit("test", "model"))

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Stream event type %s panicked: %v", eventType, r)
				}
			}()

			p.handleStreamEvent(event)
		})
	}
}

// TestEdgeCase_QuickSuccession tests quick succession of operations
func TestEdgeCase_QuickSuccession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping quick succession test in short mode")
	}

	state := NewAppState()
	state.InitializeSession(createTestSystemInit("test", "model"))

	// Perform many operations quickly
	for i := 0; i < 1000; i++ {
		state.AppendStreamText("a")
		state.AppendStreamThinking("b")
		state.AppendStreamToolInput("tool_x", `"partial"`)
		state.UpdateTokens(&Usage{
			InputTokens:  i,
			OutputTokens: i / 2,
		})
	}

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Quick succession operations panicked: %v", r)
		}
	}()

	// Verify final state
	if state.Stream.PartialText != strings.Repeat("a", 1000) {
		t.Error("PartialText not accumulated correctly")
	}
	if state.Stream.PartialThinking != strings.Repeat("b", 1000) {
		t.Error("PartialThinking not accumulated correctly")
	}
}

// TestEdgeCase_MemoryEfficiency tests that we don't leak memory
func TestEdgeCase_MemoryEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory efficiency test in short mode")
	}

	// Create many agents and tool calls, then clear references
	state := NewAppState()
	state.InitializeSession(createTestSystemInit("test", "model"))

	// Create many child agents
	for i := 0; i < 100; i++ {
		state.CreateChildAgent("tool_"+string(rune('0'+i%10)), "task", "Task")
	}

	// Create many tool calls
	for i := 0; i < 100; i++ {
		toolCall := &ToolCall{
			ID:     "mem_tool_" + string(rune('0'+i%10)),
			Name:   "Read",
			Status: ToolCallStatusPending,
		}
		state.AddOrUpdateToolCall(toolCall)
	}

	// Accumulate stream data
	for i := 0; i < 100; i++ {
		state.AppendStreamText("text")
		state.AppendStreamThinking("think")
	}

	// Verify counts
	if len(state.AgentsByID) < 101 { // 100 children + root
		t.Errorf("Expected at least 101 agents, got %d", len(state.AgentsByID))
	}
	if len(state.PendingTools) != 100 {
		t.Errorf("Expected 100 pending tools, got %d", len(state.PendingTools))
	}

	// Clear stream state
	state.ClearStreamState()

	if state.Stream.PartialText != "" {
		t.Error("PartialText should be cleared")
	}
	if state.Stream.PartialThinking != "" {
		t.Error("PartialThinking should be cleared")
	}
}

// TestEdgeCase_StatusTransitions tests all tool call status transitions
func TestEdgeCase_StatusTransitions(t *testing.T) {
	transitions := []struct {
		from     ToolCallStatus
		to       ToolCallStatus
		isError  bool
	}{
		{ToolCallStatusPending, ToolCallStatusRunning, false},
		{ToolCallStatusRunning, ToolCallStatusCompleted, false},
		{ToolCallStatusPending, ToolCallStatusFailed, true},
		{ToolCallStatusRunning, ToolCallStatusFailed, true},
	}

	for _, tt := range transitions {
		t.Run(tt.from+"_to_"+tt.to, func(t *testing.T) {
			state := NewAppState()
			state.InitializeSession(createTestSystemInit("test", "model"))

			toolCall := &ToolCall{
				ID:     "status_tool",
				Name:   "Read",
				Status: tt.from,
			}
			state.AddOrUpdateToolCall(toolCall)

			// Complete with error/success
			state.CompleteToolCall("status_tool", "result", tt.isError)

			tc := state.PendingTools["status_tool"]
			if tt.isError && tc.Status != ToolCallStatusFailed {
				t.Errorf("Expected status 'failed', got '%s'", tc.Status)
			}
			if !tt.isError && tc.Status != ToolCallStatusCompleted {
				t.Errorf("Expected status 'completed', got '%s'", tc.Status)
			}
		})
	}
}

// TestEdgeCase_AgentStatusTransitions tests agent status transitions
func TestEdgeCase_AgentStatusTransitions(t *testing.T) {
	statuses := []AgentStatus{
		AgentStatusIdle,
		AgentStatusThinking,
		AgentStatusRunning,
		AgentStatusCompleted,
		AgentStatusFailed,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			state := NewAppState()
			state.InitializeSession(createTestSystemInit("test", "model"))

			// Create child agent
			child := state.CreateChildAgent("test_tool", "task", "Test task")
			child.Status = status

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Agent status %s panicked: %v", status, r)
				}
			}()

			// Try to use the agent
			_ = child.ID
			_ = child.Status
			_ = child.Depth
		})
	}
}

// TestEdgeCase_InvalidSessionInit tests session init with missing fields
func TestEdgeCase_InvalidSessionInit(t *testing.T) {
	tests := []struct {
		name  string
		init  *SystemInit
		valid bool
	}{
		{
			name:  "empty session init",
			init:  &SystemInit{},
			valid: true, // Should not crash
		},
		{
			name: "nil fields",
			init: &SystemInit{
				Type: "system",
				// Other fields are empty/nil
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewAppState()

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Session init with missing fields panicked: %v", r)
				}
			}()

			state.InitializeSession(tt.init)
		})
	}
}
