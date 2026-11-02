using Microsoft.Data.Sqlite;
using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class SqliteEngineeringCommandStore : IEngineeringCommandStore
{
    private readonly object _lock = new();
    private readonly string _databasePath;

    public SqliteEngineeringCommandStore(IConfiguration configuration)
    {
        _databasePath = configuration["STONEHENGE_APP_DB_PATH"] ?? "/data/stonehenge-engineering.db";
        Initialize();
    }

    public EngineeringCommandRecord? Get(int commandId)
    {
        lock (_lock)
        {
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                SELECT id,
                       gun_number,
                       source_panel_id,
                       submitted_by,
                       command_type,
                       status,
                       created_utc,
                       sanitized_payload,
                       related_maintenance_request_id
                FROM engineering_commands
                WHERE id = $id;
                """;
            command.Parameters.AddWithValue("$id", commandId);

            using var reader = command.ExecuteReader();
            return reader.Read() ? ReadCommand(reader) : null;
        }
    }

    public IReadOnlyList<EngineeringCommandRecord> List(int limit = 50)
    {
        lock (_lock)
        {
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                SELECT id,
                       gun_number,
                       source_panel_id,
                       submitted_by,
                       command_type,
                       status,
                       created_utc,
                       sanitized_payload,
                       related_maintenance_request_id
                FROM engineering_commands
                ORDER BY id ASC
                LIMIT $limit;
                """;
            command.Parameters.AddWithValue("$limit", Math.Clamp(limit, 1, 200));

            using var reader = command.ExecuteReader();
            var commands = new List<EngineeringCommandRecord>();
            while (reader.Read())
            {
                commands.Add(ReadCommand(reader));
            }

            return commands;
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
                CREATE TABLE IF NOT EXISTS engineering_commands (
                    id INTEGER PRIMARY KEY,
                    gun_number INTEGER NOT NULL,
                    source_panel_id INTEGER NOT NULL,
                    submitted_by TEXT NOT NULL,
                    command_type TEXT NOT NULL,
                    status TEXT NOT NULL,
                    created_utc TEXT NOT NULL,
                    sanitized_payload TEXT NOT NULL,
                    related_maintenance_request_id INTEGER NULL
                );

                CREATE INDEX IF NOT EXISTS ix_engineering_commands_gun_panel
                    ON engineering_commands (gun_number, source_panel_id);
                """;
            command.ExecuteNonQuery();

            SeedIfMissing(connection);
        }
    }

    private SqliteConnection OpenConnection()
    {
        var connection = new SqliteConnection($"Data Source={_databasePath}");
        connection.Open();
        return connection;
    }

    private static void SeedIfMissing(SqliteConnection connection)
    {
        var now = DateTimeOffset.UtcNow;
        var seeds = new[]
        {
            new EngineeringCommandRecord(
                1001,
                1,
                1,
                "engineering.user",
                "ManualAim",
                "Completed",
                now.AddMinutes(-35).UtcDateTime,
                "{\"targetAzimuth\":140.0,\"targetElevation\":35.0}",
                null),
            new EngineeringCommandRecord(
                1002,
                2,
                2,
                "engineering.admin",
                "MaintenanceJog",
                "Completed",
                now.AddMinutes(-22).UtcDateTime,
                "{\"jogDirection\":\"up\",\"durationMs\":450}",
                null),
            new EngineeringCommandRecord(
                1003,
                2,
                10,
                "engineering.admin",
                "AutoTrackPreview",
                "RejectedBySafety",
                now.AddMinutes(-12).UtcDateTime,
                "{\"fragmentId\":\"FRAG-MOCK-2A\",\"targetQuality\":\"training\"}",
                null)
        };

        foreach (var seed in seeds)
        {
            using var command = connection.CreateCommand();
            command.CommandText = """
                INSERT OR IGNORE INTO engineering_commands (
                    id,
                    gun_number,
                    source_panel_id,
                    submitted_by,
                    command_type,
                    status,
                    created_utc,
                    sanitized_payload,
                    related_maintenance_request_id
                )
                VALUES (
                    $id,
                    $gunNumber,
                    $sourcePanelId,
                    $submittedBy,
                    $commandType,
                    $status,
                    $createdUtc,
                    $sanitizedPayload,
                    $relatedMaintenanceRequestId
                );
                """;
            command.Parameters.AddWithValue("$id", seed.Id);
            command.Parameters.AddWithValue("$gunNumber", seed.GunNumber);
            command.Parameters.AddWithValue("$sourcePanelId", seed.SourcePanelId);
            command.Parameters.AddWithValue("$submittedBy", seed.SubmittedBy);
            command.Parameters.AddWithValue("$commandType", seed.CommandType);
            command.Parameters.AddWithValue("$status", seed.Status);
            command.Parameters.AddWithValue("$createdUtc", seed.CreatedUtc.ToString("O"));
            command.Parameters.AddWithValue("$sanitizedPayload", seed.SanitizedPayload);
            command.Parameters.AddWithValue(
                "$relatedMaintenanceRequestId",
                seed.RelatedMaintenanceRequestId.HasValue ? seed.RelatedMaintenanceRequestId.Value : DBNull.Value);
            command.ExecuteNonQuery();
        }
    }

    private static EngineeringCommandRecord ReadCommand(SqliteDataReader reader)
    {
        var createdUtc = DateTimeOffset
            .Parse(reader.GetString(6), null, System.Globalization.DateTimeStyles.AssumeUniversal)
            .UtcDateTime;

        return new EngineeringCommandRecord(
            Id: reader.GetInt32(0),
            GunNumber: reader.GetInt32(1),
            SourcePanelId: reader.GetInt32(2),
            SubmittedBy: reader.GetString(3),
            CommandType: reader.GetString(4),
            Status: reader.GetString(5),
            CreatedUtc: createdUtc,
            SanitizedPayload: reader.GetString(7),
            RelatedMaintenanceRequestId: reader.IsDBNull(8) ? null : reader.GetInt32(8));
    }
}
