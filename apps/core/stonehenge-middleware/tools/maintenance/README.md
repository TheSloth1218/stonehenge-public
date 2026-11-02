# Maintenance Reset Tooling

`stonehenge-maintenance` is the host-side orchestration entrypoint for reset workflows. It is designed for maintenance windows, not normal target-feed operation.

## Actions

- `status`: show Compose service state.
- `soft-reset`: restart one selected service without reuploading PLC code or resetting PostgreSQL.
- `startup`: bring the stack up, back up PostgreSQL, recreate the database, start services, upload PLC artifacts, and put guns back into AUTO.
- `hard-reset`: back up PostgreSQL, stop the stack, restart from a clean database, upload PLC artifacts, and put guns back into AUTO.
- `complete-reset`: alias for the hard-reset workflow; use when you want a named full reset button in the maintenance dashboard.

All actions are dry-run by default. Live `startup`, `hard-reset`, and `complete-reset` require:

```bash
--execute --confirm STONEHENGE_MAINTENANCE_RESET
```

Middleware control writes use `STONEHENGE_CONTROL_TOKEN` from the process
environment or, when it is unset, from the deployment `.env` in this repository.

## Examples

Dry-run a gun-only restart:

```bash
./tools/maintenance/stonehenge-maintenance soft-reset --target gun3
```

Live restart Gun 3 container:

```bash
./tools/maintenance/stonehenge-maintenance soft-reset --target gun3 --execute
```

Dry-run complete startup:

```bash
./tools/maintenance/stonehenge-maintenance startup --confirm STONEHENGE_MAINTENANCE_RESET
```

Live hard reset:

```bash
OPENPLC_CREDENTIAL_FILE=~/.stonehenge-openplc.json \
./tools/maintenance/stonehenge-maintenance hard-reset \
  --execute \
  --confirm STONEHENGE_MAINTENANCE_RESET
```

If the OpenPLC credential file is lost, live startup and hard reset recover automatically after the normal confirmation gate:

```bash
./tools/maintenance/stonehenge-maintenance hard-reset \
  --execute \
  --confirm STONEHENGE_MAINTENANCE_RESET
```

The maintenance tool first checks `OPENPLC_CREDENTIAL_FILE`, `--credentials`, `OPENPLC_PASSWORD`, and the existing generated credential file. If none are available, credential recovery stops OpenPLC runtime containers, removes their runtime volumes, starts fresh runtimes, creates the first `admin` user with a generated password, writes `runtime-secrets/openplc-maintenance.json`, and then uploads the Gun/Safety artifacts. It does not print the generated password. If your Compose project name is not the repository directory name, set `OPENPLC_VOLUME_PREFIX` or pass `--openplc-volume-prefix`.

## Dashboard API

Middleware exposes `/maintenance` and `/api/v1/maintenance` only as a protected maintenance surface. The API is disabled unless:

```bash
STONEHENGE_MAINTENANCE_CONTROLS=true
STONEHENGE_MAINTENANCE_TOKEN=<strong-token>
```

Requests must include:

```text
X-Stonehenge-Maintenance-Token: <strong-token>
```

Generate a local token with:

```bash
openssl rand -base64 32
```

Then place it in `.env` as `STONEHENGE_MAINTENANCE_TOKEN=...` and enter the same value in the `/maintenance` dashboard token field.

The API only invokes the allowlisted `stonehenge-maintenance` actions. It rejects live destructive startup/hard/complete resets without the confirmation string.

## Notes

- PostgreSQL backups are written under `backups/`.
- OpenPLC upload credentials must come from a protected credential file or environment, never from Git. Generated reset credentials are written under `runtime-secrets/`, which is ignored by Git.
- The script uses the monorepo `stonehenge-plc-deployer` package for all PLC uploads. Override `STONEHENGE_PLC_DEPLOYER_ROOT` only when testing an alternate deployer checkout.
- The maintenance API is not intended for public networks or unattended operation.
