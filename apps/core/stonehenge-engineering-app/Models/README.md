# Models

Models are Razor view models and app-owned record types.

The main page models are:

- `EngineeringOverviewViewModel`
- `AutoStatusViewModel`
- `ManualControlViewModel`
- `MaintenancePanelViewModel`
- `AdminPanelViewModel`

Supporting models describe gun status, runtime status, safety state, FUXA frames, maintenance requests, part status, operation logs, and command records.

Keep these models UI/application focused. Middleware wire formats should be translated in services instead of leaking raw middleware JSON into Razor views.
