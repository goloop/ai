package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateStrictSchema(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		wantErr error
		wantIn  string // substring the message must name, when it fails
	}{
		{
			name: "well formed",
			schema: `{"type":"object","additionalProperties":false,
			  "required":["a","b"],
			  "properties":{"a":{"type":"string"},"b":{"type":"integer"}}}`,
		},
		{
			name: "no properties at all",
			// Not every schema describes an object; a bare string schema has
			// nothing to be strict about.
			schema: `{"type":"string"}`,
		},
		{
			name: "missing additionalProperties",
			schema: `{"type":"object","required":["a"],
			  "properties":{"a":{"type":"string"}}}`,
			wantErr: ErrBadStrictSchema,
			wantIn:  "additionalProperties",
		},
		{
			name: "additionalProperties true",
			schema: `{"type":"object","additionalProperties":true,
			  "required":["a"],"properties":{"a":{"type":"string"}}}`,
			wantErr: ErrBadStrictSchema,
			wantIn:  "additionalProperties",
		},
		{
			name: "one property left out of required",
			schema: `{"type":"object","additionalProperties":false,
			  "required":["a"],
			  "properties":{"a":{"type":"string"},"placeLocal":{"type":"string"}}}`,
			wantErr: ErrBadStrictSchema,
			wantIn:  `"placeLocal"`,
		},
		{
			name: "required missing entirely",
			schema: `{"type":"object","additionalProperties":false,
			  "properties":{"a":{"type":"string"}}}`,
			wantErr: ErrBadStrictSchema,
			wantIn:  "required",
		},
		{
			name:    "not an object",
			schema:  `["a"]`,
			wantErr: ErrBadSchema,
		},
		{
			name:    "not JSON",
			schema:  `{`,
			wantErr: ErrBadSchema,
		},
		{
			name:    "empty",
			schema:  ``,
			wantErr: ErrNoSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStrictSchema(json.RawMessage(tt.schema))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateStrictSchema() = %v, want %v", err, tt.wantErr)
			}
			if tt.wantIn != "" && !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error %q does not name %q", err, tt.wantIn)
			}
		})
	}
}

// A mistake three levels down is the one worth catching, and the message has
// to say where it is: the whole point is not repeating the search by hand.
func TestValidateStrictSchemaReportsThePath(t *testing.T) {
	schema := `{
	  "type":"object","additionalProperties":false,"required":["stories"],
	  "properties":{
	    "stories":{"type":"array","items":{
	      "type":"object","additionalProperties":false,"required":["title"],
	      "properties":{"title":{"type":"string"},"placeLocal":{"type":"string"}}
	    }}
	  }}`

	err := ValidateStrictSchema(json.RawMessage(schema))
	if !errors.Is(err, ErrBadStrictSchema) {
		t.Fatalf("ValidateStrictSchema() = %v, want ErrBadStrictSchema", err)
	}
	if !strings.Contains(err.Error(), "properties.stories.items") {
		t.Errorf("error %q does not carry the path to the fault", err)
	}
	if !strings.Contains(err.Error(), `"placeLocal"`) {
		t.Errorf("error %q does not name the missing property", err)
	}
}

// Schemas hide inside more than properties, and a strict provider checks all
// of them.
func TestValidateStrictSchemaWalksEveryNest(t *testing.T) {
	bad := `{"type":"object","additionalProperties":false,
	  "properties":{"x":{"type":"string"}}}` // no required

	nests := map[string]string{
		"anyOf":       `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"anyOf":[` + bad + `]}}}`,
		"oneOf":       `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"oneOf":[` + bad + `]}}}`,
		"allOf":       `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"allOf":[` + bad + `]}}}`,
		"$defs":       `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"type":"string"}},"$defs":{"d":` + bad + `}}`,
		"definitions": `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"type":"string"}},"definitions":{"d":` + bad + `}}`,
		"items":       `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"type":"array","items":` + bad + `}}}`,
		"prefixItems": `{"type":"object","additionalProperties":false,"required":["a"],"properties":{"a":{"type":"array","prefixItems":[` + bad + `]}}}`,
	}

	for name, schema := range nests {
		t.Run(name, func(t *testing.T) {
			err := ValidateStrictSchema(json.RawMessage(schema))
			if !errors.Is(err, ErrBadStrictSchema) {
				t.Errorf("a fault under %s went unreported: %v", name, err)
			}
		})
	}
}

// The message must be the same every run: a schema with several faults is
// still one bug report, and a randomly ordered list cannot be asserted on.
func TestValidateStrictSchemaIsDeterministic(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,
	  "required":[],
	  "properties":{"c":{"type":"string"},"a":{"type":"string"},
	                "b":{"type":"string"}}}`)

	first := ValidateStrictSchema(schema).Error()
	for range 20 {
		if got := ValidateStrictSchema(schema).Error(); got != first {
			t.Fatalf("message varies between runs:\n%s\n%s", first, got)
		}
	}
	if !strings.Contains(first, `"a", "b", "c"`) {
		t.Errorf("missing properties are not listed in order: %s", first)
	}
}
