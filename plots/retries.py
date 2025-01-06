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
    start_times = []
    end_times = []
    total_transactions = 0

    with open(file_path, "r") as file:
        content = file.read()
        records = parse_json_records(content)

        total_transactions = len(records)  # Total transactions in the file

        for record_str in records:
            try:
                record = json.loads(record_str)
                # Only consider successful transactions
                if record.get("success", True):
                    start_time = parse_timestamp(record["start_time"])
                    end_time = parse_timestamp(record["end_time"])
                    latency = (end_time - start_time).total_seconds()
                    latencies.append(latency)
                    start_times.append(start_time)
                    end_times.append(end_time)
            except json.JSONDecodeError as e:
                print(f"Error parsing record in {file_path}: {e}")
            except KeyError as e:
                print(f"Missing key in record in {file_path}: {e}")

    return latencies, start_times, end_times, total_transactions

def calculate_throughput(start_times, end_times, total_transactions):
    if total_transactions == 0:
        return 0
    # Calculate the total time span from the first start time to the last end time
    total_time = (max(end_times) - min(start_times)).total_seconds()
    if total_time == 0:
        return 0
    throughput = total_transactions / total_time
    return throughput

def plot_cdf(latencies, label, total_transactions):
    if not latencies:
        print(f"No successful transactions found for {label}. Skipping...")
        return

    sorted_latencies = np.sort(latencies)
    cdf = np.arange(1, len(sorted_latencies) + 1) / total_transactions  # Normalize by total transactions
    plt.plot(sorted_latencies, cdf, label=label)

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python script_name.py <file1> <file2>")
        sys.exit(1)

    file1 = sys.argv[1]
    file2 = sys.argv[2]

    latencies1, start_times1, end_times1, total1 = extract_latencies(file1)
    latencies2, start_times2, end_times2, total2 = extract_latencies(file2)

    if latencies1:
        plot_cdf(latencies1, label=os.path.basename(file1), total_transactions=total1)
        avg_latency1 = np.mean(latencies1)
        throughput1 = calculate_throughput(start_times1, end_times1, len(latencies1))
        print(f"{os.path.basename(file1)}:")
        print(f"  Average Latency: {avg_latency1:.4f} seconds")
        print(f"  Throughput: {throughput1:.4f} transactions per second")
    else:
        print(f"No successful transactions found in {file1}.")

    if latencies2:
        plot_cdf(latencies2, label=os.path.basename(file2), total_transactions=total2)
        avg_latency2 = np.mean(latencies2)
        throughput2 = calculate_throughput(start_times2, end_times2, len(latencies2))
        print(f"{os.path.basename(file2)}:")
        print(f"  Average Latency: {avg_latency2:.4f} seconds")
        print(f"  Throughput: {throughput2:.4f} transactions per second")
    else:
        print(f"No successful transactions found in {file2}.")

    plt.xlabel("Latency (s)")
    plt.ylabel("CDF")
    plt.title("CDF of Transaction Latencies (Successful Only)")
    plt.legend()
    plt.grid(True)
    plt.tight_layout()

    output_file = "latency_cdf.png"
    plt.savefig(output_file)
    plt.close()

    print(f"CDF plot saved to {output_file}.")
