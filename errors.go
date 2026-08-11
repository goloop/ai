package ai

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Sentinel errors returned before a request reaches the network.
var (
	ErrNoRequest  = errors.New("ai: request is nil")
	ErrNoModel    = errors.New("ai: model is required")
	ErrNoMessages = errors.New("ai: at least one message is required")

	// Format errors, reported by Request.Validate before any driver work.
	ErrBadFormat = errors.New("ai: unknown format type")
	ErrNoSchema  = errors.New("ai: FormatJSONSchema requires a schema")
	ErrBadSchema = errors.New("ai: format schema must be a JSON Schema object")

	// ErrNoFormat is returned by a driver whose provider can neither enforce
	// the requested format nor be asked for it in a way that is worth
	// trusting. A driver never drops a Format quietly.
	ErrNoFormat = errors.New("ai: provider cannot produce the requested format")

	// ErrNoText is returned by Response.JSON when the reply carried no text
	// to decode, for example a response made up entirely of tool calls.
	ErrNoText = errors.New("ai: response has no text")

	// Hosted-capability errors, reported by Request.Validate before any
	// driver work.
	ErrBadHosted       = errors.New("ai: unknown hosted capability")
	ErrBadHostedPolicy = errors.New("ai: unknown hosted policy")
	ErrDupHosted       = errors.New("ai: hosted capability requested twice")

	// ErrNoHosted is returned by a driver whose provider cannot run the
	// requested capability, or cannot run it under the constraints given. A
	// format can be emulated by asking the model nicely; a search cannot,
	// because a driver has no search engine of its own. A caller who would
	// rather have the answer without the search asks again without Hosted,
	// which is one visible if.
	//
	// Drivers wrap it to say what exactly they could not do:
	//
	//	fmt.Errorf("%w: blocked domains", ai.ErrNoHosted)
	ErrNoHosted = errors.New("ai: provider cannot run the requested hosted capability")

	// ErrHostedRequired is returned when a capability requested with
	// HostedRequired was offered to the provider and the model did not use
	// it. Unlike ErrNoHosted this is not a provider limitation: the request
	// was sent and the answer simply is not the one that was asked for.
	ErrHostedRequired = errors.New("ai: required hosted capability was not used")

	// ErrFormatWithHosted is returned by a driver whose provider cannot
	// combine a structured Format with a hosted capability in one call. It
	// is a driver decision rather than a Request.Validate one because the
	// combination is legal at some providers and not at others. The way
	// around it is two calls: one that searches and answers in prose, one
	// that reshapes that prose into the schema. See the package
	// documentation.
	ErrFormatWithHosted = errors.New("ai: provider cannot combine the requested format with a hosted capability")
)

// APIError is a normalized error for a non-success HTTP response from a
// provider. Drivers fill the fields they can parse from the provider's error
// body and keep the original JSON in Raw.
type APIError struct {
	Status  int             // HTTP status code
	Type    string          // provider error type, when given
	Code    string          // provider error code, when given
	Message string          // human-readable message, when given
	Raw     json.RawMessage // original error body
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("ai: api error %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("ai: api error %d", e.Status)
}
