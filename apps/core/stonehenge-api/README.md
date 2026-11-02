# Stonehenge Internal API

`stonehenge-api` is the shared app-to-app API surface for Stonehenge companion applications. It is separate from `stonehenge-middleware` and should not become a raw PLC, OpenPLC, Modbus, FUXA, or middleware command proxy.

Current implementation uses live middleware history for Astro target/firing data when the Stonehenge stack is available, with API-local seeded simulation data as a development fallback. It also has an engineering export path for approved app-owned part/service requests so the future Vendor app flow can be tested before live database providers replace the seeded store.

## Current Endpoint Groups

- `GET /` returns a minimal service response when Swagger is disabled.
- `GET /swagger` exposes the OpenAPI UI only when explicitly enabled.
- `GET /api/v1/system/health` returns service health.
- `GET /api/v1/system/public-status` returns low-sensitivity public-client status for scoped clients.
- `GET /api/v1/system/info` returns service metadata.
- `GET /api/v1/apps/catalog` returns the app catalog.
- `/api/v1/vendor/*` exposes vendor-safe part request and gun status simulation endpoints.
- `POST /api/v1/engineering/part-requests` accepts approved app-owned part/service requests exported by the Engineering App.
- `POST /api/v1/engineering/part-requests/{requestId}/withdraw` hides an exported request from active vendor polling without deleting completion history.
- `/api/v1/engineering/vendor-completions/*` exposes engineering sync endpoints for vendor completion events.
- `/api/v1/astro/*` exposes public sanitized target/firing history, firing result, gun summary, and timeline endpoints for the future Astro map.
- `POST /api/v1/dev/simulation/reset` resets API-local seeded memory in development only.

## Run

From the monorepo root:

```bash
docker compose up -d --build stonehenge-api
```

Swagger is disabled by default. Enable it only when needed:

```bash
STONEHENGE_API_SWAGGER_ENABLED=true docker compose up -d --build stonehenge-api
```

Then open:

```text
http://127.0.0.1:8092/swagger
```

For dev exposure on the Stonehenge host, the monorepo compose may publish the API at:

```text
http://<host-ip>:8092/swagger
```

## Dev API Keys

Protected route groups use `X-Stonehenge-Api-Key`.

Generate local development keys before normal local runs:

```bash
./scripts/generate-api-keys.sh
```

The generated plaintext keys are written to `.generated/stonehenge-api-keys.env`, which is ignored by Git. The API loads `.generated/api-key-seed.generated.json`, which contains only hashed key material.

Generated clients:

- `STONEHENGE_ENGINEERING_API_KEY` for engineering sync placeholder endpoints.
- `STONEHENGE_VENDOR_API_KEY` for vendor endpoints.
- `STONEHENGE_ASTRO_API_KEY` for optional Astro authenticated reads and scoped public status, though Astro data endpoints are public in this simulation.
- `STONEHENGE_VALIDATION_HARNESS_API_KEY` for local validation tooling.

Legacy keys remain accepted while `ApiKeyAuth:AllowLegacyDevKeys=true`: `vendor-dev-key`, `engineering-dev-key`, and `astro-dev-key`.

## Test Scripts

From this directory:

```bash
./scripts/test-vendor-api.sh
./scripts/test-astro-api.sh
./scripts/test-astro-live-history.sh
./scripts/test-api-vulns.sh
./scripts/test-api-resource-limits.sh
./scripts/test-api-safety-boundary.sh
./scripts/test-astro-scope-boundary.sh
./scripts/test-engineering-export-flow.sh
./scripts/reset-simulation.sh
```

Use another base URL when needed:

```bash
API_BASE=http://<host-ip>:8092/api/v1 ./scripts/test-vendor-api.sh
```

Reset API-local seed state before an export-flow test when useful:

```bash
RESET_SIMULATION=true API_BASE=http://<host-ip>:8092/api/v1 ./scripts/test-engineering-export-flow.sh
```

Set `VendorApi:UseSeededData=false` when testing only Engineering-App-exported requests. Seeded data is a development bridge; the intended long-term shape is live providers reading Engineering App SQLite, middleware/PostgreSQL summaries, and other app-owned sources without raw PLC/FUXA/control routes.

Astro live history is controlled with:

```json
"Stonehenge": {
  "MiddlewareBaseUrl": "http://middleware:8080",
  "UseLiveMiddlewareAstroData": true,
  "AllowSeededAstroFallback": false
}
```

When enabled, Astro endpoints read sanitized target, shot, and gun summary history through middleware history APIs backed by PostgreSQL. They do not connect directly to PostgreSQL and do not expose middleware command routes. Keep `AllowSeededAstroFallback=false` when validating live behavior so middleware/PSQL outages fail visibly instead of returning seeded history.

## Simulation Docs

See [`docs/API_SIMULATION.md`](docs/API_SIMULATION.md) for seeded flows, endpoint purpose, and bounded training vulnerability toggles. See [`docs/API_KEYS.md`](docs/API_KEYS.md) for generated local key handling.

## Boundaries

- Do not connect this API directly to OpenPLC or Modbus.
- Do not proxy raw middleware commands through this API.
- Do not expose FUXA panel IDs, PLC tags/registers, raw Modbus/register values, OpenPLC management ports, or middleware command paths.
- Do not add fire, reset, override, PLC upload, or OpenPLC runtime-management routes.
- Keep app workflow data distinct from middleware runtime state.
- The development simulation reset route resets seeded API memory only; it is not a runtime or PLC reset route.
