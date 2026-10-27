# Manual package release signing

## Intent and scope

Implement the manual signing proposal approved in the support-ticket discussion.
A pushed version tag builds packages for amd64 and arm64. Only a complete build
may create a draft release; publication follows operator signing and verification.
The approved key is `653F85BB38256DF8A96206C3E8CA2E8D8217C97E` (ProxySQL Package Builder).
Keep the private key on the signing server. No SSH automation or signing secrets
are added to GitHub Actions. GHCR publication and rc handling retain their behavior.

## Approaches

The conversation selected a draft release followed by manual signing, verification,
upload, and separate publication. Automatic server signing would require new CI
access to the signing key. Keeping immediate package publication would expose
unsigned packages before the operator could sign them. Neither alternative fits
the approved proposal.

## Interfaces and invariants

- `script/release.py`: check inventory, record build provenance, prepare one draft,
  verify signed output, upload verified assets, and separately publish.
- `script/sign-release.py`: manually sign a prepared bundle using existing server
  credentials; support the existing GPG wrapper through an explicit executable path.
- `docs/release.md`: commands usable entirely on repo.proxysql.com, with manual
  transfer instructions when preparation/verification happen on a laptop.
- Every tag has fourteen packages: three variants in RPM/DEB for two architectures,
  plus the full tarball for each architecture. RPM prerelease versions replace
  hyphens with underscores; DEBs and tarballs preserve the tag version.
- CI also attaches `BUILD-INFO.json`, binding repository, tag, commit, workflow
  run/attempt, and unsigned package digests. The signed checksum manifest covers
  these fifteen files. A fully signed draft therefore has seventeen assets.
- Preparation requires a fresh directory, a complete draft, successful matching
  workflow run/attempt, and GitHub/CI checksum agreement. Preserve original bytes
  and provenance separately from returned signing output.
- Signing validates all original inventory/checksums first, writes to a fresh
  output directory, signs all six RPMs with SHA256, then hashes final files and
  creates the detached SHA256 manifest signature. DEBs/tarballs/provenance stay
  byte-for-byte unchanged. The original unsigned bundle is retained.
- Verification trusts only the approved full fingerprint in isolated GPG and RPM
  keyrings; it requires actual RPM signatures, complete manifest coverage, and
  unchanged RPM header/payload digests and unchanged non-RPM bytes.
- Upload checks draft identity, tag/commit/run and original/current asset state,
  supports resuming a partial upload, replaces the RPM files themselves, then
  uploads checksum files. Re-download and verify all seventeen uploaded assets.
- Publication is a separate command that re-verifies the uploaded draft and changes
  only its draft flag. Preserve prerelease state.
- CI refuses to overwrite any existing release on rerun. Operators can continue
  with the existing complete draft or explicitly remove an unused draft themselves.

## Validation

Use stdlib unittest for inventory, provenance, tampering, and release-state guards.
Exercise real GPG/RPM signing with an isolated temporary test key, including wrong
key, unsigned RPM, altered package, incomplete manifest, and symlink rejection.
Lint the workflow with actionlint. Do not publish a tag or modify existing releases
while testing this PR.
