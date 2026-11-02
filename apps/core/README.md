# Stonehenge

> For the public combined stack, start with the [root README](../../README.md). The instructions below describe this app as a standalone source snapshot.

Stonehenge is now the primary monorepo for the local engineering stack. It contains the middleware, OpenPLC runtime deployment tooling, FUXA HMI assets, target feed, internal API, engineering MVC webapp, and PLC projects in one checkout.

## Repository Layout

- `stonehenge-middleware/` - Go middleware, historian/PostgreSQL integration, dispatcher, maintenance runner, FUXA image wiring, and OpenPLC runtime compose services.
- `stonehenge-api/` - ASP.NET Core Swagger API for future Stonehenge app-to-app coordination between engineering, vendor, astro, and related apps.
- `stonehenge-engineering-app/` - ASP.NET Core MVC engineering console and browser-facing workflow/API training surface.
- `stonehenge-targets/` - Target-feed generator for middleware ingest testing.
- `stonehenge-plc-deployer/` - OpenPLC packaging, deployment, verification, startup, and mass reload tooling.
- `stonehenge-control-plc/` - Shared gun-control PLC source project used to build the program deployed across all gun runtimes.
- `stonehenge-safety-plc/` - Safety PLC source project snapshot.

## Run The Full Stack

From the monorepo root:

```bash
cd /opt/Stonehenge
docker compose up -d --build
docker compose ps
```

This starts the runtime fleet, middleware, PostgreSQL historian, FUXA, internal API, engineering webapp, and target feed on one Compose project.

Primary endpoints:

- Engineering app: `https://engineering.fcu.gov`
- Stonehenge API: `https://api.fcu.gov`
- Engineering app local diagnostics: `http://127.0.0.1:8088`
- Stonehenge API local diagnostics: `http://127.0.0.1:8092`
- Middleware API/dashboard: `http://127.0.0.1:8080`
- Target feed health: `http://127.0.0.1:8090/health`
- FUXA: `http://127.0.0.1:1881` by default; the webapp embeds it through its local proxy.

OpenPLC runtime web management ports are bound to localhost by default. Use an SSH tunnel or deliberate local-only access for runtime management.

Copy `.env.example` to `.env`. Caddy binds ports 80/443 and uses the FCU Step CA
ACME provisioner. Direct Engineering App and API ports bind to loopback by
default. Swagger is disabled by default; set
`STONEHENGE_API_SWAGGER_ENABLED=true` only when it is needed.

## Deployment Runbook

Use this path for the normal "bring the lab back up" flow:

```bash
cd /opt/Stonehenge
docker compose up -d --build
docker compose ps
```

Verify the stack:

```bash
curl -fsS http://127.0.0.1:8080/ready
curl -fsS http://127.0.0.1:8080/api/v1/status
curl -fsS http://127.0.0.1:8090/health
curl -fsS https://api.fcu.gov/api/v1/system/health
curl -fsS https://engineering.fcu.gov/healthz
```

Use the PLC deployer only when the OpenPLC runtime programs need to be refreshed:

```bash
cd /opt/Stonehenge/stonehenge-plc-deployer

./tools/startup-reload \
  --gun-artifact ./artifacts/gun/stonehenge-gun-2f182b5dc6b.zip \
  --safety-artifact ./artifacts/safety/stonehenge-safety-eeea2091ebcb.zip \
  --runtimes gun1,gun2,gun3,gun4,gun5,gun6,gun7,gun8,safety \
  --include-safety \
  --allow-active-pair \
  --credentials ~/.stonehenge-openplc.json \
  --insecure \
  --execute
```

That reload path is separate from the normal Compose startup. Do not use it as a routine health check.

## Caddy, DNS, And TLS

DNS is supplied by the deployment environment and is not hardcoded in the
applications. The lab convention is `engineering.fcu.gov` for the Engineering
App and `api.fcu.gov` for the API, under the operator-managed `*.fcu.gov` zone.
Register the matching `https://engineering.fcu.gov/signin-oidc` and
`https://engineering.fcu.gov/signout-callback-oidc` URLs with the OIDC provider.

Caddy is included in the root Compose project and is the TLS edge for:

- `engineering.fcu.gov` -> `engineering-app:8080`
- `api.fcu.gov` -> `stonehenge-api:8080`

Caddy obtains and renews both certificates through
`https://ca.fcu.gov:8443/acme/acme/directory`. FCU clients must trust the FCU
lab root CA. The direct ports 8088 and 8092 are loopback-only diagnostics.

Keep these services internal or localhost-only:

- Middleware API: `127.0.0.1:8080`
- FUXA: `127.0.0.1:1881`
- OpenPLC runtime management ports
- PostgreSQL
- Modbus/OpenPLC runtime ports

Caddy forwards the standard proxy headers so ASP.NET can build correct callback
URLs and apply secure-cookie behavior:

```text
X-Forwarded-For
X-Forwarded-Proto
X-Forwarded-Host
Host
```

The Caddy configuration performs no path rewriting, authentication,
authorization, or API-key filtering. Intended bounded training behaviors remain
at the application layer. Raw middleware, PLC, FUXA, SQL, and Modbus surfaces
are not routed externally.

## Dev Auth Shortcuts

The engineering app currently uses development headers/login shortcuts until the portal auth integration lands:

```text
https://engineering.fcu.gov/dev-auth/login/employee.basic
https://engineering.fcu.gov/dev-auth/login/engineering.user
https://engineering.fcu.gov/dev-auth/login/engineering.admin
```

## Operational Notes

- The root Compose project name is pinned to `stonehenge-middleware` so existing runtime network names, maintenance runner assumptions, and host tooling remain compatible.
- Middleware maintenance controls still require the existing maintenance token and confirmation string when enabled.
- The webapp talks to middleware over REST only. It does not connect directly to OpenPLC, Modbus, or PostgreSQL.
- FUXA is still served by the middleware-side FUXA container and embedded through the webapp proxy.

## Useful Checks

```bash
docker compose ps
curl -fsS http://127.0.0.1:8080/ready
curl -fsS http://127.0.0.1:8080/api/v1/status
curl -fsS https://api.fcu.gov/api/v1/system/health
curl -fsS http://127.0.0.1:8090/health
curl -fsS https://engineering.fcu.gov/healthz
```

Run the webapp Playwright checks through the root Compose project:

```bash
docker compose --profile tools run --rm playwright npx playwright test --project=desktop-1080p
```
