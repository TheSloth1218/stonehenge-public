using Microsoft.Data.Sqlite;
using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class SqlitePartStatusStore : IPartStatusStore
{
    private readonly object _lock = new();
    private readonly string _databasePath;

    public SqlitePartStatusStore(IConfiguration configuration)
    {
        _databasePath = configuration["STONEHENGE_APP_DB_PATH"] ?? "/data/stonehenge-engineering.db";
        Initialize();
    }

    public IReadOnlyDictionary<int, GunPartStatusRecord> ListLatest()
    {
        lock (_lock)
        {
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                SELECT gun_number, firing_count, service_threshold, part_status, part_class, updated_utc
                FROM gun_part_status
                ORDER BY gun_number ASC;
                """;

            using var reader = command.ExecuteReader();
            var records = new Dictionary<int, GunPartStatusRecord>();
            while (reader.Read())
            {
                var updated = DateTimeOffset
                    .Parse(reader.GetString(5), null, System.Globalization.DateTimeStyles.AssumeUniversal)
                    .LocalDateTime;

                records[reader.GetInt32(0)] = new GunPartStatusRecord(
                    reader.GetInt32(0),
                    reader.GetInt32(1),
                    reader.GetInt32(2),
                    reader.GetString(3),
                    reader.GetString(4),
                    updated);
            }

            return records;
        }
    }

    public GunPartStatusRecord Upsert(
        int gunNumber,
        int firingCount,
        int serviceThreshold,
        string partStatus,
        string partClass)
    {
        lock (_lock)
        {
            var updated = DateTimeOffset.UtcNow;
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                INSERT INTO gun_part_status (
                    gun_number,
                    firing_count,
                    service_threshold,
                    part_status,
                    part_class,
                    updated_utc
                )
                VALUES ($gunNumber, $firingCount, $serviceThreshold, $partStatus, $partClass, $updatedUtc)
                ON CONFLICT(gun_number) DO UPDATE SET
                    firing_count = excluded.firing_count,
                    service_threshold = excluded.service_threshold,
                    part_status = excluded.part_status,
                    part_class = excluded.part_class,
                    updated_utc = excluded.updated_utc;
                """;
            command.Parameters.AddWithValue("$gunNumber", gunNumber);
            command.Parameters.AddWithValue("$firingCount", firingCount);
            command.Parameters.AddWithValue("$serviceThreshold", serviceThreshold);
            command.Parameters.AddWithValue("$partStatus", partStatus);
            command.Parameters.AddWithValue("$partClass", partClass);
            command.Parameters.AddWithValue("$updatedUtc", updated.ToString("O"));
            command.ExecuteNonQuery();

            return new GunPartStatusRecord(
                gunNumber,
                firingCount,
                serviceThreshold,
                partStatus,
                partClass,
                updated.LocalDateTime);
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
                CREATE TABLE IF NOT EXISTS gun_part_status (
                    gun_number INTEGER PRIMARY KEY,
                    firing_count INTEGER NOT NULL,
                    service_threshold INTEGER NOT NULL,
                    part_status TEXT NOT NULL,
                    part_class TEXT NOT NULL,
                    updated_utc TEXT NOT NULL
                );
                """;
            command.ExecuteNonQuery();
        }
    }

    private SqliteConnection OpenConnection()
    {
        var connection = new SqliteConnection($"Data Source={_databasePath}");
        connection.Open();
        return connection;
    }
}
