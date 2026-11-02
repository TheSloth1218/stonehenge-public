# Stonehenge OpenPLC Runtime v4 Deployment

This directory keeps the OpenPLC Runtime v4 deployment tooling, runtime inventory, known-good package artifacts, and deployment docs under Git. PLC source projects live in sibling monorepo directories; generated Runtime v4 source should be packaged from those source project build outputs, not copied into this deployer folder.

## Runtime v4 API Contract

Observed on Gun 8 container `stonehenge-middleware-openplc-control-gun8-1`, Runtime `v4.1.7`.

- Version: unauthenticated `GET /api/version` returns `{"version":"v4.1.7"}`.
- Login: `POST /api/login` with JSON `{"username":"...","password":"..."}`.
- Token response field: `access_token`.
- Authenticated requests use `Authorization: Bearer <token>`.
- Upload: `POST /api/upload-file?clean=1` multipart form-data.
- Multipart field name: `file`.
- Compilation status: `GET /api/compilation-status`.
- Compilation status values: `IDLE`, `UNZIPPING`, `COMPILING`, `SUCCESS`, `FAILED`.
- Runtime status: `GET /api/status`.
- Runtime controls: `GET /api/start-plc`, `GET /api/stop-plc`.
- Runtime logs: `GET /api/runtime-logs`.

The upload handler accepts ZIP files after static safety checks, extracts them to `core/generated`, applies `conf/` plugin configuration, and starts compilation automatically.

On the Stonehenge VM, successful compilation leaves the Runtime at `STATUS:STOPPED`; the deployment tool explicitly calls `GET /api/start-plc` and waits for `STATUS:RUNNING`.

## ZIP Layout

The ZIP root is the contents of the OpenPLC Editor-generated `build/OpenPLC Runtime v4/src` directory. Do not wrap files in an extra `src/` directory.

Required current v4 files include:

- `conf/`
- `strucpp_runtime/`
- `c_blocks.h`
- `configuration.cpp`
- `debug-map.json`
- `defines.h`
- `generated.hpp`
- `generated_debug.cpp`
- `plc.xml`
- `program.st`
- `program.st.map.json`
- at least one `pou_*.cpp`

The tooling rejects obsolete v3-only expectations such as `Config0.c`, `Res0.c`, and `glueVars.c`.

## Credentials

Do not put Runtime credentials in Git or command-line arguments. Use either environment variables or a protected JSON file:

```bash
install -m 600 /dev/null ~/.stonehenge-openplc.json
```

Then edit it locally:

```json
{"username":"admin","password":"REDACTED"}
```

Use `--credentials ~/.stonehenge-openplc.json`. The tool refuses group/world-readable credential files. `--insecure` is supported for this lab's internal/self-signed TLS only.

## Package Gun

```bash
./tools/package-program \
  --project gun \
  --source ../stonehenge-control-plc/build/'OpenPLC Runtime v4'/src \
  --output ./artifacts/gun \
  --runtime-version v4.1.x

./tools/validate-artifact \
  --artifact ./artifacts/gun/stonehenge-gun-<commit>.zip \
  --project gun
```

## Deploy Gun 8

Gun 8 is the first live upload target. Do not deploy to Gun 1 or Gun 2 while they are dispatcher-active.

Dry run:

```bash
./tools/deploy-runtime \
  --artifact ./artifacts/gun/stonehenge-gun-<commit>.zip \
  --runtime gun8 \
  --project gun \
  --inventory ./inventory/runtimes.local.json \
  --insecure \
  --dry-run
```

Live run:

```bash
./tools/deploy-runtime \
  --artifact ./artifacts/gun/stonehenge-gun-<commit>.zip \
  --runtime gun8 \
  --project gun \
  --inventory ./inventory/runtimes.local.json \
  --credentials ~/.stonehenge-openplc.json \
  --insecure \
  --execute
```

The tool validates the ZIP, checks middleware deployment readiness when the middleware API is reachable, authenticates, uploads, polls compilation, checks `RUNNING`, verifies Modbus TCP connectivity, and writes a local deployment history record.

The middleware API is currently reachable from the host through the Docker bridge at `http://172.28.0.30:8080`; host port `127.0.0.1:8080` is not published in this deployment. Modbus service names are Docker-internal, so `inventory/runtimes.local.json` includes Docker IP fallbacks for host-side verification.

## Fleet Guns

The fleet command is rolling and defaults to dry run. It refuses Gun 1 and Gun 2 unless explicitly overridden.

```bash
./tools/deploy-gun-fleet \
  --artifact ./artifacts/gun/stonehenge-gun-<commit>.zip \
  --guns 3,4,5,6,7,8 \
  --dry-run
```

## Engineering App Startup And Mass Reload

The future engineering app can call the host-side workflow directly instead of reimplementing Runtime API upload logic. Use `startup-reload` for application startup reconciliation and `mass-reload` for an explicit operator-requested PLC reload. Both commands are aliases over the same sequential deployment engine:

- dry-run by default;
- one Runtime at a time;
- validates each artifact before planning;
- checks Gun middleware readiness before Gun reload;
- refuses Gun 1 and Gun 2 unless `--allow-active-pair` is explicit;
- refuses Safety unless `--include-safety` is explicit;
- records normal deployment history for each live Runtime;
- stops on first failure unless `--continue-on-error` is explicit.

Startup reconciliation for Gun 8 and Safety:

```bash
./tools/startup-reload \
  --gun-artifact ./artifacts/gun/stonehenge-gun-2f182b5dc6b.zip \
  --safety-artifact ./artifacts/safety/stonehenge-safety-eeea2091ebcb.zip \
  --runtimes gun8,safety \
  --include-safety \
  --middleware-url http://172.28.0.30:8080 \
  --insecure \
  --dry-run
```

Mass reload for isolated Guns 3-8:

```bash
./tools/mass-reload \
  --gun-artifact ./artifacts/gun/stonehenge-gun-2f182b5dc6b.zip \
  --guns 3,4,5,6,7,8 \
  --middleware-url http://172.28.0.30:8080 \
  --insecure \
  --dry-run
```

Live execution requires credentials and `--execute`:

```bash
./tools/mass-reload \
  --gun-artifact ./artifacts/gun/stonehenge-gun-2f182b5dc6b.zip \
  --guns 3,4,5,6,7,8 \
  --credentials ~/.stonehenge-openplc.json \
  --middleware-url http://172.28.0.30:8080 \
  --insecure \
  --execute
```

The engineering app should treat a nonzero process exit as a failed startup/reload operation and surface the JSON result. Do not pass Runtime passwords as command-line arguments; use the protected credential file or inherited environment.

## Safety

Safety is intentionally separate:

```bash
./tools/package-program \
  --project safety \
  --source ../stonehenge-safety-plc/build/'OpenPLC Runtime v4'/src \
  --output ./artifacts/safety
```

Package Safety from the sibling `../stonehenge-safety-plc` source project after exporting/building Runtime v4 output there.

Safety live deployment must use `--project safety` and a single `safety` runtime. Do not use fleet deployment for Safety.

## Rollback

Rollback is redeployment of a known-good ZIP artifact by hash. Runtime v4 was not observed to expose a native rollback API. Keep known-good artifacts, manifests, SHA-256 files, and deployment history on the host.

## Failure Recovery

If compilation returns `FAILED`, review `GET /api/compilation-status` logs, do not recreate the container, and redeploy the last known-good artifact. The failed-build test should remove a required generated file only in a temporary copy and must not be committed.
