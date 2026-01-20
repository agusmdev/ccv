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
