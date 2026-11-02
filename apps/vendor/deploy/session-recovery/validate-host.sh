#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "host validation must run as root" >&2; exit 1; }
check() { [ "$(stat -c '%U:%G %a' "$1")" = "root:root 600" ] || { echo "invalid ownership/mode: $1" >&2; exit 1; }; }
check /var/backups/stonehenge-vendor/oidc/session-snapshot.db.enc
check /var/backups/stonehenge-vendor/oidc/session-snapshot.meta
check /etc/stonehenge-vendor/session-recovery.key
[ "$(stat -c '%U:%G %a' /usr/local/sbin/stonehenge-session-recovery)" = "root:root 700" ] || { echo "invalid recovery command ownership/mode" >&2; exit 1; }
check /usr/local/lib/stonehenge-session-recovery.jar
if id stonevendor >/dev/null 2>&1; then
  runuser -u stonevendor -- test ! -r /etc/stonehenge-vendor/session-recovery.key
  runuser -u stonevendor -- test ! -r /var/backups/stonehenge-vendor/oidc/session-snapshot.db.enc
fi
! realm list 2>/dev/null | grep -q . || { echo "domain membership detected" >&2; exit 1; }
echo "ownership, modes, stonevendor denial, and standalone-host state valid"
