package testutils

import (
	"context"
	"time"
)

type SimpleTimeoutManager struct {
	defaultTimeout time.Duration
}

func NewSimpleTimeoutManager() *SimpleTimeoutManager {
	return &SimpleTimeoutManager{
		defaultTimeout: 30 * time.Second,
	}
}

func (tm *SimpleTimeoutManager) CreateTimeoutContext(parent context.Context, operation string) (context.Context, context.CancelFunc) {
	timeout := tm.getTimeoutForOperation(operation)
	return context.WithTimeout(parent, timeout)
}

func (tm *SimpleTimeoutManager) getTimeoutForOperation(operation string) time.Duration {
	switch operation {
	case "server_startup":
		return 30 * time.Second
	case "lsp_request":
		return 5 * time.Second
	case "workspace_load":
		return 20 * time.Second
	case "integration_test":
		return 3 * time.Minute
	case "repository_clone":
		return 5 * time.Minute
	case "concurrent_test":
		return 5 * time.Minute
	default:
		return tm.defaultTimeout
	}
}

func (tm *SimpleTimeoutManager) CancelAllActiveContexts() {
	// Standard Go contexts handle their own cleanup
}