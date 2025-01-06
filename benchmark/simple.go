package benchmark

import (
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/workload/behavior"
	"diablo/workload/payment"
	"diablo/workload/store"
	"encoding/hex"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

var userTypes = map[string]User{
	"stubbornPaymentUser": {
		Name: "stubbornPaymentUser",
		Params: map[string]interface{}{
			"timeout":      "15s",
			"max_attempts": 5,
			"random":       true,
			"payments":     []payment.Info{},
			"duration":     0,
		},
	},
	"stubbornStoreUser": {
		Name: "stubbornStoreUser",
		Params: map[string]interface{}{
			"timeout":      "15s",
			"max_attempts": 1,
			"random":       true,
			"actions":      []store.Info{},
			"duration":     0,
		},
	},
}

type SimpleBenchmark struct {
	Blockchain string `yaml:"blockchain"`
	User       User   `yaml:"user"`
	Tps        int
}

type User struct {
	Name   string                 `yaml:"name"`
	Params map[string]interface{} `yaml:"params"`
}

func NewSimpleBenchmark(tps int, userType string, blockchain string) (Benchmark, error) {
	def, ok := userTypes[userType]
	if !ok {
		return nil, fmt.Errorf("user type %s not defined for simple benchmark", userType)
	}

	return &SimpleBenchmark{
		Blockchain: blockchain,
		User:       def,
		Tps:        tps,
	}, nil
}

func (s *SimpleBenchmark) Run(accounts []blockchain.Account, secondaries map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	wk, err := createStubbornUsersFromAccounts(accounts, s.Tps, s.Blockchain, endpoints, s.User)
	if err != nil {
		return fmt.Errorf("failed to create users: %w", err)
	}

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

	startTime := time.Now().Add(5 * time.Second)
	err = coordinator.SendStartToAll(startTime)
	if err != nil {
		return err
	}
	return nil
}

func createStubbornUsersFromAccounts(accounts []blockchain.Account, tps int, implementation string, endpoints []string, user User) ([]behavior.User, error) {
	var addresses []string
	for _, acc := range accounts {
		addresses = append(addresses, acc.Address)
	}

	users := make([]behavior.User, len(accounts))
	userType, ok := core.Users[user.Name]
	if !ok {
		return nil, fmt.Errorf("user %s not implemented", user.Name)
	}

	contractPaths := userType.ContractPaths()
	if contractPaths != nil {
		paths, ok := contractPaths[implementation]
		if !ok {
			return nil, fmt.Errorf("blockchain %s not implemented", implementation)
		}

		b, ok := blockchain.Blockchains[implementation]
		if !ok {
			return nil, fmt.Errorf("implementation %s not found", implementation)
		}

		bl, err := b.New(blockchain.Config{
			Id:         "0",
			Endpoint:   endpoints[0],
			Addresses:  addresses,
			PrivateKey: accounts[0].PrivateKey,
			Address:    accounts[0].Address,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create blockchain client for contract deployment: %w", err)
		}

		abiBytes, err := os.ReadFile(paths.AbiPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", paths.AbiPath, err)
		}

		compiledHex, err := os.ReadFile(paths.BinaryPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", paths.BinaryPath, err)
		}

		compiledBytes, err := hex.DecodeString(string(compiledHex))
		if err != nil {
			return nil, fmt.Errorf("failed to decode file %s: %w", paths.BinaryPath, err)
		}

		contractAddress, err := bl.DeployContract(string(abiBytes), compiledBytes, time.Minute, "1.0.0")
		if err != nil {
			return nil, fmt.Errorf("failed to deploy contract %s: %w", paths.BinaryPath, err)
		}

		logging.Infof("deployed contract at address: %s", contractAddress)

		user.Params["contractAddress"] = contractAddress
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

		user.Params["tps"] = userTps
		users[i], err = userType.New(implementation, blockchain.Config{
			Id:         acc.Address,
			Endpoint:   endpoints[i%len(endpoints)],
			Addresses:  addresses,
			PrivateKey: acc.PrivateKey,
			Address:    acc.Address,
		}, user.Params)
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
