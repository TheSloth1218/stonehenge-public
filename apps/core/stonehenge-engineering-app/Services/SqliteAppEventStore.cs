using Microsoft.Data.Sqlite;
using Stonehenge.Engineering.App.Models;

namespace Stonehenge.Engineering.App.Services;

public sealed class SqliteAppEventStore : IAppEventStore
{
    private const int MaxEventsPerCategory = 500;
    private readonly object _lock = new();
    private readonly string _databasePath;

    public SqliteAppEventStore(IConfiguration configuration)
    {
        _databasePath = configuration["STONEHENGE_APP_DB_PATH"] ?? "/data/stonehenge-engineering.db";
        Initialize();
    }

    public IReadOnlyList<OperationLogEntry> List(string category, int limit = 20)
    {
        lock (_lock)
        {
            using var connection = OpenConnection();
            using var command = connection.CreateCommand();
            command.CommandText = """
                SELECT id, timestamp_utc, source, action, result, notes
                FROM app_events
                WHERE category = $category
                ORDER BY datetime(timestamp_utc) DESC, id DESC
                LIMIT $limit;
                """;
            command.Parameters.AddWithValue("$category", category);
            command.Parameters.AddWithValue("$limit", Math.Clamp(limit, 1, 100));

            using var reader = command.ExecuteReader();
            var events = new List<OperationLogEntry>();
            while (reader.Read())
            {
                var timestamp = DateTimeOffset
                    .Parse(reader.GetString(1), null, System.Globalization.DateTimeStyles.AssumeUniversal)
                    .LocalDateTime;

                events.Add(new OperationLogEntry(
                    reader.GetInt32(0),
                    timestamp,
                    reader.GetString(2),
                    reader.GetString(3),
                    reader.GetString(4),
                    reader.GetString(5)));
            }

            return events;
        }
    }

    public OperationLogEntry Add(string category, string source, string action, string result, string notes)
    {
        lock (_lock)
        {
            var timestamp = DateTimeOffset.UtcNow;
            using var connection = OpenConnection();
            using var transaction = connection.BeginTransaction();
            using var command = connection.CreateCommand();
            command.Transaction = transaction;
            command.CommandText = """
                INSERT INTO app_events (category, timestamp_utc, source, action, result, notes)
                VALUES ($category, $timestampUtc, $source, $action, $result, $notes);

                SELECT last_insert_rowid();
                """;
            command.Parameters.AddWithValue("$category", category);
            command.Parameters.AddWithValue("$timestampUtc", timestamp.ToString("O"));
            command.Parameters.AddWithValue("$source", source);
            command.Parameters.AddWithValue("$action", action);
            command.Parameters.AddWithValue("$result", result);
            command.Parameters.AddWithValue("$notes", notes);

            var id = Convert.ToInt32(command.ExecuteScalar());
            PruneCategory(connection, transaction, category);
            transaction.Commit();

            return new OperationLogEntry(id, timestamp.LocalDateTime, source, action, result, notes);
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
                CREATE TABLE IF NOT EXISTS app_events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    category TEXT NOT NULL,
                    timestamp_utc TEXT NOT NULL,
                    source TEXT NOT NULL,
                    action TEXT NOT NULL,
                    result TEXT NOT NULL,
                    notes TEXT NOT NULL
                );

                CREATE INDEX IF NOT EXISTS ix_app_events_category_timestamp
                    ON app_events (category, timestamp_utc DESC, id DESC);
                """;
            command.ExecuteNonQuery();

            SeedAdminEventsIfEmpty(connection);
        }
    }

    private SqliteConnection OpenConnection()
    {
        var connection = new SqliteConnection($"Data Source={_databasePath}");
        connection.Open();
        return connection;
    }

    private static void SeedAdminEventsIfEmpty(SqliteConnection connection)
    {
        using var countCommand = connection.CreateCommand();
        countCommand.CommandText = "SELECT COUNT(*) FROM app_events WHERE category = 'admin';";
        var count = Convert.ToInt32(countCommand.ExecuteScalar());
        if (count > 0)
        {
            return;
        }

        var now = DateTimeOffset.UtcNow;
        var seedEvents = new[]
        {
            (now, "Engineering App", "Admin panel viewed", "Visual", "Admin workspace initialized"),
            (now.AddMinutes(-2), "Maintenance Requests", "Pending requests listed", "Observed", "App-owned requests loaded"),
            (now.AddMinutes(-5), "Admin Boundary", "Portal integration pending", "Planned", "Portal claims will gate this panel later")
        };

        foreach (var seed in seedEvents)
        {
            using var insertCommand = connection.CreateCommand();
            insertCommand.CommandText = """
                INSERT INTO app_events (category, timestamp_utc, source, action, result, notes)
                VALUES ('admin', $timestampUtc, $source, $action, $result, $notes);
                """;
            insertCommand.Parameters.AddWithValue("$timestampUtc", seed.Item1.ToString("O"));
            insertCommand.Parameters.AddWithValue("$source", seed.Item2);
            insertCommand.Parameters.AddWithValue("$action", seed.Item3);
            insertCommand.Parameters.AddWithValue("$result", seed.Item4);
            insertCommand.Parameters.AddWithValue("$notes", seed.Item5);
            insertCommand.ExecuteNonQuery();
        }
    }

    private static void PruneCategory(SqliteConnection connection, SqliteTransaction transaction, string category)
    {
        using var pruneCommand = connection.CreateCommand();
        pruneCommand.Transaction = transaction;
        pruneCommand.CommandText = """
            DELETE FROM app_events
            WHERE category = $category
              AND id NOT IN (
                  SELECT id
                  FROM app_events
                  WHERE category = $category
                  ORDER BY datetime(timestamp_utc) DESC, id DESC
                  LIMIT $limit
              );
            """;
        pruneCommand.Parameters.AddWithValue("$category", category);
        pruneCommand.Parameters.AddWithValue("$limit", MaxEventsPerCategory);
        pruneCommand.ExecuteNonQuery();
    }
}
