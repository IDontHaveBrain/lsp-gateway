package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewFailureNotifier(t *testing.T) {
	// Test with nil config (should use defaults)
	notifier := NewFailureNotifier(nil)
	if notifier == nil {
		t.Fatal("Expected notifier to be created with default config")
	}
	
	if notifier.config.Mode != NotificationModeInteractive {
		t.Errorf("Expected default mode to be Interactive, got %v", notifier.config.Mode)
	}
	
	// Test with custom config
	config := &NotificationConfig{
		Mode:             NotificationModeJSON,
		SeverityFilter:   FailureSeverityHigh,
		BatchThreshold:   5,
		InteractivePrompts: false,
	}
	
	notifier = NewFailureNotifier(config)
	if notifier.config.Mode != NotificationModeJSON {
		t.Errorf("Expected mode to be JSON, got %v", notifier.config.Mode)
	}
	
	if notifier.config.SeverityFilter != FailureSeverityHigh {
		t.Errorf("Expected severity filter to be High, got %v", notifier.config.SeverityFilter)
	}
}

func TestUserDecisionString(t *testing.T) {
	tests := []struct {
		decision UserDecision
		expected string
	}{
		{UserDecisionRetry, "retry"},
		{UserDecisionBypass, "bypass"},
		{UserDecisionDebug, "debug"},
		{UserDecisionSkip, "skip"},
		{UserDecisionConfigure, "configure"},
		{UserDecisionBatchBypass, "batch_bypass"},
		{UserDecisionBatchRetry, "batch_retry"},
		{UserDecisionCancel, "cancel"},
		{UserDecision(999), "unknown"},
	}
	
	for _, tt := range tests {
		if got := tt.decision.String(); got != tt.expected {
			t.Errorf("UserDecision.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestCreateFailureNotification(t *testing.T) {
	recommendations := []RecoveryRecommendation{
		{
			Action:      "restart",
			Description: "Restart the server",
			Commands:    []string{"lsp-gateway server --restart"},
			AutoFix:     false,
			Priority:    1,
		},
	}
	
	notification := CreateFailureNotification(
		"gopls", "go", FailureCategoryStartup, FailureSeverityHigh,
		"Server failed to start", errors.New("port already in use"),
		recommendations)
	
	if notification.ServerName != "gopls" {
		t.Errorf("Expected server name 'gopls', got %s", notification.ServerName)
	}
	
	if notification.Language != "go" {
		t.Errorf("Expected language 'go', got %s", notification.Language)
	}
	
	if notification.Category != FailureCategoryStartup {
		t.Errorf("Expected category Startup, got %v", notification.Category)
	}
	
	if notification.Severity != FailureSeverityHigh {
		t.Errorf("Expected severity High, got %v", notification.Severity)
	}
	
	if !notification.BypassAvailable {
		t.Error("Expected bypass to be available")
	}
	
	if len(notification.Recommendations) != 1 {
		t.Errorf("Expected 1 recommendation, got %d", len(notification.Recommendations))
	}
}

func TestCreateBatchFailureNotification(t *testing.T) {
	failures := []FailureNotification{
		{
			ServerName:      "gopls",
			Language:        "go",
			Severity:        FailureSeverityCritical,
			BypassAvailable: true,
		},
		{
			ServerName:      "pylsp",
			Language:        "python",
			Severity:        FailureSeverityMedium,
			BypassAvailable: false,
		},
		{
			ServerName:      "jdtls",
			Language:        "java",
			Severity:        FailureSeverityCritical,
			BypassAvailable: true,
		},
	}
	
	batch := CreateBatchFailureNotification(failures, "mixed")
	
	if batch.TotalCount != 3 {
		t.Errorf("Expected total count 3, got %d", batch.TotalCount)
	}
	
	if batch.CriticalCount != 2 {
		t.Errorf("Expected critical count 2, got %d", batch.CriticalCount)
	}
	
	if batch.BypassableCount != 2 {
		t.Errorf("Expected bypassable count 2, got %d", batch.BypassableCount)
	}
	
	if batch.Language != "mixed" {
		t.Errorf("Expected language 'mixed', got %s", batch.Language)
	}
}

func TestCreateFormattedError(t *testing.T) {
	notifier := NewFailureNotifier(nil)
	
	notification := FailureNotification{
		ServerName:  "gopls",
		Language:    "go",
		Category:    FailureCategoryStartup,
		Severity:    FailureSeverityHigh,
		Message:     "Failed to bind to port",
		Cause:       errors.New("address already in use"),
		Recommendations: []RecoveryRecommendation{
			{
				Description: "Try a different port",
				Commands:    []string{"server --port 8081"},
			},
		},
		BypassAvailable: true,
	}
	
	formattedError := notifier.createFormattedError(notification)
	
	if formattedError.Type != CLIErrorTypeServer {
		t.Errorf("Expected error type Server, got %v", formattedError.Type)
	}
	
	expectedMessage := "Startup server 'gopls' failed: Failed to bind to port (language: go)"
	if formattedError.Message != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, formattedError.Message)
	}
	
	if formattedError.Cause == nil {
		t.Error("Expected cause to be set")
	}
	
	// Check for bypass suggestion
	found := false
	for _, suggestion := range formattedError.Suggestions {
		if strings.Contains(suggestion, "bypass") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected bypass suggestion to be included")
	}
}

func TestMapFailureCategoryToErrorType(t *testing.T) {
	notifier := NewFailureNotifier(nil)
	
	tests := []struct {
		category    FailureCategory
		expectedType CLIErrorType
	}{
		{FailureCategoryStartup, CLIErrorTypeServer},
		{FailureCategoryRuntime, CLIErrorTypeRuntime},
		{FailureCategoryConfiguration, CLIErrorTypeConfig},
		{FailureCategoryTransport, CLIErrorTypeNetwork},
		{FailureCategoryResource, CLIErrorTypeRuntime},
		{FailureCategoryUnknown, CLIErrorTypeGeneral},
	}
	
	for _, tt := range tests {
		got := notifier.mapFailureCategoryToErrorType(tt.category)
		if got != tt.expectedType {
			t.Errorf("mapFailureCategoryToErrorType(%v) = %v, want %v",
				tt.category, got, tt.expectedType)
		}
	}
}

func TestGetSeverityIcon(t *testing.T) {
	notifier := NewFailureNotifier(nil)
	
	tests := []struct {
		severity FailureSeverity
		expected string
	}{
		{FailureSeverityLow, "⚠️"},
		{FailureSeverityMedium, "🟡"},
		{FailureSeverityHigh, "🟠"},
		{FailureSeverityCritical, "🔴"},
		{FailureSeverity(999), "❓"},
	}
	
	for _, tt := range tests {
		got := notifier.getSeverityIcon(tt.severity)
		if got != tt.expected {
			t.Errorf("getSeverityIcon(%v) = %v, want %v", tt.severity, got, tt.expected)
		}
	}
}

func TestShouldAutoBypass(t *testing.T) {
	config := &NotificationConfig{
		AutoBypassPolicy: map[FailureCategory]bool{
			FailureCategoryStartup: true,
			FailureCategoryRuntime: false,
		},
	}
	
	notifier := NewFailureNotifier(config)
	
	tests := []struct {
		name         string
		notification FailureNotification
		expected     bool
	}{
		{
			name: "bypass available and policy allows",
			notification: FailureNotification{
				Category:        FailureCategoryStartup,
				BypassAvailable: true,
			},
			expected: true,
		},
		{
			name: "bypass available but policy denies",
			notification: FailureNotification{
				Category:        FailureCategoryRuntime,
				BypassAvailable: true,
			},
			expected: false,
		},
		{
			name: "bypass not available",
			notification: FailureNotification{
				Category:        FailureCategoryStartup,
				BypassAvailable: false,
			},
			expected: false,
		},
		{
			name: "critical failure with auto bypass reason",
			notification: FailureNotification{
				Category:         FailureCategoryTransport,
				Severity:         FailureSeverityCritical,
				BypassAvailable:  true,
				AutoBypassReason: "consecutive failures",
			},
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notifier.shouldAutoBypass(tt.notification)
			if got != tt.expected {
				t.Errorf("shouldAutoBypass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractServerNames(t *testing.T) {
	notifier := NewFailureNotifier(nil)
	
	failures := []FailureNotification{
		{ServerName: "gopls"},
		{ServerName: "pylsp"},
		{ServerName: "jdtls"},
	}
	
	names := notifier.extractServerNames(failures)
	expected := []string{"gopls", "pylsp", "jdtls"}
	
	if len(names) != len(expected) {
		t.Fatalf("Expected %d names, got %d", len(expected), len(names))
	}
	
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected name %s at index %d, got %s", expected[i], i, name)
		}
	}
}

func TestExtractBypassableServerNames(t *testing.T) {
	notifier := NewFailureNotifier(nil)
	
	failures := []FailureNotification{
		{ServerName: "gopls", BypassAvailable: true},
		{ServerName: "pylsp", BypassAvailable: false},
		{ServerName: "jdtls", BypassAvailable: true},
	}
	
	names := notifier.extractBypassableServerNames(failures)
	expected := []string{"gopls", "jdtls"}
	
	if len(names) != len(expected) {
		t.Fatalf("Expected %d names, got %d", len(expected), len(names))
	}
	
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected name %s at index %d, got %s", expected[i], i, name)
		}
	}
}

func TestExtractCriticalServerNames(t *testing.T) {
	notifier := NewFailureNotifier(nil)
	
	failures := []FailureNotification{
		{ServerName: "gopls", Severity: FailureSeverityCritical, BypassAvailable: true},
		{ServerName: "pylsp", Severity: FailureSeverityMedium, BypassAvailable: true},
		{ServerName: "jdtls", Severity: FailureSeverityCritical, BypassAvailable: false},
		{ServerName: "tsserver", Severity: FailureSeverityCritical, BypassAvailable: true},
	}
	
	names := notifier.extractCriticalServerNames(failures)
	expected := []string{"gopls", "tsserver"}
	
	if len(names) != len(expected) {
		t.Fatalf("Expected %d names, got %d", len(expected), len(names))
	}
	
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected name %s at index %d, got %s", expected[i], i, name)
		}
	}
}

func TestHandleJSONFailure(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	
	config := &NotificationConfig{
		Mode: NotificationModeJSON,
	}
	notifier := NewFailureNotifier(config)
	
	notification := FailureNotification{
		ServerName: "gopls",
		Language:   "go",
		Category:   FailureCategoryStartup,
		Severity:   FailureSeverityHigh,
		Message:    "Server failed to start",
		Timestamp:  time.Now(),
	}
	
	response, err := notifier.handleJSONFailure(notification)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout
	
	// Read output
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	output := buf.String()
	
	// Verify JSON output
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	
	if result["type"] != "failure_notification" {
		t.Errorf("Expected type 'failure_notification', got %v", result["type"])
	}
	
	// Verify response
	if response.Decision != UserDecisionRetry {
		t.Errorf("Expected decision Retry, got %v", response.Decision)
	}
	
	if len(response.ServerNames) != 1 || response.ServerNames[0] != "gopls" {
		t.Errorf("Expected server names [gopls], got %v", response.ServerNames)
	}
}

func TestHandleNonInteractiveFailure(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	
	config := &NotificationConfig{
		Mode: NotificationModeNonInteractive,
		AutoBypassPolicy: map[FailureCategory]bool{
			FailureCategoryStartup: true,
		},
	}
	notifier := NewFailureNotifier(config)
	
	tests := []struct {
		name         string
		notification FailureNotification
		expectedDecision UserDecision
	}{
		{
			name: "auto bypass enabled",
			notification: FailureNotification{
				ServerName:       "gopls",
				Category:         FailureCategoryStartup,
				Severity:         FailureSeverityMedium,
				BypassAvailable:  true,
				AutoBypassReason: "startup failure",
			},
			expectedDecision: UserDecisionBypass,
		},
		{
			name: "non-critical failure",
			notification: FailureNotification{
				ServerName:      "pylsp",
				Category:        FailureCategoryRuntime,
				Severity:        FailureSeverityMedium,
				BypassAvailable: false,
			},
			expectedDecision: UserDecisionRetry,
		},
		{
			name: "critical failure",
			notification: FailureNotification{
				ServerName:      "jdtls",
				Category:        FailureCategoryTransport,
				Severity:        FailureSeverityCritical,
				BypassAvailable: false,
			},
			expectedDecision: UserDecisionSkip,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := notifier.handleNonInteractiveFailure(tt.notification)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			if response.Decision != tt.expectedDecision {
				t.Errorf("Expected decision %v, got %v", tt.expectedDecision, response.Decision)
			}
		})
	}
	
	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr
	
	// Read and discard output
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
}

func TestHandleNonInteractiveBatch(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	
	config := &NotificationConfig{
		Mode: NotificationModeNonInteractive,
		AutoBypassPolicy: map[FailureCategory]bool{
			FailureCategoryStartup: true,
		},
	}
	notifier := NewFailureNotifier(config)
	
	batch := BatchFailureNotification{
		Failures: []FailureNotification{
			{
				ServerName:       "gopls",
				Category:         FailureCategoryStartup,
				Severity:         FailureSeverityMedium,
				BypassAvailable:  true,
				AutoBypassReason: "startup failure",
			},
			{
				ServerName:      "pylsp",
				Category:        FailureCategoryRuntime,
				Severity:        FailureSeverityMedium,
				BypassAvailable: false,
			},
		},
		TotalCount: 2,
	}
	
	response, err := notifier.handleNonInteractiveBatch(batch)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Should bypass gopls (auto-bypass policy)
	if response.Decision != UserDecisionBatchBypass {
		t.Errorf("Expected decision BatchBypass, got %v", response.Decision)
	}
	
	if len(response.ServerNames) != 1 || response.ServerNames[0] != "gopls" {
		t.Errorf("Expected server names [gopls], got %v", response.ServerNames)
	}
	
	// Close writer and restore stderr
	w.Close()
	os.Stderr = oldStderr
	
	// Read and discard output
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
}

func TestHandleSilentFailure(t *testing.T) {
	config := &NotificationConfig{
		Mode: NotificationModeSilent,
		AutoBypassPolicy: map[FailureCategory]bool{
			FailureCategoryStartup: true,
		},
	}
	notifier := NewFailureNotifier(config)
	
	// Test auto-bypass scenario
	notification := FailureNotification{
		ServerName:       "gopls",
		Category:         FailureCategoryStartup,
		BypassAvailable:  true,
		AutoBypassReason: "startup failure",
	}
	
	response, err := notifier.handleSilentFailure(notification)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if response.Decision != UserDecisionBypass {
		t.Errorf("Expected decision Bypass, got %v", response.Decision)
	}
	
	// Test default retry scenario
	notification.Category = FailureCategoryRuntime
	response, err = notifier.handleSilentFailure(notification)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if response.Decision != UserDecisionRetry {
		t.Errorf("Expected decision Retry, got %v", response.Decision)
	}
}

func TestNotificationModeSelection(t *testing.T) {
	tests := []struct {
		mode         NotificationMode
		expectedFunc string
	}{
		{NotificationModeInteractive, "handleInteractiveFailure"},
		{NotificationModeNonInteractive, "handleNonInteractiveFailure"},
		{NotificationModeJSON, "handleJSONFailure"},
		{NotificationModeSilent, "handleSilentFailure"},
	}
	
	for _, tt := range tests {
		config := &NotificationConfig{
			Mode:               tt.mode,
			InteractivePrompts: false, // Disable prompts for testing
		}
		notifier := NewFailureNotifier(config)
		
		notification := FailureNotification{
			ServerName: "test-server",
			Language:   "test-lang",
			Category:   FailureCategoryStartup,
			Severity:   FailureSeverityMedium,
			Message:    "Test failure",
		}
		
		// Capture outputs to avoid polluting test output
		oldStdout := os.Stdout
		oldStderr := os.Stderr
		r1, w1, _ := os.Pipe()
		r2, w2, _ := os.Pipe()
		os.Stdout = w1
		os.Stderr = w2
		
		response, err := notifier.NotifyFailure(notification)
		
		// Restore outputs
		w1.Close()
		w2.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		
		// Read and discard outputs
		buf1 := new(bytes.Buffer)
		buf2 := new(bytes.Buffer)
		buf1.ReadFrom(r1)
		buf2.ReadFrom(r2)
		
		if err != nil {
			t.Errorf("Mode %v: unexpected error: %v", tt.mode, err)
			continue
		}
		
		if response == nil {
			t.Errorf("Mode %v: expected response, got nil", tt.mode)
			continue
		}
		
		// Verify that each mode returns appropriate responses
		switch tt.mode {
		case NotificationModeJSON:
			if response.Decision != UserDecisionRetry {
				t.Errorf("JSON mode should default to retry, got %v", response.Decision)
			}
		case NotificationModeSilent:
			if response.Decision != UserDecisionRetry {
				t.Errorf("Silent mode should default to retry, got %v", response.Decision)
			}
		case NotificationModeNonInteractive:
			// Should be retry for medium severity
			if response.Decision != UserDecisionRetry {
				t.Errorf("Non-interactive mode should retry for medium severity, got %v", response.Decision)
			}
		}
	}
}