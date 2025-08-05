package common

import (
	"fmt"
	"os"
	"strings"
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

// Global logger instances for convenience
var (
	LSPLogger     = NewSafeLogger("LSP")
	GatewayLogger = NewSafeLogger("Gateway")
	CLILogger     = NewSafeLogger("CLI")
)

// SanitizeErrorForLogging removes stack traces and verbose details from error messages
func SanitizeErrorForLogging(err interface{}) string {
	if err == nil {
		return ""
	}

	errStr := fmt.Sprintf("%v", err)

	// Check if this is a TypeScript server error with stack trace
	if strings.Contains(errStr, "TypeScript Server Error") {
		lines := strings.Split(errStr, "\n")
		if len(lines) > 0 {
			// Return only the first few lines, skip the stack trace
			var cleanLines []string
			for i, line := range lines {
				// Skip stack trace lines (lines starting with spaces and "at ")
				if i > 2 && strings.HasPrefix(strings.TrimSpace(line), "at ") {
					break
				}
				cleanLines = append(cleanLines, line)
				// Limit to first 3 lines to keep it concise
				if i >= 2 {
					break
				}
			}
			return strings.Join(cleanLines, " | ")
		}
	}

	// For other errors, limit length and remove excessive details
	if len(errStr) > 200 {
		return errStr[:200] + "..."
	}

	return errStr
}
