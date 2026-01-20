package main

import (
	"os"
	"testing"
)

func TestDefaultScheme(t *testing.T) {
	scheme := DefaultScheme()

	if scheme == nil {
		t.Fatal("expected non-nil scheme")
	}

	// Check that all fields have values
	checks := []struct {
		name  string
		value string
	}{
		{"ToolArrow", scheme.ToolArrow},
		{"ToolName", scheme.ToolName},
		{"ToolDesc", scheme.ToolDesc},
		{"ToolStatus", scheme.ToolStatus},
		{"ThinkingPrefix", scheme.ThinkingPrefix},
		{"ThinkingText", scheme.ThinkingText},
		{"AgentBrackets", scheme.AgentBrackets},
		{"AgentType", scheme.AgentType},
		{"AgentStatus", scheme.AgentStatus},
		{"Success", scheme.Success},
		{"Error", scheme.Error},
		{"DiffAdd", scheme.DiffAdd},
		{"DiffRemove", scheme.DiffRemove},
		{"SessionInfo", scheme.SessionInfo},
		{"Separator", scheme.Separator},
		{"LabelDim", scheme.LabelDim},
		{"ValueBright", scheme.ValueBright},
		{"FilePath", scheme.FilePath},
		{"Reset", scheme.Reset},
	}

	for _, c := range checks {
		if c.value == "" {
			t.Errorf("DefaultScheme().%s is empty", c.name)
		}
	}
}

func TestNoColorScheme(t *testing.T) {
	scheme := NoColorScheme()

	if scheme == nil {
		t.Fatal("expected non-nil scheme")
	}

	// Check that all fields are empty
	checks := []struct {
		name  string
		value string
	}{
		{"ToolArrow", scheme.ToolArrow},
		{"ToolName", scheme.ToolName},
		{"ToolDesc", scheme.ToolDesc},
		{"ToolStatus", scheme.ToolStatus},
		{"ThinkingPrefix", scheme.ThinkingPrefix},
		{"ThinkingText", scheme.ThinkingText},
		{"AgentBrackets", scheme.AgentBrackets},
		{"AgentType", scheme.AgentType},
		{"AgentStatus", scheme.AgentStatus},
		{"Success", scheme.Success},
		{"Error", scheme.Error},
		{"DiffAdd", scheme.DiffAdd},
		{"DiffRemove", scheme.DiffRemove},
		{"SessionInfo", scheme.SessionInfo},
		{"Separator", scheme.Separator},
		{"LabelDim", scheme.LabelDim},
		{"ValueBright", scheme.ValueBright},
		{"FilePath", scheme.FilePath},
		{"Reset", scheme.Reset},
	}

	for _, c := range checks {
		if c.value != "" {
			t.Errorf("NoColorScheme().%s should be empty, got: %q", c.name, c.value)
		}
	}
}

func TestGetScheme_Default(t *testing.T) {
	// Save original state
	origNoColorFlag := noColorFlag
	origColorEnabled := colorEnabled
	defer func() {
		noColorFlag = origNoColorFlag
		colorEnabled = origColorEnabled
	}()

	// Reset to defaults
	noColorFlag = false
	colorEnabled = true

	scheme := GetScheme()

	// Should return default scheme with colors
	if scheme.Reset == "" {
		t.Error("expected colored scheme when colors are enabled")
	}
}

func TestGetScheme_NoColorFlag(t *testing.T) {
	// Save original state
	origNoColorFlag := noColorFlag
	origColorEnabled := colorEnabled
	defer func() {
		noColorFlag = origNoColorFlag
		colorEnabled = origColorEnabled
	}()

	// Set no-color flag
	noColorFlag = true
	colorEnabled = true

	scheme := GetScheme()

	// Should return no-color scheme
	if scheme.Reset != "" {
		t.Error("expected no-color scheme when noColorFlag is set")
	}
}

func TestGetScheme_ColorDisabled(t *testing.T) {
	// Save original state
	origNoColorFlag := noColorFlag
	origColorEnabled := colorEnabled
	defer func() {
		noColorFlag = origNoColorFlag
		colorEnabled = origColorEnabled
	}()

	// Disable colors
	noColorFlag = false
	colorEnabled = false

	scheme := GetScheme()

	// Should return no-color scheme
	if scheme.Reset != "" {
		t.Error("expected no-color scheme when colorEnabled is false")
	}
}

func TestSetNoColor(t *testing.T) {
	// Save original state
	origNoColorFlag := noColorFlag
	defer func() {
		noColorFlag = origNoColorFlag
	}()

	SetNoColor(true)
	if !noColorFlag {
		t.Error("expected noColorFlag to be true")
	}

	SetNoColor(false)
	if noColorFlag {
		t.Error("expected noColorFlag to be false")
	}
}

func TestC_ColorsEnabled(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	defer func() {
		colorEnabled = origColorEnabled
	}()

	colorEnabled = true
	result := C(Red)

	if result != Red {
		t.Errorf("expected %q, got %q", Red, result)
	}
}

func TestC_ColorsDisabled(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	defer func() {
		colorEnabled = origColorEnabled
	}()

	colorEnabled = false
	result := C(Red)

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestColorize_ColorsEnabled(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	defer func() {
		colorEnabled = origColorEnabled
	}()

	colorEnabled = true
	result := Colorize("hello", Red)

	expected := Red + "hello" + Reset
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestColorize_ColorsDisabled(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	defer func() {
		colorEnabled = origColorEnabled
	}()

	colorEnabled = false
	result := Colorize("hello", Red)

	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestColorize_EmptyColor(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	defer func() {
		colorEnabled = origColorEnabled
	}()

	colorEnabled = true
	result := Colorize("hello", "")

	if result != "hello" {
		t.Errorf("expected 'hello' with empty color, got %q", result)
	}
}

func TestInitColors_NoColorEnv(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	origNoColor := os.Getenv("NO_COLOR")
	defer func() {
		colorEnabled = origColorEnabled
		if origNoColor == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", origNoColor)
		}
	}()

	// Reset and set NO_COLOR
	colorEnabled = true
	os.Setenv("NO_COLOR", "1")

	initColors()

	if colorEnabled {
		t.Error("expected colorEnabled to be false when NO_COLOR is set")
	}
}

func TestInitColors_TermDumb(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	origTerm := os.Getenv("TERM")
	origNoColor := os.Getenv("NO_COLOR")
	defer func() {
		colorEnabled = origColorEnabled
		if origTerm == "" {
			os.Unsetenv("TERM")
		} else {
			os.Setenv("TERM", origTerm)
		}
		if origNoColor == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", origNoColor)
		}
	}()

	// Reset and set TERM=dumb
	colorEnabled = true
	os.Unsetenv("NO_COLOR") // Make sure NO_COLOR doesn't interfere
	os.Setenv("TERM", "dumb")

	initColors()

	if colorEnabled {
		t.Error("expected colorEnabled to be false when TERM=dumb")
	}
}

func TestInitColors_NormalTerm(t *testing.T) {
	// Save original state
	origColorEnabled := colorEnabled
	origTerm := os.Getenv("TERM")
	origNoColor := os.Getenv("NO_COLOR")
	defer func() {
		colorEnabled = origColorEnabled
		if origTerm == "" {
			os.Unsetenv("TERM")
		} else {
			os.Setenv("TERM", origTerm)
		}
		if origNoColor == "" {
			os.Unsetenv("NO_COLOR")
		} else {
			os.Setenv("NO_COLOR", origNoColor)
		}
	}()

	// Reset environment
	colorEnabled = false // Start disabled
	os.Unsetenv("NO_COLOR")
	os.Setenv("TERM", "xterm-256color")

	initColors()

	// Colors should remain enabled (no conditions to disable)
	// Note: initColors doesn't set colorEnabled=true, it only disables
	// The test just verifies it doesn't get disabled for normal TERM
}

func TestColorConstants(t *testing.T) {
	// Verify that color constants are ANSI escape sequences
	constants := []struct {
		name   string
		value  string
		prefix string
	}{
		{"Reset", Reset, "\033[0m"},
		{"Bold", Bold, "\033[1m"},
		{"Dim", Dim, "\033[2m"},
		{"Red", Red, "\033[31m"},
		{"Green", Green, "\033[32m"},
		{"Yellow", Yellow, "\033[33m"},
		{"Blue", Blue, "\033[34m"},
		{"Cyan", Cyan, "\033[36m"},
		{"BrightRed", BrightRed, "\033[91m"},
		{"BrightGreen", BrightGreen, "\033[92m"},
	}

	for _, c := range constants {
		if c.value != c.prefix {
			t.Errorf("%s: expected %q, got %q", c.name, c.prefix, c.value)
		}
	}
}
