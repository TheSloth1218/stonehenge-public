FUNCTION_BLOCK FB_GlobalSafety
  VAR_INPUT
    global_enable : BOOL;
    emergency_stop : BOOL;
    global_maintenance_mode : BOOL;
    safety_reset_request : BOOL;
    test_fault_request : BOOL;
  END_VAR
  VAR_OUTPUT
    schema_version : INT;
    heartbeat_counter : INT;
    safety_state : INT;
    safety_ready : BOOL;
    global_fire_permissive : BOOL;
    global_inhibit_active : BOOL;
    global_inhibit_reason : INT;
    safety_fault_active : BOOL;
    safety_fault_code : INT;
    reset_required : BOOL;
    emergency_stop_active : BOOL;
    maintenance_mode_active : BOOL;
  END_VAR
  VAR
    SCHEMA_VERSION_VALUE : INT := 1;
    STATE_INITIALIZING : INT := 1;
    STATE_READY : INT := 2;
    STATE_INHIBITED : INT := 3;
    STATE_EMERGENCY_STOP : INT := 4;
    STATE_MAINTENANCE : INT := 5;
    STATE_FAULT : INT := 6;
    INHIBIT_NONE : INT := 0;
    INHIBIT_GLOBAL_DISABLED : INT := 1;
    INHIBIT_EMERGENCY_STOP_ACTIVE : INT := 2;
    INHIBIT_GLOBAL_MAINTENANCE_MODE : INT := 3;
    INHIBIT_SAFETY_FAULT_ACTIVE : INT := 4;
    INHIBIT_INITIALIZING : INT := 5;
    INHIBIT_RESET_REQUIRED : INT := 6;
    FAULT_NONE : INT := 0;
    FAULT_INVALID_CONFIGURATION : INT := 1;
    FAULT_RESET_REQUIRED : INT := 2;
    FAULT_CRITICAL_INTERNAL : INT := 5;
    INIT_SCAN_LIMIT : INT := 50;
    HEARTBEAT_SCAN_DIVIDER : INT := 50;
    initialized : BOOL := FALSE;
    init_scan_count : INT := 0;
    heartbeat_scan_count : INT := 0;
    heartbeat_word : DINT := 0;
    reset_request_previous : BOOL := FALSE;
    reset_edge : BOOL := FALSE;
    configuration_valid : BOOL := TRUE;
  END_VAR

schema_version := SCHEMA_VERSION_VALUE;
emergency_stop_active := emergency_stop;
maintenance_mode_active := global_maintenance_mode;

reset_edge := safety_reset_request AND NOT reset_request_previous;
reset_request_previous := safety_reset_request;

IF NOT initialized THEN
    initialized := TRUE;
    init_scan_count := 0;
    safety_state := STATE_INITIALIZING;
    safety_ready := FALSE;
    global_fire_permissive := FALSE;
    global_inhibit_active := TRUE;
    global_inhibit_reason := INHIBIT_INITIALIZING;
    safety_fault_active := FALSE;
    safety_fault_code := FAULT_NONE;
    reset_required := FALSE;
    heartbeat_scan_count := 0;
    heartbeat_word := 0;
END_IF;

heartbeat_scan_count := heartbeat_scan_count + 1;
IF heartbeat_scan_count >= HEARTBEAT_SCAN_DIVIDER THEN
    heartbeat_scan_count := 0;
    IF heartbeat_word >= 65535 THEN
        heartbeat_word := 0;
    ELSE
        heartbeat_word := heartbeat_word + 1;
    END_IF;
END_IF;

IF heartbeat_word > 32767 THEN
    heartbeat_counter := DINT_TO_INT(heartbeat_word - 65536);
ELSE
    heartbeat_counter := DINT_TO_INT(heartbeat_word);
END_IF;

IF init_scan_count < INIT_SCAN_LIMIT THEN
    init_scan_count := init_scan_count + 1;
END_IF;

IF emergency_stop THEN
    reset_required := TRUE;
END_IF;

IF test_fault_request THEN
    safety_fault_active := TRUE;
    safety_fault_code := FAULT_CRITICAL_INTERNAL;
    reset_required := TRUE;
END_IF;

IF reset_edge THEN
    IF (NOT emergency_stop) AND (NOT global_maintenance_mode) AND (NOT test_fault_request) AND configuration_valid THEN
        reset_required := FALSE;
        IF safety_fault_code = FAULT_CRITICAL_INTERNAL THEN
            safety_fault_active := FALSE;
            safety_fault_code := FAULT_NONE;
        END_IF;
    END_IF;
END_IF;

IF NOT configuration_valid THEN
    safety_fault_active := TRUE;
    safety_fault_code := FAULT_INVALID_CONFIGURATION;
END_IF;

safety_ready := FALSE;
global_fire_permissive := FALSE;
global_inhibit_active := TRUE;
global_inhibit_reason := INHIBIT_NONE;

IF safety_fault_active THEN
    safety_state := STATE_FAULT;
    global_inhibit_reason := INHIBIT_SAFETY_FAULT_ACTIVE;
ELSIF emergency_stop THEN
    safety_state := STATE_EMERGENCY_STOP;
    global_inhibit_reason := INHIBIT_EMERGENCY_STOP_ACTIVE;
ELSIF reset_required THEN
    safety_state := STATE_INHIBITED;
    global_inhibit_reason := INHIBIT_RESET_REQUIRED;
ELSIF global_maintenance_mode THEN
    safety_state := STATE_MAINTENANCE;
    global_inhibit_reason := INHIBIT_GLOBAL_MAINTENANCE_MODE;
ELSIF init_scan_count < INIT_SCAN_LIMIT THEN
    safety_state := STATE_INITIALIZING;
    global_inhibit_reason := INHIBIT_INITIALIZING;
ELSIF NOT global_enable THEN
    safety_state := STATE_INHIBITED;
    global_inhibit_reason := INHIBIT_GLOBAL_DISABLED;
ELSE
    safety_state := STATE_READY;
    global_inhibit_reason := INHIBIT_NONE;
    global_inhibit_active := FALSE;
END_IF;

safety_ready := (init_scan_count >= INIT_SCAN_LIMIT) AND NOT safety_fault_active;
global_fire_permissive := safety_ready AND global_enable AND NOT emergency_stop AND NOT global_maintenance_mode AND NOT safety_fault_active AND NOT reset_required;

IF global_fire_permissive THEN
    global_inhibit_active := FALSE;
    global_inhibit_reason := INHIBIT_NONE;
ELSE
    global_inhibit_active := TRUE;
END_IF;

IF reset_required AND (NOT safety_fault_active) AND (safety_fault_code = FAULT_NONE) THEN
    safety_fault_code := FAULT_RESET_REQUIRED;
END_IF;
IF NOT reset_required AND (safety_fault_code = FAULT_RESET_REQUIRED) THEN
    safety_fault_code := FAULT_NONE;
END_IF;

END_FUNCTION_BLOCK