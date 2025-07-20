package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"lsp-gateway/internal/platform"
	"os"
	"runtime"
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
	LogLevelUser // Special level for user-facing messages
)

var setupLogLevelNames = map[LogLevel]string{
	LogLevelTrace: "TRACE",
	LogLevelDebug: "DEBUG",
	LogLevelInfo:  "INFO",
	LogLevelWarn:  "WARN",
	LogLevelError: "ERROR",
	LogLevelFatal: "FATAL",
	LogLevelUser:  "USER",
}

type SetupLogEntry struct {
	Timestamp     time.Time              `json:"timestamp"`
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	UserMessage   string                 `json:"user_message,omitempty"`
	Component     string                 `json:"component"`
	Operation     string                 `json:"operation,omitempty"`
	Stage         string                 `json:"stage,omitempty"`
	StageProgress *ProgressInfo          `json:"stage_progress,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	ErrorCode     platform.ErrorCode     `json:"error_code,omitempty"`
	ErrorContext  map[string]interface{} `json:"error_context,omitempty"`
	Duration      string                 `json:"duration,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
	Caller        string                 `json:"caller,omitempty"`
	Metrics       *SetupMetrics          `json:"metrics,omitempty"`
	Suggestions   []string               `json:"suggestions,omitempty"`
	NextSteps     []string               `json:"next_steps,omitempty"`
}

type SetupMetrics struct {
	BytesDownloaded   int64              `json:"bytes_downloaded,omitempty"`
	FilesProcessed    int                `json:"files_processed,omitempty"`
	ServersInstalled  int                `json:"servers_installed,omitempty"`
	RuntimesChecked   int                `json:"runtimes_checked,omitempty"`
	ErrorsEncountered int                `json:"errors_encountered,omitempty"`
	WarningsIssued    int                `json:"warnings_issued,omitempty"`
	CommandsExecuted  int                `json:"commands_executed,omitempty"`
	RetryAttempts     int                `json:"retry_attempts,omitempty"`
	CacheHits         int                `json:"cache_hits,omitempty"`
	CacheMisses       int                `json:"cache_misses,omitempty"`
	CustomCounters    map[string]int64   `json:"custom_counters,omitempty"`
	CustomGauges      map[string]float64 `json:"custom_gauges,omitempty"`
}

type SetupLoggerConfig struct {
	Level                  LogLevel
	Component              string
	EnableJSON             bool
	EnableUserMessages     bool
	EnableProgressTracking bool
	EnableMetrics          bool
	EnableStackTrace       bool
	EnableCaller           bool
	Output                 io.Writer
	UserOutput             io.Writer // Separate output for user messages
	TimestampFormat        string
	SessionID              string
	VerboseMode            bool
	QuietMode              bool
	MaxStackTraceDepth     int
}

type SetupLogger struct {
	config       *SetupLoggerConfig
	output       io.Writer
	userOutput   io.Writer
	mu           sync.RWMutex
	fields       map[string]interface{}
	baseLogger   *log.Logger
	userLogger   *log.Logger
	metrics      *SetupMetrics
	metricsMu    sync.RWMutex
	startTime    time.Time
	operations   map[string]time.Time // Track operation start times
	operationsMu sync.RWMutex
}

func NewSetupLogger(config *SetupLoggerConfig) *SetupLogger {
	if config == nil {
		config = &SetupLoggerConfig{
			Level:                  LogLevelInfo,
			Component:              "setup",
			EnableJSON:             false, // Human-readable by default
			EnableUserMessages:     true,
			EnableProgressTracking: true,
			EnableMetrics:          true,
			EnableStackTrace:       false,
			EnableCaller:           true,
			Output:                 os.Stderr,
			UserOutput:             os.Stdout,
			TimestampFormat:        "15:04:05",
			SessionID:              generateSessionID(),
			VerboseMode:            false,
			QuietMode:              false,
			MaxStackTraceDepth:     10,
		}
	}

	if config.Output == nil {
		config.Output = os.Stderr
	}
	if config.UserOutput == nil {
		config.UserOutput = os.Stdout
	}

	logger := &SetupLogger{
		config:     config,
		output:     config.Output,
		userOutput: config.UserOutput,
		fields:     make(map[string]interface{}),
		baseLogger: log.New(config.Output, "", 0),
		userLogger: log.New(config.UserOutput, "", 0),
		metrics: &SetupMetrics{
			CustomCounters: make(map[string]int64),
			CustomGauges:   make(map[string]float64),
		},
		startTime:  time.Now(),
		operations: make(map[string]time.Time),
	}

	return logger
}

func (l *SetupLogger) WithField(key string, value interface{}) *SetupLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := l.clone()
	newLogger.fields[key] = value
	return newLogger
}

func (l *SetupLogger) WithFields(fields map[string]interface{}) *SetupLogger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := l.clone()
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

func (l *SetupLogger) WithError(err error) *SetupLogger {
	if err == nil {
		return l
	}

	fields := map[string]interface{}{
		"error": err.Error(),
	}

	if pe, ok := err.(*platform.PlatformError); ok {
		fields["error_code"] = pe.Code
		fields["error_type"] = pe.Type
		fields["error_severity"] = pe.Severity
		if pe.SystemInfo != nil {
			fields["system_info"] = pe.SystemInfo
		}
	}

	return l.WithFields(fields)
}

func (l *SetupLogger) WithOperation(operation string) *SetupLogger {
	l.operationsMu.Lock()
	l.operations[operation] = time.Now()
	l.operationsMu.Unlock()

	return l.WithField(OPERATION_WORD, operation)
}

func (l *SetupLogger) WithStage(stage string) *SetupLogger {
	return l.WithField("stage", stage)
}

func (l *SetupLogger) WithProgress(progress *ProgressInfo) *SetupLogger {
	return l.WithField("stage_progress", progress)
}

func (l *SetupLogger) UserInfo(message string) {
	if l.config.QuietMode {
		return
	}
	l.logUser(LogLevelUser, message, "")
}

func (l *SetupLogger) UserInfof(format string, args ...interface{}) {
	l.UserInfo(fmt.Sprintf(format, args...))
}

func (l *SetupLogger) UserSuccess(message string) {
	if l.config.QuietMode {
		return
	}
	l.logUser(LogLevelUser, message, "✓ "+message)
}

func (l *SetupLogger) UserWarn(message string) {
	l.logUser(LogLevelWarn, message, "⚠ "+message)
}

func (l *SetupLogger) UserError(message string) {
	l.logUser(LogLevelError, message, "✗ "+message)
}

func (l *SetupLogger) UserProgress(operation string, progress *ProgressInfo) {
	if l.config.QuietMode {
		return
	}

	var progressMsg string
	if progress != nil {
		if progress.Total > 0 {
			progressMsg = fmt.Sprintf("%s... %.1f%% (%d/%d)",
				operation, progress.Percentage, progress.Completed, progress.Total)
		} else {
			progressMsg = fmt.Sprintf("%s... %d items", operation, progress.Completed)
		}

		if progress.CurrentItem != "" {
			progressMsg += fmt.Sprintf(" [%s]", progress.CurrentItem)
		}
	} else {
		progressMsg = operation + "..."
	}

	l.WithProgress(progress).logUser(LogLevelUser, operation, progressMsg)
}

func (l *SetupLogger) Debug(message string) {
	l.log(LogLevelDebug, message, "")
}

func (l *SetupLogger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

func (l *SetupLogger) Info(message string) {
	l.log(LogLevelInfo, message, "")
}

func (l *SetupLogger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l *SetupLogger) Warn(message string) {
	l.log(LogLevelWarn, message, "")
	l.updateMetrics(LOG_FIELD_WARNINGS_ISSUED_ALT, 1)
}

func (l *SetupLogger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

func (l *SetupLogger) Error(message string) {
	l.log(LogLevelError, message, "")
	l.updateMetrics(LOG_FIELD_ERRORS_ENCOUNTERED, 1)
}

func (l *SetupLogger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l *SetupLogger) LogOperationStart(operation string, details map[string]interface{}) {
	l.WithOperation(operation).WithFields(details).Info(fmt.Sprintf("Starting %s", operation))
}

func (l *SetupLogger) LogOperationComplete(operation string, success bool) {
	l.operationsMu.RLock()
	startTime, exists := l.operations[operation]
	l.operationsMu.RUnlock()

	var duration time.Duration
	if exists {
		duration = time.Since(startTime)
		l.operationsMu.Lock()
		delete(l.operations, operation)
		l.operationsMu.Unlock()
	}

	level := LogLevelInfo
	message := fmt.Sprintf("Completed %s", operation)
	userMessage := fmt.Sprintf("✓ %s completed", operation)

	if !success {
		level = LogLevelError
		message = fmt.Sprintf("Failed %s", operation)
		userMessage = fmt.Sprintf("✗ %s failed", operation)
	}

	if duration > 0 {
		message += fmt.Sprintf(" in %v", duration)
		userMessage += fmt.Sprintf(" (%v)", duration)
	}

	logger := l.WithField(OPERATION_WORD, operation).WithField("success", success)
	if duration > 0 {
		logger = logger.WithField("duration", duration.String())
	}

	logger.log(level, message, userMessage)
}

func (l *SetupLogger) LogCommand(command string, args []string, success bool, duration time.Duration) {
	fullCommand := fmt.Sprintf("%s %s", command, strings.Join(args, " "))

	level := LogLevelDebug
	if !success {
		level = LogLevelWarn
	}

	l.WithFields(map[string]interface{}{
		"command":  command,
		"args":     args,
		"success":  success,
		"duration": duration.String(),
	}).log(level, fmt.Sprintf("Command executed: %s", fullCommand), "")

	l.updateMetrics(LOG_FIELD_COMMANDS_EXECUTED, 1)
}

func (l *SetupLogger) LogDownload(url string, size int64, progress *ProgressInfo) {
	l.WithFields(map[string]interface{}{
		"url":  url,
		"size": size,
	}).WithProgress(progress).Debug("Download progress")

	if progress != nil {
		l.UserProgress(fmt.Sprintf("Downloading %s", extractFilename(url)), progress)
	}
}

func (l *SetupLogger) LogInstallation(component, version string, stage string, progress *ProgressInfo) {
	l.WithFields(map[string]interface{}{
		"component": component,
		"version":   version,
		"stage":     stage,
	}).WithProgress(progress).Info(fmt.Sprintf("Installing %s %s: %s", component, version, stage))

	if progress != nil {
		l.UserProgress(fmt.Sprintf("Installing %s %s", component, version), progress)
	}
}

func (l *SetupLogger) LogValidation(item string, valid bool, details map[string]interface{}) {
	level := LogLevelInfo
	message := fmt.Sprintf("Validation passed for %s", item)

	if !valid {
		level = LogLevelWarn
		message = fmt.Sprintf("Validation failed for %s", item)
	}

	l.WithFields(details).log(level, message, "")
}

func (l *SetupLogger) LogRecovery(errorType string, action string, success bool) {
	level := LogLevelWarn
	message := fmt.Sprintf("Recovery attempt for %s: %s", errorType, action)

	if success {
		level = LogLevelInfo
		message += " (successful)"
	} else {
		message += " (failed)"
	}

	l.WithFields(map[string]interface{}{
		"error_type":      errorType,
		"recovery_action": action,
		"success":         success,
	}).log(level, message, "")

	l.updateMetrics("retry_attempts", 1)
}

func (l *SetupLogger) UpdateMetrics(updates map[string]interface{}) {
	l.metricsMu.Lock()
	defer l.metricsMu.Unlock()

	for key, value := range updates {
		switch v := value.(type) {
		case int:
			l.metrics.CustomCounters[key] = int64(v)
		case int64:
			l.metrics.CustomCounters[key] = v
		case float64:
			l.metrics.CustomGauges[key] = v
		case float32:
			l.metrics.CustomGauges[key] = float64(v)
		}
	}
}

func (l *SetupLogger) GetMetrics() *SetupMetrics {
	l.metricsMu.RLock()
	defer l.metricsMu.RUnlock()

	metrics := &SetupMetrics{
		BytesDownloaded:   l.metrics.BytesDownloaded,
		FilesProcessed:    l.metrics.FilesProcessed,
		ServersInstalled:  l.metrics.ServersInstalled,
		RuntimesChecked:   l.metrics.RuntimesChecked,
		ErrorsEncountered: l.metrics.ErrorsEncountered,
		WarningsIssued:    l.metrics.WarningsIssued,
		CommandsExecuted:  l.metrics.CommandsExecuted,
		RetryAttempts:     l.metrics.RetryAttempts,
		CacheHits:         l.metrics.CacheHits,
		CacheMisses:       l.metrics.CacheMisses,
		CustomCounters:    make(map[string]int64),
		CustomGauges:      make(map[string]float64),
	}

	for k, v := range l.metrics.CustomCounters {
		metrics.CustomCounters[k] = v
	}
	for k, v := range l.metrics.CustomGauges {
		metrics.CustomGauges[k] = v
	}

	return metrics
}

func (l *SetupLogger) LogSummary() {
	duration := time.Since(l.startTime)
	metrics := l.GetMetrics()

	summary := fmt.Sprintf("Setup session completed in %v", duration)

	l.WithFields(map[string]interface{}{
		"session_duration": duration.String(),
		"metrics":          metrics,
	}).Info(summary)

	if !l.config.QuietMode {
		l.UserInfo(fmt.Sprintf("Setup completed in %v", duration))
		if metrics.ErrorsEncountered > 0 {
			l.UserWarn(fmt.Sprintf("%d errors encountered", metrics.ErrorsEncountered))
		}
		if metrics.WarningsIssued > 0 {
			l.UserInfo(fmt.Sprintf("%d warnings issued", metrics.WarningsIssued))
		}
	}
}

func (l *SetupLogger) log(level LogLevel, message, userMessage string) {
	if level < l.config.Level {
		return
	}

	entry := l.createEntry(level, message, userMessage)
	l.writeEntry(entry, false)
}

func (l *SetupLogger) logUser(level LogLevel, message, userMessage string) {
	if !l.config.EnableUserMessages {
		return
	}

	entry := l.createEntry(level, message, userMessage)
	l.writeEntry(entry, true)
}

func (l *SetupLogger) createEntry(level LogLevel, message, userMessage string) *SetupLogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry := &SetupLogEntry{
		Timestamp:   time.Now(),
		Level:       setupLogLevelNames[level],
		Message:     message,
		UserMessage: userMessage,
		Component:   l.config.Component,
		SessionID:   l.config.SessionID,
	}

	if len(l.fields) > 0 {
		entry.Context = make(map[string]interface{})
		for k, v := range l.fields {
			switch k {
			case "operation":
				if op, ok := v.(string); ok {
					entry.Operation = op
				}
			case "stage":
				if stage, ok := v.(string); ok {
					entry.Stage = stage
				}
			case "stage_progress":
				if progress, ok := v.(*ProgressInfo); ok {
					entry.StageProgress = progress
				}
			case "error_code":
				if code, ok := v.(platform.ErrorCode); ok {
					entry.ErrorCode = code
				}
			case "duration":
				if duration, ok := v.(string); ok {
					entry.Duration = duration
				}
			default:
				entry.Context[k] = v
			}
		}
	}

	if l.config.EnableCaller {
		entry.Caller = l.getCaller()
	}

	if l.config.EnableMetrics {
		entry.Metrics = l.GetMetrics()
	}

	return entry
}

func (l *SetupLogger) writeEntry(entry *SetupLogEntry, userOutput bool) {
	var output string
	var writer io.Writer

	if userOutput {
		writer = l.userOutput
		if entry.UserMessage != "" {
			output = entry.UserMessage + "\n"
		} else {
			output = entry.Message + "\n"
		}
	} else {
		writer = l.output
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
	}

	if _, err := writer.Write([]byte(output)); err != nil {
	}
}

func (l *SetupLogger) formatEntryHuman(entry *SetupLogEntry) string {
	var parts []string

	parts = append(parts, entry.Timestamp.Format(l.config.TimestampFormat))
	parts = append(parts, fmt.Sprintf("[%s]", entry.Level))

	if entry.Component != "" {
		parts = append(parts, entry.Component)
	}

	if entry.Operation != "" {
		parts = append(parts, fmt.Sprintf("(%s)", entry.Operation))
	}

	if entry.Stage != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.Stage))
	}

	parts = append(parts, entry.Message)

	if entry.Duration != "" {
		parts = append(parts, fmt.Sprintf("(%s)", entry.Duration))
	}

	output := strings.Join(parts, " ") + "\n"

	if entry.StageProgress != nil && l.config.EnableProgressTracking {
		progress := entry.StageProgress
		if progress.Total > 0 {
			output += fmt.Sprintf("  Progress: %.1f%% (%d/%d)\n",
				progress.Percentage, progress.Completed, progress.Total)
		}
		if progress.CurrentItem != "" {
			output += fmt.Sprintf("  Current: %s\n", progress.CurrentItem)
		}
	}

	return output
}

func (l *SetupLogger) getCaller() string {
	_, file, line, ok := runtime.Caller(5) // Skip logging framework calls
	if !ok {
		return ""
	}

	parts := strings.Split(file, "/")
	if len(parts) > 0 {
		file = parts[len(parts)-1]
	}

	return fmt.Sprintf("%s:%d", file, line)
}

func (l *SetupLogger) clone() *SetupLogger {
	newLogger := &SetupLogger{
		config:     l.config,
		output:     l.output,
		userOutput: l.userOutput,
		fields:     make(map[string]interface{}),
		baseLogger: l.baseLogger,
		userLogger: l.userLogger,
		metrics:    l.metrics,
		startTime:  l.startTime,
		operations: l.operations,
	}

	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

func (l *SetupLogger) updateMetrics(key string, delta int64) {
	l.metricsMu.Lock()
	defer l.metricsMu.Unlock()

	switch key {
	case LOG_FIELD_ERRORS_ENCOUNTERED:
		l.metrics.ErrorsEncountered += int(delta)
	case LOG_FIELD_WARNINGS_ISSUED_ALT:
		l.metrics.WarningsIssued += int(delta)
	case LOG_FIELD_COMMANDS_EXECUTED:
		l.metrics.CommandsExecuted += int(delta)
	case "retry_attempts":
		l.metrics.RetryAttempts += int(delta)
	default:
		l.metrics.CustomCounters[key] += delta
	}
}
