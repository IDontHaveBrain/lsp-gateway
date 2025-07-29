package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	PortRangeStart    = 8080
	PortRangeEnd      = 8100
	MaxConcurrentPorts = PortRangeEnd - PortRangeStart + 1
	
	PortFileName      = ".lspg-port"
	PortRegistryDir   = ".lspg/ports"
	LockFileSuffix    = ".lock"
	
	DefaultLockTimeout = 10 * time.Second
	PortFileTimeout    = 5 * time.Minute
)

type WorkspacePortManager interface {
	AllocatePort(workspaceRoot string) (int, error)
	ReleasePort(workspaceRoot string) error
	GetAssignedPort(workspaceRoot string) (int, bool)
	IsPortAvailable(port int) bool
}

type portManagerImpl struct {
	registryPath string
	allocatedPorts map[string]int
	portLocks      map[int]*portLock
	mu             sync.RWMutex
}

type portLock struct {
	lockFile   string
	lockHandle *os.File
	mu         sync.Mutex
}

type portRegistryEntry struct {
	WorkspaceRoot string    `json:"workspace_root"`
	Port          int       `json:"port"`
	PID           int       `json:"pid"`
	AllocatedAt   time.Time `json:"allocated_at"`
	LastAccessed  time.Time `json:"last_accessed"`
}

func NewWorkspacePortManager() (WorkspacePortManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	
	registryPath := filepath.Join(homeDir, PortRegistryDir)
	if err := os.MkdirAll(registryPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create port registry directory: %w", err)
	}
	
	pm := &portManagerImpl{
		registryPath:   registryPath,
		allocatedPorts: make(map[string]int),
		portLocks:      make(map[int]*portLock),
	}
	
	if err := pm.loadExistingPorts(); err != nil {
		return nil, fmt.Errorf("failed to load existing port allocations: %w", err)
	}
	
	return pm, nil
}

func (pm *portManagerImpl) AllocatePort(workspaceRoot string) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	absPath, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path for workspace root: %w", err)
	}
	
	if existingPort, exists := pm.allocatedPorts[absPath]; exists {
		if pm.IsPortAvailable(existingPort) {
			if err := pm.updatePortAccess(absPath, existingPort); err != nil {
				return 0, fmt.Errorf("failed to update port access time: %w", err)
			}
			return existingPort, nil
		}
		
		delete(pm.allocatedPorts, absPath)
		pm.releasePortLock(existingPort)
	}
	
	targetPort := pm.calculateTargetPort(absPath)
	
	allocatedPort, err := pm.findAvailablePort(targetPort)
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	
	if err := pm.acquirePortLock(allocatedPort); err != nil {
		return 0, fmt.Errorf("failed to acquire port lock: %w", err)
	}
	
	if err := pm.createPortFile(absPath, allocatedPort); err != nil {
		pm.releasePortLock(allocatedPort)
		return 0, fmt.Errorf("failed to create port file: %w", err)
	}
	
	if err := pm.updatePortRegistry(absPath, allocatedPort); err != nil {
		pm.releasePortLock(allocatedPort)
		pm.removePortFile(absPath)
		return 0, fmt.Errorf("failed to update port registry: %w", err)
	}
	
	pm.allocatedPorts[absPath] = allocatedPort
	
	return allocatedPort, nil
}

func (pm *portManagerImpl) ReleasePort(workspaceRoot string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	absPath, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for workspace root: %w", err)
	}
	
	port, exists := pm.allocatedPorts[absPath]
	if !exists {
		return nil
	}
	
	pm.releasePortLock(port)
	pm.removePortFile(absPath)
	pm.removeFromRegistry(absPath)
	
	delete(pm.allocatedPorts, absPath)
	
	return nil
}

func (pm *portManagerImpl) GetAssignedPort(workspaceRoot string) (int, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	absPath, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return 0, false
	}
	
	port, exists := pm.allocatedPorts[absPath]
	if !exists {
		port = pm.readPortFromFile(absPath)
		if port > 0 {
			pm.allocatedPorts[absPath] = port
			return port, true
		}
		return 0, false
	}
	
	return port, true
}

func (pm *portManagerImpl) IsPortAvailable(port int) bool {
	if port < PortRangeStart || port > PortRangeEnd {
		return false
	}
	
	address := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	
	listener.Close()
	return true
}

func (pm *portManagerImpl) calculateTargetPort(workspaceRoot string) int {
	hash := sha256.Sum256([]byte(workspaceRoot))
	hashString := hex.EncodeToString(hash[:])
	
	hashInt := 0
	for i := 0; i < len(hashString) && i < 8; i++ {
		if val, err := strconv.ParseInt(string(hashString[i]), 16, 32); err == nil {
			hashInt = (hashInt << 4) + int(val)
		}
	}
	
	portOffset := hashInt % MaxConcurrentPorts
	return PortRangeStart + portOffset
}

func (pm *portManagerImpl) findAvailablePort(startPort int) (int, error) {
	for i := 0; i < MaxConcurrentPorts; i++ {
		port := PortRangeStart + ((startPort - PortRangeStart + i) % MaxConcurrentPorts)
		
		if pm.IsPortAvailable(port) && pm.canAcquirePortLock(port) {
			return port, nil
		}
	}
	
	return 0, fmt.Errorf("no available ports in range %d-%d", PortRangeStart, PortRangeEnd)
}

func (pm *portManagerImpl) acquirePortLock(port int) error {
	lockFileName := fmt.Sprintf("port_%d%s", port, LockFileSuffix)
	lockFilePath := filepath.Join(pm.registryPath, lockFileName)
	
	lockFile, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("port %d is already locked", port)
		}
		return fmt.Errorf("failed to create lock file for port %d: %w", port, err)
	}
	
	if _, err := lockFile.WriteString(fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Unix())); err != nil {
		lockFile.Close()
		os.Remove(lockFilePath)
		return fmt.Errorf("failed to write lock file content: %w", err)
	}
	
	pm.portLocks[port] = &portLock{
		lockFile:   lockFilePath,
		lockHandle: lockFile,
	}
	
	return nil
}

func (pm *portManagerImpl) canAcquirePortLock(port int) bool {
	lockFileName := fmt.Sprintf("port_%d%s", port, LockFileSuffix)
	lockFilePath := filepath.Join(pm.registryPath, lockFileName)
	
	if _, err := os.Stat(lockFilePath); os.IsNotExist(err) {
		return true
	}
	
	data, err := os.ReadFile(lockFilePath)
	if err != nil {
		return true // Assume available if can't read lock file
	}
	
	var pid int
	var timestamp int64
	if n, _ := fmt.Sscanf(string(data), "%d\n%d\n", &pid, &timestamp); n == 2 {
		lockTime := time.Unix(timestamp, 0)
		if time.Since(lockTime) > DefaultLockTimeout {
			os.Remove(lockFilePath)
			return true
		}
		
		if !pm.isProcessAlive(pid) {
			os.Remove(lockFilePath)
			return true
		}
	}
	
	return false
}

func (pm *portManagerImpl) releasePortLock(port int) {
	if portLock, exists := pm.portLocks[port]; exists {
		portLock.mu.Lock()
		defer portLock.mu.Unlock()
		
		if portLock.lockHandle != nil {
			portLock.lockHandle.Close()
		}
		
		if portLock.lockFile != "" {
			os.Remove(portLock.lockFile)
		}
		
		delete(pm.portLocks, port)
	}
}

func (pm *portManagerImpl) createPortFile(workspaceRoot string, port int) error {
	portFilePath := filepath.Join(workspaceRoot, PortFileName)
	
	portInfo := map[string]interface{}{
		"port":         port,
		"pid":          os.Getpid(),
		"created_at":   time.Now().Format(time.RFC3339),
		"workspace":    workspaceRoot,
	}
	
	data, err := json.MarshalIndent(portInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal port info: %w", err)
	}
	
	if err := os.WriteFile(portFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write port file: %w", err)
	}
	
	return nil
}

func (pm *portManagerImpl) removePortFile(workspaceRoot string) {
	portFilePath := filepath.Join(workspaceRoot, PortFileName)
	os.Remove(portFilePath)
}

func (pm *portManagerImpl) readPortFromFile(workspaceRoot string) int {
	portFilePath := filepath.Join(workspaceRoot, PortFileName)
	
	data, err := os.ReadFile(portFilePath)
	if err != nil {
		return 0
	}
	
	var portInfo map[string]interface{}
	if err := json.Unmarshal(data, &portInfo); err != nil {
		return 0
	}
	
	if portValue, exists := portInfo["port"]; exists {
		if port, ok := portValue.(float64); ok {
			return int(port)
		}
	}
	
	return 0
}

func (pm *portManagerImpl) updatePortRegistry(workspaceRoot string, port int) error {
	registryFileName := fmt.Sprintf("port_%d.json", port)
	registryFilePath := filepath.Join(pm.registryPath, registryFileName)
	
	entry := portRegistryEntry{
		WorkspaceRoot: workspaceRoot,
		Port:          port,
		PID:           os.Getpid(),
		AllocatedAt:   time.Now(),
		LastAccessed:  time.Now(),
	}
	
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry entry: %w", err)
	}
	
	if err := os.WriteFile(registryFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}
	
	return nil
}

func (pm *portManagerImpl) updatePortAccess(workspaceRoot string, port int) error {
	registryFileName := fmt.Sprintf("port_%d.json", port)
	registryFilePath := filepath.Join(pm.registryPath, registryFileName)
	
	data, err := os.ReadFile(registryFilePath)
	if err != nil {
		return pm.updatePortRegistry(workspaceRoot, port)
	}
	
	var entry portRegistryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return pm.updatePortRegistry(workspaceRoot, port)
	}
	
	entry.LastAccessed = time.Now()
	
	updatedData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated registry entry: %w", err)
	}
	
	if err := os.WriteFile(registryFilePath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to update registry file: %w", err)
	}
	
	return nil
}

func (pm *portManagerImpl) removeFromRegistry(workspaceRoot string) {
	entries, err := os.ReadDir(pm.registryPath)
	if err != nil {
		return
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			filePath := filepath.Join(pm.registryPath, entry.Name())
			
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			
			var registryEntry portRegistryEntry
			if err := json.Unmarshal(data, &registryEntry); err != nil {
				continue
			}
			
			if registryEntry.WorkspaceRoot == workspaceRoot {
				os.Remove(filePath)
				break
			}
		}
	}
}

func (pm *portManagerImpl) loadExistingPorts() error {
	entries, err := os.ReadDir(pm.registryPath)
	if err != nil {
		return nil // Directory might not exist yet
	}
	
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		
		filePath := filepath.Join(pm.registryPath, entry.Name())
		
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		
		var registryEntry portRegistryEntry
		if err := json.Unmarshal(data, &registryEntry); err != nil {
			continue
		}
		
		if time.Since(registryEntry.LastAccessed) > PortFileTimeout {
			os.Remove(filePath)
			continue
		}
		
		if !pm.isProcessAlive(registryEntry.PID) {
			os.Remove(filePath)
			continue
		}
		
		pm.allocatedPorts[registryEntry.WorkspaceRoot] = registryEntry.Port
	}
	
	return nil
}

func (pm *portManagerImpl) isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	err = process.Signal(os.Signal(nil))
	return err == nil
}