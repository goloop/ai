# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-08-11

Minor release: ask a driver what it can do, and catch a strict schema before
the provider does. Everything here is additive.

### Added
- `Capable`, an optional interface a driver may implement to describe itself,
  with `CapabilitiesOf`, `HostedCapabilityOf` and `SupportsHosted` to read it.
  Without it, applications keep the same knowledge as a hand-written table of
  provider names, in as many copies as they have layers, and that table goes
  quietly out of date the day a driver learns something new.
- `Capabilities` reports which hosted capabilities a driver runs, which of the
  `HostedWeb` settings it can express, whether `HostedRequired` can be honored,
  and - through `HostedCapability.WithFormat` - whether a capability survives
  in the same call as a structured format. That last one decides the shape of
  a feature rather than a detail of a call: without it, a search plus a schema
  takes two requests, with it, one.
- `FormatCapability` answers in `FormatMode` rather than booleans, so a
  provider that only asks the model for a schema is not recorded as one that
  enforces it. It is the same distinction `Response.Format` already draws, and
  it would have been lost in a bool.
- `ValidateStrictSchema` and `ErrBadStrictSchema`. A schema is a literal, known
  in full before the first call; a missing entry in `required` should not be
  something the provider tells you over the network, on a live key, after a
  deploy. It walks properties, items, prefixItems, `$defs`/`definitions` and
  `anyOf`/`oneOf`/`allOf`, and names the path to the fault. It is deliberately
  not part of `Request.Validate`: strictness is a provider dialect, and the
  shared validator has no business enforcing one.

### Changed
- `Capabilities` is documented as a hint and not a permission: support depends
  on the model, the account and the region as much as on the driver, so
  `ErrNoHosted`, `ErrNoFormat` and `ErrFormatWithHosted` remain the source of
  truth. A driver that does not describe itself reports nothing rather than no.
- `ErrNoFormat` and `ErrNoHosted` now say what could not be done rather than
  when that became known. A driver that learns of a limitation only from the
  provider's own refusal wraps that refusal in the same sentinel, so a caller
  degrades with one `errors.Is` either way instead of matching English prose in
  an error message; the provider's `APIError` stays reachable with `errors.As`.
- The package documentation states plainly that without `Format.Strict` a
  schema is a request, and names the symptom when the model takes it as one:
  a valid JSON reply that is the schema itself rather than data matching it.

## [1.0.0] - 2026-08-11

First stable release. This package fixes the contract every driver speaks, so
the one thing the interface could not express - a capability the provider runs
on its own side - is settled before the contract is.

### Added
- `Request.Hosted` asks the provider to run a capability itself, web search
  first among them. It is a field of its own rather than a kind of `Tool`: a
  `Tool` is a promise that the caller will answer a `ToolUse` with a
  `ToolResult`, and a hosted capability never comes back to the caller at all.
  A tool loop written before this existed keeps working unchanged, because it
  never sees a call it does not know how to answer.
- `Hosted`, `HostedKind`, `HostedPolicy` and `HostedWeb` say what is wanted.
  The web-search settings (`MaxUses`, `AllowDomains`, `BlockDomains`, `Region`)
  live in `HostedWeb`, so the shape of one capability does not become the shape
  every later one has to wear.
- `Response.Hosted` reports what each requested capability did, as a list
  rather than one value: two capabilities asked for in one request do not share
  a fate. `HostedSkipped` is the state worth having - a provider can accept a
  search tool and then answer from the model's own memory, and the two answers
  are identical from the outside.
- `HostedReport.Calls` carries how many times the provider ran the capability.
  Hosted work is billed apart from tokens, so this is the only place the cost
  of a request shows up; `Usage` stays token-only and comparable.
- `HostedRequired` turns "the model may search" into "the answer must come from
  a search", with `ErrHostedRequired` when it did not.
- `Citation`, `Text.Citations` and `Response.Citations` carry the sources
  behind an answer, attached to the text they support. An answer from a search
  that cannot be checked against its sources is worth less than no answer.
- `Chunk.Citations` and `Chunk.Hosted` give a stream what `Generate` has.
- `ErrNoHosted` for a provider that cannot run the capability, or cannot run it
  under the constraints given, and `ErrFormatWithHosted` for one that cannot
  combine it with a structured format. Neither is emulated: unlike a format, a
  search cannot be asked for in the prompt, because a driver has no search
  engine of its own.
- `Request.HostedByKind` and `Request.HostedReports` so that the drivers agree
  on what a report means instead of each deciding separately.

### Fixed
- The JSON encoding of a `Response` no longer drops `Format`. A stored answer
  that has lost the fact that its format was only asked for reads as a stronger
  answer than it is; `Hosted` and `Citations` round-trip for the same reason.
  All three are omitted when unset, so a response that asked for nothing
  encodes exactly the bytes it did before.

### Changed
- `Request.Validate` rejects an unknown hosted kind or policy, and the same
  kind requested twice.

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
