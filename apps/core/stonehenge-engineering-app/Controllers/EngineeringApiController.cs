using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Stonehenge.Engineering.App.Models;
using Stonehenge.Engineering.App.Security;
using Stonehenge.Engineering.App.Services;
using Microsoft.Extensions.Options;

namespace Stonehenge.Engineering.App.Controllers;

[ApiController]
public sealed class EngineeringApiController : ControllerBase
{
    private readonly IEngineeringDashboardData _dashboardData;
    private readonly IStonehengeMiddlewareClient _middlewareClient;
    private readonly IMaintenanceRequestStore _maintenanceRequests;
    private readonly IAppEventStore _appEvents;
    private readonly IEngineeringCommandStore _commands;
    private readonly IEngineeringPanelCatalog _panels;
    private readonly IFuxaEmbedService _fuxaEmbed;
    private readonly IStonehengeWorkflowApiClient _workflowApiClient;
    private readonly IConfiguration _configuration;
    private readonly IWebHostEnvironment _environment;
    private readonly StonehengeAuthenticationOptions _authenticationOptions;
    private readonly TrainingSecurityOptions _training;
    private readonly IPanelCapabilityService _panelCapabilities;
    private readonly ILogger<EngineeringApiController> _logger;

    public EngineeringApiController(
        IEngineeringDashboardData dashboardData,
        IStonehengeMiddlewareClient middlewareClient,
        IMaintenanceRequestStore maintenanceRequests,
        IAppEventStore appEvents,
        IEngineeringCommandStore commands,
        IEngineeringPanelCatalog panels,
        IFuxaEmbedService fuxaEmbed,
        IStonehengeWorkflowApiClient workflowApiClient,
        IConfiguration configuration,
        IWebHostEnvironment environment,
        StonehengeAuthenticationOptions authenticationOptions,
        IOptions<TrainingSecurityOptions> training,
        IPanelCapabilityService panelCapabilities,
        ILogger<EngineeringApiController> logger)
    {
        _dashboardData = dashboardData;
        _middlewareClient = middlewareClient;
        _maintenanceRequests = maintenanceRequests;
        _appEvents = appEvents;
        _commands = commands;
        _panels = panels;
        _fuxaEmbed = fuxaEmbed;
        _workflowApiClient = workflowApiClient;
        _configuration = configuration;
        _environment = environment;
        _authenticationOptions = authenticationOptions;
        _training = training.Value;
        _panelCapabilities = panelCapabilities;
        _logger = logger;
    }

    [HttpGet("/api/guns/{gunId:int}/state")]
    [Authorize(Policy = StonehengeAuth.DashboardAccessPolicy)]
    public async Task<IActionResult> GetGunState(int gunId, [FromQuery] bool poll, CancellationToken cancellationToken)
    {
        if (gunId is < 1 or > 8)
        {
            return NotFound(new { ok = false, error = "Gun not found." });
        }

        var model = await _dashboardData.GetOverviewAsync(cancellationToken);
        var gun = model.Guns.SingleOrDefault(gun => gun.GunNumber == gunId);
        if (gun is null)
        {
            return NotFound(new { ok = false, error = "Gun not found." });
        }

        if (IsSafeEmployee())
        {
            return Ok(new { ok = true, gunId, name = gun.Name });
        }

        if (!poll)
        {
            _appEvents.Add(
                "admin",
                User.StonehengeDisplayName(),
                "Gun state read",
                "Read",
                $"gunId={gunId}; status={gun.Status}; firePermission={gun.SafetyState}");
        }

        MiddlewareGunSnapshot? liveGun = null;
        try
        {
            var liveStatus = await _middlewareClient.GetStatusAsync(cancellationToken);
            liveGun = liveStatus.Guns.SingleOrDefault(gun => gun.GunNumber == gunId);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            // The dashboard model may already be using its fallback. Keep this read-only API usable
            // while making the data source explicit to the browser.
        }

        var response = new Dictionary<string, object?>
        {
            ["ok"] = true, ["gunId"] = gunId, ["name"] = gun.Name, ["panelId"] = $"gun-{gunId}-status",
            ["panelIds"] = _panels.ForGun(gunId).Select(panel => panel.PanelId).ToArray(),
            ["manualPanelId"] = $"manual-gun-{gunId}", ["autoPanelId"] = $"auto-gun-{gunId}",
            ["status"] = gun.Status, ["statusClass"] = gun.StatusClass, ["firePermission"] = gun.SafetyState,
            ["azimuth"] = gun.Azimuth, ["azimuthX10"] = Math.Round(gun.Azimuth * 10, 0),
            ["elevation"] = gun.Elevation, ["elevationX10"] = Math.Round(gun.Elevation * 10, 0),
            ["mode"] = liveGun?.Mode, ["state"] = liveGun?.State ?? gun.Status,
            ["lastSequence"] = liveGun is { ShotSequence: > 0 } ? $"SEQ-{liveGun.ShotSequence:0000}" : "Idle",
            ["middlewareSource"] = liveGun is null ? "Fallback" : "Live", ["lastUpdated"] = gun.LastUpdated
        };
        if (_training.Enabled)
        {
            response["commandIds"] = CommandsForGun(gunId).Select(command => command.Id).ToArray();
            response["routeHints"] = new[] { $"/api/commands/{CommandHintForGun(gunId)}", $"/api/guns/{gunId}/logs" };
            response["operational"] = new { telemetrySource = liveGun is null ? "MVC fallback model" : "Middleware /api/v1/status", runtimeEndpoint = RuntimeEndpointForGun(gunId), fuxaTagSource = FuxaTagSourceForGun(gunId), plcScanMs = 20, plcSettleSeconds = 5, plcPulseMs = 40, plcLockoutReloadSeconds = 10, plcAzimuthStepDegreesPerScan = 1.0, plcElevationStepDegreesPerScan = 0.5, plcSnapToleranceDegrees = 0.5 };
            AuditExposure("/api/guns/{id}/state", "FCU-INFO-001", ["plcScanMs", "plcSettleSeconds", "plcPulseMs", "plcLockoutReloadSeconds", "plcAzimuthStepDegreesPerScan", "plcElevationStepDegreesPerScan", "plcSnapToleranceDegrees"]);
            AuditExposure("/api/guns/{id}/state", "FCU-INFO-002", ["firePermission"]);
            AuditExposure("/api/guns/{id}/state", "FCU-INFO-003", ["status", "statusClass", "mode", "state", "azimuth", "azimuthX10", "elevation", "elevationX10", "middlewareSource", "lastUpdated", "telemetrySource"]);
            AuditExposure("/api/guns/{id}/state", "FCU-INFO-004", ["gunId", "panelId", "panelIds", "manualPanelId", "autoPanelId"]);
            AuditExposure("/api/guns/{id}/state", "FCU-INFO-005", ["lastSequence"]);
        }
        return Ok(response);
    }

    [HttpGet("/api/guns/{gunId:int}/logs")]
    [Authorize(Policy = StonehengeAuth.DashboardAccessPolicy)]
    public IActionResult GetGunLogs(int gunId)
    {
        if (gunId is < 1 or > 8)
        {
            return NotFound(new { ok = false, error = "Gun not found." });
        }

        if (IsSafeEmployee())
        {
            return Ok(new { ok = true, gunId });
        }

        var gunLabel = $"Gun {gunId}";
        var events = _appEvents.List("maintenance", 100)
            .Concat(_appEvents.List("admin", 100))
            .Where(entry => ContainsGun(entry, gunLabel))
            .OrderByDescending(entry => entry.Timestamp)
            .Take(25)
            .Select(ToLogDto)
            .ToArray();
        var requests = _maintenanceRequests.List()
            .Where(request => request.GunNumber == gunId)
            .Take(10)
            .Select(ToRequestBreadcrumb)
            .ToArray();

        _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "Gun logs read",
            "Read",
            $"gunId={gunId}; events={events.Length}; requestBreadcrumbs={requests.Length}");

        var logsResponse = new Dictionary<string, object?>
        {
            ["ok"] = true, ["gunId"] = gunId, ["panelId"] = $"logs-gun-{gunId}",
            ["route"] = $"/api/guns/{gunId}/logs", ["events"] = events
        };
        if (_training.Enabled)
        {
            AuditExposure("/api/guns/{id}/logs", "FCU-INFO-004", ["gunId", "panelId", "panelIds", "commandIds"]);
            AuditExposure("/api/guns/{id}/logs", "FCU-INFO-005", ["maintenanceRequests"]);
            AuditExposure("/api/guns/{id}/logs", "FCU-INFO-006", ["events"]);
            logsResponse["panelIds"] = _panels.ForGun(gunId).Select(panel => panel.PanelId).ToArray();
            logsResponse["commandIds"] = CommandsForGun(gunId).Select(command => command.Id).ToArray();
            logsResponse["routeHints"] = new[] { $"/api/commands/{CommandHintForGun(gunId)}", $"/api/commands/{CommandHintForGun(gunId)}/replay", $"/api/maintenance/requests/{RequestHintForGun(gunId)}" };
            logsResponse["maintenanceRequests"] = requests;
        }
        return Ok(logsResponse);
    }

    [HttpGet("/api/system/events")]
    [Authorize(Policy = StonehengeAuth.DashboardAccessPolicy)]
    public IActionResult GetSystemEvents()
    {
        if (IsSafeEmployee())
        {
            return Ok(new { ok = true });
        }

        var events = _appEvents.List("admin", 50)
            .Concat(_appEvents.List("maintenance", 50))
            .OrderByDescending(entry => entry.Timestamp)
            .Take(50)
            .Select(ToLogDto)
            .ToArray();

        _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "System events read",
            "Read",
            "Browser-facing system event breadcrumbs requested.");

        var systemResponse = new Dictionary<string, object?>
        {
            ["ok"] = true,
            ["route"] = "/api/system/events",
            ["station"] = "stonehenge-engineering-console",
            ["breadcrumbs"] = new { panelIds = _panels.List().Where(panel => panel.PanelId != 99).Select(panel => panel.PanelId).ToArray() },
            ["events"] = events
        };
        if (_training.Enabled)
        {
            AuditExposure("/api/system/events", "FCU-INFO-004", ["panelIds", "commandIds"]);
            AuditExposure("/api/system/events", "FCU-INFO-005", ["requestIds"]);
            AuditExposure("/api/system/events", "FCU-INFO-006", ["events", "eventStores"]);
            systemResponse["breadcrumbs"] = new
            {
                panelIds = _panels.List().Select(panel => panel.PanelId).ToArray(),
                commandIds = _commands.List(10).Select(command => command.Id).ToArray(),
                requestIds = _maintenanceRequests.List().Take(10).Select(request => request.Id).ToArray(),
                routeHints = new[] { "/api/commands/1002", "/api/guns/1/state", "/api/maintenance/requests/{requestId}", "/api/part-requests/{requestId}" }
            };
            systemResponse["operational"] = new { eventStores = new[] { "admin", "maintenance" }, classification = "TOP SECRET // FCU-OT", findingId = "FCU-INFO-006" };
        }
        return Ok(systemResponse);
    }

    [HttpGet("/api/manual/context")]
    [Authorize(Policy = StonehengeAuth.DashboardAccessPolicy)]
    public async Task<IActionResult> GetManualContext(int? gunNumber, CancellationToken cancellationToken)
    {
        if (IsSafeEmployee())
        {
            return Ok(new { ok = true });
        }

        var model = await _dashboardData.GetManualControlAsync(gunNumber, cancellationToken);
        var manualResponse = new Dictionary<string, object?>
        {
            ["ok"] = true,
            ["route"] = "/api/manual/context",
            ["panelId"] = $"manual-gun-{model.SelectedGunNumber}",
            ["selectedGunNumber"] = model.SelectedGunNumber,
            ["selectedGun"] = new
            {
                model.SelectedGun.GunNumber,
                model.SelectedGun.Name,
                model.SelectedGun.Status,
                firePermission = model.SelectedGun.SafetyState,
                model.SelectedGun.Azimuth,
                model.SelectedGun.Elevation
            },
            ["runtime"] = new
            {
                model.RuntimeStatus.ManualMode,
                model.RuntimeStatus.HmiConnection,
                model.RuntimeStatus.MiddlewareConnection,
                model.RuntimeStatus.CurrentInterlockState
            }
        };
        if (_training.Enabled)
        {
            AuditExposure("/api/manual/context", "FCU-INFO-001", ["tickMs", "slewDegreesPerSecond", "minSlewSeconds", "maxSlewSeconds", "settleSeconds", "lockoutSeconds"]);
            AuditExposure("/api/manual/context", "FCU-INFO-002", ["firePermission", "currentInterlockState"]);
            AuditExposure("/api/manual/context", "FCU-INFO-003", ["status", "manualMode", "hmiConnection", "middlewareConnection"]);
            AuditExposure("/api/manual/context", "FCU-INFO-004", ["panelId", "selectedGunNumber", "gunNumber", "name", "azimuth", "elevation"]);
            manualResponse["simulationTiming"] = new { tickMs = 250, slewDegreesPerSecond = 18, minSlewSeconds = 1, maxSlewSeconds = 12, settleSeconds = 5, lockoutSeconds = 10, classification = "TOP SECRET // FCU-OT", findingId = "FCU-INFO-001" };
        }
        return Ok(manualResponse);
    }

    [HttpGet("/api/maintenance/requests/{requestId:int}")]
    [Authorize(Policy = StonehengeAuth.DashboardAccessPolicy)]
    public IActionResult GetMaintenanceRequest(int requestId)
    {
        if (!_training.Enabled && !IsEngineering()) return Forbid();
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId);
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Maintenance request not found." });
        }

        _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "Maintenance request read",
            "Read",
            $"requestId={request.Id}; gunId={request.GunNumber}; status={request.Status}; sourceApp={request.SourceApp}");

        return Ok(new
        {
            ok = true,
            route = $"/api/maintenance/requests/{requestId}",
            request = ToRequestBreadcrumb(request),
            requestReview = new
            {
                reviewState = "pending-admin-review",
                panelId = request.RequestType == "Part" ? "part-request-panel" : "maintenance-request-panel",
                panelIds = _panels.ForGun(request.GunNumber).Select(panel => panel.PanelId).ToArray(),
                commandIds = CommandsForGun(request.GunNumber).Select(command => command.Id).ToArray(),
                commandRoute = $"/api/commands/{CommandHintForGun(request.GunNumber)}",
                approveRoute = $"/api/maintenance/requests/{requestId}/approve",
                commentRoute = $"/api/maintenance/requests/{requestId}/comment"
            }
        });
    }

    [HttpPost("/api/panels/{panelId:int}/embed-token")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult CreatePanelEmbedToken(int panelId)
    {
        var panel = _panels.Get(panelId);
        if (panel is null || !panel.Enabled)
        {
            return NotFound(new { ok = false, error = "Panel not found." });
        }

        var isAdmin = User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel);
        if (panel.RequiredAccess == StonehengeAuth.EngineeringAdminAccessPolicy && !isAdmin &&
            !(_training.Enabled && panel.PanelId == 99))
        {
            return Forbid();
        }

        var subject = User.FindFirst("sub")?.Value ?? User.FindFirst(System.Security.Claims.ClaimTypes.NameIdentifier)?.Value ?? User.StonehengeDisplayName();
        var capability = _panelCapabilities.Issue(panel.PanelId, subject);
        var frame = _fuxaEmbed.BuildPanelFrame(panel.PanelId);
        if (_training.Enabled && panel.PanelId == 99)
        {
            AuditExposure("/api/panels/99/embed-token", "FCU-ENG-003", ["panelId", "panelName", "panelType", "requiredAccess", "fuxaViewId", "iframePath", "iframeUrl", "token", "issuedTo", "expiresUtc"]);
        }
        _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "Panel token requested",
            "Issued",
            $"panelId={panel.PanelId}; panelType={panel.PanelType}; gunId={panel.GunId?.ToString() ?? "fleet"}; fuxaViewId={panel.FuxaViewId}");

        return Ok(new
        {
            ok = true,
            panelId = panel.PanelId,
            panelName = panel.PanelName,
            panelType = panel.PanelType,
            gunId = panel.GunId,
            requiredAccess = panel.RequiredAccess,
            fuxaViewId = panel.FuxaViewId,
            iframePath = frame.Enabled ? new Uri(frame.IframeUrl!).PathAndQuery : null,
            iframeUrl = frame.Enabled ? frame.IframeUrl : null,
            token = capability.Token,
            issuedTo = User.StonehengeDisplayName(),
            expiresUtc = capability.ExpiresUtc,
            warning = "development embed metadata only; no production credentials are included",
            findingId = _training.Enabled && panel.PanelId == 99 ? "FCU-ENG-003" : null
        });
    }

    [HttpGet("/api/panels")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult ListPanels()
    {
        _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "Panel metadata listed",
            "Read",
            "Sanitized panel metadata requested for future FUXA view selection.");

        return Ok(new
        {
            ok = true,
            route = "/api/panels",
            panels = _panels.List().Select(ToPanelDto).ToArray()
        });
    }

    [HttpGet("/api/commands/{commandId:int}")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult GetCommand(int commandId)
    {
        if (!_training.Enabled && !IsAdmin()) return Forbid();
        var command = _commands.Get(commandId);
        if (command is null)
        {
            return NotFound(new { ok = false, error = "Command not found." });
        }

        _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "Command object read",
            "Read",
            $"commandId={command.Id}; gunId={command.GunNumber}; panelId={command.SourcePanelId}; status={command.Status}");

        if (_training.Enabled)
        {
            AuditExposure("/api/commands/{commandId}", "FCU-ENG-002", ["command", "routeHints"]);
        }

        return Ok(new
        {
            ok = true,
            command = ToCommandDto(command),
            routeHints = new[] { $"/api/commands/{command.Id}/replay", $"/api/panels/{command.SourcePanelId}/embed-token" },
            findingId = _training.Enabled ? "FCU-ENG-002" : null
        });
    }

    [HttpPost("/api/commands/{commandId:int}/replay")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult ReplayCommand(int commandId)
    {
        if (!_training.Enabled && !IsAdmin()) return Forbid();
        var command = _commands.Get(commandId);
        if (command is null)
        {
            return NotFound(new { ok = false, error = "Command not found." });
        }

        var log = _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "Command replay simulated",
            "SimulatedReplay",
            $"Command {command.Id} ({command.CommandType}) recorded as bounded replay simulation for Gun {command.GunNumber}.");

        if (_training.Enabled)
        {
            AuditExposure("/api/commands/{commandId}/replay", "FCU-ENG-002", ["commandId", "replayStatus", "requestedBy", "eventId", "message"]);
        }

        return Ok(new
        {
            ok = true,
            commandId = command.Id,
            replayStatus = "SimulatedReplay",
            requestedBy = User.StonehengeDisplayName(),
            eventId = log.Id,
            findingId = _training.Enabled ? "FCU-ENG-002" : null,
            message = "Replay was recorded as a bounded simulation. No middleware command, PLC write, or fire command was executed."
        });
    }

    [HttpPost("/api/maintenance/requests/{requestId:int}/comment")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult AddMaintenanceComment(int requestId, [FromBody] MaintenanceCommentRequest? comment)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId);
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Maintenance request not found." });
        }

        var text = string.IsNullOrWhiteSpace(comment?.Comment)
            ? "Engineering comment placeholder added."
            : comment.Comment.Trim();
        _appEvents.Add(
            "maintenance",
            User.StonehengeDisplayName(),
            $"{request.RequestType} request comment",
            "Commented",
            $"{request.GunName}: {text}");

        return Ok(new
        {
            ok = true,
            requestId,
            request.RequestType,
            request.GunNumber,
            comment = text
        });
    }

    [HttpPost("/api/maintenance/requests")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult CreateMaintenanceRequest([FromBody] SimpleMaintenanceRequestInput input)
    {
        var gunId = ResolveGunId(input.GunId, input.AssetId);
        if (gunId is null)
        {
            return BadRequest(new { ok = false, error = "gunId or assetId is required." });
        }

        var request = _maintenanceRequests.Submit(
            gunId.Value,
            notes: "Engineering API maintenance request.",
            requestedBy: User.StonehengeDisplayName(),
            requestType: "Maintenance",
            sourceApp: "engineering-app",
            assetId: CleanAssetId(input.AssetId, gunId.Value),
            panelId: input.PanelId ?? gunId.Value,
            requestSubType: CleanRequestType(input.RequestType, "EnableMaintenance"));

        _appEvents.Add(
            "maintenance",
            User.StonehengeDisplayName(),
            "Maintenance request submitted",
            "Pending",
            $"{request.GunName}: engineering API submitted {request.RequestSubType}.");

        return Created($"/api/maintenance/requests/{request.Id}", new
        {
            ok = true,
            request = ToRequestBreadcrumb(request),
            message = "Simple maintenance request created for admin review. No middleware command was executed."
        });
    }

    [HttpGet("/api/part-requests/{requestId:int}")]
    [Authorize(Policy = StonehengeAuth.DashboardAccessPolicy)]
    public IActionResult GetPartRequest(int requestId)
    {
        if (!_training.Enabled && !IsEngineering()) return Forbid();
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId && request.RequestKind == "PartRequest");
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Part request not found." });
        }

        _appEvents.Add(
            "admin",
            User.StonehengeDisplayName(),
            "Part request read",
            "Read",
            $"requestId={request.Id}; gunId={request.GunNumber}; status={request.Status}; sourceApp={request.SourceApp}");

        if (_training.Enabled && IsEngineering() && request.Id == 2003)
        {
            return Ok(new
            {
                ok = true, route = $"/api/part-requests/{requestId}", request = ToRequestBreadcrumb(request),
                trainingBreadcrumb = new { program = "NIGHTWATCH", ticket = "ENG-1847", serviceAccount = "gmsa-idreview$", identityAlias = "count", purpose = "identity-review", findingId = "FCU-ENG-001" }
            });
        }
        return Ok(new { ok = true, route = $"/api/part-requests/{requestId}", request = ToRequestBreadcrumb(request) });
    }

    [HttpPost("/api/part-requests")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult CreatePartRequest([FromBody] SimplePartRequestInput input)
    {
        var gunId = ResolveGunId(input.GunId, input.AssetId);
        if (gunId is null)
        {
            return BadRequest(new { ok = false, error = "gunId or assetId is required." });
        }

        var request = _maintenanceRequests.Submit(
            gunId.Value,
            notes: "Engineering API part request.",
            requestedBy: User.StonehengeDisplayName(),
            requestType: "Part",
            sourceApp: "engineering-app",
            assetId: CleanAssetId(input.AssetId, gunId.Value),
            panelId: input.PanelId ?? gunId.Value,
            requestSubType: "RequestPart");

        _appEvents.Add(
            "maintenance",
            User.StonehengeDisplayName(),
            "Part request submitted",
            "Pending",
            $"{request.GunName}: engineering API submitted part request.");

        return Created($"/api/part-requests/{request.Id}", new
        {
            ok = true,
            request = ToRequestBreadcrumb(request),
            message = "Simple part request created for admin review. No middleware command was executed."
        });
    }

    [HttpPost("/api/maintenance/requests/{requestId:int}/approve")]
    [HttpPost("/api/admin/maintenance/requests/{requestId:int}/approve")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> ApproveMaintenanceRequestApi(int requestId, CancellationToken cancellationToken)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId);
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Maintenance request not found." });
        }

        if (request.Status != "Pending")
        {
            return Conflict(new { ok = false, requestId, request.Status, error = "Request is no longer pending." });
        }

        if (request.RequestType == "Part")
        {
            var approvedPart = _maintenanceRequests.Approve(requestId, User.StonehengeDisplayName(), "Approved through admin API.");
            if (approvedPart is null)
            {
                return Conflict(new { ok = false, requestId, request.Status, error = "Request is no longer pending." });
            }

            var export = await _workflowApiClient.ExportApprovedPartRequestAsync(approvedPart, cancellationToken);
            _appEvents.Add(
                "admin",
                User.StonehengeDisplayName(),
                "Part request approved",
                export.Success ? "Approved" : "Warning",
                export.Success
                    ? $"{request.GunName}: admin API approved and exported part request."
                    : $"{request.GunName}: admin API approved locally; vendor API export failed.");
            return Ok(new { ok = true, request = ToRequestBreadcrumb(approvedPart), vendorExported = export.Success });
        }

        var result = await _middlewareClient.SetGunModeAsync(request.GunNumber, "maintenance", cancellationToken);
        if (!result.Success)
        {
            _appEvents.Add("admin", User.StonehengeDisplayName(), "Maintenance API approval failed", "Error", $"{request.GunName}: {result.Error ?? result.Reason ?? "middleware rejected mode change"}");
            return StatusCode(StatusCodes.Status502BadGateway, new { ok = false, result.Error, result.Reason });
        }

        var approved = _maintenanceRequests.Approve(requestId, User.StonehengeDisplayName(), $"Approved through admin API; mode now {result.Mode ?? result.RequestedMode}.");
        _appEvents.Add("admin", User.StonehengeDisplayName(), "Maintenance request approved", "Approved", $"{request.GunName}: admin API mode now {result.Mode ?? result.RequestedMode}");
        return Ok(new { ok = true, request = ToRequestBreadcrumb(approved!) });
    }

    [HttpPost("/api/admin/maintenance/requests/{requestId:int}/reject")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public IActionResult RejectMaintenanceRequestApi(int requestId)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId && request.RequestKind == "MaintenanceMode");
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Maintenance request not found." });
        }

        var rejected = _maintenanceRequests.Deny(requestId, User.StonehengeDisplayName(), "Rejected through admin API.");
        if (rejected is null)
        {
            return Conflict(new { ok = false, requestId, request.Status, error = "Request is no longer pending." });
        }

        _appEvents.Add("admin", User.StonehengeDisplayName(), "Maintenance request rejected", "Rejected", $"{request.GunName}: admin API rejected maintenance request.");
        return Ok(new { ok = true, request = ToRequestBreadcrumb(rejected) });
    }

    [HttpPost("/api/admin/part-requests/{requestId:int}/approve")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> ApprovePartRequestApi(int requestId, CancellationToken cancellationToken)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId && request.RequestKind == "PartRequest");
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Part request not found." });
        }

        var approved = _maintenanceRequests.Approve(requestId, User.StonehengeDisplayName(), "Approved through admin API.");
        if (approved is null)
        {
            return Conflict(new { ok = false, requestId, request.Status, error = "Request is no longer pending." });
        }

        _appEvents.Add("admin", User.StonehengeDisplayName(), "Part request approved", "Approved", $"{request.GunName}: admin API approved simple part request.");
        var export = await _workflowApiClient.ExportApprovedPartRequestAsync(approved, cancellationToken);
        if (!export.Success)
        {
            _appEvents.Add("admin", User.StonehengeDisplayName(), "Part request vendor export failed", "Warning", $"{request.GunName}: local approval succeeded; vendor API export failed.");
        }

        return Ok(new
        {
            ok = true,
            request = ToRequestBreadcrumb(approved),
            vendorExported = export.Success,
            message = export.Success
                ? "Part request approved and exported for vendor completion."
                : "Part request approved locally. Vendor API export failed."
        });
    }

    [HttpPost("/api/admin/part-requests/{requestId:int}/reject")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public IActionResult RejectPartRequestApi(int requestId)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId && request.RequestKind == "PartRequest");
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Part request not found." });
        }

        var rejected = _maintenanceRequests.Deny(requestId, User.StonehengeDisplayName(), "Rejected through admin API.");
        if (rejected is null)
        {
            return Conflict(new { ok = false, requestId, request.Status, error = "Request is no longer pending." });
        }

        _appEvents.Add("admin", User.StonehengeDisplayName(), "Part request rejected", "Rejected", $"{request.GunName}: admin API rejected simple part request.");
        return Ok(new { ok = true, request = ToRequestBreadcrumb(rejected) });
    }

    [HttpGet("/api/admin/audit")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult GetAdminAudit()
    {
        if (!_training.Enabled && !IsAdmin()) return Forbid();
        return Ok(new
        {
            ok = true,
            route = "/api/admin/audit",
            panelId = "admin-audit",
            events = _appEvents.List("admin", 40).Select(ToLogDto).ToArray()
        });
    }

    [HttpGet("/api/admin/config/summary")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult GetAdminConfigSummary()
    {
        if (!_training.Enabled && !IsAdmin()) return Forbid();
        var middlewareUrl = _configuration["STONEHENGE_MIDDLEWARE_URL"] ?? "http://middleware:8080";
        return Ok(new
        {
            ok = true,
            route = "/api/admin/config/summary",
            panelIds = new[] { "admin-hmi", "admin-safety", "admin-reset-toolbox" },
            featureFlags = new
            {
                devAuth = _authenticationOptions.IsDev,
                dexOidc = _authenticationOptions.IsOidc,
                maintenanceControls = "middleware-protected",
                fuxaEmbedding = "pending"
            },
            middlewareBaseUrl = middlewareUrl
        });
    }

    [HttpGet("/api/admin/users/effective-access")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult GetEffectiveAccess()
    {
        if (!_training.Enabled && !IsAdmin()) return Forbid();
        return Ok(new
        {
            ok = true,
            route = "/api/admin/users/effective-access",
            currentUser = new
            {
                name = User.Identity?.Name,
                displayName = User.StonehengeDisplayName(),
                groups = User.Claims
                    .Where(claim => claim.Type == StonehengeAuth.GroupsClaim)
                    .Select(claim => claim.Value)
                    .ToArray(),
                policies = new
                {
                    dashboard = true,
                    engineering = User.HasStonehengeAccess(
                        StonehengeAuth.EngineeringAccessLevel,
                        StonehengeAuth.EngineeringAdminAccessLevel),
                    admin = User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel)
                }
            },
            knownDevUsers = _authenticationOptions.IsDev
                ? new object[]
                {
                    new { user = "employee.basic", groups = new[] { StonehengeAuth.EmployeesGroup } },
                    new { user = "engineering.user", groups = new[] { StonehengeAuth.EmployeesGroup, StonehengeAuth.EngineeringUsersGroup } },
                    new { user = "engineering.admin", groups = new[] { StonehengeAuth.EmployeesGroup, StonehengeAuth.EngineeringUsersGroup, StonehengeAuth.EngineeringAdminsGroup } },
                    new { user = "platform.admin", groups = new[] { StonehengeAuth.EmployeesGroup, "Domain Admins" }, engineeringMember = false }
                }
                : []
        });
    }

    [HttpGet("/api/admin/routes")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult GetAdminRoutes()
    {
        if (!_training.Enabled && !IsAdmin()) return Forbid();
        return Ok(new
        {
            ok = true,
            route = "/api/admin/routes",
            mvcRoutes = new[]
            {
                new { path = "/Engineering/Overview", policy = StonehengeAuth.DashboardAccessPolicy },
                new { path = "/Engineering/Auto", policy = StonehengeAuth.EngineeringAccessPolicy },
                new { path = "/Engineering/Manual", policy = StonehengeAuth.EngineeringAccessPolicy },
                new { path = "/Engineering/Maintenance", policy = StonehengeAuth.EngineeringAccessPolicy },
                new { path = "/Engineering/Admin", policy = StonehengeAuth.EngineeringAdminAccessPolicy }
            },
            readApiGaps = new[]
            {
                "/api/guns/{gunId}/state",
                "/api/guns/{gunId}/logs",
                "/api/manual/context",
                "/api/admin/audit",
                "/api/admin/config/summary",
                "/api/admin/users/effective-access"
            }
        });
    }

    private static bool ContainsGun(OperationLogEntry entry, string gunLabel)
    {
        return entry.Notes.Contains(gunLabel, StringComparison.OrdinalIgnoreCase)
            || entry.Action.Contains(gunLabel, StringComparison.OrdinalIgnoreCase)
            || entry.Source.Contains(gunLabel, StringComparison.OrdinalIgnoreCase);
    }

    [HttpGet("/api/panels/99/admin-diagnostics")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult GetAdminDiagnostics([FromHeader(Name = "X-Stonehenge-Panel-Token")] string? token)
    {
        var subject = User.FindFirst("sub")?.Value ?? User.FindFirst(System.Security.Claims.ClaimTypes.NameIdentifier)?.Value ?? User.StonehengeDisplayName();
        if (!_training.Enabled && !IsAdmin()) return Forbid();
        if (string.IsNullOrWhiteSpace(token) || !_panelCapabilities.Validate(token, 99, subject, out var expiresUtc))
            return Unauthorized(new { ok = false, error = "panel_capability_required" });

        var fields = new[] { "service", "environment", "authMode", "dispatcherOwner", "feedOwner", "plcTiming", "simulationTiming", "maintenanceShotThreshold" };
        _logger.LogWarning("Classified fields exposed Endpoint={Endpoint} Caller={Caller} FieldNames={FieldNames} Classification={Classification} FindingId={FindingId} CorrelationId={CorrelationId}",
            "/api/panels/99/admin-diagnostics", subject, fields, "TOP SECRET // FCU-OT", "FCU-ENG-004", HttpContext.TraceIdentifier);
        return Ok(new
        {
            ok = true,
            panelId = 99,
            panelType = "admin-diagnostics",
            capabilityExpiresUtc = expiresUtc,
            diagnostics = new
            {
                service = "stonehenge-engineering-console",
                environment = _environment.EnvironmentName,
                authMode = _authenticationOptions.Mode,
                dispatcherOwner = "middleware",
                feedOwner = "stonehenge-target-feed",
                plcTiming = new { scanMs = 20, settleSeconds = 5, pulseMs = 40, lockoutReloadSeconds = 10, azimuthStepDegreesPerScan = 1.0, elevationStepDegreesPerScan = 0.5, snapToleranceDegrees = 0.5, azimuthRateDegreesPerSecond = 50, elevationRateDegreesPerSecond = 25 },
                simulationTiming = new { tickMs = 250, slewDegreesPerSecond = 18, minSlewSeconds = 1, maxSlewSeconds = 12, settleSeconds = 5, lockoutSeconds = 10 },
                maintenanceShotThreshold = OperationalOptions.DefaultMaintenanceShotThreshold
            },
            classification = "TOP SECRET // FCU-OT",
            findings = new[] { "FCU-ENG-001", "FCU-ENG-002", "FCU-ENG-003", "FCU-ENG-004" }
        });
    }

    private bool IsAdmin() => User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel);
    private bool IsEngineering() => User.HasStonehengeAccess(StonehengeAuth.EngineeringAccessLevel, StonehengeAuth.EngineeringAdminAccessLevel);
    private bool IsSafeEmployee() => !_training.Enabled && !IsEngineering();

    private void AuditExposure(string endpoint, string findingId, IReadOnlyList<string> fields)
    {
        _logger.LogWarning("Classified fields exposed Endpoint={Endpoint} Caller={Caller} FieldNames={FieldNames} Classification={Classification} FindingId={FindingId} CorrelationId={CorrelationId}",
            endpoint, User.StonehengeDisplayName(), fields, "TOP SECRET // FCU-OT", findingId, HttpContext.TraceIdentifier);
    }

    private static string RuntimeEndpointForGun(int gunId)
    {
        var lastOctet = gunId == 1 ? 10 : gunId + 10;
        return $"OpenPLC Gun {gunId} 172.28.0.{lastOctet}:502";
    }

    private static string FuxaTagSourceForGun(int gunId)
    {
        var lastOctet = gunId == 1 ? 10 : gunId + 10;
        return $"FUXA tags 172.28.0.{lastOctet}:502";
    }

    private IEnumerable<EngineeringCommandRecord> CommandsForGun(int gunId)
    {
        return _commands.List().Where(command => command.GunNumber == gunId);
    }

    private int CommandHintForGun(int gunId)
    {
        return CommandsForGun(gunId).Select(command => command.Id).DefaultIfEmpty(1001).First();
    }

    private int RequestHintForGun(int gunId)
    {
        return _maintenanceRequests.List()
            .Where(request => request.GunNumber == gunId)
            .Select(request => request.Id)
            .DefaultIfEmpty(2001)
            .First();
    }

    private static object ToLogDto(OperationLogEntry entry)
    {
        return new
        {
            eventId = entry.Id,
            entry.Timestamp,
            entry.Source,
            entry.Action,
            entry.Result,
            entry.Notes
        };
    }

    private static object ToPanelDto(EngineeringPanelRecord panel)
    {
        return new
        {
            panelId = panel.PanelId,
            panelName = panel.PanelName,
            panelType = panel.PanelType,
            gunId = panel.GunId,
            requiredAccess = panel.RequiredAccess,
            fuxaViewId = panel.FuxaViewId,
            enabled = panel.Enabled
        };
    }

    private static object ToRequestBreadcrumb(MaintenanceRequestViewModel request)
    {
        return new
        {
            requestId = request.Id,
            requestKind = request.RequestKind,
            requestType = request.RequestSubType,
            request.GunNumber,
            gunId = request.GunNumber,
            request.GunName,
            legacyRequestType = request.RequestType,
            assetId = request.AssetId,
            panelId = request.PanelId,
            request.Status,
            request.RequestedBy,
            sourceApp = request.SourceApp,
            request.RequestedAt,
            createdUtc = request.RequestedAt.ToUniversalTime(),
            updatedUtc = (request.ReviewedAt ?? request.RequestedAt).ToUniversalTime(),
            request.ReviewedAt,
            request.ReviewedBy,
            request.Notes,
            request.ReviewNotes,
            safeRouteHint = request.SafeRouteHint
        };
    }

    private static object ToCommandDto(EngineeringCommandRecord command)
    {
        return new
        {
            commandId = command.Id,
            gunId = command.GunNumber,
            command.SourcePanelId,
            command.SubmittedBy,
            command.CommandType,
            command.Status,
            createdUtc = command.CreatedUtc,
            command.SanitizedPayload,
            command.RelatedMaintenanceRequestId
        };
    }

    private static int? ResolveGunId(int? gunId, string? assetId)
    {
        if (gunId is >= 1 and <= 8)
        {
            return gunId;
        }

        if (string.IsNullOrWhiteSpace(assetId))
        {
            return null;
        }

        var digits = new string(assetId.Where(char.IsDigit).ToArray());
        return int.TryParse(digits, out var parsed) && parsed is >= 1 and <= 8
            ? parsed
            : null;
    }

    private static string CleanAssetId(string? assetId, int gunId)
    {
        return string.IsNullOrWhiteSpace(assetId) ? $"GUN-{gunId}" : assetId.Trim();
    }

    private static string CleanRequestType(string? requestType, string fallback)
    {
        return string.IsNullOrWhiteSpace(requestType) ? fallback : requestType.Trim();
    }
}

public sealed record MaintenanceCommentRequest(string? Comment);
public sealed record SimpleMaintenanceRequestInput(string? RequestType, int? GunId, string? AssetId, int? PanelId);
public sealed record SimplePartRequestInput(int? GunId, string? AssetId, int? PanelId);
