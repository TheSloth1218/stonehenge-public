using System.Net;
using System.Security.Claims;
using System.Text.Json;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.AspNetCore.Authentication.OpenIdConnect;
using Microsoft.AspNetCore.DataProtection;
using Microsoft.AspNetCore.HttpOverrides;
using Microsoft.IdentityModel.Protocols.OpenIdConnect;

namespace Stonehenge.Engineering.App.Security;

public static class StonehengeAuthenticationExtensions
{
    private const string ReauthRequestedItem = "stonehenge:reauth-requested";
    private const string ReauthSubjectItem = "stonehenge:reauth-subject";
    private const string ReauthStartedItem = "stonehenge:reauth-started";
    private const string FreshLoginRequestedItem = "stonehenge:fresh-login-requested";

    public static StonehengeAuthenticationOptions AddStonehengeAuthentication(this WebApplicationBuilder builder)
    {
        var configuration = builder.Configuration.GetSection(StonehengeAuthenticationOptions.SectionName);
        var settings = configuration.Get<StonehengeAuthenticationOptions>() ?? new StonehengeAuthenticationOptions();
        settings.Validate();

        builder.Services.Configure<StonehengeAuthenticationOptions>(configuration);
        builder.Services.AddSingleton(settings);
        builder.Services.AddSingleton<IResetToolboxReauthorizationStore, ResetToolboxReauthorizationStore>();
        builder.Services.AddScoped<OidcAntiforgeryFilter>();
        builder.Services.AddAntiforgery(options => options.HeaderName = "X-Stonehenge-CSRF");

        var keyPath = Path.IsPathRooted(settings.DataProtectionKeysPath)
            ? settings.DataProtectionKeysPath
            : Path.Combine(builder.Environment.ContentRootPath, settings.DataProtectionKeysPath);
        Directory.CreateDirectory(keyPath);
        builder.Services
            .AddDataProtection()
            .SetApplicationName("Stonehenge.Engineering.App")
            .PersistKeysToFileSystem(new DirectoryInfo(keyPath));

        ConfigureForwardedHeaders(builder.Services, settings);

        if (settings.IsDev)
        {
            builder.Services
                .AddAuthentication(StonehengeAuth.DevScheme)
                .AddScheme<AuthenticationSchemeOptions, DevAuthenticationHandler>(StonehengeAuth.DevScheme, _ => { })
                .AddCookie(StonehengeAuth.CookieScheme, options => ConfigureCookie(options, settings));
            return settings;
        }

        builder.Services
            .AddAuthentication(options =>
            {
                options.DefaultScheme = StonehengeAuth.CookieScheme;
                options.DefaultAuthenticateScheme = StonehengeAuth.CookieScheme;
                options.DefaultSignInScheme = StonehengeAuth.CookieScheme;
                options.DefaultChallengeScheme = StonehengeAuth.SmartChallengeScheme;
                options.DefaultForbidScheme = StonehengeAuth.CookieScheme;
            })
            .AddCookie(StonehengeAuth.CookieScheme, options =>
            {
                ConfigureCookie(options, settings);
            })
            .AddOpenIdConnect(StonehengeAuth.OidcScheme, options =>
            {
                options.Authority = settings.Oidc.Authority.TrimEnd('/');
                options.ClientId = settings.Oidc.ClientId;
                options.ClientSecret = settings.Oidc.ClientSecret;
                options.CallbackPath = settings.Oidc.CallbackPath;
                options.SignedOutCallbackPath = settings.Oidc.SignedOutCallbackPath;
                options.RequireHttpsMetadata = settings.Oidc.RequireHttpsMetadata;
                options.ResponseType = OpenIdConnectResponseType.Code;
                options.UsePkce = true;
                options.SaveTokens = false;
                options.GetClaimsFromUserInfoEndpoint = true;
                options.MapInboundClaims = false;
                options.SignInScheme = StonehengeAuth.CookieScheme;

                var mappedClaimTypes = settings.Oidc.EngineeringBoundary.ClaimTypes
                    .Concat(settings.Oidc.AdminBoundary.ClaimTypes)
                    .Append(StonehengeAuth.GroupsClaim)
                    .Where(type => !string.IsNullOrWhiteSpace(type))
                    .Distinct(StringComparer.OrdinalIgnoreCase);
                foreach (var claimType in mappedClaimTypes)
                {
                    // Dex emits groups as a JSON array. Map every value, then let
                    // NormalizeClaims apply the configured allowlist and boundaries.
                    options.ClaimActions.MapJsonKey(claimType, claimType);
                }

                options.Scope.Clear();
                foreach (var scope in settings.Oidc.Scopes.Where(scope => !string.IsNullOrWhiteSpace(scope)).Distinct(StringComparer.Ordinal))
                {
                    options.Scope.Add(scope);
                }

                options.Events = new OpenIdConnectEvents
                {
                    OnRedirectToIdentityProvider = context =>
                    {
                        var forceReauthentication =
                            context.Properties.Items.TryGetValue(ReauthRequestedItem, out var reauthRequested) &&
                            string.Equals(reauthRequested, "true", StringComparison.Ordinal);
                        var forceFreshLogin =
                            context.Properties.Items.TryGetValue(FreshLoginRequestedItem, out var freshLoginRequested) &&
                            string.Equals(freshLoginRequested, "true", StringComparison.Ordinal);

                        if (forceReauthentication || forceFreshLogin)
                        {
                            context.ProtocolMessage.Prompt = "login";
                            context.ProtocolMessage.MaxAge = "0";
                        }

                        return Task.CompletedTask;
                    },
                    OnTokenValidated = context =>
                    {
                        ValidateFreshReauthentication(context);
                        return Task.CompletedTask;
                    },
                    OnTicketReceived = context =>
                    {
                        // Dex group claims may be added by the UserInfo request after token
                        // validation. Normalize only after the complete OIDC ticket exists.
                        NormalizeClaims(context.Principal, settings.Oidc);
                        return Task.CompletedTask;
                    },
                    OnRemoteFailure = context =>
                    {
                        context.HandleResponse();
                        context.Response.Redirect("/AccessDenied?reason=oidc_authentication_failed");
                        return Task.CompletedTask;
                    }
                };
            })
            .AddPolicyScheme(StonehengeAuth.SmartChallengeScheme, null, options =>
            {
                options.ForwardDefaultSelector = context => IsInteractiveBrowserRequest(context.Request)
                    ? StonehengeAuth.OidcScheme
                    : StonehengeAuth.CookieScheme;
            });

        return settings;
    }

    private static void ConfigureCookie(CookieAuthenticationOptions options, StonehengeAuthenticationOptions settings)
    {
        options.Cookie.Name = "__Host-Stonehenge.Auth.v3";
        options.Cookie.HttpOnly = true;
        options.Cookie.SecurePolicy = CookieSecurePolicy.Always;
        options.Cookie.SameSite = SameSiteMode.Lax;
        options.Cookie.Path = "/";
        options.SlidingExpiration = true;
        options.ExpireTimeSpan = TimeSpan.FromHours(8);
        options.AccessDeniedPath = "/AccessDenied";
        options.Events = new CookieAuthenticationEvents
        {
            OnRedirectToLogin = context => HandleCookieRedirect(context, StatusCodes.Status401Unauthorized),
            OnRedirectToAccessDenied = context => HandleCookieRedirect(context, StatusCodes.Status403Forbidden),
            OnValidatePrincipal = context =>
            {
                var sessionId = context.Principal?.FindFirstValue(StonehengeAuth.TrainingSessionClaim);
                if (!string.IsNullOrWhiteSpace(sessionId))
                {
                    var training = context.HttpContext.RequestServices.GetRequiredService<Microsoft.Extensions.Options.IOptions<TrainingSecurityOptions>>().Value;
                    var registry = context.HttpContext.RequestServices.GetRequiredService<ITrainingSessionRegistry>();
                    if (!training.Enabled || !registry.IsActive(sessionId, DateTimeOffset.UtcNow))
                    {
                        context.RejectPrincipal();
                        return context.HttpContext.SignOutAsync(StonehengeAuth.CookieScheme);
                    }
                }
                NormalizeClaims(context.Principal, settings.Oidc);
                return Task.CompletedTask;
            }
        };
    }

    public static AuthenticationProperties LoginProperties(string redirectUri, bool forceFreshLogin)
    {
        var properties = new AuthenticationProperties { RedirectUri = redirectUri };
        if (forceFreshLogin)
        {
            properties.Items[FreshLoginRequestedItem] = "true";
        }

        return properties;
    }

    public static AuthenticationProperties FreshReauthenticationProperties(ClaimsPrincipal principal, string redirectUri)
    {
        var subject = principal.FindFirstValue("sub") ?? principal.FindFirstValue(ClaimTypes.NameIdentifier);
        if (string.IsNullOrWhiteSpace(subject))
        {
            throw new InvalidOperationException("OIDC subject claim is required for reset-toolbox reauthentication.");
        }

        var properties = new AuthenticationProperties { RedirectUri = redirectUri };
        properties.Items[ReauthRequestedItem] = "true";
        properties.Items[ReauthSubjectItem] = subject;
        properties.Items[ReauthStartedItem] = DateTimeOffset.UtcNow.ToUnixTimeSeconds().ToString();
        return properties;
    }

    private static void ConfigureForwardedHeaders(IServiceCollection services, StonehengeAuthenticationOptions settings)
    {
        services.Configure<ForwardedHeadersOptions>(options =>
        {
            options.ForwardedHeaders = ForwardedHeaders.XForwardedFor | ForwardedHeaders.XForwardedProto | ForwardedHeaders.XForwardedHost;
            options.ForwardLimit = 1;
            options.KnownProxies.Clear();
            options.KnownNetworks.Clear();

            foreach (var proxy in settings.ForwardedHeaders.KnownProxies.Where(value => !string.IsNullOrWhiteSpace(value)))
            {
                options.KnownProxies.Add(IPAddress.Parse(proxy));
            }

            foreach (var network in settings.ForwardedHeaders.KnownNetworks.Where(value => !string.IsNullOrWhiteSpace(value)))
            {
                var parts = network.Split('/', 2);
                if (parts.Length != 2 || !int.TryParse(parts[1], out var prefixLength))
                {
                    throw new InvalidOperationException($"Invalid trusted proxy network: {network}");
                }

                options.KnownNetworks.Add(new Microsoft.AspNetCore.HttpOverrides.IPNetwork(IPAddress.Parse(parts[0]), prefixLength));
            }
        });
    }

    private static Task HandleCookieRedirect(RedirectContext<CookieAuthenticationOptions> context, int statusCode)
    {
        if (IsInteractiveBrowserRequest(context.Request))
        {
            context.Response.Redirect(context.RedirectUri);
        }
        else
        {
            context.Response.StatusCode = statusCode;
        }

        return Task.CompletedTask;
    }

    private static bool IsInteractiveBrowserRequest(HttpRequest request)
    {
        var path = request.Path;
        if (path.StartsWithSegments("/api") || path.StartsWithSegments("/internal") || path.StartsWithSegments("/healthz") || path.StartsWithSegments("/fuxa") || path.StartsWithSegments("/socket.io"))
        {
            return false;
        }

        return HttpMethods.IsGet(request.Method) && request.Headers.Accept.ToString().Contains("text/html", StringComparison.OrdinalIgnoreCase);
    }

    internal static void NormalizeClaims(ClaimsPrincipal? principal, StonehengeOidcOptions settings)
    {
        if (principal?.Identity is not ClaimsIdentity identity)
        {
            return;
        }

        var rawClaims = principal.Claims.ToArray();
        var isEngineeringMember = MatchesBoundary(rawClaims, settings.EngineeringBoundary);
        var isAdministrator = isEngineeringMember && MatchesBoundary(rawClaims, settings.AdminBoundary);

        var approved = new HashSet<string>(settings.AllowedGroups, StringComparer.OrdinalIgnoreCase);
        var groups = rawClaims
            .Where(claim => string.Equals(claim.Type, StonehengeAuth.GroupsClaim, StringComparison.OrdinalIgnoreCase))
            .SelectMany(claim => ParseClaimValues(claim.Value))
            .Where(approved.Contains)
            .ToHashSet(StringComparer.OrdinalIgnoreCase);

        if (isEngineeringMember)
        {
            groups.Add(StonehengeAuth.EngineeringUsersGroup);
        }

        if (isAdministrator)
        {
            groups.Add(StonehengeAuth.EngineeringAdminsGroup);
        }

        foreach (var claim in identity.FindAll(StonehengeAuth.GroupsClaim).ToArray())
        {
            identity.RemoveClaim(claim);
        }

        foreach (var group in groups)
        {
            identity.AddClaim(new Claim(StonehengeAuth.GroupsClaim, group));
        }

        if (!identity.HasClaim(claim => claim.Type == ClaimTypes.NameIdentifier))
        {
            var subject = identity.FindFirst("sub")?.Value;
            if (!string.IsNullOrWhiteSpace(subject))
            {
                identity.AddClaim(new Claim(ClaimTypes.NameIdentifier, subject));
            }
        }

        var displayName = identity.FindFirst("name")?.Value
            ?? identity.FindFirst(ClaimTypes.Name)?.Value
            ?? identity.FindFirst("preferred_username")?.Value
            ?? identity.FindFirst("email")?.Value
            ?? identity.FindFirst("sub")?.Value
            ?? "OIDC user";
        ReplaceClaim(identity, StonehengeAuth.DisplayNameClaim, displayName);

        ReplaceClaim(
            identity,
            StonehengeAuth.EngineeringMemberClaim,
            isEngineeringMember ? StonehengeAuth.EngineeringMemberValue : "false");

        var accessLevel = isAdministrator
            ? StonehengeAuth.EngineeringAdminAccessLevel
            : isEngineeringMember
                ? StonehengeAuth.EngineeringAccessLevel
                : StonehengeAuth.DashboardAccessLevel;
        ReplaceClaim(identity, StonehengeAuth.AccessLevelClaim, accessLevel);
    }

    private static bool MatchesBoundary(
        IReadOnlyCollection<Claim> claims,
        StonehengeClaimBoundaryOptions boundary)
    {
        var claimTypes = new HashSet<string>(
            boundary.ClaimTypes.Where(value => !string.IsNullOrWhiteSpace(value)),
            StringComparer.OrdinalIgnoreCase);
        var acceptedValues = new HashSet<string>(
            boundary.Values.Where(value => !string.IsNullOrWhiteSpace(value)),
            StringComparer.OrdinalIgnoreCase);
        var acceptedFragments = boundary.ValueFragments
            .Where(value => !string.IsNullOrWhiteSpace(value))
            .ToArray();

        return claims
            .Where(claim => claimTypes.Contains(claim.Type))
            .SelectMany(claim => ParseClaimValues(claim.Value))
            .Any(value =>
                acceptedValues.Contains(value) ||
                acceptedFragments.Any(fragment =>
                    value.Contains(fragment, StringComparison.OrdinalIgnoreCase)));
    }

    private static IEnumerable<string> ParseClaimValues(string value)
    {
        if (!value.TrimStart().StartsWith('['))
        {
            yield return value;
            yield break;
        }

        string[] values;
        try
        {
            values = JsonSerializer.Deserialize<string[]>(value) ?? [];
        }
        catch (JsonException)
        {
            yield break;
        }

        foreach (var item in values)
        {
            yield return item;
        }
    }

    private static void ReplaceClaim(ClaimsIdentity identity, string type, string value)
    {
        foreach (var claim in identity.FindAll(type).ToArray())
        {
            identity.RemoveClaim(claim);
        }

        identity.AddClaim(new Claim(type, value));
    }

    private static void ValidateFreshReauthentication(TokenValidatedContext context)
    {
        var properties = context.Properties;
        if (properties is null ||
            !properties.Items.TryGetValue(ReauthRequestedItem, out var requested) ||
            !string.Equals(requested, "true", StringComparison.Ordinal))
        {
            return;
        }

        var actualSubject = context.Principal?.FindFirstValue("sub") ?? context.Principal?.FindFirstValue(ClaimTypes.NameIdentifier);
        properties.Items.TryGetValue(ReauthSubjectItem, out var expectedSubject);
        if (string.IsNullOrWhiteSpace(actualSubject) || !string.Equals(actualSubject, expectedSubject, StringComparison.Ordinal))
        {
            context.Fail("Reset-toolbox reauthentication must use the same account.");
            return;
        }

        if (!properties.Items.TryGetValue(ReauthStartedItem, out var startedRaw) ||
            !long.TryParse(startedRaw, out var startedUnix) ||
            !long.TryParse(context.Principal?.FindFirst("auth_time")?.Value, out var authTimeUnix) ||
            authTimeUnix < startedUnix - 5)
        {
            context.Fail("The identity provider did not prove fresh authentication.");
        }
    }
}
