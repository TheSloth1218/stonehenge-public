#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "OIDC handoff installation must run as root" >&2
    exit 1
fi
if [ "$#" -ne 1 ]; then
    echo "usage: $0 /path/to/vendor-oidc.env" >&2
    exit 2
fi

SOURCE=$1
TARGET=/etc/stonehenge-vendor/stonehenge-vendor.env
test -r "$SOURCE" || {
    echo "OIDC handoff file is not readable: $SOURCE" >&2
    exit 1
}
test -r "$TARGET" || {
    echo "Vendor runtime environment is not readable: $TARGET" >&2
    exit 1
}

read_value() {
    awk -F= -v key="$1" '$1 == key {sub(/^[^=]*=/, ""); print; exit}' "$SOURCE"
}

ISSUER=$(read_value OIDC_ISSUER)
CLIENT_ID=$(read_value OIDC_CLIENT_ID)
CLIENT_SECRET=$(read_value OIDC_CLIENT_SECRET)
SCOPES=$(read_value OIDC_SCOPES)
REDIRECT_URI=$(read_value OIDC_REDIRECT_URI)

[ "$ISSUER" = "https://login.fcu.gov/dex" ] || {
    echo "unexpected OIDC_ISSUER" >&2
    exit 1
}
[ "$CLIENT_ID" = "stonehenge-vendor" ] || {
    echo "unexpected OIDC_CLIENT_ID" >&2
    exit 1
}
[ -n "$CLIENT_SECRET" ] || {
    echo "OIDC_CLIENT_SECRET is missing" >&2
    exit 1
}
[ "$SCOPES" = "openid,profile,email,groups" ] || {
    echo "unexpected OIDC_SCOPES" >&2
    exit 1
}
[ "$REDIRECT_URI" = "https://vendor.fcu.gov/login/oauth2/code/dex" ] || {
    echo "unexpected OIDC_REDIRECT_URI" >&2
    exit 1
}

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT HUP INT TERM
awk -F= '
    $1 != "SPRING_PROFILES_ACTIVE" &&
    $1 != "OIDC_ISSUER" &&
    $1 != "OIDC_CLIENT_ID" &&
    $1 != "OIDC_CLIENT_SECRET" &&
    $1 != "OIDC_SCOPES" &&
    $1 != "APP_PUBLIC_ORIGIN" &&
    $1 != "SESSION_COOKIE_SECURE" &&
    $1 != "SERVER_FORWARD_HEADERS_STRATEGY"
' "$TARGET" >"$TMP"
cat >>"$TMP" <<EOF
SPRING_PROFILES_ACTIVE=training
OIDC_ISSUER=$ISSUER
OIDC_CLIENT_ID=$CLIENT_ID
OIDC_CLIENT_SECRET=$CLIENT_SECRET
OIDC_SCOPES=$SCOPES
APP_PUBLIC_ORIGIN=https://vendor.fcu.gov
SESSION_COOKIE_SECURE=true
SERVER_FORWARD_HEADERS_STRATEGY=framework
EOF

install -o root -g root -m 0600 "$TMP" "$TARGET"
echo "Vendor OIDC handoff installed; restart sca-parts-fulfillment.service"
