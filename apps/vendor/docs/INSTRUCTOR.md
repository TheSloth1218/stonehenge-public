# Instructor Guide

This document is for the isolated Stonehenge Vendor training environment. It describes evidence and reset boundaries without including a ready-made exploit payload.

## Intended chain

1. Authenticate as a Dex-backed `EMPLOYEE` or use explicit `dev,training` profiles in a disposable lab.
2. Observe the selected Actuator mappings and configuration metadata exposed to employees.
3. Identify the legacy shipment-job export and replay routes.
4. Export a legitimate compatibility payload from an existing shipment job.
5. Use the legacy serializer clue to submit a bounded stock compatibility payload; the controller queues bytes and does not deserialize them.
6. The background worker claims the row and performs native Java deserialization in `training` only.
7. Verify command execution as the unprivileged `stonevendor` service account.
8. Enumerate the training host support command and the narrowly scoped sudo permission.
9. Identify that the `vendor-support` plugin directory is group-writable and plugins execute through the root collector.
10. Record only `/var/lib/stonehenge-vendor/training/evidence/root-execution.json` as 008 root-level evidence.
11. Only for the separately authorized 009 challenge, hand off to the root-only encrypted recovery workflow after the marker is validated.

The native shipment worker and its intentionally retained Commons Collections 3.2.1 compatibility library are the only intended direct application RCE path. The custom callback remains only as a controlled automated-test sink. Do not add command-bearing fields to catalog, shipment, inventory, template, file, API, or checkout features.

The bundled root-evidence fixture is the only supported privilege-escalation demonstration artifact. It creates no users, keys, services, firewall changes, scheduled tasks, persistence, application modifications, domain operations, or remote sessions. The normal chain stops at Vendor-host root. SCA-VENDOR-009 is an explicitly challenge-only instructor handoff after that stop; it is not a direct Engineering/OT shortcut and must not contain an Engineering Admin session.

## Challenge-only session recovery (SCA-VENDOR-009)

Do not provision this workflow until repository, Vendor-support, safe, and training tests pass. Auditd tools (`augenrules` and `ausearch`) must be installed manually before deployment, and the `auditd` service must be active on systemd hosts; this host does not currently meet that prerequisite. First run `mvn package` so the installer can verify and copy the dedicated recovery JAR from the tested Maven output; it never uses the stale live application JAR. Install it as root with `deploy/session-recovery/install.sh training`. Supply the protected instructor JSON seed only through stdin to `deploy/session-recovery/import-stdin.sh`; never put the seed in shell history, the repository, an environment variable, or command arguments. The import requires exactly four opaque Dex-artifact records: ordinary employee, non-Engineering admin decoy `edge`, target `swordsman`, and expired/revoked. The `edge` record must include `SCA-Employees` and `SCA-Admins`; `SCA-Admins` is rejected on every other record. The target must correlate request `2003`, `NIGHTWATCH`, and a Vendor sync/diagnostic correlation ID. Engineering, domain/enterprise admin, middleware, PLC, and noncanonical general-admin group variants are rejected from the edge boundary.

The encrypted snapshot, sanitized metadata, and AES-256-GCM key are root-owned mode `0600`. Run `validate-host.sh` to prove `stonevendor` cannot read the key or snapshot and the host remains non-domain-joined. `install-audit.sh` adds secret-free audit watches for key/snapshot reads and recovery execution; inspect them with `detect-reads.sh`. Journald receives only the tool's sanitized stdout. Recovery writes the selected opaque artifact to an explicitly named, mode `0600` output and prints only record ID, expiry/status, finding/correlation ID, and a token hash prefix.

Safe mode is `deploy/session-recovery/install.sh safe`: it removes the runtime key, command, and dedicated tool JAR but retains the encrypted archive, proving it cannot be decrypted locally. Vendor reset removes only local snapshot material; it cannot revoke Dex sessions or reseed identity data. The instructor must perform revocation/reseeding separately on the identity host, or exclude live artifacts when producing a safe archive. `reset.sh` removes snapshot, metadata, and key; `rollback.sh` additionally removes the command, tool JAR, and audit rule. Neither operation contacts Dex.

## Host support boundary

Provision a disposable host explicitly with `sudo deploy/vendor-support/install.sh training`. This installs the collector outside the application directory, validates the sudoers file, and adds the training-only systemd override required to permit the sudo boundary. Restart the service after provisioning.

Provisioning with `sudo deploy/vendor-support/install.sh safe` restores a root-only plugin directory, hash allowlist enforcement, a fixed sudo command, and the normal `NoNewPrivileges=true` service setting. The safe collector rejects non-root-owned, writable, symlinked, traversing, alternate-path, and non-allowlisted plugins.

## Reverse proxy boundary

Use `deploy/reverse-proxy/nginx.conf` without adding path or method filters. Nginx terminates TLS, replaces untrusted forwarding headers, and forwards only to the loopback Spring listener. It must otherwise preserve authenticated cookies, POST bodies, and all application routes. Filtering shipment-job export/replay, selected Actuator routes, synchronization routes, or administrative paths at nginx changes the intended training chain and can make the application behavior differ from the documented scenarios.

The reverse proxy does not create the RCE or LPE conditions. Those remain bounded to the Spring `training` profile, native shipment compatibility worker, opt-in support collector provisioning, and training systemd override. Switching the application to `safe` and running `deploy/vendor-support/install.sh safe` removes those intended conditions even when the same nginx site remains installed.

## API weaknesses

Training mode also contains external request ID confusion, employee access to failed completion retry, and bounded sync diagnostic disclosure. These operations remain fixed to the existing completion endpoint and never accept arbitrary URLs, methods, headers, bodies, or credentials.

## Safe verification

Run with `safe`. Employees must be denied all detailed admin routes and non-minimal Actuator endpoints. Shipment jobs must parse strict JSON, reject unknown fields and type selectors, and validate persisted shipment, order, and request relationships. Completion targets must come from the database.

The targeted unit and route-profile tests do not require Docker. The full Maven suite also contains PostgreSQL Testcontainers tests and requires a working Docker socket; a Docker-unavailable failure is an environment prerequisite failure, not targeted-test success.

## Reset

Stop the service, run `scripts/reset-local-db.sh`, and run `sudo deploy/vendor-support/reset-training.sh`. Start PostgreSQL and restart with the intended profile. The support reset restores trusted plugins and removes only the dedicated root evidence marker; preserve normal application logs needed for review.

Review `training/vulnerabilities.yml` for scenario identifiers, expected evidence, and remediations.
