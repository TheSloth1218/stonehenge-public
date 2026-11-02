# Load the OpenPLC simulation

The root Compose stack starts nine OpenPLC runtimes. The source projects and
ready-to-upload Runtime v4 ZIPs are included under
`apps/core/stonehenge-plc-deployer/artifacts/`.

After `docker compose up -d --build`, run:

```bash
./scripts/setup-plc.sh
```

The script waits for the nine runtimes, creates one generated admin credential
in `secrets/openplc.json` on the first run, then uploads the gun and Safety
programs. It marks a successful load in `secrets/plc-loaded` so a normal
deployment retry does not interrupt active PLC programs. Remove that marker
when resetting the OpenPLC volumes or intentionally reloading. The deployment tool
validates each ZIP, waits for compilation and RUNNING state, then checks Modbus.

Check the result with:

```bash
docker compose ps
curl -fsS http://127.0.0.1:8080/ready
```

The PLC source, packaging command, inventory, and individual deployment
options are documented in
[the deployer README](../apps/core/stonehenge-plc-deployer/README.md).
