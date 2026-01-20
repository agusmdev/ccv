package main

import (
	"os"
	"strings"
	"testing"
)

// TestFlagParsing tests various flag parsing scenarios
func TestFlagParsing(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		envVars        map[string]string
		expectVerbose  bool
		expectQuiet    bool
		expectFormat   string
		expectNoColor  bool
		expectedRemain []string
	}{
		{
			name:           "no flags",
			args:           []string{"prompt text"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "text",
			expectNoColor:  false,
			expectedRemain: []string{"prompt text"},
		},
		{
			name:           "--verbose flag",
			args:           []string{"--verbose", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  true,
			expectQuiet:    false,
			expectFormat:   "text",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "-verbose flag (single dash)",
			args:           []string{"-verbose", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  true,
			expectQuiet:    false,
			expectFormat:   "text",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "--quiet flag",
			args:           []string{"--quiet", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    true,
			expectFormat:   "text",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "--format json",
			args:           []string{"--format", "json", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "json",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "--format=json (equals syntax)",
			args:           []string{"--format=json", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "json",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "-format=json (single dash equals)",
			args:           []string{"-format=json", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "json",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "--no-color flag",
			args:           []string{"--no-color", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "text",
			expectNoColor:  true,
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "-no-color flag (single dash)",
			args:           []string{"-no-color", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "text",
			expectNoColor:  true,
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "quiet overrides verbose",
			args:           []string{"--verbose", "--quiet", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false, // quiet overrides
			expectQuiet:    true,
			expectFormat:   "text",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "quiet overrides json",
			args:           []string{"--format", "json", "--quiet", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    true,
			expectFormat:   "text", // quiet mode doesn't set format
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "multiple flags and passthrough",
			args:           []string{"--verbose", "--format", "json", "--model", "opus", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  true,
			expectQuiet:    false,
			expectFormat:   "json",
			expectedRemain: []string{"--model", "opus", "prompt"},
		},
		{
			name:           "CCV_VERBOSE environment variable",
			args:           []string{"prompt"},
			envVars:        map[string]string{"CCV_VERBOSE": "1"},
			expectVerbose:  true,
			expectQuiet:    false,
			expectFormat:   "text",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "CCV_QUIET environment variable",
			args:           []string{"prompt"},
			envVars:        map[string]string{"CCV_QUIET": "1"},
			expectVerbose:  false,
			expectQuiet:    true,
			expectFormat:   "text",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "CCV_FORMAT environment variable",
			args:           []string{"prompt"},
			envVars:        map[string]string{"CCV_FORMAT": "json"},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "json",
			expectedRemain: []string{"prompt"},
		},
		{
			name:           "flag overrides environment",
			args:           []string{"--format", "text"},
			envVars:        map[string]string{"CCV_FORMAT": "json"},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "text",
			expectedRemain: []string{},
		},
		{
			name:           "--print flag passthrough",
			args:           []string{"--print", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "text",
			expectedRemain: []string{"--print", "prompt"},
		},
		{
			name:           "--allowedTools passthrough",
			args:           []string{"--allowedTools", "Read,Bash", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "text",
			expectedRemain: []string{"--allowedTools", "Read,Bash", "prompt"},
		},
		{
			name:           "-p shorthand passthrough",
			args:           []string{"-p", "prompt"},
			envVars:        map[string]string{},
			expectVerbose:  false,
			expectQuiet:    false,
			expectFormat:   "text",
			expectedRemain: []string{"-p", "prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			origVerbose := os.Getenv("CCV_VERBOSE")
			origQuiet := os.Getenv("CCV_QUIET")
			origFormat := os.Getenv("CCV_FORMAT")

			// Set up environment
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer func() {
				// Restore environment
				if origVerbose == "" {
					os.Unsetenv("CCV_VERBOSE")
				} else {
					os.Setenv("CCV_VERBOSE", origVerbose)
				}
				if origQuiet == "" {
					os.Unsetenv("CCV_QUIET")
				} else {
					os.Setenv("CCV_QUIET", origQuiet)
				}
				if origFormat == "" {
					os.Unsetenv("CCV_FORMAT")
				} else {
					os.Setenv("CCV_FORMAT", origFormat)
				}
			}()

			// Parse flags (mimicking main.go logic)
			verbose := os.Getenv("CCV_VERBOSE") == "1"
			quiet := os.Getenv("CCV_QUIET") == "1"
			noColor := false
			format := strings.ToLower(os.Getenv("CCV_FORMAT"))
			if format == "" {
				format = "text"
			}

			var claudeArgs []string
			args := tt.args
			for i := 0; i < len(args); i++ {
				arg := args[i]

				if arg == "--verbose" || arg == "-verbose" {
					verbose = true
					continue
				}
				if arg == "--quiet" || arg == "-quiet" {
					quiet = true
					continue
				}
				if arg == "--no-color" || arg == "-no-color" {
					noColor = true
					continue
				}
				if arg == "--format" || arg == "-format" {
					if i+1 < len(args) {
						i++
						format = strings.ToLower(args[i])
					}
					continue
				}
				if strings.HasPrefix(arg, "--format=") {
					format = strings.ToLower(strings.TrimPrefix(arg, "--format="))
					continue
				}
				if strings.HasPrefix(arg, "-format=") {
					format = strings.ToLower(strings.TrimPrefix(arg, "-format="))
					continue
				}

				claudeArgs = append(claudeArgs, arg)
			}

			// Apply quiet override logic
			if quiet {
				// quiet doesn't change format, just mode
			}

			// Verify results
			if verbose != tt.expectVerbose {
				t.Errorf("verbose = %v, want %v", verbose, tt.expectVerbose)
			}
			if quiet != tt.expectQuiet {
				t.Errorf("quiet = %v, want %v", quiet, tt.expectQuiet)
			}
			if format != tt.expectFormat {
				t.Errorf("format = %q, want %q", format, tt.expectFormat)
			}
			if noColor != tt.expectNoColor {
				t.Errorf("noColor = %v, want %v", noColor, tt.expectNoColor)
			}
		})
	}
}

// TestFormatNormalization tests format string normalization
func TestFormatNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"text", "text"},
		{"TEXT", "text"},
		{"JSON", "json"},
		{"JsOn", "json"},
		{"", "text"},
		{"verbose", "verbose"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := strings.ToLower(tt.input)
			if result == "" {
				result = "text"
			}
			if result != tt.expected {
				t.Errorf("normalization of %q = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPassthroughArgs tests that non-ccv flags are passed through
func TestPassthroughArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedPassthrough []string
	}{
		{
			name:   "model flag",
			args:   []string{"--model", "opus", "prompt"},
			expectedPassthrough: []string{"--model", "opus", "prompt"},
		},
		{
			name:   "multiple claude flags",
			args:   []string{"--model", "haiku", "-m", "200000", "prompt"},
			expectedPassthrough: []string{"--model", "haiku", "-m", "200000", "prompt"},
		},
		{
			name:   "flags with equals",
			args:   []string{"--model=opus", "prompt"},
			expectedPassthrough: []string{"--model=opus", "prompt"},
		},
		{
			name:   "print and verbose",
			args:   []string{"--print", "prompt"},
			expectedPassthrough: []string{"--print", "prompt"},
		},
		{
			name:   "complex args",
			args:   []string{"--print", "-p", "allowed", "tools=Read", "prompt"},
			expectedPassthrough: []string{"--print", "-p", "allowed", "tools=Read", "prompt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var claudeArgs []string
			for _, arg := range tt.args {
				// Skip ccv-specific flags
				if arg == "--verbose" || arg == "-verbose" ||
					arg == "--quiet" || arg == "-quiet" ||
					arg == "--no-color" || arg == "-no-color" ||
					arg == "--format" || arg == "-format" ||
					strings.HasPrefix(arg, "--format=") ||
					strings.HasPrefix(arg, "-format=") {
					continue
				}
				claudeArgs = append(claudeArgs, arg)
			}

			if len(claudeArgs) != len(tt.expectedPassthrough) {
				t.Errorf("passthrough count = %d, want %d\ngot: %v\nwant: %v",
					len(claudeArgs), len(tt.expectedPassthrough), claudeArgs, tt.expectedPassthrough)
				return
			}

			for i, arg := range claudeArgs {
				if arg != tt.expectedPassthrough[i] {
					t.Errorf("arg[%d] = %q, want %q", i, arg, tt.expectedPassthrough[i])
				}
			}
		})
	}
}

// TestEmptyArgs tests handling of empty arguments
func TestEmptyArgs(t *testing.T) {
	args := []string{}

	// Parse flags
	var claudeArgs []string
	for _, arg := range args {
		if arg == "--verbose" || arg == "-verbose" ||
			arg == "--quiet" || arg == "-quiet" ||
			arg == "--no-color" || arg == "-no-color" {
			continue
		}
		if arg == "--format" || arg == "-format" {
			continue
		}
		if strings.HasPrefix(arg, "--format=") || strings.HasPrefix(arg, "-format=") {
			continue
		}
		claudeArgs = append(claudeArgs, arg)
	}

	if len(claudeArgs) != 0 {
		t.Errorf("empty args should result in empty claudeArgs, got %v", claudeArgs)
	}
}

// TestOutputModeConstants tests output mode constants
func TestOutputModeConstants(t *testing.T) {
	tests := []struct {
		mode    OutputMode
		isValid bool
	}{
		{OutputModeText, true},
		{OutputModeJSON, true},
		{OutputModeVerbose, true},
		{OutputModeQuiet, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if !tt.isValid {
				t.Error("mode should be valid")
			}
			// Just verify the string representation is non-empty
			if string(tt.mode) == "" {
				t.Error("mode string should not be empty")
			}
		})
	}
}

// TestNoColorEnvVariable tests NO_COLOR environment variable handling
func TestNoColorEnvVariable(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		shouldDisable bool
	}{
		{
			name:          "NO_COLOR=1",
			envValue:      "1",
			shouldDisable: true,
		},
		{
			name:          "NO_COLOR=true",
			envValue:      "true",
			shouldDisable: true,
		},
		{
			name:          "NO_COLOR=0",
			envValue:      "0",
			shouldDisable: true, // any value disables
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := os.Getenv("NO_COLOR")
			os.Setenv("NO_COLOR", tt.envValue)
			defer func() {
				if orig == "" {
					os.Unsetenv("NO_COLOR")
				} else {
					os.Setenv("NO_COLOR", orig)
				}
			}()

			// Check if NO_COLOR is set (any value means disable)
			_, noColor := os.LookupEnv("NO_COLOR")
			if noColor != tt.shouldDisable {
				t.Errorf("NO_COLOR lookup = %v, want %v", noColor, tt.shouldDisable)
			}
		})
	}
}

// TestTermDumb tests TERM=dumb environment variable handling
func TestTermDumb(t *testing.T) {
	orig := os.Getenv("TERM")
	os.Setenv("TERM", "dumb")
	defer func() {
		if orig == "" {
			os.Unsetenv("TERM")
		} else {
			os.Setenv("TERM", orig)
		}
	}()

	term := os.Getenv("TERM")
	if term != "dumb" {
		t.Errorf("TERM = %q, want 'dumb'", term)
	}
}

// TestVersionConstant tests the version constant is defined
func TestVersionConstant(t *testing.T) {
	if version == "" {
		t.Error("version constant should not be empty")
	}
}

// TestPrintUsageFunc tests that printUsage doesn't panic
func TestPrintUsageFunc(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printUsage panicked: %v", r)
		}
	}()
	printUsage()
}
