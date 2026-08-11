package ai

import "fmt"

// HostedKind identifies a capability the provider runs on its own side.
//
// The zero value is deliberately not a capability: a Hosted built by mistake,
// or one decoded from an older payload, should fail validation instead of
// quietly asking for a web search nobody wanted.
type HostedKind int

// The hosted capabilities. Web search is the first of a family; code
// execution and file search fit the same shape when a provider offers them.
const (
	HostedWebSearch HostedKind = iota + 1
)

// String renders the kind for diagnostics.
func (k HostedKind) String() string {
	switch k {
	case HostedWebSearch:
		return "web_search"
	default:
		return "unknown"
	}
}

// parseHostedKind is the inverse of String, used when a stored Response is
// read back. An unrecognized name is an error rather than a zero kind: a
// report about a capability this build does not know is not a report about no
// capability.
func parseHostedKind(s string) (HostedKind, error) {
	switch s {
	case "web_search":
		return HostedWebSearch, nil
	default:
		return 0, fmt.Errorf("ai: unknown hosted capability %q", s)
	}
}

// HostedPolicy says whether a hosted capability is an offer or part of the
// contract. Providers treat a hosted capability as optional by default: the
// model decides whether to use it, and often decides not to when it believes
// it already knows the answer.
type HostedPolicy int

// The policies. HostedAuto is the zero value and leaves the choice to the
// model, which is what every provider does on its own.
const (
	HostedAuto     HostedPolicy = iota // the model may use it
	HostedRequired                     // the answer must come from using it
)

// String renders the policy for diagnostics.
func (p HostedPolicy) String() string {
	switch p {
	case HostedRequired:
		return "required"
	default:
		return "auto"
	}
}

// Hosted asks the provider to run a capability itself. It is deliberately not
// a [Tool]: a Tool is a promise that the caller will answer a [ToolUse] with a
// [ToolResult], while a hosted capability never comes back to the caller at
// all. Keeping the two in separate request fields means a tool loop written
// before this existed keeps working unchanged, because it never sees a call it
// does not know how to answer.
//
//	req.Hosted = []ai.Hosted{{Kind: ai.HostedWebSearch}}
//
// A driver whose provider cannot run the capability returns [ErrNoHosted]
// rather than answering without it. See [HostedMode] for why.
type Hosted struct {
	// Kind is the capability wanted. It has no default: the zero value is
	// not a capability and is rejected by Request.Validate.
	Kind HostedKind

	// Policy says whether the model may skip the capability. See
	// [HostedPolicy].
	Policy HostedPolicy

	// Web carries the knobs that only mean something for HostedWebSearch.
	// They live in their own type so that the shape of web search does not
	// become the shape every later capability has to wear. Nil asks for the
	// provider's own defaults.
	Web *HostedWeb
}

// HostedWeb configures hosted web search. Every field is a constraint, not a
// hint: a driver that cannot express a non-zero field returns [ErrNoHosted]
// instead of running a wider search than was asked for. A caller who wants the
// wider search can ask for it by leaving the field alone.
type HostedWeb struct {
	// MaxUses bounds how many searches the provider may run for one request.
	// Zero leaves it to the provider. It counts searches rather than results
	// because that is the unit providers actually meter and bill.
	MaxUses int

	// AllowDomains restricts results to these domains; BlockDomains excludes
	// them. Empty means no restriction. Providers generally accept one list
	// or the other but not both at once, so a driver given both returns
	// ErrNoHosted rather than picking one and dropping the other.
	AllowDomains []string
	BlockDomains []string

	// Region is an ISO 3166-1 alpha-2 country code that biases results
	// ("UA", "DE"). Empty leaves it to the provider.
	Region string
}

// Validate reports whether the capability is one a driver can act on. An
// unknown kind or policy is rejected rather than treated as the nearest known
// one, for the same reason [Format.Validate] rejects an unknown type: nine
// drivers read this value, and a value none of them agrees on would mean nine
// different guesses about what the caller wanted.
func (h Hosted) Validate() error {
	switch h.Kind {
	case HostedWebSearch:
	default:
		return ErrBadHosted
	}

	switch h.Policy {
	case HostedAuto, HostedRequired:
	default:
		return ErrBadHostedPolicy
	}

	return nil
}

// validateHosted checks a whole request's capabilities. Two entries of the
// same kind are rejected because the second would have to either override or
// merge with the first, and a driver has no way to know which was meant.
func validateHosted(hs []Hosted) error {
	seen := make(map[HostedKind]bool, len(hs))
	for _, h := range hs {
		if err := h.Validate(); err != nil {
			return err
		}
		if seen[h.Kind] {
			return ErrDupHosted
		}
		seen[h.Kind] = true
	}
	return nil
}

// HostedMode says what became of one capability the request asked for.
type HostedMode int

// What happened to a hosted capability.
const (
	HostedNone    HostedMode = iota // it was not requested
	HostedNative                    // the provider ran it
	HostedSkipped                   // it was offered and the model did not use it
)

// String renders the mode for diagnostics.
func (m HostedMode) String() string {
	switch m {
	case HostedNative:
		return "native"
	case HostedSkipped:
		return "skipped"
	default:
		return "none"
	}
}

// parseHostedMode is the inverse of String, used when a stored Response is
// read back. As with a kind, an unrecognized name is an error: HostedNone
// means nothing was asked for, and guessing it here would erase the fact that
// something was.
func parseHostedMode(s string) (HostedMode, error) {
	switch s {
	case "", "none":
		return HostedNone, nil
	case "native":
		return HostedNative, nil
	case "skipped":
		return HostedSkipped, nil
	default:
		return HostedNone, fmt.Errorf("ai: unknown hosted mode %q", s)
	}
}

// HostedReport says what one requested capability actually did.
//
// HostedSkipped is the state worth having: a provider can accept a search tool
// and then answer from the model's own memory, and the two answers look
// identical from the outside. Without this report a caller cannot tell "looked
// and found nothing" from "never looked", which is exactly the distinction
// that decides whether an answer is worth trusting.
type HostedReport struct {
	// Kind is the capability this report is about.
	Kind HostedKind

	// Mode is what happened. See [HostedMode].
	Mode HostedMode

	// Calls is how many times the provider ran the capability, when it says
	// so. Hosted work is usually billed separately from tokens, so this is
	// the only place the cost of a request shows up. A driver that can prove
	// the capability ran but cannot get a count reports 1.
	Calls int
}
