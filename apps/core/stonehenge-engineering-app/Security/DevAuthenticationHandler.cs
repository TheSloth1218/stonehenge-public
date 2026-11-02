using System.Security.Claims;
using System.Text.Encodings.Web;
using Microsoft.AspNetCore.Authentication;
using Microsoft.Extensions.Options;
using Stonehenge.Engineering.App.Services;

namespace Stonehenge.Engineering.App.Security;

public sealed class DevAuthenticationHandler : AuthenticationHandler<AuthenticationSchemeOptions>
{
    private const string DevUserHeader = "X-Dev-User";
    public const string DevUserCookie = "Stonehenge.DevUser";

    private static readonly IReadOnlyDictionary<string, DevUserProfile> Profiles =
        new Dictionary<string, DevUserProfile>(StringComparer.OrdinalIgnoreCase)
        {
            ["employee.basic"] = new(
                UserName: "employee.basic",
                DisplayName: "Basic Employee",
                OrganizationalUnit: "Employees",
                Groups: [StonehengeAuth.EmployeesGroup]),
            ["engineering.user"] = new(
                UserName: "engineering.user",
                DisplayName: "Engineering User",
                OrganizationalUnit: "Engineering",
                Groups:
                [
                    StonehengeAuth.EmployeesGroup,
                    StonehengeAuth.EngineeringUsersGroup
                ]),
            ["engineering.admin"] = new(
                UserName: "engineering.admin",
                DisplayName: "Engineering Admin",
                OrganizationalUnit: "Engineering",
                Groups:
                [
                    StonehengeAuth.EmployeesGroup,
                    StonehengeAuth.EngineeringUsersGroup,
                    StonehengeAuth.EngineeringAdminsGroup
                ]),
            ["platform.admin"] = new(
                UserName: "platform.admin",
                DisplayName: "Platform Admin Outside Engineering",
                OrganizationalUnit: "Platform Operations",
                Groups:
                [
                    StonehengeAuth.EmployeesGroup,
                    "Domain Admins"
                ])
        };

    private readonly StonehengeAuthenticationOptions _authenticationSettings;

    public DevAuthenticationHandler(
        IOptionsMonitor<AuthenticationSchemeOptions> options,
        ILoggerFactory logger,
        UrlEncoder encoder,
        StonehengeAuthenticationOptions authenticationSettings)
        : base(options, logger, encoder)
    {
        _authenticationSettings = authenticationSettings;
    }

    protected override Task<AuthenticateResult> HandleAuthenticateAsync()
    {
        var userName = Request.Headers[DevUserHeader].FirstOrDefault();
        if (string.IsNullOrWhiteSpace(userName))
        {
            Request.Cookies.TryGetValue(DevUserCookie, out userName);
        }

        if (string.IsNullOrWhiteSpace(userName) ||
            !Profiles.TryGetValue(userName.Trim(), out var profile))
        {
            return Task.FromResult(AuthenticateResult.NoResult());
        }

        var claims = new List<Claim>
        {
            new(ClaimTypes.NameIdentifier, profile.UserName),
            new(ClaimTypes.Name, profile.DisplayName),
            new("sub", profile.UserName),
            new("name", profile.DisplayName),
            new("ou", profile.OrganizationalUnit)
        };

        claims.AddRange(profile.Groups.Select(group => new Claim(StonehengeAuth.GroupsClaim, group)));

        var identity = new ClaimsIdentity(claims, Scheme.Name);
        var principal = new ClaimsPrincipal(identity);
        StonehengeAuthenticationExtensions.NormalizeClaims(principal, _authenticationSettings.Oidc);
        var ticket = new AuthenticationTicket(principal, Scheme.Name);
        return Task.FromResult(AuthenticateResult.Success(ticket));
    }

    protected override Task HandleChallengeAsync(AuthenticationProperties properties)
    {
        Response.StatusCode = StatusCodes.Status401Unauthorized;
        Response.Headers.WWWAuthenticate = StonehengeAuth.DevScheme;
        return Task.CompletedTask;
    }

    protected override Task HandleForbiddenAsync(AuthenticationProperties properties)
    {
        var path = Request.Path.Value ?? "/";
        var displayName = Context.User.StonehengeDisplayName();
        Context.RequestServices.GetService<IAppEventStore>()?.Add(
            "security",
            displayName,
            "Access denied",
            "Forbidden",
            $"route={path}; access={Context.User.FindFirst(StonehengeAuth.AccessLevelClaim)?.Value ?? "unknown"}");

        if (AcceptsHtml() && !path.StartsWith("/api/", StringComparison.OrdinalIgnoreCase) && !path.StartsWith("/internal/", StringComparison.OrdinalIgnoreCase))
        {
            var encodedReturnUrl = UrlEncoder.Default.Encode(path);
            Response.Redirect($"/AccessDenied?returnUrl={encodedReturnUrl}");
            return Task.CompletedTask;
        }

        Response.StatusCode = StatusCodes.Status403Forbidden;
        return Task.CompletedTask;
    }

    private bool AcceptsHtml()
    {
        var accept = Request.Headers.Accept.ToString();
        return accept.Contains("text/html", StringComparison.OrdinalIgnoreCase);
    }

    private sealed record DevUserProfile(
        string UserName,
        string DisplayName,
        string OrganizationalUnit,
        IReadOnlyList<string> Groups);
}
