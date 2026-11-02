# Engineering Object IDOR Training Path

This app intentionally includes a bounded object-authorization training path for engineering users.
It exists only when `TrainingVulnerabilities:Enabled=true`; safe mode closes the
gaps while retaining normal Engineering/Admin functions.

Purpose:

- Dashboard-only users can discover safe workflow breadcrumbs.
- Engineering users can use predictable object IDs against selected engineering APIs.
- Some object APIs check only broad `EngineeringAccess` instead of object ownership or panel/gun scope.
- Dangerous actions remain bounded and do not expose raw middleware or PLC control.

## Access Boundaries

- Unauthenticated users cannot access protected pages or protected `/api/*` routes.
- `employee.basic` can read selected dashboard and workflow breadcrumbs.
- `employee.basic` cannot request panel embed tokens, read command objects, replay commands, or perform writes.
- `engineering.user` can access engineering pages and selected object APIs.
- `engineering.user` cannot access admin pages or admin write actions.
- `engineering.admin` can access normal admin UI and admin actions.
- Browser auth alone never grants `/internal/workflow/*`; that namespace requires the internal service header.

## Intentional Browser-Facing Gaps

Panel token gap:

```bash
curl -i -X POST \
  -H 'X-Dev-User: engineering.user' \
  http://HOST_IP:8088/api/panels/2/embed-token
```

Command object gap:

```bash
curl -i \
  -H 'X-Dev-User: engineering.user' \
  http://HOST_IP:8088/api/commands/1002
```

Bounded replay gap:

```bash
curl -i -X POST \
  -H 'X-Dev-User: engineering.user' \
  http://HOST_IP:8088/api/commands/1002/replay
```

Replay is a simulation. It records an app event and returns `SimulatedReplay`. It does not send raw middleware commands, fire commands, PLC writes, or arbitrary HTTP forwarding.

## Breadcrumb Discovery

Dashboard users can read selected sanitized APIs:

```bash
curl -sS \
  -H 'X-Dev-User: employee.basic' \
  http://HOST_IP:8088/api/guns/2/state

curl -sS \
  -H 'X-Dev-User: employee.basic' \
  http://HOST_IP:8088/api/guns/2/logs

curl -sS \
  -H 'X-Dev-User: employee.basic' \
  http://HOST_IP:8088/api/maintenance/requests/1
```

Breadcrumbs may include panel IDs, command IDs, gun IDs, route hints, and workflow status names. They must not include passwords, service secrets, middleware credentials, raw PLC upload paths, or raw internal service header values.

## Seed Objects

The command history store seeds stable training command IDs if they are missing:

- `1001`: Gun 1 manual aim, submitted by `engineering.user`
- `1002`: Gun 2 maintenance jog, submitted by `engineering.admin`
- `1003`: Gun 2 auto track preview, rejected by safety

Panel metadata is static:

- `1`: Gun 1 Manual Panel
- `2`: Gun 2 Manual Panel
- `10`: Auto Fire Coordination
- `99`: Admin Diagnostics

## Middleware Boundary

The Engineering App to middleware client path remains unchanged. These object APIs do not replace the existing middleware integration and do not proxy raw middleware. Middleware and PLCs remain private behind the app and the existing middleware client.
