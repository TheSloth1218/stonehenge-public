#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "snapshot import must run as root" >&2; exit 1; }
BACKUP_DIR=/var/backups/stonehenge-vendor/oidc
umask 077
exec /usr/local/sbin/stonehenge-session-recovery import \
  "$BACKUP_DIR/session-snapshot.db.enc" "$BACKUP_DIR/session-snapshot.meta" \
  /etc/stonehenge-vendor/session-recovery.key
