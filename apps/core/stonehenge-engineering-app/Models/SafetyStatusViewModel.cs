namespace Stonehenge.Engineering.App.Models;

public sealed record SafetyStatusViewModel(
    bool Enabled,
    bool EnforcementEnabled,
    bool Connected,
    bool Fresh,
    bool Ready,
    string State,
    bool GlobalFirePermissive,
    bool GlobalInhibitActive,
    string GlobalInhibitReason,
    bool FaultActive,
    string FaultName,
    bool EmergencyStopActive,
    bool MaintenanceModeActive,
    int PermissiveGunCount,
    int InhibitedGunCount,
    DateTime LastUpdated,
    IReadOnlyList<SafetyGunStatusViewModel> Guns);

public sealed record SafetyGunStatusViewModel(
    int GunNumber,
    bool FirePermissive,
    bool InhibitActive,
    string InhibitReason,
    bool Enabled,
    bool MaintenanceInhibit,
    bool ExternalInhibit);
