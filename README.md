# Stonehenge public training export

Stonehenge was designed as a Windows Active Directory backed cyber range. This
export also has a **single-host test mode** that starts the applications,
simulated PLC fleet, FUXA, and local Dex identities without Windows or AD.
The test mode is useful for code review and application exercises; the full
directory, RDP, Kerberos, SMB, and Vendor host privilege chain requires a
Windows DC and, for host-level findings, separate VMs.

The repository contains snapshots of Core/Engineering, Astro, Login/Dex, and
Vendor in `apps/`, one root [Compose stack](compose.yml), a local account and
secret bootstrap script, a Windows DC setup script, and a generic Ansible
deployment playbook. The original app documentation and vulnerable code remain
in their app directories. Source revisions and font credits are in
[docs/ASSETS.md](docs/ASSETS.md).

## One-host test mode

Use a Linux host with Docker Engine and Docker Compose v2, Python 3, and at
least 16 GB RAM. From this repository root:

```bash
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
.venv/bin/python scripts/bootstrap.py
docker compose -f compose.yml config --quiet
docker compose -f compose.yml up -d --build
./scripts/setup-plc.sh
docker compose -f compose.yml ps
```

Bootstrap runs once. It writes `.env` and `secrets/accounts.txt` with mode
`0600`, plus a Dex config template readable by the Dex container; rerunning
it preserves the accounts. The
passwords are deliberately generated per installation and are never committed.
The local Dex accounts can sign in to Login and Astro. Engineering uses its
development identity shortcuts, and Vendor uses its `dev,training` profile.
Those two apps do not use AD group claims in this mode.

Browse from the Docker host:

| App | URL |
| --- | --- |
| Login and Dex | `http://login.stonehenge.test/` |
| Astro | `http://sca.stonehenge.test/` |
| Engineering | `http://engineering.stonehenge.test/` |
| Stonehenge API | `http://api.stonehenge.test/` |
| Vendor | `http://vendor.stonehenge.test/` |

Add this line to `/etc/hosts` on the Docker host (and on any client using
an SSH port forward):

```text
127.0.0.1 login.stonehenge.test sca.stonehenge.test engineering.stonehenge.test api.stonehenge.test vendor.stonehenge.test
```

The Compose edge binds to `127.0.0.1:80` by default. Ansible users can
clone this repository on the target host, then run
`ansible-playbook -i ansible/inventory.example.ini ansible/deploy.yml` after
editing that inventory. The playbook runs the same bootstrap and Compose path.

The training API accepts its documented legacy development client keys in
test mode; these are challenge material, not operator secrets. Root `.env`
contains random runtime values. The simulation uses nine OpenPLC runtimes.
Load the included gun and Safety programs after first startup using the steps
in [docs/PLC.md](docs/PLC.md).

## Optional Windows AD mode

On a fresh Windows Server 2022 or 2025 VM, copy
`scripts/windows/` and run elevated PowerShell:

```powershell
.\Setup-StonehengeDC.ps1 -Phase Promote
# The first run may rename the server to FCU-DC and reboot. If so, run Promote again.
# Forest promotion reboots. Sign back in as domain Administrator.
.\Setup-StonehengeDC.ps1 -Phase Configure -AppHostAddress 192.0.2.10
```

Replace `192.0.2.10` with the Docker host's address. The script creates the
`fcu.gov` forest, Stonehenge users/groups, application DNS records, an LDAPS
certificate, and the intended directory challenge conditions. It writes
generated credentials to `C:\ProgramData\Stonehenge\dc-secrets.json` with
Administrators/SYSTEM access. Record the DSRM password separately. Copy
`C:\ProgramData\Stonehenge\ldaps-ca.pem` to the Docker host as
`secrets/ad-ca.crt`, then reboot the DC once more to activate LDAPS.

On the Docker host, update `.env` with these values. Use the DC's IPv4
address for `AD_DC_IP` and the generated `dexBind` password from
`dc-secrets.json` for `DEX_LDAP_BIND_PASSWORD`:

```dotenv
STONEHENGE_EDGE_HOST_IP=0.0.0.0
LOGIN_HOSTNAME=login.fcu.gov
ASTRO_HOSTNAME=sca.fcu.gov
ENGINEERING_HOSTNAME=engineering.fcu.gov
STONEHENGE_API_HOSTNAME=api.fcu.gov
VENDOR_HOSTNAME=vendor.fcu.gov
DEX_ISSUER=http://login.fcu.gov/dex
LOGIN_PORTAL_PUBLIC_ORIGIN=http://login.fcu.gov
ASTRO_PUBLIC_ORIGIN=http://sca.fcu.gov
ENGINEERING_PUBLIC_ORIGIN=http://engineering.fcu.gov
VENDOR_PUBLIC_ORIGIN=http://vendor.fcu.gov
STONEHENGE_OIDC_AUTHORITY=http://login.fcu.gov/dex
STONEHENGE_AUTH_MODE=Oidc
VENDOR_PROFILE=training
DEX_CONFIG_FILE=config.ldap.training.yaml
DEX_LDAP_HOST=fcu-dc.fcu.gov
DEX_LDAP_BIND_DN=svc_dex@fcu.gov
DEX_LDAP_BIND_PASSWORD=<generated dexBind password>
DEX_LDAP_USER_BASE_DN=DC=fcu,DC=gov
DEX_LDAP_GROUP_BASE_DN=DC=fcu,DC=gov
AD_DC_IP=<DC IPv4 address>
```

The DC script points the five application names at the Docker host. To use a
different origin or TLS, change the DNS names, all matching origins, Dex issuer,
and Caddy listener together. Then launch:

```bash
docker compose -f compose.yml -f compose.ad.yml config --quiet
docker compose -f compose.yml -f compose.ad.yml up -d --build
./scripts/setup-plc.sh
```

The training vulnerabilities are enabled by default. See
[docs/ATTACK-CHAINS.md](docs/ATTACK-CHAINS.md) and
[docs/INSTRUCTOR.md](docs/INSTRUCTOR.md) for the scenario and exercise guide.
The native Vendor host support and encrypted session recovery exercises are
host-level additions; the single-host Vendor container does not recreate a
separate Vendor VM's sudo or root boundary.

## Stop and reset

`docker compose -f compose.yml down` stops the stack without deleting data.
Volumes hold databases, Dex identities, review state, and Caddy data. Deleting
volumes resets the lab and loses that data; save anything needed first. The
Windows DC script skips an already configured lab and never resets student
passwords on rerun. Use a fresh VM snapshot for a full AD challenge reset.

This export has no selected source-code license yet. The owner must choose one
before public publication; the font rights and upstream dependencies are
separate matters documented in `docs/ASSETS.md`.
