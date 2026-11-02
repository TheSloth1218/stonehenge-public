FUNCTION_BLOCK FB_GunSafety
  VAR_INPUT
    global_fire_permissive : BOOL;
    global_fault_active : BOOL;
    reset_required : BOOL;
    gun_enabled : BOOL;
    gun_maintenance_inhibit : BOOL;
    gun_external_inhibit : BOOL;
  END_VAR
  VAR_OUTPUT
    fire_permissive : BOOL;
    inhibit_active : BOOL;
    inhibit_reason : INT;
  END_VAR
  VAR
    INHIBIT_NONE : INT := 0;
    INHIBIT_GLOBAL_INHIBIT : INT := 1;
    INHIBIT_GUN_DISABLED : INT := 2;
    INHIBIT_GUN_MAINTENANCE : INT := 3;
    INHIBIT_GUN_EXTERNAL : INT := 4;
    INHIBIT_SAFETY_FAULT : INT := 5;
    INHIBIT_RESET_REQUIRED : INT := 6;
  END_VAR

fire_permissive := global_fire_permissive AND gun_enabled AND NOT gun_maintenance_inhibit AND NOT gun_external_inhibit;

IF global_fault_active THEN
    inhibit_active := TRUE;
    inhibit_reason := INHIBIT_SAFETY_FAULT;
ELSIF reset_required THEN
    inhibit_active := TRUE;
    inhibit_reason := INHIBIT_RESET_REQUIRED;
ELSIF NOT global_fire_permissive THEN
    inhibit_active := TRUE;
    inhibit_reason := INHIBIT_GLOBAL_INHIBIT;
ELSIF NOT gun_enabled THEN
    inhibit_active := TRUE;
    inhibit_reason := INHIBIT_GUN_DISABLED;
ELSIF gun_maintenance_inhibit THEN
    inhibit_active := TRUE;
    inhibit_reason := INHIBIT_GUN_MAINTENANCE;
ELSIF gun_external_inhibit THEN
    inhibit_active := TRUE;
    inhibit_reason := INHIBIT_GUN_EXTERNAL;
ELSE
    inhibit_active := FALSE;
    inhibit_reason := INHIBIT_NONE;
END_IF;

END_FUNCTION_BLOCK