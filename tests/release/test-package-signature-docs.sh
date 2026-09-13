#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DOC="${ROOT}/docs/package-signatures.md"

[[ -f ${DOC} ]] || { echo 'docs/package-signatures.md is missing' >&2; exit 1; }
for text in \
  '653F 85BB 3825 6DF8 A962 06C3 E8CA 2E8D 8217 C97E' \
  '653F85BB38256DF8A96206C3E8CA2E8D8217C97E' \
  'https://repo.proxysql.com/ProxySQL/repo_pub_key' \
  'gpg --show-keys --with-fingerprint' \
  'gpg --verify SHA256SUMS.asc SHA256SUMS' \
  'sha256sum -c SHA256SUMS' \
  'rpmkeys --checksig' \
  'dpkg-sig --verify' \
  'gpgcheck=1' \
  'repo_gpgcheck=1' \
  'future Orchestrator release'; do
  grep -Fq -- "${text}" "${DOC}" || {
    printf 'package signature documentation is missing: %s\n' "${text}" >&2
    exit 1
  }
done

grep -Fq '(package-signatures.md)' "${ROOT}/docs/download.md"
grep -Fq '(package-signatures.md)' "${ROOT}/docs/install.md"

echo 'package signature documentation tests: PASS'
