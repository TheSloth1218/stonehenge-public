using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IAppEventStore
{
    IReadOnlyList<OperationLogEntry> List(string category, int limit = 20);

    OperationLogEntry Add(string category, string source, string action, string result, string notes);
}
