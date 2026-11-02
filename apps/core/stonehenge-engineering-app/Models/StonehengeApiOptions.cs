namespace Stonehenge.Engineering.App.Models;

public sealed class StonehengeApiOptions
{
    public bool Enabled { get; init; } = true;

    public string BaseUrl { get; init; } = "http://stonehenge-api:8092/api/v1";

    public string ApiKey { get; init; } = "engineering-dev-key";
}
