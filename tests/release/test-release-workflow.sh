#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORKFLOW="${ROOT}/.github/workflows/release.yml"
PROMOTION_WORKFLOW="${ROOT}/.github/workflows/promote-release.yml"

require_text() {
  local text=$1
  grep -Fq -- "${text}" "${WORKFLOW}" || {
    printf 'release workflow is missing: %s\n' "${text}" >&2
    return 1
  }
}

reject_text() {
  local text=$1
  if grep -Fq -- "${text}" "${WORKFLOW}"; then
    printf 'release workflow still contains forbidden publication step: %s\n' "${text}" >&2
    return 1
  fi
}

require_text 'actions/upload-artifact@v4'
require_text 'name: orchestrator-packages-${{ matrix.goarch }}'
require_text 'validate-package-stage:'
require_text 'script/release-artifacts validate-unsigned'
require_text 'script/release-artifacts validate-packages'
require_text 'script/release-artifacts write-unsigned-checksums'
require_text 'name: orchestrator-unsigned-manifest'

reject_text 'softprops/action-gh-release'
reject_text 'draft: false'
reject_text 'gh release create'
reject_text 'docker buildx imagetools create'
if grep -Eq 'retention-days: 1$' "${WORKFLOW}"; then
  echo 'digest artifact retention is too short for reviewed publication' >&2
  exit 1
fi

[[ -f ${PROMOTION_WORKFLOW} ]] || {
  echo 'promotion workflow is missing' >&2
  exit 1
}
for text in \
  'workflow_dispatch:' \
  'tag:' \
  'build_run_id:' \
  'actions: read' \
  'packages: write' \
  'gh release view' \
  'isDraft' \
  'headSha' \
  'run-id: ${{ inputs.build_run_id }}' \
  'pattern: digests-*' \
  'https://repo.proxysql.com/ProxySQL/repo_pub_key' \
  '--verify SHA256SUMS.asc SHA256SUMS' \
  'script/release-artifacts expected-unsigned' \
  'actual_checksum_names' \
  'sha256sum -c SHA256SUMS' \
  'type=raw,value=latest,enable=${{ steps.preflight.outputs.prerelease == '\''false'\'' }}' \
  'docker buildx imagetools create'; do
  grep -Fq -- "${text}" "${PROMOTION_WORKFLOW}" || {
    printf 'promotion workflow is missing: %s\n' "${text}" >&2
    exit 1
  }
done

echo 'release workflow tests: PASS'
