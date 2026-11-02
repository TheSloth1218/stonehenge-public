namespace Stonehenge.Api.Options;

public sealed class StonehengeOptions
{
    public string MiddlewareBaseUrl { get; init; } = "http://middleware:8080";

    public bool UseLiveMiddlewareAstroData { get; init; } = true;

    public bool AllowSeededAstroFallback { get; init; }
}
