#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "rollback must run as root" >&2; exit 1; }
rm -f /usr/local/sbin/stonehenge-session-recovery /usr/local/lib/stonehenge-session-recovery.jar \
  /etc/audit/rules.d/stonehenge-session-recovery.rules
command -v augenrules >/dev/null 2>&1 && augenrules --load
"$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/reset.sh"
