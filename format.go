package main

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// syntaxPattern holds a compiled regex and its associated color
type syntaxPattern struct {
	regex *regexp.Regexp
	color string
}

// syntaxPatterns holds compiled regex patterns for syntax highlighting
var syntaxPatterns []syntaxPattern

// syntaxPatternsOnce ensures patterns are compiled only once
var syntaxPatternsOnce sync.Once

// initSyntaxPatterns initializes the syntax highlighting patterns
func initSyntaxPatterns() {
	syntaxPatterns = []syntaxPattern{
		// Strings (double-quoted)
		{regexp.MustCompile(`"[^"\\]*(?:\\.[^"\\]*)*"`), Yellow},
		// Strings (single-quoted)
		{regexp.MustCompile(`'[^'\\]*(?:\\.[^'\\]*)*'`), Yellow},
		// Environment variables ($VAR, ${VAR})
		{regexp.MustCompile(`\$\{?[A-Z_][A-Z0-9_]*\}?`), Cyan},
		// Flags (--flag, -f)
		{regexp.MustCompile(`\s(--?[a-zA-Z][-a-zA-Z0-9]*)`), BrightYellow},
		// Numbers
		{regexp.MustCompile(`\b\d+\.?\d*\b`), Magenta},
	}
}

// FormatConfig holds formatting options
type FormatConfig struct {
	Colors     *ColorScheme
	ShowLineNo bool
	Indent     string
}

// DefaultFormatConfig returns the default formatting config
func DefaultFormatConfig() *FormatConfig {
	return &FormatConfig{
		Colors:     GetScheme(),
		ShowLineNo: true,
		Indent:     "  ",
	}
}

// FormatCodeBlock formats a code block with optional line numbers and syntax highlighting
func FormatCodeBlock(code string, config *FormatConfig) string {
	if config == nil {
		config = DefaultFormatConfig()
	}

	c := config.Colors
	lines := strings.Split(code, "\n")
	var result strings.Builder

	// Calculate line number width for alignment
	lineNoWidth := len(fmt.Sprintf("%d", len(lines)))

	for i, line := range lines {
		if config.ShowLineNo {
			lineNo := i + 1
			// Dim line numbers
			result.WriteString(fmt.Sprintf("%s%*d%s %s", c.LabelDim, lineNoWidth, lineNo, c.Reset, config.Indent))
		} else {
			result.WriteString(config.Indent)
		}

		// Apply basic syntax highlighting
		highlightedLine := highlightSyntax(line, c)
		result.WriteString(highlightedLine)
		result.WriteString("\n")
	}

	return result.String()
}

// highlightSyntax applies basic syntax highlighting to a line of code
func highlightSyntax(line string, c *ColorScheme) string {
	// Skip if no colors
	if c.Reset == "" {
		return line
	}

	// Initialize patterns once (thread-safe)
	syntaxPatternsOnce.Do(initSyntaxPatterns)

	result := line
	for _, p := range syntaxPatterns {
		result = p.regex.ReplaceAllStringFunc(result, func(match string) string {
			return p.color + match + c.Reset
		})
	}

	return result
}

// FormatFilePath formats a file path with appropriate highlighting
func FormatFilePath(path string, c *ColorScheme) string {
	return c.FilePath + path + c.Reset
}

// FormatCommand formats a command with appropriate highlighting
func FormatCommand(cmd string, c *ColorScheme) string {
	return Bold + cmd + c.Reset
}

// FormatBullet returns a formatted bullet point
func FormatBullet(c *ColorScheme) string {
	return c.ToolArrow + "●" + c.Reset
}

// FormatArrow returns a formatted arrow for tool calls
func FormatArrow(c *ColorScheme) string {
	return c.ToolArrow + "→" + c.Reset
}

// FormatSuccess returns a formatted success checkmark
func FormatSuccess(c *ColorScheme) string {
	return c.Success + "✓" + c.Reset
}

// FormatError returns a formatted error cross
func FormatError(c *ColorScheme) string {
	return c.Error + "✗" + c.Reset
}

// FormatDiffLine formats a diff line with + or - prefix
func FormatDiffLine(line string, isAddition bool, c *ColorScheme) string {
	if isAddition {
		return c.DiffAdd + "+ " + line + c.Reset
	}
	return c.DiffRemove + "- " + line + c.Reset
}

// FormatSectionSeparator returns a formatted section separator
func FormatSectionSeparator(width int, c *ColorScheme) string {
	return c.Separator + strings.Repeat("─", width) + c.Reset
}

// FormatLabel formats a label (like "Tokens:", "Cost:")
func FormatLabel(label string, c *ColorScheme) string {
	return c.LabelDim + label + c.Reset
}

// FormatValue formats a value (numbers, important data)
func FormatValue(value string, c *ColorScheme) string {
	return c.ValueBright + value + c.Reset
}

// FormatToolName formats a tool name
func FormatToolName(name string, c *ColorScheme) string {
	return c.ToolName + name + c.Reset
}

// FormatAgentContext formats an agent context badge
func FormatAgentContext(agentType string, status string, c *ColorScheme) string {
	return fmt.Sprintf("%s[%s%s%s: %s%s%s]%s",
		c.AgentBrackets, c.AgentType, agentType, c.AgentBrackets,
		c.AgentStatus, status, c.AgentBrackets, c.Reset)
}

// FormatThinkingPrefix formats the thinking block prefix
func FormatThinkingPrefix(c *ColorScheme) string {
	return c.ThinkingPrefix + "[THINKING]" + c.Reset
}

// IndentLines indents all lines in a string
func IndentLines(s string, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}
