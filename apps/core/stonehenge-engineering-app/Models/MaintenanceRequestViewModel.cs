namespace Stonehenge.Engineering.App.Models;

public sealed record MaintenanceRequestViewModel(
    int Id,
    int GunNumber,
    string GunName,
    string RequestType,
    string RequestKind,
    string RequestSubType,
    string RequestedBy,
    string SourceApp,
    string AssetId,
    int? PanelId,
    string Status,
    string Notes,
    DateTime RequestedAt,
    DateTime? ReviewedAt,
    string? ReviewedBy,
    string? ReviewNotes,
    string SafeRouteHint);
