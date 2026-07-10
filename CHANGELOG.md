# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
