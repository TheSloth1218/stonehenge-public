# Stonehenge PLC Deployer

Host-side tooling for packaging OpenPLC Editor Runtime v4 output and deploying it to the Stonehenge OpenPLC runtime fleet. This directory is intentionally only the deployer surface: command wrappers, Python deployment logic, runtime inventory, tests, docs, and known-good packaged artifacts.

The PLC source projects live next to this folder in the monorepo:

- `../stonehenge-control-plc/` - shared gun-control PLC source used by all gun runtimes.
- `../stonehenge-safety-plc/` - safety PLC source.

Do not add copied PLC editor source, `pous/`, `devices/`, or generated `build/` trees back into this deployer folder. Package from the PLC source project directories instead.

## Layout

- `tools/` - packaging, validation, runtime deploy, fleet deploy, startup reload, and verification commands.
- `inventory/` - runtime inventory for local Docker/OpenPLC targets.
- `artifacts/` - known-good packaged Runtime v4 ZIPs and sidecar manifests.
- `tests/` - unit tests for the deployer logic.
- `docs/deployment.md` - runtime API notes and operational examples.

## Common Commands

Package a gun Runtime v4 export from the shared gun PLC project:

```bash
./tools/package-program \
  --project gun \
  --source ../stonehenge-control-plc/build/'OpenPLC Runtime v4'/src \
  --output ./artifacts/gun \
  --runtime-version v4.1.x
```

Validate a packaged artifact:

```bash
./tools/validate-artifact \
  --artifact ./artifacts/gun/stonehenge-gun-<version>.zip \
  --project gun
```

Deploy a single runtime:

```bash
./tools/deploy-runtime \
  --artifact ./artifacts/gun/stonehenge-gun-<version>.zip \
  --runtime gun8 \
  --project gun \
  --inventory ./inventory/runtimes.local.json \
  --credentials ~/.stonehenge-openplc.json \
  --insecure \
  --execute
```

Reload the fleet from known-good artifacts:

```bash
./tools/startup-reload \
  --gun-artifact ./artifacts/gun/stonehenge-gun-2f182b5dc6b.zip \
  --safety-artifact ./artifacts/safety/stonehenge-safety-eeea2091ebcb.zip \
  --runtimes gun1,gun2,gun3,gun4,gun5,gun6,gun7,gun8,safety \
  --include-safety \
  --allow-active-pair \
  --credentials ~/.stonehenge-openplc.json \
  --insecure \
  --execute
```

## Credentials

Do not commit runtime credentials. Use a protected JSON file:

```bash
install -m 600 /dev/null ~/.stonehenge-openplc.json
```

```json
{"username":"admin","password":"REDACTED"}
```

Pass it with `--credentials ~/.stonehenge-openplc.json`. The deployer refuses group/world-readable credential files.

## Testing

```bash
python3 -m unittest discover -s tests
```
