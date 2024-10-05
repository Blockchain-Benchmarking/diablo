package cmd

import (
	"fmt"
)

func validatePort(port int) error {
	if (port <= 0) || (port > 65535) {
		return fmt.Errorf("invalid port number %d", port)
	}

	return nil
}
