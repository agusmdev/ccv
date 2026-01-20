package main

import (
	"os"

	"golang.org/x/term"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Italic  = "\033[3m"

	// Colors
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright colors
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

// ColorScheme defines colors for different output elements
type ColorScheme struct {
	// Tool calls
	ToolArrow    string // → symbol
	ToolName     string // Tool name (Bash, Read, etc.)
	ToolDesc     string // Tool description/parameters
	ToolStatus   string // Status indicators in brackets

	// Thinking
	ThinkingPrefix string // [THINKING] prefix
	ThinkingText   string // Thinking content

	// Agent context
	AgentBrackets string // [ ] brackets
	AgentType     string // Agent type name
	AgentStatus   string // running, completed, etc.

	// Status indicators
	Success string // ✓ checkmark
	Error   string // ✗ cross

	// Diff output
	DiffAdd    string // + lines
	DiffRemove string // - lines

	// Session/Summary
	SessionInfo string // Session started, model info
	Separator   string // ─── lines
	LabelDim    string // Labels like "Tokens:", "Cost:"
	ValueBright string // Values/numbers

	// File paths
	FilePath string // File paths in tool calls

	// Reset
	Reset string
}

// colorEnabled tracks whether colors should be used
var colorEnabled bool

// DefaultScheme returns the default color scheme
func DefaultScheme() *ColorScheme {
	return &ColorScheme{
		// Tool calls - cyan arrow, bold blue name
		ToolArrow:  Cyan,
		ToolName:   Bold + Blue,
		ToolDesc:   Reset,
		ToolStatus: Dim,

		// Thinking - magenta
		ThinkingPrefix: Bold + Magenta,
		ThinkingText:   Dim + Magenta,

		// Agent context - yellow
		AgentBrackets: Yellow,
		AgentType:     Bold + Yellow,
		AgentStatus:   Dim + Yellow,

		// Status
		Success: Green,
		Error:   Red,

		// Diff
		DiffAdd:    Green,
		DiffRemove: Red,

		// Session/Summary
		SessionInfo: Dim,
		Separator:   Dim,
		LabelDim:    Dim,
		ValueBright: Bold,

		// File paths
		FilePath: BrightBlue,

		Reset: Reset,
	}
}

// initColors determines if colors should be enabled based on terminal capability
func initColors() {
	colorEnabled = term.IsTerminal(int(os.Stdout.Fd()))
}

// NoColorScheme returns a scheme with no colors (empty strings)
func NoColorScheme() *ColorScheme {
	return &ColorScheme{}
}

// GetScheme returns the appropriate color scheme based on terminal capability
func GetScheme() *ColorScheme {
	if colorEnabled {
		return DefaultScheme()
	}
	return NoColorScheme()
}

// C is a helper that returns the color code if colors are enabled, empty string otherwise
func C(color string) string {
	if colorEnabled {
		return color
	}
	return ""
}

// Colorize wraps text with a color code and reset if colors are enabled
func Colorize(text, color string) string {
	if colorEnabled && color != "" {
		return color + text + Reset
	}
	return text
}
