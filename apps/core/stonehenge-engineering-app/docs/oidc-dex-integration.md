# Dex OIDC Integration

## TLS edge

The monorepo Caddy service terminates TLS for `engineering.fcu.gov` and forwards
requests unchanged to the Engineering App. Caddy obtains its certificate from
the FCU Step CA ACME directory. The application trusts forwarded headers only
from Caddy's fixed `172.28.0.5` Compose address.

Caddy does not implement authorization or filter application routes. Existing
MVC policies, API policies, training paths, and OIDC callback behavior remain
inside ASP.NET Core.

The Engineering App selects `Dev` or `Oidc` authentication from deployment
configuration. Production OIDC mode disables development headers and login
helpers.

## Trust and endpoints

- Engineering App: `https://engineering.fcu.gov`
- Dex issuer (planned): `https://login.fcu.gov/dex`
- Active Directory: `192.168.30.2` (Dex connects to it; the app does not)
- Step CA: `https://192.168.30.3:8443`
- Step CA root fingerprint:
  `6D:89:64:3A:EA:8F:18:CE:E0:B0:9B:B8:16:30:53:F3:CA:B1:20:B8:F1:7A:65:FF:B4:DC:4C:37:3B:43:80:8C`

The root certificate is baked into the runtime image system trust store.

## Dex client

Configure a confidential OIDC client:

```yaml
id: stonehenge-engineering
name: Stonehenge Engineering Console
redirectURIs:
  - https://engineering.fcu.gov/signin-oidc
```

The app requests `openid profile email groups`, uses authorization code flow
with PKCE, and requires HTTPS metadata.

Dex does not currently advertise an OIDC `end_session_endpoint`. The app's
logout action revokes its local cookie and active Reset Toolbox grant and then
shows `/auth/signed-out`. The next Sign In request uses `prompt=login` and
`max_age=0`, producing a fresh Dex token without relying on unsupported
provider logout behavior.

## Required claims

Dex must provide:

- `sub`: stable account identifier
- `name`, `preferred_username`, or `email`: display identity
- `auth_time`: required for the Reset Toolbox fresh-login proof
- an Engineering boundary claim
- an admin boundary claim when applicable

FCU Dex group mapping:

```text
SCA-Employees
SCA-Admins
SCA-Engineering-Users
SCA-Engineering-Admins
```

Engineering membership is evaluated first. Administrative membership never
grants Engineering access by itself. `SCA-Engineering-Admins` locally inherits
Engineering access, while `SCA-Admins` receives only the Overview dashboard.

## App environment

```text
STONEHENGE_AUTH_MODE=Oidc
STONEHENGE_OIDC_AUTHORITY=https://login.fcu.gov/dex
STONEHENGE_OIDC_CLIENT_ID=stonehenge-engineering
STONEHENGE_OIDC_CLIENT_SECRET=<secret from Dex>
STONEHENGE_TRUSTED_PROXY_IP=<actual reverse proxy address>
STONEHENGE_SHOW_DEV_AUTH=false
ENGINEERING_APP_HOST_IP=127.0.0.1
```

Use `STONEHENGE_TRUSTED_PROXY_NETWORK` only when the proxy address cannot be
fixed, and scope it to the smallest possible network. The proxy must send
`X-Forwarded-For`, `X-Forwarded-Proto`, and `X-Forwarded-Host`.

Do not use direct HTTP port `8088` for OIDC browser sessions. The secure
`__Host-Stonehenge.Auth` cookie and OIDC callback are intended to run through
the TLS reverse proxy.

## Custom claim mappings

If Dex emits different values, override the boundary settings through standard
ASP.NET configuration. Examples:

```text
Authentication__Oidc__EngineeringBoundary__ClaimTypes__0=groups
Authentication__Oidc__EngineeringBoundary__Values__0=SCA-Engineering-Users
Authentication__Oidc__EngineeringBoundary__Values__1=SCA-Engineering-Admins
Authentication__Oidc__AdminBoundary__ClaimTypes__0=groups
Authentication__Oidc__AdminBoundary__Values__0=SCA-Engineering-Admins
```

Authorization is read only from the signed `groups` claim. OU, department,
distinguished-name, and generic administrator groups are not Engineering
authorization inputs.
