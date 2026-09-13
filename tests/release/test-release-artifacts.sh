#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
SCRIPT="${ROOT}/script/release-artifacts"
TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

VERSION=4.31.0

expected=$(
  cat <<'EOF'
orchestrator-4.31.0-1.aarch64.rpm
orchestrator-4.31.0-1.x86_64.rpm
orchestrator-4.31.0-linux-amd64.tar.gz
orchestrator-4.31.0-linux-arm64.tar.gz
orchestrator-cli-4.31.0-1.aarch64.rpm
orchestrator-cli-4.31.0-1.x86_64.rpm
orchestrator-client-4.31.0-1.aarch64.rpm
orchestrator-client-4.31.0-1.x86_64.rpm
orchestrator-cli_4.31.0-1_amd64.deb
orchestrator-cli_4.31.0-1_arm64.deb
orchestrator-client_4.31.0-1_amd64.deb
orchestrator-client_4.31.0-1_arm64.deb
orchestrator_4.31.0-1_amd64.deb
orchestrator_4.31.0-1_arm64.deb
EOF
)
expected=$(LC_ALL=C sort <<<"${expected}")

create_assets() {
  local directory=$1 asset
  mkdir -p "${directory}"
  while IFS= read -r asset; do
    printf 'fixture for %s\n' "${asset}" >"${directory}/${asset}"
  done <<<"${expected}"
}

actual=$("${SCRIPT}" expected-unsigned "${VERSION}")
[[ ${actual} == "${expected}" ]]
[[ $(wc -l <<<"${actual}") -eq 14 ]]

for invalid in v4.31.0 4.31 4.31.0-rc rc1 4.31.0-rc0-extra '4.31.0;true'; do
  if "${SCRIPT}" expected-unsigned "${invalid}" >/dev/null 2>&1; then
    echo "expected invalid version to fail: ${invalid}" >&2
    exit 1
  fi
done
"${SCRIPT}" expected-unsigned 4.31.0-rc1 >/dev/null

assets="${TMPDIR}/assets"
create_assets "${assets}"
"${SCRIPT}" validate-unsigned "${VERSION}" "${assets}"

mkdir -p "${TMPDIR}/fake-bin"
cat >"${TMPDIR}/fake-bin/dpkg-deb" <<'EOF'
#!/usr/bin/env bash
base=${2##*/}
package=${base%%_*}
version=${base#*_}
version=${version%-1_*}
arch=${base##*_}
arch=${arch%.deb}
case $3 in
  Package) printf '%s\n' "${package}" ;;
  Version) printf '1:%s-1\n' "${version}" ;;
  Architecture) printf '%s\n' "${FAKE_DEB_ARCH:-${arch}}" ;;
esac
EOF
cat >"${TMPDIR}/fake-bin/rpm" <<'EOF'
#!/usr/bin/env bash
base=${!#}
base=${base##*/}
case ${base} in
  orchestrator-cli-*) package=orchestrator-cli ;;
  orchestrator-client-*) package=orchestrator-client ;;
  *) package=orchestrator ;;
esac
arch=${base%.rpm}
arch=${arch##*.}
printf '%s\t4.31.0\t%s\t%s\n' \
  "${package}" "${FAKE_RPM_RELEASE:-1}" "${arch}"
EOF
cat >"${TMPDIR}/fake-bin/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${FAKE_TAR_MISSING:-0} != 1 ]]; then
  printf './usr/local/orchestrator/orchestrator\n'
  if [[ ${FAKE_TAR_LONG:-0} == 1 ]]; then
    for ((line = 0; line < 20000; line++)); do
      printf './usr/local/orchestrator/resources/padding-%08d\n' "${line}"
    done
  fi
fi
EOF
chmod +x "${TMPDIR}/fake-bin/"*

PATH="${TMPDIR}/fake-bin:${PATH}" \
  "${SCRIPT}" validate-packages "${VERSION}" "${assets}"
FAKE_TAR_LONG=1 PATH="${TMPDIR}/fake-bin:${PATH}" \
  "${SCRIPT}" validate-packages "${VERSION}" "${assets}"
for failure in FAKE_DEB_ARCH=wrong FAKE_RPM_RELEASE=2 FAKE_TAR_MISSING=1; do
  if (export "${failure}"; PATH="${TMPDIR}/fake-bin:${PATH}" \
      "${SCRIPT}" validate-packages "${VERSION}" "${assets}") >/dev/null 2>&1; then
    echo "expected package metadata validation failure: ${failure}" >&2
    exit 1
  fi
done

missing="${TMPDIR}/missing"
create_assets "${missing}"
rm "${missing}/orchestrator-4.31.0-linux-arm64.tar.gz"
if "${SCRIPT}" validate-unsigned "${VERSION}" "${missing}" >/dev/null 2>&1; then
  echo 'expected a missing artifact to fail' >&2
  exit 1
fi

extra="${TMPDIR}/extra"
create_assets "${extra}"
touch "${extra}/unexpected.txt"
if "${SCRIPT}" validate-unsigned "${VERSION}" "${extra}" >/dev/null 2>&1; then
  echo 'expected an unexpected artifact to fail' >&2
  exit 1
fi

duplicate="${TMPDIR}/duplicate"
create_assets "${duplicate}"
mkdir "${duplicate}/nested"
cp "${duplicate}/orchestrator_4.31.0-1_amd64.deb" "${duplicate}/nested/"
if "${SCRIPT}" validate-unsigned "${VERSION}" "${duplicate}" >/dev/null 2>&1; then
  echo 'expected a duplicate basename to fail' >&2
  exit 1
fi

checksums="${TMPDIR}/unsigned-SHA256SUMS"
"${SCRIPT}" write-unsigned-checksums "${VERSION}" "${assets}" "${checksums}"
[[ $(wc -l <"${checksums}") -eq 14 ]]
[[ $(sed -E 's/^[0-9a-f]{64}  //' "${checksums}") == "${expected}" ]]
if grep -Evq '^[0-9a-f]{64}  [^/]+$' "${checksums}"; then
  echo 'checksum format is not sha256sum-compatible' >&2
  exit 1
fi

echo 'release artifact tests: PASS'
