using System.Security.Claims;
using System.Security.Cryptography;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.AspNetCore.DataProtection;
using Microsoft.Extensions.Options;

namespace Stonehenge.Engineering.App.Security;

public interface ITrainingTicketIssuer
{
    IssuedTrainingTicket IssueCount(bool reset, string correlationId);
    ClaimsPrincipal CreateRecoveryPrincipal(string sessionId);
}

public sealed record IssuedTrainingTicket(string CookieValue, string SessionId, string SessionHash, DateTimeOffset ExpiresUtc);

public sealed class TrainingTicketIssuer : ITrainingTicketIssuer
{
    private readonly IDataProtectionProvider _dataProtection;
    private readonly ITrainingSessionRegistry _sessions;
    private readonly TrainingSecurityOptions _options;
    private readonly ILogger<TrainingTicketIssuer> _logger;

    public TrainingTicketIssuer(IDataProtectionProvider dataProtection, ITrainingSessionRegistry sessions,
        IOptions<TrainingSecurityOptions> options, ILogger<TrainingTicketIssuer> logger)
    {
        _dataProtection = dataProtection;
        _sessions = sessions;
        _options = options.Value;
        _logger = logger;
    }

    public IssuedTrainingTicket IssueCount(bool reset, string correlationId)
    {
        if (!_options.Enabled) throw new InvalidOperationException("Training session issuance is disabled.");
        if (reset) _sessions.RevokeSubject("count");
        var sessionId = Guid.NewGuid().ToString("N");
        var issuedUtc = DateTimeOffset.UtcNow;
        var expiresUtc = issuedUtc.Add(_options.SessionLifetime);
        var principal = CountPrincipal(sessionId);
        var properties = new AuthenticationProperties { IssuedUtc = issuedUtc, ExpiresUtc = expiresUtc, IsPersistent = false, AllowRefresh = false };
        var ticket = new AuthenticationTicket(principal, properties, StonehengeAuth.CookieScheme);
        var protector = _dataProtection.CreateProtector(
            "Microsoft.AspNetCore.Authentication.Cookies.CookieAuthenticationMiddleware",
            StonehengeAuth.CookieScheme,
            "v2");
        var cookie = new TicketDataFormat(protector).Protect(ticket);
        _sessions.Register(sessionId, "count", expiresUtc, "FCU-ID-001");
        var hash = Convert.ToHexString(SHA256.HashData(System.Text.Encoding.UTF8.GetBytes(sessionId))).ToLowerInvariant();
        _logger.LogWarning("Training session issued SessionHash={SessionHash} ExpiresUtc={ExpiresUtc} FindingId={FindingId} CorrelationId={CorrelationId}", hash, expiresUtc, "FCU-ID-001", correlationId);
        return new(cookie, sessionId, hash, expiresUtc);
    }

    public ClaimsPrincipal CreateRecoveryPrincipal(string sessionId) => TrainingPrincipal("swordsman", sessionId, "FCU-AUTH-001");

    private static ClaimsPrincipal CountPrincipal(string sessionId) => TrainingPrincipal("count", sessionId, "FCU-ID-001");

    private static ClaimsPrincipal TrainingPrincipal(string subject, string sessionId, string finding)
    {
        var claims = new[]
        {
            new Claim("sub", subject), new Claim("alias", subject), new Claim(ClaimTypes.NameIdentifier, subject), new Claim(ClaimTypes.Name, subject),
            new Claim(StonehengeAuth.DisplayNameClaim, subject), new Claim(StonehengeAuth.GroupsClaim, StonehengeAuth.EngineeringUsersGroup),
            new Claim(StonehengeAuth.EngineeringMemberClaim, StonehengeAuth.EngineeringMemberValue),
            new Claim(StonehengeAuth.AccessLevelClaim, StonehengeAuth.EngineeringAccessLevel),
            new Claim(StonehengeAuth.TrainingSessionClaim, sessionId), new Claim(StonehengeAuth.TrainingFindingClaim, finding)
        };
        return new ClaimsPrincipal(new ClaimsIdentity(claims, StonehengeAuth.CookieScheme, ClaimTypes.Name, StonehengeAuth.GroupsClaim));
    }
}
