package core

import (
	"gopkg.in/yaml.v3"
	"os"
)

// Parsed Setup file.
// Describe what is the tested system and how it is deployed.
type Setup interface {
	// Return the name of the tested system.
	//
	Sysname() string

	// Return the blockchain client parameters.
	//
	Parameters() map[string]string

	// Return the set of the endpoints of the tested system.
	//
	Endpoints() []Endpoint
}

// An access point to the tested system.
// Describe an Endpoint to connect to in order to communicate with the tested
// system. This is typically the TCP/IP address of a blockchain node.
type Endpoint interface {
	// Return the address of this Endpoint.
	// The returned address is as specified by the user in the Setup
	// configuration file.
	//
	address() string

	// Return a list of tags associated with this Endpoint.
	//
	tags() []string
}

type SetupConfig struct {
	Sysname    string             `yaml:"interface"`
	Parameters map[string]string  `yaml:"parameters"`
	Endpoints  []SetupGroupConfig `yaml:"endpoints"`
}

type SetupGroupConfig struct {
	Addresses []string `yaml:"addresses"`
	Tags      []string `yaml:"tags"`
}

func ParseSetupYamlPath(path string) (Setup, error) {
	var decoder *yaml.Decoder
	var config SetupConfig
	var file *os.File
	var err error

	file, err = os.Open(path)
	if err != nil {
		return nil, err
	}

	decoder = yaml.NewDecoder(file)
	err = decoder.Decode(&config)

	file.Close()

	if err != nil {
		return nil, err
	}

	return buildParsedSetup(&config), nil
}

func buildParsedSetup(config *SetupConfig) *parsedSetup {
	var eps []Endpoint
	var addr string
	var i, nep int

	nep = 0
	for i = range config.Endpoints {
		nep += len(config.Endpoints[i].Addresses)
	}

	eps = make([]Endpoint, 0, nep)
	for i = range config.Endpoints {
		for _, addr = range config.Endpoints[i].Addresses {
			eps = append(eps, newParsedEndpoint(addr,
				config.Endpoints[i].Tags))
		}
	}

	return newParsedSetup(config.Sysname, config.Parameters, eps)
}

type parsedSetup struct {
	_sysname    string
	_parameters map[string]string
	_endpoints  []Endpoint
}

func newParsedSetup(sysname string, parameters map[string]string, endpoints []Endpoint) *parsedSetup {
	return &parsedSetup{
		_sysname:    sysname,
		_parameters: parameters,
		_endpoints:  endpoints,
	}
}

func (this *parsedSetup) Sysname() string {
	return this._sysname
}

func (this *parsedSetup) Parameters() map[string]string {
	return this._parameters
}

func (this *parsedSetup) Endpoints() []Endpoint {
	return this._endpoints
}

type parsedEndpoint struct {
	_address string
	_tags    []string
}

func newParsedEndpoint(address string, tags []string) *parsedEndpoint {
	return &parsedEndpoint{
		_address: address,
		_tags:    tags,
	}
}

func (this *parsedEndpoint) address() string {
	return this._address
}

func (this *parsedEndpoint) tags() []string {
	return this._tags
}
