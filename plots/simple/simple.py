import os
import json
import sys
from datetime import datetime
from collections import defaultdict
import matplotlib.pyplot as plt
import numpy as np
import math

def parse_timestamp(timestamp):
    if '.' in timestamp:
        base, fraction = timestamp.split('.')
        fraction = fraction.rstrip('Z')
        fraction = fraction[:6]
        timestamp = f"{base}.{fraction}Z"
    return datetime.strptime(timestamp, "%Y-%m-%dT%H:%M:%S.%fZ")

def analyze_results(directory):
    all_metrics = {}

    filenames = sorted(
        [f for f in os.listdir(directory) if f.endswith(".json")],
        key=lambda x: int(os.path.splitext(x)[0])
    )

    for filename in filenames:
        submissions = defaultdict(int)
        successes = defaultdict(int)
        latencies = []
        first_event_time = None

        with open(os.path.join(directory, filename), "r") as file:
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
                    start_time = parse_timestamp(record["start_time"])
                    end_time = parse_timestamp(record["end_time"])

                    if first_event_time is None:
                        first_event_time = start_time

                    elapsed_time = int((start_time - first_event_time).total_seconds())

                    submissions[elapsed_time] += 1
                    if record.get("success", True):
                        successes[elapsed_time] += 1
                        latencies.append((end_time - start_time).total_seconds())
                except json.JSONDecodeError as e:
                    print(f"Error parsing record in {filename}: {e}")

        total_time = max(successes.keys()) if successes else 0
        throughput = sum(successes.values()) / total_time if total_time > 0 else 0
        avg_latency = sum(latencies) / len(latencies) if latencies else 0

        all_metrics[filename] = {
            "submissions": submissions,
            "successes": successes,
            "throughput": throughput,
            "avg_latency": avg_latency,
        }
    return all_metrics

def plot_combined_graphs(metrics, output_file):
    num_files = len(metrics)
    if num_files == 0:
        print("No valid data to plot. Exiting...")
        return

    cols = min(num_files, 4)
    rows = math.ceil(num_files / cols)

    fig, axes = plt.subplots(rows * 2, cols, figsize=(16, 8 * rows), constrained_layout=True)

    if isinstance(axes, np.ndarray):
        if axes.ndim == 1:
            axes = np.expand_dims(axes, axis=0)
        elif axes.ndim == 2:
            pass
    else:
        axes = np.array([[axes]])

    for idx, (filename, data) in enumerate(metrics.items()):
        row = (idx // cols) * 2
        col = idx % cols

        submissions = data["submissions"]
        successes = data["successes"]

        if not submissions or not successes:
            print(f"No valid data to plot for {filename}. Skipping...")
            continue

        times = sorted(set(submissions.keys()).union(successes.keys()))
        submission_counts = [submissions.get(t, 0) for t in times]
        success_counts = [successes.get(t, 0) for t in times]

        max_y = max(max(submission_counts), max(success_counts), 1800)
        y_ticks = range(0, max_y + 100, 100)

        axes[row][col].plot(times, submission_counts, label="Workload", marker="x", linestyle="-", color="blue")
        if col == 0:
            axes[row][col].set_ylabel("Workload")
        axes[row][col].set_ylim(0, max_y + 20)
        axes[row][col].set_yticks(y_ticks)
        axes[row][col].grid(True)

        axes[row + 1][col].plot(times, success_counts, label="Output", marker="x", linestyle="--", color="green")
        axes[row + 1][col].set_xlabel("Time (s)")
        if col == 0:
            axes[row + 1][col].set_ylabel("Output")
        axes[row + 1][col].set_ylim(0, max_y + 20)
        axes[row + 1][col].set_yticks(y_ticks)
        axes[row + 1][col].grid(True)

    #plt.suptitle("Workload ", fontsize=16)
    plt.savefig(output_file)
    plt.close()

def plot_latency_throughput_barchart(metrics, output_file):
    if not metrics:
        print("No valid data to plot. Exiting...")
        return

    tps_values = []
    avg_latencies = []
    throughputs = []

    for filename, data in metrics.items():
        throughput = data["throughput"]
        avg_latency = data["avg_latency"]
        tps = int(filename.split('.')[0])

        tps_values.append(tps)
        avg_latencies.append(avg_latency)
        throughputs.append(throughput)

    sorted_indices = np.argsort(tps_values)
    tps_values = np.array(tps_values)[sorted_indices]
    avg_latencies = np.array(avg_latencies)[sorted_indices]
    throughputs = np.array(throughputs)[sorted_indices]

    bar_width = 0.4
    x = np.arange(len(tps_values))

    fig, ax1 = plt.subplots(figsize=(10, 6))

    ax1.bar(x - bar_width / 2, throughputs, bar_width, label='Throughput (TPS)', color='blue')

    ax2 = ax1.twinx()
    ax2.bar(x + bar_width / 2, avg_latencies, bar_width, label='Latency (s)', color='orange')

    ax1.set_xlabel("TPS")
    ax1.set_ylabel("Throughput (TPS)")
    ax2.set_ylabel("Latency (s)")
    ax1.set_xticks(x)
    ax1.set_xticklabels(tps_values)
    ax1.grid(axis='y')
    ax1.legend(loc='upper left')
    ax2.legend(loc='upper right')

    plt.tight_layout()
    plt.savefig(output_file)
    plt.close()

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python simple.py <directory>")
        sys.exit(1)

    directory = sys.argv[1]
    output_file = f"combined_{directory}.png"
    bar_chart_output_file = f"bc_{directory}.png"

    metrics = analyze_results(directory)
    plot_combined_graphs(metrics, output_file)
    plot_latency_throughput_barchart(metrics, bar_chart_output_file)

    print(f"Combined graph saved to {output_file}.")
    print(f"Bar chart saved to {bar_chart_output_file}.")

for filename, data in metrics.items():
        print(f"\nResults for {filename}:")
        print(f"  Throughput (TPS): {data['throughput']:.2f}")
        print(f"  Average Latency (s): {data['avg_latency']:.2f}")