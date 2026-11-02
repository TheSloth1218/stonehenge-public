namespace Stonehenge.Api.Models;

public sealed record ApiHealthResponse(
    string Status,
    string Service,
    string Environment,
    DateTimeOffset TimestampUtc);
