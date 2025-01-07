import json
import matplotlib.pyplot as plt

with open("coordinates.json", "r") as file:
    data = json.load(file)

throughput = []
latency = []
tps = []

for key, values in data.items():
    tps.append(int(key))
    throughput.append(values["throughput"])
    latency.append(values["latency"] / 1e9)

plt.figure(figsize=(10, 6))
plt.plot(throughput, latency, marker="o", linestyle="-", color="blue")

for i in range(len(tps)):
    plt.annotate(str(tps[i]), (throughput[i], latency[i]), textcoords="offset points", xytext=(5, 5), fontsize=8)

plt.xlabel("Throughput", fontsize=12)
plt.ylabel("Average Latency (s)", fontsize=12)
plt.grid(True, linestyle="--", alpha=0.6)
plt.tight_layout()

plt.savefig("dynamic.png")

plt.close()
