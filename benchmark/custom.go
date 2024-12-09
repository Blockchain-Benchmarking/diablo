package benchmark

import (
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/workload/behavior"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

const (
	maxTps                  = 1600
	maxThroughputLossFactor = 0.25
	maxLatencyDiff          = 30 * time.Second
)

type CustomBenchmark struct {
}

type Coordinates struct {
	Throughput int           `json:"Throughput"`
	Latency    time.Duration `json:"Latency"`
}

func (c *CustomBenchmark) Run(accounts []blockchain.Account, _ time.Duration, _ map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	tps := 200
	tpsAdd := 200

	defaultStubbornPaymentUser, ok := userTypes["stubbornPaymentUser"]
	if !ok {
		return fmt.Errorf("stubbornPaymentUser not implemented")
	}
	users, err := createStubbornUsersFromAccounts(accounts, tps, "ethereum", endpoints, defaultStubbornPaymentUser)

	if err != nil {
		return err
	}

	graph := make(map[int]Coordinates) //tps -> (latency, throughput)

	err = coordinator.SendUsersToGenerators(users)
	if err != nil {
		return err
	}

	for {
		startTime := time.Now().Add(30 * time.Second)
		endTime := startTime.Add(2 * time.Minute)
		err = coordinator.SendStartToAll(startTime)
		if err != nil {
			return err
		}

		time.Sleep(2*time.Minute + 30*time.Second + 10*time.Second) //10 additional seconds for last results to arrive

		res, ok := coordinator.CollectResultsWithInterval(startTime, endTime.Add(15*time.Second))
		logging.Infof("intermediary %d results", len(res))

		for !ok {
			logging.Warnf("missing %d results, waiting", tps*120-len(res))
			time.Sleep(5 * time.Second)
			res, ok = coordinator.CollectResultsWithInterval(startTime, endTime.Add(15*time.Second))
			logging.Infof("intermediary %d results", len(res))
		}

		logging.Infof("checking %d results", len(res))

		curPerf := Coordinates{
			Throughput: behavior.Throughput(res, 2*time.Minute),
			Latency:    behavior.AverageLatency(res),
		}

		graph[tps] = curPerf

		logging.Infof("TPS: %d, Throughput: %d/s, Latency: %s", tps, curPerf.Throughput, curPerf.Latency.String())

		if curPerf.Throughput == 0 || curPerf.Latency == 0 {
			break
		}

		newTps := 0
		if len(graph) > 1 {
			prev := previousTps(graph, tps)
			previous := graph[prev]
			ldiff := curPerf.Latency - previous.Latency

			if ldiff > maxLatencyDiff {
				logging.Warnf("latency difference reached %s", ldiff.String())
				newTps = (prev + tps) / 2
			} else if (tps - curPerf.Throughput) > int(float64(tps)*maxThroughputLossFactor) {
				logging.Warnf("throughput failed to keep up, difference was %d vs %d allowed", tps-curPerf.Throughput, int(float64(tps)*maxThroughputLossFactor))
				newTps = (prev + tps) / 2
			} else {
				logging.Infof("performance improving, increasing tps")
				newTps = int(math.Min(float64(tps+tpsAdd), maxTps))
			}

			if math.Abs(float64(prev-newTps)) < 10 {
				logging.Infof("found optimal tps with sufficient precision (prev %d, newtps %d)", prev, newTps)
				break
			}

		} else {
			newTps = tps + tpsAdd
		}

		tps = newTps

		updatedUsers, err := createStubbornUsersFromAccounts(accounts, tps, "ethereum", endpoints, defaultStubbornPaymentUser)

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

func previousTps(graph map[int]Coordinates, cur int) int {
	result := 0
	for k, _ := range graph {
		if k > result && k < cur {
			result = k
		}
	}

	return result
}
