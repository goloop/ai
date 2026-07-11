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
}
```

Helpers: `Text()` concatenates the text parts, `ToolCalls()` returns all tool
calls, and `ToolCall(name)` returns the first call to a named tool. `Raw` keeps
the provider's original JSON for fields this package does not model.

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
(match with `errors.Is`).

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
