using System.ComponentModel.DataAnnotations;
using Stonehenge.Api.Models.Vendor;

namespace Stonehenge.Api.Models.Engineering;

public sealed record EngineeringPartRequestExportRequest(
    [Required, MaxLength(WorkflowInputLimits.MaxRequestIdLength)] string RequestId,
    int SourceObjectId,
    int GunId,
    [Required, MaxLength(WorkflowInputLimits.MaxStatusLength)] string Status,
    [Required, MaxLength(WorkflowInputLimits.MaxStatusLength)] string PartStatus,
    [Required, MaxLength(WorkflowInputLimits.MaxPriorityLength)] string Priority,
    DateTimeOffset ApprovedAt,
    DateTimeOffset UpdatedAt);

public sealed record EngineeringPartRequestWithdrawResponse(
    string RequestId,
    string Status,
    bool VendorVisible,
    DateTimeOffset UpdatedAt);

public sealed record EngineeringApplyCompletionResponse(
    string CompletionId,
    bool AppliedByEngineering,
    DateTimeOffset AppliedAt);

public sealed record PendingVendorCompletionsResponse(
    IReadOnlyList<VendorCompletionDto> Completions);
