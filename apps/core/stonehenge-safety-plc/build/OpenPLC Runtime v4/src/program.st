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

PROGRAM main
  VAR
    SafetyGlobal : FB_GlobalSafety;
    Gun1Safety : FB_GunSafety;
    Gun2Safety : FB_GunSafety;
    Gun3Safety : FB_GunSafety;
    Gun4Safety : FB_GunSafety;
    Gun5Safety : FB_GunSafety;
    Gun6Safety : FB_GunSafety;
    Gun7Safety : FB_GunSafety;
    Gun8Safety : FB_GunSafety;
  END_VAR
  VAR
    CONTROL_VERSION_REQUEST AT %MW0 : INT := 0;
    GLOBAL_ENABLE AT %MW1 : INT := 0;
    EMERGENCY_STOP AT %MW2 : INT := 0;
    GLOBAL_MAINTENANCE_MODE AT %MW3 : INT := 0;
    SAFETY_RESET_REQUEST AT %MW4 : INT := 0;
    TEST_FAULT_REQUEST AT %MW5 : INT := 0;
    INPUT_RESERVED_6 AT %MW6 : INT := 0;
    INPUT_RESERVED_7 AT %MW7 : INT := 0;
    GUN1_ENABLED AT %MW16 : INT := 0;
    GUN1_MAINTENANCE_INHIBIT AT %MW17 : INT := 0;
    GUN1_EXTERNAL_INHIBIT AT %MW18 : INT := 0;
    GUN1_INPUT_RESERVED AT %MW19 : INT := 0;
    GUN2_ENABLED AT %MW20 : INT := 0;
    GUN2_MAINTENANCE_INHIBIT AT %MW21 : INT := 0;
    GUN2_EXTERNAL_INHIBIT AT %MW22 : INT := 0;
    GUN2_INPUT_RESERVED AT %MW23 : INT := 0;
    GUN3_ENABLED AT %MW24 : INT := 0;
    GUN3_MAINTENANCE_INHIBIT AT %MW25 : INT := 0;
    GUN3_EXTERNAL_INHIBIT AT %MW26 : INT := 0;
    GUN3_INPUT_RESERVED AT %MW27 : INT := 0;
    GUN4_ENABLED AT %MW28 : INT := 0;
    GUN4_MAINTENANCE_INHIBIT AT %MW29 : INT := 0;
    GUN4_EXTERNAL_INHIBIT AT %MW30 : INT := 0;
    GUN4_INPUT_RESERVED AT %MW31 : INT := 0;
    GUN5_ENABLED AT %MW32 : INT := 0;
    GUN5_MAINTENANCE_INHIBIT AT %MW33 : INT := 0;
    GUN5_EXTERNAL_INHIBIT AT %MW34 : INT := 0;
    GUN5_INPUT_RESERVED AT %MW35 : INT := 0;
    GUN6_ENABLED AT %MW36 : INT := 0;
    GUN6_MAINTENANCE_INHIBIT AT %MW37 : INT := 0;
    GUN6_EXTERNAL_INHIBIT AT %MW38 : INT := 0;
    GUN6_INPUT_RESERVED AT %MW39 : INT := 0;
    GUN7_ENABLED AT %MW40 : INT := 0;
    GUN7_MAINTENANCE_INHIBIT AT %MW41 : INT := 0;
    GUN7_EXTERNAL_INHIBIT AT %MW42 : INT := 0;
    GUN7_INPUT_RESERVED AT %MW43 : INT := 0;
    GUN8_ENABLED AT %MW44 : INT := 0;
    GUN8_MAINTENANCE_INHIBIT AT %MW45 : INT := 0;
    GUN8_EXTERNAL_INHIBIT AT %MW46 : INT := 0;
    GUN8_INPUT_RESERVED AT %MW47 : INT := 0;
    SAFETY_SCHEMA_VERSION AT %QW0 : INT := 0;
    SAFETY_HEARTBEAT_COUNTER AT %QW1 : INT := 0;
    SAFETY_STATE AT %QW2 : INT := 0;
    SAFETY_READY AT %QW3 : INT := 0;
    GLOBAL_FIRE_PERMISSIVE AT %QW4 : INT := 0;
    GLOBAL_INHIBIT_ACTIVE AT %QW5 : INT := 0;
    GLOBAL_INHIBIT_REASON AT %QW6 : INT := 0;
    SAFETY_FAULT_ACTIVE AT %QW7 : INT := 0;
    SAFETY_FAULT_CODE AT %QW8 : INT := 0;
    RESET_REQUIRED AT %QW9 : INT := 0;
    EMERGENCY_STOP_ACTIVE AT %QW10 : INT := 0;
    MAINTENANCE_MODE_ACTIVE AT %QW11 : INT := 0;
    STATUS_RESERVED_12 AT %QW12 : INT := 0;
    STATUS_RESERVED_13 AT %QW13 : INT := 0;
    STATUS_RESERVED_14 AT %QW14 : INT := 0;
    STATUS_RESERVED_15 AT %QW15 : INT := 0;
    GUN1_FIRE_PERMISSIVE AT %QW16 : INT := 0;
    GUN1_INHIBIT_ACTIVE AT %QW17 : INT := 0;
    GUN1_INHIBIT_REASON AT %QW18 : INT := 0;
    GUN1_ENABLED_STATUS AT %QW19 : INT := 0;
    GUN1_MAINTENANCE_INHIBIT_STATUS AT %QW20 : INT := 0;
    GUN1_EXTERNAL_INHIBIT_STATUS AT %QW21 : INT := 0;
    GUN1_STATUS_RESERVED_A AT %QW22 : INT := 0;
    GUN1_STATUS_RESERVED_B AT %QW23 : INT := 0;
    GUN2_FIRE_PERMISSIVE AT %QW24 : INT := 0;
    GUN2_INHIBIT_ACTIVE AT %QW25 : INT := 0;
    GUN2_INHIBIT_REASON AT %QW26 : INT := 0;
    GUN2_ENABLED_STATUS AT %QW27 : INT := 0;
    GUN2_MAINTENANCE_INHIBIT_STATUS AT %QW28 : INT := 0;
    GUN2_EXTERNAL_INHIBIT_STATUS AT %QW29 : INT := 0;
    GUN2_STATUS_RESERVED_A AT %QW30 : INT := 0;
    GUN2_STATUS_RESERVED_B AT %QW31 : INT := 0;
    GUN3_FIRE_PERMISSIVE AT %QW32 : INT := 0;
    GUN3_INHIBIT_ACTIVE AT %QW33 : INT := 0;
    GUN3_INHIBIT_REASON AT %QW34 : INT := 0;
    GUN3_ENABLED_STATUS AT %QW35 : INT := 0;
    GUN3_MAINTENANCE_INHIBIT_STATUS AT %QW36 : INT := 0;
    GUN3_EXTERNAL_INHIBIT_STATUS AT %QW37 : INT := 0;
    GUN3_STATUS_RESERVED_A AT %QW38 : INT := 0;
    GUN3_STATUS_RESERVED_B AT %QW39 : INT := 0;
    GUN4_FIRE_PERMISSIVE AT %QW40 : INT := 0;
    GUN4_INHIBIT_ACTIVE AT %QW41 : INT := 0;
    GUN4_INHIBIT_REASON AT %QW42 : INT := 0;
    GUN4_ENABLED_STATUS AT %QW43 : INT := 0;
    GUN4_MAINTENANCE_INHIBIT_STATUS AT %QW44 : INT := 0;
    GUN4_EXTERNAL_INHIBIT_STATUS AT %QW45 : INT := 0;
    GUN4_STATUS_RESERVED_A AT %QW46 : INT := 0;
    GUN4_STATUS_RESERVED_B AT %QW47 : INT := 0;
    GUN5_FIRE_PERMISSIVE AT %QW48 : INT := 0;
    GUN5_INHIBIT_ACTIVE AT %QW49 : INT := 0;
    GUN5_INHIBIT_REASON AT %QW50 : INT := 0;
    GUN5_ENABLED_STATUS AT %QW51 : INT := 0;
    GUN5_MAINTENANCE_INHIBIT_STATUS AT %QW52 : INT := 0;
    GUN5_EXTERNAL_INHIBIT_STATUS AT %QW53 : INT := 0;
    GUN5_STATUS_RESERVED_A AT %QW54 : INT := 0;
    GUN5_STATUS_RESERVED_B AT %QW55 : INT := 0;
    GUN6_FIRE_PERMISSIVE AT %QW56 : INT := 0;
    GUN6_INHIBIT_ACTIVE AT %QW57 : INT := 0;
    GUN6_INHIBIT_REASON AT %QW58 : INT := 0;
    GUN6_ENABLED_STATUS AT %QW59 : INT := 0;
    GUN6_MAINTENANCE_INHIBIT_STATUS AT %QW60 : INT := 0;
    GUN6_EXTERNAL_INHIBIT_STATUS AT %QW61 : INT := 0;
    GUN6_STATUS_RESERVED_A AT %QW62 : INT := 0;
    GUN6_STATUS_RESERVED_B AT %QW63 : INT := 0;
    GUN7_FIRE_PERMISSIVE AT %QW64 : INT := 0;
    GUN7_INHIBIT_ACTIVE AT %QW65 : INT := 0;
    GUN7_INHIBIT_REASON AT %QW66 : INT := 0;
    GUN7_ENABLED_STATUS AT %QW67 : INT := 0;
    GUN7_MAINTENANCE_INHIBIT_STATUS AT %QW68 : INT := 0;
    GUN7_EXTERNAL_INHIBIT_STATUS AT %QW69 : INT := 0;
    GUN7_STATUS_RESERVED_A AT %QW70 : INT := 0;
    GUN7_STATUS_RESERVED_B AT %QW71 : INT := 0;
    GUN8_FIRE_PERMISSIVE AT %QW72 : INT := 0;
    GUN8_INHIBIT_ACTIVE AT %QW73 : INT := 0;
    GUN8_INHIBIT_REASON AT %QW74 : INT := 0;
    GUN8_ENABLED_STATUS AT %QW75 : INT := 0;
    GUN8_MAINTENANCE_INHIBIT_STATUS AT %QW76 : INT := 0;
    GUN8_EXTERNAL_INHIBIT_STATUS AT %QW77 : INT := 0;
    GUN8_STATUS_RESERVED_A AT %QW78 : INT := 0;
    GUN8_STATUS_RESERVED_B AT %QW79 : INT := 0;
  END_VAR

  SafetyGlobal(
      global_enable := GLOBAL_ENABLE <> 0,
      emergency_stop := EMERGENCY_STOP <> 0,
      global_maintenance_mode := GLOBAL_MAINTENANCE_MODE <> 0,
      safety_reset_request := SAFETY_RESET_REQUEST <> 0,
      test_fault_request := TEST_FAULT_REQUEST <> 0
  );

  Gun1Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN1_ENABLED <> 0, gun_maintenance_inhibit := GUN1_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN1_EXTERNAL_INHIBIT <> 0);
  Gun2Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN2_ENABLED <> 0, gun_maintenance_inhibit := GUN2_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN2_EXTERNAL_INHIBIT <> 0);
  Gun3Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN3_ENABLED <> 0, gun_maintenance_inhibit := GUN3_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN3_EXTERNAL_INHIBIT <> 0);
  Gun4Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN4_ENABLED <> 0, gun_maintenance_inhibit := GUN4_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN4_EXTERNAL_INHIBIT <> 0);
  Gun5Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN5_ENABLED <> 0, gun_maintenance_inhibit := GUN5_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN5_EXTERNAL_INHIBIT <> 0);
  Gun6Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN6_ENABLED <> 0, gun_maintenance_inhibit := GUN6_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN6_EXTERNAL_INHIBIT <> 0);
  Gun7Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN7_ENABLED <> 0, gun_maintenance_inhibit := GUN7_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN7_EXTERNAL_INHIBIT <> 0);
  Gun8Safety(global_fire_permissive := SafetyGlobal.global_fire_permissive, global_fault_active := SafetyGlobal.safety_fault_active, reset_required := SafetyGlobal.reset_required, gun_enabled := GUN8_ENABLED <> 0, gun_maintenance_inhibit := GUN8_MAINTENANCE_INHIBIT <> 0, gun_external_inhibit := GUN8_EXTERNAL_INHIBIT <> 0);

  SAFETY_SCHEMA_VERSION := SafetyGlobal.schema_version;
  SAFETY_HEARTBEAT_COUNTER := SafetyGlobal.heartbeat_counter;
  SAFETY_STATE := SafetyGlobal.safety_state;
  IF SafetyGlobal.safety_ready THEN SAFETY_READY := 1; ELSE SAFETY_READY := 0; END_IF;
  IF SafetyGlobal.global_fire_permissive THEN GLOBAL_FIRE_PERMISSIVE := 1; ELSE GLOBAL_FIRE_PERMISSIVE := 0; END_IF;
  IF SafetyGlobal.global_inhibit_active THEN GLOBAL_INHIBIT_ACTIVE := 1; ELSE GLOBAL_INHIBIT_ACTIVE := 0; END_IF;
  GLOBAL_INHIBIT_REASON := SafetyGlobal.global_inhibit_reason;
  IF SafetyGlobal.safety_fault_active THEN SAFETY_FAULT_ACTIVE := 1; ELSE SAFETY_FAULT_ACTIVE := 0; END_IF;
  SAFETY_FAULT_CODE := SafetyGlobal.safety_fault_code;
  IF SafetyGlobal.reset_required THEN RESET_REQUIRED := 1; ELSE RESET_REQUIRED := 0; END_IF;
  IF SafetyGlobal.emergency_stop_active THEN EMERGENCY_STOP_ACTIVE := 1; ELSE EMERGENCY_STOP_ACTIVE := 0; END_IF;
  IF SafetyGlobal.maintenance_mode_active THEN MAINTENANCE_MODE_ACTIVE := 1; ELSE MAINTENANCE_MODE_ACTIVE := 0; END_IF;
  STATUS_RESERVED_12 := 0;
  STATUS_RESERVED_13 := 0;
  STATUS_RESERVED_14 := 0;
  STATUS_RESERVED_15 := 0;

  IF Gun1Safety.fire_permissive THEN GUN1_FIRE_PERMISSIVE := 1; ELSE GUN1_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun1Safety.inhibit_active THEN GUN1_INHIBIT_ACTIVE := 1; ELSE GUN1_INHIBIT_ACTIVE := 0; END_IF;
  GUN1_INHIBIT_REASON := Gun1Safety.inhibit_reason;
  IF GUN1_ENABLED <> 0 THEN GUN1_ENABLED_STATUS := 1; ELSE GUN1_ENABLED_STATUS := 0; END_IF;
  IF GUN1_MAINTENANCE_INHIBIT <> 0 THEN GUN1_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN1_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN1_EXTERNAL_INHIBIT <> 0 THEN GUN1_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN1_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN1_STATUS_RESERVED_A := 0;
  GUN1_STATUS_RESERVED_B := 0;

  IF Gun2Safety.fire_permissive THEN GUN2_FIRE_PERMISSIVE := 1; ELSE GUN2_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun2Safety.inhibit_active THEN GUN2_INHIBIT_ACTIVE := 1; ELSE GUN2_INHIBIT_ACTIVE := 0; END_IF;
  GUN2_INHIBIT_REASON := Gun2Safety.inhibit_reason;
  IF GUN2_ENABLED <> 0 THEN GUN2_ENABLED_STATUS := 1; ELSE GUN2_ENABLED_STATUS := 0; END_IF;
  IF GUN2_MAINTENANCE_INHIBIT <> 0 THEN GUN2_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN2_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN2_EXTERNAL_INHIBIT <> 0 THEN GUN2_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN2_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN2_STATUS_RESERVED_A := 0;
  GUN2_STATUS_RESERVED_B := 0;

  IF Gun3Safety.fire_permissive THEN GUN3_FIRE_PERMISSIVE := 1; ELSE GUN3_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun3Safety.inhibit_active THEN GUN3_INHIBIT_ACTIVE := 1; ELSE GUN3_INHIBIT_ACTIVE := 0; END_IF;
  GUN3_INHIBIT_REASON := Gun3Safety.inhibit_reason;
  IF GUN3_ENABLED <> 0 THEN GUN3_ENABLED_STATUS := 1; ELSE GUN3_ENABLED_STATUS := 0; END_IF;
  IF GUN3_MAINTENANCE_INHIBIT <> 0 THEN GUN3_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN3_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN3_EXTERNAL_INHIBIT <> 0 THEN GUN3_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN3_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN3_STATUS_RESERVED_A := 0;
  GUN3_STATUS_RESERVED_B := 0;

  IF Gun4Safety.fire_permissive THEN GUN4_FIRE_PERMISSIVE := 1; ELSE GUN4_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun4Safety.inhibit_active THEN GUN4_INHIBIT_ACTIVE := 1; ELSE GUN4_INHIBIT_ACTIVE := 0; END_IF;
  GUN4_INHIBIT_REASON := Gun4Safety.inhibit_reason;
  IF GUN4_ENABLED <> 0 THEN GUN4_ENABLED_STATUS := 1; ELSE GUN4_ENABLED_STATUS := 0; END_IF;
  IF GUN4_MAINTENANCE_INHIBIT <> 0 THEN GUN4_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN4_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN4_EXTERNAL_INHIBIT <> 0 THEN GUN4_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN4_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN4_STATUS_RESERVED_A := 0;
  GUN4_STATUS_RESERVED_B := 0;

  IF Gun5Safety.fire_permissive THEN GUN5_FIRE_PERMISSIVE := 1; ELSE GUN5_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun5Safety.inhibit_active THEN GUN5_INHIBIT_ACTIVE := 1; ELSE GUN5_INHIBIT_ACTIVE := 0; END_IF;
  GUN5_INHIBIT_REASON := Gun5Safety.inhibit_reason;
  IF GUN5_ENABLED <> 0 THEN GUN5_ENABLED_STATUS := 1; ELSE GUN5_ENABLED_STATUS := 0; END_IF;
  IF GUN5_MAINTENANCE_INHIBIT <> 0 THEN GUN5_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN5_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN5_EXTERNAL_INHIBIT <> 0 THEN GUN5_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN5_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN5_STATUS_RESERVED_A := 0;
  GUN5_STATUS_RESERVED_B := 0;

  IF Gun6Safety.fire_permissive THEN GUN6_FIRE_PERMISSIVE := 1; ELSE GUN6_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun6Safety.inhibit_active THEN GUN6_INHIBIT_ACTIVE := 1; ELSE GUN6_INHIBIT_ACTIVE := 0; END_IF;
  GUN6_INHIBIT_REASON := Gun6Safety.inhibit_reason;
  IF GUN6_ENABLED <> 0 THEN GUN6_ENABLED_STATUS := 1; ELSE GUN6_ENABLED_STATUS := 0; END_IF;
  IF GUN6_MAINTENANCE_INHIBIT <> 0 THEN GUN6_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN6_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN6_EXTERNAL_INHIBIT <> 0 THEN GUN6_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN6_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN6_STATUS_RESERVED_A := 0;
  GUN6_STATUS_RESERVED_B := 0;

  IF Gun7Safety.fire_permissive THEN GUN7_FIRE_PERMISSIVE := 1; ELSE GUN7_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun7Safety.inhibit_active THEN GUN7_INHIBIT_ACTIVE := 1; ELSE GUN7_INHIBIT_ACTIVE := 0; END_IF;
  GUN7_INHIBIT_REASON := Gun7Safety.inhibit_reason;
  IF GUN7_ENABLED <> 0 THEN GUN7_ENABLED_STATUS := 1; ELSE GUN7_ENABLED_STATUS := 0; END_IF;
  IF GUN7_MAINTENANCE_INHIBIT <> 0 THEN GUN7_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN7_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN7_EXTERNAL_INHIBIT <> 0 THEN GUN7_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN7_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN7_STATUS_RESERVED_A := 0;
  GUN7_STATUS_RESERVED_B := 0;

  IF Gun8Safety.fire_permissive THEN GUN8_FIRE_PERMISSIVE := 1; ELSE GUN8_FIRE_PERMISSIVE := 0; END_IF;
  IF Gun8Safety.inhibit_active THEN GUN8_INHIBIT_ACTIVE := 1; ELSE GUN8_INHIBIT_ACTIVE := 0; END_IF;
  GUN8_INHIBIT_REASON := Gun8Safety.inhibit_reason;
  IF GUN8_ENABLED <> 0 THEN GUN8_ENABLED_STATUS := 1; ELSE GUN8_ENABLED_STATUS := 0; END_IF;
  IF GUN8_MAINTENANCE_INHIBIT <> 0 THEN GUN8_MAINTENANCE_INHIBIT_STATUS := 1; ELSE GUN8_MAINTENANCE_INHIBIT_STATUS := 0; END_IF;
  IF GUN8_EXTERNAL_INHIBIT <> 0 THEN GUN8_EXTERNAL_INHIBIT_STATUS := 1; ELSE GUN8_EXTERNAL_INHIBIT_STATUS := 0; END_IF;
  GUN8_STATUS_RESERVED_A := 0;
  GUN8_STATUS_RESERVED_B := 0;
END_PROGRAM


CONFIGURATION Config0

  RESOURCE Res0 ON PLC
    TASK task0(INTERVAL := T#20ms,PRIORITY := 1);
    PROGRAM instance0 WITH task0 : main;
  END_RESOURCE
END_CONFIGURATION
