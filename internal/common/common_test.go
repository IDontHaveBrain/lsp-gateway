package common

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/testutil"
)

// TestConfigManager tests the ConfigManager functionality
func TestConfigManager(t *testing.T) {
	t.Parallel()

	t.Run("NewConfigManager", func(t *testing.T) {
		cm := NewConfigManager()
		if cm == nil {
			t.Error("Expected non-nil ConfigManager")
		}
	})

	t.Run("LoadAndValidateConfig_Success", func(t *testing.T) {
		tmpDir := testutil.TempDir(t)
		configFile := filepath.Join(tmpDir, "config.yaml")

		// Create valid config
		configContent := testutil.CreateConfigWithPort(8080)
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test config: %v", err)
		}

		cm := NewConfigManager()
		cfg, err := cm.LoadAndValidateConfig(configFile)
		if err != nil {
			t.Errorf("Expected successful config load: %v", err)
		}
		if cfg == nil {
			t.Error("Expected non-nil config")
			return
		}
		if cfg.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", cfg.Port)
		}
	})

	t.Run("LoadAndValidateConfig_NotFound", func(t *testing.T) {
		cm := NewConfigManager()
		nonexistentFile := "/nonexistent/config.yaml"

		_, err := cm.LoadAndValidateConfig(nonexistentFile)
		if err == nil {
			t.Error("Expected error for nonexistent config file")
		}

		var configErr *ConfigError
		if !errors.As(err, &configErr) {
			t.Errorf("Expected ConfigError, got %T", err)
		} else {
			if configErr.Type != ConfigErrorTypeNotFound {
				t.Errorf("Expected ConfigErrorTypeNotFound, got %v", configErr.Type)
			}
			if configErr.Path != nonexistentFile {
				t.Errorf("Expected path %s, got %s", nonexistentFile, configErr.Path)
			}
			if len(configErr.Suggestions) == 0 {
				t.Error("Expected suggestions for config not found")
			}
		}
	})

	t.Run("LoadAndValidateConfig_PermissionDenied", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Getuid() == 0 {
			t.Skip("Permission test not applicable on Windows or as root")
		}

		tmpDir := testutil.TempDir(t)
		configFile := filepath.Join(tmpDir, "no_read_config.yaml")

		// Create config with no read permissions
		configContent := testutil.CreateConfigWithPort(8080)
		err := os.WriteFile(configFile, []byte(configContent), 0000) // No permissions at all
		if err != nil {
			t.Fatalf("Failed to create test config: %v", err)
		}

		cm := NewConfigManager()
		_, err = cm.LoadAndValidateConfig(configFile)
		if err == nil {
			t.Error("Expected permission error")
		}

		var configErr *ConfigError
		if errors.As(err, &configErr) {
			// The actual error type depends on how the system handles no-permission files
			// It might be permission or invalid, both are acceptable for this test
			if configErr.Type != ConfigErrorTypePermission && configErr.Type != ConfigErrorTypeInvalid {
				t.Errorf("Expected ConfigErrorTypePermission or ConfigErrorTypeInvalid, got %v", configErr.Type)
			}
			// Ensure we have proper error details
			if configErr.Path != configFile {
				t.Errorf("Expected path %s, got %s", configFile, configErr.Path)
			}
			if configErr.Cause == nil {
				t.Error("Expected cause error for permission/access issue")
			}
		}
	})

	t.Run("LoadAndValidateConfig_InvalidYAML", func(t *testing.T) {
		tmpDir := testutil.TempDir(t)
		configFile := filepath.Join(tmpDir, "invalid.yaml")

		// Create invalid YAML
		invalidContent := "invalid: yaml: content: [\nunclosed"
		err := os.WriteFile(configFile, []byte(invalidContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create invalid config: %v", err)
		}

		cm := NewConfigManager()
		_, err = cm.LoadAndValidateConfig(configFile)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}

		var configErr *ConfigError
		if errors.As(err, &configErr) {
			if configErr.Type != ConfigErrorTypeInvalid {
				t.Errorf("Expected ConfigErrorTypeInvalid, got %v", configErr.Type)
			}
			if configErr.Cause == nil {
				t.Error("Expected cause error for invalid YAML")
			}
		}
	})

	t.Run("LoadAndValidateConfig_ValidationError", func(t *testing.T) {
		tmpDir := testutil.TempDir(t)
		configFile := filepath.Join(tmpDir, "invalid_config.yaml")

		// Create config that will fail validation (invalid port)
		invalidConfig := `port: -1
servers: []`
		err := os.WriteFile(configFile, []byte(invalidConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to create invalid config: %v", err)
		}

		cm := NewConfigManager()
		_, err = cm.LoadAndValidateConfig(configFile)
		if err == nil {
			t.Error("Expected validation error for invalid port")
		}

		var configErr *ConfigError
		if errors.As(err, &configErr) {
			if configErr.Type != ConfigErrorTypeValidation {
				t.Errorf("Expected ConfigErrorTypeValidation, got %v", configErr.Type)
			}
		}
	})

	t.Run("OverridePortIfSpecified", func(t *testing.T) {
		cm := NewConfigManager()
		cfg := &config.GatewayConfig{Port: 8080}

		// Test no override (same as default)
		cm.OverridePortIfSpecified(cfg, 8080, 8080)
		if cfg.Port != 8080 {
			t.Errorf("Port should remain 8080, got %d", cfg.Port)
		}

		// Test override
		cm.OverridePortIfSpecified(cfg, 9090, 8080)
		if cfg.Port != 9090 {
			t.Errorf("Port should be overridden to 9090, got %d", cfg.Port)
		}
	})
}

// TestConfigError tests the ConfigError functionality
func TestConfigError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		errorType   ConfigErrorType
		message     string
		path        string
		cause       error
		suggestions []string
		expected    string
	}{
		{
			name:      "NotFound",
			errorType: ConfigErrorTypeNotFound,
			message:   "Config not found",
			path:      "/path/to/config.yaml",
			expected:  "Configuration Not Found: Config not found",
		},
		{
			name:      "Permission",
			errorType: ConfigErrorTypePermission,
			message:   "Permission denied",
			path:      "/path/to/config.yaml",
			expected:  "Configuration Permission Error: Permission denied",
		},
		{
			name:      "Invalid",
			errorType: ConfigErrorTypeInvalid,
			message:   "Invalid syntax",
			path:      "/path/to/config.yaml",
			expected:  "Invalid Configuration: Invalid syntax",
		},
		{
			name:      "Validation",
			errorType: ConfigErrorTypeValidation,
			message:   "Validation failed",
			path:      "/path/to/config.yaml",
			expected:  "Configuration Validation Error: Validation failed",
		},
		{
			name:      "UnknownType",
			errorType: ConfigErrorType(99),
			message:   "Unknown error",
			path:      "/path/to/config.yaml",
			expected:  "Configuration Error: Unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ConfigError{
				Type:        tt.errorType,
				Message:     tt.message,
				Path:        tt.path,
				Cause:       tt.cause,
				Suggestions: tt.suggestions,
			}

			if err.Error() != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, err.Error())
			}
		})
	}

	t.Run("ErrorWithCause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &ConfigError{
			Type:    ConfigErrorTypeInvalid,
			Message: "Test error",
			Cause:   cause,
		}

		errorMsg := err.Error()
		if !strings.Contains(errorMsg, "underlying error") {
			t.Errorf("Expected error message to contain cause, got: %s", errorMsg)
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &ConfigError{
			Type:  ConfigErrorTypeInvalid,
			Cause: cause,
		}

		unwrapped := err.Unwrap()
		if unwrapped != cause {
			t.Errorf("Expected unwrapped error to be %v, got %v", cause, unwrapped)
		}
	})

	t.Run("UnwrapNil", func(t *testing.T) {
		err := &ConfigError{
			Type: ConfigErrorTypeInvalid,
		}

		unwrapped := err.Unwrap()
		if unwrapped != nil {
			t.Errorf("Expected nil unwrapped error, got %v", unwrapped)
		}
	})

	t.Run("GetSuggestions", func(t *testing.T) {
		suggestions := []string{"Fix this", "Try that"}
		err := &ConfigError{
			Type:        ConfigErrorTypeInvalid,
			Suggestions: suggestions,
		}

		got := err.GetSuggestions()
		if len(got) != len(suggestions) {
			t.Errorf("Expected %d suggestions, got %d", len(suggestions), len(got))
		}
		for i, suggestion := range suggestions {
			if got[i] != suggestion {
				t.Errorf("Expected suggestion %q, got %q", suggestion, got[i])
			}
		}
	})
}

// TestServerLifecycleManager tests the ServerLifecycleManager functionality
func TestServerLifecycleManager(t *testing.T) {
	t.Parallel()

	t.Run("NewServerLifecycleManager_Default", func(t *testing.T) {
		slm := NewServerLifecycleManager(0)
		if slm == nil {
			t.Error("Expected non-nil ServerLifecycleManager")
			return
		}
		if slm.shutdownTimeout != 30*time.Second {
			t.Errorf("Expected default timeout 30s, got %v", slm.shutdownTimeout)
		}
		if slm.errorCh == nil {
			t.Error("Expected non-nil error channel")
		}
	})

	t.Run("NewServerLifecycleManager_Custom", func(t *testing.T) {
		timeout := 10 * time.Second
		slm := NewServerLifecycleManager(timeout)
		if slm.shutdownTimeout != timeout {
			t.Errorf("Expected timeout %v, got %v", timeout, slm.shutdownTimeout)
		}
	})

	t.Run("RunHTTPServer_Success", func(t *testing.T) {
		port := testutil.AllocateTestPort(t)
		slm := NewServerLifecycleManager(1 * time.Second)

		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		config := HTTPServerConfig{
			Port:    port,
			Handler: mux,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		serverDone := make(chan error, 1)
		go func() {
			serverDone <- slm.RunHTTPServer(ctx, config)
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Test that server is running
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err != nil {
			t.Errorf("Failed to connect to server: %v", err)
		} else {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}

		// Cancel context to trigger shutdown
		cancel()

		select {
		case err := <-serverDone:
			if err != nil {
				t.Errorf("Expected clean shutdown, got error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Server shutdown timed out")
		}
	})

	t.Run("RunHTTPServer_DefaultTimeouts", func(t *testing.T) {
		port := testutil.AllocateTestPort(t)
		slm := NewServerLifecycleManager(1 * time.Second)

		config := HTTPServerConfig{
			Port:    port,
			Handler: http.NewServeMux(),
			// Leave timeouts at zero to test defaults
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		serverDone := make(chan error, 1)
		go func() {
			serverDone <- slm.RunHTTPServer(ctx, config)
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Cancel immediately to test that server starts with defaults
		cancel()

		select {
		case err := <-serverDone:
			if err != nil {
				t.Errorf("Expected clean shutdown with defaults, got: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Server with default timeouts shutdown timed out")
		}
	})

	t.Run("RunHTTPServer_PortInUse", func(t *testing.T) {
		port := testutil.AllocateTestPort(t)

		// Start first server to occupy the port
		slm1 := NewServerLifecycleManager(1 * time.Second)
		config1 := HTTPServerConfig{
			Port:    port,
			Handler: http.NewServeMux(),
		}

		ctx1, cancel1 := context.WithCancel(context.Background())
		defer cancel1()

		go func() {
			_ = slm1.RunHTTPServer(ctx1, config1)
		}()

		// Give first server time to start
		time.Sleep(100 * time.Millisecond)

		// Try to start second server on same port
		slm2 := NewServerLifecycleManager(1 * time.Second)
		config2 := HTTPServerConfig{
			Port:    port,
			Handler: http.NewServeMux(),
		}

		ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel2()

		err := slm2.RunHTTPServer(ctx2, config2)
		if err == nil {
			t.Error("Expected error when port is already in use")
		}
		if !strings.Contains(err.Error(), "HTTP server error") {
			t.Errorf("Expected HTTP server error, got: %v", err)
		}

		// Clean up first server
		cancel1()
	})

	t.Run("RunService_Success", func(t *testing.T) {
		slm := NewServerLifecycleManager(1 * time.Second)

		var startCalled, stopCalled int32

		config := ServiceConfig{
			Name: "test-service",
			StartFunc: func() error {
				atomic.StoreInt32(&startCalled, 1)
				return nil
			},
			StopFunc: func() error {
				atomic.StoreInt32(&stopCalled, 1)
				return nil
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		serviceDone := make(chan error, 1)
		go func() {
			serviceDone <- slm.RunService(ctx, config)
		}()

		// Give service time to start
		time.Sleep(100 * time.Millisecond)

		if atomic.LoadInt32(&startCalled) == 0 {
			t.Error("Expected StartFunc to be called")
		}

		// Cancel context to trigger shutdown
		cancel()

		select {
		case err := <-serviceDone:
			if err != nil {
				t.Errorf("Expected clean service shutdown, got: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Service shutdown timed out")
		}

		if atomic.LoadInt32(&stopCalled) == 0 {
			t.Error("Expected StopFunc to be called")
		}
	})

	t.Run("RunService_StartError", func(t *testing.T) {
		slm := NewServerLifecycleManager(1 * time.Second)

		expectedErr := errors.New("start failed")
		config := ServiceConfig{
			Name: "failing-service",
			StartFunc: func() error {
				return expectedErr
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := slm.RunService(ctx, config)
		if err == nil {
			t.Error("Expected error from failing StartFunc")
		}
		if !strings.Contains(err.Error(), "failing-service service error") {
			t.Errorf("Expected service error message, got: %v", err)
		}
		if !strings.Contains(err.Error(), "start failed") {
			t.Errorf("Expected original error message, got: %v", err)
		}
	})

	t.Run("RunService_NoStopFunc", func(t *testing.T) {
		slm := NewServerLifecycleManager(1 * time.Second)

		config := ServiceConfig{
			Name: "no-stop-service",
			StartFunc: func() error {
				return nil
			},
			// StopFunc is nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		serviceDone := make(chan error, 1)
		go func() {
			serviceDone <- slm.RunService(ctx, config)
		}()

		// Give service time to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context to trigger shutdown
		cancel()

		select {
		case err := <-serviceDone:
			if err != nil {
				t.Errorf("Expected clean shutdown without StopFunc, got: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Service without StopFunc shutdown timed out")
		}
	})

	t.Run("RunService_StopError", func(t *testing.T) {
		slm := NewServerLifecycleManager(1 * time.Second)

		stopErr := errors.New("stop failed")
		config := ServiceConfig{
			Name: "stop-error-service",
			StartFunc: func() error {
				return nil
			},
			StopFunc: func() error {
				return stopErr
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		serviceDone := make(chan error, 1)
		go func() {
			serviceDone <- slm.RunService(ctx, config)
		}()

		// Give service time to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context to trigger shutdown
		cancel()

		select {
		case err := <-serviceDone:
			if err == nil {
				t.Error("Expected error from failing StopFunc")
			}
			if !strings.Contains(err.Error(), "stop failed") {
				t.Errorf("Expected stop error message, got: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Service with failing StopFunc shutdown timed out")
		}
	})
}

// TestHTTPServerConfig tests the HTTPServerConfig struct
func TestHTTPServerConfig(t *testing.T) {
	t.Parallel()

	t.Run("HTTPServerConfig_Creation", func(t *testing.T) {
		mux := http.NewServeMux()
		config := HTTPServerConfig{
			Port:         8080,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 15 * time.Second,
		}

		if config.Port != 8080 {
			t.Errorf("Expected port 8080, got %d", config.Port)
		}
		if config.Handler != mux {
			t.Error("Expected handler to match")
		}
		if config.ReadTimeout != 10*time.Second {
			t.Errorf("Expected read timeout 10s, got %v", config.ReadTimeout)
		}
		if config.WriteTimeout != 15*time.Second {
			t.Errorf("Expected write timeout 15s, got %v", config.WriteTimeout)
		}
	})
}

// TestServiceConfig tests the ServiceConfig struct
func TestServiceConfig(t *testing.T) {
	t.Parallel()

	t.Run("ServiceConfig_Creation", func(t *testing.T) {
		startCalled := false
		stopCalled := false

		config := ServiceConfig{
			Name: "test-service",
			StartFunc: func() error {
				startCalled = true
				return nil
			},
			StopFunc: func() error {
				stopCalled = true
				return nil
			},
		}

		if config.Name != "test-service" {
			t.Errorf("Expected name 'test-service', got %s", config.Name)
		}

		err := config.StartFunc()
		if err != nil {
			t.Errorf("StartFunc should succeed, got: %v", err)
		}
		if !startCalled {
			t.Error("StartFunc should have been called")
		}

		err = config.StopFunc()
		if err != nil {
			t.Errorf("StopFunc should succeed, got: %v", err)
		}
		if !stopCalled {
			t.Error("StopFunc should have been called")
		}
	})
}

// TestConstants tests package constants are available
func TestConstants(t *testing.T) {
	t.Parallel()

	t.Run("ErrorConstants", func(t *testing.T) {
		if ERROR_CONFIG_NOT_FOUND == "" {
			t.Error("ERROR_CONFIG_NOT_FOUND should not be empty")
		}
		if ERROR_CONFIG_LOAD_FAILED == "" {
			t.Error("ERROR_CONFIG_LOAD_FAILED should not be empty")
		}
		if SUGGESTION_CREATE_CONFIG == "" {
			t.Error("SUGGESTION_CREATE_CONFIG should not be empty")
		}
		if FORMAT_PORT_NUMBER == "" {
			t.Error("FORMAT_PORT_NUMBER should not be empty")
		}
	})

	t.Run("FormatPortNumber", func(t *testing.T) {
		port := fmt.Sprintf(FORMAT_PORT_NUMBER, 8080)
		if port != ":8080" {
			t.Errorf("Expected ':8080', got %s", port)
		}
	})
}

// Integration test combining multiple components
func TestIntegration_ConfigAndServer(t *testing.T) {
	t.Parallel()

	t.Run("LoadConfigAndStartServer", func(t *testing.T) {
		tmpDir := testutil.TempDir(t)
		port := testutil.AllocateTestPort(t)
		configFile := filepath.Join(tmpDir, "integration_config.yaml")

		// Create config file
		configContent := testutil.CreateConfigWithPort(port)
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		// Load config using ConfigManager
		cm := NewConfigManager()
		cfg, err := cm.LoadAndValidateConfig(configFile)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Override port for testing
		cm.OverridePortIfSpecified(cfg, port, 8080)

		// Start server with loaded config
		slm := NewServerLifecycleManager(1 * time.Second)
		mux := http.NewServeMux()
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("integration-test"))
		})

		serverConfig := HTTPServerConfig{
			Port:    cfg.Port,
			Handler: mux,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		serverDone := make(chan error, 1)
		go func() {
			serverDone <- slm.RunHTTPServer(ctx, serverConfig)
		}()

		// Give server time to start
		time.Sleep(100 * time.Millisecond)

		// Test the running server
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/test", cfg.Port))
		if err != nil {
			t.Errorf("Failed to connect to integration server: %v", err)
		} else {
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}

		// Shutdown
		cancel()

		select {
		case err := <-serverDone:
			if err != nil {
				t.Errorf("Integration server shutdown error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Integration server shutdown timed out")
		}
	})
}
