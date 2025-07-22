package lsp

import (
	"fmt"
	"log"
	"os"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
)

// SimpleTestLogger is a simple implementation of TestLogger
type SimpleTestLogger struct {
	prefix  string
	verbose bool
	logger  *log.Logger
	fields  map[string]interface{}
}

// NewSimpleTestLogger creates a new simple test logger
func NewSimpleTestLogger(verbose bool) *SimpleTestLogger {
	return &SimpleTestLogger{
		verbose: verbose,
		logger:  log.New(os.Stdout, "", log.LstdFlags),
		fields:  make(map[string]interface{}),
	}
}

// Debug logs debug messages
func (l *SimpleTestLogger) Debug(format string, args ...interface{}) {
	if l.verbose {
		l.log("DEBUG", format, args...)
	}
}

// Info logs info messages
func (l *SimpleTestLogger) Info(format string, args ...interface{}) {
	l.log("INFO", format, args...)
}

// Warn logs warning messages
func (l *SimpleTestLogger) Warn(format string, args ...interface{}) {
	l.log("WARN", format, args...)
}

// Error logs error messages
func (l *SimpleTestLogger) Error(format string, args ...interface{}) {
	l.log("ERROR", format, args...)
}

// WithFields creates a new logger with additional fields
func (l *SimpleTestLogger) WithFields(fields map[string]interface{}) TestLogger {
	newLogger := &SimpleTestLogger{
		prefix:  l.prefix,
		verbose: l.verbose,
		logger:  l.logger,
		fields:  make(map[string]interface{}),
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

// WithTestCase creates a new logger with test case context
func (l *SimpleTestLogger) WithTestCase(testCase *cases.TestCase) TestLogger {
	return l.WithFields(map[string]interface{}{
		"test_id":     testCase.ID,
		"test_name":   testCase.Name,
		"test_method": testCase.Method,
		"language":    testCase.Language,
	})
}

// WithTestSuite creates a new logger with test suite context
func (l *SimpleTestLogger) WithTestSuite(testSuite *cases.TestSuite) TestLogger {
	return l.WithFields(map[string]interface{}{
		"suite_name": testSuite.Name,
		"language":   testSuite.Repository.Language,
		"workspace":  testSuite.WorkspaceDir,
	})
}

// log performs the actual logging with fields
func (l *SimpleTestLogger) log(level string, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	// Add fields to the message if present
	if len(l.fields) > 0 {
		fieldStr := ""
		for k, v := range l.fields {
			if fieldStr != "" {
				fieldStr += " "
			}
			fieldStr += fmt.Sprintf("%s=%v", k, v)
		}
		message = fmt.Sprintf("%s [%s]", message, fieldStr)
	}

	// Add prefix if present
	if l.prefix != "" {
		message = fmt.Sprintf("[%s] %s", l.prefix, message)
	}

	l.logger.Printf("[%s] %s", level, message)
}

// TestLogger interface (copied here for reference)
type TestLogger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})

	WithFields(fields map[string]interface{}) TestLogger
	WithTestCase(testCase *cases.TestCase) TestLogger
	WithTestSuite(testSuite *cases.TestSuite) TestLogger
}

// NullLogger is a logger that does nothing (for testing or when logging is disabled)
type NullLogger struct{}

// NewNullLogger creates a new null logger
func NewNullLogger() *NullLogger {
	return &NullLogger{}
}

// Debug does nothing
func (l *NullLogger) Debug(format string, args ...interface{}) {}

// Info does nothing
func (l *NullLogger) Info(format string, args ...interface{}) {}

// Warn does nothing
func (l *NullLogger) Warn(format string, args ...interface{}) {}

// Error does nothing
func (l *NullLogger) Error(format string, args ...interface{}) {}

// WithFields returns itself (no-op)
func (l *NullLogger) WithFields(fields map[string]interface{}) TestLogger {
	return l
}

// WithTestCase returns itself (no-op)
func (l *NullLogger) WithTestCase(testCase *cases.TestCase) TestLogger {
	return l
}

// WithTestSuite returns itself (no-op)
func (l *NullLogger) WithTestSuite(testSuite *cases.TestSuite) TestLogger {
	return l
}

// TimedLogger wraps another logger and adds timing information
type TimedLogger struct {
	wrapped TestLogger
	start   time.Time
}

// NewTimedLogger creates a new timed logger
func NewTimedLogger(wrapped TestLogger) *TimedLogger {
	return &TimedLogger{
		wrapped: wrapped,
		start:   time.Now(),
	}
}

// Debug logs debug messages with elapsed time
func (l *TimedLogger) Debug(format string, args ...interface{}) {
	elapsed := time.Since(l.start)
	l.wrapped.Debug("[%v] "+format, append([]interface{}{elapsed}, args...)...)
}

// Info logs info messages with elapsed time
func (l *TimedLogger) Info(format string, args ...interface{}) {
	elapsed := time.Since(l.start)
	l.wrapped.Info("[%v] "+format, append([]interface{}{elapsed}, args...)...)
}

// Warn logs warning messages with elapsed time
func (l *TimedLogger) Warn(format string, args ...interface{}) {
	elapsed := time.Since(l.start)
	l.wrapped.Warn("[%v] "+format, append([]interface{}{elapsed}, args...)...)
}

// Error logs error messages with elapsed time
func (l *TimedLogger) Error(format string, args ...interface{}) {
	elapsed := time.Since(l.start)
	l.wrapped.Error("[%v] "+format, append([]interface{}{elapsed}, args...)...)
}

// WithFields creates a new logger with additional fields
func (l *TimedLogger) WithFields(fields map[string]interface{}) TestLogger {
	return &TimedLogger{
		wrapped: l.wrapped.WithFields(fields),
		start:   l.start,
	}
}

// WithTestCase creates a new logger with test case context
func (l *TimedLogger) WithTestCase(testCase *cases.TestCase) TestLogger {
	return &TimedLogger{
		wrapped: l.wrapped.WithTestCase(testCase),
		start:   l.start,
	}
}

// WithTestSuite creates a new logger with test suite context
func (l *TimedLogger) WithTestSuite(testSuite *cases.TestSuite) TestLogger {
	return &TimedLogger{
		wrapped: l.wrapped.WithTestSuite(testSuite),
		start:   l.start,
	}
}
