using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IEngineeringDashboardData
{
    Task<EngineeringOverviewViewModel> GetOverviewAsync(CancellationToken cancellationToken = default);

    Task<AutoStatusViewModel> GetAutoStatusAsync(int? gunNumber, CancellationToken cancellationToken = default);

    Task<ManualControlViewModel> GetManualControlAsync(int? gunNumber, CancellationToken cancellationToken = default);

    Task<MaintenancePanelViewModel> GetMaintenancePanelAsync(int? gunNumber, CancellationToken cancellationToken = default);
}
