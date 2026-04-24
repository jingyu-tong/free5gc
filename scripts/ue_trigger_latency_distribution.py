#!/usr/bin/env python3

from __future__ import annotations

import argparse
import csv
import json
import math
import re
import statistics
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Iterable


ISO_RE = re.compile(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)")
TASK_ID_RE = re.compile(r"taskId=([0-9a-f-]+)")
FIELD_RES = {
    "task_id": re.compile(r'(?:^|[\s"])task_id=(?:"([^"]+)"|([^\s"]+))|(?:^|[\s"])task_id:"([^"]+)"'),
    "orchestration_id": re.compile(r'(?:^|[\s"])orchestration_id=(?:"([^"]+)"|([^\s"]+))|(?:^|[\s"])orchestration_id:"([^"]+)"'),
    "processing_task_id": re.compile(r'(?:^|[\s"])processing_task_id=(?:"([^"]+)"|([^\s"]+))|(?:^|[\s"])processing_task_id:"([^"]+)"'),
    "storage_task_id": re.compile(r'(?:^|[\s"])storage_task_id=(?:"([^"]+)"|([^\s"]+))|(?:^|[\s"])storage_task_id:"([^"]+)"'),
    "transfer_session_id": re.compile(r'(?:^|[\s"])transfer_session_id=(?:"([^"]+)"|([^\s"]+))|(?:^|[\s"])transfer_session_id:"([^"]+)"'),
}


@dataclass
class Event:
    ts: datetime
    marker: str
    raw: str
    fields: dict[str, str]


def parse_timestamp(line: str) -> datetime | None:
    match = ISO_RE.search(line)
    if not match:
        return None
    value = match.group(1)
    if "." in value:
        prefix, frac = value[:-1].split(".", 1)
        value = f"{prefix}.{frac[:6].ljust(6, '0')}Z"
    return datetime.strptime(value, "%Y-%m-%dT%H:%M:%S.%fZ" if "." in value else "%Y-%m-%dT%H:%M:%SZ")


def parse_fields(line: str) -> dict[str, str]:
    fields: dict[str, str] = {}
    for key, pattern in FIELD_RES.items():
        match = pattern.search(line)
        if match:
            fields[key] = next(group for group in match.groups() if group)
    task_match = TASK_ID_RE.search(line)
    if task_match:
        fields["task_id_from_amf"] = task_match.group(1)
    return fields


def load_events(path: Path, markers: Iterable[str]) -> list[Event]:
    wanted = tuple(markers)
    events: list[Event] = []
    for line in path.read_text(errors="ignore").splitlines():
        if not any(marker in line for marker in wanted):
            continue
        ts = parse_timestamp(line)
        if ts is None:
            continue
        marker = next(marker for marker in wanted if marker in line)
        events.append(Event(ts=ts, marker=marker, raw=line, fields=parse_fields(line)))
    return events


def millis(start: datetime, end: datetime) -> float:
    return round((end - start).total_seconds() * 1000.0, 3)


def percentile(values: list[float], pct: float) -> float | None:
    if not values:
        return None
    if len(values) == 1:
        return float(values[0])
    ordered = sorted(values)
    rank = (len(ordered) - 1) * pct
    lower = math.floor(rank)
    upper = math.ceil(rank)
    if lower == upper:
        return float(ordered[lower])
    weight = rank - lower
    return float(ordered[lower] * (1 - weight) + ordered[upper] * weight)


def index_first_by(events: list[Event], marker: str, key: str, value: str, start_idx: int = 0) -> int:
    for idx in range(start_idx, len(events)):
        event = events[idx]
        if event.marker == marker and event.fields.get(key) == value:
            return idx
    raise RuntimeError(f"missing marker {marker} with {key}={value}")


def index_last_by(events: list[Event], marker: str, key: str, value: str) -> int:
    for idx in range(len(events) - 1, -1, -1):
        event = events[idx]
        if event.marker == marker and event.fields.get(key) == value:
            return idx
    raise RuntimeError(f"missing marker {marker} with {key}={value}")


def build_records(events: list[Event]) -> list[dict[str, object]]:
    dispatch_indices = [idx for idx, event in enumerate(events) if event.marker == "Dispatch DSMF trigger"]
    records: list[dict[str, object]] = []

    for request_index, dispatch_idx in enumerate(dispatch_indices, start=1):
        dispatch = events[dispatch_idx]

        reg_idx = None
        for idx in range(dispatch_idx - 1, -1, -1):
            if events[idx].marker == "Handle Registration Request":
                reg_idx = idx
                break
        if reg_idx is None:
            raise RuntimeError(f"missing registration request before dispatch #{request_index}")
        reg = events[reg_idx]

        done_idx = None
        for idx in range(dispatch_idx + 1, len(events)):
            if events[idx].marker == "Triggered DSMF data transfer":
                done_idx = idx
                break
        if done_idx is None:
            raise RuntimeError(f"missing AMF completion after dispatch #{request_index}")
        amf_done = events[done_idx]
        task_id = amf_done.fields.get("task_id_from_amf")
        if not task_id:
            raise RuntimeError(f"missing task id in AMF completion for dispatch #{request_index}")

        dsmf_begin = events[index_first_by(events, "DSMF_CREATE_TASK_BEGIN", "task_id", task_id, dispatch_idx)]
        orch_id = dsmf_begin.fields["orchestration_id"]

        dsmf_storage_begin = events[index_first_by(events, "DSMF_SUBMIT_STORAGE_TASK_BEGIN", "task_id", task_id, dispatch_idx)]
        dsmf_storage_done = events[index_first_by(events, "DSMF_SUBMIT_STORAGE_TASK_DONE", "task_id", task_id, dispatch_idx)]
        dsmf_proc_begin = events[index_first_by(events, "DSMF_SUBMIT_PROCESSING_TASK_BEGIN", "task_id", task_id, dispatch_idx)]
        dsmf_proc_done = events[index_first_by(events, "DSMF_SUBMIT_PROCESSING_TASK_DONE", "task_id", task_id, dispatch_idx)]
        dsmf_proc_report = events[index_last_by(events, "DSMF_REPORT_PROCESSING_STATUS", "task_id", task_id)]
        dsmf_storage_report = events[index_last_by(events, "DSMF_REPORT_STORAGE_STATUS", "task_id", task_id)]
        dsmf_complete = events[index_first_by(events, "DSMF_CREATE_TASK_COMPLETE", "task_id", task_id, dispatch_idx)]

        dpf_submit = events[index_first_by(events, "DPF_SUBMIT_PROCESSING_TASK", "orchestration_id", orch_id, dispatch_idx)]
        dpf_source = events[index_first_by(events, "DPF_SOURCE_READY", "orchestration_id", orch_id, dispatch_idx)]
        dpf_result = events[index_first_by(events, "DPF_RESULT_READY", "orchestration_id", orch_id, dispatch_idx)]
        dpf_deliver_begin = events[index_first_by(events, "DPF_DELIVER_BEGIN", "orchestration_id", orch_id, dispatch_idx)]
        transfer_session_id = dpf_deliver_begin.fields["transfer_session_id"]
        dpf_open = events[index_first_by(events, "DPF_OPEN_TRANSFER_ACCEPTED", "transfer_session_id", transfer_session_id, dispatch_idx)]
        dpf_push = events[index_first_by(events, "DPF_PUSH_RESULT_COMPLETE", "transfer_session_id", transfer_session_id, dispatch_idx)]
        dpf_close = events[index_first_by(events, "DPF_CLOSE_TRANSFER_COMPLETE", "transfer_session_id", transfer_session_id, dispatch_idx)]
        dpf_deliver_done = events[index_first_by(events, "DPF_DELIVER_COMPLETE", "transfer_session_id", transfer_session_id, dispatch_idx)]

        dsf_submit = events[index_first_by(events, "DSF_SUBMIT_STORAGE_TASK", "orchestration_id", orch_id, dispatch_idx)]
        dsf_open = events[index_first_by(events, "DSF_OPEN_TRANSFER_ACCEPTED", "transfer_session_id", transfer_session_id, dispatch_idx)]
        dsf_close = events[index_first_by(events, "DSF_CLOSE_TRANSFER_COMPLETE", "transfer_session_id", transfer_session_id, dispatch_idx)]

        records.append(
            {
                "request_index": request_index,
                "task_id": task_id,
                "orchestration_id": orch_id,
                "transfer_session_id": transfer_session_id,
                "ue_registration_to_amf_dispatch_ms": millis(reg.ts, dispatch.ts),
                "amf_dispatch_to_dsmf_begin_ms": millis(dispatch.ts, dsmf_begin.ts),
                "dsmf_storage_submit_rpc_ms": millis(dsmf_storage_begin.ts, dsmf_storage_done.ts),
                "dsmf_processing_submit_rpc_ms": millis(dsmf_proc_begin.ts, dsmf_proc_done.ts),
                "dsf_storage_task_accept_ms": millis(dsmf_storage_begin.ts, dsf_submit.ts),
                "dpf_task_accept_ms": millis(dsmf_proc_begin.ts, dpf_submit.ts),
                "dpf_fetch_source_ms": millis(dpf_submit.ts, dpf_source.ts),
                "dpf_process_and_encode_ms": millis(dpf_source.ts, dpf_result.ts),
                "dpf_open_transfer_rpc_ms": millis(dpf_deliver_begin.ts, dpf_open.ts),
                "dsf_open_transfer_accept_ms": millis(dpf_deliver_begin.ts, dsf_open.ts),
                "dpf_push_result_stream_ms": millis(dpf_open.ts, dpf_push.ts),
                "dpf_close_transfer_rpc_ms": millis(dpf_push.ts, dpf_close.ts),
                "dsf_store_result_ms": millis(dsf_open.ts, dsf_close.ts),
                "dpf_delivery_total_ms": millis(dpf_deliver_begin.ts, dpf_deliver_done.ts),
                "dsmf_wait_processing_callback_ms": millis(dsmf_proc_done.ts, dsmf_proc_report.ts),
                "dsmf_wait_storage_callback_ms": millis(dsmf_storage_done.ts, dsmf_storage_report.ts),
                "dsmf_sync_task_total_ms": millis(dsmf_begin.ts, dsmf_complete.ts),
                "amf_to_dsmf_sync_total_ms": millis(dispatch.ts, amf_done.ts),
                "ue_trigger_total_from_registration_ms": millis(reg.ts, amf_done.ts),
            }
        )

    return records


def load_schedule(path: Path) -> list[dict[str, str]]:
    with path.open("r", encoding="utf-8") as handle:
        return list(csv.DictReader(handle, delimiter="\t"))


def write_csv(path: Path, fieldnames: list[str], rows: list[dict[str, object]]) -> None:
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--log", required=True)
    parser.add_argument("--schedule", required=True)
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()

    markers = [
        "Handle Registration Request",
        "Dispatch DSMF trigger",
        "Triggered DSMF data transfer",
        "DSMF_CREATE_TASK_BEGIN",
        "DSMF_SUBMIT_STORAGE_TASK_BEGIN",
        "DSMF_SUBMIT_STORAGE_TASK_DONE",
        "DSMF_SUBMIT_PROCESSING_TASK_BEGIN",
        "DSMF_SUBMIT_PROCESSING_TASK_DONE",
        "DSMF_REPORT_PROCESSING_STATUS",
        "DSMF_REPORT_STORAGE_STATUS",
        "DSMF_CREATE_TASK_COMPLETE",
        "DSF_SUBMIT_STORAGE_TASK",
        "DSF_OPEN_TRANSFER_ACCEPTED",
        "DSF_CLOSE_TRANSFER_COMPLETE",
        "DPF_SUBMIT_PROCESSING_TASK",
        "DPF_SOURCE_READY",
        "DPF_RESULT_READY",
        "DPF_DELIVER_BEGIN",
        "DPF_OPEN_TRANSFER_ACCEPTED",
        "DPF_PUSH_RESULT_COMPLETE",
        "DPF_CLOSE_TRANSFER_COMPLETE",
        "DPF_DELIVER_COMPLETE",
    ]
    events = load_events(Path(args.log), markers)
    parsed = build_records(events)
    schedule_rows = load_schedule(Path(args.schedule))
    if len(parsed) != len(schedule_rows):
        raise RuntimeError(f"schedule rows {len(schedule_rows)} != parsed requests {len(parsed)}")

    merged_rows: list[dict[str, object]] = []
    for schedule, parsed_row in zip(schedule_rows, parsed):
        merged = dict(schedule)
        merged.update(parsed_row)
        merged_rows.append(merged)

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    raw_fieldnames = list(merged_rows[0].keys()) if merged_rows else []
    write_csv(output_dir / "ue_latency_raw.csv", raw_fieldnames, merged_rows)

    total_metric_names = [
        "ue_trigger_total_from_registration_ms",
        "amf_to_dsmf_sync_total_ms",
        "dsmf_sync_task_total_ms",
        "dpf_delivery_total_ms",
    ]
    summary_rows: list[dict[str, object]] = []
    scenario_groups = sorted({row["scenario"] for row in merged_rows})
    for scenario in scenario_groups:
        scenario_rows = [row for row in merged_rows if row["scenario"] == scenario]
        for metric in total_metric_names:
            values = [float(row[metric]) for row in scenario_rows]
            summary_rows.append(
                {
                    "scenario": scenario,
                    "metric": metric,
                    "samples": len(values),
                    "min_ms": f"{min(values):.3f}",
                    "max_ms": f"{max(values):.3f}",
                    "avg_ms": f"{statistics.mean(values):.3f}",
                    "p50_ms": f"{percentile(values, 0.50):.3f}",
                    "p90_ms": f"{percentile(values, 0.90):.3f}",
                    "p95_ms": f"{percentile(values, 0.95):.3f}",
                    "p99_ms": f"{percentile(values, 0.99):.3f}",
                    "stddev_ms": f"{statistics.pstdev(values):.3f}" if len(values) > 1 else "0.000",
                }
            )
    write_csv(
        output_dir / "ue_latency_summary.csv",
        ["scenario", "metric", "samples", "min_ms", "max_ms", "avg_ms", "p50_ms", "p90_ms", "p95_ms", "p99_ms", "stddev_ms"],
        summary_rows,
    )

    stage_metric_names = [
        key for key in merged_rows[0].keys()
        if key.endswith("_ms") and key not in total_metric_names
    ] if merged_rows else []
    stage_summary_rows: list[dict[str, object]] = []
    for scenario in scenario_groups:
        scenario_rows = [row for row in merged_rows if row["scenario"] == scenario]
        for metric in stage_metric_names + total_metric_names:
            values = [float(row[metric]) for row in scenario_rows]
            stage_summary_rows.append(
                {
                    "scenario": scenario,
                    "stage": metric,
                    "avg_ms": f"{statistics.mean(values):.3f}",
                }
            )
    write_csv(output_dir / "ue_latency_stage_summary.csv", ["scenario", "stage", "avg_ms"], stage_summary_rows)

    summary_json = {
        "requests": merged_rows,
        "summary": summary_rows,
        "stage_summary": stage_summary_rows,
    }
    (output_dir / "ue_latency_summary.json").write_text(json.dumps(summary_json, indent=2), encoding="utf-8")


if __name__ == "__main__":
    main()
