# Networking and DNS

The Vendor application does not require a particular subnet or DNS suffix. Its host address, public hostname, Dex issuer, and Stonehenge API endpoint are deployment configuration. The names below use `fcu.gov` as examples and must be replaced when a different DNS zone is selected.

## Example topology

| Purpose | Example |
| --- | --- |
| Vendor VM address | `192.168.30.4/24` |
| Vendor browser URL | `https://vendor.fcu.gov` |
| Dex issuer | `https://login.fcu.gov/dex` |
| Stonehenge API | `https://api.fcu.gov/api/v1` |
| Dex callback | `https://vendor.fcu.gov/login/oauth2/code/dex` |

Create an `A` or `AAAA` record for the Vendor VM, such as `vendor.fcu.gov -> 192.168.30.25`. Prefer stable DNS names over placing VM addresses in Dex, reverse-proxy, or application configuration. Moving the VM later should then require only a DNS update and any associated firewall or routing changes.

`*.fcu.gov` describes the naming pattern or a possible certificate subject. Dex must receive the exact callback URI; it must not be configured with a wildcard redirect URI.

## Application environment

For TLS terminated by a trusted reverse proxy on the same host:

```ini
SERVER_ADDRESS=127.0.0.1
SERVER_PORT=8089
SERVER_FORWARD_HEADERS_STRATEGY=framework
SESSION_COOKIE_SECURE=true
OIDC_ISSUER=https://login.fcu.gov/dex
OIDC_CLIENT_ID=stonehenge-vendor
OIDC_CLIENT_SECRET=<unique-vendor-client-secret>
OIDC_SCOPES=openid,profile,email,groups
APP_PUBLIC_ORIGIN=https://vendor.fcu.gov
AUTH_SESSION_SECRET=<independent-random-secret>
STONEHENGE_API_BASE_URL=https://api.fcu.gov/api/v1
```

The proxy must replace, rather than append untrusted client values for, `Forwarded` or `X-Forwarded-Proto`, `X-Forwarded-Host`, and `X-Forwarded-Port`. It should forward only to `127.0.0.1:8089`. The browser-facing listener should use the certificate for the selected Vendor hostname.

The supplied nginx site is intentionally a transparent application proxy. It preserves request methods, cookies, query strings, headers, and request bodies and sends every application path through the single `location /` block. Do not add nginx `auth_request`, route denylists, method restrictions, body rewriting, or special blocks for `/admin`, `/actuator`, shipment-job replay/export, or synchronization routes. Spring Security remains responsible for authentication and application RBAC. This is also required for the isolated `training` profile's bounded RCE/LPE exercise to remain reachable after TLS termination.

Install the FCU root CA and nginx proxy with:

```bash
sudo deploy/reverse-proxy/install.sh \
  /path/to/fcu-lab-root-ca.crt \
  https://acme.fcu.gov:8443/acme/acme/directory
```

The installer rejects non-CA and non-self-issued certificates, adds the root to the Ubuntu and Java trust stores, verifies Dex discovery, obtains `vendor.fcu.gov` through ACME, and then enables TLS proxying. It does not filter application paths, including training-only routes.

When intentionally exposing Spring Boot directly instead, set `SERVER_ADDRESS=0.0.0.0` and include the actual scheme and port in the Dex callback. Direct HTTP requires `SESSION_COOKIE_SECURE=false` and is appropriate only for an isolated lab.

## Move checklist

1. Assign the VM its selected `192.168.30.x/24` address, gateway, and DNS resolvers.
2. Update the Vendor DNS record and allow time for its TTL to expire.
3. Confirm the VM can resolve and reach the configured Dex issuer and Stonehenge API.
4. Permit required egress from `192.168.30.0/24` and ingress to the reverse proxy; PostgreSQL can remain localhost-only.
5. Confirm the Dex static client contains the exact externally visible Vendor callback.
6. Restart the Vendor service after changing `/etc/stonehenge-vendor/stonehenge-vendor.env`.
7. Verify login, `/actuator/health` through the intended administrative path, request polling, and completion submission.

The database, Flyway history, shipment data, support collector paths, service account, and Vendor API key do not change merely because the VM address changes. Existing browser sessions tied to the old raw-IP origin will not carry to the hostname; users will authenticate again through Dex.
