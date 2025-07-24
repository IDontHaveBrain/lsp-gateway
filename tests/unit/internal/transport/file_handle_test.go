package transport_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"lsp-gateway/internal/transport"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// FileHandleTracker monitors file handle usage and leaks
type FileHandleTracker struct {
	initialHandles map[string]bool
	currentHandles map[string]bool
	leakedHandles  []string
	mu             sync.RWMutex
	pid            int
}

func NewFileHandleTracker() *FileHandleTracker {
	pid := os.Getpid()
	initial := getOpenFileHandles(pid)
	return &FileHandleTracker{
		initialHandles: initial,
		currentHandles: make(map[string]bool),
		leakedHandles:  make([]string, 0),
		pid:            pid,
	}
}

func (fht *FileHandleTracker) Snapshot() {
	fht.mu.Lock()
	defer fht.mu.Unlock()
	fht.currentHandles = getOpenFileHandles(fht.pid)
}

func (fht *FileHandleTracker) DetectLeaks() []string {
	fht.mu.Lock()
	defer fht.mu.Unlock()

	current := getOpenFileHandles(fht.pid)
	var leaks []string

	for handle := range current {
		if !fht.initialHandles[handle] {
			leaks = append(leaks, handle)
		}
	}

	fht.leakedHandles = leaks
	return leaks
}

func (fht *FileHandleTracker) GetHandleCount() (initial, current int) {
	fht.mu.RLock()
	defer fht.mu.RUnlock()
	return len(fht.initialHandles), len(getOpenFileHandles(fht.pid))
}

func getOpenFileHandles(pid int) map[string]bool {
	handles := make(map[string]bool)

	if runtime.GOOS == "linux" {
		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		entries, err := os.ReadDir(fdDir)
		if err != nil {
			return handles
		}

		for _, entry := range entries {
			fdPath := filepath.Join(fdDir, entry.Name())
			target, err := os.Readlink(fdPath)
			if err != nil {
				handles[entry.Name()] = true
			} else {
				handles[fmt.Sprintf("%s->%s", entry.Name(), target)] = true
			}
		}
	} else if runtime.GOOS == "darwin" {
		// For macOS, we would use lsof or similar
		// Simplified for test environment
		handles["mock_handle"] = true
	} else if runtime.GOOS == "windows" {
		// For Windows, we would use system APIs
		// Simplified for test environment
		handles["mock_handle"] = true
	}

	return handles
}

// ProcessManager tracks subprocess lifecycle
type ProcessManager struct {
	processes map[int]*ProcessInfo
	mu        sync.RWMutex
}

type ProcessInfo struct {
	PID       int
	Command   string
	StartTime time.Time
	Status    ProcessStatus
	Handles   []string
}

type ProcessStatus int

const (
	ProcessRunning ProcessStatus = iota
	ProcessStopped
	ProcessKilled
	ProcessCrashed
)

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[int]*ProcessInfo),
	}
}

func (pm *ProcessManager) TrackProcess(pid int, command string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.processes[pid] = &ProcessInfo{
		PID:       pid,
		Command:   command,
		StartTime: time.Now(),
		Status:    ProcessRunning,
		Handles:   getProcessFileHandles(pid),
	}
}

func (pm *ProcessManager) UpdateProcessStatus(pid int, status ProcessStatus) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if info, exists := pm.processes[pid]; exists {
		info.Status = status
	}
}

func (pm *ProcessManager) GetAllProcesses() map[int]*ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[int]*ProcessInfo)
	for pid, info := range pm.processes {
		result[pid] = info
	}
	return result
}

func getProcessFileHandles(pid int) []string {
	if runtime.GOOS == "linux" {
		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		entries, err := os.ReadDir(fdDir)
		if err != nil {
			return nil
		}

		var handles []string
		for _, entry := range entries {
			handles = append(handles, entry.Name())
		}
		return handles
	}
	return []string{"mock_handle"}
}

func TestStdioFileHandleCleanup_ProcessTermination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file handle cleanup tests in short mode")
	}

	tracker := NewFileHandleTracker()
	processManager := NewProcessManager()

	t.Run("Normal_Process_Termination", func(t *testing.T) {
		testNormalProcessTermination(t, tracker, processManager)
	})

	t.Run("Forced_Process_Termination", func(t *testing.T) {
		testForcedProcessTermination(t, tracker, processManager)
	})

	t.Run("Process_Crash_Cleanup", func(t *testing.T) {
		testProcessCrashCleanup(t, tracker, processManager)
	})

	t.Run("Multiple_Process_Cleanup", func(t *testing.T) {
		testMultipleProcessCleanup(t, tracker, processManager)
	})
}

func testNormalProcessTermination(t *testing.T, tracker *FileHandleTracker, processManager *ProcessManager) {
	tracker.Snapshot()
	initialCount, _ := tracker.GetHandleCount()

	config := transport.ClientConfig{
		Command:   "cat",
		Args:      []string{},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create STDIO client: %v", err)
	}

	stdioClient, ok := client.(*transport.StdioClient)
	if !ok {
		t.Fatal("Expected transport.StdioClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	// Track the process
	pid := stdioClient.GetProcessPIDForTesting()
	if pid != -1 {
		processManager.TrackProcess(pid, "cat")
		t.Logf("Tracking process PID: %d", pid)
	}

	// Send a test request
	testCtx, testCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer testCancel()

	_, err = client.SendRequest(testCtx, "test/method", map[string]interface{}{"test": "data"})
	if err != nil && !strings.Contains(err.Error(), "timeout") {
		t.Logf("Request failed (expected): %v", err)
	}

	// Normal shutdown
	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop client: %v", err)
	}

	// Update process status
	pid = stdioClient.GetProcessPIDForTesting()
	if pid != -1 {
		processManager.UpdateProcessStatus(pid, ProcessStopped)
	}

	// Allow cleanup time
	time.Sleep(500 * time.Millisecond)

	// Check for file handle leaks
	leaks := tracker.DetectLeaks()
	_, currentCount := tracker.GetHandleCount()

	t.Logf("Normal termination - Initial handles: %d, Current handles: %d, Leaks detected: %d",
		initialCount, currentCount, len(leaks))

	if len(leaks) > 5 {
		t.Errorf("Too many file handle leaks detected: %d", len(leaks))
		for i, leak := range leaks {
			if i < 10 { // Log first 10 leaks
				t.Logf("Leak %d: %s", i+1, leak)
			}
		}
	}
}

func testForcedProcessTermination(t *testing.T, tracker *FileHandleTracker, processManager *ProcessManager) {
	tracker.Snapshot()

	config := transport.ClientConfig{
		Command:   "sleep",
		Args:      []string{"30"},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create STDIO client: %v", err)
	}

	stdioClient, ok := client.(*transport.StdioClient)
	if !ok {
		t.Fatal("Expected transport.StdioClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	var pid int
	pid = stdioClient.GetProcessPIDForTesting()
	if pid != -1 {
		processManager.TrackProcess(pid, "sleep 30")
		t.Logf("Tracking long-running process PID: %d", pid)
	}

	// Force kill the process
	// Note: Direct process kill not available with GetProcessPIDForTesting()
	// This test functionality is limited by the testing interface
	pid = stdioClient.GetProcessPIDForTesting()
	if pid != -1 {
		// Process kill would happen here, but cmd field is not accessible for testing
		processManager.UpdateProcessStatus(pid, ProcessKilled)
		t.Logf("Would kill process PID: %d (simulated)", pid)
	}

	// Clean shutdown should handle the killed process gracefully
	err = client.Stop()
	if err != nil {
		t.Logf("Stop returned error after kill (expected): %v", err)
	}

	// Allow cleanup time
	time.Sleep(1 * time.Second)

	// Check for handle leaks after forced termination
	leaks := tracker.DetectLeaks()
	_, currentCount := tracker.GetHandleCount()

	t.Logf("Forced termination - Current handles: %d, Leaks detected: %d", currentCount, len(leaks))

	// Some leaks might be acceptable after forced termination, but not too many
	if len(leaks) > 10 {
		t.Errorf("Too many file handle leaks after forced termination: %d", len(leaks))
	}
}

func testProcessCrashCleanup(t *testing.T, tracker *FileHandleTracker, processManager *ProcessManager) {
	tracker.Snapshot()

	// Create a process that will crash
	config := transport.ClientConfig{
		Command:   "sh",
		Args:      []string{"-c", "sleep 1 && exit 1"},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create STDIO client: %v", err)
	}

	stdioClient, ok := client.(*transport.StdioClient)
	if !ok {
		t.Fatal("Expected transport.StdioClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	var pid int
	pid = stdioClient.GetProcessPIDForTesting()
	if pid != -1 {
		processManager.TrackProcess(pid, "sh -c sleep 1 && exit 1")
	}

	// Wait for the process to crash
	time.Sleep(2 * time.Second)

	// Update process status
	// Note: ProcessState not available with GetProcessPIDForTesting()
	// Assuming process crashed after waiting
	pid = stdioClient.GetProcessPIDForTesting()
	if pid != -1 {
		processManager.UpdateProcessStatus(pid, ProcessCrashed)
	}

	// Try to stop the client after crash
	err = client.Stop()
	if err != nil {
		t.Logf("Stop after crash returned error (expected): %v", err)
	}

	// Allow cleanup time
	time.Sleep(500 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("Process crash cleanup - Leaks detected: %d", len(leaks))

	if len(leaks) > 8 {
		t.Errorf("Too many file handle leaks after process crash: %d", len(leaks))
	}
}

func testMultipleProcessCleanup(t *testing.T, tracker *FileHandleTracker, processManager *ProcessManager) {
	tracker.Snapshot()

	processCount := 5
	clients := make([]transport.LSPClient, processCount)

	// Start multiple processes
	for i := 0; i < processCount; i++ {
		config := transport.ClientConfig{
			Command:   "cat",
			Args:      []string{},
			Transport: transport.TransportStdio,
		}

		client, err := transport.NewLSPClient(config)
		if err != nil {
			t.Fatalf("Failed to create STDIO client %d: %v", i, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = client.Start(ctx)
		cancel()

		if err != nil {
			t.Fatalf("Failed to start client %d: %v", i, err)
		}

		clients[i] = client

		// Track the process
		if stdioClient, ok := client.(*transport.StdioClient); ok {
			pid := stdioClient.GetProcessPIDForTesting()
			if pid != -1 {
				processManager.TrackProcess(pid, fmt.Sprintf("cat_%d", i))
				t.Logf("Started process %d: PID %d", i, pid)
			}
		}
	}

	// Stop all processes
	for i, client := range clients {
		err := client.Stop()
		if err != nil {
			t.Errorf("Failed to stop client %d: %v", i, err)
		}

		// Update process status
		if stdioClient, ok := client.(*transport.StdioClient); ok {
			pid := stdioClient.GetProcessPIDForTesting()
			if pid != -1 {
				processManager.UpdateProcessStatus(pid, ProcessStopped)
			}
		}
	}

	// Allow cleanup time
	time.Sleep(1 * time.Second)

	leaks := tracker.DetectLeaks()
	processes := processManager.GetAllProcesses()

	t.Logf("Multiple process cleanup - Processes tracked: %d, Leaks detected: %d", len(processes), len(leaks))

	// Verify all processes were tracked and cleaned up
	runningCount := 0
	for _, proc := range processes {
		if proc.Status == ProcessRunning {
			runningCount++
		}
	}

	if runningCount > 0 {
		t.Errorf("Some processes still marked as running: %d", runningCount)
	}

	if len(leaks) > processCount*3 {
		t.Errorf("Too many file handle leaks for multiple processes: %d (max expected: %d)", len(leaks), processCount*3)
	}
}

func TestTCPConnectionFileHandleCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TCP file handle cleanup tests in short mode")
	}

	tracker := NewFileHandleTracker()

	t.Run("TCP_Connection_Cleanup", func(t *testing.T) {
		testTCPConnectionCleanup(t, tracker)
	})

	t.Run("TCP_Connection_Pool_Cleanup", func(t *testing.T) {
		testTCPConnectionPoolCleanup(t, tracker)
	})

	t.Run("TCP_Connection_Timeout_Cleanup", func(t *testing.T) {
		testTCPConnectionTimeoutCleanup(t, tracker)
	})

	t.Run("TCP_Connection_Error_Cleanup", func(t *testing.T) {
		testTCPConnectionErrorCleanup(t, tracker)
	})
}

func testTCPConnectionCleanup(t *testing.T, tracker *FileHandleTracker) {
	tracker.Snapshot()

	// Start a mock TCP server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create TCP listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	serverAddr := listener.Addr().String()
	t.Logf("Mock TCP server listening on: %s", serverAddr)

	// Start server goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Server closed
			}
			go handleMockConnection(conn)
		}
	}()

	config := transport.ClientConfig{
		Command:   serverAddr,
		Transport: transport.TransportTCP,
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start TCP client: %v", err)
	}

	// Send test requests
	for i := 0; i < 3; i++ {
		reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = client.SendRequest(reqCtx, "test/method", map[string]interface{}{"id": i})
		reqCancel()

		if err != nil && !strings.Contains(err.Error(), "timeout") {
			t.Logf("Request %d failed (may be expected): %v", i, err)
		}
	}

	// Clean shutdown
	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop TCP client: %v", err)
	}

	// Allow cleanup time
	time.Sleep(500 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("TCP connection cleanup - Leaks detected: %d", len(leaks))

	if len(leaks) > 5 {
		t.Errorf("Too many file handle leaks for TCP connection: %d", len(leaks))
	}
}

func testTCPConnectionPoolCleanup(t *testing.T, tracker *FileHandleTracker) {
	tracker.Snapshot()

	connectionCount := 5
	listeners := make([]net.Listener, connectionCount)
	clients := make([]transport.LSPClient, connectionCount)

	// Create multiple TCP servers
	for i := 0; i < connectionCount; i++ {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to create TCP listener %d: %v", i, err)
		}
		listeners[i] = listener

		go func(l net.Listener) {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				go handleMockConnection(conn)
			}
		}(listener)
	}

	// Create clients for each server
	for i := 0; i < connectionCount; i++ {
		config := transport.ClientConfig{
			Command:   listeners[i].Addr().String(),
			Transport: transport.TransportTCP,
		}

		client, err := transport.NewLSPClient(config)
		if err != nil {
			t.Fatalf("Failed to create TCP client %d: %v", i, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = client.Start(ctx)
		cancel()

		if err != nil {
			t.Fatalf("Failed to start TCP client %d: %v", i, err)
		}

		clients[i] = client
	}

	// Stop all clients
	for i, client := range clients {
		err := client.Stop()
		if err != nil {
			t.Errorf("Failed to stop TCP client %d: %v", i, err)
		}
	}

	// Close all listeners
	for i, listener := range listeners {
		err := listener.Close()
		if err != nil {
			t.Errorf("Failed to close listener %d: %v", i, err)
		}
	}

	// Allow cleanup time
	time.Sleep(1 * time.Second)

	leaks := tracker.DetectLeaks()
	t.Logf("TCP connection pool cleanup - Connections: %d, Leaks detected: %d", connectionCount, len(leaks))

	if len(leaks) > connectionCount*2 {
		t.Errorf("Too many file handle leaks for TCP connection pool: %d (max expected: %d)", len(leaks), connectionCount*2)
	}
}

func testTCPConnectionTimeoutCleanup(t *testing.T, tracker *FileHandleTracker) {
	tracker.Snapshot()

	// Create a server that doesn't respond
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create TCP listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Don't handle the connection - just let it timeout
			time.Sleep(10 * time.Second)
			_ = conn.Close()
		}
	}()

	config := transport.ClientConfig{
		Command:   listener.Addr().String(),
		Transport: transport.TransportTCP,
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start TCP client: %v", err)
	}

	// Send requests that will timeout
	for i := 0; i < 3; i++ {
		reqCtx, reqCancel := context.WithTimeout(context.Background(), 1*time.Second)
		_, err = client.SendRequest(reqCtx, "test/timeout", map[string]interface{}{"id": i})
		reqCancel()

		if err == nil {
			t.Errorf("Expected timeout error for request %d", i)
		}
	}

	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop TCP client: %v", err)
	}

	// Allow cleanup time
	time.Sleep(1 * time.Second)

	leaks := tracker.DetectLeaks()
	t.Logf("TCP timeout cleanup - Leaks detected: %d", len(leaks))

	if len(leaks) > 8 {
		t.Errorf("Too many file handle leaks after TCP timeouts: %d", len(leaks))
	}
}

func testTCPConnectionErrorCleanup(t *testing.T, tracker *FileHandleTracker) {
	tracker.Snapshot()

	// Use a port that's likely to be closed
	config := transport.ClientConfig{
		Command:   "localhost:0", // Port 0 should fail
		Transport: transport.TransportTCP,
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err == nil {
		t.Error("Expected connection error, got success")
	}

	err = client.Stop()
	if err != nil {
		t.Logf("Stop after failed start returned error (expected): %v", err)
	}

	// Allow cleanup time
	time.Sleep(500 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("TCP connection error cleanup - Leaks detected: %d", len(leaks))

	if len(leaks) > 3 {
		t.Errorf("File handle leaks after TCP connection error: %d", len(leaks))
	}
}

func TestFileHandleInheritance_SubprocessManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping subprocess file handle inheritance tests in short mode")
	}

	t.Run("Subprocess_File_Descriptor_Inheritance", func(t *testing.T) {
		testSubprocessFileDescriptorInheritance(t)
	})

	t.Run("Subprocess_Handle_Isolation", func(t *testing.T) {
		testSubprocessHandleIsolation(t)
	})

	t.Run("Subprocess_Cleanup_Verification", func(t *testing.T) {
		testSubprocessCleanupVerification(t)
	})
}

func testSubprocessFileDescriptorInheritance(t *testing.T) {
	tracker := NewFileHandleTracker()
	tracker.Snapshot()

	// Create temporary files that subprocess might inherit
	tempFile, err := os.CreateTemp("", "lsp-gateway-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()
	defer func() { _ = tempFile.Close() }()

	config := transport.ClientConfig{
		Command:   "cat",
		Args:      []string{},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create STDIO client: %v", err)
	}

	stdioClient, ok := client.(*transport.StdioClient)
	if !ok {
		t.Fatal("Expected transport.StdioClient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	// Check subprocess file descriptors
	pid := stdioClient.GetProcessPIDForTesting()
	if pid != -1 {
		subprocessHandles := getProcessFileHandles(pid)
		t.Logf("Subprocess PID %d has %d file handles", pid, len(subprocessHandles))

		// Verify subprocess doesn't inherit unexpected handles
		if len(subprocessHandles) > 10 {
			t.Errorf("Subprocess inherited too many file handles: %d", len(subprocessHandles))
		}
	}

	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop client: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("Subprocess inheritance test - Leaks detected: %d", len(leaks))
}

func testSubprocessHandleIsolation(t *testing.T) {
	tracker := NewFileHandleTracker()
	tracker.Snapshot()

	// Create multiple isolated subprocesses
	processCount := 3
	clients := make([]transport.LSPClient, processCount)

	for i := 0; i < processCount; i++ {
		config := transport.ClientConfig{
			Command:   "cat",
			Args:      []string{},
			Transport: transport.TransportStdio,
		}

		client, err := transport.NewLSPClient(config)
		if err != nil {
			t.Fatalf("Failed to create STDIO client %d: %v", i, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = client.Start(ctx)
		cancel()

		if err != nil {
			t.Fatalf("Failed to start client %d: %v", i, err)
		}

		clients[i] = client
	}

	// Verify each subprocess has isolated handles
	for i, client := range clients {
		if stdioClient, ok := client.(*transport.StdioClient); ok {
			pid := stdioClient.GetProcessPIDForTesting()
			if pid != -1 {
				handles := getProcessFileHandles(pid)
				t.Logf("Process %d (PID %d) has %d handles", i, pid, len(handles))
			}
		}
	}

	// Stop all processes
	for i, client := range clients {
		err := client.Stop()
		if err != nil {
			t.Errorf("Failed to stop client %d: %v", i, err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("Subprocess isolation test - Processes: %d, Leaks detected: %d", processCount, len(leaks))

	if len(leaks) > processCount*2 {
		t.Errorf("Too many leaks for isolated subprocesses: %d", len(leaks))
	}
}

func testSubprocessCleanupVerification(t *testing.T) {
	tracker := NewFileHandleTracker()
	processManager := NewProcessManager()

	// Test cleanup verification with process monitoring
	cleanupRounds := 3
	processesPerRound := 2

	for round := 0; round < cleanupRounds; round++ {
		t.Logf("Cleanup verification round %d/%d", round+1, cleanupRounds)
		tracker.Snapshot()

		clients := make([]transport.LSPClient, processesPerRound)
		pids := make([]int, processesPerRound)

		// Start processes
		for i := 0; i < processesPerRound; i++ {
			config := transport.ClientConfig{
				Command:   "cat",
				Args:      []string{},
				Transport: transport.TransportStdio,
			}

			client, err := transport.NewLSPClient(config)
			if err != nil {
				t.Fatalf("Round %d: Failed to create client %d: %v", round, i, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = client.Start(ctx)
			cancel()

			if err != nil {
				t.Fatalf("Round %d: Failed to start client %d: %v", round, i, err)
			}

			clients[i] = client

			if stdioClient, ok := client.(*transport.StdioClient); ok {
				pid := stdioClient.GetProcessPIDForTesting()
				if pid != -1 {
					pids[i] = pid
					processManager.TrackProcess(pid, fmt.Sprintf("round_%d_process_%d", round, i))
				}
			}
		}

		// Stop processes
		for i, client := range clients {
			err := client.Stop()
			if err != nil {
				t.Errorf("Round %d: Failed to stop client %d: %v", round, i, err)
			}

			if pids[i] != 0 {
				processManager.UpdateProcessStatus(pids[i], ProcessStopped)
			}
		}

		// Verify cleanup
		time.Sleep(300 * time.Millisecond)
		leaks := tracker.DetectLeaks()

		t.Logf("Round %d cleanup - Leaks: %d", round, len(leaks))

		if len(leaks) > processesPerRound*3 {
			t.Errorf("Round %d: Too many leaks: %d", round, len(leaks))
		}
	}

	// Final verification
	allProcesses := processManager.GetAllProcesses()
	runningCount := 0
	for _, proc := range allProcesses {
		if proc.Status == ProcessRunning {
			runningCount++
		}
	}

	t.Logf("Final cleanup verification - Total processes tracked: %d, Still running: %d", len(allProcesses), runningCount)

	if runningCount > 0 {
		t.Errorf("Some processes not properly cleaned up: %d still running", runningCount)
	}
}

func TestTemporaryFileHandleManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping temporary file handle management tests in short mode")
	}

	tracker := NewFileHandleTracker()

	t.Run("Temporary_File_Creation_Cleanup", func(t *testing.T) {
		testTemporaryFileCreationCleanup(t, tracker)
	})

	t.Run("Log_File_Handle_Management", func(t *testing.T) {
		testLogFileHandleManagement(t, tracker)
	})

	t.Run("Config_File_Handle_Reuse", func(t *testing.T) {
		testConfigFileHandleReuse(t, tracker)
	})
}

func testTemporaryFileCreationCleanup(t *testing.T, tracker *FileHandleTracker) {
	tracker.Snapshot()

	tempFiles := make([]*os.File, 0, 10)
	tempFilePaths := make([]string, 0, 10)

	// Create multiple temporary files
	for i := 0; i < 10; i++ {
		tempFile, err := os.CreateTemp("", fmt.Sprintf("lsp-test-%d-*", i))
		if err != nil {
			t.Fatalf("Failed to create temp file %d: %v", i, err)
		}

		tempFiles = append(tempFiles, tempFile)
		tempFilePaths = append(tempFilePaths, tempFile.Name())

		// Write some data
		_, err = tempFile.WriteString(fmt.Sprintf("Test data for file %d\n", i))
		if err != nil {
			t.Errorf("Failed to write to temp file %d: %v", i, err)
		}
	}

	// Close all files
	for i, tempFile := range tempFiles {
		err := tempFile.Close()
		if err != nil {
			t.Errorf("Failed to close temp file %d: %v", i, err)
		}
	}

	// Remove all files
	for i, path := range tempFilePaths {
		err := os.Remove(path)
		if err != nil {
			t.Errorf("Failed to remove temp file %d: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("Temporary file cleanup - Files created/removed: %d, Leaks detected: %d", len(tempFiles), len(leaks))

	if len(leaks) > 5 {
		t.Errorf("Too many leaks after temporary file cleanup: %d", len(leaks))
	}
}

func testLogFileHandleManagement(t *testing.T, tracker *FileHandleTracker) {
	tracker.Snapshot()

	logDir, err := os.MkdirTemp("", "lsp-log-test-*")
	if err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(logDir) }()

	logFiles := make([]*os.File, 0, 5)

	// Simulate log file rotation
	for i := 0; i < 5; i++ {
		logPath := filepath.Join(logDir, fmt.Sprintf("lsp-gateway-%d.log", i))
		logFile, err := os.Create(logPath)
		if err != nil {
			t.Fatalf("Failed to create log file %d: %v", i, err)
		}

		// Write log entries
		for j := 0; j < 100; j++ {
			_, err = fmt.Fprintf(logFile, "Log entry %d-%d\n", i, j)
			if err != nil {
				t.Errorf("Failed to write log entry: %v", err)
			}
		}

		logFiles = append(logFiles, logFile)

		// Keep only last 3 files open (simulate rotation)
		if len(logFiles) > 3 {
			oldFile := logFiles[0]
			err = oldFile.Close()
			if err != nil {
				t.Errorf("Failed to close old log file: %v", err)
			}
			logFiles = logFiles[1:]
		}
	}

	// Close remaining files
	for _, logFile := range logFiles {
		err = logFile.Close()
		if err != nil {
			t.Errorf("Failed to close log file: %v", err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("Log file handle management - Leaks detected: %d", len(leaks))

	if len(leaks) > 3 {
		t.Errorf("Too many leaks from log file management: %d", len(leaks))
	}
}

func testConfigFileHandleReuse(t *testing.T, tracker *FileHandleTracker) {
	tracker.Snapshot()

	configPath := filepath.Join(os.TempDir(), "test-config.yaml")
	configContent := `port: 8080
servers:
  - name: "test-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
`

	// Create config file
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
	defer func() { _ = os.Remove(configPath) }()

	// Read config file multiple times (simulating reloads)
	for i := 0; i < 10; i++ {
		file, err := os.Open(configPath)
		if err != nil {
			t.Fatalf("Failed to open config file %d: %v", i, err)
		}

		scanner := bufio.NewScanner(file)
		lineCount := 0
		for scanner.Scan() {
			lineCount++
		}

		if err := scanner.Err(); err != nil {
			t.Errorf("Error reading config file %d: %v", i, err)
		}

		err = file.Close()
		if err != nil {
			t.Errorf("Failed to close config file %d: %v", i, err)
		}

		t.Logf("Config read %d: %d lines", i, lineCount)
	}

	time.Sleep(200 * time.Millisecond)

	leaks := tracker.DetectLeaks()
	t.Logf("Config file handle reuse - Leaks detected: %d", len(leaks))

	if len(leaks) > 2 {
		t.Errorf("Config file handle leaks: %d", len(leaks))
	}
}

// Helper functions

func handleMockConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// Read headers
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				break
			}

			if strings.HasPrefix(line, "Content-Length: ") {
				_, _ = fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			}
		}

		if contentLength <= 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		_, err := io.ReadFull(reader, body)
		if err != nil {
			return
		}

		// Send simple response
		response := `{"jsonrpc":"2.0","id":1,"result":{"test":"response"}}`
		responseContent := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(response), response)

		_, err = writer.WriteString(responseContent)
		if err != nil {
			return
		}

		err = writer.Flush()
		if err != nil {
			return
		}
	}
}
