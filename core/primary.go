package core

import (
	"diablo/core/remote"
	"diablo/core/result"
	"diablo/core/workload"
	"fmt"
	"gopkg.in/yaml.v3"
	"net"
	"os"
)

type Primary struct {
	NumSecondary int
	Setup        setup
	Accounts     []workload.Account
	ListenPort   int
	Coordinator  workload.Coordinator
}

func NewPrimary(port int, secondary int, setupPath string, accountsPath string) (*Primary, error) {
	Debugf("parse setup file '%s'", setupPath)
	newSetup, err := parseSetupYamlPath(setupPath)
	if err != nil {
		return nil, err
	}

	accBytes, err := os.ReadFile(accountsPath)
	if err != nil {
		return nil, err
	}

	var accounts []workload.Account
	err = yaml.Unmarshal(accBytes, &accounts)
	if err != nil {
		return nil, err
	}

	Debugf("using interface '%s'", newSetup.sysname())

	return &Primary{
		NumSecondary: secondary,
		ListenPort:   port,
		Setup:        newSetup,
		Accounts:     accounts,
	}, nil
}

func (p *Primary) Run() (result.Result, error) {
	//accept secondary connections
	Debugf("wait for %d secondary connections", p.NumSecondary)
	secondaries, err := p.acceptSecondaries()
	if err != nil {
		return nil, err
	}

	//Coordinator sends workload to secondaries
	p.Coordinator = workload.New(secondaries, p.Accounts)
	err = p.Coordinator.SendWorkload()
	if err != nil {
		return nil, err
	}

	//wait for secondaries to ack they are ready
	for i := range secondaries {
		Tracef("wait for secondary %s", secondaries[i].Addr())
		err := secondaries[i].Ready()
		if err != nil {
			return nil, err
		}
	}

	//send start signal
	Infof("start benchmark")
	for i := range secondaries {
		Tracef("send start signal to %s", secondaries[i].Addr())
		err := secondaries[i].Start(100000) //TODO
		if err != nil {
			return nil, err
		}
	}

	//Coordinator collects results
	p.Coordinator.CollectResults()

	return nil, nil
}

func (p *Primary) acceptSecondaries() ([]*remote.Secondary, error) {
	var laddr, raddr, tag string
	var remoteSecondaries []*remote.Secondary
	var listener net.Listener
	var conn net.Conn
	var err error
	var done bool
	var i int

	laddr = fmt.Sprintf("0.0.0.0:%d", p.ListenPort)
	remoteSecondaries = make([]*remote.Secondary, p.NumSecondary)

	Debugf("listen for %d secondary connections on %s", len(remoteSecondaries), laddr)
	listener, err = net.Listen("tcp", laddr)
	if err != nil {
		return nil, err
	}

	done = false

	defer func() {
		Debugf("close listener on %s", laddr)
		listener.Close()

		if done {
			return
		}

		for i = range remoteSecondaries {
			if remoteSecondaries[i] == nil {
				continue
			}

			Debugf("close connection from %s", remoteSecondaries[i].Addr())
			remoteSecondaries[i].Close()
		}
	}()

	for i = range remoteSecondaries {
		Tracef("wait for connection on %s", laddr)
		conn, err = listener.Accept()
		if err != nil {
			return nil, err
		}

		raddr = conn.RemoteAddr().String()
		Debugf("new secondary connection from %s", raddr)

		remoteSecondaries[i], err = remote.NewRemoteSecondary(conn, p.Setup.sysname(), p.Setup.parameters())
		if err != nil {
			conn.Close()
			return nil, err
		}

		Tracef("secondary %s tags:", raddr)
		for _, tag = range remoteSecondaries[i].Tags() {
			Tracef("  %s", tag)
		}
	}

	done = true

	return remoteSecondaries, nil
}
