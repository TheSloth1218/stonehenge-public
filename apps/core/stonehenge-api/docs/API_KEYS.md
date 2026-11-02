# Stonehenge API Keys

`stonehenge-api` uses scoped API keys for local app-to-app development. The checked-in defaults keep legacy development keys temporarily available, but normal local runs should use generated keys.

## Generate Local Keys

From the API directory:

```bash
cd /opt/Stonehenge/stonehenge-api
./scripts/generate-api-keys.sh
```

The script writes:

- `.generated/stonehenge-api-keys.env`
  - Plaintext local development keys.
  - Gitignored.
  - Used by local scripts and `docker compose` for the engineering app.
- `.generated/api-key-seed.generated.json`
  - Hashed key seed loaded by the API in Development.
  - Gitignored.
  - Copied into the API image when present during build.

Plaintext keys are intentionally not printed by the generator.

## Key Clients

The generated set includes:

- `STONEHENGE_ENGINEERING_API_KEY`
- `STONEHENGE_VENDOR_API_KEY`
- `STONEHENGE_ASTRO_API_KEY`
- `STONEHENGE_VALIDATION_HARNESS_API_KEY`

Astro endpoints are currently public/read-only in the dev simulation, but an Astro key is still generated so public-client scope-boundary behavior can be tested without changing the local key flow. Treat the Astro key as a low-privilege public client key that may eventually be visible in browser traffic, not as a strong secret.

The Astro key has only:

- `astro.history.read`
- `astro.results.read`
- `astro.summary.read`
- `system.publicStatus.read`

It must not grant vendor or engineering workflow scopes.

## Hashing

Generated keys use this shape:

```text
stn_dev_<keyId>_<secret>
```

The API stores only an HMAC-SHA256 hash of the full key. The development pepper defaults to:

```text
development-only-change-me
```

Override it with `STONEHENGE_API_KEY_PEPPER` only if you regenerate keys with the same pepper before rebuilding/restarting the API.

## Legacy Fallback

Legacy static development keys remain accepted while:

```json
"ApiKeyAuth": {
  "AllowLegacyDevKeys": true
}
```

The current legacy aliases are:

- `vendor-dev-key`
- `astro-dev-key`
- `engineering-dev-key`

This fallback is temporary compatibility scaffolding. Do not add new static shared keys.

## Verbose Scope Errors

`TrainingVulnerabilities:EnableVerboseScopeErrors=true` returns bounded route/scope metadata for authenticated clients that use a valid key against the wrong route group. For example, an Astro key calling a vendor endpoint may receive the caller client name, route group, required scope, accepted client type, and a generic restricted-route hint.

When the toggle is `false`, failed scope checks return only:

```json
{
  "error": "api_key_scope_required"
}
```

Verbose errors must not include valid keys, hashes, service secrets, internal service headers, FUXA/OpenPLC/PLC/control details, middleware command routes, or fire/reset/override paths.

## Safety Boundary

Do not expose generated keys through browser views, JavaScript, Swagger examples, logs, reports, or screenshots. Validation reports redact key-like values and should continue to treat API keys as local secrets.
