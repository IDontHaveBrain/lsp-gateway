package gateway

import (
	"log"
	"os"
	"testing"
	"time"

	"lsp-gateway/internal/config"
)

// TestBypassStateManagerPersistence tests bypass state persistence across restarts
func TestBypassStateManagerPersistence(t *testing.T) {
	// Create temporary directory for state file
	tmpDir, err := os.MkdirTemp("", "bypass_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := log.New(os.Stderr, "[Test] ", log.LstdFlags)

	// Create first instance of bypass state manager
	bsm1 := NewBypassStateManager(tmpDir, logger)

	// Set bypass state for a server
	serverName := "test-python-server"
	language := "python"
	reason := "Consecutive failures threshold exceeded"
	decisionType := BypassDecisionAutoConsecutive

	err = bsm1.SetBypassState(serverName, language, reason, decisionType, false)
	if err != nil {
		t.Fatalf("Failed to set bypass state: %v", err)
	}

	// Verify state is set
	if !bsm1.IsBypassed(serverName) {
		t.Errorf("Server should be bypassed")
	}

	info := bsm1.GetBypassInfo(serverName)
	if info == nil {
		t.Fatalf("Should have bypass info for server")
	}

	if info.BypassReason != reason {
		t.Errorf("Expected reason %s, got %s", reason, info.BypassReason)
	}

	if info.BypassDecisionType != decisionType {
		t.Errorf("Expected decision type %s, got %s", decisionType, info.BypassDecisionType)
	}

	// Create second instance to test persistence
	bsm2 := NewBypassStateManager(tmpDir, logger)

	// Verify state persisted
	if !bsm2.IsBypassed(serverName) {
		t.Errorf("Server bypass state should persist across restarts")
	}

	info2 := bsm2.GetBypassInfo(serverName)
	if info2 == nil {
		t.Fatalf("Should have persisted bypass info for server")
	}

	if info2.BypassReason != reason {
		t.Errorf("Expected persisted reason %s, got %s", reason, info2.BypassReason)
	}

	// Clear bypass state
	err = bsm2.ClearBypassState(serverName)
	if err != nil {
		t.Fatalf("Failed to clear bypass state: %v", err)
	}

	// Create third instance to verify clearing persisted
	bsm3 := NewBypassStateManager(tmpDir, logger)
	if bsm3.IsBypassed(serverName) {
		t.Errorf("Server should not be bypassed after clearing")
	}
}

// TestServerInstanceBypassMethods tests ServerInstance bypass functionality
func TestServerInstanceBypassMethods(t *testing.T) {
	// Create mock LSP client
	mockClient := NewMockLSPClient()

	// Create server config
	serverConfig := &config.ServerConfig{
		Name:      "test-server",
		Languages: []string{"python"},
		Command:   "pylsp",
		Args:      []string{},
	}

	// Create server instance
	server := NewServerInstance(serverConfig, mockClient)

	// Initially server should not be bypassed
	if server.IsBypassed() {
		t.Errorf("New server should not be bypassed")
	}

	if !server.CanAttemptRecovery() {
		t.Errorf("Non-bypassed server should be able to attempt recovery")
	}

	// Bypass the server
	reason := "Manual bypass for testing"
	server.SetBypassState(reason, BypassDecisionUserManual, true)

	// Verify bypass state
	if !server.IsBypassed() {
		t.Errorf("Server should be bypassed after SetBypassState")
	}

	if server.GetState() != ServerStateBypassed {
		t.Errorf("Expected state %v, got %v", ServerStateBypassed, server.GetState())
	}

	// Check bypass info
	isBypassed, bypassReason, timestamp, decisionType, userDecision := server.GetBypassInfo()
	if !isBypassed {
		t.Errorf("GetBypassInfo should indicate server is bypassed")
	}
	if bypassReason != reason {
		t.Errorf("Expected bypass reason %s, got %s", reason, bypassReason)
	}
	if decisionType != BypassDecisionUserManual {
		t.Errorf("Expected decision type %s, got %s", BypassDecisionUserManual, decisionType)
	}
	if !userDecision {
		t.Errorf("Expected user decision to be true")
	}
	if timestamp.IsZero() {
		t.Errorf("Bypass timestamp should be set")
	}

	// User manual bypasses should not allow auto recovery
	if server.CanAttemptRecovery() {
		t.Errorf("User manual bypass should not allow auto recovery")
	}

	// Server should not be active when bypassed
	if server.IsActive() {
		t.Errorf("Bypassed server should not be active")
	}

	// Clear bypass state
	server.ClearBypassState()

	// Verify bypass is cleared
	if server.IsBypassed() {
		t.Errorf("Server should not be bypassed after ClearBypassState")
	}

	if server.GetState() != ServerStateHealthy {
		t.Errorf("Expected state %v after clearing bypass, got %v", ServerStateHealthy, server.GetState())
	}

	// Server should be active again
	if !server.IsActive() {
		t.Errorf("Server should be active after clearing bypass")
	}
}

// TestBypassRecoveryFlow tests the complete bypass and recovery flow
func TestBypassRecoveryFlow(t *testing.T) {
	// Create temporary directory for state file
	tmpDir, err := os.MkdirTemp("", "bypass_recovery_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := log.New(os.Stderr, "[Test] ", log.LstdFlags)

	// Create bypass state manager
	bsm := NewBypassStateManager(tmpDir, logger)

	// Create health monitor with bypass manager
	hm := NewHealthMonitorWithBypassManager(1*time.Second, bsm)

	// Create mock LSP client
	mockClient := NewMockLSPClient()

	// Create server config
	serverConfig := &config.ServerConfig{
		Name:      "recovery-test-server",
		Languages: []string{"javascript"},
		Command:   "typescript-language-server",
		Args:      []string{"--stdio"},
	}

	// Create server instance
	server := NewServerInstance(serverConfig, mockClient)

	// Register server with health monitor
	hm.RegisterServer(server, RecoveryBypassAuto)

	// Bypass server with auto decision (allows recovery)
	reason := "Auto-bypassed due to health issues"
	server.SetBypassState(reason, BypassDecisionAutoConsecutive, false)

	// Record in bypass state manager
	err = bsm.SetBypassState(server.config.Name, server.config.Languages[0], reason, BypassDecisionAutoConsecutive, false)
	if err != nil {
		t.Fatalf("Failed to set bypass state: %v", err)
	}

	// Server should be bypassed
	if !server.IsBypassed() {
		t.Errorf("Server should be bypassed")
	}

	// Auto bypasses should allow recovery after cooldown
	// For testing, we'll manipulate the timestamp to simulate cooldown
	server.bypassTimestamp = time.Now().Add(-10 * time.Minute)

	if !server.CanAttemptRecovery() {
		t.Errorf("Auto-bypassed server should allow recovery after cooldown")
	}

	// Attempt recovery
	err = hm.AttemptBypassRecovery(server.config.Name)
	if err != nil {
		t.Fatalf("Failed to attempt bypass recovery: %v", err)
	}

	// Give some time for recovery process
	time.Sleep(100 * time.Millisecond)

	// Check recovery attempt was recorded
	info := bsm.GetBypassInfo(server.config.Name)
	if info != nil && info.RecoveryAttempts == 0 {
		t.Errorf("Recovery attempt should have been recorded")
	}
}

// TestMultiServerManagerBypassIntegration tests bypass integration with MultiServerManager
func TestMultiServerManagerBypassIntegration(t *testing.T) {
	// Create temporary directory for state file
	tmpDir, err := os.MkdirTemp("", "msm_bypass_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := log.New(os.Stderr, "[Test] ", log.LstdFlags)

	// Create minimal gateway config
	gatewayConfig := &config.GatewayConfig{
		Port: 8080,
		LanguagePools: []config.LanguageServerPool{
			{
				Language: "python",
				Servers: map[string]*config.ServerConfig{
					"pylsp": {
						Name:      "pylsp",
						Languages: []string{"python"},
						Command:   "pylsp",
						Args:      []string{},
					},
				},
				LoadBalancingConfig: &config.LoadBalancingConfig{
					Strategy:        "round_robin",
					HealthThreshold: 0.8,
				},
			},
		},
	}

	// Create MultiServerManager
	msm := NewMultiServerManager(gatewayConfig, logger)

	// Bypass a server
	serverName := "pylsp"
	reason := "Manual bypass for testing"
	err = msm.BypassServer(serverName, reason, true)
	if err != nil {
		t.Fatalf("Failed to bypass server: %v", err)
	}

	// Verify server is bypassed
	if !msm.IsServerBypassed(serverName) {
		t.Errorf("Server should be bypassed")
	}

	// Get bypass info
	info := msm.GetServerBypassInfo(serverName)
	if info == nil {
		t.Fatalf("Should have bypass info")
	}

	if info.BypassReason != reason {
		t.Errorf("Expected bypass reason %s, got %s", reason, info.BypassReason)
	}

	// Get bypassed servers by language
	bypassedServers := msm.GetBypassedServersByLanguage("python")
	if len(bypassedServers) != 1 {
		t.Errorf("Expected 1 bypassed server for python, got %d", len(bypassedServers))
	}

	// Unbypass the server
	err = msm.UnbypassServer(serverName)
	if err != nil {
		t.Fatalf("Failed to unbypass server: %v", err)
	}

	// Verify server is no longer bypassed
	if msm.IsServerBypassed(serverName) {
		t.Errorf("Server should not be bypassed after unbypass")
	}

	// Get bypass statistics
	stats := msm.GetBypassStatistics()
	if stats["total_bypassed"].(int) != 0 {
		t.Errorf("Expected 0 bypassed servers, got %d", stats["total_bypassed"])
	}
}

// TestBypassStateManagerStatistics tests bypass statistics functionality
func TestBypassStateManagerStatistics(t *testing.T) {
	// Create temporary directory for state file
	tmpDir, err := os.MkdirTemp("", "bypass_stats_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := log.New(os.Stderr, "[Test] ", log.LstdFlags)
	bsm := NewBypassStateManager(tmpDir, logger)

	// Add multiple bypassed servers with different types
	testCases := []struct {
		serverName   string
		language     string
		decisionType BypassDecisionType
		userDecision bool
	}{
		{"server1", "python", BypassDecisionUserManual, true},
		{"server2", "javascript", BypassDecisionAutoConsecutive, false},
		{"server3", "python", BypassDecisionAutoCircuitBreaker, false},
		{"server4", "java", BypassDecisionAutoHealthDegraded, false},
	}

	for _, tc := range testCases {
		err := bsm.SetBypassState(tc.serverName, tc.language, "Test reason", tc.decisionType, tc.userDecision)
		if err != nil {
			t.Fatalf("Failed to set bypass state for %s: %v", tc.serverName, err)
		}
	}

	// Test statistics
	stats := bsm.GetBypassStatistics()

	totalBypassed := stats["total_bypassed"].(int)
	if totalBypassed != 4 {
		t.Errorf("Expected 4 total bypassed servers, got %d", totalBypassed)
	}

	decisionTypeCounts := stats["by_decision_type"].(map[BypassDecisionType]int)
	if decisionTypeCounts[BypassDecisionUserManual] != 1 {
		t.Errorf("Expected 1 user manual bypass, got %d", decisionTypeCounts[BypassDecisionUserManual])
	}
	if decisionTypeCounts[BypassDecisionAutoConsecutive] != 1 {
		t.Errorf("Expected 1 auto consecutive bypass, got %d", decisionTypeCounts[BypassDecisionAutoConsecutive])
	}

	languageCounts := stats["by_language"].(map[string]int)
	if languageCounts["python"] != 2 {
		t.Errorf("Expected 2 python servers bypassed, got %d", languageCounts["python"])
	}
	if languageCounts["javascript"] != 1 {
		t.Errorf("Expected 1 javascript server bypassed, got %d", languageCounts["javascript"])
	}

	// Test cleanup of expired entries
	// For testing, we'll call cleanup immediately (normally auto bypasses expire after 7 days)
	err = bsm.CleanupExpiredEntries()
	if err != nil {
		t.Fatalf("Failed to cleanup expired entries: %v", err)
	}

	// User manual bypasses should remain, auto bypasses would be cleaned up after 7 days
	// Since we just created them, they won't be cleaned up yet
	statsAfterCleanup := bsm.GetBypassStatistics()
	totalAfterCleanup := statsAfterCleanup["total_bypassed"].(int)
	if totalAfterCleanup != 4 {
		t.Errorf("Expected 4 servers still bypassed after cleanup (not old enough), got %d", totalAfterCleanup)
	}
}