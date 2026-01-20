package main

import (
	"encoding/json"
)

// MessageType represents the type of SDK message
type MessageType string

const (
	MessageTypeSystemInit      MessageType = "system"
	MessageTypeAssistant       MessageType = "assistant"
	MessageTypeUser            MessageType = "user"
	MessageTypeResult          MessageType = "result"
	MessageTypeCompactBoundary MessageType = "compact_boundary"
)

// StreamEventType represents the type of streaming event
type StreamEventType string

const (
	StreamEventMessageStart      StreamEventType = "message_start"
	StreamEventContentBlockStart StreamEventType = "content_block_start"
	StreamEventContentBlockDelta StreamEventType = "content_block_delta"
	StreamEventContentBlockStop  StreamEventType = "content_block_stop"
	StreamEventMessageDelta      StreamEventType = "message_delta"
	StreamEventMessageStop       StreamEventType = "message_stop"
)

// ContentBlockType represents the type of content block
type ContentBlockType string

const (
	ContentBlockTypeText        ContentBlockType = "text"
	ContentBlockTypeToolUse     ContentBlockType = "tool_use"
	ContentBlockTypeToolResult  ContentBlockType = "tool_result"
	ContentBlockTypeThinking    ContentBlockType = "thinking"
	ContentBlockTypeRedactedThinking ContentBlockType = "redacted_thinking"
)

// BaseMessage is the common structure for all SDK messages
type BaseMessage struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Raw       json.RawMessage `json:"-"` // Store the original JSON for later parsing
}

// SystemInit represents the initial system message
type SystemInit struct {
	Type         string `json:"type"`
	Subtype      string `json:"subtype"`
	SessionID    string `json:"session_id"`
	Model        string `json:"model,omitempty"`
	CwdPath      string `json:"cwd,omitempty"`
	Tools        []Tool `json:"tools,omitempty"`
	McpServers   []string `json:"mcp_servers,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
}

// Tool represents a tool available to Claude
type Tool struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// AssistantMessage represents an assistant response message
type AssistantMessage struct {
	Type         string         `json:"type"`
	Message      MessageContent `json:"message"`
	SessionID    string         `json:"session_id,omitempty"`
	CostUSD      float64        `json:"cost_usd,omitempty"`
	DurationMS   int64          `json:"duration_ms,omitempty"`
	IsError      bool           `json:"is_error,omitempty"`
}

// MessageContent represents the content of a message
type MessageContent struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model,omitempty"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        *Usage         `json:"usage,omitempty"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ContentBlock represents a block of content (text, tool_use, thinking, etc.)
type ContentBlock struct {
	Type ContentBlockType `json:"type"`

	// Text content fields
	Text string `json:"text,omitempty"`

	// Thinking content fields
	Thinking string `json:"thinking,omitempty"`

	// Tool use fields
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Tool result fields
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// StreamEvent represents a streaming event from the API
type StreamEvent struct {
	Type         StreamEventType `json:"type"`
	Index        int             `json:"index,omitempty"`
	Message      *MessageContent `json:"message,omitempty"`
	ContentBlock *ContentBlock   `json:"content_block,omitempty"`
	Delta        *Delta          `json:"delta,omitempty"`
	Usage        *Usage          `json:"usage,omitempty"`
}

// Delta represents incremental content in a stream event
type Delta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	Thinking     string `json:"thinking,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// Result represents the final result message
type Result struct {
	Type       string          `json:"type"`
	Subtype    string          `json:"subtype"`
	Result     string          `json:"result,omitempty"`
	IsError    bool            `json:"is_error"`
	TotalCost  float64         `json:"total_cost_usd,omitempty"`
	TotalDuration int64        `json:"total_duration_ms,omitempty"`
	SessionID  string          `json:"session_id,omitempty"`
	NumTurns   int             `json:"num_turns,omitempty"`
	Usage      *TotalUsage     `json:"usage,omitempty"`
}

// TotalUsage represents cumulative token usage
type TotalUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	TotalTokens              int `json:"total_tokens,omitempty"`
}

// CompactBoundary represents a boundary marker in the stream
type CompactBoundary struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id,omitempty"`
}

// ToolCall represents a tool invocation with its state
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	Status    ToolCallStatus  `json:"status"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	StartTime int64           `json:"start_time,omitempty"`
	EndTime   int64           `json:"end_time,omitempty"`
}

// ToolCallStatus represents the current state of a tool call
type ToolCallStatus string

const (
	ToolCallStatusPending    ToolCallStatus = "pending"
	ToolCallStatusRunning    ToolCallStatus = "running"
	ToolCallStatusCompleted  ToolCallStatus = "completed"
	ToolCallStatusFailed     ToolCallStatus = "failed"
)

// AgentState represents the current state of an agent in the hierarchy
type AgentState struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"` // e.g., "main", "task", "explore"
	Description string       `json:"description,omitempty"`
	Status      AgentStatus  `json:"status"`
	ParentID    string       `json:"parent_id,omitempty"`
	Children    []AgentState `json:"children,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	Depth       int          `json:"depth"`
}

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusThinking  AgentStatus = "thinking"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusFailed    AgentStatus = "failed"
)

// ParseMessage attempts to parse a JSON message and return the appropriate type
func ParseMessage(data []byte) (interface{}, error) {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}
	base.Raw = data

	switch base.Type {
	case "system":
		var msg SystemInit
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case "assistant":
		var msg AssistantMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case "result":
		var msg Result
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case "compact_boundary":
		var msg CompactBoundary
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	case string(StreamEventMessageStart),
		string(StreamEventContentBlockStart),
		string(StreamEventContentBlockDelta),
		string(StreamEventContentBlockStop),
		string(StreamEventMessageDelta),
		string(StreamEventMessageStop):
		var msg StreamEvent
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return &msg, nil

	default:
		// Return raw message for unknown types
		return &base, nil
	}
}
