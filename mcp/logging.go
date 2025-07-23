package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

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

type LogMetrics struct {
	BytesProcessed int64              `json:"bytes_processed,omitempty"`
	ItemsProcessed int                `json:"items_processed,omitempty"`
	CacheHitRate   float64            `json:"cache_hit_rate,omitempty"`
	MemoryUsage    int64              `json:"memory_usage,omitempty"`
	GoroutineCount int                `json:"goroutine_count,omitempty"`
	CustomCounters map[string]int64   `json:"custom_counters,omitempty"`
	CustomGauges   map[string]float64 `json:"custom_gauges,omitempty"`
}

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

type StructuredLogger struct {
	config     *LoggerConfig
	output     io.Writer
	mu         sync.RWMutex
	fields     map[string]interface{}
	baseLogger *log.Logger

	asyncChan chan *LogEntry
	asyncDone chan struct{}
	asyncWG   sync.WaitGroup

	logCounts map[LogLevel]int64
	lastReset time.Time
	metricsMu *sync.RWMutex
}

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
		metricsMu:  &sync.RWMutex{},
	}

	if config.EnableAsyncLogging {
		logger.asyncChan = make(chan *LogEntry, config.AsyncBufferSize)
		logger.asyncDone = make(chan struct{})
		logger.asyncWG.Add(1)
		go logger.asyncWriter()
	}

	return logger
}

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
		metricsMu:  l.metricsMu,
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	newLogger.fields[key] = value

	return newLogger
}

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
		metricsMu:  l.metricsMu,
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

func (l *StructuredLogger) WithError(err error) *StructuredLogger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

func (l *StructuredLogger) WithRequestID(requestID string) *StructuredLogger {
	return l.WithField(LOG_FIELD_REQUEST_ID, requestID)
}

func (l *StructuredLogger) WithOperation(operation string) *StructuredLogger {
	return l.WithField("operation", operation)
}

func (l *StructuredLogger) WithDuration(duration time.Duration) *StructuredLogger {
	return l.WithField("duration", duration.String())
}

func (l *StructuredLogger) WithMetrics(metrics *LogMetrics) *StructuredLogger {
	return l.WithField("metrics", metrics)
}

func (l *StructuredLogger) Trace(message string) {
	l.log(LogLevelTrace, message, nil)
}

func (l *StructuredLogger) Tracef(format string, args ...interface{}) {
	l.log(LogLevelTrace, fmt.Sprintf(format, args...), nil)
}

func (l *StructuredLogger) Debug(message string) {
	l.log(LogLevelDebug, message, nil)
}

func (l *StructuredLogger) Debugf(format string, args ...interface{}) {
	l.log(LogLevelDebug, fmt.Sprintf(format, args...), nil)
}

func (l *StructuredLogger) Info(message string) {
	l.log(LogLevelInfo, message, nil)
}

func (l *StructuredLogger) Infof(format string, args ...interface{}) {
	l.log(LogLevelInfo, fmt.Sprintf(format, args...), nil)
}

func (l *StructuredLogger) Warn(message string) {
	l.log(LogLevelWarn, message, nil)
}

func (l *StructuredLogger) Warnf(format string, args ...interface{}) {
	l.log(LogLevelWarn, fmt.Sprintf(format, args...), nil)
}

func (l *StructuredLogger) Error(message string) {
	l.log(LogLevelError, message, nil)
}

func (l *StructuredLogger) Errorf(format string, args ...interface{}) {
	l.log(LogLevelError, fmt.Sprintf(format, args...), nil)
}

func (l *StructuredLogger) ErrorWithStack(message string, err error) {
	if LogLevelError < l.config.Level {
		return
	}

	entry := l.createEntry(LogLevelError, message, err)
	if l.config.EnableStackTrace {
		entry.StackTrace = l.getStackTrace()
	}
	l.updateLogCounts(LogLevelError)
	l.writeEntry(entry)
}

func (l *StructuredLogger) LogRequest(method, requestID string, params interface{}, startTime time.Time) {
	duration := time.Since(startTime)

	l.WithFields(map[string]interface{}{
		"method":             method,
		LOG_FIELD_REQUEST_ID: requestID,
		"params":             params,
		"duration":           duration.String(),
		"type":               "request",
	}).Info("Processing request")
}

func (l *StructuredLogger) LogResponse(method, requestID string, success bool, responseSize int, duration time.Duration) {
	level := LogLevelInfo
	if !success {
		level = LogLevelError
	}

	l.WithFields(map[string]interface{}{
		"method":             method,
		LOG_FIELD_REQUEST_ID: requestID,
		"success":            success,
		"response_size":      responseSize,
		"duration":           duration.String(),
		"type":               "response",
	}).log(level, "Request completed", nil)
}

func (l *StructuredLogger) LogMetrics(metrics *LogMetrics) {
	l.WithMetrics(metrics).Info("Performance metrics")
}

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

func (l *StructuredLogger) log(level LogLevel, message string, err error) {
	if level < l.config.Level {
		return
	}

	entry := l.createEntry(level, message, err)
	l.updateLogCounts(level)
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

	if err != nil {
		entry.Error = err.Error()
	}

	if l.config.EnableCaller {
		if caller := l.getCaller(); caller != "" {
			entry.Caller = caller
		}
	}

	if len(l.fields) > 0 {
		entry.Context = make(map[string]interface{})
		for k, v := range l.fields {
			switch k {
			case LOG_FIELD_REQUEST_ID:
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

	if l.config.EnableMetrics && entry.Metrics == nil {
		entry.Metrics = l.gatherRuntimeMetrics()
	}

	return entry
}

func (l *StructuredLogger) writeEntry(entry *LogEntry) {
	if l.config.EnableAsyncLogging && l.asyncChan != nil {
		select {
		case l.asyncChan <- entry:
		default:
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
			output = fmt.Sprintf("%s [%s] %s: %s\n",
				entry.Timestamp.Format(l.config.TimestampFormat),
				entry.Level, entry.Component, entry.Message)
		} else {
			output = string(jsonData) + "\n"
		}
	} else {
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
		parts = append(parts, "("+entry.Operation+")")
	}

	if entry.RequestID != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.RequestID))
	}

	parts = append(parts, entry.Message)

	if entry.Duration != "" {
		parts = append(parts, "("+entry.Duration+")")
	}

	if entry.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%s", entry.Error))
	}

	if entry.Caller != "" {
		parts = append(parts, fmt.Sprintf("caller=%s", entry.Caller))
	}

	output := strings.Join(parts, " ") + "\n"

	if len(entry.Context) > 0 {
		for k, v := range entry.Context {
			output += fmt.Sprintf("  %s=%v\n", k, v)
		}
	}

	return output
}

func (l *StructuredLogger) getCaller() string {
	_, file, line, ok := runtime.Caller(4)
	if !ok {
		return ""
	}

	parts := strings.Split(file, "/")
	if len(parts) > 0 {
		file = parts[len(parts)-1]
	}

	return file + ":" + strconv.Itoa(line)
}

func (l *StructuredLogger) getStackTrace() []string {
	var stack []string

	pcs := make([]uintptr, l.config.MaxStackTraceDepth)
	n := runtime.Callers(3, pcs)

	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !more {
			break
		}

		stack = append(stack, fmt.Sprintf("%s (%s:%d)",
			frame.Function, frame.File, frame.Line))
	}

	return stack
}

func (l *StructuredLogger) gatherRuntimeMetrics() *LogMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Prevent integer overflow when converting uint64 to int64
	var memoryUsage int64
	if memStats.Alloc > 9223372036854775807 { // max int64 value
		memoryUsage = 9223372036854775807 // clamp to max int64
	} else {
		memoryUsage = int64(memStats.Alloc)
	}

	return &LogMetrics{
		MemoryUsage:    memoryUsage,
		GoroutineCount: runtime.NumGoroutine(),
	}
}

func (l *StructuredLogger) updateLogCounts(level LogLevel) {
	l.metricsMu.Lock()
	defer l.metricsMu.Unlock()

	l.logCounts[level]++
}

func (l *StructuredLogger) asyncWriter() {
	defer l.asyncWG.Done()

	for {
		select {
		case entry := <-l.asyncChan:
			l.writeEntrySync(entry)
		case <-l.asyncDone:
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

func (l *StructuredLogger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Level = level
}

func (l *StructuredLogger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config.Level
}

func (l *StructuredLogger) GetLogCounts() map[LogLevel]int64 {
	l.metricsMu.RLock()
	defer l.metricsMu.RUnlock()

	counts := make(map[LogLevel]int64)
	for level, count := range l.logCounts {
		counts[level] = count
	}

	return counts
}

func (l *StructuredLogger) ResetLogCounts() {
	l.metricsMu.Lock()
	defer l.metricsMu.Unlock()

	for level := range l.logCounts {
		l.logCounts[level] = 0
	}
	l.lastReset = time.Now()
}

func (l *StructuredLogger) Close() error {
	if l.config.EnableAsyncLogging && l.asyncDone != nil {
		close(l.asyncDone)
		l.asyncWG.Wait()
	}
	return nil
}

func (l *StructuredLogger) IsLevelEnabled(level LogLevel) bool {
	return level >= l.config.Level
}

// GetConfig returns the logger configuration for testing purposes
func (l *StructuredLogger) GetConfig() *LoggerConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}
