using Microsoft.AspNetCore.Mvc;
using Stonehenge.Api.Models;

namespace Stonehenge.Api.Controllers;

[ApiController]
[Route("api/v1/apps")]
public sealed class AppCatalogController : ControllerBase
{
    [HttpGet("catalog")]
    [ProducesResponseType(typeof(AppCatalogResponse), StatusCodes.Status200OK)]
    public ActionResult<AppCatalogResponse> Catalog()
    {
        var apps = new[]
        {
            new StonehengeAppDescriptor(
                Id: "engineering",
                Name: "Stonehenge Engineering Console",
                Status: "active",
                Notes: "ASP.NET Core MVC operations console."),
            new StonehengeAppDescriptor(
                Id: "vendor",
                Name: "Stonehenge Vendor App",
                Status: "planned",
                Notes: "Future maintenance and part-request workflow client."),
            new StonehengeAppDescriptor(
                Id: "astro",
                Name: "Stonehenge Astro App",
                Status: "planned",
                Notes: "Future astronomy and target-planning workflow client.")
        };

        return Ok(new AppCatalogResponse(apps));
    }
}
