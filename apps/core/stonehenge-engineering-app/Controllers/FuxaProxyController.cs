using System.Net.Http.Headers;
using System.Text;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Options;
using Stonehenge.Engineering.App.Models;
using Stonehenge.Engineering.App.Security;
using Stonehenge.Engineering.App.Services;

namespace Stonehenge.Engineering.App.Controllers;

[ApiExplorerSettings(IgnoreApi = true)]
[Authorize(Policy = StonehengeAuth.EngineeringAccessPolicy)]
public sealed class FuxaProxyController : Controller
{
    private const string EmbedViewCookieName = "StonehengeFuxaEmbedView";

    private static readonly HashSet<string> BlockedShellPrefixes = new(StringComparer.OrdinalIgnoreCase)
    {
        "lab",
        "editor",
        "view",
        "home",
        "login",
        "logout",
        "projects",
        "settings"
    };

    private readonly IHttpClientFactory _httpClientFactory;
    private readonly IEngineeringPanelCatalog _panels;
    private readonly FuxaOptions _options;

    public FuxaProxyController(
        IHttpClientFactory httpClientFactory,
        IEngineeringPanelCatalog panels,
        IOptions<FuxaOptions> options)
    {
        _httpClientFactory = httpClientFactory;
        _panels = panels;
        _options = options.Value;
    }

    [HttpGet("/fuxa/embed/{viewId}")]
    public async Task<IActionResult> Embed(string viewId, CancellationToken cancellationToken)
    {
        if (!_options.Enabled || !IsSafeViewId(viewId))
        {
            return NotFound();
        }

        var panel = _panels.List().SingleOrDefault(entry =>
            entry.Enabled &&
            string.Equals(entry.FuxaViewId, viewId, StringComparison.Ordinal));
        if (panel is null)
        {
            return NotFound();
        }

        if (panel.RequiredAccess == StonehengeAuth.EngineeringAdminAccessPolicy &&
            !User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel))
        {
            return Forbid();
        }

        var response = await SendToFuxaAsync(HttpMethod.Get, "/stonehenge/embed/" + Uri.EscapeDataString(viewId), null, cancellationToken);
        if (response.StatusCode == System.Net.HttpStatusCode.NotFound)
        {
            response.Dispose();
            response = await SendToFuxaAsync(HttpMethod.Get, "/", null, cancellationToken);
        }

        if (!response.IsSuccessStatusCode)
        {
            return StatusCode((int)response.StatusCode);
        }

        Response.Cookies.Append(EmbedViewCookieName, viewId, new CookieOptions
        {
            HttpOnly = true,
            IsEssential = true,
            Path = "/",
            SameSite = SameSiteMode.Lax,
            Secure = Request.IsHttps
        });
        var html = await response.Content.ReadAsStringAsync(cancellationToken);
        html = RewriteEmbedHtml(html, viewId);
        return Content(html, "text/html; charset=utf-8");
    }

    [AcceptVerbs("GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", Route = "/fuxa/{**path}")]
    public async Task<IActionResult> Proxy(string? path, CancellationToken cancellationToken)
    {
        if (HttpMethods.IsOptions(Request.Method))
        {
            return NoContent();
        }

        if (!IsAllowedSupportingPath(path))
        {
            return NotFound();
        }

        if (!CanProxyMethod(path))
        {
            return Forbid();
        }

        var targetPath = "/" + path!.TrimStart('/');
        if (Request.QueryString.HasValue)
        {
            targetPath += Request.QueryString.Value;
        }

        var body = HttpMethods.IsGet(Request.Method) || HttpMethods.IsHead(Request.Method)
            ? null
            : Request.Body;
        using var response = await SendToFuxaAsync(new HttpMethod(Request.Method), targetPath, body, cancellationToken);
        var bytes = await response.Content.ReadAsByteArrayAsync(cancellationToken);
        if (!response.IsSuccessStatusCode)
        {
            return StatusCode((int)response.StatusCode, Encoding.UTF8.GetString(bytes));
        }

        var contentType = response.Content.Headers.ContentType?.ToString() ?? "application/octet-stream";
        return File(bytes, contentType);
    }

    [AcceptVerbs("GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", Route = "/api/version")]
    [AcceptVerbs("GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", Route = "/api/project")]
    [AcceptVerbs("GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", Route = "/socket.io/{**path}")]
    [AcceptVerbs("GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", Route = "/stonehenge/hmi/{**path}")]
    public async Task<IActionResult> ProxyAbsoluteFuxaSupportPath(string? path, CancellationToken cancellationToken)
    {
        if (HttpMethods.IsOptions(Request.Method))
        {
            return NoContent();
        }

        var requestPath = Request.Path.Value ?? string.Empty;
        if (!requestPath.Equals("/api/version", StringComparison.OrdinalIgnoreCase) &&
            !requestPath.Equals("/api/project", StringComparison.OrdinalIgnoreCase) &&
            !requestPath.StartsWith("/socket.io", StringComparison.OrdinalIgnoreCase) &&
            !requestPath.StartsWith("/stonehenge/hmi/", StringComparison.OrdinalIgnoreCase))
        {
            return NotFound();
        }

        if (!CanProxyMethod(requestPath))
        {
            return Forbid();
        }

        string? embedViewId = null;
        if (requestPath.Equals("/api/project", StringComparison.OrdinalIgnoreCase))
        {
            embedViewId = Request.Cookies[EmbedViewCookieName];
            var panel = _panels.List().SingleOrDefault(entry =>
                entry.Enabled &&
                string.Equals(entry.FuxaViewId, embedViewId, StringComparison.Ordinal));
            if (panel is null)
            {
                return NotFound();
            }
            if (panel.RequiredAccess == StonehengeAuth.EngineeringAdminAccessPolicy &&
                !User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel))
            {
                return Forbid();
            }
        }

        var targetPath = requestPath;
        if (Request.QueryString.HasValue)
        {
            targetPath += Request.QueryString.Value;
        }

        var body = HttpMethods.IsGet(Request.Method) || HttpMethods.IsHead(Request.Method)
            ? null
            : Request.Body;
        using var response = await SendToFuxaAsync(
            new HttpMethod(Request.Method), targetPath, body, cancellationToken, embedViewId);
        var bytes = await response.Content.ReadAsByteArrayAsync(cancellationToken);
        if (!response.IsSuccessStatusCode)
        {
            return StatusCode((int)response.StatusCode, Encoding.UTF8.GetString(bytes));
        }

        var contentType = response.Content.Headers.ContentType?.ToString() ?? "application/octet-stream";
        return File(bytes, contentType);
    }

    private async Task<HttpResponseMessage> SendToFuxaAsync(
        HttpMethod method,
        string pathAndQuery,
        Stream? body,
        CancellationToken cancellationToken,
        string? embedViewId = null)
    {
        var request = new HttpRequestMessage(method, pathAndQuery);
        request.Headers.TryAddWithoutValidation(
            StonehengeAuth.EngineeringAdminProxyHeader,
            User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel) ? "true" : "false");
        if (embedViewId is not null)
        {
            request.Headers.TryAddWithoutValidation(
                "Cookie",
                $"{EmbedViewCookieName}={Uri.EscapeDataString(embedViewId)}");
        }
        if (body is not null)
        {
            request.Content = new StreamContent(body);
            if (!string.IsNullOrWhiteSpace(Request.ContentType))
            {
                request.Content.Headers.ContentType = MediaTypeHeaderValue.Parse(Request.ContentType);
            }
        }

        if (Request.Headers.Accept.Count > 0)
        {
            foreach (var accept in Request.Headers.Accept)
            {
                if (accept is not null)
                {
                    request.Headers.TryAddWithoutValidation("Accept", accept);
                }
            }
        }

        var client = _httpClientFactory.CreateClient("FuxaProxy");
        return await client.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
    }

    private bool CanProxyMethod(string? path)
    {
        if (HttpMethods.IsGet(Request.Method) ||
            HttpMethods.IsHead(Request.Method) ||
            HttpMethods.IsOptions(Request.Method) ||
            IsSocketPath(path))
        {
            return true;
        }

        return User.HasStonehengeAccess(StonehengeAuth.EngineeringAdminAccessLevel);
    }

    private static bool IsSocketPath(string? path)
    {
        var normalized = path?.TrimStart('/');
        return string.Equals(normalized, "socket.io", StringComparison.OrdinalIgnoreCase) ||
            (normalized?.StartsWith("socket.io/", StringComparison.OrdinalIgnoreCase) ?? false);
    }

    private static string RewriteEmbedHtml(string html, string viewId)
    {
        html = html.Replace("<base href=\"/\">", "<base href=\"/fuxa/\">", StringComparison.OrdinalIgnoreCase);
        var encodedViewId = JavaScriptStringEncode(viewId);
        var guard = $$"""
<style id="stonehenge-app-fuxa-embed-css">
  mat-sidenav.sidenav,
  .sidenav,
  .fab-toggle,
  .fab-item,
  a.fab-toggle,
  a.fab-item,
  button.mat-mdc-icon-button,
  .header-tools-options {
    display: none !important;
    visibility: hidden !important;
    pointer-events: none !important;
    opacity: 0 !important;
    width: 0 !important;
    height: 0 !important;
    min-width: 0 !important;
    min-height: 0 !important;
  }
  .mat-drawer-content,
  .mat-sidenav-content {
    margin-left: 0 !important;
  }
</style>
<script id="stonehenge-app-fuxa-embed-guard">
(() => {
  const allowedHash = "#/view/{{encodedViewId}}";
  const forceView = () => {
    if (window.location.hash !== allowedHash) {
      window.location.replace(window.location.pathname + allowedHash);
    }
  };
  window.__STONEHENGE_EMBED_VIEW = "{{encodedViewId}}";
  window.addEventListener("hashchange", forceView);
  forceView();
})();
</script>
""";

        return html.Contains("</head>", StringComparison.OrdinalIgnoreCase)
            ? html.Replace("</head>", guard + "</head>", StringComparison.OrdinalIgnoreCase)
            : guard + html;
    }

    private static bool IsAllowedSupportingPath(string? path)
    {
        if (string.IsNullOrWhiteSpace(path))
        {
            return false;
        }

        var normalized = path.TrimStart('/');
        var firstSegment = normalized.Split('/', 2)[0];
        if (BlockedShellPrefixes.Contains(firstSegment))
        {
            return false;
        }

        return normalized.StartsWith("assets/", StringComparison.OrdinalIgnoreCase) ||
            normalized.StartsWith("socket.io/", StringComparison.OrdinalIgnoreCase) ||
            normalized.StartsWith("plugins/", StringComparison.OrdinalIgnoreCase) ||
            normalized.StartsWith("images/", StringComparison.OrdinalIgnoreCase) ||
            normalized.StartsWith("locale/", StringComparison.OrdinalIgnoreCase) ||
            normalized.StartsWith("ngsw", StringComparison.OrdinalIgnoreCase) ||
            normalized.StartsWith("manifest", StringComparison.OrdinalIgnoreCase) ||
            normalized.StartsWith("favicon", StringComparison.OrdinalIgnoreCase) ||
            normalized.EndsWith(".js", StringComparison.OrdinalIgnoreCase) ||
            normalized.EndsWith(".css", StringComparison.OrdinalIgnoreCase) ||
            normalized.EndsWith(".ico", StringComparison.OrdinalIgnoreCase) ||
            normalized.EndsWith(".png", StringComparison.OrdinalIgnoreCase) ||
            normalized.EndsWith(".svg", StringComparison.OrdinalIgnoreCase) ||
            normalized.EndsWith(".woff", StringComparison.OrdinalIgnoreCase) ||
            normalized.EndsWith(".woff2", StringComparison.OrdinalIgnoreCase);
    }

    private static bool IsSafeViewId(string viewId)
    {
        return !string.IsNullOrWhiteSpace(viewId) &&
            viewId.All(character =>
                char.IsAsciiLetterOrDigit(character) ||
                character is '-' or '_');
    }

    private static string JavaScriptStringEncode(string value)
    {
        return value
            .Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\"", "\\\"", StringComparison.Ordinal)
            .Replace("<", "\\u003c", StringComparison.Ordinal)
            .Replace(">", "\\u003e", StringComparison.Ordinal)
            .Replace("&", "\\u0026", StringComparison.Ordinal);
    }
}
