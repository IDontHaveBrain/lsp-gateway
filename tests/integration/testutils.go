package integration

import (
	"context"
	"fmt"
	"time"
)

func WaitUntil(ctx context.Context, interval, timeout time.Duration, predicate func() bool) error {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if predicate() {
		return nil
	}

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("condition not met within timeout %v", timeout)
		case <-ticker.C:
			if predicate() {
				return nil
			}
		}
	}
}
