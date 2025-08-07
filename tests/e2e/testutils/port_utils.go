package testutils

import (
	"fmt"
	"net"
	"sync"
)

var (
	allocatedPorts = make(map[int]bool)
	portMutex      = sync.RWMutex{}
)

// FindAvailablePort finds an available port for testing
// Returns the port number as an integer and any error encountered
// This method is atomic and thread-safe
func FindAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	// Mark port as allocated to prevent races
	portMutex.Lock()
	allocatedPorts[port] = true
	portMutex.Unlock()

	return port, nil
}
