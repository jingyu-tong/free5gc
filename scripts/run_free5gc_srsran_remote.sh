#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=srsran_env.sh
source "${SCRIPT_DIR}/srsran_env.sh"

REMOTE_LOG_ROOT="${REMOTE_LOG_ROOT:-/tmp/free5gc-srsran-core}"

ssh -tt "${REMOTE_HOST}" "set -euo pipefail
cd '${REMOTE_FREE5GC_ROOT}'
sudo -v
AMF_CONFIG_PATH=./config/amfcfg.dsmf-trigger.yaml \
SMF_CONFIG_PATH=./config/smfcfg.srsran.yaml \
UPF_CONFIG_PATH=./config/upfcfg.srsran.yaml \
./run.sh -p '${REMOTE_LOG_ROOT}'"
