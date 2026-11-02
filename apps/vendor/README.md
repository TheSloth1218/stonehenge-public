# SCA Parts Fulfillment Portal

> For the public combined stack, start with the [root README](../../README.md). The instructions below describe this app as a standalone source snapshot.

Maintenance Request Fulfillment & Shipping is a native Spring Boot application for local Stonehenge logistics work. It maintains PostgreSQL-backed request fulfillment orders, inventory, truck shipments, activity, synchronization history, and auxiliary shipment-processing jobs. The browser never calls `stonehenge-api` directly.

## Runtime

- Ubuntu 24, Java 21, Spring Boot 3.3.7, Thymeleaf, PostgreSQL, Flyway
- Native host deployment; Docker is not a production dependency
- Default listener: `127.0.0.1:8089`
- Service identity: `stonevendor` with supplementary training group `vendor-support`
- Profiles: `dev`, `training`, `safe`, and `test`

Install host packages:

```bash
sudo apt update
sudo apt install -y openjdk-21-jdk maven postgresql postgresql-contrib git curl sudo python3 nginx certbot
sudo systemctl enable --now postgresql
```

## Database

```bash
sudo -u postgres psql -f scripts/create-local-db.sql
```

If PostgreSQL cannot read the repository path:

```bash
install -m 0644 scripts/create-local-db.sql /tmp/create-local-db.sql
sudo -u postgres psql -f /tmp/create-local-db.sql
```

Flyway applies V1 through V7. V5 migrates all old roles to exactly `EMPLOYEE` and `ADMIN` and adds the immutable Dex subject field. V6 creates the claim-based shipment job queue. V7 removes the original `ENG-REQ-*` development request/order graph and prepares maintenance requests for API acknowledgement state. Do not edit applied migrations.

Reset development data with `./scripts/reset-local-db.sh`.

## Authentication

Dex authenticates production users through OIDC. The application maps only the signed `groups` claim to Spring authorities and stores only a local profile/audit shadow keyed by `sub`. It has no passwords, local login form, registration, role editor, Dex administration, or user-management UI.

```bash
export OIDC_ISSUER=https://login.fcu.gov/dex
export OIDC_CLIENT_ID=stonehenge-vendor
export OIDC_CLIENT_SECRET=replace-me
export OIDC_SCOPES=openid,profile,email,groups
export APP_PUBLIC_ORIGIN=https://vendor.fcu.gov
export AUTH_SESSION_SECRET="$(openssl rand -hex 32)"
export SCA_EMPLOYEE_GROUP=SCA-Employees
export SCA_ADMIN_GROUP=SCA-Admins
```

Dex emits signed AD group names and does not emit application roles. Vendor performs this local mapping:

| Signed Dex `groups` value | Vendor application role | Access |
| --- | --- | --- |
| `SCA-Employees` | `employee.basic` (`ROLE_EMPLOYEE`) | Normal Vendor workflows |
| `SCA-Admins` | `admin` (`ROLE_ADMIN` and `ROLE_EMPLOYEE`) | Vendor administration and normal workflows |
| `SCA-Engineering-Users` | none | Ignored by Vendor |
| `SCA-Engineering-Admins` | none | Ignored by Vendor |

Users with neither trusted Vendor group are denied. The `employee.basic` and `admin` names describe Vendor's local authorization roles; they are not expected as Dex claims. Admin deliberately implies employee access in this application. Engineering performs its own separate role mapping from its audience-specific token.

Register this confidential client in Dex with the callback URL matching the externally visible Vendor URL:

```yaml
staticClients:
  - id: stonehenge-vendor
    name: Stonehenge Vendor Portal
    secret: '{{ getenv "VENDOR_OIDC_CLIENT_SECRET" }}'
    redirectURIs:
      - https://vendor.fcu.gov/login/oauth2/code/dex
```

The browser is sent directly to `/oauth2/authorization/dex`. The backend uses Authorization Code Flow with PKCE S256 and requests `openid profile email groups`. Spring validates issuer, RS256 signature through discovery/JWKS, timestamps, audience/authorized party, nonce, and callback state before `OidcUserMapper` accepts immutable `sub` and trusted groups. Client credentials and token exchange remain backend-only.

The local session cookie is named `STONEHENGE_VENDOR_SESSION`, is `HttpOnly`, `Secure`, and `SameSite=Lax`, and contains only an opaque server-side session identifier. Production session identifiers are HMAC-derived with the application-specific `AUTH_SESSION_SECRET`; never reuse the Engineering or Astro value. Sign-out clears the Vendor session and cookie. The current Dex deployment does not advertise an `end_session_endpoint`, so its central SSO session remains available for a later sign-in.

The host labels above are examples, not application constants. Any deployment domain works when DNS, certificates, proxy headers, `OIDC_ISSUER`, `APP_PUBLIC_ORIGIN`, and the exact Dex redirect URI agree. Do not enter a wildcard redirect URI in Dex.

For a reverse-proxied TLS deployment, set `SERVER_FORWARD_HEADERS_STRATEGY=framework` only when the proxy replaces untrusted forwarding headers. The callback is fixed from `APP_PUBLIC_ORIGIN`, which must exactly match the Dex client registration. A complete host environment template is provided at `config/stonehenge-vendor.env.example`; install the real file outside the repository with mode `0600`. See `docs/deployment/networking.md` for the nginx, FCU root CA, ACME, subnet, and DNS checklist.

Apply a generated Dex handoff without sourcing it as shell code:

```bash
sudo deploy/install-oidc-handoff.sh /secure/path/vendor-oidc.env
sudo systemctl restart sca-parts-fulfillment.service
```

`dev` and `test` are the only profiles that enable seeded development identity support. They use `SCA_DEV_USER` and do not provide a fake login form.

## Run

Local development without Dex:

```bash
export SPRING_PROFILES_ACTIVE=dev
export SCA_DEV_USER=operator
export SPRING_DATASOURCE_URL=jdbc:postgresql://localhost:5432/stonehenge_vendor
export SPRING_DATASOURCE_USERNAME=stonehenge_vendor
export SPRING_DATASOURCE_PASSWORD=change-me-local-dev
mvn spring-boot:run
```

The Maven `spring-boot:run` goal selects `dev` by default. Override it explicitly for another mode, for example `mvn spring-boot:run -Dspring-boot.run.profiles=safe`. Packaged JAR and systemd execution do not inherit this Maven-only default.

Safe deployment with Dex:

```bash
export SPRING_PROFILES_ACTIVE=safe
export SESSION_COOKIE_SECURE=true
mvn spring-boot:run
```

Training behavior with real Dex uses `SPRING_PROFILES_ACTIVE=training`. This keeps OIDC and application RBAC active while retaining the bounded vulnerable behaviors. The `dev,training` combination deliberately substitutes the seeded development identity and must remain isolated from normal users.

Isolated training deployment with seeded identity:

```bash
export SPRING_PROFILES_ACTIVE=dev,training
export SCA_DEV_USER=operator
mvn spring-boot:run
```

Never use `training` for a normal deployment. Vulnerable behavior is not the default.

## Authorization

Employees perform normal request, order, catalog browsing, inventory, shipping, and activity workflows. Admins inherit those capabilities and additionally manage catalog records, review full inventory and sync history, retry eligible completion failures, inspect redacted health/connectivity data, and operate shipment jobs. Admins cannot administer users or Dex.

The shared Thymeleaf navigation hides admin controls from employees, while URL rules and `@PreAuthorize` enforce the same boundary on the server. CSRF protection remains enabled for browser forms.

## Stonehenge API

The backend uses the typed vendor-safe API contract:

- `GET /vendor/part-requests`
- `GET /vendor/part-requests/{requestId}`
- `POST /vendor/part-requests/{requestId}/acknowledge`
- `POST /vendor/part-requests/{requestId}/complete`
- `GET /vendor/completions`
- `GET /vendor/guns/status`

It uses `STONEHENGE_API_BASE_URL`, `STONEHENGE_VENDOR_API_KEY`, and `X-Stonehenge-Api-Key`. Approved requests are polled and upserted into the app's local workflow database with bounded retry/backoff. Beginning fulfillment acknowledges the request remotely. A shipment and inventory deduction commit locally before completion is sent with its tracking reference. A failed completion call preserves the shipment, marks synchronization failed, and can be retried without creating another shipment or deducting inventory twice.

The app never writes the API key to PostgreSQL, templates, synchronization history, or logs. It calls only `stonehenge-api`; it does not access Engineering storage, middleware, PLC, FUXA, OpenPLC, or Modbus services.

## Shipment worker

Every legitimate shipment creates an auxiliary compatibility job. The worker claims one row with a PostgreSQL lock, tracks bounded attempts, and records sanitized failures. It never replaces the main shipment transaction.

- `safe`: strict JSON DTO, fixed carrier/method enums, unknown-field rejection, and persisted relationship checks
- `training`: native Java compatibility decoding in the background worker, with the intentionally retained Commons Collections 3.2.1 legacy runtime used by the bounded black-box exercise

The training profile also exposes selected authenticated Actuator metadata and narrowly scoped authorization/trust-boundary scenarios. See `training/vulnerabilities.yml`; instructor operation is documented in `docs/INSTRUCTOR.md`. The normal README intentionally contains no exploit payload.

## Host support collector

`deploy/vendor-support` contains the opt-in host package for `SCA-VENDOR-008`. The root-owned collector is installed at `/usr/local/sbin/vendor-support`; it is never executed by the Spring application. Training provisioning makes only its plugin directory group-writable and installs one narrowly named sudo command. Safe provisioning restores root ownership, non-writable modes, canonical-path checks, and a SHA-256 plugin allowlist.

```bash
sudo deploy/vendor-support/install.sh training
# or
sudo deploy/vendor-support/install.sh safe
```

Provisioning changes users, groups, sudoers, systemd overrides, and root-owned paths, so it must be run explicitly on a disposable host. The normal service unit retains `NoNewPrivileges=true`; only the training installer adds an override. The Vendor Linux host remains standalone and is never joined to Active Directory; authentication is application-level Dex OIDC over HTTPS.

## Tests

```bash
mvn test
```

Focused unit/profile tests (including the recovery contract) run without Docker, for example:

```bash
mvn -Dtest=SessionSnapshotToolTest,TrainingRouteProfileTest,CompletionTargetProfileTest test
python3 -m unittest deploy/session-recovery/tests/test_session_recovery.py
```

The full `mvn test` suite includes PostgreSQL integration tests using Testcontainers and therefore requires a working Docker socket. Docker is permitted for tests only. Optional browser smoke coverage:

```bash
RUN_PLAYWRIGHT=true PLAYWRIGHT_BASE_URL=http://127.0.0.1:8089 mvn -Dtest=FulfillmentBrowserWorkflowTest test
```

Tests must use controlled marker/mock sinks and must never open a reverse shell.

## Systemd

The optional SCA-VENDOR-009 encrypted session-recovery handoff is documented only in the instructor guide. It accepts protected real artifacts through root-only stdin and ships with no seed or fabricated token material. Before deploying it, an operator must manually install the auditd tools (`augenrules` and `ausearch`) and ensure the `auditd` service is active on systemd hosts; this host does not currently meet that prerequisite.

The example unit is `systemd/sca-parts-fulfillment.service`. Create the `stonevendor` user/group, place environment configuration under `/etc/stonehenge-vendor`, keep application state under `/var/lib/stonehenge-vendor`, and logs under `/var/log/stonehenge-vendor`. The base unit grants no sudo, Linux capabilities, Docker socket, PLC, FUXA, Modbus, middleware, or Engineering database access. The isolated training host package adds only the support-collector boundary described above.
