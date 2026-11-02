namespace Stonehenge.Engineering.App.Models;

public sealed record FireControlViewModel(
    bool SafetyKeyUnlocked,
    bool SafetyCoverOpen,
    bool FireButtonEnabled,
    string SafetyKeyState,
    string SafetyCoverState,
    string FireCommandState,
    string LastCommand,
    string LastShotSequence,
    string AuthorizationState,
    string InterlockState);
