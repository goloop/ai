[![deps.dev](https://img.shields.io/badge/deps.dev-insights-4c8dbc)](https://deps.dev/go/github.com%2Fgoloop%2Fai) [![License](https://img.shields.io/badge/license-MIT-brightgreen)](https://github.com/goloop/ai/blob/main/LICENSE) [![License](https://img.shields.io/badge/godoc-YES-green)](https://pkg.go.dev/github.com/goloop/ai) [![Stay with Ukraine](https://img.shields.io/static/v1?label=Stay%20with&message=Ukraine%20♥&color=ffD700&labelColor=0057B8&style=flat)](https://u24.gov.ua/)


# ai

`ai` is a small, provider-agnostic interface for talking to large language model
APIs, plus the shared request and response types every provider driver speaks.
It is the core that goloop's provider packages (`anthropic`, `openai`, `gemini`,
and so on) build on.

Like the standard library's `database/sql` with its drivers, or `log/slog` with
its handlers, this package holds the common contract while a separate package
per provider implements it. A driver depends only on `ai`, so the whole set
stays free of third-party dependencies.

## Installation

```sh
go get github.com/goloop/ai
```

## The interface

```go
type Client interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
	Stream(ctx context.Context, req *Request) iter.Seq2[Chunk, error]
}
```

## Types

- `Role` and `Message` (a role plus a list of content `Part` values).
- `Part`: `Text`, `Image` (multimodal), `ToolUse`, `ToolResult` (tool calling).
- `Tool` and `ToolChoice` for function calling.
- `Request` (model, system, messages, tools, sampling knobs).
- `Response` with `Text()` and `ToolCalls()` helpers; `Chunk` for streaming.
- `Usage` for token counts; `APIError` for normalized provider errors.

## Using a provider

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/anthropic"
)

c := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"))
resp, err := c.Generate(ctx, &ai.Request{
	Model:    anthropic.ModelClaude37SonnetLatest,
	Messages: []ai.Message{ai.UserText("Hello!")},
})
```

Any provider client is an `ai.Client`, so code written against the interface
works with all of them, which makes multi-provider setups straightforward.

## Plumbing for drivers

Drivers reuse the shared configuration and transport:

- `Options` and functional options (`WithBaseURL`, `WithHTTPClient`,
  `WithTimeout`, `WithMaxRetries`, `WithHeader`), built with `NewOptions`.
- `Options.Do` - an HTTP request with retries on 429 and 5xx.
- `SSEEvents` - an iterator over Server-Sent Events data payloads.

Endpoints providers do not share (embeddings, images, audio, files, batches)
are not part of the interface; each driver exposes them as native methods.

## Documentation

Full reference: **[DOC.md](DOC.md)** (Ukrainian: **[DOC.UK.md](DOC.UK.md)**).

## License

MIT - see [LICENSE](LICENSE).
