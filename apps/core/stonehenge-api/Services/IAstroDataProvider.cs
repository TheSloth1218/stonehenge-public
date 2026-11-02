using Stonehenge.Api.Models.Astro;

namespace Stonehenge.Api.Services;

public interface IAstroDataProvider
{
    Task<IReadOnlyList<AstroTargetHistoryDto>> GetTargetHistoryAsync(AstroQuery query, CancellationToken cancellationToken);

    Task<IReadOnlyList<AstroFiringResultDto>> GetFiringResultsAsync(AstroQuery query, bool includeSummary, CancellationToken cancellationToken);

    Task<AstroShotSummaryDto?> GetShotSummaryAsync(int gunId, CancellationToken cancellationToken);

    Task<IReadOnlyList<AstroTimelineEventDto>> GetTimelineAsync(AstroQuery query, CancellationToken cancellationToken);
}
