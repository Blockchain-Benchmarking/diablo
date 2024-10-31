package core

import (
	"diablo/core/behavior"
	"diablo/core/benchmark"
	"diablo/core/logging"
	"diablo/core/network"
	"fmt"
	"gopkg.in/yaml.v3"
	"net"
	"os"
	"strings"
	"time"
)

type Primary struct {
	NumSecondary int
	SetupFile    string
	Accounts     []behavior.Account
	ListenPort   int
	Benchmark    string
	Duration     time.Duration
}

func NewPrimary(port int, secondary int, benchmark string, setupPath string, accountsPath string, duration time.Duration) (*Primary, error) {
	logging.Debugf("parse setup file '%s'", setupPath)

	accBytes, err := os.ReadFile(accountsPath)
	if err != nil {
		return nil, err
	}

	var accounts []behavior.Account
	err = yaml.Unmarshal(accBytes, &accounts)
	if err != nil {
		return nil, err
	}

	return &Primary{
		NumSecondary: secondary,
		ListenPort:   port,
		SetupFile:    setupPath,
		Accounts:     accounts,
		Benchmark:    benchmark,
		Duration:     duration,
	}, nil
}

func (p *Primary) Run() (behavior.Results, error) {
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

	c, ok := benchmark.Benchmarks[p.Benchmark]
	if !ok {
		return nil, fmt.Errorf("could not find benchmark %s", p.Benchmark)
	}

	b, err := c(p.SetupFile, secondaries)
	if err != nil {
		return nil, err
	}

	res, err := b.Run(p.Accounts, p.Duration)
	if err != nil {
		return nil, fmt.Errorf("failed during benchmark run: %w", err)
	}

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
