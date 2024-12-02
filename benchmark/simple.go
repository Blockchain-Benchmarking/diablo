package benchmark

import (
	"diablo/core"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/workload/behavior"
	"diablo/workload/payment"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"time"
)

const (
	defaultBlockchain = "ethereum"
)

var (
	defaultUser = User{
		Name: "stubbornPaymentUser",
		Params: map[string]interface{}{
			"timeout":      "15s",
			"max_attempts": 1,
			"random":       true,
			"payments":     []payment.Info{},
			"duration":     0,
		},
	}
)

type SimpleBenchmark struct {
	Blockchain string `yaml:"blockchain"`
	User       User   `yaml:"user"`
	Tps        int    // operand
}

type User struct {
	Name   string                 `yaml:"name"`
	Params map[string]interface{} `yaml:"params"`
}

func NewSimpleBenchmark(tps int) (Benchmark, error) {
	return &SimpleBenchmark{
		Blockchain: defaultBlockchain,
		User:       defaultUser,
		Tps:        tps,
	}, nil
}

func (s *SimpleBenchmark) Run(accounts []behavior.Account, _ time.Duration, secondaries map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	wk, err := createStubbornPaymentUsersFromAccounts(accounts, s.Tps, s.Blockchain, endpoints, s.User.Params)
	if err != nil {
		return fmt.Errorf("failed to create users: %w", err)
	}

	//same schedule for everyone
	logging.Infof("tps per user: %d", s.Tps/len(wk))
	perSecondary := len(wk) / len(secondaries)
	remainder := len(wk) % len(secondaries)

	i := 0
	for addr, secondary := range secondaries {
		n := perSecondary
		if remainder > 0 {
			n++
			remainder--
		}

		usersChunk := wk[i : i+n]
		logging.Infof("sending %d users to %s", len(usersChunk), addr)

		usersMap := make(map[string][]behavior.User)
		for _, u := range usersChunk {
			if usersMap[u.Name()] == nil {
				usersMap[u.Name()] = make([]behavior.User, 0)
			}
			usersMap[u.Name()] = append(usersMap[u.Name()], u)
		}

		err = coordinator.SendUsers(secondary, usersMap)
		if err != nil {
			return fmt.Errorf("failed to launch users to secondary %s: %w", addr, err)
		}

		i = i + n
	}

	err = coordinator.SendStartToAll(time.Now().Add(5 * time.Second)) //TODO report delay on secondary side
	if err != nil {
		return err
	}

	return nil
}

func createStubbornPaymentUsersFromAccounts(accounts []behavior.Account, tps int, blockchain string, endpoints []string, userParams map[string]interface{}) ([]behavior.User, error) {
	users := make([]behavior.User, len(accounts))
	userType := payment.StubbornPaymentUser{}

	var addresses []string
	for _, acc := range accounts {
		addresses = append(addresses, acc.Address)
	}

	tpsPerUser := tps / len(accounts)
	remainder := tps % len(accounts)

	var err error
	for i, acc := range accounts {
		userTps := tpsPerUser

		if remainder > 0 {
			userTps++
			remainder--
		}

		userParams["tps"] = tpsPerUser
		users[i], err = userType.New(blockchain, behavior.Config{
			Id:         strconv.Itoa(i),
			Endpoint:   endpoints[i%len(endpoints)],
			Addresses:  addresses,
			PrivateKey: acc.PrivateKey,
			Address:    acc.Address,
		}, userParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	return users, nil
}

func ParseSimpleConfig(b Benchmark, configPath string) error {
	buf, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read setup file failed: %w", err)
	}

	//Overwrite
	conf := &SimpleBenchmark{}
	err = yaml.Unmarshal(buf, conf)
	if err != nil {
		return fmt.Errorf("error parsing config yaml: %w", err)
	}

	if conf.Blockchain != "" {
		b.(*SimpleBenchmark).Blockchain = conf.Blockchain
		logging.Debugf("overwrite blockchain")
	}

	if conf.User.Name != "" && conf.User.Params != nil {
		b.(*SimpleBenchmark).User = conf.User
		logging.Debugf("overwrite user")
	}

	return nil
}
