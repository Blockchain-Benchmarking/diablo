package workload

import (
	"diablo/core/remote"
	"diablo/core/user"
	"io"
	"time"
)

var Workloads = map[string]Tuple{
	"simple": {
		NewSimpleCoordinator,
		NewSimpleGenerator,
	},
}

var Users = map[string]func(account user.Account, others []string, app string, blockchain string) (user.User, error){
	"stubborn": user.CreateStubbornUser,
}

type Tuple struct {
	NewCoordinator func(secondaries []*remote.Secondary, accounts []user.Account, userType string, app string, blockchain string) Coordinator
	NewGenerator   func(primary *remote.PrimaryConn, wk Workload, timeout time.Duration) (Generator, error)
}

type Generator interface {
	Start() error
	CollectResults() (user.Results, error)
}

type Coordinator interface {
	SendWorkload() error
	CollectResults() user.Results
}

type Workload interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
}
