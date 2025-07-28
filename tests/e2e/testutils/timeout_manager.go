package testutils

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageJava       Language = "java"
	LanguageTypeScript Language = "typescript"
	LanguageJavaScript Language = "javascript"
	LanguageGeneric    Language = "generic"
)

type SystemPerformance string

const (
	SystemFast   SystemPerformance = "fast"
	SystemNormal SystemPerformance = "normal"
	SystemSlow   SystemPerformance = "slow"
)

type TimeoutLevel string

const (
	LevelRequest   TimeoutLevel = "request"
	LevelOperation TimeoutLevel = "operation"
	LevelTest      TimeoutLevel = "test"
	LevelSuite     TimeoutLevel = "suite"
)

type TimeoutProfile struct {
	Language           Language
	SystemPerformance  SystemPerformance
	
	// Request-level timeouts (individual LSP requests)
	RequestTimeout          time.Duration
	RequestRetryTimeout     time.Duration
	
	// Operation-level timeouts (server startup, health checks, etc.)
	ServerStartup          time.Duration
	ServerStartupFastFail  time.Duration
	HealthCheck            time.Duration
	
	// Test-level timeouts (individual test methods)
	TestMethodTimeout      time.Duration
	TestSetupTimeout       time.Duration
	TestTeardownTimeout    time.Duration
	
	// Suite-level timeouts (entire test suite)
	SuiteTimeout           time.Duration
	
	// HTTP client timeouts
	HTTPTimeout            time.Duration
	HTTPConnectionTimeout  time.Duration
	HTTPKeepAlive          time.Duration
	
	// Repository operations
	CloneTimeout           time.Duration
	
	// Language server initialization
	LSPInitTimeout         time.Duration
	WorkspaceLoadTimeout   time.Duration
}

type TimeoutManager struct {
	profiles map[string]*TimeoutProfile
	mutex    sync.RWMutex
	
	// Active contexts for cleanup
	activeContexts map[string]context.CancelFunc
	contextMutex   sync.Mutex
}

func NewTimeoutManager() *TimeoutManager {
	tm := &TimeoutManager{
		profiles:       make(map[string]*TimeoutProfile),
		activeContexts: make(map[string]context.CancelFunc),
	}
	tm.initializeDefaultProfiles()
	return tm
}

func (tm *TimeoutManager) initializeDefaultProfiles() {
	// Go language profiles (fast compilation, quick startup)
	tm.profiles["go-fast"] = &TimeoutProfile{
		Language:               LanguageGo,
		SystemPerformance:      SystemFast,
		RequestTimeout:         3 * time.Second,
		RequestRetryTimeout:    5 * time.Second,
		ServerStartup:          10 * time.Second,
		ServerStartupFastFail:  2 * time.Second,
		HealthCheck:           2 * time.Second,
		TestMethodTimeout:     15 * time.Second,
		TestSetupTimeout:      5 * time.Second,
		TestTeardownTimeout:   3 * time.Second,
		SuiteTimeout:          2 * time.Minute,
		HTTPTimeout:           5 * time.Second,
		HTTPConnectionTimeout: 2 * time.Second,
		HTTPKeepAlive:         20 * time.Second,
		CloneTimeout:          120 * time.Second,
		LSPInitTimeout:        8 * time.Second,
		WorkspaceLoadTimeout:  10 * time.Second,
	}

	tm.profiles["go-normal"] = &TimeoutProfile{
		Language:               LanguageGo,
		SystemPerformance:      SystemNormal,
		RequestTimeout:         5 * time.Second,
		RequestRetryTimeout:    8 * time.Second,
		ServerStartup:          15 * time.Second,
		ServerStartupFastFail:  3 * time.Second,
		HealthCheck:           3 * time.Second,
		TestMethodTimeout:     25 * time.Second,
		TestSetupTimeout:      8 * time.Second,
		TestTeardownTimeout:   5 * time.Second,
		SuiteTimeout:          5 * time.Minute,
		HTTPTimeout:           8 * time.Second,
		HTTPConnectionTimeout: 3 * time.Second,
		HTTPKeepAlive:         30 * time.Second,
		CloneTimeout:          180 * time.Second,
		LSPInitTimeout:        12 * time.Second,
		WorkspaceLoadTimeout:  15 * time.Second,
	}

	tm.profiles["go-slow"] = &TimeoutProfile{
		Language:               LanguageGo,
		SystemPerformance:      SystemSlow,
		RequestTimeout:         8 * time.Second,
		RequestRetryTimeout:    12 * time.Second,
		ServerStartup:          25 * time.Second,
		ServerStartupFastFail:  5 * time.Second,
		HealthCheck:           5 * time.Second,
		TestMethodTimeout:     45 * time.Second,
		TestSetupTimeout:      15 * time.Second,
		TestTeardownTimeout:   8 * time.Second,
		SuiteTimeout:          10 * time.Minute,
		HTTPTimeout:           12 * time.Second,
		HTTPConnectionTimeout: 5 * time.Second,
		HTTPKeepAlive:         45 * time.Second,
		CloneTimeout:          300 * time.Second,
		LSPInitTimeout:        20 * time.Second,
		WorkspaceLoadTimeout:  25 * time.Second,
	}

	// Python language profiles (medium startup time, dependency loading)
	tm.profiles["python-fast"] = &TimeoutProfile{
		Language:               LanguagePython,
		SystemPerformance:      SystemFast,
		RequestTimeout:         5 * time.Second,
		RequestRetryTimeout:    8 * time.Second,
		ServerStartup:          15 * time.Second,
		ServerStartupFastFail:  3 * time.Second,
		HealthCheck:           3 * time.Second,
		TestMethodTimeout:     20 * time.Second,
		TestSetupTimeout:      8 * time.Second,
		TestTeardownTimeout:   5 * time.Second,
		SuiteTimeout:          3 * time.Minute,
		HTTPTimeout:           8 * time.Second,
		HTTPConnectionTimeout: 3 * time.Second,
		HTTPKeepAlive:         25 * time.Second,
		CloneTimeout:          150 * time.Second,
		LSPInitTimeout:        12 * time.Second,
		WorkspaceLoadTimeout:  15 * time.Second,
	}

	tm.profiles["python-normal"] = &TimeoutProfile{
		Language:               LanguagePython,
		SystemPerformance:      SystemNormal,
		RequestTimeout:         8 * time.Second,
		RequestRetryTimeout:    12 * time.Second,
		ServerStartup:          25 * time.Second,
		ServerStartupFastFail:  4 * time.Second,
		HealthCheck:           4 * time.Second,
		TestMethodTimeout:     35 * time.Second,
		TestSetupTimeout:      12 * time.Second,
		TestTeardownTimeout:   8 * time.Second,
		SuiteTimeout:          7 * time.Minute,
		HTTPTimeout:           10 * time.Second,
		HTTPConnectionTimeout: 4 * time.Second,
		HTTPKeepAlive:         35 * time.Second,
		CloneTimeout:          240 * time.Second,
		LSPInitTimeout:        18 * time.Second,
		WorkspaceLoadTimeout:  25 * time.Second,
	}

	tm.profiles["python-slow"] = &TimeoutProfile{
		Language:               LanguagePython,
		SystemPerformance:      SystemSlow,
		RequestTimeout:         12 * time.Second,
		RequestRetryTimeout:    18 * time.Second,
		ServerStartup:          40 * time.Second,
		ServerStartupFastFail:  6 * time.Second,
		HealthCheck:           6 * time.Second,
		TestMethodTimeout:     60 * time.Second,
		TestSetupTimeout:      20 * time.Second,
		TestTeardownTimeout:   12 * time.Second,
		SuiteTimeout:          15 * time.Minute,
		HTTPTimeout:           15 * time.Second,
		HTTPConnectionTimeout: 6 * time.Second,
		HTTPKeepAlive:         50 * time.Second,
		CloneTimeout:          360 * time.Second,
		LSPInitTimeout:        30 * time.Second,
		WorkspaceLoadTimeout:  40 * time.Second,
	}

	// Java language profiles (slow JVM startup, compilation overhead)
	tm.profiles["java-fast"] = &TimeoutProfile{
		Language:               LanguageJava,
		SystemPerformance:      SystemFast,
		RequestTimeout:         8 * time.Second,
		RequestRetryTimeout:    12 * time.Second,
		ServerStartup:          30 * time.Second,
		ServerStartupFastFail:  5 * time.Second,
		HealthCheck:           5 * time.Second,
		TestMethodTimeout:     45 * time.Second,
		TestSetupTimeout:      15 * time.Second,
		TestTeardownTimeout:   10 * time.Second,
		SuiteTimeout:          8 * time.Minute,
		HTTPTimeout:           10 * time.Second,
		HTTPConnectionTimeout: 4 * time.Second,
		HTTPKeepAlive:         40 * time.Second,
		CloneTimeout:          240 * time.Second,
		LSPInitTimeout:        25 * time.Second,
		WorkspaceLoadTimeout:  35 * time.Second,
	}

	tm.profiles["java-normal"] = &TimeoutProfile{
		Language:               LanguageJava,
		SystemPerformance:      SystemNormal,
		RequestTimeout:         12 * time.Second,
		RequestRetryTimeout:    18 * time.Second,
		ServerStartup:          45 * time.Second,
		ServerStartupFastFail:  6 * time.Second,
		HealthCheck:           6 * time.Second,
		TestMethodTimeout:     75 * time.Second,
		TestSetupTimeout:      25 * time.Second,
		TestTeardownTimeout:   15 * time.Second,
		SuiteTimeout:          12 * time.Minute,
		HTTPTimeout:           15 * time.Second,
		HTTPConnectionTimeout: 5 * time.Second,
		HTTPKeepAlive:         60 * time.Second,
		CloneTimeout:          300 * time.Second,
		LSPInitTimeout:        40 * time.Second,
		WorkspaceLoadTimeout:  60 * time.Second,
	}

	tm.profiles["java-slow"] = &TimeoutProfile{
		Language:               LanguageJava,
		SystemPerformance:      SystemSlow,
		RequestTimeout:         18 * time.Second,
		RequestRetryTimeout:    25 * time.Second,
		ServerStartup:          75 * time.Second,
		ServerStartupFastFail:  8 * time.Second,
		HealthCheck:           8 * time.Second,
		TestMethodTimeout:     120 * time.Second,
		TestSetupTimeout:      40 * time.Second,
		TestTeardownTimeout:   20 * time.Second,
		SuiteTimeout:          20 * time.Minute,
		HTTPTimeout:           20 * time.Second,
		HTTPConnectionTimeout: 8 * time.Second,
		HTTPKeepAlive:         90 * time.Second,
		CloneTimeout:          450 * time.Second,
		LSPInitTimeout:        60 * time.Second,
		WorkspaceLoadTimeout:  90 * time.Second,
	}

	// TypeScript language profiles (medium startup, npm dependency loading)
	tm.profiles["typescript-fast"] = &TimeoutProfile{
		Language:               LanguageTypeScript,
		SystemPerformance:      SystemFast,
		RequestTimeout:         6 * time.Second,
		RequestRetryTimeout:    10 * time.Second,
		ServerStartup:          20 * time.Second,
		ServerStartupFastFail:  4 * time.Second,
		HealthCheck:           4 * time.Second,
		TestMethodTimeout:     30 * time.Second,
		TestSetupTimeout:      10 * time.Second,
		TestTeardownTimeout:   6 * time.Second,
		SuiteTimeout:          5 * time.Minute,
		HTTPTimeout:           8 * time.Second,
		HTTPConnectionTimeout: 3 * time.Second,
		HTTPKeepAlive:         30 * time.Second,
		CloneTimeout:          180 * time.Second,
		LSPInitTimeout:        15 * time.Second,
		WorkspaceLoadTimeout:  20 * time.Second,
	}

	tm.profiles["typescript-normal"] = &TimeoutProfile{
		Language:               LanguageTypeScript,
		SystemPerformance:      SystemNormal,
		RequestTimeout:         10 * time.Second,
		RequestRetryTimeout:    15 * time.Second,
		ServerStartup:          35 * time.Second,
		ServerStartupFastFail:  5 * time.Second,
		HealthCheck:           5 * time.Second,
		TestMethodTimeout:     50 * time.Second,
		TestSetupTimeout:      15 * time.Second,
		TestTeardownTimeout:   10 * time.Second,
		SuiteTimeout:          10 * time.Minute,
		HTTPTimeout:           12 * time.Second,
		HTTPConnectionTimeout: 4 * time.Second,
		HTTPKeepAlive:         45 * time.Second,
		CloneTimeout:          270 * time.Second,
		LSPInitTimeout:        25 * time.Second,
		WorkspaceLoadTimeout:  35 * time.Second,
	}

	tm.profiles["typescript-slow"] = &TimeoutProfile{
		Language:               LanguageTypeScript,
		SystemPerformance:      SystemSlow,
		RequestTimeout:         15 * time.Second,
		RequestRetryTimeout:    20 * time.Second,
		ServerStartup:          60 * time.Second,
		ServerStartupFastFail:  7 * time.Second,
		HealthCheck:           7 * time.Second,
		TestMethodTimeout:     90 * time.Second,
		TestSetupTimeout:      25 * time.Second,
		TestTeardownTimeout:   15 * time.Second,
		SuiteTimeout:          18 * time.Minute,
		HTTPTimeout:           18 * time.Second,
		HTTPConnectionTimeout: 6 * time.Second,
		HTTPKeepAlive:         70 * time.Second,
		CloneTimeout:          400 * time.Second,
		LSPInitTimeout:        45 * time.Second,
		WorkspaceLoadTimeout:  60 * time.Second,
	}

	// JavaScript language profiles (similar to TypeScript but slightly faster)
	tm.profiles["javascript-fast"] = tm.profiles["typescript-fast"]
	tm.profiles["javascript-normal"] = tm.profiles["typescript-normal"]  
	tm.profiles["javascript-slow"] = tm.profiles["typescript-slow"]
}

func (tm *TimeoutManager) GetProfile(language Language, performance SystemPerformance) *TimeoutProfile {
	key := fmt.Sprintf("%s-%s", language, performance)
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	
	if profile, exists := tm.profiles[key]; exists {
		return profile
	}
	
	// Fallback to normal performance for the language
	normalKey := fmt.Sprintf("%s-normal", language)
	if profile, exists := tm.profiles[normalKey]; exists {
		log.Printf("[TimeoutManager] Profile %s not found, falling back to %s", key, normalKey)
		return profile
	}
	
	// Final fallback to Go normal (most conservative)
	log.Printf("[TimeoutManager] Profile %s not found, falling back to go-normal", key)
	return tm.profiles["go-normal"]
}

func (tm *TimeoutManager) CreateContext(parent context.Context, level TimeoutLevel, profile *TimeoutProfile, operation string) (context.Context, context.CancelFunc) {
	var timeout time.Duration
	
	switch level {
	case LevelRequest:
		timeout = profile.RequestTimeout
	case LevelOperation:
		if operation == "server_startup" {
			timeout = profile.ServerStartup
		} else if operation == "health_check" {
			timeout = profile.HealthCheck
		} else if operation == "lsp_init" {
			timeout = profile.LSPInitTimeout
		} else if operation == "workspace_load" {
			timeout = profile.WorkspaceLoadTimeout
		} else {
			timeout = profile.TestSetupTimeout
		}
	case LevelTest:
		timeout = profile.TestMethodTimeout
	case LevelSuite:
		timeout = profile.SuiteTimeout
	default:
		timeout = profile.TestMethodTimeout
	}
	
	ctx, cancel := context.WithTimeout(parent, timeout)
	
	// Track context for cleanup
	contextID := fmt.Sprintf("%s-%s-%d", level, operation, time.Now().UnixNano())
	tm.contextMutex.Lock()
	tm.activeContexts[contextID] = cancel
	tm.contextMutex.Unlock()
	
	// Wrap cancel to remove from tracking
	wrappedCancel := func() {
		tm.contextMutex.Lock()
		delete(tm.activeContexts, contextID)
		tm.contextMutex.Unlock()
		cancel()
	}
	
	log.Printf("[TimeoutManager] Created %s context for %s with timeout %v", level, operation, timeout)
	return ctx, wrappedCancel
}

func (tm *TimeoutManager) CancelAllContexts() {
	tm.contextMutex.Lock()
	defer tm.contextMutex.Unlock()
	
	for id, cancel := range tm.activeContexts {
		log.Printf("[TimeoutManager] Cancelling context %s", id)
		cancel()
	}
	tm.activeContexts = make(map[string]context.CancelFunc)
}

func (tm *TimeoutManager) GetActiveContextCount() int {
	tm.contextMutex.Lock()
	defer tm.contextMutex.Unlock()
	return len(tm.activeContexts)
}

// Convenience methods for common timeout scenarios
func (tm *TimeoutManager) CreateRequestContext(parent context.Context, profile *TimeoutProfile) (context.Context, context.CancelFunc) {
	return tm.CreateContext(parent, LevelRequest, profile, "request")
}

func (tm *TimeoutManager) CreateServerStartupContext(parent context.Context, profile *TimeoutProfile) (context.Context, context.CancelFunc) {
	return tm.CreateContext(parent, LevelOperation, profile, "server_startup")
}

func (tm *TimeoutManager) CreateHealthCheckContext(parent context.Context, profile *TimeoutProfile) (context.Context, context.CancelFunc) {
	return tm.CreateContext(parent, LevelOperation, profile, "health_check")
}

func (tm *TimeoutManager) CreateTestContext(parent context.Context, profile *TimeoutProfile) (context.Context, context.CancelFunc) {
	return tm.CreateContext(parent, LevelTest, profile, "test")
}

func (tm *TimeoutManager) CreateSuiteContext(parent context.Context, profile *TimeoutProfile) (context.Context, context.CancelFunc) {
	return tm.CreateContext(parent, LevelSuite, profile, "suite")
}

// Helper function to detect system performance (can be enhanced with actual system metrics)
func DetectSystemPerformance() SystemPerformance {
	// For now, default to normal - could be enhanced with:
	// - CPU core count
	// - Available memory
	// - Disk I/O speed
	// - Previous test execution times
	return SystemNormal
}

// Global timeout manager instance
var globalTimeoutManager *TimeoutManager
var once sync.Once

func GetTimeoutManager() *TimeoutManager {
	once.Do(func() {
		globalTimeoutManager = NewTimeoutManager()
	})
	return globalTimeoutManager
}