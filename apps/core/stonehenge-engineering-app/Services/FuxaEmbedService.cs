using Microsoft.Extensions.Options;
using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class FuxaEmbedService : IFuxaEmbedService
{
    private readonly IEngineeringPanelCatalog _panels;
    private readonly FuxaOptions _options;

    public FuxaEmbedService(IEngineeringPanelCatalog panels, IOptions<FuxaOptions> options)
    {
        _panels = panels;
        _options = options.Value;
    }

    public FuxaPanelFrameViewModel BuildPanelFrame(int panelId)
    {
        var panel = _panels.Get(panelId);
        if (panel is null)
        {
            return new FuxaPanelFrameViewModel
            {
                PanelId = panelId,
                PanelName = "Panel not found",
                PanelType = "Unknown",
                Enabled = false,
                Height = PanelHeight(),
                DisabledReason = "No known FUXA panel catalog entry exists for this selection."
            };
        }

        var iframeUrl = BuildIframeUrl(panel.FuxaViewId, out var disabledReason);
        var enabled = _options.Enabled && panel.Enabled && !string.IsNullOrWhiteSpace(iframeUrl);

        return new FuxaPanelFrameViewModel
        {
            PanelId = panel.PanelId,
            PanelName = panel.PanelName,
            PanelType = panel.PanelType,
            GunId = panel.GunId,
            RequiredAccess = panel.RequiredAccess,
            FuxaViewId = panel.FuxaViewId,
            IframeUrl = iframeUrl,
            Enabled = enabled,
            Height = PanelHeight(),
            Title = $"{_options.FrameTitlePrefix} - {panel.PanelName}",
            DisabledReason = enabled
                ? string.Empty
                : disabledReason ?? "FUXA embedding is disabled or this panel is unavailable."
        };
    }

    private string? BuildIframeUrl(string fuxaViewId, out string? disabledReason)
    {
        disabledReason = null;
        if (!_options.Enabled)
        {
            disabledReason = "FUXA embedding is disabled by configuration.";
            return null;
        }

        if (!IsSafeViewId(fuxaViewId))
        {
            disabledReason = "FUXA view id is not a valid catalog identifier.";
            return null;
        }

        if (!_options.AllowDirectExternalFuxaLinks)
        {
            var proxyTemplate = string.IsNullOrWhiteSpace(_options.LocalProxyPathTemplate)
                ? "/fuxa/embed/{fuxaViewId}"
                : _options.LocalProxyPathTemplate;
            var proxyPath = proxyTemplate.Replace("{fuxaViewId}", Uri.EscapeDataString(fuxaViewId), StringComparison.Ordinal);
            return proxyPath.StartsWith('/') ? proxyPath : "/" + proxyPath;
        }

        if (string.IsNullOrWhiteSpace(_options.BaseUrl))
        {
            disabledReason = "FUXA BaseUrl is not configured.";
            return null;
        }

        var pathTemplate = string.IsNullOrWhiteSpace(_options.EmbedPathTemplate)
            ? "/stonehenge/embed/{fuxaViewId}"
            : _options.EmbedPathTemplate;
        var path = pathTemplate.Replace("{fuxaViewId}", Uri.EscapeDataString(fuxaViewId), StringComparison.Ordinal);
        if (!path.StartsWith('/'))
        {
            path = "/" + path;
        }

        if (!Uri.TryCreate(_options.BaseUrl, UriKind.Absolute, out var baseUri) ||
            baseUri.Scheme is not ("http" or "https"))
        {
            disabledReason = "FUXA BaseUrl must be an absolute HTTP(S) URL.";
            return null;
        }

        return new Uri(baseUri, path).ToString();
    }

    private static bool IsSafeViewId(string fuxaViewId)
    {
        return !string.IsNullOrWhiteSpace(fuxaViewId) &&
            fuxaViewId.All(character =>
                char.IsAsciiLetterOrDigit(character) ||
                character is '-' or '_');
    }

    private int PanelHeight()
    {
        return Math.Clamp(_options.DefaultPanelHeight, 240, 1600);
    }
}
