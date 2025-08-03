package testutils

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
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

// FindAvailablePortString finds an available port and returns it as a string
// This is a convenience function for tests that need port as string
func FindAvailablePortString() (string, error) {
	port, err := FindAvailablePort()
	if err != nil {
		return "", err
	}
	return strconv.Itoa(port), nil
}

// ReleasePort marks a port as no longer in use
// Should be called when test cleanup is complete
func ReleasePort(port int) {
	portMutex.Lock()
	delete(allocatedPorts, port)
	portMutex.Unlock()
}

// IsPortAvailable checks if a specific port is available
// This method is atomic and thread-safe
func IsPortAvailable(port int) bool {
	portMutex.RLock()
	if allocatedPorts[port] {
		portMutex.RUnlock()
		return false
	}
	portMutex.RUnlock()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()

	// Double-check allocation map after successful bind
	portMutex.RLock()
	allocated := allocatedPorts[port]
	portMutex.RUnlock()

	return !allocated
}

// FindAvailablePortInRange finds an available port within a specific range
// This method is atomic and thread-safe
func FindAvailablePortInRange(min, max int) (int, error) {
	for attempt := 0; attempt < 3; attempt++ {
		for port := min; port <= max; port++ {
			if IsPortAvailable(port) {
				// Atomic allocation attempt
				listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
				if err != nil {
					continue // Port became unavailable, try next
				}
				_ = listener.Close()

				// Mark as allocated
				portMutex.Lock()
				if !allocatedPorts[port] {
					allocatedPorts[port] = true
					portMutex.Unlock()
					return port, nil
				}
				portMutex.Unlock()
			}
		}
		// Brief delay before retry to avoid tight loop
		time.Sleep(10 * time.Millisecond)
	}
	return 0, fmt.Errorf("no available port found in range %d-%d after 3 attempts", min, max)
}
