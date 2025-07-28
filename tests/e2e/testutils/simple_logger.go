package testutils

import (
	"log"
	"os"
)

type SimpleLogger struct {
	logger *log.Logger
}

func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{
		logger: log.New(os.Stdout, "[PYTHON-E2E] ", log.LstdFlags|log.Lshortfile),
	}
}

func (l *SimpleLogger) Debug(message string, context map[string]interface{}) {
	l.logger.Printf("[DEBUG] %s %+v", message, context)
}

func (l *SimpleLogger) Info(message string, context map[string]interface{}) {
	l.logger.Printf("[INFO] %s %+v", message, context)
}

func (l *SimpleLogger) Warn(message string, context map[string]interface{}) {
	l.logger.Printf("[WARN] %s %+v", message, context)
}

func (l *SimpleLogger) Error(message string, context map[string]interface{}) {
	l.logger.Printf("[ERROR] %s %+v", message, context)
}

func (l *SimpleLogger) Fatal(message string, context map[string]interface{}) {
	l.logger.Fatalf("[FATAL] %s %+v", message, context)
}

func (l *SimpleLogger) SetTestInfo(testName, testPhase, testID string) {
	l.logger.SetPrefix("[PYTHON-E2E-" + testName + "] ")
}

func (l *SimpleLogger) Close() {}