using Microsoft.OpenApi.Models;
using Stonehenge.Api.Auth;
using Stonehenge.Api.Models;
using Stonehenge.Api.Options;
using Stonehenge.Api.Services;

var builder = WebApplication.CreateBuilder(args);
var swaggerEnabled = builder.Configuration.GetValue<bool>("Swagger:Enabled");

builder.WebHost.ConfigureKestrel(options =>
    options.Limits.MaxRequestBodySize = WorkflowInputLimits.MaxRequestBodyBytes);

if (builder.Environment.IsDevelopment())
{
    builder.Configuration.AddJsonFile(".generated/api-key-seed.generated.json", optional: true, reloadOnChange: true);
}

builder.Services.Configure<TrainingVulnerabilityOptions>(
    builder.Configuration.GetSection("TrainingVulnerabilities"));
builder.Services.Configure<VendorApiOptions>(
    builder.Configuration.GetSection("VendorApi"));
builder.Services.Configure<ApiKeyAuthOptions>(
    builder.Configuration.GetSection("ApiKeyAuth"));
builder.Services.Configure<StonehengeOptions>(
    builder.Configuration.GetSection("Stonehenge"));
builder.Services.Configure<OperationalOptions>(
    builder.Configuration.GetSection(OperationalOptions.SectionName));
builder.Services.AddSingleton<IApiKeyAuthorizer, DevApiKeyAuthorizer>();
builder.Services.AddSingleton<IStonehengeSimulationStore, SeededStonehengeSimulationStore>();
builder.Services.AddSingleton<SeededAstroDataProvider>();
builder.Services.AddHttpClient<LiveMiddlewareAstroDataProvider>((services, client) =>
{
    var options = services.GetRequiredService<Microsoft.Extensions.Options.IOptionsMonitor<StonehengeOptions>>().CurrentValue;
    client.BaseAddress = new Uri(options.MiddlewareBaseUrl.TrimEnd('/') + "/");
    client.Timeout = TimeSpan.FromSeconds(5);
});
builder.Services.AddSingleton<IAstroDataProvider>(services => services.GetRequiredService<LiveMiddlewareAstroDataProvider>());
builder.Services.AddControllers();
if (swaggerEnabled)
{
    builder.Services.AddEndpointsApiExplorer();
    builder.Services.AddSwaggerGen(options =>
    {
        options.SwaggerDoc("v1", new OpenApiInfo
        {
            Title = "Stonehenge Internal API",
            Version = "v1",
            Description = "Internal app-to-app coordination API for Stonehenge companion applications."
        });
    });
}

var app = builder.Build();

var apiKeyOptions = app.Services.GetRequiredService<Microsoft.Extensions.Options.IOptions<ApiKeyAuthOptions>>().Value;
app.Logger.LogInformation(
    "Loaded {GeneratedApiKeyCount} generated API key credential(s). Legacy dev key fallback enabled: {LegacyFallbackEnabled}",
    apiKeyOptions.Keys.Count,
    apiKeyOptions.AllowLegacyDevKeys);

if (swaggerEnabled)
{
    app.UseSwagger();
    app.UseSwaggerUI(options =>
    {
        options.SwaggerEndpoint("/swagger/v1/swagger.json", "Stonehenge Internal API v1");
        options.RoutePrefix = "swagger";
    });
}

if (swaggerEnabled)
{
    app.MapGet("/", () => Results.Redirect("/swagger"));
}
else
{
    app.MapGet("/", () => Results.Ok(new { service = "stonehenge-api", status = "ready" }));
}
app.MapControllers();

app.Run();
