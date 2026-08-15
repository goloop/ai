package ai

// Capable is implemented by drivers that can describe themselves. It is
// optional: a driver that does not implement it simply offers no hint, and
// everything else keeps working.
//
// It exists because the knowledge of what a provider can do lives in the
// driver and, without this, gets copied into every application as a table of
// provider names - one that goes quietly out of date the day a driver learns
// something new.
type Capable interface {
	Capabilities() Capabilities
}

// Capabilities describes what a driver can be asked for.
//
// IT IS A HINT, NOT A PERMISSION. Whether a request actually succeeds depends
// on the model, the account, the region and the endpoint, none of which this
// value knows about. [ErrNoHosted], [ErrNoFormat] and [ErrFormatWithHosted]
// remain the source of truth; a caller still handles them. What this is good
// for is the decision made before the call: whether to show a "search the web"
// button at all, and whether a feature needs one request or two.
type Capabilities struct {
	// Hosted maps each capability the driver can run to what it accepts.
	// A kind that is absent is one the driver cannot run at all.
	Hosted map[HostedKind]HostedCapability

	// Format says which structured-output shapes the driver offers on an
	// ordinary request, and whether the provider enforces them or the driver
	// asks for them in the prompt. See [FormatCapability].
	Format FormatCapability

	// Images reports whether the driver can generate images. Image generation
	// is not part of the shared Client interface - each driver exposes its own
	// GenerateImage method with its own request and response types, because
	// providers do not share the shape - so this is only a hint: it lets a UI
	// build a "providers that can draw" list the same way it builds one for
	// web search, without a hand-kept table that goes stale. The zero value,
	// false, means the driver does not draw.
	Images bool
}

// HostedCapability describes one capability a driver can run.
type HostedCapability struct {
	// Web is set for [HostedWebSearch] and says which of the settings in
	// [HostedWeb] this provider can express. A setting reported false is one
	// that makes the request [ErrNoHosted] rather than one that is ignored.
	Web *HostedWebCapability

	// Policy says whether [HostedRequired] can be honored.
	Policy HostedPolicyCapability

	// WithFormat says whether this capability survives in the same call as a
	// structured format, and how. It decides the shape of a feature rather
	// than the details of a call: a provider that reports nothing here needs
	// two requests - one that searches and answers in prose, one that
	// reshapes that prose - while a provider that reports FormatEmulated or
	// FormatNative does it in one.
	//
	// FormatEmulated here means what it means on [Response.Format]: the model
	// was asked, not constrained. Reporting it as FormatNative would hand back
	// a guarantee where there is only a request.
	WithFormat FormatCapability
}

// HostedPolicyCapability says which policies a driver can honor. Auto is
// always true for a capability the driver runs at all; Required is separate
// because a provider may offer a capability and no way to insist on it.
type HostedPolicyCapability struct {
	Required bool
}

// FormatCapability says how each structured-output shape is satisfied.
// The zero value, [FormatNone] in every field, means the shape is not
// available at all - which for [HostedCapability.WithFormat] is the common
// case and the reason a feature may need two calls.
type FormatCapability struct {
	JSON       FormatMode // FormatJSON
	JSONSchema FormatMode // FormatJSONSchema
	Strict     FormatMode // FormatJSONSchema with Format.Strict set
}

// HostedWebCapability says which web-search settings a provider can express.
// Every field mirrors one field of [HostedWeb]; false means a non-zero value
// there is refused rather than silently widened.
type HostedWebCapability struct {
	MaxUses      bool
	AllowDomains bool
	BlockDomains bool

	// BothDomainLists says whether an allow list and a block list can be sent
	// together. Most providers take one or the other.
	BothDomainLists bool

	Region bool
}

// CapabilitiesOf returns what a client says it can do, or the zero
// Capabilities for a client that does not describe itself. The zero value
// claims nothing, so code written against it degrades to asking and handling
// the error, which is what it should have done anyway.
func CapabilitiesOf(c Client) Capabilities {
	if capable, ok := c.(Capable); ok {
		return capable.Capabilities()
	}
	return Capabilities{}
}

// HostedCapabilityOf returns what a client says about one capability, and
// whether it claims to run it at all.
func HostedCapabilityOf(c Client, k HostedKind) (HostedCapability, bool) {
	h, ok := CapabilitiesOf(c).Hosted[k]
	return h, ok
}

// SupportsImages reports whether a client claims it can generate images. It
// mirrors [SupportsHosted]: a client that does not describe itself reports
// false, so this answers "is this known to draw", not "is this known not to".
// Image generation itself is a native method on each driver, not part of the
// shared interface; this is only the hint a UI uses to decide whether to offer
// it.
func SupportsImages(c Client) bool {
	return CapabilitiesOf(c).Images
}

// SupportsHosted reports whether a client claims it can run a capability as
// asked for, constraints included. A client that does not describe itself
// reports false, so this answers "is this known to work", never "is this known
// to fail" - the difference matters, because the honest answer to the second
// only comes from making the request.
func SupportsHosted(c Client, h Hosted) bool {
	hc, ok := HostedCapabilityOf(c, h.Kind)
	if !ok {
		return false
	}
	if h.Policy == HostedRequired && !hc.Policy.Required {
		return false
	}
	if h.Kind == HostedWebSearch {
		return supportsWeb(hc.Web, h.Web)
	}
	return true
}

// supportsWeb checks the web-search settings against what a driver accepts.
func supportsWeb(hc *HostedWebCapability, w *HostedWeb) bool {
	if w == nil {
		return true
	}
	if hc == nil {
		return false
	}
	switch {
	case w.MaxUses > 0 && !hc.MaxUses:
		return false
	case len(w.AllowDomains) > 0 && !hc.AllowDomains:
		return false
	case len(w.BlockDomains) > 0 && !hc.BlockDomains:
		return false
	case len(w.AllowDomains) > 0 && len(w.BlockDomains) > 0 && !hc.BothDomainLists:
		return false
	case w.Region != "" && !hc.Region:
		return false
	}
	return true
}
