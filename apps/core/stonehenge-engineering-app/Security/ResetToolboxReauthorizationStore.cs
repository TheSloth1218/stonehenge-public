using System.Collections.Concurrent;
using System.Security.Claims;
using System.Security.Cryptography;

namespace Stonehenge.Engineering.App.Security;

public interface IResetToolboxReauthorizationStore
{
    string Grant(ClaimsPrincipal principal, DateTimeOffset expiresUtc);
    bool Validate(string? token, ClaimsPrincipal principal);
    void Revoke(string? token);
    void RevokeAll(ClaimsPrincipal principal);
}

public sealed class ResetToolboxReauthorizationStore : IResetToolboxReauthorizationStore
{
    private readonly ConcurrentDictionary<string, ReauthorizationGrant> _grants = new(StringComparer.Ordinal);

    public string Grant(ClaimsPrincipal principal, DateTimeOffset expiresUtc)
    {
        var subject = Subject(principal) ?? throw new InvalidOperationException("Authenticated subject is required.");
        RevokeAll(principal);

        var token = Convert.ToHexString(RandomNumberGenerator.GetBytes(32));
        _grants[token] = new ReauthorizationGrant(subject, expiresUtc);
        return token;
    }

    public bool Validate(string? token, ClaimsPrincipal principal)
    {
        var subject = Subject(principal);
        if (string.IsNullOrWhiteSpace(token) || string.IsNullOrWhiteSpace(subject) || !_grants.TryGetValue(token, out var grant))
        {
            return false;
        }

        if (grant.ExpiresUtc <= DateTimeOffset.UtcNow || !string.Equals(grant.Subject, subject, StringComparison.Ordinal))
        {
            _grants.TryRemove(token, out _);
            return false;
        }

        return true;
    }

    public void Revoke(string? token)
    {
        if (!string.IsNullOrWhiteSpace(token))
        {
            _grants.TryRemove(token, out _);
        }
    }

    public void RevokeAll(ClaimsPrincipal principal)
    {
        var subject = Subject(principal);
        if (string.IsNullOrWhiteSpace(subject))
        {
            return;
        }

        foreach (var grant in _grants.Where(item => string.Equals(item.Value.Subject, subject, StringComparison.Ordinal)).ToArray())
        {
            _grants.TryRemove(grant.Key, out _);
        }
    }

    private static string? Subject(ClaimsPrincipal principal)
    {
        return principal.FindFirstValue(ClaimTypes.NameIdentifier)
            ?? principal.FindFirstValue("sub")
            ?? principal.Identity?.Name;
    }

    private sealed record ReauthorizationGrant(string Subject, DateTimeOffset ExpiresUtc);
}
