# Stonehenge TLS Edge

Caddy terminates TLS for the two intentionally host-facing services:

- `https://engineering.fcu.gov` -> ASP.NET Engineering App
- `https://api.fcu.gov` -> Stonehenge API

Certificates are issued by the FCU Step CA through its ACME provisioner. The
public FCU lab root is mounted read-only into Caddy for ACME server validation.

The proxy deliberately performs no authentication, authorization, path
rewriting, request filtering, or response rewriting. Application auth, API-key
scope decisions, training scenarios, and bounded vulnerability behavior remain
owned by the upstream applications. FUXA, middleware, PostgreSQL, OpenPLC, and
Modbus are not routed by Caddy.

Direct app and API diagnostic ports bind to `127.0.0.1` by default. Caddy is the
only container trusted by the Engineering App to supply forwarded host, scheme,
and client-address headers.
