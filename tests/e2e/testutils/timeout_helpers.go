package testutils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// TimeoutConfig provides a convenient way to configure timeouts for common test scenarios
type TimeoutConfig struct {
	Language    Language
	Performance SystemPerformance
	profile     *TimeoutProfile
	tm          *TimeoutManager
}

// NewTimeoutConfig creates a new timeout configuration for the given language and performance level
func NewTimeoutConfig(language Language, performance SystemPerformance) *TimeoutConfig {
	tm := GetTimeoutManager()
	profile := tm.GetProfile(language, performance)
	
	return &TimeoutConfig{
		Language:    language,
		Performance: performance,
		profile:     profile,
		tm:          tm,
	}
}

// NewAutoTimeoutConfig creates a timeout configuration with auto-detected system performance
func NewAutoTimeoutConfig(language Language) *TimeoutConfig {
	performance := DetectSystemPerformance()
	return NewTimeoutConfig(language, performance)
}

// Context creation methods
func (tc *TimeoutConfig) RequestContext(parent context.Context) (context.Context, context.CancelFunc) {
	return tc.tm.CreateRequestContext(parent, tc.profile)
}

func (tc *TimeoutConfig) ServerStartupContext(parent context.Context) (context.Context, context.CancelFunc) {
	return tc.tm.CreateServerStartupContext(parent, tc.profile)
}

func (tc *TimeoutConfig) HealthCheckContext(parent context.Context) (context.Context, context.CancelFunc) {
	return tc.tm.CreateHealthCheckContext(parent, tc.profile)
}

func (tc *TimeoutConfig) TestContext(parent context.Context) (context.Context, context.CancelFunc) {
	return tc.tm.CreateTestContext(parent, tc.profile)
}

func (tc *TimeoutConfig) SuiteContext(parent context.Context) (context.Context, context.CancelFunc) {
	return tc.tm.CreateSuiteContext(parent, tc.profile)
}

// Timeout value getters for direct use
func (tc *TimeoutConfig) RequestTimeout() time.Duration {
	return tc.profile.RequestTimeout
}

func (tc *TimeoutConfig) ServerStartupTimeout() time.Duration {
	return tc.profile.ServerStartup
}

func (tc *TimeoutConfig) ServerStartupFastFailTimeout() time.Duration {
	return tc.profile.ServerStartupFastFail
}

func (tc *TimeoutConfig) HealthCheckTimeout() time.Duration {
	return tc.profile.HealthCheck
}

func (tc *TimeoutConfig) TestMethodTimeout() time.Duration {
	return tc.profile.TestMethodTimeout
}

func (tc *TimeoutConfig) SuiteTimeout() time.Duration {
	return tc.profile.SuiteTimeout
}

func (tc *TimeoutConfig) HTTPTimeout() time.Duration {
	return tc.profile.HTTPTimeout
}

func (tc *TimeoutConfig) HTTPConnectionTimeout() time.Duration {
	return tc.profile.HTTPConnectionTimeout
}

func (tc *TimeoutConfig) HTTPKeepAlive() time.Duration {
	return tc.profile.HTTPKeepAlive
}

func (tc *TimeoutConfig) CloneTimeout() time.Duration {
	return tc.profile.CloneTimeout
}

func (tc *TimeoutConfig) LSPInitTimeout() time.Duration {
	return tc.profile.LSPInitTimeout
}

func (tc *TimeoutConfig) WorkspaceLoadTimeout() time.Duration {
	return tc.profile.WorkspaceLoadTimeout
}

func (tc *TimeoutConfig) TestSetupTimeout() time.Duration {
	return tc.profile.TestSetupTimeout
}

func (tc *TimeoutConfig) TestTeardownTimeout() time.Duration {
	return tc.profile.TestTeardownTimeout
}

// HTTP client creation with proper timeout configuration
func (tc *TimeoutConfig) CreateHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   tc.profile.HTTPConnectionTimeout,
		KeepAlive: tc.profile.HTTPKeepAlive,
	}
	
	return &http.Client{
		Timeout: tc.profile.HTTPTimeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			IdleConnTimeout:       tc.profile.HTTPKeepAlive,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

// Test suite helpers
type TestSuiteTimeouts struct {
	config *TimeoutConfig
}

func NewTestSuiteTimeouts(language Language) *TestSuiteTimeouts {
	return &TestSuiteTimeouts{
		config: NewAutoTimeoutConfig(language),
	}
}

func NewTestSuiteTimeoutsWithPerformance(language Language, performance SystemPerformance) *TestSuiteTimeouts {
	return &TestSuiteTimeouts{
		config: NewTimeoutConfig(language, performance),
	}
}

func (tst *TestSuiteTimeouts) Config() *TimeoutConfig {
	return tst.config
}

func (tst *TestSuiteTimeouts) Language() Language {
	return tst.config.Language
}

func (tst *TestSuiteTimeouts) Performance() SystemPerformance {
	return tst.config.Performance
}

// Polling configuration helpers using imported types
func (tc *TimeoutConfig) QuickPollingConfig() PollingConfig {
	return PollingConfig{
		Timeout:     tc.profile.RequestTimeout,
		Interval:    50 * time.Millisecond,
		MaxInterval: 500 * time.Millisecond,
		BackoffFactor: 1.2,
		InitialBackoff: false,
		LogProgress: false,
	}
}

func (tc *TimeoutConfig) StandardPollingConfig() PollingConfig {
	return PollingConfig{
		Timeout:     tc.profile.TestMethodTimeout,
		Interval:    100 * time.Millisecond,
		MaxInterval: 2 * time.Second,
		BackoffFactor: 1.5,
		InitialBackoff: false,
		LogProgress: true,
	}
}

func (tc *TimeoutConfig) SlowPollingConfig() PollingConfig {
	return PollingConfig{
		Timeout:     tc.profile.SuiteTimeout,
		Interval:    500 * time.Millisecond,
		MaxInterval: 5 * time.Second,
		BackoffFactor: 1.8,
		InitialBackoff: false,
		LogProgress: true,
	}
}

func (tc *TimeoutConfig) ServerStartupPollingConfig() FastPollingConfig {
	return FastPollingConfig{
		InitialInterval:   50 * time.Millisecond,
		MaxInterval:       1 * time.Second,
		BackoffFactor:     1.5,
		FastFailTimeout:   tc.profile.ServerStartupFastFail,
		StartupTimeout:    tc.profile.ServerStartup,
		LogProgress:       false,
		EnableFastFail:    true,
		AdaptivePolling:   true,
		MinSystemInterval: 30 * time.Millisecond,
	}
}

// Repository configuration helper
type RepositoryConfig struct {
	CloneTimeout time.Duration
}

func (tc *TimeoutConfig) RepositoryConfig() *RepositoryConfig {
	return &RepositoryConfig{
		CloneTimeout: tc.profile.CloneTimeout,
	}
}

// Language-specific configuration helpers
func GoTimeoutConfig() *TimeoutConfig {
	return NewAutoTimeoutConfig(LanguageGo)
}

func PythonTimeoutConfig() *TimeoutConfig {
	return NewAutoTimeoutConfig(LanguagePython)
}

func JavaTimeoutConfig() *TimeoutConfig {
	return NewAutoTimeoutConfig(LanguageJava)
}

func TypeScriptTimeoutConfig() *TimeoutConfig {
	return NewAutoTimeoutConfig(LanguageTypeScript)
}

func JavaScriptTimeoutConfig() *TimeoutConfig {
	return NewAutoTimeoutConfig(LanguageJavaScript)
}

// Cleanup helper
func CleanupAllTimeouts() {
	GetTimeoutManager().CancelAllContexts()
}

// Timeout debugging helper
func (tc *TimeoutConfig) LogTimeoutInfo(operation string) {
	fmt.Printf("[TimeoutConfig] %s-%s operation: %s\n", tc.Language, tc.Performance, operation)
	fmt.Printf("  Request: %v, Operation: %v, Test: %v, Suite: %v\n",
		tc.profile.RequestTimeout,
		tc.profile.ServerStartup,
		tc.profile.TestMethodTimeout,
		tc.profile.SuiteTimeout)
}

// Language timeout comparison (for debugging timeout conflicts)
func CompareLanguageTimeouts() {
	languages := []Language{LanguageGo, LanguagePython, LanguageJava, LanguageTypeScript}
	performances := []SystemPerformance{SystemFast, SystemNormal, SystemSlow}
	
	fmt.Println("=== Language Timeout Comparison ===")
	for _, lang := range languages {
		fmt.Printf("\n%s Language:\n", lang)
		for _, perf := range performances {
			config := NewTimeoutConfig(lang, perf)
			fmt.Printf("  %s: Request=%v, Startup=%v, Test=%v, Suite=%v\n",
				perf,
				config.RequestTimeout(),
				config.ServerStartupTimeout(),
				config.TestMethodTimeout(),
				config.SuiteTimeout())
		}
	}
}