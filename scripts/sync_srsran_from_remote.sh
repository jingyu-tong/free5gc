#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=srsran_env.sh
source "${SCRIPT_DIR}/srsran_env.sh"

mkdir -p "$(dirname "${LOCAL_SRSRAN_ABS}")"

ssh "${REMOTE_HOST}" "test -f '${REMOTE_SRSRAN_ROOT}/CMakeLists.txt'"

rsync -az --delete \
  --exclude '.git/' \
  --exclude '.DS_Store' \
  --exclude 'build/' \
  --exclude 'install/' \
  --exclude 'cmake-build-*/' \
  --exclude '.cache/' \
  "${REMOTE_HOST}:${REMOTE_SRSRAN_ROOT}/" \
  "${LOCAL_SRSRAN_ABS}/"

git -C "${FREE5GC_ROOT}" status --short -- "${LOCAL_SRSRAN_ROOT}" | head -n 80
