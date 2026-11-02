namespace Stonehenge.Engineering.App.Models;

public sealed record GunPartStatusRecord(
    int GunNumber,
    int FiringCount,
    int ServiceThreshold,
    string PartStatus,
    string PartClass,
    DateTime LastUpdated);
