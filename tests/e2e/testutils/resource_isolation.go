package testutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// PortRange defines a range of ports for allocation
type PortRange struct {
	Min int
	Max int
}

// DefaultTestPortRange returns the default port range for tests
func DefaultTestPortRange() PortRange {
	return PortRange{Min: 18000, Max: 19999}
}

// ReservedPort holds information about a reserved port with its bound listener
type ReservedPort struct {
	TestID   string
	Port     int
	Listener net.Listener
}

// PortAllocator manages thread-safe port allocation for parallel tests
type PortAllocator struct {
	mu            sync.Mutex
	reservedPorts map[int]*ReservedPort
	portRange     PortRange
	lastPort      int
}

// NewPortAllocator creates a new thread-safe port allocator
func NewPortAllocator(portRange PortRange) *PortAllocator {
	return &PortAllocator{
		reservedPorts: make(map[int]*ReservedPort),
		portRange:     portRange,
		lastPort:      portRange.Min - 1,
	}
}

// AllocatePort reserves an available port for the given test ID with atomic binding
func (pa *PortAllocator) AllocatePort(testID string) (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	maxAttempts := pa.portRange.Max - pa.portRange.Min + 1
	attempts := 0

	for attempts < maxAttempts {
		pa.lastPort++
		if pa.lastPort > pa.portRange.Max {
			pa.lastPort = pa.portRange.Min
		}

		port := pa.lastPort
		attempts++

		// Check if port is already reserved
		if _, reserved := pa.reservedPorts[port]; reserved {
			continue
		}

		// Atomically bind to the port to prevent race conditions
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			// Port not available, continue to next port
			continue
		}

		// Successfully bound - reserve the port with the active listener
		pa.reservedPorts[port] = &ReservedPort{
			TestID:   testID,
			Port:     port,
			Listener: listener,
		}
		return port, nil
	}

	return 0, fmt.Errorf("no available ports in range %d-%d after %d attempts", 
		pa.portRange.Min, pa.portRange.Max, maxAttempts)
}

// ReleasePort releases a reserved port and closes its listener
func (pa *PortAllocator) ReleasePort(port int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	
	if reserved, exists := pa.reservedPorts[port]; exists {
		if reserved.Listener != nil {
			reserved.Listener.Close()
		}
		delete(pa.reservedPorts, port)
	}
}

// GetReservedPorts returns all currently reserved ports
func (pa *PortAllocator) GetReservedPorts() map[int]string {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	
	result := make(map[int]string)
	for port, reserved := range pa.reservedPorts {
		result[port] = reserved.TestID
	}
	return result
}

// GetReservedPortInfo returns detailed information about a reserved port
func (pa *PortAllocator) GetReservedPortInfo(port int) (*ReservedPort, bool) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	
	if reserved, exists := pa.reservedPorts[port]; exists {
		return &ReservedPort{
			TestID:   reserved.TestID,
			Port:     reserved.Port,
			Listener: reserved.Listener,
		}, true
	}
	return nil, false
}

// ClosePortListener closes the listener for a specific port without releasing the reservation
// This allows the test to bind to the port while keeping it reserved
func (pa *PortAllocator) ClosePortListener(port int) error {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	
	if reserved, exists := pa.reservedPorts[port]; exists {
		if reserved.Listener != nil {
			err := reserved.Listener.Close()
			reserved.Listener = nil
			return err
		}
	}
	return nil
}

// isPortAvailable checks if a port is available for binding (kept for backward compatibility)
func isPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// IsolatedDirectory manages isolated temporary directories for tests
type IsolatedDirectory struct {
	Path      string
	TestID    string
	CreatedAt time.Time
	cleanup   []func() error
	mu        sync.Mutex
}

// NewIsolatedDirectory creates an isolated temporary directory for a test
func NewIsolatedDirectory(testID string) (*IsolatedDirectory, error) {
	tempDir, err := ioutil.TempDir("", fmt.Sprintf("e2e_test_%s_", testID))
	if err != nil {
		return nil, fmt.Errorf("failed to create isolated directory: %w", err)
	}

	return &IsolatedDirectory{
		Path:      tempDir,
		TestID:    testID,
		CreatedAt: time.Now(),
		cleanup:   make([]func() error, 0),
	}, nil
}

// CreateSubDir creates a subdirectory within the isolated directory
func (id *IsolatedDirectory) CreateSubDir(name string) (string, error) {
	subDir := filepath.Join(id.Path, name)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subdirectory %s: %w", name, err)
	}
	return subDir, nil
}

// CreateTempFile creates a temporary file within the isolated directory
func (id *IsolatedDirectory) CreateTempFile(name, content string) (string, error) {
	filePath := filepath.Join(id.Path, name)
	if err := ioutil.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to create temp file %s: %w", name, err)
	}
	return filePath, nil
}

// AddCleanupTask adds a cleanup task to be executed during cleanup
func (id *IsolatedDirectory) AddCleanupTask(task func() error) {
	id.mu.Lock()
	defer id.mu.Unlock()
	id.cleanup = append(id.cleanup, task)
}

// Cleanup removes the isolated directory and executes cleanup tasks
func (id *IsolatedDirectory) Cleanup() error {
	id.mu.Lock()
	defer id.mu.Unlock()

	var errors []error

	// Execute cleanup tasks in reverse order
	for i := len(id.cleanup) - 1; i >= 0; i-- {
		if err := id.cleanup[i](); err != nil {
			errors = append(errors, err)
		}
	}

	// Remove the directory
	if err := os.RemoveAll(id.Path); err != nil {
		errors = append(errors, fmt.Errorf("failed to remove directory %s: %w", id.Path, err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	return nil
}

// ProcessManager manages isolated processes for tests
type ProcessManager struct {
	mu        sync.Mutex
	processes map[string]*ProcessInfo
}

// ProcessInfo holds information about a managed process
type ProcessInfo struct {
	TestID    string
	ProcessID int
	Command   string
	StartTime time.Time
	Cancel    context.CancelFunc
}

// NewProcessManager creates a new process manager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*ProcessInfo),
	}
}

// RegisterProcess registers a process with the manager
func (pm *ProcessManager) RegisterProcess(testID string, processID int, command string, cancel context.CancelFunc) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.processes[testID] = &ProcessInfo{
		TestID:    testID,
		ProcessID: processID,
		Command:   command,
		StartTime: time.Now(),
		Cancel:    cancel,
	}
}

// StopProcess stops a registered process
func (pm *ProcessManager) StopProcess(testID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if info, exists := pm.processes[testID]; exists {
		if info.Cancel != nil {
			info.Cancel()
		}
		delete(pm.processes, testID)
		return nil
	}
	return fmt.Errorf("process not found for test ID: %s", testID)
}

// StopAllProcesses stops all registered processes
func (pm *ProcessManager) StopAllProcesses() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for testID, info := range pm.processes {
		if info.Cancel != nil {
			info.Cancel()
		}
		delete(pm.processes, testID)
	}
}

// GetProcesses returns information about all registered processes
func (pm *ProcessManager) GetProcesses() map[string]*ProcessInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	result := make(map[string]*ProcessInfo)
	for testID, info := range pm.processes {
		result[testID] = &ProcessInfo{
			TestID:    info.TestID,
			ProcessID: info.ProcessID,
			Command:   info.Command,
			StartTime: info.StartTime,
			Cancel:    info.Cancel,
		}
	}
	return result
}

// ResourceManager manages all isolated resources for parallel test execution
type ResourceManager struct {
	portAllocator   *PortAllocator
	processManager  *ProcessManager
	directories     map[string]*IsolatedDirectory
	mu              sync.RWMutex
}

// TestResources represents all resources allocated for a single test
type TestResources struct {
	TestID    string
	Port      int
	Directory *IsolatedDirectory
	Resources map[string]interface{}
	mu        sync.RWMutex
}

// NewResourceManager creates a new resource manager for parallel test execution
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		portAllocator:  NewPortAllocator(DefaultTestPortRange()),
		processManager: NewProcessManager(),
		directories:    make(map[string]*IsolatedDirectory),
	}
}

// globalResourceManager provides backward compatibility
var globalResourceManager *ResourceManager
var globalResourceManagerOnce sync.Once

// GetGlobalResourceManager returns a global resource manager for backward compatibility
func GetGlobalResourceManager() *ResourceManager {
	globalResourceManagerOnce.Do(func() {
		globalResourceManager = NewResourceManager()
	})
	return globalResourceManager
}

// NewResourceManagerWithPortRange creates a resource manager with custom port range
func NewResourceManagerWithPortRange(portRange PortRange) *ResourceManager {
	return &ResourceManager{
		portAllocator:  NewPortAllocator(portRange),
		processManager: NewProcessManager(),
		directories:    make(map[string]*IsolatedDirectory),
	}
}

// AllocateTestResources allocates all necessary resources for a test
func (rm *ResourceManager) AllocateTestResources(testID string) (*TestResources, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if resources already allocated for this test ID
	if _, exists := rm.directories[testID]; exists {
		return nil, fmt.Errorf("resources already allocated for test ID: %s", testID)
	}

	// Allocate port
	port, err := rm.portAllocator.AllocatePort(testID)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate port: %w", err)
	}

	// Create isolated directory
	isolatedDir, err := NewIsolatedDirectory(testID)
	if err != nil {
		rm.portAllocator.ReleasePort(port)
		return nil, fmt.Errorf("failed to create isolated directory: %w", err)
	}

	// Store directory reference
	rm.directories[testID] = isolatedDir

	resources := &TestResources{
		TestID:    testID,
		Port:      port,
		Directory: isolatedDir,
		Resources: make(map[string]interface{}),
	}

	return resources, nil
}

// ReleaseTestResources releases all resources for a test
func (rm *ResourceManager) ReleaseTestResources(testID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var errors []error

	// Release port
	if reservedPorts := rm.portAllocator.GetReservedPorts(); reservedPorts != nil {
		for port, id := range reservedPorts {
			if id == testID {
				rm.portAllocator.ReleasePort(port)
				break
			}
		}
	}

	// Stop any processes
	if err := rm.processManager.StopProcess(testID); err != nil {
		// Not necessarily an error if no process was registered
	}

	// Cleanup directory
	if dir, exists := rm.directories[testID]; exists {
		if err := dir.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup directory: %w", err))
		}
		delete(rm.directories, testID)
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	return nil
}

// RegisterProcess registers a process for a test
func (rm *ResourceManager) RegisterProcess(testID string, processID int, command string, cancel context.CancelFunc) {
	rm.processManager.RegisterProcess(testID, processID, command, cancel)
}

// ClosePortListener closes the reserved port listener for a specific test
// This allows the test server to bind to the port while keeping it reserved
func (rm *ResourceManager) ClosePortListener(testID string) error {
	// Find the port for this test ID and close its listener
	reservedPorts := rm.portAllocator.GetReservedPorts()
	for port, id := range reservedPorts {
		if id == testID {
			return rm.portAllocator.ClosePortListener(port)
		}
	}
	return fmt.Errorf("no port found for test ID: %s", testID)
}

// GetResourceStatus returns status of all allocated resources
func (rm *ResourceManager) GetResourceStatus() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	status := make(map[string]interface{})
	
	// Port status
	status["ports"] = rm.portAllocator.GetReservedPorts()
	
	// Process status
	status["processes"] = rm.processManager.GetProcesses()
	
	// Directory status
	directoryStatus := make(map[string]string)
	for testID, dir := range rm.directories {
		directoryStatus[testID] = dir.Path
	}
	status["directories"] = directoryStatus

	return status
}

// Cleanup releases all resources
func (rm *ResourceManager) Cleanup() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var errors []error

	// Stop all processes
	rm.processManager.StopAllProcesses()

	// Cleanup all directories
	for testID, dir := range rm.directories {
		if err := dir.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup directory for test %s: %w", testID, err))
		}
	}

	// Clear all references
	rm.directories = make(map[string]*IsolatedDirectory)

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	return nil
}

// AddResource adds a custom resource to test resources
func (tr *TestResources) AddResource(key string, value interface{}) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.Resources[key] = value
}

// GetResource retrieves a custom resource
func (tr *TestResources) GetResource(key string) (interface{}, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	value, exists := tr.Resources[key]
	return value, exists
}

// GetPortString returns the allocated port as a string
func (tr *TestResources) GetPortString() string {
	return strconv.Itoa(tr.Port)
}

// ClosePortListener closes the reserved port listener to allow binding by the test server
// This must be called before the test server attempts to bind to the port
func (tr *TestResources) ClosePortListener() error {
	// Access the global resource manager to close the port listener
	rm := GetGlobalResourceManager()
	return rm.ClosePortListener(tr.TestID)
}