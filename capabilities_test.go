package ai

import (
	"context"
	"iter"
	"testing"
)

// plainClient describes nothing, which is what every driver written before
// Capable existed does.
type plainClient struct{}

func (plainClient) Generate(context.Context, *Request) (*Response, error) {
	return nil, nil
}
func (plainClient) Stream(context.Context, *Request) iter.Seq2[Chunk, error] {
	return func(func(Chunk, error) bool) {}
}

// searchClient describes a provider that runs web search, takes a use limit
// and an allow list but no block list, cannot be forced, and keeps a schema in
// the same call only by asking the model for it.
type searchClient struct{ plainClient }

func (searchClient) Capabilities() Capabilities {
	return Capabilities{
		Hosted: map[HostedKind]HostedCapability{
			HostedWebSearch: {
				Web: &HostedWebCapability{
					MaxUses:      true,
					AllowDomains: true,
				},
				WithFormat: FormatCapability{
					JSON:       FormatEmulated,
					JSONSchema: FormatEmulated,
				},
			},
		},
		Format: FormatCapability{
			JSON:       FormatNative,
			JSONSchema: FormatNative,
			Strict:     FormatNative,
		},
	}
}

// A driver that says nothing must not be mistaken for a driver that says no,
// and must not make anyone's code panic on a nil map.
func TestCapabilitiesOfSilentDriver(t *testing.T) {
	caps := CapabilitiesOf(plainClient{})
	if len(caps.Hosted) != 0 {
		t.Errorf("Hosted = %+v, want empty", caps.Hosted)
	}
	if caps.Format.JSON != FormatNone {
		t.Errorf("Format.JSON = %v, want none", caps.Format.JSON)
	}

	if _, ok := HostedCapabilityOf(plainClient{}, HostedWebSearch); ok {
		t.Error("a silent driver claimed a capability")
	}
	if SupportsHosted(plainClient{}, Hosted{Kind: HostedWebSearch}) {
		t.Error("SupportsHosted said yes for a driver that describes nothing")
	}
}

func TestSupportsHostedChecksConstraints(t *testing.T) {
	c := searchClient{}

	tests := []struct {
		name string
		want bool
		in   Hosted
	}{
		{"plain search", true, Hosted{Kind: HostedWebSearch}},
		{
			"a limit it accepts", true,
			Hosted{Kind: HostedWebSearch, Web: &HostedWeb{MaxUses: 3}},
		},
		{
			"an allow list it accepts", true,
			Hosted{Kind: HostedWebSearch, Web: &HostedWeb{
				AllowDomains: []string{"example.org"},
			}},
		},
		{
			"a block list it does not", false,
			Hosted{Kind: HostedWebSearch, Web: &HostedWeb{
				BlockDomains: []string{"example.org"},
			}},
		},
		{
			"a region it does not", false,
			Hosted{Kind: HostedWebSearch, Web: &HostedWeb{Region: "UA"}},
		},
		{
			"both lists at once", false,
			Hosted{Kind: HostedWebSearch, Web: &HostedWeb{
				AllowDomains: []string{"a.example"},
				BlockDomains: []string{"b.example"},
			}},
		},
		{
			"a policy it cannot honor", false,
			Hosted{Kind: HostedWebSearch, Policy: HostedRequired},
		},
		{"a capability it does not run", false, Hosted{Kind: HostedKind(99)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SupportsHosted(c, tt.in); got != tt.want {
				t.Errorf("SupportsHosted() = %v, want %v", got, tt.want)
			}
		})
	}
}

// The third axis: whether a search and a schema fit in one call, and whether
// the schema is enforced or merely requested when they do. Flattening the two
// into a bool would lose exactly the distinction Response.Format exists for.
func TestWithFormatKeepsTheModeDistinction(t *testing.T) {
	h, ok := HostedCapabilityOf(searchClient{}, HostedWebSearch)
	if !ok {
		t.Fatal("the search capability was not reported")
	}
	if h.WithFormat.JSONSchema != FormatEmulated {
		t.Errorf("WithFormat.JSONSchema = %v, want emulated",
			h.WithFormat.JSONSchema)
	}
	if h.WithFormat.Strict != FormatNone {
		t.Errorf("WithFormat.Strict = %v, want none - a provider that only "+
			"asks the model cannot promise strictness", h.WithFormat.Strict)
	}
	// The same driver enforces a schema natively when nothing is hosted; the
	// two answers are different questions and must not be merged.
	if got := CapabilitiesOf(searchClient{}).Format.Strict; got != FormatNative {
		t.Errorf("Format.Strict = %v, want native", got)
	}
}
