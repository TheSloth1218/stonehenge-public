using Microsoft.AspNetCore.Mvc;
using Stonehenge.Api.Auth;
using Stonehenge.Api.Models;
using Stonehenge.Api.Options;
using Microsoft.Extensions.Options;

namespace Stonehenge.Api.Controllers;

[ApiController]
[Route("api/v1/system")]
public sealed class SystemController : ControllerBase
{
    private readonly IConfiguration _configuration;
    private readonly IWebHostEnvironment _environment;
    private readonly IApiKeyAuthorizer _authorizer;
    private readonly IOptionsMonitor<TrainingVulnerabilityOptions> _training;

    public SystemController(
        IConfiguration configuration,
        IWebHostEnvironment environment,
        IApiKeyAuthorizer authorizer,
        IOptionsMonitor<TrainingVulnerabilityOptions> training)
    {
        _configuration = configuration;
        _environment = environment;
        _authorizer = authorizer;
        _training = training;
    }

    [HttpGet("health")]
    [ProducesResponseType(typeof(ApiHealthResponse), StatusCodes.Status200OK)]
    public ActionResult<ApiHealthResponse> Health()
    {
        return Ok(new ApiHealthResponse(
            Status: "healthy",
            Service: "stonehenge-api",
            Environment: _environment.EnvironmentName,
            TimestampUtc: DateTimeOffset.UtcNow));
    }

    [HttpGet("public-status")]
    [ProducesResponseType(typeof(PublicStatusResponse), StatusCodes.Status200OK)]
    public ActionResult<PublicStatusResponse> PublicStatus()
    {
        if (!_authorizer.HasScope(Request, ApiScopes.SystemPublicStatusRead))
        {
            return Unauthorized(new { error = "api_key_scope_required" });
        }

        return Ok(new PublicStatusResponse(
            Status: "online",
            Service: "stonehenge-api",
            ApiSurface: "public-client",
            PublicClients: ["stonehenge-astro-app"],
            TimestampUtc: DateTimeOffset.UtcNow));
    }

    [HttpGet("info")]
    [ProducesResponseType(StatusCodes.Status200OK)]
    public ActionResult<object> Info()
    {
        var response = new Dictionary<string, object?>
        {
            ["name"] = "Stonehenge Internal API",
            ["purpose"] = "Inter-app coordination surface for Stonehenge companion applications.",
            ["intendedClients"] = new[] { "stonehenge-engineering-app", "stonehenge-vendor-app", "stonehenge-astro-app" }
        };
        if (_training.CurrentValue.Enabled)
        {
            response["middlewareBaseUrl"] = _configuration["Stonehenge:MiddlewareBaseUrl"] ?? "http://middleware:8080";
            response["classification"] = "TOP SECRET // FCU-OT";
        }
        return Ok(response);
    }
}
