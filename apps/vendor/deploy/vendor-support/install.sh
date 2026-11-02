#!/bin/sh
set -eu

MODE=${1:-}
case "$MODE" in
    training|safe) ;;
    *) echo "usage: $0 training|safe" >&2; exit 2 ;;
esac
if [ "$(id -u)" -ne 0 ]; then
    echo "vendor support provisioning must run as root" >&2
    exit 1
fi
if ! command -v visudo >/dev/null 2>&1; then
    echo "visudo is required; install the sudo package" >&2
    exit 1
fi

SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PLUGIN_DIR=/opt/stonehenge/vendor/support/plugins
PROFILE_DIR=/opt/stonehenge/vendor/support/profiles
EVIDENCE_DIR=/var/lib/stonehenge-vendor/training/evidence
CONFIG_DIR=/etc/stonehenge/vendor
SUDOERS_FILE=/etc/sudoers.d/stonehenge-vendor-support
OVERRIDE_DIR=/etc/systemd/system/sca-parts-fulfillment.service.d

getent group stonevendor >/dev/null 2>&1 || groupadd --system stonevendor
getent group vendor-support >/dev/null 2>&1 || groupadd --system vendor-support
if ! id stonevendor >/dev/null 2>&1; then
    useradd --system --gid stonevendor --home-dir /var/lib/stonehenge-vendor \
        --shell /usr/sbin/nologin stonevendor
else
    usermod --gid stonevendor stonevendor
fi
usermod --append --groups vendor-support stonevendor

install -d -o root -g root -m 0755 /opt/stonehenge/vendor/support
install -d -o root -g root -m 0755 "$PLUGIN_DIR"
install -d -o root -g root -m 0755 "$PROFILE_DIR"
install -d -o root -g root -m 0755 "$EVIDENCE_DIR"
install -d -o root -g root -m 0755 "$CONFIG_DIR"
install -o root -g root -m 0755 "$SOURCE_DIR/vendor_support.py" /usr/local/sbin/vendor-support

find "$PLUGIN_DIR" -mindepth 1 -maxdepth 1 -delete
find "$PROFILE_DIR" -mindepth 1 -maxdepth 1 -delete
install -o root -g root -m 0755 "$SOURCE_DIR/plugins/process-identity" "$PLUGIN_DIR/process-identity"
install -o root -g root -m 0755 "$SOURCE_DIR/plugins/runtime-summary" "$PLUGIN_DIR/runtime-summary"
install -o root -g root -m 0644 "$SOURCE_DIR/profiles/standard.json" "$PROFILE_DIR/standard.json"

MANIFEST_TMP=$(mktemp)
CONFIG_TMP=$(mktemp)
SUDOERS_TMP=$(mktemp)
trap 'rm -f "$MANIFEST_TMP" "$CONFIG_TMP" "$SUDOERS_TMP"' EXIT HUP INT TERM
PROCESS_HASH=$(sha256sum "$PLUGIN_DIR/process-identity" | awk '{print $1}')
RUNTIME_HASH=$(sha256sum "$PLUGIN_DIR/runtime-summary" | awk '{print $1}')
printf '{\n  "plugins": {\n    "process-identity": "%s",\n    "runtime-summary": "%s"\n  }\n}\n' \
    "$PROCESS_HASH" "$RUNTIME_HASH" >"$MANIFEST_TMP"
install -o root -g root -m 0600 "$MANIFEST_TMP" "$CONFIG_DIR/trusted-plugins.json"

if [ "$MODE" = training ]; then
    SUPPORT_MODE=legacy-plugin
    chown root:vendor-support "$PLUGIN_DIR"
    chmod 0775 "$PLUGIN_DIR"
    printf '%s\n' 'stonevendor ALL=(root) NOPASSWD: /usr/local/sbin/vendor-support collect *' >"$SUDOERS_TMP"
    install -d -o root -g root -m 0755 "$OVERRIDE_DIR"
    install -o root -g root -m 0644 "$SOURCE_DIR/systemd/10-training-lpe.conf" \
        "$OVERRIDE_DIR/10-training-lpe.conf"
else
    SUPPORT_MODE=trusted-plugin
    chown root:root "$PLUGIN_DIR"
    chmod 0755 "$PLUGIN_DIR"
    printf '%s\n' 'stonevendor ALL=(root) NOPASSWD: /usr/local/sbin/vendor-support collect --profile standard' \
        >"$SUDOERS_TMP"
    rm -f "$OVERRIDE_DIR/10-training-lpe.conf"
fi

printf '{\n  "mode": "%s",\n  "profileDirectory": "%s",\n  "pluginDirectory": "%s",\n  "evidenceDirectory": "%s",\n  "allowlistManifest": "%s"\n}\n' \
    "$SUPPORT_MODE" "$PROFILE_DIR" "$PLUGIN_DIR" "$EVIDENCE_DIR" \
    "$CONFIG_DIR/trusted-plugins.json" >"$CONFIG_TMP"
install -o root -g root -m 0600 "$CONFIG_TMP" "$CONFIG_DIR/support.json"

visudo -cf "$SUDOERS_TMP"
install -o root -g root -m 0440 "$SUDOERS_TMP" "$SUDOERS_FILE"
visudo -cf "$SUDOERS_FILE"

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
    systemctl daemon-reload
fi
echo "vendor support host boundary provisioned in $MODE mode"
echo "restart sca-parts-fulfillment.service before validation"
