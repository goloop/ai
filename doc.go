// Package ai defines a single, provider-agnostic interface for talking to
// large language model APIs, together with the shared request and response
// types every provider driver speaks.
//
// The design mirrors the standard library's split between an interface and
// its drivers: like database/sql with its drivers, or log/slog with its
// handlers, package ai holds the common contract while a separate package
// per provider (anthropic, openai, gemini, and so on) implements it. A driver
// depends only on this package, so the whole set stays free of third-party
// dependencies.
//
// The contract is the Client interface:
//
//	type Client interface {
//	    Generate(ctx context.Context, req *Request) (*Response, error)
//	    Stream(ctx context.Context, req *Request) iter.Seq2[Chunk, error]
//	}
//
// A Request carries a model, an optional system prompt, a list of Messages,
// optional Tools and the usual sampling knobs. A Message is a role plus a
// list of content Parts (Text, Image, ToolUse, ToolResult), which is enough
// to express multimodal input and tool calling across providers. Generate
// returns a whole Response; Stream yields Chunks as they arrive.
//
// Endpoints that providers do not share (embeddings, image generation, audio,
// files, batches, and so on) are not part of this interface. Each driver
// exposes those as its own native methods, so the common surface stays small
// and honest while provider-specific power is still available.
//
// This package also carries the plumbing drivers reuse: Options and its
// functional configuration, Options.Do for HTTP requests with retries, and
// SSEEvents for reading Server-Sent Events streams.
package ai
