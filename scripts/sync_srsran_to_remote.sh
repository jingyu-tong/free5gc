#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=srsran_env.sh
source "${SCRIPT_DIR}/srsran_env.sh"

if [[ ! -f "${LOCAL_SRSRAN_ABS}/CMakeLists.txt" ]]; then
  echo "Local srsRAN source is missing: ${LOCAL_SRSRAN_ABS}" >&2
  echo "Run scripts/bootstrap_srsran_local.sh first." >&2
  exit 1
fi

ssh "${REMOTE_HOST}" "mkdir -p '$(dirname "${REMOTE_SRSRAN_ROOT}")'"

rsync -az --delete \
  --exclude '.git/' \
  --exclude '.DS_Store' \
  --exclude 'build/' \
  --exclude 'install/' \
  --exclude 'cmake-build-*/' \
  --exclude '.cache/' \
  "${LOCAL_SRSRAN_ABS}/" \
  "${REMOTE_HOST}:${REMOTE_SRSRAN_ROOT}/"

ssh "${REMOTE_HOST}" "test -f '${REMOTE_SRSRAN_ROOT}/CMakeLists.txt' && find '${REMOTE_SRSRAN_ROOT}' -maxdepth 1 -type f | wc -l"
