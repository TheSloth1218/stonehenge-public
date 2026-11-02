namespace Stonehenge.Engineering.App.Models;

public sealed class FuxaOptions
{
    public bool Enabled { get; init; }

    public string BaseUrl { get; init; } = string.Empty;

    public string InternalBaseUrl { get; init; } = "http://fuxa:1881";

    public string EmbedPathTemplate { get; init; } = "/#/view/{fuxaViewId}";

    public string LocalProxyPathTemplate { get; init; } = "/fuxa/embed/{fuxaViewId}";

    public string FrameTitlePrefix { get; init; } = "Stonehenge FUXA";

    public int DefaultPanelHeight { get; init; } = 720;

    public bool AllowDirectExternalFuxaLinks { get; init; }
}
