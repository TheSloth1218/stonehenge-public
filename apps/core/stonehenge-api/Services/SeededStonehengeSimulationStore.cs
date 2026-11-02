using Stonehenge.Api.Auth;
using Stonehenge.Api.Models;
using Stonehenge.Api.Models.Astro;
using Stonehenge.Api.Models.Engineering;
using Stonehenge.Api.Models.Vendor;
using Stonehenge.Api.Options;
using Microsoft.Extensions.Options;

namespace Stonehenge.Api.Services;

public sealed class SeededStonehengeSimulationStore : IStonehengeSimulationStore
{
    private readonly object _gate = new();
    private readonly List<PartRequestSeed> _partRequests = [];
    private readonly List<GunStatusSeed> _gunStatuses = [];
    private readonly List<AstroEventSeed> _astroEvents = [];
    private readonly List<VendorCompletionSeed> _completions = [];
    private readonly int _maintenanceShotThreshold;
    private int _nextPracticeRequestNumber;

    public SeededStonehengeSimulationStore(IOptions<OperationalOptions> operational)
    {
        _maintenanceShotThreshold = operational.Value.MaintenanceShotThreshold;
        ResetSeededState();
    }

    public IReadOnlyList<VendorPartRequestDefaultDto> GetVendorPartRequests(bool includeWorkflow, bool includeIntegration, bool includeSeededData)
    {
        lock (_gate)
        {
            return _partRequests
                .Where(request => request.VendorVisible && (includeSeededData || !request.IsSeeded))
                .Select(request => ToPartRequestDto(request, includeWorkflow, includeIntegration))
                .ToList();
        }
    }

    public VendorPartRequestDefaultDto? GetVendorPartRequest(string requestId, bool includeWorkflow, bool includeIntegration)
    {
        lock (_gate)
        {
            var request = _partRequests.FirstOrDefault(item => item.RequestId.Equals(requestId, StringComparison.OrdinalIgnoreCase));
            return request is not null && request.VendorVisible
                ? ToPartRequestDto(request, includeWorkflow, includeIntegration)
                : null;
        }
    }

    public VendorPartRequestDefaultDto? UpsertEngineeringPartRequest(EngineeringPartRequestExportRequest request, out bool capacityReached)
    {
        lock (_gate)
        {
            var existing = _partRequests.FirstOrDefault(item => item.RequestId.Equals(request.RequestId, StringComparison.OrdinalIgnoreCase));
            capacityReached = existing is null && _partRequests.Count >= WorkflowInputLimits.MaxPartRequests;
            if (capacityReached)
            {
                return null;
            }

            if (existing is null)
            {
                existing = new PartRequestSeed(
                    request.RequestId,
                    request.SourceObjectId,
                    request.GunId,
                    NormalizeApprovedStatus(request.Status),
                    request.PartStatus,
                    request.Priority,
                    request.ApprovedAt,
                    request.UpdatedAt,
                    acknowledged: false,
                    acknowledgedAt: null,
                    vendorVisible: true,
                    isSeeded: false);
                _partRequests.Insert(0, existing);
            }
            else
            {
                existing.SourceObjectId = request.SourceObjectId;
                existing.GunId = request.GunId;
                existing.Status = NormalizeApprovedStatus(request.Status);
                existing.PartStatus = request.PartStatus;
                existing.Priority = request.Priority;
                existing.ApprovedAt = request.ApprovedAt;
                existing.UpdatedAt = request.UpdatedAt;
                existing.VendorVisible = true;
            }

            return ToPartRequestDto(existing, includeWorkflow: true, includeIntegration: false);
        }
    }

    public EngineeringPartRequestWithdrawResponse? WithdrawEngineeringPartRequest(string requestId)
    {
        lock (_gate)
        {
            var existing = _partRequests.FirstOrDefault(item => item.RequestId.Equals(requestId, StringComparison.OrdinalIgnoreCase));
            if (existing is null)
            {
                return null;
            }

            var updatedAt = DateTimeOffset.UtcNow;
            existing.Status = "Withdrawn";
            existing.VendorVisible = false;
            existing.UpdatedAt = updatedAt;
            return new EngineeringPartRequestWithdrawResponse(existing.RequestId, existing.Status, existing.VendorVisible, updatedAt);
        }
    }

    public IReadOnlyList<VendorGunStatusDto> GetVendorGunStatuses(bool includeDiagnostics)
    {
        lock (_gate)
        {
            return _gunStatuses.Select(status => ToGunStatusDto(status, includeDiagnostics)).ToList();
        }
    }

    public VendorAcknowledgeResponse? AcknowledgePartRequest(string requestId)
    {
        lock (_gate)
        {
            var request = _partRequests.FirstOrDefault(item => item.RequestId.Equals(requestId, StringComparison.OrdinalIgnoreCase));
            if (request is null || !request.VendorVisible)
            {
                return null;
            }

            var firstAcknowledgement = !request.Acknowledged;
            var acknowledgedAt = DateTimeOffset.UtcNow;
            request.Acknowledged = true;
            request.AcknowledgedAt = acknowledgedAt;
            request.UpdatedAt = acknowledgedAt;
            if (firstAcknowledgement && IsRenewablePracticeRequest(request))
            {
                AddPracticeRequest(acknowledgedAt);
            }
            return new VendorAcknowledgeResponse(request.RequestId, "Acknowledged", true, acknowledgedAt);
        }
    }

    public VendorCompletionDto? CompletePartRequest(string requestId, VendorCompleteRequestDto request, bool allowReplay, out bool replayConflict, out bool capacityReached)
    {
        lock (_gate)
        {
            replayConflict = false;
            capacityReached = false;
            var partRequest = _partRequests.FirstOrDefault(item => item.RequestId.Equals(requestId, StringComparison.OrdinalIgnoreCase));
            if (partRequest is null || !partRequest.VendorVisible)
            {
                return null;
            }

            var existing = _completions.FirstOrDefault(item => item.RequestId.Equals(partRequest.RequestId, StringComparison.OrdinalIgnoreCase) && !item.AppliedByEngineering);
            if (existing is not null && !allowReplay)
            {
                replayConflict = true;
                return ToCompletionDto(existing);
            }

            if (_completions.Count >= WorkflowInputLimits.MaxCompletions)
            {
                capacityReached = true;
                return null;
            }

            var completedAt = DateTimeOffset.UtcNow;
            var completion = new VendorCompletionSeed(
                completionId: $"VC-{completedAt:yyyyMMddHHmmssfff}-{_completions.Count + 1:D3}",
                requestId: partRequest.RequestId,
                gunId: partRequest.GunId,
                completedAt: completedAt,
                vendorState: "CompletedByVendor",
                appliedByEngineering: false,
                appliedAt: null,
                completionNote: request.CompletionNote,
                trackingReference: request.TrackingReference);

            _completions.Insert(0, completion);
            partRequest.UpdatedAt = completedAt;
            return ToCompletionDto(completion);
        }
    }

    public IReadOnlyList<VendorCompletionDto> GetVendorCompletions()
    {
        lock (_gate)
        {
            return _completions.Select(ToCompletionDto).ToList();
        }
    }

    public IReadOnlyList<VendorCompletionDto> GetPendingVendorCompletions()
    {
        lock (_gate)
        {
            return _completions.Where(completion => !completion.AppliedByEngineering).Select(ToCompletionDto).ToList();
        }
    }

    public VendorCompletionDto? MarkCompletionApplied(string completionId, out DateTimeOffset appliedAt)
    {
        lock (_gate)
        {
            var completion = _completions.FirstOrDefault(item => item.CompletionId.Equals(completionId, StringComparison.OrdinalIgnoreCase));
            if (completion is null)
            {
                appliedAt = default;
                return null;
            }

            appliedAt = DateTimeOffset.UtcNow;
            completion.AppliedByEngineering = true;
            completion.AppliedAt = appliedAt;
            completion.VendorState = "AppliedByEngineering";
            return ToCompletionDto(completion);
        }
    }

    public IReadOnlyList<AstroTargetHistoryDto> GetTargetHistory()
    {
        lock (_gate)
        {
            return _astroEvents
                .Select(item => new AstroTargetHistoryDto(item.TargetId, item.GunId, item.MapX, item.MapY, item.Timestamp, item.Result, item.Summary))
                .ToList();
        }
    }

    public IReadOnlyList<AstroFiringResultDto> GetFiringResults(bool includeSummary)
    {
        lock (_gate)
        {
            return _astroEvents
                .Select(item => new AstroFiringResultDto(
                    item.TargetId,
                    item.GunId,
                    item.MapX,
                    item.MapY,
                    item.Timestamp,
                    item.Result,
                    item.Summary,
                    includeSummary ? AstroDiagnostics(item) : null))
                .ToList();
        }
    }

    public AstroShotSummaryDto? GetShotSummary(int gunId)
    {
        lock (_gate)
        {
            var events = _astroEvents.Where(item => item.GunId == gunId).ToList();
            if (events.Count == 0)
            {
                return null;
            }

            return new AstroShotSummaryDto(
                GunId: gunId,
                TotalShots: events.Count,
                SuccessCount: events.Count(item => item.Result == "Success"),
                FailedCount: events.Count(item => item.Result == "Failed"),
                MissedCount: events.Count(item => item.Result == "Missed"),
                AbortedCount: events.Count(item => item.Result == "Aborted"),
                LastResult: events.OrderByDescending(item => item.Timestamp).First().Result,
                LastFiredAt: events.Max(item => item.Timestamp));
        }
    }

    public DateTimeOffset ResetSeededState()
    {
        lock (_gate)
        {
            var now = DateTimeOffset.UtcNow;
            _nextPracticeRequestNumber = 1;
            _partRequests.Clear();
            _partRequests.AddRange(
            [
                new("PR-2001", 2001, 1, "ApprovedForVendor", "PartsRequired", "High", now.AddHours(-9), now.AddHours(-2), false, null, true, true),
                new("PR-2002", 2002, 2, "ApprovedForVendor", "PartsRequired", "Normal", now.AddHours(-7), now.AddHours(-1), false, null, true, true),
                new("PR-1998", 1998, 1, "Closed", "OK", "Normal", now.AddDays(-3), now.AddDays(-2), true, now.AddDays(-2), false, true)
            ]);

            _gunStatuses.Clear();
            _gunStatuses.AddRange(
            [
                new(1, "Auto", 2, "PartsRequired", "PR-2001"),
                new(2, "Manual", 1, "OK", "PR-2002"),
                new(3, "Auto", 0, "OK", null),
                new(4, "Maintenance", 8, "ServiceNeeded", null),
                new(5, "Auto", 5, "OK", null),
                new(6, "Auto", 4, "OK", null),
                new(7, "Manual", 3, "Watch", null),
                new(8, "Auto", 6, "OK", null)
            ]);

            _astroEvents.Clear();
            _astroEvents.AddRange(
            [
                new("TGT-7001", 1, 41.8, -89.2, now.AddMinutes(-96), "Success", "Auto track resolved", "Auto", "0-10", "PartsRequired"),
                new("TGT-7002", 1, 44.1, -91.6, now.AddMinutes(-84), "Missed", "Wind correction drift", "Auto", "0-10", "PartsRequired"),
                new("TGT-7003", 2, 48.5, -87.4, now.AddMinutes(-72), "Success", "Manual coordinate staging", "Manual", "0-10", "OK"),
                new("TGT-7004", 3, 52.2, -85.1, now.AddMinutes(-60), "Aborted", "Operator cancelled before fire", "Auto", "0-10", "OK"),
                new("TGT-7005", 4, 58.3, -82.9, now.AddMinutes(-48), "Failed", "Maintenance gate blocked shot", "Maintenance", "0-10", "ServiceNeeded"),
                new("TGT-7006", 5, 61.9, -80.2, now.AddMinutes(-36), "Success", "Wide-sector map point", "Auto", "0-10", "OK"),
                new("TGT-7007", 6, 66.4, -77.8, now.AddMinutes(-24), "Unknown", "Sanitized imported observation", "Auto", "0-10", "OK"),
                new("TGT-7008", 7, 70.8, -74.5, now.AddMinutes(-18), "Missed", "Manual handoff plotted", "Manual", "0-10", "Watch"),
                new("TGT-7009", 8, 75.0, -71.3, now.AddMinutes(-11), "Success", "Fleet-edge plotting point", "Auto", "0-10", "OK")
            ]);

            _completions.Clear();
            return now;
        }
    }

    private static bool IsRenewablePracticeRequest(PartRequestSeed request) =>
        request.IsSeeded &&
        request.VendorVisible &&
        request.Status == "ApprovedForVendor" &&
        request.PartStatus == "PartsRequired";

    private void AddPracticeRequest(DateTimeOffset now)
    {
        if (_partRequests.Count >= WorkflowInputLimits.MaxPartRequests)
        {
            return;
        }

        int number;
        string requestId;
        do
        {
            number = _nextPracticeRequestNumber++;
            requestId = $"PR-PRACTICE-{number:D4}";
        }
        while (_partRequests.Any(request => request.RequestId.Equals(requestId, StringComparison.OrdinalIgnoreCase)));

        _partRequests.Insert(0, new PartRequestSeed(
            requestId,
            sourceObjectId: 900_000 + number,
            gunId: ((number - 1) % 8) + 1,
            status: "ApprovedForVendor",
            partStatus: "PartsRequired",
            priority: "Normal",
            approvedAt: now,
            updatedAt: now,
            acknowledged: false,
            acknowledgedAt: null,
            vendorVisible: true,
            isSeeded: true));
    }

    private static VendorPartRequestDefaultDto ToPartRequestDto(PartRequestSeed request, bool includeWorkflow, bool includeIntegration)
    {
        return new VendorPartRequestDefaultDto(
            request.RequestId,
            request.GunId,
            request.Status,
            request.PartStatus,
            request.Priority,
            request.ApprovedAt,
            request.UpdatedAt,
            request.Acknowledged,
            includeWorkflow ? WorkflowLeak(request) : null,
            includeIntegration ? IntegrationLeak() : null);
    }

    private VendorGunStatusDto ToGunStatusDto(GunStatusSeed status, bool includeDiagnostics)
    {
        return new VendorGunStatusDto(
            status.GunId,
            status.Mode,
            status.ShotCount,
            status.PartStatus,
            status.CurrentApprovedPartRequestId,
            includeDiagnostics ? GunDiagnosticLeak() : null);
    }

    private static VendorCompletionDto ToCompletionDto(VendorCompletionSeed completion)
    {
        return new VendorCompletionDto(
            completion.CompletionId,
            completion.RequestId,
            completion.GunId,
            completion.CompletedAt,
            completion.VendorState,
            completion.AppliedByEngineering,
            completion.CompletionNote,
            completion.TrackingReference);
    }

    private static Dictionary<string, object?> WorkflowLeak(PartRequestSeed request) => new(StringComparer.Ordinal)
    {
        ["sourceApplication"] = "stonehenge-engineering-app",
        ["sourceObjectType"] = "PartRequest",
        ["sourceObjectId"] = request.SourceObjectId,
        ["engineeringState"] = "Allowed",
        ["vendorState"] = request.Status == "Withdrawn" ? "Withdrawn" : "PendingVendorCompletion",
        ["completionEvent"] = "VendorPartRequestCompleted",
        ["syncModel"] = "EngineeringAppPollsVendorCompletions"
    };

    private static Dictionary<string, object?> IntegrationLeak() => new(StringComparer.Ordinal)
    {
        ["requiredScopes"] = new[]
        {
            ApiScopes.VendorPartsRead,
            ApiScopes.VendorPartsAcknowledge,
            ApiScopes.VendorPartsComplete,
            ApiScopes.EngineeringVendorCompletionsRead,
            ApiScopes.EngineeringVendorCompletionsApply
        },
        ["serviceHeaderName"] = DevApiKeyAuthorizer.HeaderName,
        ["ownershipNote"] = "Engineering app owns local workflow state. This API stores vendor completion events for later polling."
    };

    private Dictionary<string, object?> GunDiagnosticLeak() => new(StringComparer.Ordinal)
    {
        ["statusProvider"] = "engineering-snapshot",
        ["runtimeProvider"] = "middleware-summary",
        ["lastEngineeringSync"] = DateTimeOffset.UtcNow.AddMinutes(-4),
        ["lastRuntimeSummary"] = DateTimeOffset.UtcNow.AddMinutes(-1),
        ["engineeringPartServiceThresholdShots"] = _maintenanceShotThreshold,
        ["counterResetTrigger"] = "VendorPartRequestCompleted",
        ["localEffect"] = "EngineeringAppResetsMaintenanceShotCounter"
    };

    private static Dictionary<string, object?> AstroDiagnostics(AstroEventSeed item) => new(StringComparer.Ordinal)
    {
        ["gunModeAtShot"] = item.GunModeAtShot,
        ["shotCountBucket"] = item.ShotCountBucket,
        ["publicPartStatusSummary"] = item.PublicPartStatusSummary,
        ["targetSource"] = "training-feed",
        ["historianSummarySource"] = "sanitized-snapshot"
    };

    private sealed record GunStatusSeed(
        int GunId,
        string Mode,
        int ShotCount,
        string PartStatus,
        string? CurrentApprovedPartRequestId);

    private sealed record AstroEventSeed(
        string TargetId,
        int GunId,
        double MapX,
        double MapY,
        DateTimeOffset Timestamp,
        string Result,
        string Summary,
        string GunModeAtShot,
        string ShotCountBucket,
        string PublicPartStatusSummary);

    private sealed class PartRequestSeed
    {
        public PartRequestSeed(string requestId, int sourceObjectId, int gunId, string status, string partStatus, string priority, DateTimeOffset approvedAt, DateTimeOffset updatedAt, bool acknowledged, DateTimeOffset? acknowledgedAt, bool vendorVisible, bool isSeeded)
        {
            RequestId = requestId;
            SourceObjectId = sourceObjectId;
            GunId = gunId;
            Status = status;
            PartStatus = partStatus;
            Priority = priority;
            ApprovedAt = approvedAt;
            UpdatedAt = updatedAt;
            Acknowledged = acknowledged;
            AcknowledgedAt = acknowledgedAt;
            VendorVisible = vendorVisible;
            IsSeeded = isSeeded;
        }

        public string RequestId { get; }

        public int SourceObjectId { get; set; }

        public int GunId { get; set; }

        public string Status { get; set; }

        public string PartStatus { get; set; }

        public string Priority { get; set; }

        public DateTimeOffset ApprovedAt { get; set; }

        public DateTimeOffset UpdatedAt { get; set; }

        public bool Acknowledged { get; set; }

        public DateTimeOffset? AcknowledgedAt { get; set; }

        public bool VendorVisible { get; set; }

        public bool IsSeeded { get; }
    }

    private sealed class VendorCompletionSeed
    {
        public VendorCompletionSeed(string completionId, string requestId, int gunId, DateTimeOffset completedAt, string vendorState, bool appliedByEngineering, DateTimeOffset? appliedAt, string? completionNote, string? trackingReference)
        {
            CompletionId = completionId;
            RequestId = requestId;
            GunId = gunId;
            CompletedAt = completedAt;
            VendorState = vendorState;
            AppliedByEngineering = appliedByEngineering;
            AppliedAt = appliedAt;
            CompletionNote = completionNote;
            TrackingReference = trackingReference;
        }

        public string CompletionId { get; }

        public string RequestId { get; }

        public int GunId { get; }

        public DateTimeOffset CompletedAt { get; }

        public string VendorState { get; set; }

        public bool AppliedByEngineering { get; set; }

        public DateTimeOffset? AppliedAt { get; set; }

        public string? CompletionNote { get; }

        public string? TrackingReference { get; }
    }

    private static string NormalizeApprovedStatus(string status)
    {
        return status.Equals("Withdrawn", StringComparison.OrdinalIgnoreCase)
            ? "Withdrawn"
            : "ApprovedForVendor";
    }
}
