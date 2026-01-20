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

func TestFormatMCPToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "non-MCP tool name unchanged",
			input:    "Bash",
			expected: "Bash",
		},
		{
			name:     "non-MCP tool with underscores unchanged",
			input:    "some_random_tool",
			expected: "some_random_tool",
		},
		{
			name:     "standard MCP format",
			input:    "mcp__filesystem__read_file",
			expected: "filesystem:read_file",
		},
		{
			name:     "malformed MCP name - no separator",
			input:    "mcp__filesystem",
			expected: "mcp__filesystem",
		},
		{
			name:     "plugin with underscores converts to colons",
			input:    "mcp__plugin_sub__tool",
			expected: "plugin:sub:tool",
		},
		{
			name:     "deeply nested plugin",
			input:    "mcp__plugin_a_b__tool_name",
			expected: "plugin:a:b:tool_name",
		},
		{
			name:     "MCP prefix only returns original",
			input:    "mcp__",
			expected: "mcp__",
		},
		{
			name:     "plugin with no underscores",
			input:    "mcp__fs__read",
			expected: "fs:read",
		},
		{
			name:     "tool name with underscores preserved",
			input:    "mcp__db__query_table",
			expected: "db:query_table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMCPToolName(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestHighlightSyntax_AdditionalCases(t *testing.T) {
	scheme := DefaultScheme()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "highlights flags with double dash",
			input:    `cmd --verbose --output=file`,
			contains: BrightYellow,
		},
		{
			name:     "highlights single dash flag",
			input:    `cmd -v -f`,
			contains: BrightYellow,
		},
		{
			name:     "float numbers highlighted",
			input:    `value 3.14 end`,
			contains: Magenta,
		},
		{
			name:     "escaped quotes in string",
			input:    `echo "hello \"world\""`,
			contains: Yellow,
		},
		{
			name:     "braced environment variable",
			input:    `echo ${HOME}`,
			contains: Cyan,
		},
		{
			name:     "multiple patterns in same line",
			input:    `echo "hello" --flag 42 $HOME`,
			contains: Yellow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highlightSyntax(tt.input, scheme)
			// Check that reset is present (indicating coloring was applied)
			if !strings.Contains(result, Reset) {
				t.Errorf("expected reset code in result: %q", result)
			}
			// For the "contains" color check, we verify the color code appears
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expected color code %q in result: %q", tt.contains, result)
			}
		})
	}
}

func TestHighlightSyntax_EmptyString(t *testing.T) {
	scheme := DefaultScheme()

	result := highlightSyntax("", scheme)

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatCodeBlock_CustomIndent(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		indent      string
		showLineNo  bool
		expectIndent string
	}{
		{
			name:        "custom indent with spaces",
			code:        "line1",
			indent:      "    ",
			showLineNo:  false,
			expectIndent: "    ",
		},
		{
			name:        "custom indent with tab",
			code:        "line1",
			indent:      "\t",
			showLineNo:  false,
			expectIndent: "\t",
		},
		{
			name:        "zero-width indent",
			code:        "line1",
			indent:      "",
			showLineNo:  false,
			expectIndent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &FormatConfig{
				Colors:     NoColorScheme(),
				ShowLineNo: tt.showLineNo,
				Indent:     tt.indent,
			}

			result := FormatCodeBlock(tt.code, config)
			lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")

			if len(lines) != 1 {
				t.Fatalf("expected 1 line, got %d", len(lines))
			}

			if !strings.HasPrefix(lines[0], tt.expectIndent+"line1") {
				t.Errorf("expected line to start with %q, got: %q", tt.expectIndent, lines[0])
			}
		})
	}
}

func TestFormatCodeBlock_LineNumberWidth(t *testing.T) {
	tests := []struct {
		name          string
		lineCount     int
		minWidth      int
	}{
		{
			name:      "single line - width 1",
			lineCount: 1,
			minWidth:  1,
		},
		{
			name:      "9 lines - width 1",
			lineCount: 9,
			minWidth:  1,
		},
		{
			name:      "10 lines - width 2",
			lineCount: 10,
			minWidth:  2,
		},
		{
			name:      "99 lines - width 2",
			lineCount: 99,
			minWidth:  2,
		},
		{
			name:      "100 lines - width 3",
			lineCount: 100,
			minWidth:  3,
		},
		{
			name:      "999 lines - width 3",
			lineCount: 999,
			minWidth:  3,
		},
		{
			name:      "1000 lines - width 4",
			lineCount: 1000,
			minWidth:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build code with specified number of lines
			var code strings.Builder
			for i := 0; i < tt.lineCount; i++ {
				code.WriteString("line\n")
			}

			config := &FormatConfig{
				Colors:     NoColorScheme(),
				ShowLineNo: true,
				Indent:     "  ",
			}

			result := FormatCodeBlock(strings.TrimSuffix(code.String(), "\n"), config)
			lines := strings.Split(strings.TrimSuffix(result, "\n"), "\n")

			if len(lines) != tt.lineCount {
				t.Fatalf("expected %d lines, got %d", tt.lineCount, len(lines))
			}

			// Check first line has correct line number width
			firstLine := lines[0]
			// Format: "<lineNo><space><indent>..."
			// Find where the space after line number is
			spaceIdx := strings.Index(firstLine, " ")
			if spaceIdx < tt.minWidth {
				t.Errorf("expected line number width at least %d, got %d", tt.minWidth, spaceIdx)
			}
		})
	}
}

// FuzzFormatMCPToolName tests FormatMCPToolName with random string input
func FuzzFormatMCPToolName(f *testing.F) {
	// Seed corpus with valid cases
	f.Add([]byte("Bash"))
	f.Add([]byte("mcp__filesystem__read_file"))
	f.Add([]byte("mcp__plugin_sub__tool"))
	f.Add([]byte("mcp__"))
	f.Add([]byte("some_random_tool"))
	f.Add([]byte(""))
	f.Add([]byte("mcp__a__b"))
	f.Add([]byte("mcp__a_b_c__tool_name"))

	// Add edge cases
	f.Add([]byte("mcp"))
	f.Add([]byte("__"))
	f.Add([]byte("mcp____"))
	f.Add([]byte("mcp___tool"))

	// Add special characters
	f.Add([]byte("mcp__plugin__tool\x00"))
	f.Add([]byte("mcp__plugin\x80__tool"))

	// Add long strings
	longInput := make([]byte, 0, 10000)
	longInput = append(longInput, []byte("mcp__")...)
	for i := 0; i < 5000; i++ {
		longInput = append(longInput, 'a')
	}
	longInput = append(longInput, []byte("__")...)
	for i := 0; i < 5000; i++ {
		longInput = append(longInput, 'b')
	}
	f.Add(longInput)

	f.Fuzz(func(t *testing.T, data []byte) {
		input := string(data)

		// FormatMCPToolName should never panic, even with malformed input
		result := FormatMCPToolName(input)

		// Result should be a valid string (not nil)
		// The function either transforms valid MCP names or returns the original
		_ = result
	})
}

// FuzzHighlightSyntax tests highlightSyntax with random string input
func FuzzHighlightSyntax(f *testing.F) {
	scheme := DefaultScheme()

	// Seed corpus with valid cases
	f.Add([]byte(`echo "hello"`))
	f.Add([]byte(`echo 'world'`))
	f.Add([]byte(`count 42 items`))
	f.Add([]byte(`echo $HOME`))
	f.Add([]byte(`cmd --verbose --output=file`))
	f.Add([]byte(`cmd -v -f`))
	f.Add([]byte(`value 3.14 end`))
	f.Add([]byte(""))
	f.Add([]byte("simple text"))
	f.Add([]byte("echo \"hello\" --flag 42 $HOME"))

	// Add edge cases - special regex characters
	f.Add([]byte(`echo "test" && ls | grep .go`))
	f.Add([]byte(`echo "(nested)[braces]{symbols}"`))
	f.Add([]byte(`echo "$$VAR"`))

	// Add null bytes and invalid UTF-8
	f.Add([]byte("test\x00"))
	f.Add([]byte("test\xff"))

	// Add strings with many quotes to potentially break regex
	manyQuotes := make([]byte, 0, 10000)
	for i := 0; i < 5000; i++ {
		manyQuotes = append(manyQuotes, []byte(`"test"`)...)
	}
	f.Add(manyQuotes)

	// Add long line
	longLine := make([]byte, 0, 1024*1024)
	for i := 0; i < 1024*1024; i++ {
		longLine = append(longLine, 'a')
	}
	f.Add(longLine)

	f.Fuzz(func(t *testing.T, data []byte) {
		input := string(data)

		// highlightSyntax should never panic, even with malformed input
		// It handles regex operations internally
		result := highlightSyntax(input, scheme)

		// Result should be a valid string
		_ = result
	})
}

// FuzzIndentLines tests IndentLines with random string input
func FuzzIndentLines(f *testing.F) {
	// Seed corpus with valid cases
	f.Add([]byte("hello"))
	f.Add([]byte("line1\nline2\nline3"))
	f.Add([]byte(""))
	f.Add([]byte("\n\n"))
	f.Add([]byte("line1\n\nline3"))
	f.Add([]byte("single line with no newlines"))

	// Add edge cases with many newlines
	manyNewlines := make([]byte, 0, 10000)
	for i := 0; i < 5000; i++ {
		manyNewlines = append(manyNewlines, '\n')
	}
	f.Add(manyNewlines)

	// Add very long line
	longLine := make([]byte, 0, 100*1024)
	for i := 0; i < 100*1024; i++ {
		longLine = append(longLine, 'a')
	}
	f.Add(longLine)

	f.Fuzz(func(t *testing.T, data []byte) {
		input := string(data)

		// Test with various indents
		indents := []string{"", "  ", "\t", "    ", "          "}

		for _, indent := range indents {
			result := IndentLines(input, indent)

			// Result should be a valid string
			// Verify that empty input returns empty (or only indent)
			if input == "" {
				// Empty input should give empty output
				if result != "" {
					t.Errorf("empty input should give empty output, got %q", result)
				}
			}
		}
	})
}
