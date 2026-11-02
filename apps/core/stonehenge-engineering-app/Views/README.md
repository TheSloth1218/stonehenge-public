# Views

Razor views define the operator workspaces.

- `Engineering/Overview.cshtml` - STN dashboard status and array overview.
- `Engineering/Auto.cshtml` - automatic operation workspace and auto HMI embed.
- `Engineering/Manual.cshtml` - manual operation workspace, HMI embed, filtered logs, and protected fire-control UI.
- `Engineering/Maintenance.cshtml` - app-owned maintenance and part request workflow.
- `Engineering/Admin.cshtml` - admin HMI selection, safety/admin controls, requests, logs, and reset toolbox.
- `Shared/_Layout.cshtml` - app shell, header, navigation, scripts, and shared styling references.
- `Shared/_FuxaPanelFrame.cshtml` - reusable embedded FUXA frame shell.

Views should avoid direct middleware calls. Use controller view models and `/api/*` polling endpoints for dynamic updates.
