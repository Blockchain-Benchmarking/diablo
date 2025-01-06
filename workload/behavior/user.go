package behavior

import (
	"diablo/blockchain"
	"diablo/core/logging"
	"time"
)

type User interface {
	Empty() User
	UnmarshalUsers(buf []byte) ([]User, error)

	//EmptyResult should return an empty Result structure
	EmptyResult() Result

	New(blockchain string, config blockchain.Config, params map[string]interface{}) (User, error)

	Name() string
	ID() string

	Run(results chan Result, stop chan struct{})
	Restart(info RestartInfo) error

	ContractPaths() map[string]ContractInfo
}

type RestartInfo struct {
	NewParameters User
	RestartTime   time.Time
}

type ContractInfo struct {
	AbiPath    string
	BinaryPath string
}

type Result interface {
	Start() time.Time
	End() time.Time
	Success() bool
	GetID() string

	Type() string
	Empty() Result
	UnmarshalResults(buf []byte) ([]Result, error)
}

// Throughput calculates the throughput over the given duration
func Throughput(results []Result, duration time.Duration) int {
	if len(results) == 0 {
		return 0
	}

	successes := 0
	for _, result := range results {
		if result.Success() {
			successes++
		}
	}

	return int(float64(successes) / duration.Seconds())
}

// AverageLatency calculates the average latency in seconds
func AverageLatency(results []Result) time.Duration {
	if results == nil || len(results) == 0 {
		logging.Warnf("can't calculate average latency for empty results")
		return 0
	}

	sum := time.Duration(0)
	total := 0
	for _, result := range results {
		if result.Success() {
			sum += result.End().Sub(result.Start())
			total++
		}
	}

	if total == 0 {
		return 0
	}

	return sum / time.Duration(total)
}
