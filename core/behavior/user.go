package behavior

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

type Account struct {
	Address    string `yaml:"address"`
	PrivateKey string `yaml:"private"`
}

type User interface {
	Name() string
	Run(wg *sync.WaitGroup, results chan Results, stop chan struct{})
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
}

type Results interface {
	Encode() ([]byte, error)
	Decode(src *bufio.Reader) error
	Merge(other Results) Results
	PrintResult(dest io.Writer) error
}

type Config struct {
	Endpoint   string
	Addresses  []string
	PrivateKey string
	Address    string
}

func (c *Config) Encode(dest io.Writer) error {
	// Encode Endpoint
	if err := encodeString(dest, c.Endpoint); err != nil {
		return fmt.Errorf("failed to encode Endpoint: %w", err)
	}

	// Encode Addresses slice length
	if err := binary.Write(dest, binary.LittleEndian, int32(len(c.Addresses))); err != nil {
		return fmt.Errorf("failed to encode Addresses slice length: %w", err)
	}

	// Encode each string in the Addresses slice
	for _, other := range c.Addresses {
		if err := encodeString(dest, other); err != nil {
			return fmt.Errorf("failed to encode Addresses: %w", err)
		}
	}

	// Encode PrivateKey
	if err := encodeString(dest, c.PrivateKey); err != nil {
		return fmt.Errorf("failed to encode PrivateKey: %w", err)
	}

	// Encode Address
	if err := encodeString(dest, c.Address); err != nil {
		return fmt.Errorf("failed to encode Address: %w", err)
	}

	return nil
}

func encodeString(dest io.Writer, str string) error {
	// First, write the length of the string as int32
	length := int32(len(str))
	if err := binary.Write(dest, binary.LittleEndian, length); err != nil {
		return err
	}

	// Then, write the string bytes
	if _, err := dest.Write([]byte(str)); err != nil {
		return err
	}

	return nil
}

func (c *Config) Decode(src io.Reader) error {
	var err error

	// Decode Endpoint
	if c.Endpoint, err = decodeString(src); err != nil {
		return fmt.Errorf("failed to decode Endpoint: %w", err)
	}

	// Decode Addresses slice length
	var othersLen int32
	if err := binary.Read(src, binary.LittleEndian, &othersLen); err != nil {
		return fmt.Errorf("failed to decode Addresses slice length: %w", err)
	}

	// Decode each string in the Addresses slice
	c.Addresses = make([]string, othersLen)
	for i := int32(0); i < othersLen; i++ {
		if c.Addresses[i], err = decodeString(src); err != nil {
			return fmt.Errorf("failed to decode Addresses[%d]: %w", i, err)
		}
	}

	// Decode PrivateKey
	if c.PrivateKey, err = decodeString(src); err != nil {
		return fmt.Errorf("failed to decode PrivateKey: %w", err)
	}

	// Decode Address
	if c.Address, err = decodeString(src); err != nil {
		return fmt.Errorf("failed to decode Address: %w", err)
	}

	return nil
}

func decodeString(src io.Reader) (string, error) {
	// Read the length of the string
	var length int32
	if err := binary.Read(src, binary.LittleEndian, &length); err != nil {
		return "", err
	}

	// Read the string bytes
	buf := make([]byte, length)
	if _, err := io.ReadFull(src, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}
