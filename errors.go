package ai

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Sentinel errors returned before a request reaches the network.
var (
	ErrNoModel    = errors.New("ai: model is required")
	ErrNoMessages = errors.New("ai: at least one message is required")
	ErrNoAPIKey   = errors.New("ai: API key is required")
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
