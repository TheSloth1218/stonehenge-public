namespace Stonehenge.Api.Auth;

public interface IApiKeyAuthorizer
{
    ApiKeyPrincipal? Resolve(HttpRequest request);

    ScopeAuthorizationResult RequireScope(HttpRequest request, string requiredScope);

    bool HasScope(HttpRequest request, string requiredScope);
}
