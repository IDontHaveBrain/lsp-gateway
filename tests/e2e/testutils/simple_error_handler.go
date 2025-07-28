package testutils

import (
	"context"
	"strings"
	"time"
)

type SimpleErrorHandler struct {
	maxRetries int
	retryDelay time.Duration
}

func NewSimpleErrorHandler() *SimpleErrorHandler {
	return &SimpleErrorHandler{
		maxRetries: 3,
		retryDelay: 2 * time.Second,
	}
}

func (eh *SimpleErrorHandler) HandleError(ctx context.Context, err error, operation string) error {
	if err == nil {
		return nil
	}

	if !eh.isRetryableError(err) {
		return err
	}

	// Simple retry logic with exponential backoff
	delay := eh.retryDelay
	for attempt := 0; attempt < eh.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Simple retry delay
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}

	return err
}

func (eh *SimpleErrorHandler) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"temporary failure",
		"network",
		"context deadline exceeded",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

func (eh *SimpleErrorHandler) IsCircuitBreakerOpen(operation string) bool {
	// No circuit breaker in simple implementation
	return false
}