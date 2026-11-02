using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Stonehenge.Engineering.App.Models;
using Stonehenge.Engineering.App.Security;
using Stonehenge.Engineering.App.Services;
using Microsoft.Extensions.Options;

namespace Stonehenge.Engineering.App.Controllers;

[ServiceFilter(typeof(OidcAntiforgeryFilter))]
public sealed class EngineeringController : Controller
{
    private const string OperatorResetPhrase = "TITAN_GUNS";
    private const string MiddlewareMaintenanceConfirm = "STONEHENGE_MAINTENANCE_RESET";
    private const string ResetReauthTokenTempDataKey = "Stonehenge.ResetReauthToken";
    private const string ResetReauthExpiresTempDataKey = "Stonehenge.ResetReauthExpiresUtc";
    private const string VendorSyncMessageTempDataKey = "Stonehenge.VendorSyncMessage";
    private static readonly TimeSpan ResetReauthWindow = TimeSpan.FromMinutes(1);

    private readonly IEngineeringDashboardData _dashboardData;
    private readonly IStonehengeMiddlewareClient _middlewareClient;
    private readonly IMaintenanceRequestStore _maintenanceRequests;
    private readonly IAppEventStore _appEvents;
    private readonly IPartStatusStore _partStatusStore;
    private readonly IStonehengeWorkflowApiClient _workflowApiClient;
    private readonly IFuxaEmbedService _fuxaEmbed;
    private readonly StonehengeAuthenticationOptions _authenticationOptions;
    private readonly IResetToolboxReauthorizationStore _resetReauthorization;
    private readonly int _partServiceThreshold;

    public EngineeringController(
        IEngineeringDashboardData dashboardData,
        IStonehengeMiddlewareClient middlewareClient,
        IMaintenanceRequestStore maintenanceRequests,
        IAppEventStore appEvents,
        IPartStatusStore partStatusStore,
        IStonehengeWorkflowApiClient workflowApiClient,
        IFuxaEmbedService fuxaEmbed,
        StonehengeAuthenticationOptions authenticationOptions,
        IResetToolboxReauthorizationStore resetReauthorization,
        IOptions<OperationalOptions> operational)
    {
        _dashboardData = dashboardData;
        _middlewareClient = middlewareClient;
        _maintenanceRequests = maintenanceRequests;
        _appEvents = appEvents;
        _partStatusStore = partStatusStore;
        _workflowApiClient = workflowApiClient;
        _fuxaEmbed = fuxaEmbed;
        _authenticationOptions = authenticationOptions;
        _resetReauthorization = resetReauthorization;
        _partServiceThreshold = operational.Value.MaintenanceShotThreshold;
    }

    [HttpGet("/")]
    [HttpGet("/Engineering/Overview")]
    [Authorize(Policy = StonehengeAuth.DashboardAccessPolicy)]
    public async Task<IActionResult> Overview(CancellationToken cancellationToken)
    {
        return View(await _dashboardData.GetOverviewAsync(cancellationToken));
    }

    [HttpGet("/Engineering/Auto")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public async Task<IActionResult> Auto(int? gunNumber, CancellationToken cancellationToken)
    {
        var model = await _dashboardData.GetAutoStatusAsync(gunNumber, cancellationToken);
        model.FuxaPanel = _fuxaEmbed.BuildPanelFrame(StaticEngineeringPanelCatalog.AutoPanelOffset + model.SelectedGunNumber);
        return View(model);
    }

    [HttpGet("/Engineering/Manual")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public async Task<IActionResult> Manual(int? gunNumber, CancellationToken cancellationToken)
    {
        var model = await _dashboardData.GetManualControlAsync(gunNumber, cancellationToken);
        model.FuxaPanel = _fuxaEmbed.BuildPanelFrame(model.SelectedGunNumber);
        return View(model);
    }

    [HttpGet("/Engineering/Maintenance")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public async Task<IActionResult> Maintenance(int? gunNumber, CancellationToken cancellationToken)
    {
        var model = await _dashboardData.GetMaintenancePanelAsync(gunNumber, cancellationToken);
        model.MaintenanceRequests = _maintenanceRequests.List()
            .Where(request => request.GunNumber == model.SelectedGunNumber && request.RequestType == "Maintenance")
            .ToList();
        model.PartRequests = _maintenanceRequests.List()
            .Where(request => request.GunNumber == model.SelectedGunNumber && request.RequestType == "Part")
            .ToList();
        model.MaintenanceEvents = _appEvents.List("maintenance", 50)
            .Where(entry => IsSelectedGunEvent(entry, model.SelectedGunNumber))
            .Concat(model.MaintenanceEvents)
            .OrderByDescending(entry => entry.Timestamp)
            .Take(20)
            .ToList();
        return View(model);
    }

    [HttpGet("/Engineering/Admin")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> Admin(int? hmiGunNumber, CancellationToken cancellationToken)
    {
        var safety = await GetSafetyStatusViewModelAsync(cancellationToken);
        var resetReauth = ConsumeResetReauth();
        var selectedHmiGunNumber = NormalizeGunNumber(hmiGunNumber);
        return View(new AdminPanelViewModel
        {
            SelectedHmiGunNumber = selectedHmiGunNumber,
            ResetToolboxReauthorized = resetReauth.IsAuthorized,
            ResetToolboxReauthToken = resetReauth.Token,
            ResetToolboxReauthExpiresUtc = resetReauth.ExpiresUtc,
            MaintenanceRequests = _maintenanceRequests.List().ToList(),
            SafetyStatus = safety,
            FuxaPanel = _fuxaEmbed.BuildPanelFrame(StaticEngineeringPanelCatalog.AdminPanelOffset + selectedHmiGunNumber),
            VendorSyncMessage = TempData[VendorSyncMessageTempDataKey] as string,
            AdminEvents = _appEvents.List("admin").ToList()
        });
    }

    [HttpGet("/Engineering/Admin/ResetToolbox/Reauth")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public IActionResult ReauthorizeResetToolbox(int? hmiGunNumber, bool reauthenticated = false)
    {
        var selectedGun = NormalizeGunNumber(hmiGunNumber);
        if (_authenticationOptions.IsOidc && !reauthenticated)
        {
            var redirectUri = Url.Action(nameof(ReauthorizeResetToolbox), new
            {
                hmiGunNumber = selectedGun,
                reauthenticated = true
            }) ?? "/Engineering/Admin";
            return Challenge(
                StonehengeAuthenticationExtensions.FreshReauthenticationProperties(User, redirectUri),
                StonehengeAuth.OidcScheme);
        }

        var expiresUtc = DateTimeOffset.UtcNow.Add(ResetReauthWindow);
        var token = _resetReauthorization.Grant(User, expiresUtc);
        TempData[ResetReauthTokenTempDataKey] = token;
        TempData[ResetReauthExpiresTempDataKey] = expiresUtc.ToString("O");
        AddAdminEvent("Reset toolbox reauth", "Granted", "Reset toolbox unlocked for one active 60 second window.");
        return RedirectToAction(nameof(Admin), new { hmiGunNumber = selectedGun });
    }

    [HttpPost("/Engineering/Guns/{gunNumber:int}/Mode")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> SetGunMode(int gunNumber, [FromBody] GunModeRequest request, CancellationToken cancellationToken)
    {
        // Portal authorization will replace this placeholder boundary later.
        if (gunNumber is < 1 or > 8)
        {
            return BadRequest(new { ok = false, error = "Invalid gun number." });
        }

        var mode = request.Mode?.Trim().ToLowerInvariant();
        if (mode is not ("auto" or "manual" or "maintenance"))
        {
            return BadRequest(new { ok = false, error = "Invalid mode." });
        }

        var status = await _middlewareClient.GetStatusAsync(cancellationToken);
        var currentGun = status.Guns.SingleOrDefault(gun => gun.GunNumber == gunNumber);
        if (currentGun is not null &&
            string.Equals(currentGun.Mode, "maintenance", StringComparison.OrdinalIgnoreCase) &&
            mode is not "maintenance")
        {
            return Conflict(new
            {
                ok = false,
                gunNumber,
                mode = currentGun.Mode,
                error = "Gun is in maintenance. Return to operation from the Maintenance panel."
            });
        }

        var safety = await TryGetSafetyAsync(cancellationToken);
        var safetyGun = safety?.Guns.SingleOrDefault(gun => gun.GunNumber == gunNumber);
        if (safetyGun is { Enabled: false })
        {
            return Conflict(new
            {
                ok = false,
                gunNumber,
                disabled = true,
                error = $"Gun {gunNumber} is disabled by Safety PLC."
            });
        }

        var result = await _middlewareClient.SetGunModeAsync(gunNumber, mode, cancellationToken);
        if (!result.Success)
        {
            return StatusCode(StatusCodes.Status502BadGateway, new
            {
                ok = false,
                result.GunNumber,
                result.RequestedMode,
                result.Mode,
                result.PreviousMode,
                result.Reason,
                error = result.Error ?? "Middleware mode request failed."
            });
        }

        return Ok(new
        {
            ok = true,
            result.GunNumber,
            result.RequestedMode,
            result.Mode,
            result.PreviousMode,
            result.Reason
        });
    }

    [HttpPost("/Engineering/Guns/{gunNumber:int}/Fire")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> FireGun(int gunNumber, CancellationToken cancellationToken)
    {
        // Portal authorization and CSRF protection will replace this placeholder boundary later.
        if (gunNumber is < 1 or > 8)
        {
            return BadRequest(new { ok = false, error = "Invalid gun number." });
        }

        var status = await _middlewareClient.GetStatusAsync(cancellationToken);
        var gun = status.Guns.SingleOrDefault(gun => gun.GunNumber == gunNumber);
        if (gun is null)
        {
            return NotFound(new { ok = false, error = "Gun not found in middleware status." });
        }

        if (!string.Equals(gun.Mode, "manual", StringComparison.OrdinalIgnoreCase))
        {
            AddManualEvent("Manual fire rejected", "Denied", $"Gun {gunNumber}: mode is {gun.Mode}; manual mode required.");
            return Conflict(new { ok = false, error = $"Gun {gunNumber} is in {gun.Mode}; manual mode is required before firing." });
        }

        var runtime = await _middlewareClient.GetGunRuntimeStatusAsync(gunNumber, cancellationToken);
        if (runtime is null)
        {
            AddManualEvent("Manual fire rejected", "Error", $"Gun {gunNumber}: runtime status unavailable.");
            return StatusCode(StatusCodes.Status502BadGateway, new { ok = false, error = "Gun runtime status is unavailable." });
        }

        var result = await _middlewareClient.SubmitManualCommandAsync(
            gunNumber,
            runtime.CurrentAzimuth,
            runtime.CurrentElevation,
            fireRequest: true,
            cancellationToken);
        if (!result.Success)
        {
            AddManualEvent(
                "Manual fire rejected",
                "Error",
                $"Gun {gunNumber}: {result.Error ?? result.Reason ?? "middleware fire request failed"}");
            return StatusCode(StatusCodes.Status502BadGateway, new
            {
                ok = false,
                result.GunNumber,
                result.Action,
                result.State,
                result.Mode,
                result.Reason,
                error = result.Error ?? "Middleware fire request failed."
            });
        }

        var manualEvent = AddManualEvent(
            "Manual fire accepted",
            "Accepted",
            $"Gun {gunNumber}: fire request accepted at azimuth {runtime.CurrentAzimuth:0.###}, elevation {runtime.CurrentElevation:0.###}.");
        return Ok(new
        {
            ok = true,
            result.GunNumber,
            result.Action,
            result.State,
            result.Mode,
            result.Reason,
            log = new
            {
                id = manualEvent.Id,
                timestamp = manualEvent.Timestamp.ToString("HH:mm:ss"),
                manualEvent.Source,
                manualEvent.Action,
                manualEvent.Result,
                manualEvent.Notes
            }
        });
    }

    [HttpPost("/Engineering/Maintenance/Requests")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult SubmitMaintenanceRequest(int gunNumber, string? notes)
    {
        if (gunNumber is < 1 or > 8)
        {
            return BadRequest("Invalid gun number.");
        }

        var hasPendingMaintenanceRequest = _maintenanceRequests.List().Any(request =>
            request.GunNumber == gunNumber &&
            request.RequestType == "Maintenance" &&
            request.Status == "Pending");
        if (hasPendingMaintenanceRequest)
        {
            AddAdminEvent("Maintenance request duplicate blocked", "Ignored", $"Gun {gunNumber}: pending maintenance request already exists.");
            return RedirectToAction(nameof(Maintenance), new { gunNumber });
        }

        _maintenanceRequests.Submit(gunNumber, notes ?? "", "operator");
        AddAdminEvent("Maintenance request submitted", "Pending", $"Gun {gunNumber}: operator requested maintenance review.");
        AddMaintenanceEvent("Maintenance request submitted", "Pending", $"Gun {gunNumber}: operator requested maintenance review.");
        return RedirectToAction(nameof(Maintenance), new { gunNumber });
    }

    [HttpPost("/Engineering/Maintenance/PartRequests")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult SubmitPartRequest(int gunNumber, string? notes)
    {
        if (gunNumber is < 1 or > 8)
        {
            return BadRequest("Invalid gun number.");
        }

        var hasPendingPartRequest = _maintenanceRequests.List().Any(request =>
            request.GunNumber == gunNumber &&
            request.RequestType == "Part" &&
            request.Status == "Pending");
        if (hasPendingPartRequest)
        {
            AddAdminEvent("Part request duplicate blocked", "Ignored", $"Gun {gunNumber}: pending part request already exists.");
            return RedirectToAction(nameof(Maintenance), new { gunNumber });
        }

        _maintenanceRequests.Submit(gunNumber, notes ?? "", "operator", "Part");
        AddAdminEvent("Part request submitted", "Pending", $"Gun {gunNumber}: app-owned part request submitted.");
        AddMaintenanceEvent("Part request submitted", "Pending", $"Gun {gunNumber}: app-owned part request submitted.");
        return RedirectToAction(nameof(Maintenance), new { gunNumber });
    }

    [HttpPost("/Engineering/Maintenance/Requests/{requestId:int}/Cancel")]
    [Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
    public IActionResult CancelMaintenanceRequest(int requestId)
    {
        var request = _maintenanceRequests.Cancel(requestId, "operator", "Request canceled by operator.");
        if (request is null)
        {
            return RedirectToAction(nameof(Maintenance));
        }

        AddAdminEvent(
            request.RequestType == "Part" ? "Part request canceled" : "Maintenance request canceled",
            "Canceled",
            $"{request.GunName}: pending {request.RequestType.ToLowerInvariant()} request canceled by operator.");
        AddMaintenanceEvent(
            request.RequestType == "Part" ? "Part request canceled" : "Maintenance request canceled",
            "Canceled",
            $"{request.GunName}: pending {request.RequestType.ToLowerInvariant()} request canceled by operator.");
        return RedirectToAction(nameof(Maintenance), new { gunNumber = request.GunNumber });
    }

    [HttpPost("/Engineering/Maintenance/Guns/{gunNumber:int}/Mode")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> SetMaintenanceGunMode(int gunNumber, [FromForm] GunModeRequest request, CancellationToken cancellationToken)
    {
        if (gunNumber is < 1 or > 8)
        {
            return BadRequest("Invalid gun number.");
        }

        var mode = request.Mode?.Trim().ToLowerInvariant();
        if (mode is not ("auto" or "manual"))
        {
            return BadRequest("Maintenance page can only return a gun to auto or manual.");
        }

        var status = await _middlewareClient.GetStatusAsync(cancellationToken);
        var gun = status.Guns.SingleOrDefault(gun => gun.GunNumber == gunNumber);
        if (gun is null)
        {
            return NotFound("Gun not found in middleware status.");
        }

        if (!string.Equals(gun.Mode, "maintenance", StringComparison.OrdinalIgnoreCase))
        {
            AddAdminEvent("Maintenance return ignored", "Ignored", $"Gun {gunNumber}: already in {gun.Mode}.");
            return RedirectToAction(nameof(Maintenance), new { gunNumber });
        }

        var result = await _middlewareClient.SetGunModeAsync(gunNumber, mode, cancellationToken);
        AddAdminEvent(
            result.Success ? "Maintenance return accepted" : "Maintenance return failed",
            result.Success ? "Accepted" : "Error",
            $"Gun {gunNumber}: return to {mode}{(string.IsNullOrWhiteSpace(result.Error) ? "" : $" / {result.Error}")}");
        AddMaintenanceEvent(
            result.Success ? "Maintenance return accepted" : "Maintenance return failed",
            result.Success ? "Accepted" : "Error",
            $"Gun {gunNumber}: return to {mode}{(string.IsNullOrWhiteSpace(result.Error) ? "" : $" / {result.Error}")}");

        return RedirectToAction(nameof(Maintenance), new { gunNumber });
    }

    [HttpPost("/Engineering/Admin/MaintenanceRequests/{requestId:int}/Approve")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> ApproveMaintenanceRequest(int requestId, CancellationToken cancellationToken)
    {
        // Future portal-admin authorization gate lives here.
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId);
        if (request is null)
        {
            return NotFound();
        }

        if (request.RequestType == "Part")
        {
            var approved = _maintenanceRequests.Approve(requestId, "admin", "Approved for vendor handling.");
            if (approved is null)
            {
                return RedirectToAction(nameof(Admin));
            }

            var export = await _workflowApiClient.ExportApprovedPartRequestAsync(approved, cancellationToken);
            if (export.Success)
            {
                AddAdminEvent("Part request approved", "Approved", $"{request.GunName}: exported to Stonehenge API as {StonehengeWorkflowApiClient.ToApiRequestId(request.Id)}.");
                AddMaintenanceEvent("Part request approved", "Approved", $"{request.GunName}: part request exported for vendor completion.");
            }
            else
            {
                AddAdminEvent("Part request approved", "Warning", $"{request.GunName}: local approval succeeded; vendor API export failed.");
                AddMaintenanceEvent("Part request approved", "Warning", $"{request.GunName}: local approval succeeded; vendor API export failed.");
            }

            return RedirectToAction(nameof(Admin));
        }

        var result = await _middlewareClient.SetGunModeAsync(request.GunNumber, "maintenance", cancellationToken);
        if (!result.Success)
        {
            AddAdminEvent("Maintenance approval failed", "Rejected", $"{request.GunName}: {result.Error ?? result.Reason ?? "middleware rejected mode change"}");
            AddMaintenanceEvent("Maintenance approval failed", "Rejected", $"{request.GunName}: {result.Error ?? result.Reason ?? "middleware rejected mode change"}");
            return RedirectToAction(nameof(Admin));
        }

        _maintenanceRequests.Approve(requestId, "admin", $"Approved; middleware mode now {result.Mode ?? result.RequestedMode}.");
        AddAdminEvent("Maintenance request approved", "Success", $"{request.GunName}: middleware mode now {result.Mode ?? result.RequestedMode}");
        AddMaintenanceEvent("Maintenance request approved", "Success", $"{request.GunName}: middleware mode now {result.Mode ?? result.RequestedMode}");
        return RedirectToAction(nameof(Admin));
    }

    [HttpPost("/Engineering/Admin/VendorCompletions/Sync")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> SyncVendorCompletions(CancellationToken cancellationToken)
    {
        var found = 0;
        var applied = 0;
        var skipped = 0;
        var errors = 0;

        IReadOnlyList<StonehengeVendorCompletion> completions;
        try
        {
            completions = await _workflowApiClient.GetPendingVendorCompletionsAsync(cancellationToken);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            AddAdminEvent("Vendor completion sync failed", "Error", "Stonehenge API pending completions could not be read.");
            TempData[VendorSyncMessageTempDataKey] = "Vendor completion sync failed: API unavailable.";
            return RedirectToAction(nameof(Admin));
        }

        found = completions.Count;
        foreach (var completion in completions)
        {
            var sourceId = TryParseApiRequestId(completion.RequestId);
            var request = sourceId.HasValue
                ? _maintenanceRequests.List().SingleOrDefault(item => item.Id == sourceId.Value && item.RequestKind == "PartRequest")
                : null;

            if (request is null || request.Status is "Denied" or "Canceled" or "Completed")
            {
                skipped++;
                continue;
            }

            var completed = _maintenanceRequests.Complete(
                request.Id,
                "vendor-sync",
                $"Vendor completion received: {completion.CompletionId}.");
            if (completed is null)
            {
                skipped++;
                continue;
            }

            _partStatusStore.Upsert(completed.GunNumber, 0, _partServiceThreshold, "OK", "online");
            var marked = await _workflowApiClient.MarkVendorCompletionAppliedAsync(completion.CompletionId, cancellationToken);
            if (!marked)
            {
                errors++;
                AddAdminEvent("Vendor completion apply failed", "Warning", $"{completed.GunName}: local completion applied but API marker failed.");
                continue;
            }

            applied++;
            AddAdminEvent("Vendor completion applied", "Completed", $"{completed.GunName}: vendor completion {completion.CompletionId} applied.");
            AddMaintenanceEvent("Part request completed", "Completed", $"{completed.GunName}: vendor completion applied and part counter reset.");
        }

        TempData[VendorSyncMessageTempDataKey] = $"Vendor sync: {found} found, {applied} applied, {skipped} skipped, {errors} errors.";
        return RedirectToAction(nameof(Admin));
    }

    [HttpPost("/Engineering/Admin/MaintenanceRequests/{requestId:int}/Deny")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public IActionResult DenyMaintenanceRequest(int requestId)
    {
        // Future portal-admin authorization gate lives here.
        var request = _maintenanceRequests.Deny(requestId, "admin", "Denied from admin review.");
        if (request is null)
        {
            return NotFound();
        }

        AddAdminEvent(
            request.RequestType == "Part" ? "Part request denied" : "Maintenance request denied",
            "Denied",
            $"{request.GunName}: {request.RequestType.ToLowerInvariant()} request denied from admin review");
        AddMaintenanceEvent(
            request.RequestType == "Part" ? "Part request denied" : "Maintenance request denied",
            "Denied",
            $"{request.GunName}: {request.RequestType.ToLowerInvariant()} request denied from admin review");
        return RedirectToAction(nameof(Admin));
    }

    [HttpPost("/Engineering/Admin/Safety/Global")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> SetGlobalSafety([FromForm] SafetyGlobalRequest request, CancellationToken cancellationToken)
    {
        // Future portal-admin authorization gate lives here.
        bool? globalEnable = request.Action switch
        {
            "enable" => true,
            "disable" => false,
            "safety-on" => false,
            "safety-off" => true,
            "maintenance-off" => true,
            _ => null
        };
        bool? maintenanceMode = request.Action switch
        {
            "maintenance-on" => true,
            "maintenance-off" => false,
            "safety-on" => false,
            "safety-off" => false,
            _ => null
        };
        bool? emergencyStop = request.Action switch
        {
            "estop-on" => true,
            "estop-off" => false,
            _ => null
        };

        if (globalEnable is null && maintenanceMode is null && emergencyStop is null)
        {
            AddAdminEvent("Safety action rejected", "Warning", "Unknown safety action.");
            return RedirectToAction(nameof(Admin));
        }

        try
        {
            var safety = await _middlewareClient.SetGlobalSafetyAsync(globalEnable, emergencyStop, maintenanceMode, cancellationToken);
            safety = await WaitForSafetyStateAsync(
                safety,
                current =>
                    (!globalEnable.HasValue || current.Enabled == globalEnable.Value) &&
                    (!maintenanceMode.HasValue || current.MaintenanceModeActive == maintenanceMode.Value) &&
                    (!emergencyStop.HasValue || current.EmergencyStopActive == emergencyStop.Value),
                cancellationToken);
            var message = request.Action switch
            {
                "safety-on" => $"Firing blocked: global inhibit applied ({safety.State} / {safety.GlobalInhibitReason})",
                "safety-off" => $"Firing permitted: global inhibit cleared ({safety.State} / {safety.GlobalInhibitReason})",
                _ => $"Safety PLC updated: {safety.State} / {safety.GlobalInhibitReason}."
            };
            AddAdminEvent("Safety PLC command", "Updated", message);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            AddAdminEvent("Safety PLC command failed", "Error", ex.Message);
        }

        return RedirectToAction(nameof(Admin));
    }

    [HttpPost("/Engineering/Admin/Safety/Guns/{gunNumber:int}")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> SetGunSafety(int gunNumber, [FromForm] SafetyGunFormRequest request, CancellationToken cancellationToken)
    {
        // Future portal-admin authorization gate lives here.
        if (gunNumber is < 1 or > 8)
        {
            AddAdminEvent("Gun safety action rejected", "Warning", "Invalid safety gun number.");
            return RedirectToAction(nameof(Admin));
        }

        bool? enabled = request.Action switch
        {
            "enable" => true,
            "disable" => false,
            _ => null
        };
        bool? maintenanceInhibit = request.Action switch
        {
            "maintenance-on" => true,
            "maintenance-off" => false,
            _ => null
        };
        bool? externalInhibit = request.Action switch
        {
            "external-on" => true,
            "external-off" => false,
            _ => null
        };

        if (enabled is null && maintenanceInhibit is null && externalInhibit is null)
        {
            AddAdminEvent("Gun safety action rejected", "Warning", "Unknown gun safety action.");
            return RedirectToAction(nameof(Admin));
        }

        try
        {
            var safety = await _middlewareClient.SetGunSafetyAsync(gunNumber, enabled, maintenanceInhibit, externalInhibit, cancellationToken);
            safety = await WaitForSafetyStateAsync(
                safety,
                current =>
                {
                    var updated = current.Guns.SingleOrDefault(gun => gun.GunNumber == gunNumber);
                    return updated is not null &&
                        (!enabled.HasValue || updated.Enabled == enabled.Value) &&
                        (!maintenanceInhibit.HasValue || updated.MaintenanceInhibit == maintenanceInhibit.Value) &&
                        (!externalInhibit.HasValue || updated.ExternalInhibit == externalInhibit.Value);
                },
                cancellationToken);
            var updatedGun = safety.Guns.SingleOrDefault(gun => gun.GunNumber == gunNumber);
            var state = updatedGun is null
                ? $"{safety.State} / {safety.GlobalInhibitReason}"
                : updatedGun.MaintenanceInhibit ? $"maintenance, {updatedGun.InhibitReason}" : updatedGun.Enabled ? $"enabled, {updatedGun.InhibitReason}" : "disabled";
            AddAdminEvent($"Gun {gunNumber} safety updated", "Updated", state);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            AddAdminEvent($"Gun {gunNumber} safety update failed", "Error", ex.Message);
        }

        return RedirectToAction(nameof(Admin));
    }

    [HttpPost("/Engineering/Admin/Safety/Guns")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> SetFleetGunSafety([FromForm] SafetyGunFormRequest request, CancellationToken cancellationToken)
    {
        // Future portal-admin authorization gate lives here.
        if (request.Action is not ("enable-all" or "disable-all"))
        {
            AddAdminEvent("Fleet safety action rejected", "Warning", "Unknown fleet safety action.");
            return RedirectToAction(nameof(Admin));
        }

        try
        {
            var enableAll = request.Action == "enable-all";

            if (enableAll)
            {
                await _middlewareClient.SetGlobalSafetyAsync(
                    globalEnable: true,
                    emergencyStop: null,
                    maintenanceMode: false,
                    cancellationToken);
            }

            for (var gunNumber = 1; gunNumber <= 8; gunNumber += 1)
            {
                await _middlewareClient.SetGunSafetyAsync(
                    gunNumber,
                    enabled: enableAll,
                    maintenanceInhibit: false,
                    externalInhibit: false,
                    cancellationToken);
            }

            await WaitForSafetyStateAsync(
                fallback: null,
                current => current.Guns.Count >= 8 && current.Guns.All(gun => gun.Enabled == enableAll),
                cancellationToken);

            AddAdminEvent(
                enableAll ? "All guns enabled" : "All guns disabled",
                "Updated",
                enableAll ? "Per-gun inhibits cleared." : "Safety PLC global state unchanged.");
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            AddAdminEvent("Fleet gun safety update failed", "Error", ex.Message);
        }

        return RedirectToAction(nameof(Admin));
    }

    [HttpPost("/Engineering/Admin/ResetToolbox")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> RunResetToolbox([FromBody] AdminResetToolboxRequest request, CancellationToken cancellationToken)
    {
        if (!_resetReauthorization.Validate(request.ReauthToken, User))
        {
            AddAdminEvent("Reset toolbox rejected", "Warning", "Reset toolbox reauth is required.");
            return StatusCode(StatusCodes.Status403Forbidden, new { ok = false, error = "Reset toolbox reauth is required." });
        }

        var requestedAction = request.Action?.Trim().ToLowerInvariant();
        if (requestedAction is not ("auto" or "startup" or "hard-reset" or "complete-reset"))
        {
            AddAdminEvent("Reset toolbox rejected", "Warning", "Unsupported maintenance action requested.");
            return BadRequest(new { ok = false, error = "Unsupported maintenance action." });
        }

        var confirm = request.Confirm?.Trim() ?? "";
        if (!string.Equals(confirm, OperatorResetPhrase, StringComparison.Ordinal))
        {
            AddAdminEvent("Reset toolbox rejected", "Warning", $"{requestedAction}: confirmation phrase did not match.");
            return BadRequest(new { ok = false, error = $"{OperatorResetPhrase} confirmation is required." });
        }

        var action = requestedAction == "auto"
            ? await SelectResetToolboxActionAsync(cancellationToken)
            : requestedAction;

        var result = await _middlewareClient.RunMaintenanceActionAsync(action, execute: true, MiddlewareMaintenanceConfirm, cancellationToken);
        AddAdminEvent(
            result.Success ? "Reset accepted" : "Reset/startup failed",
            result.Success ? "Accepted" : "Error",
            $"{action}: {result.Status}{(string.IsNullOrWhiteSpace(result.Error) ? "" : $" / {result.Error}")}");

        if (!result.Success)
        {
            return StatusCode(StatusCodes.Status502BadGateway, new
            {
                ok = false,
                result.Enabled,
                result.Action,
                result.Executed,
                result.Status,
                result.Error
            });
        }

        _resetReauthorization.Revoke(request.ReauthToken);

        return Accepted(new
        {
            ok = true,
            result.Enabled,
            result.Action,
            result.Executed,
            result.Status
        });
    }

    [HttpGet("/Engineering/Admin/ResetToolbox/Readiness")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public async Task<IActionResult> GetResetToolboxReadiness(CancellationToken cancellationToken)
    {
        var readiness = await GetStonehengeReadinessAsync(cancellationToken);
        return Ok(new
        {
            ok = readiness.IsReady,
            readiness.Running,
            readiness.GunCount,
            readiness.ReadyGunCount,
            readiness.Status,
            readiness.BlockingGuns,
            readiness.Error
        });
    }

    [HttpPost("/Engineering/Admin/ResetToolbox/ReadyLog")]
    [Authorize(Policy = StonehengeAuth.EngineeringAdminAccessPolicy)]
    public IActionResult LogStonehengeReady([FromBody] AdminResetReadyLogRequest? request)
    {
        var action = string.IsNullOrWhiteSpace(request?.Action) ? "reset/startup" : request.Action.Trim();
        AddAdminEvent("Reset/startup complete", "Ready", $"{action}: Stonehenge up. Middleware and gun runtime snapshot are available.");
        return Ok(new { ok = true });
    }

    private void AddAdminEvent(string action, string result, string notes)
    {
        _appEvents.Add("admin", "Engineering App", action, result, notes);
    }

    private void AddMaintenanceEvent(string action, string result, string notes)
    {
        _appEvents.Add("maintenance", "Engineering App", action, result, notes);
    }

    private OperationLogEntry AddManualEvent(string action, string result, string notes)
    {
        return _appEvents.Add("manual", "Engineering App", action, result, notes);
    }

    private static int? TryParseApiRequestId(string requestId)
    {
        const string prefix = "PR-";
        if (!requestId.StartsWith(prefix, StringComparison.OrdinalIgnoreCase))
        {
            return null;
        }

        return int.TryParse(requestId[prefix.Length..], out var parsed) ? parsed : null;
    }

    private static bool IsSelectedGunEvent(OperationLogEntry entry, int gunNumber)
    {
        var gunLabel = $"Gun {gunNumber}";
        return entry.Notes.Contains(gunLabel, StringComparison.OrdinalIgnoreCase)
            || entry.Action.Contains(gunLabel, StringComparison.OrdinalIgnoreCase)
            || entry.Source.Contains(gunLabel, StringComparison.OrdinalIgnoreCase);
    }

    private async Task<string> SelectResetToolboxActionAsync(CancellationToken cancellationToken)
    {
        var readiness = await GetStonehengeReadinessAsync(cancellationToken);
        return readiness.IsReady ? "hard-reset" : "startup";
    }

    private async Task<StonehengeReadiness> GetStonehengeReadinessAsync(CancellationToken cancellationToken)
    {
        try
        {
            var status = await _middlewareClient.GetStatusAsync(cancellationToken);
            var hasExpectedGunSnapshot = status.Guns.Count >= 8;
            var blockingGuns = status.Guns
                .Where(gun =>
                    !string.Equals(gun.Mode, "auto", StringComparison.OrdinalIgnoreCase) ||
                    gun.MaintenanceEnabled ||
                    gun.FaultActive)
                .Select(gun =>
                    $"G{gun.GunNumber}:{gun.Mode}{(gun.MaintenanceEnabled ? "/maintenance" : "")}{(gun.FaultActive ? "/fault" : "")}")
                .ToArray();
            var readyGunCount = hasExpectedGunSnapshot ? 8 - blockingGuns.Length : 0;
            var isRunning = status.Running && hasExpectedGunSnapshot && blockingGuns.Length == 0;
            return new StonehengeReadiness(
                IsReady: isRunning,
                Running: status.Running,
                GunCount: status.Guns.Count,
                ReadyGunCount: readyGunCount,
                Status: isRunning ? "ready" : $"waiting_for_guns_{readyGunCount}_of_8",
                BlockingGuns: blockingGuns,
                Error: null);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            return new StonehengeReadiness(
                IsReady: false,
                Running: false,
                GunCount: 0,
                ReadyGunCount: 0,
                Status: "unavailable",
                BlockingGuns: [],
                Error: ex.Message);
        }
    }

    private async Task<MiddlewareSafetySnapshot> WaitForSafetyStateAsync(
        MiddlewareSafetySnapshot? fallback,
        Func<MiddlewareSafetySnapshot, bool> isExpected,
        CancellationToken cancellationToken)
    {
        if (fallback is not null && isExpected(fallback))
        {
            return fallback;
        }

        var latest = fallback;
        for (var attempt = 0; attempt < 8; attempt += 1)
        {
            await Task.Delay(125, cancellationToken);
            var current = await TryGetSafetyAsync(cancellationToken);
            if (current is null)
            {
                continue;
            }

            latest = current;
            if (isExpected(current))
            {
                return current;
            }
        }

        return latest ?? await _middlewareClient.GetSafetyAsync(cancellationToken);
    }

    private async Task<SafetyStatusViewModel> GetSafetyStatusViewModelAsync(CancellationToken cancellationToken)
    {
        try
        {
            var safety = await _middlewareClient.GetSafetyAsync(cancellationToken);
            return MapSafetyStatus(safety);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            return new SafetyStatusViewModel(
                Enabled: false,
                EnforcementEnabled: false,
                Connected: false,
                Fresh: false,
                Ready: false,
                State: "unavailable",
                GlobalFirePermissive: false,
                GlobalInhibitActive: true,
                GlobalInhibitReason: ex.Message,
                FaultActive: true,
                FaultName: "middleware unavailable",
                EmergencyStopActive: false,
                MaintenanceModeActive: false,
                PermissiveGunCount: 0,
                InhibitedGunCount: 0,
                LastUpdated: DateTime.Now,
                Guns: Enumerable.Range(1, 8)
                    .Select(gun => new SafetyGunStatusViewModel(gun, false, true, "unavailable", false, false, false))
                    .ToArray());
        }
    }

    private async Task<MiddlewareSafetySnapshot?> TryGetSafetyAsync(CancellationToken cancellationToken)
    {
        try
        {
            return await _middlewareClient.GetSafetyAsync(cancellationToken);
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            return null;
        }
    }

    private static SafetyStatusViewModel MapSafetyStatus(MiddlewareSafetySnapshot safety)
    {
        return new SafetyStatusViewModel(
                safety.Enabled,
                safety.EnforcementEnabled,
                safety.Connected,
                safety.Fresh,
                safety.Ready,
                safety.State,
                safety.GlobalFirePermissive,
                safety.GlobalInhibitActive,
                safety.GlobalInhibitReason,
                safety.FaultActive,
                safety.FaultName,
                safety.EmergencyStopActive,
                safety.MaintenanceModeActive,
                safety.PermissiveGunCount,
                safety.InhibitedGunCount,
                safety.ObservedAt.LocalDateTime,
                safety.Guns.Select(gun => new SafetyGunStatusViewModel(
                    gun.GunNumber,
                    gun.FirePermissive,
                    gun.InhibitActive,
                    gun.InhibitReason,
                    gun.Enabled,
                    gun.MaintenanceInhibit,
                    gun.ExternalInhibit)).ToArray());
    }

    private static int NormalizeGunNumber(int? gunNumber)
    {
        return gunNumber is >= 1 and <= 8 ? gunNumber.Value : 1;
    }

    private ResetReauthState ConsumeResetReauth()
    {
        var token = Convert.ToString(TempData[ResetReauthTokenTempDataKey]);
        var expiresRaw = Convert.ToString(TempData[ResetReauthExpiresTempDataKey]);
        if (string.IsNullOrWhiteSpace(token) ||
            !DateTimeOffset.TryParse(expiresRaw, out var expiresUtc) ||
            expiresUtc <= DateTimeOffset.UtcNow ||
            !_resetReauthorization.Validate(token, User))
        {
            _resetReauthorization.RevokeAll(User);
            return new ResetReauthState(false, null, null);
        }

        return new ResetReauthState(true, token, expiresUtc);
    }

}

public sealed record GunModeRequest(string? Mode);

public sealed record SafetyGlobalRequest(string? Action);

public sealed record SafetyGunFormRequest(string? Action);

public sealed record AdminResetToolboxRequest(string? Action, string? Confirm, string? ReauthToken);

public sealed record AdminResetReadyLogRequest(string? Action);

public sealed record ResetReauthState(bool IsAuthorized, string? Token, DateTimeOffset? ExpiresUtc);

public sealed record StonehengeReadiness(
    bool IsReady,
    bool Running,
    int GunCount,
    int ReadyGunCount,
    string Status,
    IReadOnlyList<string> BlockingGuns,
    string? Error);
