namespace Stonehenge.Engineering.App.Models;

public sealed class EngineeringOverviewViewModel
{
    public List<GunStatusViewModel> Guns { get; init; } = [];

    public List<OperationLogEntry> OperationLog { get; init; } = [];
}
