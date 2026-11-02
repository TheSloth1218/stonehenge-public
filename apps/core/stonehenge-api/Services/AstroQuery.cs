namespace Stonehenge.Api.Services;

public sealed record AstroQuery(
    DateTimeOffset? Start,
    DateTimeOffset? End,
    DateTimeOffset? Since,
    int? GunId,
    int Limit,
    int Offset)
{
    private const int MaximumOffset = 10_000;

    public DateTimeOffset? EffectiveStart => Since ?? Start;

    public static AstroQuery Normalize(DateTimeOffset? start, DateTimeOffset? end, DateTimeOffset? since, int? gunId, int? limit, int? offset)
    {
        var boundedLimit = Math.Clamp(limit.GetValueOrDefault(100), 1, 500);
        var boundedOffset = Math.Clamp(offset.GetValueOrDefault(0), 0, MaximumOffset);
        var boundedGun = gunId is >= 1 and <= 8 ? gunId : null;
        return new AstroQuery(start, end, since, boundedGun, boundedLimit, boundedOffset);
    }
}
