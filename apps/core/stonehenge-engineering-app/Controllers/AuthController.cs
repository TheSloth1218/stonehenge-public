using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Stonehenge.Engineering.App.Security;

namespace Stonehenge.Engineering.App.Controllers;

public sealed class AuthController : Controller
{
    private readonly StonehengeAuthenticationOptions _authenticationOptions;
    private readonly IResetToolboxReauthorizationStore _resetReauthorization;

    public AuthController(
        StonehengeAuthenticationOptions authenticationOptions,
        IResetToolboxReauthorizationStore resetReauthorization)
    {
        _authenticationOptions = authenticationOptions;
        _resetReauthorization = resetReauthorization;
    }

    [AllowAnonymous]
    [HttpGet("/auth/login")]
    public IActionResult Login(string? returnUrl = null, bool fresh = false)
    {
        if (!_authenticationOptions.IsOidc)
        {
            return NotFound();
        }

        return Challenge(
            StonehengeAuthenticationExtensions.LoginProperties(SafeReturnUrl(returnUrl), fresh),
            StonehengeAuth.OidcScheme);
    }

    [Authorize]
    [ValidateAntiForgeryToken]
    [HttpPost("/auth/logout")]
    public async Task<IActionResult> Logout()
    {
        _resetReauthorization.RevokeAll(User);

        if (_authenticationOptions.IsOidc)
        {
            // Dex does not currently publish an end_session_endpoint. Clear the
            // local application session and let the signed-out page request a
            // fresh provider login explicitly.
            await HttpContext.SignOutAsync(StonehengeAuth.CookieScheme);
            return LocalRedirect("/auth/signed-out");
        }

        Response.Cookies.Delete(DevAuthenticationHandler.DevUserCookie);
        return LocalRedirect("/auth/signed-out");
    }

    [AllowAnonymous]
    [HttpGet("/auth/signed-out")]
    public IActionResult SignedOut()
    {
        return View();
    }

    private string SafeReturnUrl(string? returnUrl)
    {
        return !string.IsNullOrWhiteSpace(returnUrl) && Url.IsLocalUrl(returnUrl)
            ? returnUrl
            : "/";
    }
}
