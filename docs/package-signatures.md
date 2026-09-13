# Package signature verification

ProxySQL plans to sign packages beginning with a **future Orchestrator release**.
Until the first signed version is named here, do not assume that current or
older release assets have signatures. Existing releases will not be modified
retroactively.

Signed releases use the existing ProxySQL Package Builder OpenPGP key:

```text
Fingerprint: 653F 85BB 3825 6DF8 A962 06C3 E8CA 2E8D 8217 C97E
             653F85BB38256DF8A96206C3E8CA2E8D8217C97E
Public key:  https://repo.proxysql.com/ProxySQL/repo_pub_key
```

Do not trust a key based only on its name or short key ID. Obtain the expected
full fingerprint through a trusted ProxySQL channel, download the key over
HTTPS, and compare all 40 hexadecimal characters before importing it:

```bash
curl -fSLo proxysql-package-signing-key.asc \
  https://repo.proxysql.com/ProxySQL/repo_pub_key
gpg --show-keys --with-fingerprint proxysql-package-signing-key.asc
gpg --import proxysql-package-signing-key.asc
```

The fingerprint displayed by GnuPG must exactly equal the value above.

## Verify a complete release download

Download the desired primary artifacts, `SHA256SUMS`, and
`SHA256SUMS.asc` from the same GitHub release into one otherwise-empty
directory. First authenticate the checksum manifest, then validate the files:

```bash
gpg --verify SHA256SUMS.asc SHA256SUMS
sha256sum -c SHA256SUMS
```

The first command must report a good signature associated with the expected
full fingerprint. The second must report `OK` for every downloaded primary
artifact. Do not use an artifact when either command fails or when a required
file is absent from the signed manifest.

`SHA256SUMS` covers the final bytes of all tarballs, DEBs, and RPMs. The
checksum signature is the portable integrity and authenticity check across
package formats.

## Format-specific verification

Tarballs have ASCII-armored detached OpenPGP signatures. In addition to the
signed checksum manifest, verify a tarball directly:

```bash
gpg --verify orchestrator-X.Y.Z-linux-amd64.tar.gz.asc \
  orchestrator-X.Y.Z-linux-amd64.tar.gz
```

Replace the version and architecture with the downloaded filename and confirm
the expected full fingerprint.

RPMs have an embedded `rpmsign` signature. Import the verified public key into
a disposable RPM database and check a package before installation:

```bash
RPM_VERIFY_DB=$(mktemp -d)
rpmkeys --dbpath "$RPM_VERIFY_DB" --import proxysql-package-signing-key.asc
rpmkeys --checksig --dbpath "$RPM_VERIFY_DB" \
  orchestrator-X.Y.Z-1.x86_64.rpm
rm -rf "$RPM_VERIFY_DB"
```

The result must report valid digests and signatures; the disposable database
contains only the full-fingerprint key checked above. For RPM repository
configuration, `gpgcheck=1` checks package signatures. `repo_gpgcheck=1`
instead checks repository metadata and works only when that metadata is also
signed; GitHub release assets do not provide signed RPM repository metadata.

DEBs have an embedded `dpkg-sig` signature with role `builder`:

```bash
dpkg-sig --verify orchestrator_X.Y.Z-1_amd64.deb
```

Require `GOODSIG` from the expected key. Standard APT and `dpkg -i` do not
normally enforce `dpkg-sig`, so verify `SHA256SUMS.asc` and `SHA256SUMS` as
shown above even when checking the embedded DEB signature.

## What is and is not signed

Each signed release contains embedded signatures in six DEBs and six RPMs,
detached signatures for the two full-package tarballs, and a signed checksum
manifest for all fourteen primary artifacts. The `orchestrator-cli` and
`orchestrator-client` variants are distributed as DEB and RPM packages; only
the full `orchestrator` package has a tarball.

These OpenPGP signatures cover downloadable release artifacts. They do not
authenticate the `ghcr.io/proxysql/orchestrator` container image.
