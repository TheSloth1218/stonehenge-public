namespace Stonehenge.Api.Models;

public sealed record ApiInfoResponse(
    string Name,
    string Purpose,
    string MiddlewareBaseUrl,
    IReadOnlyList<string> IntendedClients);
