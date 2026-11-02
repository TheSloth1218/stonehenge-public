# Instructor guide

## Start the range

Run the commands in the [README](../README.md). The default Compose stack has
training vulnerabilities enabled. Read the generated local Dex passwords from
`secrets/accounts.txt`. The accounts are `employee@sca.local` and
`operator@sca.local`; Engineering uses development identities, and Vendor uses
the seeded `operator` employee. The five web origins are listed in the README.

For the Windows route, run
[Setup-StonehengeDC.ps1](../scripts/windows/Setup-StonehengeDC.ps1) on a fresh
Windows Server 2022/2025 VM, then select the LDAP Compose overlay as shown in
the README. The DC writes generated passwords to
`C:\ProgramData\Stonehenge\dc-secrets.json`.

## Exercise map

| Area | One-host test mode | Windows AD mode | Separate Vendor VM |
| --- | --- | --- | --- |
| Public API and legacy client keys | Yes | Yes | — |
| Astro reviewer XSS | Yes | Yes | — |
| Engineering object and role gaps | Yes, development identities | Yes, Count through Dex | — |
| Vendor Actuator, replay, deserialization, sync gaps | Yes, seeded operator | Yes, employee through Dex | — |
| Anonymous LDAP, Chopper RDP, legacy computer, delegation | — | Yes | — |
| Vendor support plugin to root, keytab and SMB handoff | — | Requires domain joined Vendor host | Yes |

The detailed scoring key is [ATTACK-CHAINS.md](ATTACK-CHAINS.md). The steps below
reproduce each application path.

## Public API

Start with anonymous system and Astro reads:

```bash
curl -fsS http://127.0.0.1:8092/api/v1/system/info
curl -fsS 'http://127.0.0.1:8092/api/v1/astro/target-history?limit=5'
curl -fsS 'http://127.0.0.1:8092/api/v1/astro/firing-results?fields=all'
```

Then use the checked-in `astro-dev-key`, `vendor-dev-key`, and
`engineering-dev-key` values in the `X-Stonehenge-Api-Key` header. Compare
`/api/v1/vendor/part-requests?fields=all`,
`/api/v1/vendor/guns/status?debug=true`, and
`/api/v1/engineering/vendor-completions/pending` with the wrong client key.
The wrong-key response reveals the required scope.

## Astro reviewer

The Staff application form claims its profile URL is LinkedIn-only, but
accepts the Astro `/ulysses-map` route. Generate the supplied payload URL:

```bash
cd apps/astro
npm ci
ASTRO_PUBLIC_ORIGIN=http://sca.stonehenge.test \
  npx tsx tools/training/generate-astro-xss-payload.ts
```

Submit that URL in the Staff application form. The reviewer visits it, and the
same-origin collector stores six synthetic metadata fields. The submission
status and collector are at `/api/astro/apply/<submission-id>` and
`/api/training/collect/<submission-id>`. The sink and payload generator are in
`apps/astro/components/training/requested-target-fallback.tsx` and
`apps/astro/tools/training/generate-astro-xss-payload.ts`.

## Engineering

In one-host mode, send `X-Dev-User: engineering.user` to Engineering. The
supplied Playwright test names the complete role and data boundaries in
`apps/core/stonehenge-engineering-app/tools/playwright/tests/training-profiles.spec.js`.

```bash
curl -fsS -H 'X-Dev-User: engineering.user' \
  http://127.0.0.1:8088/api/part-requests/2003
curl -fsS -H 'X-Dev-User: engineering.user' \
  http://127.0.0.1:8088/api/commands/1002
curl -fsS -X POST -H 'X-Dev-User: engineering.user' \
  http://127.0.0.1:8088/api/commands/1002/replay
curl -fsS -X POST -H 'X-Dev-User: engineering.user' \
  http://127.0.0.1:8088/api/panels/99/embed-token
```

Use the returned panel token in `X-Stonehenge-Panel-Token` on
`/api/panels/99/admin-diagnostics`. Gun state, logs, manual context, and system
events expose the six operational information categories in the scoring key.
Under AD, sign in as Count instead of supplying the development header.

## Vendor

The default `dev,training` profile signs requests in as the seeded `operator`
employee. Read `/actuator/info` and `/actuator/mappings` to find the
compatibility worker. Make a normal practice shipment to create a job; then
retrieve `/admin/shipment-jobs/<id>/export` and submit its Base64 payload to
`POST /admin/shipment-jobs/replay`. The worker deserializes Java objects using
Commons Collections 3.2.1. Replace the exported bytes with a
CommonsCollections6 payload to demonstrate code execution in the Vendor
process. The controller, worker, and profile switch are in
`apps/vendor/src/main/java/gov/sca/fulfillment/shipping/jobs/` and
`apps/vendor/training/vulnerabilities.yml`.

For the other Vendor gaps, submit a different external request ID during
completion, retry an eligible failed `/admin/sync/<id>/retry`, and read
`/admin/sync/<id>` as an employee. Compare the audit and stock records before
and after. Repeat one valid Vendor API completion to observe the duplicate
API-local event.

## Windows and native Vendor host

The DC script creates:

- anonymous LDAP visibility into staff names and Chopper's temporary password
  in his description; Chopper and Grim have RDP access;
- pre-created `FCU-LEGACY1$` with the predictable machine password
  `fcu-legacy1` and a password-reset right over Grim;
- constrained delegation from that computer to `time/fcu-dc.fcu.gov`, with
  `svc_flightops` as the one delegable Domain Admin;
- the `ENG-1847` clue in Public Documents and the
  `Engineering-Access` SMB share containing Count's generated password.

Enumerate LDAP anonymously, authenticate as Chopper, then inspect the
`FCU-LEGACY1$` ACL. Authenticate with its machine password, reset Grim, and
test Grim's Dex/RDP access. The optional delegation exercise uses
S4U2Self/S4U2Proxy as `svc_flightops` and changes the final ticket service
name from `time` to `ldap` on the same DC.

The separate Vendor-host exercise uses the native installer in
`apps/vendor/deploy/vendor-support/install.sh`. The training profile lets
`stonevendor` write a support plugin and run the root-owned collector through
sudo; `apps/vendor/deploy/vendor-support/fixtures/root-evidence` records the
proof. The one-host Vendor container includes the web vulnerability but does
not configure sudo, a host keytab, or a joined Linux VM.
