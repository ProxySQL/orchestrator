# Releasing and publishing packages

Orchestrator releases use a staged workflow. GitHub Actions builds unsigned
packages and untagged container images by digest. The release host validates
and signs the packages with the ProxySQL Package Builder key, prepares a draft,
and verifies the downloaded draft. An operator publishes that draft and only
then promotes the container digests to version tags.

The private signing key never enters GitHub Actions.

## Trigger the build

Create only a strict stable or release-candidate tag:

```bash
git tag v4.31.0
git push origin v4.31.0
```

Accepted forms are `vMAJOR.MINOR.PATCH` and
`vMAJOR.MINOR.PATCH-rcNUMBER`. The tag triggers the
[`Release` workflow](../.github/workflows/release.yml). Record its numeric run
ID and full source commit SHA.

The package matrix builds natively on amd64 and arm64. For each architecture,
[`build.sh`](../build.sh) produces:

- one full `orchestrator` tarball;
- `orchestrator`, `orchestrator-cli`, and `orchestrator-client` DEBs; and
- `orchestrator`, `orchestrator-cli`, and `orchestrator-client` RPMs.

Across both architectures, the unsigned stage is exactly two tarballs, six
DEBs, and six RPMs. `validate-package-stage` checks this exact manifest and
publishes an `unsigned-SHA256SUMS` workflow artifact. These hashes identify
the signing inputs; the final DEB and RPM hashes change when their embedded
signatures are added.

In parallel, `docker-build` pushes each architecture only by digest and stores
the two digest files as short-lived workflow artifacts. It does not create a
version, minor, or `latest` tag.

## Prepare a signed draft

On the release host, follow the release-kraken
`ORCHESTRATOR_RUNBOOK.md`. Supply the exact tag and successful Release workflow
run ID:

```bash
cd /root/release-kraken
./release-orchestrator.sh prepare v4.31.0 RUN_ID
./release-orchestrator.sh verify v4.31.0
```

The prepare command independently resolves the tag, requires the workflow run
to match its full source SHA, downloads only that run's artifacts, revalidates
all hashes and package metadata, and signs copies of the packages. It creates a
draft release containing exactly:

- the two tarballs and their detached `.asc` signatures;
- six internally signed DEBs and six internally signed RPMs; and
- `SHA256SUMS` plus its detached `SHA256SUMS.asc` signature.

It then downloads and verifies the draft again. Re-running prepare may replace
assets only in a matching draft; it refuses to mutate a published release or a
draft targeting another commit.

Review the draft's target SHA, notes, complete asset list, and release status.
The public verification procedure and fingerprint are documented in [Package
signature verification](package-signatures.md).

## Publish the packages

After review and approval, publish from the release host:

```bash
./release-orchestrator.sh publish v4.31.0
```

This repeats remote target, manifest, checksum, and signature verification
before changing the draft to public. A stable version is a regular Latest
GitHub release. A tag such as `v4.31.0-rc1` is a prerelease.

Published releases and tags are immutable for this automation. Correct a
published-release problem with a new version.

## Promote the container image

After the signed package release is public, dispatch
[`Promote Release`](../.github/workflows/promote-release.yml) with the original
tag and Release workflow run ID:

```bash
gh workflow run 'Promote Release' -R ProxySQL/orchestrator \
  -f tag=v4.31.0 -f build_run_id=RUN_ID
```

The protected `orchestrator-production` environment is the approval boundary.
The workflow verifies that the tag, build run, public release, full commit SHA,
prerelease state, and exact asset manifest agree. It requires both amd64 and
arm64 image digests, then creates:

- `X.Y.Z` and `X.Y` for every release; and
- `latest` only for a stable release, never a release candidate.

It finally verifies that the multi-architecture manifest contains
`linux/amd64` and `linux/arm64`. These tags select the images built in the
original release run; OpenPGP package signatures do not sign OCI images.

## Verify the result

Use the customer procedure in [Package signature
verification](package-signatures.md) against a clean download directory. Also
inspect the container manifest:

```bash
docker buildx imagetools inspect ghcr.io/proxysql/orchestrator:X.Y.Z
docker run --rm --platform linux/amd64 \
  ghcr.io/proxysql/orchestrator:X.Y.Z orchestrator --version
docker run --rm --platform linux/arm64 \
  ghcr.io/proxysql/orchestrator:X.Y.Z orchestrator --version
```

## Local package reproduction

The release workflow invokes `build.sh` directly. See [Building and
testing](build.md). To build ARM64 packages on a non-ARM host, use an arm64
container:

```bash
docker run --rm -it --platform linux/arm64 \
  -v "$PWD:/src" -w /src \
  ubuntu:24.04 bash -c '
    apt-get update &&
    apt-get install -y golang git ruby ruby-dev build-essential rpm &&
    gem install --no-document fpm &&
    ./build.sh -a arm64
  '
```

Locally reproduced packages are not official release artifacts and are not
signed unless they pass through the production release process.

## Failure handling

- If a package build or staging validation fails, rerun the failed job. Never
  select a run unless the complete Release workflow concluded successfully.
- If prepare fails before creating a draft, correct the cause and rerun it with
  the same immutable tag and run ID.
- If a matching draft exists, prepare can safely replace its assets and verify
  them again. It cannot replace public assets.
- If container promotion fails, leave the signed release untouched and rerun
  promotion with the same tag and build run after fixing the issue.
- Digest and package workflow artifacts are retained for fourteen days.
  Complete review and promotion within that window or rerun the build from the
  unchanged tag.

## Permissions and secrets

The tag-triggered workflow has `contents: read` and `packages: write`; it cannot
publish a GitHub release. The promotion workflow adds `actions: read` so it can
retrieve the selected run's digests and uses the protected production
environment.

The signing host has the private key and a narrowly scoped GitHub credential
with Actions read and Contents write access to `ProxySQL/orchestrator`. Its
passphrase is kept in a root-owned mode-`0600` secrets file. No GPG private-key
material or passphrase belongs in GitHub repository or environment secrets.
