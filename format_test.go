package main

import (
	"strings"
	"testing"
)

func TestFormatCodeBlock(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		showLineNo     bool
		expectedLines  int
		containsLineNo bool
	}{
		{
			name:           "single line with line numbers",
			code:           "fmt.Println(\"hello\")",
			showLineNo:     true,
			expectedLines:  1,
			containsLineNo: true,
		},
		{
			name:           "multiple lines with line numbers",
			code:           "line1\nline2\nline3",
			showLineNo:     true,
			expectedLines:  3,
			containsLineNo: true,
		},
		{
			name:           "without line numbers",
			code:           "line1\nline2",
			showLineNo:     false,
			expectedLines:  2,
			containsLineNo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FormatConfig{
				Colors:     NoColorScheme(),
				ShowLineNo: tt.showLineNo,
				Indent:     "  ",
			}

			result := FormatCodeBlock(tt.code, config)
			lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")

			if len(lines) != tt.expectedLines {
				t.Errorf("expected %d lines, got %d", tt.expectedLines, len(lines))
			}

			// Check if line numbers are present
			hasLineNo := strings.Contains(result, "1 ")
			if hasLineNo != tt.containsLineNo {
				t.Errorf("line number presence: expected %v, got %v", tt.containsLineNo, hasLineNo)
			}
		})
	}
}

func TestFormatCodeBlock_NilConfig(t *testing.T) {
	// Should use defaults when config is nil
	result := FormatCodeBlock("test code", nil)

	if result == "" {
		t.Error("expected non-empty result with nil config")
	}
	if !strings.Contains(result, "test code") {
		t.Errorf("expected code to be in result, got: %q", result)
	}
}

func TestHighlightSyntax_NoColors(t *testing.T) {
	scheme := NoColorScheme()

	input := `echo "hello" --flag 123`
	result := highlightSyntax(input, scheme)

	// With no colors, output should be unchanged
	if result != input {
		t.Errorf("expected unchanged input with no colors, got: %q", result)
	}
}

func TestHighlightSyntax_WithColors(t *testing.T) {
	scheme := DefaultScheme()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "highlights double-quoted strings",
			input:    `echo "hello world"`,
			contains: Yellow, // String color
		},
		{
			name:     "highlights single-quoted strings",
			input:    `echo 'hello world'`,
			contains: Yellow,
		},
		{
			name:     "highlights numbers",
			input:    `count 42 items`,
			contains: Magenta, // Number color
		},
		{
			name:     "highlights environment variables",
			input:    `echo $HOME`,
			contains: Cyan, // Env var color
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highlightSyntax(tt.input, scheme)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected color code %q in result: %q", tt.contains, result)
			}
			if !strings.Contains(result, Reset) {
				t.Errorf("expected reset code in result: %q", result)
			}
		})
	}
}

func TestHighlightSyntax_ThreadSafety(t *testing.T) {
	// Run concurrent calls to verify thread safety of sync.Once initialization
	scheme := DefaultScheme()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_ = highlightSyntax(`echo "test" --flag 123`, scheme)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestFormatFilePath(t *testing.T) {
	scheme := DefaultScheme()
	path := "/path/to/file.go"

	result := FormatFilePath(path, scheme)

	if !strings.Contains(result, path) {
		t.Errorf("expected path in result, got: %q", result)
	}
	if !strings.Contains(result, scheme.FilePath) {
		t.Errorf("expected FilePath color, got: %q", result)
	}
	if !strings.HasSuffix(result, scheme.Reset) {
		t.Errorf("expected Reset at end, got: %q", result)
	}
}

func TestFormatCommand(t *testing.T) {
	scheme := DefaultScheme()
	cmd := "go build"

	result := FormatCommand(cmd, scheme)

	if !strings.Contains(result, cmd) {
		t.Errorf("expected command in result, got: %q", result)
	}
	if !strings.Contains(result, Bold) {
		t.Errorf("expected Bold in result, got: %q", result)
	}
}

func TestFormatBullet(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatBullet(scheme)

	if !strings.Contains(result, "●") {
		t.Errorf("expected bullet character, got: %q", result)
	}
}

func TestFormatArrow(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatArrow(scheme)

	if !strings.Contains(result, "→") {
		t.Errorf("expected arrow character, got: %q", result)
	}
}

func TestFormatSuccess(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatSuccess(scheme)

	if !strings.Contains(result, "✓") {
		t.Errorf("expected checkmark character, got: %q", result)
	}
	if !strings.Contains(result, scheme.Success) {
		t.Errorf("expected Success color, got: %q", result)
	}
}

func TestFormatError(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatError(scheme)

	if !strings.Contains(result, "✗") {
		t.Errorf("expected cross character, got: %q", result)
	}
	if !strings.Contains(result, scheme.Error) {
		t.Errorf("expected Error color, got: %q", result)
	}
}

func TestFormatDiffLine(t *testing.T) {
	scheme := DefaultScheme()

	tests := []struct {
		name       string
		line       string
		isAddition bool
		prefix     string
		color      string
	}{
		{
			name:       "addition line",
			line:       "new content",
			isAddition: true,
			prefix:     "+ ",
			color:      scheme.DiffAdd,
		},
		{
			name:       "removal line",
			line:       "old content",
			isAddition: false,
			prefix:     "- ",
			color:      scheme.DiffRemove,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDiffLine(tt.line, tt.isAddition, scheme)

			if !strings.Contains(result, tt.prefix) {
				t.Errorf("expected prefix %q, got: %q", tt.prefix, result)
			}
			if !strings.Contains(result, tt.line) {
				t.Errorf("expected line content, got: %q", result)
			}
			if !strings.Contains(result, tt.color) {
				t.Errorf("expected color %q, got: %q", tt.color, result)
			}
		})
	}
}

func TestFormatSectionSeparator(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatSectionSeparator(20, scheme)

	if !strings.Contains(result, "─") {
		t.Errorf("expected separator character, got: %q", result)
	}
	// Count separators (each ─ is 3 bytes in UTF-8)
	count := strings.Count(result, "─")
	if count != 20 {
		t.Errorf("expected 20 separator chars, got %d", count)
	}
}

func TestFormatLabel(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatLabel("Tokens:", scheme)

	if !strings.Contains(result, "Tokens:") {
		t.Errorf("expected label text, got: %q", result)
	}
	if !strings.Contains(result, scheme.LabelDim) {
		t.Errorf("expected LabelDim color, got: %q", result)
	}
}

func TestFormatValue(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatValue("1234", scheme)

	if !strings.Contains(result, "1234") {
		t.Errorf("expected value text, got: %q", result)
	}
	if !strings.Contains(result, scheme.ValueBright) {
		t.Errorf("expected ValueBright color, got: %q", result)
	}
}

func TestFormatToolName(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatToolName("Bash", scheme)

	if !strings.Contains(result, "Bash") {
		t.Errorf("expected tool name, got: %q", result)
	}
	if !strings.Contains(result, scheme.ToolName) {
		t.Errorf("expected ToolName color, got: %q", result)
	}
}

func TestFormatAgentContext(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatAgentContext("main", "running", scheme)

	if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
		t.Errorf("expected brackets, got: %q", result)
	}
	if !strings.Contains(result, "main") {
		t.Errorf("expected agent type, got: %q", result)
	}
	if !strings.Contains(result, "running") {
		t.Errorf("expected status, got: %q", result)
	}
}

func TestFormatThinkingPrefix(t *testing.T) {
	scheme := DefaultScheme()

	result := FormatThinkingPrefix(scheme)

	if !strings.Contains(result, "[THINKING]") {
		t.Errorf("expected [THINKING], got: %q", result)
	}
	if !strings.Contains(result, scheme.ThinkingPrefix) {
		t.Errorf("expected ThinkingPrefix color, got: %q", result)
	}
}

func TestIndentLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		indent   string
		expected string
	}{
		{
			name:     "single line",
			input:    "hello",
			indent:   "  ",
			expected: "  hello",
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			indent:   "  ",
			expected: "  line1\n  line2\n  line3",
		},
		{
			name:     "empty lines preserved",
			input:    "line1\n\nline3",
			indent:   "  ",
			expected: "  line1\n\n  line3",
		},
		{
			name:     "no indent",
			input:    "hello",
			indent:   "",
			expected: "hello",
		},
		{
			name:     "tab indent",
			input:    "line1\nline2",
			indent:   "\t",
			expected: "\tline1\n\tline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IndentLines(tt.input, tt.indent)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDefaultFormatConfig(t *testing.T) {
	config := DefaultFormatConfig()

	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if config.Colors == nil {
		t.Error("expected Colors to be set")
	}
	if config.Indent == "" {
		t.Error("expected Indent to be set")
	}
	if !config.ShowLineNo {
		t.Error("expected ShowLineNo to be true by default")
	}
}
