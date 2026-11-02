namespace Stonehenge.Engineering.App.Models;

public sealed record GunStatusViewModel(
    int GunNumber,
    string Name,
    string Status,
    string StatusClass,
    double Azimuth,
    double Elevation,
    string SafetyState,
    DateTime LastUpdated);
