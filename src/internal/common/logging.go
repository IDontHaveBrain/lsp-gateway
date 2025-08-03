package common

import (
	"fmt"
	"os"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
	LogFatal
)

var logLevelNames = map[LogLevel]string{
	LogDebug: "DEBUG",
	LogInfo:  "INFO",
	LogWarn:  "WARN",
	LogError: "ERROR",
	LogFatal: "FATAL",
}

// SafeLogger provides STDIO-safe logging that only writes to stderr
type SafeLogger struct {
	prefix string
	level  LogLevel
}

// NewSafeLogger creates a new safe logger with the given prefix
func NewSafeLogger(prefix string) *SafeLogger {
	return &SafeLogger{
		prefix: prefix,
		level:  LogInfo, // Default to INFO level
	}
}

// SetLevel sets the minimum log level
func (l *SafeLogger) SetLevel(level LogLevel) {
	l.level = level
}

// log writes a message to stderr with timestamp and level
func (l *SafeLogger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006/01/02 15:04:05")
	levelName := logLevelNames[level]
	message := fmt.Sprintf(format, args...)

	fmt.Fprintf(os.Stderr, "%s [%s] %s: %s\n", timestamp, levelName, l.prefix, message)
}

// Debug logs a debug message
func (l *SafeLogger) Debug(format string, args ...interface{}) {
	l.log(LogDebug, format, args...)
}

// Info logs an info message
func (l *SafeLogger) Info(format string, args ...interface{}) {
	l.log(LogInfo, format, args...)
}

// Warn logs a warning message
func (l *SafeLogger) Warn(format string, args ...interface{}) {
	l.log(LogWarn, format, args...)
}

// Error logs an error message
func (l *SafeLogger) Error(format string, args ...interface{}) {
	l.log(LogError, format, args...)
}

// Fatal logs a fatal message and exits
func (l *SafeLogger) Fatal(format string, args ...interface{}) {
	l.log(LogFatal, format, args...)
	os.Exit(1)
}

// Global logger instances for convenience
var (
	LSPLogger     = NewSafeLogger("LSP")
	GatewayLogger = NewSafeLogger("Gateway")
	CLILogger     = NewSafeLogger("CLI")
)

// Package-level convenience functions
func SafeLog(format string, args ...interface{}) {
	logger := NewSafeLogger("SAFE")
	logger.Info(format, args...)
}

func SafeError(format string, args ...interface{}) {
	logger := NewSafeLogger("SAFE")
	logger.Error(format, args...)
}

func SafeDebug(format string, args ...interface{}) {
	logger := NewSafeLogger("SAFE")
	logger.Debug(format, args...)
}
