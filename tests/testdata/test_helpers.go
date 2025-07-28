package testdata

import (
	"context"
	"time"
)

type TestContext struct {
	Context  context.Context
	Cancel   context.CancelFunc
	Timeout  time.Duration
	Metadata map[string]interface{}
}

func NewTestContext(timeout time.Duration) *TestContext {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return &TestContext{
		Context:  ctx,
		Cancel:   cancel,
		Timeout:  timeout,
		Metadata: make(map[string]interface{}),
	}
}

func (tc *TestContext) Cleanup() {
	if tc.Cancel != nil {
		tc.Cancel()
	}
}


type TestRunner struct {
	context  *TestContext
	failures []string
	warnings []string
}

func NewTestRunner(timeout time.Duration) *TestRunner {
	return &TestRunner{
		context:  NewTestContext(timeout),
		failures: make([]string, 0),
		warnings: make([]string, 0),
	}
}

func (tr *TestRunner) Context() context.Context {
	return tr.context.Context
}

func (tr *TestRunner) AddFailure(message string) {
	tr.failures = append(tr.failures, message)
}

func (tr *TestRunner) AddWarning(message string) {
	tr.warnings = append(tr.warnings, message)
}

func (tr *TestRunner) HasFailures() bool {
	return len(tr.failures) > 0
}

func (tr *TestRunner) GetFailures() []string {
	return tr.failures
}

func (tr *TestRunner) GetWarnings() []string {
	return tr.warnings
}

func (tr *TestRunner) Cleanup() {
	tr.context.Cleanup()
}
