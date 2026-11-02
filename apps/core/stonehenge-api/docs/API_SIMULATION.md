# Stonehenge API Simulation

`stonehenge-api` currently exposes API-local workflow data seeded for development. The route shapes are meant to survive the later move to live providers backed by Engineering App SQLite, middleware/PostgreSQL summaries, and other app-owned sources. It does not call OpenPLC, Modbus, FUXA, or raw middleware control paths.

## Purpose

The API gives future clients stable contracts before those apps exist:

- Vendor app: polls approved part requests, reads vendor-safe gun status, and sends part completion events.
- Astro app: reads sanitized target history and firing results for map plotting.
- Engineering sync: accepts exported app-owned part/service requests and exposes vendor completion events for the engineering app to poll.

## Dev API Keys

Use `X-Stonehenge-Api-Key` for protected route groups.

Generate local development keys before normal dev runs:

```bash
./scripts/generate-api-keys.sh
```

That creates `.generated/stonehenge-api-keys.env` for local scripts and `.generated/api-key-seed.generated.json` for the API to load hashed keys in Development. See `docs/API_KEYS.md` for the full key flow.

Legacy static keys remain accepted while `ApiKeyAuth:AllowLegacyDevKeys` is `true`:

| Key | Scopes |
| --- | --- |
| `vendor-dev-key` | `vendor.parts.read`, `vendor.guns.read`, `vendor.parts.acknowledge`, `vendor.parts.complete` |
| `astro-dev-key` | `astro.history.read`, `astro.results.read`, `astro.summary.read`, `system.publicStatus.read` |
| `engineering-dev-key` | `engineering.partRequests.write`, `engineering.vendorCompletions.read`, `engineering.vendorCompletions.apply` |

Astro endpoints are public by default for this dev simulation. The generated or legacy Astro key is accepted but not required for Astro reads, and can access the scoped low-sensitivity `GET /api/v1/system/public-status` endpoint.

## Vendor Flow

1. `GET /api/v1/vendor/part-requests` returns approved vendor-visible part requests.
2. `GET /api/v1/vendor/guns/status` returns an empty collection in the safe profile. Training mode returns the bounded per-gun mode, counter, part state, and optional diagnostics used by the exercise.
3. `POST /api/v1/vendor/part-requests/{requestId}/acknowledge` marks a request acknowledged in API-local memory. The first acknowledgement of a seeded practice request also creates one fresh `PR-PRACTICE-*` request, so beginning a practice fulfillment cannot exhaust the available pool. Repeated acknowledgements are idempotent.
4. `POST /api/v1/vendor/part-requests/{requestId}/complete` creates a vendor completion event in API-local memory.
5. `GET /api/v1/vendor/completions` lists local completion events for dev testing.

Set `VendorApi:UseSeededData=false` to hide seeded vendor requests and test only exported engineering requests.

## Engineering Sync

- `POST /api/v1/engineering/part-requests` upserts an approved app-owned part/service request.
- `POST /api/v1/engineering/part-requests/{requestId}/withdraw` hides an exported request from active vendor polling without deleting completion events.
- `GET /api/v1/engineering/vendor-completions/pending` returns completion events that have not been marked applied.
- `POST /api/v1/engineering/vendor-completions/{completionId}/applied` marks one local completion event as applied.

The exported request object is intentionally small: request ID, source object ID, gun ID, status, part status, compatibility priority text, and timestamps. This is a general service/parts request, not a parts catalog or procurement system.

Do not add SKUs, line items, quantities, quotes, suppliers, inventory, invoices, or shipping workflow fields.

## Astro Flow

- In the safe profile, Astro list endpoints return empty collections and shot summary returns 404 so no target/firing history, gun assignment, exact timestamp/result, or shot counter is published.
- In training mode, `target-history`, `firing-results`, `timeline`/`events`, and `guns/{gunId}/shot-summary` retain the bounded exercise behavior.

When `Stonehenge:UseLiveMiddlewareAstroData=true`, Astro reads target, shot, and gun summary history through the middleware history API, which is backed by PostgreSQL. The API does not connect directly to PostgreSQL and does not proxy middleware control routes. Seeded fallback is controlled separately by `Stonehenge:AllowSeededAstroFallback`; keep it `false` when validating live behavior so middleware/PSQL outages fail visibly instead of returning seeded history.

The seeded target/firing history includes examples across Guns 1-8 so live providers can be swapped in without changing the public Astro route shape. Live history can include any gun that appears in middleware history.

Astro list endpoints accept bounded polling/replay query parameters:

- `limit`
- `offset`
- `start` or `from`
- `end` or `to`
- `since`
- `gunId`

Astro responses do not expose middleware URLs, command routes, PLC tags/registers, FUXA panel IDs, service tokens, reset/fire/override details, or OpenPLC ports.

## Training Vulnerability Toggles

Every bounded gap is subordinate to `TrainingVulnerabilities:Enabled`, mapped by
Compose from `STONEHENGE_TRAINING_VULNS_ENABLED`. The root defaults to `false`;
no individual compatibility toggle can activate a gap while the root is off.

| Toggle | Default | Behavior |
| --- | --- | --- |
| `Enabled` | `false` | Single safe/training profile root for every toggle below. |
| `EnableVendorFieldExpansionLeak` | `true` | `?fields=all` or `?include=workflow,integration` adds bounded workflow/integration metadata to vendor part requests. |
| `EnableVendorGunStatusDiagnosticLeak` | `true` | `?include=source` or `?debug=true` adds bounded source/diagnostic metadata to vendor gun status, including `engineeringPartServiceThresholdShots` from canonical `Operational:MaintenanceShotThreshold` (Compose default `200`). |
| `EnableAstroFieldExpansionLeak` | `true` | `?fields=all` or `?include=summary` adds bounded plotting summary metadata to Astro firing results. |
| `EnableCompletionReplay` | `true` | Repeated vendor completion POSTs create duplicate local completion events. |
| `EnableCrossClientScopeConfusion` | `false` | When false, API keys must have exact scopes for each route group. |
| `EnableVerboseScopeErrors` | `true` | When true, valid but under-scoped keys receive bounded caller client, route group, required scope, accepted client type, and restricted-route hint metadata. When false, scope failures return only `api_key_scope_required`. |

These are intentionally bounded. They must not leak raw middleware command paths, FUXA identifiers, OpenPLC management details, PLC tags/registers, or real secrets.

## Development Reset

Development builds expose a seeded-memory reset endpoint:

```http
POST /api/v1/dev/simulation/reset
X-Stonehenge-Api-Key: engineering-dev-key
```

This resets only API-local seeded memory. It does not call the engineering app, middleware, FUXA, OpenPLC, Modbus, or any PLC-facing path.

## Test Scripts

From `stonehenge-api/`:

```bash
./scripts/generate-api-keys.sh
./scripts/test-vendor-api.sh
./scripts/test-astro-api.sh
./scripts/test-api-vulns.sh
./scripts/test-api-safety-boundary.sh
./scripts/test-astro-scope-boundary.sh
./scripts/test-engineering-export-flow.sh
./scripts/reset-simulation.sh
```

Override the API base when testing from another host:

```bash
API_BASE=http://<host-ip>:8092/api/v1 ./scripts/test-vendor-api.sh
```
