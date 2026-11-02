using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IPartStatusStore
{
    IReadOnlyDictionary<int, GunPartStatusRecord> ListLatest();

    GunPartStatusRecord Upsert(
        int gunNumber,
        int firingCount,
        int serviceThreshold,
        string partStatus,
        string partClass);
}
