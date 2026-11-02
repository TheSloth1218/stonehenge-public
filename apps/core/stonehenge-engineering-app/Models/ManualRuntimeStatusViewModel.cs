namespace Stonehenge.Engineering.App.Models;

public sealed record ManualRuntimeStatusViewModel(
    string ManualMode,
    string HmiConnection,
    string MiddlewareConnection,
    string OperatorSession,
    string CurrentInterlockState,
    string LastManualEvent,
    DateTime LastUpdated);
