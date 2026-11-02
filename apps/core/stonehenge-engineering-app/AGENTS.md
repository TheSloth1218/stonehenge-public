# Stonehenge Webapp Agent Context

Use this file when starting the future ASP.NET operator webapp. The middleware repo is the source of truth for API contracts and deployment wiring.

## Boundaries

- Build the webapp as an operator UI over middleware REST APIs.
- Do not connect the webapp directly to OpenPLC runtimes, Modbus, or PostgreSQL.
- Do not write target, assignment, audit, or engagement rows directly.
- Do not redesign dispatcher ownership, safety enforcement, PLC register maps, or target-feed schema.
- Keep maintenance controls separate from normal operator workflows.

## Default Connectivity

The local Compose stack binds host-facing dashboards and runtime management ports to localhost by default:

- Middleware: `http://127.0.0.1:8080`
- Dashboard: `http://127.0.0.1:8080/dashboard`
- Maintenance panel: `http://127.0.0.1:8080/maintenance`
- FUXA: `http://127.0.0.1:1881`
- OpenPLC Gun 1: `https://127.0.0.1:8443`
- OpenPLC Gun 2: `https://127.0.0.1:8444`
- OpenPLC Gun 3: `https://127.0.0.1:8445`
- OpenPLC Gun 4: `https://127.0.0.1:8446`
- OpenPLC Gun 5: `https://127.0.0.1:8447`
- OpenPLC Gun 6: `https://127.0.0.1:8448`
- OpenPLC Gun 7: `https://127.0.0.1:8449`
- OpenPLC Safety: `https://127.0.0.1:8450`
- OpenPLC Gun 8: `https://127.0.0.1:8451`

For remote development, use an SSH tunnel or a deliberate reverse proxy. Do not expose OpenPLC web management ports or the maintenance API to a public network.

## Middleware APIs To Use

- `GET /ready`: process readiness.
- `GET /api/v1/status`: dashboard-compatible full runtime status.
- `GET /api/v1/dispatcher`: dispatcher readiness, queue, assignments, and uncertainty state.
- `GET /api/v1/target-feed`: ingest/feed state and queue pressure.
- `GET /api/v1/fragments`: recent fragments/targets.
- `GET /api/v1/history`: historical events where enabled.
- `GET /api/v1/safety`: safety controller and permissive state.
- `PUT /api/v1/guns/{id}/mode`: operator mode changes.
- `PUT /api/v1/guns/{id}/auto`: AUTO executor enable/disable where supported.
- `POST /api/v1/ingest/fragments`: target-feed/test ingest only; normal webapp screens should not generate targets unless explicitly building a test tool.

Prefer existing API shapes from `internal/api`. Keep `/api/v1/status` backward compatible for the temporary dashboard and FUXA-facing workflows.

## Maintenance Controls

Maintenance HTTP controls are disabled by default:

```text
STONEHENGE_MAINTENANCE_CONTROLS=false
```

When enabled for a maintenance window, requests must include:

```text
X-Stonehenge-Maintenance-Token: <strong-token>
```

Destructive startup, hard-reset, and complete-reset actions also require the confirmation string:

```text
STONEHENGE_MAINTENANCE_RESET
```

The maintenance script rotates OpenPLC runtime passwords during startup/hard/complete reset. Generated credentials are written to `runtime-secrets/openplc-maintenance.json`, which is ignored by Git. If current credentials are unavailable during a live reset, the script recreates OpenPLC runtime containers/volumes and creates a fresh admin credential before uploading PLC artifacts. Do not display these passwords in the webapp.

## Future Webapp Prompt

Use this prompt when beginning the ASP.NET app:

```text
Build the Stonehenge ASP.NET operator webapp using /opt/stonehenge-middleware as the middleware contract. Read AGENTS.md and AGENTS.webapp.md first. The webapp must use middleware REST APIs only; do not connect directly to OpenPLC, Modbus, or PostgreSQL. Middleware defaults to http://127.0.0.1:8080, and remote access should go through SSH tunnel or reverse proxy. Preserve dispatcher, safety, PLC register, and target-feed boundaries. Maintenance controls are protected and disabled by default; normal operator screens must not expose destructive reset actions unless explicitly building the maintenance area.
```
