namespace Stonehenge.Engineering.App.Security;

public sealed class TrainingSecurityOptions
{
    public const string SectionName = "TrainingVulnerabilities";

    public bool Enabled { get; set; }
    public int SessionLifetimeMinutes { get; set; } = 15;
    public int PanelTokenLifetimeSeconds { get; set; } = 90;

    public TimeSpan SessionLifetime => TimeSpan.FromMinutes(Math.Clamp(SessionLifetimeMinutes, 1, 30));
    public TimeSpan PanelTokenLifetime => TimeSpan.FromSeconds(Math.Clamp(PanelTokenLifetimeSeconds, 30, 300));
}
