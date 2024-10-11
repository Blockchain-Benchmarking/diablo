package workload

import (
	"diablo/core/logging"
	"diablo/core/remote"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"io"
	"sync"
	"time"
)

type SimpleGenerator struct {
	primary *remote.PrimaryConn
	users   []User
	wg      *sync.WaitGroup
	results chan interface{}
	timeout time.Duration
}

type SimpleResult []UserResult

func (s *SimpleResult) Encode(dest io.Writer) error {
	logging.Infof("writing length %d", int32(len(*s)))
	if err := binary.Write(dest, binary.LittleEndian, int32(len(*s))); err != nil {
		return err
	}

	for _, userResult := range *s {
		if err := binary.Write(dest, binary.LittleEndian, int32(len(userResult))); err != nil {
			return err
		}

		for _, action := range userResult {
			if err := binary.Write(dest, binary.LittleEndian, action.SubmitTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.CommitTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.AbortTime); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.retries); err != nil {
				return err
			}
			if err := binary.Write(dest, binary.LittleEndian, action.HasError); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SimpleResult) Decode(src io.Reader) error {
	var resultLength int32
	if err := binary.Read(src, binary.LittleEndian, &resultLength); err != nil {
		return fmt.Errorf("failed to read result length: %w", err)
	}

	*s = make(SimpleResult, resultLength)

	for i := int32(0); i < resultLength; i++ {
		var userResultLength int32
		if err := binary.Read(src, binary.LittleEndian, &userResultLength); err != nil {
			return fmt.Errorf("failed to read user result length at %d: %w", i, err)
		}

		userResult := make(UserResult, userResultLength)

		for j := int32(0); j < userResultLength; j++ {
			action := &Action{}

			if err := binary.Read(src, binary.LittleEndian, &action.SubmitTime); err != nil {
				return fmt.Errorf("failed to read submit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.CommitTime); err != nil {
				return fmt.Errorf("failed to read commit time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.AbortTime); err != nil {
				return fmt.Errorf("failed to read abort time at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.retries); err != nil {
				return fmt.Errorf("failed to read retries at %d, %d: %w", i, j, err)
			}
			if err := binary.Read(src, binary.LittleEndian, &action.HasError); err != nil {
				return fmt.Errorf("failed to read has error at %d, %d: %w", i, j, err)
			}

			userResult[j] = action
		}

		(*s)[i] = userResult
	}

	return nil
}

func (s *SimpleResult) Merge(other *SimpleResult) *SimpleResult {
	result := append(*s, *other...)

	return &result
}

type UserResult []*Action

type Action struct {
	SubmitTime int64 // 0 if not submitted
	CommitTime int64 // 0 if not committed
	AbortTime  int64 // 0 if not aborted
	retries    int32
	HasError   bool
}

func (s *SimpleResult) PrintResult(dest io.Writer) error {
	buf, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshall result: %w", err)
	}

	_, err = dest.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}

func NewSimpleGenerator(primary *remote.PrimaryConn, wk *UserInfoWorkload, timeout time.Duration) (*SimpleGenerator, error) {
	users := make([]User, len(wk.UserInfos))
	for i, u := range wk.UserInfos {
		//TODO endpoint
		client, err := ethclient.Dial("ws://127.0.0.1:9000")
		if err != nil {
			return nil, err
		}

		logging.Debugf("assigning account with private key length: %d", len(u.Account.PrivateKey))
		users[i] = &SimpleUser{
			client: client,
			info:   u,
		}
	}

	return &SimpleGenerator{primary, users, &sync.WaitGroup{}, make(chan interface{}, len(users)), timeout}, nil
}

func (g *SimpleGenerator) Start() error {
	g.wg.Add(len(g.users))
	for _, user := range g.users {
		go user.Run(g.wg, g.results)
	}

	return nil
}

func (g *SimpleGenerator) CollectResults() (Results, error) {
	//TODO implement me
	logging.Infof("waiting for users to finish executing")
	g.wg.Wait()
	close(g.results)
	logging.Infof("users finished executing")

	var results SimpleResult
	for result := range g.results {
		logging.Infof("received results")
		if userResult, ok := result.(UserResult); ok {
			results = append(results, userResult)
		} else {
			logging.Warnf("unknown result type received")
		}
	}

	return &results, nil
}
