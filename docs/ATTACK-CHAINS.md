# Stonehenge instructor answer key

The original Stonehenge scoring plan totals **100 points**. This export enables
application findings by default. Windows and native Vendor-host findings need
the hosts described in [INSTRUCTOR.md](INSTRUCTOR.md). A missing host does not
make its findings available in the one-host test mode.

## Intended route

```text
anonymous LDAP -> Chopper -> DC clue ENG-1847
        or
FCU-LEGACY1$ -> reset Grim -> DC clue ENG-1847
        -> Vendor employee -> shipment replay -> Java deserialization
        -> stonevendor -> support plugin -> root on native Vendor VM
        -> AES keytab -> restricted SMB share -> Count password
        -> Count Dex session -> Engineering role and object findings

Astro Staff application -> URL allowlist bypass -> reviewer DOM XSS
                        -> six synthetic metadata clues

Public API -> anonymous operational reads and legacy client-key scopes
```

## Scoring

| Area | Points | Evidence |
| --- | ---: | --- |
| Anonymous LDAP and Chopper | 5 | Anonymous staff description reveals Chopper's temporary password; Chopper can authenticate to DC RDP. |
| FCU-AD-001 | 15 | Predictable `FCU-LEGACY1$` machine login; exact reset right over Grim; ENG-1847; Vendor root and AES keytab; restricted Engineering SMB handoff; Count login with Engineering User role. |
| Public and legacy API | 15 | Anonymous system/Astro history; three scoped legacy keys; expanded Vendor DTOs; Engineering completion queue; verbose scope denial. |
| FCU-ASTRO-001 | 10 | Astro map URL accepted by Staff form; unescaped target executes in reviewer; same-origin collector receives synthetic metadata. |
| FCU-INFO-001 through 006 | 6 | Firing-cycle timing, safety state, readiness, target assignments, maintenance counters, operational history. |
| FCU-ENG-001 | 3 | Cross-role workflow read, including part request 2003 and NIGHTWATCH/ENG-1847 breadcrumb. |
| FCU-ENG-002 | 4 | Engineering User reads command 1002 and performs application-local simulated replay. |
| FCU-ENG-003 | 3 | Engineering User obtains panel 99 capability. |
| FCU-ENG-004 | 4 | Panel capability reads administrator diagnostics. |
| SCA-VENDOR-001 | 4 | Employee reads Actuator mappings/info and serializer clue. |
| SCA-VENDOR-002 | 4 | Employee exports and replays a legitimate compatibility shipment job. |
| SCA-VENDOR-003 | 9 | CommonsCollections6 object stream reaches `ObjectInputStream.readObject` and executes as `stonevendor`. |
| SCA-VENDOR-004 | 3 | Submitted external request ID overrides persisted completion relationship. |
| SCA-VENDOR-005 | 3 | Employee retries an eligible failed completion. |
| SCA-VENDOR-006 | 3 | Employee reads detailed sync diagnostics. |
| Vendor API completion replay | 2 | Repeating a completion creates duplicate API-local workflow/audit events. |
| SCA-VENDOR-008 | 7 | Writable plugin loaded by sudo support collector yields controlled root evidence on native Vendor host. |
| **Total** | **100** | |

## Other paths

`FCU-AD-EGG-001` is a zero-point delegation exercise: `FCU-LEGACY1$` can
request a ticket for `time/fcu-dc.fcu.gov` while impersonating the one
delegable Domain Admin, `svc_flightops`; changing the ticket service name to
`ldap` reaches the same DC. `SCA-VENDOR-007` is unimplemented.
`SCA-VENDOR-009` and `FCU-AUTH-001` require a separately supplied recovery
seed and are absent from this export. The Count cookie minting utility is an
instructor fixture, not a contestant path.

The application-specific details and corrected behavior are in
[Vendor's vulnerability manifest](../apps/vendor/training/vulnerabilities.yml),
[Engineering's training profile](../apps/core/stonehenge-engineering-app/docs/training-vulnerabilities.md),
and [Astro's reviewer code](../apps/astro/tools/training/generate-astro-xss-payload.ts).
