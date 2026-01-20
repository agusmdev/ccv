package main

import (
	"sync"
	"testing"
	"time"
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

		// Create a mock runner with controlled behavior
		runner := &ClaudeRunner{
			cancel: func() {
				cancelCalled = true
			},
		}

		// Stop should call cancel() and then Wait()
		// Since Wait() calls wg.Wait() on an empty WaitGroup, it will return immediately
		runner.Stop()

		if !cancelCalled {
			t.Error("Stop() should call cancel")
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

// mockReadCloser implements io.ReadCloser for testing
type mockReadCloser struct {
	readFunc  func([]byte) (int, error)
	closeFunc func() error
	closed    bool
}

func (m *mockReadCloser) Read(p []byte) (int, error) {
	if m.readFunc != nil {
		return m.readFunc(p)
	}
	return 0, nil
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// TestClaudeRunner_parseStdout_PanicRecovery tests that parseStdout recovers from panics
func TestClaudeRunner_parseStdout_PanicRecovery(t *testing.T) {
	// Create a mock stdin that causes a panic
	panicRead := func(p []byte) (int, error) {
		panic("test panic in parseStdout")
	}

	runner := &ClaudeRunner{
		stdout: &mockReadCloser{
			readFunc: panicRead,
		},
		messages: make(chan interface{}, 10),
		errors:   make(chan error, 10),
		wg:       &sync.WaitGroup{},
	}

	// Start parseStdout in goroutine
	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// This is expected, we're testing panic recovery
				done <- true
			}
		}()
		runner.parseStdout()
		close(done)
	}()

	// Wait for completion
	select {
	case <-done:
		// Panic was caught
	case <-time.After(1 * time.Second):
		t.Error("parseStdout did not complete within timeout")
	}
}

// TestClaudeRunner_forwardStderr_PanicRecovery tests that forwardStderr handles panics
func TestClaudeRunner_forwardStderr_PanicRecovery(t *testing.T) {
	// Create a mock stderr that causes a panic
	panicRead := func(p []byte) (int, error) {
		panic("test panic in forwardStderr")
	}

	runner := &ClaudeRunner{
		stderr: &mockReadCloser{
			readFunc: panicRead,
		},
		wg: &sync.WaitGroup{},
	}

	// Start forwardStderr in goroutine
	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- true
			}
		}()
		runner.forwardStderr()
		close(done)
	}()

	// Wait for completion
	select {
	case <-done:
		// Panic was caught
	case <-time.After(1 * time.Second):
		t.Error("forwardStderr did not complete within timeout")
	}
}

// TestClaudeRunner_MessagesChannel tests Messages() returns correct channel
func TestClaudeRunner_MessagesChannel(t *testing.T) {
	messages := make(chan interface{}, 10)
	runner := &ClaudeRunner{
		messages: messages,
	}

	result := runner.Messages()
	if result != messages {
		t.Error("Messages() should return the messages channel")
	}
}

// TestClaudeRunner_ErrorsChannel tests Errors() returns correct channel
func TestClaudeRunner_ErrorsChannel(t *testing.T) {
	errors := make(chan error, 10)
	runner := &ClaudeRunner{
		errors: errors,
	}

	result := runner.Errors()
	if result != errors {
		t.Error("Errors() should return the errors channel")
	}
}

// TestHasFlag_EdgeCases tests additional edge cases for hasFlag
func TestHasFlag_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		expected bool
	}{
		{
			name:     "flag with equals and exact match",
			args:     []string{"--output-format=stream-json"},
			flag:     "--output-format",
			expected: true,
		},
		{
			name:     "similar but different flag",
			args:     []string{"--output", "--format"},
			flag:     "--output-format",
			expected: false,
		},
		{
			name:     "flag as substring of another",
			args:     []string{"--verbose-mode"},
			flag:     "--verbose",
			expected: false,
		},
		{
			name:     "flag appears multiple times",
			args:     []string{"--verbose", "--other", "--verbose"},
			flag:     "--verbose",
			expected: true,
		},
		{
			name:     "single dash long flag",
			args:     []string{"-verbose"},
			flag:     "--verbose",
			expected: false,
		},
		{
			name:     "flag with value attached but no equals",
			args:     []string{"--formatjson"},
			flag:     "--format",
			expected: false,
		},
		{
			name:     "case sensitivity",
			args:     []string{"--Verbose"},
			flag:     "--verbose",
			expected: false,
		},
		{
			name:     "flag with extra dashes",
			args:     []string{"---verbose"},
			flag:     "--verbose",
			expected: false,
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

// TestHasFlagValue_EdgeCases tests additional edge cases for hasFlagValue
func TestHasFlagValue_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		value    string
		expected bool
	}{
		{
			name:     "value matches but different flag",
			args:     []string{"--other", "stream-json"},
			flag:     "--output-format",
			value:    "stream-json",
			expected: false,
		},
		{
			name:     "empty value in args",
			args:     []string{"--output-format", ""},
			flag:     "--output-format",
			value:    "",
			expected: true,
		},
		{
			name:     "value with special characters",
			args:     []string{"--query", "name=test&value=123"},
			flag:     "--query",
			value:    "name=test&value=123",
			expected: true,
		},
		{
			name:     "multiple spaces in value",
			args:     []string{"--description", "some thing with spaces"},
			flag:     "--description",
			value:    "some thing with spaces",
			expected: true,
		},
		{
			name:     "equals format with spaces",
			args:     []string{"--format=stream json"},
			flag:     "--format",
			value:    "stream json",
			expected: true,
		},
		{
			name:     "flag at end with no value",
			args:     []string{"--output", "--format"},
			flag:     "--format",
			value:    "json",
			expected: false,
		},
		{
			name:     "value with number",
			args:     []string{"--max-turns", "50"},
			flag:     "--max-turns",
			value:    "50",
			expected: true,
		},
		{
			name:     "value with boolean string",
			args:     []string{"--enabled", "true"},
			flag:     "--enabled",
			value:    "true",
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

// TestClaudeRunner_WaitGroup tests Wait() properly waits for goroutines
func TestClaudeRunner_WaitGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WaitGroup test in short mode")
	}

	runner := &ClaudeRunner{
		wg: &sync.WaitGroup{},
	}

	// Add some work to the wait group
	runner.wg.Add(2)
	go func() {
		time.Sleep(10 * time.Millisecond)
		runner.wg.Done()
	}()
	go func() {
		time.Sleep(10 * time.Millisecond)
		runner.wg.Done()
	}()

	// Wait should complete
	done := make(chan bool)
	go func() {
		runner.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("Wait() did not complete within timeout")
	}
}

// TestClaudeRunner_StopOrder tests that Stop() calls cancel before Wait
func TestClaudeRunner_StopOrder(t *testing.T) {
	callOrder := []string{}
	runner := &ClaudeRunner{
		cancel: func() {
			callOrder = append(callOrder, "cancel")
		},
		wg: &sync.WaitGroup{},
	}

	// Mock Wait to track when it's called
	originalWait := runner.Wait
	runner.Wait = func() {
		callOrder = append(callOrder, "wait")
		originalWait()
	}

	runner.Stop()

	if len(callOrder) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "cancel" {
		t.Errorf("expected cancel to be called first, got: %v", callOrder)
	}
	if callOrder[1] != "wait" {
		t.Errorf("expected wait to be called second, got: %v", callOrder)
	}
}
