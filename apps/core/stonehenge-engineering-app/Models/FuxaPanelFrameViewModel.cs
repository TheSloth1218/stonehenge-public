namespace Stonehenge.Engineering.App.Models;

public sealed class FuxaPanelFrameViewModel
{
    public int PanelId { get; init; }

    public string PanelName { get; init; } = "Unknown Panel";

    public string PanelType { get; init; } = "Unknown";

    public int? GunId { get; init; }

    public string RequiredAccess { get; init; } = string.Empty;

    public string FuxaViewId { get; init; } = string.Empty;

    public string? IframeUrl { get; init; }

    public bool Enabled { get; init; }

    public int Height { get; init; }

    public string Title { get; init; } = "FUXA Panel";

    public string DisabledReason { get; init; } = "FUXA panel is unavailable.";
}
