namespace Stonehenge.Api.Models.Dev;

public sealed record SimulationResetResponse(
    string Status,
    string Scope,
    DateTimeOffset ResetAt,
    int VendorGunCount,
    int AstroEventCount);
