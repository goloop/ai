package ai

import (
	"bufio"
	"errors"
	"io"
	"iter"
	"strings"
)

// Limits on a Server-Sent Events stream. A single line is bounded by the
// scanner; the event is bounded separately, because an event is made of any
// number of "data:" lines and a stream that never sends the blank line ending
// one would otherwise be accumulated without bound.
const (
	maxSSELine  = 4 * 1024 * 1024
	maxSSEEvent = 16 * 1024 * 1024
)

// ErrSSEEventTooLarge is yielded by [SSEEvents] when one event's data grows
// past 16 MiB before the blank line that ends it, after which iteration stops.
// No well-formed streaming reply comes near that; a stream that does is
// malformed or hostile, and is cut off rather than buffered.
var ErrSSEEventTooLarge = errors.New("ai: SSE event too large")

// SSEEvents returns an iterator over the data payloads of a Server-Sent Events
// stream read from r. Each yielded string is the concatenated "data:" content
// of one event, with the field prefix and the single optional leading space
// removed and multi-line data joined by newlines. Comment lines (starting with
// ":") and other fields (event:, id:, retry:) are ignored, which is enough for
// the streaming chat APIs the drivers target. A read error is yielded once with
// an empty payload, after which iteration stops.
//
// A line longer than 4 MiB fails with [bufio.ErrTooLong]; an event whose data
// grows past 16 MiB fails with [ErrSSEEventTooLarge]. Both bound the memory a
// stream can make the reader hold.
//
// The caller is responsible for closing r.
func SSEEvents(r io.Reader) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), maxSSELine)

		var data strings.Builder
		hasData := false
		flush := func() bool {
			if !hasData {
				return true
			}
			s := data.String()
			data.Reset()
			hasData = false
			return yield(s, nil)
		}

		for sc.Scan() {
			line := strings.TrimSuffix(sc.Text(), "\r")
			if line == "" { // event boundary
				if !flush() {
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") { // comment
				continue
			}
			if v, ok := strings.CutPrefix(line, "data:"); ok || line == "data" {
				if !ok {
					v = ""
				}
				v = strings.TrimPrefix(v, " ")
				size := data.Len() + len(v)
				if hasData {
					size++
				}
				if size > maxSSEEvent {
					yield("", ErrSSEEventTooLarge)
					return
				}
				if hasData {
					data.WriteByte('\n')
				}
				data.WriteString(v)
				hasData = true
			}
		}

		if err := sc.Err(); err != nil {
			yield("", err)
			return
		}
		flush()
	}
}
