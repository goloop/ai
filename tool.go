package ai

import "encoding/json"

// Tool describes a function the model may call. Schema is a JSON Schema object
// describing the tool's input; drivers pass it through to the provider in the
// shape that provider expects.
type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

// ToolChoice controls whether and how the model may call tools in a Request.
type ToolChoice int

// The tool-calling strategies. ToolAuto lets the model decide, ToolNone forbids
// tool calls, and ToolRequired forces the model to call at least one tool.
const (
	ToolAuto ToolChoice = iota
	ToolNone
	ToolRequired
)
