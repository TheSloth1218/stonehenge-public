using Stonehenge.Api.Models.Astro;
using Stonehenge.Api.Models.Engineering;
using Stonehenge.Api.Models.Vendor;

namespace Stonehenge.Api.Services;

public interface IStonehengeSimulationStore
{
    IReadOnlyList<VendorPartRequestDefaultDto> GetVendorPartRequests(bool includeWorkflow, bool includeIntegration, bool includeSeededData);

    VendorPartRequestDefaultDto? GetVendorPartRequest(string requestId, bool includeWorkflow, bool includeIntegration);

    VendorPartRequestDefaultDto? UpsertEngineeringPartRequest(EngineeringPartRequestExportRequest request, out bool capacityReached);

    EngineeringPartRequestWithdrawResponse? WithdrawEngineeringPartRequest(string requestId);

    IReadOnlyList<VendorGunStatusDto> GetVendorGunStatuses(bool includeDiagnostics);

    VendorAcknowledgeResponse? AcknowledgePartRequest(string requestId);

    VendorCompletionDto? CompletePartRequest(string requestId, VendorCompleteRequestDto request, bool allowReplay, out bool replayConflict, out bool capacityReached);

    IReadOnlyList<VendorCompletionDto> GetVendorCompletions();

    IReadOnlyList<VendorCompletionDto> GetPendingVendorCompletions();

    VendorCompletionDto? MarkCompletionApplied(string completionId, out DateTimeOffset appliedAt);

    IReadOnlyList<AstroTargetHistoryDto> GetTargetHistory();

    IReadOnlyList<AstroFiringResultDto> GetFiringResults(bool includeSummary);

    AstroShotSummaryDto? GetShotSummary(int gunId);

    DateTimeOffset ResetSeededState();
}
