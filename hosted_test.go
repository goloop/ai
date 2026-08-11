package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestHostedValidate(t *testing.T) {
	tests := []struct {
		name string
		in   Hosted
		want error
	}{
		{"web search", Hosted{Kind: HostedWebSearch}, nil},
		{"required", Hosted{Kind: HostedWebSearch, Policy: HostedRequired}, nil},
		{"zero kind", Hosted{}, ErrBadHosted},
		{"unknown kind", Hosted{Kind: HostedKind(99)}, ErrBadHosted},
		{
			"unknown policy",
			Hosted{Kind: HostedWebSearch, Policy: HostedPolicy(99)},
			ErrBadHostedPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); !errors.Is(err, tt.want) {
				t.Errorf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

// A zero Hosted must not mean "web search". The kind starts at one so that a
// value built by mistake is caught instead of silently asking for something.
func TestHostedZeroValueIsNotACapability(t *testing.T) {
	if (Hosted{}).Kind == HostedWebSearch {
		t.Fatal("the zero Hosted asks for web search")
	}
}

func TestRequestValidateHosted(t *testing.T) {
	base := func(hs ...Hosted) *Request {
		return &Request{
			Model:    "m",
			Messages: []Message{UserText("hi")},
			Hosted:   hs,
		}
	}

	tests := []struct {
		name string
		req  *Request
		want error
	}{
		{"none", base(), nil},
		{"one", base(Hosted{Kind: HostedWebSearch}), nil},
		{"bad kind", base(Hosted{}), ErrBadHosted},
		{
			"same kind twice",
			base(
				Hosted{Kind: HostedWebSearch},
				Hosted{Kind: HostedWebSearch, Policy: HostedRequired},
			),
			ErrDupHosted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); !errors.Is(err, tt.want) {
				t.Errorf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRequestHostedByKind(t *testing.T) {
	req := &Request{Hosted: []Hosted{{
		Kind:   HostedWebSearch,
		Policy: HostedRequired,
		Web:    &HostedWeb{MaxUses: 3},
	}}}

	got, ok := req.HostedByKind(HostedWebSearch)
	if !ok {
		t.Fatal("HostedByKind() did not find the requested capability")
	}
	if got.Policy != HostedRequired || got.Web.MaxUses != 3 {
		t.Errorf("HostedByKind() = %+v, want the request's own value", got)
	}

	if _, ok := req.HostedByKind(HostedKind(99)); ok {
		t.Error("HostedByKind() found a capability that was not requested")
	}

	var nilReq *Request
	if _, ok := nilReq.HostedByKind(HostedWebSearch); ok {
		t.Error("HostedByKind() on a nil request found something")
	}
}

func TestRequestHostedReports(t *testing.T) {
	tests := []struct {
		name    string
		req     *Request
		calls   map[HostedKind]int
		want    []HostedReport
		wantErr error
	}{
		{
			name: "nothing requested",
			req:  &Request{},
			want: nil,
		},
		{
			name:  "nil request",
			req:   nil,
			calls: map[HostedKind]int{HostedWebSearch: 1},
			want:  nil,
		},
		{
			name:  "used",
			req:   &Request{Hosted: []Hosted{{Kind: HostedWebSearch}}},
			calls: map[HostedKind]int{HostedWebSearch: 2},
			want: []HostedReport{
				{Kind: HostedWebSearch, Mode: HostedNative, Calls: 2},
			},
		},
		{
			name: "offered and skipped",
			req:  &Request{Hosted: []Hosted{{Kind: HostedWebSearch}}},
			want: []HostedReport{
				{Kind: HostedWebSearch, Mode: HostedSkipped},
			},
		},
		{
			name:  "a zero count is not a use",
			req:   &Request{Hosted: []Hosted{{Kind: HostedWebSearch}}},
			calls: map[HostedKind]int{HostedWebSearch: 0},
			want: []HostedReport{
				{Kind: HostedWebSearch, Mode: HostedSkipped},
			},
		},
		{
			name: "required and skipped",
			req: &Request{Hosted: []Hosted{{
				Kind:   HostedWebSearch,
				Policy: HostedRequired,
			}}},
			wantErr: ErrHostedRequired,
		},
		{
			name: "required and used",
			req: &Request{Hosted: []Hosted{{
				Kind:   HostedWebSearch,
				Policy: HostedRequired,
			}}},
			calls: map[HostedKind]int{HostedWebSearch: 1},
			want: []HostedReport{
				{Kind: HostedWebSearch, Mode: HostedNative, Calls: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.req.HostedReports(tt.calls)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("HostedReports() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != nil {
					t.Errorf("HostedReports() = %+v, want no reports "+
						"alongside an error", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("HostedReports() = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("report %d = %+v, want %+v",
						i, got[i], tt.want[i])
				}
			}
		})
	}
}

// The reports come back in the order the request listed them, so a caller can
// match them up positionally with what it asked for.
func TestHostedReportsKeepRequestOrder(t *testing.T) {
	// A second kind does not exist yet, so the order is checked with the one
	// that does plus a value from the space reserved for later kinds.
	const later = HostedKind(2)

	req := &Request{Hosted: []Hosted{
		{Kind: later},
		{Kind: HostedWebSearch},
	}}

	got, err := req.HostedReports(map[HostedKind]int{HostedWebSearch: 1})
	if err != nil {
		t.Fatalf("HostedReports() error = %v", err)
	}
	if len(got) != 2 || got[0].Kind != later || got[1].Kind != HostedWebSearch {
		t.Errorf("HostedReports() = %+v, want request order", got)
	}
}

func TestHostedStrings(t *testing.T) {
	if got := HostedWebSearch.String(); got != "web_search" {
		t.Errorf("HostedWebSearch.String() = %q", got)
	}
	if got := HostedKind(99).String(); got != "unknown" {
		t.Errorf("HostedKind(99).String() = %q", got)
	}
	if got := HostedAuto.String(); got != "auto" {
		t.Errorf("HostedAuto.String() = %q", got)
	}
	if got := HostedRequired.String(); got != "required" {
		t.Errorf("HostedRequired.String() = %q", got)
	}
	if got := HostedNone.String(); got != "none" {
		t.Errorf("HostedNone.String() = %q", got)
	}
	if got := HostedNative.String(); got != "native" {
		t.Errorf("HostedNative.String() = %q", got)
	}
	if got := HostedSkipped.String(); got != "skipped" {
		t.Errorf("HostedSkipped.String() = %q", got)
	}
}

func TestResponseCitations(t *testing.T) {
	resp := &Response{Parts: []Part{
		Text{Text: "one", Citations: []Citation{{URL: "https://a"}}},
		ToolUse{Name: "t"},
		Text{Text: "two", Citations: []Citation{
			{URL: "https://b"}, {URL: "https://c"},
		}},
		Text{Text: "no sources"},
	}}

	got := resp.Citations()
	want := []string{"https://a", "https://b", "https://c"}
	if len(got) != len(want) {
		t.Fatalf("Citations() = %+v, want %d entries", got, len(want))
	}
	for i, u := range want {
		if got[i].URL != u {
			t.Errorf("citation %d = %q, want %q", i, got[i].URL, u)
		}
	}

	if c := (&Response{}).Citations(); c != nil {
		t.Errorf("Citations() on an empty response = %+v, want nil", c)
	}
}

// A response carries how it was produced, not only what it says: a stored
// answer that lost its format mode or its hosted reports reads as a stronger
// answer than it is.
func TestResponseJSONRoundTripHosted(t *testing.T) {
	in := Response{
		Model: "m",
		Parts: []Part{Text{
			Text: "прив'іт",
			Citations: []Citation{{
				URL:       "https://example.org/a",
				Title:     "A",
				CitedText: "джерело",
				StartByte: 2,
				EndByte:   9,
			}},
		}},
		StopReason: "end_turn",
		Usage:      Usage{InputTokens: 3, OutputTokens: 4},
		Format:     FormatEmulated,
		Hosted: []HostedReport{
			{Kind: HostedWebSearch, Mode: HostedNative, Calls: 2},
		},
	}

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var out Response
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if out.Format != FormatEmulated {
		t.Errorf("Format = %v, want emulated", out.Format)
	}
	if len(out.Hosted) != 1 || out.Hosted[0] != in.Hosted[0] {
		t.Errorf("Hosted = %+v, want %+v", out.Hosted, in.Hosted)
	}

	text, ok := out.Parts[0].(Text)
	if !ok {
		t.Fatalf("part 0 = %T, want Text", out.Parts[0])
	}
	if len(text.Citations) != 1 || text.Citations[0] != in.Parts[0].(Text).Citations[0] {
		t.Errorf("Citations = %+v, want the ones that went in", text.Citations)
	}
}

// A response that asked for nothing encodes exactly as it did before hosted
// capabilities existed, so stored payloads and their readers are untouched.
func TestResponseJSONOmitsUnusedHostedFields(t *testing.T) {
	data, err := json.Marshal(Response{
		Model: "m",
		Parts: []Part{Text{Text: "hi"}},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	for _, key := range []string{"format", "hosted", "citations"} {
		if strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("encoded response mentions %q: %s", key, data)
		}
	}
}

func TestResponseJSONRejectsUnknownNames(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"format", `{"format":"guessed"}`},
		{"hosted kind", `{"hosted":[{"kind":"telepathy","mode":"native"}]}`},
		{"hosted mode", `{"hosted":[{"kind":"web_search","mode":"maybe"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out Response
			if err := json.Unmarshal([]byte(tt.in), &out); err == nil {
				t.Errorf("Unmarshal(%s) = nil, want an error", tt.in)
			}
		})
	}
}
