# Stonehenge Safety PLC

OpenPLC Editor v4 project for the independent Stonehenge Safety PLC. This runtime is separate from the gun control PLC runtime and only publishes global and per-gun fire permissives for eight guns.

The Safety PLC does not choose targets, dispatch targets, aim guns, send fire commands, track gun command sequences, replace middleware arbitration, or directly control gun movement.

## Layout

This project follows the simple Stonehenge gun PLC source layout: `project.json`, `devices`, `pous/programs`, `pous/function-blocks`, `docs`, and `tests`.

Core source files:

- `pous/programs/main.st`
- `pous/function-blocks/FB_GlobalSafety.st`
- `pous/function-blocks/FB_GunSafety.st`
- `docs/register-map.md`
- `docs/middleware-integration.md`
- `tests/manual-test-plan.md`

A previous safety skeleton, if present, was copied to the sibling backup directory `C:\dev\stonehenge-safety-plc-legacy-backup-20260629-090721` before this version was written.

## OpenPLC Task

- Task: `task0`
- Trigger: cyclic
- Interval: `T#20ms`
- Program instance: `instance0`
- Program: `main`

## Register Model

Status values are Modbus holding registers backed by `%QW0..%QW79`, exposed at holding-register offsets `0..79`, and read with function code `0x03`.

Engineering inputs are writable Modbus holding registers backed by `%MW0..%MW47`, exposed at holding-register offsets `1024..1071`, and read or written with function codes `0x03`, `0x06`, or `0x10`.

See `docs/register-map.md` for zero-based offsets and one-based client addresses.

## States

| Value | State |
|---:|---|
| 0 | Offline |
| 1 | Initializing |
| 2 | Ready |
| 3 | Inhibited |
| 4 | EmergencyStop |
| 5 | Maintenance |
| 6 | Fault |

Startup remains in `Initializing` for 50 scans. At the configured 20 ms scan, this is approximately one second. Fire permissive is false during initialization.

## Logic

Safety ready is equivalent to:

```text
initialization_complete AND NOT safety_fault_active
```

It indicates the Safety PLC is initialized and internally healthy. It does not mean firing is currently permitted.

Global fire permissive is equivalent to:

```text
safety_ready AND global_enable AND NOT emergency_stop AND NOT global_maintenance_mode AND NOT safety_fault_active AND NOT reset_required
```

Each gun fire permissive is equivalent to:

```text
global_fire_permissive AND gun_enabled AND NOT gun_maintenance_inhibit AND NOT gun_external_inhibit
```

A gun may be enabled and healthy while its fire permissive is false because a global condition or per-gun inhibit is active.

## Emergency Stop

Emergency stop immediately removes the global permissive and all per-gun permissives, sets state `EmergencyStop`, sets global inhibit reason `2`, and latches `reset_required`.

When emergency stop clears, permissive remains false until an explicit reset edge is received.

## Reset

`SAFETY_RESET_REQUEST` is edge-triggered. A reset edge clears `reset_required` only when emergency stop is inactive, global maintenance mode is inactive, no test fault request is active, and configuration is valid.

A constantly held reset bit does not repeatedly reset state. If reset is held while emergency stop or test fault is still active, the bit must be released and asserted again after the blocking condition clears.

## Heartbeat

The heartbeat is a 16-bit word incremented every 50 scans. At `T#20ms`, this is approximately once per second. It wraps from 65535 to 0. Because IEC `INT` is signed, OpenPLC debug views may show values above 32767 as negative; Modbus clients should treat the register as an unsigned 16-bit word.

## Open And Build

1. Open OpenPLC Editor v4.
2. Open/import `C:\dev\stonehenge-safety-plc`.
3. Confirm project name `Stonehenge Safety PLC`.
4. Confirm task `task0`, cyclic `T#20ms`, priority `1`.
5. Confirm program instance `instance0` runs `main`.
6. Select board `OpenPLC Runtime v4`.
7. Build/export manually in the Editor.

## Deploy To Separate Runtime

Do not deploy this to the existing gun PLC runtime.

1. Provision or identify the separate Safety PLC OpenPLC Runtime.
2. Use endpoint placeholders until deployment is fixed: `SAFETY_PLC_HOST` and `SAFETY_PLC_PORT=502`.
3. Build/export the Runtime v4 program from OpenPLC Editor.
4. In the Safety PLC Runtime web UI, stop the PLC.
5. Upload the newly exported Safety PLC program.
6. Compile it in the Runtime web UI.
7. Start the Safety PLC runtime.
8. Read holding-register offset 1, `heartbeat_counter`, and confirm it changes.
9. Write only the `%MW` engineering inputs at holding-register offsets `1024..1071` documented in `docs/register-map.md`.
10. Verify the gun control PLC runtime remains separate and unchanged.

## Validation

Use `tests/manual-test-plan.md`. Initial validation should use only non-firing checks: enable global safety, enable one gun, observe permissives, then exercise inhibits and reset behavior.
