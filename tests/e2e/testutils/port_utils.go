package testutils

import (
	"fmt"
	"net"
	"strconv"
)

// FindAvailablePort finds an available port for testing
// Returns the port number as an integer and any error encountered
func FindAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// FindAvailablePortString finds an available port and returns it as a string
// This is a convenience function for tests that need port as string
func FindAvailablePortString() (string, error) {
	port, err := FindAvailablePort()
	if err != nil {
		return "", err
	}
	return strconv.Itoa(port), nil
}

// IsPortAvailable checks if a specific port is available
func IsPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// FindAvailablePortInRange finds an available port within a specific range
func FindAvailablePortInRange(min, max int) (int, error) {
	for port := min; port <= max; port++ {
		if IsPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", min, max)
}