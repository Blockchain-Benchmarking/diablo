package core

import (
	"diablo-benchmark/blockchains"
	"diablo-benchmark/blockchains/workloadgenerators"
	"diablo-benchmark/communication"
	"diablo-benchmark/core/configs"
	"go.uber.org/zap"
)

// Master
type Master struct {
	Server            *communication.MasterServer // TCP server identified with the master for all clients to connect to
	workloadGenerator workloadgenerators.WorkloadGenerator
	benchmarkConfig   *configs.BenchConfig
	chainConfig       *configs.ChainConfig
}

// Initialise the master server and return an instance of the master
// This will be passed back to the main
func InitMaster(listenAddr string, expectedClients int, wg workloadgenerators.WorkloadGenerator, bConfig *configs.BenchConfig, cConfig *configs.ChainConfig) *Master {
	s, err := communication.SetupMasterTCP(listenAddr, expectedClients)
	if err != nil {
		// TODO remove panic
		panic(err)
	}

	return &Master{
		Server:            s,
		workloadGenerator: wg,
		benchmarkConfig:   bConfig,
		chainConfig:       cConfig,
	}
}

// Main functionality to run
// Holds the majority of the work
func (ms *Master) Run() {
	// First, set up the blockchain
	err := ms.workloadGenerator.BlockchainSetup()

	if err != nil {
		zap.L().Error("encountered error with blockchain setup",
			zap.String("error", err.Error()))
		return
	}

	// Next, init the workload generator
	err = ms.workloadGenerator.InitParams()
	if err != nil {
		zap.L().Error("encountered error with workloadgenerator InitParams",
			zap.String("error", err.Error()))
		return
	}

	// Get the client connections ready
	clientReadyChannel := make(chan bool, 1)
	go ms.Server.HandleClients(clientReadyChannel)
	<-clientReadyChannel
	close(clientReadyChannel)

	// Parse the config files
	// Run all preparation

	// Run through the benchmark suite
	// Step 1: send "PREPARE" to clients, make sure we can communicate.
	errs := ms.Server.PrepareBenchmarkClients()

	if errs != nil {
		// We have errors
		ms.Server.CloseClients()
		ms.Server.Close()
		zap.L().Error("Encountered errors in clients",
			zap.Strings("errors", errs))
	}

	// Number of clients connected
	zap.L().Info("Benchmark clients all connected.",
		zap.Int("clients", len(ms.Server.Clients)))

	// Set up the blockchain information

	// Step 2: Blockchain type (tells which interface they should be using)
	// get the blockchain byte
	bcMessage, err := blockchains.MatchStringToMessage(ms.chainConfig.Name)

	if err != nil {
		ms.Server.CloseClients()
		ms.Server.Close()
	}

	errs = ms.Server.SendBlockchainType(bcMessage)

	if errs != nil {
		zap.L().Error("failed to send blockchain type",
			zap.Strings("errors", errs))
		ms.Server.CloseClients()
		ms.Server.Close()
		return
	}

	// Step 3: Prepare the workload for the benchmark
	// TODO: generate workloads
	workload, err := ms.workloadGenerator.GenerateWorkload()

	if err != nil {
		zap.L().Error("failed to generate workload",
			zap.String("error", err.Error()))
	}

	// Step 4: Distribute benchmark
	ms.Server.SendWorkload(workload)

	// Step 5: run the bench
	ms.Server.RunBenchmark()

	// Step 6 (once all have completed) - get the results
	ms.Server.GetResults()

	// Step 7 - store results
	// TODO: store results

	// Step 8: Close all connections
	ms.Server.CloseClients()
	ms.Server.Close()
}
