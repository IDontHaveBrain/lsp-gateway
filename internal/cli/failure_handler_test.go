package cli

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
)

func TestCLIFailureHandler_Creation(t *testing.T) {
	config := DefaultFailureHandlerConfig()
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	
	handler := NewCLIFailureHandler(config, logger)
	
	if handler == nil {
		t.Fatal("Expected failure handler to be created, got nil")
	}
	
	if handler.config != config {
		t.Error("Expected config to be set correctly")
	}
	
	if handler.logger != logger {
		t.Error("Expected logger to be set correctly")
	}
	
	if handler.retryAttempts == nil {
		t.Error("Expected retry attempts map to be initialized")
	}
}

func TestCLIFailureHandler_HandleServerStartupFailure(t *testing.T) {
	config := &FailureHandlerConfig{
		Interactive:      false, // Non-interactive for testing
		MaxRetryAttempts: 2,
		LogFailures:      false, // Disable logging for clean test output
		AutoBypassPolicies: map[gateway.FailureCategory]bool{
			gateway.FailureCategoryStartup: true, // Auto-bypass startup failures
		},
	}
	
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	// Test startup failure handling
	err := fmt.Errorf("server failed to start: connection refused")
	recommendations := []gateway.RecoveryRecommendation{
		{
			Action:      "check_installation",
			Description: "Verify server installation",
			Commands:    []string{"status servers"},
			Priority:    1,
		},
	}
	
	handlerErr := handler.HandleServerStartupFailure("gopls", "go", err, recommendations)
	
	// Should not return error in non-interactive mode with auto-bypass
	if handlerErr != nil {
		t.Errorf("Expected no error in non-interactive mode with auto-bypass, got: %v", handlerErr)
	}
}

func TestCLIFailureHandler_HandleRuntimeFailure(t *testing.T) {
	config := &FailureHandlerConfig{
		Interactive:      false,
		MaxRetryAttempts: 1,
		LogFailures:      false,
		AutoBypassPolicies: map[gateway.FailureCategory]bool{
			gateway.FailureCategoryTransport: true,
		},
	}
	
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	// Test transport error (should auto-bypass)
	transportErr := fmt.Errorf("connection reset by peer")
	context := map[string]interface{}{
		"method": "textDocument/definition",
		"uri":    "file:///test.go",
	}
	
	handlerErr := handler.HandleRuntimeFailure("gopls", "go", transportErr, context)
	
	if handlerErr != nil {
		t.Errorf("Expected no error for transport failure with auto-bypass, got: %v", handlerErr)
	}
	
	// Test general runtime error (should not auto-bypass)
	runtimeErr := fmt.Errorf("unexpected error")
	handlerErr = handler.HandleRuntimeFailure("pylsp", "python", runtimeErr, context)
	
	if handlerErr != nil {
		t.Errorf("Expected no error for runtime failure in non-interactive mode, got: %v", handlerErr)
	}
}

func TestCLIFailureHandler_HandleConfigurationFailure(t *testing.T) {
	config := DefaultFailureHandlerConfig()
	config.Interactive = false
	config.LogFailures = false
	
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	configErr := fmt.Errorf("invalid configuration: missing required field")
	handlerErr := handler.HandleConfigurationFailure("jdtls", "java", configErr)
	
	if handlerErr != nil {
		t.Errorf("Expected no error for configuration failure in non-interactive mode, got: %v", handlerErr)
	}
}

func TestCLIFailureHandler_HandleBatchFailures(t *testing.T) {
	config := &FailureHandlerConfig{
		Interactive:      false,
		BatchThreshold:   2,
		LogFailures:      false,
		AutoBypassPolicies: map[gateway.FailureCategory]bool{
			gateway.FailureCategoryStartup: true,
		},
	}
	
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	// Create multiple failures
	failures := []gateway.FailureNotification{
		{
			ServerName:      "gopls",
			Language:        "go",
			Category:        gateway.FailureCategoryStartup,
			Severity:        gateway.FailureSeverityHigh,
			Message:         "Failed to start gopls",
			BypassAvailable: true,
			Timestamp:       time.Now(),
		},
		{
			ServerName:      "pylsp",
			Language:        "python", 
			Category:        gateway.FailureCategoryStartup,
			Severity:        gateway.FailureSeverityHigh,
			Message:         "Failed to start pylsp",
			BypassAvailable: true,
			Timestamp:       time.Now(),
		},
		{
			ServerName:      "typescript-language-server",
			Language:        "typescript",
			Category:        gateway.FailureCategoryConfiguration,
			Severity:        gateway.FailureSeverityMedium,
			Message:         "Configuration error",
			BypassAvailable: true,
			Timestamp:       time.Now(),
		},
	}
	
	handlerErr := handler.HandleBatchFailures(failures)
	
	if handlerErr != nil {
		t.Errorf("Expected no error for batch failures in non-interactive mode, got: %v", handlerErr)
	}
}

func TestCLIFailureHandler_SetNonInteractiveMode(t *testing.T) {
	config := DefaultFailureHandlerConfig()
	config.Interactive = true
	
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	// Verify initial state
	if !handler.config.Interactive {
		t.Error("Expected handler to be in interactive mode initially")
	}
	
	// Set non-interactive mode
	handler.SetNonInteractiveMode(true)
	
	// Verify state changed
	if handler.config.Interactive {
		t.Error("Expected handler to be in non-interactive mode after SetNonInteractiveMode")
	}
}

func TestCLIFailureHandler_SetJSONMode(t *testing.T) {
	config := DefaultFailureHandlerConfig()
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	// Set JSON mode
	handler.SetJSONMode()
	
	// Note: This test mainly ensures the method doesn't panic
	// Actual JSON output testing would require capturing stdout
	if handler.notifier == nil {
		t.Error("Expected notifier to be set after SetJSONMode")
	}
}

func TestFailureHandlerConfig_Defaults(t *testing.T) {
	config := DefaultFailureHandlerConfig()
	
	if !config.Interactive {
		t.Error("Expected default config to be interactive")
	}
	
	if config.BatchThreshold != 3 {
		t.Errorf("Expected default batch threshold to be 3, got %d", config.BatchThreshold)
	}
	
	if config.MaxRetryAttempts != 3 {
		t.Errorf("Expected default max retry attempts to be 3, got %d", config.MaxRetryAttempts)
	}
	
	if config.RetryDelay != 2*time.Second {
		t.Errorf("Expected default retry delay to be 2s, got %v", config.RetryDelay)
	}
	
	if !config.LogFailures {
		t.Error("Expected default config to log failures")
	}
	
	if config.AutoBypassPolicies == nil {
		t.Error("Expected auto bypass policies to be initialized")
	}
	
	// Check specific auto-bypass policy defaults
	if config.AutoBypassPolicies[gateway.FailureCategoryTransport] != true {
		t.Error("Expected transport failures to be auto-bypassed by default")
	}
}

func TestDetermineSeverity(t *testing.T) {
	config := DefaultFailureHandlerConfig()
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	testCases := []struct {
		name           string
		err            error
		expectedSeverity gateway.FailureSeverity
	}{
		{
			name:           "nil error",
			err:            nil,
			expectedSeverity: gateway.FailureSeverityLow,
		},
		{
			name:           "critical error - panic",
			err:            fmt.Errorf("panic: runtime error"),
			expectedSeverity: gateway.FailureSeverityCritical,
		},
		{
			name:           "critical error - out of memory",
			err:            fmt.Errorf("out of memory"),
			expectedSeverity: gateway.FailureSeverityCritical,
		},
		{
			name:           "high severity - connection refused",
			err:            fmt.Errorf("connection refused"),
			expectedSeverity: gateway.FailureSeverityHigh,
		},
		{
			name:           "high severity - timeout",
			err:            fmt.Errorf("operation timeout"),
			expectedSeverity: gateway.FailureSeverityHigh,
		},
		{
			name:           "medium severity - broken pipe",
			err:            fmt.Errorf("broken pipe"),
			expectedSeverity: gateway.FailureSeverityMedium,
		},
		{
			name:           "low severity - generic error",
			err:            fmt.Errorf("some generic error"),
			expectedSeverity: gateway.FailureSeverityLow,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			severity := handler.determineSeverity(tc.err)
			if severity != tc.expectedSeverity {
				t.Errorf("Expected severity %v, got %v for error: %v", 
					tc.expectedSeverity, severity, tc.err)
			}
		})
	}
}

func TestIsTransportError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection error",
			err:      fmt.Errorf("connection failed"),
			expected: true,
		},
		{
			name:     "transport error",
			err:      fmt.Errorf("transport layer error"),
			expected: true,
		},
		{
			name:     "network error",
			err:      fmt.Errorf("network unreachable"),
			expected: true,
		},
		{
			name:     "dial error",
			err:      fmt.Errorf("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      fmt.Errorf("broken pipe"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isTransportError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %v for error %v, got %v", tc.expected, tc.err, result)
			}
		})
	}
}

func TestRetryAttemptTracking(t *testing.T) {
	config := &FailureHandlerConfig{
		Interactive:      false,
		MaxRetryAttempts: 2,
		LogFailures:      false,
		AutoBypassPolicies: make(map[gateway.FailureCategory]bool),
	}
	
	logger := log.New(os.Stderr, "[TEST] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	serverName := "test-server"
	language := "test-lang"
	err := fmt.Errorf("test error")
	
	// First failure - should be retried
	handlerErr := handler.HandleServerStartupFailure(serverName, language, err, nil)
	if handlerErr != nil {
		t.Errorf("First failure should be handled without error, got: %v", handlerErr)
	}
	
	// Check retry count
	serverKey := fmt.Sprintf("%s-%s", serverName, language)
	if attempts, exists := handler.retryAttempts[serverKey]; !exists || attempts != 1 {
		t.Errorf("Expected retry attempts to be 1, got: %d (exists: %v)", attempts, exists)
	}
	
	// Second failure - should still be retried
	handlerErr = handler.HandleServerStartupFailure(serverName, language, err, nil)
	if handlerErr != nil {
		t.Errorf("Second failure should be handled without error, got: %v", handlerErr)
	}
	
	// Check retry count
	if attempts := handler.retryAttempts[serverKey]; attempts != 2 {
		t.Errorf("Expected retry attempts to be 2, got: %d", attempts)
	}
	
	// Third failure - should exceed max retries and force bypass
	handlerErr = handler.HandleServerStartupFailure(serverName, language, err, nil)
	if handlerErr != nil {
		t.Errorf("Third failure should be handled (with forced bypass), got: %v", handlerErr)
	}
}

// Benchmark test for failure handler performance
func BenchmarkCLIFailureHandler_HandleSingleFailure(b *testing.B) {
	config := &FailureHandlerConfig{
		Interactive:      false,
		LogFailures:      false,
		AutoBypassPolicies: map[gateway.FailureCategory]bool{
			gateway.FailureCategoryStartup: true,
		},
	}
	
	logger := log.New(os.Stderr, "[BENCH] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	err := fmt.Errorf("benchmark test error")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serverName := fmt.Sprintf("server-%d", i%10) // Vary server names
		handler.HandleServerStartupFailure(serverName, "go", err, nil)
	}
}

func BenchmarkCLIFailureHandler_HandleBatchFailures(b *testing.B) {
	config := &FailureHandlerConfig{
		Interactive:      false,
		LogFailures:      false,
		BatchThreshold:   3,
		AutoBypassPolicies: map[gateway.FailureCategory]bool{
			gateway.FailureCategoryStartup: true,
		},
	}
	
	logger := log.New(os.Stderr, "[BENCH] ", log.LstdFlags)
	handler := NewCLIFailureHandler(config, logger)
	
	// Create batch of failures
	failures := make([]gateway.FailureNotification, 5)
	for i := range failures {
		failures[i] = gateway.FailureNotification{
			ServerName:      fmt.Sprintf("server-%d", i),
			Language:        "go",
			Category:        gateway.FailureCategoryStartup,
			Severity:        gateway.FailureSeverityHigh,
			Message:         "Benchmark failure",
			BypassAvailable: true,
			Timestamp:       time.Now(),
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.HandleBatchFailures(failures)
	}
}