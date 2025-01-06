import os
import json
import sys
from datetime import datetime
from collections import defaultdict
import matplotlib.pyplot as plt
import math

def parse_timestamp(timestamp):
    if '.' in timestamp:
        base, fraction = timestamp.split('.')
        fraction = fraction.rstrip('Z')
        fraction = fraction[:6]
        timestamp = f"{base}.{fraction}Z"
    return datetime.strptime(timestamp, "%Y-%m-%dT%H:%M:%S.%fZ")

def analyze_file(filename):
    transfer_workload = defaultdict(int)
    transfer_successes = defaultdict(int)
    read_write_workload = defaultdict(int)
    read_write_successes = defaultdict(int)
    earliest_time = None
    latest_time = None

    with open(filename, "r") as file:
        content = file.read().strip()
        records = content.split("}{")
        if records:
            records[0] += "}"
            records[-1] = "{" + records[-1]
            for i in range(1, len(records) - 1):
                records[i] = "{" + records[i] + "}"

        for record_str in records:
            try:
                record = json.loads(record_str)
                kind = record["kind"]
                start_time = parse_timestamp(record["start_time"])
                end_time = parse_timestamp(record["end_time"])
                success = record.get("success", True)

                if earliest_time is None or start_time < earliest_time:
                    earliest_time = start_time
                if latest_time is None or end_time > latest_time:
                    latest_time = end_time

                elapsed_time = int((start_time - earliest_time).total_seconds())

                if kind == "transfer":
                    transfer_workload[elapsed_time] += 1
                    if success:
                        transfer_successes[elapsed_time] += 1
                elif kind in ["read", "write"]:
                    read_write_workload[elapsed_time] += 1
                    if success:
                        read_write_successes[elapsed_time] += 1
            except json.JSONDecodeError as e:
                print(f"Error parsing record in {filename}: {e}")

    return {
        "transfer_workload": transfer_workload,
        "transfer_successes": transfer_successes,
        "read_write_workload": read_write_workload,
        "read_write_successes": read_write_successes,
        "earliest_time": earliest_time,
        "latest_time": latest_time,
    }

def plot_combined_graphs(directory, metrics_list, filenames, output_file):
    num_files = len(metrics_list)
    if num_files == 0:
        print("No valid data to plot. Exiting...")
        return

    rows = num_files * 2
    fig, axes = plt.subplots(rows, 1, figsize=(12, 5 * num_files))

    for idx, (metrics, filename) in enumerate(zip(metrics_list, filenames)):
        transfer_workload = metrics["transfer_workload"]
        transfer_successes = metrics["transfer_successes"]
        read_write_workload = metrics["read_write_workload"]
        read_write_successes = metrics["read_write_successes"]

        earliest_time = metrics["earliest_time"]
        latest_time = metrics["latest_time"]

        if not earliest_time or not latest_time:
            print(f"No valid time range found for {filename}. Skipping...")
            continue

        duration = int((latest_time - earliest_time).total_seconds())
        active_times = set(transfer_workload.keys()).union(read_write_workload.keys())
        active_times = sorted(active_times)

        if not active_times:
            print(f"No activity found in {filename}. Skipping...")
            continue

        max_active_time = max(active_times)

        times = list(range(max_active_time + 1))

        transfer_workload_values = [transfer_workload.get(t, 0) for t in times]
        read_write_workload_values = [read_write_workload.get(t, 0) for t in times]

        transfer_success_values = [transfer_successes.get(t, 0) for t in times]
        read_write_success_values = [read_write_successes.get(t, 0) for t in times]

        axes[idx * 2].plot(times, transfer_workload_values, label="Transfer Workload", linestyle="-", marker="o")
        axes[idx * 2].plot(times, read_write_workload_values, label="Read+Write Workload", linestyle="--", marker="x")
        axes[idx * 2].set_title(f"Workload Curves for {filename}")
        axes[idx * 2].set_xlabel("Time (s)")
        axes[idx * 2].set_ylabel("Workload")
        axes[idx * 2].set_xlim(0, max_active_time)
        axes[idx * 2].legend()
        axes[idx * 2].grid(True)

        axes[idx * 2 + 1].plot(times, transfer_success_values, label="Transfer Successes", linestyle="-", marker="o")
        axes[idx * 2 + 1].plot(times, read_write_success_values, label="Read+Write Successes", linestyle="--", marker="x")
        axes[idx * 2 + 1].set_title(f"Success Curves for {filename}")
        axes[idx * 2 + 1].set_xlabel("Time (s)")
        axes[idx * 2 + 1].set_ylabel("Successes")
        axes[idx * 2 + 1].set_xlim(0, max_active_time)
        axes[idx * 2 + 1].legend()
        axes[idx * 2 + 1].grid(True)

    plt.tight_layout()
    plt.savefig(output_file)
    plt.close()

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python script_name.py <directory>")
        sys.exit(1)

    directory = sys.argv[1]
    output_file = f"combined_graphs_{os.path.basename(directory)}.png"

    filenames = sorted(
        [f for f in os.listdir(directory) if f.endswith(".json")],
        key=lambda x: os.path.splitext(x)[0]
    )

    metrics_list = []
    for filename in filenames:
        file_path = os.path.join(directory, filename)
        print(f"Analyzing {file_path}...")
        metrics = analyze_file(file_path)
        metrics_list.append(metrics)

    plot_combined_graphs(directory, metrics_list, filenames, output_file)

    print(f"Combined graphs saved to {output_file}.")
