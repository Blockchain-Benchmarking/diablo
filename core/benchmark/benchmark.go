package benchmark

import (
	"diablo/core/behavior"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

type Benchmark interface {
	Run(accounts []behavior.Account, duration time.Duration) (behavior.Results, error)
	StopCoordinator()
}

func ParseSetup(setupPath string, setup interface{}) error {
	buf, err := os.ReadFile(setupPath)
	if err != nil {
		return fmt.Errorf("read setup file failed: %w", err)
	}

	err = yaml.Unmarshal(buf, setup)
	if err != nil {
		return fmt.Errorf("error parsing setup yaml: %w", err)
	}

	return nil
}
