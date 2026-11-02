namespace Stonehenge.Engineering.App.Services;

public interface IStonehengeMiddlewareClient
{
    Task<MiddlewareStatusSnapshot> GetStatusAsync(CancellationToken cancellationToken = default);

    Task<MiddlewareGunRuntimeSnapshot?> GetGunRuntimeStatusAsync(int gunNumber, CancellationToken cancellationToken = default);

    Task<MiddlewareModeChangeResult> SetGunModeAsync(int gunNumber, string mode, CancellationToken cancellationToken = default);

    Task<MiddlewareCommandResult> FireGunAsync(int gunNumber, CancellationToken cancellationToken = default);

    Task<MiddlewareCommandResult> SubmitManualCommandAsync(
        int gunNumber,
        double targetAzimuth,
        double targetElevation,
        bool fireRequest,
        CancellationToken cancellationToken = default);

    Task<MiddlewareSafetySnapshot> GetSafetyAsync(CancellationToken cancellationToken = default);

    Task<MiddlewareSafetySnapshot> SetGlobalSafetyAsync(
        bool? globalEnable,
        bool? emergencyStop,
        bool? maintenanceMode,
        CancellationToken cancellationToken = default);

    Task<MiddlewareSafetySnapshot> SetGunSafetyAsync(
        int gunNumber,
        bool? enabled,
        bool? maintenanceInhibit,
        bool? externalInhibit,
        CancellationToken cancellationToken = default);

    Task<MiddlewareMaintenanceResult> RunMaintenanceActionAsync(
        string action,
        bool execute,
        string confirm,
        CancellationToken cancellationToken = default);
}

public sealed record MiddlewareStatusSnapshot(
    bool Running,
    bool IsLive,
    DateTimeOffset ObservedAt,
    IReadOnlyList<MiddlewareGunSnapshot> Guns);

public sealed record MiddlewareGunSnapshot(
    int GunNumber,
    string Name,
    string Mode,
    string State,
    double CurrentAzimuth,
    double CurrentElevation,
    double TargetAzimuth,
    double TargetElevation,
    string SafetyState,
    bool FaultActive,
    string FaultCode,
    string HardLimitStatus,
    bool MaintenanceEnabled,
    int ShotSequence);

public sealed record MiddlewareGunRuntimeSnapshot(
    int GunNumber,
    string State,
    double CurrentAzimuth,
    double CurrentElevation,
    int AcceptedSequence,
    int ShotSequence);

public sealed record MiddlewareModeChangeResult(
    bool Success,
    int GunNumber,
    string RequestedMode,
    string? Mode,
    string? PreviousMode,
    string? Reason,
    string? Error);

public sealed record MiddlewareCommandResult(
    bool Success,
    int GunNumber,
    string Action,
    string? State,
    string? Mode,
    string? Reason,
    string? Error);

public sealed record MiddlewareMaintenanceResult(
    bool Success,
    bool Enabled,
    string Action,
    bool Executed,
    string Status,
    string? Error,
    string? Stdout,
    string? Stderr);

public sealed record MiddlewareSafetySnapshot(
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
    DateTimeOffset ObservedAt,
    IReadOnlyList<MiddlewareSafetyGunSnapshot> Guns);

public sealed record MiddlewareSafetyGunSnapshot(
    int GunNumber,
    bool FirePermissive,
    bool InhibitActive,
    string InhibitReason,
    bool Enabled,
    bool MaintenanceInhibit,
    bool ExternalInhibit);
