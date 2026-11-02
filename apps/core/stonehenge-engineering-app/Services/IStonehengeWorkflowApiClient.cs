using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public interface IStonehengeWorkflowApiClient
{
    Task<StonehengeApiExportResult> ExportApprovedPartRequestAsync(MaintenanceRequestViewModel request, CancellationToken cancellationToken);

    Task<IReadOnlyList<StonehengeVendorCompletion>> GetPendingVendorCompletionsAsync(CancellationToken cancellationToken);

    Task<bool> MarkVendorCompletionAppliedAsync(string completionId, CancellationToken cancellationToken);
}

public sealed record StonehengeApiExportResult(bool Success, string? Error);

public sealed record StonehengeVendorCompletion(
    string CompletionId,
    string RequestId,
    int GunId,
    DateTimeOffset CompletedAt,
    string VendorState,
    bool AppliedByEngineering,
    string? CompletionNote,
    string? TrackingReference);
