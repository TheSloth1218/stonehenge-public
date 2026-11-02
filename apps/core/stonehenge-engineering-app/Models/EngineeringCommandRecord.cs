namespace Stonehenge.Engineering.App.Models;

public sealed record EngineeringCommandRecord(
    int Id,
    int GunNumber,
    int SourcePanelId,
    string SubmittedBy,
    string CommandType,
    string Status,
    DateTime CreatedUtc,
    string SanitizedPayload,
    int? RelatedMaintenanceRequestId);

public sealed record EngineeringPanelRecord(
    int PanelId,
    string PanelName,
    string PanelType,
    int? GunId,
    string RequiredAccess,
    string FuxaViewId,
    bool Enabled)
{
    public string Name => PanelName;

    public int? GunNumber => GunId;
}
