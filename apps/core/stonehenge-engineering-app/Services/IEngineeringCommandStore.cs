using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IEngineeringCommandStore
{
    EngineeringCommandRecord? Get(int commandId);

    IReadOnlyList<EngineeringCommandRecord> List(int limit = 50);
}
