package ai

// Request is a provider-agnostic generation request. Only Model and Messages
// are required; the remaining fields are applied when set. Temperature and
// TopP are pointers so that "unset" is distinct from an explicit zero.
type Request struct {
	Model       string
	System      string // optional system prompt
	Messages    []Message
	Tools       []Tool
	ToolChoice  ToolChoice
	MaxTokens   int
	Temperature *float64
	TopP        *float64
	Stop        []string
}

// Validate reports whether the request has the minimum a provider needs.
func (r *Request) Validate() error {
	if r.Model == "" {
		return ErrNoModel
	}
	if len(r.Messages) == 0 {
		return ErrNoMessages
	}
	return nil
}
