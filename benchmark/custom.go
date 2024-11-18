package benchmark

import (
	"diablo/core"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/workload/behavior"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	maxLatencyDiff = 30
)

type CustomBenchmark struct {
}

type Coordinates struct {
	Throughput int           `json:"Throughput"`
	Latency    time.Duration `json:"Latency"`
}

func (c *CustomBenchmark) Run(accounts []behavior.Account, _ time.Duration, secondaries map[string]*network.Secondary, coordinator *core.Coordinator) error {
	endpoints := []string{"ws://127.0.0.1:9000", "ws://127.0.0.1:9001", "ws://127.0.0.1:9002"}
	users, err := createUsersFromAccounts(accounts, "stubbornPaymentUser", "ethereum", endpoints, map[string]interface{}{
		"timeout":      "60s",
		"max_attempts": 1,
	})

	if err != nil {
		return err
	}

	graph := make(map[int]Coordinates) //tps -> (latency, throughput)

	//Send all the users with initial workload of x tps for a certain duration
	tps := 200
	err = coordinator.SendUsersToGenerators(tps, time.Minute, users)
	if err != nil {
		return err
	}

	err = coordinator.SendStartToAll(time.Now().Add(3*time.Second), 10)
	if err != nil {
		return err
	}

	time.Sleep(3 * time.Second)

	var previousTps int
	for {
		time.Sleep(1 * time.Minute)

		res := coordinator.CollectNewResults()
		logging.Infof("intermediary %d results", len(res))

		curPerf := Coordinates{
			Throughput: behavior.Throughput(res),
			Latency:    behavior.AverageLatency(res),
		}

		graph[tps] = curPerf

		logging.Infof("TPS: %d, Throughput: %d, Latency: %s", tps, curPerf.Throughput, curPerf.Latency.String())

		if curPerf.Throughput == 0 || curPerf.Latency == 0 {
			break
		}

		if len(graph) > 1 {
			previous := graph[previousTps]
			ldiff := curPerf.Latency - previous.Latency
			_ = curPerf.Throughput - previous.Throughput

			if ldiff.Seconds() > maxLatencyDiff {
				logging.Warnf("latency difference reached %s", ldiff.String())
				break
			}
		}

		previousTps = tps
		tps = tps + 200

		err = coordinator.UpdateAllUsersTps(tps, time.Minute)
	}

	err = coordinator.SendStopToAll()

	file, err := os.Create("coordinates.json")
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err.Error())
	}
	defer file.Close()

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %s:", err.Error())
	}

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to file: %s", err.Error())
	}

	return nil
}
