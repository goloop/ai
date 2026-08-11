package ai

import "fmt"

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

	// Format asks for a structured response, JSON or JSON matching a schema.
	// Nil, the default, asks for nothing. Drivers map it onto native provider
	// support where it exists and request it in the prompt where it does not;
	// [Response.Format] reports which happened. See [Format].
	Format *Format

	// Hosted asks the provider to run capabilities of its own, web search
	// first among them. It is separate from Tools on purpose: a hosted
	// capability never produces a [ToolUse] the caller has to answer, so code
	// that loops over tool calls and knows nothing about this field keeps
	// working unchanged. [Response.Hosted] reports what each one did. See
	// [Hosted].
	//
	// Not every provider can run every capability, and a driver that cannot
	// returns [ErrNoHosted] rather than answering without it.
	Hosted []Hosted
}

// Validate reports whether the request has the minimum a provider needs. A nil
// request is reported as ErrNoRequest rather than panicking, so a driver can
// forward a bad call as a normal error.
func (r *Request) Validate() error {
	if r == nil {
		return ErrNoRequest
	}
	if r.Model == "" {
		return ErrNoModel
	}
	if len(r.Messages) == 0 {
		return ErrNoMessages
	}
	if err := r.Format.Validate(); err != nil {
		return err
	}
	return validateHosted(r.Hosted)
}

// HostedByKind returns the requested capability of the given kind and whether
// it was requested at all. Request.Validate rejects a kind listed twice, so
// there is at most one to find.
func (r *Request) HostedByKind(k HostedKind) (Hosted, bool) {
	if r == nil {
		return Hosted{}, false
	}
	for _, h := range r.Hosted {
		if h.Kind == k {
			return h, true
		}
	}
	return Hosted{}, false
}

// HostedReports pairs everything the request asked for with what the provider
// did, in request order, and enforces [HostedRequired] while it is at it. It
// lives here so that nine drivers agree on the answer instead of each deciding
// separately what "the provider skipped it" means, the same reason
// [Format.Instruction] is shared.
//
// A driver calls it once it has read the reply, passing how many times each
// capability ran:
//
//	reports, err := req.HostedReports(map[ai.HostedKind]int{
//		ai.HostedWebSearch: searches,
//	})
//
// A driver that can prove a capability ran but cannot get a count from its
// provider passes 1; a kind missing from calls, or present with a zero count,
// was not used. A request that asked for nothing yields no reports and no
// error, which is what every call written before this existed does.
//
// A capability asked for with HostedRequired and not used is [ErrHostedRequired]
// rather than a response the caller has to inspect: a required search that did
// not happen makes the answer something other than what was asked for, and an
// answer from the model's own memory is indistinguishable from a researched one
// by the time it reaches the caller.
func (r *Request) HostedReports(calls map[HostedKind]int) ([]HostedReport, error) {
	if r == nil || len(r.Hosted) == 0 {
		return nil, nil
	}

	reports := make([]HostedReport, 0, len(r.Hosted))
	for _, h := range r.Hosted {
		n := calls[h.Kind]
		mode := HostedSkipped
		if n > 0 {
			mode = HostedNative
		}
		if mode != HostedNative && h.Policy == HostedRequired {
			return nil, fmt.Errorf("%w: %s", ErrHostedRequired, h.Kind)
		}
		reports = append(reports, HostedReport{
			Kind:  h.Kind,
			Mode:  mode,
			Calls: n,
		})
	}

	return reports, nil
}
