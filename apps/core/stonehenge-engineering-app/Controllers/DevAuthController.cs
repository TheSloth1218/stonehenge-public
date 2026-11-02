using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Stonehenge.Engineering.App.Security;

namespace Stonehenge.Engineering.App.Controllers;

[AllowAnonymous]
public sealed class DevAuthController : Controller
{
    private readonly StonehengeAuthenticationOptions _authenticationOptions;

    private static readonly HashSet<string> SupportedUsers = new(StringComparer.OrdinalIgnoreCase)
    {
        "employee.basic",
        "engineering.user",
        "engineering.admin",
        "platform.admin"
    };

    public DevAuthController(StonehengeAuthenticationOptions authenticationOptions)
    {
        _authenticationOptions = authenticationOptions;
    }

    [HttpGet("/dev-auth/login/{userName}")]
    public IActionResult Login(string userName, string? returnUrl = null)
    {
        if (!_authenticationOptions.IsDev)
        {
            return NotFound();
        }

        if (!SupportedUsers.Contains(userName))
        {
            return BadRequest("Unknown development user.");
        }

        Response.Cookies.Append(
            DevAuthenticationHandler.DevUserCookie,
            userName,
            new CookieOptions
            {
                HttpOnly = true,
                SameSite = SameSiteMode.Lax,
                Secure = false,
                IsEssential = true,
                Path = "/"
            });

        return LocalRedirect(SafeReturnUrl(returnUrl));
    }

    [HttpGet("/dev-auth/logout")]
    public IActionResult Logout()
    {
        if (!_authenticationOptions.IsDev)
        {
            return NotFound();
        }

        Response.Cookies.Delete(DevAuthenticationHandler.DevUserCookie);
        return LocalRedirect("/");
    }

    private static string SafeReturnUrl(string? returnUrl)
    {
        return string.IsNullOrWhiteSpace(returnUrl) || !Uri.IsWellFormedUriString(returnUrl, UriKind.Relative)
            ? "/"
            : returnUrl;
    }
}
