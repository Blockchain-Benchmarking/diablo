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
	maxLatencyDiff = 30 * time.Second
)

type CustomBenchmark struct {
}

type Coordinates struct {
	Throughput int           `json:"Throughput"`
	Latency    time.Duration `json:"Latency"`
}

func (c *CustomBenchmark) Run(accounts []behavior.Account, _ time.Duration, secondaries map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	var tps, tpsFactor float64
	tps = 200
	tpsFactor = 2

	users, err := createStubbornPaymentUsersFromAccounts(accounts, int(tps), "ethereum", endpoints, map[string]interface{}{
		"timeout":      "15s",
		"max_attempts": 5,
		"random":       true,
	})

	if err != nil {
		return err
	}

	graph := make(map[int]Coordinates) //tps -> (latency, throughput)

	err = coordinator.SendUsersToGenerators(users)
	if err != nil {
		return err
	}

	var previousTps float64
	for {
		startTime := time.Now().Add(3 * time.Second)
		err = coordinator.SendStartToAll(time.Now().Add(3 * time.Second))
		if err != nil {
			return err
		}

		time.Sleep(2*time.Minute + 3*time.Second)
		endTime := time.Now()

		var res []behavior.Result
		res = append(res, coordinator.CollectNewResults(startTime, endTime)...)
		logging.Infof("intermediary %d results", len(res))

		for len(res) <= int(tps)*100 {
			logging.Warnf("missing %d results, waiting", int(tps)*100-len(res))
			time.Sleep(2 * time.Second)
			res = append(res, coordinator.CollectNewResults(startTime, endTime)...)
			logging.Infof("intermediary %d results", len(res))
		}

		logging.Infof("checking %d results", len(res))

		curPerf := Coordinates{
			Throughput: behavior.Throughput(res, 2*time.Minute),
			Latency:    behavior.AverageLatency(res),
		}

		graph[int(tps)] = curPerf

		logging.Infof("TPS: %d, Throughput: %d, Latency: %s", int(tps), curPerf.Throughput, curPerf.Latency.String())

		if curPerf.Throughput == 0 || curPerf.Latency == 0 {
			break
		}

		if len(graph) > 1 {
			previous := graph[int(previousTps)]
			ldiff := curPerf.Latency - previous.Latency
			tdiff := curPerf.Throughput - previous.Throughput

			if ldiff > maxLatencyDiff {
				logging.Warnf("latency difference reached %s", ldiff.String())
				tpsFactor = 0.8
			} else if tdiff <= 0 {
				logging.Warnf("throughput difference stagnated or decreased (%d), reducing tps", tdiff)
				tpsFactor = 0.9
			} else {
				tpsFactor = 2
				logging.Infof("performance improving increasing tps")
			}
		}

		previousTps = tps
		tps = tps * tpsFactor

		updatedUsers, err := createStubbornPaymentUsersFromAccounts(accounts, int(tps), "ethereum", endpoints, map[string]interface{}{
			"timeout":      "15s",
			"max_attempts": 5,
			"random":       true,
		})

		if err != nil {
			return err
		}

		err = coordinator.SendUsersToGenerators(updatedUsers)
		if err != nil {
			return err
		}
	}

	err = coordinator.SendStopToAll()
	if err != nil {
		return err
	}

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
