package mcp

import (
	"fmt"
	"sync"
	"time"
)

// ClientState represents the connection state of the MCP client
type ClientState int

const (
	// Disconnected means the client is not connected
	Disconnected ClientState = iota
	// Connecting means the client is attempting to establish a connection
	Connecting
	// Connected means the client has established a connection
	Connected
	// Initialized means the client has completed the initialization handshake
	Initialized
)

// String returns a string representation of the client state
func (s ClientState) String() string {
	switch s {
	case Disconnected:
		return "Disconnected"
	case Connecting:
		return "Connecting"
	case Connected:
		return "Connected"
	case Initialized:
		return "Initialized"
	default:
		return "Unknown"
	}
}

// StateManager manages the client connection state with thread-safe operations
type StateManager struct {
	mu              sync.RWMutex
	state           ClientState
	lastStateChange time.Time
	stateListeners  []StateChangeListener
}

// StateChangeListener is a callback for state changes
type StateChangeListener func(oldState, newState ClientState)

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		state:           Disconnected,
		lastStateChange: time.Now(),
		stateListeners:  make([]StateChangeListener, 0),
	}
}

// GetState returns the current state
func (sm *StateManager) GetState() ClientState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// SetState updates the state and notifies listeners
func (sm *StateManager) SetState(newState ClientState) error {
	sm.mu.Lock()
	oldState := sm.state
	
	// Validate state transition
	if err := sm.validateStateTransition(oldState, newState); err != nil {
		sm.mu.Unlock()
		return err
	}
	
	sm.state = newState
	sm.lastStateChange = time.Now()
	listeners := make([]StateChangeListener, len(sm.stateListeners))
	copy(listeners, sm.stateListeners)
	sm.mu.Unlock()
	
	// Notify listeners outside of lock
	for _, listener := range listeners {
		listener(oldState, newState)
	}
	
	return nil
}

// validateStateTransition checks if a state transition is valid
func (sm *StateManager) validateStateTransition(from, to ClientState) error {
	// Define valid transitions
	validTransitions := map[ClientState][]ClientState{
		Disconnected: {Connecting},
		Connecting:   {Connected, Disconnected},
		Connected:    {Initialized, Disconnected},
		Initialized:  {Disconnected},
	}
	
	allowedStates, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("invalid from state: %s", from)
	}
	
	for _, allowed := range allowedStates {
		if allowed == to {
			return nil
		}
	}
	
	return fmt.Errorf("invalid state transition from %s to %s", from, to)
}

// AddStateListener adds a listener for state changes
func (sm *StateManager) AddStateListener(listener StateChangeListener) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.stateListeners = append(sm.stateListeners, listener)
}

// GetLastStateChange returns the time of the last state change
func (sm *StateManager) GetLastStateChange() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastStateChange
}

// IsConnected returns true if the client is in Connected or Initialized state
func (sm *StateManager) IsConnected() bool {
	state := sm.GetState()
	return state == Connected || state == Initialized
}

// IsInitialized returns true if the client is in Initialized state
func (sm *StateManager) IsInitialized() bool {
	return sm.GetState() == Initialized
}

// WaitForState waits for the state to reach the target state or timeout
func (sm *StateManager) WaitForState(targetState ClientState, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 10 * time.Millisecond
	
	for {
		if sm.GetState() == targetState {
			return nil
		}
		
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for state %s, current state: %s", targetState, sm.GetState())
		}
		
		time.Sleep(checkInterval)
	}
}