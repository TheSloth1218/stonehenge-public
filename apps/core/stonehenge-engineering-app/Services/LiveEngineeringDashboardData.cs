using Stonehenge.Engineering.App.Models;
using Microsoft.Extensions.Options;

namespace Stonehenge.Engineering.App.Services;

public sealed class LiveEngineeringDashboardData : IEngineeringDashboardData
{
    private readonly int _partServiceThreshold;
    private readonly IStonehengeMiddlewareClient _middlewareClient;
    private readonly IPartStatusStore _partStatusStore;
    private readonly IAppEventStore _appEvents;
    private readonly MockEngineeringDashboardData _fallback = new();
    private readonly ILogger<LiveEngineeringDashboardData> _logger;

    public LiveEngineeringDashboardData(
        IStonehengeMiddlewareClient middlewareClient,
        IPartStatusStore partStatusStore,
        IAppEventStore appEvents,
        ILogger<LiveEngineeringDashboardData> logger,
        IOptions<OperationalOptions> operational)
    {
        _middlewareClient = middlewareClient;
        _partStatusStore = partStatusStore;
        _appEvents = appEvents;
        _logger = logger;
        _partServiceThreshold = operational.Value.MaintenanceShotThreshold;
    }

    public async Task<EngineeringOverviewViewModel> GetOverviewAsync(CancellationToken cancellationToken = default)
    {
        var fallback = await _fallback.GetOverviewAsync(cancellationToken);
        var status = await TryGetStatusAsync(cancellationToken);
        if (status is null || status.Guns.Count == 0)
        {
            return fallback;
        }

        var safety = await TryGetSafetyAsync(cancellationToken);
        return new EngineeringOverviewViewModel
        {
            Guns = MapGuns(status, safety),
            OperationLog = BuildLiveLog(status, "Overview")
        };
    }

    public async Task<AutoStatusViewModel> GetAutoStatusAsync(int? gunNumber, CancellationToken cancellationToken = default)
    {
        var fallback = await _fallback.GetAutoStatusAsync(gunNumber, cancellationToken);
        var status = await TryGetStatusAsync(cancellationToken);
        if (status is null || status.Guns.Count == 0)
        {
            return fallback;
        }

        var safety = await TryGetSafetyAsync(cancellationToken);
        var guns = MapGuns(status, safety);
        var selectedGunNumber = NormalizeGunNumber(gunNumber);
        var selectedGun = guns.Single(gun => gun.GunNumber == selectedGunNumber);
        var liveGun = status.Guns.Single(gun => gun.GunNumber == selectedGunNumber);

        return new AutoStatusViewModel
        {
            Guns = guns,
            SelectedGunNumber = selectedGunNumber,
            SelectedGun = selectedGun,
            RuntimeStatus = new AutoRuntimeStatusViewModel(
                AutoMode: liveGun.Mode,
                AutoState: liveGun.State,
                HmiConnection: IsGunSafetyEnabled(safety, selectedGunNumber) && liveGun.Mode == "auto" ? "Ready" : "Gated",
                MiddlewareConnection: "Live",
                LastSequence: liveGun.ShotSequence > 0 ? $"SEQ-{liveGun.ShotSequence:0000}" : "Idle",
                LastAutoEvent: IsGunSafetyEnabled(safety, selectedGunNumber) ? liveGun.Mode == "auto" ? "Middleware status live" : $"Gun currently in {liveGun.Mode}" : "Gun disabled by safety PLC",
                LastUpdated: status.ObservedAt.LocalDateTime,
                OperatorResetAvailable: liveGun.FaultActive),
            RecentEvents = BuildLiveLog(status, $"Auto view selected {selectedGun.Name}")
        };
    }

    public async Task<ManualControlViewModel> GetManualControlAsync(int? gunNumber, CancellationToken cancellationToken = default)
    {
        var fallback = await _fallback.GetManualControlAsync(gunNumber, cancellationToken);
        var status = await TryGetStatusAsync(cancellationToken);
        if (status is null || status.Guns.Count == 0)
        {
            return fallback;
        }

        var safety = await TryGetSafetyAsync(cancellationToken);
        var guns = MapGuns(status, safety);
        var selectedGunNumber = NormalizeGunNumber(gunNumber);
        var selectedGun = guns.Single(gun => gun.GunNumber == selectedGunNumber);
        var liveGun = status.Guns.Single(gun => gun.GunNumber == selectedGunNumber);

        return new ManualControlViewModel
        {
            Guns = guns,
            SelectedGunNumber = selectedGunNumber,
            SelectedGun = selectedGun,
            RuntimeStatus = new ManualRuntimeStatusViewModel(
                ManualMode: liveGun.Mode,
                HmiConnection: IsGunSafetyEnabled(safety, selectedGunNumber) && liveGun.Mode == "manual" ? "Ready" : "Gated",
                MiddlewareConnection: "Live",
                OperatorSession: "Portal identity pending",
                CurrentInterlockState: selectedGun.SafetyState,
                LastManualEvent: IsGunSafetyEnabled(safety, selectedGunNumber) ? liveGun.Mode == "manual" ? "Manual HMI available" : $"Waiting for {liveGun.Mode} to manual transition" : "Gun disabled by safety PLC",
                LastUpdated: status.ObservedAt.LocalDateTime),
            FireControl = fallback.FireControl with
            {
                InterlockState = selectedGun.SafetyState
            },
            FilteredGunLog = BuildSelectedGunLog(
                _appEvents.List("manual", 25),
                selectedGunNumber,
                BuildLiveLog(status, $"Manual view selected {selectedGun.Name}"))
        };
    }

    public async Task<MaintenancePanelViewModel> GetMaintenancePanelAsync(int? gunNumber, CancellationToken cancellationToken = default)
    {
        var fallback = await _fallback.GetMaintenancePanelAsync(gunNumber, cancellationToken);
        var status = await TryGetStatusAsync(cancellationToken);
        if (status is null || status.Guns.Count == 0)
        {
            return fallback;
        }

        var selectedGunNumber = NormalizeGunNumber(gunNumber);
        var liveGun = status.Guns.Single(gun => gun.GunNumber == selectedGunNumber);
        var safety = await TryGetSafetyAsync(cancellationToken);
        var partSnapshots = status.Guns
            .Select(gun => _partStatusStore.Upsert(
                gun.GunNumber,
                Math.Max(0, gun.ShotSequence),
                _partServiceThreshold,
                BuildPartStatus(gun),
                BuildPartClass(gun)))
            .ToDictionary(record => record.GunNumber);

        var maintenanceGuns = status.Guns.Select(gun =>
        {
            var part = partSnapshots[gun.GunNumber];
            return new GunMaintenanceStatusViewModel(
                gun.GunNumber,
                $"Gun {gun.GunNumber}",
                BuildMaintenanceState(gun),
                BuildMaintenanceClass(gun),
                gun.Mode,
                part.PartStatus,
                part.PartClass,
                part.FiringCount,
                part.ServiceThreshold,
                $"MNT-MOCK-{gun.GunNumber:00}",
                status.ObservedAt.LocalDateTime.ToString("HH:mm:ss"),
                true);
        }).ToList();

        var selectedGun = maintenanceGuns.Single(gun => gun.GunNumber == selectedGunNumber);
        return new MaintenancePanelViewModel
        {
            Guns = maintenanceGuns,
            SelectedGunNumber = selectedGunNumber,
            SelectedGun = selectedGun,
            OrderTemplate = new MaintenanceOrderTemplateViewModel(
                OrderId: $"MNT-MOCK-{selectedGun.GunNumber:00}",
                Priority: liveGun.FaultActive ? "High" : "Normal",
                RequestedBy: "Engineering Console",
                TargetVendor: "App-local maintenance draft",
                RequestedWindow: "Next approved maintenance window",
                Summary: $"Live maintenance context for {selectedGun.Name}",
                Status: "Draft placeholder only"),
            MaintenanceEvents = BuildLiveLog(status, $"Maintenance view selected {selectedGun.Name}")
        };
    }

    private async Task<MiddlewareStatusSnapshot?> TryGetStatusAsync(CancellationToken cancellationToken)
    {
        try
        {
            return await _middlewareClient.GetStatusAsync(cancellationToken);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            _logger.LogWarning(ex, "Middleware status unavailable; falling back to mock dashboard data.");
            return null;
        }
    }

    private async Task<MiddlewareSafetySnapshot?> TryGetSafetyAsync(CancellationToken cancellationToken)
    {
        try
        {
            return await _middlewareClient.GetSafetyAsync(cancellationToken);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            _logger.LogWarning(ex, "Middleware safety unavailable; rendering gun status without safety PLC overlay.");
            return null;
        }
    }

    private static int NormalizeGunNumber(int? gunNumber)
    {
        return gunNumber is >= 1 and <= 8 ? gunNumber.Value : 1;
    }

    private static List<GunStatusViewModel> MapGuns(MiddlewareStatusSnapshot status, MiddlewareSafetySnapshot? safety)
    {
        return status.Guns.Select(gun => new GunStatusViewModel(
            gun.GunNumber,
            $"Gun {gun.GunNumber}",
            BuildGunStatus(gun, safety),
            MapStatusClass(gun, safety),
            gun.CurrentAzimuth,
            gun.CurrentElevation,
            BuildSafetyState(gun, safety),
            status.ObservedAt.LocalDateTime)).ToList();
    }

    private static string BuildGunStatus(MiddlewareGunSnapshot gun, MiddlewareSafetySnapshot? safety)
    {
        if (!IsGunSafetyEnabled(safety, gun.GunNumber))
        {
            return "Disabled";
        }

        if (gun.FaultActive)
        {
            return "Fault";
        }

        if (gun.Mode == "maintenance" || gun.MaintenanceEnabled)
        {
            return "Maintenance";
        }

        return gun.State;
    }

    private static string BuildSafetyState(MiddlewareGunSnapshot gun, MiddlewareSafetySnapshot? safety)
    {
        var safetyGun = safety?.Guns.SingleOrDefault(item => item.GunNumber == gun.GunNumber);
        if (safetyGun is null)
        {
            return FormatSafetyState(gun.SafetyState);
        }

        if (!safetyGun.Enabled)
        {
            return "Fire Blocked";
        }

        if (safetyGun.InhibitActive)
        {
            return "Fire Blocked";
        }

        return safetyGun.FirePermissive ? "Fire Permitted" : FormatSafetyState(gun.SafetyState);
    }

    private static string FormatSafetyState(string safetyState)
    {
        return safetyState.ToLowerInvariant() switch
        {
            "permissive" => "Fire Permitted",
            "interlocked" or "locked out" or "unknown" => "Fire Blocked",
            _ => safetyState
        };
    }

    private static string BuildMaintenanceState(MiddlewareGunSnapshot gun)
    {
        if (gun.FaultActive)
        {
            return "Fault Review";
        }

        if (gun.Mode == "maintenance" || gun.MaintenanceEnabled)
        {
            return "Maintenance";
        }

        return "Ready";
    }

    private static string BuildMaintenanceClass(MiddlewareGunSnapshot gun)
    {
        if (gun.FaultActive)
        {
            return "fault";
        }

        if (gun.Mode == "maintenance" || gun.MaintenanceEnabled)
        {
            return "warning";
        }

        return "online";
    }

    private string BuildPartStatus(MiddlewareGunSnapshot gun)
    {
        return gun.ShotSequence >= _partServiceThreshold ? "Service Needed" : "OK";
    }

    private string BuildPartClass(MiddlewareGunSnapshot gun)
    {
        return gun.ShotSequence >= _partServiceThreshold ? "warning" : "online";
    }

    private static string MapStatusClass(MiddlewareGunSnapshot gun, MiddlewareSafetySnapshot? safety)
    {
        if (!IsGunSafetyEnabled(safety, gun.GunNumber))
        {
            return "offline";
        }

        if (gun.FaultActive)
        {
            return "fault";
        }

        if (gun.Mode == "maintenance" || gun.MaintenanceEnabled)
        {
            return "warning";
        }

        return gun.SafetyState == "Permissive" ? "online" : "warning";
    }

    private static bool IsGunSafetyEnabled(MiddlewareSafetySnapshot? safety, int gunNumber)
    {
        var safetyGun = safety?.Guns.SingleOrDefault(gun => gun.GunNumber == gunNumber);
        return safetyGun?.Enabled ?? true;
    }

    private static List<OperationLogEntry> BuildLiveLog(MiddlewareStatusSnapshot status, string action)
    {
        var now = status.ObservedAt.LocalDateTime;
        return
        [
            new OperationLogEntry(5001, now, "Middleware", "Status Poll", status.Running ? "Success" : "Warning", "Live /api/v1/status snapshot rendered"),
            new OperationLogEntry(5002, now.AddSeconds(-5), "Engineering App", action, "Success", "MVC view hydrated from middleware client"),
            new OperationLogEntry(5003, now.AddSeconds(-10), "Safety Boundary", "Status Map", "Observed", "Safety and mode values are read-only on this view")
        ];
    }

    private static List<OperationLogEntry> BuildSelectedGunLog(
        IReadOnlyList<OperationLogEntry> persistedEvents,
        int gunNumber,
        List<OperationLogEntry> liveEvents)
    {
        var gunLabel = $"Gun {gunNumber}";
        var selectedPersistedEvents = persistedEvents
            .Where(entry =>
                entry.Source.Contains(gunLabel, StringComparison.OrdinalIgnoreCase) ||
                entry.Action.Contains(gunLabel, StringComparison.OrdinalIgnoreCase) ||
                entry.Notes.Contains(gunLabel, StringComparison.OrdinalIgnoreCase))
            .OrderByDescending(entry => entry.Timestamp)
            .ToList();

        return selectedPersistedEvents
            .Concat(liveEvents)
            .Take(20)
            .ToList();
    }
}
