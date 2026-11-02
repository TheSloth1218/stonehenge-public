using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class MockEngineeringDashboardData : IEngineeringDashboardData
{
    public Task<EngineeringOverviewViewModel> GetOverviewAsync(CancellationToken cancellationToken = default)
    {
        var now = DateTime.Now;

        return Task.FromResult(new EngineeringOverviewViewModel
        {
            Guns = BuildGuns(now),
            OperationLog =
            [
                new OperationLogEntry(1001, now.AddMinutes(-1), "Middleware", "Status Poll", "Success", "Refreshed gun telemetry snapshot"),
                new OperationLogEntry(1002, now.AddMinutes(-3), "Safety PLC", "Interlock Check", "Warning", "Gun 3 reported interlocked state"),
                new OperationLogEntry(1003, now.AddMinutes(-5), "Engineering App", "Dashboard Load", "Success", "MVC dashboard rendered with mock data"),
                new OperationLogEntry(1004, now.AddMinutes(-7), "Middleware", "Gun 7 Query", "Fault", "Fault state received from simulated runtime"),
                new OperationLogEntry(1005, now.AddMinutes(-9), "Operator Console", "View Overview", "Success", "Read-only overview accessed")
            ]
        });
    }

    public Task<AutoStatusViewModel> GetAutoStatusAsync(int? gunNumber, CancellationToken cancellationToken = default)
    {
        var now = DateTime.Now;
        var guns = BuildGuns(now);
        var selectedGunNumber = gunNumber is >= 1 and <= 8 ? gunNumber.Value : 1;
        var selectedGun = guns.Single(gun => gun.GunNumber == selectedGunNumber);

        return Task.FromResult(new AutoStatusViewModel
        {
            Guns = guns,
            SelectedGunNumber = selectedGunNumber,
            SelectedGun = selectedGun,
            RuntimeStatus = new AutoRuntimeStatusViewModel(
                AutoMode: selectedGun.StatusClass is "offline" or "fault" ? "Unavailable" : "Standby",
                AutoState: selectedGun.StatusClass switch
                {
                    "online" => "Monitoring",
                    "warning" => "Interlock Hold",
                    "fault" => "Faulted",
                    "offline" => "Disconnected",
                    _ => "Unknown"
                },
                HmiConnection: "Placeholder",
                MiddlewareConnection: "Mocked",
                LastSequence: $"SEQ-MOCK-{selectedGun.GunNumber:00}",
                LastAutoEvent: "No real control action executed",
                LastUpdated: now.AddSeconds(-8),
                OperatorResetAvailable: false),
            RecentEvents =
            [
                new OperationLogEntry(2001, now.AddMinutes(-1), "Auto Status", "Status Poll", "Success", "Auto status poll completed"),
                new OperationLogEntry(2002, now.AddMinutes(-2), selectedGun.Name, "Telemetry Refresh", "Success", "Selected gun telemetry refreshed"),
                new OperationLogEntry(2003, now.AddMinutes(-3), "HMI Placeholder", "Initialize", "Success", "HMI placeholder initialized"),
                new OperationLogEntry(2004, now.AddMinutes(-4), "Operator Actions", "No-op", "Success", "No real control action executed")
            ]
        });
    }

    public Task<ManualControlViewModel> GetManualControlAsync(int? gunNumber, CancellationToken cancellationToken = default)
    {
        var now = DateTime.Now;
        var guns = BuildGuns(now);
        var selectedGunNumber = gunNumber is >= 1 and <= 8 ? gunNumber.Value : 1;
        var selectedGun = guns.Single(gun => gun.GunNumber == selectedGunNumber);

        return Task.FromResult(new ManualControlViewModel
        {
            Guns = guns,
            SelectedGunNumber = selectedGunNumber,
            SelectedGun = selectedGun,
            RuntimeStatus = new ManualRuntimeStatusViewModel(
                ManualMode: "Manual",
                HmiConnection: "Placeholder",
                MiddlewareConnection: "Mocked",
                OperatorSession: "Dev admin",
                CurrentInterlockState: selectedGun.SafetyState,
                LastManualEvent: "Manual workspace rendered with mock data",
                LastUpdated: now.AddSeconds(-6)),
            FireControl = new FireControlViewModel(
                SafetyKeyUnlocked: false,
                SafetyCoverOpen: false,
                FireButtonEnabled: false,
                SafetyKeyState: "Locked",
                SafetyCoverState: "Closed",
                FireCommandState: "Visual placeholder only",
                LastCommand: "None",
                LastShotSequence: $"SEQ-MOCK-{selectedGun.GunNumber:00}",
                AuthorizationState: "Dev admin",
                InterlockState: selectedGun.SafetyState),
            FilteredGunLog =
            [
                new OperationLogEntry(3001, now.AddMinutes(-1), "Engineering App", "Manual view loaded", "Success", "Selected gun context initialized"),
                new OperationLogEntry(3002, now.AddMinutes(-2), "HMI Placeholder", "Manual panel render", "Success", "Mock manual HMI displayed"),
                new OperationLogEntry(3003, now.AddMinutes(-3), "Middleware", "Selected gun status poll", "Success", "Telemetry snapshot refreshed"),
                new OperationLogEntry(3004, now.AddMinutes(-4), "Safety PLC", "Manual safety check", "Simulated", "No real PLC call executed"),
                new OperationLogEntry(3005, now.AddMinutes(-5), "Operator Console", "Fire control panel viewed", "Logged", "Visual-only protected panel rendered")
            ]
        });
    }

    public Task<MaintenancePanelViewModel> GetMaintenancePanelAsync(int? gunNumber, CancellationToken cancellationToken = default)
    {
        var now = DateTime.Now;
        var guns = BuildMaintenanceGuns();
        var selectedGunNumber = gunNumber is >= 1 and <= 8 ? gunNumber.Value : 1;
        var selectedGun = guns.Single(gun => gun.GunNumber == selectedGunNumber);

        return Task.FromResult(new MaintenancePanelViewModel
        {
            Guns = guns,
            SelectedGunNumber = selectedGunNumber,
            SelectedGun = selectedGun,
            OrderTemplate = new MaintenanceOrderTemplateViewModel(
                OrderId: $"MNT-MOCK-{selectedGun.GunNumber:00}",
                Priority: selectedGun.MaintenanceClass is "fault" ? "High" : "Normal",
                RequestedBy: "Engineering Console",
                TargetVendor: "App-local maintenance draft",
                RequestedWindow: "Next approved maintenance window",
                Summary: $"Mock maintenance request template for {selectedGun.Name}",
                Status: "Draft placeholder only"),
            MaintenanceEvents =
            [
                new OperationLogEntry(4001, now.AddMinutes(-2), "Engineering App", "Maintenance view loaded", "Success", "Mock maintenance panel rendered"),
                new OperationLogEntry(4002, now.AddMinutes(-6), selectedGun.Name, "Maintenance context selected", "Success", "Selected gun maintenance state displayed"),
                new OperationLogEntry(4003, now.AddMinutes(-11), "Maintenance Planner", "Order template viewed", "Draft", "No middleware, vendor, or PLC call executed"),
                new OperationLogEntry(4004, now.AddMinutes(-18), "Admin Boundary", "Permission note", "Required later", "App-side maintenance gating will require admin permissions")
            ]
        });
    }

    private static List<GunStatusViewModel> BuildGuns(DateTime now)
    {
        return
        [
            new GunStatusViewModel(1, "Gun 1", "Online", "online", 140.0, 35.0, "Fire Permitted", now.AddSeconds(-12)),
            new GunStatusViewModel(2, "Gun 2", "Online", "online", 95.0, 28.5, "Fire Permitted", now.AddSeconds(-16)),
            new GunStatusViewModel(3, "Gun 3", "Warning", "warning", 210.0, 42.0, "Fire Blocked", now.AddSeconds(-22)),
            new GunStatusViewModel(4, "Gun 4", "Online", "online", 15.0, 18.0, "Fire Permitted", now.AddSeconds(-18)),
            new GunStatusViewModel(5, "Gun 5", "Offline", "offline", 0.0, 0.0, "Fire Blocked", now.AddMinutes(-4)),
            new GunStatusViewModel(6, "Gun 6", "Online", "online", 300.0, 22.5, "Fire Permitted", now.AddSeconds(-19)),
            new GunStatusViewModel(7, "Gun 7", "Fault", "fault", 175.0, 10.0, "Fire Blocked", now.AddSeconds(-45)),
            new GunStatusViewModel(8, "Gun 8", "Online", "online", 260.0, 31.5, "Fire Permitted", now.AddSeconds(-14))
        ];
    }

    private static List<GunMaintenanceStatusViewModel> BuildMaintenanceGuns()
    {
        return
        [
            new GunMaintenanceStatusViewModel(1, "Gun 1", "Ready", "online", "Auto capable", "OK", "online", 32, 200, "MNT-MOCK-01", "Today 20:10", true),
            new GunMaintenanceStatusViewModel(2, "Gun 2", "Ready", "online", "Auto capable", "OK", "online", 77, 200, "MNT-MOCK-02", "Today 19:55", true),
            new GunMaintenanceStatusViewModel(3, "Gun 3", "Needs Inspection", "warning", "Maintenance hold", "Service Needed", "warning", 214, 200, "MNT-MOCK-03", "Today 18:42", true),
            new GunMaintenanceStatusViewModel(4, "Gun 4", "Ready", "online", "Auto capable", "OK", "online", 19, 200, "MNT-MOCK-04", "Today 19:20", true),
            new GunMaintenanceStatusViewModel(5, "Gun 5", "Offline", "offline", "Maintenance mode", "Service Needed", "warning", 241, 200, "MNT-MOCK-05", "Yesterday 22:15", true),
            new GunMaintenanceStatusViewModel(6, "Gun 6", "Ready", "online", "Auto capable", "OK", "online", 126, 200, "MNT-MOCK-06", "Today 20:01", true),
            new GunMaintenanceStatusViewModel(7, "Gun 7", "Fault Review", "fault", "Locked out", "Service Needed", "warning", 305, 200, "MNT-MOCK-07", "Today 17:33", true),
            new GunMaintenanceStatusViewModel(8, "Gun 8", "Ready", "online", "Auto capable", "OK", "online", 88, 200, "MNT-MOCK-08", "Today 19:48", true)
        ];
    }
}
