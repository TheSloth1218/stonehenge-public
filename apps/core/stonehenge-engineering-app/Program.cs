using Stonehenge.Engineering.App.Models;
using Stonehenge.Engineering.App.Security;
using Stonehenge.Engineering.App.Services;

var builder = WebApplication.CreateBuilder(args);

builder.Services.AddControllersWithViews();
builder.Services.Configure<FuxaOptions>(builder.Configuration.GetSection("Fuxa"));
builder.Services.Configure<StonehengeApiOptions>(builder.Configuration.GetSection("StonehengeApi"));
builder.Services.Configure<OperationalOptions>(builder.Configuration.GetSection(OperationalOptions.SectionName));
builder.Services.Configure<TrainingSecurityOptions>(builder.Configuration.GetSection(TrainingSecurityOptions.SectionName));
builder.AddStonehengeAuthentication();
builder.Services.AddAuthorization(options =>
{
    options.AddPolicy(StonehengeAuth.AuthenticatedUserPolicy, policy =>
        policy.RequireAuthenticatedUser());
    options.AddPolicy(StonehengeAuth.DashboardAccessPolicy, policy =>
        policy.RequireAuthenticatedUser());
    options.AddPolicy(StonehengeAuth.EngineeringAccessPolicy, policy =>
        policy.RequireAuthenticatedUser()
            .RequireAssertion(context =>
                context.User.HasStonehengeAccess(
                    StonehengeAuth.EngineeringAccessLevel,
                    StonehengeAuth.EngineeringAdminAccessLevel)));
    options.AddPolicy(StonehengeAuth.EngineeringAdminAccessPolicy, policy =>
        policy.RequireAuthenticatedUser()
            .RequireAssertion(context =>
                context.User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel)));
    options.AddPolicy(StonehengeAuth.InternalWorkflowAccessPolicy, policy =>
        policy.RequireAssertion(context =>
        {
            // TODO: replace this development-only shared header with real service-to-service
            // authentication, such as mTLS, signed service tokens, or managed API keys.
            if (context.Resource is not HttpContext httpContext)
            {
                return false;
            }

            return string.Equals(
                httpContext.Request.Headers[StonehengeAuth.InternalServiceHeader].FirstOrDefault(),
                StonehengeAuth.DevelopmentInternalServiceToken,
                StringComparison.Ordinal);
        }));
});
builder.Services.AddHealthChecks();
builder.Services.AddSingleton<MockEngineeringDashboardData>();
builder.Services.AddSingleton<IMaintenanceRequestStore, SqliteMaintenanceRequestStore>();
builder.Services.AddSingleton<IAppEventStore, SqliteAppEventStore>();
builder.Services.AddSingleton<IPartStatusStore, SqlitePartStatusStore>();
builder.Services.AddSingleton<IEngineeringCommandStore, SqliteEngineeringCommandStore>();
builder.Services.AddSingleton<IEngineeringPanelCatalog, StaticEngineeringPanelCatalog>();
builder.Services.AddSingleton<IFuxaEmbedService, FuxaEmbedService>();
builder.Services.AddSingleton<ITrainingSessionRegistry, TrainingSessionRegistry>();
builder.Services.AddSingleton<ITrainingTicketIssuer, TrainingTicketIssuer>();
builder.Services.AddSingleton<IPanelCapabilityService, PanelCapabilityService>();
builder.Services.AddSingleton<IRecoveryTokenValidator, RecoveryTokenValidator>();
builder.Services.AddHttpClient("FuxaProxy", client =>
{
    var fuxaUrl = builder.Configuration["Fuxa:InternalBaseUrl"] ?? "http://fuxa:1881";
    client.BaseAddress = new Uri(fuxaUrl);
    var proxyToken = builder.Configuration["STONEHENGE_FUXA_PROXY_TOKEN"];
    if (!string.IsNullOrWhiteSpace(proxyToken))
    {
        client.DefaultRequestHeaders.Add("X-Stonehenge-Fuxa-Proxy-Token", proxyToken);
    }
    client.Timeout = TimeSpan.FromSeconds(10);
});
builder.Services.AddHttpClient<IStonehengeMiddlewareClient, StonehengeMiddlewareClient>(client =>
{
    var middlewareUrl = builder.Configuration["STONEHENGE_MIDDLEWARE_URL"] ?? "http://middleware:8080";
    client.BaseAddress = new Uri(middlewareUrl);
    var controlToken = builder.Configuration["STONEHENGE_CONTROL_TOKEN"];
    if (!string.IsNullOrWhiteSpace(controlToken))
    {
        client.DefaultRequestHeaders.Add("X-Stonehenge-Control-Token", controlToken);
    }
    client.Timeout = TimeSpan.FromSeconds(3);
});
builder.Services.AddHttpClient<IStonehengeWorkflowApiClient, StonehengeWorkflowApiClient>((serviceProvider, client) =>
{
    var options = serviceProvider.GetRequiredService<Microsoft.Extensions.Options.IOptionsMonitor<StonehengeApiOptions>>().CurrentValue;
    client.BaseAddress = new Uri(options.BaseUrl.TrimEnd('/') + "/");
    client.DefaultRequestHeaders.Add("X-Stonehenge-Api-Key", options.ApiKey);
    client.Timeout = TimeSpan.FromSeconds(5);
});
builder.Services.AddScoped<IEngineeringDashboardData, LiveEngineeringDashboardData>();

var app = builder.Build();

if (TrainingCli.IsCommand(args))
{
    Environment.ExitCode = TrainingCli.Run(args, app.Services);
    return;
}

app.UseForwardedHeaders();
app.UseStaticFiles();
app.UseAuthentication();
app.UseAuthorization();

app.MapHealthChecks("/healthz");

app.MapControllerRoute(
    name: "default",
    pattern: "{controller=Engineering}/{action=Overview}/{id?}");

app.Run();
