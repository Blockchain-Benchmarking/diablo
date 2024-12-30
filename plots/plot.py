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
plt.plot(throughput, latency, marker="o", linestyle="-", color="blue", label="Latency vs Throughput")

for i in range(len(tps)):
    plt.annotate(str(tps[i]), (throughput[i], latency[i]), textcoords="offset points", xytext=(5, 5), fontsize=8)

plt.title("Latency as a Function of Throughput", fontsize=14)
plt.xlabel("Throughput", fontsize=12)
plt.ylabel("Latency", fontsize=12)
plt.grid(True, linestyle="--", alpha=0.6)
plt.legend()
plt.tight_layout()

plt.savefig("latency_vs_throughput.png")

plt.close()
