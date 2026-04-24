#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=srsran_env.sh
source "${SCRIPT_DIR}/srsran_env.sh"

ssh "${REMOTE_HOST}" "set -euo pipefail
cd '${REMOTE_SRSRAN_ROOT}'
cmake -S . -B build -DCMAKE_BUILD_TYPE='${SRSRAN_BUILD_TYPE}' ${SRSRAN_CMAKE_ARGS}
cmake --build build -j\"\$(nproc)\"
if [[ -x build/apps/gnb/gnb ]]; then
  build/apps/gnb/gnb --version || true
elif command -v gnb >/dev/null 2>&1; then
  gnb --version || true
fi"
