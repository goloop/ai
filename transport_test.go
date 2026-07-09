package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testOptions(baseURL string, retries int) Options {
	return NewOptions("key", WithBaseURL(baseURL), WithMaxRetries(retries))
}

func TestDoSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-test") != "1" {
			t.Errorf("header not applied: %q", r.Header.Get("x-test"))
		}
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	o := testOptions(srv.URL, 2)
	h := http.Header{}
	h.Set("x-test", "1")
	resp, err := o.Do(context.Background(), http.MethodGet, srv.URL, nil, h)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestDoRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	// Fast backoff: use a client with no artificial delay; MaxRetries=3.
	o := testOptions(srv.URL, 3)
	resp, err := o.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestDoReturnsFinalErrorResponseWithBody(t *testing.T) {
	// After exhausting retries, Do must return the last response (not a bare
	// error) so the caller can read the provider's error body. This is the
	// core fix that surfaces 429/5xx details in every driver.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":"rate limited: try later"}`)
	}))
	defer srv.Close()

	o := testOptions(srv.URL, 1)
	resp, err := o.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("want response, got err %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"error":"rate limited: try later"}` {
		t.Errorf("body = %q", body)
	}
}

func TestDoDoesNotRetryNonRetriable(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotImplemented) // 501 - not transient
	}))
	defer srv.Close()

	o := testOptions(srv.URL, 5)
	resp, err := o.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1 (501 not retried)", got)
	}
}

func TestDoHonorsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	o := testOptions(srv.URL, 2)
	start := time.Now()
	resp, err := o.Do(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Errorf("Retry-After not honored: waited %v, want >= 1s", elapsed)
	}
}

func TestDoCanceledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before the first retry sleep

	o := testOptions(srv.URL, 3)
	_, err := o.Do(ctx, http.MethodGet, srv.URL, nil, nil)
	if err == nil {
		t.Fatal("want error for canceled context")
	}
}

func TestDoTransportError(t *testing.T) {
	o := testOptions("http://127.0.0.1:1", 0)
	_, err := o.Do(context.Background(), http.MethodGet, "http://127.0.0.1:1", nil, nil)
	if err == nil {
		t.Fatal("want transport error")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("transport error should not be *APIError, got %v", err)
	}
}

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 250 * time.Millisecond},
		{2, 500 * time.Millisecond},
		{3, time.Second},
		{6, 8 * time.Second},
		{7, 8 * time.Second},  // guard, no overflow
		{40, 8 * time.Second}, // guard, no negative from overflow
	}
	for _, c := range cases {
		if got := backoff(c.attempt); got != c.want {
			t.Errorf("backoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestIsRetriable(t *testing.T) {
	retriable := []int{429, 500, 502, 503, 504, 529}
	for _, s := range retriable {
		if !isRetriable(s) {
			t.Errorf("isRetriable(%d) = false, want true", s)
		}
	}
	notRetriable := []int{200, 400, 401, 404, 501, 505}
	for _, s := range notRetriable {
		if isRetriable(s) {
			t.Errorf("isRetriable(%d) = true, want false", s)
		}
	}
}
