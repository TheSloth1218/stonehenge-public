# Stonehenge Target Feed Agent Notes

This repository is the out-of-scope Stonehenge target-feed service. It belongs under `/opt/stonehenge-targets` on Linux deployments. Keep this guide environment-agnostic and Linux-first; do not assume Windows paths or shell behavior.

## Scope

This service only generates fictional Ulysses fragment tracks and submits them to the Stonehenge middleware ingest endpoint.

This service must not:

- Assign guns.
- Communicate with PLCs.
- Access PostgreSQL.
- Read middleware gun state.
- Determine intercept results.
- Include middleware code.
- Join a private Docker network belonging to the Stonehenge stack.

The in-scope middleware, PLCs, HMI, safety controller, and PostgreSQL belong under `/opt/stonehenge`.

## Runtime Contract

Destination endpoint:

```text
TARGET_ENDPOINT=http://<middleware-host>:8080/api/v1/ingest/fragments
```

The target feed posts JSON tracks to the middleware. It can run on the same host, a development workstation, or a separate competition machine.

Payload shape:

```json
{
  "external_id": "USEA-TRACKING-NET-000001",
  "source_id": "USEA-TRACKING-NET",
  "bearing": 247.3,
  "elevation": 41.8,
  "map_x": 34.2,
  "map_y": 52.7,
  "priority": "HIGH",
  "terrain_type": "LAND",
  "strategic_zone": "SAN_SALVACION",
  "time_to_impact_seconds": 75,
  "observed_at": "2026-06-26T14:30:00Z"
}
```

Notes:

- `external_id` is monotonic for the current process and prefixed by `TARGET_SOURCE_ID`.
- Retries preserve the same `external_id` and exact payload.
- The middleware currently accepts the fields it recognizes; `source_id` is feed-side metadata and may be ignored by middleware.

## Key Packages

- `cmd/stonehenge-target-feed`: main entrypoint.
- `internal/config`: environment-based config and endpoint redaction.
- `internal/generator`: Usea map coordinate generation, terrain classification, strategic zones, priority weighting.
- `internal/client`: HTTP submit client with bounded retries.
- `internal/runner`: continuous, burst, and once execution modes.
- `internal/server`: `/health` and `/status`.
- `internal/model`: shared feed request/status types.

## Configuration

Environment variables:

```bash
export TARGET_ENDPOINT=http://127.0.0.1:8080/api/v1/ingest/fragments
export TARGET_INTERVAL_SECONDS=12
export TARGET_REQUEST_TIMEOUT_SECONDS=5
export TARGET_SOURCE_ID=USEA-TRACKING-NET
export TARGET_AUTH_TOKEN=
export TARGET_SEED=
export TARGET_MAX_RETRIES=5
export TARGET_RETRY_DELAY_SECONDS=3
export TARGET_MODE=continuous
export TARGET_BURST_COUNT=10
export FEED_LISTEN_ADDRESS=0.0.0.0:8090
```

Supported modes:

- `continuous`: generate indefinitely at `TARGET_INTERVAL_SECONDS`.
- `burst`: generate `TARGET_BURST_COUNT` tracks, then exit.
- `once`: generate one track, then exit.

Set `TARGET_SEED` for deterministic output during tests.

## Commands

Run tests:

```bash
go test ./...
```

Build:

```bash
go build ./cmd/stonehenge-target-feed
```

Run once:

```bash
TARGET_MODE=once \
TARGET_ENDPOINT=http://127.0.0.1:8080/api/v1/ingest/fragments \
go run ./cmd/stonehenge-target-feed
```

Run continuous:

```bash
TARGET_MODE=continuous \
TARGET_INTERVAL_SECONDS=12 \
TARGET_ENDPOINT=http://127.0.0.1:8080/api/v1/ingest/fragments \
go run ./cmd/stonehenge-target-feed
```

Docker:

```bash
cp .env.example .env
docker compose up --build
```

## Health And Status

```bash
curl http://127.0.0.1:8090/health
curl http://127.0.0.1:8090/status
```

`/status` must never expose bearer tokens or URL credentials. Use `config.RedactEndpoint` for endpoint display.

## Retry Rules

Retry:

- Network errors.
- Timeouts.
- HTTP `429`.
- HTTP `5xx`.

Do not retry:

- Permanent validation/authentication `4xx` responses.

Continuous mode should not exit just because the middleware is temporarily unavailable.

## Integration Test With Middleware

Start middleware in ingest mode, then from this repository:

```bash
export TARGET_ENDPOINT=http://127.0.0.1:8080/api/v1/ingest/fragments
export TARGET_MODE=burst
export TARGET_BURST_COUNT=8
go run ./cmd/stonehenge-target-feed
```

Verify middleware:

```bash
curl http://127.0.0.1:8080/api/v1/status
curl http://127.0.0.1:8080/api/v1/fragments
```

## Deployment Notes

- Install under `/opt/stonehenge-targets`.
- Keep this Compose stack separate from `/opt/stonehenge`.
- Do not depend on middleware container names or private Docker networks.
- Use IPs, DNS names, localhost, or routed remote hosts in `TARGET_ENDPOINT`.
- If deployed remotely, firewall the middleware ingest listener so only the target-feed host can reach it.
