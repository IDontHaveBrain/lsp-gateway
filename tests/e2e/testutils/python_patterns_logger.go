package testutils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Context    map[string]interface{} `json:"context,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Source     LogSource              `json:"source"`
	TestInfo   TestInfo               `json:"test_info,omitempty"`
}

type LogSource struct {
	File     string `json:"file"`
	Function string `json:"function"`
	Line     int    `json:"line"`
}

type TestInfo struct {
	TestName  string `json:"test_name,omitempty"`
	TestPhase string `json:"test_phase,omitempty"`
	TestID    string `json:"test_id,omitempty"`
}

type TestLogger struct {
	ServerLogs    []string          `json:"server_logs"`
	RequestLogs   []RequestLog      `json:"request_logs"`
	ErrorLogs     []ErrorLog        `json:"error_logs"`
	MetricsLogs   []MetricsSnapshot `json:"metrics_logs"`
	mu            sync.RWMutex
}

type RequestLog struct {
	Method    string                 `json:"method"`
	URL       string                 `json:"url"`
	Duration  time.Duration          `json:"duration"`
	Status    int                    `json:"status"`
	Error     string                 `json:"error,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

type ErrorLog struct {
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Recoverable bool                 `json:"recoverable"`
	RetryCount  int                  `json:"retry_count"`
}

type MetricsSnapshot struct {
	TotalRequests     int64         `json:"total_requests"`
	SuccessfulReqs    int64         `json:"successful_requests"`
	FailedRequests    int64         `json:"failed_requests"`
	AverageLatency    time.Duration `json:"average_latency"`
	ThroughputRPS     float64       `json:"throughput_rps"`
	MemoryUsageMB     int64         `json:"memory_usage_mb"`
	Timestamp         time.Time     `json:"timestamp"`
}

type LoggingConfig struct {
	Level            LogLevel
	EnableConsole    bool
	EnableFile       bool
	FilePath         string
	EnableJSON       bool
	EnableStackTrace bool
	BufferSize       int
	FlushInterval    time.Duration
	MaxFileSize      int64
	MaxBackups       int
	MaxAge           int
	TestInfo         TestInfo
}

func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:            LogLevelInfo,
		EnableConsole:    true,
		EnableFile:       true,
		FilePath:         "/tmp/python-patterns-test.log",
		EnableJSON:       true,
		EnableStackTrace: false,
		BufferSize:       1000,
		FlushInterval:    5 * time.Second,
		MaxFileSize:      100 * 1024 * 1024, // 100MB
		MaxBackups:       5,
		MaxAge:           30,
	}
}

type PythonPatternsLogger struct {
	config      LoggingConfig
	entries     []LogEntry
	testLogger  *TestLogger
	fileWriter  *os.File
	mu          sync.RWMutex
	stopChan    chan struct{}
	running     bool
}

func NewPythonPatternsLogger(config LoggingConfig) *PythonPatternsLogger {
	logger := &PythonPatternsLogger{
		config:     config,
		entries:    make([]LogEntry, 0, config.BufferSize),
		testLogger: &TestLogger{
			ServerLogs:  make([]string, 0),
			RequestLogs: make([]RequestLog, 0),
			ErrorLogs:   make([]ErrorLog, 0),
			MetricsLogs: make([]MetricsSnapshot, 0),
		},
		stopChan: make(chan struct{}),
	}

	if config.EnableFile {
		logger.initializeFileWriter()
	}

	logger.startBackgroundFlusher()
	return logger
}

func (l *PythonPatternsLogger) initializeFileWriter() {
	dir := filepath.Dir(l.config.FilePath)
	if dir != "." {
		os.MkdirAll(dir, 0755)
	}

	file, err := os.OpenFile(l.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file %s: %v\n", l.config.FilePath, err)
		return
	}

	l.fileWriter = file
}

func (l *PythonPatternsLogger) startBackgroundFlusher() {
	l.running = true
	go func() {
		ticker := time.NewTicker(l.config.FlushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-l.stopChan:
				l.flush()
				return
			case <-ticker.C:
				l.flush()
			}
		}
	}()
}

func (l *PythonPatternsLogger) log(level LogLevel, message string, context map[string]interface{}) {
	if level < l.config.Level {
		return
	}

	_, file, line, ok := runtime.Caller(2)
	var source LogSource
	if ok {
		source = LogSource{
			File:     filepath.Base(file),
			Function: getFunctionName(2),
			Line:     line,
		}
	}

	entry := LogEntry{
		Timestamp:  time.Now(),
		Level:      level.String(),
		Message:    message,
		Context:    context,
		Source:     source,
		TestInfo:   l.config.TestInfo,
		StackTrace: l.getStackTrace(level),
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	
	if len(l.entries) >= l.config.BufferSize {
		l.mu.Unlock()
		l.flush()
	} else {
		l.mu.Unlock()
	}

	if l.config.EnableConsole {
		l.printToConsole(entry)
	}
}

func (l *PythonPatternsLogger) getStackTrace(level LogLevel) string {
	if !l.config.EnableStackTrace || level < LogLevelError {
		return ""
	}

	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func getFunctionName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip + 1)
	if !ok {
		return "unknown"
	}
	
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	
	return fn.Name()
}

func (l *PythonPatternsLogger) printToConsole(entry LogEntry) {
	if l.config.EnableJSON {
		jsonData, _ := json.Marshal(entry)
		fmt.Println(string(jsonData))
	} else {
		contextStr := ""
		if len(entry.Context) > 0 {
			if contextData, err := json.Marshal(entry.Context); err == nil {
				contextStr = fmt.Sprintf(" | %s", string(contextData))
			}
		}
		
		fmt.Printf("[%s] %s %s:%d - %s%s\n",
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.Level,
			entry.Source.File,
			entry.Source.Line,
			entry.Message,
			contextStr)
	}
}

func (l *PythonPatternsLogger) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.entries) == 0 {
		return
	}

	if l.fileWriter != nil {
		for _, entry := range l.entries {
			var logLine string
			if l.config.EnableJSON {
				if jsonData, err := json.Marshal(entry); err == nil {
					logLine = string(jsonData) + "\n"
				}
			} else {
				logLine = fmt.Sprintf("[%s] %s - %s\n",
					entry.Timestamp.Format("2006-01-02 15:04:05"),
					entry.Level,
					entry.Message)
			}
			
			l.fileWriter.WriteString(logLine)
		}
		l.fileWriter.Sync()
	}

	l.entries = l.entries[:0]
}

func (l *PythonPatternsLogger) Debug(message string, context map[string]interface{}) {
	l.log(LogLevelDebug, message, context)
}

func (l *PythonPatternsLogger) Info(message string, context map[string]interface{}) {
	l.log(LogLevelInfo, message, context)
}

func (l *PythonPatternsLogger) Warn(message string, context map[string]interface{}) {
	l.log(LogLevelWarn, message, context)
}

func (l *PythonPatternsLogger) Error(message string, context map[string]interface{}) {
	l.log(LogLevelError, message, context)
}

func (l *PythonPatternsLogger) Fatal(message string, context map[string]interface{}) {
	l.log(LogLevelFatal, message, context)
	l.Close()
	log.Fatal(message)
}

func (l *PythonPatternsLogger) LogServerOutput(output string) {
	l.testLogger.mu.Lock()
	defer l.testLogger.mu.Unlock()
	
	l.testLogger.ServerLogs = append(l.testLogger.ServerLogs, output)
	
	if len(l.testLogger.ServerLogs) > 1000 {
		l.testLogger.ServerLogs = l.testLogger.ServerLogs[100:]
	}

	l.Debug("Server output captured", map[string]interface{}{
		"output": output,
	})
}

func (l *PythonPatternsLogger) LogRequest(method, url string, duration time.Duration, status int, err error) {
	l.testLogger.mu.Lock()
	defer l.testLogger.mu.Unlock()

	requestLog := RequestLog{
		Method:    method,
		URL:       url,
		Duration:  duration,
		Status:    status,
		Timestamp: time.Now(),
	}

	if err != nil {
		requestLog.Error = err.Error()
	}

	l.testLogger.RequestLogs = append(l.testLogger.RequestLogs, requestLog)
	
	if len(l.testLogger.RequestLogs) > 1000 {
		l.testLogger.RequestLogs = l.testLogger.RequestLogs[100:]
	}

	l.Debug("Request logged", map[string]interface{}{
		"method":   method,
		"url":      url,
		"duration": duration,
		"status":   status,
		"error":    err,
	})
}

func (l *PythonPatternsLogger) LogError(errorType, message string, context map[string]interface{}, recoverable bool, retryCount int) {
	l.testLogger.mu.Lock()
	defer l.testLogger.mu.Unlock()

	errorLog := ErrorLog{
		Type:        errorType,
		Message:     message,
		Context:     context,
		Timestamp:   time.Now(),
		Recoverable: recoverable,
		RetryCount:  retryCount,
	}

	l.testLogger.ErrorLogs = append(l.testLogger.ErrorLogs, errorLog)
	
	if len(l.testLogger.ErrorLogs) > 500 {
		l.testLogger.ErrorLogs = l.testLogger.ErrorLogs[50:]
	}

	l.Error("Error logged", map[string]interface{}{
		"error_type":  errorType,
		"message":     message,
		"context":     context,
		"recoverable": recoverable,
		"retry_count": retryCount,
	})
}

func (l *PythonPatternsLogger) LogMetrics(metrics *PythonPatternsMetrics) {
	l.testLogger.mu.Lock()
	defer l.testLogger.mu.Unlock()

	snapshot := MetricsSnapshot{
		TotalRequests:  metrics.TotalRequests,
		SuccessfulReqs: metrics.SuccessfulRequests,
		FailedRequests: metrics.FailedRequests,
		ThroughputRPS:  metrics.ThroughputRPS,
		MemoryUsageMB:  metrics.MemoryUsage / (1024 * 1024),
		Timestamp:      time.Now(),
	}

	if len(metrics.ResponseTimes) > 0 {
		var total time.Duration
		for _, duration := range metrics.ResponseTimes {
			total += duration
		}
		snapshot.AverageLatency = total / time.Duration(len(metrics.ResponseTimes))
	}

	l.testLogger.MetricsLogs = append(l.testLogger.MetricsLogs, snapshot)
	
	if len(l.testLogger.MetricsLogs) > 100 {
		l.testLogger.MetricsLogs = l.testLogger.MetricsLogs[10:]
	}

	l.Info("Metrics logged", map[string]interface{}{
		"total_requests":   snapshot.TotalRequests,
		"successful_reqs":  snapshot.SuccessfulReqs,
		"failed_requests":  snapshot.FailedRequests,
		"throughput_rps":   snapshot.ThroughputRPS,
		"memory_usage_mb":  snapshot.MemoryUsageMB,
		"average_latency":  snapshot.AverageLatency,
	})
}

func (l *PythonPatternsLogger) GetTestLogger() *TestLogger {
	l.testLogger.mu.RLock()
	defer l.testLogger.mu.RUnlock()

	return &TestLogger{
		ServerLogs:  append([]string(nil), l.testLogger.ServerLogs...),
		RequestLogs: append([]RequestLog(nil), l.testLogger.RequestLogs...),
		ErrorLogs:   append([]ErrorLog(nil), l.testLogger.ErrorLogs...),
		MetricsLogs: append([]MetricsSnapshot(nil), l.testLogger.MetricsLogs...),
	}
}

func (l *PythonPatternsLogger) GetRecentEntries(limit int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.entries) {
		limit = len(l.entries)
	}

	entries := make([]LogEntry, limit)
	startIndex := len(l.entries) - limit
	copy(entries, l.entries[startIndex:])

	return entries
}

func (l *PythonPatternsLogger) ClearLogs() {
	l.mu.Lock()
	l.entries = l.entries[:0]
	l.mu.Unlock()

	l.testLogger.mu.Lock()
	l.testLogger.ServerLogs = l.testLogger.ServerLogs[:0]
	l.testLogger.RequestLogs = l.testLogger.RequestLogs[:0]
	l.testLogger.ErrorLogs = l.testLogger.ErrorLogs[:0]
	l.testLogger.MetricsLogs = l.testLogger.MetricsLogs[:0]
	l.testLogger.mu.Unlock()

	l.Info("Logs cleared", nil)
}

func (l *PythonPatternsLogger) SetTestInfo(testName, testPhase, testID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.config.TestInfo = TestInfo{
		TestName:  testName,
		TestPhase: testPhase,
		TestID:    testID,
	}

	l.Info("Test info updated", map[string]interface{}{
		"test_name":  testName,
		"test_phase": testPhase,
		"test_id":    testID,
	})
}

func (l *PythonPatternsLogger) GetLogStats() map[string]interface{} {
	l.mu.RLock()
	entryCount := len(l.entries)
	l.mu.RUnlock()

	l.testLogger.mu.RLock()
	serverLogCount := len(l.testLogger.ServerLogs)
	requestLogCount := len(l.testLogger.RequestLogs)
	errorLogCount := len(l.testLogger.ErrorLogs)
	metricsLogCount := len(l.testLogger.MetricsLogs)
	l.testLogger.mu.RUnlock()

	return map[string]interface{}{
		"total_entries":      entryCount,
		"server_logs":        serverLogCount,
		"request_logs":       requestLogCount,
		"error_logs":         errorLogCount,
		"metrics_logs":       metricsLogCount,
		"config":             l.config,
		"file_writer_active": l.fileWriter != nil,
	}
}

func (l *PythonPatternsLogger) Close() {
	if !l.running {
		return
	}

	l.running = false
	close(l.stopChan)

	l.flush()

	if l.fileWriter != nil {
		l.fileWriter.Close()
		l.fileWriter = nil
	}

	l.Info("Logger closed", nil)
}