# Releasing and publishing packages

`orchestrator` release artifacts are built and published automatically by the [`Release` workflow](../.github/workflows/release.yml). The workflow fires on any pushed tag matching `v*` and produces:

- `tar.gz`, `.deb`, and `.rpm` packages attached to a GitHub Release
- A multi-arch Docker image pushed to `ghcr.io/proxysql/orchestrator`

Both artifact sets cover `linux/amd64` and `linux/arm64`.

## Triggering a release

```
    git tag v4.30.1
    git push origin v4.30.1
```

That's the whole trigger. The workflow picks up the tag name as the release version, strips the leading `v`, and uses `4.30.1` for package versions and Docker tags.

### Prerelease (release candidate) tags

Tags that contain `rc` (e.g. `v4.30.1-rc1`) are handled specially:

- The GitHub Release is marked as a prerelease.
- The Docker `latest` tag is **not** updated — only the specific version tags are pushed.

Use this for testing the release pipeline end-to-end on a fork before cutting a real release.

## What runs

The workflow has three jobs:

### 1. `build-and-release` (matrix: amd64, arm64)

Runs on `ubuntu-latest` (amd64) and `ubuntu-24.04-arm` (arm64) — GitHub's native ARM runners, free for public repos. No QEMU, no cross-compilation: each job builds on its own native architecture, so CGO (`go-sqlite3`) works without any special toolchain.

Each matrix job:

1. Installs Go, `fpm`, and `rpmbuild`.
2. Runs `./build.sh -a <goarch>` with `RELEASE_VERSION` set from the tag.
3. Collects everything `build.sh` writes to `/tmp/orchestrator-release/` and uploads it to the GitHub Release.

Per arch, `build.sh` produces three variants (see `package_linux` in [`build.sh`](../build.sh)):

- `orchestrator` — full package (binary + web resources + sample configs + systemd unit)
- `orchestrator-cli` — binary only
- `orchestrator-client` — the `orchestrator-client` shell script only

Each variant is emitted as `.tar.gz`, `.deb`, and `.rpm`. Package names differ by arch (`_amd64.deb` / `_arm64.deb`, `.x86_64.rpm` / `.aarch64.rpm`), so the two matrix jobs don't collide when uploading to the same Release.

Both matrix jobs call `softprops/action-gh-release@v2` — the action is idempotent and will attach to the existing Release created by whichever job finishes first.

### 2. `docker-build` (matrix: linux/amd64, linux/arm64)

Each arch builds [`docker/Dockerfile`](../docker/Dockerfile) natively on its own runner and pushes **by digest** to `ghcr.io/proxysql/orchestrator` (no tag yet). The digest is uploaded as a workflow artifact for the merge job to consume.

### 3. `docker-merge`

Downloads both digests, runs `docker/metadata-action` to compute tags from the git tag:

- `type=semver,pattern={{version}}` — e.g. `4.30.1`
- `type=semver,pattern={{major}}.{{minor}}` — e.g. `4.30`
- `type=raw,value=latest` — only when the tag does **not** contain `rc`

Then uses `docker buildx imagetools create` to assemble a multi-arch manifest under all those tags, and inspects the result.

## Verifying a release

### GitHub Release page

After the workflow completes, the Release page should list (for version `X.Y.Z`):

```
orchestrator-X.Y.Z-linux-amd64.tar.gz
orchestrator-X.Y.Z-linux-arm64.tar.gz
orchestrator_X.Y.Z-1_amd64.deb
orchestrator_X.Y.Z-1_arm64.deb
orchestrator-X.Y.Z-1.x86_64.rpm
orchestrator-X.Y.Z-1.aarch64.rpm
```

Plus the `-cli` and `-client` variants in `.deb` and `.rpm` form for each arch.

### Docker manifest

```
    docker buildx imagetools inspect ghcr.io/proxysql/orchestrator:X.Y.Z
```

The output should list both `linux/amd64` and `linux/arm64` entries.

Pull and smoke-test each arch:

```
    docker run --rm --platform linux/amd64 ghcr.io/proxysql/orchestrator:X.Y.Z orchestrator --version
    docker run --rm --platform linux/arm64 ghcr.io/proxysql/orchestrator:X.Y.Z orchestrator --version
```

## Local reproduction

The release workflow does the same thing you can do locally with `build.sh` — see [Building and testing](build.md) and [`build.sh`](../build.sh). To produce ARM64 packages on a non-ARM host (outside of CI), run inside an arm64 container:

```
    docker run --rm -it --platform linux/arm64 \
      -v $PWD:/src -w /src \
      ubuntu:24.04 bash -c '
        apt-get update &&
        apt-get install -y golang git ruby ruby-dev build-essential rpm &&
        gem install --no-document fpm &&
        ./build.sh -a arm64
      '
```

## Permissions and secrets

The workflow relies on `GITHUB_TOKEN` with `contents: write` (for the Release) and `packages: write` (for GHCR) — both declared in [`release.yml`](../.github/workflows/release.yml). No additional secrets are required.

## Troubleshooting

- **Workflow didn't run after tagging.** The tag must start with `v` (see the `on.push.tags` filter). Tags pushed without `git push --tags` or without pushing the specific ref won't trigger it.
- **A matrix job failed mid-way and part of the release is missing.** The workflow uses `fail-fast: false`, so the other arch still completes. Re-running only the failed job from the Actions UI is safe — the GitHub Release and GHCR both accept re-uploads (fpm uses `-f` to overwrite, `action-gh-release` replaces files of the same name).
- **Docker manifest is missing one arch.** If `docker-build` succeeded for only one arch, `docker-merge` will still run but `imagetools create` will produce a single-arch manifest. Re-run the failed `docker-build` job, then re-run `docker-merge`.
