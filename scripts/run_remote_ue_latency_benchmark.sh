#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-/usr/local/go/bin/go}"
ITERATIONS="${ITERATIONS:-30}"
RESULT_TAG="${RESULT_TAG:-$(date +%Y%m%d)-ue-trigger-prepared-latency}"
DEFAULT_OUTPUT_BASE="/tmp/data-framework-benchmark"
if [[ -d /data/chensb-data ]]; then
  DEFAULT_OUTPUT_BASE="/data/chensb-data/data-framework-benchmark"
fi
REMOTE_OUTPUT_ROOT="${REMOTE_OUTPUT_ROOT:-${DEFAULT_OUTPUT_BASE}/${RESULT_TAG}}"
PREPARED_ROOT="${PREPARED_ROOT:-${ROOT_DIR}/prepared_csi_streams/raw}"
TEST_DIR="${ROOT_DIR}/test"
UE_CONFIG="${TEST_DIR}/ueRanEmulator/config/ueranem.amf-dsmf.yaml"
AMF_TEMPLATE="${ROOT_DIR}/config/amfcfg.dsmf-trigger.yaml"
SCENARIO_FILTER="${SCENARIO_FILTER:-}"
DPF_RAN_INGRESS_URL="${DPF_RAN_INGRESS_URL:-http://127.0.0.32:8071/v1/ran/csi/latest}"
DPF_RAN_INGRESS_HTTP2_URL="${DPF_RAN_INGRESS_HTTP2_URL:-http://127.0.0.32:8071/v1/ran/csi/latest}"
DPF_RAN_INGRESS_HTTP3_URL="${DPF_RAN_INGRESS_HTTP3_URL:-https://127.0.0.32:8072/v1/ran/csi/latest}"
DPF_RAN_INGRESS_QUIC_URL="${DPF_RAN_INGRESS_QUIC_URL:-quic://127.0.0.32:8073/v1/ran/csi/latest}"
TRANSPORT_PROTOCOLS="${TRANSPORT_PROTOCOLS:-HTTP2,HTTP3,QUIC}"
PAYLOAD_PROTOCOLS="${PAYLOAD_PROTOCOLS:-JSON,PROTOBUF}"
PACKET_WINDOW_SIZES="${PACKET_WINDOW_SIZES:-1}"
NETEM_LOSS_PERCENTS="${NETEM_LOSS_PERCENTS:-0}"
NETEM_SCOPE="${NETEM_SCOPE:-data-plane-ports}"

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
  if [[ "${NETEM_ACTIVE:-0}" == "1" ]]; then
    "${ROOT_DIR}/scripts/netem_data_plane.sh" clear >/dev/null 2>&1 || true
    NETEM_ACTIVE=0
  fi
}

trap cleanup EXIT

sudo_cmd() {
  if [[ -n "${FREE5GC_SUDO_PASSWORD:-}" ]]; then
    printf '%s\n' "${FREE5GC_SUDO_PASSWORD}" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

build_tools() {
  (cd "${TEST_DIR}" && "${GO_BIN}" build -o "${REMOTE_OUTPUT_ROOT}/bin/seed_ue_subscriber" ./cmd/seed_ue_subscriber)
  (cd "${TEST_DIR}" && "${GO_BIN}" build -o "${REMOTE_OUTPUT_ROOT}/bin/ueRanEmulator" ./ueRanEmulator)
  (cd "${ROOT_DIR}/NFs/dpf" && "${GO_BIN}" build -o "${REMOTE_OUTPUT_ROOT}/bin/ran_ingress_client" ./cmd/ran_ingress_client)
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

write_packet_window() {
  local source_file="$1"
  local line_number="$2"
  local window_size="$3"
  local output_file="$4"
  python3 - "$source_file" "$line_number" "$window_size" "$output_file" <<'PY'
import sys
from pathlib import Path

source_file = Path(sys.argv[1])
line_number = int(sys.argv[2])
window_size = int(sys.argv[3])
output_file = Path(sys.argv[4])
if window_size < 1:
    raise SystemExit(f"packet window size must be >= 1, got {window_size}")

with source_file.open("r", encoding="utf-8") as src:
    header = src.readline()
    selected = []
    for current, line in enumerate(src, start=2):
        if current < line_number:
            continue
        selected.append(line)
        if len(selected) >= window_size:
            break

if len(selected) < window_size:
    raise SystemExit(
        f"not enough CSI rows in {source_file}: line={line_number} window={window_size} got={len(selected)}"
    )

output_file.parent.mkdir(parents=True, exist_ok=True)
with output_file.open("w", encoding="utf-8") as dst:
    dst.write(header)
    dst.writelines(selected)
PY
}

patch_amf_config() {
  local source_scenario="$1"
  local packet_url="$2"
  local transport_protocol="$3"
  local payload_protocol="$4"
  local output_cfg="$5"
  python3 - "$AMF_TEMPLATE" "$source_scenario" "$packet_url" "$transport_protocol" "$payload_protocol" "$output_cfg" <<'PY'
import re
import sys
from pathlib import Path
from urllib.parse import urlparse

template = Path(sys.argv[1]).read_text(encoding="utf-8")
source_scenario = sys.argv[2]
packet_url = sys.argv[3]
transport_protocol = sys.argv[4]
payload_protocol = sys.argv[5]
output_cfg = Path(sys.argv[6])
parsed = urlparse(packet_url)
if parsed.scheme not in ("http", "https") or not parsed.hostname or not parsed.path:
    raise SystemExit(f"invalid DPF RAN ingress URL: {packet_url}")
port = parsed.port or (443 if parsed.scheme == "https" else 80)

template = re.sub(r'(^\s*sourceScenario:\s*).*$',
                  rf'\1{source_scenario}',
                  template,
                  flags=re.MULTILINE)
template = re.sub(r'(^\s*transportProtocol:\s*).*$',
                  rf'\1{transport_protocol}',
                  template,
                  flags=re.MULTILINE)
template = re.sub(r'(^\s*payloadProtocol:\s*).*$',
                  rf'\1{payload_protocol}',
                  template,
                  flags=re.MULTILINE)
template = re.sub(r'(^\s*sourceTypeDetail:\s*).*$',
                  rf'\1ran-estimated-csi-{source_scenario.lower()}-{transport_protocol.lower()}-{payload_protocol.lower()}',
                  template,
                  flags=re.MULTILINE)
template = re.sub(
    r'(^\s*ingressEndpoint:\n)(?:^\s{8}\S.*\n)+',
    (
        r'\1'
        f'        scheme: {parsed.scheme}\n'
        f'        host: {parsed.hostname}\n'
        f'        port: {port}\n'
        f'        path: {parsed.path}\n'
    ),
    template,
    flags=re.MULTILINE,
)
template = re.sub(r'(^\s*origin:\s*).*$',
                  r'\1ran-dpf-ingress',
                  template,
                  flags=re.MULTILINE)
template = re.sub(r'(^\s*scenario:\s*).*$',
                  rf'\1full-flow-{transport_protocol.lower()}-{payload_protocol.lower()}',
                  template,
                  flags=re.MULTILINE)

output_cfg.parent.mkdir(parents=True, exist_ok=True)
output_cfg.write_text(template, encoding="utf-8")
PY
}

post_ran_packet() {
  local packet_path="$1"
  local transport_protocol="${2:-HTTP2}"
  local url="${DPF_RAN_INGRESS_HTTP2_URL}"
  case "${transport_protocol}" in
    HTTP3) url="${DPF_RAN_INGRESS_HTTP3_URL}" ;;
    QUIC) url="${DPF_RAN_INGRESS_QUIC_URL}" ;;
  esac
  local last_status=0
  for _ in $(seq 1 50); do
    if "${REMOTE_OUTPUT_ROOT}/bin/ran_ingress_client" \
      -protocol "${transport_protocol}" \
      -url "${url}" \
      -packet-id latest \
      -input "${packet_path}" \
      -timeout 2s; then
      return 0
    fi
    last_status=$?
    sleep 0.2
  done
  echo "failed to send RAN CSI packet over ${transport_protocol} to ${url}" >&2
  return "${last_status}"
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
  sudo_cmd pkill -f "./bin/upf -c" >/dev/null 2>&1 || true
  sleep 2
  sudo_cmd ip link del upfgtp >/dev/null 2>&1 || true
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

parse_csv_list() {
  local raw="$1"
  local -n out_ref="$2"
  IFS=',' read -r -a out_ref <<< "${raw// /}"
}

run_scenario() {
  local scenario="$1"
  local source_scenario="$2"
  local transport_protocol="$3"
  local payload_protocol="$4"
  local packet_window_size="$5"
  local loss_percent="$6"
  local protocol="${transport_protocol}-${payload_protocol}"
  local scenario_dir="${REMOTE_OUTPUT_ROOT}/loss-${loss_percent}/${scenario}/packet-window-${packet_window_size}/${protocol}"
  local schedule_path="${scenario_dir}/driver_schedule.tsv"
  local packet_path="${scenario_dir}/current_packet.tsv"
  local amf_cfg="${scenario_dir}/amfcfg.yaml"
  local core_dir="${scenario_dir}/core"

  mkdir -p "${scenario_dir}"
  force_cleanup_stack
  make_schedule "${scenario}" "${schedule_path}"
  python3 - "${schedule_path}" "${transport_protocol}" "${payload_protocol}" "${packet_window_size}" "${loss_percent}" "${NETEM_SCOPE}" <<'PY'
import csv
import sys
from pathlib import Path

path = Path(sys.argv[1])
transport_protocol = sys.argv[2]
payload_protocol = sys.argv[3]
packet_window_size = sys.argv[4]
loss_percent = sys.argv[5]
netem_scope = sys.argv[6]
rows = []
with path.open("r", encoding="utf-8", newline="") as handle:
    reader = csv.DictReader(handle, delimiter="\t")
    fieldnames = list(reader.fieldnames or [])
    for row in reader:
        row["transport_protocol"] = transport_protocol
        row["payload_protocol"] = payload_protocol
        row["protocol"] = f"{transport_protocol}/{payload_protocol}"
        row["packet_window_size"] = packet_window_size
        row["loss_percent"] = loss_percent
        row["netem_scope"] = netem_scope
        rows.append(row)

for name in ["transport_protocol", "payload_protocol", "protocol", "packet_window_size", "loss_percent", "netem_scope"]:
    if name not in fieldnames:
        fieldnames.append(name)

with path.open("w", encoding="utf-8", newline="") as handle:
    writer = csv.DictWriter(handle, fieldnames=fieldnames, delimiter="\t")
    writer.writeheader()
    writer.writerows(rows)
PY
  patch_amf_config "${source_scenario}" "${DPF_RAN_INGRESS_URL}" "${transport_protocol}" "${payload_protocol}" "${amf_cfg}"
  start_core "${amf_cfg}" "${core_dir}"
  local free5gc_log
  free5gc_log="$(find "${core_dir}" -name free5gc.log -type f | sort | tail -n 1)"
  if [[ -z "${free5gc_log}" ]]; then
    echo "free5gc.log not found under ${core_dir}" >&2
    return 1
  fi

  "${REMOTE_OUTPUT_ROOT}/bin/seed_ue_subscriber" -c "${UE_CONFIG}" > "${scenario_dir}/seed.out" 2>&1

  while IFS=$'\t' read -r row_scenario sequence_global scenario_iteration source_file line_number row_transport row_payload row_protocol row_packet_window_size row_loss_percent row_netem_scope; do
    write_packet_window "${source_file}" "${line_number}" "${packet_window_size}" "${packet_path}"
    post_ran_packet "${packet_path}" "${transport_protocol}"
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

TRANSPORT_LIST=()
PAYLOAD_LIST=()
PACKET_WINDOW_LIST=()
LOSS_LIST=()
parse_csv_list "${TRANSPORT_PROTOCOLS}" TRANSPORT_LIST
parse_csv_list "${PAYLOAD_PROTOCOLS}" PAYLOAD_LIST
parse_csv_list "${PACKET_WINDOW_SIZES}" PACKET_WINDOW_LIST
parse_csv_list "${NETEM_LOSS_PERCENTS}" LOSS_LIST

NETEM_ACTIVE=0

for loss_percent in "${LOSS_LIST[@]}"; do
  "${ROOT_DIR}/scripts/netem_data_plane.sh" clear >/dev/null 2>&1 || true
  NETEM_ACTIVE=0
  if [[ "${loss_percent}" != "0" && "${loss_percent}" != "0%" ]]; then
    LOSS_PERCENT="${loss_percent%%%}" "${ROOT_DIR}/scripts/netem_data_plane.sh" apply
    NETEM_ACTIVE=1
  fi
  for item in "${SCENARIOS[@]}"; do
    scenario="${item%%:*}"
    source_scenario="${item##*:}"
    if ! scenario_selected "${scenario}"; then
      continue
    fi
    for packet_window_size in "${PACKET_WINDOW_LIST[@]}"; do
      for transport_protocol in "${TRANSPORT_LIST[@]}"; do
        for payload_protocol in "${PAYLOAD_LIST[@]}"; do
          run_scenario "${scenario}" "${source_scenario}" "${transport_protocol}" "${payload_protocol}" "${packet_window_size}" "${loss_percent%%%}"
        done
      done
    done
  done
done

"${ROOT_DIR}/scripts/netem_data_plane.sh" clear >/dev/null 2>&1 || true
NETEM_ACTIVE=0

printf '%s\n' "${REMOTE_OUTPUT_ROOT}"
