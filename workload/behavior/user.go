package behavior

import (
	"diablo/core/logging"
	"sync"
	"time"
)

type User interface {
	Empty() User
	UnmarshalUsers(buf []byte) ([]User, error)
	EmptyResult() Result
	New(blockchain string, config Config, params map[string]interface{}) (User, error)

	Name() string
	ID() string

	Run(wg *sync.WaitGroup, results chan Result, stop chan struct{})
	Restart(info RestartInfo) error
}

type RestartInfo struct {
	NewParameters User
	RestartTime   time.Time
}

type Result interface {
	Start() time.Time
	End() time.Time
	Success() bool

	Type() string
	Empty() Result
	UnmarshalResults(buf []byte) ([]Result, error)
}

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

	logging.Infof("total success %d/%d, total latency %s, result is %s", total, len(results), sum.String(), (sum / time.Duration(total)).String())
	return sum / time.Duration(total)
}

type Account struct {
	Address    string `yaml:"address"`
	PrivateKey string `yaml:"private"`
}

type Config struct {
	Id         string   `json:"id"`
	Endpoint   string   `json:"endpoint"`
	Addresses  []string `json:"addresses"`
	PrivateKey string   `json:"private_key"`
	Address    string   `json:"address"`
}
