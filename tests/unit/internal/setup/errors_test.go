package setup_test

import (
	"bytes"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/setup"
	"strings"
	"testing"
	"time"
)

func TestPlatformErrorCreation(t *testing.T) {
	err := platform.setup.NewUnsupportedPlatformError("unsupported-os")

	if err.Code != platform.ErrCodeUnsupportedPlatform {
		t.Errorf("Expected error code %s, got %s", platform.ErrCodeUnsupportedPlatform, err.Code)
	}

	if !strings.Contains(err.UserMessage, "unsupported-os") {
		t.Errorf("Expected user message to contain platform name, got: %s", err.UserMessage)
	}

	guidance := err.GetUserGuidance()
	if guidance.Problem == "" {
		t.Error("Expected user guidance to have a problem description")
	}
}

func TestInstallationError(t *testing.T) {
	err := setup.NewRuntimeNotFoundError("go").
		WithVersion("1.21.0").
		WithTargetPath("/usr/local/go").
		WithMetadata("source", "golang.org")

	if err.Component != "go" {
		t.Errorf("Expected component 'go', got %s", err.Component)
	}

	if err.Version != "1.21.0" {
		t.Errorf("Expected version '1.21.0', got %s", err.Version)
	}

	if err.TargetPath != "/usr/local/go" {
		t.Errorf("Expected target path '/usr/local/go', got %s", err.TargetPath)
	}

	if err.Metadata["source"] != "golang.org" {
		t.Errorf("Expected metadata source 'golang.org', got %v", err.Metadata["source"])
	}

	if len(err.Alternatives) == 0 {
		t.Error("Expected alternatives to be provided for missing runtime")
	}
}

func TestValidationError(t *testing.T) {
	err := setup.NewConfigValidationFailedError("port", "invalid", "must be between 1 and 65535").
		WithExpected("1-65535").
		WithValidValues([]string{"8080", "3000", "9000"})

	if err.Field != "port" {
		t.Errorf("Expected field 'port', got %s", err.Field)
	}

	if err.Value != "invalid" {
		t.Errorf("Expected value 'invalid', got %v", err.Value)
	}

	if len(err.ValidValues) != 3 {
		t.Errorf("Expected 3 valid values, got %d", len(err.ValidValues))
	}
}

func TestCommandError(t *testing.T) {
	err := setup.setup.NewCommandError(ErrCodeCommandFailed, "git", []string{"clone", "repo"}, 128).
		WithDuration(5*time.Second).
		WithOutput("stdout content", "stderr content")

	if err.Command != "git" {
		t.Errorf("Expected command 'git', got %s", err.Command)
	}

	if err.ExitCode != 128 {
		t.Errorf("Expected exit code 128, got %d", err.ExitCode)
	}

	if err.Duration != 5*time.Second {
		t.Errorf("Expected duration 5s, got %v", err.Duration)
	}

	if err.Stdout != "stdout content" {
		t.Errorf("Expected stdout 'stdout content', got %s", err.Stdout)
	}
}

func TestProgressInfo(t *testing.T) {
	progress := setup.NewProgressInfo("test-operation", 100)

	if progress.Stage != "test-operation" {
		t.Errorf("Expected stage 'test-operation', got %s", progress.Stage)
	}

	if progress.Total != 100 {
		t.Errorf("Expected total 100, got %d", progress.Total)
	}

	progress.Update(25, "item-25")

	if progress.Completed != 25 {
		t.Errorf("Expected completed 25, got %d", progress.Completed)
	}

	if progress.CurrentItem != "item-25" {
		t.Errorf("Expected current item 'item-25', got %s", progress.CurrentItem)
	}

	if progress.Percentage != 25.0 {
		t.Errorf("Expected percentage 25.0, got %f", progress.Percentage)
	}

	progress.Update(100, "completed")
	if !progress.IsComplete() {
		t.Error("Expected progress to be complete")
	}
}

func TestSetupLogger(t *testing.T) {
	var logOutput bytes.Buffer
	var userOutput bytes.Buffer

	logger := setup.NewSetupLogger(&SetupLoggerConfig{
		Level:              LogLevelDebug,
		Component:          "test",
		EnableJSON:         false,
		EnableUserMessages: true,
		Output:             &logOutput,
		UserOutput:         &userOutput,
		VerboseMode:        true,
	})

	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")

	logger.UserInfo("User info message")
	logger.UserSuccess("User success message")
	logger.UserWarn("User warning message")
	logger.UserError("User error message")

	logContent := logOutput.String()
	if !strings.Contains(logContent, "Debug message") {
		t.Error("Expected debug message in log output")
	}
	if !strings.Contains(logContent, "WARN") && !strings.Contains(logContent, "Warning") {
		t.Error("Expected warning level in log output")
	}

	userContent := userOutput.String()
	if !strings.Contains(userContent, "✓ User success message") {
		t.Error("Expected success symbol in user output")
	}
	if !strings.Contains(userContent, "⚠ User warning message") {
		t.Error("Expected warning symbol in user output")
	}
}

func TestSetupLoggerWithFields(t *testing.T) {
	var logOutput bytes.Buffer

	logger := setup.NewSetupLogger(&SetupLoggerConfig{
		Level:      LogLevelInfo,
		Component:  "test",
		EnableJSON: true,
		Output:     &logOutput,
	})

	enrichedLogger := logger.WithFields(map[string]interface{}{
		"operation": "test-op",
		"component": "test-component",
		"stage":     "validation",
	})

	enrichedLogger.Info("Test message with fields")

	logContent := logOutput.String()
	if !strings.Contains(logContent, "test-op") {
		t.Error("Expected operation field in log output")
	}
	if !strings.Contains(logContent, "validation") {
		t.Error("Expected stage field in log output")
	}
}

func TestErrorHandler(t *testing.T) {
	var logOutput bytes.Buffer
	var userOutput bytes.Buffer

	logger := setup.NewSetupLogger(&SetupLoggerConfig{
		Level:              LogLevelDebug,
		Component:          "test",
		EnableUserMessages: true,
		Output:             &logOutput,
		UserOutput:         &userOutput,
	})

	errorHandler := setup.setup.NewErrorHandler(logger)

	ctx := &ErrorContext{
		Operation: "test-operation",
		Component: "test-component",
		Logger:    logger,
		Metadata:  map[string]interface{}{"test": "data"},
	}

	platformErr := platform.setup.NewInsufficientPermissionsError("write", "/restricted/path")
	err := errorHandler.HandleError(ctx, platformErr)

	if err == nil {
		t.Error("Expected error to be returned")
	}

	userContent := userOutput.String()
	// Check for various forms of permission denied message
	if !strings.Contains(userContent, "permission") && !strings.Contains(userContent, "Permission") &&
		!strings.Contains(userContent, "access") && !strings.Contains(userContent, "Access") {
		t.Errorf("Expected permission denied message in user output, got: %s", userContent)
	}
}

func TestRetryableErrors(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Network error should be retryable",
			err:      setup.NewNetworkError("download", "http://example.com", fmt.Errorf("timeout")),
			expected: true,
		},
		{
			name:     "Command timeout should be retryable",
			err:      setup.setup.NewCommandError(ErrCodeCommandTimeout, "curl", []string{}, 124),
			expected: true,
		},
		{
			name:     "Validation error should not be retryable",
			err:      setup.NewConfigValidationFailedError("port", "invalid", "must be numeric"),
			expected: false,
		},
		{
			name:     "Generic timeout error should be retryable",
			err:      fmt.Errorf("operation timeout"),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsRetryableError(tc.err)
			if result != tc.expected {
				t.Errorf("Expected retryable=%v, got %v for error: %v", tc.expected, result, tc.err)
			}
		})
	}
}

func TestErrorUtilities(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrappedErr := WrapWithContext(originalErr, "test-op", "test-component", map[string]interface{}{
		"key": "value",
	})

	if wrappedErr == nil {
		t.Fatal("Expected wrapped error")
	}

	if unwrapped := wrappedErr.(*platform.PlatformError).Unwrap(); unwrapped != originalErr {
		t.Error("Expected to unwrap to original error")
	}

	criticalErr := platform.setup.NewPlatformError("TEST_CRITICAL", "test", "test", "test")
	criticalErr.Severity = platform.SeverityCritical

	if !IsCriticalError(criticalErr) {
		t.Error("Expected critical error to be detected")
	}

	nonCriticalErr := platform.setup.NewPlatformError("TEST_LOW", "test", "test", "test")
	nonCriticalErr.Severity = platform.SeverityLow

	if IsCriticalError(nonCriticalErr) {
		t.Error("Expected non-critical error to not be detected as critical")
	}
}

func TestMetricsTracking(t *testing.T) {
	logger := setup.NewSetupLogger(&SetupLoggerConfig{
		Level:         LogLevelInfo,
		Component:     "test",
		EnableMetrics: true,
	})

	logger.UpdateMetrics(map[string]interface{}{
		"files_processed":  10,
		"bytes_downloaded": int64(1024 * 1024),
		"success_rate":     0.95,
	})

	logger.Error("Test error")  // Should increment error count
	logger.Warn("Test warning") // Should increment warning count

	metrics := logger.GetMetrics()

	if metrics.ErrorsEncountered != 1 {
		t.Errorf("Expected 1 error, got %d", metrics.ErrorsEncountered)
	}

	if metrics.WarningsIssued != 1 {
		t.Errorf("Expected 1 warning, got %d", metrics.WarningsIssued)
	}

	if metrics.CustomCounters["files_processed"] != 10 {
		t.Errorf("Expected files_processed=10, got %d", metrics.CustomCounters["files_processed"])
	}

	if metrics.CustomGauges["success_rate"] != 0.95 {
		t.Errorf("Expected success_rate=0.95, got %f", metrics.CustomGauges["success_rate"])
	}
}

func BenchmarkLoggerCreation(b *testing.B) {
	config := &SetupLoggerConfig{
		Level:     LogLevelInfo,
		Component: "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setup.setup.NewSetupLogger(config)
	}
}

func BenchmarkErrorCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setup.NewRuntimeNotFoundError("go").
			WithVersion("1.21.0").
			WithMetadata("source", "test")
	}
}

func BenchmarkProgressUpdate(b *testing.B) {
	progress := setup.NewProgressInfo("benchmark", 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		progress.Update(int64(i), fmt.Sprintf("item-%d", i))
	}
}
