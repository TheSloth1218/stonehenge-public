using System.Security.Cryptography;
using System.Text;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Options;
using Stonehenge.Engineering.App.Security;

namespace Stonehenge.Engineering.App.Controllers;

[ApiController]
public sealed class TrainingRecoveryController : ControllerBase
{
    private readonly TrainingSecurityOptions _training;
    private readonly IRecoveryTokenValidator _validator;
    private readonly ITrainingSessionRegistry _sessions;
    private readonly ITrainingTicketIssuer _issuer;
    private readonly ILogger<TrainingRecoveryController> _logger;

    public TrainingRecoveryController(IOptions<TrainingSecurityOptions> training, IRecoveryTokenValidator validator,
        ITrainingSessionRegistry sessions, ITrainingTicketIssuer issuer, ILogger<TrainingRecoveryController> logger)
    {
        _training = training.Value;
        _validator = validator;
        _sessions = sessions;
        _issuer = issuer;
        _logger = logger;
    }

    [HttpPost("/training/recovery/dex")]
    public async Task<IActionResult> Recover([FromBody] RecoveryRequest? request, CancellationToken cancellationToken)
    {
        if (!_training.Enabled) return NotFound();
        var correlation = HttpContext.TraceIdentifier;
        var tokenHash = Hash(request?.IdToken ?? string.Empty);
        var result = await _validator.ValidateAsync(request?.IdToken ?? string.Empty, cancellationToken);
        if (!result.Succeeded)
        {
            _logger.LogWarning("Recovery denied TokenHash={TokenHash} FindingId={FindingId} CorrelationId={CorrelationId} Reason={Reason}", tokenHash, "FCU-AUTH-001", correlation, result.Reason);
            return Unauthorized(new { ok = false, reason = result.Reason, correlation });
        }
        if (!_sessions.TryConsumeReplay(result.ReplayId!, result.ExpiresUtc))
        {
            _logger.LogWarning("Recovery denied TokenHash={TokenHash} FindingId={FindingId} CorrelationId={CorrelationId} Reason={Reason}", tokenHash, "FCU-AUTH-001", correlation, "replay");
            return Conflict(new { ok = false, reason = "replay", correlation });
        }

        var sessionId = Guid.NewGuid().ToString("N");
        var expires = DateTimeOffset.UtcNow.Add(_training.SessionLifetime);
        _sessions.Register(sessionId, "swordsman", expires, "FCU-AUTH-001");
        var principal = _issuer.CreateRecoveryPrincipal(sessionId);
        await HttpContext.SignInAsync(StonehengeAuth.CookieScheme, principal,
            new AuthenticationProperties { IssuedUtc = DateTimeOffset.UtcNow, ExpiresUtc = expires, IsPersistent = false, AllowRefresh = false });
        _logger.LogWarning("Recovery accepted TokenHash={TokenHash} SessionHash={SessionHash} ClaimNames={ClaimNames} FindingId={FindingId} CorrelationId={CorrelationId}",
            tokenHash, Hash(sessionId), result.ClaimNames, "FCU-AUTH-001", correlation);
        return Ok(new { ok = true, subject = "swordsman", access = "EngineeringUser", expiresUtc = expires, correlation });
    }

    private static string Hash(string value) => Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(value))).ToLowerInvariant();
}

public sealed record RecoveryRequest(string? IdToken);
