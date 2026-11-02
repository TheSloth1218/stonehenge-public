# Gun 1 OpenPLC Integration

This phase adds a narrow reusable command/status path for the existing Gun 1
OpenPLC runtime. It does not modify the PLC program, register map, simulated
assignment loop, FUXA project, or safety PLC design.

Phase 3 wrapped this proven Gun 1 path in a reusable per-gun registry. Later
dispatcher phases added separate OpenPLC runtime and registry definitions for
Guns 2-8. The Gun control register map, sequence commit-last behavior, REST
response shapes, and FUXA Gun 1 telemetry offsets remain unchanged. Additional
guns should be added as new registry definitions and runtime containers rather
than by copying the Gun 1 implementation.

## Docker Network

The Compose stack uses the `stonehenge-control` bridge network with static IPAM:

| Service | DNS name | Fixed IP |
| --- | --- | --- |
| Gun 1 OpenPLC | `openplc-control-gun1` | `172.28.0.10` |
| Safety OpenPLC | `openplc-safety` | `172.28.0.11` |
| Gun 2 OpenPLC | `openplc-control-gun2` | `172.28.0.12` |
| FUXA | `fuxa` | `172.28.0.20` |
| Middleware | `middleware` | `172.28.0.30` |
| PostgreSQL | `postgres` | `172.28.0.40` |

FUXA should be manually changed to connect to:

```text
Host: 172.28.0.10
Port: 502
Modbus unit ID: 1
```

The middleware uses Docker DNS and connects to:

```text
Gun 1: openplc-control-gun1:502
Gun 2: openplc-control-gun2:502
```

OpenPLC web deployment endpoints bind to localhost by default:

```text
Safety PLC:        https://127.0.0.1:8450
Gun 1 control PLC: https://127.0.0.1:8443
Gun 2 control PLC: https://127.0.0.1:8444
```

OpenPLC Modbus TCP remains internal to the Compose network; port `502` is not
published on the Linux host. Use SSH tunnels or deliberate reverse proxying for
remote maintenance.

## Environment

```bash
PLC_HOST=openplc-control-gun1
PLC_PORT=502
PLC_UNIT_ID=1
PLC_COMMAND_ACK_TIMEOUT_SECONDS=5
PLC_POLL_INTERVAL_MS=100
PLC_AUTO_TELEMETRY_MODBUS_ENABLED=true
PLC_AUTO_TELEMETRY_MODBUS_ADDR=0.0.0.0:1502
PLC_AUTO_TELEMETRY_UNIT_ID=1
PLC_AUTO_TELEMETRY_POLL_INTERVAL=500ms
PLC_AUTO_TELEMETRY_STALE_AFTER=3s
PLC_FAULT_RESET_TIMEOUT=30s
```

## Register Map

FUXA displays one-based holding-register addresses. The middleware uses
`github.com/goburrow/modbus`, which sends zero-based PDU addresses.

The proven effective translation is:

- Command `%MW0..%MW12`: FUXA `1025..1037`, Goburrow PDU `1024..1036`.
- Status `%QW20..%QW38`: FUXA `21..39`, Goburrow PDU `20..38`.

Command holding-register writes:

| PLC variable | FUXA one-based address | Goburrow zero-based PDU address |
| --- | --- | --- |
| `%MW0` CommandSequenceLow | `1025` | `1024` |
| `%MW1` CommandSequenceHigh | `1026` | `1025` |
| `%MW2` Mode | `1027` | `1026` |
| `%MW3` TargetAzimuth | `1028` | `1027` |
| `%MW4` TargetElevation | `1029` | `1028` |
| `%MW5` FragmentLow | `1030` | `1029` |
| `%MW6` FragmentHigh | `1031` | `1030` |
| `%MW7` FireRequest | `1032` | `1031` |
| `%MW8` Maintenance | `1033` | `1032` |
| `%MW9` AdminOverride | `1034` | `1033` |
| `%MW10` FaultReset | `1035` | `1034` |
| `%MW11` SafetyFirePermissive | `1036` | `1035` |
| `%MW12` SafetyExtendedPermissive | `1037` | `1036` |

Status holding-register reads are contiguous from Goburrow PDU address `20`
through `38`:

| PLC variable | FUXA one-based address | Goburrow zero-based PDU address |
| --- | --- | --- |
| `%QW20` AcceptedSequenceLow | `21` | `20` |
| `%QW21` AcceptedSequenceHigh | `22` | `21` |
| `%QW22` State | `23` | `22` |
| `%QW23` CurrentAzimuth | `24` | `23` |
| `%QW24` CurrentElevation | `25` | `24` |
| `%QW25` ActiveFragmentLow | `26` | `25` |
| `%QW26` ActiveFragmentHigh | `27` | `26` |
| `%QW27` TargetAccepted | `28` | `27` |
| `%QW28` OnTarget | `29` | `28` |
| `%QW29` Settling | `30` | `29` |
| `%QW30` Firing | `31` | `30` |
| `%QW31` Lockout | `32` | `31` |
| `%QW32` Ready | `33` | `32` |
| `%QW33` FaultActive | `34` | `33` |
| `%QW34` FaultCode | `35` | `34` |
| `%QW35` ShotSequenceLow | `36` | `35` |
| `%QW36` ShotSequenceHigh | `37` | `36` |
| `%QW37` SettlingSeconds | `38` | `37` |
| `%QW38` LockoutSeconds | `39` | `38` |

Angles are encoded in tenths of a degree. Signed PLC words, such as negative
extended elevation values, are converted through signed `int16` and transported
as unsigned `uint16` words.

## Sequencing

The command service serializes submissions for Gun 1. Each command reads the
current accepted sequence, increments it with normal unsigned 32-bit rollover,
writes `%MW1` through `%MW12` as the payload at PDU addresses `1025..1036`,
then writes `%MW0` `CommandSequenceLow` last at PDU address `1024` as the
commit register.

The API reports success only after status shows the submitted sequence in
`AcceptedSequenceHigh/Low`. A Modbus write alone is not treated as success.

The result distinguishes:

- transport failure
- acknowledgment timeout
- PLC fault/rejection
- accepted command

## API

Read current PLC status. This is the safe read-only diagnostic path; it reads
status PDU addresses `20..38`, returns `raw_registers`, and decodes the same
words into named status fields:

```bash
curl http://localhost:8080/api/v1/guns/1/plc-status
```

Submit a manual Gun 1 command:

```bash
curl -X POST http://localhost:8080/api/v1/guns/1/commands/manual \
  -H "X-Stonehenge-Control-Token: $STONEHENGE_CONTROL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "target_azimuth": 120.0,
    "target_elevation": 30.0,
    "fragment_id": 1234,
    "fire_request": false,
    "maintenance": false,
    "admin_override": false,
    "fault_reset": false,
    "safety_fire_permissive": true,
    "safety_extended_permissive": false
  }'
```

The manual endpoint forces Manual mode (`1`) and never accepts arbitrary Modbus
addresses or caller-supplied command sequences. Middleware requires the control
token before it reaches the PLC command service.

## Middleware Operational Mode And Fault Reset

Middleware is the sole Gun 1 operational-mode authority in this phase. FUXA
supervisory actions should call middleware REST APIs and must not pulse OpenPLC
mode or reset registers directly.

```text
Manual API -----------+
AUTO Controller ------+-> Arbitration -> Shared Command Service -> PLC
Fault Reset API ------+
```

Modes:

- `manual`: manual PLC commands may be submitted when no AUTO or reset lifecycle is active.
- `auto`: AUTO target execution may submit through the shared command service.
- `maintenance`: blocks manual commands, AUTO engagements, and new reset attempts.
- `auto_suspended`: controller-owned state after AUTO faults or protocol failures.

Read mode:

```bash
curl http://localhost:8080/api/v1/guns/1/mode
```

Request mode:

```bash
curl -X PUT http://localhost:8080/api/v1/guns/1/mode \
  -H "X-Stonehenge-Control-Token: $STONEHENGE_CONTROL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"mode":"manual"}'
```

The legacy AUTO endpoint remains compatible:

- `PUT /api/v1/guns/1/auto` with `{"enabled":true}` validates and requests mode `auto`.
- `PUT /api/v1/guns/1/auto` with `{"enabled":false}` requests mode `manual`.

Conflicts use HTTP `409` and include a structured reason:

```json
{
  "error": "command_conflict",
  "reason": "fault_reset_active",
  "mode": "manual"
}
```

Fault reset is operator-driven:

```bash
curl -X POST http://localhost:8080/api/v1/guns/1/fault-reset \
  -H "X-Stonehenge-Control-Token: $STONEHENGE_CONTROL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resume_auto":true}'
```

The reset command uses source `operator_fault_reset`, `fire_request=false`,
both safety permissives false, and current PLC position as the safe target
angle. It still uses normal sequence allocation, commit-last register ordering,
exact acknowledgment, command audit, and lifecycle tracking.

First-phase resettable fault policy:

- resettable: fault codes `1` target range, `2` safety blocked, `3` invalid mode, `4` sequence error.
- not resettable: fault code `5` internal and unknown fault codes.

Reset success requires acknowledgment, fault cleared, and Ready/no fault at
lifecycle completion. If the prior mode was `auto_suspended` and resume is
requested, middleware returns to `auto` and the AUTO controller resumes idle.
The failed target is not retried automatically. Failed resets leave AUTO
suspended and require operator investigation.

## Gun 1 AUTO Controller

The current AUTO path is middleware-owned and dispatcher-fed:

```text
Target Feed
  -> Middleware Target Ingestion
  -> Fleet Dispatcher
  -> Target Ownership
  -> Gun 1 AUTO Executor
  -> Shared Command Service
  -> PLC
  -> Lifecycle Tracker
  -> Engagement History

FUXA
  -> observes PLC and middleware AUTO status
```

FUXA is an observer/supervisor in this phase. It is not the target-command
transport and should not generate AUTO commands.

Routes:

```bash
curl -X PUT http://localhost:8080/api/v1/guns/1/auto \
  -H "X-Stonehenge-Control-Token: $STONEHENGE_CONTROL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true}'

curl http://localhost:8080/api/v1/guns/1/auto

curl -X POST http://localhost:8080/api/v1/targets \
  -H "X-Stonehenge-Control-Token: $STONEHENGE_CONTROL_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "target_id": 9001,
    "azimuth": 220.5,
    "elevation": 40.0,
    "priority": 50,
    "target_class": "test_fragment",
    "detected_at": "2026-06-28T17:30:00Z"
  }'
```

Operational states:

- `disabled`: AUTO will not start new engagements. Queued targets are retained.
- `idle`: AUTO is enabled and has no queued or active target.
- `target_queued`: one target is waiting for Gun 1 to become ready.
- `engaging`: the controller is submitting the command through the shared command service.
- `waiting_for_completion`: the command was acknowledged and lifecycle tracking is pending.
- `suspended_fault`: PLC fault was observed; no automatic reset is attempted.
- `waiting_for_safety`: Gun 1 owns an assigned target but no sequence,
  arbitration, or PLC command has been allocated because Safety is not
  permissive.
- `suspended_unavailable`: safety trust, transport, timeout, or protocol state prevents more AUTO work.

Each executor accepts only one dispatcher assignment for its configured gun and
does not inspect the global queue. It rejects assignments for another gun,
duplicate active assignments, disabled executor state, stale generations, and
terminal assignments. It submits Mode Auto, normal elevation only,
`fire_request=true`, `admin_override=false`, `maintenance=false`, and no
extended permissive.
Safety denial leaves ownership on the same target in `waiting_for_safety`;
sequence allocation and arbitration happen only after Safety is trusted and
permissive.

Lifecycle handling:

- `completed_fired`: clears the active target, records success, and returns to `idle` if AUTO remains enabled.
- `completed_no_fire`: abnormal for an AUTO fire-requested command; clears the active target but records a failure result.
- `faulted`: transitions to `suspended_fault` and preserves active target context.
- `completion_timeout` or `protocol_state`: transitions to `suspended_unavailable` and preserves diagnostics.

Manual and AUTO commands share the same submission lock in `GunCommandService`.
If a manual command changes the accepted sequence while AUTO is waiting for a
lifecycle result, the existing lifecycle tracker reports `protocol_state` and
AUTO suspends.

## Read-Only AUTO Telemetry Modbus Server

FUXA 1.3.3 did not reliably update internal runtime tags from server-side
scripts/API responses in this deployment. The middleware therefore exposes Gun
1 AUTO-controller status as a separate read-only Modbus TCP device. Operator
actions still use middleware REST APIs; the telemetry device exposes no writable
command registers.

```text
Gun 1 AUTO controller
  -> immutable telemetry snapshot
  -> middleware Modbus TCP telemetry server
  -> FUXA read-only Modbus device

FUXA button
  -> middleware REST API
```

FUXA telemetry device settings:

```text
Host: middleware
Port: 1502
Modbus unit ID: 1
Register type: holding registers
```

The Compose file exposes port `1502` only to the internal Docker network by
default. It is not published to the Linux host.

The server supports holding-register reads with function `0x03` and input
register reads with function `0x04`. All write functions are rejected with
Modbus exception `0x01 Illegal Function`.

The server uses zero-based Modbus protocol offsets. FUXA normally displays
one-based addresses:

| Offset | FUXA addr | Name |
| --- | --- | --- |
| 0 | 1 | schema_version |
| 1 | 2 | controller_healthy |
| 2 | 3 | auto_enabled |
| 3 | 4 | auto_state_code |
| 4 | 5 | queued_target_id_low |
| 5 | 6 | queued_target_id_high |
| 6 | 7 | active_target_id_low |
| 7 | 8 | active_target_id_high |
| 8 | 9 | active_sequence_low |
| 9 | 10 | active_sequence_high |
| 10 | 11 | last_result_code |
| 11 | 12 | last_error_code |
| 12 | 13 | last_state_change_age_seconds_low |
| 13 | 14 | last_state_change_age_seconds_high |
| 14 | 15 | target_azimuth_tenths |
| 15 | 16 | target_elevation_tenths |
| 16 | 17 | target_priority |
| 17 | 18 | plc_state_code |
| 18 | 19 | plc_ready |
| 19 | 20 | plc_fault_active |
| 20 | 21 | plc_fault_code |
| 21 | 22 | plc_shot_sequence_low |
| 22 | 23 | plc_shot_sequence_high |
| 23 | 24 | plc_accepted_sequence_low |
| 24 | 25 | plc_accepted_sequence_high |
| 25 | 26 | plc_current_azimuth_tenths |
| 26 | 27 | plc_current_elevation_tenths |
| 27 | 28 | plc_settling |
| 28 | 29 | plc_firing |
| 29 | 30 | plc_lockout |
| 30 | 31 | plc_on_target |
| 31 | 32 | reserved |
| 32 | 33 | operational_mode_code |
| 33 | 34 | active_operation_code |
| 34 | 35 | fault_reset_active |
| 35 | 36 | auto_suspended |
| 36 | 37 | last_arbitration_error_code |
| 37 | 38 | last_fault_reset_result_code |
| 38 | 39 | last_fault_reset_sequence_low |
| 39 | 40 | last_fault_reset_sequence_high |
| 40 | 41 | last_fault_code_before |
| 41 | 42 | last_fault_code_after |
| 42 | 43 | mode_state_age_seconds_low |
| 43 | 44 | mode_state_age_seconds_high |
| 44 | 45 | reset_state_age_seconds_low |
| 45 | 46 | reset_state_age_seconds_high |
| 46 | 47 | reserved |
| 47 | 48 | reserved |
| 48 | 49 | external_feed_configured |
| 49 | 50 | external_feed_healthy |
| 50 | 51 | external_feed_receiving |
| 51 | 52 | pending_target_count |
| 52 | 53 | active_feed_target_low |
| 53 | 54 | active_feed_target_high |
| 54 | 55 | received_count_low |
| 55 | 56 | received_count_high |
| 56 | 57 | accepted_count_low |
| 57 | 58 | accepted_count_high |
| 58 | 59 | duplicate_count_low |
| 59 | 60 | duplicate_count_high |
| 60 | 61 | rejected_count_low |
| 61 | 62 | rejected_count_high |
| 62 | 63 | last_feed_error_code |
| 63 | 64 | reserved |

State codes: `0 disabled`, `1 idle`, `2 target_queued`, `3 engaging`,
`4 waiting_for_completion`, `5 suspended_fault`, `6 suspended_unavailable`,
`255 unknown`.

Lifecycle result codes: `0 none`, `1 accepted`, `2 completed_fired`,
`3 completed_no_fire`, `4 faulted`, `5 completion_timeout`,
`6 protocol_state`, `255 unknown`.

Last error codes: `0 none`, `1 plc_unreachable`, `2 gun_not_ready`,
`3 gun_faulted`, `4 safety_not_permissive`, `5 command_rejected`,
`6 lifecycle_timeout`, `7 protocol_state`, `8 persistence_error`,
`9 controller_unavailable`, `255 other`.

Operational mode codes: `0 manual`, `1 auto`, `2 maintenance`,
`3 auto_suspended`, `255 unknown`.

Active operation codes: `0 none`, `1 manual_command`, `2 auto_engagement`,
`3 fault_reset`, `255 unknown`.

Fault reset result codes: `0 none`, `1 succeeded`, `2 rejected`,
`3 acknowledgment_failed`, `4 fault_persisted`, `5 lifecycle_timeout`,
`6 protocol_state`, `7 internal_error`, `255 unknown`.

Arbitration error codes: `0 none`, `1 wrong_mode`, `2 manual_active`,
`3 auto_active`, `4 fault_reset_active`, `5 maintenance_active`,
`6 plc_not_ready`, `7 plc_faulted`, `8 auto_suspended`,
`9 transition_conflict`, `10 reset_not_allowed`, `255 other`.

External feed error codes: `0 none`, `1 validation`, `2 duplicate`,
`3 queue_full`, `4 stale`, `5 auto_unavailable`, `6 auto_suspended`,
`255 other`.

Booleans are `0=false` and `1=true`. Unsigned 32-bit values use low word first,
then high word. Signed values such as elevation tenths use signed 16-bit
two's-complement encoding inside the Modbus `uint16` register.

PLC status is refreshed by a middleware background poller, defaulting to every
`500ms`. Modbus reads use the latest in-memory snapshot and do not query
PostgreSQL or open a new PLC read per register request. `controller_healthy` is
`1` only when the AUTO controller is running and PLC status is fresher than
`PLC_AUTO_TELEMETRY_STALE_AFTER`, default `3s`. AUTO disabled is still healthy
when the controller and PLC telemetry are fresh.

Future range plan:

```text
0-63      Gun 1 AUTO, arbitration, fault reset, and external feed
64-127    Gun 2 future range
128-191   Gun 3 future range
...
```

Example read-only test after deployment from inside the Compose network:

```bash
docker run --rm --network stonehenge-middleware_stonehenge-control python:3-alpine python3 - <<'PY'
import socket, struct
req = struct.pack(">HHHBBHH", 1, 0, 6, 1, 3, 0, 64)
with socket.create_connection(("middleware", 1502), timeout=2) as s:
    s.sendall(req)
    print(s.recv(128).hex())
PY
```

## Audit

When PostgreSQL history is enabled, manual and AUTO API command attempts are
recorded in `gun_command_audit`. Audit write failures are logged as historian
failures and do not change a PLC acknowledgment result.

The HTTP response is acknowledgment-based: it returns after
`AcceptedSequenceHigh/Low` exactly matches the submitted 32-bit command
sequence. That proves PLC acceptance only. Accepted commands are then tracked
asynchronously and the same audit row is updated by `id` when a terminal
lifecycle result is detected.

Results:

- `accepted`: initial acknowledgment row; lifecycle completion is pending.
- `completed_fired`: `fire_request=true`, `ShotSequence` incremented relative to the baseline captured before submission, and the PLC returned to Ready.
- `completed_no_fire`: no shot occurred and the PLC returned to Ready with no active fault after meaningful movement/reset progress.
- `faulted`: PLC entered Fault or reported `fault_active=true`.
- `completion_timeout`: acknowledgment succeeded but no terminal result arrived before `PLC_LIFECYCLE_COMPLETION_TIMEOUT_SECONDS`.
- `protocol_state`: accepted sequence regressed or changed away from the tracked command.

Lifecycle tracking reads status only. It does not write command registers. It
uses these defaults:

```bash
PLC_LIFECYCLE_POLL_INTERVAL_MS=200
PLC_LIFECYCLE_COMPLETION_TIMEOUT_SECONDS=30
PLC_LIFECYCLE_MAX_CONSECUTIVE_READ_ERRORS=5
```

Example initial row:

```text
id=42 result=accepted resulting_state=2 shot_sequence=NULL completion_time=NULL
```

Example completed row:

```text
id=42 result=completed_fired resulting_state=1 fault_code=0 shot_sequence=1 completion_time=...
```

Inspect recent command lifecycle records:

```bash
docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT id, command_time, completion_time, submitted_sequence, result, resulting_state, fault_code, shot_sequence, error FROM gun_command_audit ORDER BY id DESC LIMIT 20;"
```

Rows created before lifecycle tracking was added remain acknowledgment-only.

AUTO engagement records are stored in `gun_engagements` and link the target to
the command audit row after submission. Inspect recent records:

```bash
docker compose exec postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT id, target_id, command_audit_id, submitted_sequence, state, result, final_shot_sequence, fault_code, error FROM gun_engagements ORDER BY id DESC LIMIT 20;"
```

## Verification

Safe checks:

```bash
docker compose config
docker compose up -d openplc-control-gun1 fuxa postgres middleware
docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' stonehenge-middleware-openplc-control-gun1-1
curl http://localhost:8080/api/v1/guns/1/plc-status
```

Do not send a real firing command during deployment verification. Use
`"fire_request": false` unless you are intentionally testing the PLC program in
the lab.

After deployment, a controlled AUTO test requires an explicit safety decision:

1. Verify read-only status: `curl http://localhost:8080/api/v1/guns/1/plc-status`.
2. Enable the temporary lab permissive only in a controlled test environment by
   setting `PLC_AUTO_SAFETY_FIRE_PERMISSIVE=true` and recreating middleware.
3. Enable AUTO with `PUT /api/v1/guns/1/auto`.
4. Submit one mock target with `POST /api/v1/targets`.
5. Watch `GET /api/v1/guns/1/auto` and inspect `gun_engagements` plus
   `gun_command_audit`.
