package benchmark

import (
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/network"
	"time"
)

type Benchmark interface {
	Run(accounts []blockchain.Account, duration time.Duration, secondaries map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error
}
