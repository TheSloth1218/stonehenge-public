using System.Net.Http.Json;
using System.Text.Json.Serialization;
using Microsoft.Extensions.Options;
using Stonehenge.Api.Models.Astro;
using Stonehenge.Api.Options;

namespace Stonehenge.Api.Services;

public sealed class LiveMiddlewareAstroDataProvider : IAstroDataProvider
{
    private readonly HttpClient _httpClient;
    private readonly IAstroDataProvider _fallback;
    private readonly IOptionsMonitor<StonehengeOptions> _options;
    private readonly ILogger<LiveMiddlewareAstroDataProvider> _logger;

    public LiveMiddlewareAstroDataProvider(
        HttpClient httpClient,
        SeededAstroDataProvider fallback,
        IOptionsMonitor<StonehengeOptions> options,
        ILogger<LiveMiddlewareAstroDataProvider> logger)
    {
        _httpClient = httpClient;
        _fallback = fallback;
        _options = options;
        _logger = logger;
    }

    public async Task<IReadOnlyList<AstroTargetHistoryDto>> GetTargetHistoryAsync(AstroQuery query, CancellationToken cancellationToken)
    {
        if (!_options.CurrentValue.UseLiveMiddlewareAstroData)
        {
            return await _fallback.GetTargetHistoryAsync(query, cancellationToken);
        }

        return await TryLive<IReadOnlyList<AstroTargetHistoryDto>>(
            async () =>
            {
                var fragments = await GetFragmentsAsync(query, cancellationToken);
                return fragments
                    .Where(item => !query.GunId.HasValue || item.AssignedGunId == query.GunId.Value)
                    .Select(item => new AstroTargetHistoryDto(
                        TargetId: TargetId(item),
                        GunId: item.AssignedGunId ?? 0,
                        MapX: item.MapX,
                        MapY: item.MapY,
                        ObservedAt: item.ObservedAt,
                        Result: ResultFromFragment(item.Status),
                        Summary: TargetSummary(item),
                        Source: "middleware-history"))
                    .ToList();
            },
            () => _fallback.GetTargetHistoryAsync(query, cancellationToken),
            "target history");
    }

    public async Task<IReadOnlyList<AstroFiringResultDto>> GetFiringResultsAsync(AstroQuery query, bool includeSummary, CancellationToken cancellationToken)
    {
        if (!_options.CurrentValue.UseLiveMiddlewareAstroData)
        {
            return await _fallback.GetFiringResultsAsync(query, includeSummary, cancellationToken);
        }

        return await TryLive<IReadOnlyList<AstroFiringResultDto>>(
            async () =>
            {
                var shots = await GetShotsAsync(query, cancellationToken);
                var fragments = await GetFragmentsAsync(JoinQuery(), cancellationToken);
                var fragmentsById = fragments
                    .Where(item => !string.IsNullOrWhiteSpace(item.Id))
                    .GroupBy(item => item.Id, StringComparer.OrdinalIgnoreCase)
                    .ToDictionary(group => group.Key, group => group.First(), StringComparer.OrdinalIgnoreCase);
                var fragmentsByExternalId = fragments
                    .Where(item => !string.IsNullOrWhiteSpace(item.ExternalId))
                    .GroupBy(item => item.ExternalId!, StringComparer.OrdinalIgnoreCase)
                    .ToDictionary(group => group.Key, group => group.First(), StringComparer.OrdinalIgnoreCase);

                return shots
                    .Where(item => !query.GunId.HasValue || item.GunId == query.GunId.Value)
                    .Select(item =>
                    {
                        var fragment = MatchFragment(item, fragmentsById, fragmentsByExternalId);
                        var result = fragment is not null ? ResultFromFragment(fragment.Status) : ResultFromShot(item.Result);
                        return new AstroFiringResultDto(
                            TargetId: TargetId(item, fragment),
                            GunId: item.GunId,
                            MapX: fragment?.MapX ?? 0,
                            MapY: fragment?.MapY ?? 0,
                            FiredAt: item.FiredAt,
                            Result: result,
                            Summary: FiringSummary(item, fragment, result),
                            DiagnosticSummary: includeSummary ? FiringDiagnostics(item, fragment) : null,
                            Source: "middleware-history");
                    })
                    .ToList();
            },
            () => _fallback.GetFiringResultsAsync(query, includeSummary, cancellationToken),
            "firing results");
    }

    public async Task<AstroShotSummaryDto?> GetShotSummaryAsync(int gunId, CancellationToken cancellationToken)
    {
        if (!_options.CurrentValue.UseLiveMiddlewareAstroData)
        {
            return await _fallback.GetShotSummaryAsync(gunId, cancellationToken);
        }

        return await TryLive<AstroShotSummaryDto?>(
            async () =>
            {
                var summary = await GetGunRuntimeSummaryAsync(gunId, cancellationToken);
                if (summary is null)
                {
                    return null;
                }

                var shots = await GetShotsAsync(new AstroQuery(null, null, null, gunId, 500, 0), cancellationToken);
                var gunShots = shots.Where(item => item.GunId == gunId).ToList();
                var lastShot = gunShots.OrderByDescending(item => item.FiredAt).FirstOrDefault();
                return new AstroShotSummaryDto(
                    GunId: gunId,
                    TotalShots: (int)Math.Min(summary.TotalShots, int.MaxValue),
                    SuccessCount: gunShots.Count(item => IsSuccessfulShot(item.Result)),
                    FailedCount: gunShots.Count(item => item.Result.Equals("Failed", StringComparison.OrdinalIgnoreCase)),
                    MissedCount: gunShots.Count(item => item.Result.Equals("Missed", StringComparison.OrdinalIgnoreCase)),
                    AbortedCount: gunShots.Count(item => item.Result.Equals("Aborted", StringComparison.OrdinalIgnoreCase)),
                    LastResult: lastShot is null ? "Unknown" : ResultFromShot(lastShot.Result),
                    LastFiredAt: summary.LastFiredAt ?? lastShot?.FiredAt);
            },
            () => _fallback.GetShotSummaryAsync(gunId, cancellationToken),
            $"gun {gunId} shot summary");
    }

    public async Task<IReadOnlyList<AstroTimelineEventDto>> GetTimelineAsync(AstroQuery query, CancellationToken cancellationToken)
    {
        if (!_options.CurrentValue.UseLiveMiddlewareAstroData)
        {
            return await _fallback.GetTimelineAsync(query, cancellationToken);
        }

        return await TryLive<IReadOnlyList<AstroTimelineEventDto>>(
            async () =>
            {
                var fragments = await GetFragmentsAsync(query with { Offset = 0, Limit = 500 }, cancellationToken);
                var shots = await GetShotsAsync(query with { Offset = 0, Limit = 500 }, cancellationToken);
                var joinFragments = await GetFragmentsAsync(JoinQuery(), cancellationToken);
                var fragmentsById = fragments
                    .Concat(joinFragments)
                    .Where(item => !string.IsNullOrWhiteSpace(item.Id))
                    .GroupBy(item => item.Id, StringComparer.OrdinalIgnoreCase)
                    .ToDictionary(group => group.Key, group => group.First(), StringComparer.OrdinalIgnoreCase);
                var fragmentsByExternalId = fragments
                    .Concat(joinFragments)
                    .Where(item => !string.IsNullOrWhiteSpace(item.ExternalId))
                    .GroupBy(item => item.ExternalId!, StringComparer.OrdinalIgnoreCase)
                    .ToDictionary(group => group.Key, group => group.First(), StringComparer.OrdinalIgnoreCase);

                var events = new List<AstroTimelineEventDto>();
                foreach (var fragment in fragments)
                {
                    if (query.GunId.HasValue && fragment.AssignedGunId != query.GunId.Value)
                    {
                        continue;
                    }

                    var targetId = TargetId(fragment);
                    events.Add(new AstroTimelineEventDto(
                        EventId: $"{fragment.Id}:observed",
                        EventType: "target_observed",
                        TargetId: targetId,
                        GunId: fragment.AssignedGunId,
                        MapX: fragment.MapX,
                        MapY: fragment.MapY,
                        Timestamp: fragment.ObservedAt,
                        Result: ResultFromFragment(fragment.Status),
                        Summary: TargetSummary(fragment),
                        Source: "middleware-history"));

                    if (fragment.AssignedGunId.HasValue)
                    {
                        events.Add(new AstroTimelineEventDto(
                            EventId: $"{fragment.Id}:assigned",
                            EventType: "target_assigned",
                            TargetId: targetId,
                            GunId: fragment.AssignedGunId,
                            MapX: fragment.MapX,
                            MapY: fragment.MapY,
                            Timestamp: fragment.ReceivedAt,
                            Result: "Assigned",
                            Summary: $"Gun {fragment.AssignedGunId.Value} assigned {targetId}.",
                            Source: "middleware-history"));
                    }

                    if (fragment.InterceptedAt.HasValue)
                    {
                        events.Add(new AstroTimelineEventDto(
                            EventId: $"{fragment.Id}:intercepted",
                            EventType: "target_intercepted",
                            TargetId: targetId,
                            GunId: fragment.AssignedGunId,
                            MapX: fragment.MapX,
                            MapY: fragment.MapY,
                            Timestamp: fragment.InterceptedAt.Value,
                            Result: "Intercepted",
                            Summary: fragment.AssignedGunId.HasValue
                                ? $"Gun {fragment.AssignedGunId.Value} intercepted {targetId}."
                                : $"{targetId} intercepted.",
                            Source: "middleware-history"));
                    }

                    if (fragment.MissedAt.HasValue)
                    {
                        events.Add(new AstroTimelineEventDto(
                            EventId: $"{fragment.Id}:missed",
                            EventType: "target_missed",
                            TargetId: targetId,
                            GunId: fragment.AssignedGunId,
                            MapX: fragment.MapX,
                            MapY: fragment.MapY,
                            Timestamp: fragment.MissedAt.Value,
                            Result: "Missed",
                            Summary: fragment.AssignedGunId.HasValue
                                ? $"Gun {fragment.AssignedGunId.Value} missed {targetId}."
                                : $"{targetId} missed.",
                            Source: "middleware-history"));
                    }
                }

                foreach (var shot in shots)
                {
                    if (query.GunId.HasValue && shot.GunId != query.GunId.Value)
                    {
                        continue;
                    }

                    var fragment = MatchFragment(shot, fragmentsById, fragmentsByExternalId);
                    events.Add(new AstroTimelineEventDto(
                        EventId: $"{shot.Id}:fired",
                        EventType: "gun_fired",
                        TargetId: TargetId(shot, fragment),
                        GunId: shot.GunId,
                        MapX: fragment?.MapX,
                        MapY: fragment?.MapY,
                        Timestamp: shot.FiredAt,
                        Result: ResultFromShot(shot.Result),
                        Summary: FiringSummary(shot, fragment, ResultFromShot(shot.Result)),
                        Source: "middleware-history"));
                }

                return events
                    .Where(item => (!query.EffectiveStart.HasValue || item.Timestamp >= query.EffectiveStart.Value) &&
                        (!query.End.HasValue || item.Timestamp <= query.End.Value))
                    .OrderByDescending(item => item.Timestamp)
                    .Skip(query.Offset)
                    .Take(query.Limit)
                    .ToList();
            },
            () => _fallback.GetTimelineAsync(query, cancellationToken),
            "timeline");
    }

    private static AstroQuery JoinQuery()
    {
        return new AstroQuery(null, null, null, null, 500, 0);
    }

    private async Task<IReadOnlyList<MiddlewareFragmentDto>> GetFragmentsAsync(AstroQuery query, CancellationToken cancellationToken)
    {
        var response = await _httpClient.GetFromJsonAsync<MiddlewareListResponse<MiddlewareFragmentDto>>(
            $"api/v1/history/fragments{HistoryQuery(query)}",
            cancellationToken);
        return response?.Items ?? [];
    }

    private async Task<IReadOnlyList<MiddlewareShotDto>> GetShotsAsync(AstroQuery query, CancellationToken cancellationToken)
    {
        var response = await _httpClient.GetFromJsonAsync<MiddlewareListResponse<MiddlewareShotDto>>(
            $"api/v1/history/shots{HistoryQuery(query)}",
            cancellationToken);
        return response?.Items ?? [];
    }

    private async Task<MiddlewareGunSummaryDto?> GetGunRuntimeSummaryAsync(int gunId, CancellationToken cancellationToken)
    {
        return await _httpClient.GetFromJsonAsync<MiddlewareGunSummaryDto>(
            $"api/v1/history/guns/{gunId}/summary",
            cancellationToken);
    }

    private async Task<T> TryLive<T>(Func<Task<T>> live, Func<Task<T>> fallback, string operation)
    {
        try
        {
            return await live();
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException or System.Text.Json.JsonException)
        {
            if (!_options.CurrentValue.AllowSeededAstroFallback)
            {
                _logger.LogError(ex, "Live Astro data unavailable for {Operation}", operation);
                throw;
            }

            _logger.LogWarning(ex, "Falling back to seeded Astro data for {Operation}", operation);
            return await fallback();
        }
    }

    private static string HistoryQuery(AstroQuery query)
    {
        var parts = new List<string>
        {
            $"limit={query.Limit}",
            $"offset={query.Offset}"
        };
        if (query.EffectiveStart.HasValue)
        {
            parts.Add($"start={Uri.EscapeDataString(query.EffectiveStart.Value.UtcDateTime.ToString("O"))}");
        }
        if (query.End.HasValue)
        {
            parts.Add($"end={Uri.EscapeDataString(query.End.Value.UtcDateTime.ToString("O"))}");
        }
        return "?" + string.Join("&", parts);
    }

    private static MiddlewareFragmentDto? MatchFragment(
        MiddlewareShotDto shot,
        IReadOnlyDictionary<string, MiddlewareFragmentDto> byId,
        IReadOnlyDictionary<string, MiddlewareFragmentDto> byExternalId)
    {
        if (!string.IsNullOrWhiteSpace(shot.FragmentId) && byId.TryGetValue(shot.FragmentId, out var byFragmentId))
        {
            return byFragmentId;
        }
        if (!string.IsNullOrWhiteSpace(shot.ExternalFragmentId) && byExternalId.TryGetValue(shot.ExternalFragmentId, out var byExternalIdMatch))
        {
            return byExternalIdMatch;
        }
        return null;
    }

    private static string TargetId(MiddlewareFragmentDto fragment)
    {
        return !string.IsNullOrWhiteSpace(fragment.ExternalId) ? fragment.ExternalId! : fragment.Id;
    }

    private static string TargetId(MiddlewareShotDto shot, MiddlewareFragmentDto? fragment)
    {
        if (fragment is not null)
        {
            return TargetId(fragment);
        }
        if (!string.IsNullOrWhiteSpace(shot.ExternalFragmentId))
        {
            return shot.ExternalFragmentId!;
        }
        return !string.IsNullOrWhiteSpace(shot.FragmentId) ? shot.FragmentId! : shot.Id;
    }

    private static string TargetSummary(MiddlewareFragmentDto fragment)
    {
        var targetId = TargetId(fragment);
        return fragment.AssignedGunId.HasValue
            ? $"{targetId} {ResultFromFragment(fragment.Status).ToLowerInvariant()} by Gun {fragment.AssignedGunId.Value}."
            : $"{targetId} {ResultFromFragment(fragment.Status).ToLowerInvariant()}.";
    }

    private static string FiringSummary(MiddlewareShotDto shot, MiddlewareFragmentDto? fragment, string result)
    {
        return $"Gun {shot.GunId} fired on {TargetId(shot, fragment)}: {result}.";
    }

    private static IReadOnlyDictionary<string, object?> FiringDiagnostics(MiddlewareShotDto shot, MiddlewareFragmentDto? fragment)
    {
        return new Dictionary<string, object?>(StringComparer.Ordinal)
        {
            ["targetStatus"] = fragment?.Status,
            ["targetObservedAt"] = fragment?.ObservedAt,
            ["shotSequence"] = shot.ShotSequence,
            ["shellType"] = shot.ShellType,
            ["historySource"] = "middleware-postgres"
        };
    }

    private static string ResultFromFragment(string? status)
    {
        return status?.ToUpperInvariant() switch
        {
            "INTERCEPTED" => "Intercepted",
            "MISSED" => "Missed",
            "ASSIGNED" => "Assigned",
            "ACTIVE" => "Active",
            "EXPIRED" => "Expired",
            _ => string.IsNullOrWhiteSpace(status) ? "Unknown" : ToTitle(status)
        };
    }

    private static string ResultFromShot(string? result)
    {
        return result?.ToUpperInvariant() switch
        {
            "FIRED" => "Fired",
            "SUCCESS" => "Success",
            "FAILED" => "Failed",
            "MISSED" => "Missed",
            "ABORTED" => "Aborted",
            _ => string.IsNullOrWhiteSpace(result) ? "Unknown" : ToTitle(result)
        };
    }

    private static bool IsSuccessfulShot(string? result)
    {
        return result?.Equals("Success", StringComparison.OrdinalIgnoreCase) == true ||
            result?.Equals("Intercepted", StringComparison.OrdinalIgnoreCase) == true ||
            result?.Equals("Fired", StringComparison.OrdinalIgnoreCase) == true ||
            result?.Equals("FIRED", StringComparison.OrdinalIgnoreCase) == true;
    }

    private static string ToTitle(string value)
    {
        return string.Join(' ', value.Split(['_', '-'], StringSplitOptions.RemoveEmptyEntries)
            .Select(item => string.Concat(item[..1].ToUpperInvariant(), item[1..].ToLowerInvariant())));
    }

    private sealed record MiddlewareListResponse<T>(
        [property: JsonPropertyName("items")] IReadOnlyList<T> Items);

    private sealed record MiddlewareFragmentDto(
        [property: JsonPropertyName("id")] string Id,
        [property: JsonPropertyName("external_id")] string? ExternalId,
        [property: JsonPropertyName("received_at")] DateTimeOffset ReceivedAt,
        [property: JsonPropertyName("observed_at")] DateTimeOffset ObservedAt,
        [property: JsonPropertyName("map_x")] double MapX,
        [property: JsonPropertyName("map_y")] double MapY,
        [property: JsonPropertyName("status")] string Status,
        [property: JsonPropertyName("assigned_gun_id")] int? AssignedGunId,
        [property: JsonPropertyName("intercepted_at")] DateTimeOffset? InterceptedAt,
        [property: JsonPropertyName("missed_at")] DateTimeOffset? MissedAt);

    private sealed record MiddlewareShotDto(
        [property: JsonPropertyName("id")] string Id,
        [property: JsonPropertyName("gun_id")] int GunId,
        [property: JsonPropertyName("fragment_id")] string? FragmentId,
        [property: JsonPropertyName("external_fragment_id")] string? ExternalFragmentId,
        [property: JsonPropertyName("fired_at")] DateTimeOffset FiredAt,
        [property: JsonPropertyName("shell_type")] string ShellType,
        [property: JsonPropertyName("shot_sequence")] long ShotSequence,
        [property: JsonPropertyName("result")] string Result);

    private sealed record MiddlewareGunSummaryDto(
        [property: JsonPropertyName("gun_id")] int GunId,
        [property: JsonPropertyName("total_shots")] long TotalShots,
        [property: JsonPropertyName("last_fired_at")] DateTimeOffset? LastFiredAt);
}
