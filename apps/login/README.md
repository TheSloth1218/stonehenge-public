# Stonehenge Login Portal

> For the public combined stack, start with the [root README](../../README.md). The instructions below describe this app as a standalone source snapshot.

Dex-backed OIDC identity portal for Stonehenge applications. The portal and each
application maintain their own HTTP-only session. Every app trusts the same Dex
issuer, so adding another app does not require sharing cookies or application
credentials.

## FCU interface theme

The portal and Dex credential screen share the Stonehenge FCU interface theme:
the dark royal-blue SCA shield, FCU flag colors, ACES04 typography, cyan tactical
lines, and restrained red/silver authority accents. Dex receives the theme from
`dex/theme` through the read-only Compose mount at
`/srv/dex/web/themes/stonehenge`.

The ACES04 font files are proprietary local assets and remain ignored by Git.
Before building on a new host, place `ACES04.ttf`, `ACES04_Heavy.ttf`, and
`ACES04_PA.ttf` in both `public/` and `dex/theme/`. The shield, flag, and theme
CSS are repository assets.

## Local mock identity

1. Copy `.env.example` to `.env` and replace the development secrets.
2. Start the stack with `docker compose up --build -d`.
3. Open the configured `LOGIN_PORTAL_PUBLIC_ORIGIN`.
4. Sign in with `employee@sca.local` / `stonehenge-dev`.

The mock password database is intentionally temporary. It is not the future AD
directory and must not be used as a production credential store.

Generate a replacement bcrypt hash with
`npm run dex:hash-password -- <new-password>` inside the Node container, then
replace the `staticPasswords` hash in `dex/config.local.yaml`.

## DNS or reverse proxy deployment

OIDC depends on exact public URLs. Set `DEX_ISSUER`,
`LOGIN_PORTAL_PUBLIC_ORIGIN`, and `ASTRO_PUBLIC_ORIGIN` to their externally
visible HTTPS origins, then use the same values in the reverse proxy and Astro
compose environment. The issuer must remain stable after users begin signing in.

Caddy publishes `https://login.fcu.gov/` for the portal,
`https://login.fcu.gov/dex/` for Dex, and `https://sca.fcu.gov/` for Astro.
It enrolls certificates through the FCU Smallstep ACME directory. Application
ports `3000`, `3002`, and `5556` remain localhost-only for diagnostics.
Start this Compose project before Astro so it creates the shared
`stonehenge-edge` network.

Vendor is registered as the confidential OIDC client `stonehenge-vendor` with
the callback `https://vendor.fcu.gov/login/oauth2/code/dex`. Its client secret
is supplied through `VENDOR_OIDC_CLIENT_SECRET`; never reuse another
application's secret.

The Vendor Spring application should use these settings:

```text
SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_DEX_PROVIDER=dex
SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_DEX_CLIENT_ID=stonehenge-vendor
SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_DEX_CLIENT_SECRET=<vendor secret>
SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_DEX_CLIENT_AUTHENTICATION_METHOD=client_secret_basic
SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_DEX_AUTHORIZATION_GRANT_TYPE=authorization_code
SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_DEX_REDIRECT_URI={baseUrl}/login/oauth2/code/{registrationId}
SPRING_SECURITY_OAUTH2_CLIENT_REGISTRATION_DEX_SCOPE=openid,profile,email,groups
SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_DEX_ISSUER_URI=https://login.fcu.gov/dex
SPRING_SECURITY_OAUTH2_CLIENT_PROVIDER_DEX_USER_NAME_ATTRIBUTE=preferred_username
SERVER_FORWARD_HEADERS_STRATEGY=framework
```

The Vendor JVM must trust the FCU root CA so Spring can retrieve issuer metadata
and exchange authorization codes over HTTPS. The application must map the Dex
`groups` claim to its own server-side roles; receiving a group claim does not
grant a role by itself.

Engineering is registered as `stonehenge-engineering` with callback
`https://engineering.fcu.gov/signin-oidc`. Configure the application with the
Dex issuer, its unique client secret, the `openid profile email groups` scopes,
and PKCE. Engineering remains responsible for enforcing its OU-first RBAC rules;
Dex only authenticates the user and emits trusted directory group claims.

`engineering.fcu.gov` resolves to the Engineering host, where its HTTPS reverse
proxy forwards to Kestrel on the private application port. Set
`STONEHENGE_TRUSTED_PROXY_IP` to the source address Kestrel sees for that local
proxy and keep port `8088` private after the HTTPS route is active. Do not point
Engineering DNS at the separate Login/SCA edge host.

Forward `Host`, `X-Forwarded-Host`, `X-Forwarded-Proto`, and the client address.
Terminate TLS at the proxy or the services, but do not mix HTTP callback URLs
with an HTTPS issuer. The apps do not derive callback URLs from untrusted request
headers; they use the configured canonical origins.

## Adding Active Directory

`dex/config.upstream-oidc.example.yaml` is preferred for true cross-application
browser SSO. A domain controller alone is not an OIDC provider; front AD with
AD FS or another AD-backed OIDC provider, register Dex's `/callback`, set the
`DEX_UPSTREAM_OIDC_*` variables, and mount that template. The upstream provider
then owns the reusable browser login session while every Stonehenge app
continues to trust only Dex.

`dex/config.ldap.yaml` authenticates FCU Active Directory users over
verified LDAPS. Dex remains the OIDC provider trusted by Astro and the central
login portal; the applications never connect to LDAP directly. LDAP itself has
no reusable browser session, so users may be prompted again when a new Dex
authorization request requires authentication.

1. The FCU root from `https://ca.fcu.gov:8443/roots.pem` is tracked at
   `dex/ca/fcu-lab-pki-root-ca.crt` and mounted read-only as
   `/etc/dex/ldap-ca.pem`.
2. Install it into the host trust store with
   `sudo ./scripts/install-fcu-root-ca.sh`.
3. Create a read-only directory bind account and set the `DEX_LDAP_*`
   environment variables.
4. Set `DEX_CONFIG_FILE=config.ldap.yaml`.
5. Recreate Dex and confirm user and group claims with a low-privilege account.

The expected directory endpoint is `fcu-dc.fcu.gov:636`, and the default search
base is `DC=fcu,DC=gov`. Certificate and hostname verification remain enabled.
Do not set `insecureSkipVerify`.

Dex limits the OIDC `groups` claim to the canonical SCA authorization groups.
Applications translate those group names into their own roles:

| Dex group claim | Application role |
| --- | --- |
| `SCA-Employees` | `employee.basic` |
| `SCA-Admins` | `admin` |
| `SCA-Engineering-Users` | `engineering.user` |
| `SCA-Engineering-Admins` | `engineering.admin` |

Dex does not emit an application-specific `role` claim. Each application must
authorize the authenticated identity from the signed `groups` claim and must
not infer privileges from display names, email addresses, or client input.

The Smallstep ACME directory is
`https://ca.fcu.gov:8443/acme/acme/directory`. The CA endpoint must present a
currently valid HTTPS certificate before an ACME client can enroll or renew
certificates.

No application auth code changes are needed. Keep app authorization local to
each application and use Dex groups only as identity claims.

## LDAP safe and CPTC training profiles

`dex/config.ldap.yaml` remains the safe active base. The explicit
`dex/config.ldap.safe.yaml` profile has the same behavior and no cross-client
trust. The inert `dex/config.ldap.training.yaml` profile differs only in this
bounded Dex relationship:

```yaml
- id: stonehenge-engineering
  trustedPeers: [stonehenge-vendor]
```

This permits a token obtained through a normal Vendor OIDC authorization flow
to request Dex's dynamic audience scope
`audience:server:client_id:stonehenge-engineering`. It does not grant a group or
application role. Engineering must continue to perform its own authorization.
There is no reciprocal relationship, and the login portal and Astro are not
trusted peers. Every client retains its own environment-supplied secret and
redirect URI.

Validate both profiles before selection. These checks read the templates but
do not start or change a service:

```sh
npm run test:dex-profiles
DEX_CONFIG_FILE=config.ldap.safe.yaml docker compose config --quiet
DEX_CONFIG_FILE=config.ldap.training.yaml docker compose config --quiet
```

Only an authorized training operator should select the training profile. From
the repository root, use a shell-local selection so no secret file is read or
rewritten, validate the rendered Compose model, and then recreate only Dex:

```sh
export DEX_CONFIG_FILE=config.ldap.training.yaml
docker compose config --quiet
docker compose up -d --no-deps --force-recreate dex
```

Confirm the running container's selected read-only mount without displaying
environment variables or configuration secrets:

```sh
docker inspect stonehenge-dex --format '{{ range .Mounts }}{{ if eq .Destination "/etc/dex/config.yaml" }}{{ .Source }} -> {{ .Destination }} (rw={{ .RW }}){{ end }}{{ end }}'
docker compose ps dex
```

Rollback explicitly to the safe LDAP profile and recreate only Dex:

```sh
export DEX_CONFIG_FILE=config.ldap.safe.yaml
docker compose config --quiet
docker compose up -d --no-deps --force-recreate dex
docker inspect stonehenge-dex --format '{{ range .Mounts }}{{ if eq .Destination "/etc/dex/config.yaml" }}{{ .Source }} -> {{ .Destination }} (rw={{ .RW }}){{ end }}{{ end }}'
```

After either operation, complete an ordinary low-privilege OIDC login and
application authorization check. A real swordsman seed must originate in an
instructor-authenticated OIDC flow and remain an opaque runtime secret outside
Git. Do not paste a token into a config, command history, test fixture, or
runbook; these profiles add no token minting or impersonation endpoint.
