package ai

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSSEEventsPreservesEmptyData(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"data:\n\n", ""},
		{"data\n\n", ""},
		{"data:\ndata: text\n\n", "\ntext"},
		{"data: text\ndata:\n\n", "text\n"},
		{"data:\ndata:\n\n", "\n"},
	} {
		got := collect(t, tt.input)
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("input %q: got %q, want one event %q", tt.input, got, tt.want)
		}
	}
}

func TestRetryDelayLargeSeconds(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": {"9223372036854775807"}}}
	if got := retryDelay(resp, 1); got != 30*time.Second {
		t.Fatalf("delay = %v, want 30s", got)
	}
}

var responseTextResult string

func BenchmarkResponseText(b *testing.B) {
	for _, count := range []int{1, 16} {
		name := "single"
		if count > 1 {
			name = "multipart"
		}
		b.Run(name, func(b *testing.B) {
			r := Response{}
			for range count {
				r.Parts = append(r.Parts, Text{Text: strings.Repeat("hello ", 16)})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				responseTextResult = r.Text()
			}
		})
	}
}
