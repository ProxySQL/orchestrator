# Releasing and signing packages

A pushed `v*` tag starts the [Release workflow](../.github/workflows/release.yml).
Native amd64 and arm64 jobs each build seven Linux packages. A final job waits
for **both** builds, validates all fourteen packages, and creates a **draft**
GitHub Release with `BUILD-INFO.json`. An operator signs and verifies the packages
before explicitly publishing the draft.

Container images still publish automatically to `ghcr.io/proxysql/orchestrator`.
Holding the package release as a draft does not hold back GHCR images. Container
signing and coordinated container publication are outside this procedure.

## Approved signing key

- Identity: `ProxySQL Package Builder <builder@proxysql.com>`
- Full fingerprint: `653F85BB38256DF8A96206C3E8CA2E8D8217C97E`
- [Public key](https://repo.proxysql.com/ProxySQL/proxysql-3.0.x/repo_pub_key)

RPMs receive embedded RSA/SHA256 signatures. DEBs and tarballs remain unchanged
and are authenticated by the detached signature on `SHA256SUMS`; they do not
receive embedded package signatures. The manifest also covers `BUILD-INFO.json`.
The private key remains in the signing server's existing keyring. GitHub Actions
has no signing key, passphrase, or SSH access to that server.

## Trigger a release

```bash
git tag v4.32.1
git push origin v4.32.1
```

Wait for the entire Release workflow to succeed, then locate its draft release.
The workflow fails rather than creating a partial draft if a build or inventory
check fails. Each architecture produces:

- Full `orchestrator` tarball.
- Full, CLI-only and client-only variants in both `.deb` and `.rpm` formats.

The final job records the repository, tag, exact commit, workflow run and attempt,
and original package SHA-256 hashes in `BUILD-INFO.json`. There are fifteen assets
at this stage: fourteen packages and build provenance.

### Release candidates

Tags containing `rc`, such as `v4.32.1-rc1`, retain the existing behavior:

- The draft and published GitHub Release are marked as a prerelease.
- GHCR does not update `latest`; version tags still publish automatically.
- RPM filenames use `4.32.1_rc1`; DEB and tarball filenames use `4.32.1-rc1`.

## Prepare, sign, verify and upload

These commands can all run on `repo.proxysql.com`. Alternatively, run preparation,
verification, upload and publication on a laptop and transfer the bundle manually
for signing as described below. Use a checkout containing these scripts; Python
3.9+, authenticated `gh`, GnuPG and RPM tools (`rpm`, `rpmkeys`, `rpmsign`) are
required. On macOS the verification tools are available through `brew install gnupg rpm`; on Ubuntu install `gnupg rpm`.

Use one operator per tag. Keep the original preparation directory until the
release has been published and verified. It contains the trusted local release
identity, unsigned checksums, provenance and original package backups.

### 1. Download one specific draft into a fresh directory

```bash
cd /path/to/orchestrator
python3 script/release.py prepare v4.32.1 /path/to/releases/v4.32.1
```

`--repo OWNER/REPO` selects a fork when testing. The command requires a draft,
exact artifact coverage, matching GitHub and CI digests, a tag pointing at the
recorded commit, and a successful matching workflow run/attempt. It refuses an
existing preparation directory. The result is:

```text
v4.32.1/
  release.json           # trusted local snapshot of the draft and build identity
  UNSIGNED-SHA256SUMS     # original checksums, including BUILD-INFO.json
  unsigned/              # original fourteen packages and BUILD-INFO.json
```

### 2. Sign manually using the existing server keyring

On the signing server, from the same script checkout:

```bash
python3 script/sign-release.py /path/to/releases/v4.32.1
```

Use an interactive terminal if the existing key requires a passphrase. An
established server-side GPG wrapper can be selected with `--gpg
/root/path/to/signing-gpg`; credentials must stay in the existing root-only
configuration or GPG agent. The script does not provision credentials or change
GPG/RPM configuration. Its RPM command-line macros select the approved full
fingerprint and SHA256, including compatibility with the server's RPM 4.17.

The script validates the original inventory/checksums first and creates a fresh
`signed/` directory. It signs **all six RPM files**, computes checksums **after**
RPM signing, and creates `SHA256SUMS.asc`. It then verifies the result using an
isolated public keyring. All fifteen original files are returned, with the RPMs
replaced by their signed versions, plus the two checksum files: seventeen files.
An existing signing output is never overwritten; `--output /fresh/path` selects
a different output directory after an interrupted attempt.

### 3. Verify against the original preparation directory

```bash
python3 script/release.py verify /path/to/releases/v4.32.1
```

The public key is downloaded from the approved URL, and its **full fingerprint**
is checked before import into temporary GPG and RPM keyrings containing only that
key. `--public-key /path/to/repo_pub_key` uses a previously downloaded certificate
with the same mandatory fingerprint check. Verification requires:

- A valid SHA256 signature on the complete checksum manifest.
- A valid RSA/SHA256 signature on every RPM; an unsigned RPM is rejected even if
  `rpmkeys --checksig` returns success for its digests alone.
- Unchanged RPM header/payload digests relative to the original downloads.
- Unchanged DEB, tarball and provenance bytes.
- Exact artifact coverage, with no missing, extra, empty or symlinked files.

### 4. Upload verified files and check their uploaded bytes

```bash
python3 script/release.py upload /path/to/releases/v4.32.1
```

The command verifies again, confirms the draft identity and tag/build context,
then replaces the **RPM files themselves** using `gh release upload --clobber`.
It uploads the checksum manifest/signature after the RPMs. Existing DEBs, tarballs
and provenance are left in place. It downloads all seventeen release assets into
a fresh temporary directory, repeats signature/inventory checks, and requires
byte-for-byte digest agreement with the locally verified result. Only then does
it write a local `uploaded.json` receipt. The release remains a draft.

`--clobber` deletes an old asset before uploading its replacement. Original
packages remain in `unsigned/`. If an upload fails, rerun the same upload command;
it accepts already uploaded verified files and restores missing RPMs from the
verified signed output. Empty `starter` assets left by interrupted RPM/checksum
uploads are also replaced; malformed completed assets still stop the operation. Unexpected asset changes or changed release identity
stop the operation. Never use returned server metadata to replace the original
trusted local `release.json` or `unsigned/` directory.

## Manual transfer when using a laptop

The repository scripts do not connect to any server. Transfer the prepared bundle
and make the same `script/release.py` and `script/sign-release.py` available on the
signing server. For example, run these transfers yourself:

```bash
scp -P 1022 -r /laptop/releases/v4.32.1 root@repo.proxysql.com:/root/orchestrator-signing/
ssh -t -p 1022 root@repo.proxysql.com
# On the server, from the script checkout:
python3 script/sign-release.py /root/orchestrator-signing/v4.32.1
# Back on the laptop, download ONLY the signed output into a fresh directory:
scp -P 1022 -r root@repo.proxysql.com:/root/orchestrator-signing/v4.32.1/signed /laptop/returned-v4.32.1
python3 script/release.py verify /laptop/releases/v4.32.1 --signed /laptop/returned-v4.32.1
python3 script/release.py upload /laptop/releases/v4.32.1 --signed /laptop/returned-v4.32.1
```

Return the signed RPMs, unchanged DEBs/tarballs and provenance, and **both**
checksum files. Returning only the manifest would leave unsigned RPMs on GitHub.
Private keys and passphrases are never part of the transferred bundle.

## Publish explicitly

After upload succeeds and release notes are ready, run a separate command:

```bash
python3 script/release.py publish /path/to/releases/v4.32.1
```

This requires the local upload receipt and re-downloads/re-verifies the current
remote assets against it. It then invokes `gh release edit --draft=false`,
preserving the release's prerelease flag. Published or replaced drafts are
rejected by preparation and upload commands.

## Workflow reruns and troubleshooting

- Architecture jobs retain intermediate packages for seven days. Only the final
  job creates the draft, so the two architecture jobs cannot race on release assets.
- A failed architecture job prevents draft creation. Re-run the failed build and
  the dependent final job.
- The final job refuses any existing release, including a draft, rather than
  overwriting packages after signing has started. Continue using an existing
  complete draft. To rebuild an unused draft, explicitly remove it first after
  confirming no signing/publication operation is in progress, then rerun the build.
- Preparation failures leave any downloaded bytes for inspection. Fix the source
  issue and choose another fresh preparation directory.
- The recorded workflow attempt must have succeeded and the tag must still point
  at its commit. A later workflow rerun does not invalidate an earlier successful
  attempt. Do not edit trusted inventory files to bypass a failed check.
- Container builds and manifest merging keep their existing independent jobs.
  Inspect images with `docker buildx imagetools inspect
  ghcr.io/proxysql/orchestrator:4.32.1`.

For local build reproduction see [Building and testing](build.md) and
[`build.sh`](../build.sh).

## Tests

```bash
python3 -m unittest discover -s tests/release -v
```

Crypto integration tests generate a temporary test key and build temporary RPM
fixtures; they do not sign with the production key or access live releases. They
require `gpg`, `rpm`, `rpmkeys`, `rpmbuild` and `rpmsign`; the release-tools CI job
installs these dependencies. Network-facing tests stub the GitHub boundary.
