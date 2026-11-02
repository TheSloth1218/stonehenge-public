namespace Stonehenge.Engineering.App.Models;

public sealed record OperationLogEntry(
    int Id,
    DateTime Timestamp,
    string Source,
    string Action,
    string Result,
    string Notes);
