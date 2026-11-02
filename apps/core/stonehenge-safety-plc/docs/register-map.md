# Stonehenge Safety PLC Register Map

All addresses are zero-based Modbus offsets. One-based addresses are shown for clients that display register 1 as the first register.

Status values use holding registers backed by `%QW0..%QW79` and are read with Modbus function code `0x03` at holding-register offsets `0..79`.

Engineering inputs use holding registers backed by `%MW0..%MW47` and are read or written with Modbus function codes `0x03`, `0x06`, or `0x10` at holding-register offsets `1024..1071`.

## Holding Registers: Engineering Inputs

| Offset | One-based | IEC | Name | Notes |
|---:|---:|---|---|---|
| 1024 | 41025 | `%MW0` | control_version_request | Reserved; write 0. |
| 1025 | 41026 | `%MW1` | global_enable | 0=false, nonzero=true. |
| 1026 | 41027 | `%MW2` | emergency_stop | 0=false, nonzero=true; latches reset_required. |
| 1027 | 41028 | `%MW3` | global_maintenance_mode | 0=false, nonzero=true. |
| 1028 | 41029 | `%MW4` | safety_reset_request | Rising edge requests reset. |
| 1029 | 41030 | `%MW5` | test_fault_request | Nonzero latches test safety fault. |
| 1030 | 41031 | `%MW6` | reserved | Write 0. |
| 1031 | 41032 | `%MW7` | reserved | Write 0. |

Each gun uses a four-register holding block.

| Gun | Start offset | One-based start | IEC start |
|---:|---:|---:|---|
| 1 | 1040 | 41041 | `%MW16` |
| 2 | 1044 | 41045 | `%MW20` |
| 3 | 1048 | 41049 | `%MW24` |
| 4 | 1052 | 41053 | `%MW28` |
| 5 | 1056 | 41057 | `%MW32` |
| 6 | 1060 | 41061 | `%MW36` |
| 7 | 1064 | 41065 | `%MW40` |
| 8 | 1068 | 41069 | `%MW44` |

Within each gun input block: +0 `gun_enabled`, +1 `gun_maintenance_inhibit`, +2 `gun_external_inhibit`, +3 reserved.

## Holding Registers: Status

| Offset | One-based | IEC | Name | Notes |
|---:|---:|---|---|---|
| 0 | 40001 | `%QW0` | schema_version | Current value 1. |
| 1 | 40002 | `%QW1` | heartbeat_counter | Unsigned 16-bit word; increments every 50 scans. |
| 2 | 40003 | `%QW2` | safety_state | See state table. |
| 3 | 40004 | `%QW3` | safety_ready | 1 after initialization when no active safety fault is present; separate from fire permissive. |
| 4 | 40005 | `%QW4` | global_fire_permissive | 1 when global firing is allowed. |
| 5 | 40006 | `%QW5` | global_inhibit_active | 1 when global firing is inhibited. |
| 6 | 40007 | `%QW6` | global_inhibit_reason | See global inhibit table. |
| 7 | 40008 | `%QW7` | safety_fault_active | 1 when safety fault is active. |
| 8 | 40009 | `%QW8` | safety_fault_code | See fault table. |
| 9 | 40010 | `%QW9` | reset_required | 1 when explicit reset is required. |
| 10 | 40011 | `%QW10` | emergency_stop_active | Mirrors emergency stop input. |
| 11 | 40012 | `%QW11` | maintenance_mode_active | Mirrors global maintenance input. |
| 12 | 40013 | `%QW12` | reserved | 0. |
| 13 | 40014 | `%QW13` | reserved | 0. |
| 14 | 40015 | `%QW14` | reserved | 0. |
| 15 | 40016 | `%QW15` | reserved | 0. |

Each gun uses an eight-register status block.

| Gun | Start offset | One-based start | IEC start |
|---:|---:|---:|---|
| 1 | 16 | 40017 | `%QW16` |
| 2 | 24 | 40025 | `%QW24` |
| 3 | 32 | 40033 | `%QW32` |
| 4 | 40 | 40041 | `%QW40` |
| 5 | 48 | 40049 | `%QW48` |
| 6 | 56 | 40057 | `%QW56` |
| 7 | 64 | 40065 | `%QW64` |
| 8 | 72 | 40073 | `%QW72` |

Within each gun status block: +0 `gun_fire_permissive`, +1 `gun_inhibit_active`, +2 `gun_inhibit_reason`, +3 `gun_enabled_status`, +4 `gun_maintenance_inhibit_status`, +5 `gun_external_inhibit_status`, +6 reserved, +7 reserved.

## Safety States

| Value | State |
|---:|---|
| 0 | Offline |
| 1 | Initializing |
| 2 | Ready |
| 3 | Inhibited |
| 4 | EmergencyStop |
| 5 | Maintenance |
| 6 | Fault |

## Global Inhibit Reasons

Precedence: safety fault, emergency stop, reset required, maintenance, initializing, global disabled, none.

| Value | Reason |
|---:|---|
| 0 | None |
| 1 | Global disabled |
| 2 | Emergency stop active |
| 3 | Global maintenance mode |
| 4 | Safety fault active |
| 5 | Initializing |
| 6 | Reset required |
| 255 | Unknown |

## Per-Gun Inhibit Reasons

Precedence: safety fault, reset required, global inhibit, gun disabled, gun maintenance inhibit, gun external inhibit, none.

| Value | Reason |
|---:|---|
| 0 | None |
| 1 | Global inhibit |
| 2 | Gun disabled |
| 3 | Gun maintenance inhibit |
| 4 | Gun external inhibit |
| 5 | Safety fault |
| 6 | Reset required |
| 255 | Unknown |

## Fault Codes

| Value | Fault |
|---:|---|
| 0 | None |
| 1 | Invalid configuration |
| 2 | Reset required |
| 3 | Internal state error |
| 4 | Heartbeat/internal timing fault |
| 5 | Critical internal fault |
| 255 | Unknown |

## Notes

- `heartbeat_counter` is a 16-bit Modbus word. IEC `INT` debug views may display values above 32767 as negative.
- `test_fault_request` is for deterministic validation. It is not a middleware firing command.
- The Safety PLC never writes gun commands and never tracks gun command sequences.

