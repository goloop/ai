package ai

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func collect(t *testing.T, input string) []string {
	t.Helper()
	var out []string
	for data, err := range SSEEvents(strings.NewReader(input)) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out = append(out, data)
	}
	return out
}

func TestSSEEventsBasic(t *testing.T) {
	got := collect(t, "data: hello\n\ndata: world\n\n")
	want := []string{"hello", "world"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSSEEventsNoSpaceAfterColon(t *testing.T) {
	got := collect(t, "data:hello\n\n")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestSSEEventsMultilineData(t *testing.T) {
	got := collect(t, "data: line1\ndata: line2\n\n")
	if len(got) != 1 || got[0] != "line1\nline2" {
		t.Errorf("got %q, want joined by newline", got)
	}
}

func TestSSEEventsCRLF(t *testing.T) {
	got := collect(t, "data: hello\r\n\r\n")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestSSEEventsIgnoresCommentsAndFields(t *testing.T) {
	got := collect(t, ": keep-alive\nevent: message\nid: 7\ndata: payload\n\n")
	if len(got) != 1 || got[0] != "payload" {
		t.Errorf("got %q", got)
	}
}

func TestSSEEventsFinalWithoutBlankLine(t *testing.T) {
	// A stream that ends at EOF without a trailing blank line still flushes
	// the accumulated data.
	got := collect(t, "data: last")
	if len(got) != 1 || got[0] != "last" {
		t.Errorf("got %q", got)
	}
}

func TestSSEEventsEmpty(t *testing.T) {
	got := collect(t, "")
	if len(got) != 0 {
		t.Errorf("got %q, want none", got)
	}
}

func TestSSEEventsError(t *testing.T) {
	sentinel := errors.New("boom")
	var gotErr error
	for _, err := range SSEEvents(errReader{err: sentinel}) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if !errors.Is(gotErr, sentinel) {
		t.Errorf("gotErr = %v, want sentinel", gotErr)
	}
}

func TestSSEEventsEarlyStop(t *testing.T) {
	// Consumer stops after the first event; iteration must not panic or hang.
	n := 0
	for range SSEEvents(strings.NewReader("data: a\n\ndata: b\n\ndata: c\n\n")) {
		n++
		break
	}
	if n != 1 {
		t.Errorf("n = %d, want 1", n)
	}
}

// errReader returns err on the first read.
type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

var _ io.Reader = errReader{}
