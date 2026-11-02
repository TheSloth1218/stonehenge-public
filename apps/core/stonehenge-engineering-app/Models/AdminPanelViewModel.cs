namespace Stonehenge.Engineering.App.Models;

public sealed class AdminPanelViewModel
{
    public int SelectedHmiGunNumber { get; init; } = 1;

    public bool ResetToolboxReauthorized { get; init; }

    public string? ResetToolboxReauthToken { get; init; }

    public DateTimeOffset? ResetToolboxReauthExpiresUtc { get; init; }

    public required List<MaintenanceRequestViewModel> MaintenanceRequests { get; init; }

    public required List<OperationLogEntry> AdminEvents { get; init; }

    public required SafetyStatusViewModel SafetyStatus { get; init; }

    public FuxaPanelFrameViewModel? FuxaPanel { get; init; }

    public string? VendorSyncMessage { get; init; }

    public int PendingMaintenanceCount => MaintenanceRequests.Count(request => request.Status == "Pending");
}
