#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
    echo "reverse proxy installation must run as root" >&2
    exit 1
fi
if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 /path/to/fcu-root-ca.crt [acme-directory-url]" >&2
    exit 2
fi

ROOT_CA_FILE=$1
ACME_DIRECTORY=${2:-https://acme.fcu.gov:8443/acme/acme/directory}
SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SITE=/etc/nginx/sites-available/stonehenge-vendor
SITE_LINK=/etc/nginx/sites-enabled/stonehenge-vendor

test -r "$ROOT_CA_FILE" || {
    echo "FCU root CA is not readable: $ROOT_CA_FILE" >&2
    exit 1
}
openssl x509 -in "$ROOT_CA_FILE" -noout >/dev/null
openssl x509 -in "$ROOT_CA_FILE" -noout -text | grep -q 'CA:TRUE' || {
    echo "supplied certificate is not a CA certificate" >&2
    exit 1
}
SUBJECT=$(openssl x509 -in "$ROOT_CA_FILE" -noout -subject -nameopt RFC2253 | sed 's/^subject=//')
ISSUER=$(openssl x509 -in "$ROOT_CA_FILE" -noout -issuer -nameopt RFC2253 | sed 's/^issuer=//')
[ "$SUBJECT" = "$ISSUER" ] || {
    echo "supplied certificate is not a self-issued root CA" >&2
    exit 1
}

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y nginx certbot ca-certificates ca-certificates-java

install -o root -g root -m 0644 "$ROOT_CA_FILE" \
    /usr/local/share/ca-certificates/fcu-lab-root-ca.crt
update-ca-certificates
curl --fail --silent --show-error \
    https://login.fcu.gov/dex/.well-known/openid-configuration >/dev/null

install -d -o root -g root -m 0755 /var/www/stonehenge-acme
install -o root -g root -m 0644 "$SOURCE_DIR/nginx-bootstrap.conf" "$SITE"
ln -sfn "$SITE" "$SITE_LINK"
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl enable --now nginx
/usr/sbin/nginx -s reload

REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt certbot certonly \
    --non-interactive \
    --agree-tos \
    --register-unsafely-without-email \
    --webroot \
    --webroot-path /var/www/stonehenge-acme \
    --server "$ACME_DIRECTORY" \
    --cert-name vendor.fcu.gov \
    --domain vendor.fcu.gov

install -d -o root -g root -m 0755 /etc/letsencrypt/renewal-hooks/deploy
install -o root -g root -m 0755 "$SOURCE_DIR/reload-nginx.sh" \
    /etc/letsencrypt/renewal-hooks/deploy/stonehenge-vendor-reload-nginx
install -o root -g root -m 0644 "$SOURCE_DIR/nginx.conf" "$SITE"
nginx -t
/usr/sbin/nginx -s reload

echo "Stonehenge Vendor reverse proxy installed at https://vendor.fcu.gov"
