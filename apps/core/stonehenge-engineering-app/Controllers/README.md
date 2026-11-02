# Controllers

Controllers define the web and API boundary for the Engineering Console.

## Page Controllers

- `EngineeringController` renders the MVC workspaces: Overview, Auto, Manual, Maintenance, and Admin.
- `AccessController` renders access-denied pages.
- `DevAuthController` provides the temporary development login helper used before portal SSO exists.

## API Controllers

- `EngineeringApiController` exposes browser-facing `/api/*` endpoints for page polling and workflow actions.
- `InternalWorkflowController` exposes `/internal/workflow/*` for future service-to-service workflow integration. These routes require the internal service header and must not proxy middleware.
- `FuxaProxyController` serves app-local FUXA embed paths from known catalog entries. It should not become a generic open proxy.

Controllers should stay thin: validate access, bind request data, call services, and return views or shaped DTOs.
