package benchmark

import (
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/logging"
	"diablo/core/network"
	"diablo/workload/behavior"
	"diablo/workload/payment"
	"diablo/workload/store"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"time"
)

const (
	defaultBlockchain = "ethereum"
)

var userTypes = map[string]User{
	"stubbornPaymentUser": {
		Name: "stubbornPaymentUser",
		Params: map[string]interface{}{
			"timeout":      "15s",
			"max_attempts": 1,
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
		CompiledContractPath: "workload/store/contract/StoreCompiled",
		AbiPath:              "workload/store/contract/Store.abi",
	},
}

type SimpleBenchmark struct {
	Blockchain string `yaml:"blockchain"`
	User       User   `yaml:"user"`
	Tps        int    // operand
}

type User struct {
	Name                 string                 `yaml:"name"`
	Params               map[string]interface{} `yaml:"params"`
	CompiledContractPath string
	AbiPath              string
}

func NewSimpleBenchmark(tps int, userType string) (Benchmark, error) {
	def, ok := userTypes[userType]
	if !ok {
		return nil, fmt.Errorf("user type %s not defined for simple benchmark", userType)
	}

	return &SimpleBenchmark{
		Blockchain: defaultBlockchain,
		User:       def,
		Tps:        tps,
	}, nil
}

func (s *SimpleBenchmark) Run(accounts []blockchain.Account, d time.Duration, secondaries map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	wk, err := createStubbornUsersFromAccounts(accounts, s.Tps, s.Blockchain, endpoints, s.User)
	if err != nil {
		return fmt.Errorf("failed to create users: %w", err)
	}

	//same schedule for everyone
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

	startTime := time.Now().Add(5 * time.Second)
	err = coordinator.SendStartToAll(startTime) //TODO report delay on secondary side
	if err != nil {
		return err
	}

	time.Sleep(d + 10*time.Second)

	res, _ := coordinator.CollectResultsWithInterval(startTime, startTime.Add(d))
	latency := behavior.AverageLatency(res)
	throughput := behavior.Throughput(res, d)
	logging.Infof("Average Latency: %s, Throughput: %d/s", latency.String(), throughput)

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

	if user.AbiPath != "" && user.CompiledContractPath != "" {
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

		abiBytes, err := os.ReadFile(user.AbiPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", user.AbiPath, err)
		}

		compiledBytes, err := os.ReadFile(user.CompiledContractPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", user.CompiledContractPath, err)
		}

		contractAddress, err := bl.DeployContract(string(abiBytes), []byte("0x"+string(compiledBytes)))
		if err != nil {
			return nil, fmt.Errorf("failed to deploy contract %s: %w", user.CompiledContractPath, err)
		}

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
			Id:         strconv.Itoa(i),
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
