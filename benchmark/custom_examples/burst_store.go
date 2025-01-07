package benchmark

import (
	"diablo/benchmark"
	"diablo/blockchain"
	"diablo/core"
	"diablo/core/network"
	"diablo/workload/behavior"
	"fmt"
	"time"
)

type BurstStore struct{}

func (c *BurstStore) Run(accounts []blockchain.Account, secondaries map[string]*network.Secondary, coordinator *core.Coordinator, endpoints []string) error {
	defaultStubbornPaymentUser, ok := benchmark.UserTypes["stubbornPaymentUser"]
	if !ok {
		return fmt.Errorf("stubbornPaymentUser not implemented")
	}

	wk, err := benchmark.CreateStubbornUsersFromAccounts(accounts[:300], 600, "ethereum", endpoints, defaultStubbornPaymentUser)
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

	burstStoreUser, ok := benchmark.UserTypes["stubbornStoreUser"]
	if !ok {
		return fmt.Errorf("stubbornStoreUser not implemented")
	}

	burstStoreUser.Params["duration"] = "30s"
	wk, err = benchmark.CreateStubbornUsersFromAccounts(accounts[300:], 3000, "ethereum", endpoints, burstStoreUser)
	if err != nil {
		return fmt.Errorf("failed to create users: %w", err)
	}

	//start payment users
	startTime := time.Now().Add(5 * time.Second)
	err = coordinator.SendStartToAll(startTime)
	if err != nil {
		return err
	}

	perSecondary = len(wk) / len(secondaries)
	remainder = len(wk) % len(secondaries)

	i = 0
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

	startTime = startTime.Add(time.Minute)
	err = coordinator.SendStartToAll(startTime)
	if err != nil {
		return err
	}

	time.Sleep(2*time.Minute + 30*time.Second)

	err = coordinator.SendStopToAll()
	if err != nil {
		return err
	}

	return nil
}
