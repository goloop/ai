package ai

import (
	"fmt"
	"net/http"
	"time"
)

// Options is the shared client configuration every driver understands. Drivers
// build it from an API key and functional options with [NewOptions] and reuse
// it for transport (see [Options.Do]).
type Options struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
	MaxRetries int
	Headers    http.Header
}

// String renders the Options with the API key redacted, so printing a client
// or its options with %v or %+v never leaks the credential into logs.
func (o Options) String() string {
	key := "unset"
	if o.APIKey != "" {
		key = "[redacted]"
	}
	return fmt.Sprintf(
		"ai.Options{APIKey:%s BaseURL:%q Timeout:%s MaxRetries:%d}",
		key, o.BaseURL, o.Timeout, o.MaxRetries)
}

// GoString redacts the API key for the %#v verb, matching String.
func (o Options) GoString() string { return o.String() }

// Option configures Options. The same options work across every provider, so
// client construction looks the same everywhere.
type Option func(*Options)

// WithBaseURL overrides the provider's default API base URL. It is useful for
// proxies, gateways, mock servers and self-hosted deployments.
func WithBaseURL(u string) Option {
	return func(o *Options) { o.BaseURL = u }
}

// WithHTTPClient sets the HTTP client used for requests. When set, its own
// timeout takes precedence over WithTimeout.
func WithHTTPClient(c *http.Client) Option {
	return func(o *Options) { o.HTTPClient = c }
}

// WithTimeout sets the per-request timeout used when no custom HTTP client is
// provided.
func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.Timeout = d }
}

// WithMaxRetries sets how many times [Options.Do] retries a request on HTTP 429
// or 5xx responses. Zero disables retrying.
func WithMaxRetries(n int) Option {
	return func(o *Options) { o.MaxRetries = n }
}

// WithHeader adds a header sent with every request, for example a custom API
// version or a beta feature flag.
func WithHeader(key, value string) Option {
	return func(o *Options) {
		if o.Headers == nil {
			o.Headers = http.Header{}
		}
		o.Headers.Add(key, value)
	}
}

// NewOptions builds Options from an API key and functional options, filling in
// defaults: a 60s timeout, two retries and an HTTP client when none is given.
func NewOptions(apiKey string, opts ...Option) Options {
	o := Options{
		APIKey:     apiKey,
		Timeout:    60 * time.Second,
		MaxRetries: 2,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{Timeout: o.Timeout}
	}
	return o
}
