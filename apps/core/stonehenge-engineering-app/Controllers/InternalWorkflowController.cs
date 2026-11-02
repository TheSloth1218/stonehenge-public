using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Stonehenge.Engineering.App.Models;
using Stonehenge.Engineering.App.Security;
using Stonehenge.Engineering.App.Services;

namespace Stonehenge.Engineering.App.Controllers;

[ApiController]
[Authorize(Policy = StonehengeAuth.InternalWorkflowAccessPolicy)]
[Route("internal/workflow")]
public sealed class InternalWorkflowController : ControllerBase
{
    private readonly IMaintenanceRequestStore _maintenanceRequests;
    private readonly IAppEventStore _appEvents;

    public InternalWorkflowController(
        IMaintenanceRequestStore maintenanceRequests,
        IAppEventStore appEvents)
    {
        _maintenanceRequests = maintenanceRequests;
        _appEvents = appEvents;
    }

    [HttpGet("health")]
    public IActionResult Health()
    {
        _appEvents.Add(
            "admin",
            "Internal Workflow API",
            "Internal workflow health read",
            "Read",
            "Service-to-service workflow health checked.");

        return Ok(new
        {
            ok = true,
            service = "stonehenge-engineering-workflow",
            routes = new[]
            {
                "/internal/workflow/maintenance-requests",
                "/internal/workflow/part-requests"
            },
            storage = "sqlite",
            middlewareProxy = false
        });
    }

    [HttpGet("maintenance-requests")]
    public IActionResult ListMaintenanceRequests([FromQuery] string? type = null, [FromQuery] string? status = null)
    {
        var requests = _maintenanceRequests.List().AsEnumerable();
        if (!string.IsNullOrWhiteSpace(type))
        {
            requests = requests.Where(request =>
                string.Equals(request.RequestType, type, StringComparison.OrdinalIgnoreCase)
                || string.Equals(request.RequestKind, type, StringComparison.OrdinalIgnoreCase)
                || string.Equals(request.RequestSubType, type, StringComparison.OrdinalIgnoreCase));
        }

        if (!string.IsNullOrWhiteSpace(status))
        {
            requests = requests.Where(request => string.Equals(request.Status, status, StringComparison.OrdinalIgnoreCase));
        }

        var items = requests.Select(ToWorkflowDto).ToArray();
        _appEvents.Add(
            "admin",
            "Internal Workflow API",
            "Internal maintenance requests listed",
            "Read",
            $"items={items.Length}; type={Clean(type, "any")}; status={Clean(status, "any")}");

        return Ok(new
        {
            ok = true,
            items
        });
    }

    [HttpGet("maintenance-requests/{requestId:int}")]
    public IActionResult GetMaintenanceRequest(int requestId)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId);
        if (request is not null)
        {
            _appEvents.Add(
                "admin",
                "Internal Workflow API",
                "Internal maintenance request read",
                "Read",
                $"requestId={request.Id}; gunId={request.GunNumber}; status={request.Status}");
        }

        return request is null
            ? NotFound(new { ok = false, error = "Workflow request not found." })
            : Ok(new { ok = true, item = ToWorkflowDto(request) });
    }

    [HttpPost("maintenance-requests")]
    public IActionResult CreateMaintenanceRequest([FromBody] WorkflowMaintenanceRequestInput input)
    {
        var gunId = ResolveGunId(input.GunId, input.AssetId);
        if (gunId is null)
        {
            return BadRequest(new { ok = false, error = "GunId or an AssetId like Gun 1 is required." });
        }

        var requestedBy = Clean(input.RequestedBy, "internal-service");
        var request = _maintenanceRequests.Submit(
            gunId.Value,
            "Internal workflow maintenance request.",
            requestedBy,
            "Maintenance",
            Clean(input.SourceApp, "internal-api"),
            CleanAssetId(input.AssetId, gunId.Value),
            input.PanelId ?? gunId.Value,
            Clean(input.RequestType, "EnableMaintenance"));

        _appEvents.Add(
            "maintenance",
            Clean(input.SourceApp, "Internal Workflow API"),
            "Maintenance request submitted",
            "Pending",
            $"{request.GunName}: internal maintenance request submitted.");

        return Created($"/internal/workflow/maintenance-requests/{request.Id}", new
        {
            ok = true,
            item = ToWorkflowDto(request)
        });
    }

    [HttpPost("part-requests")]
    public IActionResult CreatePartRequest([FromBody] WorkflowPartRequestInput input)
    {
        var gunId = ResolveGunId(input.GunId, input.AssetId);
        if (gunId is null)
        {
            return BadRequest(new { ok = false, error = "GunId or AssetId is required." });
        }

        var requestedBy = Clean(input.RequestedBy, "internal-service");
        var request = _maintenanceRequests.Submit(
            gunId.Value,
            "Internal workflow part request.",
            requestedBy,
            "Part",
            Clean(input.SourceApp, "internal-api"),
            CleanAssetId(input.AssetId, gunId.Value),
            input.PanelId ?? gunId.Value,
            "RequestPart");

        _appEvents.Add(
            "maintenance",
            Clean(input.SourceApp, "Internal Workflow API"),
            "Part request submitted",
            "Pending",
            $"{request.GunName}: internal part request submitted.");

        return Created($"/internal/workflow/part-requests/{request.Id}", new
        {
            ok = true,
            item = ToWorkflowDto(request)
        });
    }

    [HttpGet("part-requests")]
    public IActionResult ListPartRequests([FromQuery] string? status = null)
    {
        var requests = _maintenanceRequests.List()
            .Where(request => request.RequestKind == "PartRequest");

        if (!string.IsNullOrWhiteSpace(status))
        {
            requests = requests.Where(request => string.Equals(request.Status, status, StringComparison.OrdinalIgnoreCase));
        }

        var items = requests.Select(ToWorkflowDto).ToArray();
        _appEvents.Add(
            "admin",
            "Internal Workflow API",
            "Internal part requests listed",
            "Read",
            $"items={items.Length}; status={Clean(status, "any")}");

        return Ok(new
        {
            ok = true,
            items
        });
    }

    [HttpGet("part-requests/{requestId:int}")]
    public IActionResult GetPartRequest(int requestId)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId && request.RequestKind == "PartRequest");
        if (request is not null)
        {
            _appEvents.Add(
                "admin",
                "Internal Workflow API",
                "Internal part request read",
                "Read",
                $"requestId={request.Id}; gunId={request.GunNumber}; status={request.Status}");
        }

        return request is null
            ? NotFound(new { ok = false, error = "Part request not found." })
            : Ok(new { ok = true, item = ToWorkflowDto(request) });
    }

    [HttpPost("maintenance-requests/{requestId:int}/status")]
    public IActionResult UpdateMaintenanceRequestStatus(int requestId, [FromBody] WorkflowStatusUpdateInput input)
    {
        return UpdateRequestStatus(requestId, input);
    }

    [HttpPost("part-requests/{requestId:int}/status")]
    public IActionResult UpdatePartRequestStatus(int requestId, [FromBody] WorkflowStatusUpdateInput input)
    {
        var request = _maintenanceRequests.List().SingleOrDefault(request => request.Id == requestId && request.RequestKind == "PartRequest");
        if (request is null)
        {
            return NotFound(new { ok = false, error = "Part request not found." });
        }

        return UpdateRequestStatus(requestId, input);
    }

    private static object ToWorkflowDto(MaintenanceRequestViewModel request)
    {
        return new
        {
            request.Id,
            requestKind = request.RequestKind,
            requestType = request.RequestSubType,
            request.RequestedBy,
            sourceApp = request.SourceApp,
            assetId = request.AssetId,
            panelId = request.PanelId,
            gunId = request.GunNumber,
            request.Status,
            createdUtc = request.RequestedAt.ToUniversalTime(),
            updatedUtc = (request.ReviewedAt ?? request.RequestedAt).ToUniversalTime(),
            request.ReviewedBy,
            request.ReviewNotes,
            safeRouteHint = request.SafeRouteHint
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

    private static string? NormalizeStatus(string? status)
    {
        return status?.Trim().ToLowerInvariant() switch
        {
            "approved" or "approve" => "Approved",
            "denied" or "deny" or "rejected" => "Denied",
            "canceled" or "cancelled" or "cancel" => "Canceled",
            _ => null
        };
    }

    private static string Clean(string? value, string? fallback)
    {
        return string.IsNullOrWhiteSpace(value) ? fallback ?? "" : value.Trim();
    }

    private IActionResult UpdateRequestStatus(int requestId, WorkflowStatusUpdateInput input)
    {
        var normalizedStatus = NormalizeStatus(input.Status);
        if (normalizedStatus is null)
        {
            return BadRequest(new { ok = false, error = "Status must be Approved, Denied, or Canceled." });
        }

        var reviewedBy = Clean(input.UpdatedBy, "internal-service");
        var notes = Clean(input.Notes, $"Internal request status changed to {normalizedStatus}.");
        var updated = normalizedStatus switch
        {
            "Approved" => _maintenanceRequests.Approve(requestId, reviewedBy, notes),
            "Denied" => _maintenanceRequests.Deny(requestId, reviewedBy, notes),
            "Canceled" => _maintenanceRequests.Cancel(requestId, reviewedBy, notes),
            _ => null
        };

        if (updated is null)
        {
            return NotFound(new { ok = false, error = "Pending workflow request not found." });
        }

        _appEvents.Add(
            "maintenance",
            "Internal Workflow API",
            $"{updated.RequestKind} {normalizedStatus.ToLowerInvariant()}",
            normalizedStatus,
            $"{updated.GunName}: request status updated by {reviewedBy}.");

        return Ok(new
        {
            ok = true,
            item = ToWorkflowDto(updated)
        });
    }

    private static string CleanAssetId(string? assetId, int gunId)
    {
        return string.IsNullOrWhiteSpace(assetId) ? $"GUN-{gunId}" : assetId.Trim();
    }
}

public sealed record WorkflowMaintenanceRequestInput(
    string? RequestedBy,
    string? SourceApp,
    string? AssetId,
    int? GunId,
    int? PanelId,
    string? RequestType);

public sealed record WorkflowPartRequestInput(
    string? RequestedBy,
    string? SourceApp,
    string? AssetId,
    int? GunId,
    int? PanelId);

public sealed record WorkflowStatusUpdateInput(
    string? Status,
    string? UpdatedBy,
    string? Notes);
