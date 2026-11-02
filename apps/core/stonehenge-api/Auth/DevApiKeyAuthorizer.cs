using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;
using Microsoft.Extensions.Options;
using Stonehenge.Api.Options;

namespace Stonehenge.Api.Auth;

public sealed class DevApiKeyAuthorizer : IApiKeyAuthorizer
{
    public const string HeaderName = "X-Stonehenge-Api-Key";
    private static readonly Regex GeneratedKeyPattern = new(
        @"^stn_(?:dev|live)_(?<keyId>ak_[A-Za-z0-9]+)_(?<secret>[A-Za-z0-9_-]+)$",
        RegexOptions.Compiled);

    private static readonly IReadOnlyDictionary<string, ApiKeyPrincipal> Principals =
        new Dictionary<string, ApiKeyPrincipal>(StringComparer.Ordinal)
        {
            ["vendor-dev-key"] = new(
                "vendor-dev-key",
                new HashSet<string>(StringComparer.Ordinal)
                {
                    ApiScopes.VendorPartsRead,
                    ApiScopes.VendorGunsRead,
                    ApiScopes.VendorPartsAcknowledge,
                    ApiScopes.VendorPartsComplete
                },
                KeyId: "legacy_vendor",
                ClientName: "vendor-app",
                DisplayName: "Legacy Vendor Dev Key",
                IsLegacyDevAlias: true),
            ["astro-dev-key"] = new(
                "astro-dev-key",
                new HashSet<string>(StringComparer.Ordinal)
                {
                    ApiScopes.AstroHistoryRead,
                    ApiScopes.AstroResultsRead,
                    ApiScopes.AstroSummaryRead,
                    ApiScopes.SystemPublicStatusRead
                },
                KeyId: "legacy_astro",
                ClientName: "astro-app",
                DisplayName: "Legacy Astro Dev Key",
                IsLegacyDevAlias: true),
            ["engineering-dev-key"] = new(
                "engineering-dev-key",
                new HashSet<string>(StringComparer.Ordinal)
                {
                    ApiScopes.EngineeringVendorCompletionsRead,
                    ApiScopes.EngineeringVendorCompletionsApply,
                    ApiScopes.EngineeringPartRequestsWrite
                },
                KeyId: "legacy_engineering",
                ClientName: "engineering-app",
                DisplayName: "Legacy Engineering Dev Key",
                IsLegacyDevAlias: true)
        };

    private readonly IOptionsMonitor<TrainingVulnerabilityOptions> _trainingOptions;
    private readonly IOptionsMonitor<ApiKeyAuthOptions> _authOptions;
    private readonly ILogger<DevApiKeyAuthorizer> _logger;

    public DevApiKeyAuthorizer(
        IOptionsMonitor<TrainingVulnerabilityOptions> trainingOptions,
        IOptionsMonitor<ApiKeyAuthOptions> authOptions,
        ILogger<DevApiKeyAuthorizer> logger)
    {
        _trainingOptions = trainingOptions;
        _authOptions = authOptions;
        _logger = logger;
    }

    public ApiKeyPrincipal? Resolve(HttpRequest request)
    {
        if (!request.Headers.TryGetValue(HeaderName, out var values))
        {
            return null;
        }

        var key = values.FirstOrDefault();
        if (string.IsNullOrWhiteSpace(key))
        {
            return null;
        }

        var options = _authOptions.CurrentValue;
        var generated = ResolveGeneratedKey(key.Trim(), options);
        if (generated is not null)
        {
            return generated;
        }

        if (options.AllowLegacyDevKeys && Principals.TryGetValue(key.Trim(), out var principal))
        {
            return principal;
        }

        return null;
    }

    public bool HasScope(HttpRequest request, string requiredScope)
    {
        return RequireScope(request, requiredScope).Succeeded;
    }

    public ScopeAuthorizationResult RequireScope(HttpRequest request, string requiredScope)
    {
        var principal = Resolve(request);
        if (principal is null)
        {
            return new ScopeAuthorizationResult(false, requiredScope, null);
        }

        var allowed = (_trainingOptions.CurrentValue.Enabled && _trainingOptions.CurrentValue.EnableCrossClientScopeConfusion) ||
            principal.Scopes.Contains(requiredScope);
        return new ScopeAuthorizationResult(allowed, requiredScope, principal);
    }

    private ApiKeyPrincipal? ResolveGeneratedKey(string submittedKey, ApiKeyAuthOptions options)
    {
        var match = GeneratedKeyPattern.Match(submittedKey);
        if (!match.Success)
        {
            return null;
        }

        var keyId = match.Groups["keyId"].Value;
        var configured = options.Keys.FirstOrDefault(key =>
            key.Enabled &&
            key.KeyId.Equals(keyId, StringComparison.Ordinal) &&
            (key.ExpiresAt is null || key.ExpiresAt > DateTimeOffset.UtcNow));

        if (configured is null || string.IsNullOrWhiteSpace(configured.KeyHash))
        {
            return null;
        }

        var submittedHash = ComputeKeyHash(options.Pepper, submittedKey);
        if (!ConstantTimeEquals(configured.KeyHash, submittedHash))
        {
            _logger.LogWarning("API key hash mismatch for keyId {KeyId}", keyId);
            return null;
        }

        return new ApiKeyPrincipal(
            KeyName: configured.KeyId,
            Scopes: configured.Scopes.ToHashSet(StringComparer.Ordinal),
            KeyId: configured.KeyId,
            ClientName: configured.ClientName,
            DisplayName: configured.DisplayName);
    }

    public static string ComputeKeyHash(string pepper, string fullKey)
    {
        var pepperBytes = Encoding.UTF8.GetBytes(pepper);
        var keyBytes = Encoding.UTF8.GetBytes(fullKey);
        using var hmac = new HMACSHA256(pepperBytes);
        return Convert.ToHexString(hmac.ComputeHash(keyBytes)).ToLowerInvariant();
    }

    private static bool ConstantTimeEquals(string expectedHex, string actualHex)
    {
        var expected = Encoding.UTF8.GetBytes(expectedHex);
        var actual = Encoding.UTF8.GetBytes(actualHex);
        return expected.Length == actual.Length && CryptographicOperations.FixedTimeEquals(expected, actual);
    }
}
