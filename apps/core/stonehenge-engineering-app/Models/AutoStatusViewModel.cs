namespace Stonehenge.Engineering.App.Models;

public sealed class AutoStatusViewModel
{
    public List<GunStatusViewModel> Guns { get; init; } = [];

    public int SelectedGunNumber { get; init; }

    public required GunStatusViewModel SelectedGun { get; init; }

    public required AutoRuntimeStatusViewModel RuntimeStatus { get; init; }

    public List<OperationLogEntry> RecentEvents { get; init; } = [];

    public FuxaPanelFrameViewModel? FuxaPanel { get; set; }
}
