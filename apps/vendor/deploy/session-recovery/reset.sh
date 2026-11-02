#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "session recovery reset must run as root" >&2; exit 1; }
rm -f /var/backups/stonehenge-vendor/oidc/session-snapshot.db.enc \
  /var/backups/stonehenge-vendor/oidc/session-snapshot.meta \
  /etc/stonehenge-vendor/session-recovery.key
echo "session snapshot, sanitized metadata, and recovery key removed"
