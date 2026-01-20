package main

import (
	"encoding/json"
	"fmt"
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

func TestPrintToolCall_EnterPlanMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_enter_plan", "EnterPlanMode", map[string]interface{}{})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "[PLAN MODE]") {
		t.Errorf("expected '[PLAN MODE]' indicator, got: %q", output)
	}
	if !strings.Contains(output, "Entering plan mode") {
		t.Errorf("expected 'Entering plan mode' message, got: %q", output)
	}
}

func TestPrintToolCall_ExitPlanMode_Basic(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_exit_plan", "ExitPlanMode", map[string]interface{}{})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "[PLAN MODE]") {
		t.Errorf("expected '[PLAN MODE]' indicator, got: %q", output)
	}
	if !strings.Contains(output, "Exiting plan mode") {
		t.Errorf("expected 'Exiting plan mode' message, got: %q", output)
	}
}

func TestPrintToolCall_ExitPlanMode_WithPermissions(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_exit_plan", "ExitPlanMode", map[string]interface{}{
		"allowedPrompts": []interface{}{
			map[string]interface{}{
				"tool":   "Bash",
				"prompt": "run tests",
			},
			map[string]interface{}{
				"tool":   "Bash",
				"prompt": "build the project",
			},
		},
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "[PLAN MODE]") {
		t.Errorf("expected '[PLAN MODE]' indicator, got: %q", output)
	}
	if !strings.Contains(output, "Requested permissions") {
		t.Errorf("expected 'Requested permissions', got: %q", output)
	}
	if !strings.Contains(output, "Bash:") {
		t.Errorf("expected 'Bash:' permission label, got: %q", output)
	}
	if !strings.Contains(output, "run tests") {
		t.Errorf("expected 'run tests' permission, got: %q", output)
	}
	if !strings.Contains(output, "build the project") {
		t.Errorf("expected 'build the project' permission, got: %q", output)
	}
}

func TestPrintToolCall_ExitPlanMode_WithRemoteSync(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_exit_plan", "ExitPlanMode", map[string]interface{}{
		"pushToRemote":     true,
		"remoteSessionId":  "session-abc-123",
		"remoteSessionUrl": "https://claude.ai/session/abc-123",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "[PLAN MODE]") {
		t.Errorf("expected '[PLAN MODE]' indicator, got: %q", output)
	}
	if !strings.Contains(output, "Remote sync") {
		t.Errorf("expected 'Remote sync' indicator, got: %q", output)
	}
	if !strings.Contains(output, "session-abc-123") {
		t.Errorf("expected session ID, got: %q", output)
	}
	if !strings.Contains(output, "https://claude.ai/session/abc-123") {
		t.Errorf("expected session URL, got: %q", output)
	}
}

func TestPrintToolCall_Context7ResolveLibraryID(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_ctx7", "mcp__context7__resolve-library-id", map[string]interface{}{
		"libraryName": "react",
		"id":          "react@18",
		"version":     "18.2.0",
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Should show the shortened MCP tool name
	if !strings.Contains(output, "context7:resolve-library-id") {
		t.Errorf("expected shortened tool name 'context7:resolve-library-id', got: %q", output)
	}
	// Should show library name
	if !strings.Contains(output, "Library:") {
		t.Errorf("expected 'Library:' label, got: %q", output)
	}
	if !strings.Contains(output, "react") {
		t.Errorf("expected library name 'react', got: %q", output)
	}
	// Should show ID
	if !strings.Contains(output, "ID:") {
		t.Errorf("expected 'ID:' label, got: %q", output)
	}
	if !strings.Contains(output, "react@18") {
		t.Errorf("expected library ID 'react@18', got: %q", output)
	}
	// Version should NOT show in normal mode
	if strings.Contains(output, "Version:") {
		t.Errorf("version should not show in normal mode, got: %q", output)
	}
}

func TestPrintToolCall_Context7ResolveLibraryID_Verbose(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	toolCall := createTestToolCall("tool_ctx7", "mcp__context7__resolve-library-id", map[string]interface{}{
		"libraryName": "vue",
		"id":          "vue@3",
		"version":     "3.3.0",
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Version SHOULD show in verbose mode
	if !strings.Contains(output, "Version:") {
		t.Errorf("expected 'Version:' label in verbose mode, got: %q", output)
	}
	if !strings.Contains(output, "3.3.0") {
		t.Errorf("expected version '3.3.0' in verbose mode, got: %q", output)
	}
}

func TestPrintToolCall_Context7QueryDocs(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_qry", "mcp__context7__query-docs", map[string]interface{}{
		"id":    "react@18",
		"query": "how to use useEffect",
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Should show the shortened MCP tool name
	if !strings.Contains(output, "context7:query-docs") {
		t.Errorf("expected shortened tool name 'context7:query-docs', got: %q", output)
	}
	// Should show library ID
	if !strings.Contains(output, "Library ID:") {
		t.Errorf("expected 'Library ID:' label, got: %q", output)
	}
	if !strings.Contains(output, "react@18") {
		t.Errorf("expected library ID 'react@18', got: %q", output)
	}
	// Should show query
	if !strings.Contains(output, "Query:") {
		t.Errorf("expected 'Query:' label, got: %q", output)
	}
	if !strings.Contains(output, "how to use useEffect") {
		t.Errorf("expected query 'how to use useEffect', got: %q", output)
	}
}

func TestPrintToolCall_Context7QueryDocs_LongQuery(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Create a query longer than 120 chars
	longQuery := "how to use useEffect with dependencies array and cleanup function in functional components with typescript and react hooks"
	toolCall := createTestToolCall("tool_qry_long", "mcp__context7__query-docs", map[string]interface{}{
		"id":    "react@18",
		"query": longQuery,
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Should show truncated query with "..."
	if !strings.Contains(output, "...") {
		t.Errorf("expected truncated query with '...', got: %q", output)
	}
	// Should NOT show "Full query:" in normal mode
	if strings.Contains(output, "Full query:") {
		t.Errorf("should not show full query in normal mode, got: %q", output)
	}
}

func TestPrintToolCall_Context7QueryDocs_Verbose(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	// Create a query longer than 120 chars to test verbose mode expansion
	longQuery := "how to use useEffect with dependencies array and cleanup function in functional components with typescript and react hooks"
	toolCall := createTestToolCall("tool_qry_verb", "mcp__context7__query-docs", map[string]interface{}{
		"id":    "next@14",
		"query": longQuery,
		"limit": float64(10),
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Should show "Full query:" in verbose mode
	if !strings.Contains(output, "Full query:") {
		t.Errorf("expected 'Full query:' label in verbose mode, got: %q", output)
	}
	// Should show limit in verbose mode
	if !strings.Contains(output, "Limit:") {
		t.Errorf("expected 'Limit:' label in verbose mode, got: %q", output)
	}
	if !strings.Contains(output, "10") {
		t.Errorf("expected limit '10' in verbose mode, got: %q", output)
	}
}

// TestPrintToolCall_AskUserQuestion tests the AskUserQuestion tool formatting
func TestPrintToolCall_AskUserQuestion(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_ask", "AskUserQuestion", map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "What would you like to do?",
				"header":   "Choice",
				"multiSelect": false,
				"options": []interface{}{
					map[string]interface{}{
						"label": "Option A",
						"description": "Do something cool",
					},
					map[string]interface{}{
						"label": "Option B",
						"description": "Do something else",
					},
				},
			},
		},
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "AskUserQuestion") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "What would you like to do?") {
		t.Errorf("expected question text, got: %q", output)
	}
	if !strings.Contains(output, "Option A") {
		t.Errorf("expected option A, got: %q", output)
	}
	if !strings.Contains(output, "Option B") {
		t.Errorf("expected option B, got: %q", output)
	}
}

// TestPrintToolCall_TodoWrite tests the TodoWrite tool formatting
func TestPrintToolCall_TodoWrite(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_todo", "TodoWrite", map[string]interface{}{
		"todos": []interface{}{
			map[string]interface{}{
				"content": "First task",
				"status": "pending",
			},
			map[string]interface{}{
				"content": "Second task",
				"status": "in_progress",
			},
			map[string]interface{}{
				"content": "Third task",
				"status": "completed",
			},
		},
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "TodoWrite") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "First task") {
		t.Errorf("expected first task, got: %q", output)
	}
	// Should have status icons
	if !strings.Contains(output, "○") {
		t.Errorf("expected pending icon, got: %q", output)
	}
	if !strings.Contains(output, "◐") {
		t.Errorf("expected in_progress icon, got: %q", output)
	}
	if !strings.Contains(output, "●") {
		t.Errorf("expected completed icon, got: %q", output)
	}
}

// TestPrintToolCall_WebSearch tests the WebSearch tool formatting
func TestPrintToolCall_WebSearch(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_web", "WebSearch", map[string]interface{}{
		"query": "golang best practices",
		"allowed_domains": []interface{}{"go.dev", "golang.org"},
		"blocked_domains": []interface{}{"spam.com"},
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "WebSearch") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "golang best practices") {
		t.Errorf("expected query, got: %q", output)
	}
	if !strings.Contains(output, "Allowed") {
		t.Errorf("expected allowed domains, got: %q", output)
	}
	if !strings.Contains(output, "Blocked") {
		t.Errorf("expected blocked domains, got: %q", output)
	}
}

// TestPrintToolCall_WebFetch tests the WebFetch tool formatting
func TestPrintToolCall_WebFetch(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_fetch", "WebFetch", map[string]interface{}{
		"url": "https://example.com/article",
		"prompt": "Summarize this article",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "WebFetch") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "https://example.com/article") {
		t.Errorf("expected URL, got: %q", output)
	}
	if !strings.Contains(output, "Prompt") {
		t.Errorf("expected prompt label, got: %q", output)
	}
}

// TestPrintToolCall_WebFetch_Verbose tests WebFetch in verbose mode
func TestPrintToolCall_WebFetch_Verbose(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	// Create a long prompt to test truncation and verbose expansion
	longPrompt := strings.Repeat("word ", 50) // 400 chars
	toolCall := createTestToolCall("tool_fetch", "WebFetch", map[string]interface{}{
		"url": "https://example.com/article",
		"prompt": longPrompt,
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "WebFetch") {
		t.Errorf("expected tool name, got: %q", output)
	}
	// In verbose mode, should show full prompt
	if !strings.Contains(output, "Full prompt") {
		t.Errorf("expected full prompt in verbose mode, got: %q", output)
	}
}

// TestPrintToolCall_LS tests the LS tool formatting
func TestPrintToolCall_LS(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_ls", "LS", map[string]interface{}{
		"path": "/home/user/project",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "LS") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "/home/user/project") {
		t.Errorf("expected path, got: %q", output)
	}
}

// TestPrintToolCall_NotebookRead tests the NotebookRead tool formatting
func TestPrintToolCall_NotebookRead(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_nb", "NotebookRead", map[string]interface{}{
		"notebook_path": "/path/to/notebook.ipynb",
		"offset": float64(5),
		"limit": float64(10),
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "NotebookRead") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "/path/to/notebook.ipynb") {
		t.Errorf("expected notebook path, got: %q", output)
	}
}

// TestPrintToolCall_NotebookEdit tests the NotebookEdit tool formatting
func TestPrintToolCall_NotebookEdit(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_nbedit", "NotebookEdit", map[string]interface{}{
		"notebook_path": "/path/to/notebook.ipynb",
		"cell_id": "cell_123",
		"edit_mode": "replace",
		"cell_type": "code",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "NotebookEdit") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "cell_123") {
		t.Errorf("expected cell ID, got: %q", output)
	}
	if !strings.Contains(output, "replace") {
		t.Errorf("expected edit mode, got: %q", output)
	}
}

// TestPrintToolCall_KillShell tests the KillShell tool formatting
func TestPrintToolCall_KillShell(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("tool_kill", "KillShell", map[string]interface{}{
		"shell_id": "shell_abc123",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "KillShell") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "shell_abc123") {
		t.Errorf("expected shell ID, got: %q", output)
	}
}

// TestPrintToolCall_TaskOutput tests the TaskOutput tool formatting
func TestPrintToolCall_TaskOutput(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("task_out", "TaskOutput", map[string]interface{}{
		"task_id": "task_xyz",
		"block": true,
		"timeout": float64(30000),
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "TaskOutput") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "task_xyz") {
		t.Errorf("expected task ID, got: %q", output)
	}
	if !strings.Contains(output, "blocking") {
		t.Errorf("expected blocking indicator, got: %q", output)
	}
}

// TestHandleStreamEvent_ContentBlockStop tests content_block_stop handling
func TestHandleStreamEvent_ContentBlockStop(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))
	p.state.AppendStreamThinking("some thinking")
	w.Reset()

	// Send content_block_stop event
	event := createTestStreamEvent(StreamEventContentBlockStop, nil, nil)
	p.handleStreamEvent(event)

	output := w.String()
	// Should have reset colors and added newline for thinking
	if !strings.Contains(output, "\n") {
		t.Errorf("expected newline after thinking block stop, got: %q", output)
	}
}

// TestHandleStreamEvent_MessageDelta tests message_delta handling with usage
func TestHandleStreamEvent_MessageDelta(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	// Send message_delta with usage
	event := &StreamEvent{
		Type: StreamEventMessageDelta,
		Delta: &Delta{
			StopReason: "end_turn",
		},
		Usage: &Usage{
			InputTokens:  500,
			OutputTokens: 200,
		},
	}
	p.handleStreamEvent(event)

	// Tokens should be updated
	if p.state.TotalTokens.InputTokens != 500 {
		t.Errorf("expected input tokens 500, got %d", p.state.TotalTokens.InputTokens)
	}
	if p.state.TotalTokens.OutputTokens != 200 {
		t.Errorf("expected output tokens 200, got %d", p.state.TotalTokens.OutputTokens)
	}
}

// TestHandleStreamEvent_MessageStop tests message_stop handling
func TestHandleStreamEvent_MessageStop(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	event := createTestStreamEvent(StreamEventMessageStop, nil, nil)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("message_stop handling panicked: %v", r)
		}
	}()

	p.handleStreamEvent(event)
}

// TestHandleContentBlockStart_TaskAgentCreation tests Task tool creates child agent
func TestHandleContentBlockStart_TaskAgentCreation(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	// Send Task tool content_block_start
	block := &ContentBlock{
		Type: ContentBlockTypeToolUse,
		ID:   "task_tool_123",
		Name: "Task",
		Input: json.RawMessage(`{
			"subagent_type": "Explore",
			"description": "Explore the codebase",
			"model": "haiku"
		}`),
	}

	event := createTestStreamEvent(StreamEventContentBlockStart, nil, block)
	p.handleContentBlockStart(event)

	// Child agent should be created
	childAgent, ok := p.state.AgentsByID["task_tool_123"]
	if !ok {
		t.Fatal("expected child agent to be created")
	}
	if childAgent.Type != "Explore" {
		t.Errorf("expected agent type 'Explore', got '%s'", childAgent.Type)
	}
	if childAgent.Description != "Explore the codebase" {
		t.Errorf("expected description 'Explore the codebase', got '%s'", childAgent.Description)
	}
	if childAgent.ParentID != "main" {
		t.Errorf("expected parent ID 'main', got '%s'", childAgent.ParentID)
	}
}

// TestHandleContentBlockDelta_PartialJSON tests partial JSON accumulation
func TestHandleContentBlockDelta_PartialJSON(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))
	w.Reset()

	toolID := "tool_123"

	// Send multiple partial JSON deltas
	event1 := createTestStreamEvent(StreamEventContentBlockDelta, &Delta{
		Type:        string(DeltaTypeInputJSONDelta),
		PartialJSON: `{"command":`,
	}, &ContentBlock{ID: toolID})

	event2 := createTestStreamEvent(StreamEventContentBlockDelta, &Delta{
		Type:        string(DeltaTypeInputJSONDelta),
		PartialJSON: `"ls -la"`,
	}, &ContentBlock{ID: toolID})

	p.handleContentBlockDelta(event1)
	p.handleContentBlockDelta(event2)

	// Partial tool input should be accumulated
	result := p.state.GetStreamToolInput(toolID)
	expected := `{"command":"ls -la"`
	if result != expected {
		t.Errorf("expected accumulated input %q, got %q", expected, result)
	}
}

// TestProcessMessage_CompactBoundary tests compact_boundary message handling
func TestProcessMessage_CompactBoundary(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	msg := &CompactBoundary{
		Type:      "compact_boundary",
		Subtype:   "context_trim",
		SessionID: "test-session",
	}

	p.processMessage(msg)

	// CompactBoundary should be skipped in text mode
	output := w.String()
	if strings.Contains(output, "compact_boundary") {
		t.Error("compact_boundary should not produce output in text mode")
	}
}

// TestProcessMessage_JSONMode_CompactBoundary tests compact_boundary in JSON mode
func TestProcessMessage_JSONMode_CompactBoundary(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeJSON)

	msg := &CompactBoundary{
		Type:      "compact_boundary",
		Subtype:   "context_trim",
		SessionID: "test-session",
	}

	p.processMessage(msg)

	output := w.String()

	// In JSON mode, should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Errorf("expected valid JSON output, got error: %v, output: %s", err, output)
	}
	if result["type"] != "compact_boundary" {
		t.Errorf("expected type 'compact_boundary', got: %v", result["type"])
	}
}

// TestPrintAgentContext_NestedAgents tests nested agent context display
func TestPrintAgentContext_NestedAgents(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))

	// Create nested child agents
	child1 := p.state.CreateChildAgent("child_1", "task", "First task")
	p.state.SetCurrentAgent("child_1")
	w.Reset()

	child2 := p.state.CreateChildAgent("child_2", "explore", "Explore")
	p.state.SetCurrentAgent("child_2")
	w.Reset()

	p.printAgentContext()

	output := w.String()
	// Should show indentation based on depth
	expectedIndent := strings.Repeat("  ", child2.Depth)
	if !strings.HasPrefix(output, expectedIndent) {
		t.Errorf("expected indentation of depth %d, got: %q", child2.Depth, output)
	}
	// Should show agent type
	if !strings.Contains(output, "explore") {
		t.Errorf("expected agent type 'explore', got: %q", output)
	}
}

// TestPrintAgentContext_VerboseMode tests agent context in verbose mode
func TestPrintAgentContext_VerboseMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	p.state.InitializeSession(createTestSystemInit("test", "model"))
	p.state.RootAgent.Description = "Main agent doing work"
	w.Reset()

	p.printAgentContext()

	output := w.String()
	// In verbose mode, should show description
	if !strings.Contains(output, "Main agent doing work") {
		t.Errorf("expected description in verbose mode, got: %q", output)
	}
}

// TestPrintFinalSummary_DurationFormats tests different duration formats
func TestPrintFinalSummary_DurationFormats(t *testing.T) {
	tests := []struct {
		name         string
		duration     int64
		shouldContain string
	}{
		{
			name:         "milliseconds",
			duration:     500,
			shouldContain: "500ms",
		},
		{
			name:         "seconds",
			duration:     5000,
			shouldContain: "5.0s",
		},
		{
			name:         "minutes and seconds",
			duration:     125000, // 2m 5s
			shouldContain: "2m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, w := newTestOutputProcessor(OutputModeText)

			p.result = createTestResult(0.01, tt.duration, 2)
			p.state.TotalTokens = &TotalUsage{
				InputTokens:  1000,
				OutputTokens: 500,
			}

			p.printFinalSummary()

			output := w.String()
			if !strings.Contains(output, tt.shouldContain) {
				t.Errorf("expected duration format %q, got: %q", tt.shouldContain, output)
			}
		})
	}
}

// TestPrintFinalSummary_ZeroValues tests summary with zero values
func TestPrintFinalSummary_ZeroValues(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	// Set up with zero values
	p.result = &Result{
		TotalCost:  0,
		DurationMS: 0,
		NumTurns:   0,
	}
	p.state.TotalTokens = &TotalUsage{
		InputTokens:  0,
		OutputTokens: 0,
	}

	p.printFinalSummary()

	output := w.String()
	// Should not print anything when all values are zero
	if strings.Contains(output, "Tokens:") {
		t.Error("should not print tokens with zero values")
	}
}

// TestPrintFinalSummary_CacheInfo tests cache information display
func TestPrintFinalSummary_CacheInfo(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.result = createTestResult(0.01, 5000, 2)
	p.state.TotalTokens = &TotalUsage{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheReadInputTokens: 200,
		CacheCreationInputTokens: 50,
	}

	p.printFinalSummary()

	output := w.String()
	if !strings.Contains(output, "Cache") {
		t.Error("expected cache information in summary")
	}
	if !strings.Contains(output, "read") {
		t.Error("expected cache read information")
	}
	if !strings.Contains(output, "created") {
		t.Error("expected cache creation information")
	}
}

// TestHandleProcessMessages_PanicRecovery tests panic recovery in ProcessMessages
func TestHandleProcessMessages_PanicRecovery(t *testing.T) {
	messages := make(chan interface{}, 10)
	errors := make(chan error, 10)

	p := &OutputProcessor{
		mode:   OutputModeText,
		writer: &mockWriter{},
		state:  NewAppState(),
		colors: NoColorScheme(),
	}

	// Send a message that might cause issues
	messages <- &BaseMessage{Type: "unknown"}
	close(messages)
	close(errors)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ProcessMessages panicked: %v", r)
		}
	}()

	p.ProcessMessages(messages, errors)
}

// TestHandleContentBlock_ToolUseWithNilInput tests tool_use block with nil input
func TestHandleContentBlock_ToolUseWithNilInput(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))

	toolCall := createTestToolCall("tool_nil", "Read", nil)
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	block := &ContentBlock{
		Type:  ContentBlockTypeToolUse,
		ID:    "tool_nil",
		Name:  "Read",
		Input: nil,
	}

	// Should not panic with nil input
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("processContentBlock with nil input panicked: %v", r)
		}
	}()

	p.processContentBlock(block)
}

// TestHandleDefaultResult tests the default tool result handler
func TestHandleDefaultResult(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("unknown_tool", "SomeUnknownTool", map[string]interface{}{
		"param": "value",
	})

	block := createTestToolResultBlock("unknown_tool", "result content", false)
	p.state.InitializeSession(createTestSystemInit("test", "model"))

	handleDefaultResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "SomeUnknownTool") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "completed") {
		t.Errorf("expected completion status, got: %q", output)
	}
}

// TestHandleDefaultResult_VerboseMode tests default handler in verbose mode
func TestHandleDefaultResult_VerboseMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	toolCall := createTestToolCall("unknown_tool", "SomeTool", map[string]interface{}{})

	block := createTestToolResultBlock("unknown_tool", "multi\nline\nresult", false)

	handleDefaultResult(p, toolCall, block)

	output := w.String()
	// In verbose mode, should show result content
	if !strings.Contains(output, "multi") {
		t.Errorf("expected result content in verbose mode, got: %q", output)
	}
	if !strings.Contains(output, "line") {
		t.Errorf("expected result content in verbose mode, got: %q", output)
	}
}

// TestHandleDefaultResult_WithError tests default handler with error
func TestHandleDefaultResult_WithError(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("failed_tool", "FailingTool", map[string]interface{}{})

	block := createTestToolResultBlock("failed_tool", "error details", true)

	handleDefaultResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error indicator, got: %q", output)
	}
}

// TestProcessToolResult_TaskResultSwitchesParent tests Task result switches back to parent
func TestProcessToolResult_TaskResultSwitchesParent(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))

	// Create a child agent
	child := p.state.CreateChildAgent("task_tool", "Task", "Do something")
	p.state.SetCurrentAgent("task_tool")
	w.Reset()

	// Complete the Task tool
	block := createTestToolResultBlock("task_tool", "task completed", false)
	p.processToolResult(block)

	// Should switch back to parent (main)
	if p.state.CurrentAgent.ID != "main" {
		t.Errorf("expected current agent to be 'main', got '%s'", p.state.CurrentAgent.ID)
	}
	// Child should be marked completed
	if child.Status != AgentStatusCompleted {
		t.Errorf("expected child status 'completed', got '%s'", child.Status)
	}
}

// TestProcessMessages_ErrorChannelDoesNotStopProcessing tests that errors on the error channel don't stop message processing
func TestProcessMessages_ErrorChannelDoesNotStopProcessing(t *testing.T) {
	messages := make(chan interface{}, 10)
	errors := make(chan error, 10)

	p, _ := newTestOutputProcessor(OutputModeText)

	// Send an error followed by a valid message
	errors <- fmt.Errorf("test error")
	messages <- createTestSystemInit("test", "model")

	close(messages)
	close(errors)

	// Should process both the error and the message without stopping
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ProcessMessages panicked with error channel: %v", r)
		}
	}()

	p.ProcessMessages(messages, errors)
}

// TestProcessMessages_MessagesChannelCloseTriggersFinalSummary tests that closing messages channel triggers printFinalSummary
func TestProcessMessages_MessagesChannelCloseTriggersFinalSummary(t *testing.T) {
	messages := make(chan interface{}, 10)
	errors := make(chan error, 10)

	p, w := newTestOutputProcessor(OutputModeText)

	// Set up result to verify summary is printed
	p.result = createTestResult(0.01, 5000, 2)
	p.state.TotalTokens = &TotalUsage{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	close(messages)
	close(errors)

	p.ProcessMessages(messages, errors)

	output := w.String()
	if !strings.Contains(output, "Tokens:") {
		t.Error("expected final summary to be printed when messages channel closes")
	}
}

// TestProcessMessage_JSONMode_MarshalError tests JSON mode handles marshal errors gracefully
func TestProcessMessage_JSONMode_MarshalError(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeJSON)

	// Create a message that can't be marshaled (contains a channel)
	type BadMessage struct {
		Type    string
		Channel chan int
	}
	msg := &BadMessage{
		Type:    "bad",
		Channel: make(chan int),
	}

	// Should not panic, just log error to stderr
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("processMessage panicked on unmarshalable type: %v", r)
		}
	}()

	p.processMessage(msg)
}

// TestHandleContentBlockStart_CreatesToolCall tests content_block_start creates tool call
func TestHandleContentBlockStart_CreatesToolCall(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))

	toolID := "tool_123"
	block := &ContentBlock{
		Type:  ContentBlockTypeToolUse,
		ID:    toolID,
		Name:  "Read",
		Input: json.RawMessage(`{"file_path": "/path/to/file.go"}`),
	}

	event := createTestStreamEvent(StreamEventContentBlockStart, nil, block)
	p.handleContentBlockStart(event)

	// Tool call should be in pending tools
	toolCall, ok := p.state.PendingTools[toolID]
	if !ok {
		t.Fatal("expected tool call to be created")
	}
	if toolCall.Name != "Read" {
		t.Errorf("expected tool name 'Read', got '%s'", toolCall.Name)
	}
	if toolCall.Status != ToolCallStatusPending {
		t.Errorf("expected status 'pending', got '%s'", toolCall.Status)
	}
}

// TestHandleContentBlockStart_NilContentBlock tests content_block_start with nil ContentBlock
func TestHandleContentBlockStart_NilContentBlock(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	event := createTestStreamEvent(StreamEventContentBlockStart, nil, nil)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("handleContentBlockStart with nil ContentBlock panicked: %v", r)
		}
	}()

	p.handleContentBlockStart(event)
}

// TestHandleContentBlockDelta_NilDelta tests content_block_delta with nil delta
func TestHandleContentBlockDelta_NilDelta(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))

	event := createTestStreamEvent(StreamEventContentBlockDelta, nil, nil)

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("handleContentBlockDelta with nil delta panicked: %v", r)
		}
	}()

	p.handleContentBlockDelta(event)
}

// TestHandleWebSearchResult_WithResults tests WebSearch result handler with results
func TestHandleWebSearchResult_WithResults(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("web_1", "WebSearch", map[string]interface{}{
		"query": "golang testing",
	})
	block := createTestToolResultBlock("web_1", "result1\nresult2\nresult3", false)

	handleWebSearchResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "result1") {
		t.Errorf("expected result1 in output, got: %q", output)
	}
	if !strings.Contains(output, "result2") {
		t.Errorf("expected result2 in output, got: %q", output)
	}
}

// TestHandleWebSearchResult_EmptyResults tests WebSearch result handler with empty results
func TestHandleWebSearchResult_EmptyResults(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("web_2", "WebSearch", map[string]interface{}{})
	block := createTestToolResultBlock("web_2", "", false)

	handleWebSearchResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "(no results)") {
		t.Errorf("expected '(no results)' message, got: %q", output)
	}
}

// TestHandleWebSearchResult_WithError tests WebSearch result handler with error
func TestHandleWebSearchResult_WithError(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("web_3", "WebSearch", map[string]interface{}{})
	block := createTestToolResultBlock("web_3", "some error content", true)

	handleWebSearchResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error indicator, got: %q", output)
	}
	if !strings.Contains(output, "Search failed") {
		t.Errorf("expected 'Search failed' message, got: %q", output)
	}
}

// TestHandleKillShellResult_Success tests KillShell result handler success case
func TestHandleKillShellResult_Success(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("kill_1", "KillShell", map[string]interface{}{
		"shell_id": "shell_123",
	})
	block := createTestToolResultBlock("kill_1", "", false)

	handleKillShellResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success indicator, got: %q", output)
	}
	if !strings.Contains(output, "Shell terminated") {
		t.Errorf("expected 'Shell terminated' message, got: %q", output)
	}
}

// TestHandleKillShellResult_Failure tests KillShell result handler failure case
func TestHandleKillShellResult_Failure(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("kill_2", "KillShell", map[string]interface{}{
		"shell_id": "shell_456",
	})
	block := createTestToolResultBlock("kill_2", "shell not found", true)

	handleKillShellResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error indicator, got: %q", output)
	}
	if !strings.Contains(output, "failed") {
		t.Errorf("expected 'failed' message, got: %q", output)
	}
}

// TestHandleKillShellResult_VerboseMode tests KillShell result in verbose mode
func TestHandleKillShellResult_VerboseMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	toolCall := createTestToolCall("kill_3", "KillShell", map[string]interface{}{
		"shell_id": "shell_789",
	})
	block := createTestToolResultBlock("kill_3", "Terminated shell shell_789\nExit code: 0", false)

	handleKillShellResult(p, toolCall, block)

	output := w.String()
	// In verbose mode, should show the content
	if !strings.Contains(output, "Terminated shell") {
		t.Errorf("expected verbose output content, got: %q", output)
	}
}

// TestHandleTaskOutputResult_WithContent tests TaskOutput result handler with content
func TestHandleTaskOutputResult_WithContent(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("task_out_1", "TaskOutput", map[string]interface{}{
		"task_id": "task_123",
	})
	block := createTestToolResultBlock("task_out_1", "line1\nline2\nline3", false)

	handleTaskOutputResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "line1") {
		t.Errorf("expected line1 in output, got: %q", output)
	}
	if !strings.Contains(output, "line2") {
		t.Errorf("expected line2 in output, got: %q", output)
	}
}

// TestHandleTaskOutputResult_WithError tests TaskOutput result handler with error
func TestHandleTaskOutputResult_WithError(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("task_out_2", "TaskOutput", map[string]interface{}{
		"task_id": "task_456",
	})
	block := createTestToolResultBlock("task_out_2", "", true)

	handleTaskOutputResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error indicator, got: %q", output)
	}
	if !strings.Contains(output, "Failed to retrieve") {
		t.Errorf("expected error message, got: %q", output)
	}
}

// TestHandleTaskOutputResult_EmptyContent tests TaskOutput result handler with empty content
func TestHandleTaskOutputResult_EmptyContent(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("task_out_3", "TaskOutput", map[string]interface{}{
		"task_id": "task_789",
	})
	block := createTestToolResultBlock("task_out_3", "", false)

	handleTaskOutputResult(p, toolCall, block)

	output := w.String()
	// Empty content without error should be silent
	if strings.Contains(output, "✗") {
		t.Errorf("empty non-error should be silent, got: %q", output)
	}
}

// TestHandleGrepResult_WithError tests Grep result handler with error
func TestHandleGrepResult_WithError(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("grep_err", "Grep", map[string]interface{}{
		"pattern": "test",
	})
	block := createTestToolResultBlock("grep_err", "permission denied", true)

	handleGrepResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error indicator, got: %q", output)
	}
	if !strings.Contains(output, "Search failed") {
		t.Errorf("expected 'Search failed' message, got: %q", output)
	}
}

// TestHandleGrepResult_NoMatches tests Grep result handler with no matches
func TestHandleGrepResult_NoMatches(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("grep_empty", "Grep", map[string]interface{}{
		"pattern": "nonexistent",
	})
	block := createTestToolResultBlock("grep_empty", "", false)

	handleGrepResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "(no matches)") {
		t.Errorf("expected '(no matches)' message, got: %q", output)
	}
}

// TestHandleGlobResult_WithError tests Glob result handler with error
func TestHandleGlobResult_WithError(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("glob_err", "Glob", map[string]interface{}{
		"pattern": "**/*.go",
	})
	block := createTestToolResultBlock("glob_err", "invalid pattern", true)

	handleGlobResult(p, toolCall, block)

	output := w.String()
	if !strings.Contains(output, "✗") {
		t.Errorf("expected error indicator, got: %q", output)
	}
	if !strings.Contains(output, "Search failed") {
		t.Errorf("expected 'Search failed' message, got: %q", output)
	}
}

// TestHandleBashResult_VerboseMode tests Bash result handler in verbose mode
func TestHandleBashResult_VerboseMode(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	toolCall := createTestToolCall("bash_v", "Bash", map[string]interface{}{
		"command": "ls -la",
		"description": "List files",
	})
	block := createTestToolResultBlock("bash_v", "file1.txt\nfile2.txt\nfile3.txt", false)

	handleBashResult(p, toolCall, block)

	output := w.String()
	// Should show all lines
	if !strings.Contains(output, "file1.txt") {
		t.Errorf("expected file1.txt in output, got: %q", output)
	}
	// Each line should be indented
	if !strings.Contains(output, "  file1.txt") {
		t.Errorf("expected indented output, got: %q", output)
	}
}

// TestPrintToolCall_MultiEdit tests MultiEdit tool formatting
func TestPrintToolCall_MultiEdit(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("multi_1", "MultiEdit", map[string]interface{}{
		"file_path": "/path/to/file.go",
		"edits": []interface{}{
			map[string]interface{}{
				"old_string": "old1",
				"new_string": "new1",
			},
			map[string]interface{}{
				"old_string": "old2",
				"new_string": "new2",
			},
		},
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "MultiEdit") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "/path/to/file.go") {
		t.Errorf("expected file path, got: %q", output)
	}
	// Should show diffs for each edit
	if !strings.Contains(output, "- old1") {
		t.Errorf("expected first diff removal, got: %q", output)
	}
	if !strings.Contains(output, "+ new1") {
		t.Errorf("expected first diff addition, got: %q", output)
	}
	if !strings.Contains(output, "- old2") {
		t.Errorf("expected second diff removal, got: %q", output)
	}
	if !strings.Contains(output, "+ new2") {
		t.Errorf("expected second diff addition, got: %q", output)
	}
}

// TestPrintToolCall_MultiEdit_SingleEdit tests MultiEdit with a single edit
func TestPrintToolCall_MultiEdit_SingleEdit(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("multi_2", "MultiEdit", map[string]interface{}{
		"file_path": "/path/to/file.go",
		"edits": []interface{}{
			map[string]interface{}{
				"old_string": "old",
				"new_string": "new",
			},
		},
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "- old") {
		t.Errorf("expected diff removal, got: %q", output)
	}
	// Should NOT have separator for single edit
	if strings.Contains(output, "---") {
		t.Errorf("should not have edit separator for single edit, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightNavigate tests Playwright navigate tool
func TestPrintToolCall_PlaywrightNavigate(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("nav_1", "navigate", map[string]interface{}{
		"url": "https://example.com",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "navigate") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "https://example.com") {
		t.Errorf("expected URL, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightNavigate_WithTimeout tests Playwright navigate with timeout
func TestPrintToolCall_PlaywrightNavigate_WithTimeout(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	toolCall := createTestToolCall("nav_2", "navigate", map[string]interface{}{
		"url":     "https://example.com",
		"timeout": float64(30000),
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "30000ms") {
		t.Errorf("expected timeout in verbose mode, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightClick tests Playwright click tool
func TestPrintToolCall_PlaywrightClick(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("click_1", "click", map[string]interface{}{
		"selector": "button#submit",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "click") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "button#submit") {
		t.Errorf("expected selector, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightType tests Playwright type tool
func TestPrintToolCall_PlaywrightType(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("type_1", "type", map[string]interface{}{
		"selector": "input#name",
		"text":     "hello world",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "type") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "input#name") {
		t.Errorf("expected element selector, got: %q", output)
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected text, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightType_LongText tests Playwright type with long text truncation
func TestPrintToolCall_PlaywrightType_LongText(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	longText := strings.Repeat("word ", 30) // 180 chars
	toolCall := createTestToolCall("type_2", "type", map[string]interface{}{
		"selector": "textarea",
		"text":     longText,
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Should truncate long text
	if !strings.Contains(output, "...") {
		t.Errorf("expected truncated text with '...', got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightFill tests Playwright fill tool
func TestPrintToolCall_PlaywrightFill(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("fill_1", "fill", map[string]interface{}{
		"selector": "input#email",
		"text":     "test@example.com",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "fill") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "input#email") {
		t.Errorf("expected element selector, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightScreenshot tests Playwright screenshot tool
func TestPrintToolCall_PlaywrightScreenshot(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("shot_1", "screenshot", map[string]interface{}{
		"path": "/path/to/screenshot.png",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "screenshot") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "/path/to/screenshot.png") {
		t.Errorf("expected file path, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightScreenshot_Verbose tests screenshot in verbose mode with type
func TestPrintToolCall_PlaywrightScreenshot_Verbose(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	toolCall := createTestToolCall("shot_2", "screenshot", map[string]interface{}{
		"path": "/path/to/screenshot.png",
		"type": "jpeg",
	})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "jpeg") {
		t.Errorf("expected screenshot type in verbose mode, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightSnapshot tests Playwright snapshot tool
func TestPrintToolCall_PlaywrightSnapshot(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("snap_1", "snapshot", map[string]interface{}{})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "snapshot") {
		t.Errorf("expected tool name, got: %q", output)
	}
}

// TestPrintToolCall_PlaywrightSnapshot_Verbose tests snapshot in verbose mode
func TestPrintToolCall_PlaywrightSnapshot_Verbose(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeVerbose)

	toolCall := createTestToolCall("snap_2", "snapshot", map[string]interface{}{})

	p.printToolCall(toolCall)

	output := w.String()
	if !strings.Contains(output, "Capturing page state") {
		t.Errorf("expected verbose message in verbose mode, got: %q", output)
	}
}

// TestPrintToolCall_MCPNavigate tests MCP-style navigate tool
func TestPrintToolCall_MCPNavigate(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("nav_3", "mcp__browser__navigate", map[string]interface{}{
		"url": "https://example.com/page",
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Should show shortened MCP tool name
	if !strings.Contains(output, "browser:navigate") {
		t.Errorf("expected shortened tool name, got: %q", output)
	}
	if !strings.Contains(output, "https://example.com/page") {
		t.Errorf("expected URL, got: %q", output)
	}
}

// TestPrintToolCall_MCPScreenshot tests MCP-style screenshot tool
func TestPrintToolCall_MCPScreenshot(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	toolCall := createTestToolCall("shot_3", "mcp__browser__screenshot", map[string]interface{}{
		"path": "/tmp/screenshot.png",
	})

	p.printToolCall(toolCall)

	output := w.String()
	// Should show shortened MCP tool name
	if !strings.Contains(output, "browser:screenshot") {
		t.Errorf("expected shortened tool name, got: %q", output)
	}
	if !strings.Contains(output, "/tmp/screenshot.png") {
		t.Errorf("expected file path, got: %q", output)
	}
}

// TestProcessToolResult_UnknownTool tests tool result for unknown tool type
func TestProcessToolResult_UnknownTool(t *testing.T) {
	p, w := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))
	toolCall := createTestToolCall("unknown", "UnknownTool", map[string]interface{}{
		"param": "value",
	})
	p.state.AddOrUpdateToolCall(toolCall)
	w.Reset()

	block := createTestToolResultBlock("unknown", "completed successfully", false)
	p.processToolResult(block)

	output := w.String()
	// Should use default handler
	if !strings.Contains(output, "UnknownTool") {
		t.Errorf("expected tool name, got: %q", output)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("expected success indicator, got: %q", output)
	}
}

// TestProcessContentBlock_InvalidJSONInToolInput tests tool_use with invalid JSON input
func TestProcessContentBlock_InvalidJSONInToolInput(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))

	toolCall := createTestToolCall("tool_bad", "Bash", nil)
	p.state.AddOrUpdateToolCall(toolCall)

	block := &ContentBlock{
		Type:  ContentBlockTypeToolUse,
		ID:    "tool_bad",
		Name:  "Bash",
		Input: json.RawMessage(`{invalid json}`),
	}

	// Should not panic with invalid JSON
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("processContentBlock with invalid JSON panicked: %v", r)
		}
	}()

	p.processContentBlock(block)
}

// TestProcessContentBlock_NilContentBlockFields tests content block with nil fields
func TestProcessContentBlock_NilContentBlockFields(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	// Create blocks with various nil field combinations
	tests := []struct {
		name string
		block *ContentBlock
	}{
		{
			name: "nil text field",
			block: &ContentBlock{
				Type: ContentBlockTypeText,
				Text: "",
			},
		},
		{
			name: "nil thinking field",
			block: &ContentBlock{
				Type:     ContentBlockTypeThinking,
				Thinking: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("processContentBlock with nil field panicked: %v", r)
				}
			}()
			p.processContentBlock(tt.block)
		})
	}
}

// TestProcessMessage_AssistantMessageWithNilUsage tests assistant message with nil usage
func TestProcessMessage_AssistantMessageWithNilUsage(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	msg := &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_test",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{},
			Usage:   nil,
		},
	}

	// Should not panic with nil usage
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("processMessage with nil usage panicked: %v", r)
		}
	}()

	p.processMessage(msg)
}

// TestHandleAssistantMessage_WithUsage tests assistant message updates token usage
func TestHandleAssistantMessage_WithUsage(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	msg := &AssistantMessage{
		Type: "assistant",
		Message: MessageContent{
			ID:      "msg_test",
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{},
			Usage: &Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
		},
	}

	p.handleAssistantMessage(msg)

	if p.state.TotalTokens.InputTokens != 1000 {
		t.Errorf("expected input tokens 1000, got %d", p.state.TotalTokens.InputTokens)
	}
	if p.state.TotalTokens.OutputTokens != 500 {
		t.Errorf("expected output tokens 500, got %d", p.state.TotalTokens.OutputTokens)
	}
}

// TestHandleStreamEvent_NilContentBlockInDelta tests content_block_delta with nil ContentBlock
func TestHandleStreamEvent_ContentBlockDelta_NilContentBlock(t *testing.T) {
	p, _ := newTestOutputProcessor(OutputModeText)

	p.state.InitializeSession(createTestSystemInit("test", "model"))

	// Event with delta but nil content block - shouldn't crash
	event := &StreamEvent{
		Type:  StreamEventContentBlockDelta,
		Delta: &Delta{
			Type: string(DeltaTypeInputJSONDelta),
			PartialJSON: `{"test":`,
		},
		ContentBlock: nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("handleContentBlockDelta with nil ContentBlock panicked: %v", r)
		}
	}()

	p.handleContentBlockDelta(event)
}
