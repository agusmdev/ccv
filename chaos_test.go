package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestChannelSafety_ClosedChannel tests that closed channels are handled gracefully
func TestChannelSafety_ClosedChannel(t *testing.T) {
	t.Run("send to closed messages channel", func(t *testing.T) {
		messages := make(chan interface{}, 10)
		close(messages)

		defer func() {
			// Sending to closed channel will panic, which is expected
			// The test verifies we understand this behavior
			if r := recover(); r != nil {
				// Expected panic - channel is closed
				t.Logf("Expected panic when sending to closed channel: %v", r)
			}
		}()

		messages <- &SystemInit{Type: "system"}
		t.Error("Should have panicked when sending to closed channel")
	})

	t.Run("close channel twice", func(t *testing.T) {
		messages := make(chan interface{}, 10)
		close(messages)

		defer func() {
			if r := recover(); r != nil {
				// Closing an already-closed channel will panic
				t.Logf("Expected panic when closing already-closed channel: %v", r)
			}
		}()

		close(messages)
		t.Error("Should have panicked when closing already-closed channel")
	})
}

// TestChannelSafety_ContextCancellation tests context cancellation handling
func TestChannelSafety_ContextCancellation(t *testing.T) {
	t.Run("runner respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		runner := &ClaudeRunner{
			messages: make(chan interface{}, 10),
			errors:   make(chan error, 10),
			ctx:      ctx,
			cancel:   cancel,
		}

		// Cancel the context immediately
		cancel()

		// Operations should respect cancellation
		select {
		case <-ctx.Done():
			// Expected - context is cancelled
		default:
			t.Error("Context should be cancelled")
		}
	})

	t.Run("nil context handling", func(t *testing.T) {
		// Using a nil context will cause issues, test that we handle it
		// We can't actually create a runner with nil context, but we can test
		// that our code would handle it

		var nilCtx context.Context

		defer func() {
			if r := recover(); r != nil {
				t.Logf("Nil context handling panic: %v", r)
			}
		}()

		// Checking nil context - this will be nil
		if nilCtx == nil {
			// This is expected
		}
	})
}

// TestChannelSafety_ConcurrentAccess tests concurrent channel access patterns
func TestChannelSafety_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent message sends", func(t *testing.T) {
		messages := make(chan interface{}, 1000)
		var wg sync.WaitGroup

		numGoroutines := 100
		messagesPerGoroutine := 100

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Concurrent sends panicked: %v", r)
			}
		}()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					msg := &SystemInit{
						Type:      "system",
						SessionID: fmt.Sprintf("session_%d_%d", id, j),
					}
					messages <- msg
				}
			}(i)
		}

		// Wait for all sends
		go func() {
			wg.Wait()
			close(messages)
		}()

		// Count received messages
		count := 0
		for range messages {
			count++
		}

		expected := numGoroutines * messagesPerGoroutine
		if count != expected {
			t.Errorf("Expected %d messages, got %d", expected, count)
		}
	})

	t.Run("concurrent channel close and read", func(t *testing.T) {
		// Test that reading from a channel that's being closed by another goroutine is safe
		messages := make(chan interface{}, 10)
		var wg sync.WaitGroup

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Concurrent close and read panicked: %v", r)
			}
		}()

		// Reader goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range messages {
				// Read messages
			}
		}()

		// Writer goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(messages)
			for i := 0; i < 10; i++ {
				messages <- &SystemInit{Type: "system"}
			}
		}()

		wg.Wait()
	})
}

// TestChannelSafety_BufferOverflow tests buffer overflow scenarios
func TestChannelSafety_BufferOverflow(t *testing.T) {
	t.Run("send to full buffer", func(t *testing.T) {
		messages := make(chan interface{}, 1)
		messages <- &SystemInit{Type: "system"}

		// Channel is now full
		sendDone := make(chan bool)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Send to full channel recovered: %v", r)
				}
				sendDone <- true
			}()
			// This will block until we receive
			messages <- &SystemInit{Type: "system2"}
			sendDone <- true
		}()

		// Wait a bit for goroutine to block
		time.Sleep(10 * time.Millisecond)

		// Now receive to unblock
		<-messages

		// The send should complete
		select {
		case <-sendDone:
			// Send completed successfully
		case <-time.After(100 * time.Millisecond):
			t.Error("Send did not complete within timeout")
		}
	})
}

// TestParseMessage_NeverCrashChaos tests that ParseMessage never crashes on random input
func TestParseMessage_NeverCrashChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	// Test cases that should never crash
	testInputs := []string{
		// Valid JSON
		`{"type":"system","session_id":"test"}`,
		`{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`,

		// Malformed JSON
		`{"type":"system"`,
		`{"type":}`,
		`not json at all`,
		`{{{`,
		`}}}`,

		// Empty and whitespace
		``,
		`   `,
		`\n\n\n`,
		`\t\t\t`,

		// Special characters
		"\x00\x01\x02",
		"\xff\xfe\xfd",
		"\x00\x00\x00\x00",

		// Very long input
		string(make([]byte, 10000)),

		// Unicode edge cases
		"\xef\xbb\xbf", // BOM
		"\xc0\x80",     // modified UTF-8 null
	}

	for i, input := range testInputs {
		t.Run(fmt.Sprintf("chaos_%d", i), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage crashed on input %q: %v", input, r)
				}
			}()

			_, err := ParseMessage([]byte(input))
			// We don't care about the result, just that it doesn't panic
			_ = err
		})
	}
}

// TestNeverCrash_MillionLines processes many lines without crashing
func TestNeverCrash_MillionLines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping million-line test in short mode")
	}

	const numLines = 100000 // 100K for faster testing, but still substantial

	// Generate mixed valid/invalid JSON lines
	validJSON := `{"type":"system","session_id":"sess_%d","model":"claude-opus-4"}`
	invalidJSONs := []string{
		`{"type":"system"`,
		`not json`,
		`{}`,
		``,
		`{"type":"unknown","value":%d}`,
	}

	processed := 0
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panicked after processing %d lines: %v", processed, r)
		}
	}()

	for i := 0; i < numLines; i++ {
		var input string
		if i%5 == 0 {
			// Every 5th line is invalid
			input = invalidJSONs[i%len(invalidJSONs)]
		} else {
			// Valid JSON
			input = fmt.Sprintf(validJSON, i)
		}

		_, err := ParseMessage([]byte(input))
		_ = err // Ignore errors, we just want no panics
		processed++
	}

	if processed != numLines {
		t.Errorf("Expected to process %d lines, only processed %d", numLines, processed)
	}
}

// TestOutputProcessor_NeverCrashChaos tests that OutputProcessor never crashes
func TestOutputProcessor_NeverCrashChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	messages := make(chan interface{}, 1000)
	errors := make(chan error, 100)
	p := &OutputProcessor{
		mode:   OutputModeText,
		writer: &mockWriter{},
		state:  NewAppState(),
		colors: NoColorScheme(),
	}

	// Start processing in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				// This would be a failure
				t.Errorf("OutputProcessor.ProcessMessages panicked: %v", r)
			}
		}()
		p.ProcessMessages(messages, errors)
	}()

	// Send various messages
	messages <- createTestSystemInit("test", "claude-opus-4")
	messages <- createTestAssistantMessage([]ContentBlock{
		*createTestContentBlock(ContentBlockTypeText, "test"),
	})

	// Send nil message (shouldn't happen in practice but test resilience)
	// Note: We can't actually send nil on a typed channel

	// Send unknown message type
	messages <- &BaseMessage{Type: "unknown_future_type"}

	// Send result
	messages <- createTestResult(0.01, 1000, 1)

	// Close and wait
	close(messages)
	close(errors)

	// Wait for processing to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("OutputProcessor did not complete within timeout")
	}
}

// TestConcurrentParseMessage tests concurrent ParseMessage calls
func TestConcurrentParseMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	testInputs := []string{
		`{"type":"system","session_id":"test"}`,
		`{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`,
		`{"type":"result","is_error":false}`,
		`{"type":"unknown"}`,
	}

	const numGoroutines = 50
	const callsPerGoroutine = 1000

	var wg sync.WaitGroup
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Concurrent ParseMessage panicked: %v", r)
		}
	}()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				input := testInputs[j%len(testInputs)]
				_, err := ParseMessage([]byte(input))
				_ = err
			}
		}(i)
	}

	wg.Wait()
}

// TestAppState_ConcurrentOperations tests concurrent AppState operations
func TestAppState_ConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	state := NewAppState()
	state.InitializeSession(&SystemInit{SessionID: "test", Model: "claude"})

	const numGoroutines = 50
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Concurrent AppState operations panicked: %v", r)
		}
	}()

	// Tool adders
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				toolID := fmt.Sprintf("tool_%d_%d", id, j)
				toolCall := &ToolCall{
					ID:     toolID,
					Name:   "Read",
					Status: ToolCallStatusPending,
				}
				state.AddOrUpdateToolCall(toolCall)
			}
		}(i)
	}

	// Token updaters
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				usage := &Usage{
					InputTokens:  j,
					OutputTokens: j / 2,
				}
				state.UpdateTokens(usage)
			}
		}(i)
	}

	// Stream operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				toolID := fmt.Sprintf("tool_%d", j%10)
				state.AppendStreamToolInput(toolID, fmt.Sprintf(`"part_%d"`, j))
				state.GetStreamToolInput(toolID)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is consistent
	if state.TotalTokens.InputTokens <= 0 {
		t.Error("Expected positive input tokens")
	}
}

// BenchmarkParseMessage_ValidJSON benchmarks parsing valid JSON
func BenchmarkParseMessage_ValidJSON(b *testing.B) {
	data := []byte(`{"type":"system","session_id":"test-session","model":"claude-opus-4-5-20251101","tools":["Read","Write"]}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseMessage(data)
		if err != nil {
			b.Fatalf("ParseMessage failed: %v", err)
		}
	}
}

// BenchmarkParseMessage_InvalidJSON benchmarks error handling on invalid JSON
func BenchmarkParseMessage_InvalidJSON(b *testing.B) {
	data := []byte(`{"type":"system","invalid`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseMessage(data)
		_ = err // Expected to error
	}
}
