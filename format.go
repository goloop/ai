package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatType selects the shape the model must produce.
type FormatType int

// The response shapes. FormatText is the zero value and asks for nothing, so a
// Request without a Format behaves exactly as before.
const (
	FormatText       FormatType = iota // free-form text, the provider default
	FormatJSON                         // a single valid JSON value, any shape
	FormatJSONSchema                   // JSON matching Format.Schema
)

// String renders the type for diagnostics.
func (t FormatType) String() string {
	switch t {
	case FormatJSON:
		return "json"
	case FormatJSONSchema:
		return "json_schema"
	default:
		return "text"
	}
}

// Format asks for a structured response. It is provider-agnostic: a driver
// maps it onto whatever its provider offers natively, and asks for it in the
// prompt when the provider offers nothing. Either way a driver never ignores
// it silently - see [FormatMode].
//
//	req.Format = &ai.Format{
//		Type:   ai.FormatJSONSchema,
//		Name:   "seo",
//		Schema: schema, // a JSON Schema object
//	}
//
// Nil means no structured output was requested, which is the default.
type Format struct {
	// Type is the shape wanted. The zero value, FormatText, asks for nothing.
	Type FormatType

	// Name labels the schema. Some providers require a name; drivers that do
	// substitute DefaultSchemaName when this is empty. It has no effect for
	// FormatJSON.
	Name string

	// Schema is a JSON Schema object describing the wanted value. It is
	// required for FormatJSONSchema and ignored otherwise.
	Schema json.RawMessage

	// Strict asks the provider to enforce the schema exactly rather than
	// treat it as guidance. Providers that cannot do this ignore the flag;
	// it never changes whether the request is accepted.
	Strict bool
}

// DefaultSchemaName is the schema name drivers use when a Format that needs
// one leaves Name empty.
const DefaultSchemaName = "response"

// SchemaName returns Name, or DefaultSchemaName when Name is empty.
func (f *Format) SchemaName() string {
	if f == nil || f.Name == "" {
		return DefaultSchemaName
	}
	return f.Name
}

// Validate reports whether the format is one a driver can act on. A nil
// Format is valid: it asks for nothing.
//
// An unknown Type is rejected rather than treated as one of the known ones.
// Nine drivers switch over this value, and a type none of them agrees on would
// mean nine different guesses about what the caller wanted.
func (f *Format) Validate() error {
	if f == nil {
		return nil
	}

	switch f.Type {
	case FormatText, FormatJSON:
		return nil
	case FormatJSONSchema:
		if len(f.Schema) == 0 {
			return ErrNoSchema
		}
		if !isJSONObject(f.Schema) {
			return ErrBadSchema
		}
		return nil
	default:
		return ErrBadFormat
	}
}

// isJSONObject reports whether b is a valid JSON object. A JSON Schema is an
// object; providers reject anything else, and an array or a bare literal here
// is a caller who passed the wrong value rather than a schema a driver could
// forward.
func isJSONObject(b []byte) bool {
	if !json.Valid(b) {
		return false
	}
	for _, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return c == '{'
		}
	}
	return false
}

// Instruction returns the wording a driver puts in the prompt when its
// provider cannot enforce the format itself. It lives here so that all nine
// drivers ask for the same thing in the same words; a driver with native
// support never calls it.
//
// It returns an empty string when nothing was asked for.
func (f *Format) Instruction() string {
	if f == nil {
		return ""
	}
	switch f.Type {
	case FormatJSON, FormatJSONSchema:
	default:
		// Including an unknown type: Validate rejects it, and guessing here
		// would put words in the caller's mouth.
		return ""
	}

	var b strings.Builder
	b.WriteString("Reply with a single JSON value and nothing else. ")
	b.WriteString("Do not wrap it in a code fence, and do not add any ")
	b.WriteString("explanation before or after it.")

	if f.Type == FormatJSONSchema && len(f.Schema) > 0 {
		b.WriteString("\n\nThe value must match this JSON Schema:\n")
		b.Write(f.Schema)
	}

	return b.String()
}

// AppendInstruction returns system with Instruction appended after a blank
// line, which is how a driver folds the request into an existing system
// prompt. An empty system prompt yields the instruction alone, and a Format
// that asks for nothing leaves system untouched.
func (f *Format) AppendInstruction(system string) string {
	instruction := f.Instruction()
	if instruction == "" {
		return system
	}
	if system == "" {
		return instruction
	}
	return system + "\n\n" + instruction
}

// FormatMode says how a driver satisfied the Format of a request. It is
// reported on [Response] so that a caller can tell an enforced answer from a
// requested one - a provider that was merely asked nicely can still reply with
// prose, and that is worth knowing when output turns out malformed.
type FormatMode int

// How a format was satisfied.
const (
	FormatNone     FormatMode = iota // no format was requested
	FormatNative                     // the provider enforced it
	FormatEmulated                   // the driver asked for it in the prompt
)

// String renders the mode for diagnostics.
func (m FormatMode) String() string {
	switch m {
	case FormatNative:
		return "native"
	case FormatEmulated:
		return "emulated"
	default:
		return "none"
	}
}

// parseFormatMode is the inverse of String, used when a stored Response is
// read back. An unrecognized name is an error rather than FormatNone, because
// FormatNone claims no format was requested and that would turn an unreadable
// value into a confident lie.
func parseFormatMode(s string) (FormatMode, error) {
	switch s {
	case "", "none":
		return FormatNone, nil
	case "native":
		return FormatNative, nil
	case "emulated":
		return FormatEmulated, nil
	default:
		return FormatNone, fmt.Errorf("ai: unknown format mode %q", s)
	}
}
