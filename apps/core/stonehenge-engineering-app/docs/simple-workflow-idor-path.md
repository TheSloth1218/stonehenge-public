# Simple Workflow IDOR Path

This path uses existing Engineering App maintenance and part requests. It is
enabled only by the safe-by-default `TrainingVulnerabilities:Enabled` root. It
does not add replay review UI/storage and does not expose middleware, PLCs,
service tokens, or raw command execution.

## Roles

- `employee.basic`: dashboard/status and selected read-only breadcrumbs only.
- `engineering.user`: normal engineering pages plus simple maintenance and part request creation.
- `engineering.admin`: admin page and request approval/rejection.
- Internal workflow service: `/internal/workflow/*` only with `X-Stonehenge-Internal-Service: dev-internal-api`.

## Request Shape

Requests stay intentionally small:

- request ID
- request kind: `MaintenanceMode` or `PartRequest`
- request type: `EnableMaintenance`, `RequestPart`, or similar short request label
- gun ID
- asset ID
- panel ID
- status
- requested by
- source app
- created/updated/review timestamps
- safe route hint

The app does not add part names, part numbers, quantities, procurement fields, long reasons, or detailed ticket descriptions.

## Intended Training Chain

1. `employee.basic` can read sanitized request details by predictable ID through browser-facing APIs:
   - `GET /api/maintenance/requests/{requestId}`
   - `GET /api/part-requests/{requestId}`
2. Those responses expose safe breadcrumbs such as gun ID, asset ID, panel ID, status, and route hints.
3. `employee.basic` cannot create requests, approve requests, fetch panel tokens, read command objects, replay commands, or access `/internal/workflow/*`.
4. `engineering.user` can create simple maintenance and part requests with supplied gun/asset/panel IDs.
5. The create APIs intentionally perform broad role checks but weak object-level validation. They trust submitted object IDs more than they should.
6. `engineering.user` cannot approve or reject admin workflow actions.
7. `engineering.admin` can approve/reject requests.
8. `/internal/workflow/*` is service-to-service only and is not satisfied by browser/user auth.

## Seed Breadcrumbs

The SQLite request store seeds stable examples without overwriting runtime data:

- `2001`: maintenance mode request, Gun 1, `GUN-1`, panel 1, pending
- `2002`: maintenance mode request, Gun 2, `GUN-2`, panel 2, pending admin approval
- `2003`: part request, Gun 2, `GUN-2`, panel 2, pending

## Replay Boundary

`POST /api/commands/{commandId}/replay` remains a bounded backend-only training artifact. It records a local app event and returns `SimulatedReplay`. It does not create an admin review item, call middleware, write PLC state, fire a gun, proxy raw middleware routes, or bypass safety.

## Example Calls

Employee read-only breadcrumb:

```bash
curl -i -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/api/maintenance/requests/2001
curl -i -H 'X-Dev-User: employee.basic' http://HOST_IP:8088/api/part-requests/2003
```

Employee blocked from writes:

```bash
curl -i -X POST -H 'X-Dev-User: employee.basic' -H 'Content-Type: application/json' \
  -d '{"requestType":"EnableMaintenance","gunId":4,"assetId":"GUN-4","panelId":4}' \
  http://HOST_IP:8088/api/maintenance/requests
```

Engineering user object-level gap:

```bash
curl -i -X POST -H 'X-Dev-User: engineering.user' -H 'Content-Type: application/json' \
  -d '{"requestType":"EnableMaintenance","gunId":7,"assetId":"GUN-2","panelId":99}' \
  http://HOST_IP:8088/api/maintenance/requests

curl -i -X POST -H 'X-Dev-User: engineering.user' -H 'Content-Type: application/json' \
  -d '{"gunId":8,"assetId":"GUN-1","panelId":2}' \
  http://HOST_IP:8088/api/part-requests
```

Engineering user blocked from approval:

```bash
curl -i -X POST -H 'X-Dev-User: engineering.user' http://HOST_IP:8088/api/admin/part-requests/2003/approve
```

Admin approval:

```bash
curl -i -X POST -H 'X-Dev-User: engineering.admin' http://HOST_IP:8088/api/admin/part-requests/2003/approve
curl -i -X POST -H 'X-Dev-User: engineering.admin' http://HOST_IP:8088/api/admin/maintenance/requests/2001/reject
```

Internal service access:

```bash
curl -i -H 'X-Stonehenge-Internal-Service: dev-internal-api' http://HOST_IP:8088/internal/workflow/part-requests
```
