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

	InitApp() error
	Run(wg *sync.WaitGroup, results chan Result, stop chan struct{})
	DeliverWorkload(schedule Schedule) error
}

type Result interface {
	Start() time.Time
	End() time.Time
	Success() bool

	Type() string
	Empty() Result
	UnmarshalResults(buf []byte) ([]Result, error)
}

func Throughput(results []Result) int {
	if len(results) == 0 {
		return 0
	}

	minStart := results[0].Start()
	maxEnd := results[0].End()

	successes := 0
	for _, result := range results {
		if result.Success() {
			successes++
		}

		if result.Start().Before(minStart) {
			minStart = result.Start()
		}
		if result.End().After(maxEnd) {
			maxEnd = result.End()
		}
	}

	totalDuration := maxEnd.Sub(minStart).Seconds()

	if totalDuration == 0 {
		return 0
	}

	return int(float64(successes) / totalDuration)
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

/**
 * Workload type
 */

const (
	ScheduleInteractionsName = "schedule-interactions"
	ScheduleRatesName        = "schedule-rates"
)

type Schedule interface {
	Name() string
	Empty() Schedule
}

type ScheduleInteractions struct {
	Interactions   [][]byte    `json:"interactions"`
	ExecutionTimes []time.Time `json:"execution_times"`
}

func (ScheduleInteractions) Name() string {
	return ScheduleInteractionsName
}

func (ScheduleInteractions) Empty() Schedule {
	return &ScheduleInteractions{}
}

type ScheduleRates struct {
	StartTime time.Time `json:"start_time"` //0 if same as benchmark
	Rates     []Rate    `json:"rates"`
}

type Rate struct {
	Duration time.Duration `json:"duration"` //0 if it should run as long as possible
	Tps      int           `json:"tps"`
}

func (ScheduleRates) Name() string {
	return ScheduleRatesName
}

func (ScheduleRates) Empty() Schedule {
	return &ScheduleRates{}
}
