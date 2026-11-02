#include "generated.hpp"

#include <iostream>

using strucpp::Program_MAIN;

void submitFire(Program_MAIN& program) {
    program.GUN_MODE_COMMAND = 1;
    program.GUN_TARGET_AZIMUTH_X10 = 0;
    program.GUN_TARGET_ELEVATION_X10 = 50;
    program.GUN_FIRE_REQUEST = 1;
    program.GUN_COMMAND_SEQUENCE_LOW = 1;
    program.run();
}

void submitExtendedMove(Program_MAIN& program) {
    program.GUN_MODE_COMMAND = 1;
    program.GUN_TARGET_AZIMUTH_X10 = 0;
    program.GUN_TARGET_ELEVATION_X10 = -300;
    program.GUN_ADMIN_OVERRIDE_ACTIVE = 1;
    program.GUN_COMMAND_SEQUENCE_LOW = 1;
    program.run();
}

bool revokedLeaseBlocksFire() {
    Program_MAIN program;
    program.run();
    program.GUN_SAFETY_FIRE_PERMISSIVE_IN = 1;
    submitFire(program);
    program.GUN_SAFETY_FIRE_PERMISSIVE_IN = 0;
    for (int scan = 0; scan < 252; ++scan) {
        program.run();
    }
    return program.GUN_SHOT_SEQUENCE_LOW == 0 && program.GUN_FAULT_CODE == 2;
}

bool expiredLeaseBlocksFire() {
    Program_MAIN program;
    program.run();
    program.GUN_SAFETY_FIRE_PERMISSIVE_IN = 1;
    submitFire(program);
    for (int scan = 0; scan < 252; ++scan) {
        program.run();
    }
    return program.GUN_SHOT_SEQUENCE_LOW == 0 && program.GUN_FAULT_CODE == 2;
}

bool refreshedLeaseAllowsFire() {
    Program_MAIN program;
    program.run();
    program.GUN_SAFETY_FIRE_PERMISSIVE_IN = 1;
    submitFire(program);
    for (int scan = 0; scan < 252; ++scan) {
        program.GUN_SAFETY_FIRE_PERMISSIVE_IN = scan + 2;
        program.run();
    }
    return program.GUN_SHOT_SEQUENCE_LOW == 1 && program.GUN_FAULT_CODE == 0;
}

bool revokedExtendedLeaseStopsMotion() {
    Program_MAIN program;
    program.run();
    program.GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN = 1;
    submitExtendedMove(program);
    program.GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN = 0;
    program.run();
    return program.GUN_FAULT_CODE == 2 && program.GUN_CURRENT_ELEVATION_X10 > -300;
}

bool expiredExtendedLeaseStopsMotion() {
    Program_MAIN program;
    program.run();
    program.GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN = 1;
    submitExtendedMove(program);
    for (int scan = 0; scan < 102; ++scan) {
        program.run();
    }
    return program.GUN_FAULT_CODE == 2;
}

bool refreshedExtendedLeaseAllowsMotion() {
    Program_MAIN program;
    program.run();
    program.GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN = 1;
    submitExtendedMove(program);
    for (int scan = 0; scan < 152; ++scan) {
        program.GUN_SAFETY_EXTENDED_MOTION_PERMISSIVE_IN = scan + 2;
        program.run();
    }
    return program.GUN_CURRENT_ELEVATION_X10 == -300 && program.GUN_FAULT_CODE == 0;
}

int main() {
    if (!revokedLeaseBlocksFire()) {
        std::cerr << "revoked safety lease allowed firing\n";
        return 1;
    }
    if (!expiredLeaseBlocksFire()) {
        std::cerr << "expired safety lease allowed firing\n";
        return 1;
    }
    if (!refreshedLeaseAllowsFire()) {
        std::cerr << "refreshed safety lease blocked normal firing\n";
        return 1;
    }
    if (!revokedExtendedLeaseStopsMotion()) {
        std::cerr << "revoked extended-motion lease allowed continued motion\n";
        return 1;
    }
    if (!expiredExtendedLeaseStopsMotion()) {
        std::cerr << "expired extended-motion lease allowed continued motion\n";
        return 1;
    }
    if (!refreshedExtendedLeaseAllowsMotion()) {
        std::cerr << "refreshed extended-motion lease blocked authorized motion\n";
        return 1;
    }
    return 0;
}
