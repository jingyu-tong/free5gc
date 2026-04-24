#!/usr/bin/env python3
"""Prepare line-oriented CSI streams for downstream streaming tests.

This script normalizes the datasets under ``data/`` into a consistent
``time x csi_features`` text format.

Supported sources:
1. Intel 5300 CSI logs in ``data/gesture/*.dat`` and ``data/vehicle/*.dat``
2. Localization snapshots in ``data/localization/s01.json``

Each stream is exported as one TSV text file so downstream experiments can
read one line at a time and simulate real-time acquisition without first
encoding the data as JSON or protobuf.
"""

from __future__ import annotations

import argparse
import json
import re
import statistics
from collections import Counter, defaultdict
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterator, List, Optional, Sequence, Tuple


INTEL_CODE = 187
LOCALIZATION_TIME_FORMAT = "%Y-%m-%d %H:%M:%S:%f"


@dataclass
class IntelFrame:
    frame_idx: int
    timestamp_low: int
    bfee_count: int
    nrx: int
    ntx: int
    rssi: List[int]
    noise: int
    agc: int
    perm: List[int]
    rate: int
    real: List[int]
    imag: List[int]


def signed_int8(value: int) -> int:
    return value - 256 if value >= 128 else value


def sanitize_name(name: str) -> str:
    return re.sub(r"[^a-zA-Z0-9._-]+", "_", name)


def ensure_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)


def write_kv_text(path: Path, items: Sequence[Tuple[str, object]]) -> None:
    lines = [f"{key}={value}" for key, value in items]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def tsv_escape(value: object) -> str:
    text = str(value)
    return text.replace("\t", " ").replace("\n", " ")


def write_tsv(path: Path, header: Sequence[str], rows: Iterator[Sequence[object]]) -> None:
    with path.open("w", encoding="utf-8") as f:
        f.write("\t".join(header) + "\n")
        for row in rows:
            f.write("\t".join(tsv_escape(cell) for cell in row) + "\n")


def parse_datetime_to_epoch_us(value: str) -> int:
    dt = datetime.strptime(value, LOCALIZATION_TIME_FORMAT)
    return int(dt.timestamp() * 1_000_000)


def positive_diffs(values: Sequence[int]) -> List[int]:
    diffs: List[int] = []
    for a, b in zip(values, values[1:]):
        if b >= a:
            diffs.append(b - a)
    return [item for item in diffs if item > 0]


def estimate_hz_from_timestamps(timestamps: Sequence[int]) -> Optional[float]:
    diffs = positive_diffs(timestamps)
    if not diffs:
        return None
    median_us = statistics.median(diffs)
    if median_us <= 0:
        return None
    return 1_000_000.0 / median_us


def unwrap_uint_timestamps(timestamps: Sequence[int], bits: int = 32) -> List[int]:
    if not timestamps:
        return []
    modulus = 1 << bits
    offset = 0
    unwrapped = [timestamps[0]]
    previous = timestamps[0]
    for current in timestamps[1:]:
        if current < previous:
            offset += modulus
        unwrapped.append(current + offset)
        previous = current
    return unwrapped


def build_link_feature_names(nrx: int, ntx: int, subcarrier_count: int = 30) -> List[str]:
    features: List[str] = []
    for subcarrier in range(subcarrier_count):
        for rx in range(1, nrx + 1):
            for tx in range(1, ntx + 1):
                features.append(f"sc{subcarrier:02d}_rx{rx}_tx{tx}")
    return features


def intel_csi_vector(payload: bytes, nrx: int, ntx: int, perm: Optional[List[int]] = None) -> Tuple[List[int], List[int]]:
    calc_len = (30 * (nrx * ntx * 8 * 2 + 3) + 7) // 8
    if len(payload) < calc_len:
        raise ValueError("payload shorter than expected CSI matrix")

    raw_real: List[List[int]] = []
    raw_imag: List[List[int]] = []
    bit_index = 0

    for _subcarrier in range(30):
        bit_index += 3
        remainder = bit_index % 8
        sub_real: List[int] = []
        sub_imag: List[int] = []
        for _link in range(nrx * ntx):
            byte_index = bit_index // 8
            real = ((payload[byte_index] >> remainder) | (payload[byte_index + 1] << (8 - remainder))) & 0xFF
            imag = ((payload[byte_index + 1] >> remainder) | (payload[byte_index + 2] << (8 - remainder))) & 0xFF
            sub_real.append(signed_int8(real))
            sub_imag.append(signed_int8(imag))
            bit_index += 16
        raw_real.append(sub_real)
        raw_imag.append(sub_imag)

    return apply_rx_permutation(raw_real, raw_imag, nrx, ntx, perm)


def apply_rx_permutation(
    raw_real: List[List[int]],
    raw_imag: List[List[int]],
    nrx: int,
    ntx: int,
    perm: Optional[List[int]] = None,
) -> Tuple[List[int], List[int]]:
    if perm is None:
        perm = list(range(1, nrx + 1))

    triangle = {1: 1, 2: 3, 3: 6}
    use_perm = nrx > 1 and sum(perm[:nrx]) == triangle.get(nrx, -1)

    flattened_real: List[int] = []
    flattened_imag: List[int] = []

    for subcarrier in range(30):
        ordered_real = [[0 for _tx in range(ntx)] for _rx in range(nrx)]
        ordered_imag = [[0 for _tx in range(ntx)] for _rx in range(nrx)]

        for link in range(nrx * ntx):
            tx_idx = link % ntx
            rx_idx = link // ntx
            target_rx = perm[rx_idx] - 1 if use_perm else rx_idx
            if 0 <= target_rx < nrx and 0 <= tx_idx < ntx:
                ordered_real[target_rx][tx_idx] = raw_real[subcarrier][link]
                ordered_imag[target_rx][tx_idx] = raw_imag[subcarrier][link]

        for rx_idx in range(nrx):
            for tx_idx in range(ntx):
                flattened_real.append(ordered_real[rx_idx][tx_idx])
                flattened_imag.append(ordered_imag[rx_idx][tx_idx])

    return flattened_real, flattened_imag


def iter_intel5300_frames(path: Path) -> Iterator[IntelFrame]:
    data = path.read_bytes()
    cur = 0
    frame_idx = 0

    while cur < len(data) - 3:
        field_len = (data[cur] << 8) + data[cur + 1]
        code = data[cur + 2]
        cur += 3

        record_len = field_len - 1
        if record_len < 0 or cur + record_len > len(data):
            break

        record = data[cur : cur + record_len]
        cur += record_len

        if code != INTEL_CODE or len(record) < 20:
            continue

        timestamp_low = record[0] + (record[1] << 8) + (record[2] << 16) + (record[3] << 24)
        bfee_count = record[4] + (record[5] << 8)
        nrx = record[8]
        ntx = record[9]
        rssi = [record[10], record[11], record[12]]
        noise = signed_int8(record[13])
        agc = record[14]
        antenna_sel = record[15]
        csi_len = record[16] + (record[17] << 8)
        rate = record[18] + (record[19] << 8)
        perm = [((antenna_sel >> shift) & 0x3) + 1 for shift in (0, 2, 4)]
        payload = record[20 : 20 + csi_len]
        real, imag = intel_csi_vector(payload, nrx, ntx, perm)

        yield IntelFrame(
            frame_idx=frame_idx,
            timestamp_low=timestamp_low,
            bfee_count=bfee_count,
            nrx=nrx,
            ntx=ntx,
            rssi=rssi,
            noise=noise,
            agc=agc,
            perm=perm,
            rate=rate,
            real=real,
            imag=imag,
        )
        frame_idx += 1


def scan_intel5300_stream(path: Path) -> Dict[str, object]:
    timestamps: List[int] = []
    nrx_values: Counter[int] = Counter()
    ntx_values: Counter[int] = Counter()
    frame_count = 0

    data = path.read_bytes()
    cur = 0
    while cur < len(data) - 3:
        field_len = (data[cur] << 8) + data[cur + 1]
        code = data[cur + 2]
        cur += 3
        record_len = field_len - 1
        if record_len < 0 or cur + record_len > len(data):
            break
        record = data[cur : cur + record_len]
        cur += record_len
        if code != INTEL_CODE or len(record) < 20:
            continue
        frame_count += 1
        timestamps.append(record[0] + (record[1] << 8) + (record[2] << 16) + (record[3] << 24))
        nrx_values[record[8]] += 1
        ntx_values[record[9]] += 1

    dominant_nrx = nrx_values.most_common(1)[0][0] if nrx_values else 0
    dominant_ntx = ntx_values.most_common(1)[0][0] if ntx_values else 0

    return {
        "frame_count": frame_count,
        "timestamp_start": timestamps[0] if timestamps else None,
        "timestamp_end": timestamps[-1] if timestamps else None,
        "estimated_hz": estimate_hz_from_timestamps(timestamps),
        "nrx_histogram": dict(nrx_values),
        "ntx_histogram": dict(ntx_values),
        "dominant_nrx": dominant_nrx,
        "dominant_ntx": dominant_ntx,
        "feature_count": 30 * dominant_nrx * dominant_ntx if dominant_nrx and dominant_ntx else None,
    }


def export_intel5300_stream(
    path: Path,
    stream_info: Dict[str, object],
    output_path: Path,
) -> Dict[str, object]:
    frames = list(iter_intel5300_frames(path))
    if not frames:
        return {}

    stream_start = frames[0].timestamp_low
    nrx = frames[0].nrx
    ntx = frames[0].ntx
    feature_names = build_link_feature_names(nrx, ntx)
    ensure_dir(output_path.parent)

    raw_timestamps_low = [frame.timestamp_low for frame in frames]
    raw_timestamps = unwrap_uint_timestamps(raw_timestamps_low, bits=32)
    stream_start = raw_timestamps[0]
    time_axis = [timestamp - stream_start for timestamp in raw_timestamps]
    header = [
        "time_axis_us",
        "raw_timestamp_axis",
        "raw_timestamp_low",
        "frame_idx",
        "bfee_count",
        "rssi_a",
        "rssi_b",
        "rssi_c",
        "noise",
        "agc",
        "perm1",
        "perm2",
        "perm3",
        "rate",
    ]
    for feature_name in feature_names:
        header.append(f"{feature_name}_real")
        header.append(f"{feature_name}_imag")

    def rows() -> Iterator[Sequence[object]]:
        for idx, frame in enumerate(frames):
            row: List[object] = [
                time_axis[idx],
                raw_timestamps[idx],
                raw_timestamps_low[idx],
                frame.frame_idx,
                frame.bfee_count,
                frame.rssi[0],
                frame.rssi[1],
                frame.rssi[2],
                frame.noise,
                frame.agc,
                frame.perm[0],
                frame.perm[1],
                frame.perm[2],
                frame.rate,
            ]
            for real_value, imag_value in zip(frame.real, frame.imag):
                row.append(real_value)
                row.append(imag_value)
            yield row

    write_tsv(output_path, header, rows())
    return {
        "export_file": str(output_path),
        "frame_count": len(frames),
        "estimated_hz": stream_info["estimated_hz"],
    }


def localization_stream_id(gateway_name: str) -> str:
    return f"localization/{gateway_name}"


def load_localization_records(path: Path) -> Dict[str, List[Dict[str, object]]]:
    records = json.loads(path.read_text(encoding="utf-8"))
    grouped: Dict[str, List[Dict[str, object]]] = defaultdict(list)

    for idx, item in enumerate(records):
        truth = item.get("truth", [])
        position = item.get("position", [])
        for server in item.get("xServer", []):
            frequency = server.get("frequency")
            for gateway_name, gateway in server.get("gateways", {}).items():
                created_time = gateway.get("createdTime")
                if not created_time:
                    continue
                grouped[gateway_name].append(
                    {
                        "frame_idx": idx,
                        "timestamp_us": parse_datetime_to_epoch_us(created_time),
                        "created_time": created_time,
                        "phase": gateway.get("phase", []),
                        "rss": gateway.get("rss", []),
                        "phaseOffset": gateway.get("phaseOffset", []),
                        "position": gateway.get("position", []),
                        "aoa": gateway.get("aoa", {}),
                        "truth": truth,
                        "record_position": position,
                        "tagId": item.get("tagId"),
                        "savedTime": item.get("savedTime"),
                        "frequency": frequency,
                        "sourcefile": gateway.get("sourcefile"),
                    }
                )

    for gateway_name in grouped:
        grouped[gateway_name].sort(key=lambda record: record["timestamp_us"])

    return grouped


def scan_localization_stream(records: Sequence[Dict[str, object]]) -> Dict[str, object]:
    timestamps = [int(item["timestamp_us"]) for item in records]
    return {
        "frame_count": len(records),
        "timestamp_start": timestamps[0] if timestamps else None,
        "timestamp_end": timestamps[-1] if timestamps else None,
        "estimated_hz": estimate_hz_from_timestamps(timestamps),
        "feature_count": 16,
    }


def export_localization_stream(
    gateway_name: str,
    records: Sequence[Dict[str, object]],
    stream_info: Dict[str, object],
    output_path: Path,
) -> Dict[str, object]:
    if not records:
        return {}

    stream_start = int(records[0]["timestamp_us"])
    feature_names = [f"bin_{idx:02d}" for idx in range(16)]
    ensure_dir(output_path.parent)

    time_axis = [int(item["timestamp_us"]) - stream_start for item in records]
    raw_timestamps = [int(item["timestamp_us"]) for item in records]
    header = ["time_axis_us", "raw_timestamp_axis", "frame_idx"]
    for feature_name in feature_names:
        header.append(f"{feature_name}_phase")
        header.append(f"{feature_name}_rss")

    def rows() -> Iterator[Sequence[object]]:
        for idx, item in enumerate(records):
            row: List[object] = [time_axis[idx], raw_timestamps[idx], item["frame_idx"]]
            for phase_value, rss_value in zip(item["phase"], item["rss"]):
                row.append(phase_value)
                row.append(rss_value)
            yield row

    write_tsv(output_path, header, rows())
    return {
        "export_file": str(output_path),
        "frame_count": len(records),
        "estimated_hz": stream_info["estimated_hz"],
    }


def build_intel_stream_info(path: Path, stats: Dict[str, object]) -> Dict[str, object]:
    rel_path = path.as_posix()
    if "gesture/" in rel_path:
        match = re.match(r"csi_(a\d+)_(\d+)\.dat$", path.name)
        label = match.group(1) if match else path.stem
        repetition = int(match.group(2)) if match else None
        task = "gesture_classification"
        scenario = "gesture"
        extra: Dict[str, object] = {}
    elif "localization/" in rel_path:
        match = re.match(r"(per\d+)_(\d+)_(\d+)\.dat$", path.name)
        person_id = match.group(1) if match else None
        angle_deg = int(match.group(2)) if match else None
        repetition = int(match.group(3)) if match else None
        label = f"{person_id}_angle_{angle_deg:03d}" if person_id is not None and angle_deg is not None else path.stem
        task = "angle_estimation"
        scenario = "localization"
        extra = {
            "person_id": person_id,
            "angle_deg": angle_deg,
        }
    else:
        match = re.match(r"Car_(\d+)_(\d+)\.dat$", path.name)
        label = f"speed_{match.group(1)}" if match else path.stem
        repetition = int(match.group(2)) if match else None
        task = "vehicle_speed_estimation"
        scenario = "vehicle"
        extra = {}

    info = {
        "stream_id": f"{scenario}/{path.stem}",
        "scenario": scenario,
        "task": task,
        "label": label,
        "repetition": repetition,
        "source_file": rel_path,
        "estimated_hz": stats["estimated_hz"],
        "frame_count": stats["frame_count"],
        "dominant_nrx": stats["dominant_nrx"],
        "dominant_ntx": stats["dominant_ntx"],
        "feature_count": stats["feature_count"],
        "timestamp_start": stats["timestamp_start"],
        "timestamp_end": stats["timestamp_end"],
    }
    info.update(extra)
    return info


def build_localization_stream_info(gateway_name: str, stats: Dict[str, object]) -> Dict[str, object]:
    return {
        "stream_id": localization_stream_id(gateway_name),
        "scenario": "localization",
        "task": "indoor_localization",
        "label": gateway_name,
        "source_file": "data/localization/s01.json",
        "estimated_hz": stats["estimated_hz"],
        "frame_count": stats["frame_count"],
        "feature_count": 16,
        "timestamp_start": stats["timestamp_start"],
        "timestamp_end": stats["timestamp_end"],
    }


def write_readme(output_dir: Path, args: argparse.Namespace) -> None:
    readme = f"""# Prepared CSI Streams

该目录是从 `data/` 原始数据整理出来的流式测试包。

## 统一格式

每个 stream 对应一个完整 TSV 文本文件，都是 `time x csi_data` 结构：

- `time_axis_us`: 每一帧对应的相对时间轴，单位微秒
- `feature_layout`: 特征维度定义
- `components`: 真正的 CSI 数据矩阵
  - `gesture` / `vehicle`: `real` 和 `imag`
  - `localization`: `phase` 和 `rss`
- `frame_meta`: 每一帧的辅助信息和标签

## 导出策略

- `export_layout`: `raw_text_lines`
- 不做 stride 采样
- 不做 chunk 切分
- 每行就是一帧，后续可自行按 packet size / window / stride 做实验

## 目录说明

- `manifest.tsv`: 全量流索引和统计
- `raw/<scenario>/<stream_id>.tsv`: 每条 stream 的完整原始文本文件
- 第一行是表头
- 从第二行开始，每一行就是一帧

## 重新导出

```bash
python3 scripts/prepare_csi_streams.py
```
"""
    (output_dir / "README.md").write_text(readme, encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Prepare raw CSI text streams from data/")
    parser.add_argument("--data-root", default="data", help="Input data root")
    parser.add_argument("--output-dir", default="prepared_csi_streams", help="Output directory")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    data_root = Path(args.data_root)
    output_dir = Path(args.output_dir)
    raw_root = output_dir / "raw"
    ensure_dir(raw_root)

    manifest_rows: List[Dict[str, object]] = []

    gesture_files = sorted((data_root / "gesture").glob("*.dat"))
    localization_dat_files = sorted((data_root / "localization").glob("*.dat"))
    vehicle_dat_files = sorted((data_root / "vehicle").glob("*.dat"))
    vehicle_mat_files = sorted((data_root / "vehicle").glob("*.mat"))

    scenario_counts: Counter[str] = Counter()
    scenario_frames: Counter[str] = Counter()

    for path in gesture_files + localization_dat_files + vehicle_dat_files:
        stats = scan_intel5300_stream(path)
        stream_info = build_intel_stream_info(path, stats)
        stream_output_dir = raw_root / stream_info["scenario"] / f"{sanitize_name(path.stem)}.tsv"
        exported = export_intel5300_stream(
            path=path,
            stream_info=stream_info,
            output_path=stream_output_dir,
        )
        manifest_rows.append({**stream_info, **exported, "export_layout": "raw_text_lines"})
        scenario_counts[stream_info["scenario"]] += 1
        scenario_frames[stream_info["scenario"]] += int(stream_info["frame_count"])

    localization_path = data_root / "localization" / "s01.json"
    if localization_path.exists():
        grouped = load_localization_records(localization_path)
        for gateway_name, records in sorted(grouped.items()):
            stats = scan_localization_stream(records)
            stream_info = build_localization_stream_info(gateway_name, stats)
            stream_output_dir = raw_root / "localization" / f"{sanitize_name(gateway_name)}.tsv"
            exported = export_localization_stream(
                gateway_name=gateway_name,
                records=records,
                stream_info=stream_info,
                output_path=stream_output_dir,
            )
            manifest_rows.append({**stream_info, **exported, "export_layout": "raw_text_lines"})
            scenario_counts["localization"] += 1
            scenario_frames["localization"] += int(stream_info["frame_count"])

    vehicle_basenames = {path.stem for path in vehicle_dat_files}
    write_readme(output_dir, args)
    write_kv_text(
        output_dir / "summary.txt",
        [
            ("generated_at", datetime.now().isoformat(timespec="seconds")),
            ("source_root", data_root),
            ("export_layout", "raw_text_lines"),
            ("stream_count", len(manifest_rows)),
            ("gesture_streams", scenario_counts.get("gesture", 0)),
            ("vehicle_streams", scenario_counts.get("vehicle", 0)),
            ("localization_streams", scenario_counts.get("localization", 0)),
            ("gesture_frames", scenario_frames.get("gesture", 0)),
            ("vehicle_frames", scenario_frames.get("vehicle", 0)),
            ("localization_frames", scenario_frames.get("localization", 0)),
        ],
    )
    auxiliary_lines = ["path\tpaired_with_dat\tnote"]
    for mat_path in vehicle_mat_files:
        auxiliary_lines.append(
            f"{mat_path.as_posix()}\t{str(mat_path.stem in vehicle_basenames).lower()}\tMATLAB complex matrix sidecar; not exported into raw text stream files."
        )
    (output_dir / "auxiliary_files.tsv").write_text("\n".join(auxiliary_lines) + "\n", encoding="utf-8")

    manifest_lines = [
        "\t".join(
            [
                "stream_id",
                "scenario",
                "task",
                "label",
                "source_file",
                "frame_count",
                "feature_count",
                "estimated_hz",
                "timestamp_start",
                "timestamp_end",
                "person_id",
                "angle_deg",
                "export_layout",
                "export_file",
            ]
        )
    ]
    for row in manifest_rows:
        manifest_lines.append(
            "\t".join(
                [
                    str(row.get("stream_id", "")),
                    str(row.get("scenario", "")),
                    str(row.get("task", "")),
                    str(row.get("label", "")),
                    str(row.get("source_file", "")),
                    str(row.get("frame_count", "")),
                    str(row.get("feature_count", "")),
                    str(row.get("estimated_hz", "")),
                    str(row.get("timestamp_start", "")),
                    str(row.get("timestamp_end", "")),
                    str(row.get("person_id", "")),
                    str(row.get("angle_deg", "")),
                    str(row.get("export_layout", "")),
                    str(row.get("export_file", "")),
                ]
            )
        )
    (output_dir / "manifest.tsv").write_text(
        "\n".join(manifest_lines) + "\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
