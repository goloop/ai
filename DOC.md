# ai - reference

The full reference for the `ai` package: the provider-agnostic `Client`
interface, the shared request and response model every driver speaks, and the
transport plumbing driver authors reuse.

Ukrainian version: **[DOC.UK.md](DOC.UK.md)**.

## Contents

- [Mental model](#mental-model)
- [The Client interface](#the-client-interface)
- [Building a request](#building-a-request)
- [Generate](#generate)
- [Stream](#stream)
- [Tools](#tools)
- [Structured output](#structured-output)
- [Hosted capabilities](#hosted-capabilities)
- [Knowing before asking](#knowing-before-asking)
- [Strict schemas, checked before sending](#strict-schemas-checked-before-sending)
- [Multimodal input](#multimodal-input)
- [Errors](#errors)
- [For driver authors: Options and transport](#for-driver-authors-options-and-transport)

## Mental model

Package `ai` mirrors the standard library's split between an interface and its
drivers, the way `database/sql` splits from its drivers or `log/slog` splits
from its handlers. This package holds the common contract; a separate package
per provider (`anthropic`, `openai`, `gemini`, and so on) implements it. A
driver depends only on `ai`, so the whole set stays free of third-party
dependencies.

Code written against the interface runs on any provider - swap the constructor,
keep the calls:

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)

var client ai.Client = anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
```

Endpoints providers do not share (embeddings, image generation, audio, files,
batches) are deliberately absent from the interface. Each driver exposes those
as its own native methods, so the common surface stays small and honest.

## The Client interface

```go
type Client interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
	Stream(ctx context.Context, req *Request) iter.Seq2[Chunk, error]
}
```

`Generate` returns a whole `Response`; `Stream` yields `Chunk` values as they
arrive over a range-over-func iterator (Go 1.23+).

## Building a request

A `Request` carries a model, an optional system prompt, the conversation, and
the usual sampling knobs:

```go
type Request struct {
	Model       string
	System      string // optional system prompt
	Messages    []Message
	Tools       []Tool
	ToolChoice  ToolChoice
	MaxTokens   int
	Temperature *float64 // pointer: unset is distinct from an explicit 0
	TopP        *float64
	Stop        []string
	Format      *Format // ask for JSON; see "Structured output"
}
```

Only `Model` and `Messages` are required; `Request.Validate` enforces that and
returns `ErrNoModel` or `ErrNoMessages`. `Temperature` and `TopP` are pointers
so an unset value is distinct from an explicit zero.

A `Message` is a role plus a list of content `Part` values:

```go
type Message struct {
	Role  Role
	Parts []Part
}
```

`Role` is one of `RoleSystem`, `RoleUser`, `RoleAssistant`, `RoleTool`. The
concrete parts are `Text`, `Image`, `ToolUse` and `ToolResult`. `Part` is a
closed interface - it cannot be implemented outside this package, so drivers
switch over it exhaustively.

`Message` and `Response` marshal to and from JSON with each part tagged by a
`"type"` field (`text`, `image`, `tool_use`, `tool_result`), so a `Request` or
`Response` can be persisted, queued or cached and decoded back into its concrete
part types. An unknown `type` is an error rather than a silently dropped part.
`Image.Data` is base64 and `ToolUse.Input` stays raw JSON.

Constructors cover the common single-text-part case:

```go
ai.UserText("What is the capital of France?")
ai.SystemText("You are a terse assistant.")
ai.AssistantText("Paris.")
```

## Generate

```go
resp, err := client.Generate(ctx, &ai.Request{
	Model:    "the-model",
	Messages: []ai.Message{ai.UserText("Say hello.")},
})
if err != nil {
	// handle error
}
fmt.Println(resp.Text())
```

`Response` holds the assistant's output blocks plus bookkeeping:

```go
type Response struct {
	Model      string
	Parts      []Part
	StopReason string
	Usage      Usage
	Raw        json.RawMessage // the provider's original JSON
	Format     FormatMode      // how Request.Format was satisfied
}
```

Helpers: `Text()` concatenates the text parts, `JSON(&v)` decodes that text,
`ToolCalls()` returns all tool calls, and `ToolCall(name)` returns the first
call to a named tool. `Raw` keeps the provider's original JSON for fields this
package does not model.

## Stream

`Stream` returns an `iter.Seq2[Chunk, error]`; range over it:

```go
for chunk, err := range client.Stream(ctx, req) {
	if err != nil {
		// the stream stops after the first error
		break
	}
	fmt.Print(chunk.Text)
	if chunk.Done {
		fmt.Printf("\nusage: %+v\n", chunk.Usage)
	}
}
```

```go
type Chunk struct {
	Text     string    // incremental text delta
	ToolCall *ToolUse  // set when the chunk carries a completed tool call
	Usage    *Usage    // set on the Done chunk
	Done     bool      // marks the final chunk
	Raw      json.RawMessage
}
```

Drivers set `Usage` on the `Done` chunk; its counts are zero when the provider
did not report usage.

## Tools

Describe a callable function with `Tool`, then read back the model's calls:

```go
req := &ai.Request{
	Model:      "the-model",
	Messages:   []ai.Message{ai.UserText("Weather in Kyiv?")},
	Tools:      []ai.Tool{{Name: "get_weather", Description: "Current weather", Schema: schema}},
	ToolChoice: ai.ToolAuto,
}
resp, _ := client.Generate(ctx, req)
for _, call := range resp.ToolCalls() {
	// call.Name, call.Input (json.RawMessage), call.ID
}
```

`ToolChoice` is `ToolAuto` (model decides), `ToolNone` (forbid calls) or
`ToolRequired` (force at least one call). `Tool.Schema` is a JSON Schema object;
drivers pass it through in the shape the provider expects. To answer a tool
call, append a `RoleTool` message with a `ToolResult` whose `ID` matches the
`ToolUse`.

## Structured output

Set `Request.Format` when the reply has to be JSON rather than prose:

```go
resp, err := client.Generate(ctx, &ai.Request{
	Model:    "the-model",
	Messages: []ai.Message{ai.UserText("Draft SEO fields for this article.")},
	Format: &ai.Format{
		Type:   ai.FormatJSONSchema,
		Name:   "seo",
		Schema: json.RawMessage(`{"type":"object", ...}`),
	},
})
if err != nil {
	// handle error
}

var seo SEO
if err := resp.JSON(&seo); err != nil {
	// the model did not answer with the value it was asked for
}
```

```go
type Format struct {
	Type   FormatType      // FormatText (default), FormatJSON, FormatJSONSchema
	Name   string          // schema name, for providers that require one
	Schema json.RawMessage // a JSON Schema; required for FormatJSONSchema
	Strict bool            // ask the provider to enforce the schema exactly
}
```

`FormatJSON` asks for any valid JSON value; `FormatJSONSchema` asks for one
matching `Schema`. `Request.Validate` rejects a schema format with no schema
(`ErrNoSchema`) or with a schema that is not valid JSON (`ErrBadSchema`),
before a driver builds anything out of it.

**Providers differ, and a driver never hides it.** Where the provider supports
structured output natively, the driver uses it. Where it does not, the driver
asks for the format in the prompt, in wording shared by every driver
(`Format.Instruction`), so provider-agnostic code stays portable. A driver that
can do neither returns `ErrNoFormat`; none of them silently drops the request.
`Response.Format` reports which happened:

| `FormatMode` | Meaning |
|---|---|
| `FormatNone` | nothing was requested |
| `FormatNative` | the provider enforced it |
| `FormatEmulated` | the driver asked for it in the prompt |

An emulated format is a request, not a guarantee. The distinction is worth
acting on, not just reading:

```go
resp, err := client.Generate(ctx, req)
if err != nil {
	return err
}

var seo SEO
if err := resp.JSON(&seo); err != nil {
	if resp.Format == ai.FormatEmulated {
		// The model was asked, not constrained. A retry, a smaller schema or
		// a different provider are all reasonable; the request itself is fine.
	}
	return err
}
```

Code that cannot proceed without an enforced shape should say so - check for
`resp.Format == ai.FormatNative` before the call is worth making, or validate
the decoded value afterwards. `Response.JSON` proves the reply parses, not that
it satisfies the schema; no driver validates against `Format.Schema` locally.

### Decoding the reply

`Response.JSON(&v)` decodes the response text. It unwraps a reply that is
wrapped whole in a Markdown code fence - a provider that was only asked in the
prompt tends to add one - and then decodes strictly:

| Reply | Result |
|---|---|
| `{"a":"x"}` | decodes |
| the same value wrapped whole in a fenced block | decodes |
| `Sure! {"a":"x"}` | error: prose around the value |
| `{"a":"x"} {"b":"y"}` | error: two values |
| a fenced block inside prose | error: the reply is prose |

It deliberately does **not** hunt for the first `{` and the last `}`. Salvaging
JSON out of arbitrary text is how a wrong answer gets read as a right one. A
reply with no text at all (only tool calls) gives `ErrNoText`.

## Hosted capabilities

Some providers can run work on their own side, web search first among them.
`Request.Hosted` asks for it:

```go
resp, err := client.Generate(ctx, &ai.Request{
	Model:    "the-model",
	Messages: []ai.Message{ai.UserText("What shipped this week?")},
	Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
})
for _, c := range resp.Citations() {
	fmt.Println(c.Title, c.URL)
}
```

This is a separate field from `Tools`, and that is the whole design. A `Tool`
is a promise that *you* will answer a `ToolUse` with a `ToolResult`. A hosted
capability never comes back to you: the provider runs it and folds the result
into the answer. Putting both in one slice would mean every tool loop ever
written has to learn which calls to answer and which to ignore - code that
still compiles and now behaves differently, the worst kind of change. A loop
that has never heard of `Hosted` keeps working exactly as it did.

### Narrowing the search

```go
ai.Hosted{
	Kind: ai.HostedWebSearch,
	Web: &ai.HostedWeb{
		MaxUses:      3,
		AllowDomains: []string{"example.org"},
		Region:       "UA",
	},
}
```

Every field of `HostedWeb` is a constraint, not a hint. Providers differ in
what they can express: one takes a use limit, another only a list of allowed
domains, a third nothing at all. A driver that cannot express a field you set
returns `ErrNoHosted` rather than running a wider search than you asked for -
an answer built from sources you excluded is worse than no answer.

### Sources

Citations hang off the `Text` they support, because which sentence a source
backs is the point of having sources:

```go
type Citation struct {
	URL       string
	Title     string
	CitedText string // the source's own words, when the provider reports them
	StartByte int    // span of the Text this source backs, when known
	EndByte   int
}
```

Providers report one or the other, rarely both. Some give you the fragment of
the source they used and no position in your answer; some give you a byte range
into your answer and never the source's words. Both zero offsets mean "this
source backs the whole part". A driver whose provider counts in something other
than bytes converts or leaves the range at zero, so a range is never wrong even
when it is absent.

### Did it actually search?

`Response.Hosted` says what became of each capability you asked for:

```go
for _, h := range resp.Hosted {
	switch h.Mode {
	case ai.HostedNative:  // it ran, h.Calls times
	case ai.HostedSkipped: // it was offered and the model did not use it
	}
}
```

`HostedSkipped` is the state worth watching. A provider can accept a search
tool and then answer from the model's own memory, and from the outside the two
answers look identical - which is exactly how a confident invention gets
mistaken for a researched answer. When you cannot accept that, say so:

```go
Hosted: []ai.Hosted{{Kind: ai.HostedWebSearch, Policy: ai.HostedRequired}}
```

and a search that did not happen is `ErrHostedRequired` instead of an answer.

`HostedReport.Calls` is how many times the provider ran it. Hosted work is
usually billed apart from tokens, so this is the only place that cost shows up;
`Usage` stays token-only.

### When a provider cannot

`ErrNoHosted`, before the request leaves. Unlike a `Format`, which a driver can
ask for in the prompt, a search cannot be emulated - a driver has no search
engine of its own. If you would rather have the answer without the search, ask
again without `Hosted`; that is one visible `if` rather than a silent
difference in what an answer is based on.

### Structured output and search together

Several providers refuse a strict schema and a server-side tool in one call. A
driver that knows its provider cannot combine them returns
`ErrFormatWithHosted` before the request leaves, rather than letting the
provider reject it in its own words or return JSON that quietly does not match
the schema.

The way around it is two calls: one that searches and answers in prose, one
without `Hosted` that reshapes that prose into the schema. It costs twice and
it is predictable, which is the better trade when the alternative is a
well-formed value that is not the value you asked for.

### Streaming

`Chunk.Citations` carries sources as their events arrive, and `Chunk.Hosted`
reports on the final chunk. A citation arrives in the chunk the provider
announces it in, which is not necessarily the chunk carrying the text it
supports, so assemble your own list rather than pairing them position by
position.

## Knowing before asking

A driver may describe itself through the optional `Capable` interface, read
with three helpers that also work on a driver that stays silent:

```go
caps := ai.CapabilitiesOf(client)                       // zero value if silent
hc, ok := ai.HostedCapabilityOf(client, ai.HostedWebSearch)
if ai.SupportsHosted(client, ai.Hosted{Kind: ai.HostedWebSearch}) {
    // show the "search the web" control
}
```

`Capabilities` answers three questions the applications were keeping as
hand-written provider tables: which hosted capabilities a driver runs, which
`HostedWeb` settings it accepts (a setting reported `false` is refused with
`ErrNoHosted`, never silently widened), and - through
`HostedCapability.WithFormat` - whether a capability survives in the same call
as a structured format. That last one decides the shape of a feature: a
provider that reports nothing there needs two requests, one that searches in
prose and one that reshapes it; a provider that reports a mode does it in one.

`WithFormat` and `Capabilities.Format` answer in `FormatMode`, not booleans,
so a provider that only asks the model for a schema (`FormatEmulated`) is
never mistaken for one that enforces it (`FormatNative`) - the same
distinction `Response.Format` draws after the call.

**It is a hint, not a permission.** Support also depends on the model, the
account and the region, none of which this value knows. `ErrNoHosted`,
`ErrNoFormat` and `ErrFormatWithHosted` remain the source of truth, and a
caller still handles them. A silent driver reports nothing rather than no, so
code written against `Capabilities` degrades to asking and handling the error.

## Strict schemas, checked before sending

Providers that enforce a schema strictly demand that every object set
`additionalProperties: false` and list every property in `required`. A schema
is a literal, fully known before the first call - and without a check, a typo
in it is discovered as a 400 from the provider, over the network, on a live
key, after a deploy.

```go
if err := ai.ValidateStrictSchema(schema); err != nil {
    // ai: strict schema is not provider-compatible:
    //   properties.stories.items: "required" is missing "placeLocal"
}
```

It walks `properties`, `items`, `prefixItems`, `$defs`/`definitions` and
`anyOf`/`oneOf`/`allOf`, and names the path to the fault. The openai driver
runs it automatically when `Format.Strict` is set; calling it in a test is the
way to catch the mistake at build time. It checks the strict-mode rules and
nothing else - it is not a JSON Schema validator, and says nothing about
whether the schema describes what you meant. Errors match
`ErrBadStrictSchema` with `errors.Is`.

One more wording worth knowing: without `Strict`, a schema is a request, and
the classic failure is a reply that is valid JSON and is the schema itself
rather than data matching it. Nothing errors; the run looks like an empty
answer. `ValidateStrictSchema` cannot catch that - `Response.Format` reporting
`FormatEmulated` is the signal to validate the decoded value yourself.

## Multimodal input

`Image` is an image content part - provide either inline bytes or a URL:

```go
ai.Message{Role: ai.RoleUser, Parts: []ai.Part{
	ai.Text{Text: "What is in this image?"},
	ai.Image{MIME: "image/png", Data: pngBytes},
}}
```

Provide inline `Data` with its `MIME` type, or a `URL` when the provider
supports fetching remote images. Drivers base64-encode `Data` as their wire
format requires.

## Errors

A non-success HTTP response is normalized into `*APIError`:

```go
type APIError struct {
	Status  int             // HTTP status code
	Type    string          // provider error type, when given
	Code    string          // provider error code, when given
	Message string          // human-readable message, when given
	Raw     json.RawMessage // original error body
}
```

Use `errors.As` to inspect it:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == 429 {
	// rate limited
}
```

Request validation returns the sentinels `ErrNoModel` and `ErrNoMessages`
(match with `errors.Is`), and `ErrBadHosted`, `ErrBadHostedPolicy` or
`ErrDupHosted` for a malformed `Hosted`.

Hosted capabilities add two more, both from drivers rather than validation:
`ErrNoHosted` when the provider cannot run what was asked for (or cannot run it
under the given constraints), and `ErrFormatWithHosted` when it cannot combine
one with a structured format. `ErrHostedRequired` says the request was sent and
the model did not use a capability that `HostedRequired` made part of the
contract.

## For driver authors: Options and transport

`ai` carries the plumbing every driver reuses, so drivers stay thin.

`Options` is the shared configuration built from an API key and functional
options:

```go
o := ai.NewOptions(apiKey,
	ai.WithBaseURL("https://api.example.com"),
	ai.WithHTTPClient(httpClient),
	ai.WithTimeout(30*time.Second),
	ai.WithMaxRetries(3),
	ai.WithHeader("X-Custom", "value"),
)
```

`Options.Do` performs an HTTP request with retries and jittered backoff,
returning the final response (including the last failed one, so drivers can read
the provider's error body):

```go
resp, err := o.Do(ctx, http.MethodPost, url, body, headers)
```

`SSEEvents` reads a Server-Sent Events stream as an iterator of `data:` payloads:

```go
for data, err := range ai.SSEEvents(resp.Body) {
	// data is one SSE event's payload
}
```

These three - `Options`, `Options.Do` and `SSEEvents` - are all a driver needs
to speak HTTP and streaming consistently with the rest of the set.
