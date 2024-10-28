package core

import (
	"diablo/core/behavior"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/core/workload"
	"fmt"
	"gopkg.in/yaml.v3"
	"net"
	"os"
	"strings"
)

type Primary struct {
	workload.Coordinator

	NumSecondary int
	Setup        *Setup
	Accounts     []behavior.Account
	ListenPort   int
}

func NewPrimary(port int, secondary int, setupPath string, accountsPath string) (*Primary, error) {
	logging.Debugf("parse setup file '%s'", setupPath)
	newSetup, err := ParseSetup(setupPath)
	if err != nil {
		return nil, err
	}

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
		Setup:        newSetup,
		Accounts:     accounts,
	}, nil
}

func (p *Primary) Run() (behavior.Results, error) {
	logging.Debugf("wait for %d secondary connections", p.NumSecondary)
	secondaries, err := p.acceptSecondaries()
	if err != nil {
		return nil, err
	}

	// select coordinator and send workload
	t, ok := workload.Workloads[p.Setup.Workload.Name]
	if !ok {
		return nil, fmt.Errorf("coordinator for workload '%s' not found", p.Setup.Workload)
	}

	res, err := t.Coordinator.Run(secondaries, p.Accounts, p.Setup.User.Name, p.Setup.Interface, p.Setup.User.Params, p.Setup.Workload.Params)
	if err != nil {
		return nil, fmt.Errorf("failed during coordinator run: %w", err)
	}

	for _, sec := range secondaries {
		err := sec.Close()
		if err != nil {
			logging.Errorf("failed to close connection %s: %s", sec.Addr(), err.Error())
		}
	}

	return res, nil
}

func (p *Primary) acceptSecondaries() ([]*network.Secondary, error) {
	var laddr, raddr string
	var remoteSecondaries []*network.Secondary
	var listener net.Listener
	var conn net.Conn
	var err error
	var done bool
	var i int

	laddr = fmt.Sprintf("0.0.0.0:%d", p.ListenPort)
	remoteSecondaries = make([]*network.Secondary, p.NumSecondary)

	logging.Debugf("listen for %d secondary connections on %s", len(remoteSecondaries), laddr)
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

		for i = range remoteSecondaries {
			if remoteSecondaries[i] == nil {
				continue
			}

			logging.Debugf("close connection from %s", remoteSecondaries[i].Addr())
			remoteSecondaries[i].Close()
		}
	}()

	for i = range remoteSecondaries {
		logging.Tracef("wait for connection on %s", laddr)
		conn, err = listener.Accept()
		if err != nil {
			return nil, err
		}

		raddr = conn.RemoteAddr().String()
		logging.Debugf("new secondary connection from %s", raddr)

		remoteSecondaries[i], err = network.NewRemoteSecondary(conn, p.Setup.Workload.Name)
		if err != nil {
			conn.Close()
			return nil, err
		}

		logging.Tracef("secondary %s tags: [%s]", raddr, strings.Join(remoteSecondaries[i].Tags(), ", "))
	}

	done = true

	return remoteSecondaries, nil
}
