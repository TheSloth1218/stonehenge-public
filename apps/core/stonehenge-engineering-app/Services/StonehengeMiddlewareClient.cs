using System.Net.Http.Json;
using System.Text.Json.Serialization;
using System.Text.Json;

namespace Stonehenge.Engineering.App.Services;

public sealed class StonehengeMiddlewareClient : IStonehengeMiddlewareClient
{
    private readonly HttpClient _httpClient;
    private readonly string _maintenanceToken;
    private readonly ILogger<StonehengeMiddlewareClient> _logger;

    public StonehengeMiddlewareClient(HttpClient httpClient, IConfiguration configuration, ILogger<StonehengeMiddlewareClient> logger)
    {
        _httpClient = httpClient;
        _maintenanceToken = configuration["STONEHENGE_MAINTENANCE_TOKEN"] ?? "";
        _logger = logger;
    }

    public async Task<MiddlewareStatusSnapshot> GetStatusAsync(CancellationToken cancellationToken = default)
    {
        using var response = await _httpClient.GetAsync("/api/v1/status", cancellationToken);
        response.EnsureSuccessStatusCode();

        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken);
        var root = document.RootElement;
        var observedAt = DateTimeOffset.Now;
        var guns = new List<MiddlewareGunSnapshot>();

        if (root.TryGetProperty("guns", out var gunsElement) && gunsElement.ValueKind == JsonValueKind.Array)
        {
            foreach (var gun in gunsElement.EnumerateArray())
            {
                var gunNumber = gun.GetInt("gun_id") ?? gun.GetInt("id") ?? 0;
                if (gunNumber is < 1 or > 8)
                {
                    continue;
                }

                guns.Add(new MiddlewareGunSnapshot(
                    GunNumber: gunNumber,
                    Name: gun.GetString("name") ?? $"Gun {gunNumber}",
                    Mode: NormalizeMode(gun.GetString("mode")),
                    State: gun.GetString("state_name") ?? gun.GetString("state") ?? "Unknown",
                    CurrentAzimuth: gun.GetDouble("current_azimuth") ?? FromTenths(gun.GetDouble("current_azimuth_tenths")) ?? 0,
                    CurrentElevation: gun.GetDouble("current_elevation") ?? FromTenths(gun.GetDouble("current_elevation_tenths")) ?? 0,
                    TargetAzimuth: gun.GetDouble("target_azimuth") ?? 0,
                    TargetElevation: gun.GetDouble("target_elevation") ?? 0,
                    SafetyState: BuildSafetyState(gun),
                    FaultActive: gun.GetBool("fault_active") ?? IsFaultCodeActive(gun.GetString("fault_code") ?? gun.GetString("fault_name")),
                    FaultCode: gun.GetString("fault_code") ?? gun.GetString("fault_name") ?? "",
                    HardLimitStatus: gun.GetString("hard_limit_status") ?? "",
                    MaintenanceEnabled: gun.GetBool("maintenance_enabled") ?? string.Equals(gun.GetString("mode"), "maintenance", StringComparison.OrdinalIgnoreCase),
                    ShotSequence: gun.GetInt("shot_sequence") ?? gun.GetInt("shot_sequence_low") ?? 0));
            }
        }

        guns = await ApplyArbitrationModesAsync(guns, cancellationToken);

        return new MiddlewareStatusSnapshot(
            Running: root.GetBool("running") ?? false,
            IsLive: true,
            ObservedAt: observedAt,
            Guns: guns.OrderBy(gun => gun.GunNumber).ToArray());
    }

    private static bool IsFaultCodeActive(string? faultCode)
    {
        if (string.IsNullOrWhiteSpace(faultCode))
        {
            return false;
        }

        return faultCode.Trim().ToLowerInvariant() switch
        {
            "none" or "ok" or "0" => false,
            _ => true
        };
    }

    public async Task<MiddlewareGunRuntimeSnapshot?> GetGunRuntimeStatusAsync(int gunNumber, CancellationToken cancellationToken = default)
    {
        if (gunNumber is < 1 or > 8)
        {
            return null;
        }

        using var response = await _httpClient.GetAsync($"/api/v1/guns/{gunNumber}/plc-status", cancellationToken);
        if (!response.IsSuccessStatusCode)
        {
            return null;
        }

        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken);
        var root = document.RootElement;

        return new MiddlewareGunRuntimeSnapshot(
            GunNumber: gunNumber,
            State: root.GetString("state_name") ?? root.GetString("state") ?? "Unknown",
            CurrentAzimuth: root.GetDouble("current_azimuth") ?? FromTenths(root.GetDouble("current_azimuth_tenths")) ?? 0,
            CurrentElevation: root.GetDouble("current_elevation") ?? FromTenths(root.GetDouble("current_elevation_tenths")) ?? 0,
            AcceptedSequence: root.GetInt("accepted_sequence") ?? root.GetInt("accepted_sequence_low") ?? 0,
            ShotSequence: root.GetInt("shot_sequence") ?? root.GetInt("shot_sequence_low") ?? 0);
    }

    public async Task<MiddlewareModeChangeResult> SetGunModeAsync(int gunNumber, string mode, CancellationToken cancellationToken = default)
    {
        if (gunNumber is < 1 or > 8)
        {
            return new MiddlewareModeChangeResult(false, gunNumber, mode, null, null, null, "Invalid gun number.");
        }

        var normalizedMode = NormalizeMode(mode).ToLowerInvariant();
        if (normalizedMode is not ("auto" or "manual" or "maintenance"))
        {
            return new MiddlewareModeChangeResult(false, gunNumber, mode, null, null, null, "Invalid mode.");
        }

        using var response = await _httpClient.PutAsJsonAsync($"/api/v1/guns/{gunNumber}/mode", new { mode = normalizedMode }, cancellationToken);
        var text = await response.Content.ReadAsStringAsync(cancellationToken);
        JsonDocument? document = null;

        try
        {
            if (!string.IsNullOrWhiteSpace(text))
            {
                document = JsonDocument.Parse(text);
            }

            var root = document?.RootElement;
            var returnedMode = root?.GetString("mode");
            var reason = root?.GetString("reason");
            var error = root?.GetString("error") ?? root?.GetString("message");

            return new MiddlewareModeChangeResult(
                Success: response.IsSuccessStatusCode,
                GunNumber: gunNumber,
                RequestedMode: normalizedMode,
                Mode: returnedMode,
                PreviousMode: root?.GetString("previous_mode"),
                Reason: reason,
                Error: response.IsSuccessStatusCode ? null : error ?? reason ?? $"HTTP {(int)response.StatusCode}");
        }
        catch (JsonException ex)
        {
            _logger.LogWarning(ex, "Middleware mode response was not JSON.");
            return new MiddlewareModeChangeResult(
                Success: response.IsSuccessStatusCode,
                GunNumber: gunNumber,
                RequestedMode: normalizedMode,
                Mode: null,
                PreviousMode: null,
                Reason: null,
                Error: response.IsSuccessStatusCode ? null : text);
        }
        finally
        {
            document?.Dispose();
        }
    }

    public async Task<MiddlewareCommandResult> FireGunAsync(int gunNumber, CancellationToken cancellationToken = default)
    {
        if (gunNumber is < 1 or > 8)
        {
            return new MiddlewareCommandResult(false, gunNumber, "fire-request", null, null, null, "Invalid gun number.");
        }

        using var response = await _httpClient.PostAsync($"/api/v1/guns/{gunNumber}/fire-request", content: null, cancellationToken);
        var text = await response.Content.ReadAsStringAsync(cancellationToken);
        JsonDocument? document = null;

        try
        {
            if (!string.IsNullOrWhiteSpace(text))
            {
                document = JsonDocument.Parse(text);
            }

            var root = document?.RootElement;
            var error = root?.GetString("error") ?? root?.GetString("message");
            return new MiddlewareCommandResult(
                Success: response.IsSuccessStatusCode,
                GunNumber: gunNumber,
                Action: "fire-request",
                State: root?.GetString("state_name") ?? root?.GetString("state"),
                Mode: NormalizeMode(root?.GetString("mode")),
                Reason: root?.GetString("reason"),
                Error: response.IsSuccessStatusCode ? null : error ?? $"HTTP {(int)response.StatusCode}");
        }
        catch (JsonException ex)
        {
            _logger.LogWarning(ex, "Middleware fire response was not JSON.");
            return new MiddlewareCommandResult(
                Success: response.IsSuccessStatusCode,
                GunNumber: gunNumber,
                Action: "fire-request",
                State: null,
                Mode: null,
                Reason: null,
                Error: response.IsSuccessStatusCode ? null : text);
        }
        finally
        {
            document?.Dispose();
        }
    }

    public async Task<MiddlewareCommandResult> SubmitManualCommandAsync(
        int gunNumber,
        double targetAzimuth,
        double targetElevation,
        bool fireRequest,
        CancellationToken cancellationToken = default)
    {
        if (gunNumber is < 1 or > 8)
        {
            return new MiddlewareCommandResult(false, gunNumber, "manual-command", null, null, null, "Invalid gun number.");
        }

        using var response = await _httpClient.PostAsJsonAsync(
            $"/api/v1/guns/{gunNumber}/commands/manual",
            new
            {
                target_azimuth = targetAzimuth,
                target_elevation = targetElevation,
                fire_request = fireRequest,
                admin_override = false,
                safety_extended_permissive = false
            },
            cancellationToken);
        var text = await response.Content.ReadAsStringAsync(cancellationToken);
        JsonDocument? document = null;

        try
        {
            if (!string.IsNullOrWhiteSpace(text))
            {
                document = JsonDocument.Parse(text);
            }

            var root = document?.RootElement;
            var error = root?.GetString("error") ?? root?.GetString("message");
            return new MiddlewareCommandResult(
                Success: response.IsSuccessStatusCode,
                GunNumber: gunNumber,
                Action: fireRequest ? "manual-fire-command" : "manual-command",
                State: root?.GetString("state_name") ?? root?.GetString("state"),
                Mode: NormalizeMode(root?.GetString("mode")),
                Reason: root?.GetString("reason"),
                Error: response.IsSuccessStatusCode ? null : error ?? $"HTTP {(int)response.StatusCode}");
        }
        catch (JsonException ex)
        {
            _logger.LogWarning(ex, "Middleware manual command response was not JSON.");
            return new MiddlewareCommandResult(
                Success: response.IsSuccessStatusCode,
                GunNumber: gunNumber,
                Action: fireRequest ? "manual-fire-command" : "manual-command",
                State: null,
                Mode: null,
                Reason: null,
                Error: response.IsSuccessStatusCode ? null : text);
        }
        finally
        {
            document?.Dispose();
        }
    }

    public async Task<MiddlewareSafetySnapshot> GetSafetyAsync(CancellationToken cancellationToken = default)
    {
        using var response = await _httpClient.GetAsync("/api/v1/safety", cancellationToken);
        response.EnsureSuccessStatusCode();

        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken);
        return MapSafety(document.RootElement);
    }

    public async Task<MiddlewareSafetySnapshot> SetGlobalSafetyAsync(
        bool? globalEnable,
        bool? emergencyStop,
        bool? maintenanceMode,
        CancellationToken cancellationToken = default)
    {
        var request = new SafetyGlobalRequest(globalEnable, emergencyStop, maintenanceMode);
        using var response = await _httpClient.PutAsJsonAsync("/api/v1/safety/global", request, cancellationToken);
        response.EnsureSuccessStatusCode();

        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken);
        var root = document.RootElement;
        return root.TryGetProperty("safety", out var safetyElement) ? MapSafety(safetyElement) : MapSafety(root);
    }

    public async Task<MiddlewareSafetySnapshot> SetGunSafetyAsync(
        int gunNumber,
        bool? enabled,
        bool? maintenanceInhibit,
        bool? externalInhibit,
        CancellationToken cancellationToken = default)
    {
        if (gunNumber is < 1 or > 8)
        {
            throw new ArgumentOutOfRangeException(nameof(gunNumber), "Gun number must be between 1 and 8.");
        }

        var request = new SafetyGunRequest(enabled, maintenanceInhibit, externalInhibit);
        using var response = await _httpClient.PutAsJsonAsync($"/api/v1/safety/guns/{gunNumber}", request, cancellationToken);
        response.EnsureSuccessStatusCode();

        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken);
        var root = document.RootElement;
        return root.TryGetProperty("safety", out var safetyElement) ? MapSafety(safetyElement) : MapSafety(root);
    }

    public async Task<MiddlewareMaintenanceResult> RunMaintenanceActionAsync(
        string action,
        bool execute,
        string confirm,
        CancellationToken cancellationToken = default)
    {
        if (string.IsNullOrWhiteSpace(_maintenanceToken))
        {
            return new MiddlewareMaintenanceResult(
                Success: false,
                Enabled: false,
                Action: action,
                Executed: execute,
                Status: "not_configured",
                Error: "STONEHENGE_MAINTENANCE_TOKEN is not configured for the engineering app.",
                Stdout: null,
                Stderr: null);
        }

        using var request = new HttpRequestMessage(HttpMethod.Post, "/api/v1/maintenance/actions");
        request.Headers.Add("X-Stonehenge-Maintenance-Token", _maintenanceToken);
        request.Content = JsonContent.Create(new
        {
            action,
            execute,
            confirm
        });

        using var response = await _httpClient.SendAsync(request, cancellationToken);
        var text = await response.Content.ReadAsStringAsync(cancellationToken);

        try
        {
            using var document = JsonDocument.Parse(string.IsNullOrWhiteSpace(text) ? "{}" : text);
            var root = document.RootElement;
            var error = root.GetString("error");
            return new MiddlewareMaintenanceResult(
                Success: response.IsSuccessStatusCode,
                Enabled: root.GetBool("enabled") ?? false,
                Action: root.GetString("action") ?? action,
                Executed: root.GetBool("executed") ?? execute,
                Status: root.GetString("status") ?? $"HTTP {(int)response.StatusCode}",
                Error: response.IsSuccessStatusCode ? null : error ?? $"HTTP {(int)response.StatusCode}",
                Stdout: root.GetString("stdout"),
                Stderr: root.GetString("stderr"));
        }
        catch (JsonException ex)
        {
            _logger.LogWarning(ex, "Middleware maintenance response was not JSON.");
            return new MiddlewareMaintenanceResult(
                Success: response.IsSuccessStatusCode,
                Enabled: false,
                Action: action,
                Executed: execute,
                Status: response.IsSuccessStatusCode ? "accepted" : "failed",
                Error: response.IsSuccessStatusCode ? null : text,
                Stdout: response.IsSuccessStatusCode ? text : null,
                Stderr: null);
        }
    }

    private static double? FromTenths(double? value)
    {
        return value is null ? null : value / 10.0;
    }

    private async Task<List<MiddlewareGunSnapshot>> ApplyArbitrationModesAsync(List<MiddlewareGunSnapshot> guns, CancellationToken cancellationToken)
    {
        var mapped = new List<MiddlewareGunSnapshot>(guns.Count);
        foreach (var gun in guns)
        {
            var arbitrationMode = await TryGetArbitrationModeAsync(gun.GunNumber, cancellationToken);
            mapped.Add(string.IsNullOrWhiteSpace(arbitrationMode) ? gun : gun with { Mode = NormalizeMode(arbitrationMode) });
        }

        return mapped;
    }

    private static MiddlewareSafetySnapshot MapSafety(JsonElement root)
    {
        var permissiveGunCount = 0;
        var inhibitedGunCount = 0;
        var guns = new List<MiddlewareSafetyGunSnapshot>();
        if (root.TryGetProperty("guns", out var gunsElement) && gunsElement.ValueKind == JsonValueKind.Array)
        {
            foreach (var gun in gunsElement.EnumerateArray())
            {
                var gunNumber = gun.GetInt("gun_id") ?? gun.GetInt("id") ?? 0;
                if (gunNumber is < 1 or > 8)
                {
                    continue;
                }

                var firePermissive = gun.GetBool("fire_permissive") ?? false;
                var inhibitActive = gun.GetBool("inhibit_active") ?? false;
                if (gun.GetBool("fire_permissive") == true)
                {
                    permissiveGunCount += 1;
                }

                if (inhibitActive)
                {
                    inhibitedGunCount += 1;
                }

                guns.Add(new MiddlewareSafetyGunSnapshot(
                    GunNumber: gunNumber,
                    FirePermissive: firePermissive,
                    InhibitActive: inhibitActive,
                    InhibitReason: gun.GetString("inhibit_reason_name") ?? gun.GetString("inhibit_reason") ?? "none",
                    Enabled: ReadStatusBool(gun, "enabled_status") ?? gun.GetBool("enabled") ?? false,
                    MaintenanceInhibit: ReadStatusBool(gun, "maintenance_inhibit_status") ?? gun.GetBool("maintenance_inhibit") ?? false,
                    ExternalInhibit: ReadStatusBool(gun, "external_inhibit_status") ?? gun.GetBool("external_inhibit") ?? false));
            }
        }

        return new MiddlewareSafetySnapshot(
            Enabled: root.GetBool("enabled") ?? false,
            EnforcementEnabled: root.GetBool("enforcement_enabled") ?? false,
            Connected: root.GetBool("connected") ?? false,
            Fresh: root.GetBool("fresh") ?? false,
            Ready: root.GetBool("ready") ?? false,
            State: root.GetString("state") ?? "unknown",
            GlobalFirePermissive: root.GetBool("global_fire_permissive") ?? false,
            GlobalInhibitActive: root.GetBool("global_inhibit_active") ?? false,
            GlobalInhibitReason: root.GetString("global_inhibit_reason_name") ?? "unknown",
            FaultActive: root.GetBool("safety_fault_active") ?? false,
            FaultName: root.GetString("safety_fault_name") ?? "unknown",
            EmergencyStopActive: root.GetBool("emergency_stop_active") ?? false,
            MaintenanceModeActive: root.GetBool("maintenance_mode_active") ?? false,
            PermissiveGunCount: permissiveGunCount,
            InhibitedGunCount: inhibitedGunCount,
            ObservedAt: DateTimeOffset.Now,
            Guns: guns.OrderBy(gun => gun.GunNumber).ToArray());
    }

    private static bool? ReadStatusBool(JsonElement element, string propertyName)
    {
        var direct = element.GetBool(propertyName);
        if (direct is not null)
        {
            return direct;
        }

        var text = element.GetString(propertyName);
        if (string.IsNullOrWhiteSpace(text))
        {
            return null;
        }

        return text.Trim().ToLowerInvariant() switch
        {
            "enabled" or "enable" or "active" or "on" or "ready" or "permissive" => true,
            "disabled" or "disable" or "inactive" or "off" or "clear" or "cleared" => false,
            _ => null
        };
    }

    private async Task<string?> TryGetArbitrationModeAsync(int gunNumber, CancellationToken cancellationToken)
    {
        try
        {
            using var response = await _httpClient.GetAsync($"/api/v1/guns/{gunNumber}/mode", cancellationToken);
            if (!response.IsSuccessStatusCode)
            {
                return null;
            }

            await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
            using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken);
            return document.RootElement.GetString("mode");
        }
        catch (Exception ex) when (ex is HttpRequestException or TaskCanceledException or JsonException)
        {
            _logger.LogDebug(ex, "Unable to read arbitration mode for gun {GunNumber}; using status snapshot mode.", gunNumber);
            return null;
        }
    }

    private static string NormalizeMode(string? mode)
    {
        return string.IsNullOrWhiteSpace(mode) ? "unknown" : mode.Trim().ToLowerInvariant();
    }

    private static string BuildSafetyState(JsonElement gun)
    {
        if (gun.GetBool("safety_permissive") is false)
        {
            return "Interlocked";
        }

        if (gun.GetBool("admin_override_active") is true)
        {
            return "Override Active";
        }

        var hardLimitStatus = gun.GetString("hard_limit_status");
        if (!string.IsNullOrWhiteSpace(hardLimitStatus) && !string.Equals(hardLimitStatus, "OK", StringComparison.OrdinalIgnoreCase))
        {
            return hardLimitStatus;
        }

        return "Permissive";
    }
}

internal sealed record SafetyGlobalRequest(
    [property: JsonPropertyName("global_enable")] bool? GlobalEnable,
    [property: JsonPropertyName("emergency_stop")] bool? EmergencyStop,
    [property: JsonPropertyName("maintenance_mode")] bool? MaintenanceMode);

internal sealed record SafetyGunRequest(
    [property: JsonPropertyName("enabled")] bool? Enabled,
    [property: JsonPropertyName("maintenance_inhibit")] bool? MaintenanceInhibit,
    [property: JsonPropertyName("external_inhibit")] bool? ExternalInhibit);

internal static class JsonElementExtensions
{
    public static string? GetString(this JsonElement element, string propertyName)
    {
        return element.TryGetProperty(propertyName, out var value) && value.ValueKind is not JsonValueKind.Null
            ? value.ToString()
            : null;
    }

    public static int? GetInt(this JsonElement element, string propertyName)
    {
        if (!element.TryGetProperty(propertyName, out var value))
        {
            return null;
        }

        if (value.ValueKind == JsonValueKind.Number && value.TryGetInt32(out var number))
        {
            return number;
        }

        return int.TryParse(value.ToString(), out var parsed) ? parsed : null;
    }

    public static double? GetDouble(this JsonElement element, string propertyName)
    {
        if (!element.TryGetProperty(propertyName, out var value))
        {
            return null;
        }

        if (value.ValueKind == JsonValueKind.Number && value.TryGetDouble(out var number))
        {
            return number;
        }

        return double.TryParse(value.ToString(), out var parsed) ? parsed : null;
    }

    public static bool? GetBool(this JsonElement element, string propertyName)
    {
        if (!element.TryGetProperty(propertyName, out var value))
        {
            return null;
        }

        return value.ValueKind switch
        {
            JsonValueKind.True => true,
            JsonValueKind.False => false,
            JsonValueKind.String when bool.TryParse(value.GetString(), out var parsed) => parsed,
            JsonValueKind.Number when value.TryGetInt32(out var number) => number != 0,
            _ => null
        };
    }
}
