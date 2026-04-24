#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=srsran_env.sh
source "${SCRIPT_DIR}/srsran_env.sh"

ssh "${REMOTE_HOST}" "mkdir -p '${REMOTE_FREE5GC_ROOT}/ran/srsran' '${REMOTE_FREE5GC_ROOT}/scripts'"

rsync -az --delete "${FREE5GC_ROOT}/ran/srsran/" "${REMOTE_HOST}:${REMOTE_FREE5GC_ROOT}/ran/srsran/"
rsync -az \
  "${FREE5GC_ROOT}/scripts/srsran_env.sh" \
  "${FREE5GC_ROOT}/scripts/bootstrap_srsran_local.sh" \
  "${FREE5GC_ROOT}/scripts/sync_srsran_to_remote.sh" \
  "${FREE5GC_ROOT}/scripts/sync_srsran_from_remote.sh" \
  "${FREE5GC_ROOT}/scripts/build_srsran_remote.sh" \
  "${FREE5GC_ROOT}/scripts/run_free5gc_srsran_remote.sh" \
  "${FREE5GC_ROOT}/scripts/run_srsran_gnb_remote.sh" \
  "${REMOTE_HOST}:${REMOTE_FREE5GC_ROOT}/scripts/"

rsync -az \
  "${FREE5GC_ROOT}/config/smfcfg.srsran.yaml" \
  "${FREE5GC_ROOT}/config/upfcfg.srsran.yaml" \
  "${REMOTE_HOST}:${REMOTE_FREE5GC_ROOT}/config/"

rsync -az "${FREE5GC_ROOT}/run.sh" "${REMOTE_HOST}:${REMOTE_FREE5GC_ROOT}/run.sh"
