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
	maxTps                  = 1000
	maxThroughputLossFactor = 0.25
	maxLatencyDiff          = 30 * time.Second
)

type CustomBenchmark struct {
}

type Coordinates struct {
	Throughput int           `json:"Throughput"`
	Latency    time.Duration `json:"Latency"`
}

func (c *CustomBenchmark) Run(accounts []behavior.Account, _ time.Duration, _ map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	var tps, tpsFactor float64
	tps = 200
	tpsFactor = 2

	users, err := createStubbornPaymentUsersFromAccounts(accounts, int(tps), "ethereum", endpoints, map[string]interface{}{
		"timeout":      "15s",
		"max_attempts": 1,
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
	var lastGoodTps float64
	for {
		increasingTps := true
		startTime := time.Now().Add(10 * time.Second)
		endTime := startTime.Add(2 * time.Minute)
		err = coordinator.SendStartToAll(time.Now().Add(10 * time.Second))
		if err != nil {
			return err
		}

		time.Sleep(2*time.Minute + 10*time.Second + 10*time.Second) //10 additional seconds for last results to arrive

		res, ok := coordinator.CollectResultsWithInterval(startTime, endTime)
		logging.Infof("intermediary %d results", len(res))

		for !ok {
			logging.Warnf("missing %d results, waiting", int(tps)*120-len(res))
			time.Sleep(5 * time.Second)
			res, ok = coordinator.CollectResultsWithInterval(startTime, endTime)
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

			if ldiff > maxLatencyDiff {
				logging.Warnf("latency difference reached %s", ldiff.String())
				increasingTps = false
			} else if (int(tps) - curPerf.Throughput) <= int(tps*maxThroughputLossFactor) {
				logging.Warnf("throughput failed to keep up, reducing tps")
				increasingTps = false
			}

			if increasingTps && ldiff <= maxLatencyDiff {
				tpsFactor = 2
				logging.Infof("perf improving, increase tps")
			} else if !increasingTps {
				logging.Warnf("decreasing tps")
				tpsFactor = 0.5
			}

		}

		if tpsFactor == 2 {
			lastGoodTps = tps
		}

		previousTps = tps
		tps = tps * tpsFactor
		if int(tps) >= maxTps {
			logging.Warnf("reached max tps limit")
			break
		}

		if tps < lastGoodTps && !increasingTps {
			logging.Infof("found optimal tps %d", int(lastGoodTps))
			break
		}

		updatedUsers, err := createStubbornPaymentUsersFromAccounts(accounts, int(tps), "ethereum", endpoints, map[string]interface{}{
			"timeout":      "15s",
			"max_attempts": 1,
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
