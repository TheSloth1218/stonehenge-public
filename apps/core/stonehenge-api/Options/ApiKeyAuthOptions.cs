namespace Stonehenge.Api.Options;

public sealed class ApiKeyAuthOptions
{
    public string Pepper { get; init; } = "development-only-change-me";

    public bool AllowLegacyDevKeys { get; init; } = true;

    public List<ApiKeyCredentialOptions> Keys { get; init; } = [];
}

public sealed class ApiKeyCredentialOptions
{
    public string KeyId { get; init; } = "";

    public string DisplayName { get; init; } = "";

    public string ClientName { get; init; } = "";

    public string KeyHash { get; init; } = "";

    public List<string> Scopes { get; init; } = [];

    public bool Enabled { get; init; } = true;

    public DateTimeOffset CreatedAt { get; init; } = DateTimeOffset.MinValue;

    public DateTimeOffset? ExpiresAt { get; init; }

    public string? Notes { get; init; }
}
