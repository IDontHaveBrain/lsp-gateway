package base

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lsp-gateway/src/internal/constants"
)

// withCIEnvironment simulates different CI environments for testing
func withCIEnvironment(ciType string, testFunc func()) {
	// Save original environment
	origCI := os.Getenv("CI")
	origGitHubActions := os.Getenv("GITHUB_ACTIONS")

	defer func() {
		if origCI == "" {
			os.Unsetenv("CI")
		} else {
			os.Setenv("CI", origCI)
		}
		if origGitHubActions == "" {
			os.Unsetenv("GITHUB_ACTIONS")
		} else {
			os.Setenv("GITHUB_ACTIONS", origGitHubActions)
		}
	}()

	switch ciType {
	case "regular":
		os.Setenv("CI", "true")
		os.Unsetenv("GITHUB_ACTIONS")
	case "windows":
		os.Setenv("CI", "true")
		os.Setenv("GITHUB_ACTIONS", "true")
	case "none":
		os.Unsetenv("CI")
		os.Unsetenv("GITHUB_ACTIONS")
	}

	testFunc()
}

func TestNewTimeoutManager(t *testing.T) {
	tm := NewTimeoutManager()

	if tm.operationType != OperationRequest {
		t.Errorf("Expected default operation type to be %s, got %s", OperationRequest, tm.operationType)
	}

	if tm.globalMultiplier != 1.0 {
		t.Errorf("Expected default global multiplier to be 1.0, got %f", tm.globalMultiplier)
	}

	if tm.customTimeouts == nil {
		t.Error("Expected customTimeouts map to be initialized")
	}

	if len(tm.customTimeouts) != 0 {
		t.Errorf("Expected empty customTimeouts map, got %d entries", len(tm.customTimeouts))
	}
}

func TestTimeoutManager_BuilderPattern(t *testing.T) {
	tm := NewTimeoutManager()

	// Test method chaining
	result := tm.ForOperation(OperationInitialize).
		WithCustomTimeout("go", 5*time.Second).
		WithGlobalMultiplier(2.0)

	if result != tm {
		t.Error("Builder pattern should return same instance for method chaining")
	}

	// Verify state changes
	if tm.operationType != OperationInitialize {
		t.Errorf("Expected operation type %s, got %s", OperationInitialize, tm.operationType)
	}

	if tm.globalMultiplier != 2.0 {
		t.Errorf("Expected global multiplier 2.0, got %f", tm.globalMultiplier)
	}

	if tm.customTimeouts["go"] != 5*time.Second {
		t.Errorf("Expected custom timeout 5s for go, got %v", tm.customTimeouts["go"])
	}
}

func TestTimeoutManager_ForOperation(t *testing.T) {
	tm := NewTimeoutManager()

	tests := []OperationType{
		OperationInitialize,
		OperationRequest,
		OperationShutdown,
	}

	for _, opType := range tests {
		tm.ForOperation(opType)
		if tm.operationType != opType {
			t.Errorf("Expected operation type %s, got %s", opType, tm.operationType)
		}
	}
}

func TestTimeoutManager_WithCustomTimeout(t *testing.T) {
	tm := NewTimeoutManager()

	testCases := []struct {
		language string
		timeout  time.Duration
	}{
		{"go", 10 * time.Second},
		{"python", 20 * time.Second},
		{"java", 60 * time.Second},
		{"unknown", 5 * time.Second},
	}

	for _, tc := range testCases {
		tm.WithCustomTimeout(tc.language, tc.timeout)
		if tm.customTimeouts[tc.language] != tc.timeout {
			t.Errorf("Expected custom timeout %v for %s, got %v", tc.timeout, tc.language, tm.customTimeouts[tc.language])
		}
	}
}

func TestTimeoutManager_WithGlobalMultiplier(t *testing.T) {
	tm := NewTimeoutManager()

	testMultipliers := []float64{0.5, 1.0, 1.5, 2.0, 5.0}

	for _, multiplier := range testMultipliers {
		tm.WithGlobalMultiplier(multiplier)
		if tm.globalMultiplier != multiplier {
			t.Errorf("Expected global multiplier %f, got %f", multiplier, tm.globalMultiplier)
		}
	}
}

func TestTimeoutManager_GetTimeout_SupportedLanguages(t *testing.T) {
	tm := NewTimeoutManager().ForOperation(OperationRequest)

	testCases := []struct {
		language        string
		expectedTimeout time.Duration
	}{
		{"go", 15 * time.Second},
		{"javascript", 15 * time.Second},
		{"typescript", 15 * time.Second},
		{"rust", 15 * time.Second},
		{"python", 30 * time.Second},
		{"java", 90 * time.Second},
	}

	for _, tc := range testCases {
		timeout := tm.GetTimeout(tc.language)
		if timeout != tc.expectedTimeout {
			t.Errorf("Expected %v timeout for %s, got %v", tc.expectedTimeout, tc.language, timeout)
		}
	}
}

func TestTimeoutManager_GetTimeout_OperationTypes(t *testing.T) {
	testCases := []struct {
		operation OperationType
		language  string
		expected  time.Duration
	}{
		{OperationInitialize, "go", 15 * time.Second},
		{OperationInitialize, "python", 45 * time.Second}, // basedpyright has 45s init timeout
		{OperationInitialize, "java", 90 * time.Second},
		{OperationRequest, "go", 15 * time.Second},
		{OperationRequest, "python", 30 * time.Second},
		{OperationRequest, "java", 90 * time.Second},
		{OperationShutdown, "go", constants.ProcessShutdownTimeout},
		{OperationShutdown, "python", constants.ProcessShutdownTimeout},
		{OperationShutdown, "java", constants.ProcessShutdownTimeout},
	}

	for _, tc := range testCases {
		tm := NewTimeoutManager().ForOperation(tc.operation)
		timeout := tm.GetTimeout(tc.language)
		if timeout != tc.expected {
			t.Errorf("Expected %v timeout for %s/%s, got %v", tc.expected, tc.operation, tc.language, timeout)
		}
	}
}

func TestTimeoutManager_GetTimeout_UnknownOperation(t *testing.T) {
	tm := NewTimeoutManager()
	tm.operationType = OperationType("unknown")

	// Should fall back to request timeout for unknown operation
	timeout := tm.GetTimeout("go")
	expected := 15 * time.Second // Go request timeout
	if timeout != expected {
		t.Errorf("Expected fallback to request timeout %v for unknown operation, got %v", expected, timeout)
	}
}

func TestTimeoutManager_GetTimeout_UnknownLanguage(t *testing.T) {
	tm := NewTimeoutManager().ForOperation(OperationRequest)

	// Should fall back to default timeout for unknown language
	timeout := tm.GetTimeout("unknown")
	expected := constants.DefaultRequestTimeout
	if timeout != expected {
		t.Errorf("Expected default request timeout %v for unknown language, got %v", expected, timeout)
	}

	// Test with initialize operation
	tm.ForOperation(OperationInitialize)
	timeout = tm.GetTimeout("unknown")
	expected = constants.DefaultInitializeTimeout
	if timeout != expected {
		t.Errorf("Expected default initialize timeout %v for unknown language, got %v", expected, timeout)
	}
}

func TestTimeoutManager_GetTimeout_CustomTimeouts(t *testing.T) {
	tm := NewTimeoutManager().
		WithCustomTimeout("go", 25*time.Second).
		WithCustomTimeout("python", 45*time.Second)

	// Custom timeouts should override language defaults
	if timeout := tm.GetTimeout("go"); timeout != 25*time.Second {
		t.Errorf("Expected custom timeout 25s for go, got %v", timeout)
	}

	if timeout := tm.GetTimeout("python"); timeout != 45*time.Second {
		t.Errorf("Expected custom timeout 45s for python, got %v", timeout)
	}

	// Non-custom language should use default
	if timeout := tm.GetTimeout("java"); timeout != 90*time.Second {
		t.Errorf("Expected default timeout 90s for java, got %v", timeout)
	}
}

func TestTimeoutManager_GetTimeout_GlobalMultiplier(t *testing.T) {
	testCases := []struct {
		multiplier  float64
		language    string
		baseTimeout time.Duration
	}{
		{0.5, "go", 15 * time.Second},
		{2.0, "python", 30 * time.Second},
		{1.5, "java", 90 * time.Second},
	}

	for _, tc := range testCases {
		tm := NewTimeoutManager().WithGlobalMultiplier(tc.multiplier)
		timeout := tm.GetTimeout(tc.language)
		expected := time.Duration(float64(tc.baseTimeout) * tc.multiplier)
		if timeout != expected {
			t.Errorf("Expected timeout %v with multiplier %f for %s, got %v", expected, tc.multiplier, tc.language, timeout)
		}
	}
}

func TestTimeoutManager_GetTimeout_CustomTimeoutWithMultiplier(t *testing.T) {
	tm := NewTimeoutManager().
		WithCustomTimeout("go", 10*time.Second).
		WithGlobalMultiplier(3.0)

	timeout := tm.GetTimeout("go")
	expected := 30 * time.Second // 10s * 3.0
	if timeout != expected {
		t.Errorf("Expected custom timeout with multiplier 30s, got %v", timeout)
	}
}

func TestTimeoutManager_GetTimeout_ZeroMultiplier(t *testing.T) {
	tm := NewTimeoutManager().WithGlobalMultiplier(0.0)

	timeout := tm.GetTimeout("go")
	if timeout != 0 {
		t.Errorf("Expected zero timeout with zero multiplier, got %v", timeout)
	}
}

func TestTimeoutManager_CreateContext(t *testing.T) {
	tm := NewTimeoutManager()
	parent := context.Background()

	ctx, cancel := tm.CreateContext(parent, "go")
	defer cancel()

	if ctx == nil {
		t.Error("Expected non-nil context")
	}

	// Verify the context has the correct timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected context to have deadline")
	}

	expectedTimeout := tm.GetTimeout("go")
	actualDuration := time.Until(deadline)

	// Allow small tolerance for execution time
	tolerance := 100 * time.Millisecond
	if actualDuration < expectedTimeout-tolerance || actualDuration > expectedTimeout+tolerance {
		t.Errorf("Expected timeout around %v, got %v", expectedTimeout, actualDuration)
	}
}

func TestTimeoutManager_CreateContext_NilParent(t *testing.T) {
	tm := NewTimeoutManager()

	ctx, cancel := tm.CreateContext(nil, "go")
	defer cancel()

	if ctx == nil {
		t.Error("Expected non-nil context even with nil parent")
	}

	_, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected context to have deadline")
	}
}

func TestTimeoutManager_GetOverallTimeout(t *testing.T) {
	tm := NewTimeoutManager().ForOperation(OperationRequest)

	testCases := []struct {
		name      string
		languages []string
		expected  time.Duration
	}{
		{
			name:      "single language",
			languages: []string{"go"},
			expected:  15 * time.Second,
		},
		{
			name:      "multiple languages - max timeout",
			languages: []string{"go", "python", "java"},
			expected:  90 * time.Second, // Java has the highest timeout
		},
		{
			name:      "mixed fast languages",
			languages: []string{"go", "typescript", "rust"},
			expected:  15 * time.Second,
		},
		{
			name:      "python and java",
			languages: []string{"python", "java"},
			expected:  90 * time.Second, // Java wins
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := tm.GetOverallTimeout(tc.languages)
			if timeout != tc.expected {
				t.Errorf("Expected overall timeout %v for languages %v, got %v", tc.expected, tc.languages, timeout)
			}
		})
	}
}

func TestTimeoutManager_GetOverallTimeout_EmptyLanguages(t *testing.T) {
	testCases := []struct {
		operation OperationType
		expected  time.Duration
	}{
		{OperationRequest, constants.DefaultRequestTimeout},
		{OperationInitialize, constants.DefaultInitializeTimeout},
		{OperationShutdown, constants.ProcessShutdownTimeout},
	}

	for _, tc := range testCases {
		tm := NewTimeoutManager().ForOperation(tc.operation)
		timeout := tm.GetOverallTimeout([]string{})
		if timeout != tc.expected {
			t.Errorf("Expected default timeout %v for empty languages with %s operation, got %v", tc.expected, tc.operation, timeout)
		}
	}
}

func TestTimeoutManager_GetOverallTimeout_UnknownLanguages(t *testing.T) {
	tm := NewTimeoutManager().ForOperation(OperationRequest)

	// All unknown languages should fall back to default
	timeout := tm.GetOverallTimeout([]string{"unknown1", "unknown2"})
	expected := constants.DefaultRequestTimeout
	if timeout != expected {
		t.Errorf("Expected default timeout %v for unknown languages, got %v", expected, timeout)
	}
}

func TestTimeoutManager_GetOverallTimeout_WithMultiplier(t *testing.T) {
	tm := NewTimeoutManager().
		ForOperation(OperationRequest).
		WithGlobalMultiplier(2.0)

	languages := []string{"go", "java"}
	timeout := tm.GetOverallTimeout(languages)
	expected := 180 * time.Second // Java 90s * 2.0 multiplier
	if timeout != expected {
		t.Errorf("Expected overall timeout with multiplier %v, got %v", expected, timeout)
	}
}

func TestTimeoutManager_CreateOverallContext(t *testing.T) {
	tm := NewTimeoutManager()
	parent := context.Background()
	languages := []string{"go", "python", "java"}

	ctx, cancel := tm.CreateOverallContext(parent, languages)
	defer cancel()

	if ctx == nil {
		t.Error("Expected non-nil context")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected context to have deadline")
	}

	expectedTimeout := tm.GetOverallTimeout(languages)
	actualDuration := time.Until(deadline)

	// Allow tolerance for execution time
	tolerance := 100 * time.Millisecond
	if actualDuration < expectedTimeout-tolerance || actualDuration > expectedTimeout+tolerance {
		t.Errorf("Expected timeout around %v, got %v", expectedTimeout, actualDuration)
	}
}

func TestTimeoutManager_LogTimeoutInfo(t *testing.T) {
	// This test mainly ensures the method doesn't panic
	// Actual log output verification would require more complex setup
	tm := NewTimeoutManager()

	// Should not panic for any language
	languages := []string{"go", "python", "java", "unknown"}
	for _, lang := range languages {
		tm.LogTimeoutInfo(lang) // Should not panic
	}
}

func TestTimeoutManager_GetSupportedLanguages(t *testing.T) {
	tm := NewTimeoutManager()
	languages := tm.GetSupportedLanguages()

	expected := []string{"java", "python", "go", "javascript", "typescript", "rust", "csharp"}
	if len(languages) != len(expected) {
		t.Errorf("Expected %d languages, got %d", len(expected), len(languages))
	}

	// Convert to map for easier checking
	langMap := make(map[string]bool)
	for _, lang := range languages {
		langMap[lang] = true
	}

	for _, expectedLang := range expected {
		if !langMap[expectedLang] {
			t.Errorf("Expected language %s not found in supported languages", expectedLang)
		}
	}
}

func TestTimeoutManager_EdgeCases(t *testing.T) {
	t.Run("negative multiplier", func(t *testing.T) {
		tm := NewTimeoutManager().WithGlobalMultiplier(-1.0)
		timeout := tm.GetTimeout("go")
		// Negative multiplier should result in negative duration
		if timeout >= 0 {
			t.Errorf("Expected negative timeout with negative multiplier, got %v", timeout)
		}
	})

	t.Run("very large multiplier", func(t *testing.T) {
		tm := NewTimeoutManager().WithGlobalMultiplier(1000.0)
		timeout := tm.GetTimeout("go")
		expected := 15000 * time.Second // 15s * 1000
		if timeout != expected {
			t.Errorf("Expected very large timeout %v, got %v", expected, timeout)
		}
	})

	t.Run("empty string language", func(t *testing.T) {
		tm := NewTimeoutManager()
		timeout := tm.GetTimeout("")
		// Should fall back to default
		if timeout != constants.DefaultRequestTimeout {
			t.Errorf("Expected default timeout for empty language, got %v", timeout)
		}
	})
}

func TestTimeoutManager_OperationConstants(t *testing.T) {
	// Verify the operation constants are properly defined
	if OperationInitialize != "initialize" {
		t.Errorf("Expected OperationInitialize to be 'initialize', got %s", OperationInitialize)
	}
	if OperationRequest != "request" {
		t.Errorf("Expected OperationRequest to be 'request', got %s", OperationRequest)
	}
	if OperationShutdown != "shutdown" {
		t.Errorf("Expected OperationShutdown to be 'shutdown', got %s", OperationShutdown)
	}
}

func TestTimeoutManager_IntegrationWithConstants(t *testing.T) {
	// Test that TimeoutManager properly integrates with constants package
	tm := NewTimeoutManager().ForOperation(OperationRequest)

	// Test that timeouts match what constants package returns
	for _, lang := range []string{"go", "python", "java", "javascript", "typescript", "rust"} {
		tmTimeout := tm.GetTimeout(lang)

		// Remove CI multipliers from constants for fair comparison
		// We'll test non-CI environment by temporarily unsetting CI vars
		oldCI := os.Getenv("CI")
		oldGitHubActions := os.Getenv("GITHUB_ACTIONS")
		os.Unsetenv("CI")
		os.Unsetenv("GITHUB_ACTIONS")

		constantsTimeoutNonCI := constants.GetRequestTimeout(lang)

		// Restore original environment
		if oldCI != "" {
			os.Setenv("CI", oldCI)
		}
		if oldGitHubActions != "" {
			os.Setenv("GITHUB_ACTIONS", oldGitHubActions)
		}

		if tmTimeout != constantsTimeoutNonCI {
			t.Errorf("TimeoutManager timeout %v doesn't match constants timeout %v for language %s", tmTimeout, constantsTimeoutNonCI, lang)
		}
	}
}

func TestTimeoutManager_ConcurrentAccess(t *testing.T) {
	// Test that TimeoutManager can handle concurrent access safely
	tm := NewTimeoutManager()

	done := make(chan bool, 3)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			_ = tm.GetTimeout("go")
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			tm.WithCustomTimeout("test", time.Duration(i)*time.Second)
		}
		done <- true
	}()

	// Concurrent multiplier changes
	go func() {
		for i := 0; i < 100; i++ {
			tm.WithGlobalMultiplier(float64(i))
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should not crash - if we get here, the test passed
}

func TestTimeoutManager_ContextCancellation(t *testing.T) {
	tm := NewTimeoutManager().WithCustomTimeout("test", 50*time.Millisecond)

	ctx, cancel := tm.CreateContext(context.Background(), "test")

	// Context should be canceled after timeout
	select {
	case <-ctx.Done():
		// Expected - context should be canceled due to timeout
		if ctx.Err() != context.DeadlineExceeded {
			t.Errorf("Expected DeadlineExceeded error, got %v", ctx.Err())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should have been canceled by timeout")
	}

	cancel() // Clean up
}

func TestTimeoutManager_ChainedOperations(t *testing.T) {
	// Test complex chained operations
	result := NewTimeoutManager().
		ForOperation(OperationInitialize).
		WithCustomTimeout("go", 20*time.Second).
		WithCustomTimeout("python", 40*time.Second).
		WithGlobalMultiplier(1.5).
		ForOperation(OperationRequest). // Should override the initialize operation
		WithGlobalMultiplier(2.0)       // Should override the 1.5 multiplier

	// Verify final state
	if result.operationType != OperationRequest {
		t.Errorf("Expected final operation type %s, got %s", OperationRequest, result.operationType)
	}

	if result.globalMultiplier != 2.0 {
		t.Errorf("Expected final multiplier 2.0, got %f", result.globalMultiplier)
	}

	// Custom timeouts should be preserved
	goTimeout := result.GetTimeout("go")
	expected := 40 * time.Second // 20s * 2.0 multiplier
	if goTimeout != expected {
		t.Errorf("Expected go timeout %v, got %v", expected, goTimeout)
	}
}

// CI Environment Tests - Critical Missing Coverage
func TestGetTimeout_RegularCI(t *testing.T) {
	withCIEnvironment("regular", func() {
		tests := []struct {
			name     string
			language string
			opType   OperationType
			base     time.Duration
			expected time.Duration
		}{
			{"Java Request CI", "java", OperationRequest, 90 * time.Second, time.Duration(float64(90*time.Second) * 1.2)},
			{"Python Request CI", "python", OperationRequest, 30 * time.Second, time.Duration(float64(30*time.Second) * 1.2)},
			{"Go Request CI", "go", OperationRequest, 15 * time.Second, time.Duration(float64(15*time.Second) * 1.2)},
			{"JavaScript Request CI", "javascript", OperationRequest, 15 * time.Second, time.Duration(float64(15*time.Second) * 1.2)},
			{"TypeScript Request CI", "typescript", OperationRequest, 15 * time.Second, time.Duration(float64(15*time.Second) * 1.2)},
			{"Rust Request CI", "rust", OperationRequest, 15 * time.Second, time.Duration(float64(15*time.Second) * 1.2)},
			{"Java Initialize CI", "java", OperationInitialize, 90 * time.Second, time.Duration(float64(90*time.Second) * 1.2)},
			{"Python Initialize CI", "python", OperationInitialize, 45 * time.Second, time.Duration(float64(45*time.Second) * 1.2)},
			{"Go Initialize CI", "go", OperationInitialize, 15 * time.Second, time.Duration(float64(15*time.Second) * 1.2)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tm := NewTimeoutManager().ForOperation(tt.opType)
				timeout := tm.GetTimeout(tt.language)
				assert.Equal(t, tt.expected, timeout, "CI should apply 1.2x multiplier for %s %s", tt.language, tt.opType)
			})
		}
	})
}

func TestGetTimeout_WindowsCI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows CI test only runs on Windows")
	}

	withCIEnvironment("windows", func() {
		tests := []struct {
			name       string
			language   string
			opType     OperationType
			base       time.Duration
			multiplier float64
		}{
			// Java gets 2x multiplier on Windows CI
			{"Java Request Windows CI", "java", OperationRequest, 90 * time.Second, 2.0},
			{"Java Initialize Windows CI", "java", OperationInitialize, 90 * time.Second, 2.0},
			// All other languages get 1.5x multiplier on Windows CI
			{"Python Request Windows CI", "python", OperationRequest, 30 * time.Second, 1.5},
			{"Python Initialize Windows CI", "python", OperationInitialize, 30 * time.Second, 1.5},
			{"Go Request Windows CI", "go", OperationRequest, 15 * time.Second, 1.5},
			{"Go Initialize Windows CI", "go", OperationInitialize, 15 * time.Second, 1.5},
			{"JavaScript Request Windows CI", "javascript", OperationRequest, 15 * time.Second, 1.5},
			{"TypeScript Request Windows CI", "typescript", OperationRequest, 15 * time.Second, 1.5},
			{"Rust Request Windows CI", "rust", OperationRequest, 15 * time.Second, 1.5},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tm := NewTimeoutManager().ForOperation(tt.opType)
				timeout := tm.GetTimeout(tt.language)

				var expected time.Duration
				if tt.multiplier == 2.0 {
					expected = tt.base * 2
				} else {
					expected = time.Duration(float64(tt.base) * tt.multiplier)
				}

				assert.Equal(t, expected, timeout,
					"Windows CI should apply %vx multiplier for %s %s",
					tt.multiplier, tt.language, tt.opType)
			})
		}
	})
}

func TestGetTimeout_NonWindowsCI_RegularCI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Non-Windows CI test skipped on Windows")
	}

	withCIEnvironment("windows", func() {
		// On non-Windows platforms, even with GITHUB_ACTIONS=true,
		// should behave like regular CI (1.2x multiplier)
		tm := NewTimeoutManager().ForOperation(OperationRequest)

		// Java should get 1.2x multiplier (not 2x since not actually Windows)
		javaTimeout := tm.GetTimeout("java")
		expected := time.Duration(float64(90*time.Second) * 1.2)
		assert.Equal(t, expected, javaTimeout, "Non-Windows with GITHUB_ACTIONS should use regular CI multiplier")

		// Python should get 1.2x multiplier (not 1.5x since not actually Windows)
		pythonTimeout := tm.GetTimeout("python")
		expected = time.Duration(float64(30*time.Second) * 1.2)
		assert.Equal(t, expected, pythonTimeout, "Non-Windows with GITHUB_ACTIONS should use regular CI multiplier")
	})
}

func TestGetTimeout_NoCIEnvironmentExplicit(t *testing.T) {
	withCIEnvironment("none", func() {
		tm := NewTimeoutManager().ForOperation(OperationRequest)

		// No multipliers should be applied in non-CI environment
		tests := []struct {
			language string
			expected time.Duration
		}{
			{"java", 90 * time.Second},
			{"python", 30 * time.Second},
			{"go", 15 * time.Second},
			{"javascript", 15 * time.Second},
			{"typescript", 15 * time.Second},
			{"rust", 15 * time.Second},
		}

		for _, tt := range tests {
			t.Run(tt.language+" no CI", func(t *testing.T) {
				timeout := tm.GetTimeout(tt.language)
				assert.Equal(t, tt.expected, timeout, "No CI environment should use base timeouts")
			})
		}
	})
}

func TestGetTimeout_CICombinedWithGlobalMultiplier(t *testing.T) {
	withCIEnvironment("regular", func() {
		// CI multiplier (1.2x) combined with global multiplier (2.0x)
		// The CI multiplier is applied first by constants.GetRequestTimeout()
		// Then global multiplier is applied by TimeoutManager
		tm := NewTimeoutManager().
			ForOperation(OperationRequest).
			WithGlobalMultiplier(2.0)

		// Java: base 90s * CI(1.2) * global(2.0) = 216s
		timeout := tm.GetTimeout("java")
		ciAppliedTimeout := time.Duration(float64(90*time.Second) * 1.2)
		expected := time.Duration(float64(ciAppliedTimeout) * 2.0)
		assert.Equal(t, expected, timeout, "CI and global multipliers should combine")

		// Python: base 30s * CI(1.2) * global(2.0) = 72s
		timeout = tm.GetTimeout("python")
		ciAppliedTimeout = time.Duration(float64(30*time.Second) * 1.2)
		expected = time.Duration(float64(ciAppliedTimeout) * 2.0)
		assert.Equal(t, expected, timeout)
	})
}

func TestGetTimeout_CustomTimeout_NotAffectedByCI(t *testing.T) {
	withCIEnvironment("regular", func() {
		tm := NewTimeoutManager().
			WithCustomTimeout("java", 60*time.Second)

		// Custom timeout should bypass CI multipliers entirely
		// Only global multiplier should apply (which is 1.0 by default)
		timeout := tm.GetTimeout("java")
		assert.Equal(t, 60*time.Second, timeout, "Custom timeout should not be affected by CI multipliers")
	})

	withCIEnvironment("windows", func() {
		if runtime.GOOS == "windows" {
			tm := NewTimeoutManager().
				WithCustomTimeout("java", 60*time.Second)

			// Even on Windows CI, custom timeout should bypass CI multipliers
			timeout := tm.GetTimeout("java")
			assert.Equal(t, 60*time.Second, timeout, "Custom timeout should not be affected by Windows CI multipliers")
		}
	})
}

func TestGetOverallTimeout_WithCI(t *testing.T) {
	withCIEnvironment("regular", func() {
		tm := NewTimeoutManager().ForOperation(OperationRequest)

		languages := []string{"go", "python", "java"}
		timeout := tm.GetOverallTimeout(languages)

		// Should use Java's CI-adjusted timeout as maximum
		// Java: 90s * 1.2 CI multiplier = 108s
		expected := time.Duration(float64(90*time.Second) * 1.2)
		assert.Equal(t, expected, timeout, "Overall timeout should use max CI-adjusted timeout")
	})
}

func TestGetOverallTimeout_WindowsCI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows CI test only runs on Windows")
	}

	withCIEnvironment("windows", func() {
		tm := NewTimeoutManager().ForOperation(OperationRequest)

		languages := []string{"go", "python", "java"}
		timeout := tm.GetOverallTimeout(languages)

		// Should use Java's Windows CI timeout as maximum
		// Java: 90s * 2.0 Windows CI multiplier = 180s
		expected := 180 * time.Second
		assert.Equal(t, expected, timeout, "Overall timeout should use max Windows CI-adjusted timeout")
	})
}

func TestCreateContext_WithCI(t *testing.T) {
	withCIEnvironment("regular", func() {
		tm := NewTimeoutManager().ForOperation(OperationRequest)

		ctx, cancel := tm.CreateContext(context.Background(), "java")
		defer cancel()

		// Check that context has deadline with CI-adjusted timeout
		deadline, hasDeadline := ctx.Deadline()
		require.True(t, hasDeadline, "Context should have deadline")

		// Should use Java's CI-adjusted timeout (90s * 1.2 = 108s)
		expectedTimeout := time.Duration(float64(90*time.Second) * 1.2)
		actualDuration := time.Until(deadline)

		// Allow tolerance for execution time
		tolerance := 100 * time.Millisecond
		assert.True(t,
			actualDuration >= expectedTimeout-tolerance && actualDuration <= expectedTimeout+tolerance,
			"Context deadline should reflect CI-adjusted timeout, expected ~%v, got %v",
			expectedTimeout, actualDuration)
	})
}

func TestCI_EnvironmentDetection_EdgeCases(t *testing.T) {
	// Test various CI environment variable combinations
	tests := []struct {
		name               string
		ciValue            string
		githubActionsValue string
		expectedCI         bool
		expectedWindowsCI  bool
	}{
		{"CI=true only", "true", "", true, false},
		{"CI=false with GITHUB_ACTIONS", "false", "true", true, false}, // GITHUB_ACTIONS=true implies CI
		{"CI=1 (truthy)", "1", "", false, false},                       // Only "true" is accepted
		{"CI=yes (not exactly true)", "yes", "", false, false},
		{"Empty values", "", "", false, false},
		{"GITHUB_ACTIONS=true only", "", "true", true, runtime.GOOS == "windows"},
		{"Both true", "true", "true", true, runtime.GOOS == "windows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment
			origCI := os.Getenv("CI")
			origGitHubActions := os.Getenv("GITHUB_ACTIONS")

			defer func() {
				if origCI == "" {
					os.Unsetenv("CI")
				} else {
					os.Setenv("CI", origCI)
				}
				if origGitHubActions == "" {
					os.Unsetenv("GITHUB_ACTIONS")
				} else {
					os.Setenv("GITHUB_ACTIONS", origGitHubActions)
				}
			}()

			// Set test environment
			if tt.ciValue == "" {
				os.Unsetenv("CI")
			} else {
				os.Setenv("CI", tt.ciValue)
			}
			if tt.githubActionsValue == "" {
				os.Unsetenv("GITHUB_ACTIONS")
			} else {
				os.Setenv("GITHUB_ACTIONS", tt.githubActionsValue)
			}

			tm := NewTimeoutManager().ForOperation(OperationRequest)
			javaTimeout := tm.GetTimeout("java")

			if !tt.expectedCI {
				// No CI - should use base timeout
				assert.Equal(t, 90*time.Second, javaTimeout, "Should use base timeout when not CI")
			} else if tt.expectedWindowsCI {
				// Windows CI - Java gets 2x multiplier
				assert.Equal(t, 180*time.Second, javaTimeout, "Should use Windows CI multiplier")
			} else {
				// Regular CI - Java gets 1.2x multiplier
				expected := time.Duration(float64(90*time.Second) * 1.2)
				assert.Equal(t, expected, javaTimeout, "Should use regular CI multiplier")
			}
		})
	}
}

// Benchmark CI overhead
func BenchmarkGetTimeout_NoCI(b *testing.B) {
	withCIEnvironment("none", func() {
		tm := NewTimeoutManager().ForOperation(OperationRequest)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tm.GetTimeout("java")
		}
	})
}

func BenchmarkGetTimeout_WithCI(b *testing.B) {
	withCIEnvironment("regular", func() {
		tm := NewTimeoutManager().ForOperation(OperationRequest)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tm.GetTimeout("java")
		}
	})
}
