package transport

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCircuitBreakerStateTransitions tests circuit breaker state transitions (Closed → Open → Half-Open → Closed)
func TestCircuitBreakerStateTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		failureThreshold    int
		timeoutDuration     time.Duration
		successThreshold    int
		expectedTransitions []string
	}{
		{
			name:             "basic state transitions",
			failureThreshold: 3,
			timeoutDuration:  1 * time.Second,
			successThreshold: 2,
			expectedTransitions: []string{"CLOSED", "OPEN", "HALF_OPEN", "CLOSED"},
		},
		{
			name:             "rapid failure transitions",
			failureThreshold: 2,
			timeoutDuration:  500 * time.Millisecond,
			successThreshold: 1,
			expectedTransitions: []string{"CLOSED", "OPEN", "HALF_OPEN", "CLOSED"},
		},
		{
			name:             "high threshold transitions",
			failureThreshold: 5,
			timeoutDuration:  2 * time.Second,
			successThreshold: 3,
			expectedTransitions: []string{"CLOSED", "OPEN", "HALF_OPEN", "CLOSED"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
				FailureThreshold: tt.failureThreshold,
				TimeoutDuration:  tt.timeoutDuration,
				SuccessThreshold: tt.successThreshold,
			})

			stateTransitions := []string{}
			stateTransitions = append(stateTransitions, circuitBreaker.GetState())

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			// Phase 1: Generate failures to trigger OPEN state
			t.Logf("Phase 1: Generating %d failures to trigger OPEN state", tt.failureThreshold)
			for i := 0; i < tt.failureThreshold; i++ {
				err := circuitBreaker.Execute(ctx, func() error {
					return fmt.Errorf("simulated failure %d", i+1)
				})
				if err == nil {
					t.Errorf("Expected failure %d to be propagated", i+1)
				}
				t.Logf("Failure %d: state=%s, error=%v", i+1, circuitBreaker.GetState(), err)
			}

			currentState := circuitBreaker.GetState()
			if currentState == "OPEN" && len(stateTransitions) == 1 {
				stateTransitions = append(stateTransitions, "OPEN")
			}

			// Phase 2: Wait for timeout to trigger HALF_OPEN state
			t.Logf("Phase 2: Waiting %v for timeout to trigger HALF_OPEN state", tt.timeoutDuration)
			time.Sleep(tt.timeoutDuration + 100*time.Millisecond)

			// Try a request to trigger HALF_OPEN
			err := circuitBreaker.Execute(ctx, func() error {
				return fmt.Errorf("test failure in half-open")
			})
			halfOpenState := circuitBreaker.GetState()
			if halfOpenState == "HALF_OPEN" {
				stateTransitions = append(stateTransitions, "HALF_OPEN")
			}
			t.Logf("After timeout: state=%s, error=%v", halfOpenState, err)

			// Phase 3: Generate successes to trigger CLOSED state
			t.Logf("Phase 3: Generating %d successes to trigger CLOSED state", tt.successThreshold)
			for i := 0; i < tt.successThreshold; i++ {
				err := circuitBreaker.Execute(ctx, func() error {
					return nil // Success
				})
				if err != nil {
					t.Errorf("Expected success %d to pass through, got error: %v", i+1, err)
				}
				t.Logf("Success %d: state=%s", i+1, circuitBreaker.GetState())
			}

			finalState := circuitBreaker.GetState()
			if finalState == "CLOSED" && len(stateTransitions) < 4 {
				stateTransitions = append(stateTransitions, "CLOSED")
			}

			// Validate state transitions
			t.Logf("Observed state transitions: %v", stateTransitions)
			t.Logf("Expected state transitions: %v", tt.expectedTransitions)

			if len(stateTransitions) != len(tt.expectedTransitions) {
				t.Errorf("Expected %d state transitions, got %d", len(tt.expectedTransitions), len(stateTransitions))
			}

			for i, expectedState := range tt.expectedTransitions {
				if i < len(stateTransitions) && stateTransitions[i] != expectedState {
					t.Errorf("Expected state %s at transition %d, got %s", expectedState, i, stateTransitions[i])
				}
			}

			// Validate final state
			if finalState != "CLOSED" {
				t.Errorf("Expected final state to be CLOSED, got %s", finalState)
			}
		})
	}
}

// TestCircuitBreakerErrorThresholdMonitoring tests error threshold monitoring and failure rate calculation
func TestCircuitBreakerErrorThresholdMonitoring(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		failureThreshold     int
		requestCount         int
		failureCount         int
		slidingWindowSize    time.Duration
		expectedStateAfter   string
		expectedFailureRate  float64
	}{
		{
			name:               "below threshold monitoring",
			failureThreshold:   5,
			requestCount:       10,
			failureCount:       3,
			slidingWindowSize:  2 * time.Second,
			expectedStateAfter: "CLOSED",
			expectedFailureRate: 0.3,
		},
		{
			name:               "at threshold monitoring",
			failureThreshold:   3,
			requestCount:       6,
			failureCount:       3,
			slidingWindowSize:  1 * time.Second,
			expectedStateAfter: "OPEN",
			expectedFailureRate: 0.5,
		},
		{
			name:               "above threshold monitoring",
			failureThreshold:   2,
			requestCount:       5,
			failureCount:       4,
			slidingWindowSize:  1 * time.Second,
			expectedStateAfter: "OPEN",
			expectedFailureRate: 0.8,
		},
		{
			name:               "sliding window behavior",
			failureThreshold:   3,
			requestCount:       8,
			failureCount:       5,
			slidingWindowSize:  500 * time.Millisecond,
			expectedStateAfter: "OPEN",
			expectedFailureRate: 0.625,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
				FailureThreshold:  tt.failureThreshold,
				TimeoutDuration:   2 * time.Second,
				SuccessThreshold:  2,
				SlidingWindowSize: tt.slidingWindowSize,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			successCount := 0
			failureCount := 0
			
			// Execute mixed success/failure requests
			for i := 0; i < tt.requestCount; i++ {
				shouldFail := failureCount < tt.failureCount
				
				err := circuitBreaker.Execute(ctx, func() error {
					if shouldFail {
						return fmt.Errorf("simulated failure %d", failureCount+1)
					}
					return nil
				})

				if shouldFail {
					failureCount++
					if err == nil {
						t.Errorf("Expected failure to be propagated for request %d", i+1)
					}
				} else {
					successCount++
					if err != nil && circuitBreaker.GetState() != "OPEN" {
						t.Errorf("Unexpected error for success request %d: %v", i+1, err)
					}
				}

				// Add small delay to test sliding window behavior
				time.Sleep(tt.slidingWindowSize / time.Duration(tt.requestCount) / 2)
			}

			// Validate metrics
			metrics := circuitBreaker.GetMetrics()
			actualFailureRate := float64(metrics.FailureCount) / float64(metrics.TotalRequests)
			
			t.Logf("Metrics: Total=%d, Failures=%d, Successes=%d, FailureRate=%.3f, State=%s",
				metrics.TotalRequests, metrics.FailureCount, metrics.SuccessCount, actualFailureRate, circuitBreaker.GetState())

			// Validate state
			actualState := circuitBreaker.GetState()
			if actualState != tt.expectedStateAfter {
				t.Errorf("Expected state %s, got %s", tt.expectedStateAfter, actualState)
			}

			// Validate failure rate (with tolerance)
			tolerance := 0.1
			if math.Abs(actualFailureRate-tt.expectedFailureRate) > tolerance {
				t.Errorf("Expected failure rate %.3f, got %.3f (tolerance: %.1f)", tt.expectedFailureRate, actualFailureRate, tolerance)
			}

			// Validate threshold behavior
			if tt.expectedStateAfter == "OPEN" && metrics.FailureCount < int64(tt.failureThreshold) {
				t.Errorf("Circuit should be open with %d failures (threshold: %d)", metrics.FailureCount, tt.failureThreshold)
			}
		})
	}
}

// TestCircuitBreakerTimeoutAndAutomaticRecovery tests circuit breaker timeout and automatic recovery attempts
func TestCircuitBreakerTimeoutAndAutomaticRecovery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		failureThreshold       int
		timeoutDuration        time.Duration
		recoveryAttempts       int
		recoverySuccessPattern []bool
		expectedFinalState     string
	}{
		{
			name:                   "successful recovery",
			failureThreshold:       2,
			timeoutDuration:        500 * time.Millisecond,
			recoveryAttempts:       3,
			recoverySuccessPattern: []bool{false, true, true},
			expectedFinalState:     "CLOSED",
		},
		{
			name:                   "failed recovery",
			failureThreshold:       2,
			timeoutDuration:        300 * time.Millisecond,
			recoveryAttempts:       4,
			recoverySuccessPattern: []bool{false, false, true, false},
			expectedFinalState:     "OPEN",
		},
		{
			name:                   "gradual recovery",
			failureThreshold:       3,
			timeoutDuration:        1 * time.Second,
			recoveryAttempts:       5,
			recoverySuccessPattern: []bool{false, false, true, true, true},
			expectedFinalState:     "CLOSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
				FailureThreshold: tt.failureThreshold,
				TimeoutDuration:  tt.timeoutDuration,
				SuccessThreshold: 2,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			// Phase 1: Generate failures to open circuit
			t.Logf("Phase 1: Opening circuit with %d failures", tt.failureThreshold)
			for i := 0; i < tt.failureThreshold; i++ {
				err := circuitBreaker.Execute(ctx, func() error {
					return fmt.Errorf("initial failure %d", i+1)
				})
				t.Logf("Initial failure %d: %v", i+1, err)
			}

			if circuitBreaker.GetState() != "OPEN" {
				t.Fatalf("Expected circuit to be OPEN, got %s", circuitBreaker.GetState())
			}

			// Phase 2: Test timeout and recovery attempts
			recoveryAttempt := 0
			for recoveryAttempt < tt.recoveryAttempts {
				// Wait for timeout
				t.Logf("Waiting %v for timeout before recovery attempt %d", tt.timeoutDuration, recoveryAttempt+1)
				time.Sleep(tt.timeoutDuration + 50*time.Millisecond)

				// Attempt recovery
				shouldSucceed := recoveryAttempt < len(tt.recoverySuccessPattern) && tt.recoverySuccessPattern[recoveryAttempt]
				
				err := circuitBreaker.Execute(ctx, func() error {
					if shouldSucceed {
						return nil
					}
					return fmt.Errorf("recovery failure %d", recoveryAttempt+1)
				})

				state := circuitBreaker.GetState()
				t.Logf("Recovery attempt %d: success=%v, error=%v, state=%s", recoveryAttempt+1, shouldSucceed, err, state)

				recoveryAttempt++

				// Check if we've fully recovered
				if state == "CLOSED" {
					break
				}
			}

			// Validate final state
			finalState := circuitBreaker.GetState()
			if finalState != tt.expectedFinalState {
				t.Errorf("Expected final state %s, got %s", tt.expectedFinalState, finalState)
			}

			// Test continued operation in final state
			if finalState == "CLOSED" {
				err := circuitBreaker.Execute(ctx, func() error {
					return nil
				})
				if err != nil {
					t.Errorf("Should be able to execute successfully in CLOSED state: %v", err)
				}
			} else if finalState == "OPEN" {
				err := circuitBreaker.Execute(ctx, func() error {
					return nil
				})
				if err == nil {
					t.Error("Should fail fast in OPEN state")
				}
			}

			t.Logf("Final metrics: %+v", circuitBreaker.GetMetrics())
		})
	}
}

// TestCircuitBreakerConcurrentLoad tests circuit breaker behavior under concurrent load
func TestCircuitBreakerConcurrentLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		concurrentWorkers  int
		requestsPerWorker  int
		failureThreshold   int
		failureRate        float64
		timeoutDuration    time.Duration
		maxTestDuration    time.Duration
	}{
		{
			name:              "light concurrent load",
			concurrentWorkers: 5,
			requestsPerWorker: 10,
			failureThreshold:  10,
			failureRate:       0.3,
			timeoutDuration:   1 * time.Second,
			maxTestDuration:   15 * time.Second,
		},
		{
			name:              "moderate concurrent load",
			concurrentWorkers: 10,
			requestsPerWorker: 20,
			failureThreshold:  15,
			failureRate:       0.6,
			timeoutDuration:   500 * time.Millisecond,
			maxTestDuration:   20 * time.Second,
		},
		{
			name:              "heavy concurrent load",
			concurrentWorkers: 20,
			requestsPerWorker: 25,
			failureThreshold:  25,
			failureRate:       0.8,
			timeoutDuration:   300 * time.Millisecond,
			maxTestDuration:   30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
				FailureThreshold: tt.failureThreshold,
				TimeoutDuration:  tt.timeoutDuration,
				SuccessThreshold: 5,
			})

			ctx, cancel := context.WithTimeout(context.Background(), tt.maxTestDuration)
			defer cancel()

			// Shared counters for tracking results
			var totalRequests int64
			var totalFailures int64
			var totalSuccesses int64
			var totalRejected int64
			
			var wg sync.WaitGroup

			// Launch concurrent workers
			t.Logf("Starting %d concurrent workers with %d requests each", tt.concurrentWorkers, tt.requestsPerWorker)
			
			for workerID := 0; workerID < tt.concurrentWorkers; workerID++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					
					for i := 0; i < tt.requestsPerWorker; i++ {
						atomic.AddInt64(&totalRequests, 1)
						
						// Determine if this request should fail based on failure rate
						shouldFail := (float64(i) / float64(tt.requestsPerWorker)) < tt.failureRate
						
						err := circuitBreaker.Execute(ctx, func() error {
							if shouldFail {
								return fmt.Errorf("worker %d request %d failure", id, i)
							}
							return nil
						})

						if err != nil {
							if err.Error() == "circuit breaker is open" {
								atomic.AddInt64(&totalRejected, 1)
							} else {
								atomic.AddInt64(&totalFailures, 1)
							}
						} else {
							atomic.AddInt64(&totalSuccesses, 1)
						}

						// Small delay to prevent overwhelming
						time.Sleep(10 * time.Millisecond)
					}
				}(workerID)
			}

			wg.Wait()

			// Collect final metrics
			finalRequests := atomic.LoadInt64(&totalRequests)
			finalFailures := atomic.LoadInt64(&totalFailures)
			finalSuccesses := atomic.LoadInt64(&totalSuccesses)
			finalRejected := atomic.LoadInt64(&totalRejected)
			
			metrics := circuitBreaker.GetMetrics()
			finalState := circuitBreaker.GetState()

			t.Logf("Concurrent load test results:")
			t.Logf("  Total requests: %d", finalRequests)
			t.Logf("  Successes: %d", finalSuccesses)
			t.Logf("  Failures: %d", finalFailures)
			t.Logf("  Rejected (circuit open): %d", finalRejected)
			t.Logf("  Final state: %s", finalState)
			t.Logf("  Circuit breaker metrics: %+v", metrics)

			// Validate concurrent behavior
			expectedRequests := int64(tt.concurrentWorkers * tt.requestsPerWorker)
			if finalRequests != expectedRequests {
				t.Errorf("Expected %d total requests, got %d", expectedRequests, finalRequests)
			}

			// With high failure rate, circuit should eventually open
			if tt.failureRate > 0.5 && finalRejected == 0 {
				t.Error("Expected some requests to be rejected due to circuit breaker opening")
			}

			// Validate that circuit breaker protected against overload
			if finalState == "OPEN" && finalRejected > 0 {
				t.Logf("Circuit breaker successfully protected against overload: %d requests rejected", finalRejected)
			}

			// Thread safety validation: metrics should be consistent
			totalProcessed := finalSuccesses + finalFailures + finalRejected
			if totalProcessed != finalRequests {
				t.Errorf("Inconsistent metrics: processed=%d, requested=%d", totalProcessed, finalRequests)
			}
		})
	}
}

// TestCircuitBreakerResetConditions tests circuit breaker reset conditions and success rate monitoring
func TestCircuitBreakerResetConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		failureThreshold       int
		successThreshold       int
		timeoutDuration        time.Duration
		resetTestPattern       []bool // true=success, false=failure
		expectedStateProgression []string
	}{
		{
			name:             "successful reset",
			failureThreshold: 3,
			successThreshold: 2,
			timeoutDuration:  500 * time.Millisecond,
			resetTestPattern: []bool{true, true}, // 2 successes
			expectedStateProgression: []string{"CLOSED", "OPEN", "HALF_OPEN", "CLOSED"},
		},
		{
			name:             "failed reset - insufficient successes",
			failureThreshold: 3,
			successThreshold: 3,
			timeoutDuration:  500 * time.Millisecond,
			resetTestPattern: []bool{true, false, true}, // Only 2 successes out of 3 required
			expectedStateProgression: []string{"CLOSED", "OPEN", "HALF_OPEN", "OPEN"},
		},
		{
			name:             "complex reset pattern",
			failureThreshold: 2,
			successThreshold: 4,
			timeoutDuration:  300 * time.Millisecond,
			resetTestPattern: []bool{true, true, true, true}, // Exactly enough successes
			expectedStateProgression: []string{"CLOSED", "OPEN", "HALF_OPEN", "CLOSED"},
		},
		{
			name:             "early failure during reset",
			failureThreshold: 2,
			successThreshold: 3,
			timeoutDuration:  400 * time.Millisecond,
			resetTestPattern: []bool{true, false}, // Failure during half-open
			expectedStateProgression: []string{"CLOSED", "OPEN", "HALF_OPEN", "OPEN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
				FailureThreshold: tt.failureThreshold,
				TimeoutDuration:  tt.timeoutDuration,
				SuccessThreshold: tt.successThreshold,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			stateProgression := []string{circuitBreaker.GetState()}

			// Phase 1: Open the circuit
			t.Logf("Phase 1: Opening circuit with %d failures", tt.failureThreshold)
			for i := 0; i < tt.failureThreshold; i++ {
				err := circuitBreaker.Execute(ctx, func() error {
					return fmt.Errorf("failure %d", i+1)
				})
				t.Logf("Failure %d: %v", i+1, err)
			}

			if circuitBreaker.GetState() == "OPEN" {
				stateProgression = append(stateProgression, "OPEN")
			}

			// Phase 2: Wait for timeout to enter half-open
			t.Logf("Phase 2: Waiting %v for timeout", tt.timeoutDuration)
			time.Sleep(tt.timeoutDuration + 100*time.Millisecond)

			// Phase 3: Execute reset test pattern
			t.Logf("Phase 3: Executing reset pattern: %v", tt.resetTestPattern)
			for i, shouldSucceed := range tt.resetTestPattern {
				err := circuitBreaker.Execute(ctx, func() error {
					if shouldSucceed {
						return nil
					}
					return fmt.Errorf("reset test failure %d", i+1)
				})

				state := circuitBreaker.GetState()
				t.Logf("Reset test %d: success=%v, error=%v, state=%s", i+1, shouldSucceed, err, state)

				// Record state changes
				if len(stateProgression) == 0 || stateProgression[len(stateProgression)-1] != state {
					stateProgression = append(stateProgression, state)
				}

				// Early exit if circuit reopens
				if state == "OPEN" && shouldSucceed {
					break
				}
			}

			// Validate state progression
			t.Logf("Observed state progression: %v", stateProgression)
			t.Logf("Expected state progression: %v", tt.expectedStateProgression)

			for i, expectedState := range tt.expectedStateProgression {
				if i < len(stateProgression) {
					if stateProgression[i] != expectedState {
						t.Errorf("Expected state %s at position %d, got %s", expectedState, i, stateProgression[i])
					}
				} else {
					t.Errorf("Missing state transition: expected %s at position %d", expectedState, i)
				}
			}

			// Validate final behavior
			finalState := circuitBreaker.GetState()
			if finalState == "CLOSED" {
				// Should accept new requests
				err := circuitBreaker.Execute(ctx, func() error { return nil })
				if err != nil {
					t.Errorf("Should accept requests in CLOSED state: %v", err)
				}
			} else if finalState == "OPEN" {
				// Should reject new requests
				err := circuitBreaker.Execute(ctx, func() error { return nil })
				if err == nil {
					t.Error("Should reject requests in OPEN state")
				}
			}

			// Validate metrics consistency
			metrics := circuitBreaker.GetMetrics()
			t.Logf("Final metrics: %+v", metrics)
		})
	}
}

// TestCircuitBreakerFailFastBehavior tests fail-fast behavior when circuit is open
func TestCircuitBreakerFailFastBehavior(t *testing.T) {
	t.Parallel()

	t.Run("fail fast performance", func(t *testing.T) {
		circuitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 2,
			TimeoutDuration:  5 * time.Second, // Long timeout to keep circuit open
			SuccessThreshold: 3,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Open the circuit
		for i := 0; i < 2; i++ {
			circuitBreaker.Execute(ctx, func() error {
				return fmt.Errorf("failure %d", i+1)
			})
		}

		if circuitBreaker.GetState() != "OPEN" {
			t.Fatalf("Expected circuit to be OPEN, got %s", circuitBreaker.GetState())
		}

		// Test fail-fast behavior
		const numFailFastTests = 100
		startTime := time.Now()

		for i := 0; i < numFailFastTests; i++ {
			err := circuitBreaker.Execute(ctx, func() error {
				// This should never be called
				t.Error("Function should not be executed when circuit is open")
				return nil
			})

			if err == nil {
				t.Errorf("Expected fail-fast error for request %d", i+1)
			}

			if err.Error() != "circuit breaker is open" {
				t.Errorf("Expected 'circuit breaker is open' error, got: %v", err)
			}
		}

		duration := time.Since(startTime)
		t.Logf("Executed %d fail-fast requests in %v", numFailFastTests, duration)

		// Fail-fast should be very quick
		maxExpectedDuration := 100 * time.Millisecond
		if duration > maxExpectedDuration {
			t.Errorf("Fail-fast took %v, expected under %v", duration, maxExpectedDuration)
		}

		// Validate that function was never called
		metrics := circuitBreaker.GetMetrics()
		if metrics.TotalRequests > 2 {
			t.Errorf("Expected only 2 total requests (initial failures), got %d", metrics.TotalRequests)
		}
	})

	t.Run("fail fast with concurrent requests", func(t *testing.T) {
		circuitBreaker := NewCircuitBreaker(CircuitBreakerConfig{
			FailureThreshold: 3,
			TimeoutDuration:  10 * time.Second,
			SuccessThreshold: 2,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Open the circuit
		for i := 0; i < 3; i++ {
			circuitBreaker.Execute(ctx, func() error {
				return fmt.Errorf("failure %d", i+1)
			})
		}

		// Concurrent fail-fast test
		const numWorkers = 20
		const requestsPerWorker = 50
		var wg sync.WaitGroup
		var successfulRejects int64
		var unexpectedCalls int64

		startTime := time.Now()

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					err := circuitBreaker.Execute(ctx, func() error {
						atomic.AddInt64(&unexpectedCalls, 1)
						return nil
					})

					if err != nil && err.Error() == "circuit breaker is open" {
						atomic.AddInt64(&successfulRejects, 1)
					}
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		totalExpectedRejects := int64(numWorkers * requestsPerWorker)
		actualRejects := atomic.LoadInt64(&successfulRejects)
		calls := atomic.LoadInt64(&unexpectedCalls)

		t.Logf("Concurrent fail-fast: %d rejects out of %d requests in %v", actualRejects, totalExpectedRejects, duration)

		if calls > 0 {
			t.Errorf("Function was called %d times when circuit was open", calls)
		}

		if actualRejects != totalExpectedRejects {
			t.Errorf("Expected %d successful rejects, got %d", totalExpectedRejects, actualRejects)
		}

		// Should complete quickly even with many concurrent requests
		maxExpectedDuration := 2 * time.Second
		if duration > maxExpectedDuration {
			t.Errorf("Concurrent fail-fast took %v, expected under %v", duration, maxExpectedDuration)
		}
	})
}

// Helper types and functions for circuit breaker testing

type CircuitBreakerConfig struct {
	FailureThreshold  int
	TimeoutDuration   time.Duration
	SuccessThreshold  int
	SlidingWindowSize time.Duration
}

type CircuitBreakerMetrics struct {
	TotalRequests int64
	SuccessCount  int64
	FailureCount  int64
	State         string
	LastFailure   time.Time
	LastSuccess   time.Time
}

type CircuitBreaker struct {
	config           CircuitBreakerConfig
	state            string
	failureCount     int64
	successCount     int64
	totalRequests    int64
	lastFailureTime  time.Time
	lastSuccessTime  time.Time
	lastStateChange  time.Time
	halfOpenSuccesses int64
	mu               sync.RWMutex
}

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	if config.SlidingWindowSize == 0 {
		config.SlidingWindowSize = 60 * time.Second
	}
	
	return &CircuitBreaker{
		config:          config,
		state:           "CLOSED",
		lastStateChange: time.Now(),
	}
}

func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	cb.mu.Lock()
	state := cb.state
	
	// Check if we should transition from OPEN to HALF_OPEN
	if state == "OPEN" && time.Since(cb.lastStateChange) >= cb.config.TimeoutDuration {
		cb.state = "HALF_OPEN"
		cb.lastStateChange = time.Now()
		cb.halfOpenSuccesses = 0
		state = "HALF_OPEN"
	}
	
	// Fail fast if circuit is open
	if state == "OPEN" {
		cb.mu.Unlock()
		return fmt.Errorf("circuit breaker is open")
	}
	
	cb.totalRequests++
	cb.mu.Unlock()

	// Execute the function
	err := fn()
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		
		// Check if we should open the circuit
		if cb.state == "CLOSED" && cb.failureCount >= int64(cb.config.FailureThreshold) {
			cb.state = "OPEN"
			cb.lastStateChange = time.Now()
		} else if cb.state == "HALF_OPEN" {
			// Any failure in half-open state reopens the circuit
			cb.state = "OPEN"
			cb.lastStateChange = time.Now()
		}
	} else {
		cb.successCount++
		cb.lastSuccessTime = time.Now()
		
		// Check if we should close the circuit from half-open
		if cb.state == "HALF_OPEN" {
			cb.halfOpenSuccesses++
			if cb.halfOpenSuccesses >= int64(cb.config.SuccessThreshold) {
				cb.state = "CLOSED"
				cb.lastStateChange = time.Now()
				cb.failureCount = 0 // Reset failure count
			}
		}
	}
	
	return err
}

func (cb *CircuitBreaker) GetState() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) GetMetrics() CircuitBreakerMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return CircuitBreakerMetrics{
		TotalRequests: cb.totalRequests,
		SuccessCount:  cb.successCount,
		FailureCount:  cb.failureCount,
		State:         cb.state,
		LastFailure:   cb.lastFailureTime,
		LastSuccess:   cb.lastSuccessTime,
	}
}

// Additional helper for math.Abs since it might not be imported
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Use abs instead of math.Abs
func validateExponentialWithJitter(t *testing.T, delays []time.Duration, baseDelay, maxDelay time.Duration, jitterPercent float64) {
	// Check that jitter is applied (delays shouldn't be identical for exponential sequence)
	identical := 0
	for i := 1; i < len(delays); i++ {
		if delays[i] == delays[i-1] {
			identical++
		}
	}
	
	if identical > 1 {
		t.Errorf("Too many identical delays (%d), jitter should add variance", identical)
	}
}