package ai

import (
	"context"
	"iter"
)

// Client is the contract every provider driver implements. Generate performs a
// single request and returns the whole response. Stream performs a request and
// returns an iterator over response chunks; the iterator yields a zero Chunk
// with a non-nil error and stops if the stream fails.
type Client interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
	Stream(ctx context.Context, req *Request) iter.Seq2[Chunk, error]
}
