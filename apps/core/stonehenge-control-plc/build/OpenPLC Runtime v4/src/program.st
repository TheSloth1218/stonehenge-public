FUNCTION_BLOCK FB_GunControl
  VAR_INPUT
    command_sequence : UDINT;
    mode_command : INT;
    target_azimuth_x10 : INT;
    target_elevation_x10 : INT;
    assigned_fragment_id : DINT;
    fire_request : BOOL;
    maintenance_enabled : BOOL;
    admin_override_active : BOOL;
    fault_reset : BOOL;
    safety_fire_permissive : BOOL;
    safety_extended_motion_permissive : BOOL;
  END_VAR
  VAR_OUTPUT
    accepted_command_sequence : UDINT;
    gun_state : INT;
    current_azimuth_x10 : INT;
    current_elevation_x10 : INT;
    active_fragment_id : DINT;
    target_accepted : BOOL;
    on_target : BOOL;
    settling : BOOL;
    firing : BOOL;
    post_fire_lockout : BOOL;
    ready : BOOL;
    fault_active : BOOL;
    fault_code : INT;
    shot_sequence : DINT;
    settling_seconds_remaining : INT;
    lockout_seconds_remaining : INT;
  END_VAR
  VAR
    initialized : BOOL := FALSE;
    command_pending : BOOL := FALSE;
    pending_azimuth_x10 : INT := 0;
    pending_elevation_x10 : INT := 50;
    pending_fragment_id : DINT := 0;
    accepted_mode : INT := 0;
    accepted_fire_request : BOOL := FALSE;
    accepted_maintenance_enabled : BOOL := FALSE;
    accepted_admin_override_active : BOOL := FALSE;
    accepted_safety_fire_permissive : BOOL := FALSE;
    accepted_safety_extended_motion_permissive : BOOL := FALSE;
    extended_allowed : BOOL := FALSE;
    command_valid : BOOL := FALSE;
    sequence_is_newer : BOOL := FALSE;
    sequence_is_stale : BOOL := FALSE;
    az_delta : INT := 0;
    az_abs : INT := 0;
    el_delta : INT := 0;
    el_abs : INT := 0;
    settle_count : INT := 0;
    lockout_count : INT := 0;
    fire_pulse_count : INT := 0;
    fire_counted : BOOL := FALSE;
  END_VAR

  IF NOT initialized THEN
      initialized := TRUE;
      gun_state := 1;
      current_azimuth_x10 := 0;
      current_elevation_x10 := 50;
      pending_azimuth_x10 := 0;
      pending_elevation_x10 := 50;
      pending_fragment_id := 0;
      active_fragment_id := 0;
      accepted_mode := 0;
      accepted_fire_request := FALSE;
      accepted_maintenance_enabled := FALSE;
      accepted_admin_override_active := FALSE;
      accepted_safety_fire_permissive := FALSE;
      accepted_safety_extended_motion_permissive := FALSE;
      accepted_command_sequence := 0;
      shot_sequence := 0;
      fault_active := FALSE;
      fault_code := 0;
      settle_count := 0;
      lockout_count := 0;
      fire_pulse_count := 0;
      fire_counted := FALSE;
      command_pending := FALSE;
  END_IF;

  target_accepted := FALSE;
  firing := FALSE;
  ready := FALSE;
  settling := FALSE;
  post_fire_lockout := FALSE;
  on_target := FALSE;
  settling_seconds_remaining := 0;
  lockout_seconds_remaining := 0;

  sequence_is_newer := FALSE;
  sequence_is_stale := FALSE;
  IF command_sequence <> accepted_command_sequence THEN
      IF command_sequence = 0 THEN
          sequence_is_stale := TRUE;
      ELSE
          IF accepted_command_sequence = 0 THEN
              sequence_is_newer := TRUE;
          ELSE
              IF command_sequence > accepted_command_sequence THEN
                  IF (command_sequence - accepted_command_sequence) <= UDINT#2147483647 THEN
                      sequence_is_newer := TRUE;
                  ELSE
                      sequence_is_stale := TRUE;
                  END_IF;
              ELSE
                  IF (accepted_command_sequence - command_sequence) > UDINT#2147483647 THEN
                      sequence_is_newer := TRUE;
                  ELSE
                      sequence_is_stale := TRUE;
                  END_IF;
              END_IF;
          END_IF;
      END_IF;
  END_IF;

  IF fault_reset THEN
      fault_active := FALSE;
      fault_code := 0;
      gun_state := 1; (* READY *)

      command_pending := FALSE;
      command_valid := FALSE;
      fire_counted := FALSE;

      settle_count := 0;
      lockout_count := 0;
      fire_pulse_count := 0;

      active_fragment_id := 0;
      target_accepted := FALSE;

      (* Consume the currently presented sequence so the same bad
         command cannot immediately trigger the fault again, without
         allowing an older reset write to regress the retained sequence. *)
      IF sequence_is_newer THEN
          accepted_command_sequence := command_sequence;
      END_IF;
  END_IF;

  extended_allowed := admin_override_active AND safety_extended_motion_permissive;

  IF (NOT fault_reset)
     AND ((gun_state = 2) OR (gun_state = 3))
     AND ((pending_elevation_x10 < 50) OR (pending_elevation_x10 > 850))
     AND (NOT extended_allowed) THEN
      command_pending := FALSE;
      settling := FALSE;
      settle_count := 0;
      fault_active := TRUE;
      fault_code := 2;
      gun_state := 7;
  END_IF;

  IF (NOT fault_reset)
     AND (command_sequence <> accepted_command_sequence)
     AND (NOT fault_active) THEN
      command_valid := sequence_is_newer;

      IF sequence_is_stale THEN
          command_valid := FALSE;
          fault_active := TRUE;
          fault_code := 4;
          gun_state := 7;
      END_IF;

      IF command_valid AND (mode_command <> 0) AND (mode_command <> 1) AND (mode_command <> 2) THEN
          command_valid := FALSE;
          accepted_command_sequence := command_sequence;
          fault_active := TRUE;
          fault_code := 3;
          gun_state := 7;
      END_IF;

      IF command_valid THEN
          IF (target_azimuth_x10 < 0) OR (target_azimuth_x10 > 3599) THEN
              command_valid := FALSE;
          END_IF;

          IF extended_allowed THEN
              IF (target_elevation_x10 < -300) OR (target_elevation_x10 > 900) THEN
                  command_valid := FALSE;
              END_IF;
          ELSE
              IF (target_elevation_x10 < 50) OR (target_elevation_x10 > 850) THEN
                  command_valid := FALSE;
              END_IF;
          END_IF;

          IF NOT command_valid THEN
              accepted_command_sequence := command_sequence;
              fault_active := TRUE;
              fault_code := 1;
              gun_state := 7;
          END_IF;
      END_IF;

      IF command_valid THEN
          IF ((mode_command = 2) OR maintenance_enabled) AND (assigned_fragment_id <> 0) THEN
              command_valid := FALSE;
              accepted_command_sequence := command_sequence;
              fault_active := TRUE;
              fault_code := 4;
              gun_state := 7;
          END_IF;
      END_IF;

      IF command_valid THEN
          accepted_mode := mode_command;
          pending_azimuth_x10 := target_azimuth_x10;
          pending_elevation_x10 := target_elevation_x10;
          pending_fragment_id := assigned_fragment_id;
          accepted_fire_request := fire_request;
          accepted_maintenance_enabled := maintenance_enabled;
          accepted_admin_override_active := admin_override_active;
          accepted_safety_fire_permissive := safety_fire_permissive;
          accepted_safety_extended_motion_permissive := safety_extended_motion_permissive;
          command_pending := TRUE;
          target_accepted := TRUE;
          accepted_command_sequence := command_sequence;

          IF (gun_state = 1) OR (gun_state = 6) THEN
              active_fragment_id := assigned_fragment_id;
              gun_state := 2;
          END_IF;
      END_IF;
  END_IF;

  IF gun_state = 0 THEN
      gun_state := 1;
  END_IF;

  CASE gun_state OF
      1:
          ready := TRUE;
          on_target := TRUE;
          IF accepted_maintenance_enabled OR (accepted_mode = 2) THEN
              gun_state := 6;
          ELSE
              IF command_pending THEN
                  active_fragment_id := pending_fragment_id;
                  gun_state := 2;
              END_IF;
          END_IF;

      2:
          az_delta := pending_azimuth_x10 - current_azimuth_x10;
          IF az_delta > 1800 THEN
              az_delta := az_delta - 3600;
          END_IF;
          IF az_delta < -1800 THEN
              az_delta := az_delta + 3600;
          END_IF;

          az_abs := az_delta;
          IF az_abs < 0 THEN
              az_abs := 0 - az_abs;
          END_IF;

          el_delta := pending_elevation_x10 - current_elevation_x10;
          el_abs := el_delta;
          IF el_abs < 0 THEN
              el_abs := 0 - el_abs;
          END_IF;

          IF az_abs <= 5 THEN
              current_azimuth_x10 := pending_azimuth_x10;
          ELSE
              IF az_delta > 0 THEN
                  current_azimuth_x10 := current_azimuth_x10 + 10;
              ELSE
                  current_azimuth_x10 := current_azimuth_x10 - 10;
              END_IF;
          END_IF;

          IF current_azimuth_x10 > 3599 THEN
              current_azimuth_x10 := current_azimuth_x10 - 3600;
          END_IF;
          IF current_azimuth_x10 < 0 THEN
              current_azimuth_x10 := current_azimuth_x10 + 3600;
          END_IF;

          IF el_abs <= 5 THEN
              current_elevation_x10 := pending_elevation_x10;
          ELSE
              IF el_delta > 0 THEN
                  current_elevation_x10 := current_elevation_x10 + 5;
              ELSE
                  current_elevation_x10 := current_elevation_x10 - 5;
              END_IF;
          END_IF;

          IF (current_azimuth_x10 = pending_azimuth_x10) AND (current_elevation_x10 = pending_elevation_x10) THEN
              on_target := TRUE;
              command_pending := FALSE;
              IF accepted_mode = 2 THEN
                  gun_state := 6;
              ELSE
                  settle_count := 250;
                  gun_state := 3;
              END_IF;
          END_IF;

      3:
          on_target := TRUE;
          IF settle_count > 0 THEN
              settling := TRUE;
              settling_seconds_remaining := (settle_count + 49) / 50;
              settle_count := settle_count - 1;
          ELSE
              settling := FALSE;
              settling_seconds_remaining := 0;
              ready := TRUE;
              IF accepted_fire_request THEN
                  IF safety_fire_permissive THEN
                      fire_pulse_count := 2;
                      fire_counted := FALSE;
                      gun_state := 4;
                  ELSE
                      fault_active := TRUE;
                      fault_code := 2;
                      gun_state := 7;
                  END_IF;
              ELSE
                  gun_state := 1;
              END_IF;
          END_IF;

      4:
          IF NOT safety_fire_permissive THEN
              firing := FALSE;
              fire_pulse_count := 0;
              fault_active := TRUE;
              fault_code := 2;
              gun_state := 7;
          ELSE
              firing := TRUE;
              IF NOT fire_counted THEN
                  shot_sequence := shot_sequence + 1;
                  fire_counted := TRUE;
              END_IF;

              IF fire_pulse_count > 0 THEN
                  fire_pulse_count := fire_pulse_count - 1;
              ELSE
                  firing := FALSE;
                  active_fragment_id := 0;
                  lockout_count := 500;
                  gun_state := 5;
              END_IF;
          END_IF;

      5:
          post_fire_lockout := TRUE;
          lockout_seconds_remaining := (lockout_count + 49) / 50;
          IF lockout_count > 0 THEN
              lockout_count := lockout_count - 1;
          ELSE
              post_fire_lockout := FALSE;
              lockout_seconds_remaining := 0;
              fire_counted := FALSE;
              IF command_pending THEN
                  active_fragment_id := pending_fragment_id;
                  gun_state := 2;
              ELSE
                  gun_state := 1;
              END_IF;
          END_IF;

      6:
          ready := TRUE;
          on_target := TRUE;
          IF NOT accepted_maintenance_enabled AND (accepted_mode <> 2) THEN
              gun_state := 1;
          END_IF;
          IF command_pending THEN
              active_fragment_id := 0;
              gun_state := 2;
          END_IF;
          IF accepted_fire_request THEN
              IF accepted_admin_override_active AND safety_fire_permissive THEN
                  fire_pulse_count := 2;
                  fire_counted := FALSE;
                  gun_state := 4;
              ELSE
                  fault_active := TRUE;
                  fault_code := 2;
                  gun_state := 7;
              END_IF;
          END_IF;

      7:
          fault_active := TRUE;
          ready := FALSE;

  ELSE
      fault_active := TRUE;
      fault_code := 5;
      gun_state := 7;
  END_CASE;

  ready := gun_state = 1;
  settling := gun_state = 3;
  firing := gun_state = 4;
  post_fire_lockout := gun_state = 5;
  on_target := (current_azimuth_x10 = pending_azimuth_x10) AND (current_elevation_x10 = pending_elevation_x10);
  fault_active := fault_active OR (gun_state = 7);
  IF gun_state = 7 THEN
      ready := FALSE;
  END_IF;
END_FUNCTION_BLOCK

PROGRAM main
  VAR
    Gun : FB_GunControl;
    command_sequence_udint : UDINT := 0;
    sequence_high_word : UDINT := 0;
    sequence_low_word : UDINT := 0;
    assigned_fragment_dint : DINT := 0;
    high_word : DINT := 0;
    low_word : DINT := 0;
    safety_fire_lease_last : INT := 0;
    safety_fire_lease_age_scans : INT := 101;
    safety_extended_lease_last : INT := 0;
    safety_extended_lease_age_scans : INT := 101;
  END_VAR
  VAR
    GUN_COMMAND_SEQUENCE_LOW AT %MW0 : INT := 0;
    GUN_COMMAND_SEQUENCE_HIGH AT %MW1 : INT := 0;
    GUN_MODE_COMMAND AT %MW2 : INT := 0;
    GUN_TARGET_AZIMUTH_X10 AT %MW3 : INT := 0;
    GUN_TARGET_ELEVATION_X10 AT %MW4 : INT := 50;
    GUN_ASSIGNED_FRAGMENT_ID_LOW AT %MW5 : INT := 0;
    GUN_ASSIGNED_FRAGMENT_ID_HIGH AT %MW6 : INT := 0;
    GUN_FIRE_REQUEST AT %MW7 : INT := 0;
    GUN_MAINTENANCE_ENABLED AT %MW8 : INT := 0;
    GUN_ADMIN_OVERRIDE_ACTIVE AT %MW9 : INT := 0;
    GUN_FAULT_RESET AT %MW10 : INT := 0;
    GUN_SAFETY_FIRE_PERMISSIVE_IN AT %MW11 : INT := 0;
    GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN AT %MW12 : INT := 0;
    GUN_ACCEPTED_COMMAND_SEQUENCE_LOW AT %QW20 : INT := 0;
    GUN_ACCEPTED_COMMAND_SEQUENCE_HIGH AT %QW21 : INT := 0;
    GUN_STATE AT %QW22 : INT := 0;
    GUN_CURRENT_AZIMUTH_X10 AT %QW23 : INT := 0;
    GUN_CURRENT_ELEVATION_X10 AT %QW24 : INT := 50;
    GUN_ACTIVE_FRAGMENT_ID_LOW AT %QW25 : INT := 0;
    GUN_ACTIVE_FRAGMENT_ID_HIGH AT %QW26 : INT := 0;
    GUN_TARGET_ACCEPTED AT %QW27 : INT := 0;
    GUN_ON_TARGET AT %QW28 : INT := 0;
    GUN_SETTLING AT %QW29 : INT := 0;
    GUN_FIRING AT %QW30 : INT := 0;
    GUN_POST_FIRE_LOCKOUT AT %QW31 : INT := 0;
    GUN_READY AT %QW32 : INT := 0;
    GUN_FAULT_ACTIVE AT %QW33 : INT := 0;
    GUN_FAULT_CODE AT %QW34 : INT := 0;
    GUN_SHOT_SEQUENCE_LOW AT %QW35 : INT := 0;
    GUN_SHOT_SEQUENCE_HIGH AT %QW36 : INT := 0;
    GUN_SETTLING_SECONDS_REMAINING AT %QW37 : INT := 0;
    GUN_LOCKOUT_SECONDS_REMAINING AT %QW38 : INT := 0;
    GUN_SAFETY_FIRE_PERMISSIVE_OUT AT %QW39 : INT := 0;
    GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_OUT AT %QW40 : INT := 0;
  END_VAR

  IF GUN_COMMAND_SEQUENCE_LOW < 0 THEN
      sequence_low_word := DINT_TO_UDINT(INT_TO_DINT(GUN_COMMAND_SEQUENCE_LOW) + 65536);
  ELSE
      sequence_low_word := INT_TO_UDINT(GUN_COMMAND_SEQUENCE_LOW);
  END_IF;
  IF GUN_COMMAND_SEQUENCE_HIGH < 0 THEN
      sequence_high_word := DINT_TO_UDINT(INT_TO_DINT(GUN_COMMAND_SEQUENCE_HIGH) + 65536);
  ELSE
      sequence_high_word := INT_TO_UDINT(GUN_COMMAND_SEQUENCE_HIGH);
  END_IF;
  command_sequence_udint := sequence_low_word + (sequence_high_word * UDINT#65536);

  (* Sequence 0 is reserved for startup/no-command. Rollover advances from
     16#FFFFFFFF to 1, and FB_GunControl performs the rollover-aware compare. *)

  assigned_fragment_dint := INT_TO_DINT(GUN_ASSIGNED_FRAGMENT_ID_LOW);
  IF GUN_ASSIGNED_FRAGMENT_ID_LOW < 0 THEN
      assigned_fragment_dint := assigned_fragment_dint + 65536;
  END_IF;
  IF GUN_ASSIGNED_FRAGMENT_ID_HIGH > 0 THEN
      assigned_fragment_dint := assigned_fragment_dint + (INT_TO_DINT(GUN_ASSIGNED_FRAGMENT_ID_HIGH) * 65536);
  END_IF;

  IF GUN_SAFETY_FIRE_PERMISSIVE_IN <> safety_fire_lease_last THEN
      safety_fire_lease_last := GUN_SAFETY_FIRE_PERMISSIVE_IN;
      safety_fire_lease_age_scans := 0;
  ELSE
      IF safety_fire_lease_age_scans < 101 THEN
          safety_fire_lease_age_scans := safety_fire_lease_age_scans + 1;
      END_IF;
  END_IF;

  IF GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN <> safety_extended_lease_last THEN
      safety_extended_lease_last := GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN;
      safety_extended_lease_age_scans := 0;
  ELSE
      IF safety_extended_lease_age_scans < 101 THEN
          safety_extended_lease_age_scans := safety_extended_lease_age_scans + 1;
      END_IF;
  END_IF;

  Gun(
      command_sequence := command_sequence_udint,
      mode_command := GUN_MODE_COMMAND,
      target_azimuth_x10 := GUN_TARGET_AZIMUTH_X10,
      target_elevation_x10 := GUN_TARGET_ELEVATION_X10,
      assigned_fragment_id := assigned_fragment_dint,
      fire_request := GUN_FIRE_REQUEST <> 0,
      maintenance_enabled := GUN_MAINTENANCE_ENABLED <> 0,
      admin_override_active := GUN_ADMIN_OVERRIDE_ACTIVE <> 0,
      fault_reset := GUN_FAULT_RESET <> 0,
      safety_fire_permissive := (GUN_SAFETY_FIRE_PERMISSIVE_IN <> 0) AND (safety_fire_lease_age_scans <= 100),
      safety_extended_motion_permissive := (GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN <> 0) AND (safety_extended_lease_age_scans <= 100)
  );

  sequence_high_word := Gun.accepted_command_sequence / UDINT#65536;
  sequence_low_word := Gun.accepted_command_sequence - (sequence_high_word * UDINT#65536);
  IF sequence_low_word > UDINT#32767 THEN
      GUN_ACCEPTED_COMMAND_SEQUENCE_LOW := DINT_TO_INT(UDINT_TO_DINT(sequence_low_word) - 65536);
  ELSE
      GUN_ACCEPTED_COMMAND_SEQUENCE_LOW := UDINT_TO_INT(sequence_low_word);
  END_IF;
  IF sequence_high_word > UDINT#32767 THEN
      GUN_ACCEPTED_COMMAND_SEQUENCE_HIGH := DINT_TO_INT(UDINT_TO_DINT(sequence_high_word) - 65536);
  ELSE
      GUN_ACCEPTED_COMMAND_SEQUENCE_HIGH := UDINT_TO_INT(sequence_high_word);
  END_IF;

  GUN_STATE := Gun.gun_state;
  GUN_CURRENT_AZIMUTH_X10 := Gun.current_azimuth_x10;
  GUN_CURRENT_ELEVATION_X10 := Gun.current_elevation_x10;

  high_word := Gun.active_fragment_id / 65536;
  low_word := Gun.active_fragment_id - (high_word * 65536);
  IF low_word > 32767 THEN
      GUN_ACTIVE_FRAGMENT_ID_LOW := DINT_TO_INT(low_word - 65536);
  ELSE
      GUN_ACTIVE_FRAGMENT_ID_LOW := DINT_TO_INT(low_word);
  END_IF;
  GUN_ACTIVE_FRAGMENT_ID_HIGH := DINT_TO_INT(high_word);

  IF Gun.target_accepted THEN GUN_TARGET_ACCEPTED := 1; ELSE GUN_TARGET_ACCEPTED := 0; END_IF;
  IF Gun.on_target THEN GUN_ON_TARGET := 1; ELSE GUN_ON_TARGET := 0; END_IF;
  IF Gun.settling THEN GUN_SETTLING := 1; ELSE GUN_SETTLING := 0; END_IF;
  IF Gun.firing THEN GUN_FIRING := 1; ELSE GUN_FIRING := 0; END_IF;
  IF Gun.post_fire_lockout THEN GUN_POST_FIRE_LOCKOUT := 1; ELSE GUN_POST_FIRE_LOCKOUT := 0; END_IF;
  IF Gun.ready THEN GUN_READY := 1; ELSE GUN_READY := 0; END_IF;
  IF Gun.fault_active THEN GUN_FAULT_ACTIVE := 1; ELSE GUN_FAULT_ACTIVE := 0; END_IF;

  GUN_FAULT_CODE := Gun.fault_code;

  high_word := Gun.shot_sequence / 65536;
  low_word := Gun.shot_sequence - (high_word * 65536);
  IF low_word > 32767 THEN
      GUN_SHOT_SEQUENCE_LOW := DINT_TO_INT(low_word - 65536);
  ELSE
      GUN_SHOT_SEQUENCE_LOW := DINT_TO_INT(low_word);
  END_IF;
  GUN_SHOT_SEQUENCE_HIGH := DINT_TO_INT(high_word);

  GUN_SETTLING_SECONDS_REMAINING := Gun.settling_seconds_remaining;
  GUN_LOCKOUT_SECONDS_REMAINING := Gun.lockout_seconds_remaining;
  GUN_SAFETY_FIRE_PERMISSIVE_OUT := GUN_SAFETY_FIRE_PERMISSIVE_IN;
  GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_OUT := GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN;
END_PROGRAM


CONFIGURATION Config0

  RESOURCE Res0 ON PLC
    TASK task0(INTERVAL := T#20ms,PRIORITY := 1);
    PROGRAM instance0 WITH task0 : main;
  END_RESOURCE
END_CONFIGURATION
