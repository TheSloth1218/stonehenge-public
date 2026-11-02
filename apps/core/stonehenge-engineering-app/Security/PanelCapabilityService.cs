using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Microsoft.AspNetCore.DataProtection;
using Microsoft.Extensions.Options;

namespace Stonehenge.Engineering.App.Security;

public interface IPanelCapabilityService
{
    PanelCapability Issue(int panelId, string subject);
    bool Validate(string token, int panelId, string subject, out DateTimeOffset expiresUtc);
}

public sealed record PanelCapability(string Token, DateTimeOffset ExpiresUtc, string TokenHash);

public sealed class PanelCapabilityService : IPanelCapabilityService
{
    private readonly IDataProtector _protector;
    private readonly TrainingSecurityOptions _options;

    public PanelCapabilityService(IDataProtectionProvider provider, IOptions<TrainingSecurityOptions> options)
    {
        _protector = provider.CreateProtector("Stonehenge.Engineering.PanelCapability.v1");
        _options = options.Value;
    }

    public PanelCapability Issue(int panelId, string subject)
    {
        var expiry = DateTimeOffset.UtcNow.Add(_options.PanelTokenLifetime);
        var payload = JsonSerializer.Serialize(new PanelPayload(panelId, subject, expiry, Guid.NewGuid().ToString("N")));
        var token = _protector.Protect(payload);
        return new(token, expiry, Hash(token));
    }

    public bool Validate(string token, int panelId, string subject, out DateTimeOffset expiresUtc)
    {
        expiresUtc = default;
        try
        {
            var payload = JsonSerializer.Deserialize<PanelPayload>(_protector.Unprotect(token));
            if (payload is null || payload.PanelId != panelId || payload.ExpiresUtc <= DateTimeOffset.UtcNow ||
                !string.Equals(payload.Subject, subject, StringComparison.Ordinal)) return false;
            expiresUtc = payload.ExpiresUtc;
            return true;
        }
        catch (CryptographicException) { return false; }
        catch (JsonException) { return false; }
    }

    private static string Hash(string value) => Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(value))).ToLowerInvariant();
    private sealed record PanelPayload(int PanelId, string Subject, DateTimeOffset ExpiresUtc, string Nonce);
}
