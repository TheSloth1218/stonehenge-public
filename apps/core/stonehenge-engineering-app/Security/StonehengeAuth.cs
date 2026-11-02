namespace Stonehenge.Engineering.App.Security;

public static class StonehengeAuth
{
    public const string DevScheme = "StonehengeDev";
    public const string CookieScheme = "StonehengeCookie";
    public const string OidcScheme = "StonehengeOidc";
    public const string SmartChallengeScheme = "StonehengeChallenge";

    public const string GroupsClaim = "groups";
    public const string DisplayNameClaim = "display_name";
    public const string AccessLevelClaim = "access_level";
    public const string EngineeringMemberClaim = "engineering_member";
    public const string EngineeringMemberValue = "true";
    public const string TrainingSessionClaim = "stonehenge_training_session";
    public const string TrainingFindingClaim = "stonehenge_training_finding";

    public const string EmployeesGroup = "STONEHENGE_EMPLOYEES";
    public const string EngineeringUsersGroup = "STONEHENGE_ENGINEERING_USERS";
    public const string EngineeringAdminsGroup = "STONEHENGE_ENGINEERING_ADMINS";

    public const string AuthenticatedUserPolicy = "AuthenticatedUser";
    public const string DashboardAccessPolicy = "DashboardAccess";
    public const string EngineeringAccessPolicy = "EngineeringAccess";
    public const string EngineeringAdminAccessPolicy = "EngineeringAdminAccess";
    public const string InternalWorkflowAccessPolicy = "InternalWorkflowAccess";

    public const string InternalServiceHeader = "X-Stonehenge-Internal-Service";
    public const string EngineeringAdminProxyHeader = "X-Stonehenge-Engineering-Admin";
    public const string DevelopmentInternalServiceToken = "dev-internal-api";

    public const string DashboardAccessLevel = "Dashboard";
    public const string EngineeringAccessLevel = "Engineering";
    public const string EngineeringAdminAccessLevel = "EngineeringAdmin";
}
