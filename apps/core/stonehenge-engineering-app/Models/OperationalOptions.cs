namespace Stonehenge.Engineering.App.Models;

public sealed class OperationalOptions
{
    public const string SectionName = "Operational";
    public const int DefaultMaintenanceShotThreshold = 200;
    public int MaintenanceShotThreshold { get; init; } = DefaultMaintenanceShotThreshold;
}
