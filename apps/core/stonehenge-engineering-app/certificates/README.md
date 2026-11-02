# FCU Lab Root CA

`fcu-lab-root-ca.crt` is the public Step CA root used to validate internal FCU
TLS services, including Dex at `login.fcu.gov`.

- Subject: `O=FCU Lab PKI, CN=FCU Lab PKI Root CA`
- SHA-256: `6D:89:64:3A:EA:8F:18:CE:E0:B0:9B:B8:16:30:53:F3:CA:B1:20:B8:F1:7A:65:FF:B4:DC:4C:37:3B:43:80:8C`
- Valid through: `2036-07-12 18:15:06 UTC`

The root is installed in the application container's system trust store. Do
not replace it without verifying the new fingerprint out of band with the PKI
administrator.
