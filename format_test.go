package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestFormatValidate(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)

	cases := []struct {
		name string
		f    *Format
		want error
	}{
		{"nil asks for nothing", nil, nil},
		{"text asks for nothing", &Format{}, nil},
		{"json needs no schema", &Format{Type: FormatJSON}, nil},
		{"json ignores a schema", &Format{Type: FormatJSON, Schema: schema}, nil},
		{"json_schema with a schema", &Format{Type: FormatJSONSchema, Schema: schema}, nil},
		{"json_schema without one", &Format{Type: FormatJSONSchema}, ErrNoSchema},
		{
			"json_schema with broken JSON",
			&Format{Type: FormatJSONSchema, Schema: json.RawMessage(`{"type":`)},
			ErrBadSchema,
		},
		{
			"json_schema with leading space",
			&Format{Type: FormatJSONSchema, Schema: json.RawMessage("  \n{}")},
			nil,
		},
		// A schema is an object. Anything else is a caller who passed the
		// wrong value, and every provider would reject it downstream.
		{
			"json_schema given an array",
			&Format{Type: FormatJSONSchema, Schema: json.RawMessage(`[{"a":1}]`)},
			ErrBadSchema,
		},
		{
			"json_schema given a literal",
			&Format{Type: FormatJSONSchema, Schema: json.RawMessage(`null`)},
			ErrBadSchema,
		},
		{
			"json_schema given a string",
			&Format{Type: FormatJSONSchema, Schema: json.RawMessage(`"{}"`)},
			ErrBadSchema,
		},
		// An unknown type must not be guessed at: nine drivers switch over it.
		{"unknown type", &Format{Type: FormatType(99)}, ErrBadFormat},
		{"negative type", &Format{Type: FormatType(-1)}, ErrBadFormat},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.f.Validate(); !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestRequestValidateChecksFormat makes sure a bad format is caught before a
// driver builds a provider payload out of it.
func TestRequestValidateChecksFormat(t *testing.T) {
	req := &Request{
		Model:    "m",
		Messages: []Message{{Role: RoleUser, Parts: []Part{Text{Text: "hi"}}}},
		Format:   &Format{Type: FormatJSONSchema},
	}
	if err := req.Validate(); !errors.Is(err, ErrNoSchema) {
		t.Errorf("Validate() = %v, want %v", err, ErrNoSchema)
	}

	req.Format = nil
	if err := req.Validate(); err != nil {
		t.Errorf("a request without a format must stay valid: %v", err)
	}
}

func TestSchemaName(t *testing.T) {
	var nilFormat *Format
	cases := map[string]struct {
		f    *Format
		want string
	}{
		"nil":   {nilFormat, DefaultSchemaName},
		"empty": {&Format{Type: FormatJSONSchema}, DefaultSchemaName},
		"set":   {&Format{Type: FormatJSONSchema, Name: "seo"}, "seo"},
	}
	for name, c := range cases {
		if got := c.f.SchemaName(); got != c.want {
			t.Errorf("%s: SchemaName() = %q, want %q", name, got, c.want)
		}
	}
}

// TestInstruction pins the wording every driver without native support hands
// to its provider. All nine must ask for the same thing in the same words.
func TestInstruction(t *testing.T) {
	if got := (*Format)(nil).Instruction(); got != "" {
		t.Errorf("nil format instruction = %q, want empty", got)
	}
	if got := (&Format{}).Instruction(); got != "" {
		t.Errorf("text format instruction = %q, want empty", got)
	}
	if got := (&Format{Type: FormatType(99)}).Instruction(); got != "" {
		t.Errorf("unknown format instruction = %q, want empty", got)
	}

	plain := (&Format{Type: FormatJSON}).Instruction()
	for _, want := range []string{"single JSON value", "code fence"} {
		if !strings.Contains(plain, want) {
			t.Errorf("instruction does not mention %q: %s", want, plain)
		}
	}
	if strings.Contains(plain, "Schema") {
		t.Errorf("a schemaless format must not mention a schema: %s", plain)
	}

	schema := `{"type":"object","properties":{"a":{"type":"string"}}}`
	withSchema := (&Format{
		Type:   FormatJSONSchema,
		Schema: json.RawMessage(schema),
	}).Instruction()
	if !strings.Contains(withSchema, schema) {
		t.Errorf("instruction does not carry the schema: %s", withSchema)
	}
}

func TestAppendInstruction(t *testing.T) {
	f := &Format{Type: FormatJSON}

	if got := f.AppendInstruction(""); got != f.Instruction() {
		t.Errorf("empty system prompt should yield the instruction alone: %q", got)
	}

	got := f.AppendInstruction("You are terse.")
	if !strings.HasPrefix(got, "You are terse.\n\n") {
		t.Errorf("existing system prompt was not kept first: %q", got)
	}
	if !strings.HasSuffix(got, f.Instruction()) {
		t.Errorf("instruction was not appended: %q", got)
	}

	// A format that asks for nothing must leave the prompt exactly as it was.
	for _, f := range []*Format{nil, {}} {
		if got := f.AppendInstruction("You are terse."); got != "You are terse." {
			t.Errorf("system prompt was changed by a no-op format: %q", got)
		}
	}
}

func TestFormatStrings(t *testing.T) {
	types := map[FormatType]string{
		FormatText: "text", FormatJSON: "json", FormatJSONSchema: "json_schema",
		FormatType(42): "text",
	}
	for tp, want := range types {
		if got := tp.String(); got != want {
			t.Errorf("FormatType(%d).String() = %q, want %q", tp, got, want)
		}
	}

	modes := map[FormatMode]string{
		FormatNone: "none", FormatNative: "native", FormatEmulated: "emulated",
		FormatMode(42): "none",
	}
	for m, want := range modes {
		if got := m.String(); got != want {
			t.Errorf("FormatMode(%d).String() = %q, want %q", m, got, want)
		}
	}
}
