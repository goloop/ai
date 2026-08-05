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
