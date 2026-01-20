package main

import (
	"testing"
)

func TestHasFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		expected bool
	}{
		{
			name:     "flag present without value",
			args:     []string{"--print", "--verbose"},
			flag:     "--print",
			expected: true,
		},
		{
			name:     "flag present with equals value",
			args:     []string{"--output-format=stream-json"},
			flag:     "--output-format",
			expected: true,
		},
		{
			name:     "flag not present",
			args:     []string{"--print", "--verbose"},
			flag:     "--quiet",
			expected: false,
		},
		{
			name:     "empty args",
			args:     []string{},
			flag:     "--print",
			expected: false,
		},
		{
			name:     "similar flag prefix not matched",
			args:     []string{"--verbose-mode"},
			flag:     "--verbose",
			expected: false,
		},
		{
			name:     "exact flag with equals sign",
			args:     []string{"--verbose=true"},
			flag:     "--verbose",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasFlag(tt.args, tt.flag)
			if result != tt.expected {
				t.Errorf("hasFlag(%v, %q) = %v, want %v", tt.args, tt.flag, result, tt.expected)
			}
		})
	}
}

func TestHasFlagValue(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		value    string
		expected bool
	}{
		{
			name:     "flag with value as separate args",
			args:     []string{"--output-format", "stream-json"},
			flag:     "--output-format",
			value:    "stream-json",
			expected: true,
		},
		{
			name:     "flag with value using equals",
			args:     []string{"--output-format=stream-json"},
			flag:     "--output-format",
			value:    "stream-json",
			expected: true,
		},
		{
			name:     "flag present but wrong value",
			args:     []string{"--output-format", "json"},
			flag:     "--output-format",
			value:    "stream-json",
			expected: false,
		},
		{
			name:     "flag not present",
			args:     []string{"--print"},
			flag:     "--output-format",
			value:    "stream-json",
			expected: false,
		},
		{
			name:     "flag at end without value",
			args:     []string{"--print", "--output-format"},
			flag:     "--output-format",
			value:    "stream-json",
			expected: false,
		},
		{
			name:     "empty args",
			args:     []string{},
			flag:     "--output-format",
			value:    "stream-json",
			expected: false,
		},
		{
			name:     "mixed flags",
			args:     []string{"--print", "--output-format", "stream-json", "--verbose"},
			flag:     "--output-format",
			value:    "stream-json",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasFlagValue(tt.args, tt.flag, tt.value)
			if result != tt.expected {
				t.Errorf("hasFlagValue(%v, %q, %q) = %v, want %v", tt.args, tt.flag, tt.value, result, tt.expected)
			}
		})
	}
}

func TestClaudeRunnerChannels(t *testing.T) {
	// Test that the channels are properly typed
	// We can't actually run the full runner without claude CLI being available,
	// but we can test the channel interface
	t.Run("Messages channel type", func(t *testing.T) {
		// Create a typed channel similar to what ClaudeRunner uses
		messages := make(chan interface{}, 100)

		// Send a test message
		msg := &SystemInit{Type: "system", SessionID: "test"}
		messages <- msg
		close(messages)

		// Receive and verify
		received := <-messages
		if _, ok := received.(*SystemInit); !ok {
			t.Errorf("expected *SystemInit, got %T", received)
		}
	})

	t.Run("Errors channel type", func(t *testing.T) {
		errors := make(chan error, 10)

		// Test sending an error
		testErr := &testError{message: "test error"}
		errors <- testErr
		close(errors)

		received := <-errors
		if received.Error() != "test error" {
			t.Errorf("expected 'test error', got %q", received.Error())
		}
	})
}

// testError is a simple error implementation for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
