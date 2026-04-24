#!/usr/bin/env bash

set -euo pipefail

FREE5GC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${SRSRAN_ENV_FILE:-${FREE5GC_ROOT}/ran/srsran/srsran.env}"

if [[ -f "${ENV_FILE}" ]]; then
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
else
  # shellcheck disable=SC1091
  source "${FREE5GC_ROOT}/ran/srsran/srsran.env.example"
fi

REMOTE_HOST="${REMOTE_HOST:-chensb@172.27.33.78}"
REMOTE_FREE5GC_ROOT="${REMOTE_FREE5GC_ROOT:-/data/chensb-data/data-framework/free5gc}"
LOCAL_SRSRAN_ROOT="${LOCAL_SRSRAN_ROOT:-external/srsRAN}"
REMOTE_SRSRAN_ROOT="${REMOTE_SRSRAN_ROOT:-${REMOTE_FREE5GC_ROOT}/external/srsRAN}"
SRSRAN_REPO="${SRSRAN_REPO:-https://github.com/srsran/srsRAN_Project.git}"
SRSRAN_BRANCH="${SRSRAN_BRANCH:-main}"
SRSRAN_BUILD_TYPE="${SRSRAN_BUILD_TYPE:-RelWithDebInfo}"
SRSRAN_CMAKE_ARGS="${SRSRAN_CMAKE_ARGS:--DENABLE_UHD=OFF -DENABLE_ZEROMQ=ON -DENABLE_EXPORT=ON -DENABLE_BACKWARD=OFF -DENABLE_WERROR=OFF}"

LOCAL_SRSRAN_ABS="${FREE5GC_ROOT}/${LOCAL_SRSRAN_ROOT}"
