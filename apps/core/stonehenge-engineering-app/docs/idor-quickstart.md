# Stonehenge IDOR Quickstart

This app intentionally includes a bounded IDOR training path around workflow objects. The point is to practice object authorization, not to bypass PLC safety or send raw middleware commands.

## What The IDOR Is

Low-privilege users can read sanitized workflow request details by predictable object IDs:

- `GET /api/maintenance/requests/{id}`
- `GET /api/part-requests/{id}`

For example, `employee.basic` can read request metadata for seeded IDs like `2001` and `2003`, even though that user cannot create, approve, deny, close, replay, or execute anything.

The response is intentionally useful but sanitized. It should include breadcrumbs such as:

- request ID
- request kind/type
- gun ID
- asset ID
- panel ID
- status
- priority
- source app
- created/updated timestamps
- safe route hints

It must not include credentials, raw middleware commands, PLC upload paths, or direct runtime access.

## Dev Users

Use these headers to simulate portal users:

```bash
X-Dev-User: employee.basic
X-Dev-User: engineering.user
X-Dev-User: engineering.admin
```

Browser login shortcuts:

```text
http://HOST_IP:8088/dev-auth/login/employee.basic
http://HOST_IP:8088/dev-auth/login/engineering.user
http://HOST_IP:8088/dev-auth/login/engineering.admin
```

## Manual Checks

Read a seeded maintenance request as a dashboard-only user:

```bash
curl -i \
  -H 'X-Dev-User: employee.basic' \
  http://HOST_IP:8088/api/maintenance/requests/2001
```

Expected:

- `200 OK`
- response contains request metadata
- response does not contain secrets or middleware credentials

Read a seeded part request:

```bash
curl -i \
  -H 'X-Dev-User: employee.basic' \
  http://HOST_IP:8088/api/part-requests/2003
```

Expected:

- `200 OK`
- response contains request metadata
- procurement/vendor fields stay de-emphasized or absent

Confirm the same user cannot write:

```bash
curl -i -X POST \
  -H 'X-Dev-User: employee.basic' \
  -H 'Content-Type: application/json' \
  -d '{"requestType":"EnableMaintenance","gunId":4,"assetId":"GUN-4","panelId":4}' \
  http://HOST_IP:8088/api/maintenance/requests
```

Expected:

- `403 Forbidden`

Confirm the same user cannot approve:

```bash
curl -i -X POST \
  -H 'X-Dev-User: employee.basic' \
  http://HOST_IP:8088/api/admin/maintenance/requests/2001/approve
```

Expected:

- `403 Forbidden`

## Engineering User Boundary

`engineering.user` can create normal workflow requests:

```bash
curl -i -X POST \
  -H 'X-Dev-User: engineering.user' \
  -H 'Content-Type: application/json' \
  -d '{"requestType":"EnableMaintenance","gunId":7,"assetId":"GUN-7","panelId":7}' \
  http://HOST_IP:8088/api/maintenance/requests
```

Expected:

- `201 Created`
- returned `requestId` is predictable and can be read through the browser-facing request detail route

But `engineering.user` still cannot approve admin workflow actions:

```bash
curl -i -X POST \
  -H 'X-Dev-User: engineering.user' \
  http://HOST_IP:8088/api/admin/maintenance/requests/2001/approve
```

Expected:

- `403 Forbidden`

## Admin Boundary

`engineering.admin` can use admin workflow actions:

```bash
curl -i -X POST \
  -H 'X-Dev-User: engineering.admin' \
  http://HOST_IP:8088/api/admin/maintenance/requests/2001/approve
```

Expected:

- `200 OK` if the request is still pending
- another bounded status if the request is already resolved

Browser auth alone must not access internal service routes:

```bash
curl -i \
  -H 'X-Dev-User: engineering.admin' \
  http://HOST_IP:8088/internal/workflow/health
```

Expected:

- `403 Forbidden`

## Test Command

Run the current validation suite:

```bash
docker compose run --rm playwright npx playwright test \
  tests/api-boundary-training.spec.js \
  tests/engineering-object-idor.spec.js \
  --project=desktop-1080p
```

Current expected result:

- IDOR workflow behavior tests pass
- source-inspection subtest may skip when source files are not mounted in the Playwright container
