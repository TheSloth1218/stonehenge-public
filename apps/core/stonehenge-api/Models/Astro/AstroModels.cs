namespace Stonehenge.Api.Models.Astro;

public sealed record AstroTargetHistoryDto(
    string TargetId,
    int GunId,
    double MapX,
    double MapY,
    DateTimeOffset ObservedAt,
    string Result,
    string? Summary = null,
    string? Source = null);

public sealed record AstroFiringResultDto(
    string TargetId,
    int GunId,
    double MapX,
    double MapY,
    DateTimeOffset FiredAt,
    string Result,
    string? Summary = null,
    IReadOnlyDictionary<string, object?>? DiagnosticSummary = null,
    string? Source = null);

public sealed record AstroShotSummaryDto(
    int GunId,
    int TotalShots,
    int SuccessCount,
    int FailedCount,
    int MissedCount,
    int AbortedCount,
    string LastResult,
    DateTimeOffset? LastFiredAt);

public sealed record AstroTimelineEventDto(
    string EventId,
    string EventType,
    string TargetId,
    int? GunId,
    double? MapX,
    double? MapY,
    DateTimeOffset Timestamp,
    string? Result = null,
    string? Summary = null,
    string? Source = null);
