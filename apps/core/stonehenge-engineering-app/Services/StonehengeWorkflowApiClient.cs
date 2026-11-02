using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.Extensions.Options;
using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class StonehengeWorkflowApiClient : IStonehengeWorkflowApiClient
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    private readonly HttpClient _httpClient;
    private readonly IOptionsMonitor<StonehengeApiOptions> _options;
    private readonly ILogger<StonehengeWorkflowApiClient> _logger;

    public StonehengeWorkflowApiClient(
        HttpClient httpClient,
        IOptionsMonitor<StonehengeApiOptions> options,
        ILogger<StonehengeWorkflowApiClient> logger)
    {
        _httpClient = httpClient;
        _options = options;
        _logger = logger;
    }

    public async Task<StonehengeApiExportResult> ExportApprovedPartRequestAsync(MaintenanceRequestViewModel request, CancellationToken cancellationToken)
    {
        if (!_options.CurrentValue.Enabled)
        {
            return new StonehengeApiExportResult(false, "Stonehenge API sync disabled.");
        }

        var payload = new
        {
            requestId = ToApiRequestId(request.Id),
            sourceObjectId = request.Id,
            gunId = request.GunNumber,
            status = "ApprovedForVendor",
            partStatus = "Parts",
            priority = "Normal",
            approvedAt = ToUtcOffset(request.ReviewedAt ?? DateTime.UtcNow),
            updatedAt = DateTimeOffset.UtcNow
        };

        try
        {
            using var response = await _httpClient.PostAsJsonAsync("engineering/part-requests", payload, JsonOptions, cancellationToken);
            if (response.IsSuccessStatusCode)
            {
                return new StonehengeApiExportResult(true, null);
            }

            return new StonehengeApiExportResult(false, $"API export failed: {(int)response.StatusCode} {response.ReasonPhrase}");
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException)
        {
            _logger.LogWarning(ex, "Stonehenge API part request export failed for request {RequestId}", request.Id);
            return new StonehengeApiExportResult(false, "API export failed.");
        }
    }

    public async Task<IReadOnlyList<StonehengeVendorCompletion>> GetPendingVendorCompletionsAsync(CancellationToken cancellationToken)
    {
        if (!_options.CurrentValue.Enabled)
        {
            return [];
        }

        var response = await _httpClient.GetFromJsonAsync<PendingVendorCompletionsApiResponse>("engineering/vendor-completions/pending", JsonOptions, cancellationToken);
        return response?.Completions ?? [];
    }

    public async Task<bool> MarkVendorCompletionAppliedAsync(string completionId, CancellationToken cancellationToken)
    {
        if (!_options.CurrentValue.Enabled)
        {
            return false;
        }

        using var response = await _httpClient.PostAsync($"engineering/vendor-completions/{Uri.EscapeDataString(completionId)}/applied", content: null, cancellationToken);
        return response.IsSuccessStatusCode;
    }

    public static string ToApiRequestId(int requestId) => $"PR-{requestId}";

    private static DateTimeOffset ToUtcOffset(DateTime value)
    {
        return value.Kind == DateTimeKind.Unspecified
            ? new DateTimeOffset(DateTime.SpecifyKind(value, DateTimeKind.Local)).ToUniversalTime()
            : new DateTimeOffset(value).ToUniversalTime();
    }

    private sealed record PendingVendorCompletionsApiResponse(IReadOnlyList<StonehengeVendorCompletion> Completions);
}
