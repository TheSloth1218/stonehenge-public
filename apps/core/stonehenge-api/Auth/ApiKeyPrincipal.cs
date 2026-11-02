namespace Stonehenge.Api.Auth;

public sealed record ApiKeyPrincipal(
    string KeyName,
    IReadOnlySet<string> Scopes,
    string? KeyId = null,
    string? ClientName = null,
    string? DisplayName = null,
    bool IsLegacyDevAlias = false);
