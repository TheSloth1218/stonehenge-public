# Stonehenge Middleware Agent Notes

This repository is the in-scope Stonehenge middleware service. It belongs under `/opt/stonehenge` on Linux deployments. Keep this guide environment-agnostic and Linux-first; do not assume Windows paths or shell behavior.

## Scope

This service owns:

- REST API and dashboard/HMI-facing state.
- Fragment ingest boundary.
- Assignment coordinator.
- Eight-gun simulation and timing model.
- Simulated PLC adapter.
- Simulated safety controller.
- Runtime state and operational event publishing.

This service must not directly implement the out-of-scope target generator. That is in the separate `stonehenge-targets` repository.

Deferred for Linux deployment phase:

- PostgreSQL persistence.
- Modbus/OpenPLC adapter.
- FUXA integration.
- AD/RBAC enforcement.

## Current Architecture

Flow:

```text
FragmentSource -> Engine/assignment coordinator -> PLC Adapter -> HistorySink
                                      |
                                      v
                              Safety Controller
```

Key packages:

- `cmd/stonehenge-control`: main entrypoint.
- `internal/api`: REST API, dashboard, map asset route, ingest endpoint.
- `internal/fragments`: `Source`, `InternalRandomSource`, `ExternalIngestSource`.
- `internal/sim`: core engine, assignment, gun state machine, timing, operational events.
- `internal/plc`: `Adapter` interface and `SimulatedAdapter`.
- `internal/safety`: `Controller` interface and `SimulatedController`.
- `internal/history`: `Sink` interface, `InMemorySink`, `NoOpSink`.
- `internal/store`: live in-memory runtime store.
- `internal/terrain`: Usea map terrain/priority classification.
- `images`: embedded tactical map image.
- `internal/dashboard/static`: plain HTML/CSS/JS dashboard.

## Important Boundaries

- Keep operational control and safety logic separate.
- Do not add destructive endpoints such as `/destroy`, `/disable-safety`, or `/force-unsafe-fire`.
- Hard safety limits belong to `internal/safety`, not writable control registers.
- Admin override is state/workflow metadata, not a trivial safety bypass.
- `MAINTENANCE` removes guns from automatic assignment but does not bypass normal limits.
- `admin_override_active` is separate from maintenance mode.
- Dashboard reads `/api/v1/status`; it must remain backward compatible.

## Fragment Ingest Contract

Endpoint:

```text
POST /api/v1/ingest/fragments
```

Accepted request fields:

```json
{
  "external_id": "USEA-TRACKING-NET-000001",
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

- `source_id` is currently ignored by the middleware if sent by future clients.
- Accepted fragments must use the same assignment path as internal RNG fragments.
- Duplicate `external_id` values are rejected by the ingest source.

## Modes And Timing

Gun modes:

- `AUTO`
- `MANUAL`
- `MAINTENANCE`

Gun state machine and timing:

- Slew duration is computed by angular delta with configured min/max clamps.
- `ON_TARGET_SETTLING` lasts exactly 5 seconds by default.
- Firing then increments `shot_sequence` exactly once in `completeFire`.
- `POST_FIRE_LOCKOUT` lasts exactly 10 seconds by default.

## Config

Local default:

```text
configs/config.example.json
```

Linux ingest-oriented example:

```text
configs/config.linux.example.json
```

For external target feed testing, use ingest mode:

```bash
export STONEHENGE_CONFIG=/opt/stonehenge/configs/config.linux.example.json
go run ./cmd/stonehenge-control
```

## Commands

Run tests:

```bash
go test ./...
```

Build:

```bash
go build ./cmd/stonehenge-control
```

Run locally:

```bash
go run ./cmd/stonehenge-control
```

Docker:

```bash
docker compose up --build
```

## Integration Test With Target Feed

From `stonehenge-targets`:

```bash
export TARGET_ENDPOINT=http://127.0.0.1:8080/api/v1/ingest/fragments
export TARGET_MODE=burst
export TARGET_BURST_COUNT=8
export FEED_LISTEN_ADDRESS=127.0.0.1:18090
go run ./cmd/stonehenge-target-feed
```

Verify middleware:

```bash
curl http://127.0.0.1:8080/api/v1/status
curl http://127.0.0.1:8080/api/v1/fragments
```

## Future Extension Points

- Add `PostgresHistorySink` behind `internal/history.Sink`.
- Add `ModbusPLCAdapter` behind `internal/plc.Adapter`.
- Add real safety PLC integration behind `internal/safety.Controller`.
- Add additional fragment feed clients behind `internal/fragments.Source`.

Keep simulated mode working whenever adding real integrations.
