namespace Stonehenge.Engineering.App.Models;

public sealed class MaintenancePanelViewModel
{
    public required List<GunMaintenanceStatusViewModel> Guns { get; init; }

    public required int SelectedGunNumber { get; init; }

    public required GunMaintenanceStatusViewModel SelectedGun { get; init; }

    public required MaintenanceOrderTemplateViewModel OrderTemplate { get; init; }

    public required List<OperationLogEntry> MaintenanceEvents { get; set; }

    public List<MaintenanceRequestViewModel> MaintenanceRequests { get; set; } = [];

    public List<MaintenanceRequestViewModel> PartRequests { get; set; } = [];
}
