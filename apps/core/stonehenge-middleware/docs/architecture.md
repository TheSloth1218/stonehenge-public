# Stonehenge Middleware Architecture

## Current Boundaries

The middleware is organized around replaceable boundaries:

```text
FragmentSource -> Engine/assignment coordinator -> PLC Adapter -> HistorySink
                                      |
                                      v
                              Safety Controller
```

`internal/sim.Engine` remains the assignment coordinator and owns the simulated gun timing model for now: target assignment, slewing, settling, fire request, completed firing, and post-fire lockout.

## Deployment Topology

The Linux Compose stack is the current competition-mode deployment shape. It
starts PostgreSQL, middleware, FUXA, the Safety OpenPLC runtime, and eight Gun
OpenPLC runtimes on the private `stonehenge-control` Docker bridge.

Host-facing ports bind to localhost by default:

| Surface | Default URL |
| --- | --- |
| Middleware API/dashboard | `http://127.0.0.1:8080` |
| FUXA | `http://127.0.0.1:1881` |
| Gun 1 OpenPLC web | `https://127.0.0.1:8443` |
| Gun 2 OpenPLC web | `https://127.0.0.1:8444` |
| Gun 3 OpenPLC web | `https://127.0.0.1:8445` |
| Gun 4 OpenPLC web | `https://127.0.0.1:8446` |
| Gun 5 OpenPLC web | `https://127.0.0.1:8447` |
| Gun 6 OpenPLC web | `https://127.0.0.1:8448` |
| Gun 7 OpenPLC web | `https://127.0.0.1:8449` |
| Safety OpenPLC web | `https://127.0.0.1:8450` |
| Gun 8 OpenPLC web | `https://127.0.0.1:8451` |

Remote development should use SSH tunnels or an intentional reverse proxy. The
default deployment does not require legacy host-side runtime aliases. Modbus
port `502` for OpenPLC runtimes is not published on the host; middleware reaches
PLC runtimes by Docker service name.

## Fragment Sources

`internal/fragments.Source` provides the feed boundary.

- `InternalRandomSource` is the development source and preserves the existing terrain, strategic-zone, priority, and map behavior.
- `ExternalIngestSource` accepts API-submitted fragments and tracks duplicate/replay rejection.

Both sources submit through `Engine.SubmitFragment`, so assignment behavior is shared.

## Runtime State Versus History

`internal/store.MemoryStore` holds live runtime state used by the API and dashboard. `internal/history.Sink` receives durable operational events through one centralized engine event path.

Implemented sinks:

- `InMemorySink`
- `NoOpSink`
- `PostgresHistorySink`

PostgreSQL migrations run idempotently at middleware startup. Live runtime state
still stays in memory; PostgreSQL stores operational history and command audit
records.

## PLC Integration

`internal/plc.Adapter` is currently implemented by `SimulatedAdapter` for the
main simulation loop. A separate OpenPLC Modbus command/status service exists
for manual API commands and the first AUTO controller. Phase 3 wraps that
service in a reusable per-gun registry: each configured gun owns its command
service, sequence floor, lifecycle tracker, arbitration state, AUTO executor,
fault-reset state, and Safety gun mapping. Production configures Guns 1-8 as
separate OpenPLC runtimes using the same Gun PLC register protocol. The fleet
dispatcher is the only component that selects queued target ownership. Guns 1-8
are active dispatcher execution candidates in fleet mode; operational
eligibility is still evaluated per gun from configuration, Runtime freshness,
Safety permissive state, arbitration state, active assignment state, and
uncertainty. Gun 1 telemetry offsets are unchanged.

The dispatcher/AUTO split is:

```text
Target feed queue -> Fleet dispatcher -> persistent target ownership
                                  |
                                  v
                          Gun N AUTO executor -> command service
                                              -> OpenPLC -> lifecycle tracker
                                              -> command/engagement history
```

Only one active assignment per gun and one active assignment per target are
supported. There is no per-gun backlog, preemption, post-commit reassignment, or
automatic fault reset in this phase.
The target-feed manager still owns deduplication, bounded queue storage,
priority-then-oldest ordering, and expiration, but it no longer directly calls
AUTO. Each AUTO executor accepts only dispatcher assignments for its gun
and rejects wrong-gun, stale, duplicate, terminal, or disabled assignments.
FUXA observes PLC and middleware state; it is not the AUTO command path.

FUXA reads middleware-owned AUTO status through a separate read-only Modbus TCP
telemetry server:

```text
Gun 1 AUTO controller -> immutable telemetry snapshot
Safety PLC poller ----^
                       -> middleware Modbus TCP telemetry server
                       -> FUXA read-only Modbus tags
```

The telemetry server is internal to the Compose network at `middleware:1502`
and exposes only holding/input register reads for Gun 1 AUTO status, target-feed
status, arbitration state, and Safety PLC status. It does not write OpenPLC
registers, does not publish command registers, and does not query PostgreSQL on
Modbus requests.

The registry is the extension point for later guns:

```text
GunDefinition -> Gun instance
              -> command service + sequence state
              -> arbitration + lifecycle tracker
              -> AUTO state + Safety gun lookup
```

The dispatcher foundation and ownership persistence evaluate all configured guns
with one deterministic evaluator and persist a decision audit. In fleet
production config, Guns 1-8 have `dispatcher_eligible=true` and
`auto_executor_enabled=true`. Guns never inspect the queue directly; dispatcher
ownership is the only path to AUTO command submission.

## Dispatcher, Ownership, And Recovery

Dispatcher modes:

- `disabled`: no target selection or ownership assignment.
- `shadow`: inspect queued targets, evaluate Guns 1-8, calculate scores, and
  persist decisions without ownership, arbitration, sequence allocation,
  command submission, or queue removal.
- `active`: persist a decision, atomically create ownership, then offer the
  assignment to the selected gun executor.

Ownership is stored in `target_assignments`. Active states are unique by target
and by gun. Assignment generation increments on reassignment and executor
updates are guarded by assignment ID plus generation. Decision audit is stored
in `dispatcher_decisions` with top-level target, mode, outcome, selected gun,
eligible count, scoring version, selected score, and JSON candidate results.

Scoring version `dispatcher-v1-angular-elevation-gunid` requires eligibility
before scoring. Score is shortest circular azimuth movement plus weighted
absolute elevation movement. Ties are resolved by lowest gun ID. Azimuth
envelopes are configured as non-wrapping inclusive `[min,max]` degrees in this
phase; movement cost itself uses circular shortest distance near 0/360.

Temporary Safety inhibits do not cause dispatcher requeue. The dispatcher
requires Safety to be configured, connected, fresh, and schema-supported. Gun 1
then retains ownership and enters `waiting_for_safety` until Safety becomes
permissive. Waiting for Safety does not allocate a sequence, hold arbitration,
or submit a PLC command.

Startup reconciliation loads active ownership before dispatching new work.
Unaccepted `assigned` rows are requeued as `restart_reconciled_unaccepted`.
Accepted, waiting, committed, or uncertain rows remain visible for future
recovery handling and are not blindly duplicated. New assignment is blocked
until `ownership_reconciled=true` and `dispatcher_ready=true`.

Dispatcher APIs:

- `GET /api/v1/dispatcher`
- `GET /api/v1/dispatcher/decisions`
- `GET /api/v1/dispatcher/decisions/{id}`
- `GET /api/v1/dispatcher/assignments`
- `GET /api/v1/dispatcher/assignments/{id}`

`POST /api/v1/ingest/fragments` is gated by dispatcher readiness and returns
HTTP 503 with `error=stack_not_ready` and `retry_after_seconds` when the stack
is not ready for new external targets. Already accepted queued targets are not
discarded merely because the stack later degrades.

## Gun Runtime Restart Contract

Gun PLC command identity is middleware-owned and per gun. At startup,
middleware seeds each gun command service from the highest positive
`gun_command_audit.submitted_sequence` for that gun, then allocates the next
command above both that persisted floor and the PLC's currently accepted
sequence. Audit rows with `submitted_sequence=0` are transport-failure records
and are ignored as sequence floors. This preserves monotonic command IDs when an
OpenPLC Runtime restarts with non-retained accepted-sequence registers.

The live Gun PLC shot counter is currently a PLC status counter, not a retained
lifetime counter. If OpenPLC restarts and reports a lower shot sequence,
middleware treats the first fresh post-reconnect status as the new runtime
baseline for later commands and does not classify the backward movement itself
as a successful shot. A later fire command is completed only after that command
observes a shot increment from its own baseline. Lifetime shot history remains
authoritative in PostgreSQL until the PLC project is updated to retain or
restore the counter deliberately.

Position telemetry after a Gun PLC restart is the fresh PLC-reported startup
position, currently the project home position. Middleware does not infer that a
simulated gun retained its pre-restart position. Operational state initializes
fail-safe: no fabricated firing, lockout, active command, arbitration lease,
fault, or maintenance state is restored by middleware unless the PLC or
middleware-owned configuration explicitly reports it. AUTO mode is
middleware-owned: Guns 1-8 are restored from configuration in the Linux fleet
config unless overridden by `GUNS_CONFIG_JSON`.

## Safety PLC

The original `internal/safety.Controller` remains the simulated hard-limit
controller used by the eight-gun simulation. A separate Safety PLC service in
the same package polls `openplc-safety:502` for the Windows-authored Safety PLC
program:

```text
Windows OpenPLC Editor -> openplc-safety runtime
openplc-safety:502 -> middleware Safety PLC poller -> CanFire(gunID)
CanFire -> manual/AUTO firing gate -> configured gun command service
```

Safety status defaults to Modbus input-register reads (`%IW0..%IW79`) with
`SAFETY_PLC_STATUS_REGISTER_TYPE=input`; holding-register fallback is
configurable for runtime diagnostics. The live OpenPLC Runtime v4 holding
layout exposes status `%QW0..%QW79` at raw offsets `0..79` and memory words
`%MW0..` after the configured `%QW` block. `SAFETY_PLC_MW_REGISTER_BASE`
defaults to `80` and is added to logical Safety `%MW` control offsets for
writes. Safety reset pulses logical `MW4` for
`SAFETY_PLC_RESET_PULSE_DURATION`, default `750ms`. The REST control contract
uses `maintenance_mode`, `enabled`, `maintenance_inhibit`, and
`external_inhibit`; legacy prefixed aliases are accepted only when they do not
conflict. Safety control handlers use strict JSON decoding and reject unknown or
empty no-op bodies before any Modbus write. Enforcement is enabled in the Phase
2 lab stack. Unavailable, stale, heartbeat-frozen, unsupported, faulted, or
inhibited safety state denies firing fail-safe. Movement remains allowed.
Manual fire denials happen before Gun 1 sequence allocation or command writes.
AUTO keeps the retained target in `waiting_for_safety` without taking an
arbitration lease until Safety becomes trustworthy and permissive. Denial is not
treated as a Gun PLC fault.

## Maintenance And Reset Operations

Normal operator workflows use middleware APIs. Maintenance reset workflows are
host-side orchestration and are deliberately gated:

- `soft-reset`: restarts middleware, FUXA, Safety, all guns, or a selected gun
  container without uploading PLC code or resetting PostgreSQL.
- `startup`: starts the stack, backs up PostgreSQL, recreates the database,
  runs migrations through middleware startup, rotates OpenPLC runtime
  passwords, uploads Gun/Safety PLC artifacts, and returns guns to AUTO.
- `hard-reset`: backs up PostgreSQL, stops the stack, recreates the database,
  starts the stack, rotates OpenPLC runtime passwords, uploads PLC artifacts,
  and returns guns to AUTO.
- `complete-reset`: alias for the hard-reset workflow for dashboard wording.

Live startup, hard-reset, and complete-reset require `--execute --confirm
STONEHENGE_MAINTENANCE_RESET`. The HTTP maintenance API and `/maintenance`
panel are disabled unless `STONEHENGE_MAINTENANCE_CONTROLS=true` and requests
include `X-Stonehenge-Maintenance-Token`.

OpenPLC runtime credentials are not stored in Git. Startup and hard-reset use
the current credential from `OPENPLC_CREDENTIAL_FILE`, `OPENPLC_PASSWORD`, or
the existing generated file at `runtime-secrets/openplc-maintenance.json`; then
they rotate all runtime passwords and rewrite that generated credential file
with mode `0600`. If no current credential is available during a live reset, the
maintenance tool recreates only the OpenPLC runtime containers and volumes,
creates the first `admin` user with a new generated password, stores it in the
same protected credential file, and then uploads the PLC artifacts.

## Configuration Reference

Operational defaults live in `.env.example`, `docker-compose.yml`, and
`configs/config.linux.example.json`. A grouped environment-variable reference is
kept in `docs/configuration.md`.

Host-published OpenPLC management ports are localhost-only by default and should
stay that way. Temporary remote access should expose only the middleware
dashboard/maintenance port and FUXA port; Docker-internal service-name
networking remains the path for middleware, FUXA, PostgreSQL, and OpenPLC
runtime communication.

## Deferred Work

AD/RBAC enforcement and the future ASP.NET operator webapp are intentionally
deferred. The temporary dashboard, maintenance panel, FUXA, fleet dispatcher,
eight OpenPLC gun runtimes, Safety PLC polling, PostgreSQL history, and external
target ingest are active in the current middleware stack.
