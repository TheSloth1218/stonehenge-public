using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IMaintenanceRequestStore
{
    IReadOnlyList<MaintenanceRequestViewModel> List();

    MaintenanceRequestViewModel Submit(
        int gunNumber,
        string notes,
        string requestedBy,
        string requestType = "Maintenance",
        string sourceApp = "engineering-app",
        string? assetId = null,
        int? panelId = null,
        string? requestSubType = null);

    MaintenanceRequestViewModel? Approve(int requestId, string reviewedBy, string reviewNotes);

    MaintenanceRequestViewModel? Deny(int requestId, string reviewedBy, string reviewNotes);

    MaintenanceRequestViewModel? Cancel(int requestId, string reviewedBy, string reviewNotes);

    MaintenanceRequestViewModel? Complete(int requestId, string reviewedBy, string reviewNotes);
}
