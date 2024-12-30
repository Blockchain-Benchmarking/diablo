package benchmark

import (
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/network"
	"time"
)

// CustomBenchmark is to be written by the user using core.Coordinator methods to communicate the workload and collect the results from the secondaries
type CustomBenchmark struct{}

func (c *CustomBenchmark) Run(accounts []blockchain.Account, _ time.Duration, _ map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	//TODO: Write benchmark here
	return nil
}
