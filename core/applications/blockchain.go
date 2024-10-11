package applications

/**
var Blockchains = map[string]func() types.IBalancer{
	"round-robin": round_robin.NewRoundRobin,
}*/

type BlockchainInterface interface {
	Application
}

type Application interface{}
