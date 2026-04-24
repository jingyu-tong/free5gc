#!/usr/bin/env python3

from __future__ import annotations

import argparse
import csv
import json
import re
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Iterable


ISO_RE = re.compile(r"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)")
TASK_ID_RE = re.compile(r"taskId=([0-9a-f-]+)")
FIELD_RES = {
    "task_id": re.compile(r'(?:^|[\s"])task_id=([^\s"]+)|(?:^|[\s"])task_id:"([^"]+)"'),
    "orchestration_id": re.compile(r'(?:^|[\s"])orchestration_id=([^\s"]+)|(?:^|[\s"])orchestration_id:"([^"]+)"'),
    "processing_task_id": re.compile(r'(?:^|[\s"])processing_task_id=([^\s"]+)|(?:^|[\s"])processing_task_id:"([^"]+)"'),
    "storage_task_id": re.compile(r'(?:^|[\s"])storage_task_id=([^\s"]+)|(?:^|[\s"])storage_task_id:"([^"]+)"'),
    "transfer_session_id": re.compile(r'(?:^|[\s"])transfer_session_id=([^\s"]+)|(?:^|[\s"])transfer_session_id:"([^"]+)"'),
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


def latest_event(events: list[Event], marker: str) -> Event:
    matches = [event for event in events if event.marker == marker]
    if not matches:
        raise RuntimeError(f"missing marker: {marker}")
    return matches[-1]


def first_after(events: list[Event], marker: str, ts: datetime) -> Event:
    for event in events:
        if event.marker == marker and event.ts >= ts:
            return event
    raise RuntimeError(f"missing marker {marker} after {ts.isoformat()}Z")


def last_before(events: list[Event], marker: str, ts: datetime) -> Event:
    matches = [event for event in events if event.marker == marker and event.ts <= ts]
    if not matches:
        raise RuntimeError(f"missing marker {marker} before {ts.isoformat()}Z")
    return matches[-1]


def first_by(events: list[Event], marker: str, key: str, value: str) -> Event:
    for event in events:
        if event.marker == marker and event.fields.get(key) == value:
            return event
    raise RuntimeError(f"missing marker {marker} with {key}={value}")


def last_by(events: list[Event], marker: str, key: str, value: str) -> Event:
    matches = [event for event in events if event.marker == marker and event.fields.get(key) == value]
    if not matches:
        raise RuntimeError(f"missing marker {marker} with {key}={value}")
    return matches[-1]


def millis(start: datetime, end: datetime) -> float:
    return round((end - start).total_seconds() * 1000.0, 3)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--log", required=True)
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

    amf_dispatch = latest_event(events, "Dispatch DSMF trigger")
    reg = last_before(events, "Handle Registration Request", amf_dispatch.ts)
    dsmf_begin = first_after(events, "DSMF_CREATE_TASK_BEGIN", amf_dispatch.ts)
    task_id = dsmf_begin.fields["task_id"]
    orch_id = dsmf_begin.fields["orchestration_id"]
    amf_done = first_after(events, "Triggered DSMF data transfer", dsmf_begin.ts)
    dsmf_storage_begin = first_by(events, "DSMF_SUBMIT_STORAGE_TASK_BEGIN", "task_id", task_id)
    dsmf_storage_done = first_by(events, "DSMF_SUBMIT_STORAGE_TASK_DONE", "task_id", task_id)
    dsmf_proc_begin = first_by(events, "DSMF_SUBMIT_PROCESSING_TASK_BEGIN", "task_id", task_id)
    dsmf_proc_done = first_by(events, "DSMF_SUBMIT_PROCESSING_TASK_DONE", "task_id", task_id)
    dsmf_proc_report = last_by(events, "DSMF_REPORT_PROCESSING_STATUS", "task_id", task_id)
    dsmf_storage_report = last_by(events, "DSMF_REPORT_STORAGE_STATUS", "task_id", task_id)
    dsmf_complete = first_by(events, "DSMF_CREATE_TASK_COMPLETE", "task_id", task_id)

    dpf_submit = first_by(events, "DPF_SUBMIT_PROCESSING_TASK", "orchestration_id", orch_id)
    dpf_source = first_by(events, "DPF_SOURCE_READY", "orchestration_id", orch_id)
    dpf_result = first_by(events, "DPF_RESULT_READY", "orchestration_id", orch_id)
    dpf_deliver_begin = first_by(events, "DPF_DELIVER_BEGIN", "orchestration_id", orch_id)
    transfer_session_id = dpf_deliver_begin.fields["transfer_session_id"]
    dpf_open = first_by(events, "DPF_OPEN_TRANSFER_ACCEPTED", "transfer_session_id", transfer_session_id)
    dpf_push = first_by(events, "DPF_PUSH_RESULT_COMPLETE", "transfer_session_id", transfer_session_id)
    dpf_close = first_by(events, "DPF_CLOSE_TRANSFER_COMPLETE", "transfer_session_id", transfer_session_id)
    dpf_deliver_done = first_by(events, "DPF_DELIVER_COMPLETE", "transfer_session_id", transfer_session_id)

    dsf_submit = first_by(events, "DSF_SUBMIT_STORAGE_TASK", "orchestration_id", orch_id)
    dsf_open = first_by(events, "DSF_OPEN_TRANSFER_ACCEPTED", "transfer_session_id", transfer_session_id)
    dsf_close = first_by(events, "DSF_CLOSE_TRANSFER_COMPLETE", "transfer_session_id", transfer_session_id)

    stages = [
        ("ue_registration_to_amf_dispatch", reg.ts, amf_dispatch.ts),
        ("amf_dispatch_to_dsmf_begin", amf_dispatch.ts, dsmf_begin.ts),
        ("dsmf_storage_submit_rpc", dsmf_storage_begin.ts, dsmf_storage_done.ts),
        ("dsmf_processing_submit_rpc", dsmf_proc_begin.ts, dsmf_proc_done.ts),
        ("dsf_storage_task_accept", dsmf_storage_begin.ts, dsf_submit.ts),
        ("dpf_task_accept", dsmf_proc_begin.ts, dpf_submit.ts),
        ("dpf_fetch_source", dpf_submit.ts, dpf_source.ts),
        ("dpf_process_and_encode", dpf_source.ts, dpf_result.ts),
        ("dpf_open_transfer_rpc", dpf_deliver_begin.ts, dpf_open.ts),
        ("dsf_open_transfer_accept", dpf_deliver_begin.ts, dsf_open.ts),
        ("dpf_push_result_stream", dpf_open.ts, dpf_push.ts),
        ("dpf_close_transfer_rpc", dpf_push.ts, dpf_close.ts),
        ("dsf_store_result", dsf_open.ts, dsf_close.ts),
        ("dpf_delivery_total", dpf_deliver_begin.ts, dpf_deliver_done.ts),
        ("dsmf_wait_processing_callback", dsmf_proc_done.ts, dsmf_proc_report.ts),
        ("dsmf_wait_storage_callback", dsmf_storage_done.ts, dsmf_storage_report.ts),
        ("dsmf_sync_task_total", dsmf_begin.ts, dsmf_complete.ts),
        ("amf_to_dsmf_sync_total", amf_dispatch.ts, amf_done.ts),
        ("ue_trigger_total_from_registration", reg.ts, amf_done.ts),
    ]

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    rows = []
    for name, start, end in stages:
        rows.append({
            "stage": name,
            "start_utc": start.isoformat() + "Z",
            "end_utc": end.isoformat() + "Z",
            "duration_ms": millis(start, end),
        })

    csv_path = output_dir / "ue_trigger_latency_breakdown.csv"
    with csv_path.open("w", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=["stage", "start_utc", "end_utc", "duration_ms"])
        writer.writeheader()
        writer.writerows(rows)

    summary = {
        "task_id": task_id,
        "orchestration_id": orch_id,
        "transfer_session_id": transfer_session_id,
        "breakdown": rows,
    }
    json_path = output_dir / "ue_trigger_latency_breakdown.json"
    json_path.write_text(json.dumps(summary, indent=2))

    print(json.dumps(summary, indent=2))


if __name__ == "__main__":
    main()
