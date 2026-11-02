# Middleware Integration Contract

Future middleware integration should treat the Safety PLC as an independent read-mostly authority for fire permissives.

Requirements:

- Read global status and all eight gun status blocks in one coherent poll where possible.
- Poll approximately every 250 to 500 ms.
- Treat the heartbeat as stale if unchanged beyond a configurable timeout.
- Treat Modbus read errors as safety unavailable.
- Treat safety unavailable as fire denied.
- Allow aiming and telemetry while fire is denied.
- Require both `global_fire_permissive` and the selected gun's `gun_fire_permissive` before issuing a firing command to the gun PLC.
- Expose inhibit reasons exactly as read from the Safety PLC; do not rewrite them in middleware.
- Read Safety PLC status from holding-register offsets `0..79` with function code `0x03`; these registers are backed by `%QW0..%QW79`.
- Write only engineering input holding registers at offsets `1024..1071`; these registers are backed by `%MW0..%MW47` and support function codes `0x03`, `0x06`, and `0x10`.
- Do not write Safety PLC status values.
- Do not use the Safety PLC to choose targets, dispatch targets, aim guns, or assign gun command sequences.

Endpoint placeholders:

```text
SAFETY_PLC_HOST=<deployment specific>
SAFETY_PLC_PORT=502
```

Safety unavailable means fire denied, not plant unavailable. Middleware may continue non-firing workflows such as target staging, aiming, status display, and telemetry collection.
