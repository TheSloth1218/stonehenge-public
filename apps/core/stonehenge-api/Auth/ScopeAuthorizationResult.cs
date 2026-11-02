namespace Stonehenge.Api.Auth;

public sealed record ScopeAuthorizationResult(
    bool Succeeded,
    string RequiredScope,
    ApiKeyPrincipal? Principal);
