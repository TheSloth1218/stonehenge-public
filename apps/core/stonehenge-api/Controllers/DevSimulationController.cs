using Microsoft.AspNetCore.Mvc;
using Stonehenge.Api.Auth;
using Stonehenge.Api.Models.Dev;
using Stonehenge.Api.Services;

namespace Stonehenge.Api.Controllers;

[ApiController]
[Route("api/v1/dev")]
public sealed class DevSimulationController : ControllerBase
{
    private readonly IApiKeyAuthorizer _authorizer;
    private readonly IWebHostEnvironment _environment;
    private readonly IStonehengeSimulationStore _store;

    public DevSimulationController(
        IApiKeyAuthorizer authorizer,
        IWebHostEnvironment environment,
        IStonehengeSimulationStore store)
    {
        _authorizer = authorizer;
        _environment = environment;
        _store = store;
    }

    [HttpPost("simulation/reset")]
    [ProducesResponseType(typeof(SimulationResetResponse), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status404NotFound)]
    public ActionResult<SimulationResetResponse> ResetSimulation()
    {
        if (!_environment.IsDevelopment())
        {
            return NotFound();
        }

        if (!_authorizer.HasScope(Request, ApiScopes.EngineeringVendorCompletionsApply))
        {
            return Unauthorized(new
            {
                error = "api_key_scope_required",
                requiredScope = ApiScopes.EngineeringVendorCompletionsApply
            });
        }

        var resetAt = _store.ResetSeededState();
        return Ok(new SimulationResetResponse(
            Status: "reset",
            Scope: "seeded-api-memory",
            ResetAt: resetAt,
            VendorGunCount: _store.GetVendorGunStatuses(includeDiagnostics: false).Count,
            AstroEventCount: _store.GetTargetHistory().Count));
    }
}
