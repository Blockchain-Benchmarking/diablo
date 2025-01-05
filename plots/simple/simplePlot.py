import os
import json
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

def analyze_results(directory):
    all_metrics = {}

    for filename in os.listdir(directory):
        if filename.endswith(".json"):
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

def plot_graphs(metrics, output_dir):
    """Generate and save graphs based on submissions and successes."""
    for filename, data in metrics.items():
        submissions = data["submissions"]
        successes = data["successes"]

        if not submissions or not successes:
            print(f"No valid data to plot for {filename}. Skipping...")
            continue

        # Extract sorted times and counts
        times = sorted(set(submissions.keys()).union(successes.keys()))
        submission_counts = [submissions.get(t, 0) for t in times]
        success_counts = [successes.get(t, 0) for t in times]

        # Determine y-axis limits
        max_y = max(max(submission_counts), max(success_counts), 1000)
        y_ticks = range(0, max_y + 50, 50)

        # Create subplots
        fig, axes = plt.subplots(2, 1, figsize=(12, 12), sharex=False)

        # Plot Submissions
        axes[0].plot(times, submission_counts, label="Submissions", marker="o", linestyle="-", color="blue")
        axes[0].set_title(f"Submissions Over Time for {filename}")
        axes[0].set_ylabel("Count (TPS)")
        axes[0].set_ylim(0, max_y + 20)
        axes[0].set_yticks(y_ticks)
        axes[0].grid(True)

        # Plot Successes
        axes[1].plot(times, success_counts, label="Successes", marker="x", linestyle="--", color="green")
        axes[1].set_title(f"Successes Over Time for {filename}")
        axes[1].set_xlabel("Elapsed Time (s)")
        axes[1].set_ylabel("Count (TPS)")
        axes[1].set_ylim(0, max_y + 20)
        axes[1].set_yticks(y_ticks)
        axes[1].grid(True)

        # Adjust layout and save
        plt.tight_layout()
        plot_file = f"{os.path.splitext(filename)[0]}_workload.png"
        plt.savefig(os.path.join(output_dir, plot_file))
        plt.close()

if __name__ == "__main__":
    directory = "."
    output_dir = "."

    if not os.path.exists(output_dir):
        os.makedirs(output_dir)

    metrics = analyze_results(directory)
    plot_graphs(metrics, output_dir)

    for filename, data in metrics.items():
        print(f"\nResults for {filename}:")
        print(f"  Throughput (TPS): {data['throughput']:.2f}")
        print(f"  Average Latency (s): {data['avg_latency']:.2f}")
        print(f"  Total Submissions: {len(data['submissions'])}")
        print(f"  Total Successes: {len(data['successes'])}")
