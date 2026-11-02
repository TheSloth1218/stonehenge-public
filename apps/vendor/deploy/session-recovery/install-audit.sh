#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "audit installation must run as root" >&2; exit 1; }
command -v augenrules >/dev/null 2>&1 || { echo "augenrules is required" >&2; exit 1; }
command -v ausearch >/dev/null 2>&1 || { echo "ausearch is required" >&2; exit 1; }
if command -v systemctl >/dev/null 2>&1; then
  systemctl is-active --quiet auditd || { echo "auditd must be active" >&2; exit 1; }
fi
RULE=/etc/audit/rules.d/stonehenge-session-recovery.rules
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT HUP INT TERM
printf '%s\n' \
  '-w /etc/stonehenge-vendor/session-recovery.key -p r -k vendor_session_recovery_key' \
  '-w /var/backups/stonehenge-vendor/oidc/session-snapshot.db.enc -p r -k vendor_session_snapshot' \
  '-w /usr/local/sbin/stonehenge-session-recovery -p x -k vendor_session_recovery_use' >"$TMP"
install -o root -g root -m 0600 "$TMP" "$RULE"
augenrules --load
echo "secret-free audit watches installed; query with ausearch -k vendor_session_recovery_use"
