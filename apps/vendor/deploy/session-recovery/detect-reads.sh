#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "audit inspection must run as root" >&2; exit 1; }
STATUS=0
if command -v ausearch >/dev/null 2>&1; then
  ausearch -k vendor_session_recovery_key -k vendor_session_snapshot -k vendor_session_recovery_use --start recent || STATUS=$?
else
  echo "ausearch unavailable" >&2
  STATUS=1
fi
if command -v journalctl >/dev/null 2>&1; then
  journalctl --since "-1 hour" SYSLOG_IDENTIFIER=stonehenge-session-recovery \
    --output=short-iso --no-pager || true
fi
if [ "$STATUS" -eq 0 ]; then
  echo "audit output contains kernel file events and sanitized tool status only"
fi
exit "$STATUS"
