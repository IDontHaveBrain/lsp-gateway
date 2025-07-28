package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TimeoutConfig struct {
	ServerStartup       time.Duration
	ServerHealthCheck   time.Duration
	LSPRequest          time.Duration
	WorkspaceLoad       time.Duration
	IntegrationTest     time.Duration
	CleanupOperation    time.Duration
	RepositoryClone     time.Duration
	NetworkOperation    time.Duration
	ResourceWait        time.Duration
	ConcurrentTest      time.Duration
}

func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		ServerStartup:       30 * time.Second,
		ServerHealthCheck:   15 * time.Second,
		LSPRequest:          5 * time.Second,
		WorkspaceLoad:       20 * time.Second,
		IntegrationTest:     3 * time.Minute,
		CleanupOperation:    10 * time.Second,
		RepositoryClone:     5 * time.Minute,
		NetworkOperation:    30 * time.Second,
		ResourceWait:        5 * time.Second,
		ConcurrentTest:      5 * time.Minute,
	}
}

type TimeoutType string

const (
	TimeoutServerStartup     TimeoutType = "server_startup"
	TimeoutServerHealthCheck TimeoutType = "server_health_check"
	TimeoutLSPRequest        TimeoutType = "lsp_request"
	TimeoutWorkspaceLoad     TimeoutType = "workspace_load"
	TimeoutIntegrationTest   TimeoutType = "integration_test"
	TimeoutCleanupOperation  TimeoutType = "cleanup_operation"
	TimeoutRepositoryClone   TimeoutType = "repository_clone"
	TimeoutNetworkOperation  TimeoutType = "network_operation"
	TimeoutResourceWait      TimeoutType = "resource_wait"
	TimeoutConcurrentTest    TimeoutType = "concurrent_test"
)

type TimeoutEvent struct {
	Type      TimeoutType           `json:"type"`
	Operation string                `json:"operation"`
	Duration  time.Duration         `json:"duration"`
	Timestamp time.Time             `json:"timestamp"`
	Context   map[string]interface{} `json:"context"`
	Exceeded  bool                  `json:"exceeded"`
}

type PythonPatternsTimeoutManager struct {
	config         TimeoutConfig
	logger         *PythonPatternsLogger
	timeoutEvents  []TimeoutEvent
	activeContexts map[string]context.CancelFunc
	escalationRules map[TimeoutType][]EscalationRule
	mu             sync.RWMutex
}

type EscalationRule struct {
	ThresholdPercentage float64
	Action              EscalationAction
	ActionParams        map[string]interface{}
}

type EscalationAction string

const (
	EscalationActionLog        EscalationAction = "log"
	EscalationActionRetry      EscalationAction = "retry"
	EscalationActionExtend     EscalationAction = "extend"
	EscalationActionFallback   EscalationAction = "fallback"
	EscalationActionEmergency  EscalationAction = "emergency"
)

func NewPythonPatternsTimeoutManager(logger *PythonPatternsLogger) *PythonPatternsTimeoutManager {
	tm := &PythonPatternsTimeoutManager{
		config:          DefaultTimeoutConfig(),
		logger:          logger,
		timeoutEvents:   make([]TimeoutEvent, 0),
		activeContexts:  make(map[string]context.CancelFunc),
		escalationRules: make(map[TimeoutType][]EscalationRule),
	}

	tm.initializeEscalationRules()
	return tm
}

func (tm *PythonPatternsTimeoutManager) initializeEscalationRules() {
	tm.escalationRules[TimeoutServerStartup] = []EscalationRule{
		{50.0, EscalationActionLog, map[string]interface{}{"level": "warning"}},
		{80.0, EscalationActionExtend, map[string]interface{}{"factor": 1.5}},
		{100.0, EscalationActionEmergency, map[string]interface{}{"fallback": "restart"}},
	}

	tm.escalationRules[TimeoutLSPRequest] = []EscalationRule{
		{80.0, EscalationActionLog, map[string]interface{}{"level": "warning"}},
		{100.0, EscalationActionRetry, map[string]interface{}{"max_retries": 2}},
	}

	tm.escalationRules[TimeoutWorkspaceLoad] = []EscalationRule{
		{60.0, EscalationActionLog, map[string]interface{}{"level": "info"}},
		{90.0, EscalationActionExtend, map[string]interface{}{"factor": 2.0}},
		{100.0, EscalationActionFallback, map[string]interface{}{"action": "skip_test"}},
	}

	tm.escalationRules[TimeoutIntegrationTest] = []EscalationRule{
		{70.0, EscalationActionLog, map[string]interface{}{"level": "warning"}},
		{100.0, EscalationActionEmergency, map[string]interface{}{"cleanup": true}},
	}

	tm.escalationRules[TimeoutRepositoryClone] = []EscalationRule{
		{50.0, EscalationActionLog, map[string]interface{}{"level": "info"}},
		{80.0, EscalationActionRetry, map[string]interface{}{"max_retries": 3}},
		{100.0, EscalationActionFallback, map[string]interface{}{"fallback": "cached_repo"}},
	}
}

func (tm *PythonPatternsTimeoutManager) CreateTimeoutContext(parent context.Context, timeoutType TimeoutType, operation string) (context.Context, context.CancelFunc) {
	timeout := tm.getTimeout(timeoutType)
	
	ctx, cancel := context.WithTimeout(parent, timeout)
	
	contextKey := fmt.Sprintf("%s_%s_%d", timeoutType, operation, time.Now().UnixNano())
	
	tm.mu.Lock()
	tm.activeContexts[contextKey] = cancel
	tm.mu.Unlock()

	tm.logger.Debug("Created timeout context", map[string]interface{}{
		"timeout_type": timeoutType,
		"operation":    operation,
		"timeout":      timeout,
		"context_key":  contextKey,
	})

	wrappedCancel := func() {
		tm.mu.Lock()
		delete(tm.activeContexts, contextKey)
		tm.mu.Unlock()
		cancel()
	}

	go tm.monitorTimeout(ctx, timeoutType, operation, timeout)

	return ctx, wrappedCancel
}

func (tm *PythonPatternsTimeoutManager) getTimeout(timeoutType TimeoutType) time.Duration {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	switch timeoutType {
	case TimeoutServerStartup:
		return tm.config.ServerStartup
	case TimeoutServerHealthCheck:
		return tm.config.ServerHealthCheck
	case TimeoutLSPRequest:
		return tm.config.LSPRequest
	case TimeoutWorkspaceLoad:
		return tm.config.WorkspaceLoad
	case TimeoutIntegrationTest:
		return tm.config.IntegrationTest
	case TimeoutCleanupOperation:
		return tm.config.CleanupOperation
	case TimeoutRepositoryClone:
		return tm.config.RepositoryClone
	case TimeoutNetworkOperation:
		return tm.config.NetworkOperation
	case TimeoutResourceWait:
		return tm.config.ResourceWait
	case TimeoutConcurrentTest:
		return tm.config.ConcurrentTest
	default:
		return 30 * time.Second
	}
}

func (tm *PythonPatternsTimeoutManager) monitorTimeout(ctx context.Context, timeoutType TimeoutType, operation string, timeout time.Duration) {
	startTime := time.Now()
	
	rules := tm.escalationRules[timeoutType]
	if len(rules) == 0 {
		return
	}

	ticker := time.NewTicker(timeout / 10) // Check 10 times during timeout period
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			duration := time.Since(startTime)
			exceeded := duration >= timeout

			event := TimeoutEvent{
				Type:      timeoutType,
				Operation: operation,
				Duration:  duration,
				Timestamp: time.Now(),
				Context: map[string]interface{}{
					"timeout_configured": timeout,
					"duration_actual":    duration,
				},
				Exceeded: exceeded,
			}

			tm.recordTimeoutEvent(event)

			if exceeded {
				tm.logger.Warn("Operation exceeded timeout", map[string]interface{}{
					"timeout_type": timeoutType,
					"operation":    operation,
					"timeout":      timeout,
					"actual":       duration,
				})
			} else {
				tm.logger.Debug("Operation completed within timeout", map[string]interface{}{
					"timeout_type": timeoutType,
					"operation":    operation,
					"duration":     duration,
				})
			}
			return

		case <-ticker.C:
			elapsed := time.Since(startTime)
			percentage := (float64(elapsed) / float64(timeout)) * 100

			tm.checkEscalationRules(timeoutType, operation, percentage, elapsed, timeout)
		}
	}
}

func (tm *PythonPatternsTimeoutManager) checkEscalationRules(timeoutType TimeoutType, operation string, percentage float64, elapsed, timeout time.Duration) {
	rules := tm.escalationRules[timeoutType]
	
	for _, rule := range rules {
		if percentage >= rule.ThresholdPercentage {
			tm.executeEscalationAction(timeoutType, operation, rule, elapsed, timeout)
		}
	}
}

func (tm *PythonPatternsTimeoutManager) executeEscalationAction(timeoutType TimeoutType, operation string, rule EscalationRule, elapsed, timeout time.Duration) {
	tm.logger.Debug("Executing escalation action", map[string]interface{}{
		"timeout_type": timeoutType,
		"operation":    operation,
		"action":       rule.Action,
		"threshold":    rule.ThresholdPercentage,
		"elapsed":      elapsed,
		"timeout":      timeout,
	})

	switch rule.Action {
	case EscalationActionLog:
		level := rule.ActionParams["level"].(string)
		tm.logEscalation(level, timeoutType, operation, elapsed, timeout)
		
	case EscalationActionRetry:
		maxRetries := rule.ActionParams["max_retries"].(int)
		tm.logger.Info("Timeout escalation suggests retry", map[string]interface{}{
			"timeout_type": timeoutType,
			"operation":    operation,
			"max_retries":  maxRetries,
		})
		
	case EscalationActionExtend:
		factor := rule.ActionParams["factor"].(float64)
		newTimeout := time.Duration(float64(timeout) * factor)
		tm.extendTimeout(timeoutType, newTimeout)
		
	case EscalationActionFallback:
		fallbackAction := rule.ActionParams["fallback"].(string)
		tm.logger.Warn("Timeout escalation activating fallback", map[string]interface{}{
			"timeout_type":    timeoutType,
			"operation":       operation,
			"fallback_action": fallbackAction,
		})
		
	case EscalationActionEmergency:
		tm.logger.Error("Timeout escalation: emergency mode activated", map[string]interface{}{
			"timeout_type": timeoutType,
			"operation":    operation,
			"action_params": rule.ActionParams,
		})
	}
}

func (tm *PythonPatternsTimeoutManager) logEscalation(level string, timeoutType TimeoutType, operation string, elapsed, timeout time.Duration) {
	percentage := (float64(elapsed) / float64(timeout)) * 100
	
	logData := map[string]interface{}{
		"timeout_type": timeoutType,
		"operation":    operation,
		"elapsed":      elapsed,
		"timeout":      timeout,
		"percentage":   percentage,
	}

	switch level {
	case "warning":
		tm.logger.Warn("Timeout escalation warning", logData)
	case "error":
		tm.logger.Error("Timeout escalation error", logData)
	default:
		tm.logger.Info("Timeout escalation info", logData)
	}
}

func (tm *PythonPatternsTimeoutManager) extendTimeout(timeoutType TimeoutType, newTimeout time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	switch timeoutType {
	case TimeoutServerStartup:
		tm.config.ServerStartup = newTimeout
	case TimeoutServerHealthCheck:
		tm.config.ServerHealthCheck = newTimeout
	case TimeoutLSPRequest:
		tm.config.LSPRequest = newTimeout
	case TimeoutWorkspaceLoad:
		tm.config.WorkspaceLoad = newTimeout
	case TimeoutIntegrationTest:
		tm.config.IntegrationTest = newTimeout
	case TimeoutCleanupOperation:
		tm.config.CleanupOperation = newTimeout
	case TimeoutRepositoryClone:
		tm.config.RepositoryClone = newTimeout
	case TimeoutNetworkOperation:
		tm.config.NetworkOperation = newTimeout
	case TimeoutResourceWait:
		tm.config.ResourceWait = newTimeout
	case TimeoutConcurrentTest:
		tm.config.ConcurrentTest = newTimeout
	}

	tm.logger.Info("Timeout extended", map[string]interface{}{
		"timeout_type": timeoutType,
		"new_timeout":  newTimeout,
	})
}

func (tm *PythonPatternsTimeoutManager) recordTimeoutEvent(event TimeoutEvent) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	tm.timeoutEvents = append(tm.timeoutEvents, event)
	
	if len(tm.timeoutEvents) > 1000 {
		tm.timeoutEvents = tm.timeoutEvents[100:]
	}
}

func (tm *PythonPatternsTimeoutManager) GetTimeoutStats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := map[string]interface{}{
		"current_config":   tm.config,
		"active_contexts":  len(tm.activeContexts),
		"timeout_events":   len(tm.timeoutEvents),
		"events_by_type":   make(map[string]int),
		"exceeded_events":  make(map[string]int),
		"average_duration": make(map[string]time.Duration),
	}

	eventsByType := make(map[TimeoutType][]TimeoutEvent)
	for _, event := range tm.timeoutEvents {
		eventsByType[event.Type] = append(eventsByType[event.Type], event)
		
		stats["events_by_type"].(map[string]int)[string(event.Type)]++
		
		if event.Exceeded {
			stats["exceeded_events"].(map[string]int)[string(event.Type)]++
		}
	}

	for timeoutType, events := range eventsByType {
		if len(events) > 0 {
			var total time.Duration
			for _, event := range events {
				total += event.Duration
			}
			stats["average_duration"].(map[string]time.Duration)[string(timeoutType)] = total / time.Duration(len(events))
		}
	}

	return stats
}

func (tm *PythonPatternsTimeoutManager) CancelAllActiveContexts() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.logger.Info("Cancelling all active timeout contexts", map[string]interface{}{
		"active_contexts": len(tm.activeContexts),
	})

	for contextKey, cancel := range tm.activeContexts {
		cancel()
		tm.logger.Debug("Cancelled context", map[string]interface{}{
			"context_key": contextKey,
		})
	}

	tm.activeContexts = make(map[string]context.CancelFunc)
}

func (tm *PythonPatternsTimeoutManager) GetRecentTimeoutEvents(limit int) []TimeoutEvent {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if limit <= 0 || limit > len(tm.timeoutEvents) {
		limit = len(tm.timeoutEvents)
	}

	events := make([]TimeoutEvent, limit)
	startIndex := len(tm.timeoutEvents) - limit
	copy(events, tm.timeoutEvents[startIndex:])

	return events
}

func (tm *PythonPatternsTimeoutManager) UpdateTimeout(timeoutType TimeoutType, newTimeout time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	switch timeoutType {
	case TimeoutServerStartup:
		tm.config.ServerStartup = newTimeout
	case TimeoutServerHealthCheck:
		tm.config.ServerHealthCheck = newTimeout
	case TimeoutLSPRequest:
		tm.config.LSPRequest = newTimeout
	case TimeoutWorkspaceLoad:
		tm.config.WorkspaceLoad = newTimeout
	case TimeoutIntegrationTest:
		tm.config.IntegrationTest = newTimeout
	case TimeoutCleanupOperation:
		tm.config.CleanupOperation = newTimeout
	case TimeoutRepositoryClone:
		tm.config.RepositoryClone = newTimeout
	case TimeoutNetworkOperation:
		tm.config.NetworkOperation = newTimeout
	case TimeoutResourceWait:
		tm.config.ResourceWait = newTimeout
	case TimeoutConcurrentTest:
		tm.config.ConcurrentTest = newTimeout
	}

	tm.logger.Info("Timeout configuration updated", map[string]interface{}{
		"timeout_type": timeoutType,
		"new_timeout":  newTimeout,
	})
}