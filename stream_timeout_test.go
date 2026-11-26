package ai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStreamTimeoutAfterHeaders(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		time.Sleep(180 * time.Millisecond)
		io.WriteString(w, "done")
	}))
	defer s.Close()
	opts := NewOptions("", WithTimeout(100*time.Millisecond), WithMaxRetries(0))
	resp, err := opts.DoStream(t.Context(), "GET", s.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != "done" {
		t.Fatal(string(data), err)
	}
}
