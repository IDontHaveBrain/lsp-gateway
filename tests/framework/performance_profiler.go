package framework

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// PerformanceProfiler tracks and measures performance metrics during testing
type PerformanceProfiler struct {
	// State
	isActive       bool
	operations     map[string]*ActiveOperation
	baselineMemory int64
	startTime      time.Time

	// Configuration
	sampleInterval time.Duration
	maxOperations  int

	// Metrics tracking
	totalOperations  int64
	operationCounter int64

	// Call tracking
	StartCalls          []context.Context
	StopCalls           []interface{}
	StartOperationCalls []string
	EndOperationCalls   []string

	// Function fields for behavior customization
	StartFunc          func(ctx context.Context) error
	StopFunc           func() error
	StartOperationFunc func(operationName string) (*PerformanceMetrics, error)
	EndOperationFunc   func(operationID string) (*PerformanceMetrics, error)

	mu sync.RWMutex
}

// ActiveOperation tracks an ongoing performance measurement
type ActiveOperation struct {
	ID          string
	Name        string
	StartTime   time.Time
	StartMemory int64
	Samples     []PerformanceSample
}

// PerformanceSample represents a point-in-time performance measurement
type PerformanceSample struct {
	Timestamp      time.Time
	MemoryUsed     int64
	GoroutineCount int
}

// NewPerformanceProfiler creates a new performance profiler
func NewPerformanceProfiler() *PerformanceProfiler {
	return &PerformanceProfiler{
		operations:     make(map[string]*ActiveOperation),
		sampleInterval: time.Millisecond * 100,
		maxOperations:  100,

		// Initialize call tracking
		StartCalls:          make([]context.Context, 0),
		StopCalls:           make([]interface{}, 0),
		StartOperationCalls: make([]string, 0),
		EndOperationCalls:   make([]string, 0),
	}
}

// Start begins performance profiling
func (p *PerformanceProfiler) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.StartCalls = append(p.StartCalls, ctx)

	if p.StartFunc != nil {
		return p.StartFunc(ctx)
	}

	if p.isActive {
		return fmt.Errorf("performance profiler is already active")
	}

	// Capture baseline metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	p.baselineMemory = int64(m.Alloc)
	p.startTime = time.Now()
	p.isActive = true

	return nil
}

// Stop ends performance profiling
func (p *PerformanceProfiler) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.StopCalls = append(p.StopCalls, struct{}{})

	if p.StopFunc != nil {
		return p.StopFunc()
	}

	if !p.isActive {
		return nil // Already stopped
	}

	// End all active operations
	for id := range p.operations {
		p.endOperationInternal(id)
	}

	p.isActive = false

	return nil
}

// StartOperation begins tracking a specific operation
func (p *PerformanceProfiler) StartOperation(operationName string) (*PerformanceMetrics, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.StartOperationCalls = append(p.StartOperationCalls, operationName)

	if p.StartOperationFunc != nil {
		return p.StartOperationFunc(operationName)
	}

	if !p.isActive {
		return nil, fmt.Errorf("profiler is not active")
	}

	if len(p.operations) >= p.maxOperations {
		return nil, fmt.Errorf("maximum operations limit reached")
	}

	// Generate unique operation ID
	p.operationCounter++
	operationID := fmt.Sprintf("%s-%d", operationName, p.operationCounter)

	// Capture starting metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	operation := &ActiveOperation{
		ID:          operationID,
		Name:        operationName,
		StartTime:   time.Now(),
		StartMemory: int64(m.Alloc),
		Samples:     make([]PerformanceSample, 0),
	}

	p.operations[operationID] = operation
	p.totalOperations++

	// Return initial metrics
	return &PerformanceMetrics{
		OperationID:        operationID,
		OperationDuration:  0,
		MemoryAllocated:    int64(m.Alloc),
		MemoryFreed:        0,
		GoroutineCount:     runtime.NumGoroutine(),
		FileOperations:     0,
		NetworkRequests:    0,
		CacheOperations:    0,
		ServerCreations:    0,
		ServerDestructions: 0,
		ErrorCount:         0,
		WarningCount:       0,
	}, nil
}

// EndOperation completes tracking for a specific operation
func (p *PerformanceProfiler) EndOperation(operationID string) (*PerformanceMetrics, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.EndOperationCalls = append(p.EndOperationCalls, operationID)

	if p.EndOperationFunc != nil {
		return p.EndOperationFunc(operationID)
	}

	return p.endOperationInternal(operationID), nil
}

// endOperationInternal handles the internal logic for ending an operation
func (p *PerformanceProfiler) endOperationInternal(operationID string) *PerformanceMetrics {
	operation, exists := p.operations[operationID]
	if !exists {
		return &PerformanceMetrics{
			OperationID: operationID,
			ErrorCount:  1,
		}
	}

	// Capture ending metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	endTime := time.Now()

	// Calculate final metrics
	duration := endTime.Sub(operation.StartTime)
	currentMemory := int64(m.Alloc)
	memoryDelta := currentMemory - operation.StartMemory

	finalMetrics := &PerformanceMetrics{
		OperationID:        operationID,
		OperationDuration:  duration,
		MemoryAllocated:    currentMemory,
		MemoryFreed:        maxInt64(0, -memoryDelta),
		GoroutineCount:     runtime.NumGoroutine(),
		FileOperations:     0, // Could be enhanced to track actual file ops
		NetworkRequests:    0, // Could be enhanced to track actual network ops
		CacheOperations:    0, // Could be enhanced to track actual cache ops
		ServerCreations:    0, // Could be enhanced based on operation type
		ServerDestructions: 0, // Could be enhanced based on operation type
		ErrorCount:         0,
		WarningCount:       0,
	}

	// Clean up
	delete(p.operations, operationID)

	return finalMetrics
}

// GetActiveOperations returns a list of currently active operations
func (p *PerformanceProfiler) GetActiveOperations() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	operations := make([]string, 0, len(p.operations))
	for id := range p.operations {
		operations = append(operations, id)
	}

	return operations
}

// IsActive returns whether the profiler is currently active
func (p *PerformanceProfiler) IsActive() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.isActive
}

// GetTotalOperations returns the total number of operations tracked
func (p *PerformanceProfiler) GetTotalOperations() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.totalOperations
}

// Reset clears all tracking data and stops profiling
func (p *PerformanceProfiler) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop all active operations
	for id := range p.operations {
		p.endOperationInternal(id)
	}

	// Reset state
	p.isActive = false
	p.operations = make(map[string]*ActiveOperation)
	p.totalOperations = 0
	p.operationCounter = 0

	// Clear call tracking
	p.StartCalls = make([]context.Context, 0)
	p.StopCalls = make([]interface{}, 0)
	p.StartOperationCalls = make([]string, 0)
	p.EndOperationCalls = make([]string, 0)

	// Reset function fields
	p.StartFunc = nil
	p.StopFunc = nil
	p.StartOperationFunc = nil
	p.EndOperationFunc = nil
}

// Helper function to get max of two int64 values
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
