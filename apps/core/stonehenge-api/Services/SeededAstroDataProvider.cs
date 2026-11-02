using Stonehenge.Api.Models.Astro;

namespace Stonehenge.Api.Services;

public sealed class SeededAstroDataProvider : IAstroDataProvider
{
    private readonly IStonehengeSimulationStore _store;

    public SeededAstroDataProvider(IStonehengeSimulationStore store)
    {
        _store = store;
    }

    public Task<IReadOnlyList<AstroTargetHistoryDto>> GetTargetHistoryAsync(AstroQuery query, CancellationToken cancellationToken)
    {
        var items = _store.GetTargetHistory()
            .Where(item => MatchesTargetQuery(item, query))
            .OrderByDescending(item => item.ObservedAt)
            .Skip(query.Offset)
            .Take(query.Limit)
            .Select(item => item with { Source = item.Source ?? "seeded-api-memory" })
            .ToList();
        return Task.FromResult<IReadOnlyList<AstroTargetHistoryDto>>(items);
    }

    public Task<IReadOnlyList<AstroFiringResultDto>> GetFiringResultsAsync(AstroQuery query, bool includeSummary, CancellationToken cancellationToken)
    {
        var items = _store.GetFiringResults(includeSummary)
            .Where(item => MatchesFiringQuery(item, query))
            .OrderByDescending(item => item.FiredAt)
            .Skip(query.Offset)
            .Take(query.Limit)
            .Select(item => item with { Source = item.Source ?? "seeded-api-memory" })
            .ToList();
        return Task.FromResult<IReadOnlyList<AstroFiringResultDto>>(items);
    }

    public Task<AstroShotSummaryDto?> GetShotSummaryAsync(int gunId, CancellationToken cancellationToken)
    {
        return Task.FromResult(_store.GetShotSummary(gunId));
    }

    public async Task<IReadOnlyList<AstroTimelineEventDto>> GetTimelineAsync(AstroQuery query, CancellationToken cancellationToken)
    {
        var targets = await GetTargetHistoryAsync(query with { Offset = 0, Limit = 500 }, cancellationToken);
        var shots = await GetFiringResultsAsync(query with { Offset = 0, Limit = 500 }, includeSummary: false, cancellationToken);
        var events = new List<AstroTimelineEventDto>();

        events.AddRange(targets.Select(item => new AstroTimelineEventDto(
            EventId: $"{item.TargetId}:observed",
            EventType: "target_observed",
            TargetId: item.TargetId,
            GunId: item.GunId == 0 ? null : item.GunId,
            MapX: item.MapX,
            MapY: item.MapY,
            Timestamp: item.ObservedAt,
            Result: item.Result,
            Summary: item.Summary,
            Source: item.Source ?? "seeded-api-memory")));

        events.AddRange(shots.Select(item => new AstroTimelineEventDto(
            EventId: $"{item.TargetId}:fired:{item.FiredAt:O}",
            EventType: "gun_fired",
            TargetId: item.TargetId,
            GunId: item.GunId,
            MapX: item.MapX,
            MapY: item.MapY,
            Timestamp: item.FiredAt,
            Result: item.Result,
            Summary: item.Summary,
            Source: item.Source ?? "seeded-api-memory")));

        return events
            .Where(item => MatchesTimelineQuery(item, query))
            .OrderByDescending(item => item.Timestamp)
            .Skip(query.Offset)
            .Take(query.Limit)
            .ToList();
    }

    private static bool MatchesTargetQuery(AstroTargetHistoryDto item, AstroQuery query)
    {
        return MatchesTime(item.ObservedAt, query) && (!query.GunId.HasValue || item.GunId == query.GunId.Value);
    }

    private static bool MatchesFiringQuery(AstroFiringResultDto item, AstroQuery query)
    {
        return MatchesTime(item.FiredAt, query) && (!query.GunId.HasValue || item.GunId == query.GunId.Value);
    }

    private static bool MatchesTimelineQuery(AstroTimelineEventDto item, AstroQuery query)
    {
        return MatchesTime(item.Timestamp, query) && (!query.GunId.HasValue || item.GunId == query.GunId.Value);
    }

    private static bool MatchesTime(DateTimeOffset timestamp, AstroQuery query)
    {
        return (!query.EffectiveStart.HasValue || timestamp >= query.EffectiveStart.Value) &&
            (!query.End.HasValue || timestamp <= query.End.Value);
    }
}
