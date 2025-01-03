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
	baseTps      = 200
	incrementTps = 100
	maxTps       = 800

	maxLatencyDiff = 5 * time.Second
)

type CustomAdaptBenchmark struct{}

type Coordinates struct {
	Throughput int           `json:"throughput"`
	Latency    time.Duration `json:"latency"`
}

func (c *CustomAdaptBenchmark) Run(accounts []blockchain.Account, _ time.Duration, _ map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	tps := baseTps
	limit := maxTps
	latencyDiff := maxLatencyDiff
	increment := incrementTps

	defaultStubbornPaymentUser, ok := userTypes["stubbornStoreUser"]
	if !ok {
		return fmt.Errorf("stubbornStoreUser not implemented")
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

bench:
	for {
		startTime := time.Now().Add(30 * time.Second)
		endTime := startTime.Add(2 * time.Minute)
		err = coordinator.SendStartToAll(startTime)
		if err != nil {
			return err
		}

		time.Sleep(2*time.Minute + 30*time.Second + 20*time.Second) //10 additional seconds for last results to arrive

		res, ok := coordinator.CollectResultsWithInterval(startTime, endTime.Add(20*time.Second))
		logging.Infof("intermediary %d results", len(res))

		for !ok {
			logging.Warnf("missing %d results, waiting", tps*120-len(res))
			time.Sleep(5 * time.Second)
			res, ok = coordinator.CollectResultsWithInterval(startTime, endTime.Add(20*time.Second))
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

			if ldiff > latencyDiff || curPerf.Throughput < int(float64(tps)*0.95) {
				limit = tps
				newTps = (prev + tps) / 2
				latencyDiff = 1 * time.Second
			} else {
				newTps = int(math.Min(float64(tps+increment), float64(limit)))
				_, ok := graph[newTps]
				for ok {
					increment = increment / 2
					if increment == 0 {
						break bench
					}
					newTps = int(math.Min(float64(tps+increment), float64(limit)))
					_, ok = graph[newTps]
				}
			}

			if math.Abs(float64(prev-newTps)) < 10 {
				logging.Infof("found optimal tps with sufficient precision (prev %d, newtps %d)", prev, newTps)
				break
			}

		} else {
			newTps = tps + incrementTps
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
