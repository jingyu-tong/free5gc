#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
ITERATIONS="${ITERATIONS:-30}"
RESULT_TAG="${RESULT_TAG:-$(date +%Y%m%d)-ue-trigger-prepared-latency}"
REMOTE_OUTPUT_ROOT="${REMOTE_OUTPUT_ROOT:-/tmp/data-framework-benchmark/${RESULT_TAG}}"
PREPARED_ROOT="${PREPARED_ROOT:-${ROOT_DIR}/prepared_csi_streams/raw}"
TEST_DIR="${ROOT_DIR}/test"
UE_CONFIG="${TEST_DIR}/ueRanEmulator/config/ueranem.amf-dsmf.yaml"
AMF_TEMPLATE="${ROOT_DIR}/config/amfcfg.dsmf-trigger.yaml"
SCENARIO_FILTER="${SCENARIO_FILTER:-}"

mkdir -p "${REMOTE_OUTPUT_ROOT}/bin"

SCENARIOS=(
  "gesture:GESTURE_RECOGNITION_CSI"
  "localization:POSITIONING_CSI"
  "vehicle:VEHICLE_CSI"
)

RUN_SHELL_PID=""
CURRENT_UE_PID=""

cleanup() {
  if [[ -n "${CURRENT_UE_PID}" ]]; then
    kill -TERM "${CURRENT_UE_PID}" >/dev/null 2>&1 || true
    sleep 1
    kill -KILL "${CURRENT_UE_PID}" >/dev/null 2>&1 || true
    wait "${CURRENT_UE_PID}" >/dev/null 2>&1 || true
    CURRENT_UE_PID=""
  fi
  if [[ -n "${RUN_SHELL_PID}" ]]; then
    if [[ -f "${ROOT_DIR}/run.pid" ]]; then
      kill -INT "$(cat "${ROOT_DIR}/run.pid")" >/dev/null 2>&1 || true
    fi
    force_cleanup_stack
    wait "${RUN_SHELL_PID}" >/dev/null 2>&1 || true
    RUN_SHELL_PID=""
  fi
}

trap cleanup EXIT

build_tools() {
  (cd "${TEST_DIR}" && "${GO_BIN}" build -o "${REMOTE_OUTPUT_ROOT}/bin/seed_ue_subscriber" ./cmd/seed_ue_subscriber)
  (cd "${TEST_DIR}" && "${GO_BIN}" build -o "${REMOTE_OUTPUT_ROOT}/bin/ueRanEmulator" ./ueRanEmulator)
}

make_schedule() {
  local scenario="$1"
  local out_path="$2"
  python3 - "$scenario" "$out_path" "$PREPARED_ROOT" "$ITERATIONS" <<'PY'
import csv
import sys
from pathlib import Path

scenario = sys.argv[1]
out_path = Path(sys.argv[2])
prepared_root = Path(sys.argv[3])
iterations = int(sys.argv[4])

scenario_dir = prepared_root / scenario
rows = []
sequence = 0
for source_file in sorted(scenario_dir.glob("*.tsv")):
    with source_file.open("r", encoding="utf-8") as handle:
        header = handle.readline()
        for line_number, _line in enumerate(handle, start=2):
            sequence += 1
            rows.append({
                "scenario": scenario,
                "sequence_global": sequence,
                "scenario_iteration": len(rows) + 1,
                "source_file": str(source_file),
                "line_number": line_number,
            })
            if len(rows) >= iterations:
                break
    if len(rows) >= iterations:
        break

if len(rows) < iterations:
    raise SystemExit(f"not enough packets for {scenario}: need {iterations}, got {len(rows)}")

out_path.parent.mkdir(parents=True, exist_ok=True)
with out_path.open("w", newline="", encoding="utf-8") as handle:
    writer = csv.DictWriter(
        handle,
        fieldnames=["scenario", "sequence_global", "scenario_iteration", "source_file", "line_number"],
        delimiter="\t",
    )
    writer.writeheader()
    writer.writerows(rows)
PY
}

write_single_packet() {
  local source_file="$1"
  local line_number="$2"
  local output_file="$3"
  python3 - "$source_file" "$line_number" "$output_file" <<'PY'
import sys
from pathlib import Path

source_file = Path(sys.argv[1])
line_number = int(sys.argv[2])
output_file = Path(sys.argv[3])

with source_file.open("r", encoding="utf-8") as src:
    header = src.readline()
    selected = None
    for current, line in enumerate(src, start=2):
        if current == line_number:
            selected = line
            break

if selected is None:
    raise SystemExit(f"line {line_number} not found in {source_file}")

output_file.parent.mkdir(parents=True, exist_ok=True)
with output_file.open("w", encoding="utf-8") as dst:
    dst.write(header)
    dst.write(selected)
PY
}

patch_amf_config() {
  local source_scenario="$1"
  local packet_path="$2"
  local output_cfg="$3"
  python3 - "$AMF_TEMPLATE" "$source_scenario" "$packet_path" "$output_cfg" <<'PY'
import re
import sys
from pathlib import Path

template = Path(sys.argv[1]).read_text(encoding="utf-8")
source_scenario = sys.argv[2]
packet_path = sys.argv[3]
output_cfg = Path(sys.argv[4])

template = re.sub(r'(^\s*sourceScenario:\s*).*$',
                  rf'\1{source_scenario}',
                  template,
                  flags=re.MULTILINE)
template = re.sub(r'(^\s*sourceTypeDetail:\s*).*$',
                  rf'\1ue-trigger-single-packet-{source_scenario.lower()}',
                  template,
                  flags=re.MULTILINE)
template = re.sub(r'(^\s*path:\s*).*$',
                  rf'\1{packet_path}',
                  template,
                  flags=re.MULTILINE)

output_cfg.parent.mkdir(parents=True, exist_ok=True)
output_cfg.write_text(template, encoding="utf-8")
PY
}

start_core() {
  local amf_cfg="$1"
  local core_dir="$2"
  mkdir -p "${core_dir}"
  rm -f "${ROOT_DIR}/run.pid"
  (
    cd "${ROOT_DIR}"
    AMF_CONFIG_PATH="${amf_cfg}" ./run.sh -p "${core_dir}"
  ) > "${core_dir}/run.out" 2>&1 &
  RUN_SHELL_PID=$!

  local deadline=$((SECONDS + 90))
  while (( SECONDS < deadline )); do
    if find "${core_dir}" -name free5gc.log -type f | grep -q .; then
      sleep 5
      return 0
    fi
    sleep 2
  done
  echo "free5gc.log not created under ${core_dir}" >&2
  return 1
}

stop_core() {
  cleanup
}

count_matches() {
  local log_path="$1"
  local pattern="$2"
  if [[ ! -f "${log_path}" ]]; then
    printf '0\n'
    return 0
  fi
  grep -c -- "${pattern}" "${log_path}" || true
}

wait_for_completion() {
  local log_path="$1"
  local baseline_count="$2"
  local scenario="$3"
  local iteration="$4"
  local timeout_seconds="${5:-120}"
  local deadline=$((SECONDS + timeout_seconds))

  while (( SECONDS < deadline )); do
    local current_count
    current_count="$(count_matches "${log_path}" "Triggered DSMF data transfer")"
    if (( current_count > baseline_count )); then
      return 0
    fi
    if [[ -n "${CURRENT_UE_PID}" ]] && ! kill -0 "${CURRENT_UE_PID}" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done

  echo "timed out waiting for DSMF completion: scenario=${scenario} iteration=${iteration}" >&2
  return 1
}

stop_ue_iteration() {
  if [[ -z "${CURRENT_UE_PID}" ]]; then
    return 0
  fi
  if kill -0 "${CURRENT_UE_PID}" >/dev/null 2>&1; then
    kill -TERM "${CURRENT_UE_PID}" >/dev/null 2>&1 || true
    sleep 1
  fi
  if kill -0 "${CURRENT_UE_PID}" >/dev/null 2>&1; then
    kill -KILL "${CURRENT_UE_PID}" >/dev/null 2>&1 || true
  fi
  wait "${CURRENT_UE_PID}" >/dev/null 2>&1 || true
  CURRENT_UE_PID=""
}

force_cleanup_stack() {
  local patterns=(
    "./run.sh -p"
    "./bin/nrf -c"
    "./bin/amf -c"
    "./bin/smf -c"
    "./bin/udr -c"
    "./bin/pcf -c"
    "./bin/udm -c"
    "./bin/nssf -c"
    "./bin/ausf -c"
    "./bin/chf -c"
    "./bin/nef -c"
    "./bin/dsmf -c"
    "./bin/dpf -c"
    "./bin/dsf -c"
  )

  for pattern in "${patterns[@]}"; do
    pkill -f "${pattern}" >/dev/null 2>&1 || true
  done
  sudo pkill -f "./bin/upf -c" >/dev/null 2>&1 || true
  sleep 2
  sudo ip link del upfgtp >/dev/null 2>&1 || true
  rm -f "${ROOT_DIR}/run.pid"
}

scenario_selected() {
  local scenario="$1"
  if [[ -z "${SCENARIO_FILTER}" ]]; then
    return 0
  fi

  local normalized=",${SCENARIO_FILTER// /},"
  [[ "${normalized}" == *",${scenario},"* ]]
}

run_scenario() {
  local scenario="$1"
  local source_scenario="$2"
  local scenario_dir="${REMOTE_OUTPUT_ROOT}/${scenario}"
  local schedule_path="${scenario_dir}/driver_schedule.tsv"
  local packet_path="${scenario_dir}/current_packet.tsv"
  local amf_cfg="${scenario_dir}/amfcfg.yaml"
  local core_dir="${scenario_dir}/core"

  mkdir -p "${scenario_dir}"
  force_cleanup_stack
  make_schedule "${scenario}" "${schedule_path}"
  patch_amf_config "${source_scenario}" "${packet_path}" "${amf_cfg}"
  start_core "${amf_cfg}" "${core_dir}"
  local free5gc_log
  free5gc_log="$(find "${core_dir}" -name free5gc.log -type f | sort | tail -n 1)"
  if [[ -z "${free5gc_log}" ]]; then
    echo "free5gc.log not found under ${core_dir}" >&2
    return 1
  fi

  "${REMOTE_OUTPUT_ROOT}/bin/seed_ue_subscriber" -c "${UE_CONFIG}" > "${scenario_dir}/seed.out" 2>&1

  while IFS=$'\t' read -r row_scenario sequence_global scenario_iteration source_file line_number; do
    write_single_packet "${source_file}" "${line_number}" "${packet_path}"
    local completion_count
    completion_count="$(count_matches "${free5gc_log}" "Triggered DSMF data transfer")"
    "${REMOTE_OUTPUT_ROOT}/bin/ueRanEmulator" -c "${UE_CONFIG}" > "${scenario_dir}/ue_iter_${scenario_iteration}.out" 2>&1 &
    CURRENT_UE_PID=$!
    wait_for_completion "${free5gc_log}" "${completion_count}" "${scenario}" "${scenario_iteration}"
    stop_ue_iteration
    sleep 2
  done < <(tail -n +2 "${schedule_path}")

  stop_core

  mkdir -p "${scenario_dir}/results"
  python3 "${ROOT_DIR}/scripts/ue_trigger_latency_distribution.py" \
    --log "${free5gc_log}" \
    --schedule "${schedule_path}" \
    --output-dir "${scenario_dir}/results"
}

build_tools

for item in "${SCENARIOS[@]}"; do
  scenario="${item%%:*}"
  source_scenario="${item##*:}"
  if ! scenario_selected "${scenario}"; then
    continue
  fi
  run_scenario "${scenario}" "${source_scenario}"
done

printf '%s\n' "${REMOTE_OUTPUT_ROOT}"
