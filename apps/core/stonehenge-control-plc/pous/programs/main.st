PROGRAM main
  VAR
    Gun : FB_GunControl;
    GUN_COMMAND_SEQUENCE_LOW : INT AT %MW0 := 0;
    GUN_COMMAND_SEQUENCE_HIGH : INT AT %MW1 := 0;
    GUN_MODE_COMMAND : INT AT %MW2 := 0;
    GUN_TARGET_AZIMUTH_X10 : INT AT %MW3 := 0;
    GUN_TARGET_ELEVATION_X10 : INT AT %MW4 := 50;
    GUN_ASSIGNED_FRAGMENT_ID_LOW : INT AT %MW5 := 0;
    GUN_ASSIGNED_FRAGMENT_ID_HIGH : INT AT %MW6 := 0;
    GUN_FIRE_REQUEST : INT AT %MW7 := 0;
    GUN_MAINTENANCE_ENABLED : INT AT %MW8 := 0;
    GUN_ADMIN_OVERRIDE_ACTIVE : INT AT %MW9 := 0;
    GUN_FAULT_RESET : INT AT %MW10 := 0;
    GUN_SAFETY_FIRE_PERMISSIVE_IN : INT AT %MW11 := 0;
    GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN : INT AT %MW12 := 0;
    GUN_ACCEPTED_COMMAND_SEQUENCE_LOW : INT AT %QW20 := 0;
    GUN_ACCEPTED_COMMAND_SEQUENCE_HIGH : INT AT %QW21 := 0;
    GUN_STATE : INT AT %QW22 := 0;
    GUN_CURRENT_AZIMUTH_X10 : INT AT %QW23 := 0;
    GUN_CURRENT_ELEVATION_X10 : INT AT %QW24 := 50;
    GUN_ACTIVE_FRAGMENT_ID_LOW : INT AT %QW25 := 0;
    GUN_ACTIVE_FRAGMENT_ID_HIGH : INT AT %QW26 := 0;
    GUN_TARGET_ACCEPTED : INT AT %QW27 := 0;
    GUN_ON_TARGET : INT AT %QW28 := 0;
    GUN_SETTLING : INT AT %QW29 := 0;
    GUN_FIRING : INT AT %QW30 := 0;
    GUN_POST_FIRE_LOCKOUT : INT AT %QW31 := 0;
    GUN_READY : INT AT %QW32 := 0;
    GUN_FAULT_ACTIVE : INT AT %QW33 := 0;
    GUN_FAULT_CODE : INT AT %QW34 := 0;
    GUN_SHOT_SEQUENCE_LOW : INT AT %QW35 := 0;
    GUN_SHOT_SEQUENCE_HIGH : INT AT %QW36 := 0;
    GUN_SETTLING_SECONDS_REMAINING : INT AT %QW37 := 0;
    GUN_LOCKOUT_SECONDS_REMAINING : INT AT %QW38 := 0;
    GUN_SAFETY_FIRE_PERMISSIVE_OUT : INT AT %QW39 := 0;
    GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_OUT : INT AT %QW40 := 0;
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
