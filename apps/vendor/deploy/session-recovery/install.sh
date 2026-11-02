#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo "session snapshot installation must run as root" >&2; exit 1; }
[ "$#" -eq 1 ] || { echo "usage: $0 training|safe" >&2; exit 2; }
MODE=$1
[ "$MODE" = training ] || [ "$MODE" = safe ] || { echo "mode must be training or safe" >&2; exit 2; }
SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BUILD_JAR=${STONEHENGE_RECOVERY_BUILD_JAR:-"$SOURCE_DIR/../../target/sca-parts-fulfillment-0.1.0-SNAPSHOT.jar"}
TOOL_JAR=/usr/local/lib/stonehenge-session-recovery.jar
BACKUP_DIR=/var/backups/stonehenge-vendor/oidc
KEY=/etc/stonehenge-vendor/session-recovery.key
[ "$MODE" != safe ] || {
  rm -f /usr/local/sbin/stonehenge-session-recovery "$TOOL_JAR" "$KEY"
  echo "safe mode: recovery disabled and runtime key removed; encrypted archive retained"
  exit 0
}
[ -d /etc/stonehenge-vendor ] || { echo "/etc/stonehenge-vendor must already exist" >&2; exit 1; }
install -d -o root -g root -m 0700 "$BACKUP_DIR"
command -v jar >/dev/null 2>&1 || { echo "jar inspection tool is required" >&2; exit 1; }
jar tf "$BUILD_JAR" | grep -qx 'BOOT-INF/classes/gov/sca/fulfillment/recovery/SessionSnapshotTool.class' \
  || { echo "tested Maven build does not contain SessionSnapshotTool" >&2; exit 1; }
install -o root -g root -m 0600 "$BUILD_JAR" "$TOOL_JAR"
install -o root -g root -m 0700 "$SOURCE_DIR/session-recovery" /usr/local/sbin/stonehenge-session-recovery
if [ ! -e "$KEY" ]; then
  /usr/local/sbin/stonehenge-session-recovery keygen \
    "$BACKUP_DIR/session-snapshot.db.enc" "$BACKUP_DIR/session-snapshot.meta" "$KEY"
fi
echo "training recovery installed; import remains pending secure instructor stdin"
