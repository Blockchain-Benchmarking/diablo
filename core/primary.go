// Package core provides the critical handlers and structures that run the
// benchmark. This includes the code for the primary and secondary nodes as well
// as any major handlers (workload, results, etc.). This code should not be
// required to be augmented when integrating a new blockchain or adding
// new workloads.
package core

import (
	"diablo-benchmark/blockchains"
	"diablo-benchmark/blockchains/workloadgenerators"
	"diablo-benchmark/communication"
	"diablo-benchmark/core/configs"
	"diablo-benchmark/core/results"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Primary benchmark server, acts as the orchestrator for the benchmark
type Primary struct {
	Server            *communication.PrimaryServer         // TCP server identified with the primary for all secondaries to connect to
	workloadGenerator workloadgenerators.WorkloadGenerator // Workload generator implementation that will generate the transactions
	benchmarkConfig   *configs.BenchConfig                 // Benchmark configuration about the workload
	chainConfig       *configs.ChainConfig                 // Chain configuration containing information about the nodes
}

// InitPrimary initialises the primary server and returns an instance of the primary
// This will be passed back to the main
func InitPrimary(listenAddr string, expectedSecondaries int, wg workloadgenerators.WorkloadGenerator, bConfig *configs.BenchConfig, cConfig *configs.ChainConfig) *Primary {
	s, err := communication.SetupPrimaryTCP(listenAddr, expectedSecondaries)
	if err != nil {
		// TODO remove panic
		panic(err)
	}

	// Return a new primary instance with the active communication set up
	return &Primary{
		Server:            s,
		workloadGenerator: wg,
		benchmarkConfig:   bConfig,
		chainConfig:       cConfig,
	}
}

// Run provides the main functionality to run
// Holds the majority of the work
// TODO: under construction!
func (p *Primary) Run() {
	// First, set up the blockchain
	err := p.workloadGenerator.BlockchainSetup()

	if err != nil {
		zap.L().Error("encountered error with blockchain setup",
			zap.String("error", err.Error()))
		return
	}

	// Next, init the workload generator
	err = p.workloadGenerator.InitParams()
	if err != nil {
		zap.L().Error("encountered error with workloadgenerator InitParams",
			zap.String("error", err.Error()))
		return
	}

	// Get the secondary connections ready
	secondaryReadyChannel := make(chan bool, 1)
	go p.Server.HandleSecondaries(secondaryReadyChannel)
	<-secondaryReadyChannel
	close(secondaryReadyChannel)

	// Parse the config files
	// Run all preparation

	// Run through the benchmark suite
	// Step 1: send "PREPARE" to secondaries, make sure we can communicate.
	errs := p.Server.PrepareBenchmarkSecondaries(uint32(p.benchmarkConfig.Threads))

	if errs != nil {
		// We have errors
		p.Server.CloseSecondaries()
		p.Server.Close()
		zap.L().Error("Encountered errors in secondaries",
			zap.Strings("errors", errs))
	}

	// Number of secondaries connected
	zap.L().Info("Benchmark secondaries all connected.",
		zap.Int("secondaries", len(p.Server.Secondaries)))

	// Set up the blockchain information

	// Step 2: Blockchain type (tells which interface they should be using)
	// get the blockchain byte
	bcMessage, err := blockchains.MatchStringToMessage(p.chainConfig.Name)

	if err != nil {
		p.Server.CloseSecondaries()
		p.Server.Close()
	}

	errs = p.Server.SendBlockchainType(bcMessage)

	if errs != nil {
		zap.L().Error("failed to send blockchain type",
			zap.Strings("errors", errs))
		p.Server.CloseSecondaries()
		p.Server.Close()
		return
	}

	// Step 3: Prepare the workload for the benchmark
	// TODO: generate workloads
	workload, err := p.workloadGenerator.GenerateWorkload()

	if err != nil {
		zap.L().Error("failed to generate workload",
			zap.String("error", err.Error()))
	}

	// Step 4: Distribute benchmark
	errs = p.Server.SendWorkload(workload)
	if errs != nil {
		fmt.Println(errs)
	}

	// Step 5: run the bench
	errs = p.Server.RunBenchmark()
	if errs != nil {
		fmt.Println(errs)
	}

	// Wait until everyone is done
	time.Sleep(10 * time.Second)

	// Step 6 (once all have completed) - get the results
	// TODO: Need to store the results
	rawResults, errs := p.Server.GetResults()
	if errs != nil {
		fmt.Println(errs)
	}

	// TODO: @CHRIS
	aggregatedResults := results.CalculateAggregatedResults(rawResults)

	// Step 7 - store results
	p.Server.SendFin()

	time.Sleep(2 * time.Second)

	// Display the results
	results.Display(aggregatedResults)
	// Write the results to a file
	err = results.WriteResultsToFile(p.benchmarkConfig.Path, p.chainConfig.Path, aggregatedResults, "results")
	if err != nil {
		zap.L().Error("Encountered error when saving results",
			zap.Error(err))
	}

	// Step 8: Close all connections
	p.Server.CloseSecondaries()
	p.Server.Close()
}
