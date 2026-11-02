namespace Stonehenge.Engineering.App.Models;

public sealed record AutoRuntimeStatusViewModel(
    string AutoMode,
    string AutoState,
    string HmiConnection,
    string MiddlewareConnection,
    string LastSequence,
    string LastAutoEvent,
    DateTime LastUpdated,
    bool OperatorResetAvailable);
