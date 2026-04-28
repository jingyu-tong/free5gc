#!/usr/bin/env bash

set -euo pipefail

DEV="${NETEM_DEV:-lo}"
LOSS_PERCENT="${LOSS_PERCENT:-0}"
DELAY_MS="${DELAY_MS:-0}"
JITTER_MS="${JITTER_MS:-0}"
PORTS="${NETEM_PORTS:-8071,8072,8073,50073,50074}"

sudo_cmd() {
  if [[ -n "${FREE5GC_SUDO_PASSWORD:-}" ]]; then
    printf '%s\n' "${FREE5GC_SUDO_PASSWORD}" | sudo -S "$@"
  else
    sudo "$@"
  fi
}

clear_netem() {
  sudo_cmd tc qdisc del dev "${DEV}" root >/dev/null 2>&1 || true
}

apply_netem() {
  clear_netem

  if [[ "${LOSS_PERCENT}" == "0" || "${LOSS_PERCENT}" == "0%" ]]; then
    return 0
  fi

  sudo_cmd tc qdisc add dev "${DEV}" root handle 1: prio bands 4
  sudo_cmd tc qdisc add dev "${DEV}" parent 1:4 handle 40: netem loss "${LOSS_PERCENT}%"

  local delay_args=()
  if [[ "${DELAY_MS}" != "0" && "${DELAY_MS}" != "0ms" ]]; then
    delay_args=(delay "${DELAY_MS}")
    if [[ "${JITTER_MS}" != "0" && "${JITTER_MS}" != "0ms" ]]; then
      delay_args+=( "${JITTER_MS}" )
    fi
    sudo_cmd tc qdisc change dev "${DEV}" parent 1:4 handle 40: netem loss "${LOSS_PERCENT}%" "${delay_args[@]}"
  fi

  IFS=',' read -r -a ports <<< "${PORTS// /}"
  for port in "${ports[@]}"; do
    [[ -z "${port}" ]] && continue
    for proto in tcp udp; do
      sudo_cmd tc filter add dev "${DEV}" protocol ip parent 1: prio 1 flower ip_proto "${proto}" dst_port "${port}" flowid 1:4
      sudo_cmd tc filter add dev "${DEV}" protocol ip parent 1: prio 1 flower ip_proto "${proto}" src_port "${port}" flowid 1:4
    done
  done
}

show_netem() {
  sudo_cmd tc qdisc show dev "${DEV}" || true
  sudo_cmd tc filter show dev "${DEV}" parent 1: || true
}

case "${1:-}" in
  apply)
    apply_netem
    show_netem
    ;;
  clear)
    clear_netem
    ;;
  show)
    show_netem
    ;;
  *)
    echo "usage: LOSS_PERCENT=<0-100> $0 {apply|clear|show}" >&2
    exit 2
    ;;
esac
