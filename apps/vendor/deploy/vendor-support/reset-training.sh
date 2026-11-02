#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "training reset must run as root" >&2
    exit 1
fi
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
"$SCRIPT_DIR/install.sh" training
rm -f /var/lib/stonehenge-vendor/training/evidence/root-execution.json
echo "trusted training plugins restored and root evidence removed"
