package ai

import (
	"bytes"
	"context"
	"net/http"
	"time"
)

// Do sends an HTTP request using the configured client and headers, retrying on
// HTTP 429 and 5xx responses up to MaxRetries with exponential backoff. The
// Options headers are applied first, then the per-call headers override them.
// The caller owns the returned response body and must close it.
func (o Options) Do(
	ctx context.Context,
	method, url string,
	body []byte,
	headers http.Header,
) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= o.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
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
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = &APIError{Status: resp.StatusCode}
			continue
		}

		return resp, nil
	}

	return nil, lastErr
}

// backoff returns the delay before retry attempt n (n >= 1): 250ms, 500ms,
// 1s, ... capped at 8s.
func backoff(attempt int) time.Duration {
	d := (250 * time.Millisecond) << (attempt - 1)
	if d > 8*time.Second {
		return 8 * time.Second
	}
	return d
}
