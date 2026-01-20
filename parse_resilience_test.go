package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestParseMessage_MalformedJSON tests that malformed JSON returns errors gracefully
func TestParseMessage_MalformedJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "missing closing brace",
			json: `{"type":"system","session_id":"test"`,
		},
		{
			name: "missing opening brace",
			json: `"type":"system","session_id":"test"}`,
		},
		{
			name: "missing closing bracket",
			json: `{"type":"system","tools":[1,2,3`,
		},
		{
			name: "missing quote",
			json: `{type:"system","session_id":"test"}`,
		},
		{
			name: "trailing comma",
			json: `{"type":"system",}`,
		},
		{
			name: "missing colon",
			json: `{"type" "system"}`,
		},
		{
			name: "just garbage",
			json: `asdfghjkl`,
		},
		{
			name: "empty string",
			json: ``,
		},
		{
			name: "single quote instead of double",
			json: `{'type':'system'}`,
		},
		{
			name: "unescaped control character",
			json: "{\"type\":\"system\",\"value\":\"line1\nline2\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMessage([]byte(tt.json))
			if err == nil {
				t.Errorf("ParseMessage(%q) expected error for malformed JSON, got nil", tt.json)
			}
		})
	}
}

// TestParseMessage_InvalidUnicode tests invalid Unicode handling
func TestParseMessage_InvalidUnicode(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "invalid UTF-8 sequence",
			json: `{"type":"system","value":"\xff\xfe"}`,
		},
		{
			name: "incomplete UTF-8 sequence",
			json: `{"type":"system","value":"\xc0"}`,
		},
		{
			name: "overlong encoding",
			json: `{"type":"system","value":"\xf0\x82\x82\xac"}`,
		},
		{
			name: "high surrogate without low surrogate",
			json: `{"type":"system","value":"\ud800"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic - may return error or parse depending on Go's json handling
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage(%q) panicked: %v", tt.json, r)
				}
			}()

			_, err := ParseMessage([]byte(tt.json))
			// Result can be error or success depending on how Go's json handles it
			// The important thing is it doesn't panic
			_ = err
		})
	}
}

// TestParseMessage_NullBytes tests handling of null bytes in input
func TestParseMessage_NullBytes(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "null byte in middle",
			json: `{"type":"sys` + "\x00" + `tem","value":"test"}`,
		},
		{
			name: "null byte at start",
			json: "\x00" + `{"type":"system","value":"test"}`,
		},
		{
			name: "null byte at end",
			json: `{"type":"system","value":"test"}` + "\x00",
		},
		{
			name: "multiple null bytes",
			json: "\x00\x00\x00" + `{"type":"system"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage with null byte panicked: %v", r)
				}
			}()

			_, err := ParseMessage([]byte(tt.json))
			// Null bytes will cause JSON parse error, which is fine
			_ = err
		})
	}
}

// TestParseMessage_ExtremelyLongLine tests handling of very long JSON lines
func TestParseMessage_ExtremelyLongLine(t *testing.T) {
	tests := []struct {
		name       string
		sizeInKB   int
		shouldFail bool
	}{
		{name: "10KB", sizeInKB: 10, shouldFail: false},
		{name: "100KB", sizeInKB: 100, shouldFail: false},
		{name: "500KB", sizeInKB: 500, shouldFail: false},
		{name: "1MB", sizeInKB: 1024, shouldFail: false},
		{name: "2MB", sizeInKB: 2048, shouldFail: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a large JSON string with a long text field
			size := tt.sizeInKB * 1024
			longText := strings.Repeat("a", size-200) // Account for JSON overhead

			jsonStr := `{"type":"assistant","message":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"` + longText + `"}]}}`

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage with %s panicked: %v", tt.name, r)
				}
			}()

			msg, err := ParseMessage([]byte(jsonStr))
			if tt.shouldFail {
				if err == nil {
					t.Errorf("ParseMessage(%s) expected error, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("ParseMessage(%s) unexpected error: %v", tt.name, err)
				}
				if msg == nil {
					t.Errorf("ParseMessage(%s) returned nil message", tt.name)
				}
			}
		})
	}
}

// TestParseMessage_DeeplyNestedJSON tests handling of deeply nested JSON structures
func TestParseMessage_DeeplyNestedJSON(t *testing.T) {
	tests := []struct {
		name string
		depth int
	}{
		{name: "depth 10", depth: 10},
		{name: "depth 50", depth: 50},
		{name: "depth 100", depth: 100},
		{name: "depth 500", depth: 500},
		{name: "depth 1000", depth: 1000},
		{name: "depth 10000", depth: 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build deeply nested JSON
			var buf bytes.Buffer
			buf.WriteString(`{"type":"assistant"`)
			for i := 0; i < tt.depth; i++ {
				buf.WriteString(`,"nested`)
				buf.WriteString(string(rune('a' + i%26)))
				buf.WriteString(`":{`)
			}
			buf.WriteString(`"value":"deep"`)
			for i := 0; i < tt.depth; i++ {
				buf.WriteString(`}`)
			}
			buf.WriteString(`}`)

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage with depth %d panicked: %v", tt.depth, r)
				}
			}()

			_, err := ParseMessage(buf.Bytes())
			// May fail with stack overflow or other errors, but shouldn't panic
			_ = err
		})
	}
}

// TestParseMessage_WideJSON tests JSON with many fields at same level
func TestParseMessage_WideJSON(t *testing.T) {
	tests := []struct {
		name      string
		numFields int
	}{
		{name: "10 fields", numFields: 10},
		{name: "100 fields", numFields: 100},
		{name: "1000 fields", numFields: 1000},
		{name: "10000 fields", numFields: 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build JSON with many fields
			var buf bytes.Buffer
			buf.WriteString(`{"type":"system",`)
			for i := 0; i < tt.numFields; i++ {
				buf.WriteString(`"field`)
				buf.WriteString(string(rune('0' + i%10)))
				buf.WriteString(`":`)
				buf.WriteString(string(rune('0' + i%10)))
				if i < tt.numFields-1 {
					buf.WriteString(`,`)
				}
			}
			buf.WriteString(`}`)

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage with %s panicked: %v", tt.name, r)
				}
			}()

			_, err := ParseMessage(buf.Bytes())
			_ = err
		})
	}
}

// TestParseMessage_SpecialCharacters tests various special character sequences
func TestParseMessage_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "valid unicode escape",
			json: `{"type":"system","value":"\u0048\u0065\u006c\u006c\u006f"}`,
		},
		{
			name: "emoji characters",
			json: `{"type":"system","value":"😀🎉🚀❤️"}`,
		},
		{
			name: "mixed script",
			json: `{"type":"system","value":"Hello世界مرحبا"}`,
		},
		{
			name: "newline escapes",
			json: `{"type":"system","value":"line1\nline2\ttabred\bcarriage\rr"}`,
		},
		{
			name: "unicode surrogate pair",
			json: `{"type":"system","value":"\uD83D\uDE00"}`,
		},
		{
			name: "valid but unusual escapes",
			json: `{"type":"system","value":"\/\\\""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic and should parse successfully
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage(%q) panicked: %v", tt.json, r)
				}
			}()

			_, err := ParseMessage([]byte(tt.json))
			if err != nil {
				t.Errorf("ParseMessage(%q) unexpected error: %v", tt.json, err)
			}
		})
	}
}

// TestParseMessage_DuplicateKeys tests JSON with duplicate keys
func TestParseMessage_DuplicateKeys(t *testing.T) {
	// Go's JSON decoder uses the last value for duplicate keys
	jsonStr := `{"type":"system","type":"assistant","session_id":"test"}`

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ParseMessage with duplicate keys panicked: %v", r)
		}
	}()

	msg, err := ParseMessage([]byte(jsonStr))
	if err != nil {
		t.Errorf("ParseMessage with duplicate keys failed: %v", err)
	}
	// The second "type" value should win
	base, ok := msg.(*BaseMessage)
	if !ok {
		t.Fatal("expected BaseMessage")
	}
	// Go's json.Decoder uses the last value, so type should be "assistant"
	if base.Type != "assistant" {
		t.Logf("Note: duplicate key handling resulted in type=%s (Go uses last value)", base.Type)
	}
}

// TestParseMessage_NumericEdgeCases tests numeric edge cases
func TestParseMessage_NumericEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "very large integer",
			json: `{"type":"system","value":999999999999999999999}`,
		},
		{
			name: "negative zero",
			json: `{"type":"system","value":-0}`,
		},
		{
			name: "scientific notation large",
			json: `{"type":"system","value":1e308}`,
		},
		{
			name: "scientific notation small",
			json: `{"type":"system","value":1e-308}`,
		},
		{
			name: "infinity represented as string",
			json: `{"type":"system","value":"Infinity"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage(%q) panicked: %v", tt.json, r)
				}
			}()

			_, err := ParseMessage([]byte(tt.json))
			_ = err
		})
	}
}

// TestParseMessage_AllNilFields tests handling of messages with all nil/empty fields
func TestParseMessage_AllNilFields(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "all null optional fields",
			json: `{"type":"system","subtype":null,"session_id":null,"model":null,"tools":null}`,
		},
		{
			name: "empty objects",
			json: `{"type":"system","mcp_servers":{},"slash_commands":[]}`,
		},
		{
			name: "empty arrays",
			json: `{"type":"system","tools":[],"agents":[],"skills":[]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage(%q) panicked: %v", tt.json, r)
				}
			}()

			_, err := ParseMessage([]byte(tt.json))
			_ = err
		})
	}
}

// TestParseMessage_TypeSwitchUnknown tests that unknown message types are handled gracefully
func TestParseMessage_TypeSwitchUnknown(t *testing.T) {
	unknownTypes := []string{
		"future_message_type_v99",
		"unknown_event",
		"hypothetical_type",
		"test_type",
		"MESSAGE_TYPE",
		"message-variant",
		"random_xyz",
	}

	for _, typ := range unknownTypes {
		t.Run(typ, func(t *testing.T) {
			jsonStr := `{"type":"` + typ + `","value":"test"}`

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseMessage with unknown type %s panicked: %v", typ, r)
				}
			}()

			msg, err := ParseMessage([]byte(jsonStr))
			if err != nil {
				t.Errorf("ParseMessage with unknown type %s failed: %v", typ, err)
			}
			if msg == nil {
				t.Errorf("ParseMessage with unknown type %s returned nil", typ)
			}
			// Should return BaseMessage for unknown types
			base, ok := msg.(*BaseMessage)
			if !ok {
				t.Errorf("ParseMessage with unknown type %s should return *BaseMessage, got %T", typ, msg)
			} else if base.Type != typ {
				t.Errorf("BaseMessage.Type = %q, want %q", base.Type, typ)
			}
		})
	}
}
