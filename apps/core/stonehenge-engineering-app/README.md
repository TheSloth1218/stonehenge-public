# Stonehenge Engineering Console

ASP.NET Core MVC operator console for the Stonehenge lab stack. The app is the browser-facing engineering UI over the Stonehenge middleware API, FUXA HMI panels, and app-owned workflow state.

The console currently provides:

- STN dashboard status for the 8-gun Stonehenge array.
- Auto, Manual, Maintenance, and Admin MVC workspaces.
- Middleware-backed gun status, mode transitions, manual fire requests, maintenance approvals, and reset/startup actions.
- FUXA panel embedding through known panel catalog entries and an app-local proxy path.
- SQLite-backed app workflow state for maintenance requests, part requests, app events, part counters, and command records.
- Development auth using `X-Dev-User` so the app can model dashboard, engineering, and admin boundaries before portal SSO is connected.
- Internal service routes under `/internal/workflow/*` for future service-to-service workflow integration.

## Boundaries

The engineering app talks to middleware through `IStonehengeMiddlewareClient`. It does not connect directly to OpenPLC runtimes, Modbus, or PostgreSQL.

Keep these boundaries intact:

- Use middleware REST APIs for runtime state and control requests.
- Keep FUXA access embedded/proxied through app-owned catalog entries.
- Keep maintenance/reset controls isolated from normal operator pages.
- Keep SQLite for app-owned workflow state only, not middleware history or PLC state.
- Do not add PLC upload routes, OpenPLC management routes, or raw middleware command replay routes.

## Main Routes

MVC pages:

- `GET /` redirects to the engineering dashboard.
- `GET /Engineering/Overview`
- `GET /Engineering/Auto?gunNumber=1`
- `GET /Engineering/Manual?gunNumber=1`
- `GET /Engineering/Maintenance?gunNumber=1`
- `GET /Engineering/Admin?hmiGunNumber=1`

Browser-facing API routes live under `/api/*` and use normal app auth.

Internal workflow routes live under `/internal/workflow/*` and require the development internal service header:

```text
X-Stonehenge-Internal-Service: dev-internal-api
```

## Authentication Modes

The app defaults to `Authentication:Mode=Dev`. This keeps the current CPTC training identities and intentionally vulnerable training paths available without an external identity provider:

```sh
curl -H 'X-Dev-User: employee.basic' http://127.0.0.1:8088/Engineering/Overview
curl -H 'X-Dev-User: engineering.user' http://127.0.0.1:8088/Engineering/Manual
curl -H 'X-Dev-User: engineering.admin' http://127.0.0.1:8088/Engineering/Admin
curl -H 'X-Dev-User: platform.admin' http://127.0.0.1:8088/Engineering/Overview
```

There is also a local dev-login helper when `STONEHENGE_SHOW_DEV_AUTH=true`:

```text
/dev-auth/login/engineering.admin?returnUrl=/Engineering/Admin
```

`platform.admin` simulates an administrator who is outside the Engineering OU.
It receives dashboard-only access and is used to verify that broad directory
administrator membership does not bypass the Engineering boundary.

Set `STONEHENGE_AUTH_MODE=Oidc` to use Dex or another standards-compliant OIDC provider. OIDC mode disables `X-Dev-User` and returns `404` from `/dev-auth/*`. The planned FCU deployment values are:

```text
STONEHENGE_OIDC_AUTHORITY=https://login.fcu.gov/dex
STONEHENGE_OIDC_CLIENT_ID=stonehenge-engineering
STONEHENGE_OIDC_CLIENT_SECRET=<untracked-client-secret>
STONEHENGE_TRUSTED_PROXY_IP=<reverse-proxy-container-ip>
```

Alternatively, trust a narrowly scoped proxy subnet with `STONEHENGE_TRUSTED_PROXY_NETWORK`. Do not use a broad network range. The configured sign-in redirect URI at the provider is:

```text
https://engineering.example.test/signin-oidc
```

For FCU DNS, configure this callback URL in Dex:

```text
https://engineering.fcu.gov/signin-oidc
```

Dex does not publish an OIDC `end_session_endpoint`. Logout therefore clears
the Engineering App cookie and any active Reset Toolbox reauthorization grant,
then renders a local signed-out page. Its Sign In action requests a fresh Dex
login with `prompt=login` and `max_age=0` so updated directory groups are issued
in a new token.

The Engineering App does not query Active Directory at `192.168.30.2`
directly. Dex owns the LDAP/AD connection and the app trusts only validated
OIDC claims.

The provider must emit a stable `sub`, a fresh `auth_time` when prompted, and
claims that identify both Engineering membership and administrative access.
Authorization is derived only from Dex's signed `groups` claim.

For the FCU Dex deployment, Engineering access matches:

- `groups=SCA-Engineering-Users`
- `groups=SCA-Engineering-Admins` (inherits Engineering access)

Engineering Admin access requires:

- `SCA-Engineering-Admins`

Current Dex role mapping:

- `SCA-Employees` -> dashboard (`employee.basic`)
- `SCA-Admins` -> general administrator role; Overview only in this app
- `SCA-Engineering-Users` -> Engineering (`engineering.user`)
- `SCA-Engineering-Admins` -> Engineering Admin (`engineering.admin`) and inherits Engineering access

`SCA-Admins` does not imply either Engineering role, and `SCA-Employees` does
not grant Engineering access. Any authenticated user can access Overview.

The matching claim types and exact values are configurable under
`Authentication:Oidc:EngineeringBoundary` and
`Authentication:Oidc:AdminBoundary`. The app converts accepted external groups
to canonical internal Stonehenge claims.

OIDC sessions use a secure, HTTP-only `__Host-Stonehenge.Auth.v3` cookie. Data-protection keys persist in the `engineering-app-data` volume. The Reset Toolbox performs a fresh OIDC login (`prompt=login`, `max_age=0`), verifies the same subject and fresh `auth_time`, then grants one server-side 60-second window.

The app image trusts the FCU Step CA public root in
`certificates/fcu-lab-root-ca.crt`. TLS certificate validation remains enabled;
the app does not use an insecure certificate callback.

For reverse-proxy deployment, terminate TLS at the proxy, pass `X-Forwarded-Proto` and `X-Forwarded-Host`, bind the app port to loopback with `ENGINEERING_APP_HOST_IP=127.0.0.1`, and configure only the actual proxy IP or subnet as trusted.

The CPTC training weaknesses are intentionally retained in Dev mode and in the documented bounded API paths. Enabling OIDC changes identity establishment; it does not silently remove the deliberate object-authorization and disclosure exercises.

## Run From Monorepo

From `/opt/Stonehenge`:

```sh
docker compose up -d --build
```

Open the app:

```text
http://<host-ip>:8088
```

The app listens on port `8080` inside the container. The top-level compose file exposes it as `${ENGINEERING_APP_PORT:-8088}`.

## Important Configuration

Common environment variables:

```text
STONEHENGE_MIDDLEWARE_URL=http://middleware:8080
STONEHENGE_CONTROL_TOKEN=<shared middleware control token>
STONEHENGE_FUXA_PROXY_TOKEN=<shared Engineering-to-FUXA proxy token>
STONEHENGE_APP_DB_PATH=/data/stonehenge-engineering.db
STONEHENGE_SHOW_DEV_AUTH=true
STONEHENGE_AUTH_MODE=Dev
FUXA_EMBED_ENABLED=true
FUXA_BASE_URL=
FUXA_INTERNAL_BASE_URL=http://fuxa:1881
FUXA_EMBED_PATH_TEMPLATE=/stonehenge/embed/{fuxaViewId}
FUXA_LOCAL_PROXY_PATH_TEMPLATE=/fuxa/embed/{fuxaViewId}
FUXA_DEFAULT_PANEL_HEIGHT=720
```

In Docker, app data is stored in the `engineering-app-data` volume at `/data/stonehenge-engineering.db`.

## Directory Guide

- `Controllers/` - MVC page controllers plus browser-facing and internal API controllers.
- `Models/` - Razor view models and app workflow record types.
- `Services/` - middleware client, dashboard data adapter, SQLite stores, FUXA catalog/embed helpers.
- `Security/` - temporary development auth and authorization policy constants.
- `Views/` - Razor pages, shared layout, and shared FUXA frame partial.
- `wwwroot/` - CSS, JavaScript, fonts, and media assets.
- `fuxa-imports/` - FUXA import/reference artifacts used to build HMI panels.
- `tools/fuxa/` - helper scripts for FUXA panel installation/proxy customization.
- `tools/playwright/` - containerized Playwright validation environment.
- `docs/` - security training notes, API surface notes, and IDOR walkthroughs.

## Validation

From the app folder:

```sh
dotnet test
```

From the monorepo root, validate compose:

```sh
docker compose config >/tmp/stonehenge-compose.out
```

Playwright tooling is available through the `playwright` compose profile and `tools/playwright`.
