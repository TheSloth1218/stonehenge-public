using Microsoft.Data.Sqlite;
using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class SqliteMaintenanceRequestStore : IMaintenanceRequestStore
{
    private readonly object _lock = new();
    private readonly string _databasePath;

    public SqliteMaintenanceRequestStore(IConfiguration configuration)
    {
        _databasePath = configuration["STONEHENGE_APP_DB_PATH"] ?? "/data/stonehenge-engineering.db";
        Initialize();
    }

    public IReadOnlyList<MaintenanceRequestViewModel> List()
    {
        lock (_lock)
        {
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                SELECT id,
                       gun_number,
                       gun_name,
                       request_type,
                       request_kind,
                       request_sub_type,
                       requested_by,
                       source_app,
                       asset_id,
                       panel_id,
                       status,
                       notes,
                       requested_at_utc,
                       reviewed_at_utc,
                       reviewed_by,
                       review_notes
                FROM maintenance_requests
                ORDER BY datetime(requested_at_utc) DESC, id DESC;
                """;

            using var reader = command.ExecuteReader();
            var requests = new List<MaintenanceRequestViewModel>();
            while (reader.Read())
            {
                requests.Add(ReadRequest(reader));
            }

            return requests;
        }
    }

    public MaintenanceRequestViewModel Submit(
        int gunNumber,
        string notes,
        string requestedBy,
        string requestType = "Maintenance",
        string sourceApp = "engineering-app",
        string? assetId = null,
        int? panelId = null,
        string? requestSubType = null)
    {
        lock (_lock)
        {
            var requestedAt = DateTimeOffset.UtcNow;
            var normalizedType = NormalizeRequestType(requestType);
            var requestKind = RequestKindFor(normalizedType);
            var normalizedSubType = string.IsNullOrWhiteSpace(requestSubType)
                ? DefaultSubType(normalizedType)
                : requestSubType.Trim();
            var requestNotes = string.IsNullOrWhiteSpace(notes)
                ? DefaultNotes(normalizedType)
                : notes.Trim();
            var normalizedAssetId = string.IsNullOrWhiteSpace(assetId) ? $"GUN-{gunNumber}" : assetId.Trim();

            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                INSERT INTO maintenance_requests (
                    gun_number,
                    gun_name,
                    request_type,
                    request_kind,
                    request_sub_type,
                    requested_by,
                    source_app,
                    asset_id,
                    panel_id,
                    status,
                    notes,
                    requested_at_utc
                )
                VALUES (
                    $gunNumber,
                    $gunName,
                    $requestType,
                    $requestKind,
                    $requestSubType,
                    $requestedBy,
                    $sourceApp,
                    $assetId,
                    $panelId,
                    'Pending',
                    $notes,
                    $requestedAtUtc
                )
                RETURNING id,
                          gun_number,
                          gun_name,
                          request_type,
                          request_kind,
                          request_sub_type,
                          requested_by,
                          source_app,
                          asset_id,
                          panel_id,
                          status,
                          notes,
                          requested_at_utc,
                          reviewed_at_utc,
                          reviewed_by,
                          review_notes;
                """;
            command.Parameters.AddWithValue("$gunNumber", gunNumber);
            command.Parameters.AddWithValue("$gunName", $"Gun {gunNumber}");
            command.Parameters.AddWithValue("$requestType", normalizedType);
            command.Parameters.AddWithValue("$requestKind", requestKind);
            command.Parameters.AddWithValue("$requestSubType", normalizedSubType);
            command.Parameters.AddWithValue("$requestedBy", requestedBy);
            command.Parameters.AddWithValue("$sourceApp", string.IsNullOrWhiteSpace(sourceApp) ? "engineering-app" : sourceApp.Trim());
            command.Parameters.AddWithValue("$assetId", normalizedAssetId);
            command.Parameters.AddWithValue("$panelId", panelId.HasValue ? panelId.Value : DBNull.Value);
            command.Parameters.AddWithValue("$notes", requestNotes);
            command.Parameters.AddWithValue("$requestedAtUtc", requestedAt.ToString("O"));

            using var reader = command.ExecuteReader();
            if (!reader.Read())
            {
                throw new InvalidOperationException("SQLite did not return the submitted maintenance request.");
            }

            return ReadRequest(reader);
        }
    }

    public MaintenanceRequestViewModel? Approve(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Approved", reviewedBy, reviewNotes);
    }

    public MaintenanceRequestViewModel? Deny(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Denied", reviewedBy, reviewNotes);
    }

    public MaintenanceRequestViewModel? Cancel(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Canceled", reviewedBy, reviewNotes, pendingOnly: true);
    }

    public MaintenanceRequestViewModel? Complete(int requestId, string reviewedBy, string reviewNotes)
    {
        return Review(requestId, "Completed", reviewedBy, reviewNotes);
    }

    private MaintenanceRequestViewModel? Review(int requestId, string status, string reviewedBy, string reviewNotes, bool pendingOnly = false)
    {
        lock (_lock)
        {
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                UPDATE maintenance_requests
                SET status = $status,
                    reviewed_by = $reviewedBy,
                    reviewed_at_utc = $reviewedAtUtc,
                    review_notes = $reviewNotes
                WHERE id = $id
                  AND ($pendingOnly = 0 OR status = 'Pending')
                RETURNING id,
                          gun_number,
                          gun_name,
                          request_type,
                          request_kind,
                          request_sub_type,
                          requested_by,
                          source_app,
                          asset_id,
                          panel_id,
                          status,
                          notes,
                          requested_at_utc,
                          reviewed_at_utc,
                          reviewed_by,
                          review_notes;
                """;
            command.Parameters.AddWithValue("$id", requestId);
            command.Parameters.AddWithValue("$status", status);
            command.Parameters.AddWithValue("$reviewedBy", reviewedBy);
            command.Parameters.AddWithValue("$reviewedAtUtc", DateTimeOffset.UtcNow.ToString("O"));
            command.Parameters.AddWithValue("$reviewNotes", reviewNotes);
            command.Parameters.AddWithValue("$pendingOnly", pendingOnly ? 1 : 0);

            using var reader = command.ExecuteReader();
            return reader.Read() ? ReadRequest(reader) : null;
        }
    }

    private void Initialize()
    {
        var directory = Path.GetDirectoryName(_databasePath);
        if (!string.IsNullOrWhiteSpace(directory))
        {
            Directory.CreateDirectory(directory);
        }

        lock (_lock)
        {
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                CREATE TABLE IF NOT EXISTS maintenance_requests (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    gun_number INTEGER NOT NULL,
                    gun_name TEXT NOT NULL,
                    request_type TEXT NOT NULL DEFAULT 'Maintenance',
                    request_kind TEXT NOT NULL DEFAULT 'MaintenanceMode',
                    request_sub_type TEXT NOT NULL DEFAULT 'EnableMaintenance',
                    requested_by TEXT NOT NULL,
                    source_app TEXT NOT NULL DEFAULT 'engineering-app',
                    asset_id TEXT NOT NULL DEFAULT '',
                    panel_id INTEGER NULL,
                    status TEXT NOT NULL,
                    notes TEXT NOT NULL,
                    requested_at_utc TEXT NOT NULL,
                    reviewed_at_utc TEXT NULL,
                    reviewed_by TEXT NULL,
                    review_notes TEXT NULL
                );

                CREATE INDEX IF NOT EXISTS ix_maintenance_requests_status_requested
                    ON maintenance_requests (status, requested_at_utc DESC, id DESC);
                """;
            command.ExecuteNonQuery();

            EnsureColumn(connection, "request_type", "TEXT NOT NULL DEFAULT 'Maintenance'");
            EnsureColumn(connection, "request_kind", "TEXT NOT NULL DEFAULT 'MaintenanceMode'");
            EnsureColumn(connection, "request_sub_type", "TEXT NOT NULL DEFAULT 'EnableMaintenance'");
            EnsureColumn(connection, "source_app", "TEXT NOT NULL DEFAULT 'engineering-app'");
            EnsureColumn(connection, "asset_id", "TEXT NOT NULL DEFAULT ''");
            EnsureColumn(connection, "panel_id", "INTEGER NULL");
            BackfillSimpleWorkflowColumns(connection);
            SeedSimpleWorkflowRequests(connection);
        }
    }

    private SqliteConnection OpenConnection()
    {
        var connection = new SqliteConnection($"Data Source={_databasePath}");
        connection.Open();
        return connection;
    }

    private static void EnsureColumn(SqliteConnection connection, string columnName, string definition)
    {
        using var command = connection.CreateCommand();
        command.CommandText = "PRAGMA table_info(maintenance_requests);";
        using var reader = command.ExecuteReader();
        while (reader.Read())
        {
            if (string.Equals(reader.GetString(1), columnName, StringComparison.OrdinalIgnoreCase))
            {
                return;
            }
        }

        using var alterCommand = connection.CreateCommand();
        alterCommand.CommandText = $"ALTER TABLE maintenance_requests ADD COLUMN {columnName} {definition};";
        alterCommand.ExecuteNonQuery();
    }

    private static MaintenanceRequestViewModel ReadRequest(SqliteDataReader reader)
    {
        var requestedAt = DateTimeOffset
            .Parse(reader.GetString(12), null, System.Globalization.DateTimeStyles.AssumeUniversal)
            .LocalDateTime;
        DateTime? reviewedAt = reader.IsDBNull(13)
            ? null
            : DateTimeOffset
                .Parse(reader.GetString(13), null, System.Globalization.DateTimeStyles.AssumeUniversal)
                .LocalDateTime;

        return new MaintenanceRequestViewModel(
            Id: reader.GetInt32(0),
            GunNumber: reader.GetInt32(1),
            GunName: reader.GetString(2),
            RequestType: reader.GetString(3),
            RequestKind: reader.GetString(4),
            RequestSubType: reader.GetString(5),
            RequestedBy: reader.GetString(6),
            SourceApp: reader.GetString(7),
            AssetId: string.IsNullOrWhiteSpace(reader.GetString(8)) ? $"GUN-{reader.GetInt32(1)}" : reader.GetString(8),
            PanelId: reader.IsDBNull(9) ? null : reader.GetInt32(9),
            Status: reader.GetString(10),
            Notes: reader.GetString(11),
            RequestedAt: requestedAt,
            ReviewedAt: reviewedAt,
            ReviewedBy: reader.IsDBNull(14) ? null : reader.GetString(14),
            ReviewNotes: reader.IsDBNull(15) ? null : reader.GetString(15),
            SafeRouteHint: reader.GetString(4) == "PartRequest"
                ? $"/api/part-requests/{reader.GetInt32(0)}"
                : $"/api/maintenance/requests/{reader.GetInt32(0)}");
    }

        private static void BackfillSimpleWorkflowColumns(SqliteConnection connection)
        {
            using var command = connection.CreateCommand();
            command.CommandText = """
                UPDATE maintenance_requests
                SET request_kind = CASE WHEN request_type = 'Part' THEN 'PartRequest' ELSE 'MaintenanceMode' END,
                    request_sub_type = CASE WHEN request_type = 'Part' THEN 'RequestPart' ELSE 'EnableMaintenance' END,
                    source_app = CASE
                        WHEN requested_by LIKE '%internal%' THEN 'internal-api'
                        ELSE COALESCE(NULLIF(source_app, ''), 'engineering-app')
                    END,
                    asset_id = CASE
                        WHEN asset_id IS NULL OR asset_id = '' THEN 'GUN-' || gun_number
                        ELSE asset_id
                    END,
                    panel_id = COALESCE(panel_id, gun_number)
                WHERE request_kind IS NULL
                   OR request_sub_type IS NULL
                   OR source_app IS NULL
                   OR asset_id IS NULL
                   OR asset_id = '';
                """;
            command.ExecuteNonQuery();

            using var normalizeSeedCommand = connection.CreateCommand();
            normalizeSeedCommand.CommandText = """
                UPDATE maintenance_requests
                SET requested_by = 'engineering.user',
                    source_app = 'engineering-app',
                    notes = 'Seeded request breadcrumb: ' || CASE
                        WHEN request_kind = 'PartRequest' THEN '/api/part-requests/' || id
                        ELSE '/api/maintenance/requests/' || id
                    END
                WHERE id IN (2001, 2002, 2003)
                  AND (requested_by <> 'engineering.user' OR source_app <> 'engineering-app' OR notes LIKE 'Seeded workflow breadcrumb:%');
                """;
            normalizeSeedCommand.ExecuteNonQuery();
        }

        private static void SeedSimpleWorkflowRequests(SqliteConnection connection)
        {
            var now = DateTimeOffset.UtcNow;
            var seeds = new[]
            {
                new { Id = 2001, Gun = 1, Kind = "MaintenanceMode", Type = "Maintenance", SubType = "EnableMaintenance", Status = "Pending", Route = "/api/maintenance/requests/2001" },
                new { Id = 2002, Gun = 2, Kind = "MaintenanceMode", Type = "Maintenance", SubType = "EnableMaintenance", Status = "PendingAdminApproval", Route = "/api/maintenance/requests/2002" },
                new { Id = 2003, Gun = 2, Kind = "PartRequest", Type = "Part", SubType = "RequestPart", Status = "Pending", Route = "/api/part-requests/2003" }
            };

            foreach (var seed in seeds)
            {
                using var command = connection.CreateCommand();
                command.CommandText = """
                    INSERT OR IGNORE INTO maintenance_requests (
                        id,
                        gun_number,
                        gun_name,
                        request_type,
                        request_kind,
                        request_sub_type,
                        requested_by,
                        source_app,
                        asset_id,
                        panel_id,
                        status,
                        notes,
                        requested_at_utc
                    )
                    VALUES (
                        $id,
                        $gunNumber,
                        $gunName,
                        $requestType,
                        $requestKind,
                        $requestSubType,
                        'engineering.user',
                        'engineering-app',
                        $assetId,
                        $panelId,
                        $status,
                        $notes,
                        $requestedAtUtc
                    );
                    """;
                command.Parameters.AddWithValue("$id", seed.Id);
                command.Parameters.AddWithValue("$gunNumber", seed.Gun);
                command.Parameters.AddWithValue("$gunName", $"Gun {seed.Gun}");
                command.Parameters.AddWithValue("$requestType", seed.Type);
                command.Parameters.AddWithValue("$requestKind", seed.Kind);
                command.Parameters.AddWithValue("$requestSubType", seed.SubType);
                command.Parameters.AddWithValue("$assetId", $"GUN-{seed.Gun}");
                command.Parameters.AddWithValue("$panelId", seed.Gun);
                command.Parameters.AddWithValue("$status", seed.Status);
                command.Parameters.AddWithValue("$notes", $"Seeded request breadcrumb: {seed.Route}");
                command.Parameters.AddWithValue("$requestedAtUtc", now.AddMinutes(-seed.Id % 100).ToString("O"));
                command.ExecuteNonQuery();
            }
        }

        private static string NormalizeRequestType(string requestType)
        {
            return string.Equals(requestType, "Part", StringComparison.OrdinalIgnoreCase)
                ? "Part"
                : "Maintenance";
        }

        private static string RequestKindFor(string requestType)
        {
            return requestType == "Part" ? "PartRequest" : "MaintenanceMode";
        }

        private static string DefaultSubType(string requestType)
        {
            return requestType == "Part" ? "RequestPart" : "EnableMaintenance";
        }

    private static string DefaultNotes(string requestType)
    {
        return requestType == "Part"
            ? "Part service request submitted from maintenance panel."
            : "Maintenance requested from operator panel.";
    }
}
