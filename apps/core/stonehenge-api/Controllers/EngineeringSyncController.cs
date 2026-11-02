using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Options;
using Stonehenge.Api.Auth;
using Stonehenge.Api.Models.Engineering;
using Stonehenge.Api.Models.Vendor;
using Stonehenge.Api.Options;
using Stonehenge.Api.Services;

namespace Stonehenge.Api.Controllers;

[ApiController]
[Route("api/v1/engineering")]
public sealed class EngineeringSyncController : ControllerBase
{
    private readonly IApiKeyAuthorizer _authorizer;
    private readonly IStonehengeSimulationStore _store;
    private readonly IOptionsMonitor<TrainingVulnerabilityOptions> _trainingOptions;

    public EngineeringSyncController(
        IApiKeyAuthorizer authorizer,
        IStonehengeSimulationStore store,
        IOptionsMonitor<TrainingVulnerabilityOptions> trainingOptions)
    {
        _authorizer = authorizer;
        _store = store;
        _trainingOptions = trainingOptions;
    }

    [HttpGet("vendor-completions/pending")]
    [ProducesResponseType(typeof(PendingVendorCompletionsResponse), StatusCodes.Status200OK)]
    public ActionResult<PendingVendorCompletionsResponse> PendingVendorCompletions()
    {
        if (!Require(ApiScopes.EngineeringVendorCompletionsRead, out var denied))
        {
            return denied;
        }

        return Ok(new PendingVendorCompletionsResponse(_store.GetPendingVendorCompletions()));
    }

    [HttpPost("part-requests")]
    [ProducesResponseType(typeof(VendorPartRequestDefaultDto), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status409Conflict)]
    public ActionResult<VendorPartRequestDefaultDto> ExportPartRequest([FromBody] EngineeringPartRequestExportRequest request)
    {
        if (!Require(ApiScopes.EngineeringPartRequestsWrite, out var denied))
        {
            return denied;
        }

        if (string.IsNullOrWhiteSpace(request.RequestId) || request.SourceObjectId <= 0 || request.GunId is < 1 or > 8)
        {
            return BadRequest(new { error = "invalid_part_request_export" });
        }

        var exported = _store.UpsertEngineeringPartRequest(request, out var capacityReached);
        return capacityReached
            ? Conflict(new { error = "workflow_store_capacity_reached" })
            : Ok(exported);
    }

    [HttpPost("part-requests/{requestId}/withdraw")]
    [ProducesResponseType(typeof(EngineeringPartRequestWithdrawResponse), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status404NotFound)]
    public ActionResult<EngineeringPartRequestWithdrawResponse> WithdrawPartRequest(string requestId)
    {
        if (!Require(ApiScopes.EngineeringPartRequestsWrite, out var denied))
        {
            return denied;
        }

        var response = _store.WithdrawEngineeringPartRequest(requestId);
        return response is null ? NotFound() : Ok(response);
    }

    [HttpPost("vendor-completions/{completionId}/applied")]
    [ProducesResponseType(typeof(EngineeringApplyCompletionResponse), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status404NotFound)]
    public ActionResult<EngineeringApplyCompletionResponse> MarkCompletionApplied(string completionId)
    {
        if (!Require(ApiScopes.EngineeringVendorCompletionsApply, out var denied))
        {
            return denied;
        }

        var completion = _store.MarkCompletionApplied(completionId, out var appliedAt);
        if (completion is null)
        {
            return NotFound();
        }

        return Ok(new EngineeringApplyCompletionResponse(completion.CompletionId, true, appliedAt));
    }

    private bool Require(string scope, out ActionResult denied)
    {
        var result = _authorizer.RequireScope(Request, scope);
        if (result.Succeeded)
        {
            denied = Ok();
            return true;
        }

        denied = Unauthorized(ScopeError(result, "engineering", ["engineering-app"]));
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
}
