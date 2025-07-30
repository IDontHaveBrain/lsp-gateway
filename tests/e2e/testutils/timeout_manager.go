package testutils

import (
	"context"
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
	Category          SystemPerformance
	
	// Core operation timeouts
	ServerStartup     time.Duration
	ServerStartupFastFail time.Duration
	HealthCheck       time.Duration
	
	// Request and test timeouts
	RequestTimeout    time.Duration
	TestTimeout       time.Duration
	TestMethodTimeout time.Duration
	SuiteTimeout      time.Duration
	
	// HTTP client timeouts
	HTTPTimeout          time.Duration
	HTTPConnectionTimeout time.Duration
	HTTPKeepAlive        time.Duration
	
	// Repository and LSP timeouts
	CloneTimeout         time.Duration
	LSPInitTimeout       time.Duration
	WorkspaceLoadTimeout time.Duration
	TestSetupTimeout     time.Duration
	TestTeardownTimeout  time.Duration
}

type TimeoutManager struct {
	profiles map[SystemPerformance]*TimeoutProfile
	mutex    sync.RWMutex
}

func NewTimeoutManager() *TimeoutManager {
	tm := &TimeoutManager{
		profiles: make(map[SystemPerformance]*TimeoutProfile),
	}
	tm.initializeSimplifiedProfiles()
	return tm
}

func (tm *TimeoutManager) initializeSimplifiedProfiles() {
	// Fast: Lightweight servers (Go, lightweight services)
	tm.profiles[SystemFast] = &TimeoutProfile{
		Category:              SystemFast,
		ServerStartup:         8 * time.Second,
		ServerStartupFastFail: 4 * time.Second,
		HealthCheck:           1 * time.Second,
		RequestTimeout:        3 * time.Second,
		TestTimeout:           15 * time.Second,
		TestMethodTimeout:     15 * time.Second,
		SuiteTimeout:          3 * time.Minute,
		HTTPTimeout:           10 * time.Second,
		HTTPConnectionTimeout: 5 * time.Second,
		HTTPKeepAlive:         30 * time.Second,
		CloneTimeout:          120 * time.Second,
		LSPInitTimeout:        5 * time.Second,
		WorkspaceLoadTimeout:  8 * time.Second,
		TestSetupTimeout:      10 * time.Second,
		TestTeardownTimeout:   5 * time.Second,
	}

	// Normal: Standard servers (Python, TypeScript/JavaScript)
	tm.profiles[SystemNormal] = &TimeoutProfile{
		Category:              SystemNormal,
		ServerStartup:         15 * time.Second,
		ServerStartupFastFail: 8 * time.Second,
		HealthCheck:           2 * time.Second,
		RequestTimeout:        5 * time.Second,
		TestTimeout:           30 * time.Second,
		TestMethodTimeout:     30 * time.Second,
		SuiteTimeout:          8 * time.Minute,
		HTTPTimeout:           15 * time.Second,
		HTTPConnectionTimeout: 8 * time.Second,
		HTTPKeepAlive:         60 * time.Second,
		CloneTimeout:          120 * time.Second,
		LSPInitTimeout:        10 * time.Second,
		WorkspaceLoadTimeout:  15 * time.Second,
		TestSetupTimeout:      20 * time.Second,
		TestTeardownTimeout:   10 * time.Second,
	}

	// Slow: Heavy servers (Java JVM, heavy services with long startup)
	tm.profiles[SystemSlow] = &TimeoutProfile{
		Category:              SystemSlow,
		ServerStartup:         30 * time.Second,
		ServerStartupFastFail: 15 * time.Second,
		HealthCheck:           3 * time.Second,
		RequestTimeout:        8 * time.Second,
		TestTimeout:           60 * time.Second,
		TestMethodTimeout:     60 * time.Second,
		SuiteTimeout:          15 * time.Minute,
		HTTPTimeout:           25 * time.Second,
		HTTPConnectionTimeout: 12 * time.Second,
		HTTPKeepAlive:         90 * time.Second,
		CloneTimeout:          120 * time.Second,
		LSPInitTimeout:        20 * time.Second,
		WorkspaceLoadTimeout:  30 * time.Second,
		TestSetupTimeout:      45 * time.Second,
		TestTeardownTimeout:   20 * time.Second,
	}
}



func (tm *TimeoutManager) GetProfile(language Language, performance SystemPerformance) *TimeoutProfile {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	
	// Map language to performance category based on server characteristics
	category := tm.categorizeLanguage(language, performance)
	
	if profile, exists := tm.profiles[category]; exists {
		return profile
	}
	
	// Fallback to Normal if category not found
	log.Printf("[TimeoutManager] Category %s not found, falling back to Normal", category)
	return tm.profiles[SystemNormal]
}

// categorizeLanguage maps languages to performance categories based on server characteristics
func (tm *TimeoutManager) categorizeLanguage(language Language, performance SystemPerformance) SystemPerformance {
	// For Java, always use Slow category due to JVM startup overhead
	if language == LanguageJava {
		if performance == SystemFast {
			return SystemNormal // Java fast -> Normal (still heavy)
		}
		return SystemSlow
	}
	
	// For Go, favor Fast category due to lightweight nature
	if language == LanguageGo {
		if performance == SystemSlow {
			return SystemNormal // Go slow -> Normal
		}
		return SystemFast
	}
	
	// For Python, TypeScript, JavaScript: use requested performance directly
	return performance
}

func (tm *TimeoutManager) CreateContext(parent context.Context, level TimeoutLevel, profile *TimeoutProfile, operation string) (context.Context, context.CancelFunc) {
	var timeout time.Duration
	
	switch level {
	case LevelRequest:
		timeout = profile.RequestTimeout
	case LevelOperation:
		if operation == "server_startup" || operation == "lsp_init" || operation == "workspace_load" {
			timeout = profile.ServerStartup
		} else if operation == "health_check" {
			timeout = profile.HealthCheck
		} else {
			timeout = profile.TestTimeout
		}
	case LevelTest:
		timeout = profile.TestTimeout
	case LevelSuite:
		timeout = profile.SuiteTimeout
	default:
		timeout = profile.TestTimeout
	}
	
	ctx, cancel := context.WithTimeout(parent, timeout)
	log.Printf("[TimeoutManager] Created %s context for %s with timeout %v (category: %s)", level, operation, timeout, profile.Category)
	return ctx, cancel
}

func (tm *TimeoutManager) CancelAllContexts() {
	// Simplified: no active context tracking
	log.Printf("[TimeoutManager] CancelAllContexts called - no contexts to track in simplified version")
}

func (tm *TimeoutManager) GetActiveContextCount() int {
	// Simplified: no active context tracking
	return 0
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