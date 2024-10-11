package workload

import "io"

type Coordinator interface {
	SendWorkload() error
	CollectResults() Results
}

type Generator interface {
	Start() error
	CollectResults() (Results, error)
}

type Results interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
	//Merge(other Results) Results
	PrintResult(dest io.Writer) error
}
