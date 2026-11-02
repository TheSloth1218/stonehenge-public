using System.ComponentModel.DataAnnotations;

namespace Stonehenge.Api.Models.Vendor;

public sealed record VendorPartRequestDefaultDto(
    string RequestId,
    int GunId,
    string Status,
    string PartStatus,
    string Priority,
    DateTimeOffset ApprovedAt,
    DateTimeOffset UpdatedAt,
    bool Acknowledged,
    IReadOnlyDictionary<string, object?>? Workflow = null,
    IReadOnlyDictionary<string, object?>? Integration = null);

public sealed record VendorGunStatusDto(
    int GunId,
    string Mode,
    int ShotCount,
    string PartStatus,
    string? CurrentApprovedPartRequestId,
    IReadOnlyDictionary<string, object?>? Diagnostics = null);

public sealed record VendorCompleteRequestDto(
    [MaxLength(WorkflowInputLimits.MaxCompletionNoteLength)] string? CompletionNote,
    [MaxLength(WorkflowInputLimits.MaxTrackingReferenceLength)] string? TrackingReference);

public sealed record VendorCompletionDto(
    string CompletionId,
    string RequestId,
    int GunId,
    DateTimeOffset CompletedAt,
    string VendorState,
    bool AppliedByEngineering,
    string? CompletionNote,
    string? TrackingReference);

public sealed record VendorAcknowledgeResponse(
    string RequestId,
    string VendorState,
    bool Acknowledged,
    DateTimeOffset AcknowledgedAt);
