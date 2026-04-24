#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=srsran_env.sh
source "${SCRIPT_DIR}/srsran_env.sh"

mkdir -p "$(dirname "${LOCAL_SRSRAN_ABS}")"

if [[ -e "${LOCAL_SRSRAN_ABS}/CMakeLists.txt" ]]; then
  echo "srsRAN source already exists: ${LOCAL_SRSRAN_ABS}"
  exit 0
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

archive_url="${SRSRAN_REPO%.git}/archive/refs/heads/${SRSRAN_BRANCH}.tar.gz"
archive_path="${tmp_dir}/srsRAN_Project.tar.gz"

download_archive() {
  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --connect-timeout 20 --output "${archive_path}" "${archive_url}"
    return $?
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -O "${archive_path}" "${archive_url}"
    return $?
  fi
  return 1
}

if download_archive; then
  tar -xzf "${archive_path}" -C "${tmp_dir}"
  mv "${tmp_dir}"/srsRAN_Project-* "${tmp_dir}/srsRAN_Project"
else
  echo "Archive download failed; falling back to shallow git clone." >&2
  git clone --depth 1 --single-branch --branch "${SRSRAN_BRANCH}" "${SRSRAN_REPO}" "${tmp_dir}/srsRAN_Project"
fi

mkdir -p "${LOCAL_SRSRAN_ABS}"
rsync -a "${tmp_dir}/srsRAN_Project/" "${LOCAL_SRSRAN_ABS}/"

cat > "${LOCAL_SRSRAN_ABS}/FREE5GC_VENDOR_SOURCE.txt" <<EOF
This srsRAN_Project source tree is vendored into the free5GC project repository.
The nested upstream .git directory is intentionally removed so this repository
tracks local srsRAN source changes directly.

Upstream: ${SRSRAN_REPO}
Archive: ${archive_url}
Branch: ${SRSRAN_BRANCH}
Bootstrapped: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

git -C "${FREE5GC_ROOT}" status --short -- "${LOCAL_SRSRAN_ROOT}" | head -n 40
