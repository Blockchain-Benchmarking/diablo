package core

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type Setup struct {
	Interface string     `yaml:"interface"`
	User      User       `yaml:"user"`
	Workload  Workload   `yaml:"workload"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

type Workload struct {
	Name   string
	Params map[string]string `yaml:"params"`
}

type User struct {
	Name   string                 `yaml:"name"`
	Params map[string]interface{} `yaml:"params"`
}

type Endpoint struct {
	Addresses []string `yaml:"addresses"`
	Tags      []string `yaml:"tags"`
}

func ParseSetup(file string) (*Setup, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read setup file: %w", err)
	}

	var setup Setup
	err = yaml.Unmarshal(buf, &setup)
	if err != nil {
		return nil, fmt.Errorf("failed to parse setup file: %w", err)
	}

	return &setup, nil
}
