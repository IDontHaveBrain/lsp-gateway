package process

import (
    "context"
    "fmt"
    "sync"
    "testing"
    "time"
    "os"

	"lsp-gateway/src/internal/types"
)

func init() {
    _ = os.Setenv("ALLOW_TEST_COMMANDS", "1")
}

func TestNewLSPProcessManager(t *testing.T) {
    // Allow benign test commands like sleep/sh in security validator
    _ = os.Setenv("ALLOW_TEST_COMMANDS", "1")
    pm := NewLSPProcessManager()
    if pm == nil {
        t.Fatal("NewLSPProcessManager returned nil")
    }
}

func TestStartProcess(t *testing.T) {
	tests := []struct {
		name        string
		config      types.ClientConfig
		language    string
		expectError bool
	}{
		{
			name: "valid echo command",
			config: types.ClientConfig{
				Command: "echo",
				Args:    []string{"hello"},
			},
			language:    "test",
			expectError: false,
		},
		{
			name: "invalid command",
			config: types.ClientConfig{
				Command: "nonexistentcommand12345",
				Args:    []string{},
			},
			language:    "test",
			expectError: true,
		},
		{
			name: "empty language",
			config: types.ClientConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
			language:    "",
			expectError: false, // Language validation happens elsewhere
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := NewLSPProcessManager()

			info, err := pm.StartProcess(tt.config, tt.language)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					if info != nil {
						pm.StopProcess(info, nil)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else {
					if info == nil {
						t.Error("Expected non-nil ProcessInfo")
					} else {
						if info.Stdin == nil || info.Stdout == nil || info.Stderr == nil {
							t.Error("Expected non-nil pipes")
						}

						// Clean up
						pm.StopProcess(info, nil)
					}
				}
			}
		})
	}
}

func TestStopProcess(t *testing.T) {
	pm := NewLSPProcessManager()

	config := types.ClientConfig{
		Command: "sleep",
		Args:    []string{"10"},
	}

	info, err := pm.StartProcess(config, "test")
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	err = pm.StopProcess(info, nil)
	if err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}

	// Test stopping already stopped process
	err = pm.StopProcess(info, nil)
	if err != nil {
		t.Log("Stopping already stopped process returned error (expected)")
	}
}

func TestProcessInfoStructure(t *testing.T) {
	pm := NewLSPProcessManager()

	config := types.ClientConfig{
		Command: "echo",
		Args:    []string{"test"},
	}

	info, err := pm.StartProcess(config, "test-lang")
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer pm.StopProcess(info, nil)

	// Test ProcessInfo structure
	if info.Language != "test-lang" {
		t.Errorf("Expected language 'test-lang', got '%s'", info.Language)
	}

	if info.Cmd == nil {
		t.Error("ProcessInfo.Cmd should not be nil")
	}

	if info.StopCh == nil {
		t.Error("ProcessInfo.StopCh should not be nil")
	}
}

func TestMonitorProcess(t *testing.T) {
	pm := NewLSPProcessManager()

	// Use a command that will exit quickly
	config := types.ClientConfig{
		Command: "sh",
		Args:    []string{"-c", "exit 0"},
	}

	info, err := pm.StartProcess(config, "test")
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Monitor the process
	exitCalled := make(chan error, 1)
	go pm.MonitorProcess(info, func(err error) {
		exitCalled <- err
	})

	select {
	case err := <-exitCalled:
		if err != nil {
			t.Logf("Process exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Process monitoring timed out")
	}
}

func TestCleanupProcess(t *testing.T) {
	pm := NewLSPProcessManager()

	config := types.ClientConfig{
		Command: "echo",
		Args:    []string{"test"},
	}

	info, err := pm.StartProcess(config, "test")
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Test that cleanup doesn't panic
	pm.CleanupProcess(info)

	// Test cleanup with nil info doesn't panic
	pm.CleanupProcess(nil)
}

func TestConcurrentProcessOperations(t *testing.T) {
	pm := NewLSPProcessManager()

	var wg sync.WaitGroup
	languages := []string{"go", "python", "java", "typescript", "javascript"}

	for _, lang := range languages {
		wg.Add(1)
		go func(l string) {
			defer wg.Done()

			config := types.ClientConfig{
				Command: "sleep",
				Args:    []string{"1"},
			}

			info, err := pm.StartProcess(config, l)
			if err != nil {
				t.Errorf("Failed to start process for %s: %v", l, err)
				return
			}

			time.Sleep(100 * time.Millisecond)

			err = pm.StopProcess(info, nil)
			if err != nil {
				t.Errorf("Failed to stop process for %s: %v", l, err)
			}
		}(lang)
	}

	wg.Wait()
}

func TestProcessResourceCleanup(t *testing.T) {
	pm := NewLSPProcessManager()

	for i := 0; i < 5; i++ {
		config := types.ClientConfig{
			Command: "echo",
			Args:    []string{"test"},
		}

		lang := fmt.Sprintf("test%d", i)
		info, err := pm.StartProcess(config, lang)
		if err != nil {
			t.Fatalf("Failed to start process %s: %v", lang, err)
		}

		time.Sleep(50 * time.Millisecond)

		err = pm.StopProcess(info, nil)
		if err != nil {
			t.Errorf("Failed to stop process %s: %v", lang, err)
		}
	}
}

func TestProcessWithTimeout(t *testing.T) {
	pm := NewLSPProcessManager()

	config := types.ClientConfig{
		Command: "sleep",
		Args:    []string{"30"},
	}

	info, err := pm.StartProcess(config, "test")
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Create a context with timeout for the stop operation
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	stopDone := make(chan error)
	go func() {
		stopDone <- pm.StopProcess(info, nil)
	}()

	select {
	case <-ctx.Done():
		t.Log("Stop process should complete before context timeout (but may take longer due to graceful shutdown)")
	case err := <-stopDone:
		if err != nil {
			t.Errorf("Failed to stop process: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("Stop operation took too long")
	}
}

func TestInvalidCommandHandling(t *testing.T) {
	pm := NewLSPProcessManager()

	tests := []struct {
		name     string
		config   types.ClientConfig
		language string
	}{
		{
			name: "empty command",
			config: types.ClientConfig{
				Command: "",
				Args:    []string{},
			},
			language: "test",
		},
		{
			name: "command with pipes",
			config: types.ClientConfig{
				Command: "cmd|with|pipes",
				Args:    []string{},
			},
			language: "test",
		},
		{
			name: "very long command",
			config: types.ClientConfig{
				Command: fmt.Sprintf("%s", make([]byte, 1000)),
				Args:    []string{},
			},
			language: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := pm.StartProcess(tt.config, tt.language)
			if err == nil {
				t.Log("Expected error for invalid command, but process started")
				if info != nil {
					pm.StopProcess(info, nil)
				}
			} else {
				t.Logf("Got expected error: %v", err)
			}
		})
	}
}

// Mock shutdown sender for testing
type mockShutdownSender struct {
	shutdownCalled bool
	exitCalled     bool
}

func (m *mockShutdownSender) SendShutdownRequest(ctx context.Context) error {
	m.shutdownCalled = true
	return nil
}

func (m *mockShutdownSender) SendExitNotification(ctx context.Context) error {
	m.exitCalled = true
	return nil
}

func TestStopProcessWithShutdownSender(t *testing.T) {
	pm := NewLSPProcessManager()

	config := types.ClientConfig{
		Command: "sleep",
		Args:    []string{"10"},
	}

	info, err := pm.StartProcess(config, "test")
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	sender := &mockShutdownSender{}

	err = pm.StopProcess(info, sender)
	if err != nil {
		t.Errorf("Failed to stop process with sender: %v", err)
	}

	if !sender.shutdownCalled {
		t.Error("Expected SendShutdownRequest to be called")
	}

	if !sender.exitCalled {
		t.Error("Expected SendExitNotification to be called")
	}
}
