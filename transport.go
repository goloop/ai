package ai

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"time"
)

// Do sends an HTTP request using the configured client and headers, retrying
// transient responses (HTTP 429, 500, 502, 503, 504 and 529) up to MaxRetries
// with exponential backoff. A Retry-After header on the response is honored,
// capped at 30s. The Options headers are applied first, then the per-call
// headers override them.
//
// On the final attempt the response is returned as-is even when its status is
// an error, so the caller can read the provider's error body (drivers check the
// status and call their own error parser). Do only returns a non-nil error for
// a transport-level failure (no response) or a canceled context. The caller
// owns the returned response body and must close it.
//
// Retries repeat the request, including non-idempotent POSTs. A transport error
// may occur after the server already accepted the request, so a retried POST
// can execute twice; use WithMaxRetries(0) to disable retrying when that is a
// concern.
func (o Options) Do(
	ctx context.Context,
	method, url string,
	body []byte,
	headers http.Header,
) (*http.Response, error) {
	var lastErr error
	var delay time.Duration

	for attempt := 0; attempt <= o.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(
			ctx, method, url, bytes.NewReader(body),
		)
		if err != nil {
			return nil, err
		}
		for k, vs := range o.Headers {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		for k, vs := range headers {
			req.Header[http.CanonicalHeaderKey(k)] = append([]string(nil), vs...)
		}

		resp, err := o.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			delay = backoff(attempt + 1)
			continue
		}

		// Retry transient statuses while attempts remain. On the last
		// attempt, fall through and return the response so the caller can
		// read the error body.
		if attempt < o.MaxRetries && isRetriable(resp.StatusCode) {
			delay = retryDelay(resp, attempt+1)
			resp.Body.Close()
			lastErr = &APIError{Status: resp.StatusCode}
			continue
		}

		return resp, nil
	}

	return nil, lastErr
}

// isRetriable reports whether an HTTP status is worth retrying: rate limiting,
// gateway/availability errors and Anthropic's 529 overloaded status. Other 5xx
// (for example 501 Not Implemented) are not transient and are not retried.
func isRetriable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
		529:                            // Anthropic: overloaded
		return true
	default:
		return false
	}
}

// retryDelay returns the delay before the next attempt. It honors a Retry-After
// header (delta-seconds or an HTTP date), capped at 30s, and otherwise falls
// back to exponential backoff.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	const maxWait = 30 * time.Second
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
			return min(time.Duration(secs)*time.Second, maxWait)
		}
		if t, err := http.ParseTime(ra); err == nil {
			d := time.Until(t)
			if d < 0 {
				d = 0
			}
			return min(d, maxWait)
		}
	}
	return backoff(attempt)
}

// backoff returns the delay before retry attempt n (n >= 1): 250ms, 500ms,
// 1s, ... capped at 8s. The cap is applied before the shift to avoid
// overflowing the duration for large attempt counts.
func backoff(attempt int) time.Duration {
	if attempt > 6 {
		return 8 * time.Second
	}
	d := (250 * time.Millisecond) << (attempt - 1)
	if d > 8*time.Second {
		return 8 * time.Second
	}
	return d
}
