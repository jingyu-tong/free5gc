#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=srsran_env.sh
source "${SCRIPT_DIR}/srsran_env.sh"

MODE="${1:-zmq}"
CONFIG_PATH="${REMOTE_FREE5GC_ROOT}/ran/srsran/gnb_${MODE}.yaml"
LOG_DIR="${REMOTE_FREE5GC_ROOT}/ran/srsran/logs"

ssh -tt "${REMOTE_HOST}" "set -euo pipefail
mkdir -p '${LOG_DIR}'
cd '${REMOTE_SRSRAN_ROOT}'
if [[ -x build/apps/gnb/gnb ]]; then
  exec sudo -E build/apps/gnb/gnb -c '${CONFIG_PATH}' 2>&1 | tee '${LOG_DIR}/gnb_${MODE}.log'
fi
exec sudo -E gnb -c '${CONFIG_PATH}' 2>&1 | tee '${LOG_DIR}/gnb_${MODE}.log'"
