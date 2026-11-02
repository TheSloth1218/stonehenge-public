namespace Stonehenge.Engineering.App.Security;

public sealed class StonehengeAuthenticationOptions
{
    public const string SectionName = "Authentication";
    public const string DevMode = "Dev";
    public const string OidcMode = "Oidc";

    public string Mode { get; set; } = DevMode;
    public string DataProtectionKeysPath { get; set; } = "/data/data-protection-keys";
    public StonehengeOidcOptions Oidc { get; set; } = new();
    public StonehengeForwardedHeadersOptions ForwardedHeaders { get; set; } = new();

    public bool IsDev => string.Equals(Mode, DevMode, StringComparison.OrdinalIgnoreCase);
    public bool IsOidc => string.Equals(Mode, OidcMode, StringComparison.OrdinalIgnoreCase);

    public void Validate()
    {
        if (!IsDev && !IsOidc)
        {
            throw new InvalidOperationException("Authentication:Mode must be Dev or Oidc.");
        }

        if (!IsOidc)
        {
            return;
        }

        var missing = new List<string>();
        AddIfMissing(missing, Oidc.Authority, "Authentication:Oidc:Authority");
        AddIfMissing(missing, Oidc.ClientId, "Authentication:Oidc:ClientId");
        AddIfMissing(missing, Oidc.ClientSecret, "Authentication:Oidc:ClientSecret");
        AddBoundaryIfIncomplete(missing, Oidc.EngineeringBoundary, "Authentication:Oidc:EngineeringBoundary");
        AddBoundaryIfIncomplete(missing, Oidc.AdminBoundary, "Authentication:Oidc:AdminBoundary");

        if (!ForwardedHeaders.KnownProxies.Any(value => !string.IsNullOrWhiteSpace(value)) &&
            !ForwardedHeaders.KnownNetworks.Any(value => !string.IsNullOrWhiteSpace(value)))
        {
            missing.Add("Authentication:ForwardedHeaders:KnownProxies or KnownNetworks");
        }

        if (missing.Count > 0)
        {
            throw new InvalidOperationException($"OIDC authentication is incomplete. Configure: {string.Join(", ", missing)}.");
        }
    }

    private static void AddIfMissing(List<string> missing, string? value, string setting)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            missing.Add(setting);
        }
    }

    private static void AddBoundaryIfIncomplete(
        List<string> missing,
        StonehengeClaimBoundaryOptions boundary,
        string setting)
    {
        if (!boundary.ClaimTypes.Any(value => !string.IsNullOrWhiteSpace(value)) ||
            (!boundary.Values.Any(value => !string.IsNullOrWhiteSpace(value)) &&
             !boundary.ValueFragments.Any(value => !string.IsNullOrWhiteSpace(value))))
        {
            missing.Add(setting);
        }
    }
}

public sealed class StonehengeOidcOptions
{
    public string Authority { get; set; } = string.Empty;
    public string ClientId { get; set; } = string.Empty;
    public string ClientSecret { get; set; } = string.Empty;
    public string CallbackPath { get; set; } = "/signin-oidc";
    public string SignedOutCallbackPath { get; set; } = "/signout-callback-oidc";
    public bool RequireHttpsMetadata { get; set; } = true;
    public List<string> Scopes { get; set; } = ["openid", "profile", "email", "groups"];
    public List<string> AllowedGroups { get; set; } =
    [
        StonehengeAuth.EmployeesGroup,
        StonehengeAuth.EngineeringUsersGroup,
        StonehengeAuth.EngineeringAdminsGroup
    ];
    public StonehengeClaimBoundaryOptions EngineeringBoundary { get; set; } = new()
    {
        ClaimTypes = ["groups"],
        Values =
        [
            StonehengeAuth.EngineeringUsersGroup,
            StonehengeAuth.EngineeringAdminsGroup,
            "SCA-Engineering-Users",
            "SCA-Engineering-Admins"
        ]
    };
    public StonehengeClaimBoundaryOptions AdminBoundary { get; set; } = new()
    {
        ClaimTypes = ["groups"],
        Values = [StonehengeAuth.EngineeringAdminsGroup, "SCA-Engineering-Admins"]
    };
}

public sealed class StonehengeClaimBoundaryOptions
{
    public List<string> ClaimTypes { get; set; } = [];
    public List<string> Values { get; set; } = [];
    public List<string> ValueFragments { get; set; } = [];
}

public sealed class StonehengeForwardedHeadersOptions
{
    public List<string> KnownProxies { get; set; } = [];
    public List<string> KnownNetworks { get; set; } = [];
}
