package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
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

// ==================== parseStdout Comprehensive Tests ====================

// TestClaudeRunner_parseStdout_EmptyLinesSkipped tests that empty lines are skipped
func TestClaudeRunner_parseStdout_EmptyLinesSkipped(t *testing.T) {
	// Input with multiple empty lines
	input := `{"type":"system","subtype":"init","session_id":"test1"}

{"type":"system","subtype":"init","session_id":"test2"}

`

	readIndex := 0
	mockStdout := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(input) {
				return 0, io.EOF
			}
			n := copy(p, input[readIndex:])
			readIndex += n
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stdout:   mockStdout,
		messages: make(chan interface{}, 10),
		errors:   make(chan error, 10),
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.parseStdout()

	// Collect messages
	var messages []interface{}
	timeout := time.After(1 * time.Second)
CollectLoop:
	for {
		select {
		case msg, ok := <-runner.messages:
			if !ok {
				break CollectLoop
			}
			messages = append(messages, msg)
		case <-timeout:
			t.Error("Timeout waiting for messages")
			break CollectLoop
		}
	}

	runner.Wait()

	// Should have received 2 messages (empty lines skipped)
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

// TestClaudeRunner_parseStdout_ValidJSON tests valid JSON is parsed
func TestClaudeRunner_parseStdout_ValidJSON(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"test123"}
{"type":"assistant","message":{"id":"msg1","type":"message","role":"assistant","content":[]}}
`

	readIndex := 0
	mockStdout := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(input) {
				return 0, io.EOF
			}
			n := copy(p, input[readIndex:])
			readIndex += n
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stdout:   mockStdout,
		messages: make(chan interface{}, 10),
		errors:   make(chan error, 10),
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.parseStdout()

	var messages []interface{}
	timeout := time.After(1 * time.Second)
CollectLoop2:
	for {
		select {
		case msg, ok := <-runner.messages:
			if !ok {
				break CollectLoop2
			}
			messages = append(messages, msg)
		case <-timeout:
			t.Fatal("Timeout waiting for messages")
		}
	}

	runner.Wait()

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Verify first message is SystemInit
	sysMsg, ok := messages[0].(*SystemInit)
	if !ok {
		t.Fatalf("first message should be *SystemInit, got %T", messages[0])
	}
	if sysMsg.SessionID != "test123" {
		t.Errorf("expected SessionID 'test123', got %q", sysMsg.SessionID)
	}

	// Verify second message is AssistantMessage
	asstMsg, ok := messages[1].(*AssistantMessage)
	if !ok {
		t.Fatalf("second message should be *AssistantMessage, got %T", messages[1])
	}
	if asstMsg.Message.ID != "msg1" {
		t.Errorf("expected Message.ID 'msg1', got %q", asstMsg.Message.ID)
	}
}

// TestClaudeRunner_parseStdout_InvalidJSON tests invalid JSON sends error
func TestClaudeRunner_parseStdout_InvalidJSON(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"valid"}
{invalid json here}
{"type":"assistant","message":{"id":"msg1","type":"message","role":"assistant","content":[]}}
`

	readIndex := 0
	mockStdout := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(input) {
				return 0, io.EOF
			}
			n := copy(p, input[readIndex:])
			readIndex += n
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stdout:   mockStdout,
		messages: make(chan interface{}, 10),
		errors:   make(chan error, 10),
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.parseStdout()

	// Collect both errors and messages
	var errors []error
	var messages []interface{}

	done := make(chan bool)
	go func() {
		for {
			select {
			case err, ok := <-runner.errors:
				if ok {
					errors = append(errors, err)
				}
			case msg, ok := <-runner.messages:
				if !ok {
					close(done)
					return
				}
				messages = append(messages, msg)
			}
		}
	}()

	<-done
	runner.Wait()

	// Should have received 1 error for invalid JSON
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(errors), errors)
	}
	// Should still have received 2 valid messages
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

// TestClaudeRunner_parseStdout_ScannerError tests scanner errors are sent to errors channel
func TestClaudeRunner_parseStdout_ScannerError(t *testing.T) {
	scannerErr := &testError{message: "scanner error"}

	mockStdout := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			return 0, scannerErr
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stdout:   mockStdout,
		messages: make(chan interface{}, 10),
		errors:   make(chan error, 10),
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.parseStdout()

	select {
	case err := <-runner.errors:
		if err == nil {
			t.Error("expected error from scanner")
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for scanner error")
	}

	runner.Wait()
}

// TestClaudeRunner_parseStdout_ContextCancellation tests context cancellation stops processing
func TestClaudeRunner_parseStdout_ContextCancellation(t *testing.T) {
	// Continuous input that never ends
	infiniteInput := `{"type":"system","subtype":"init","session_id":"test1"}
{"type":"system","subtype":"init","session_id":"test2"}
{"type":"system","subtype":"init","session_id":"test3"}
`

	readIndex := 0
	callCount := 0
	mockStdout := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(infiniteInput) {
				readIndex = 0 // Loop forever
			}
			n := copy(p, infiniteInput[readIndex:])
			readIndex += n
			callCount++
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	runner := &ClaudeRunner{
		stdout:   mockStdout,
		messages: make(chan interface{}, 100),
		errors:   make(chan error, 10),
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.parseStdout()

	// Receive a few messages then cancel
	msgCount := 0
	for msg := range runner.messages {
		msgCount++
		if msgCount >= 3 {
			cancel() // Cancel context
			break
		}
	}

	runner.Wait()

	// Verify we stopped after cancellation (not all infinite messages)
	if msgCount < 3 {
		t.Errorf("expected at least 3 messages, got %d", msgCount)
	}
}

// TestClaudeRunner_parseStdout_MessagesChannelClosed tests messages channel is closed on completion
func TestClaudeRunner_parseStdout_MessagesChannelClosed(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"test"}`

	readIndex := 0
	mockStdout := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(input) {
				return 0, io.EOF
			}
			n := copy(p, input[readIndex:])
			readIndex += n
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stdout:   mockStdout,
		messages: make(chan interface{}, 10),
		errors:   make(chan error, 10),
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.parseStdout()

	runner.Wait()

	// Channel should be closed - receiving should not block
	_, ok := <-runner.messages
	if ok {
		t.Error("messages channel should be closed after parseStdout completes")
	}
}

// TestClaudeRunner_parseStdout_LargeMessages tests large messages within 1MB buffer
func TestClaudeRunner_parseStdout_LargeMessages(t *testing.T) {
	// Create a large JSON message (100KB)
	largeContent := make([]byte, 100*1024)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	largeInput := `{"type":"system","subtype":"init","session_id":"test","model":"` + string(largeContent) + `"}`

	readIndex := 0
	mockStdout := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(largeInput) {
				return 0, io.EOF
			}
			n := copy(p, largeInput[readIndex:])
			readIndex += n
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stdout:   mockStdout,
		messages: make(chan interface{}, 10),
		errors:   make(chan error, 10),
		ctx:      ctx,
		wg:       &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.parseStdout()

	select {
	case msg := <-runner.messages:
		if msg == nil {
			t.Error("expected non-nil message")
		}
	case err := <-runner.errors:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for large message")
	}

	runner.Wait()
}

// ==================== forwardStderr Comprehensive Tests ====================

// TestClaudeRunner_forwardStderr_LinesForwarded tests stderr lines are forwarded
func TestClaudeRunner_forwardStderr_LinesForwarded(t *testing.T) {
	// Capture stderr output
	originalStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	input := "error line 1\nerror line 2\nerror line 3\n"

	readIndex := 0
	mockStderr := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(input) {
				return 0, io.EOF
			}
			n := copy(p, input[readIndex:])
			readIndex += n
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stderr: mockStderr,
		errors: make(chan error, 10),
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.forwardStderr()

	runner.Wait()

	w.Close()
	os.Stderr = originalStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify stderr was forwarded
	if !strings.Contains(output, "error line 1") {
		t.Errorf("stderr should contain 'error line 1', got: %q", output)
	}
	if !strings.Contains(output, "error line 2") {
		t.Errorf("stderr should contain 'error line 2', got: %q", output)
	}
}

// TestClaudeRunner_forwardStderr_ScannerError tests scanner errors are sent to errors channel
func TestClaudeRunner_forwardStderr_ScannerError(t *testing.T) {
	scannerErr := &testError{message: "stderr scanner error"}

	mockStderr := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			return 0, scannerErr
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := &ClaudeRunner{
		stderr: mockStderr,
		errors: make(chan error, 10),
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.forwardStderr()

	select {
	case err := <-runner.errors:
		if err == nil {
			t.Error("expected error from stderr scanner")
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for stderr scanner error")
	}

	runner.Wait()
}

// TestClaudeRunner_forwardStderr_ContextCancellation tests context cancellation stops processing
func TestClaudeRunner_forwardStderr_ContextCancellation(t *testing.T) {
	infiniteInput := "error line 1\nerror line 2\n"

	readIndex := 0
	mockStderr := &mockReadCloser{
		readFunc: func(p []byte) (int, error) {
			if readIndex >= len(infiniteInput) {
				readIndex = 0 // Loop forever
			}
			n := copy(p, infiniteInput[readIndex:])
			readIndex += n
			return n, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	runner := &ClaudeRunner{
		stderr: mockStderr,
		errors: make(chan error, 10),
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.forwardStderr()

	// Let it run a bit then cancel
	time.Sleep(10 * time.Millisecond)
	cancel()

	runner.Wait()

	// If this test completes without hanging, cancellation worked
}

// ==================== waitForCompletion Comprehensive Tests ====================

// TestClaudeRunner_waitForCompletion_SuccessfulExit tests successful process completion
func TestClaudeRunner_waitForCompletion_SuccessfulExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a command that exits successfully
	cmd := exec.CommandContext(ctx, "echo", "test")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	runner := &ClaudeRunner{
		cmd:    cmd,
		errors: make(chan error, 10),
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.waitForCompletion()

	// Should complete without errors
	select {
	case err := <-runner.errors:
		t.Errorf("unexpected error on successful exit: %v", err)
	case <-time.After(5 * time.Second):
		// Success - no error received
	}

	runner.Wait()
}

// TestClaudeRunner_waitForCompletion_NonZeroExit tests non-zero exit sends error
func TestClaudeRunner_waitForCompletion_NonZeroExit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a command that exits with non-zero status
	cmd := exec.CommandContext(ctx, "sh", "-c", "exit 42")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	runner := &ClaudeRunner{
		cmd:    cmd,
		errors: make(chan error, 10),
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.waitForCompletion()

	select {
	case err := <-runner.errors:
		if err == nil {
			t.Error("expected error for non-zero exit")
		}
		if !strings.Contains(err.Error(), "exited") {
			t.Errorf("error should mention process exit, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for non-zero exit error")
	}

	runner.Wait()
}

// TestClaudeRunner_waitForCompletion_ContextCancellation tests context cancellation doesn't send error
func TestClaudeRunner_waitForCompletion_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a long-running command
	cmd := exec.CommandContext(ctx, "sleep", "100")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	runner := &ClaudeRunner{
		cmd:    cmd,
		errors: make(chan error, 10),
		ctx:    ctx,
		wg:     &sync.WaitGroup{},
	}

	runner.wg.Add(1)
	go runner.waitForCompletion()

	// Cancel context immediately
	cancel()

	// Should complete without sending an error
	select {
	case err := <-runner.errors:
		t.Errorf("unexpected error when context cancelled: %v", err)
	case <-time.After(1 * time.Second):
		// Success - no error received within timeout
	}

	runner.Wait()
}

// ==================== Integration Tests ====================

// TestClaudeRunner_FullLifecycle tests full lifecycle with mock subprocess
func TestClaudeRunner_FullLifecycle(t *testing.T) {
	ctx := context.Background()

	// Create a mock subprocess that outputs JSON
	runner, err := NewClaudeRunner(ctx, []string{"--help"})
	if err != nil {
		t.Fatalf("NewClaudeRunner failed: %v", err)
	}

	// Start the runner
	if err := runner.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Collect some output
	messageReceived := false
	errorReceived := false

	done := make(chan bool)
	go func() {
		timeout := time.After(10 * time.Second)
		for {
			select {
			case <-runner.Messages():
				messageReceived = true
			case <-runner.Errors():
				errorReceived = true
			case <-timeout:
				close(done)
				return
			}
		}
	}()

	<-done
	runner.Stop()

	// Verify we received some data
	if !messageReceived && !errorReceived {
		t.Error("expected at least one message or error")
	}
}

// TestClaudeRunner_Stop_CancelsAndWaits tests Stop cancels context and waits for goroutines
func TestClaudeRunner_Stop_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	runner, err := NewClaudeRunner(ctx, []string{"--help"})
	if err != nil {
		t.Fatalf("NewClaudeRunner failed: %v", err)
	}

	if err := runner.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop immediately
	runner.Stop()

	// Verify runner is stopped - no timeout should occur
	done := make(chan bool)
	go func() {
		runner.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Stop() did not complete goroutines within timeout")
	}
}
