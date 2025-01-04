package benchmark

import (
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/network"
)

type Benchmark interface {
	Run(accounts []blockchain.Account, secondaries map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error
}
