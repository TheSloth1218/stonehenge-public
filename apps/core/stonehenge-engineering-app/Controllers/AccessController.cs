using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Stonehenge.Engineering.App.Security;

namespace Stonehenge.Engineering.App.Controllers;

[AllowAnonymous]
public sealed class AccessController : Controller
{
    [HttpGet("/AccessDenied")]
    public IActionResult Denied(string? returnUrl = null)
    {
        Response.StatusCode = StatusCodes.Status403Forbidden;
        ViewData["ReturnUrl"] = string.IsNullOrWhiteSpace(returnUrl) ? "/" : returnUrl;
        ViewData["AccessLevel"] = User.FindFirst(StonehengeAuth.AccessLevelClaim)?.Value ?? "Unauthenticated";
        ViewData["DisplayName"] = User.StonehengeDisplayName();
        ViewData["Groups"] = string.Join(", ", User.Claims
            .Where(claim => claim.Type == StonehengeAuth.GroupsClaim)
            .Select(claim => claim.Value)
            .DefaultIfEmpty("none"));
        return View();
    }
}
