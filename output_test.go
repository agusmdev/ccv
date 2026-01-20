package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewOutputProcessor(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		verbose  bool
		quiet    bool
		expected OutputMode
	}{
		{
			name:     "default mode",
			format:   "",
			verbose:  false,
			quiet:    false,
			expected: OutputModeText,
		},
		{
			name:     "json format",
			format:   "json",
			verbose:  false,
			quiet:    false,
			expected: OutputModeJSON,
		},
		{
			name:     "verbose mode",
			format:   "",
			verbose:  true,
			quiet:    false,
			expected: OutputModeVerbose,
		},
		{
			name:     "quiet mode",
			format:   "",
			verbose:  false,
			quiet:    true,
			expected: OutputModeQuiet,
		},
		{
			name:     "quiet overrides verbose",
			format:   "",
			verbose:  true,
			quiet:    true,
			expected: OutputModeQuiet,
		},
		{
			name:     "quiet overrides json",
			format:   "json",
			verbose:  false,
			quiet:    true,
			expected: OutputModeQuiet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOutputProcessor(tt.format, tt.verbose, tt.quiet)
			if p.mode != tt.expected {
				t.Errorf("NewOutputProcessor(%q, %v, %v).mode = %v, want %v",
					tt.format, tt.verbose, tt.quiet, p.mode, tt.expected)
			}
			if p.state == nil {
				t.Error("expected state to be initialized")
			}
			if p.colors == nil {
				t.Error("expected colors to be initialized")
			}
		})
	}
}

func TestProcessMessage_SystemInit(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	sysInit := createTestSystemInit("session-123", "claude-opus-4-5-20251101")
	p.processMessage(sysInit)

	output := w.String()
	if !strings.Contains(output, "Session started") {
		t.Errorf("expected output to contain 'Session started', got: %s", output)
	}
	if !strings.Contains(output, "claude-opus-4-5-20251101") {
		t.Errorf("expected output to contain model name, got: %s", output)
	}
	if p.state.SessionID != "session-123" {
		t.Errorf("expected session ID to be set, got: %s", p.state.SessionID)
	}
}

func TestProcessMessage_SystemInit_QuietMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeQuiet)

	sysInit := createTestSystemInit("session-123", "claude-opus-4-5-20251101")
	p.processMessage(sysInit)

	output := w.String()
	if strings.Contains(output, "Session started") {
		t.Errorf("quiet mode should not output session info, got: %s", output)
	}
}

func TestProcessMessage_JSONMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeJSON)

	sysInit := createTestSystemInit("session-123", "claude-opus-4-5-20251101")
	p.processMessage(sysInit)

	output := w.String()

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v, output: %s", err, output)
	}

	if result["type"] != "system" {
		t.Errorf("expected type 'system', got: %v", result["type"])
	}
}

func TestHandleStreamEvent_TextDelta(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session first
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset() // Clear session output

	// Send text delta
	event := createTestStreamEvent(StreamEventContentBlockDelta, &Delta{
		Type: string(DeltaTypeTextDelta),
		Text: "Hello, ",
	}, nil)
	p.handleStreamEvent(event)

	// Send another delta
	event2 := createTestStreamEvent(StreamEventContentBlockDelta, &Delta{
		Type: string(DeltaTypeTextDelta),
		Text: "World!",
	}, nil)
	p.handleStreamEvent(event2)

	output := w.String()
	if output != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got: %q", output)
	}

	// Check stream state accumulated
	if p.state.Stream.PartialText != "Hello, World!" {
		t.Errorf("expected stream state to accumulate text, got: %q", p.state.Stream.PartialText)
	}
}

func TestHandleStreamEvent_ThinkingDelta(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session first
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	// Send thinking delta
	event := createTestStreamEvent(StreamEventContentBlockDelta, &Delta{
		Type:     string(DeltaTypeThinkingDelta),
		Thinking: "Let me think...",
	}, nil)
	p.handleStreamEvent(event)

	output := w.String()
	if !strings.Contains(output, "[THINKING]") {
		t.Errorf("expected [THINKING] prefix, got: %q", output)
	}
	if !strings.Contains(output, "Let me think...") {
		t.Errorf("expected thinking content, got: %q", output)
	}
}

func TestHandleStreamEvent_ThinkingDelta_QuietMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeQuiet)

	// Initialize session first
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	// Send thinking delta - should not output in quiet mode
	event := createTestStreamEvent(StreamEventContentBlockDelta, &Delta{
		Type:     string(DeltaTypeThinkingDelta),
		Thinking: "Let me think...",
	}, nil)
	p.handleStreamEvent(event)

	output := w.String()
	if output != "" {
		t.Errorf("quiet mode should not output thinking, got: %q", output)
	}
}

func TestPrintToolCall_Bash(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_1", "Bash", map[string]interface{}{
		"command":     "ls -la",
		"description": "List files",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Bash") {
		t.Errorf("expected tool name 'Bash', got: %q", output)
	}
	if !strings.Contains(output, "ls -la") {
		t.Errorf("expected command 'ls -la', got: %q", output)
	}
	if !strings.Contains(output, "→") {
		t.Errorf("expected arrow symbol, got: %q", output)
	}
}

func TestPrintToolCall_Read(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_2", "Read", map[string]interface{}{
		"file_path": "/path/to/file.go",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Read") {
		t.Errorf("expected tool name 'Read', got: %q", output)
	}
	if !strings.Contains(output, "/path/to/file.go") {
		t.Errorf("expected file path, got: %q", output)
	}
}

func TestPrintToolCall_Write(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_3", "Write", map[string]interface{}{
		"file_path": "/path/to/new.go",
		"content":   "line1\nline2\nline3",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Write") {
		t.Errorf("expected tool name 'Write', got: %q", output)
	}
	if !strings.Contains(output, "/path/to/new.go") {
		t.Errorf("expected file path, got: %q", output)
	}
	if !strings.Contains(output, "3 lines") {
		t.Errorf("expected line count, got: %q", output)
	}
}

func TestPrintToolCall_Edit(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_4", "Edit", map[string]interface{}{
		"file_path":  "/path/to/edit.go",
		"old_string": "old text",
		"new_string": "new text",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Edit") {
		t.Errorf("expected tool name 'Edit', got: %q", output)
	}
	if !strings.Contains(output, "/path/to/edit.go") {
		t.Errorf("expected file path, got: %q", output)
	}
	// Should show diff
	if !strings.Contains(output, "- old text") {
		t.Errorf("expected diff removal line, got: %q", output)
	}
	if !strings.Contains(output, "+ new text") {
		t.Errorf("expected diff addition line, got: %q", output)
	}
}

func TestPrintToolCall_Glob(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_5", "Glob", map[string]interface{}{
		"pattern": "**/*.go",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Glob") {
		t.Errorf("expected tool name 'Glob', got: %q", output)
	}
	if !strings.Contains(output, "**/*.go") {
		t.Errorf("expected pattern, got: %q", output)
	}
}

func TestPrintToolCall_Grep(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_6", "Grep", map[string]interface{}{
		"pattern": "TODO",
		"glob":    "*.go",
		"path":    "./src",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Grep") {
		t.Errorf("expected tool name 'Grep', got: %q", output)
	}
	if !strings.Contains(output, "TODO") {
		t.Errorf("expected pattern, got: %q", output)
	}
}

func TestPrintToolCall_Skill(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_7", "Skill", map[string]interface{}{
		"skill": "commit",
		"args":  "-m 'test'",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "/commit") {
		t.Errorf("expected skill formatted as /commit, got: %q", output)
	}
}

func TestPrintToolCall_Task(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_8", "Task", map[string]interface{}{
		"description":   "Explore codebase",
		"subagent_type": "Explore",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Task") {
		t.Errorf("expected tool name 'Task', got: %q", output)
	}
	if !strings.Contains(output, "[Explore]") {
		t.Errorf("expected subagent type '[Explore]', got: %q", output)
	}
	if !strings.Contains(output, "Explore codebase") {
		t.Errorf("expected description 'Explore codebase', got: %q", output)
	}
}

func TestPrintToolCall_TaskWithModel(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_9", "Task", map[string]interface{}{
		"description":   "Quick search task",
		"subagent_type": "Explore",
		"model":         "haiku",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Task") {
		t.Errorf("expected tool name 'Task', got: %q", output)
	}
	if !strings.Contains(output, "haiku") {
		t.Errorf("expected model 'haiku', got: %q", output)
	}
}

func TestPrintToolCall_TaskInBackground(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_10", "Task", map[string]interface{}{
		"description":       "Long running task",
		"subagent_type":     "general-purpose",
		"run_in_background": true,
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Task") {
		t.Errorf("expected tool name 'Task', got: %q", output)
	}
	if !strings.Contains(output, "Background") {
		t.Errorf("expected background flag, got: %q", output)
	}
	if !strings.Contains(output, "running") {
		t.Errorf("expected 'running' for background flag, got: %q", output)
	}
}

func TestPrintToolCall_TaskWithMaxTurns(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_11", "Task", map[string]interface{}{
		"description":   "Plan architecture",
		"subagent_type": "Plan",
		"max_turns":     50.0,
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Task") {
		t.Errorf("expected tool name 'Task', got: %q", output)
	}
	if !strings.Contains(output, "Max turns") {
		t.Errorf("expected max_turns indicator, got: %q", output)
	}
	if !strings.Contains(output, "50") {
		t.Errorf("expected '50' for max_turns, got: %q", output)
	}
}

func TestProcessToolResult_Bash(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session and add a pending tool call
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	toolCall := createTestToolCall("tool_bash", "Bash", map[string]interface{}{
		"command": "echo hello",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	// Process tool result
	block := createTestToolResultBlock("tool_bash", "hello\nworld", false)
	p.processToolResult(block)

	output := w.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("expected output to contain 'hello', got: %q", output)
	}
	if !strings.Contains(output, "world") {
		t.Errorf("expected output to contain 'world', got: %q", output)
	}
}

func TestProcessToolResult_BashError(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session and add a pending tool call
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	toolCall := createTestToolCall("tool_bash_err", "Bash", map[string]interface{}{
		"command": "false",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	// Process error tool result
	block := createTestToolResultBlock("tool_bash_err", "error output", true)
	p.processToolResult(block)

	output := w.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error indicator, got: %q", output)
	}
	if !strings.Contains(output, "Command failed") {
		t.Errorf("expected 'Command failed' message, got: %q", output)
	}
}

func TestProcessToolResult_Glob(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session and add a pending tool call
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	toolCall := createTestToolCall("tool_glob", "Glob", map[string]interface{}{
		"pattern": "*.go",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	// Process tool result
	block := createTestToolResultBlock("tool_glob", "main.go\nrunner.go\ntypes.go", false)
	p.processToolResult(block)

	output := w.String()
	if !strings.Contains(output, "main.go") {
		t.Errorf("expected 'main.go' in output, got: %q", output)
	}
}

func TestProcessToolResult_GlobNoMatches(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session and add a pending tool call
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	toolCall := createTestToolCall("tool_glob_empty", "Glob", map[string]interface{}{
		"pattern": "*.xyz",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	// Process empty tool result
	block := createTestToolResultBlock("tool_glob_empty", "", false)
	p.processToolResult(block)

	output := w.String()
	if !strings.Contains(output, "(no matches)") {
		t.Errorf("expected '(no matches)' message, got: %q", output)
	}
}

func TestProcessToolResult_Grep(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session and add a pending tool call
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	toolCall := createTestToolCall("tool_grep", "Grep", map[string]interface{}{
		"pattern": "TODO",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	// Process tool result
	block := createTestToolResultBlock("tool_grep", "main.go:10: // TODO: fix this\nrunner.go:50: // TODO: cleanup", false)
	p.processToolResult(block)

	output := w.String()
	if !strings.Contains(output, "main.go:10") {
		t.Errorf("expected grep match in output, got: %q", output)
	}
}

func TestProcessToolResult_QuietMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeQuiet)

	// Initialize session and add a pending tool call
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	toolCall := createTestToolCall("tool_quiet", "Bash", map[string]interface{}{
		"command": "echo hello",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	// Process tool result - should not output in quiet mode
	block := createTestToolResultBlock("tool_quiet", "hello", false)
	p.processToolResult(block)

	output := w.String()
	if output != "" {
		t.Errorf("quiet mode should not output tool results, got: %q", output)
	}
}

func TestHandleResult(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	result := createTestResult(0.05, 5000, 3)
	p.handleResult(result)

	if p.result == nil {
		t.Error("expected result to be stored")
	}
	if p.result.TotalCost != 0.05 {
		t.Errorf("expected total cost 0.05, got: %f", p.result.TotalCost)
	}
	if p.state.TotalTokens.InputTokens != 1000 {
		t.Errorf("expected input tokens 1000, got: %d", p.state.TotalTokens.InputTokens)
	}
}

func TestPrintFinalSummary(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Set up result and tokens
	p.result = createTestResult(0.05, 5000, 3)
	p.state.TotalTokens = &TotalUsage{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheReadInputTokens: 200,
	}

	p.printFinalSummary()

	output := w.String()
	if !strings.Contains(output, "Tokens:") {
		t.Errorf("expected 'Tokens:' in output, got: %q", output)
	}
	if !strings.Contains(output, "Cost:") {
		t.Errorf("expected 'Cost:' in output, got: %q", output)
	}
	if !strings.Contains(output, "Duration:") {
		t.Errorf("expected 'Duration:' in output, got: %q", output)
	}
	if !strings.Contains(output, "Turns:") {
		t.Errorf("expected 'Turns:' in output, got: %q", output)
	}
}

func TestPrintFinalSummary_QuietMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeQuiet)

	// Set up result and tokens
	p.result = createTestResult(0.05, 5000, 3)
	p.state.TotalTokens = &TotalUsage{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	p.printFinalSummary()

	output := w.String()
	if output != "" {
		t.Errorf("quiet mode should not print summary, got: %q", output)
	}
}

func TestPrintFinalSummary_NoData(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// No result or tokens
	p.printFinalSummary()

	output := w.String()
	if strings.Contains(output, "Tokens:") {
		t.Errorf("should not print tokens without data, got: %q", output)
	}
}

func TestPrintDiff(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.printDiff("old line 1\nold line 2", "new line 1\nnew line 2\nnew line 3")

	output := w.String()
	if !strings.Contains(output, "- old line 1") {
		t.Errorf("expected removal line, got: %q", output)
	}
	if !strings.Contains(output, "+ new line 1") {
		t.Errorf("expected addition line, got: %q", output)
	}
}

func TestPrintAgentContext(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session to create root agent
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	p.printAgentContext()

	output := w.String()
	if !strings.Contains(output, "main") {
		t.Errorf("expected agent type 'main', got: %q", output)
	}
}

func TestPrintAgentContext_QuietMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeQuiet)

	// Initialize session
	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	p.printAgentContext()

	output := w.String()
	if output != "" {
		t.Errorf("quiet mode should not print agent context, got: %q", output)
	}
}

func TestProcessContentBlock_Text(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	block := createTestContentBlock(ContentBlockTypeText, "Hello World")
	p.processContentBlock(block)

	output := w.String()
	// Text content adds newlines but doesn't re-output the text
	// (it was streamed earlier)
	if !strings.Contains(output, "\n") {
		t.Errorf("expected newlines for text block, got: %q", output)
	}
}

func TestProcessContentBlock_Thinking(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	block := &ContentBlock{
		Type:     ContentBlockTypeThinking,
		Thinking: "Let me analyze this...",
	}
	p.processContentBlock(block)

	output := w.String()
	if !strings.Contains(output, "[THINKING]") {
		t.Errorf("expected [THINKING] prefix, got: %q", output)
	}
	if !strings.Contains(output, "Let me analyze this...") {
		t.Errorf("expected thinking content, got: %q", output)
	}
}

func TestProcessContentBlock_ToolUse(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Initialize session
	p.state.InitializeSession(createTestSystemInit("test", "model"))

	// Add tool call to pending
	toolCall := createTestToolCall("tool_test", "Read", map[string]interface{}{
		"file_path": "/path/to/file.go",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	// Process tool use block
	block := createTestToolUseBlock("tool_test", "Read", map[string]interface{}{
		"file_path": "/path/to/file.go",
	})
	p.processContentBlock(block)

	output := w.String()
	if !strings.Contains(output, "Read") {
		t.Errorf("expected tool name, got: %q", output)
	}
}
