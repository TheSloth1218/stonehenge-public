using Stonehenge.Engineering.App.Models;
using Stonehenge.Engineering.App.Security;
using Microsoft.Extensions.Options;

namespace Stonehenge.Engineering.App.Services;

public sealed class StaticEngineeringPanelCatalog : IEngineeringPanelCatalog
{
    public const int AutoPanelOffset = 100;
    public const int AdminPanelOffset = 300;

    private readonly IReadOnlyList<EngineeringPanelRecord> _panels;

    public StaticEngineeringPanelCatalog(IOptions<TrainingSecurityOptions> training)
    {
        var panels = Enumerable.Range(1, 8)
        .SelectMany(gunNumber => new[]
        {
            new EngineeringPanelRecord(
                PanelId: gunNumber,
                PanelName: $"Gun {gunNumber} Manual Panel",
                PanelType: "Manual",
                GunId: gunNumber,
                RequiredAccess: StonehengeAuth.EngineeringAccessPolicy,
                FuxaViewId: $"v_stonehenge_gun{gunNumber}_manual",
                Enabled: true),
            new EngineeringPanelRecord(
                PanelId: AutoPanelOffset + gunNumber,
                PanelName: $"Gun {gunNumber} Auto Panel",
                PanelType: "Auto",
                GunId: gunNumber,
                RequiredAccess: StonehengeAuth.EngineeringAccessPolicy,
                FuxaViewId: $"v_stonehenge_gun{gunNumber}_auto",
                Enabled: true),
            new EngineeringPanelRecord(
                PanelId: AdminPanelOffset + gunNumber,
                PanelName: $"Gun {gunNumber} Admin Manual Panel",
                PanelType: "Admin",
                GunId: gunNumber,
                RequiredAccess: StonehengeAuth.EngineeringAdminAccessPolicy,
                FuxaViewId: $"v_stonehenge_gun{gunNumber}_admin_manual",
                Enabled: true)
        })
        .ToList();
        panels.Add(new EngineeringPanelRecord(99, "Admin Diagnostics", "admin-diagnostics", null,
            StonehengeAuth.EngineeringAdminAccessPolicy, "admin-diagnostics", true));
        _panels = panels;
    }

    public IReadOnlyList<EngineeringPanelRecord> List()
    {
        return _panels;
    }

    public EngineeringPanelRecord? Get(int panelId)
    {
        return _panels.SingleOrDefault(panel => panel.PanelId == panelId);
    }

    public IReadOnlyList<EngineeringPanelRecord> ForGun(int gunId)
    {
        return _panels
            .Where(panel => panel.GunId == gunId)
            .ToArray();
    }
}
