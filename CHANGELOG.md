# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.1] - 2026-08-05

### Documentation
- `FormatEmulated` is spelled out as a different thing from `FormatNative`
  rather than a weaker one: the model was asked, not constrained. The reference
  shows what to do about it - check for `FormatNative` when an enforced shape
  is required, or validate the decoded value - and states plainly that
  `Response.JSON` proves the reply parses, not that it matches the schema.

## [0.4.0] - 2026-08-05

### Added
- `Request.Format` asks for a structured response - `FormatJSON` for any valid
  JSON value, `FormatJSONSchema` for one matching a JSON Schema. Until now only
  a driver's own native request type could ask for JSON, so every caller that
  wanted a structured answer through the provider-agnostic interface had to
  strip code fences and hunt for braces by hand.
- `Response.JSON(&v)` decodes the reply. It unwraps a response wrapped whole in
  a Markdown code fence and then decodes strictly: prose around the value, or a
  second value after it, is an error rather than something to salvage. A reply
  with no text returns `ErrNoText`.
- `Response.Format` reports how the request's `Format` was satisfied:
  `FormatNative` (the provider enforced it), `FormatEmulated` (the driver asked
  for it in the prompt) or `FormatNone`. An emulated format is a request, not a
  guarantee, and a caller can now tell the difference.
- `Format.Instruction` and `Format.AppendInstruction` hold the wording a driver
  puts in the prompt when its provider has no native support, so all drivers
  ask for the same thing in the same words.
- `ErrNoSchema` and `ErrBadSchema`, returned by `Request.Validate` for a schema
  format with no schema or with one that is not a JSON Schema object;
  `ErrBadFormat` for an unrecognised `FormatType`, which is rejected rather than
  guessed at because every driver switches over it; `ErrNoFormat` for a driver
  whose provider can neither enforce the format nor be asked for it usefully;
  `ErrNoText` for `Response.JSON`.

### Changed
- `Request.Validate` now also validates `Format`. A request without one is
  unaffected: `Format` is nil by default and asks for nothing.

## [0.3.0] - 2026-07-11

### Added
- `Message` and `Response` now marshal to and from JSON with a `"type"`
  discriminator per part (`text`, `image`, `tool_use`, `tool_result`), so a
  `Request` or `Response` can be persisted and decoded back into its concrete
  part types. An unknown part type is an error, not a silently dropped part.
- `ErrNoRequest`, returned by `Request.Validate` for a nil request instead of
  panicking.
- `Options.String`/`GoString` redact the API key, so printing a client or its
  options with `%v`/`%+v`/`%#v` no longer leaks the credential.

### Fixed
- `Options.Do` no longer panics on a nil `HTTPClient` (falls back to
  `http.DefaultClient`) and no longer returns `(nil, nil)` for a negative
  `MaxRetries` (treated as zero), so a directly constructed `Options` is safe.

### Changed
- `Usage` fields now carry `input_tokens`/`output_tokens` JSON tags for a
  stable, provider-neutral wire format.

## [0.2.1] - 2026-07-10

### Documentation
- Added reference documentation: `DOC.md` and its Ukrainian mirror `DOC.UK.md`,
  linked from the README.

## [0.2.0] - 2026-07-10

### Removed
- `ErrNoAPIKey` sentinel. It was never returned by any code, and the core
  cannot enforce key presence because keyless providers (Ollama) are valid.

### Changed
- HTTP 500 is no longer retried. Driver requests are non-idempotent POSTs, and
  a 500 may mean the provider already did the work (and charged for it) before
  failing to respond. Retries stay on 429, 502, 503, 504 and 529.
- Backoff now applies equal jitter, so many clients that hit a 429 at once do
  not retry in lockstep.

## [0.1.1] - 2026-07-09

### Fixed
- `Options.Do` now returns the final error response after exhausting retries
  instead of a bare status error, so drivers can read and report the provider's
  error body. Retries are limited to transient statuses (429, 500, 502, 503,
  504, 529); other 5xx are no longer retried. A `Retry-After` header is honored
  (capped at 30s), and the backoff cap is applied before the shift to avoid
  overflow at large retry counts.

### Added
- `SystemText` helper, symmetric with `UserText` and `AssistantText`.
- `Response.ToolCall(name)` to fetch the first tool call by name.
- Test suite (transport, SSE, request/response, options), a property-based fuzz
  test for the SSE parser, and runnable examples.

### Changed
- Clarified the `Chunk.Usage` contract: drivers set it on the Done chunk, with
  zero counts when the provider did not report usage.

## [0.1.0]

First release: the provider-agnostic interface and shared types for goloop AI
provider drivers.

### Added
- `Client` interface: `Generate` and `Stream` (via `iter.Seq2`).
- Shared types: `Role`, `Message`, `Part` (`Text`, `Image`, `ToolUse`,
  `ToolResult`), `Tool`, `ToolChoice`, `Request`, `Response`, `Chunk`, `Usage`.
- `APIError` and the `ErrNoModel`, `ErrNoMessages`, `ErrNoAPIKey` sentinels.
- Driver plumbing: `Options` with functional options, `Options.Do` (HTTP with
  retries and backoff on 429 and 5xx) and `SSEEvents` for Server-Sent Events.
