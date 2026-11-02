using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IEngineeringPanelCatalog
{
    IReadOnlyList<EngineeringPanelRecord> List();

    EngineeringPanelRecord? Get(int panelId);

    IReadOnlyList<EngineeringPanelRecord> ForGun(int gunId);
}
