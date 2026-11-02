namespace Stonehenge.Api.Options;

public sealed class OperationalOptions
{
    public const string SectionName = "Operational";
    public int MaintenanceShotThreshold { get; init; } = 200;
}
