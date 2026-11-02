namespace Stonehenge.Api.Models;

public sealed record PublicStatusResponse(
    string Status,
    string Service,
    string ApiSurface,
    IReadOnlyList<string> PublicClients,
    DateTimeOffset TimestampUtc);
