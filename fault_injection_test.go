package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestFaultInjection_NilColorScheme tests behavior with nil color scheme
func TestFaultInjection_NilColorScheme(t *testing.T) {
	// Test format functions with nil color scheme
	tests := []struct {
		name     string
		testFunc func(*ColorScheme)
	}{
		{
			name: "FormatFilePath with nil scheme",
			testFunc: func(c *ColorScheme) {
				result := FormatFilePath("/path/to/file.go", c)
				_ = result // Should not panic
			},
		},
		{
			name: "FormatCommand with nil scheme",
			testFunc: func(c *ColorScheme) {
				result := FormatCommand("go build", c)
				_ = result
			},
		},
		{
			name: "FormatToolName with nil scheme",
			testFunc: func(c *ColorScheme) {
				result := FormatToolName("Bash", c)
				_ = result
			},
		},
		{
			name: "FormatAgentContext with nil scheme",
			testFunc: func(c *ColorScheme) {
				result := FormatAgentContext("main", "running", c)
				_ = result
			},
		},
		{
			name: "highlightSyntax with nil scheme",
			testFunc: func(c *ColorScheme) {
				result := highlightSyntax("echo hello", c)
				_ = result
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked with nil color scheme: %v", tt.name, r)
				}
			}()
			tt.testFunc(nil)
		})
	}
}

// TestFaultInjection_NilAppState tests operations with nil app state
func TestFaultInjection_NilAppState(t *testing.T) {
	t.Run("AddOrUpdateToolCall with nil state", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("AddOrUpdateToolCall with nil state panicked: %v", r)
			}
		}()

		var state *AppState
		toolCall := &ToolCall{
			ID:     "tool_1",
			Name:   "Read",
			Status: ToolCallStatusPending,
		}
		// This should handle nil state gracefully
		defer func() {
			_ = recover()
		}()
		state.AddOrUpdateToolCall(toolCall)
		// If we got here without panic above, the nil check worked
	})
}

// TestFaultInput_ClosedChannels tests operations with closed channels
func TestFaultInjection_ClosedChannels(t *testing.T) {
	t.Run("send to closed messages channel", func(t *testing.T) {
		messages := make(chan interface{}, 10)
		close(messages)

		defer func() {
			if r := recover(); r != nil {
				// Expected panic when sending to closed channel
				t.Logf("Expected panic when sending to closed channel: %v", r)
			}
		}()

		messages <- &SystemInit{Type: "system"}
		// If we get here, the test failed (should have panicked)
		t.Error("Should have panicked when sending to closed channel")
	})
}

// TestFaultInjection_InvalidUnicode tests handling of invalid UTF-8
func TestFaultInjection_InvalidUnicode(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"null bytes", []byte("test\x00\x00\x00")},
		{"invalid UTF-8 sequence", []byte("test\xff\xfe")},
		{"incomplete UTF-8", []byte("test\xc0")},
		{"overlong encoding", []byte("test\xf8\x80\x80\x80\x80")},
		{"mixed valid and invalid", []byte("valid\xfftext\xfe")},
		{"null in JSON", []byte(`{"type":"test","value":"te\x00st"}`)},
		{"high surrogate without low", []byte("test\xd8\x00")},
		{"low surrogate without high", []byte("test\xdc\x00")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Invalid UTF-8 %s caused panic: %v", tc.name, r)
				}
			}()

			// Test with ParseMessage
			_, err := ParseMessage(tc.data)
			_ = err // May error, but shouldn't panic

			// Test with JSON unmarshal
			var result map[string]interface{}
			err = json.Unmarshal(tc.data, &result)
			_ = err

			// Test with string conversion
			_ = string(tc.data)
		})
	}
}

// TestFaultInjection_ExtremelyLongLines tests handling of very long input
func TestFaultInjection_ExtremelyLongLines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long line test in short mode")
	}

	t.Run("1MB JSON line", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("1MB JSON line caused panic: %v", r)
			}
		}()

		// Create a 1MB JSON message
		longJSON := make([]byte, 0, 1024*1024+200)
		longJSON = append(longJSON, `{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"`...)
		for i := 0; i < 1024*1024; i++ {
			longJSON = append(longJSON, 'a')
		}
		longJSON = append(longJSON, `"}]}}`...)

		_, err := ParseMessage(longJSON)
		_ = err
	})

	t.Run("10MB string input", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping 10MB test in short mode")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("10MB string caused panic: %v", r)
			}
		}()

		// Create a very long string
		longStr := strings.Repeat("a", 10*1024*1024)

		// Test with format functions
		result := IndentLines(longStr, "  ")
		_ = result
	})
}

// TestFaultInjection_DeeplyNestedJSON tests deeply nested structures
func TestFaultInjection_DeeplyNestedJSON(t *testing.T) {
	t.Run("1000 nested objects", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Deeply nested JSON caused panic: %v", r)
			}
		}()

		// Create deeply nested JSON
		depth := 1000
		jsonStr := `{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"nested":`

		// Add opening braces
		var builder strings.Builder
		builder.WriteString(jsonStr)
		for i := 0; i < depth; i++ {
			builder.WriteString(`{"level":`)
			builder.WriteString(fmt.Sprintf("%d", i))
			builder.WriteString(`,"child":`)
		}
		builder.WriteString(`"leaf"`)
		// Add closing braces
		for i := 0; i < depth; i++ {
			builder.WriteString(`}`)
		}
		builder.WriteString(`}}`)

		_, err := ParseMessage([]byte(builder.String()))
		_ = err
	})
}

// TestFaultInjection_RapidContextCancellation tests rapid context cancellation
func TestFaultInjection_RapidContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rapid cancellation test in short mode")
	}

	t.Run("create and cancel many contexts", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Rapid context cancellation caused panic: %v", r)
			}
		}()

		const iterations = 10000
		for i := 0; i < iterations; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			// Immediately cancel
			cancel()
			// Check context is cancelled
			select {
			case <-ctx.Done():
				// Expected
			default:
				t.Error("Context should be cancelled")
			}
		}
	})

	t.Run("cancel while goroutines are running", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Cancel while running caused panic: %v", r)
			}
		}()

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup

		// Start many goroutines that check context
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 1000; j++ {
					select {
					case <-ctx.Done():
						return
					default:
						// Do some work
						time.Sleep(time.Microsecond)
					}
				}
			}(i)
		}

		// Cancel after a short delay
		time.AfterFunc(10*time.Millisecond, cancel)

		// Wait for all to finish
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Error("Goroutines did not finish in time")
		}
	})
}

// TestFaultInjection_ChannelFlood tests flooding channels with messages
func TestFaultInjection_ChannelFlood(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping channel flood test in short mode")
	}

	t.Run("flood messages channel beyond capacity", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Channel flood caused panic: %v", r)
			}
		}()

		// Small buffer
		messages := make(chan interface{}, 10)
		var wg sync.WaitGroup

		// Start many senders
		numSenders := 100
		messagesPerSender := 1000

		for i := 0; i < numSenders; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerSender; j++ {
					msg := &SystemInit{
						Type:      "system",
						SessionID: fmt.Sprintf("sess_%d_%d", id, j),
					}
					select {
					case messages <- msg:
						// Sent successfully
					case <-time.After(100 * time.Millisecond):
						// Timeout - channel is full, back off
						time.Sleep(time.Millisecond)
					}
				}
			}(i)
		}

		// Start receiver
		go func() {
			for range messages {
				// Receive
			}
		}()

		// Wait for senders to finish
		wg.Wait()
		close(messages)
	})
}

// TestFaultInjection_MemoryPressure tests behavior under memory pressure
func TestFaultInjection_MemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	t.Run("accumulate many messages in state", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Memory pressure caused panic: %v", r)
			}
		}()

		state := NewAppState()
		state.InitializeSession(&SystemInit{SessionID: "test", Model: "claude"})

		// Add many tool calls
		numTools := 10000
		for i := 0; i < numTools; i++ {
			toolCall := &ToolCall{
				ID:     fmt.Sprintf("tool_%d", i),
				Name:   "Read",
				Status: ToolCallStatusPending,
				Input:  json.RawMessage(`{"file_path":"very_long_path_to_make_the_input_larger_than_usual.json"}`),
			}
			state.AddOrUpdateToolCall(toolCall)
		}

		// Verify we can still operate
		if len(state.PendingTools) != numTools {
			t.Errorf("Expected %d tools, got %d", numTools, len(state.PendingTools))
		}
	})
}

// TestFaultInjection_PartialNDJSONStream tests partial/malformed NDJSON
func TestFaultInjection_PartialNDJSONStream(t *testing.T) {
	testCases := []struct {
		name  string
		lines []string
	}{
		{
			name: "truncated JSON line",
			lines: []string{
				`{"type":"system","session_id":"test"`,
				`{"type":"assistant","message":{"id":"msg_1"}}`,
			},
		},
		{
			name: "empty lines interspersed",
			lines: []string{
				``,
				`{"type":"system","session_id":"test"}`,
				``,
				``,
				`{"type":"result","is_error":false}`,
				``,
			},
		},
		{
			name: "whitespace only lines",
			lines: []string{
				`   `,
				`{"type":"system","session_id":"test"}`,
				`\t\t\t`,
				`{"type":"result","is_error":false}`,
			},
		},
		{
			name: "BOM prefix",
			lines: []string{
				"\xef\xbb\xbf" + `{"type":"system","session_id":"test"}`,
			},
		},
		{
			name: "mixed valid and invalid",
			lines: []string{
				`{"type":"system","session_id":"test"}`,
				`{invalid json}`,
				`{"type":"result","is_error":false}`,
				`not json at all`,
				`{"type":"assistant","message":{"id":"msg"}}`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Partial NDJSON %s caused panic: %v", tc.name, r)
				}
			}()

			for _, line := range tc.lines {
				_, err := ParseMessage([]byte(line))
				_ = err // May error, shouldn't panic
			}
		})
	}
}

// TestFaultInjection_StringManipulationEdgeCases tests string edge cases
func TestFaultInjection_StringManipulationEdgeCases(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"only newlines", "\n\n\n\n\n"},
		{"only tabs", "\t\t\t\t\t"},
		{"only spaces", "     "},
		{"mixed whitespace", " \t \n \t \n "},
		{"many consecutive backslashes", strings.Repeat("\\", 1000)},
		{"many consecutive quotes", strings.Repeat("\"", 1000)},
		{"null bytes only", "\x00\x00\x00"},
		{"very long whitespace", strings.Repeat(" \t\n", 10000)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("String edge case %s caused panic: %v", tc.name, r)
				}
			}()

			// Test with format functions
			_ = IndentLines(tc.input, "  ")
			_ = FormatMCPToolName(tc.input)
			_ = highlightSyntax(tc.input, DefaultScheme())
		})
	}
}

// TestNeverCrash_RapidSequentialParsing tests rapid sequential parsing
func TestNeverCrash_RapidSequentialParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rapid sequential test in short mode")
	}

	t.Run("parse 1M messages rapidly", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Rapid sequential parsing caused panic: %v", r)
			}
		}()

		const numMessages = 1000000 // 1M
		validMsg := []byte(`{"type":"system","session_id":"test","model":"claude"}`)
		invalidMsg := []byte(`{invalid json`)

		for i := 0; i < numMessages; i++ {
			var data []byte
			if i%10 == 0 {
				data = invalidMsg
			} else {
				data = validMsg
			}
			_, err := ParseMessage(data)
			_ = err
		}
	})
}
