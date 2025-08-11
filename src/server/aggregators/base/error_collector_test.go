package base

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewErrorCollector tests the constructor
func TestNewErrorCollector(t *testing.T) {
	collector := NewErrorCollector()

	if collector == nil {
		t.Fatal("NewErrorCollector returned nil")
	}

	if collector.HasErrors() {
		t.Error("New collector should not have errors")
	}

	if collector.GetErrorCount() != 0 {
		t.Errorf("Expected 0 errors, got %d", collector.GetErrorCount())
	}

	errors := collector.GetErrors()
	if errors != nil {
		t.Errorf("Expected nil errors, got %v", errors)
	}
}

// TestErrorCollector_BasicOperations tests basic Add and retrieval operations
func TestErrorCollector_BasicOperations(t *testing.T) {
	collector := NewErrorCollector()

	// Test adding a simple error
	testErr := fmt.Errorf("test error")
	collector.Add("go", testErr)

	if !collector.HasErrors() {
		t.Error("Expected HasErrors to return true")
	}

	if collector.GetErrorCount() != 1 {
		t.Errorf("Expected 1 error, got %d", collector.GetErrorCount())
	}

	errors := collector.GetErrors()
	if len(errors) != 1 {
		t.Errorf("Expected 1 error message, got %d", len(errors))
	}

	expectedMsg := "go: test error"
	if errors[0] != expectedMsg {
		t.Errorf("Expected '%s', got '%s'", expectedMsg, errors[0])
	}

	// Test success count calculation
	successCount := collector.GetSuccessCount(5)
	if successCount != 4 {
		t.Errorf("Expected 4 successes out of 5 total, got %d", successCount)
	}
}

// TestErrorCollector_NilErrorHandling tests that nil errors are ignored
func TestErrorCollector_NilErrorHandling(t *testing.T) {
	collector := NewErrorCollector()

	// Add nil error using Add
	collector.Add("go", nil)

	// Add nil error using AddTyped
	collector.AddTyped("python", nil, ErrorTypeGeneral)

	if collector.HasErrors() {
		t.Error("Collector should not have errors after adding nil errors")
	}

	if collector.GetErrorCount() != 0 {
		t.Errorf("Expected 0 errors, got %d", collector.GetErrorCount())
	}
}

// TestErrorCollector_AutomaticTypeDetection tests automatic error type detection
func TestErrorCollector_AutomaticTypeDetection(t *testing.T) {
	testCases := []struct {
		name         string
		error        error
		expectedType ErrorType
	}{
		// Timeout errors
		{"context.DeadlineExceeded", context.DeadlineExceeded, ErrorTypeTimeout},
		{"timeout message", fmt.Errorf("operation timeout occurred"), ErrorTypeTimeout},
		{"deadline exceeded message", fmt.Errorf("deadline exceeded"), ErrorTypeTimeout},
		{"context deadline exceeded", fmt.Errorf("context deadline exceeded"), ErrorTypeTimeout},
		{"request timeout", fmt.Errorf("request timeout"), ErrorTypeTimeout},

		// Connection errors
		{"connection refused", fmt.Errorf("connection refused"), ErrorTypeConnection},
		{"network unreachable", fmt.Errorf("network unreachable"), ErrorTypeConnection},
		{"dial failed", fmt.Errorf("dial failed"), ErrorTypeConnection},
		{"broken pipe", fmt.Errorf("broken pipe"), ErrorTypeConnection},
		{"no such file", fmt.Errorf("no such file or directory"), ErrorTypeConnection},
		{"process not found", fmt.Errorf("process not found"), ErrorTypeConnection},
		{"executable not found", fmt.Errorf("executable file not found"), ErrorTypeConnection},
		{"connect error", fmt.Errorf("failed to connect"), ErrorTypeConnection},

		// Protocol errors
		{"json parse error", fmt.Errorf("json parse error"), ErrorTypeProtocol},
		{"rpc error", fmt.Errorf("rpc protocol error"), ErrorTypeProtocol},
		{"protocol violation", fmt.Errorf("protocol violation"), ErrorTypeProtocol},
		{"invalid response", fmt.Errorf("invalid response format"), ErrorTypeProtocol},
		{"unmarshal failed", fmt.Errorf("unmarshal failed"), ErrorTypeProtocol},
		{"decode error", fmt.Errorf("decode error"), ErrorTypeProtocol},

		// Unsupported errors
		{"method not found", fmt.Errorf("method not found"), ErrorTypeUnsupported},
		{"not supported", fmt.Errorf("operation not supported"), ErrorTypeUnsupported},
		{"unsupported feature", fmt.Errorf("unsupported feature"), ErrorTypeUnsupported},
		{"not implemented", fmt.Errorf("not implemented"), ErrorTypeUnsupported},
		{"capability missing", fmt.Errorf("capability not available"), ErrorTypeUnsupported},
		{"methodnotfound", fmt.Errorf("methodnotfound"), ErrorTypeUnsupported},

		// General errors (fallback)
		{"generic error", fmt.Errorf("something went wrong"), ErrorTypeGeneral},
		{"database error", fmt.Errorf("database query failed"), ErrorTypeGeneral},
		{"unknown error", fmt.Errorf("unknown error occurred"), ErrorTypeGeneral},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewErrorCollector()
			collector.Add("test", tc.error)

			errors := collector.GetErrorsByType(tc.expectedType)
			if len(errors) != 1 {
				t.Errorf("Expected 1 error of type %s, got %d", tc.expectedType, len(errors))

				// Debug: show what type was actually detected
				allErrors := collector.GetErrorsByType(ErrorTypeTimeout)
				allErrors = append(allErrors, collector.GetErrorsByType(ErrorTypeConnection)...)
				allErrors = append(allErrors, collector.GetErrorsByType(ErrorTypeProtocol)...)
				allErrors = append(allErrors, collector.GetErrorsByType(ErrorTypeUnsupported)...)
				allErrors = append(allErrors, collector.GetErrorsByType(ErrorTypeGeneral)...)

				if len(allErrors) > 0 {
					t.Errorf("Error was classified as type %s instead", allErrors[0].ErrorType)
				}
				return
			}

			if errors[0].ErrorType != tc.expectedType {
				t.Errorf("Expected error type %s, got %s", tc.expectedType, errors[0].ErrorType)
			}

			if errors[0].Error.Error() != tc.error.Error() {
				t.Errorf("Expected error message '%s', got '%s'", tc.error.Error(), errors[0].Error.Error())
			}

			if errors[0].Language != "test" {
				t.Errorf("Expected language 'test', got '%s'", errors[0].Language)
			}
		})
	}
}

// TestErrorCollector_AddTyped tests explicit error type specification
func TestErrorCollector_AddTyped(t *testing.T) {
	collector := NewErrorCollector()

	// Add error with explicit type that differs from what would be auto-detected
	timeoutErr := fmt.Errorf("general error message")
	collector.AddTyped("go", timeoutErr, ErrorTypeTimeout)

	// Verify it uses the explicit type, not auto-detection
	timeoutErrors := collector.GetErrorsByType(ErrorTypeTimeout)
	if len(timeoutErrors) != 1 {
		t.Errorf("Expected 1 timeout error, got %d", len(timeoutErrors))
	}

	generalErrors := collector.GetErrorsByType(ErrorTypeGeneral)
	if len(generalErrors) != 0 {
		t.Errorf("Expected 0 general errors, got %d", len(generalErrors))
	}
}

// TestErrorCollector_ThreadSafety tests concurrent access patterns
func TestErrorCollector_ThreadSafety(t *testing.T) {
	collector := NewErrorCollector()
	languages := []string{"go", "python", "typescript", "java", "rust"}
	errorTypes := []ErrorType{
		ErrorTypeTimeout, ErrorTypeConnection, ErrorTypeProtocol,
		ErrorTypeUnsupported, ErrorTypeGeneral,
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	errorsPerGoroutine := 10

	// Launch multiple goroutines adding errors
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < errorsPerGoroutine; j++ {
				language := languages[j%len(languages)]
				errorType := errorTypes[j%len(errorTypes)]
				err := fmt.Errorf("error %d-%d", goroutineID, j)

				if j%2 == 0 {
					collector.Add(language, err)
				} else {
					collector.AddTyped(language, err, errorType)
				}
			}
		}(i)
	}

	// Launch goroutines reading data concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				_ = collector.HasErrors()
				_ = collector.GetErrorCount()
				_ = collector.GetErrors()
				_ = collector.GetLanguagesWithErrors()
				_ = collector.GetErrorSummary()
				_ = collector.GetSuccessCount(100)

				// Read by type
				for _, errorType := range errorTypes {
					_ = collector.GetErrorsByType(errorType)
				}

				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	// Verify final state
	expectedTotal := numGoroutines * errorsPerGoroutine
	actualCount := collector.GetErrorCount()

	if actualCount != expectedTotal {
		t.Errorf("Expected %d total errors, got %d", expectedTotal, actualCount)
	}

	if !collector.HasErrors() {
		t.Error("Expected collector to have errors after concurrent operations")
	}

	// Verify all languages are present
	languagesWithErrors := collector.GetLanguagesWithErrors()
	if len(languagesWithErrors) != len(languages) {
		t.Errorf("Expected %d languages with errors, got %d", len(languages), len(languagesWithErrors))
	}
}

// TestErrorCollector_ErrorRetrieval tests various error retrieval methods
func TestErrorCollector_ErrorRetrieval(t *testing.T) {
	collector := NewErrorCollector()

	// Add multiple errors with different types and languages
	collector.AddTyped("go", fmt.Errorf("timeout in go"), ErrorTypeTimeout)
	collector.AddTyped("python", fmt.Errorf("connection failed"), ErrorTypeConnection)
	collector.AddTyped("go", fmt.Errorf("protocol error"), ErrorTypeProtocol)
	collector.AddTyped("java", fmt.Errorf("not supported"), ErrorTypeUnsupported)
	collector.AddTyped("rust", fmt.Errorf("general error"), ErrorTypeGeneral)

	// Test GetErrors
	errors := collector.GetErrors()
	if len(errors) != 5 {
		t.Errorf("Expected 5 error messages, got %d", len(errors))
	}

	// Verify formatted messages
	expectedMessages := []string{
		"go: timeout in go",
		"python: connection failed",
		"go: protocol error",
		"java: not supported",
		"rust: general error",
	}

	for i, expected := range expectedMessages {
		if errors[i] != expected {
			t.Errorf("Expected message '%s', got '%s'", expected, errors[i])
		}
	}

	// Test GetErrorsByType
	timeoutErrors := collector.GetErrorsByType(ErrorTypeTimeout)
	if len(timeoutErrors) != 1 {
		t.Errorf("Expected 1 timeout error, got %d", len(timeoutErrors))
	}
	if timeoutErrors[0].Language != "go" {
		t.Errorf("Expected timeout error from 'go', got '%s'", timeoutErrors[0].Language)
	}

	// Test GetLanguagesWithErrors
	languages := collector.GetLanguagesWithErrors()
	expectedLanguages := []string{"go", "python", "java", "rust"}
	sort.Strings(languages)
	sort.Strings(expectedLanguages)

	if len(languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(languages))
	}

	for i, expected := range expectedLanguages {
		if languages[i] != expected {
			t.Errorf("Expected language '%s', got '%s'", expected, languages[i])
		}
	}

	// Test GetErrorCount and GetSuccessCount
	if collector.GetErrorCount() != 5 {
		t.Errorf("Expected 5 errors, got %d", collector.GetErrorCount())
	}

	if collector.GetSuccessCount(10) != 5 {
		t.Errorf("Expected 5 successes out of 10, got %d", collector.GetSuccessCount(10))
	}

	// Test HasErrors
	if !collector.HasErrors() {
		t.Error("Expected HasErrors to return true")
	}
}

// TestErrorCollector_ErrorSummary tests error summary generation
func TestErrorCollector_ErrorSummary(t *testing.T) {
	collector := NewErrorCollector()

	// Test empty collector
	summary := collector.GetErrorSummary()
	if summary != "No errors" {
		t.Errorf("Expected 'No errors', got '%s'", summary)
	}

	// Add errors of different types
	collector.AddTyped("go", fmt.Errorf("timeout error"), ErrorTypeTimeout)
	collector.AddTyped("python", fmt.Errorf("connection error"), ErrorTypeConnection)
	collector.AddTyped("java", fmt.Errorf("another timeout"), ErrorTypeTimeout)
	collector.AddTyped("rust", fmt.Errorf("protocol issue"), ErrorTypeProtocol)

	summary = collector.GetErrorSummary()

	// Verify summary contains error type counts and messages
	if !strings.Contains(summary, "timeout (2)") {
		t.Errorf("Expected summary to contain 'timeout (2)', got: %s", summary)
	}

	if !strings.Contains(summary, "connection (1)") {
		t.Errorf("Expected summary to contain 'connection (1)', got: %s", summary)
	}

	if !strings.Contains(summary, "protocol (1)") {
		t.Errorf("Expected summary to contain 'protocol (1)', got: %s", summary)
	}

	// Verify it contains specific error messages
	if !strings.Contains(summary, "go: timeout error") {
		t.Errorf("Expected summary to contain 'go: timeout error', got: %s", summary)
	}

	if !strings.Contains(summary, "python: connection error") {
		t.Errorf("Expected summary to contain 'python: connection error', got: %s", summary)
	}
}

// TestErrorCollector_LanguageContext tests language-specific error handling
func TestErrorCollector_LanguageContext(t *testing.T) {
	collector := NewErrorCollector()

	// Add multiple errors for the same language
	collector.Add("go", fmt.Errorf("first error"))
	collector.Add("go", fmt.Errorf("second error"))
	collector.Add("python", fmt.Errorf("python error"))

	// Test language retrieval
	languages := collector.GetLanguagesWithErrors()
	sort.Strings(languages)

	expectedLanguages := []string{"go", "python"}
	if len(languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(languages))
	}

	for i, expected := range expectedLanguages {
		if languages[i] != expected {
			t.Errorf("Expected language '%s', got '%s'", expected, languages[i])
		}
	}

	// Verify error messages contain correct language context
	errors := collector.GetErrors()
	if len(errors) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(errors))
	}

	// Check that errors are properly associated with languages
	goErrors := 0
	pythonErrors := 0
	for _, errMsg := range errors {
		if strings.HasPrefix(errMsg, "go:") {
			goErrors++
		} else if strings.HasPrefix(errMsg, "python:") {
			pythonErrors++
		}
	}

	if goErrors != 2 {
		t.Errorf("Expected 2 Go errors, got %d", goErrors)
	}

	if pythonErrors != 1 {
		t.Errorf("Expected 1 Python error, got %d", pythonErrors)
	}
}

// TestErrorCollector_TimestampHandling tests that timestamps are recorded
func TestErrorCollector_TimestampHandling(t *testing.T) {
	collector := NewErrorCollector()

	before := time.Now()
	collector.Add("go", fmt.Errorf("test error"))
	after := time.Now()

	errors := collector.GetErrorsByType(ErrorTypeGeneral)
	if len(errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(errors))
	}

	timestamp := errors[0].Timestamp
	if timestamp.Before(before) || timestamp.After(after) {
		t.Errorf("Timestamp %v should be between %v and %v", timestamp, before, after)
	}

	// Test timestamp ordering in concurrent scenarios
	collector.Clear()
	var wg sync.WaitGroup
	timestamps := make([]time.Time, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			time.Sleep(time.Microsecond * time.Duration(index)) // Small delay variation
			collector.AddTyped("test", fmt.Errorf("error %d", index), ErrorTypeGeneral)
		}(i)
	}

	wg.Wait()

	allErrors := collector.GetErrorsByType(ErrorTypeGeneral)
	if len(allErrors) != 100 {
		t.Errorf("Expected 100 errors, got %d", len(allErrors))
	}

	// Verify all timestamps are present and reasonable
	for i, langErr := range allErrors {
		if langErr.Timestamp.IsZero() {
			t.Errorf("Error %d has zero timestamp", i)
		}
		timestamps[i] = langErr.Timestamp
	}
}

// TestErrorCollector_ClearOperations tests the Clear method
func TestErrorCollector_ClearOperations(t *testing.T) {
	collector := NewErrorCollector()

	// Add some errors
	collector.Add("go", fmt.Errorf("error 1"))
	collector.Add("python", fmt.Errorf("error 2"))
	collector.Add("java", fmt.Errorf("error 3"))

	// Verify errors are present
	if !collector.HasErrors() {
		t.Error("Expected collector to have errors before clear")
	}

	if collector.GetErrorCount() != 3 {
		t.Errorf("Expected 3 errors before clear, got %d", collector.GetErrorCount())
	}

	// Clear errors
	collector.Clear()

	// Verify state is reset
	if collector.HasErrors() {
		t.Error("Expected collector to have no errors after clear")
	}

	if collector.GetErrorCount() != 0 {
		t.Errorf("Expected 0 errors after clear, got %d", collector.GetErrorCount())
	}

	errors := collector.GetErrors()
	if errors != nil {
		t.Errorf("Expected nil errors after clear, got %v", errors)
	}

	languages := collector.GetLanguagesWithErrors()
	if len(languages) != 0 {
		t.Errorf("Expected 0 languages after clear, got %d", len(languages))
	}

	summary := collector.GetErrorSummary()
	if summary != "No errors" {
		t.Errorf("Expected 'No errors' after clear, got '%s'", summary)
	}

	// Test that collector can be used normally after clear
	collector.Add("rust", fmt.Errorf("new error after clear"))

	if !collector.HasErrors() {
		t.Error("Expected collector to have errors after adding post-clear")
	}

	if collector.GetErrorCount() != 1 {
		t.Errorf("Expected 1 error after post-clear add, got %d", collector.GetErrorCount())
	}
}

// TestErrorCollector_EdgeCases tests various edge cases
func TestErrorCollector_EdgeCases(t *testing.T) {
	collector := NewErrorCollector()

	// Test empty language string
	collector.Add("", fmt.Errorf("error with empty language"))
	errors := collector.GetErrors()
	if len(errors) != 1 {
		t.Errorf("Expected 1 error with empty language, got %d", len(errors))
	}
	if errors[0] != ": error with empty language" {
		t.Errorf("Expected ': error with empty language', got '%s'", errors[0])
	}

	// Test very long error messages
	longMessage := strings.Repeat("very long error message ", 100)
	collector.Clear()
	collector.Add("test", fmt.Errorf("%s", longMessage))

	errors = collector.GetErrors()
	if len(errors) != 1 {
		t.Errorf("Expected 1 error with long message, got %d", len(errors))
	}
	if !strings.Contains(errors[0], longMessage) {
		t.Error("Long error message should be preserved")
	}

	// Test special characters in language names
	collector.Clear()
	specialLangs := []string{"go-lang", "c++", "c#", "f#", "objective-c"}
	for _, lang := range specialLangs {
		collector.Add(lang, fmt.Errorf("error in %s", lang))
	}

	languages := collector.GetLanguagesWithErrors()
	if len(languages) != len(specialLangs) {
		t.Errorf("Expected %d languages with special characters, got %d", len(specialLangs), len(languages))
	}

	// Test filtering by non-existent error type
	nonExistentErrors := collector.GetErrorsByType("nonexistent")
	if len(nonExistentErrors) != 0 {
		t.Errorf("Expected 0 errors for non-existent type, got %d", len(nonExistentErrors))
	}

	// Test GetSuccessCount with zero total
	successCount := collector.GetSuccessCount(0)
	expectedSuccessCount := 0 - len(specialLangs) // Should be negative
	if successCount != expectedSuccessCount {
		t.Errorf("Expected %d successes with 0 total, got %d", expectedSuccessCount, successCount)
	}
}

// TestErrorCollector_LargeVolume tests behavior with large numbers of errors
func TestErrorCollector_LargeVolume(t *testing.T) {
	collector := NewErrorCollector()

	// Add a large number of errors
	numErrors := 10000
	languages := []string{"go", "python", "java", "typescript", "rust", "c++", "c#", "php"}

	for i := 0; i < numErrors; i++ {
		lang := languages[i%len(languages)]
		err := fmt.Errorf("error number %d", i)
		collector.Add(lang, err)
	}

	// Test basic operations still work efficiently
	start := time.Now()

	count := collector.GetErrorCount()
	if count != numErrors {
		t.Errorf("Expected %d errors, got %d", numErrors, count)
	}

	hasErrors := collector.HasErrors()
	if !hasErrors {
		t.Error("Expected to have errors with large volume")
	}

	langs := collector.GetLanguagesWithErrors()
	if len(langs) != len(languages) {
		t.Errorf("Expected %d unique languages, got %d", len(languages), len(langs))
	}

	_ = collector.GetErrorSummary() // Should not panic or hang

	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Errorf("Large volume operations took too long: %v", elapsed)
	}
}

// BenchmarkErrorCollector_Add benchmarks the Add operation
func BenchmarkErrorCollector_Add(b *testing.B) {
	collector := NewErrorCollector()
	err := fmt.Errorf("benchmark error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.Add("go", err)
	}
}

// BenchmarkErrorCollector_ConcurrentAdd benchmarks concurrent Add operations
func BenchmarkErrorCollector_ConcurrentAdd(b *testing.B) {
	collector := NewErrorCollector()
	err := fmt.Errorf("benchmark error")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.Add("go", err)
		}
	})
}

// BenchmarkErrorCollector_GetErrors benchmarks GetErrors operation
func BenchmarkErrorCollector_GetErrors(b *testing.B) {
	collector := NewErrorCollector()

	// Pre-populate with errors
	for i := 0; i < 1000; i++ {
		collector.Add("go", fmt.Errorf("error %d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collector.GetErrors()
	}
}
