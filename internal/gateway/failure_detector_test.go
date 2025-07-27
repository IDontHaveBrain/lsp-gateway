package gateway

import (
	"errors"
	"testing"
	"time"
)

// Mock implementations for testing

type mockFailureSubscriber struct {
	detectedFailures []string
	resolvedFailures []string
}

func (m *mockFailureSubscriber) OnFailureDetected(failure *FailureInfo) {
	m.detectedFailures = append(m.detectedFailures, failure.ID)
}

func (m *mockFailureSubscriber) OnFailureResolved(serverName string, failure *FailureInfo) {
	m.resolvedFailures = append(m.resolvedFailures, failure.ID)
}

func createTestFailureDetector() *FailureDetector {
	healthMonitor := NewHealthMonitor(1 * time.Second)
	circuitBreakerMgr := NewCircuitBreakerManager()
	config := DefaultFailureDetectorConfig()
	config.MaxRecentFailures = 5
	config.FailureRetentionTime = 10 * time.Minute
	return NewFailureDetectorWithConfig(healthMonitor, circuitBreakerMgr, config)
}

func TestFailureDetectorCreation(t *testing.T) {
	fd := createTestFailureDetector()
	if fd == nil {
		t.Fatal("Failed to create failure detector")
	}
	
	if len(fd.rules) == 0 {
		t.Error("No detection rules were initialized")
	}
	
	if fd.config.MaxRecentFailures != 5 {
		t.Errorf("Expected MaxRecentFailures to be 5, got %d", fd.config.MaxRecentFailures)
	}
}

func TestFailureCategoryString(t *testing.T) {
	tests := []struct {
		category FailureCategory
		expected string
	}{
		{FailureCategoryStartup, "startup"},
		{FailureCategoryRuntime, "runtime"},
		{FailureCategoryConfiguration, "configuration"},
		{FailureCategoryTransport, "transport"},
		{FailureCategoryResource, "resource"},
		{FailureCategoryUnknown, "unknown"},
	}
	
	for _, test := range tests {
		if test.category.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.category.String())
		}
	}
}

func TestFailureSeverityString(t *testing.T) {
	tests := []struct {
		severity FailureSeverity
		expected string
	}{
		{FailureSeverityLow, "low"},
		{FailureSeverityMedium, "medium"},
		{FailureSeverityHigh, "high"},
		{FailureSeverityCritical, "critical"},
	}
	
	for _, test := range tests {
		if test.severity.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.severity.String())
		}
	}
}

func TestDetectStartupTimeoutFailure(t *testing.T) {
	fd := createTestFailureDetector()
	
	err := errors.New("timeout occurred during start process")
	context := &FailureContext{
		ServerName: "test-server",
		Language:   "python",
	}
	
	failure := fd.DetectFailure(err, context)
	if failure == nil {
		t.Fatal("Expected failure to be detected")
	}
	
	if failure.Category != FailureCategoryStartup {
		t.Errorf("Expected startup category, got %s", failure.Category.String())
	}
	
	if failure.Severity != FailureSeverityHigh {
		t.Errorf("Expected high severity, got %s", failure.Severity.String())
	}
	
	if failure.Context.ServerName != "test-server" {
		t.Errorf("Expected server name 'test-server', got %s", failure.Context.ServerName)
	}
	
	if len(failure.RecoveryRecommendations) == 0 {
		t.Error("Expected recovery recommendations to be provided")
	}
}

func TestDetectConnectionRefusedFailure(t *testing.T) {
	fd := createTestFailureDetector()
	
	err := errors.New("connection refused by server")
	context := &FailureContext{
		ServerName: "test-server",
		Language:   "javascript",
	}
	
	failure := fd.DetectFailure(err, context)
	if failure == nil {
		t.Fatal("Expected failure to be detected")
	}
	
	if failure.Category != FailureCategoryTransport {
		t.Errorf("Expected transport category, got %s", failure.Category.String())
	}
	
	if failure.Severity != FailureSeverityHigh {
		t.Errorf("Expected high severity, got %s", failure.Severity.String())
	}
	
	// Check for restart recommendation
	hasRestartRecommendation := false
	for _, rec := range failure.RecoveryRecommendations {
		if rec.Action == "restart_server" {
			hasRestartRecommendation = true
			break
		}
	}
	
	if !hasRestartRecommendation {
		t.Error("Expected restart_server recommendation for connection refused failure")
	}
}

func TestDetectMemoryFailure(t *testing.T) {
	fd := createTestFailureDetector()
	
	err := errors.New("out of memory: cannot allocate more")
	context := &FailureContext{
		ServerName: "memory-hungry-server",
		Language:   "java",
	}
	
	failure := fd.DetectFailure(err, context)
	if failure == nil {
		t.Fatal("Expected failure to be detected")
	}
	
	if failure.Category != FailureCategoryResource {
		t.Errorf("Expected resource category, got %s", failure.Category.String())
	}
	
	if failure.Severity != FailureSeverityCritical {
		t.Errorf("Expected critical severity, got %s", failure.Severity.String())
	}
	
	// Check for memory-specific recommendations
	hasMemoryRecommendation := false
	for _, rec := range failure.RecoveryRecommendations {
		if rec.Action == "increase_memory_limit" {
			hasMemoryRecommendation = true
			break
		}
	}
	
	if !hasMemoryRecommendation {
		t.Error("Expected increase_memory_limit recommendation for memory failure")
	}
}

func TestDetectGenericFailure(t *testing.T) {
	fd := createTestFailureDetector()
	
	err := errors.New("some unknown error occurred")
	context := &FailureContext{
		ServerName: "test-server",
		Language:   "rust",
	}
	
	failure := fd.DetectFailure(err, context)
	if failure == nil {
		t.Fatal("Expected failure to be detected")
	}
	
	if failure.Category != FailureCategoryUnknown {
		t.Errorf("Expected unknown category, got %s", failure.Category.String())
	}
	
	if failure.Severity != FailureSeverityMedium {
		t.Errorf("Expected medium severity, got %s", failure.Severity.String())
	}
	
	if !failure.IsRecoverable {
		t.Error("Generic failures should be marked as recoverable")
	}
}

func TestAnalyzeServerHealthWithConsecutiveFailures(t *testing.T) {
	fd := createTestFailureDetector()
	
	healthInfo := &HealthStatusInfo{
		IsHealthy:           false,
		ConsecutiveFailures: 5,
		ResponseTime:        2 * time.Second,
		ErrorRate:           0.15,
		LastHealthCheck:     time.Now(),
	}
	
	failures := fd.AnalyzeServerHealth("unhealthy-server", healthInfo)
	if len(failures) == 0 {
		t.Fatal("Expected health analysis to detect failures")
	}
	
	// Should detect consecutive failures
	hasConsecutiveFailure := false
	for _, failure := range failures {
		if failure.Category == FailureCategoryRuntime && 
		   failure.ErrorMessage != "" && 
		   failure.ErrorMessage != "some message about consecutive failures" {
			hasConsecutiveFailure = true
			break
		}
	}
	
	if !hasConsecutiveFailure {
		t.Error("Expected consecutive failure detection in health analysis")
	}
}

func TestAnalyzeServerHealthWithHighErrorRate(t *testing.T) {
	fd := createTestFailureDetector()
	
	healthInfo := &HealthStatusInfo{
		IsHealthy:           false,
		ConsecutiveFailures: 1,
		ResponseTime:        1 * time.Second,
		ErrorRate:           0.35, // 35% error rate, above 25% threshold
		LastHealthCheck:     time.Now(),
	}
	
	failures := fd.AnalyzeServerHealth("error-prone-server", healthInfo)
	if len(failures) == 0 {
		t.Fatal("Expected health analysis to detect high error rate failure")
	}
	
	// Should detect high error rate
	hasErrorRateFailure := false
	for _, failure := range failures {
		if failure.Category == FailureCategoryRuntime && 
		   failure.Severity == FailureSeverityHigh {
			hasErrorRateFailure = true
			break
		}
	}
	
	if !hasErrorRateFailure {
		t.Error("Expected error rate failure detection in health analysis")
	}
}

func TestAnalyzeCircuitBreakerState(t *testing.T) {
	fd := createTestFailureDetector()
	
	// Create a circuit breaker and force it to open state
	cb := NewCircuitBreaker(3, 30*time.Second, 2)
	cb.ForceOpen()
	
	failure := fd.AnalyzeCircuitBreakerState("circuit-broken-server", cb)
	if failure == nil {
		t.Fatal("Expected circuit breaker analysis to detect failure")
	}
	
	if failure.Category != FailureCategoryRuntime {
		t.Errorf("Expected runtime category, got %s", failure.Category.String())
	}
	
	if failure.Severity != FailureSeverityHigh {
		t.Errorf("Expected high severity, got %s", failure.Severity.String())
	}
	
	if failure.DetectionSource != "circuit_breaker" {
		t.Errorf("Expected detection source 'circuit_breaker', got %s", failure.DetectionSource)
	}
}

func TestFailureSubscription(t *testing.T) {
	fd := createTestFailureDetector()
	subscriber := &mockFailureSubscriber{}
	
	fd.Subscribe(subscriber)
	
	err := errors.New("test error for subscription")
	context := &FailureContext{
		ServerName: "subscription-test-server",
		Language:   "go",
	}
	
	failure := fd.DetectFailure(err, context)
	if failure == nil {
		t.Fatal("Expected failure to be detected")
	}
	
	// Give some time for goroutine to execute
	time.Sleep(10 * time.Millisecond)
	
	if len(subscriber.detectedFailures) != 1 {
		t.Errorf("Expected 1 detected failure notification, got %d", len(subscriber.detectedFailures))
	}
	
	if subscriber.detectedFailures[0] != failure.ID {
		t.Errorf("Expected failure ID %s, got %s", failure.ID, subscriber.detectedFailures[0])
	}
}

func TestGetRecentFailures(t *testing.T) {
	fd := createTestFailureDetector()
	
	serverName := "recent-failures-test"
	
	// Generate multiple failures
	for i := 0; i < 3; i++ {
		err := errors.New("test error " + string(rune(i+'A')))
		context := &FailureContext{
			ServerName: serverName,
			Language:   "python",
		}
		fd.DetectFailure(err, context)
	}
	
	recentFailures := fd.GetRecentFailures(serverName)
	if len(recentFailures) != 3 {
		t.Errorf("Expected 3 recent failures, got %d", len(recentFailures))
	}
	
	// Test server with no failures
	noFailures := fd.GetRecentFailures("non-existent-server")
	if len(noFailures) != 0 {
		t.Errorf("Expected 0 failures for non-existent server, got %d", len(noFailures))
	}
}

func TestFailureRetentionLimit(t *testing.T) {
	fd := createTestFailureDetector()
	
	serverName := "retention-test-server"
	
	// Generate more failures than the max retention limit (5)
	for i := 0; i < 8; i++ {
		err := errors.New("test error " + string(rune(i+'A')))
		context := &FailureContext{
			ServerName: serverName,
			Language:   "javascript",
		}
		fd.DetectFailure(err, context)
	}
	
	recentFailures := fd.GetRecentFailures(serverName)
	if len(recentFailures) != 5 {
		t.Errorf("Expected 5 recent failures (max limit), got %d", len(recentFailures))
	}
}

func TestGetFailuresByCategory(t *testing.T) {
	fd := createTestFailureDetector()
	
	serverName := "category-test-server"
	
	// Generate failures of different categories
	startupErr := errors.New("timeout during start")
	transportErr := errors.New("connection refused")
	memoryErr := errors.New("out of memory")
	
	context := &FailureContext{
		ServerName: serverName,
		Language:   "java",
	}
	
	fd.DetectFailure(startupErr, context)
	fd.DetectFailure(transportErr, context)
	fd.DetectFailure(memoryErr, context)
	
	startupFailures := fd.GetFailuresByCategory(serverName, FailureCategoryStartup)
	if len(startupFailures) != 1 {
		t.Errorf("Expected 1 startup failure, got %d", len(startupFailures))
	}
	
	transportFailures := fd.GetFailuresByCategory(serverName, FailureCategoryTransport)
	if len(transportFailures) != 1 {
		t.Errorf("Expected 1 transport failure, got %d", len(transportFailures))
	}
	
	resourceFailures := fd.GetFailuresByCategory(serverName, FailureCategoryResource)
	if len(resourceFailures) != 1 {
		t.Errorf("Expected 1 resource failure, got %d", len(resourceFailures))
	}
}

func TestGetFailuresBySeverity(t *testing.T) {
	fd := createTestFailureDetector()
	
	serverName := "severity-test-server"
	
	// Generate failures with different severities
	startupErr := errors.New("timeout during start") // High severity
	memoryErr := errors.New("out of memory")         // Critical severity
	
	context := &FailureContext{
		ServerName: serverName,
		Language:   "rust",
	}
	
	fd.DetectFailure(startupErr, context)
	fd.DetectFailure(memoryErr, context)
	
	highSeverityFailures := fd.GetFailuresBySeverity(serverName, FailureSeverityHigh)
	if len(highSeverityFailures) != 1 {
		t.Errorf("Expected 1 high severity failure, got %d", len(highSeverityFailures))
	}
	
	criticalSeverityFailures := fd.GetFailuresBySeverity(serverName, FailureSeverityCritical)
	if len(criticalSeverityFailures) != 1 {
		t.Errorf("Expected 1 critical severity failure, got %d", len(criticalSeverityFailures))
	}
}

func TestFailureInfoJSON(t *testing.T) {
	failure := &FailureInfo{
		ID:              "test-failure-123",
		Category:        FailureCategoryStartup,
		Severity:        FailureSeverityHigh,
		Timestamp:       time.Now(),
		ErrorMessage:    "Test error message",
		RootCause:       "Test root cause",
		DetectionSource: "test_detector",
		IsRecoverable:   true,
		Context: &FailureContext{
			ServerName: "test-server",
			Language:   "python",
		},
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "restart_server",
				Description: "Restart the server",
				AutoFix:     true,
				Priority:    1,
			},
		},
	}
	
	jsonData, err := failure.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert failure to JSON: %v", err)
	}
	
	if len(jsonData) == 0 {
		t.Error("JSON data should not be empty")
	}
	
	// Basic validation that it contains expected fields
	jsonStr := string(jsonData)
	expectedFields := []string{
		"test-failure-123",
		"startup",
		"high",
		"Test error message",
		"test-server",
		"restart_server",
	}
	
	for _, field := range expectedFields {
		if !containsString(jsonStr, field) {
			t.Errorf("JSON should contain field %s", field)
		}
	}
}

func TestDetermineSeverityFromConsecutiveFailures(t *testing.T) {
	fd := createTestFailureDetector()
	
	tests := []struct {
		failures int
		expected FailureSeverity
	}{
		{1, FailureSeverityLow},
		{2, FailureSeverityLow},
		{3, FailureSeverityMedium},
		{4, FailureSeverityMedium},
		{5, FailureSeverityHigh},
		{9, FailureSeverityHigh},
		{10, FailureSeverityCritical},
		{15, FailureSeverityCritical},
	}
	
	for _, test := range tests {
		result := fd.determineSeverityFromConsecutiveFailures(test.failures)
		if result != test.expected {
			t.Errorf("For %d failures, expected %s severity, got %s", 
				test.failures, test.expected.String(), result.String())
		}
	}
}

func TestGenerateHealthFailureRecommendations(t *testing.T) {
	fd := createTestFailureDetector()
	
	healthInfo := &HealthStatusInfo{
		ConsecutiveFailures: 7,
		RecoveryAttempts:    5,
	}
	
	recommendations := fd.generateHealthFailureRecommendations(healthInfo)
	if len(recommendations) == 0 {
		t.Fatal("Expected health failure recommendations")
	}
	
	// Should have restart recommendation
	hasRestart := false
	hasConfigCheck := false
	hasReplace := false
	
	for _, rec := range recommendations {
		switch rec.Action {
		case "restart_server":
			hasRestart = true
		case "check_server_config":
			hasConfigCheck = true
		case "replace_server":
			hasReplace = true
		}
	}
	
	if !hasRestart {
		t.Error("Expected restart_server recommendation")
	}
	
	if !hasConfigCheck {
		t.Error("Expected check_server_config recommendation for high consecutive failures")
	}
	
	if !hasReplace {
		t.Error("Expected replace_server recommendation for high recovery attempts")
	}
}

// Helper function to check if a string contains a substring
func containsString(str, substr string) bool {
	return len(str) >= len(substr) && 
		   (str == substr || 
		    str[:len(substr)] == substr || 
		    str[len(str)-len(substr):] == substr ||
		    findInString(str, substr))
}

func findInString(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests

func BenchmarkDetectFailure(b *testing.B) {
	fd := createTestFailureDetector()
	err := errors.New("benchmark test error")
	context := &FailureContext{
		ServerName: "benchmark-server",
		Language:   "go",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd.DetectFailure(err, context)
	}
}

func BenchmarkAnalyzeServerHealth(b *testing.B) {
	fd := createTestFailureDetector()
	healthInfo := &HealthStatusInfo{
		IsHealthy:           false,
		ConsecutiveFailures: 5,
		ResponseTime:        3 * time.Second,
		ErrorRate:           0.3,
		LastHealthCheck:     time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd.AnalyzeServerHealth("benchmark-server", healthInfo)
	}
}