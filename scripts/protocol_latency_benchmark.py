#!/usr/bin/env python3

import argparse
import csv
import hashlib
import json
import math
import os
import pathlib
import statistics
import time
import urllib.error
import urllib.request


REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent
DEFAULT_BASE_URL = "http://127.0.0.31:8010/ndsmf-data-service/v1/tasks"
DEFAULT_SOURCE_PATH = pathlib.Path("/tmp/data-framework-test/source.json")


def percentile(values, pct):
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


def resolve_result_path(result_uri):
    if not result_uri.startswith("file://"):
        return None

    raw_path = result_uri.replace("file://", "", 1)
    candidate = pathlib.Path(raw_path)
    if candidate.is_absolute():
        return candidate
    return REPO_ROOT / candidate


def build_request(protocol, source_path, sequence, task_timeout_seconds):
    return {
        "requestId": f"benchmark-{protocol.lower()}-{sequence}-{time.time_ns()}",
        "resultMode": "sync",
        "transportProtocol": "HTTP2",
        "payloadProtocol": protocol,
        "dataSource": {
            "sourceId": f"benchmark-source-{protocol.lower()}",
            "sourceCategory": "SENSING_CSI",
            "sourceScenario": "BREATHING_CSI",
            "sourceTypeDetail": "latency-benchmark-json-file",
            "ingressEndpoint": {
                "scheme": "file",
                "path": str(source_path),
            },
            "encoding": "json",
        },
        "processingSteps": [
            {
                "name": "passthrough",
                "version": "v1",
                "parameters": [
                    {"key": "extract", "value": "all"},
                    {"key": "protocol", "value": protocol.lower()},
                    {"key": "mode", "value": "latency-benchmark"},
                ],
            }
        ],
        "outputSchema": "demo.v1.result",
        "taskTimeoutSeconds": task_timeout_seconds,
        "labels": {
            "test": "latency-distribution",
            "payload": protocol.lower(),
        },
    }


def issue_request(base_url, payload):
    request = urllib.request.Request(
        base_url,
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    started = time.perf_counter_ns()
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            raw = response.read().decode("utf-8")
            elapsed_ms = (time.perf_counter_ns() - started) / 1_000_000
            return response.status, json.loads(raw), elapsed_ms
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8")
        elapsed_ms = (time.perf_counter_ns() - started) / 1_000_000
        return exc.code, json.loads(raw), elapsed_ms


def raw_fieldnames():
    return [
        "protocol",
        "phase",
        "sequence",
        "request_id",
        "http_status",
        "latency_ms",
        "task_id",
        "orchestration_id",
        "state",
        "processing_state",
        "storage_state",
        "result_uri",
        "result_exists",
        "result_size_bytes",
        "result_sha256",
        "error",
    ]


def summary_fieldnames():
    return [
        "protocol",
        "measured_requests",
        "successful_requests",
        "failed_requests",
        "min_ms",
        "max_ms",
        "avg_ms",
        "p50_ms",
        "p90_ms",
        "p95_ms",
        "p99_ms",
        "stddev_ms",
    ]


def main():
    parser = argparse.ArgumentParser(description="Benchmark JSON vs PROTOBUF latency distribution for DSMF tasks")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL)
    parser.add_argument("--iterations", type=int, default=30)
    parser.add_argument("--warmup", type=int, default=3)
    parser.add_argument("--task-timeout-seconds", type=int, default=60)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--source-path", default=str(DEFAULT_SOURCE_PATH))
    args = parser.parse_args()

    output_dir = pathlib.Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    source_path = pathlib.Path(args.source_path)
    source_path.parent.mkdir(parents=True, exist_ok=True)
    source_path.write_text(
        json.dumps(
            {
                "sensor": "csi",
                "samples": list(range(1, 9)),
                "ts": "2026-04-23T09:00:00Z",
                "tag": "latency-distribution",
            }
        )
        + "\n",
        encoding="utf-8",
    )

    raw_rows = []
    summary_rows = []

    for protocol in ("JSON", "PROTOBUF"):
        measured_latencies = []
        failures = 0
        sequence = 0

        for phase, total in (("warmup", args.warmup), ("measure", args.iterations)):
            for _ in range(total):
                sequence += 1
                payload = build_request(protocol, source_path, sequence, args.task_timeout_seconds)
                status, body, latency_ms = issue_request(args.base_url, payload)
                task = body.get("task", body)
                result_uri = task.get("resultUri", "")
                result_path = resolve_result_path(result_uri) if result_uri else None
                result_exists = bool(result_path and result_path.exists())
                result_size = result_path.stat().st_size if result_exists else 0
                result_sha256 = ""
                if result_exists:
                    result_sha256 = hashlib.sha256(result_path.read_bytes()).hexdigest()

                row = {
                    "protocol": protocol,
                    "phase": phase,
                    "sequence": sequence,
                    "request_id": payload["requestId"],
                    "http_status": status,
                    "latency_ms": f"{latency_ms:.3f}",
                    "task_id": task.get("taskId", ""),
                    "orchestration_id": task.get("orchestrationId", ""),
                    "state": task.get("state", ""),
                    "processing_state": task.get("processingState", ""),
                    "storage_state": task.get("storageState", ""),
                    "result_uri": result_uri,
                    "result_exists": str(result_exists).lower(),
                    "result_size_bytes": result_size,
                    "result_sha256": result_sha256,
                    "error": body.get("error", ""),
                }
                raw_rows.append(row)

                if phase != "measure":
                    continue

                if status == 200 and task.get("state") == "COMPLETED":
                    measured_latencies.append(latency_ms)
                else:
                    failures += 1

        summary_rows.append(
            {
                "protocol": protocol,
                "measured_requests": args.iterations,
                "successful_requests": len(measured_latencies),
                "failed_requests": failures,
                "min_ms": f"{min(measured_latencies):.3f}" if measured_latencies else "",
                "max_ms": f"{max(measured_latencies):.3f}" if measured_latencies else "",
                "avg_ms": f"{statistics.mean(measured_latencies):.3f}" if measured_latencies else "",
                "p50_ms": f"{percentile(measured_latencies, 0.50):.3f}" if measured_latencies else "",
                "p90_ms": f"{percentile(measured_latencies, 0.90):.3f}" if measured_latencies else "",
                "p95_ms": f"{percentile(measured_latencies, 0.95):.3f}" if measured_latencies else "",
                "p99_ms": f"{percentile(measured_latencies, 0.99):.3f}" if measured_latencies else "",
                "stddev_ms": f"{statistics.pstdev(measured_latencies):.3f}" if measured_latencies else "",
            }
        )

    raw_csv = output_dir / "protocol_latency_raw.csv"
    with raw_csv.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=raw_fieldnames())
        writer.writeheader()
        writer.writerows(raw_rows)

    summary_csv = output_dir / "protocol_latency_summary.csv"
    with summary_csv.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=summary_fieldnames())
        writer.writeheader()
        writer.writerows(summary_rows)

    summary_json = output_dir / "protocol_latency_summary.json"
    summary_json.write_text(json.dumps(summary_rows, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

    print(raw_csv)
    print(summary_csv)
    print(summary_json)
    print(json.dumps(summary_rows, ensure_ascii=False))


if __name__ == "__main__":
    main()
