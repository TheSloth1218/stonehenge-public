# Training vulnerability profiles

`TrainingVulnerabilities:Enabled` is the only deployment root for the bounded
training surfaces. It maps to `STONEHENGE_TRAINING_VULNS_ENABLED` and defaults
to `false` in both app settings and Compose. The API's individual legacy gap
switches are subordinate to this root and cannot activate a gap while it is
false.

## Profiles

Safe profile (`false`): training session issuance is disabled; any previously
issued training-marked cookie is rejected; recovery returns 404; employee
workflow IDOR, EngineeringUser command detail/replay, and EngineeringUser admin
reads are denied. Panel 99 requires EngineeringAdmin and a short-lived signed
panel capability. Vendor/Astro/public/employee DTOs omit classified operational
fields.

Training profile (`true`): the existing bounded workflow, command-detail,
app-local replay, API expansion, and diagnostics gaps are enabled. They do not
add a middleware proxy or a mutation path. EngineeringUser can mint a 30-300
second signed capability for canonical panel `99` (`admin-diagnostics`).

## Count cookie artifact (FCU-ID-001)

The tool must run as UID 0. It uses the application's existing Data Protection
ring and the `StonehengeCookie` cookie middleware purpose, and writes JSON
directly to the requested mode-0600 file. It never writes the cookie to stdout.

```bash
install -d -m 700 /tmp/stonehenge-training
STONEHENGE_TRAINING_VULNS_ENABLED=true docker compose run --rm --no-deps \
  -v /tmp/stonehenge-training:/out engineering-app \
  training-session mint --output /out/count-cookie.json

STONEHENGE_TRAINING_VULNS_ENABLED=true docker compose run --rm --no-deps \
  -v /tmp/stonehenge-training:/out engineering-app \
  training-session reset --output /out/count-cookie.json
```

`reset` revokes every active `count` session before minting the replacement.
Disabling the root immediately makes all training-marked cookies invalid.
The mode-0600 JSON integration contract is exactly `schemaVersion=1`,
`subjectAlias="count"`, `accessLevel="engineering.user"`, `expiresAtUtc`,
`cookieName="__Host-Stonehenge.Auth.v3"`, secret `cookieValue`, secret
correlation `sessionId`, `correlationId="NIGHTWATCH-ENG-1847"`, and
`trainingMarker=true`. The issuer audit uses that same fixed correlation ID;
neither secret field is written to stdout or logs.
Verify that fail-closed transition without printing the cookie:

```bash
COUNT_COOKIE=$(jq -r .cookieValue /tmp/stonehenge-training/count-cookie.json)
curl --silent --output /dev/null --write-out '%{http_code}\n' \
  --cookie "__Host-Stonehenge.Auth.v3=${COUNT_COOKIE}" https://engineering.fcu.gov/api/commands/1002
STONEHENGE_TRAINING_VULNS_ENABLED=false docker compose up -d --force-recreate engineering-app
curl --silent --output /dev/null --write-out '%{http_code}\n' \
  --cookie "__Host-Stonehenge.Auth.v3=${COUNT_COOKIE}" https://engineering.fcu.gov/api/commands/1002
unset COUNT_COOKIE
```

The first status is `200` in training; the second is `401` after safe-mode
recreation because cookie validation rejects the training session marker.

## Dex recovery (FCU-AUTH-001)

`POST /training/recovery/dex` accepts `{ "idToken": "..." }` only in training.
It validates current Dex metadata/JWKS, exact issuer, RS256, lifetime, a
nonblank opaque `sub`, an audience set containing `stonehenge-engineering`
and optionally `stonehenge-vendor` with no other audience, exact `azp`
`stonehenge-vendor`, and exact signed
LDAP `preferred_username` `swordsman`, required employee
and EngineeringUser groups, and all privileged exclusions. It signs in with the
normal Engineering cookie and keeps `SaveTokens=false`. Token values are never
logged. Replay protection uses `jti` when present or a full-token hash otherwise.
Metadata/JWKS network failures and timeouts fail closed as
`metadata_unavailable`.

## Classified read mappings

- FCU-INFO-001: exact firing-cycle and simulation timing.
- FCU-INFO-002: safety permissive, freshness, inhibit, interlock, and fail-safe state.
- FCU-INFO-003: real-time readiness, fault, mode, and availability state.
- FCU-INFO-004: target assignment, gun selection, and gun/panel association metadata.
- FCU-INFO-005: maintenance state, shot sequence, and shot/service counters.
- FCU-INFO-006: operational history, exact event time/result, and historical engagement inference.

Command detail and its bounded simulated replay are FCU-ENG-002. Panel 99
discovery/capability issuance is FCU-ENG-003. Panel 99 diagnostics is
FCU-ENG-004. A route emits one audit per applicable finding, with only the
exact returned field names relevant to that finding.

Audit logs record endpoint, caller, field names, classification, finding ID,
and correlation only. They never record credential values.

PLC timing remains a distinct 20 ms scan / 5 s settle / 40 ms pulse / 10 s
lockout-reload model with 1.0-degree azimuth and 0.5-degree elevation steps per
scan (50/25 degrees per second) and a 0.5-degree snap tolerance. Simulation remains a
250 ms tick / 18 degrees-per-second slew clamped to 1-12 seconds / 5 s settle /
10 s lockout model. The Vendor training DTO reads
`STONEHENGE_MAINTENANCE_SHOT_THRESHOLD` (default 200). Panel diagnostics uses
the canonical constant 200 and never enumerates or copies arbitrary
configuration; only the allowlisted environment and auth mode come from
runtime configuration/options.

## Validation and deployment

Run safe and training separately, recreating the two app containers between
profiles:

```bash
STONEHENGE_TRAINING_VULNS_ENABLED=false docker compose config --quiet
STONEHENGE_TRAINING_VULNS_ENABLED=false docker compose up -d --build stonehenge-api engineering-app caddy
TRAINING_PROFILE=safe docker compose --profile tools run --rm playwright \
  npx playwright test tests/training-profiles.spec.js --project=desktop-1080p

STONEHENGE_TRAINING_VULNS_ENABLED=true docker compose config --quiet
STONEHENGE_TRAINING_VULNS_ENABLED=true docker compose up -d --build stonehenge-api engineering-app caddy
TRAINING_PROFILE=training docker compose --profile tools run --rm playwright \
  npx playwright test tests/training-profiles.spec.js tests/engineering-object-idor.spec.js \
  tests/api-boundary-training.spec.js --project=desktop-1080p

# Recovery cryptographic/JWKS matrix (separate OIDC-mode recreation):
STONEHENGE_AUTH_MODE=Oidc STONEHENGE_TRAINING_VULNS_ENABLED=true \
STONEHENGE_OIDC_AUTHORITY=http://playwright:9191/dex \
STONEHENGE_OIDC_CLIENT_ID=stonehenge-engineering STONEHENGE_OIDC_CLIENT_SECRET=test-only \
STONEHENGE_OIDC_REQUIRE_HTTPS_METADATA=false \
docker compose up -d --build --force-recreate engineering-app
docker compose --profile tools run --rm --use-aliases playwright \
  npx playwright test tests/recovery-intake.spec.js --project=desktop-1080p --workers=1

docker compose ps
curl --fail http://127.0.0.1:8088/healthz
curl --fail http://127.0.0.1:8092/api/v1/system/health
```

During a quiesced training window, explicitly prove the two new reads do not
change middleware/PLC/Safety state:

```bash
snapshot() {
  curl -fsS http://127.0.0.1:8080/api/v1/status | \
    jq -S '{guns:[.guns[]|{id,mode,state,shot_sequence,maintenance_enabled,safety_permissive}]}'
  curl -fsS http://127.0.0.1:8080/api/v1/safety | \
    jq -S '{enabled,enforcement_enabled,global_fire_permissive,global_inhibit_active,safety_fault_active,guns}'
}
snapshot > /tmp/stonehenge-state-before.json
curl -fsS -X POST -H 'X-Dev-User: engineering.user' \
  http://127.0.0.1:8088/api/commands/1002/replay >/dev/null
PANEL_TOKEN=$(curl -fsS -X POST -H 'X-Dev-User: engineering.user' \
  http://127.0.0.1:8088/api/panels/99/embed-token | jq -r .token)
curl -fsS -H 'X-Dev-User: engineering.user' \
  -H "X-Stonehenge-Panel-Token: ${PANEL_TOKEN}" \
  http://127.0.0.1:8088/api/panels/99/admin-diagnostics >/dev/null
unset PANEL_TOKEN
snapshot > /tmp/stonehenge-state-after.json
diff -u /tmp/stonehenge-state-before.json /tmp/stonehenge-state-after.json
```

The diff must be empty. Do not run this assertion while the active dispatcher
is legitimately changing gun state or shot sequence.

Before deployment, revalidate the branch and Compose source with
`git branch --show-current`, `git status --short`, and `docker compose config`.
Deployment is intentionally not performed by implementation work.

Rollback is configuration-only: set `STONEHENGE_TRAINING_VULNS_ENABLED=false`,
recreate `stonehenge-api` and `engineering-app`, and verify old training cookies
are rejected. If code rollback is also required, revert the implementation
commit normally; do not delete the shared Data Protection volume. A reset
artifact is locally recoverable only if the caller retained its prior file;
revoked sessions cannot be reactivated.
