package nbsv

import (
	"diablo-benchmark/core"
	"fmt"
	"strings"

	"github.com/bsv-blockchain/go-sdk/transaction/broadcaster"
)

// BlockchainInterface is the entry point registered in diablo.go under the
// "bsv" key. Build a benchmark against Teranode + arcade by pointing the setup
// endpoints at the arcade (ARC) HTTP base URL.
type BlockchainInterface struct {
}

// Builder runs on the primary. The `env` slice carries `keys=<path>` entries
// pointing at the pre-funded keys.yaml (see keys.go).
func (this *BlockchainInterface) Builder(params map[string]string, env []string, endpoints map[string][]string, logger core.Logger) (core.BlockchainBuilder, error) {
	logger.Debugf("new bsv builder")

	if err := configureFee(params); err != nil {
		return nil, err
	}

	envmap, err := parseEnvmap(env)
	if err != nil {
		return nil, err
	}

	builder := newBuilder(logger)

	for key, values := range envmap {
		if key == "keys" {
			for _, value := range values {
				logger.Debugf("load keys from '%s'", value)

				accs, err := loadKeyfile(value)
				if err != nil {
					return nil, err
				}

				builder.addAccounts(accs)
			}

			continue
		}

		return nil, fmt.Errorf("unknown environment key '%s'", key)
	}

	return builder, nil
}

// Client runs on each secondary. view[0] is the arcade/ARC base URL.
func (this *BlockchainInterface) Client(params map[string]string, env, view []string, logger core.Logger) (core.BlockchainClient, error) {
	logger.Tracef("new bsv client")

	if err := configureFee(params); err != nil {
		return nil, err
	}

	arcURL := view[0]
	if !strings.HasPrefix(arcURL, "http://") && !strings.HasPrefix(arcURL, "https://") {
		arcURL = "http://" + arcURL
	}

	logger.Tracef("broadcast via ARC at '%s'", arcURL)

	bcast := &broadcaster.Arc{
		ApiUrl: arcURL,
		ApiKey: params["arc_api_key"], // optional; empty for an open arcade
	}

	confirmer := newConfirmer(logger, bcast, params)

	return newClient(logger, bcast, confirmer), nil
}

func parseEnvmap(env []string) (map[string][]string, error) {
	ret := make(map[string][]string)

	for _, element := range env {
		eqindex := strings.Index(element, "=")
		if eqindex < 0 {
			return nil, fmt.Errorf("unexpected environment '%s'", element)
		}

		key := element[:eqindex]
		value := element[eqindex+1:]

		ret[key] = append(ret[key], value)
	}

	return ret, nil
}
