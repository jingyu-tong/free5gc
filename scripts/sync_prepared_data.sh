#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_DIR="${ROOT_DIR}/prepared_csi_streams/"
REMOTE_USER="${REMOTE_USER:-chensb}"
REMOTE_HOST="${REMOTE_HOST:-172.27.33.78}"
REMOTE_ROOT="${REMOTE_ROOT:-/data/chensb-data/data-framework/free5gc}"
REMOTE_DIR="${REMOTE_ROOT}/prepared_csi_streams/"

if [[ ! -d "${LOCAL_DIR}" ]]; then
  echo "missing local directory: ${LOCAL_DIR}" >&2
  exit 1
fi

ssh "${REMOTE_USER}@${REMOTE_HOST}" "mkdir -p '${REMOTE_DIR}'"
rsync -av --delete "${LOCAL_DIR}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}"

echo "synced ${LOCAL_DIR} -> ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}"
