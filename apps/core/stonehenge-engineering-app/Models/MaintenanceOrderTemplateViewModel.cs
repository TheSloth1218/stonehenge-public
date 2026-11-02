namespace Stonehenge.Engineering.App.Models;

public sealed record MaintenanceOrderTemplateViewModel(
    string OrderId,
    string Priority,
    string RequestedBy,
    string TargetVendor,
    string RequestedWindow,
    string Summary,
    string Status);
