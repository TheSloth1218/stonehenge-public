# Stonehenge Middleware

Middleware, deployment tooling, OpenPLC sources, and operator dashboards for the Stonehenge OT gun-control simulation.

## Quick Start

```bash
cp .env.example .env
docker compose up -d --build
curl http://127.0.0.1:8080/ready
```

Open:

- Dashboard: `http://127.0.0.1:8080/dashboard`
- Maintenance console: `http://127.0.0.1:8080/maintenance`
- FUXA: `http://127.0.0.1:1881`

The default Compose stack starts PostgreSQL, middleware, FUXA, Safety OpenPLC, and eight Gun OpenPLC runtimes. Host-facing ports bind to `127.0.0.1` by default. For temporary remote dashboard access, set `MIDDLEWARE_HOST_IP=0.0.0.0` and `FUXA_HOST_IP=0.0.0.0`; keep all `OPENPLC_*_HOST_IP` values on `127.0.0.1`. Middleware starts in external target-ingest mode with dispatcher support enabled.

## Common Commands

```bash
go test ./...
go build ./cmd/stonehenge-control
docker compose config
docker compose ps
docker compose logs -f middleware
```

Health and state:

```bash
curl http://127.0.0.1:8080/ready
curl http://127.0.0.1:8080/api/v1/status
curl http://127.0.0.1:8080/api/v1/dispatcher
curl http://127.0.0.1:8080/api/v1/target-feed
```

Submit a target:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/ingest/fragments \
  -H "Authorization: Bearer $STONEHENGE_INGEST_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "external_id":"USEA-TRACKING-NET-000001",
    "bearing":247.3,
    "elevation":41.8,
    "map_x":34.2,
    "map_y":52.7,
    "priority":"HIGH",
    "terrain_type":"LAND",
    "strategic_zone":"SAN_SALVACION",
    "time_to_impact_seconds":75,
    "observed_at":"2026-06-26T14:30:00Z"
  }'
```

Gun, Safety, simulator, and manual-target writes likewise require
`X-Stonehenge-Control-Token: $STONEHENGE_CONTROL_TOKEN`. Empty tokens disable
the corresponding write boundary while health and read-only routes remain available.

## Maintenance Tooling

Dry-run a soft reset for one gun:

```bash
./tools/maintenance/stonehenge-maintenance soft-reset --target gun3
```

Dry-run startup reset planning:

```bash
./tools/maintenance/stonehenge-maintenance startup --confirm STONEHENGE_MAINTENANCE_RESET
```

Live startup, hard reset, and complete reset require `--execute` plus the confirmation string. These workflows rotate OpenPLC runtime passwords before upload and write the new credential file to `runtime-secrets/openplc-maintenance.json` unless `--rotated-credentials` is provided. Current OpenPLC credentials come from `OPENPLC_CREDENTIAL_FILE`, `--credentials`, `OPENPLC_PASSWORD`, or the existing generated credential file. If no current credential file is available during a live reset, the maintenance tool recreates the OpenPLC runtime containers/volumes, creates a fresh admin credential, and then uploads the PLC artifacts.

Maintenance HTTP controls are disabled by default. Enable only on a maintenance network:

```bash
STONEHENGE_MAINTENANCE_CONTROLS=true
STONEHENGE_MAINTENANCE_TOKEN=change-me
```

The maintenance API runs the host-mounted script from `/host` and uses the Docker socket mounted into the middleware container. Keep this enabled only during maintenance windows.

## Repository Map

- `cmd/stonehenge-control`: middleware entrypoint.
- `AGENTS.webapp.md`: future ASP.NET webapp integration context and API boundaries.
- `internal/`: Go service packages. See `internal/README.md`.
- `migrations/`: PostgreSQL schema migrations.
- `tools/maintenance/`: stack startup, soft reset, hard reset, and complete reset orchestration.
- `docs/`: architecture, configuration, and detailed register/API references.

PLC sources and deployment tooling live outside this middleware package in the monorepo:

- `../stonehenge-control-plc/`: shared gun-control PLC source.
- `../stonehenge-safety-plc/`: safety PLC source.
- `../stonehenge-plc-deployer/`: OpenPLC package, validation, upload, startup reload, and fleet deployment tooling.

The target generator remains a separate package. It posts fragments to `POST /api/v1/ingest/fragments`; middleware owns readiness, queueing, dispatcher selection, execution, and lifecycle completion.
