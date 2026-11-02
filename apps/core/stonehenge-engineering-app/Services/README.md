# Services

Services hold the app's integration and persistence boundaries.

## Middleware Boundary

`IStonehengeMiddlewareClient` and `StonehengeMiddlewareClient` are the only intended path from the webapp to middleware runtime APIs. The app should not talk directly to OpenPLC, Modbus, or middleware PostgreSQL.

## Dashboard Data

`IEngineeringDashboardData` is the page-facing data adapter. `LiveEngineeringDashboardData` hydrates views from middleware and app-owned stores. `MockEngineeringDashboardData` remains available as a fallback/scaffold source.

## SQLite Stores

SQLite stores persist app-owned data:

- maintenance requests
- part requests/status counters
- app event logs
- engineering command records

SQLite is not the source of truth for PLC state, dispatcher state, target history, or middleware audit tables.

## FUXA Embedding

`IEngineeringPanelCatalog`, `StaticEngineeringPanelCatalog`, and `IFuxaEmbedService` map selected guns/pages to known FUXA panel IDs and embed URLs. Keep this catalog explicit so the MVC app chooses the panel instead of exposing FUXA navigation.
