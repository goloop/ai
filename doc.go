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
// # Structured output
//
// Request.Format asks for JSON, optionally matching a JSON Schema, and
// Response.JSON decodes the reply into a Go value. Providers differ in what
// they can enforce: a driver uses native support where it exists, asks for the
// format in the prompt where it does not, and reports which it did in
// Response.Format. No driver drops a Format silently.
//
// FormatEmulated is not a weaker FormatNative, it is a different thing: the
// model was asked, not constrained. Code that cannot proceed without an
// enforced shape should check for FormatNative, or validate the decoded value
// itself - Response.JSON proves the reply parses, not that it matches the
// schema.
//
// # Hosted capabilities
//
// Request.Hosted asks the provider to do work on its own side, web search
// first among them:
//
//	req.Hosted = []ai.Hosted{{Kind: ai.HostedWebSearch}}
//
// A hosted capability is not a Tool and does not live in Request.Tools. A Tool
// is a promise that the caller will answer a ToolUse with a ToolResult; a
// hosted capability never comes back to the caller at all. Keeping them apart
// is what lets a tool loop written before this existed keep working: it never
// sees a call it does not know how to answer.
//
// The sources behind an answer arrive as Citation values on the Text they
// support, and Response.Citations flattens them when only the list matters. An
// answer from a search that cannot be checked against its sources is worth
// less than no answer, so a driver whose provider reports sources always
// carries them through.
//
// Response.Hosted reports what each requested capability actually did.
// HostedSkipped is the state worth watching for: a provider can accept a
// search tool and then answer from the model's own memory, and from the
// outside the two answers are identical. A caller that cannot accept that asks
// with Policy: ai.HostedRequired, and gets ErrHostedRequired instead of an
// answer that was never researched.
//
// Not every provider can run every capability. A driver that cannot returns
// ErrNoHosted rather than answering without the search: unlike a Format, which
// a driver can ask for in the prompt, a search cannot be emulated, because a
// driver has no search engine of its own. A caller who would rather have the
// answer anyway asks again without Hosted, which is one visible if rather than
// a silent difference in what an answer is based on.
//
// # Knowing before asking
//
// A driver may describe itself through the optional Capable interface, read
// with CapabilitiesOf, HostedCapabilityOf and SupportsHosted:
//
//	if ai.SupportsHosted(client, ai.Hosted{Kind: ai.HostedWebSearch}) {
//	    // show the "search the web" control
//	}
//
// This is a hint, not a permission. Whether a request succeeds depends on the
// model, the account and the region as much as on the driver, so ErrNoHosted
// and ErrNoFormat stay the source of truth and a caller still handles them.
// What Capabilities is for is the decision taken before the call: whether to
// offer a feature at all, and whether it needs one request or two. A driver
// that does not describe itself reports nothing rather than no, so code
// written against this degrades to asking and handling the answer.
//
// SupportsImages answers the same kind of question for image generation:
//
//	if ai.SupportsImages(client) {
//	    // offer the "generate an image" control
//	}
//
// Image generation is not part of the Client interface - each driver exposes
// its own GenerateImage with its own types, because providers do not share the
// shape - so this hint is only what a UI needs to decide whether to offer it.
//
// # Structured output and hosted capabilities together
//
// Several providers refuse a strict schema and a server-side tool in the same
// call, and the ones that accept it do not all honor both. A driver that knows
// its provider cannot combine them returns ErrFormatWithHosted before the
// request leaves, rather than letting the provider reject it in its own words
// or, worse, return JSON that does not match the schema.
//
// The way around it is two calls: one that searches and answers in prose, and
// one without Hosted that reshapes that prose into the schema. It costs twice
// and it is predictable, which is the better trade when the alternative is a
// well-formed value that quietly does not match what was asked for.
// HostedCapability.WithFormat says in advance which of the two a provider
// needs.
//
// Format.Strict is worth its own warning. Without it a schema is a request:
// the model is shown the shape and asked to follow it, and the classic
// failure is a reply that is valid JSON and is the schema itself rather than
// data matching it - nothing errors, and the result reads as an empty answer.
// With it, providers that enforce schemas also demand that every object set
// additionalProperties to false and list all of its properties in required; a
// schema that forgets one is rejected by the provider, over the network, at
// run time. ValidateStrictSchema checks those rules against a schema literal
// before it is ever sent, and names the path to what is wrong.
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
