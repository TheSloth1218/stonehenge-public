using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Options;
using Stonehenge.Api.Auth;
using Stonehenge.Api.Models.Vendor;
using Stonehenge.Api.Options;
using Stonehenge.Api.Services;

namespace Stonehenge.Api.Controllers;

[ApiController]
[Route("api/v1/vendor")]
public sealed class VendorController : ControllerBase
{
    private readonly IApiKeyAuthorizer _authorizer;
    private readonly IStonehengeSimulationStore _store;
    private readonly IOptionsMonitor<TrainingVulnerabilityOptions> _trainingOptions;
    private readonly IOptionsMonitor<VendorApiOptions> _vendorOptions;
    private readonly ILogger<VendorController> _logger;

    public VendorController(
        IApiKeyAuthorizer authorizer,
        IStonehengeSimulationStore store,
        IOptionsMonitor<TrainingVulnerabilityOptions> trainingOptions,
        IOptionsMonitor<VendorApiOptions> vendorOptions,
        ILogger<VendorController> logger)
    {
        _authorizer = authorizer;
        _store = store;
        _trainingOptions = trainingOptions;
        _vendorOptions = vendorOptions;
        _logger = logger;
    }

    [HttpGet("part-requests")]
    [ProducesResponseType(typeof(IReadOnlyList<VendorPartRequestDefaultDto>), StatusCodes.Status200OK)]
    public ActionResult<object> PartRequests([FromQuery] string? fields, [FromQuery] string? include)
    {
        if (!Require(ApiScopes.VendorPartsRead, out var denied))
        {
            return denied;
        }

        var (includeWorkflow, includeIntegration) = VendorExpansion(fields, include);
        AuditExpansion(includeWorkflow);
        var requests = _store.GetVendorPartRequests(includeWorkflow, includeIntegration, _vendorOptions.CurrentValue.UseSeededData);
        if (!TrainingEnabled)
        {
            return Ok(requests.Select(SafePartRequest).ToArray());
        }
        Audit("FCU-INFO-004", ["gunId"]);
        Audit("FCU-INFO-005", ["partStatus"]);
        Audit("FCU-INFO-006", ["approvedAt", "updatedAt"]);
        return Ok(requests);
    }

    [HttpGet("part-requests/{requestId}")]
    [ProducesResponseType(typeof(VendorPartRequestDefaultDto), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status404NotFound)]
    public ActionResult<object> PartRequest(string requestId, [FromQuery] string? fields, [FromQuery] string? include)
    {
        if (!Require(ApiScopes.VendorPartsRead, out var denied))
        {
            return denied;
        }

        var (includeWorkflow, includeIntegration) = VendorExpansion(fields, include);
        AuditExpansion(includeWorkflow);
        var request = _store.GetVendorPartRequest(requestId, includeWorkflow, includeIntegration);
        if (request is null) return NotFound();
        if (!TrainingEnabled) return Ok(SafePartRequest(request));
        Audit("FCU-INFO-004", ["gunId"]);
        Audit("FCU-INFO-005", ["partStatus"]);
        Audit("FCU-INFO-006", ["approvedAt", "updatedAt"]);
        return Ok(request);
    }

    [HttpGet("guns/status")]
    [ProducesResponseType(typeof(IReadOnlyList<VendorGunStatusDto>), StatusCodes.Status200OK)]
    public ActionResult<object> GunStatus([FromQuery] string? include, [FromQuery] bool debug = false)
    {
        if (!Require(ApiScopes.VendorGunsRead, out var denied))
        {
            return denied;
        }

        if (!TrainingEnabled) return Ok(Array.Empty<object>());

        var includeDiagnostics = _trainingOptions.CurrentValue.EnableVendorGunStatusDiagnosticLeak
            && (debug || HasToken(include, "source"));
        Audit("FCU-INFO-003", ["mode"]);
        Audit("FCU-INFO-004", ["gunId"]);
        Audit("FCU-INFO-004", ["currentApprovedPartRequestId"]);
        Audit("FCU-INFO-005", ["shotCount", "partStatus"]);
        if (includeDiagnostics)
        {
            Audit("FCU-INFO-003", ["statusProvider", "runtimeProvider"]);
            Audit("FCU-INFO-005", ["engineeringPartServiceThresholdShots", "counterResetTrigger", "localEffect"]);
            Audit("FCU-INFO-006", ["lastEngineeringSync", "lastRuntimeSummary"]);
        }
        return Ok(_store.GetVendorGunStatuses(includeDiagnostics));
    }

    [HttpPost("part-requests/{requestId}/acknowledge")]
    [ProducesResponseType(typeof(VendorAcknowledgeResponse), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status404NotFound)]
    public ActionResult<object> Acknowledge(string requestId)
    {
        if (!Require(ApiScopes.VendorPartsAcknowledge, out var denied))
        {
            return denied;
        }

        var response = _store.AcknowledgePartRequest(requestId);
        if (response is null) return NotFound();
        if (!TrainingEnabled) return Ok(new { response.RequestId, response.VendorState, response.Acknowledged });
        Audit("FCU-INFO-006", ["acknowledgedAt"]);
        return Ok(response);
    }

    [HttpPost("part-requests/{requestId}/complete")]
    [ProducesResponseType(typeof(VendorCompletionDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(VendorCompletionDto), StatusCodes.Status409Conflict)]
    [ProducesResponseType(StatusCodes.Status404NotFound)]
    public ActionResult<object> Complete(string requestId, [FromBody] VendorCompleteRequestDto request)
    {
        if (!Require(ApiScopes.VendorPartsComplete, out var denied))
        {
            return denied;
        }

        var completion = _store.CompletePartRequest(
            requestId,
            request,
            _trainingOptions.CurrentValue.Enabled && _trainingOptions.CurrentValue.EnableCompletionReplay,
            out var replayConflict,
            out var capacityReached);

        if (capacityReached)
        {
            return Conflict(new { error = "workflow_store_capacity_reached" });
        }

        if (completion is null)
        {
            return NotFound();
        }

        if (!TrainingEnabled)
        {
            var safe = SafeCompletion(completion);
            return replayConflict ? Conflict(safe) : Ok(safe);
        }
        Audit("FCU-INFO-004", ["gunId"]);
        Audit("FCU-INFO-006", ["completedAt"]);
        return replayConflict ? Conflict(completion) : Ok(completion);
    }

    [HttpGet("completions")]
    [ProducesResponseType(typeof(IReadOnlyList<VendorCompletionDto>), StatusCodes.Status200OK)]
    public ActionResult<object> Completions()
    {
        if (!Require(ApiScopes.VendorPartsRead, out var denied))
        {
            return denied;
        }

        var completions = _store.GetVendorCompletions();
        if (!TrainingEnabled) return Ok(completions.Select(SafeCompletion).ToArray());
        Audit("FCU-INFO-004", ["gunId"]);
        Audit("FCU-INFO-006", ["completedAt"]);
        return Ok(completions);
    }

    private bool Require(string scope, out ActionResult denied)
    {
        var result = _authorizer.RequireScope(Request, scope);
        if (result.Succeeded)
        {
            denied = Ok();
            return true;
        }

        denied = Unauthorized(ScopeError(result, "vendor", ["vendor-app"]));
        return false;
    }

    private object ScopeError(ScopeAuthorizationResult result, string routeGroup, string[] acceptedClientTypes)
    {
        if (!_trainingOptions.CurrentValue.Enabled || !_trainingOptions.CurrentValue.EnableVerboseScopeErrors || result.Principal is null)
        {
            return new { error = "api_key_scope_required" };
        }

        return new
        {
            error = "api_key_scope_required",
            callerClient = result.Principal.ClientName ?? "unknown-client",
            routeGroup,
            requiredScope = result.RequiredScope,
            acceptedClientTypes,
            hint = "restricted app-to-app route"
        };
    }

    private (bool Workflow, bool Integration) VendorExpansion(string? fields, string? include)
    {
        var enabled = _trainingOptions.CurrentValue.Enabled && _trainingOptions.CurrentValue.EnableVendorFieldExpansionLeak;
        if (!enabled)
        {
            return (false, false);
        }

        var all = string.Equals(fields, "all", StringComparison.OrdinalIgnoreCase);
        return (all || HasToken(include, "workflow"), all || HasToken(include, "integration"));
    }

    private void AuditExpansion(bool workflow)
    {
        if (workflow) Audit("FCU-INFO-004", ["sourceObjectId"]);
        if (workflow) Audit("FCU-INFO-006", ["completionEvent", "syncModel"]);
    }

    private bool TrainingEnabled => _trainingOptions.CurrentValue.Enabled;

    private static object SafePartRequest(VendorPartRequestDefaultDto request) => new
    {
        request.RequestId,
        request.Status,
        request.Priority,
        request.Acknowledged
    };

    private static object SafeCompletion(VendorCompletionDto completion) => new
    {
        completion.CompletionId,
        completion.RequestId,
        completion.VendorState,
        completion.AppliedByEngineering,
        completion.CompletionNote,
        completion.TrackingReference
    };

    private void Audit(string findingId, IReadOnlyList<string> fields)
    {
        var caller = _authorizer.Resolve(Request)?.ClientName ?? "unknown-client";
        _logger.LogWarning("Classified fields exposed Endpoint={Endpoint} Caller={Caller} FieldNames={FieldNames} Classification={Classification} FindingId={FindingId} CorrelationId={CorrelationId}",
            Request.Path.Value, caller, fields, "TOP SECRET // FCU-OT", findingId, HttpContext.TraceIdentifier);
    }

    private static bool HasToken(string? value, string token)
    {
        return value?.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .Any(item => item.Equals(token, StringComparison.OrdinalIgnoreCase)) == true;
    }
}
