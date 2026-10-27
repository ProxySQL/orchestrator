# Manual Release Signing Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Hold new package releases as drafts until manual signing and verification.

**Architecture:** Native architecture jobs upload intermediate artifacts. One final
job validates all packages and creates the draft with build provenance. Python
stdlib commands manage preparation, manual signing, verification/upload and publication.

**Tech Stack:** GitHub Actions, GitHub CLI, Python 3.9+, GnuPG, RPM 4.17+.

**Spec:** `docs/superpowers/specs/2026-10-27-manual-release-signing-design.md`

## Global Constraints

- Approved key: `653F85BB38256DF8A96206C3E8CA2E8D8217C97E`.
- No private key exports, CI signing credentials or automated server access.
- Preserve rc prerelease handling and independent GHCR publication.
- Draft assets: fourteen packages and BUILD-INFO.json; signed assets add two checksum files.
- All mutating GitHub operations must be restricted to the prepared draft.

## Review Focus

- Re-running a build must not overwrite a published or already signed release.
- Prerelease RPM filenames differ from DEB and tarball filenames.
- An unsigned RPM can return a successful checksig exit status on RPM 4.17.
- Returned signing output must not be able to replace the trusted local inventory.
- Partial uploads must be recoverable without accidentally publishing the release.

### Task 1: Inventory and provenance

**Files:** `script/release.py`, `tests/release/test_release.py`, `.github/workflows/release.yml`.
**Interfaces:** `expected_names(tag, arch=None)`, `check_packages(directory, tag, arch=None)`,
`prepare(tag, directory, repo)`, BUILD-INFO.json schema version 1.

- [ ] Write tests for fourteen package inventory, seven per architecture, rc names,
  missing/extra files, symlinks and invalid tag names. Run and observe failure.
- [ ] Implement inventory and build provenance checks; add draft preparation with
  recorded release identity, workflow run/attempt, original checksums and backup files.
- [ ] Split architecture builds and final draft creation. Fail missing files and
  refuse existing releases. Preserve container jobs and prerelease expression.
- [ ] Run inventory/provenance tests and actionlint.

### Task 2: Signing and verification

**Files:** `script/sign-release.py`, `script/release.py`, `tests/release/test_release.py`.
**Interfaces:** `sign(directory, output=None, gpg="gpg")`,
`verify(directory, signed=None, public_key=None) -> dict[str, str]`.

- [ ] Write tests for changed unsigned inventory, wrong signing key, unsigned RPMs,
  incomplete/changed manifests, modified DEBs/tarballs and unchanged RPM payloads.
- [ ] Implement fresh output signing using installed RPM macros and existing GPG
  credentials; create final checksums and detached signature after RPM signing.
- [ ] Implement isolated full-fingerprint GPG/RPM verification; run real crypto tests
  with a temporary key and a public unsigned RPM fixture, never the production key.

### Task 3: Verified upload, publication and documentation

**Files:** `script/release.py`, `tests/release/test_release.py`, `docs/release.md`.
**Interfaces:** `upload(directory, signed=None, public_key=None)`,
`publish(directory, public_key=None)`.

- [ ] Write state-guard tests for published/replaced drafts, changed tag/run, altered
  assets and partial uploads. Assert upload cannot invoke publication.
- [ ] Implement verified upload and full re-download checks. Implement publication
  separately, requiring re-verification and preserving prerelease state.
- [ ] Document server-only and manual-transfer operation, approved key, seventeen
  assets, backup/resume behavior, rc handling, and independent GHCR publication.
- [ ] Run the full release-tool suite, syntax/lint and diff checks. Obtain independent
  branch review, fix important findings with regression tests, commit, push the
  feature branch and open a PR against master.
