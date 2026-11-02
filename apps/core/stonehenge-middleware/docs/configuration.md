# Configuration Reference

This file lists the operational environment variables used by the current Linux
Compose deployment and host-side maintenance tooling. `.env.example` contains
the recommended local defaults.

## Compose Exposure

These variables control host bindings only. Defaults are localhost-only.
For temporary remote operator access, expose only `MIDDLEWARE_HOST_IP` and
`FUXA_HOST_IP`. Keep every `OPENPLC_*_HOST_IP` value on `127.0.0.1`; middleware
and FUXA still use Docker service names and internal ports for container-to-container traffic.

| Variable | Default | Purpose |
| --- | --- | --- |
| `MIDDLEWARE_HOST_IP` | `127.0.0.1` | Host interface for middleware API, dashboard, and maintenance panel. |
| `MIDDLEWARE_PORT` | `8080` | Host middleware port. |
| `FUXA_HOST_IP` | `127.0.0.1` | Host interface for FUXA. |
| `FUXA_PORT` | `1881` | Host FUXA port. |
| `OPENPLC_IMAGE` | `ghcr.io/autonomy-logic/openplc-runtime:latest` | OpenPLC runtime container image. |
| `OPENPLC_CONTROL_HOST_IP` | `127.0.0.1` | Host interface for Gun 1 OpenPLC web/discovery. |
| `OPENPLC_WEB_PORT` | `8443` | Gun 1 OpenPLC web port. |
| `OPENPLC_DISCOVERY_PORT` | `33333` | Gun 1 UDP discovery port. |
| `OPENPLC_GUN2_HOST_IP`..`OPENPLC_GUN8_HOST_IP` | `127.0.0.1` | Host interfaces for Guns 2-8 OpenPLC web/discovery. |
| `OPENPLC_GUN2_WEB_PORT`..`OPENPLC_GUN8_WEB_PORT` | `8444`..`8451` | Gun 2-8 OpenPLC web ports. |
| `OPENPLC_GUN2_DISCOVERY_PORT`..`OPENPLC_GUN8_DISCOVERY_PORT` | `33334`..`33341` | Gun 2-8 UDP discovery ports. |
| `OPENPLC_SAFETY_HOST_IP` | `127.0.0.1` | Host interface for Safety OpenPLC web/discovery. |
| `OPENPLC_SAFETY_WEB_PORT` | `8450` | Safety OpenPLC web port. |
| `OPENPLC_SAFETY_DISCOVERY_PORT` | `33340` | Safety UDP discovery port. |

## Middleware Process

| Variable | Default | Purpose |
| --- | --- | --- |
| `STONEHENGE_CONFIG` | `/app/configs/config.linux.example.json` in Compose | JSON config file loaded before env overrides. |
| `HISTORY_ADAPTER` | `postgres` in Compose | `memory`, `noop`, or `postgres`. |
| `GUNS_CONFIG_JSON` | unset | Optional JSON override for the configured gun fleet. Leave unset for Guns 1-8 from `configs/config.linux.example.json`. |

## PostgreSQL

| Variable | Default | Purpose |
| --- | --- | --- |
| `POSTGRES_HOST` | `postgres` | Middleware PostgreSQL host. |
| `POSTGRES_PORT` | `5432` | Middleware PostgreSQL port. |
| `POSTGRES_DB` | `stonehenge` | Database name. |
| `POSTGRES_USER` | `stonehenge_writer` | Database user. |
| `POSTGRES_PASSWORD` | `stonehenge_dev_password` in Compose | Database password. Set a deployment-specific value. |
| `POSTGRES_SSLMODE` | `disable` | PostgreSQL SSL mode. |
| `POSTGRES_MAX_CONNECTIONS` | `10` | Middleware pool limit. |
| `POSTGRES_WRITE_QUEUE_SIZE` | `1000` | Asynchronous history write queue size. |
| `POSTGRES_RETRY_INTERVAL_SECONDS` | `5` | Retry delay for failed history writes. |

## Gun PLC Runtime

| Variable | Default | Purpose |
| --- | --- | --- |
| `PLC_HOST` | `openplc-control-gun1` | Legacy/single-gun PLC host; fleet guns normally use per-gun config. |
| `PLC_PORT` | `502` | PLC Modbus port. |
| `PLC_UNIT_ID` | `1` | PLC Modbus unit ID. |
| `PLC_COMMAND_ACK_TIMEOUT_SECONDS` | `5` | Wait for PLC command acknowledgment. |
| `PLC_POLL_INTERVAL_MS` | `100` | PLC status poll interval. |
| `PLC_LIFECYCLE_POLL_INTERVAL_MS` | `200` | Lifecycle tracker poll interval. |
| `PLC_LIFECYCLE_COMPLETION_TIMEOUT_SECONDS` | `30` | Timeout waiting for terminal PLC result. |
| `PLC_LIFECYCLE_MAX_CONSECUTIVE_READ_ERRORS` | `5` | Consecutive read errors before PLC state is considered degraded. |
| `PLC_AUTO_READINESS_POLL_INTERVAL_MS` | `500` | AUTO executor readiness poll interval. |
| `PLC_AUTO_SAFETY_FIRE_PERMISSIVE` | `false` | Lab-only permissive override. Keep false for Safety PLC enforcement. |
| `PLC_FAULT_ON_SAFETY_BLOCKED` | `false` | Whether simulated safety blocks fault simulated gun commands. |
| `PLC_FAULT_RESET_TIMEOUT` | `30s` | Fault reset operation timeout. |
| `PLC_STARTUP_MODE` | `manual` | Startup mode: `manual`, `auto`, or `maintenance`. |
| `PLC_STARTUP_MODE_RETRY_TIMEOUT` | `5m` in Compose | Retry window for applying startup mode. |

## Middleware Modbus Telemetry

This is read-only telemetry for FUXA and diagnostics. It is exposed inside the
Compose network as `middleware:1502`, not as a host port.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PLC_AUTO_TELEMETRY_MODBUS_ENABLED` | `true` | Enable middleware read-only Modbus telemetry server. |
| `PLC_AUTO_TELEMETRY_MODBUS_ADDR` | `0.0.0.0:1502` | Bind address inside the middleware container. |
| `PLC_AUTO_TELEMETRY_UNIT_ID` | `1` | Modbus unit ID. |
| `PLC_AUTO_TELEMETRY_POLL_INTERVAL` | `500ms` | Snapshot poll interval. |
| `PLC_AUTO_TELEMETRY_STALE_AFTER` | `3s` | Telemetry stale threshold. |

## Safety PLC

| Variable | Default | Purpose |
| --- | --- | --- |
| `SAFETY_PLC_ENABLED` | `true` | Enable Safety OpenPLC polling/control. |
| `SAFETY_PLC_ENFORCEMENT` | `true` in Compose | Deny fire when Safety is unavailable, stale, faulted, inhibited, or non-permissive. |
| `SAFETY_PLC_HOST` | `openplc-safety` | Safety PLC Docker service name. |
| `SAFETY_PLC_PORT` | `502` | Safety PLC Modbus port. |
| `SAFETY_PLC_UNIT_ID` | `1` | Safety PLC unit ID. |
| `SAFETY_PLC_POLL_INTERVAL` | `250ms` | Safety status poll interval. |
| `SAFETY_PLC_STALE_AFTER` | `2s` | Safety freshness threshold. |
| `SAFETY_PLC_STATUS_REGISTER_TYPE` | `input` | Safety status register type, usually `input`. |
| `SAFETY_PLC_MW_REGISTER_BASE` | `80` | Base offset for Safety `%MW` control writes in Runtime v4 layout. |
| `SAFETY_PLC_RESET_PULSE_DURATION` | `750ms` | Safety reset pulse duration. |

## Target Feed And Dispatcher

| Variable | Default | Purpose |
| --- | --- | --- |
| `TARGET_FEED_ENABLED` | `true` | Enable external fragment ingest and queueing. |
| `TARGET_FEED_MAX_PENDING` | `25` | Max pending queued target-feed records. Active assignments do not count. |
| `TARGET_FEED_TARGET_TTL` | `2m` | Accepted target TTL. |
| `TARGET_FEED_HEALTHY_WINDOW` | `30s` | Freshness window used for target-feed health. |
| `TARGET_FEED_DISPATCH_POLL_INTERVAL_MS` | `500` | Target-feed dispatch polling interval. |
| `DISPATCHER_ENABLED` | `true` | Enable fleet dispatcher. |
| `DISPATCHER_MODE` | `active` | `disabled`, `shadow`, or `active`. |
| `DISPATCHER_POLL_INTERVAL_MS` | `500` | Dispatcher polling interval. |
| `DISPATCHER_ASSIGNMENT_LEASE_DURATION` | `90s` | Assignment lease duration. |
| `DISPATCHER_LEASE_RENEWAL_INTERVAL` | `30s` | Lease renewal cadence. |
| `DISPATCHER_RECONCILIATION_INTERVAL` | `5s` | Ownership reconciliation cadence. |
| `DISPATCHER_SHADOW_DECISION_RETENTION` | `10m` | Retention for shadow decisions. |
| `DISPATCHER_ELEVATION_WEIGHT` | `1` | Elevation cost weight in dispatcher scoring. |

## Protected Controls

| Variable | Default | Purpose |
| --- | --- | --- |
| `STONEHENGE_CONTROL_TOKEN` | unset | Required `X-Stonehenge-Control-Token` value for every gun, Safety, simulator, and manual-target mutation. An unset token disables those writes. |
| `STONEHENGE_INGEST_TOKEN` | unset | Required bearer token for `POST /api/v1/ingest/fragments`. An unset token disables fragment ingest. |
| `STONEHENGE_FUXA_PROXY_TOKEN` | unset | Required `X-Stonehenge-Fuxa-Proxy-Token` value for the FUXA HMI control proxy. The Engineering app supplies it server-side. |
| `STONEHENGE_TEST_CONTROLS` | `false` | Enable lab-only test-control API. Keep disabled outside controlled testing. |
| `STONEHENGE_TEST_TOKEN` | unset | Token for test-control API when enabled. |
| `STONEHENGE_MAINTENANCE_CONTROLS` | `false` | Enable `/api/v1/maintenance` and `/maintenance` action execution. |
| `STONEHENGE_MAINTENANCE_TOKEN` | unset | Required `X-Stonehenge-Maintenance-Token` value when maintenance controls are enabled. |
| `STONEHENGE_MAINTENANCE_SCRIPT` | `/host/stonehenge-middleware/tools/maintenance/stonehenge-maintenance` | Maintenance script path inside middleware container. Compose mounts the monorepo at `/host` when maintenance controls are used. |
| `STONEHENGE_MAINTENANCE_TIMEOUT` | `20m` | Maximum runtime for a maintenance action launched through the API. |
| `STONEHENGE_HOST_REPO` | monorepo parent path in Compose | Host path mounted into maintenance runner containers as `/host`. In the monorepo this should resolve to `/opt/Stonehenge`. |
| `STONEHENGE_MAINTENANCE_RUNNER_IMAGE` | `stonehenge-middleware-middleware` | Local image used for detached live startup/hard-reset runner containers. |
| `STONEHENGE_MIDDLEWARE_URL` | `http://127.0.0.1:8080` in host tool | Middleware URL used by `tools/maintenance/stonehenge-maintenance`. |
| `COMPOSE_PROJECT_NAME` | `stonehenge-middleware` in Compose examples | Keeps container/volume/network names stable when maintenance is invoked from `/host` inside the middleware container. |

## Host-Side OpenPLC Maintenance Credentials

These variables are used by the monorepo `stonehenge-plc-deployer` tooling and
`tools/maintenance/stonehenge-maintenance`, not by the middleware process.

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENPLC_CREDENTIAL_FILE` | unset | Protected JSON file with current OpenPLC `username` and `password`. |
| `OPENPLC_USERNAME` | prompt or `admin` in init/recovery | OpenPLC username. |
| `OPENPLC_PASSWORD` | prompt when needed | Current OpenPLC password. Avoid shell history exposure. |
| `OPENPLC_NEW_PASSWORD` | generated | Optional requested new password for rotation/init. If unset, a random password is generated. |
| `OPENPLC_ROTATED_CREDENTIAL_FILE` | `runtime-secrets/openplc-maintenance.json` | Output file for rotated/generated runtime credentials. |
| `OPENPLC_VOLUME_PREFIX` | Compose project name or repo directory name | Volume-name prefix used when credential recovery has to remove OpenPLC runtime volumes. |
| `COMPOSE_PROJECT_NAME` | repo directory name | Docker Compose project name; affects generated network and volume names. |
| `STONEHENGE_PLC_DEPLOYER_ROOT` | `../stonehenge-plc-deployer` from middleware root | Optional override for the canonical OpenPLC deployer checkout. |

## Integration Test Variable

`STONEHENGE_POSTGRES_TEST_DSN` enables the optional PostgreSQL integration test
in `internal/history`; it is not needed for normal operation.
