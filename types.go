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
	Type           string   `json:"type"`
	Subtype        string   `json:"subtype"`
	SessionID      string   `json:"session_id"`
	Model          string   `json:"model,omitempty"`
	CwdPath        string   `json:"cwd,omitempty"`
	Tools          []string `json:"tools,omitempty"`
	McpServers     []interface{} `json:"mcp_servers,omitempty"`
	PermissionMode string   `json:"permission_mode,omitempty"`
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

// StreamEventWrapper wraps a stream event from the Claude CLI
type StreamEventWrapper struct {
	Type      string       `json:"type"`
	Event     *StreamEvent `json:"event"`
	SessionID string       `json:"session_id,omitempty"`
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

// StreamState manages partial/streaming data during message processing
type StreamState struct {
	PartialText     string            `json:"partial_text"`
	PartialThinking string            `json:"partial_thinking"`
	PartialToolInput map[string]string `json:"partial_tool_input"` // tool ID -> partial JSON
	CurrentIndex    int               `json:"current_index"`      // Current content block index
}

// NewStreamState creates a new StreamState
func NewStreamState() *StreamState {
	return &StreamState{
		PartialToolInput: make(map[string]string),
	}
}

// Reset clears all streaming state
func (s *StreamState) Reset() {
	s.PartialText = ""
	s.PartialThinking = ""
	s.PartialToolInput = make(map[string]string)
	s.CurrentIndex = 0
}

// AppState manages the complete application state including agent hierarchy
type AppState struct {
	// Tool call tracking
	PendingTools map[string]*ToolCall `json:"pending_tools"` // tool ID -> ToolCall

	// Agent hierarchy
	RootAgent    *AgentState `json:"root_agent"`    // Root agent (main)
	CurrentAgent *AgentState `json:"current_agent"` // Currently active agent
	AgentsByID   map[string]*AgentState `json:"agents_by_id"` // Quick lookup by tool use ID

	// Token tracking
	TotalTokens *TotalUsage `json:"total_tokens"`

	// Streaming state
	Stream *StreamState `json:"stream"`

	// Session info
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

// NewAppState creates a new application state
func NewAppState() *AppState {
	return &AppState{
		PendingTools: make(map[string]*ToolCall),
		AgentsByID:   make(map[string]*AgentState),
		TotalTokens: &TotalUsage{},
		Stream:      NewStreamState(),
	}
}

// InitializeSession initializes the session with system info
func (a *AppState) InitializeSession(sysInit *SystemInit) {
	a.SessionID = sysInit.SessionID
	a.Model = sysInit.Model

	// Create root agent
	a.RootAgent = &AgentState{
		ID:        "main",
		Type:      "main",
		Status:    AgentStatusIdle,
		ToolCalls: make([]ToolCall, 0),
		Children:  make([]AgentState, 0),
		Depth:     0,
	}
	a.CurrentAgent = a.RootAgent
	a.AgentsByID["main"] = a.RootAgent
}

// AddOrUpdateToolCall adds or updates a tool call
func (a *AppState) AddOrUpdateToolCall(toolCall *ToolCall) {
	a.PendingTools[toolCall.ID] = toolCall

	// Add to current agent
	if a.CurrentAgent != nil {
		// Check if tool call already exists in agent
		found := false
		for i, tc := range a.CurrentAgent.ToolCalls {
			if tc.ID == toolCall.ID {
				a.CurrentAgent.ToolCalls[i] = *toolCall
				found = true
				break
			}
		}
		if !found {
			a.CurrentAgent.ToolCalls = append(a.CurrentAgent.ToolCalls, *toolCall)
		}
	}
}

// CompleteToolCall marks a tool call as completed
func (a *AppState) CompleteToolCall(toolID string, result string, isError bool) {
	if tc, ok := a.PendingTools[toolID]; ok {
		if isError {
			tc.Status = ToolCallStatusFailed
		} else {
			tc.Status = ToolCallStatusCompleted
		}
		tc.Result = result
		tc.IsError = isError

		// Update in current agent's tool calls
		if a.CurrentAgent != nil {
			for i, t := range a.CurrentAgent.ToolCalls {
				if t.ID == toolID {
					a.CurrentAgent.ToolCalls[i] = *tc
					break
				}
			}
		}
	}
}

// CreateChildAgent creates a new child agent from a Task tool call
func (a *AppState) CreateChildAgent(parentToolID string, agentType string, description string) *AgentState {
	parent := a.CurrentAgent
	if parent == nil {
		parent = a.RootAgent
	}

	child := &AgentState{
		ID:          parentToolID,
		Type:        agentType,
		Description: description,
		Status:      AgentStatusRunning,
		ParentID:    parent.ID,
		ToolCalls:   make([]ToolCall, 0),
		Children:    make([]AgentState, 0),
		Depth:       parent.Depth + 1,
	}

	// Add to parent's children
	parent.Children = append(parent.Children, *child)

	// Register in lookup map
	a.AgentsByID[child.ID] = child

	return child
}

// SetCurrentAgent sets the currently active agent
func (a *AppState) SetCurrentAgent(agentID string) {
	if agent, ok := a.AgentsByID[agentID]; ok {
		a.CurrentAgent = agent
		a.CurrentAgent.Status = AgentStatusRunning
	}
}

// UpdateTokens updates the total token usage
func (a *AppState) UpdateTokens(usage *Usage) {
	if usage == nil {
		return
	}

	a.TotalTokens.InputTokens += usage.InputTokens
	a.TotalTokens.OutputTokens += usage.OutputTokens
	a.TotalTokens.CacheCreationInputTokens += usage.CacheCreationInputTokens
	a.TotalTokens.CacheReadInputTokens += usage.CacheReadInputTokens
	a.TotalTokens.TotalTokens = a.TotalTokens.InputTokens + a.TotalTokens.OutputTokens
}

// AppendStreamText appends text to the current streaming state
func (a *AppState) AppendStreamText(text string) {
	a.Stream.PartialText += text
}

// AppendStreamThinking appends thinking to the current streaming state
func (a *AppState) AppendStreamThinking(thinking string) {
	a.Stream.PartialThinking += thinking
}

// AppendStreamToolInput appends partial tool input JSON
func (a *AppState) AppendStreamToolInput(toolID string, partialJSON string) {
	if current, ok := a.Stream.PartialToolInput[toolID]; ok {
		a.Stream.PartialToolInput[toolID] = current + partialJSON
	} else {
		a.Stream.PartialToolInput[toolID] = partialJSON
	}
}

// GetStreamToolInput gets the current partial tool input for a tool
func (a *AppState) GetStreamToolInput(toolID string) string {
	if input, ok := a.Stream.PartialToolInput[toolID]; ok {
		return input
	}
	return ""
}

// ClearStreamState resets the streaming state after a complete message
func (a *AppState) ClearStreamState() {
	a.Stream.Reset()
}

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

	case "stream_event":
		var wrapper StreamEventWrapper
		if err := json.Unmarshal(data, &wrapper); err != nil {
			return nil, err
		}
		if wrapper.Event != nil {
			return wrapper.Event, nil
		}
		return &wrapper, nil

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
