using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class InMemoryMaintenanceRequestStore : IMaintenanceRequestStore
{
    private readonly object _lock = new();
    private readonly List<MaintenanceRequestViewModel> _requests = [];
    private int _nextId = 1000;

    public IReadOnlyList<MaintenanceRequestViewModel> List()
    {
        lock (_lock)
        {
            return _requests
                .OrderByDescending(request => request.RequestedAt)
                .Select(Clone)
                .ToList();
        }
    }

    public MaintenanceRequestViewModel Submit(
        int gunNumber,
        string notes,
        string requestedBy,
        string requestType = "Maintenance",
        string sourceApp = "engineering-app",
        string? assetId = null,
        int? panelId = null,
        string? requestSubType = null)
    {
        lock (_lock)
        {
            var normalizedType = string.Equals(requestType, "Part", StringComparison.OrdinalIgnoreCase) ? "Part" : "Maintenance";
            var kind = normalizedType == "Part" ? "PartRequest" : "MaintenanceMode";
            var request = new MaintenanceRequestViewModel(
                Id: ++_nextId,
                GunNumber: gunNumber,
                GunName: $"Gun {gunNumber}",
                RequestType: normalizedType,
                RequestKind: kind,
                RequestSubType: string.IsNullOrWhiteSpace(requestSubType)
                    ? (normalizedType == "Part" ? "RequestPart" : "EnableMaintenance")
                    : requestSubType.Trim(),
                RequestedBy: requestedBy,
                SourceApp: string.IsNullOrWhiteSpace(sourceApp) ? "engineering-app" : sourceApp.Trim(),
                AssetId: string.IsNullOrWhiteSpace(assetId) ? $"GUN-{gunNumber}" : assetId.Trim(),
                PanelId: panelId,
                Status: "Pending",
                Notes: string.IsNullOrWhiteSpace(notes) ? "Maintenance requested from operator panel." : notes.Trim(),
                RequestedAt: DateTime.Now,
                ReviewedAt: null,
                ReviewedBy: null,
                ReviewNotes: null,
                SafeRouteHint: kind == "PartRequest" ? $"/api/part-requests/{_nextId}" : $"/api/maintenance/requests/{_nextId}");

            _requests.Add(request);
            return Clone(request);
        }
    }

    public MaintenanceRequestViewModel? Approve(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Approved", reviewedBy, reviewNotes);
    }

    public MaintenanceRequestViewModel? Deny(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Denied", reviewedBy, reviewNotes);
    }

    public MaintenanceRequestViewModel? Cancel(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Canceled", reviewedBy, reviewNotes, pendingOnly: true);
    }

    public MaintenanceRequestViewModel? Complete(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Completed", reviewedBy, reviewNotes);
    }

    private MaintenanceRequestViewModel? Review(int requestId, string status, string reviewedBy, string reviewNotes, bool pendingOnly = false)
    {
        lock (_lock)
        {
            var index = _requests.FindIndex(request => request.Id == requestId);
            if (index < 0)
            {
                return null;
            }

            if (pendingOnly && !string.Equals(_requests[index].Status, "Pending", StringComparison.OrdinalIgnoreCase))
            {
                return null;
            }

            var reviewed = _requests[index] with
            {
                Status = status,
                ReviewedAt = DateTime.Now,
                ReviewedBy = reviewedBy,
                ReviewNotes = reviewNotes
            };
            _requests[index] = reviewed;
            return Clone(reviewed);
        }
    }

    private static MaintenanceRequestViewModel Clone(MaintenanceRequestViewModel request)
    {
        return request with { };
    }
}
