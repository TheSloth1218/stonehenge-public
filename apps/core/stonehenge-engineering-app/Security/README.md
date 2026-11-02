# Security

This folder contains the temporary development authentication model used before portal SSO is integrated.

- `DevAuthenticationHandler` reads `X-Dev-User` and maps known users to claims.
- `StonehengeAuth` defines policy names, group names, and the internal service header/token names.
- `ClaimsPrincipalExtensions` contains small helpers used by controllers/views.

Current development identities:

- `employee.basic` - dashboard/status user.
- `engineering.user` - engineering operator user.
- `engineering.admin` - engineering admin user.

Future portal integration should replace the dev handler while preserving the policy boundaries: dashboard, engineering, engineering admin, and internal workflow service access.

Training security is safe-by-default under `TrainingVulnerabilities:Enabled`.
`TrainingSessionRegistry` revokes/checks marked cookies, `TrainingTicketIssuer`
creates genuine cookie middleware tickets, `RecoveryTokenValidator` validates
Dex recovery ID tokens without saving them, and `PanelCapabilityService`
protects the bounded panel 99 read. See `../docs/training-vulnerabilities.md`.
