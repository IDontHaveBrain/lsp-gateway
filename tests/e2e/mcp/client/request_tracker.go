package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	mcp "lsp-gateway/tests/mcp"
)

type RequestTracker struct {
	requests        map[string]*PendingRequest
	timeoutManager  *TimeoutManager
	logger          *TestLogger
	metrics         *RequestMetrics
	
	defaultTimeout  time.Duration
	maxPending      int
	activePending   int32
	
	mu               sync.RWMutex
	shutdown         chan struct{}
	shutdownOnce     sync.Once
	cleanupTicker    *time.Ticker
}

type PendingRequest struct {
	ID              string
	Request         *mcp.MCPRequest
	ResponseCh      chan *mcp.MCPResponse
	ErrorCh         chan error
	CreatedAt       time.Time
	Timeout         time.Duration
	Context         context.Context
	CancelFunc      context.CancelFunc
	RetryCount      int
	LastAttempt     time.Time
	ExpiresAt       time.Time
}

type TimeoutManager struct {
	requestTracker  *RequestTracker
	logger          *TestLogger
	
	timeoutCh       chan string
	running         bool
	mu              sync.RWMutex
	stopCh          chan struct{}
}

type RequestMetrics struct {
	TotalRequests     int64
	ActiveRequests    int64
	CompletedRequests int64
	TimeoutRequests   int64
	CanceledRequests  int64
	FailedRequests    int64
	AverageLatency    time.Duration
	MaxPendingReached int32
	RequestsByMethod  map[string]*MethodMetrics
	mu                sync.RWMutex
}

type MethodMetrics struct {
	RequestCount    int64
	SuccessCount    int64
	FailureCount    int64
	TimeoutCount    int64
	AverageLatency  time.Duration
	MaxLatency      time.Duration
	MinLatency      time.Duration
	mu              sync.RWMutex
}

const (
	DefaultMaxPending     = 1000
	DefaultTimeout        = 30 * time.Second
	CleanupInterval       = 1 * time.Minute
	ExpiredRequestCleanup = 5 * time.Minute
)

func NewRequestTracker(defaultTimeout time.Duration, logger *TestLogger) *RequestTracker {
	rt := &RequestTracker{
		requests:       make(map[string]*PendingRequest),
		logger:         logger,
		defaultTimeout: defaultTimeout,
		maxPending:     DefaultMaxPending,
		shutdown:       make(chan struct{}),
		metrics: &RequestMetrics{
			RequestsByMethod: make(map[string]*MethodMetrics),
		},
	}
	
	rt.timeoutManager = NewTimeoutManager(rt, logger)
	rt.cleanupTicker = time.NewTicker(CleanupInterval)
	
	go rt.cleanupLoop()
	rt.timeoutManager.Start()
	
	return rt
}

func (rt *RequestTracker) TrackRequest(ctx context.Context, request *mcp.MCPRequest) (*PendingRequest, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	if len(rt.requests) >= rt.maxPending {
		atomic.StoreInt32(&rt.metrics.MaxPendingReached, int32(len(rt.requests)))
		return nil, fmt.Errorf("maximum pending requests (%d) reached", rt.maxPending)
	}
	
	requestID := fmt.Sprintf("%v", request.ID)
	if _, exists := rt.requests[requestID]; exists {
		return nil, fmt.Errorf("request ID %s already exists", requestID)
	}
	
	timeout := rt.defaultTimeout
	if deadline, ok := ctx.Deadline(); ok {
		ctxTimeout := time.Until(deadline)
		if ctxTimeout < timeout {
			timeout = ctxTimeout
		}
	}
	
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	now := time.Now()
	
	pendingReq := &PendingRequest{
		ID:         requestID,
		Request:    request,
		ResponseCh: make(chan *mcp.MCPResponse, 1),
		ErrorCh:    make(chan error, 1),
		CreatedAt:  now,
		Timeout:    timeout,
		Context:    requestCtx,
		CancelFunc: cancel,
		RetryCount: 0,
		LastAttempt: now,
		ExpiresAt:  now.Add(timeout),
	}
	
	rt.requests[requestID] = pendingReq
	atomic.AddInt32(&rt.activePending, 1)
	atomic.AddInt64(&rt.metrics.TotalRequests, 1)
	atomic.StoreInt64(&rt.metrics.ActiveRequests, int64(len(rt.requests)))
	
	rt.updateMethodMetrics(request.Method, "request")
	rt.timeoutManager.ScheduleTimeout(requestID, timeout)
	
	rt.logger.Debug("Request tracked", "requestID", requestID, "method", request.Method, "timeout", timeout)
	return pendingReq, nil
}

func (rt *RequestTracker) CompleteRequest(requestID string, response *mcp.MCPResponse, err error) error {
	rt.mu.Lock()
	pendingReq, exists := rt.requests[requestID]
	if !exists {
		rt.mu.Unlock()
		return fmt.Errorf("request ID %s not found", requestID)
	}
	delete(rt.requests, requestID)
	rt.mu.Unlock()
	
	atomic.AddInt32(&rt.activePending, -1)
	atomic.AddInt64(&rt.metrics.CompletedRequests, 1)
	atomic.StoreInt64(&rt.metrics.ActiveRequests, int64(atomic.LoadInt32(&rt.activePending)))
	
	latency := time.Since(pendingReq.CreatedAt)
	rt.updateAverageLatency(latency)
	
	method := pendingReq.Request.Method
	if err != nil {
		atomic.AddInt64(&rt.metrics.FailedRequests, 1)
		rt.updateMethodMetrics(method, "failure")
		
		select {
		case pendingReq.ErrorCh <- err:
		default:
			rt.logger.Warn("Error channel full for request", "requestID", requestID)
		}
	} else {
		rt.updateMethodMetrics(method, "success")
		
		select {
		case pendingReq.ResponseCh <- response:
		default:
			rt.logger.Warn("Response channel full for request", "requestID", requestID)
		}
	}
	
	rt.updateMethodLatency(method, latency)
	pendingReq.CancelFunc()
	
	rt.logger.Debug("Request completed", "requestID", requestID, "method", method, "latency", latency, "success", err == nil)
	return nil
}

func (rt *RequestTracker) TimeoutRequest(requestID string) error {
	rt.mu.Lock()
	pendingReq, exists := rt.requests[requestID]
	if !exists {
		rt.mu.Unlock()
		return fmt.Errorf("request ID %s not found for timeout", requestID)
	}
	delete(rt.requests, requestID)
	rt.mu.Unlock()
	
	atomic.AddInt32(&rt.activePending, -1)
	atomic.AddInt64(&rt.metrics.TimeoutRequests, 1)
	atomic.StoreInt64(&rt.metrics.ActiveRequests, int64(atomic.LoadInt32(&rt.activePending)))
	
	latency := time.Since(pendingReq.CreatedAt)
	rt.updateAverageLatency(latency)
	rt.updateMethodMetrics(pendingReq.Request.Method, "timeout")
	rt.updateMethodLatency(pendingReq.Request.Method, latency)
	
	timeoutErr := fmt.Errorf("request timeout after %v", pendingReq.Timeout)
	
	select {
	case pendingReq.ErrorCh <- timeoutErr:
	default:
		rt.logger.Warn("Error channel full for timeout", "requestID", requestID)
	}
	
	pendingReq.CancelFunc()
	
	rt.logger.Warn("Request timed out", "requestID", requestID, "method", pendingReq.Request.Method, "timeout", pendingReq.Timeout)
	return nil
}

func (rt *RequestTracker) CancelRequest(requestID string) error {
	rt.mu.Lock()
	pendingReq, exists := rt.requests[requestID]
	if !exists {
		rt.mu.Unlock()
		return fmt.Errorf("request ID %s not found for cancellation", requestID)
	}
	delete(rt.requests, requestID)
	rt.mu.Unlock()
	
	atomic.AddInt32(&rt.activePending, -1)
	atomic.AddInt64(&rt.metrics.CanceledRequests, 1)
	atomic.StoreInt64(&rt.metrics.ActiveRequests, int64(atomic.LoadInt32(&rt.activePending)))
	
	cancelErr := fmt.Errorf("request canceled")
	
	select {
	case pendingReq.ErrorCh <- cancelErr:
	default:
		rt.logger.Warn("Error channel full for cancellation", "requestID", requestID)
	}
	
	pendingReq.CancelFunc()
	
	rt.logger.Debug("Request canceled", "requestID", requestID, "method", pendingReq.Request.Method)
	return nil
}

func (rt *RequestTracker) GetPendingRequest(requestID string) (*PendingRequest, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	req, exists := rt.requests[requestID]
	return req, exists
}

func (rt *RequestTracker) GetActivePendingCount() int {
	return int(atomic.LoadInt32(&rt.activePending))
}

func (rt *RequestTracker) GetAllPendingRequests() map[string]*PendingRequest {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	
	requests := make(map[string]*PendingRequest)
	for id, req := range rt.requests {
		requests[id] = req
	}
	return requests
}

func (rt *RequestTracker) GetMetrics() *RequestMetrics {
	rt.metrics.mu.RLock()
	defer rt.metrics.mu.RUnlock()
	
	methodMetrics := make(map[string]*MethodMetrics)
	for method, metrics := range rt.metrics.RequestsByMethod {
		methodMetrics[method] = &MethodMetrics{
			RequestCount:   atomic.LoadInt64(&metrics.RequestCount),
			SuccessCount:   atomic.LoadInt64(&metrics.SuccessCount),
			FailureCount:   atomic.LoadInt64(&metrics.FailureCount),
			TimeoutCount:   atomic.LoadInt64(&metrics.TimeoutCount),
			AverageLatency: metrics.AverageLatency,
			MaxLatency:     metrics.MaxLatency,
			MinLatency:     metrics.MinLatency,
		}
	}
	
	return &RequestMetrics{
		TotalRequests:     atomic.LoadInt64(&rt.metrics.TotalRequests),
		ActiveRequests:    atomic.LoadInt64(&rt.metrics.ActiveRequests),
		CompletedRequests: atomic.LoadInt64(&rt.metrics.CompletedRequests),
		TimeoutRequests:   atomic.LoadInt64(&rt.metrics.TimeoutRequests),
		CanceledRequests:  atomic.LoadInt64(&rt.metrics.CanceledRequests),
		FailedRequests:    atomic.LoadInt64(&rt.metrics.FailedRequests),
		AverageLatency:    rt.metrics.AverageLatency,
		MaxPendingReached: atomic.LoadInt32(&rt.metrics.MaxPendingReached),
		RequestsByMethod:  methodMetrics,
	}
}

func (rt *RequestTracker) Close() error {
	rt.shutdownOnce.Do(func() {
		close(rt.shutdown)
	})
	
	if rt.cleanupTicker != nil {
		rt.cleanupTicker.Stop()
	}
	
	if rt.timeoutManager != nil {
		rt.timeoutManager.Stop()
	}
	
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	for requestID, pendingReq := range rt.requests {
		shutdownErr := fmt.Errorf("request tracker shutdown")
		
		select {
		case pendingReq.ErrorCh <- shutdownErr:
		default:
		}
		
		pendingReq.CancelFunc()
		delete(rt.requests, requestID)
	}
	
	rt.logger.Info("Request tracker closed")
	return nil
}

func (rt *RequestTracker) cleanupLoop() {
	for {
		select {
		case <-rt.cleanupTicker.C:
			rt.cleanupExpiredRequests()
		case <-rt.shutdown:
			return
		}
	}
}

func (rt *RequestTracker) cleanupExpiredRequests() {
	now := time.Now()
	expiredIDs := make([]string, 0)
	
	rt.mu.RLock()
	for requestID, pendingReq := range rt.requests {
		if now.After(pendingReq.ExpiresAt.Add(ExpiredRequestCleanup)) {
			expiredIDs = append(expiredIDs, requestID)
		}
	}
	rt.mu.RUnlock()
	
	for _, requestID := range expiredIDs {
		rt.TimeoutRequest(requestID)
		rt.logger.Debug("Cleaned up expired request", "requestID", requestID)
	}
	
	if len(expiredIDs) > 0 {
		rt.logger.Info("Cleaned up expired requests", "count", len(expiredIDs))
	}
}

func (rt *RequestTracker) updateAverageLatency(latency time.Duration) {
	rt.metrics.mu.Lock()
	defer rt.metrics.mu.Unlock()
	
	if rt.metrics.AverageLatency == 0 {
		rt.metrics.AverageLatency = latency
	} else {
		rt.metrics.AverageLatency = (rt.metrics.AverageLatency + latency) / 2
	}
}

func (rt *RequestTracker) updateMethodMetrics(method, eventType string) {
	rt.metrics.mu.Lock()
	defer rt.metrics.mu.Unlock()
	
	metrics, exists := rt.metrics.RequestsByMethod[method]
	if !exists {
		metrics = &MethodMetrics{
			MinLatency: time.Hour, // Initialize to a high value
		}
		rt.metrics.RequestsByMethod[method] = metrics
	}
	
	switch eventType {
	case "request":
		atomic.AddInt64(&metrics.RequestCount, 1)
	case "success":
		atomic.AddInt64(&metrics.SuccessCount, 1)
	case "failure":
		atomic.AddInt64(&metrics.FailureCount, 1)
	case "timeout":
		atomic.AddInt64(&metrics.TimeoutCount, 1)
	}
}

func (rt *RequestTracker) updateMethodLatency(method string, latency time.Duration) {
	rt.metrics.mu.Lock()
	defer rt.metrics.mu.Unlock()
	
	metrics, exists := rt.metrics.RequestsByMethod[method]
	if !exists {
		return
	}
	
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	
	if metrics.AverageLatency == 0 {
		metrics.AverageLatency = latency
	} else {
		metrics.AverageLatency = (metrics.AverageLatency + latency) / 2
	}
	
	if latency > metrics.MaxLatency {
		metrics.MaxLatency = latency
	}
	
	if latency < metrics.MinLatency {
		metrics.MinLatency = latency
	}
}

func NewTimeoutManager(requestTracker *RequestTracker, logger *TestLogger) *TimeoutManager {
	return &TimeoutManager{
		requestTracker: requestTracker,
		logger:         logger,
		timeoutCh:      make(chan string, 1000),
		stopCh:         make(chan struct{}),
	}
}

func (tm *TimeoutManager) Start() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.running {
		return
	}
	
	tm.running = true
	go tm.timeoutLoop()
	tm.logger.Debug("Timeout manager started")
}

func (tm *TimeoutManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if !tm.running {
		return
	}
	
	tm.running = false
	close(tm.stopCh)
	tm.logger.Debug("Timeout manager stopped")
}

func (tm *TimeoutManager) ScheduleTimeout(requestID string, timeout time.Duration) {
	go func() {
		select {
		case <-time.After(timeout):
			select {
			case tm.timeoutCh <- requestID:
			default:
				tm.logger.Warn("Timeout channel full, dropping timeout for request", "requestID", requestID)
			}
		case <-tm.stopCh:
			return
		}
	}()
}

func (tm *TimeoutManager) timeoutLoop() {
	for {
		select {
		case requestID := <-tm.timeoutCh:
			if err := tm.requestTracker.TimeoutRequest(requestID); err != nil {
				tm.logger.Debug("Failed to timeout request", "requestID", requestID, "error", err)
			}
		case <-tm.stopCh:
			return
		}
	}
}