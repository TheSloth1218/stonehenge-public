using System.Security.Cryptography;
using System.Text;
using System.IdentityModel.Tokens.Jwt;
using System.Security.Claims;
using Microsoft.IdentityModel.Protocols;
using Microsoft.IdentityModel.Protocols.OpenIdConnect;
using Microsoft.IdentityModel.Tokens;

namespace Stonehenge.Engineering.App.Security;

public interface IRecoveryTokenValidator
{
    Task<RecoveryValidationResult> ValidateAsync(string token, CancellationToken cancellationToken);
}

public sealed record RecoveryValidationResult(bool Succeeded, string Reason, string? ReplayId, DateTimeOffset ExpiresUtc, IReadOnlyList<string> ClaimNames);

public sealed class RecoveryTokenValidator : IRecoveryTokenValidator
{
    private static readonly HashSet<string> ForbiddenGroups = new(StringComparer.OrdinalIgnoreCase)
    {
        "SCA-Engineering-Admins", "SCA-Admins", "Domain Admins"
    };
    private static readonly HashSet<string> AllowedClaimNames = new(StringComparer.Ordinal)
    {
        "iss", "sub", "aud", "exp", "nbf", "iat", "jti", "azp", "groups",
        "name", "preferred_username", "email", "email_verified", "auth_time", "nonce", "at_hash", "c_hash"
    };
    private readonly StonehengeAuthenticationOptions _authentication;

    public RecoveryTokenValidator(StonehengeAuthenticationOptions authentication)
    {
        _authentication = authentication;
    }

    public async Task<RecoveryValidationResult> ValidateAsync(string token, CancellationToken cancellationToken)
    {
        if (string.IsNullOrWhiteSpace(token)) return Denied("token_missing");
        if (string.IsNullOrWhiteSpace(_authentication.Oidc.Authority)) return Denied("oidc_not_configured");
        try
        {
            var authority = _authentication.Oidc.Authority.TrimEnd('/');
            var metadata = new ConfigurationManager<OpenIdConnectConfiguration>(
                $"{authority}/.well-known/openid-configuration",
                new OpenIdConnectConfigurationRetriever(),
                new HttpDocumentRetriever { RequireHttps = _authentication.Oidc.RequireHttpsMetadata });
            var configuration = await metadata.GetConfigurationAsync(cancellationToken);
            var parameters = new TokenValidationParameters
            {
                ValidateIssuer = true,
                ValidIssuer = _authentication.Oidc.Authority.TrimEnd('/'),
                ValidateIssuerSigningKey = true,
                IssuerSigningKeys = configuration.SigningKeys,
                RequireSignedTokens = true,
                ValidateLifetime = true,
                RequireExpirationTime = true,
                ClockSkew = TimeSpan.FromSeconds(30),
                ValidateAudience = true,
                ValidAudience = "stonehenge-engineering",
                ValidAlgorithms = [SecurityAlgorithms.RsaSha256],
                NameClaimType = "sub",
                RoleClaimType = "groups"
            };
            var handler = new JwtSecurityTokenHandler { MapInboundClaims = false };
            var principal = handler.ValidateToken(token, parameters, out var validated);
            if (validated is not JwtSecurityToken jwt || !string.Equals(jwt.Header.Alg, SecurityAlgorithms.RsaSha256, StringComparison.Ordinal))
                return Denied("algorithm_invalid");

            var audiences = principal.FindAll("aud").SelectMany(c => Split(c.Value)).ToArray();
            var audienceSet = audiences.ToHashSet(StringComparer.Ordinal);
            if (!audienceSet.Contains("stonehenge-engineering") || audienceSet.Count != audiences.Length
                || audienceSet.Any(audience => audience is not ("stonehenge-engineering" or "stonehenge-vendor")))
                return Denied("aud_invalid");
            if (principal.FindFirst("azp")?.Value != "stonehenge-vendor") return Denied("azp_invalid");
            if (string.IsNullOrWhiteSpace(principal.FindFirst("sub")?.Value)
                || principal.FindFirst("preferred_username")?.Value != "swordsman")
                return Denied("subject_invalid");
            var groups = principal.FindAll("groups").SelectMany(c => Split(c.Value)).ToHashSet(StringComparer.OrdinalIgnoreCase);
            if (!groups.Contains("SCA-Employees") || !groups.Contains("SCA-Engineering-Users")) return Denied("groups_required");
            if (groups.Any(group => ForbiddenGroups.Contains(group) || IsPrivileged(group))) return Denied("privileged_group");
            if (principal.Claims.Any(claim => IsForbiddenClaimName(claim.Type))) return Denied("privileged_claim");
            if (principal.Claims.Any(claim => !AllowedClaimNames.Contains(claim.Type))) return Denied("claim_not_allowed");
            var replayId = principal.FindFirst("jti")?.Value;
            if (string.IsNullOrWhiteSpace(replayId))
                replayId = Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(token))).ToLowerInvariant();
            var claimNames = principal.Claims.Select(c => c.Type).Distinct(StringComparer.Ordinal).Order().ToArray();
            return new(true, "accepted", replayId, jwt.ValidTo, claimNames);
        }
        catch (SecurityTokenExpiredException) { return Denied("expired"); }
        catch (SecurityTokenInvalidLifetimeException) { return Denied("expired"); }
        catch (SecurityTokenNotYetValidException) { return Denied("not_yet_valid"); }
        catch (SecurityTokenInvalidAudienceException) { return Denied("aud_invalid"); }
        catch (SecurityTokenInvalidIssuerException) { return Denied("issuer_invalid"); }
        catch (SecurityTokenInvalidSignatureException) { return Denied("signature_invalid"); }
        catch (SecurityTokenInvalidAlgorithmException) { return Denied("algorithm_invalid"); }
        catch (SecurityTokenException) { return Denied("token_invalid"); }
        catch (ArgumentException) { return Denied("token_invalid"); }
        catch (FormatException) { return Denied("token_invalid"); }
        catch (HttpRequestException) { return Denied("metadata_unavailable"); }
        catch (TaskCanceledException) { return Denied("metadata_unavailable"); }
        catch (InvalidOperationException) { return Denied("metadata_unavailable"); }
        catch (IOException) { return Denied("metadata_unavailable"); }
    }

    private static IEnumerable<string> Split(string value)
    {
        if (value.StartsWith('['))
        {
            try { return System.Text.Json.JsonSerializer.Deserialize<string[]>(value) ?? []; }
            catch (System.Text.Json.JsonException) { return []; }
        }
        return [value];
    }

    private static bool IsPrivileged(string value) =>
        value.Contains("admin", StringComparison.OrdinalIgnoreCase) || value.Contains("middleware", StringComparison.OrdinalIgnoreCase) ||
        value.Contains("openplc", StringComparison.OrdinalIgnoreCase) || value.Contains("plc", StringComparison.OrdinalIgnoreCase);

    private static bool IsForbiddenClaimName(string name) =>
        name.Contains("admin", StringComparison.OrdinalIgnoreCase) || name.Contains("privilege", StringComparison.OrdinalIgnoreCase) ||
        name.Contains("middleware", StringComparison.OrdinalIgnoreCase) || name.Contains("openplc", StringComparison.OrdinalIgnoreCase) ||
        name.StartsWith("plc", StringComparison.OrdinalIgnoreCase) || name.Contains("role", StringComparison.OrdinalIgnoreCase);

    private static RecoveryValidationResult Denied(string reason) => new(false, reason, null, default, []);
}
