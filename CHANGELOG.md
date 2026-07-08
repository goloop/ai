# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
