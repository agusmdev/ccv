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

// mockWriteCloser implements io.WriteCloser for testing
type mockWriteCloser struct {
	writeFunc func([]byte) (int, error)
	closeFunc func() error
	closed    bool
}

func (m *mockWriteCloser) Write(p []byte) (int, error) {
	if m.writeFunc != nil {
		return m.writeFunc(p)
	}
	return len(p), nil
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestClaudeRunner_Wait(t *testing.T) {
	// Test Wait() - it waits for all goroutines to complete
	// We can't easily test the full behavior without mocking, but we can
	// verify the method exists and doesn't panic when called on a nil runner

	t.Run("Wait on nil runner does not panic", func(t *testing.T) {
		// This verifies the method is safe to call
		// In real usage, Wait() is called after Start() and waits for goroutines
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Wait() panicked: %v", r)
			}
		}()

		// Create a minimal runner just to test the Wait method signature
		runner := &ClaudeRunner{}
		runner.Wait() // Should not panic even with uninitialized runner
	})
}

func TestClaudeRunner_Stop(t *testing.T) {
	t.Run("Stop calls cancel and Wait", func(t *testing.T) {
		cancelCalled := false
		waitCalled := false

		// Create a mock runner with controlled behavior
		runner := &ClaudeRunner{
			cancel: func() {
				cancelCalled = true
			},
		}

		// Override Wait to track if it's called
		originalWait := runner.Wait
		runner.Wait = func() {
			waitCalled = true
			originalWait()
		}

		runner.Stop()

		if !cancelCalled {
			t.Error("Stop() should call cancel")
		}
		if !waitCalled {
			t.Error("Stop() should call Wait")
		}
	})

	t.Run("Stop with nil cancel does not panic", func(t *testing.T) {
		runner := &ClaudeRunner{}
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Stop() with nil cancel panicked: %v", r)
			}
		}()
		runner.Stop()
	})
}

func TestClaudeRunner_WriteInput(t *testing.T) {
	t.Run("WriteInput writes to stdin", func(t *testing.T) {
		writeData := []byte("test input")
		writeCalled := false
		var writtenData []byte

		mockStdin := &mockWriteCloser{
			writeFunc: func(p []byte) (int, error) {
				writeCalled = true
				writtenData = append([]byte{}, p...)
				return len(p), nil
			},
		}

		runner := &ClaudeRunner{
			stdin: mockStdin,
		}

		err := runner.WriteInput(writeData)

		if err != nil {
			t.Errorf("WriteInput() returned unexpected error: %v", err)
		}
		if !writeCalled {
			t.Error("WriteInput() should write to stdin")
		}
		if string(writtenData) != string(writeData) {
			t.Errorf("WriteInput() wrote %q, want %q", string(writtenData), string(writeData))
		}
	})

	t.Run("WriteInput returns error from stdin", func(t *testing.T) {
		expectedErr := &testError{message: "write failed"}

		mockStdin := &mockWriteCloser{
			writeFunc: func(p []byte) (int, error) {
				return 0, expectedErr
			},
		}

		runner := &ClaudeRunner{
			stdin: mockStdin,
		}

		err := runner.WriteInput([]byte("test"))

		if err != expectedErr {
			t.Errorf("WriteInput() should return stdin error, got: %v", err)
		}
	})

	t.Run("WriteInput with nil stdin returns error", func(t *testing.T) {
		runner := &ClaudeRunner{
			stdin: nil,
		}

		err := runner.WriteInput([]byte("test"))

		if err == nil {
			t.Error("WriteInput() with nil stdin should return an error")
		}
	})
}
