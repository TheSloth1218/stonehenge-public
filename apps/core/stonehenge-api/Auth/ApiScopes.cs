namespace Stonehenge.Api.Auth;

public static class ApiScopes
{
    public const string VendorPartsRead = "vendor.parts.read";
    public const string VendorGunsRead = "vendor.guns.read";
    public const string VendorPartsAcknowledge = "vendor.parts.acknowledge";
    public const string VendorPartsComplete = "vendor.parts.complete";
    public const string AstroHistoryRead = "astro.history.read";
    public const string AstroResultsRead = "astro.results.read";
    public const string AstroSummaryRead = "astro.summary.read";
    public const string SystemPublicStatusRead = "system.publicStatus.read";
    public const string EngineeringVendorCompletionsRead = "engineering.vendorCompletions.read";
    public const string EngineeringVendorCompletionsApply = "engineering.vendorCompletions.apply";
    public const string EngineeringPartRequestsWrite = "engineering.partRequests.write";
}
