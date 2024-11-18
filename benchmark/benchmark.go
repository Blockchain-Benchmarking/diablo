package benchmark

import (
	"diablo/core"
	"diablo/core/network"
	"diablo/workload/behavior"
	"time"
)

type Benchmark interface {
	Run(accounts []behavior.Account, duration time.Duration, secondaries map[string]*network.Secondary, coordinator *core.Coordinator) error
}
