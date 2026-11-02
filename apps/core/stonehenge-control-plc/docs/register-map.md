# Stonehenge Control PLC Register Map

The local gun runtime uses Modbus holding registers mapped first to IEC `%QW` words, followed by `%MW` words. Command inputs are `%MW0..%MW12` and start after the configured `%QW` block at holding register 1024. Status outputs are `%QW20..%QW38`. Addresses are zero-based; conceptual 4xxxx addresses are included for HMI convention.

Low/high 32-bit order is low word first, then high word. Command and accepted sequences are unsigned 32-bit values. Sequence `0` is reserved for startup/no-command, so rollover advances from `16#FFFFFFFF` to `1`.

Sequence comparison uses a half-range rollover rule: a nonzero candidate sequence is newer when it is within the next `2^31 - 1` values after the retained accepted sequence, including wraparound. Equal sequence values are duplicates, and values outside that forward window are stale.

| Address | 4xxxx | Variable | Direction | Type | Notes |
|---:|---:|---|---|---|---|
| 1024 | 41025 | command_sequence_low | input | INT | `%MW0`, UDINT low word |
| 1025 | 41026 | command_sequence_high | input | INT | `%MW1`, UDINT high word |
| 1026 | 41027 | mode_command | input | INT | `%MW2`, 0=AUTO, 1=MANUAL, 2=MAINTENANCE |
| 1027 | 41028 | target_azimuth_x10 | input | INT | `%MW3`, 0..3599 tenths degree |
| 1028 | 41029 | target_elevation_x10 | input | INT | `%MW4`, normal 50..850, override -300..900 |
| 1029 | 41030 | assigned_fragment_id_low | input | INT | `%MW5`, DINT low word |
| 1030 | 41031 | assigned_fragment_id_high | input | INT | `%MW6`, DINT high word |
| 1031 | 41032 | fire_request | input | INT | `%MW7`, 0=false, nonzero=true |
| 1032 | 41033 | maintenance_enabled | input | INT | `%MW8`, 0=false, nonzero=true |
| 1033 | 41034 | admin_override_active | input | INT | `%MW9`, 0=false, nonzero=true |
| 1034 | 41035 | fault_reset | input | INT | `%MW10`, 0=false, nonzero=true |
| 1035 | 41036 | safety_fire_permissive_lease | input | INT | `%MW11`, 0=denied; changing nonzero lease=permitted |
| 1036 | 41037 | safety_extended_motion_permissive_lease | input | INT | `%MW12`, 0=denied; changing nonzero lease=permitted |
| 20 | 40021 | accepted_command_sequence_low | output | INT | UDINT low word |
| 21 | 40022 | accepted_command_sequence_high | output | INT | UDINT high word |
| 22 | 40023 | gun_state | output | INT | 0=OFFLINE, 1=READY, 2=SLEWING, 3=SETTLING, 4=FIRING, 5=LOCKOUT, 6=MAINTENANCE, 7=FAULT |
| 23 | 40024 | current_azimuth_x10 | output | INT | tenths degree |
| 24 | 40025 | current_elevation_x10 | output | INT | signed tenths degree |
| 25 | 40026 | active_fragment_id_low | output | INT | DINT low word |
| 26 | 40027 | active_fragment_id_high | output | INT | DINT high word |
| 27 | 40028 | target_accepted | output | INT | 0=false, 1=true pulse |
| 28 | 40029 | on_target | output | INT | 0=false, 1=true |
| 29 | 40030 | settling | output | INT | 0=false, 1=true |
| 30 | 40031 | firing | output | INT | 0=false, 1=true short pulse |
| 31 | 40032 | post_fire_lockout | output | INT | 0=false, 1=true |
| 32 | 40033 | ready | output | INT | 0=false, 1=true |
| 33 | 40034 | fault_active | output | INT | 0=false, 1=true |
| 34 | 40035 | fault_code | output | INT | 0=NONE, 1=TARGET_OUT_OF_RANGE, 2=SAFETY_BLOCKED, 3=INVALID_MODE, 4=COMMAND_SEQUENCE_ERROR, 5=INTERNAL_STATE_ERROR |
| 35 | 40036 | shot_sequence_low | output | INT | DINT low word |
| 36 | 40037 | shot_sequence_high | output | INT | DINT high word |
| 37 | 40038 | settling_seconds_remaining | output | INT | seconds |
| 38 | 40039 | lockout_seconds_remaining | output | INT | seconds |
| 39 | 40040 | safety_fire_permissive_echo | output | INT | mirrors the raw lease at address 11 |
| 40 | 40041 | safety_extended_motion_permissive_echo | output | INT | mirrors the raw lease at address 12 |

Hard safety limits are not writable registers. Extended elevation requires both `admin_override_active` and a live `safety_extended_motion_permissive_lease`.

## Runtime Verification

Use zero-based holding-register addresses.

1. Confirm the PLC is ready: read register 22 = 1, register 32 = 1, register 33 = 0.
2. Write register 1027 = 900 and read register 1027 back. Expected: 900 persists.
3. Start writing changing nonzero values to register 1035 at least once per second and read register 1035 back. Expected: the latest lease value persists.
4. Prepare a movement-only command before changing the sequence: write register 1026 = 0, register 1028 = 450, registers 1029 = 1 and 1030 = 0, register 1031 = 0.
5. Write register 1024 = 1 last. Expected: register 20 becomes 1.
6. Observe register 22 transition through 2, 3, and back to 1.
7. Confirm register 29 equals 1 only while register 22 equals 3, and register 32 equals 1 only while register 22 equals 1.
8. Confirm register 35 does not increment and register 36 remains unchanged.
9. Change staged target or fragment registers without changing register 1024. Expected: the active target/fragment behavior does not change until a new sequence is written.
10. Firing check: keep the register 1035 lease changing, stage a valid target, write register 1031 = 1, then write a new sequence to register 1024 last. Observe register 22 transition through 2, 3, 4, 5, and back to 1. Confirm register 35 increments by exactly one and register 36 remains unchanged unless the low word rolls over.
11. Revocation check: submit another firing command while the lease is changing, then write register 1035 = 0 during slew or settling. Confirm register 22 becomes 7, register 34 becomes 2, and register 35 does not increment.
12. Fault reset check: force a fault with register 1035 = 0, register 1031 = 1, and a new sequence in register 1024. Confirm register 33 = 1 and register 34 = 2. Then write register 1034 = 1, confirm register 33 = 0 and register 34 = 0, and write register 1034 = 0 again. The accepted sequence must not decrease.
13. Extended-motion check: write register 1033 = 1, keep register 1036 changing with nonzero values, and submit an elevation outside 50..850 but within -300..900. Stop refreshing register 1036 or write it to 0 during slew or settling. Confirm register 22 becomes 7 and register 34 becomes 2.
