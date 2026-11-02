# Vendor Identity Handoff

This document defines the boundary after `SCA-VENDOR-008`. The repository implements the optional
`SCA-VENDOR-009` recovery tooling, but 009 is a challenge-only identity handoff, not an Engineering
application-session stage.

## Current boundary

The application uses Dex for interactive OIDC authentication over HTTPS. The Vendor API key remains Vendor-scoped and must not be reused as Engineering authorization.

In the isolated training profile, the normal chain stops after the support collector records controlled
root execution on the Vendor host. Root access must not automatically provide Engineering scope, PLC
access, middleware administration, Domain Admin, Dex signing keys, or a real administrator session.
The stopping point remains Vendor-host root unless the instructor explicitly enables `SCA-VENDOR-009`.

## Standalone host requirement

The Vendor Linux host must remain standalone. It must not be joined to an Active Directory domain and must not receive a domain machine account, Kerberos keytab, SSSD/realmd configuration, Winbind configuration, or SMB recovery-share credentials.

Dex may use an upstream enterprise identity provider, but that relationship belongs to Dex. The Vendor application remains an ordinary OIDC client and does not communicate with a domain controller.

When explicitly enabled by the instructor, `SCA-VENDOR-009` permits only the bounded, challenge-only
Vendor-client/Dex artifact for the `swordsman` Engineering operator. It is a Vendor recovery artifact
correlated to the training scenario; it is not an Engineering application session and does not authorize
access to the Engineering application. An Engineering application session, Engineering client secret,
Dex signing key, LDAP secret, password, or Engineering Admin material is forbidden. The handoff uses
standard HTTPS identity endpoints and does not require domain enrollment.

The repository supplies the recovery mechanism but no live artifact is present or seeded in Git or on
this host. A protected instructor seed must be supplied separately. Revocation and reseeding on the
identity host are separate integration steps, as is `FCU-AUTH-001`; Vendor reset and recovery tooling do
not perform them.

## Handoff controls

- Keep the Vendor host outside all identity-provider and domain trust stores.
- Use separate OIDC clients and exact redirect URIs for Vendor and Engineering.
- Keep Dex clients and audiences separate between Vendor and Engineering.
- Request `openid profile email groups` and use Authorization Code Flow with PKCE S256.
- Treat `sub` as the durable identity; display names, email addresses, and usernames are mutable profile data.
- Read authorization only from the signed Dex `groups` claim. Map `SCA-Employees` to the local
  `employee.basic` role and `SCA-Admins` to local `admin`, with `admin` implying normal employee
  access. Ignore `SCA-Engineering-Users` and `SCA-Engineering-Admins`; deny identities with no
  recognized Vendor group.
- Use synthetic, short-lived training session material that is revoked during reset.
- Keep the challenge artifact absent from the safe profile and from normal Vendor deployments.
- Do not colocate real Dex signing keys or production Engineering sessions on the Vendor host.
