# Internal Package Guide

This service is intentionally narrow: generate fictional target observations and submit them to middleware. It must not assign guns, command PLCs, or write middleware state directly.

## Runtime Flow

```text
config
  -> generator
  -> runner mode
  -> client
  -> middleware ingest
  -> status server
```

Continuous mode adds middleware readiness checks before target generation:

```text
GET /api/v1/dispatcher
GET /api/v1/target-feed/targets
```

## Packages

- `client`: HTTP client for ingest submission and readiness polling. It classifies accepted, duplicate, stale, queue-full, 503 Retry-After, permanent, and transient responses.
- `config`: environment parsing, defaults, duration conversion, validation, and endpoint redaction for status output.
- `generator`: deterministic or random Ulysses fragment generation, terrain/zone classification, priority selection, target IDs, and JSON payload shape.
- `model`: shared request, submit-result, readiness, and status types.
- `runner`: `once`, `burst`, and `continuous` mode orchestration. Continuous mode preserves a generated payload across retryable failures until accepted, terminal, expired, or shutdown.
- `server`: `/health` and `/status` HTTP server plus status counters.

## Continuous Mode Rules

- Poll middleware readiness before sending.
- Wait the startup stabilization delay after readiness first becomes true, then recheck.
- Keep target generation bounded by middleware outstanding count and queue depth.
- Generate a new target only after the current pending target is accepted, terminal, duplicate, expired, or intentionally stopped.
- Preserve the exact same external ID and JSON payload across network errors, HTTP `503`, and HTTP `429`.
- Honor `Retry-After` on `503`.
- Use queue-full backoff on `429`.
- Use transient network backoff for timeouts and connection failures.
- Record `expired_unsent` when local TTL expires before acceptance.
- On SIGINT/SIGTERM, stop cleanly and expose any pending target in status output.

## Middleware Response Handling

- `2xx`: accepted unless body contains `{"duplicate":true}`.
- `409` with duplicate external ID text: duplicate terminal response.
- `410`: stale/expired terminal response.
- `429`: queue-full retryable response.
- `503`: retryable response; `Retry-After` wins when present.
- Other `5xx` and network errors: retryable.
- Other `4xx`: permanent rejection.

## Status Counters

`/status` includes generated, accepted, duplicate, rejected, retry, HTTP 503, HTTP 429, expired-unsent, and failed counts. It also reports the last generated ID, pending external ID, redacted endpoint, source ID, mode, service start time, and last successful submission time.

## Testing

Use mocked HTTP tests for retry correctness:

```bash
go test ./...
```

Build:

```bash
go build ./cmd/stonehenge-target-feed
```
