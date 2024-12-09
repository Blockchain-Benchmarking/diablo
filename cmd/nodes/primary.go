package nodes

import (
	"diablo/benchmark"
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/workload/behavior"
	"fmt"
	"gopkg.in/yaml.v3"
	"net"
	"os"
	"strings"
	"time"
)

type Primary struct {
	NumSecondary int
	ConfigFile   string
	Accounts     []blockchain.Account
	ListenPort   int
	Duration     time.Duration

	Benchmark string
	//if simple benchmark:
	Tps       int
	Endpoints []string
	User      string
}

func NewPrimary(port int, secondary int, benchmark string, configPath string, accountsPath string, duration time.Duration, tps int, user string, endpoints []string) (*Primary, error) {
	accBytes, err := os.ReadFile(accountsPath)
	if err != nil {
		return nil, err
	}

	var accounts []blockchain.Account
	err = yaml.Unmarshal(accBytes, &accounts)
	if err != nil {
		return nil, err
	}

	return &Primary{
		NumSecondary: secondary,
		ListenPort:   port,
		ConfigFile:   configPath,
		Accounts:     accounts,
		Duration:     duration,

		Benchmark: benchmark,
		Tps:       tps,
		Endpoints: endpoints,
		User:      user,
	}, nil
}

func (p *Primary) Run() ([]behavior.Result, error) {
	logging.Debugf("wait for %d secondary connections", p.NumSecondary)
	secondaries, err := p.acceptSecondaries()
	if err != nil {
		return nil, err
	}

	defer func() {
		for _, sec := range secondaries {
			err := sec.Close()
			if err != nil {
				logging.Errorf("failed to close connection %s: %s", sec.Addr(), err.Error())
			}
		}
	}()

	var b benchmark.Benchmark
	switch p.Benchmark {
	case "simple":
		b, err = benchmark.NewSimpleBenchmark(p.Tps, p.User)
		if err != nil {
			return nil, fmt.Errorf("failed to create benchmark: %s", err)
		}

		if p.ConfigFile != "" {
			err = benchmark.ParseSimpleConfig(b, p.ConfigFile)
			if err != nil {
				return nil, fmt.Errorf("failed to parse config file: %s", err)
			}
		}

	case "custom":
		b = &benchmark.CustomBenchmark{}
	default:
		return nil, fmt.Errorf("unknown benchmark: %s", p.Benchmark)
	}

	coordinator := core.NewCoordinator(secondaries)
	err = b.Run(p.Accounts, p.Duration, secondaries, coordinator, p.Endpoints)
	if err != nil {
		return nil, fmt.Errorf("failed during benchmark run: %w", err)
	}

	res := coordinator.CollectResults()
	coordinator.Stop()

	return res, nil
}

func (p *Primary) acceptSecondaries() (map[string]*network.Secondary, error) {
	var listener net.Listener
	var conn net.Conn
	var err error
	var done bool

	laddr := fmt.Sprintf("0.0.0.0:%d", p.ListenPort)
	remoteSecondaries := make(map[string]*network.Secondary, p.NumSecondary)

	logging.Debugf("listen for %d secondary connections on %s", p.NumSecondary, laddr)
	listener, err = net.Listen("tcp", laddr)
	if err != nil {
		return nil, err
	}

	done = false

	defer func() {
		logging.Debugf("close listener on %s", laddr)
		listener.Close()

		if done {
			return
		}

		for addr, s := range remoteSecondaries {
			if s == nil {
				continue
			}

			logging.Debugf("close connection from %s", addr)
			s.Close()
		}
	}()

	for i := 0; i < p.NumSecondary; i++ {
		logging.Tracef("wait for connection on %s", laddr)
		conn, err = listener.Accept()
		if err != nil {
			return nil, err
		}

		raddr := conn.RemoteAddr().String()
		logging.Debugf("new secondary connection from %s", raddr)

		remoteSecondaries[conn.RemoteAddr().String()], err = network.NewRemoteSecondary(conn, p.Duration)
		if err != nil {
			conn.Close()
			return nil, err
		}

		logging.Tracef("secondary %s tags: [%s]", raddr, strings.Join(remoteSecondaries[conn.RemoteAddr().String()].Tags(), ", "))
	}

	done = true

	return remoteSecondaries, nil
}
