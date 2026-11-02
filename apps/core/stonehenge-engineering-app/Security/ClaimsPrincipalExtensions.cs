using System.Security.Claims;

namespace Stonehenge.Engineering.App.Security;

public static class ClaimsPrincipalExtensions
{
    public static bool HasStonehengeGroup(this ClaimsPrincipal user, string group)
    {
        return user.Claims.Any(claim =>
            string.Equals(claim.Type, StonehengeAuth.GroupsClaim, StringComparison.OrdinalIgnoreCase) &&
            string.Equals(claim.Value, group, StringComparison.OrdinalIgnoreCase));
    }

    public static string StonehengeDisplayName(this ClaimsPrincipal user)
    {
        return user.FindFirstValue(StonehengeAuth.DisplayNameClaim)
            ?? user.Identity?.Name
            ?? "unauthenticated";
    }

    public static bool HasStonehengeAccess(this ClaimsPrincipal user, params string[] accessLevels)
    {
        var actual = user.FindFirstValue(StonehengeAuth.AccessLevelClaim);
        return !string.IsNullOrWhiteSpace(actual) &&
            accessLevels.Contains(actual, StringComparer.OrdinalIgnoreCase);
    }

    public static bool IsEngineeringMember(this ClaimsPrincipal user)
    {
        return user.HasClaim(
            StonehengeAuth.EngineeringMemberClaim,
            StonehengeAuth.EngineeringMemberValue);
    }
}
