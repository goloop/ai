package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Usage reports how many tokens a request consumed.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
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

	// Format reports how the driver satisfied [Request.Format]: enforced by
	// the provider, asked for in the prompt, or not requested at all. An
	// emulated format is a request, not a guarantee, so a malformed reply is
	// worth reading differently depending on this. See [FormatMode].
	Format FormatMode

	// Hosted reports what became of each capability [Request.Hosted] asked
	// for, in request order. It is a list rather than one value because a
	// request may ask for several capabilities and they do not share a fate:
	// the model can search and skip running code in the same reply. A
	// request that asked for nothing gets no reports. See [HostedReport].
	Hosted []HostedReport
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

// JSON decodes the response text into v, which is what a request carrying a
// [Format] is asking for:
//
//	var seo SEO
//	if err := resp.JSON(&seo); err != nil { ... }
//
// The text must be one JSON value. A reply wrapped whole in a Markdown code
// fence is unwrapped first, because a provider that was only asked for JSON in
// the prompt tends to add one; anything else - prose around the value, a
// second value after it - is an error rather than something to go hunting
// through. Salvaging JSON out of arbitrary text is how a wrong answer gets
// read as a right one.
//
// JSON does not consult [Response.Format]: text that is JSON decodes whatever
// produced it.
func (r *Response) JSON(v any) error {
	text := strings.TrimSpace(r.Text())
	if text == "" {
		return ErrNoText
	}

	dec := json.NewDecoder(strings.NewReader(unfence(text)))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("ai: response is not JSON: %w", err)
	}

	// A well-formed value followed by anything at all means the reply was not
	// only the value, so it is not the answer that was asked for.
	var rest json.RawMessage
	if err := dec.Decode(&rest); !errors.Is(err, io.EOF) {
		return fmt.Errorf("ai: response has trailing content after the JSON value")
	}

	return nil
}

// fence marks a Markdown code block.
const fence = "```"

// unfence unwraps a string that is exactly one Markdown code block, dropping
// the info string ("json") on the opening line. Anything else is returned
// unchanged: a fence in the middle of prose is not a reply that happens to be
// wrapped, it is prose.
func unfence(s string) string {
	if len(s) < 2*len(fence) ||
		!strings.HasPrefix(s, fence) || !strings.HasSuffix(s, fence) {
		return s
	}

	body := s[len(fence) : len(s)-len(fence)]

	// A closing fence inside the body means there were several blocks, so the
	// string is not one wrapped value.
	if strings.Contains(body, fence) {
		return s
	}

	// The opening line may carry an info string ("json"); it is part of the
	// fence, not of the value. An info string labels what follows it, so a
	// first line with nothing after it was the value all along - that is how
	// a fenced scalar looks.
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		info := strings.TrimSpace(body[:nl])
		if strings.TrimSpace(body[nl+1:]) != "" &&
			!strings.ContainsAny(info, "{[\"") {
			body = body[nl+1:]
		}
	}

	return strings.TrimSpace(body)
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

// ToolCall returns the first tool call with the given name and whether one was
// found. It is a convenience for dispatching a single expected tool.
func (r *Response) ToolCall(name string) (ToolUse, bool) {
	for _, p := range r.Parts {
		if tu, ok := p.(ToolUse); ok && tu.Name == name {
			return tu, true
		}
	}
	return ToolUse{}, false
}

// Chunk is one increment of a streaming response from [Client.Stream]. Text is
// the incremental text delta; ToolCall is set when the chunk carries a
// completed tool call; Done marks the final chunk. Drivers set Usage on the
// Done chunk; its counts are zero when the provider did not report usage. Raw
// keeps the provider's original event JSON.
type Chunk struct {
	Text     string
	ToolCall *ToolUse
	Usage    *Usage
	Done     bool
	Raw      json.RawMessage

	// Citations carries the sources a hosted capability reported, as the
	// events announcing them arrive. It is a list because one event can name
	// several sources. A citation is delivered in the chunk the provider
	// reports it in, which is not necessarily the chunk carrying the text it
	// supports, so a caller assembling a document keeps its own list rather
	// than pairing them position by position. See [Citation].
	Citations []Citation

	// Hosted reports what each requested capability did. Drivers set it on
	// the Done chunk, where the counts are finally known, so it mirrors
	// [Response.Hosted] for a stream. See [HostedReport].
	Hosted []HostedReport
}
