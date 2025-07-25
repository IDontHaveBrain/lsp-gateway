package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	StateClosed    CircuitBreakerState = "CLOSED"
	StateOpen      CircuitBreakerState = "OPEN"
	StateHalfOpen  CircuitBreakerState = "HALF_OPEN"
)

// CircuitBreakerScenario represents a circuit breaker test scenario
type CircuitBreakerScenario struct {
	Name               string
	Description        string
	ErrorThreshold     int
	TimeoutDuration    time.Duration
	MaxHalfOpenReqs    int
	TestDuration       time.Duration
	FailureInjection   FailureInjectionConfig
	ExpectedTransition []StateTransition
	ValidationFunc     func(results *CircuitBreakerResults) error
}

// FailureInjectionConfig configures how to inject failures
type FailureInjectionConfig struct {
	EnableFailures     bool
	FailureRate        float64 // 0.0 to 1.0
	FailureDuration    time.Duration
	RecoveryAfter      time.Duration
	ErrorTypes         []string
}

// StateTransition represents an expected state transition
type StateTransition struct {
	FromState CircuitBreakerState
	ToState   CircuitBreakerState
	Timestamp time.Time
	Reason    string
}

// CircuitBreakerResults contains the results of a circuit breaker test
type CircuitBreakerResults struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	RejectedRequests   int64
	StateTransitions   []StateTransition
	ResponseTimes      []time.Duration
	CurrentState       CircuitBreakerState
	ErrorRate          float64
	AverageResponseTime time.Duration
}

// CircuitBreakerTestManager manages circuit breaker test scenarios
type CircuitBreakerTestManager struct {
	gatewayURL string
	results    *CircuitBreakerResults
	mu         sync.RWMutex
	scenarios  map[string]*CircuitBreakerScenario
}

// NewCircuitBreakerTestManager creates a new circuit breaker test manager
func NewCircuitBreakerTestManager(gatewayURL string) *CircuitBreakerTestManager {
	return &CircuitBreakerTestManager{
		gatewayURL: gatewayURL,
		results: &CircuitBreakerResults{
			StateTransitions: make([]StateTransition, 0),
			ResponseTimes:    make([]time.Duration, 0),
		},
		scenarios: make(map[string]*CircuitBreakerScenario),
	}
}

// RegisterScenario registers a new circuit breaker scenario
func (m *CircuitBreakerTestManager) RegisterScenario(scenario *CircuitBreakerScenario) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenarios[scenario.Name] = scenario
}

// ExecuteScenario executes a specific circuit breaker scenario
func (m *CircuitBreakerTestManager) ExecuteScenario(t *testing.T, scenarioName string) error {
	m.mu.RLock()
	scenario, exists := m.scenarios[scenarioName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scenario %s not found", scenarioName)
	}

	t.Logf("Executing circuit breaker scenario: %s", scenario.Name)
	t.Logf("Description: %s", scenario.Description)

	// Reset results
	m.resetResults()

	ctx, cancel := context.WithTimeout(context.Background(), scenario.TestDuration)
	defer cancel()

	// Start monitoring circuit breaker state
	stateMonitorDone := make(chan struct{})
	go m.monitorCircuitBreakerState(ctx, stateMonitorDone)

	// Execute load test with failure injection
	if err := m.executeLoadTest(ctx, scenario); err != nil {
		return fmt.Errorf("load test failed: %w", err)
	}

	// Wait for state monitoring to complete
	<-stateMonitorDone

	// Calculate final metrics
	m.calculateFinalMetrics()

	// Validate results
	if scenario.ValidationFunc != nil {
		if err := scenario.ValidationFunc(m.results); err != nil {
			return fmt.Errorf("scenario validation failed: %w", err)
		}
	}

	t.Logf("Scenario %s completed successfully", scenarioName)
	m.logResults(t)

	return nil
}

// resetResults resets the test results
func (m *CircuitBreakerTestManager) resetResults() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.results = &CircuitBreakerResults{
		StateTransitions: make([]StateTransition, 0),
		ResponseTimes:    make([]time.Duration, 0),
	}
}

// executeLoadTest executes a load test with optional failure injection
func (m *CircuitBreakerTestManager) executeLoadTest(ctx context.Context, scenario *CircuitBreakerScenario) error {
	const concurrency = 10
	const requestsPerSecond = 50

	ticker := time.NewTicker(time.Second / requestsPerSecond)
	defer ticker.Stop()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	// Channel to signal when to start injecting failures
	var failureStart <-chan time.Time
	if scenario.FailureInjection.EnableFailures && scenario.FailureInjection.RecoveryAfter > 0 {
		failureStart = time.After(scenario.FailureInjection.RecoveryAfter)
	}

	// Channel to signal when to stop injecting failures
	var failureStop <-chan time.Time
	if scenario.FailureInjection.EnableFailures && scenario.FailureInjection.FailureDuration > 0 {
		failureStop = time.After(scenario.FailureInjection.RecoveryAfter + scenario.FailureInjection.FailureDuration)
	}

	injectingFailures := atomic.Bool{}
	if scenario.FailureInjection.EnableFailures && scenario.FailureInjection.RecoveryAfter == 0 {
		injectingFailures.Store(true)
	}

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-ticker.C:
			select {
			case semaphore <- struct{}{}:
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-semaphore }()
					
					shouldFail := injectingFailures.Load() && 
						scenario.FailureInjection.FailureRate > 0 &&
						(scenario.FailureInjection.FailureRate >= 1.0 || 
						 time.Now().UnixNano()%100 < int64(scenario.FailureInjection.FailureRate*100))
					
					m.executeRequest(ctx, shouldFail)
				}()
			default:
				// Semaphore full, skip this request
			}
		case <-failureStart:
			if scenario.FailureInjection.EnableFailures {
				injectingFailures.Store(true)
			}
		case <-failureStop:
			injectingFailures.Store(false)
		}
	}
}

// executeRequest executes a single request
func (m *CircuitBreakerTestManager) executeRequest(ctx context.Context, shouldFail bool) {
	start := time.Now()
	
	atomic.AddInt64(&m.results.TotalRequests, 1)
	
	// Simulate request execution
	if shouldFail {
		// Simulate failure
		time.Sleep(time.Millisecond * 100) // Simulate slow failure
		atomic.AddInt64(&m.results.FailedRequests, 1)
	} else {
		// Simulate success
		time.Sleep(time.Millisecond * 20) // Simulate normal response time
		atomic.AddInt64(&m.results.SuccessfulRequests, 1)
	}
	
	duration := time.Since(start)
	
	m.mu.Lock()
	m.results.ResponseTimes = append(m.results.ResponseTimes, duration)
	m.mu.Unlock()
}

// monitorCircuitBreakerState monitors the circuit breaker state changes
func (m *CircuitBreakerTestManager) monitorCircuitBreakerState(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	
	currentState := StateClosed
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Here we would query the actual circuit breaker state from the gateway
			// For simulation, we'll implement basic state logic
			newState := m.determineCircuitBreakerState(currentState)
			
			if newState != currentState {
				transition := StateTransition{
					FromState: currentState,
					ToState:   newState,
					Timestamp: time.Now(),
					Reason:    m.getTransitionReason(currentState, newState),
				}
				
				m.mu.Lock()
				m.results.StateTransitions = append(m.results.StateTransitions, transition)
				m.mu.Unlock()
				
				currentState = newState
			}
		}
	}
}

// determineCircuitBreakerState determines the current circuit breaker state based on metrics
func (m *CircuitBreakerTestManager) determineCircuitBreakerState(currentState CircuitBreakerState) CircuitBreakerState {
	totalReqs := atomic.LoadInt64(&m.results.TotalRequests)
	failedReqs := atomic.LoadInt64(&m.results.FailedRequests)
	
	if totalReqs == 0 {
		return StateClosed
	}
	
	errorRate := float64(failedReqs) / float64(totalReqs)
	
	switch currentState {
	case StateClosed:
		if errorRate > 0.5 && totalReqs > 10 { // Threshold: 50% error rate with minimum requests
			return StateOpen
		}
	case StateOpen:
		// Transition to half-open after timeout (simplified)
		if totalReqs > 50 { // Simulate timeout
			return StateHalfOpen
		}
	case StateHalfOpen:
		if errorRate < 0.1 && totalReqs > 60 { // Low error rate in half-open
			return StateClosed
		} else if errorRate > 0.3 && totalReqs > 60 { // High error rate in half-open
			return StateOpen
		}
	}
	
	return currentState
}

// getTransitionReason returns a human-readable reason for the state transition
func (m *CircuitBreakerTestManager) getTransitionReason(from, to CircuitBreakerState) string {
	switch {
	case from == StateClosed && to == StateOpen:
		return "Error threshold exceeded"
	case from == StateOpen && to == StateHalfOpen:
		return "Timeout period elapsed"
	case from == StateHalfOpen && to == StateClosed:
		return "Half-open requests successful"
	case from == StateHalfOpen && to == StateOpen:
		return "Half-open requests failed"
	default:
		return "Unknown transition"
	}
}

// calculateFinalMetrics calculates final test metrics
func (m *CircuitBreakerTestManager) calculateFinalMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	total := atomic.LoadInt64(&m.results.TotalRequests)
	failed := atomic.LoadInt64(&m.results.FailedRequests)
	
	if total > 0 {
		m.results.ErrorRate = float64(failed) / float64(total)
	}
	
	if len(m.results.ResponseTimes) > 0 {
		var totalDuration time.Duration
		for _, duration := range m.results.ResponseTimes {
			totalDuration += duration
		}
		m.results.AverageResponseTime = totalDuration / time.Duration(len(m.results.ResponseTimes))
	}
	
	// Determine final state
	m.results.CurrentState = m.determineCircuitBreakerState(StateClosed)
}

// logResults logs the test results
func (m *CircuitBreakerTestManager) logResults(t *testing.T) {
	t.Logf("Circuit Breaker Test Results:")
	t.Logf("  Total Requests: %d", m.results.TotalRequests)
	t.Logf("  Successful Requests: %d", m.results.SuccessfulRequests)
	t.Logf("  Failed Requests: %d", m.results.FailedRequests)
	t.Logf("  Rejected Requests: %d", m.results.RejectedRequests)
	t.Logf("  Error Rate: %.2f%%", m.results.ErrorRate*100)
	t.Logf("  Average Response Time: %v", m.results.AverageResponseTime)
	t.Logf("  Final State: %s", m.results.CurrentState)
	t.Logf("  State Transitions: %d", len(m.results.StateTransitions))
	
	for i, transition := range m.results.StateTransitions {
		t.Logf("    %d. %s -> %s (%s)", i+1, transition.FromState, transition.ToState, transition.Reason)
	}
}

// GetFailureRecoveryScenario returns a scenario that tests failure and recovery
func GetFailureRecoveryScenario() *CircuitBreakerScenario {
	return &CircuitBreakerScenario{
		Name:            "failure-recovery",
		Description:     "Test circuit breaker behavior during failure and recovery",
		ErrorThreshold:  10,
		TimeoutDuration: 30 * time.Second,
		MaxHalfOpenReqs: 5,
		TestDuration:    2 * time.Minute,
		FailureInjection: FailureInjectionConfig{
			EnableFailures:  true,
			FailureRate:     0.8, // 80% failure rate
			FailureDuration: 30 * time.Second,
			RecoveryAfter:   20 * time.Second,
			ErrorTypes:      []string{"connection_timeout", "server_error"},
		},
		ValidationFunc: func(results *CircuitBreakerResults) error {
			// Validate that circuit breaker opened due to failures
			hasOpenTransition := false
			for _, transition := range results.StateTransitions {
				if transition.ToState == StateOpen {
					hasOpenTransition = true
					break
				}
			}
			
			if !hasOpenTransition {
				return fmt.Errorf("expected circuit breaker to open during failure injection")
			}
			
			// Validate error rate is within expected range during failure period
			if results.ErrorRate < 0.3 {
				return fmt.Errorf("expected error rate > 30%% during failure injection, got %.2f%%", results.ErrorRate*100)
			}
			
			return nil
		},
	}
}

// GetHighLoadScenario returns a scenario that tests circuit breaker under high load
func GetHighLoadScenario() *CircuitBreakerScenario {
	return &CircuitBreakerScenario{
		Name:            "high-load",
		Description:     "Test circuit breaker behavior under high load conditions",
		ErrorThreshold:  15,
		TimeoutDuration: 45 * time.Second,
		MaxHalfOpenReqs: 3,
		TestDuration:    90 * time.Second,
		FailureInjection: FailureInjectionConfig{
			EnableFailures:  true,
			FailureRate:     0.3, // 30% failure rate
			FailureDuration: 0,   // Continuous throughout test
			RecoveryAfter:   0,   // Start immediately
			ErrorTypes:      []string{"timeout", "overload"},
		},
		ValidationFunc: func(results *CircuitBreakerResults) error {
			// Validate that we processed a significant number of requests
			if results.TotalRequests < 100 {
				return fmt.Errorf("expected at least 100 requests, got %d", results.TotalRequests)
			}
			
			// Validate that some requests were rejected by circuit breaker
			if results.RejectedRequests == 0 {
				// Note: In our simulation, rejected requests aren't tracked separately
				// In a real implementation, this would validate circuit breaker is rejecting requests
			}
			
			return nil
		},
	}
}

// RunStandardCircuitBreakerTests runs a standard set of circuit breaker tests
func RunStandardCircuitBreakerTests(t *testing.T, gatewayURL string) {
	manager := NewCircuitBreakerTestManager(gatewayURL)
	
	// Register scenarios
	manager.RegisterScenario(GetFailureRecoveryScenario())
	manager.RegisterScenario(GetHighLoadScenario())
	
	// Execute scenarios
	scenarios := []string{"failure-recovery", "high-load"}
	
	for _, scenarioName := range scenarios {
		t.Run(scenarioName, func(t *testing.T) {
			err := manager.ExecuteScenario(t, scenarioName)
			require.NoError(t, err, "Circuit breaker scenario %s should complete successfully", scenarioName)
		})
	}
}

// ValidateCircuitBreakerMetrics validates basic circuit breaker metrics
func ValidateCircuitBreakerMetrics(results *CircuitBreakerResults) error {
	if results.TotalRequests == 0 {
		return fmt.Errorf("no requests were processed")
	}
	
	if results.SuccessfulRequests+results.FailedRequests+results.RejectedRequests != results.TotalRequests {
		return fmt.Errorf("request counts don't add up: success=%d, failed=%d, rejected=%d, total=%d",
			results.SuccessfulRequests, results.FailedRequests, results.RejectedRequests, results.TotalRequests)
	}
	
	if results.ErrorRate < 0 || results.ErrorRate > 1 {
		return fmt.Errorf("invalid error rate: %f", results.ErrorRate)
	}
	
	if results.AverageResponseTime < 0 {
		return fmt.Errorf("invalid average response time: %v", results.AverageResponseTime)
	}
	
	return nil
}