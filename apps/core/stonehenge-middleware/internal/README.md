# Internal Package Guide

This directory contains the middleware runtime. Packages are intentionally split around ownership boundaries: API, dispatcher, PLC IO, Safety, history, and target feed.

## Request And Runtime Flow

```text
target generator / dashboard / operator
  -> internal/api
  -> targetfeed or gun/manual endpoints
  -> dispatcher / arbitration
  -> autocontrol
  -> plc command service
  -> OpenPLC runtimes
  -> lifecycle, history, dashboard state
```

## Packages

- `api`: REST routes, dashboard routes, ingest boundary, manual gun controls, Safety controls, dispatcher visibility, and protected maintenance API.
- `arbitration`: per-gun mode and ownership state. It prevents AUTO, manual, maintenance, and reset workflows from overlapping unsafely.
- `autocontrol`: automatic gun executor. It accepts assignments for a gun, checks readiness and Safety, submits PLC commands, and commits lifecycle evidence.
- `config`: JSON and environment configuration loading/validation.
- `dashboard`: embedded static web assets for `/dashboard` and `/maintenance`.
- `dispatcher`: eight-gun target selection, eligibility, assignment leasing, reconciliation, and decision metadata.
- `fragments`: internal RNG and external ingest fragment sources, including duplicate/replay handling.
- `guns`: registry of configured gun instances and their PLC, AUTO, arbitration, and lifecycle services.
- `history`: history sink interfaces plus memory/noop/PostgreSQL implementations, migrations, command audit, engagement audit, dispatcher persistence, and Safety events.
- `maintenance`: disabled-by-default wrapper for protected host maintenance scripts. It allowlists reset actions and enforces reset confirmation.
- `model`: shared API/runtime DTOs for guns, fragments, events, and status.
- `plc`: simulated PLC adapter, OpenPLC Modbus command/status service, command audit model, and lifecycle tracker.
- `safety`: simulated Safety controller plus Safety OpenPLC Modbus polling/control service.
- `sim`: older in-process simulation engine and assignment path used by dashboard/status compatibility.
- `store`: in-memory live state for dashboard-facing guns, fragments, and events.
- `targetfeed`: external target-feed queue, readiness gates, duplicate/stale handling, and dispatcher handoff.
- `telemetry`: Modbus telemetry server for AUTO/runtime state visibility.
- `terrain`: Usea map terrain and strategic-zone classification.
- `testcontrol`: explicitly gated test pause/resolution hooks for race and dispatcher validation.

## Boundaries

- Target generators never select guns or command PLCs; they only submit fragments.
- Dispatcher owns automatic gun selection and assignment ownership.
- PLC services only command the addressed gun runtime and never bypass Safety.
- Safety decisions are separate from operational mode and dispatcher eligibility.
- Maintenance reset APIs are not normal operational controls and remain disabled unless explicitly enabled with a token.
