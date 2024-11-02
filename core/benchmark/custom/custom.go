package custom

import (
	"diablo/core/behavior"
	"diablo/core/network"
	"diablo/core/workload"
)

type Benchmark struct {
	Setup
	secondaries map[string]*network.Secondary
	coordinator *workload.Coordinator
	results     chan behavior.Results
}

type Setup struct {
}
