# Stonehenge Target Feed

Standalone target generator for the Stonehenge middleware ingest API.

The feed generates fictional Ulysses fragment tracks and posts them to middleware. It does not select guns, command PLCs, read PostgreSQL, inspect gun state, or join the Stonehenge PLC Compose network.

## Quick Start

```bash
cp .env.example .env
go test ./...
TARGET_MODE=continuous \
TARGET_ENDPOINT=http://127.0.0.1:8080/api/v1/ingest/fragments \
go run ./cmd/stonehenge-target-feed
```

Health and status:

```bash
curl http://127.0.0.1:8090/health
curl http://127.0.0.1:8090/status
```

Docker:

```bash
cp .env.example .env
docker compose up --build
```

## Modes

- `once`: generate and submit one target, then exit.
- `burst`: generate `TARGET_BURST_COUNT` targets, then exit.
- `continuous`: poll middleware readiness and generate targets until stopped or `TARGET_RUN_DURATION_SECONDS` expires.

Examples:

```bash
TARGET_MODE=once go run ./cmd/stonehenge-target-feed
TARGET_MODE=burst TARGET_BURST_COUNT=8 go run ./cmd/stonehenge-target-feed
TARGET_MODE=continuous TARGET_INTERVAL_SECONDS=12 TARGET_JITTER_SECONDS=4 go run ./cmd/stonehenge-target-feed
```

## Configuration

Runtime configuration is environment-based. `configs/config.example.yaml` is a deployment reference, not a loaded config file.

Common settings:

```env
TARGET_ENDPOINT=http://127.0.0.1:8080/api/v1/ingest/fragments
TARGET_MODE=continuous
TARGET_INTERVAL_SECONDS=10
TARGET_JITTER_SECONDS=4
TARGET_SOURCE_ID=USEA-TRACKING-NET
TARGET_MAX_OUTSTANDING_TARGETS=30
TARGET_MAX_PENDING_QUEUE_THRESHOLD=25
TARGET_TTL_SECONDS=600
TARGET_QUEUE_FULL_BACKOFF_SECONDS=30
TARGET_TRANSIENT_ERROR_BACKOFF_SECONDS=10
FEED_LISTEN_ADDRESS=0.0.0.0:8090
```

Set `TARGET_AUTH_TOKEN` only when middleware requires a bearer token. Status output redacts credentials and tokens.

## Middleware Contract

Submit targets to:

```text
POST /api/v1/ingest/fragments
```

Continuous mode polls:

```text
GET /api/v1/dispatcher
GET /api/v1/target-feed/targets
```

The generator sends only when middleware is ready, ownership is reconciled, ingest is allowed, queue depth is below threshold, and outstanding target count is below the configured limit.

Accepted targets return HTTP `202`. Queue-full returns `429` with `{"error":"target_queue_full","state":...}`. Stack-not-ready returns `503` with `Retry-After` and `{"error":"stack_not_ready","state":...}`. Duplicate external IDs are recorded and not retried forever.

## Repository Map

- `cmd/stonehenge-target-feed`: binary entrypoint.
- `internal/`: generator implementation. See `internal/README.md`.
- `configs/`: reference deployment settings.
- `Dockerfile` and `docker-compose.yml`: standalone feed container.

Install under `/opt/stonehenge-targets` for Linux deployments. Keep it separate from the middleware stack and use a routable `TARGET_ENDPOINT`.
