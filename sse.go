package ai

import (
	"bufio"
	"io"
	"iter"
	"strings"
)

// SSEEvents returns an iterator over the data payloads of a Server-Sent Events
// stream read from r. Each yielded string is the concatenated "data:" content
// of one event, with the field prefix and the single optional leading space
// removed and multi-line data joined by newlines. Comment lines (starting with
// ":") and other fields (event:, id:, retry:) are ignored, which is enough for
// the streaming chat APIs the drivers target. A read error is yielded once with
// an empty payload, after which iteration stops.
//
// The caller is responsible for closing r.
func SSEEvents(r io.Reader) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

		var data strings.Builder
		flush := func() bool {
			if data.Len() == 0 {
				return true
			}
			s := data.String()
			data.Reset()
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
			if v, ok := strings.CutPrefix(line, "data:"); ok {
				v = strings.TrimPrefix(v, " ")
				if data.Len() > 0 {
					data.WriteByte('\n')
				}
				data.WriteString(v)
			}
		}

		if err := sc.Err(); err != nil {
			yield("", err)
			return
		}
		flush()
	}
}
