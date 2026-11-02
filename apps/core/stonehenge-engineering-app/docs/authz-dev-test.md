# Development Authz Test Notes

This build uses a development authentication handler only. It reads `X-Dev-User`, turns the value into normal ASP.NET Core claims, and then controllers use policies and `[Authorize]`.

For browser testing without Dex, use one of these explicit dev-login URLs:

```text
http://HOST_IP:8088/dev-auth/login/employee.basic
http://HOST_IP:8088/dev-auth/login/engineering.user
http://HOST_IP:8088/dev-auth/login/engineering.admin
http://HOST_IP:8088/dev-auth/login/platform.admin
http://HOST_IP:8088/dev-auth/logout
```

The login route stores the same dev identity in a local `Stonehenge.DevUser` cookie. There is still no silent default user.

Supported users:

- `employee.basic`: `STONEHENGE_EMPLOYEES`
- `engineering.user`: `STONEHENGE_EMPLOYEES`, `STONEHENGE_ENGINEERING_USERS`
- `engineering.admin`: `STONEHENGE_EMPLOYEES`, `STONEHENGE_ENGINEERING_USERS`, `STONEHENGE_ENGINEERING_ADMINS`
- `platform.admin`: `STONEHENGE_EMPLOYEES`, `Domain Admins`, but outside Engineering

No header, an empty header, or an unknown user is unauthenticated.

## Page Checks

```bash
curl -i http://HOST_IP:8088/Engineering/Overview
curl -i -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/Engineering/Overview
curl -i -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/Engineering/Manual
curl -i -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/Engineering/Manual
curl -i -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/Engineering/Admin
curl -i -H 'X-Dev-User: engineering.admin' http://HOST_IP:8088/Engineering/Admin
```

Expected results:

- No `X-Dev-User`: protected pages return `401`.
- `employee.basic`: overview returns `200`; auto/manual/maintenance/admin return `403`.
- `engineering.user`: overview/auto/manual/maintenance return `200`; admin page returns `403`.
- `engineering.admin`: all current MVC pages return `200`.
- `platform.admin`: overview returns `200`; auto/manual/maintenance/admin return `403`.

## Intentional Read Gaps

These gaps require `TrainingVulnerabilities:Enabled=true`. In safe mode,
workflow IDOR, command detail/replay, and EngineeringUser admin reads are
denied, and dashboard DTOs omit classified operational breadcrumbs.

Some read-only API endpoints are intentionally weaker than their pages for later training scenarios.

Dashboard-level status/log endpoints:

```bash
curl -sS -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/api/guns/1/state
curl -sS -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/api/guns/1/logs
curl -sS -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/api/system/events
curl -sS -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/api/manual/context?gunNumber=1
```

Engineering-level admin read endpoints:

```bash
curl -sS -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/api/admin/audit
curl -sS -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/api/admin/config/summary
curl -sS -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/api/admin/users/effective-access
curl -sS -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/api/admin/routes
```

These endpoints expose route, policy, gun, request, panel, and event breadcrumbs. They must not expose middleware tokens, OpenPLC credentials, signing keys, or service passwords.

## State-Changing Boundaries

Level 1 dashboard-only users must not perform control actions:

```bash
curl -i -X POST -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/Engineering/Guns/1/Fire
curl -i -X POST -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/Engineering/Maintenance/Requests
```

Engineering users can submit normal engineering actions, but not admin writes:

```bash
curl -i -X POST -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/api/maintenance/requests/1/comment \
  -H 'Content-Type: application/json' \
  -d '{"comment":"engineering review note"}'

curl -i -X POST -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/Engineering/Admin/Safety/Global \
  -d 'action=safety-on'
```

Admin users can access admin pages and write actions:

```bash
curl -i -H 'X-Dev-User: engineering.admin' http://HOST_IP:8088/Engineering/Admin
```
