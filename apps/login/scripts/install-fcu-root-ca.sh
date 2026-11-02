#!/usr/bin/env bash
set -euo pipefail

SOURCE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/dex/ca/fcu-lab-pki-root-ca.crt"

if [[ ! -r "$SOURCE" ]]; then
  printf 'FCU root CA not found at %s\n' "$SOURCE" >&2
  exit 1
fi

if command -v update-ca-trust >/dev/null 2>&1; then
  install -d -m 0755 /etc/pki/ca-trust/source/anchors
  install -m 0644 "$SOURCE" /etc/pki/ca-trust/source/anchors/fcu-lab-pki-root-ca.crt
  update-ca-trust extract
elif command -v update-ca-certificates >/dev/null 2>&1; then
  install -d -m 0755 /usr/local/share/ca-certificates
  install -m 0644 "$SOURCE" /usr/local/share/ca-certificates/fcu-lab-pki-root-ca.crt
  update-ca-certificates
else
  printf 'Unsupported system CA trust store\n' >&2
  exit 1
fi

printf 'Installed FCU Lab PKI root CA\n'
