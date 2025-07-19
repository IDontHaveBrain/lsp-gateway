package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents different logging levels
type LogLevel int

const (
	LogLevelTrace LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

var logLevelNames = map[LogLevel]string{
	LogLevelTrace: "TRACE",
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
	LogLevelFatal: "FATAL",
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Component  string                 `json:"component"`
	Operation  string                 `json:"operation,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Duration   string                 `json:"duration,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Caller     string                 `json:"caller,omitempty"`
	StackTrace []string               `json:"stack_trace,omitempty"`
	Metrics    *LogMetrics            `json:"metrics,omitempty"`
}

// LogMetrics represents performance and operational metrics
type LogMetrics struct {
	BytesProcessed int64              `json:"bytes_processed,omitempty"`
	ItemsProcessed int                `json:"items_processed,omitempty"`
	CacheHitRate   float64            `json:"cache_hit_rate,omitempty"`
	MemoryUsage    int64              `json:"memory_usage,omitempty"`
	GoroutineCount int                `json:"goroutine_count,omitempty"`
	CustomCounters map[string]int64   `json:"custom_counters,omitempty"`
	CustomGauges   map[string]float64 `json:"custom_gauges,omitempty"`
}

// LoggerConfig represents logger configuration
type LoggerConfig struct {
	Level              LogLevel
	Component          string
	EnableJSON         bool
	EnableStackTrace   bool
	EnableCaller       bool
	EnableMetrics      bool
	Output             io.Writer
	IncludeTimestamp   bool
	TimestampFormat    string
	MaxStackTraceDepth int
	EnableAsyncLogging bool
	AsyncBufferSize    int
}

// StructuredLogger provides comprehensive structured logging capabilities
type StructuredLogger struct {
	config     *LoggerConfig
	output     io.Writer
	mu         sync.RWMutex
	fields     map[string]interface{}
	baseLogger *log.Logger

	// Async logging support
	asyncChan chan *LogEntry
	asyncDone chan struct{}
	asyncWG   sync.WaitGroup

	// Metrics tracking
	logCounts map[LogLevel]int64
	lastReset time.Time
	metricsMu sync.RWMutex
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(config *LoggerConfig) *StructuredLogger {
	if config == nil {
		config = &LoggerConfig{
			Level:              LogLevelInfo,
			Component:          "mcp",
			EnableJSON:         true,
			EnableStackTrace:   false,
			EnableCaller:       true,
			EnableMetrics:      true,
			Output:             os.Stderr,
			IncludeTimestamp:   true,
			TimestampFormat:    time.RFC3339Nano,
			MaxStackTraceDepth: 10,
			EnableAsyncLogging: false,
			AsyncBufferSize:    1000,
		}
	}

	if config.Output == nil {
		config.Output = os.Stderr
	}

	logger := &StructuredLogger{
		config:     config,
		output:     config.Output,
		fields:     make(map[string]interface{}),
		baseLogger: log.New(config.Output, "", 0),
		logCounts:  make(map[LogLevel]int64),
		lastReset:  time.Now(),
	}

	// Initialize async logging if enabled
	if config.EnableAsyncLogging {
		logger.asyncChan = make(chan *LogEntry, config.AsyncBufferSize)
		logger.asyncDone = make(chan struct{})
		logger.asyncWG.Add(1)
		go logger.asyncWriter()
	}

	return logger
}

// WithField adds a field to the logger context
func (l *StructuredLogger) WithField(key string, value interface{}) *StructuredLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &StructuredLogger{
		config:     l.config,
		output:     l.output,
		fields:     make(map[string]interface{}),
		baseLogger: l.baseLogger,
		asyncChan:  l.asyncChan,
		asyncDone:  l.asyncDone,
		logCounts:  l.logCounts,
		lastReset:  l.lastReset,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new field
	newLogger.fields[key] = value

	return newLogger
}

// WithFields adds multiple fields to the logger context
func (l *StructuredLogger) WithFields(fields map[string]interface{}) *StructuredLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &StructuredLogger{
		config:     l.config,
		output:     l.output,
		fields:     make(map[string]interface{}),
		baseLogger: l.baseLogger,
		asyncChan:  l.asyncChan,
		asyncDone:  l.asyncDone,
		logCounts:  l.logCounts,
		lastReset:  l.lastReset,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

// WithError adds an error to the logger context
func (l *StructuredLogger) WithError(err error) *StructuredLogger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// WithRequestID adds a request ID for tracing
func (l *StructuredLogger) WithRequestID(requestID string) *StructuredLogger {
	return l.WithField("request_id", requestID)
}

// WithOperation adds an operation name for tracking
func (l *StructuredLogger) WithOperation(operation string) *StructuredLogger {
	return l.WithField("operation", operation)
}

// WithDuration adds duration timing information
func (l *StructuredLogger) WithDuration(duration time.Duration) *StructuredLogger {
	return l.WithField("duration", duration.String())
}

// WithMetrics adds performance metrics
func (l *StructuredLogger) WithMetrics(metrics *LogMetrics) *StructuredLogger {
	return l.WithField("metrics", metrics)
}

// Trace logs a trace level message
func (l *StructuredLogger) Trace(message string) {
	l.log(LogLevelTrace, message, nil)
}

// Tracef logs a formatted trace level message
func (l *StructuredLogger) Tracef(format string, args ...interface{}) {
	l.log(LogLevelTrace, fmt.Sprintf(format, args...), nil)
}

// Debug logs a debug level message
func (l *StructuredLogger) Debug(message string) {
	l.log(LogLevelDebug, message, nil)
}

// Debugf logs a formatted debug level message
func (l *StructuredLogger) Debugf(format string, args ...interface{}) {
	l.log(LogLevelDebug, fmt.Sprintf(format, args...), nil)
}

// Info logs an info level message
func (l *StructuredLogger) Info(message string) {
	l.log(LogLevelInfo, message, nil)
}

// Infof logs a formatted info level message
func (l *StructuredLogger) Infof(format string, args ...interface{}) {
	l.log(LogLevelInfo, fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning level message
func (l *StructuredLogger) Warn(message string) {
	l.log(LogLevelWarn, message, nil)
}

// Warnf logs a formatted warning level message
func (l *StructuredLogger) Warnf(format string, args ...interface{}) {
	l.log(LogLevelWarn, fmt.Sprintf(format, args...), nil)
}

// Error logs an error level message
func (l *StructuredLogger) Error(message string) {
	l.log(LogLevelError, message, nil)
}

// Errorf logs a formatted error level message
func (l *StructuredLogger) Errorf(format string, args ...interface{}) {
	l.log(LogLevelError, fmt.Sprintf(format, args...), nil)
}

// ErrorWithStack logs an error with stack trace
func (l *StructuredLogger) ErrorWithStack(message string, err error) {
	entry := l.createEntry(LogLevelError, message, err)
	if l.config.EnableStackTrace {
		entry.StackTrace = l.getStackTrace()
	}
	l.writeEntry(entry)
}

// Fatal logs a fatal level message and exits
func (l *StructuredLogger) Fatal(message string) {
	l.log(LogLevelFatal, message, nil)
	os.Exit(1)
}

// Fatalf logs a formatted fatal level message and exits
func (l *StructuredLogger) Fatalf(format string, args ...interface{}) {
	l.log(LogLevelFatal, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// LogRequest logs an incoming request with details
func (l *StructuredLogger) LogRequest(method, requestID string, params interface{}, startTime time.Time) {
	duration := time.Since(startTime)

	l.WithFields(map[string]interface{}{
		"method":     method,
		"request_id": requestID,
		"params":     params,
		"duration":   duration.String(),
		"type":       "request",
	}).Info("Processing request")
}

// LogResponse logs a response with details
func (l *StructuredLogger) LogResponse(method, requestID string, success bool, responseSize int, duration time.Duration) {
	level := LogLevelInfo
	if !success {
		level = LogLevelError
	}

	l.WithFields(map[string]interface{}{
		"method":        method,
		"request_id":    requestID,
		"success":       success,
		"response_size": responseSize,
		"duration":      duration.String(),
		"type":          "response",
	}).log(level, "Request completed", nil)
}

// LogMetrics logs performance metrics
func (l *StructuredLogger) LogMetrics(metrics *LogMetrics) {
	l.WithMetrics(metrics).Info("Performance metrics")
}

// LogConnectionEvent logs connection-related events
func (l *StructuredLogger) LogConnectionEvent(event string, details map[string]interface{}) {
	fields := map[string]interface{}{
		"event": event,
		"type":  "connection",
	}

	for k, v := range details {
		fields[k] = v
	}

	l.WithFields(fields).Info("Connection event")
}

// LogErrorRecovery logs error recovery attempts
func (l *StructuredLogger) LogErrorRecovery(errorType string, recoveryAction string, success bool) {
	level := LogLevelWarn
	if success {
		level = LogLevelInfo
	}

	l.WithFields(map[string]interface{}{
		"error_type":      errorType,
		"recovery_action": recoveryAction,
		"success":         success,
		"type":            "recovery",
	}).log(level, "Error recovery attempt", nil)
}

// Core logging implementation

func (l *StructuredLogger) log(level LogLevel, message string, err error) {
	if level < l.config.Level {
		return
	}

	entry := l.createEntry(level, message, err)
	l.writeEntry(entry)
}

func (l *StructuredLogger) createEntry(level LogLevel, message string, err error) *LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry := &LogEntry{
		Level:     logLevelNames[level],
		Message:   message,
		Component: l.config.Component,
	}

	if l.config.IncludeTimestamp {
		entry.Timestamp = time.Now()
	}

	// Add error information
	if err != nil {
		entry.Error = err.Error()
	}

	// Add caller information
	if l.config.EnableCaller {
		if caller := l.getCaller(); caller != "" {
			entry.Caller = caller
		}
	}

	// Add context fields
	if len(l.fields) > 0 {
		entry.Context = make(map[string]interface{})
		for k, v := range l.fields {
			switch k {
			case "request_id":
				if requestID, ok := v.(string); ok {
					entry.RequestID = requestID
				}
			case "operation":
				if operation, ok := v.(string); ok {
					entry.Operation = operation
				}
			case "duration":
				if duration, ok := v.(string); ok {
					entry.Duration = duration
				}
			case "metrics":
				if metrics, ok := v.(*LogMetrics); ok {
					entry.Metrics = metrics
				}
			default:
				entry.Context[k] = v
			}
		}
	}

	// Add runtime metrics if enabled
	if l.config.EnableMetrics && entry.Metrics == nil {
		entry.Metrics = l.gatherRuntimeMetrics()
	}

	// Update log counts
	l.updateLogCounts(level)

	return entry
}

func (l *StructuredLogger) writeEntry(entry *LogEntry) {
	if l.config.EnableAsyncLogging && l.asyncChan != nil {
		select {
		case l.asyncChan <- entry:
			// Successfully queued
		default:
			// Buffer full, write synchronously
			l.writeEntrySync(entry)
		}
	} else {
		l.writeEntrySync(entry)
	}
}

func (l *StructuredLogger) writeEntrySync(entry *LogEntry) {
	var output string

	if l.config.EnableJSON {
		jsonData, err := json.Marshal(entry)
		if err != nil {
			// Fallback to simple format if JSON marshaling fails
			output = fmt.Sprintf("%s [%s] %s: %s\n",
				entry.Timestamp.Format(l.config.TimestampFormat),
				entry.Level, entry.Component, entry.Message)
		} else {
			output = string(jsonData) + "\n"
		}
	} else {
		// Human-readable format
		output = l.formatEntryHuman(entry)
	}

	_, _ = l.output.Write([]byte(output))
}

func (l *StructuredLogger) formatEntryHuman(entry *LogEntry) string {
	var parts []string

	if l.config.IncludeTimestamp {
		parts = append(parts, entry.Timestamp.Format(l.config.TimestampFormat))
	}

	parts = append(parts, fmt.Sprintf("[%s]", entry.Level))
	parts = append(parts, entry.Component)

	if entry.Operation != "" {
		parts = append(parts, fmt.Sprintf("(%s)", entry.Operation))
	}

	if entry.RequestID != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.RequestID))
	}

	parts = append(parts, entry.Message)

	if entry.Duration != "" {
		parts = append(parts, fmt.Sprintf("(%s)", entry.Duration))
	}

	if entry.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%s", entry.Error))
	}

	if entry.Caller != "" {
		parts = append(parts, fmt.Sprintf("caller=%s", entry.Caller))
	}

	output := strings.Join(parts, " ") + "\n"

	// Add context fields if any
	if len(entry.Context) > 0 {
		for k, v := range entry.Context {
			output += fmt.Sprintf("  %s=%v\n", k, v)
		}
	}

	return output
}

func (l *StructuredLogger) getCaller() string {
	// Skip runtime.Callers, getCaller, log, and the actual log method
	_, file, line, ok := runtime.Caller(4)
	if !ok {
		return ""
	}

	// Get just the filename, not the full path
	parts := strings.Split(file, "/")
	if len(parts) > 0 {
		file = parts[len(parts)-1]
	}

	return fmt.Sprintf("%s:%d", file, line)
}

func (l *StructuredLogger) getStackTrace() []string {
	var stack []string

	// Skip some frames to get to the actual caller
	pcs := make([]uintptr, l.config.MaxStackTraceDepth)
	n := runtime.Callers(5, pcs)

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !more {
			break
		}

		// Format: function (file:line)
		stack = append(stack, fmt.Sprintf("%s (%s:%d)",
			frame.Function, frame.File, frame.Line))
	}

	return stack
}

func (l *StructuredLogger) gatherRuntimeMetrics() *LogMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &LogMetrics{
		MemoryUsage:    int64(memStats.Alloc),
		GoroutineCount: runtime.NumGoroutine(),
	}
}

func (l *StructuredLogger) updateLogCounts(level LogLevel) {
	l.metricsMu.Lock()
	defer l.metricsMu.Unlock()

	l.logCounts[level]++
}

// Async logging support

func (l *StructuredLogger) asyncWriter() {
	defer l.asyncWG.Done()

	for {
		select {
		case entry := <-l.asyncChan:
			l.writeEntrySync(entry)
		case <-l.asyncDone:
			// Drain remaining entries
			for {
				select {
				case entry := <-l.asyncChan:
					l.writeEntrySync(entry)
				default:
					return
				}
			}
		}
	}
}

// Utility and management functions

// SetLevel changes the logging level
func (l *StructuredLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Level = level
}

// GetLevel returns the current logging level
func (l *StructuredLogger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config.Level
}

// GetLogCounts returns current log counts by level
func (l *StructuredLogger) GetLogCounts() map[LogLevel]int64 {
	l.metricsMu.RLock()
	defer l.metricsMu.RUnlock()

	counts := make(map[LogLevel]int64)
	for level, count := range l.logCounts {
		counts[level] = count
	}

	return counts
}

// ResetLogCounts resets the log counters
func (l *StructuredLogger) ResetLogCounts() {
	l.metricsMu.Lock()
	defer l.metricsMu.Unlock()

	for level := range l.logCounts {
		l.logCounts[level] = 0
	}
	l.lastReset = time.Now()
}

// Close gracefully shuts down the logger
func (l *StructuredLogger) Close() error {
	if l.config.EnableAsyncLogging && l.asyncDone != nil {
		close(l.asyncDone)
		l.asyncWG.Wait()
	}
	return nil
}

// IsLevelEnabled checks if a log level is enabled
func (l *StructuredLogger) IsLevelEnabled(level LogLevel) bool {
	return level >= l.config.Level
}

// ParseLogLevel parses a string log level
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "TRACE":
		return LogLevelTrace, nil
	case "DEBUG":
		return LogLevelDebug, nil
	case "INFO":
		return LogLevelInfo, nil
	case "WARN", "WARNING":
		return LogLevelWarn, nil
	case "ERROR":
		return LogLevelError, nil
	case "FATAL":
		return LogLevelFatal, nil
	default:
		return LogLevelInfo, fmt.Errorf("unknown log level: %s", level)
	}
}
