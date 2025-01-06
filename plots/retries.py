import os
import json
import sys
from datetime import datetime
import matplotlib.pyplot as plt
import numpy as np

def parse_timestamp(timestamp):
    if '.' in timestamp:
        base, fraction = timestamp.split('.')
        fraction = fraction.rstrip('Z')
        fraction = fraction[:6]
        timestamp = f"{base}.{fraction}Z"
    return datetime.strptime(timestamp, "%Y-%m-%dT%H:%M:%S.%fZ")

def parse_json_records(file_content):
    records = file_content.strip().split("}{")
    if records:
        records[0] += "}"
        records[-1] = "{" + records[-1]
        for i in range(1, len(records) - 1):
            records[i] = "{" + records[i] + "}"
    return records

def extract_latencies(file_path):
    latencies = []

    with open(file_path, "r") as file:
        content = file.read()
        records = parse_json_records(content)

        for record_str in records:
            try:
                record = json.loads(record_str)
                start_time = parse_timestamp(record["start_time"])
                end_time = parse_timestamp(record["end_time"])
                latency = (end_time - start_time).total_seconds()
                latencies.append(latency)
            except json.JSONDecodeError as e:
                print(f"Error parsing record in {file_path}: {e}")
            except KeyError as e:
                print(f"Missing key in record in {file_path}: {e}")

    return latencies

def plot_cdf(latencies, label):
    sorted_latencies = np.sort(latencies)
    cdf = np.arange(1, len(sorted_latencies) + 1) / len(sorted_latencies)
    plt.plot(sorted_latencies, cdf, label=label)

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python script_name.py <file1> <file2>")
        sys.exit(1)

    file1 = sys.argv[1]
    file2 = sys.argv[2]

    latencies1 = extract_latencies(file1)
    latencies2 = extract_latencies(file2)

    if latencies1:
        plot_cdf(latencies1, label=os.path.basename(file1))
    else:
        print(f"No latencies found in {file1}.")

    if latencies2:
        plot_cdf(latencies2, label=os.path.basename(file2))
    else:
        print(f"No latencies found in {file2}.")

    plt.xlabel("Latency (s)")
    plt.ylabel("CDF")
    plt.title("CDF of Transaction Latencies")
    plt.legend()
    plt.grid(True)
    plt.tight_layout()

    output_file = "latency_cdf.png"
    plt.savefig(output_file)
    plt.close()

    print(f"CDF plot saved to {output_file}.")
