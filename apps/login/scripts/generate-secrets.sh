#!/usr/bin/env sh
set -eu

random_secret() {
  node -e "process.stdout.write(require('node:crypto').randomBytes(32).toString('base64url'))"
}

printf 'LOGIN_PORTAL_OIDC_CLIENT_SECRET=%s\n' "$(random_secret)"
printf 'ASTRO_OIDC_CLIENT_SECRET=%s\n' "$(random_secret)"
printf 'LOGIN_PORTAL_SESSION_SECRET=%s\n' "$(random_secret)"
printf 'ASTRO_SESSION_SECRET=%s\n' "$(random_secret)"
