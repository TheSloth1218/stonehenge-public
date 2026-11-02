# Stonehenge Control PLC

OpenPLC Editor v4 project for the Stonehenge gun-control PLC program. This is the shared source used to build the program deployed across the gun runtime fleet.

## Layout

This project intentionally uses the same simple file layout as the known-good gun-control reference project:

- `project.json`
- `devices/configuration.json`
- `devices/pin-mapping.json`
- `pous/programs/main.st`
- `pous/function-blocks/FB_GunControl.st`
- `docs/register-map.md`

There are no `.declaration.st` or `.implementation.st` files and no generated `build` directory checked into the project.

## Open In Editor

1. Open OpenPLC Editor v4.
2. Open/import `C:\dev\stonehenge-control-plc`.
3. Confirm project name `Stonehenge Control PLC`.
4. Confirm task `task0`, cyclic `T#20ms`, priority `1`.
5. Confirm program instance `instance0` runs `main`.
6. Select board `OpenPLC Simulator`.
7. Build manually in the Editor.

## Deploy To OpenPLC Runtime v4

Do not upload stale files from an existing `build` directory. Regenerate/export from this project source after edits.

1. Open/import `C:\dev\stonehenge-control-plc` in OpenPLC Editor v4.
2. Confirm task `task0` runs program instance `instance0` of `main`.
3. Select board `OpenPLC Runtime v4`.
4. Build/export the Runtime v4 program from the Editor.
5. In the OpenPLC Runtime web UI on the Linux VM, stop the PLC.
6. Upload the newly exported program.
7. Compile it in the Runtime web UI.
8. Start the PLC.

## Timing

The function block uses scan counters, not TON timers:

- 250 scans = 5 second settling delay.
- 500 scans = 10 second post-fire lockout.
- 2 scans = firing pulse.
- 100 unchanged scans = expired fire or extended-motion safety lease.

## Command Sequence

The command sequence is an unsigned 32-bit value split low word first across `%MW0/%MW1`. Sequence `0` is reserved for startup/no-command. Rollover advances from `16#FFFFFFFF` to `1`.

The PLC uses a half-range rollover comparison: equal is duplicate, a nonzero value within the next `2^31 - 1` sequence values is newer, and other values are stale.

## Modes

| Value | Mode |
|---:|---|
| 0 | AUTO |
| 1 | MANUAL |
| 2 | MAINTENANCE |

## States

| Value | State |
|---:|---|
| 0 | OFFLINE/uninitialized |
| 1 | READY |
| 2 | SLEWING |
| 3 | SETTLING |
| 4 | FIRING |
| 5 | LOCKOUT |
| 6 | MAINTENANCE |
| 7 | FAULT |

## Fault Codes

| Value | Fault |
|---:|---|
| 0 | NONE |
| 1 | TARGET_OUT_OF_RANGE |
| 2 | SAFETY_BLOCKED |
| 3 | INVALID_MODE |
| 4 | COMMAND_SEQUENCE_ERROR |
| 5 | INTERNAL_STATE_ERROR |

## Manual Tests

Safe movement-only validation:

1. Confirm initial status: state register 22 / `%QW22 = 1`, ready register 32 / `%QW32 = 1`, fault register 33 / `%QW33 = 0`.
2. Write mode register 1026 / `%MW2 = 0`, azimuth register 1027 / `%MW3`, elevation register 1028 / `%MW4`, fragment id registers 1029 and 1030 / `%MW5/%MW6`, and fire request register 1031 / `%MW7 = 0`.
3. Write command sequence register 1024 / `%MW0 = 1` last.
4. Observe register 20 / `%QW20` become `1`.
5. Observe register 22 / `%QW22` transition READY, SLEWING, SETTLING, READY.
6. Confirm ready register 32 / `%QW32` and settling register 29 / `%QW29` always match the numeric state.
7. Confirm shot sequence `%QW35/%QW36` does not increment.
8. Change staged target or fragment registers without changing `%MW0`; confirm the active target/fragment does not change until a new sequence is written.

Firing cycle:

1. Keep writing changing nonzero values to holding register 1035 / `%MW11` for the fire-permissive lease.
2. Stage a valid target and write fire request register 1031 / `%MW7 = 1`.
3. Write a new command sequence in register 1024 / `%MW0` last.
4. Observe register 22 / `%QW22` transition through SLEWING, SETTLING, FIRING, LOCKOUT, READY.
5. Confirm `shot_sequence` split across `%QW35/%QW36` increments exactly once.

Negative elevation rejection:

1. Write `%MW9 = 0` and `%MW12 = 0`.
2. Write `%MW4 = -10` with a new command sequence.
3. Expected: state FAULT and fault code TARGET_OUT_OF_RANGE.

Admin override extended motion:

1. Write `%MW9 = 1` and keep writing changing nonzero values to `%MW12` for the extended-motion lease.
2. Use an elevation from `-300` through `900` while continuing to refresh `%MW12`.
3. Firing still requires the changing nonzero `%MW11` fire-permissive lease.
4. Stop refreshing `%MW12` or write `%MW12 = 0` during extended slew or settling; expect state FAULT and fault code SAFETY_BLOCKED.

`shot_sequence` is split across `%QW35/%QW36` and increments once per firing cycle.
