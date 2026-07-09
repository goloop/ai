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

// FuzzSSEEvents checks that the parser never panics on arbitrary input and that
// parsing is stable: re-encoding the events it produces and parsing them again
// yields exactly the same events.
func FuzzSSEEvents(f *testing.F) {
	for _, s := range []string{
		"data: hello\n\ndata: world\n\n",
		"data:hi\ndata:there\n\n",
		": comment\nevent: x\nid: 1\ndata: y\n\n",
		"data: last",
		"\n\n\n",
		"data:\ndata:x\n\n",
		"data: a\ndata:\ndata: b\n\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		var events []string
		for data, err := range SSEEvents(strings.NewReader(in)) {
			if err != nil {
				return // a strings.Reader never errors; nothing to check
			}
			events = append(events, data)
		}

		// A carriage return is a structural line terminator that the parser
		// strips as CRLF tolerance, so an event carrying one cannot round-trip
		// through data lines. The no-panic and termination guarantees above
		// still hold; only the stability property below needs a clean payload.
		for _, e := range events {
			if strings.ContainsRune(e, '\r') {
				return
			}
		}

		// Re-encode each event as data lines and parse again.
		var b strings.Builder
		for _, e := range events {
			for _, line := range strings.Split(e, "\n") {
				b.WriteString("data: ")
				b.WriteString(line)
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		}
		var round []string
		for data, err := range SSEEvents(strings.NewReader(b.String())) {
			if err != nil {
				t.Fatalf("re-parse error: %v", err)
			}
			round = append(round, data)
		}

		if len(round) != len(events) {
			t.Fatalf("event count changed: %d -> %d (%q)", len(events), len(round), events)
		}
		for i := range events {
			if round[i] != events[i] {
				t.Fatalf("event %d changed: %q -> %q", i, events[i], round[i])
			}
		}
	})
}

// errReader returns err on the first read.
type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

var _ io.Reader = errReader{}
