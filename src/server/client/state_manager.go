package client

import (
	"encoding/json"
	"sync"
	"time"
)

type pendingRequest struct {
	respCh chan json.RawMessage
	done   chan struct{}
}

type ClientStateManager struct {
	mu             sync.RWMutex
	writeMu        sync.Mutex
	active         bool
	requests       map[string]*pendingRequest
	nextID         int
	timeoutsMu     sync.RWMutex
	recentTimeouts map[string]time.Time
}

func NewClientStateManager() *ClientStateManager {
	return &ClientStateManager{
		requests:       make(map[string]*pendingRequest),
		recentTimeouts: make(map[string]time.Time),
	}
}

func (sm *ClientStateManager) IsActive() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.active
}

func (sm *ClientStateManager) SetActive(active bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.active = active
}

func (sm *ClientStateManager) GenerateRequestID() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.nextID++
	return string(rune(sm.nextID))
}

func (sm *ClientStateManager) AddPendingRequest(id string, pr *pendingRequest) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.requests[id] = pr
}

func (sm *ClientStateManager) GetPendingRequest(id string) (*pendingRequest, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	pr, exists := sm.requests[id]
	return pr, exists
}

func (sm *ClientStateManager) RemovePendingRequest(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.requests, id)
}

func (sm *ClientStateManager) LockWrite() {
	sm.writeMu.Lock()
}

func (sm *ClientStateManager) UnlockWrite() {
	sm.writeMu.Unlock()
}

func (sm *ClientStateManager) AddRecentTimeout(id string) {
	sm.timeoutsMu.Lock()
	defer sm.timeoutsMu.Unlock()
	sm.recentTimeouts[id] = time.Now()
}

func (sm *ClientStateManager) IsRecentTimeout(id string) bool {
	sm.timeoutsMu.RLock()
	defer sm.timeoutsMu.RUnlock()
	if timeoutTime, exists := sm.recentTimeouts[id]; exists {
		return time.Since(timeoutTime) < 5*time.Second
	}
	return false
}

func (sm *ClientStateManager) CleanupOldTimeouts() {
	sm.timeoutsMu.Lock()
	defer sm.timeoutsMu.Unlock()

	cutoff := time.Now().Add(-10 * time.Second)
	for id, t := range sm.recentTimeouts {
		if t.Before(cutoff) {
			delete(sm.recentTimeouts, id)
		}
	}
}
