package ai

import (
	"encoding/json"
	"fmt"
)

// This file gives Message and Response a stable, provider-neutral JSON encoding
// so a Request or Response can be persisted, queued or cached and decoded back.
// The closed Part interface cannot be unmarshaled directly, so each part is
// tagged with a "type" discriminator and reconstructed into its concrete type.

// partWire is the on-the-wire form of a Part. Only the fields relevant to the
// discriminated type are populated; the rest are omitted.
type partWire struct {
	Type    string          `json:"type"`
	Text    string          `json:"text,omitempty"`
	MIME    string          `json:"media_type,omitempty"`
	Data    []byte          `json:"data,omitempty"` // base64 in JSON
	URL     string          `json:"url,omitempty"`
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"` // raw JSON, not a string
	ToolID  string          `json:"tool_use_id,omitempty"`
	Content string          `json:"content,omitempty"`
	IsError bool            `json:"is_error,omitempty"`
}

// partToWire converts a concrete Part into its wire form. An unknown part type
// is an error rather than a silently dropped or mis-encoded value.
func partToWire(p Part) (partWire, error) {
	switch v := p.(type) {
	case Text:
		return partWire{Type: "text", Text: v.Text}, nil
	case Image:
		return partWire{Type: "image", MIME: v.MIME, Data: v.Data, URL: v.URL}, nil
	case ToolUse:
		return partWire{Type: "tool_use", ID: v.ID, Name: v.Name, Input: v.Input}, nil
	case ToolResult:
		return partWire{Type: "tool_result", ToolID: v.ID, Content: v.Content, IsError: v.IsError}, nil
	default:
		return partWire{}, fmt.Errorf("ai: cannot marshal unknown part type %T", p)
	}
}

// partFromWire reconstructs a concrete Part from its wire form. An unknown or
// empty type is an error rather than a dropped part.
func partFromWire(w partWire) (Part, error) {
	switch w.Type {
	case "text":
		return Text{Text: w.Text}, nil
	case "image":
		return Image{MIME: w.MIME, Data: w.Data, URL: w.URL}, nil
	case "tool_use":
		return ToolUse{ID: w.ID, Name: w.Name, Input: w.Input}, nil
	case "tool_result":
		return ToolResult{ID: w.ToolID, Content: w.Content, IsError: w.IsError}, nil
	default:
		return nil, fmt.Errorf("ai: unknown part type %q", w.Type)
	}
}

// partsToWire converts a slice of parts, stopping at the first unknown type.
func partsToWire(parts []Part) ([]partWire, error) {
	out := make([]partWire, len(parts))
	for i, p := range parts {
		w, err := partToWire(p)
		if err != nil {
			return nil, err
		}
		out[i] = w
	}
	return out, nil
}

// partsFromWire reconstructs a slice of parts, stopping at the first unknown
// type.
func partsFromWire(wires []partWire) ([]Part, error) {
	out := make([]Part, len(wires))
	for i, w := range wires {
		p, err := partFromWire(w)
		if err != nil {
			return nil, err
		}
		out[i] = p
	}
	return out, nil
}

// messageWire is the on-the-wire form of a Message.
type messageWire struct {
	Role  Role       `json:"role"`
	Parts []partWire `json:"parts,omitempty"`
}

// MarshalJSON encodes the message with each part tagged by its "type".
func (m Message) MarshalJSON() ([]byte, error) {
	parts, err := partsToWire(m.Parts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(messageWire{Role: m.Role, Parts: parts})
}

// UnmarshalJSON reconstructs the message and its concrete part types.
func (m *Message) UnmarshalJSON(data []byte) error {
	var w messageWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	parts, err := partsFromWire(w.Parts)
	if err != nil {
		return err
	}
	m.Role = w.Role
	m.Parts = parts
	return nil
}

// responseWire is the on-the-wire form of a Response.
type responseWire struct {
	Model      string          `json:"model,omitempty"`
	Parts      []partWire      `json:"parts,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
	Usage      Usage           `json:"usage"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// MarshalJSON encodes the response with each output part tagged by its "type".
func (r Response) MarshalJSON() ([]byte, error) {
	parts, err := partsToWire(r.Parts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(responseWire{
		Model:      r.Model,
		Parts:      parts,
		StopReason: r.StopReason,
		Usage:      r.Usage,
		Raw:        r.Raw,
	})
}

// UnmarshalJSON reconstructs the response and its concrete part types.
func (r *Response) UnmarshalJSON(data []byte) error {
	var w responseWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	parts, err := partsFromWire(w.Parts)
	if err != nil {
		return err
	}
	r.Model = w.Model
	r.Parts = parts
	r.StopReason = w.StopReason
	r.Usage = w.Usage
	r.Raw = w.Raw
	return nil
}
