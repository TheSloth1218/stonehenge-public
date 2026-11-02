namespace Stonehenge.Engineering.App.Models;

public sealed class ManualControlViewModel
{
    public List<GunStatusViewModel> Guns { get; init; } = [];

    public int SelectedGunNumber { get; init; }

    public required GunStatusViewModel SelectedGun { get; init; }

    public required ManualRuntimeStatusViewModel RuntimeStatus { get; init; }

    public required FireControlViewModel FireControl { get; init; }

    public List<OperationLogEntry> FilteredGunLog { get; init; } = [];

    public FuxaPanelFrameViewModel? FuxaPanel { get; set; }
}
