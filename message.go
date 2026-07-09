package ai

import "encoding/json"

// Part is a single piece of a Message's content. The concrete part types are
// Text, Image, ToolUse and ToolResult. The set is closed: Part cannot be
// implemented outside this package, so drivers can switch over it exhaustively.
type Part interface {
	isPart()
}

// Text is a plain-text content part.
type Text struct {
	Text string
}

func (Text) isPart() {}

// Image is an image content part. Provide either inline Data with its MIME
// type, or a URL when the provider supports fetching remote images. Drivers
// encode Data as base64 as required by their wire format.
type Image struct {
	MIME string // for example "image/png" or "image/jpeg"
	Data []byte // inline image bytes, or nil when URL is set
	URL  string // remote image URL, or "" when Data is set
}

func (Image) isPart() {}

// ToolUse is a request from the assistant to call a tool. Input is the raw
// JSON arguments object produced by the model, validated against the matching
// [Tool] schema by the caller.
type ToolUse struct {
	ID    string          // provider-assigned call identifier
	Name  string          // name of the tool to invoke
	Input json.RawMessage // arguments as a JSON object
}

func (ToolUse) isPart() {}

// ToolResult carries the result of a tool call back to the model. ID must
// match the [ToolUse] it answers. Set IsError to report that the tool failed.
type ToolResult struct {
	ID      string
	Content string
	IsError bool
}

func (ToolResult) isPart() {}

// Message is one turn in a conversation: a role and its content parts.
type Message struct {
	Role  Role
	Parts []Part
}

// UserText returns a user Message containing a single text part.
func UserText(s string) Message {
	return Message{Role: RoleUser, Parts: []Part{Text{Text: s}}}
}

// AssistantText returns an assistant Message containing a single text part.
func AssistantText(s string) Message {
	return Message{Role: RoleAssistant, Parts: []Part{Text{Text: s}}}
}

// SystemText returns a system Message containing a single text part. Drivers
// fold system messages into the provider's system prompt; the Request.System
// field is an equivalent shorthand for a single instruction.
func SystemText(s string) Message {
	return Message{Role: RoleSystem, Parts: []Part{Text{Text: s}}}
}
