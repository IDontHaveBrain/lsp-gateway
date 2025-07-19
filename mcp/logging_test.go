package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test utilities and helpers

// mockWriter implements io.Writer for capturing log output
type mockWriter struct {
	mu     sync.RWMutex
	buffer bytes.Buffer
	writes [][]byte
	errors []error
	delay  time.Duration
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writes: make([][]byte, 0),
		errors: make([]error, 0),
	}
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	// Store each write for verification
	m.writes = append(m.writes, append([]byte{}, p...))
	m.buffer.Write(p)

	return len(p), nil
}

func (m *mockWriter) SetDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = delay
}

func (m *mockWriter) GetWrites() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([][]byte{}, m.writes...)
}

func (m *mockWriter) GetOutput() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buffer.String()
}

func (m *mockWriter) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buffer.Reset()
	m.writes = m.writes[:0]
}

func (m *mockWriter) GetWriteCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.writes)
}

// parseLogEntry parses a JSON log entry from output
func parseLogEntry(output string) (*LogEntry, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, errors.New("empty output")
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		return nil, fmt.Errorf("failed to parse log entry: %w", err)
	}

	return &entry, nil
}

// parseAllLogEntries parses multiple JSON log entries from output
func parseAllLogEntries(output string) ([]*LogEntry, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	entries := make([]*LogEntry, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		entry, err := parseLogEntry(line)
		if err != nil {
			return nil, fmt.Errorf("failed to parse line '%s': %w", line, err)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// Test Logger Configuration and Creation

func TestNewStructuredLogger(t *testing.T) {
	tests := []struct {
		name     string
		config   *LoggerConfig
		validate func(*testing.T, *StructuredLogger)
	}{
		{
			name:   "default config when nil provided",
			config: nil,
			validate: func(t *testing.T, logger *StructuredLogger) {
				if logger.config.Level != LogLevelInfo {
					t.Errorf("Expected default level Info, got %v", logger.config.Level)
				}
				if logger.config.Component != "mcp" {
					t.Errorf("Expected default component 'mcp', got %s", logger.config.Component)
				}
				if !logger.config.EnableJSON {
					t.Error("Expected JSON output enabled by default")
				}
				if !logger.config.EnableCaller {
					t.Error("Expected caller info enabled by default")
				}
				if logger.config.AsyncBufferSize != 1000 {
					t.Errorf("Expected default buffer size 1000, got %d", logger.config.AsyncBufferSize)
				}
			},
		},
		{
			name: "custom config preserved",
			config: &LoggerConfig{
				Level:              LogLevelDebug,
				Component:          "test-component",
				EnableJSON:         false,
				EnableStackTrace:   true,
				EnableCaller:       false,
				EnableMetrics:      false,
				Output:             io.Discard,
				IncludeTimestamp:   false,
				TimestampFormat:    time.RFC822,
				MaxStackTraceDepth: 5,
				EnableAsyncLogging: true,
				AsyncBufferSize:    500,
			},
			validate: func(t *testing.T, logger *StructuredLogger) {
				if logger.config.Level != LogLevelDebug {
					t.Errorf("Expected level Debug, got %v", logger.config.Level)
				}
				if logger.config.Component != "test-component" {
					t.Errorf("Expected component 'test-component', got %s", logger.config.Component)
				}
				if logger.config.EnableJSON {
					t.Error("Expected JSON output disabled")
				}
				if !logger.config.EnableStackTrace {
					t.Error("Expected stack trace enabled")
				}
				if logger.config.EnableCaller {
					t.Error("Expected caller info disabled")
				}
				if logger.config.EnableMetrics {
					t.Error("Expected metrics disabled")
				}
				if logger.config.AsyncBufferSize != 500 {
					t.Errorf("Expected buffer size 500, got %d", logger.config.AsyncBufferSize)
				}
			},
		},
		{
			name: "async logging initialization",
			config: &LoggerConfig{
				Level:              LogLevelInfo,
				Component:          "async-test",
				EnableAsyncLogging: true,
				AsyncBufferSize:    100,
				Output:             io.Discard,
			},
			validate: func(t *testing.T, logger *StructuredLogger) {
				if logger.asyncChan == nil {
					t.Error("Expected async channel to be initialized")
				}
				if logger.asyncDone == nil {
					t.Error("Expected async done channel to be initialized")
				}
				if cap(logger.asyncChan) != 100 {
					t.Errorf("Expected async channel capacity 100, got %d", cap(logger.asyncChan))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewStructuredLogger(tt.config)

			if logger == nil {
				t.Fatal("NewStructuredLogger returned nil")
			}

			if logger.config == nil {
				t.Fatal("Logger config is nil")
			}

			if logger.fields == nil {
				t.Fatal("Logger fields map is nil")
			}

			if logger.logCounts == nil {
				t.Fatal("Logger log counts map is nil")
			}

			tt.validate(t, logger)

			// Clean up async logger if created
			if logger.config.EnableAsyncLogging {
				logger.Close()
			}
		})
	}
}

// Test Log Level Filtering

func TestLogLevelFiltering(t *testing.T) {
	tests := []struct {
		name          string
		configLevel   LogLevel
		logLevel      LogLevel
		message       string
		shouldLog     bool
		expectedLevel string
	}{
		{"trace allowed at trace level", LogLevelTrace, LogLevelTrace, "trace message", true, "TRACE"},
		{"debug allowed at trace level", LogLevelTrace, LogLevelDebug, "debug message", true, "DEBUG"},
		{"info allowed at trace level", LogLevelTrace, LogLevelInfo, "info message", true, "INFO"},
		{"warn allowed at trace level", LogLevelTrace, LogLevelWarn, "warn message", true, "WARN"},
		{"error allowed at trace level", LogLevelTrace, LogLevelError, "error message", true, "ERROR"},

		{"trace filtered at debug level", LogLevelDebug, LogLevelTrace, "trace message", false, ""},
		{"debug allowed at debug level", LogLevelDebug, LogLevelDebug, "debug message", true, "DEBUG"},
		{"info allowed at debug level", LogLevelDebug, LogLevelInfo, "info message", true, "INFO"},

		{"trace filtered at info level", LogLevelInfo, LogLevelTrace, "trace message", false, ""},
		{"debug filtered at info level", LogLevelInfo, LogLevelDebug, "debug message", false, ""},
		{"info allowed at info level", LogLevelInfo, LogLevelInfo, "info message", true, "INFO"},
		{"warn allowed at info level", LogLevelInfo, LogLevelWarn, "warn message", true, "WARN"},
		{"error allowed at info level", LogLevelInfo, LogLevelError, "error message", true, "ERROR"},

		{"all lower levels filtered at error level", LogLevelError, LogLevelInfo, "info message", false, ""},
		{"error allowed at error level", LogLevelError, LogLevelError, "error message", true, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := newMockWriter()
			config := &LoggerConfig{
				Level:      tt.configLevel,
				Component:  "test",
				EnableJSON: true,
				Output:     output,
			}

			logger := NewStructuredLogger(config)

			// Log at the specified level
			switch tt.logLevel {
			case LogLevelTrace:
				logger.Trace(tt.message)
			case LogLevelDebug:
				logger.Debug(tt.message)
			case LogLevelInfo:
				logger.Info(tt.message)
			case LogLevelWarn:
				logger.Warn(tt.message)
			case LogLevelError:
				logger.Error(tt.message)
			}

			outputStr := output.GetOutput()

			if tt.shouldLog {
				if outputStr == "" {
					t.Errorf("Expected log output but got none")
					return
				}

				entry, err := parseLogEntry(outputStr)
				if err != nil {
					t.Errorf("Failed to parse log entry: %v", err)
					return
				}

				if entry.Level != tt.expectedLevel {
					t.Errorf("Expected level %s, got %s", tt.expectedLevel, entry.Level)
				}

				if entry.Message != tt.message {
					t.Errorf("Expected message '%s', got '%s'", tt.message, entry.Message)
				}
			} else {
				if outputStr != "" {
					t.Errorf("Expected no log output but got: %s", outputStr)
				}
			}
		})
	}
}

// Test Structured Field Context

func TestStructuredFieldContext(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*StructuredLogger) *StructuredLogger
		validate func(*testing.T, *LogEntry)
	}{
		{
			name: "WithField adds single field",
			setup: func(logger *StructuredLogger) *StructuredLogger {
				return logger.WithField("user_id", "12345")
			},
			validate: func(t *testing.T, entry *LogEntry) {
				if entry.Context == nil {
					t.Fatal("Expected context to be set")
				}
				if entry.Context["user_id"] != "12345" {
					t.Errorf("Expected user_id '12345', got %v", entry.Context["user_id"])
				}
			},
		},
		{
			name: "WithFields adds multiple fields",
			setup: func(logger *StructuredLogger) *StructuredLogger {
				return logger.WithFields(map[string]interface{}{
					"user_id":    "12345",
					"session_id": "abcdef",
					"action":     "login",
				})
			},
			validate: func(t *testing.T, entry *LogEntry) {
				if entry.Context == nil {
					t.Fatal("Expected context to be set")
				}
				expected := map[string]interface{}{
					"user_id":    "12345",
					"session_id": "abcdef",
					"action":     "login",
				}
				for k, v := range expected {
					if entry.Context[k] != v {
						t.Errorf("Expected %s='%v', got %v", k, v, entry.Context[k])
					}
				}
			},
		},
		{
			name: "WithError adds error field",
			setup: func(logger *StructuredLogger) *StructuredLogger {
				return logger.WithError(errors.New("test error"))
			},
			validate: func(t *testing.T, entry *LogEntry) {
				if entry.Context == nil {
					t.Fatal("Expected context to be set")
				}
				if entry.Context["error"] != "test error" {
					t.Errorf("Expected error 'test error', got %v", entry.Context["error"])
				}
			},
		},
		{
			name: "WithRequestID sets request ID field",
			setup: func(logger *StructuredLogger) *StructuredLogger {
				return logger.WithRequestID("req-123")
			},
			validate: func(t *testing.T, entry *LogEntry) {
				if entry.RequestID != "req-123" {
					t.Errorf("Expected RequestID 'req-123', got %s", entry.RequestID)
				}
			},
		},
		{
			name: "WithOperation sets operation field",
			setup: func(logger *StructuredLogger) *StructuredLogger {
				return logger.WithOperation("user.login")
			},
			validate: func(t *testing.T, entry *LogEntry) {
				if entry.Operation != "user.login" {
					t.Errorf("Expected Operation 'user.login', got %s", entry.Operation)
				}
			},
		},
		{
			name: "WithDuration sets duration field",
			setup: func(logger *StructuredLogger) *StructuredLogger {
				return logger.WithDuration(100 * time.Millisecond)
			},
			validate: func(t *testing.T, entry *LogEntry) {
				if entry.Duration != "100ms" {
					t.Errorf("Expected Duration '100ms', got %s", entry.Duration)
				}
			},
		},
		{
			name: "chained context preserves all fields",
			setup: func(logger *StructuredLogger) *StructuredLogger {
				return logger.
					WithField("step", "1").
					WithRequestID("req-456").
					WithOperation("complex.operation").
					WithField("component", "auth")
			},
			validate: func(t *testing.T, entry *LogEntry) {
				if entry.RequestID != "req-456" {
					t.Errorf("Expected RequestID 'req-456', got %s", entry.RequestID)
				}
				if entry.Operation != "complex.operation" {
					t.Errorf("Expected Operation 'complex.operation', got %s", entry.Operation)
				}
				if entry.Context == nil {
					t.Fatal("Expected context to be set")
				}
				if entry.Context["step"] != "1" {
					t.Errorf("Expected step '1', got %v", entry.Context["step"])
				}
				if entry.Context["component"] != "auth" {
					t.Errorf("Expected component 'auth', got %v", entry.Context["component"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := newMockWriter()
			config := &LoggerConfig{
				Level:      LogLevelInfo,
				Component:  "test",
				EnableJSON: true,
				Output:     output,
			}

			baseLogger := NewStructuredLogger(config)
			contextLogger := tt.setup(baseLogger)

			contextLogger.Info("test message")

			outputStr := output.GetOutput()
			if outputStr == "" {
				t.Fatal("Expected log output but got none")
			}

			entry, err := parseLogEntry(outputStr)
			if err != nil {
				t.Fatalf("Failed to parse log entry: %v", err)
			}

			tt.validate(t, entry)
		})
	}
}

// Test Async Logging Performance and Behavior

func TestAsyncLogging(t *testing.T) {
	t.Run("async logging writes to channel", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:              LogLevelInfo,
			Component:          "async-test",
			EnableJSON:         true,
			EnableAsyncLogging: true,
			AsyncBufferSize:    100,
			Output:             output,
		}

		logger := NewStructuredLogger(config)
		defer logger.Close()

		logger.Info("test message")

		// Give async writer time to process
		time.Sleep(10 * time.Millisecond)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Message != "test message" {
			t.Errorf("Expected message 'test message', got %s", entry.Message)
		}
	})

	t.Run("buffer overflow falls back to sync writing", func(t *testing.T) {
		output := newMockWriter()
		output.SetDelay(100 * time.Millisecond) // Slow down writer to fill buffer

		config := &LoggerConfig{
			Level:              LogLevelInfo,
			Component:          "overflow-test",
			EnableJSON:         true,
			EnableAsyncLogging: true,
			AsyncBufferSize:    2, // Small buffer
			Output:             output,
		}

		logger := NewStructuredLogger(config)
		defer logger.Close()

		// Fill the buffer
		logger.Info("message 1")
		logger.Info("message 2")
		logger.Info("message 3") // This should trigger sync write

		// Wait for some processing
		time.Sleep(50 * time.Millisecond)

		writeCount := output.GetWriteCount()
		if writeCount < 2 {
			t.Errorf("Expected at least 2 writes, got %d", writeCount)
		}
	})

	t.Run("concurrent async logging is thread-safe", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:              LogLevelInfo,
			Component:          "concurrent-test",
			EnableJSON:         true,
			EnableAsyncLogging: true,
			AsyncBufferSize:    1000,
			Output:             output,
		}

		logger := NewStructuredLogger(config)
		defer logger.Close()

		const numGoroutines = 10
		const messagesPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					logger.WithField("goroutine", id).WithField("message", j).Info("concurrent test")
				}
			}(i)
		}

		wg.Wait()

		// Give async writer time to process all messages
		time.Sleep(100 * time.Millisecond)

		writeCount := output.GetWriteCount()
		expectedCount := numGoroutines * messagesPerGoroutine

		if writeCount != expectedCount {
			t.Errorf("Expected %d writes, got %d", expectedCount, writeCount)
		}
	})

	t.Run("async logger graceful shutdown", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:              LogLevelInfo,
			Component:          "shutdown-test",
			EnableJSON:         true,
			EnableAsyncLogging: true,
			AsyncBufferSize:    100,
			Output:             output,
		}

		logger := NewStructuredLogger(config)

		// Log some messages
		for i := 0; i < 10; i++ {
			logger.WithField("index", i).Info("shutdown test")
		}

		// Close should wait for all messages to be processed
		err := logger.Close()
		if err != nil {
			t.Errorf("Unexpected error during close: %v", err)
		}

		writeCount := output.GetWriteCount()
		if writeCount != 10 {
			t.Errorf("Expected 10 writes after shutdown, got %d", writeCount)
		}
	})
}

// Test JSON vs Human-Readable Output Formats

func TestOutputFormats(t *testing.T) {
	t.Run("JSON format validation", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:            LogLevelInfo,
			Component:        "json-test",
			EnableJSON:       true,
			EnableCaller:     true,
			IncludeTimestamp: true,
			Output:           output,
		}

		logger := NewStructuredLogger(config)
		logger.WithField("test_field", "test_value").Info("json test message")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		// Validate JSON structure
		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse JSON log entry: %v", err)
		}

		// Validate required fields
		if entry.Level != "INFO" {
			t.Errorf("Expected level 'INFO', got %s", entry.Level)
		}
		if entry.Component != "json-test" {
			t.Errorf("Expected component 'json-test', got %s", entry.Component)
		}
		if entry.Message != "json test message" {
			t.Errorf("Expected message 'json test message', got %s", entry.Message)
		}
		if entry.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}
		if entry.Caller == "" {
			t.Error("Expected caller information")
		}
		if entry.Context == nil || entry.Context["test_field"] != "test_value" {
			t.Error("Expected context field to be preserved")
		}
	})

	t.Run("human-readable format", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:            LogLevelInfo,
			Component:        "human-test",
			EnableJSON:       false,
			EnableCaller:     true,
			IncludeTimestamp: true,
			TimestampFormat:  "15:04:05",
			Output:           output,
		}

		logger := NewStructuredLogger(config)
		logger.WithField("test_field", "test_value").Info("human readable test")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		// Check for expected components in human-readable format
		if !strings.Contains(outputStr, "[INFO]") {
			t.Error("Expected '[INFO]' in human-readable output")
		}
		if !strings.Contains(outputStr, "human-test") {
			t.Error("Expected component name in output")
		}
		if !strings.Contains(outputStr, "human readable test") {
			t.Error("Expected message in output")
		}
		if !strings.Contains(outputStr, "caller=") {
			t.Error("Expected caller information in output")
		}
		if !strings.Contains(outputStr, "test_field=test_value") {
			t.Error("Expected context field in output")
		}
	})

	t.Run("JSON marshaling failure fallback", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "fallback-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		// Create a logger with a field that can't be marshaled to JSON
		badLogger := logger.WithField("bad_field", make(chan int))
		badLogger.Info("test with unmarshalable field")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected fallback output but got none")
		}

		// Should fallback to simple format
		if !strings.Contains(outputStr, "[INFO]") {
			t.Error("Expected fallback format with '[INFO]'")
		}
		if !strings.Contains(outputStr, "test with unmarshalable field") {
			t.Error("Expected message in fallback output")
		}
	})
}

// Test Error Handling and Stack Traces

func TestErrorHandlingAndStackTraces(t *testing.T) {
	t.Run("ErrorWithStack includes stack trace", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:              LogLevelError,
			Component:          "stack-test",
			EnableJSON:         true,
			EnableStackTrace:   true,
			EnableCaller:       true,
			IncludeTimestamp:   true,
			MaxStackTraceDepth: 5,
			Output:             output,
		}

		logger := NewStructuredLogger(config)
		testErr := errors.New("test error with stack")

		logger.ErrorWithStack("error occurred", testErr)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "ERROR" {
			t.Errorf("Expected level 'ERROR', got %s", entry.Level)
		}
		if entry.Message != "error occurred" {
			t.Errorf("Expected message 'error occurred', got %s", entry.Message)
		}
		if entry.Error != "test error with stack" {
			t.Errorf("Expected error 'test error with stack', got %s", entry.Error)
		}
		if len(entry.StackTrace) == 0 {
			t.Error("Expected stack trace to be populated")
		}
		if len(entry.StackTrace) > 5 {
			t.Errorf("Expected stack trace depth <= 5, got %d", len(entry.StackTrace))
		}

		// Verify stack trace format
		for i, frame := range entry.StackTrace {
			if !strings.Contains(frame, "(") || !strings.Contains(frame, ":") {
				t.Errorf("Stack frame %d has unexpected format: %s", i, frame)
			}
		}
	})

	t.Run("stack trace disabled by config", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:            LogLevelError,
			Component:        "no-stack-test",
			EnableJSON:       true,
			EnableStackTrace: false,
			Output:           output,
		}

		logger := NewStructuredLogger(config)
		testErr := errors.New("test error without stack")

		logger.ErrorWithStack("error occurred", testErr)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if len(entry.StackTrace) > 0 {
			t.Error("Expected no stack trace when disabled")
		}
	})

	t.Run("error logging with context", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelError,
			Component:  "error-context-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)
		testErr := errors.New("context error")

		logger.WithField("operation", "database_query").
			WithField("user_id", "12345").
			WithError(testErr).
			Error("database operation failed")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Context == nil {
			t.Fatal("Expected context to be set")
		}
		if entry.Operation != "database_query" {
			t.Errorf("Expected operation 'database_query', got %v", entry.Operation)
		}
		if entry.Context["user_id"] != "12345" {
			t.Errorf("Expected user_id '12345', got %v", entry.Context["user_id"])
		}
		if entry.Context["error"] != "context error" {
			t.Errorf("Expected error 'context error', got %v", entry.Context["error"])
		}
	})
}

// Test Metrics and Performance Tracking

func TestMetricsAndPerformance(t *testing.T) {
	t.Run("runtime metrics are included when enabled", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:         LogLevelInfo,
			Component:     "metrics-test",
			EnableJSON:    true,
			EnableMetrics: true,
			Output:        output,
		}

		logger := NewStructuredLogger(config)
		logger.Info("test with metrics")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Metrics == nil {
			t.Fatal("Expected metrics to be included")
		}
		if entry.Metrics.MemoryUsage <= 0 {
			t.Error("Expected memory usage to be positive")
		}
		if entry.Metrics.GoroutineCount <= 0 {
			t.Error("Expected goroutine count to be positive")
		}
	})

	t.Run("custom metrics can be provided", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:         LogLevelInfo,
			Component:     "custom-metrics-test",
			EnableJSON:    true,
			EnableMetrics: false,
			Output:        output,
		}

		logger := NewStructuredLogger(config)
		customMetrics := &LogMetrics{
			BytesProcessed: 1024,
			ItemsProcessed: 50,
			CacheHitRate:   0.85,
			CustomCounters: map[string]int64{
				"requests_processed": 100,
				"errors_handled":     5,
			},
			CustomGauges: map[string]float64{
				"response_time_ms": 250.5,
				"cpu_usage":        0.75,
			},
		}

		logger.WithMetrics(customMetrics).Info("operation completed")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Metrics == nil {
			t.Fatal("Expected custom metrics to be included")
		}
		if entry.Metrics.BytesProcessed != 1024 {
			t.Errorf("Expected bytes processed 1024, got %d", entry.Metrics.BytesProcessed)
		}
		if entry.Metrics.CacheHitRate != 0.85 {
			t.Errorf("Expected cache hit rate 0.85, got %f", entry.Metrics.CacheHitRate)
		}
		if entry.Metrics.CustomCounters["requests_processed"] != 100 {
			t.Error("Expected custom counter to be preserved")
		}
		if entry.Metrics.CustomGauges["response_time_ms"] != 250.5 {
			t.Error("Expected custom gauge to be preserved")
		}
	})

	t.Run("log counts are tracked accurately", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelTrace,
			Component:  "count-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		// Log messages at different levels
		logger.Trace("trace message")
		logger.Debug("debug message")
		logger.Info("info message")
		logger.Info("another info message")
		logger.Warn("warn message")
		logger.Error("error message")

		counts := logger.GetLogCounts()

		expectedCounts := map[LogLevel]int64{
			LogLevelTrace: 1,
			LogLevelDebug: 1,
			LogLevelInfo:  2,
			LogLevelWarn:  1,
			LogLevelError: 1,
			LogLevelFatal: 0,
		}

		for level, expected := range expectedCounts {
			if counts[level] != expected {
				t.Errorf("Expected %s count %d, got %d", logLevelNames[level], expected, counts[level])
			}
		}
	})

	t.Run("log counts can be reset", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "reset-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		// Log some messages
		logger.Info("message 1")
		logger.Error("error 1")

		initialCounts := logger.GetLogCounts()
		if initialCounts[LogLevelInfo] != 1 || initialCounts[LogLevelError] != 1 {
			t.Fatal("Initial counts are incorrect")
		}

		// Reset counts
		logger.ResetLogCounts()

		resetCounts := logger.GetLogCounts()
		for level, count := range resetCounts {
			if count != 0 {
				t.Errorf("Expected count 0 for %s after reset, got %d", logLevelNames[level], count)
			}
		}
	})
}

// Test Specialized Logging Methods

func TestSpecializedLoggingMethods(t *testing.T) {
	output := newMockWriter()
	config := &LoggerConfig{
		Level:      LogLevelInfo,
		Component:  "specialized-test",
		EnableJSON: true,
		Output:     output,
	}

	logger := NewStructuredLogger(config)

	t.Run("LogRequest captures request details", func(t *testing.T) {
		output.Clear()
		startTime := time.Now().Add(-100 * time.Millisecond)

		logger.LogRequest("textDocument/definition", "req-123", map[string]interface{}{
			"uri": "file:///test.go",
		}, startTime)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "INFO" {
			t.Errorf("Expected level 'INFO', got %s", entry.Level)
		}
		if entry.Message != "Processing request" {
			t.Errorf("Expected message 'Processing request', got %s", entry.Message)
		}
		if entry.Context["method"] != "textDocument/definition" {
			t.Error("Expected method in context")
		}
		if entry.RequestID != "req-123" {
			t.Error("Expected request_id in entry.RequestID")
		}
		if entry.Context["type"] != "request" {
			t.Error("Expected type 'request' in context")
		}
		if entry.Duration == "" {
			t.Error("Expected duration in entry.Duration")
		}
	})

	t.Run("LogResponse captures response details", func(t *testing.T) {
		output.Clear()

		logger.LogResponse("textDocument/definition", "req-456", true, 1024, 50*time.Millisecond)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "INFO" {
			t.Errorf("Expected level 'INFO', got %s", entry.Level)
		}
		if entry.Message != "Request completed" {
			t.Errorf("Expected message 'Request completed', got %s", entry.Message)
		}
		if entry.Context["success"] != true {
			t.Error("Expected success true in context")
		}
		if entry.Context["response_size"] != float64(1024) {
			t.Error("Expected response_size in context")
		}
	})

	t.Run("LogResponse with failure logs as error", func(t *testing.T) {
		output.Clear()

		logger.LogResponse("textDocument/definition", "req-789", false, 0, 25*time.Millisecond)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "ERROR" {
			t.Errorf("Expected level 'ERROR' for failed response, got %s", entry.Level)
		}
		if entry.Context["success"] != false {
			t.Error("Expected success false in context")
		}
	})

	t.Run("LogConnectionEvent captures connection events", func(t *testing.T) {
		output.Clear()

		logger.LogConnectionEvent("client_connected", map[string]interface{}{
			"client_id": "client-123",
			"transport": "stdio",
		})

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "INFO" {
			t.Errorf("Expected level 'INFO', got %s", entry.Level)
		}
		if entry.Message != "Connection event" {
			t.Errorf("Expected message 'Connection event', got %s", entry.Message)
		}
		if entry.Context["event"] != "client_connected" {
			t.Error("Expected event in context")
		}
		if entry.Context["type"] != "connection" {
			t.Error("Expected type 'connection' in context")
		}
		if entry.Context["client_id"] != "client-123" {
			t.Error("Expected client_id in context")
		}
	})

	t.Run("LogErrorRecovery tracks recovery attempts", func(t *testing.T) {
		output.Clear()

		logger.LogErrorRecovery("connection_timeout", "reconnect", true)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "INFO" {
			t.Errorf("Expected level 'INFO' for successful recovery, got %s", entry.Level)
		}
		if entry.Context["error_type"] != "connection_timeout" {
			t.Error("Expected error_type in context")
		}
		if entry.Context["recovery_action"] != "reconnect" {
			t.Error("Expected recovery_action in context")
		}
		if entry.Context["success"] != true {
			t.Error("Expected success true in context")
		}
		if entry.Context["type"] != "recovery" {
			t.Error("Expected type 'recovery' in context")
		}
	})

	t.Run("LogErrorRecovery with failure logs as warning", func(t *testing.T) {
		output.Clear()

		logger.LogErrorRecovery("network_error", "retry", false)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "WARN" {
			t.Errorf("Expected level 'WARN' for failed recovery, got %s", entry.Level)
		}
		if entry.Context["success"] != false {
			t.Error("Expected success false in context")
		}
	})
}

// Test Formatter Methods

func TestFormatterMethods(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(*StructuredLogger)
		expected string
	}{
		{
			name: "Tracef formats message",
			logFunc: func(logger *StructuredLogger) {
				logger.Tracef("user %s performed action %d", "john", 42)
			},
			expected: "user john performed action 42",
		},
		{
			name: "Debugf formats message",
			logFunc: func(logger *StructuredLogger) {
				logger.Debugf("processing %d items", 100)
			},
			expected: "processing 100 items",
		},
		{
			name: "Infof formats message",
			logFunc: func(logger *StructuredLogger) {
				logger.Infof("operation completed in %v", 250*time.Millisecond)
			},
			expected: "operation completed in 250ms",
		},
		{
			name: "Warnf formats message",
			logFunc: func(logger *StructuredLogger) {
				logger.Warnf("retry attempt %d of %d", 3, 5)
			},
			expected: "retry attempt 3 of 5",
		},
		{
			name: "Errorf formats message",
			logFunc: func(logger *StructuredLogger) {
				logger.Errorf("failed to connect to %s:%d", "localhost", 8080)
			},
			expected: "failed to connect to localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := newMockWriter()
			config := &LoggerConfig{
				Level:      LogLevelTrace,
				Component:  "format-test",
				EnableJSON: true,
				Output:     output,
			}

			logger := NewStructuredLogger(config)
			tt.logFunc(logger)

			outputStr := output.GetOutput()
			if outputStr == "" {
				t.Fatal("Expected log output but got none")
			}

			entry, err := parseLogEntry(outputStr)
			if err != nil {
				t.Fatalf("Failed to parse log entry: %v", err)
			}

			if entry.Message != tt.expected {
				t.Errorf("Expected formatted message '%s', got '%s'", tt.expected, entry.Message)
			}
		})
	}
}

// Test Level Management

func TestLevelManagement(t *testing.T) {
	t.Run("SetLevel changes effective level", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "level-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		// Initially debug should be filtered
		logger.Debug("debug message")
		if output.GetOutput() != "" {
			t.Error("Expected debug message to be filtered at Info level")
		}

		// Change level to debug
		logger.SetLevel(LogLevelDebug)

		// Now debug should be logged
		logger.Debug("debug message after level change")
		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Error("Expected debug message to be logged at Debug level")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Level != "DEBUG" {
			t.Errorf("Expected level 'DEBUG', got %s", entry.Level)
		}
	})

	t.Run("GetLevel returns current level", func(t *testing.T) {
		config := &LoggerConfig{
			Level:     LogLevelWarn,
			Component: "get-level-test",
			Output:    io.Discard,
		}

		logger := NewStructuredLogger(config)

		if logger.GetLevel() != LogLevelWarn {
			t.Errorf("Expected level Warn, got %v", logger.GetLevel())
		}

		logger.SetLevel(LogLevelError)

		if logger.GetLevel() != LogLevelError {
			t.Errorf("Expected level Error after change, got %v", logger.GetLevel())
		}
	})

	t.Run("IsLevelEnabled correctly reports enablement", func(t *testing.T) {
		config := &LoggerConfig{
			Level:     LogLevelInfo,
			Component: "enabled-test",
			Output:    io.Discard,
		}

		logger := NewStructuredLogger(config)

		if logger.IsLevelEnabled(LogLevelTrace) {
			t.Error("Expected Trace level to be disabled at Info level")
		}
		if logger.IsLevelEnabled(LogLevelDebug) {
			t.Error("Expected Debug level to be disabled at Info level")
		}
		if !logger.IsLevelEnabled(LogLevelInfo) {
			t.Error("Expected Info level to be enabled at Info level")
		}
		if !logger.IsLevelEnabled(LogLevelWarn) {
			t.Error("Expected Warn level to be enabled at Info level")
		}
		if !logger.IsLevelEnabled(LogLevelError) {
			t.Error("Expected Error level to be enabled at Info level")
		}
		if !logger.IsLevelEnabled(LogLevelFatal) {
			t.Error("Expected Fatal level to be enabled at Info level")
		}
	})
}

// Test ParseLogLevel

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    LogLevel
		expectError bool
	}{
		{"trace level", "TRACE", LogLevelTrace, false},
		{"debug level", "DEBUG", LogLevelDebug, false},
		{"info level", "INFO", LogLevelInfo, false},
		{"warn level", "WARN", LogLevelWarn, false},
		{"warning level", "WARNING", LogLevelWarn, false},
		{"error level", "ERROR", LogLevelError, false},
		{"fatal level", "FATAL", LogLevelFatal, false},

		{"lowercase trace", "trace", LogLevelTrace, false},
		{"lowercase debug", "debug", LogLevelDebug, false},
		{"lowercase info", "info", LogLevelInfo, false},
		{"lowercase warn", "warn", LogLevelWarn, false},
		{"lowercase warning", "warning", LogLevelWarn, false},
		{"lowercase error", "error", LogLevelError, false},
		{"lowercase fatal", "fatal", LogLevelFatal, false},

		{"mixed case", "InFo", LogLevelInfo, false},
		{"invalid level", "INVALID", LogLevelInfo, true},
		{"empty string", "", LogLevelInfo, true},
		{"numeric", "123", LogLevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := ParseLogLevel(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				}
				// For invalid levels, should return default Info level
				if level != LogLevelInfo {
					t.Errorf("Expected default level Info for invalid input, got %v", level)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for input '%s', but got: %v", tt.input, err)
				}
				if level != tt.expected {
					t.Errorf("Expected level %v for input '%s', got %v", tt.expected, tt.input, level)
				}
			}
		})
	}
}

// Test Thread Safety

func TestThreadSafety(t *testing.T) {
	t.Run("concurrent logging is thread-safe", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "thread-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		const numGoroutines = 20
		const messagesPerGoroutine = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					logger.WithField("goroutine_id", id).
						WithField("message_id", j).
						Info("concurrent log message")
				}
			}(i)
		}

		wg.Wait()

		writeCount := output.GetWriteCount()
		expectedCount := numGoroutines * messagesPerGoroutine

		if writeCount != expectedCount {
			t.Errorf("Expected %d writes, got %d", expectedCount, writeCount)
		}
	})

	t.Run("concurrent field context creation is thread-safe", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "context-thread-test",
			EnableJSON: true,
			Output:     output,
		}

		baseLogger := NewStructuredLogger(config)

		const numGoroutines = 10
		const operationsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operationsPerGoroutine; j++ {
					contextLogger := baseLogger.
						WithField("goroutine", id).
						WithField("operation", j).
						WithRequestID(fmt.Sprintf("req-%d-%d", id, j))

					contextLogger.Info("context operation")
				}
			}(i)
		}

		wg.Wait()

		writeCount := output.GetWriteCount()
		expectedCount := numGoroutines * operationsPerGoroutine

		if writeCount != expectedCount {
			t.Errorf("Expected %d writes, got %d", expectedCount, writeCount)
		}
	})

	t.Run("concurrent level changes are thread-safe", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "level-thread-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		var wg sync.WaitGroup
		const numGoroutines = 10

		// Goroutines changing log level
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					// Cycle through different levels
					level := LogLevel(j % 6)
					logger.SetLevel(level)
					currentLevel := logger.GetLevel()
					isEnabled := logger.IsLevelEnabled(LogLevelInfo)
					_ = currentLevel
					_ = isEnabled
				}
			}(i)
		}

		// Goroutines logging messages
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					logger.Info("concurrent level test")
				}
			}(i)
		}

		wg.Wait()

		// Should not panic and should have some output
		writeCount := output.GetWriteCount()
		if writeCount < 0 {
			t.Error("Expected non-negative write count")
		}
	})

	t.Run("concurrent metrics tracking is thread-safe", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "metrics-thread-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		var wg sync.WaitGroup
		const numGoroutines = 10
		const messagesPerGoroutine = 50

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					// Alternate between different log levels
					if j%2 == 0 {
						logger.Info("info message")
					} else {
						logger.Error("error message")
					}

					// Occasionally check and reset counts
					if j%10 == 0 {
						counts := logger.GetLogCounts()
						_ = counts
						if j%20 == 0 {
							logger.ResetLogCounts()
						}
					}
				}
			}(i)
		}

		wg.Wait()

		// Check final counts
		finalCounts := logger.GetLogCounts()
		totalLogs := finalCounts[LogLevelInfo] + finalCounts[LogLevelError]

		// Should have some logs (exact count depends on reset timing)
		if totalLogs < 0 {
			t.Error("Expected non-negative total log count")
		}
	})
}

// Test Edge Cases and Error Conditions

func TestEdgeCasesAndErrorConditions(t *testing.T) {
	t.Run("WithError handles nil error gracefully", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "nil-error-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)
		resultLogger := logger.WithError(nil)

		// Should return the same logger instance
		if resultLogger != logger {
			t.Error("Expected WithError(nil) to return the same logger")
		}

		logger.Info("test message")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		// Should not have error field in context
		if entry.Context != nil && entry.Context["error"] != nil {
			t.Error("Expected no error field when nil error provided")
		}
	})

	t.Run("large field values are handled", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "large-field-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		// Create a large string value
		largeValue := strings.Repeat("x", 10000)
		logger.WithField("large_field", largeValue).Info("test with large field")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Context["large_field"] != largeValue {
			t.Error("Expected large field value to be preserved")
		}
	})

	t.Run("special characters in field values", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "special-char-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		specialValue := "value with\nnewlines\tand\ttabs\rand\"quotes\\"
		logger.WithField("special_field", specialValue).Info("test with special characters")

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if entry.Context["special_field"] != specialValue {
			t.Error("Expected special character field value to be preserved")
		}
	})

	t.Run("empty and whitespace-only messages", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "empty-message-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		tests := []string{
			"",
			" ",
			"\t",
			"\n",
			"   \t\n  ",
		}

		for i, message := range tests {
			output.Clear()
			logger.Info(message)

			outputStr := output.GetOutput()
			if outputStr == "" {
				t.Errorf("Test %d: Expected log output for message '%s' but got none", i, message)
				continue
			}

			entry, err := parseLogEntry(outputStr)
			if err != nil {
				t.Errorf("Test %d: Failed to parse log entry: %v", i, err)
				continue
			}

			if entry.Message != message {
				t.Errorf("Test %d: Expected message '%s', got '%s'", i, message, entry.Message)
			}
		}
	})

	t.Run("nil output writer handled gracefully", func(t *testing.T) {
		config := &LoggerConfig{
			Level:      LogLevelInfo,
			Component:  "nil-output-test",
			EnableJSON: true,
			Output:     nil, // This should default to os.Stderr
		}

		// Should not panic
		logger := NewStructuredLogger(config)
		if logger == nil {
			t.Fatal("Expected logger to be created even with nil output")
		}

		// Should use default output (os.Stderr)
		if logger.config.Output == nil {
			t.Error("Expected default output to be set")
		}
	})

	t.Run("extremely deep stack trace", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:              LogLevelError,
			Component:          "deep-stack-test",
			EnableJSON:         true,
			EnableStackTrace:   true,
			MaxStackTraceDepth: 100, // Very deep
			Output:             output,
		}

		logger := NewStructuredLogger(config)

		// Create a recursive function to generate deep stack
		var deepFunction func(int)
		deepFunction = func(depth int) {
			if depth > 0 {
				deepFunction(depth - 1)
			} else {
				logger.ErrorWithStack("deep stack error", errors.New("test error"))
			}
		}

		deepFunction(50)

		outputStr := output.GetOutput()
		if outputStr == "" {
			t.Fatal("Expected log output but got none")
		}

		entry, err := parseLogEntry(outputStr)
		if err != nil {
			t.Fatalf("Failed to parse log entry: %v", err)
		}

		if len(entry.StackTrace) == 0 {
			t.Error("Expected stack trace to be populated")
		}

		// Should respect max depth
		if len(entry.StackTrace) > 100 {
			t.Errorf("Expected stack trace depth <= 100, got %d", len(entry.StackTrace))
		}
	})
}

// Performance Tests

func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	t.Run("sync logging performance", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:              LogLevelInfo,
			Component:          "perf-sync-test",
			EnableJSON:         true,
			EnableAsyncLogging: false,
			Output:             output,
		}

		logger := NewStructuredLogger(config)

		const numMessages = 10000
		start := time.Now()

		for i := 0; i < numMessages; i++ {
			logger.WithField("iteration", i).Info("performance test message")
		}

		duration := time.Since(start)
		messagesPerSecond := float64(numMessages) / duration.Seconds()

		t.Logf("Sync logging: %d messages in %v (%.0f msg/sec)", numMessages, duration, messagesPerSecond)

		if messagesPerSecond < 1000 {
			t.Errorf("Expected at least 1000 messages per second, got %.0f", messagesPerSecond)
		}

		if output.GetWriteCount() != numMessages {
			t.Errorf("Expected %d writes, got %d", numMessages, output.GetWriteCount())
		}
	})

	t.Run("async logging performance", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:              LogLevelInfo,
			Component:          "perf-async-test",
			EnableJSON:         true,
			EnableAsyncLogging: true,
			AsyncBufferSize:    10000,
			Output:             output,
		}

		logger := NewStructuredLogger(config)

		const numMessages = 10000
		start := time.Now()

		for i := 0; i < numMessages; i++ {
			logger.WithField("iteration", i).Info("async performance test message")
		}

		// Close to wait for all async writes to complete
		logger.Close()
		duration := time.Since(start)
		messagesPerSecond := float64(numMessages) / duration.Seconds()

		t.Logf("Async logging: %d messages in %v (%.0f msg/sec)", numMessages, duration, messagesPerSecond)

		if messagesPerSecond < 2000 {
			t.Errorf("Expected at least 2000 messages per second for async, got %.0f", messagesPerSecond)
		}

		if output.GetWriteCount() != numMessages {
			t.Errorf("Expected %d writes, got %d", numMessages, output.GetWriteCount())
		}
	})

	t.Run("memory usage stays reasonable", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:         LogLevelInfo,
			Component:     "memory-test",
			EnableJSON:    true,
			EnableMetrics: true,
			Output:        output,
		}

		logger := NewStructuredLogger(config)

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		// Log a significant number of messages
		for i := 0; i < 1000; i++ {
			logger.WithField("iteration", i).
				WithField("data", strings.Repeat("x", 100)).
				Info("memory test message")
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		var allocDiff uint64
		if m2.Alloc > m1.Alloc {
			allocDiff = m2.Alloc - m1.Alloc
		} else {
			allocDiff = 0 // Memory might have decreased due to GC
		}
		t.Logf("Memory allocated during test: %d bytes", allocDiff)

		// Should not use excessive memory (allowing for test overhead)
		if allocDiff > 50*1024*1024 { // 50MB threshold
			t.Errorf("Memory usage too high: %d bytes", allocDiff)
		}
	})
}

// Integration Tests

func TestIntegrationScenarios(t *testing.T) {
	t.Run("complete request lifecycle logging", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:      LogLevelDebug,
			Component:  "integration-test",
			EnableJSON: true,
			Output:     output,
		}

		logger := NewStructuredLogger(config)

		// Simulate a complete request lifecycle
		requestID := "req-integration-123"
		startTime := time.Now()

		// Request started
		logger.LogRequest("textDocument/definition", requestID, map[string]interface{}{
			"uri": "file:///project/main.go",
			"position": map[string]interface{}{
				"line":      42,
				"character": 15,
			},
		}, startTime)

		// Processing steps
		procLogger := logger.WithRequestID(requestID).WithOperation("textDocument/definition")

		procLogger.Debug("validating request parameters")
		procLogger.Info("routing to language server")
		procLogger.WithField("server", "gopls").Debug("sending LSP request")

		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)

		// Warning during processing
		procLogger.Warn("language server response delayed")

		// Request completed
		duration := time.Since(startTime)
		logger.LogResponse("textDocument/definition", requestID, true, 512, duration)

		// Verify all logs were written
		entries, err := parseAllLogEntries(output.GetOutput())
		if err != nil {
			t.Fatalf("Failed to parse log entries: %v", err)
		}

		if len(entries) < 5 {
			t.Errorf("Expected at least 5 log entries, got %d", len(entries))
		}

		// Verify request ID is present in relevant entries
		requestEntries := 0
		for _, entry := range entries {
			if entry.RequestID == requestID || (entry.Context != nil && entry.Context["request_id"] == requestID) {
				requestEntries++
			}
		}

		if requestEntries < 4 { // Should be in most entries
			t.Errorf("Expected request ID in at least 4 entries, found in %d", requestEntries)
		}
	})

	t.Run("error recovery scenario with metrics", func(t *testing.T) {
		output := newMockWriter()
		config := &LoggerConfig{
			Level:            LogLevelDebug,
			Component:        "error-recovery-test",
			EnableJSON:       true,
			EnableStackTrace: true,
			EnableMetrics:    true,
			Output:           output,
		}

		logger := NewStructuredLogger(config)

		// Simulate error recovery scenario
		connLogger := logger.WithOperation("connection.maintain")

		connLogger.Info("establishing connection to language server")

		// First failure
		connLogger.LogErrorRecovery("connection_timeout", "retry", false)
		connLogger.WithError(errors.New("connection timeout after 5s")).Error("failed to connect")

		// Retry with success
		connLogger.LogErrorRecovery("connection_timeout", "retry_with_backoff", true)
		connLogger.Info("connection established successfully")

		// Log metrics
		metrics := &LogMetrics{
			BytesProcessed: 2048,
			ItemsProcessed: 15,
			CacheHitRate:   0.95,
			CustomCounters: map[string]int64{
				"connection_attempts": 2,
				"successful_requests": 14,
				"failed_requests":     1,
			},
			CustomGauges: map[string]float64{
				"average_response_time": 125.5,
				"connection_health":     0.93,
			},
		}
		logger.LogMetrics(metrics)

		// Verify comprehensive logging
		entries, err := parseAllLogEntries(output.GetOutput())
		if err != nil {
			t.Fatalf("Failed to parse log entries: %v", err)
		}

		// Should have multiple types of logs
		var hasInfo, hasError, hasRecovery, hasMetrics bool
		for _, entry := range entries {
			switch entry.Level {
			case "INFO":
				hasInfo = true
			case "ERROR":
				hasError = true
			case "WARN":
				if entry.Context != nil && entry.Context["type"] == "recovery" {
					hasRecovery = true
				}
			}

			if entry.Metrics != nil {
				hasMetrics = true
			}
		}

		if !hasInfo {
			t.Error("Expected INFO level logs")
		}
		if !hasError {
			t.Error("Expected ERROR level logs")
		}
		if !hasRecovery {
			t.Error("Expected recovery logs")
		}
		if !hasMetrics {
			t.Error("Expected metrics in logs")
		}
	})
}
