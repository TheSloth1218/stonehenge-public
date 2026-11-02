# Stonehenge Control Authority Astro App

> For the public combined stack, start with the [root README](../../README.md). The instructions below describe this app as a standalone source snapshot.

Public-facing Stonehenge Control Authority website for an authorized CPTC mock competition environment.

The app uses the included ACES04 TrueType font package. Credits and redistribution details are in the [asset notes](../../docs/ASSETS.md). The font paths are:

```text
public/fonts/ACES04.ttf
public/fonts/ACES04_Heavy.ttf
public/fonts/ACES04_PA.ttf
public/fonts/ACES04_PA_Heavy.ttf
```

Create the deployment environment and run with Docker Compose:

```bash
cp .env.example .env
# Set the canonical DNS names, upstream API endpoint, and secrets in .env.
docker compose up --build
```

Then open the configured `ASTRO_PUBLIC_ORIGIN`. Caddy publishes the canonical
HTTPS origin and proxies to Astro over the shared `stonehenge-edge` Docker
network. Port `3000` remains bound to localhost for diagnostics.
Start `/opt/login-app` first so that shared edge network exists.

For local development without Docker, `npm run dev` also binds to `0.0.0.0`.

## Stonehenge API

Astro is observational and talks only to `stonehenge-api` through its own Next backend route. Configure the backend with:

```bash
STONEHENGE_API_BASE_URL=http://<stonehenge-api-host>:8092/api/v1
STONEHENGE_ASTRO_API_KEY=<generated astro key>
```

The frontend polls `/api/astro/live`; that backend route injects `X-Stonehenge-Api-Key` and normalizes middleware-history records for the map. Do not point Astro at middleware, PostgreSQL, SQLite, OpenPLC, Modbus, FUXA, PLC runtimes, or command/control routes.

## FCU OIDC sign-in

Astro is registered with the Dex stack in `/opt/login-app` as the
`stonehenge-astro` OIDC client. Authentication is optional and does not gate or
change the public tracker. A successful sign-in adds only the authenticated
display name and local sign-out control to the top bar.

Configure both compose projects with matching values. The names below illustrate
the expected `*.fcu.gov` layout and are replaceable environment values:

```bash
DEX_ISSUER=https://login.fcu.gov/dex
DEX_INTERNAL_ISSUER=https://login.fcu.gov/dex
ASTRO_PUBLIC_ORIGIN=https://sca.fcu.gov
ASTRO_OIDC_CLIENT_SECRET=<same random value configured in Dex>
ASTRO_SESSION_SECRET=<independent random value of at least 32 bytes>
```

Astro validates the Dex ID token signature, issuer, audience, expiry, and nonce,
then stores only a signed, HTTP-only application session. It does not place OIDC
tokens in browser storage. Future apps join the identity plane by registering
their own client and callback while retaining their own application session.
With an upstream OIDC provider such as AD FS, that provider's browser session
supplies cross-app SSO. Dex's local-password and direct LDAP connectors validate
the same identity but prompt once per application because LDAP has no browser
session.

For DNS or a reverse proxy, replace all public origins with their canonical
external HTTPS URLs in both projects. The issuer and callback URLs must match
exactly. `DEX_INTERNAL_ISSUER` may remain a private service URL when the app and
Dex share a network; it is used only for server-side token and key requests.

## Validation

Install dependencies and Playwright browsers, then run the end-to-end smoke tests:

```bash
npm install
npm run test:e2e:install
npm run test:e2e
```

The Playwright suite starts the Next dev server automatically and validates the public tracker on desktop and mobile Chromium profiles.

## Automated Competition Reviewer

The challenge service is part of every supported deployment; there is no safe-mode environment selector. The Staff page exposes an application form whose profile field is presented as LinkedIn-only. The backend accepts canonical `https://www.linkedin.com/in/...` profiles but intentionally also accepts the configured Astro origin at `/ulysses-map`. That bypass can reach the isolated unsafe missing-target highlighter. Applications are accepted at `POST /api/astro/apply`, with bounded status available at `GET /api/astro/apply/{submissionId}`.

The app and reviewer share only an Astro-owned SQLite database at `ASTRO_REVIEW_DB_PATH`. Jobs move through:

```text
queued -> claimed -> visiting -> completed
                    |-> retryable_failed -> claimed
                    |-> permanently_failed
                    |-> expired
```

Claims are transactional and leased. Expired leases are reclaimed, failures use bounded exponential backoff, attempts are capped, and the persistent `astro-review-data` Docker volume survives either service restarting. Submitted query values are redacted from ordinary worker logs.

The reviewer uses one isolated Playwright context per job. Top-level navigation is limited to either the exact submitted LinkedIn profile or the configured Astro `/ulysses-map` training bypass. LinkedIn browsing remains confined to `https://www.linkedin.com`; the Astro path permits only same-origin Next assets, the live public API, and the exact submission collector. Other origins, paths, popups, tabs, and downloads are blocked. Its current `SyntheticTrainingReviewerIdentityProvider` seeds the non-authenticating marker `stonehenge-training-employee-session-v1`, `employee.basic`, and the exact synthetic metadata `campaign=NIGHTWATCH`, `migrationJob=ENG-1847`, `directoryAlias=count`, `entryUser=chopper`, `legacyComputer=FCU-LEGACY1`, and `relatedRequestId=2003` only on the Astro origin, never on LinkedIn. None of these values authenticates anywhere.

The training payload may submit a nonempty subset of only those six metadata fields to the same-origin, per-submission collector. Unknown fields, JWTs, credential/API-key patterns, bodies over 4 KiB, and individual values over 1 KiB are rejected. Accepted submissions emit a values-free structured detection with finding ID `FCU-ASTRO-001`, caller/job, field names, and a unique correlation ID. The vulnerable renderer and its application and collector routes are always active.

Future identity replacement is intentionally narrow:

```text
AD -> Dex over LDAPS -> OIDC reviewer identity -> isolated Playwright context
```

A future `DexReviewerIdentityProvider` will maintain and verify low-privilege storage state for `sca.mapreview`, reauthenticate on expiry, and implement the existing identity interface. The queue and worker require no rewrite. Human Astro sign-in already uses the separate HTTP-only OIDC application session; the synthetic reviewer marker remains intentionally isolated from it.

Start the competition-ready stack:

```bash
docker compose up -d --build
curl "${ASTRO_PUBLIC_ORIGIN}/api/health"
curl "${ASTRO_REVIEWER_HEALTH_URL}"
```

Run validation without host Node/npm:

```bash
docker compose build astro-test
docker compose run --rm --no-deps astro-test npm run lint
docker compose run --rm --no-deps astro-test npm run typecheck
docker compose run --rm --no-deps astro-test npm run test:unit
docker compose run --rm astro-test npm run test:e2e
docker run --rm --network host \
  -e ASTRO_PUBLIC_ORIGIN="$ASTRO_PUBLIC_ORIGIN" \
  -e ASTRO_REVIEWER_HEALTH_URL="$ASTRO_REVIEWER_HEALTH_URL" \
  astro-app-astro-reviewer npm run training:test:reviewer
bash tools/training/run-reviewer-recovery.sh
```

The demonstration writes sanitized Markdown and JSON evidence under `artifacts/training/astro-reviewer/`. It never records the Stonehenge API key, environment dumps, cookies, or raw exploit query values.

The reviewer runtime installs only production dependencies. Its `test` build
target contains the development dependencies needed for lint, type checks, and
tests. Audit both targets when updating the lockfile; a production-only audit
does not describe the test image.

## Map Reference

The regional tracker supports a private local bitmap layer for authorized competition environments. Place the map image here before building:

```text
public/maps/Usean_continent.jpg
```

The regional map uses that JPG directly, with live intercept dots and the public feed rendered over it.
