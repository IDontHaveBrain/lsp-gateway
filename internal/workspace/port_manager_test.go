package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWorkspacePortManager(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	if pm == nil {
		t.Fatal("WorkspacePortManager is nil")
	}
}

func TestCalculateTargetPort(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	impl := pm.(*portManagerImpl)
	
	testCases := []struct {
		workspaceRoot string
		expectInRange bool
	}{
		{"/home/user/project1", true},
		{"/home/user/project2", true},
		{"/different/path/project", true},
		{"", true},
	}
	
	for _, tc := range testCases {
		port := impl.calculateTargetPort(tc.workspaceRoot)
		
		if tc.expectInRange {
			if port < PortRangeStart || port > PortRangeEnd {
				t.Errorf("Port %d for workspace %s is outside range %d-%d", 
					port, tc.workspaceRoot, PortRangeStart, PortRangeEnd)
			}
		}
	}
	
	// Test deterministic behavior
	port1 := impl.calculateTargetPort("/test/workspace")
	port2 := impl.calculateTargetPort("/test/workspace")
	if port1 != port2 {
		t.Errorf("Expected deterministic port assignment, got %d and %d", port1, port2)
	}
}

func TestPortAllocationAndRelease(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	tempDir := t.TempDir()
	
	// Test port allocation
	port, err := pm.AllocatePort(tempDir)
	if err != nil {
		t.Fatalf("Failed to allocate port: %v", err)
	}
	
	if port < PortRangeStart || port > PortRangeEnd {
		t.Errorf("Allocated port %d is outside valid range %d-%d", 
			port, PortRangeStart, PortRangeEnd)
	}
	
	// Test port file creation
	portFilePath := filepath.Join(tempDir, PortFileName)
	if _, err := os.Stat(portFilePath); os.IsNotExist(err) {
		t.Errorf("Port file was not created at %s", portFilePath)
	}
	
	// Verify port file content
	data, err := os.ReadFile(portFilePath)
	if err != nil {
		t.Fatalf("Failed to read port file: %v", err)
	}
	
	var portInfo map[string]interface{}
	if err := json.Unmarshal(data, &portInfo); err != nil {
		t.Fatalf("Failed to parse port file JSON: %v", err)
	}
	
	if portValue, exists := portInfo["port"]; !exists {
		t.Error("Port file missing 'port' field")
	} else if int(portValue.(float64)) != port {
		t.Errorf("Port file contains wrong port number: expected %d, got %d", 
			port, int(portValue.(float64)))
	}
	
	// Test getting assigned port
	assignedPort, exists := pm.GetAssignedPort(tempDir)
	if !exists {
		t.Error("Failed to get assigned port")
	}
	if assignedPort != port {
		t.Errorf("GetAssignedPort returned wrong port: expected %d, got %d", 
			port, assignedPort)
	}
	
	// Test port release
	if err := pm.ReleasePort(tempDir); err != nil {
		t.Fatalf("Failed to release port: %v", err)
	}
	
	// Verify port file is removed
	if _, err := os.Stat(portFilePath); !os.IsNotExist(err) {
		t.Error("Port file was not removed after release")
	}
	
	// Verify port is no longer assigned
	_, exists = pm.GetAssignedPort(tempDir)
	if exists {
		t.Error("Port is still assigned after release")
	}
}

func TestPortReallocation(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	tempDir := t.TempDir()
	
	// Allocate port twice for same workspace
	port1, err := pm.AllocatePort(tempDir)
	if err != nil {
		t.Fatalf("Failed to allocate port first time: %v", err)
	}
	
	port2, err := pm.AllocatePort(tempDir)
	if err != nil {
		t.Fatalf("Failed to allocate port second time: %v", err)
	}
	
	if port1 != port2 {
		t.Errorf("Expected same port for repeated allocation, got %d and %d", 
			port1, port2)
	}
}

func TestPortCollisionResolution(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	
	// Allocate ports for different workspaces
	port1, err := pm.AllocatePort(tempDir1)
	if err != nil {
		t.Fatalf("Failed to allocate port for workspace 1: %v", err)
	}
	
	port2, err := pm.AllocatePort(tempDir2)
	if err != nil {
		t.Fatalf("Failed to allocate port for workspace 2: %v", err)
	}
	
	if port1 == port2 {
		t.Errorf("Expected different ports for different workspaces, both got %d", port1)
	}
}

func TestIsPortAvailable(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	// Test with invalid port numbers
	if pm.IsPortAvailable(0) {
		t.Error("Port 0 should not be available")
	}
	
	if pm.IsPortAvailable(65536) {
		t.Error("Port 65536 should not be available")
	}
	
	if pm.IsPortAvailable(PortRangeStart - 1) {
		t.Error("Port below range should not be available")
	}
	
	if pm.IsPortAvailable(PortRangeEnd + 1) {
		t.Error("Port above range should not be available")
	}
	
	// Test with valid port in range - use a less common port
	testPort := PortRangeStart + 15 // 8095 is less likely to be in use
	if !pm.IsPortAvailable(testPort) {
		t.Logf("Port %d is not available (may be in use by another service)", testPort)
		// Try another port
		testPort = PortRangeEnd - 5 // 8095
		if !pm.IsPortAvailable(testPort) {
			t.Logf("Port %d is also not available, skipping availability test", testPort)
		}
	}
}

func TestPortRegistry(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	impl := pm.(*portManagerImpl)
	tempDir := t.TempDir()
	
	port, err := pm.AllocatePort(tempDir)
	if err != nil {
		t.Fatalf("Failed to allocate port: %v", err)
	}
	
	// Check registry file exists
	registryFileName := filepath.Join(impl.registryPath, fmt.Sprintf("port_%d.json", port))
	if _, err := os.Stat(registryFileName); os.IsNotExist(err) {
		t.Errorf("Registry file was not created at %s", registryFileName)
	}
	
	// Verify registry content
	data, err := os.ReadFile(registryFileName)
	if err != nil {
		t.Fatalf("Failed to read registry file: %v", err)
	}
	
	var entry portRegistryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("Failed to parse registry JSON: %v", err)
	}
	
	if entry.Port != port {
		t.Errorf("Registry entry has wrong port: expected %d, got %d", port, entry.Port)
	}
	
	if entry.WorkspaceRoot != tempDir {
		t.Errorf("Registry entry has wrong workspace root: expected %s, got %s", 
			tempDir, entry.WorkspaceRoot)
	}
	
	if entry.PID != os.Getpid() {
		t.Errorf("Registry entry has wrong PID: expected %d, got %d", 
			os.Getpid(), entry.PID)
	}
}

func TestPortLocking(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	impl := pm.(*portManagerImpl)
	
	// Test successful lock acquisition - use a unique port for this test
	testPort := PortRangeStart + 10
	
	// Ensure port is not already locked
	impl.releasePortLock(testPort)
	
	if err := impl.acquirePortLock(testPort); err != nil {
		t.Fatalf("Failed to acquire port lock: %v", err)
	}
	
	// Test lock file exists
	lockFileName := fmt.Sprintf("port_%d%s", testPort, LockFileSuffix)
	lockFilePath := filepath.Join(impl.registryPath, lockFileName)
	if _, err := os.Stat(lockFilePath); os.IsNotExist(err) {
		t.Errorf("Lock file was not created at %s", lockFilePath)
	}
	
	// Test that same port cannot be locked again
	if err := impl.acquirePortLock(testPort); err == nil {
		t.Error("Expected error when trying to lock already locked port")
	}
	
	// Test lock release
	impl.releasePortLock(testPort)
	if _, err := os.Stat(lockFilePath); !os.IsNotExist(err) {
		t.Error("Lock file was not removed after release")
	}
}

func TestPortErrorHandling(t *testing.T) {
	t.Parallel()
	// Test various error conditions
	err := NewPortAllocationFailedError("/test/workspace", nil)
	if err.Type != PortErrorTypeAllocation {
		t.Errorf("Expected allocation error type, got %s", err.Type)
	}
	
	err = NewPortNotAvailableError(8080, "/test/workspace")
	if err.Port != 8080 {
		t.Errorf("Expected port 8080 in error, got %d", err.Port)
	}
	
	err = NewPortRangeExhaustedError("/test/workspace")
	if err.Metadata["port_range_start"] != PortRangeStart {
		t.Errorf("Expected port range start %d in metadata, got %v", 
			PortRangeStart, err.Metadata["port_range_start"])
	}
}

func TestCleanupStalePorts(t *testing.T) {
	t.Parallel()
	pm, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	impl := pm.(*portManagerImpl)
	
	// Create a stale registry entry
	staleEntry := portRegistryEntry{
		WorkspaceRoot: "/nonexistent/workspace",
		Port:          8085,
		PID:           99999, // Non-existent PID
		AllocatedAt:   time.Now().Add(-time.Hour),
		LastAccessed:  time.Now().Add(-time.Hour),
	}
	
	registryFileName := filepath.Join(impl.registryPath, "port_8085.json")
	data, _ := json.MarshalIndent(staleEntry, "", "  ")
	os.WriteFile(registryFileName, data, 0644)
	
	// Load existing ports should clean up stale entries
	impl.loadExistingPorts()
	
	// Verify stale entry was removed
	if _, err := os.Stat(registryFileName); !os.IsNotExist(err) {
		t.Error("Stale registry entry was not cleaned up")
	}
}

func BenchmarkPortManagerAllocation(b *testing.B) {
	pm, err := NewWorkspacePortManager()
	if err != nil {
		b.Fatalf("Failed to create WorkspacePortManager: %v", err)
	}
	
	tempDirs := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		tempDirs[i] = filepath.Join(os.TempDir(), fmt.Sprintf("bench_test_%d", i))
		os.MkdirAll(tempDirs[i], 0755)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := pm.AllocatePort(tempDirs[i])
		if err != nil {
			b.Fatalf("Failed to allocate port: %v", err)
		}
	}
	
	b.StopTimer()
	
	// Cleanup
	for i := 0; i < b.N; i++ {
		pm.ReleasePort(tempDirs[i])
		os.RemoveAll(tempDirs[i])
	}
}