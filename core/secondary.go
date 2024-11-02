package core

import (
	"diablo/core/logging"
	"diablo/core/messaging"
	"diablo/core/network"
	"diablo/core/workload"
	"fmt"
	"net"
	"sync"
)

type Secondary struct {
	ConnectAddr string
	Tags        []string

	PrimaryConn   *network.PrimaryConn
	PrimaryParams *messaging.PrimaryInit
}

func NewSecondary(primary string, tags []string) (*Secondary, error) {
	return &Secondary{
		ConnectAddr: primary,
		Tags:        tags,
	}, nil
}

func (s *Secondary) Run() error {
	logging.Debugf("connect to primary on tcp address: %s", s.ConnectAddr)
	conn, err := net.Dial("tcp", s.ConnectAddr)
	if err != nil {
		return fmt.Errorf("cannot connect to tcp %s: %s",
			s.ConnectAddr, err.Error())
	} else {
		logging.Debugf("connected to primary on tcp address: %s",
			conn.RemoteAddr().String())
	}

	s.PrimaryConn = network.NewPrimaryConn(conn)

	logging.Debugf("wait for primary parameters at address %s", s.PrimaryConn.LocalAddr())

	//read init message from primary connection
	msg, err := s.PrimaryConn.Read()
	if err != nil {
		return fmt.Errorf("cannot read from primary connection: %w", err)
	}

	primaryMsg, ok := msg.(*messaging.PrimaryInit)
	if !ok {
		return fmt.Errorf("primary init message type got %s", msg.Type())
	}

	logging.Debugf("primary init message received")

	g, err := workload.NewGenerator(s.PrimaryConn, primaryMsg.Duration)
	if err != nil {
		return fmt.Errorf("failed to create generator: %w", err)
	}

	logging.Debugf("running generator")

	//create generator
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go g.Run(wg)

	logging.Infof("sending secondary init message")

	//send init message back to primary
	secondaryMsg := messaging.SecondaryInit{Tags: s.Tags}
	err = s.PrimaryConn.Send(&secondaryMsg)
	if err != nil {
		return fmt.Errorf("failed to send secondary init message: %w", err)
	}

	logging.Infof("waiting for generator to stop")

	wg.Wait()

	logging.Infof("secondary done")

	return nil
}
