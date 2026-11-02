# API Surface Model

The Engineering App has separate API surfaces. They are intentionally not interchangeable.

## Browser-Facing Engineering App API

Routes under `/api/*` are browser-facing routes for the Engineering App UI. They use normal ASP.NET Core authentication, claims, and authorization policies:

- `DashboardAccess`
- `EngineeringAccess`
- `EngineeringAdminAccess`

These routes are part of the controlled ASP.NET auth/authz training surface. Some read-only APIs intentionally require weaker access than their matching pages, while state-changing control actions remain gated.

Examples:

- `/api/guns/{gunId}/state`
- `/api/guns/{gunId}/logs`
- `/api/system/events`
- `/api/manual/context`
- `/api/admin/config/summary`

Do not require the internal workflow service header for normal `/api/*` browser routes.

## Engineering Internal Workflow API

Routes under `/internal/workflow/*` are service-to-service routes for future integration with a separate Stonehenge Integration API. They expose app-owned workflow data only:

- maintenance requests
- part requests
- request status
- workflow health

Development guard:

```text
X-Stonehenge-Internal-Service: dev-internal-api
```

This is development-only. It must later be replaced with real service-to-service authentication such as mTLS, managed API keys, or signed service tokens.

Normal browser login is not sufficient for `/internal/workflow/*`. Even `engineering.admin` must include the internal service header.

## Middleware Boundary

The existing Engineering App to middleware path remains unchanged:

```text
Browser -> Engineering App /api/* or MVC action -> app service/client -> private middleware -> PLCs
```

Do not reroute existing middleware calls through `/internal/workflow/*`.

The internal workflow API must not proxy raw middleware routes, arbitrary URLs, or PLC operations. Do not add routes like:

- `/internal/workflow/middleware/proxy`
- `/internal/workflow/raw`
- `/internal/workflow/plc/upload`
- `/internal/workflow/guns/{id}/commands/raw`

Middleware and PLC management surfaces remain localhost/private-only.

## Future Stonehenge Integration API

The Stonehenge Integration API will be a separate internal backend service built later. It is not implemented in this app.

Future expected path:

```text
Stonehenge Integration API -> Engineering App /internal/workflow/* -> Engineering App SQLite workflow data
```

Other backend apps, such as the public Astro app or future vendor app, may call the Stonehenge Integration API. Public users should not call the Stonehenge Integration API directly.

## Deployment Notes

In lab or production reverse-proxy/firewall configuration, block these from untrusted networks:

- `/internal/*`
- `/debug/*`
- future internal Swagger/OpenAPI routes

Only trusted internal service networks should reach `/internal/workflow/*`.
