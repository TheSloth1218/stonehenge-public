# Stonehenge Safety PLC Manual Test Plan

Use zero-based Modbus holding-register offsets. Status outputs are `%QW0..%QW79` at offsets `0..79` and are read with function code `0x03`. Engineering inputs are `%MW0..%MW47` at offsets `1024..1071` and are read or written with function codes `0x03`, `0x06`, or `0x10`. Start with all engineering inputs at 0 unless a test states otherwise.

State values: 1 Initializing, 2 Ready, 3 Inhibited, 4 EmergencyStop, 5 Maintenance, 6 Fault.

Global inhibit reasons: 0 None, 1 Global disabled, 2 Emergency stop active, 3 Global maintenance mode, 4 Safety fault active, 5 Initializing, 6 Reset required.

Per-gun inhibit reasons: 0 None, 1 Global inhibit, 2 Gun disabled, 3 Gun maintenance inhibit, 4 Gun external inhibit, 5 Safety fault, 6 Reset required.

## 1. Startup Initialization

Inputs: all engineering input holding registers 0.

Expected during first 50 scans: status holding register 2 state = 1, register 3 safety_ready = 0, register 4 global_fire_permissive = 0, register 5 global_inhibit_active = 1, register 6 global_inhibit_reason = 5.

Expected after initialization: state = 3, safety_ready = 1, global_fire_permissive = 0, global_inhibit_reason = 1 because global_enable is false.

## 2. Global Disabled

Inputs: engineering input offset 1025 global_enable = 0; all gun enables 0.

Expected: state = 3, safety_ready = 1, global permissive = 0, global inhibit reason = 1, every gun permissive = 0, every gun inhibit reason = 1 because global inhibit precedes gun disabled.

## 3. Global Enable

Inputs: write engineering input offset 1025 global_enable = 1 after startup.

Expected: state = 2, safety_ready = 1, global_fire_permissive = 1, global_inhibit_active = 0, reason = 0. Guns remain inhibited until individually enabled.

## 4. Gun Disabled

Inputs: global_enable = 1; Gun 1 engineering input offset 1040 = 0.

Expected: Gun 1 fire permissive status register 16 = 0, inhibit active register 17 = 1, inhibit reason register 18 = 2.

## 5. Gun Enabled

Inputs: global_enable = 1; Gun 1 engineering input offset 1040 = 1; Gun 1 maintenance and external inhibits = 0.

Expected: Gun 1 fire permissive = 1, inhibit active = 0, inhibit reason = 0, enabled status = 1.

## 6. Per-Gun Maintenance Inhibit

Inputs: global_enable = 1; Gun 1 enabled = 1; Gun 1 maintenance inhibit engineering input offset 1041 = 1.

Expected: Gun 1 fire permissive = 0, inhibit active = 1, inhibit reason = 3. Gun 2 remains governed only by its own inputs.

## 7. Per-Gun External Inhibit

Inputs: global_enable = 1; Gun 1 enabled = 1; Gun 1 maintenance inhibit = 0; Gun 1 external inhibit engineering input offset 1042 = 1.

Expected: Gun 1 fire permissive = 0, inhibit active = 1, inhibit reason = 4.

## 8. Global Maintenance Mode

Inputs: global_enable = 1; Gun 1 enabled = 1; engineering input offset 1027 global_maintenance_mode = 1.

Expected: state = 5, safety_ready = 1, global permissive = 0, global inhibit reason = 3, maintenance_mode_active = 1, all gun permissives = 0, each gun inhibit reason = 1 unless safety fault or reset is active.

## 9. Emergency Stop Activation

Inputs: global_enable = 1; Gun 1 enabled = 1; engineering input offset 1026 emergency_stop = 1.

Expected immediately: state = 4, safety_ready = 1 if initialization is complete, global permissive = 0, global inhibit reason = 2, reset_required = 1, emergency_stop_active = 1, all gun permissives = 0, gun inhibit reason = 6 because reset_required precedes global inhibit in per-gun status.

## 10. Emergency Stop Clearance Without Reset

Inputs: after test 9, write emergency_stop = 0 and leave reset request 0.

Expected: state = 3, global permissive = 0, reset_required = 1, global inhibit reason = 6, all gun permissives = 0.

## 11. Valid Reset

Inputs: emergency_stop = 0, global_maintenance_mode = 0, test_fault_request = 0. Pulse safety_reset_request engineering input offset 1028 from 0 to 1, then back to 0.

Expected: reset_required = 0. If global_enable = 1, state = 2 and global permissive = 1 after the pulse.

## 12. Invalid Reset While Emergency Stop Remains Active

Inputs: emergency_stop = 1; pulse safety_reset_request 0 to 1 to 0.

Expected: reset_required remains 1, state remains 4, global inhibit reason remains 2.

## 13. Test Fault Activation

Inputs: global_enable = 1; engineering input offset 1029 test_fault_request = 1.

Expected: state = 6, safety_ready = 0, safety_fault_active = 1, safety_fault_code = 5, reset_required = 1, global permissive = 0, global inhibit reason = 4, all gun inhibit reasons = 5.

## 14. Fault Clearance And Reset

Inputs: after test 13, write test_fault_request = 0. Pulse safety_reset_request 0 to 1 to 0 while emergency_stop = 0 and global_maintenance_mode = 0.

Expected: safety_fault_active = 0, fault code returns to 0, reset_required = 0. If global_enable = 1, state = 2 and global permissive = 1.

## 15. Heartbeat Increment

Inputs: any safe state.

Expected: status holding register 1 changes approximately once per second at the configured 20 ms scan and does not depend on fire permissive.

## 16. Heartbeat Wrap Behavior

Practical local runtime validation may require leaving the PLC running until the 16-bit word wraps. Expected: heartbeat wraps from 65535 to 0. Debug views may show signed negative values above 32767; Modbus clients should treat the register as unsigned.

## 17. Only Intended Gun Is Inhibited

Inputs: global_enable = 1; enable Guns 1 and 2; set Gun 1 external inhibit = 1; leave Gun 2 external inhibit = 0.

Expected: Gun 1 permissive = 0 with reason 4. Gun 2 permissive = 1 with reason 0.

## 18. All Guns Inhibited During Global Conditions

Inputs: global_enable = 1; enable all guns; set global_maintenance_mode = 1 or emergency_stop = 1.

Expected: every gun fire permissive = 0. During maintenance, global reason = 3 and per-gun reason = 1. During emergency stop, global reason = 2 and reset_required = 1.

## 19. Inhibit-Reason Precedence

Inputs: global_enable = 0, Gun 1 enabled = 0, Gun 1 external inhibit = 1.

Expected: Gun 1 inhibit reason = 1 because global inhibit precedes gun disabled and external inhibit. Then set global_enable = 1; expected Gun 1 reason becomes 2. Then set Gun 1 enabled = 1; expected reason becomes 4.

## 20. Safe Startup After Runtime Restart

Inputs before restart may include global_enable = 1 and gun_enabled = 1. Restart the Safety PLC runtime.

Expected immediately after restart: no fire permissive during initialization. After initialization, permissive follows current holding-register inputs. If registers are cleared by runtime startup, global_enable and gun enables default false and permissives remain false.

