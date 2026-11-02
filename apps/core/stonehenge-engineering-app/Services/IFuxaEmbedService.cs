using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IFuxaEmbedService
{
    FuxaPanelFrameViewModel BuildPanelFrame(int panelId);
}
