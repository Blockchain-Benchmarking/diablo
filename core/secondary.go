package core

import (
	"diablo/core/logging"
	"diablo/core/remote"
	"diablo/core/workload"
	"fmt"
	"net"
	"time"
)

type Secondary struct {
	workload.Generator

	ConnectAddr string
	Tags        []string

	PrimaryConn   *remote.PrimaryConn
	PrimaryParams *remote.MsgPrimaryParameters
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

	s.PrimaryConn = remote.NewPrimaryConn(conn)

	logging.Debugf("send secondary parameters")
	s.PrimaryParams, err = s.PrimaryConn.Init(&remote.MsgSecondaryParameters{
		Tags: s.Tags,
	})

	if err != nil {
		return err
	}

	logging.Debugf("wait for workload")
	//wait for the coordinator to send the workload
	wk := &workload.SimpleWorkload{}
	err = wk.Decode(s.PrimaryConn.Reader())
	if err != nil {
		return err
	}

	//create users
	//TODO use interface
	s.Generator, err = workload.NewSimpleGenerator(s.PrimaryConn, wk, 5*time.Minute) //TODO
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
	err = s.Start()
	if err != nil {
		return err
	}

	//send results to primary
	results, err := s.CollectResults()
	if err != nil {
		return err
	}

	err = results.Encode(s.PrimaryConn.Writer())
	if err != nil {
		return err
	}

	return s.PrimaryConn.Writer().Flush()
}
