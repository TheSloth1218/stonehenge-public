using Microsoft.Data.Sqlite;

namespace Stonehenge.Engineering.App.Security;

public interface ITrainingSessionRegistry
{
    bool IsActive(string sessionId, DateTimeOffset now);
    void Register(string sessionId, string subject, DateTimeOffset expiresUtc, string findingId);
    void Revoke(string sessionId);
    void RevokeSubject(string subject);
    bool TryConsumeReplay(string replayId, DateTimeOffset expiresUtc);
}

public sealed class TrainingSessionRegistry : ITrainingSessionRegistry
{
    private readonly string _databasePath;
    private readonly object _sync = new();

    public TrainingSessionRegistry(IConfiguration configuration)
    {
        _databasePath = configuration["STONEHENGE_APP_DB_PATH"] ?? "/data/stonehenge-engineering.db";
        var directory = Path.GetDirectoryName(_databasePath);
        if (!string.IsNullOrWhiteSpace(directory)) Directory.CreateDirectory(directory);
        using var connection = Open();
        using var command = connection.CreateCommand();
        command.CommandText = """
            CREATE TABLE IF NOT EXISTS training_sessions (
                session_id TEXT PRIMARY KEY,
                subject TEXT NOT NULL,
                expires_utc TEXT NOT NULL,
                finding_id TEXT NOT NULL,
                revoked_utc TEXT NULL
            );
            CREATE INDEX IF NOT EXISTS ix_training_sessions_subject ON training_sessions(subject);
            CREATE TABLE IF NOT EXISTS training_token_replays (
                replay_id TEXT PRIMARY KEY,
                expires_utc TEXT NOT NULL,
                consumed_utc TEXT NOT NULL
            );
            """;
        command.ExecuteNonQuery();
    }

    public bool IsActive(string sessionId, DateTimeOffset now)
    {
        lock (_sync)
        {
            using var connection = Open();
            using var command = connection.CreateCommand();
            command.CommandText = "SELECT expires_utc, revoked_utc FROM training_sessions WHERE session_id=$id";
            command.Parameters.AddWithValue("$id", sessionId);
            using var reader = command.ExecuteReader();
            return reader.Read() && reader.IsDBNull(1) && DateTimeOffset.Parse(reader.GetString(0)) > now;
        }
    }

    public void Register(string sessionId, string subject, DateTimeOffset expiresUtc, string findingId)
    {
        lock (_sync)
        {
            using var connection = Open();
            using var command = connection.CreateCommand();
            command.CommandText = "INSERT INTO training_sessions(session_id,subject,expires_utc,finding_id) VALUES($id,$subject,$expires,$finding)";
            command.Parameters.AddWithValue("$id", sessionId);
            command.Parameters.AddWithValue("$subject", subject);
            command.Parameters.AddWithValue("$expires", expiresUtc.ToString("O"));
            command.Parameters.AddWithValue("$finding", findingId);
            command.ExecuteNonQuery();
        }
    }

    public void Revoke(string sessionId) => RevokeWhere("session_id=$value", sessionId);
    public void RevokeSubject(string subject) => RevokeWhere("subject=$value", subject);

    public bool TryConsumeReplay(string replayId, DateTimeOffset expiresUtc)
    {
        lock (_sync)
        {
            using var connection = Open();
            using var cleanup = connection.CreateCommand();
            cleanup.CommandText = "DELETE FROM training_token_replays WHERE expires_utc <= $now";
            cleanup.Parameters.AddWithValue("$now", DateTimeOffset.UtcNow.ToString("O"));
            cleanup.ExecuteNonQuery();
            using var command = connection.CreateCommand();
            command.CommandText = "INSERT OR IGNORE INTO training_token_replays(replay_id,expires_utc,consumed_utc) VALUES($id,$expires,$now)";
            command.Parameters.AddWithValue("$id", replayId);
            command.Parameters.AddWithValue("$expires", expiresUtc.ToString("O"));
            command.Parameters.AddWithValue("$now", DateTimeOffset.UtcNow.ToString("O"));
            return command.ExecuteNonQuery() == 1;
        }
    }

    private void RevokeWhere(string predicate, string value)
    {
        lock (_sync)
        {
            using var connection = Open();
            using var command = connection.CreateCommand();
            command.CommandText = $"UPDATE training_sessions SET revoked_utc=$now WHERE {predicate} AND revoked_utc IS NULL";
            command.Parameters.AddWithValue("$now", DateTimeOffset.UtcNow.ToString("O"));
            command.Parameters.AddWithValue("$value", value);
            command.ExecuteNonQuery();
        }
    }

    private SqliteConnection Open()
    {
        var connection = new SqliteConnection($"Data Source={_databasePath}");
        connection.Open();
        return connection;
    }
}
