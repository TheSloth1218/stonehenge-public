using Microsoft.AspNetCore.Antiforgery;
using Microsoft.AspNetCore.Mvc;
using Microsoft.AspNetCore.Mvc.Filters;

namespace Stonehenge.Engineering.App.Security;

public sealed class OidcAntiforgeryFilter : IAsyncAuthorizationFilter
{
    private readonly StonehengeAuthenticationOptions _authenticationOptions;
    private readonly IAntiforgery _antiforgery;

    public OidcAntiforgeryFilter(
        StonehengeAuthenticationOptions authenticationOptions,
        IAntiforgery antiforgery)
    {
        _authenticationOptions = authenticationOptions;
        _antiforgery = antiforgery;
    }

    public async Task OnAuthorizationAsync(AuthorizationFilterContext context)
    {
        if (!_authenticationOptions.IsOidc || HttpMethods.IsGet(context.HttpContext.Request.Method) || HttpMethods.IsHead(context.HttpContext.Request.Method) || HttpMethods.IsOptions(context.HttpContext.Request.Method))
        {
            return;
        }

        try
        {
            await _antiforgery.ValidateRequestAsync(context.HttpContext);
        }
        catch (AntiforgeryValidationException)
        {
            context.Result = new BadRequestObjectResult(new { error = "antiforgery_validation_failed" });
        }
    }
}
