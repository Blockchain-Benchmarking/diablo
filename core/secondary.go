package core

import (
	"diablo/core/messaging"
	"diablo/core/remote"
	"diablo/core/workload"
	"fmt"
	"net"
)

type Secondary struct {
	ConnectAddr string
	Tags        []string

	PrimaryConn   *remote.PrimaryConn
	PrimaryParams *messaging.MsgPrimaryParameters
}

func NewSecondary(primary string, port int, tags []string) (*Secondary, error) {
	return &Secondary{
		ConnectAddr: fmt.Sprintf("%s:%d", primary, port),
		Tags:        tags,
	}, nil
}

func (s *Secondary) Run() error {
	Debugf("connect to primary on tcp address: %s", s.ConnectAddr)
	conn, err := net.Dial("tcp", s.ConnectAddr)
	if err != nil {
		return fmt.Errorf("cannot connect to tcp %s: %s",
			s.ConnectAddr, err.Error())
	} else {
		Debugf("connected to primary on tcp address: %s",
			conn.RemoteAddr().String())
	}

	s.PrimaryConn = remote.NewPrimaryConn(conn)

	s.PrimaryParams, err = s.PrimaryConn.Init(&messaging.MsgSecondaryParameters{
		Tags: s.Tags,
	})

	if err != nil {
		return err
	}

	//wait for the coordinator to send the workload
	wk := &workload.UserInfoWorkload{}
	err = wk.Decode(s.PrimaryConn.Reader())
	if err != nil {
		return err
	}

	//create users
	//TODO use interface
	var generator workload.Generator
	generator, err = workload.NewSimpleGenerator(s.PrimaryConn, wk)
	if err != nil {
		return err
	}

	//send ready signal
	err = s.PrimaryConn.SyncReady()
	if err != nil {
		return err
	}

	//wait for start signal
	_, err = s.PrimaryConn.WaitStart() //TODO consider duration
	if err != nil {
		return err
	}

	//start generator
	err = generator.Start()
	if err != nil {
		return err
	}

	//collect results somehow
	_, err = generator.CollectResults()
	if err != nil {
		return err
	}

	//send results somehow to primary

	return nil
}
