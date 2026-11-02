namespace Stonehenge.Engineering.App.Models;

public sealed record GunMaintenanceStatusViewModel(
    int GunNumber,
    string Name,
    string MaintenanceState,
    string MaintenanceClass,
    string CurrentMode,
    string PartStatus,
    string PartClass,
    int FiringCount,
    int ServiceThreshold,
    string WorkOrder,
    string LastInspection,
    bool AdminRequired);
