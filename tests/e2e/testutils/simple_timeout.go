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
		defaultTimeout: 10 * time.Second,
	}
}

func (tm *SimpleTimeoutManager) CreateTimeoutContext(parent context.Context, operation string) (context.Context, context.CancelFunc) {
	timeout := tm.getTimeoutForOperation(operation)
	return context.WithTimeout(parent, timeout)
}

func (tm *SimpleTimeoutManager) getTimeoutForOperation(operation string) time.Duration {
	switch operation {
	case "server_startup":
		return 2 * time.Second
	case "lsp_request":
		return 3 * time.Second
	case "workspace_load":
		return 8 * time.Second
	case "integration_test":
		return 15 * time.Second
	case "repository_clone":
		return 2 * time.Minute
	case "concurrent_test":
		return 2 * time.Minute
	default:
		return tm.defaultTimeout
	}
}

func (tm *SimpleTimeoutManager) CancelAllActiveContexts() {
	// Standard Go contexts handle their own cleanup
}