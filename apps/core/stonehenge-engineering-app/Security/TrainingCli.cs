using System.Text.Json;

namespace Stonehenge.Engineering.App.Security;

public static class TrainingCli
{
    private const string CorrelationId = "NIGHTWATCH-ENG-1847";

    public static bool IsCommand(string[] args) => args.Length > 0 && args[0] == "training-session";

    public static int Run(string[] args, IServiceProvider services)
    {
        if (!IsRoot()) return 77;
        var action = args.ElementAtOrDefault(1);
        var outputIndex = Array.IndexOf(args, "--output");
        if (action is not ("mint" or "reset") || outputIndex < 0 || outputIndex + 1 >= args.Length) return 64;
        var output = Path.GetFullPath(args[outputIndex + 1]);
        var parent = Path.GetDirectoryName(output);
        if (string.IsNullOrWhiteSpace(parent) || !Directory.Exists(parent)) return 73;
        if (File.Exists(output) && new FileInfo(output).LinkTarget is not null) return 73;
        if (File.Exists(output)) File.SetUnixFileMode(output, UnixFileMode.UserRead | UnixFileMode.UserWrite);

        IssuedTrainingTicket? issued = null;
        try
        {
            var issuer = services.GetRequiredService<ITrainingTicketIssuer>();
            issued = issuer.IssueCount(action == "reset", CorrelationId);
            var artifact = JsonSerializer.SerializeToUtf8Bytes(new
            {
                schemaVersion = 1,
                subjectAlias = "count",
                accessLevel = "engineering.user",
                expiresAtUtc = issued.ExpiresUtc,
                cookieName = "__Host-Stonehenge.Auth.v3",
                cookieValue = issued.CookieValue,
                sessionId = issued.SessionId,
                correlationId = CorrelationId,
                trainingMarker = true
            });
            using var stream = new FileStream(output, new FileStreamOptions
            {
                Mode = FileMode.Create,
                Access = FileAccess.Write,
                Share = FileShare.None,
                Options = FileOptions.WriteThrough,
                UnixCreateMode = UnixFileMode.UserRead | UnixFileMode.UserWrite
            });
            stream.Write(artifact);
            stream.Flush(true);
            File.SetUnixFileMode(output, UnixFileMode.UserRead | UnixFileMode.UserWrite);
            return 0;
        }
        catch
        {
            if (issued is not null) services.GetRequiredService<ITrainingSessionRegistry>().Revoke(issued.SessionId);
            return 1;
        }
    }

    private static bool IsRoot()
    {
        try
        {
            return File.ReadLines("/proc/self/status")
                .FirstOrDefault(line => line.StartsWith("Uid:", StringComparison.Ordinal))?
                .Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries).ElementAtOrDefault(1) == "0";
        }
        catch { return false; }
    }
}
