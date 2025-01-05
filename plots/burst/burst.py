import json
import sys
from datetime import datetime
from collections import defaultdict
import matplotlib.pyplot as plt

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

                elapsed_time = int((start_time - start_time.replace(hour=0, minute=0, second=0, microsecond=0)).total_seconds())

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
    }

def plot_graphs(metrics, output_file):
    fig, axes = plt.subplots(2, 1, figsize=(12, 10))

    transfer_workload = metrics["transfer_workload"]
    transfer_successes = metrics["transfer_successes"]
    read_write_workload = metrics["read_write_workload"]
    read_write_successes = metrics["read_write_successes"]

    times = sorted(set(transfer_workload.keys()).union(read_write_workload.keys()))

    transfer_workload_values = [transfer_workload.get(t, 0) for t in times]
    read_write_workload_values = [read_write_workload.get(t, 0) for t in times]

    transfer_success_values = [transfer_successes.get(t, 0) for t in times]
    read_write_success_values = [read_write_successes.get(t, 0) for t in times]

    # Plot workloads
    axes[0].plot(times, transfer_workload_values, label="Transfer Workload", linestyle="-", marker="o")
    axes[0].plot(times, read_write_workload_values, label="Read+Write Workload", linestyle="--", marker="x")

    # Plot successes
    axes[1].plot(times, transfer_success_values, label="Transfer Successes", linestyle="-", marker="o")
    axes[1].plot(times, read_write_success_values, label="Read+Write Successes", linestyle="--", marker="x")

    axes[0].set_title("Workload Curves")
    axes[0].set_xlabel("Time (s)")
    axes[0].set_ylabel("Workload")
    axes[0].legend()
    axes[0].grid(True)

    axes[1].set_title("Success Curves")
    axes[1].set_xlabel("Time (s)")
    axes[1].set_ylabel("Successes")
    axes[1].legend()
    axes[1].grid(True)

    plt.tight_layout()
    plt.savefig(output_file)
    plt.close()

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python script_name.py <json_file>")
        sys.exit(1)

    json_file = sys.argv[1]
    output_file = f"combined_graph_{json_file}.png"

    metrics = analyze_file(json_file)
    plot_graphs(metrics, output_file)

    print(f"Graph saved to {output_file}.")
