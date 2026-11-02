using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Options;
using Stonehenge.Api.Models.Astro;
using Stonehenge.Api.Options;
using Stonehenge.Api.Services;

namespace Stonehenge.Api.Controllers;

[ApiController]
[Route("api/v1/astro")]
public sealed class AstroController : ControllerBase
{
    private readonly IAstroDataProvider _astroData;
    private readonly IOptionsMonitor<TrainingVulnerabilityOptions> _trainingOptions;
    private readonly ILogger<AstroController> _logger;

    public AstroController(IAstroDataProvider astroData, IOptionsMonitor<TrainingVulnerabilityOptions> trainingOptions, ILogger<AstroController> logger)
    {
        _astroData = astroData;
        _trainingOptions = trainingOptions;
        _logger = logger;
    }

    [HttpGet("target-history")]
    [ProducesResponseType(typeof(IReadOnlyList<AstroTargetHistoryDto>), StatusCodes.Status200OK)]
    public async Task<ActionResult<IReadOnlyList<AstroTargetHistoryDto>>> TargetHistory(
        [FromQuery(Name = "from")] DateTimeOffset? from,
        [FromQuery] DateTimeOffset? start,
        [FromQuery(Name = "to")] DateTimeOffset? to,
        [FromQuery] DateTimeOffset? end,
        [FromQuery] DateTimeOffset? since,
        [FromQuery] int? gunId,
        [FromQuery] int? limit,
        [FromQuery] int? offset,
        CancellationToken cancellationToken)
    {
        if (!TrainingEnabled) return Ok(Array.Empty<AstroTargetHistoryDto>());
        var query = Query(start ?? from, end ?? to, since, gunId, limit, offset);
        var history = await _astroData.GetTargetHistoryAsync(query, cancellationToken);
        Audit("/api/v1/astro/target-history", "FCU-INFO-004", ["targetId", "gunId"]);
        Audit("/api/v1/astro/target-history", "FCU-INFO-006", ["mapX", "mapY", "observedAt", "result", "summary", "source"]);
        return Ok(history);
    }

    [HttpGet("firing-results")]
    [ProducesResponseType(typeof(IReadOnlyList<AstroFiringResultDto>), StatusCodes.Status200OK)]
    public async Task<ActionResult<IReadOnlyList<AstroFiringResultDto>>> FiringResults(
        [FromQuery] string? fields,
        [FromQuery] string? include,
        [FromQuery(Name = "from")] DateTimeOffset? from,
        [FromQuery] DateTimeOffset? start,
        [FromQuery(Name = "to")] DateTimeOffset? to,
        [FromQuery] DateTimeOffset? end,
        [FromQuery] DateTimeOffset? since,
        [FromQuery] int? gunId,
        [FromQuery] int? limit,
        [FromQuery] int? offset,
        CancellationToken cancellationToken)
    {
        if (!TrainingEnabled) return Ok(Array.Empty<AstroFiringResultDto>());
        var includeSummary = _trainingOptions.CurrentValue.EnableAstroFieldExpansionLeak
            && (string.Equals(fields, "all", StringComparison.OrdinalIgnoreCase) || HasToken(include, "summary"));
        Audit("/api/v1/astro/firing-results", "FCU-INFO-004", ["targetId", "gunId"]);
        Audit("/api/v1/astro/firing-results", "FCU-INFO-006", ["mapX", "mapY", "firedAt", "result", "summary", "source"]);
        if (includeSummary)
        {
            Audit("/api/v1/astro/firing-results", "FCU-INFO-003", ["gunModeAtShot"]);
            Audit("/api/v1/astro/firing-results", "FCU-INFO-004", ["targetSource"]);
            Audit("/api/v1/astro/firing-results", "FCU-INFO-005", ["shotCountBucket", "publicPartStatusSummary"]);
            Audit("/api/v1/astro/firing-results", "FCU-INFO-006", ["historianSummarySource"]);
        }
        var query = Query(start ?? from, end ?? to, since, gunId, limit, offset);
        return Ok(await _astroData.GetFiringResultsAsync(query, includeSummary, cancellationToken));
    }

    [HttpGet("guns/{gunId:int}/shot-summary")]
    [ProducesResponseType(typeof(AstroShotSummaryDto), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status404NotFound)]
    public async Task<ActionResult<AstroShotSummaryDto>> ShotSummary(int gunId, CancellationToken cancellationToken)
    {
        if (!TrainingEnabled) return NotFound();
        var summary = await _astroData.GetShotSummaryAsync(gunId, cancellationToken);
        if (summary is not null)
        {
            Audit("/api/v1/astro/guns/{gunId}/shot-summary", "FCU-INFO-004", ["gunId"]);
            Audit("/api/v1/astro/guns/{gunId}/shot-summary", "FCU-INFO-005", ["totalShots", "successCount", "failedCount", "missedCount", "abortedCount"]);
            Audit("/api/v1/astro/guns/{gunId}/shot-summary", "FCU-INFO-006", ["lastResult", "lastFiredAt"]);
        }
        return summary is null ? NotFound() : Ok(summary);
    }

    [HttpGet("timeline")]
    [ProducesResponseType(typeof(IReadOnlyList<AstroTimelineEventDto>), StatusCodes.Status200OK)]
    public async Task<ActionResult<IReadOnlyList<AstroTimelineEventDto>>> Timeline(
        [FromQuery(Name = "from")] DateTimeOffset? from,
        [FromQuery] DateTimeOffset? start,
        [FromQuery(Name = "to")] DateTimeOffset? to,
        [FromQuery] DateTimeOffset? end,
        [FromQuery] DateTimeOffset? since,
        [FromQuery] int? gunId,
        [FromQuery] int? limit,
        [FromQuery] int? offset,
        CancellationToken cancellationToken)
    {
        if (!TrainingEnabled) return Ok(Array.Empty<AstroTimelineEventDto>());
        var query = Query(start ?? from, end ?? to, since, gunId, limit, offset);
        var timeline = await _astroData.GetTimelineAsync(query, cancellationToken);
        var endpoint = Request.Path.Value ?? "/api/v1/astro/timeline";
        Audit(endpoint, "FCU-INFO-004", ["targetId", "gunId"]);
        Audit(endpoint, "FCU-INFO-006", ["eventId", "eventType", "mapX", "mapY", "timestamp", "result", "summary", "source"]);
        return Ok(timeline);
    }

    [HttpGet("events")]
    [ProducesResponseType(typeof(IReadOnlyList<AstroTimelineEventDto>), StatusCodes.Status200OK)]
    public Task<ActionResult<IReadOnlyList<AstroTimelineEventDto>>> Events(
        [FromQuery(Name = "from")] DateTimeOffset? from,
        [FromQuery] DateTimeOffset? start,
        [FromQuery(Name = "to")] DateTimeOffset? to,
        [FromQuery] DateTimeOffset? end,
        [FromQuery] DateTimeOffset? since,
        [FromQuery] int? gunId,
        [FromQuery] int? limit,
        [FromQuery] int? offset,
        CancellationToken cancellationToken)
    {
        return Timeline(from, start, to, end, since, gunId, limit, offset, cancellationToken);
    }

    private static AstroQuery Query(DateTimeOffset? start, DateTimeOffset? end, DateTimeOffset? since, int? gunId, int? limit, int? offset)
    {
        return AstroQuery.Normalize(start, end, since, gunId, limit, offset);
    }

    private bool TrainingEnabled => _trainingOptions.CurrentValue.Enabled;

    private void Audit(string endpoint, string findingId, IReadOnlyList<string> fields)
    {
        _logger.LogWarning("Classified fields exposed Endpoint={Endpoint} Caller={Caller} FieldNames={FieldNames} Classification={Classification} FindingId={FindingId} CorrelationId={CorrelationId}",
            endpoint, "astro-api-client", fields, "TOP SECRET // FCU-OT", findingId, HttpContext.TraceIdentifier);
    }

    private static bool HasToken(string? value, string token)
    {
        return value?.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Any(item => item.Equals(token, StringComparison.OrdinalIgnoreCase)) == true;
    }
}
