package ai

import (
	"encoding/json"
	"strings"
)

// Usage reports how many tokens a request consumed.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response is the result of a non-streaming [Client.Generate] call. Parts holds
// the assistant's output blocks (text and any tool calls); Raw keeps the
// provider's original JSON for access to fields this package does not model.
type Response struct {
	Model      string
	Parts      []Part
	StopReason string
	Usage      Usage
	Raw        json.RawMessage
}

// Text returns the concatenation of all text parts in the response.
func (r *Response) Text() string {
	var b strings.Builder
	for _, p := range r.Parts {
		if t, ok := p.(Text); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

// ToolCalls returns the tool-call parts the model produced, in order.
func (r *Response) ToolCalls() []ToolUse {
	var out []ToolUse
	for _, p := range r.Parts {
		if tu, ok := p.(ToolUse); ok {
			out = append(out, tu)
		}
	}
	return out
}

// Chunk is one increment of a streaming response from [Client.Stream]. Text is
// the incremental text delta; ToolCall is set when the chunk carries part of a
// tool call; Usage is set on the final chunk when the provider reports it; Done
// marks the last chunk. Raw keeps the provider's original event JSON.
type Chunk struct {
	Text     string
	ToolCall *ToolUse
	Usage    *Usage
	Done     bool
	Raw      json.RawMessage
}
