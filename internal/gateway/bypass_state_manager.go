package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BypassStateEntry represents a stored bypass state for a server
type BypassStateEntry struct {
	ServerName         string             `json:"server_name"`
	BypassReason       string             `json:"bypass_reason"`
	BypassTimestamp    time.Time          `json:"bypass_timestamp"`
	UserBypassDecision bool               `json:"user_bypass_decision"`
	BypassDecisionType BypassDecisionType `json:"bypass_decision_type"`
	Language           string             `json:"language"`
	RecoveryAttempts   int                `json:"recovery_attempts"`
	LastRecoveryTime   time.Time          `json:"last_recovery_time"`
}

// BypassStateData represents the complete bypass state data
type BypassStateData struct {
	Version    string               `json:"version"`
	UpdateTime time.Time            `json:"update_time"`
	Entries    []*BypassStateEntry  `json:"entries"`
}

// BypassStateManager manages persistent bypass decisions across gateway restarts
type BypassStateManager struct {
	stateFilePath string
	data          *BypassStateData
	mu            sync.RWMutex
	logger        *log.Logger
}

// NewBypassStateManager creates a new bypass state manager
func NewBypassStateManager(stateDir string, logger *log.Logger) *BypassStateManager {
	if logger == nil {
		logger = log.New(os.Stderr, "[BypassStateManager] ", log.LstdFlags)
	}

	stateFilePath := filepath.Join(stateDir, "bypass_state.json")
	
	bsm := &BypassStateManager{
		stateFilePath: stateFilePath,
		data: &BypassStateData{
			Version:    "1.0",
			UpdateTime: time.Now(),
			Entries:    make([]*BypassStateEntry, 0),
		},
		logger: logger,
	}

	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		logger.Printf("Warning: Failed to create bypass state directory %s: %v", stateDir, err)
	}

	// Load existing state
	if err := bsm.loadState(); err != nil {
		logger.Printf("Warning: Failed to load bypass state from %s: %v", stateFilePath, err)
	}

	return bsm
}

// SetBypassState records a bypass decision for a server
func (bsm *BypassStateManager) SetBypassState(serverName, language, reason string, decisionType BypassDecisionType, userDecision bool) error {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()

	// Remove any existing entry for this server
	bsm.removeEntryLocked(serverName)

	// Add new entry
	entry := &BypassStateEntry{
		ServerName:         serverName,
		BypassReason:       reason,
		BypassTimestamp:    time.Now(),
		UserBypassDecision: userDecision,
		BypassDecisionType: decisionType,
		Language:           language,
		RecoveryAttempts:   0,
		LastRecoveryTime:   time.Time{},
	}

	bsm.data.Entries = append(bsm.data.Entries, entry)
	bsm.data.UpdateTime = time.Now()

	bsm.logger.Printf("Bypass state set for server %s: %s (type: %s, user: %t)", 
		serverName, reason, decisionType, userDecision)

	return bsm.saveState()
}

// ClearBypassState removes bypass state for a server
func (bsm *BypassStateManager) ClearBypassState(serverName string) error {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()

	if bsm.removeEntryLocked(serverName) {
		bsm.data.UpdateTime = time.Now()
		bsm.logger.Printf("Bypass state cleared for server %s", serverName)
		return bsm.saveState()
	}

	return nil
}

// IsBypassed checks if a server is bypassed
func (bsm *BypassStateManager) IsBypassed(serverName string) bool {
	bsm.mu.RLock()
	defer bsm.mu.RUnlock()

	return bsm.findEntryLocked(serverName) != nil
}

// GetBypassInfo returns bypass information for a server
func (bsm *BypassStateManager) GetBypassInfo(serverName string) *BypassStateEntry {
	bsm.mu.RLock()
	defer bsm.mu.RUnlock()

	entry := bsm.findEntryLocked(serverName)
	if entry == nil {
		return nil
	}

	// Return a copy to avoid race conditions
	return &BypassStateEntry{
		ServerName:         entry.ServerName,
		BypassReason:       entry.BypassReason,
		BypassTimestamp:    entry.BypassTimestamp,
		UserBypassDecision: entry.UserBypassDecision,
		BypassDecisionType: entry.BypassDecisionType,
		Language:           entry.Language,
		RecoveryAttempts:   entry.RecoveryAttempts,
		LastRecoveryTime:   entry.LastRecoveryTime,
	}
}

// GetAllBypassedServers returns all bypassed servers
func (bsm *BypassStateManager) GetAllBypassedServers() []*BypassStateEntry {
	bsm.mu.RLock()
	defer bsm.mu.RUnlock()

	result := make([]*BypassStateEntry, len(bsm.data.Entries))
	for i, entry := range bsm.data.Entries {
		result[i] = &BypassStateEntry{
			ServerName:         entry.ServerName,
			BypassReason:       entry.BypassReason,
			BypassTimestamp:    entry.BypassTimestamp,
			UserBypassDecision: entry.UserBypassDecision,
			BypassDecisionType: entry.BypassDecisionType,
			Language:           entry.Language,
			RecoveryAttempts:   entry.RecoveryAttempts,
			LastRecoveryTime:   entry.LastRecoveryTime,
		}
	}

	return result
}

// GetBypassedServersByLanguage returns bypassed servers for a specific language
func (bsm *BypassStateManager) GetBypassedServersByLanguage(language string) []*BypassStateEntry {
	bsm.mu.RLock()
	defer bsm.mu.RUnlock()

	var result []*BypassStateEntry
	for _, entry := range bsm.data.Entries {
		if entry.Language == language {
			result = append(result, &BypassStateEntry{
				ServerName:         entry.ServerName,
				BypassReason:       entry.BypassReason,
				BypassTimestamp:    entry.BypassTimestamp,
				UserBypassDecision: entry.UserBypassDecision,
				BypassDecisionType: entry.BypassDecisionType,
				Language:           entry.Language,
				RecoveryAttempts:   entry.RecoveryAttempts,
				LastRecoveryTime:   entry.LastRecoveryTime,
			})
		}
	}

	return result
}

// CanAttemptRecovery checks if a server can attempt recovery
func (bsm *BypassStateManager) CanAttemptRecovery(serverName string) bool {
	bsm.mu.RLock()
	defer bsm.mu.RUnlock()

	entry := bsm.findEntryLocked(serverName)
	if entry == nil {
		return false
	}

	// User manual bypasses require explicit user intervention
	if entry.BypassDecisionType == BypassDecisionUserManual {
		return false
	}

	// Check cooldown period based on recovery attempts
	cooldownDuration := bsm.calculateCooldownDuration(entry.RecoveryAttempts)
	return time.Since(entry.LastRecoveryTime) > cooldownDuration
}

// RecordRecoveryAttempt records a recovery attempt for a server
func (bsm *BypassStateManager) RecordRecoveryAttempt(serverName string) error {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()

	entry := bsm.findEntryLocked(serverName)
	if entry == nil {
		return fmt.Errorf("server %s not found in bypass state", serverName)
	}

	entry.RecoveryAttempts++
	entry.LastRecoveryTime = time.Now()
	bsm.data.UpdateTime = time.Now()

	bsm.logger.Printf("Recovery attempt %d recorded for server %s", 
		entry.RecoveryAttempts, serverName)

	return bsm.saveState()
}

// CleanupExpiredEntries removes old bypass entries based on cleanup policy
func (bsm *BypassStateManager) CleanupExpiredEntries() error {
	bsm.mu.Lock()
	defer bsm.mu.Unlock()

	var remaining []*BypassStateEntry
	now := time.Now()
	cleanupCount := 0

	for _, entry := range bsm.data.Entries {
		shouldKeep := true

		// Cleanup policy: remove entries older than 7 days for auto bypasses
		// Keep user manual bypasses indefinitely unless explicitly cleared
		if entry.BypassDecisionType != BypassDecisionUserManual {
			if now.Sub(entry.BypassTimestamp) > 7*24*time.Hour {
				shouldKeep = false
				cleanupCount++
			}
		}

		if shouldKeep {
			remaining = append(remaining, entry)
		}
	}

	if cleanupCount > 0 {
		bsm.data.Entries = remaining
		bsm.data.UpdateTime = time.Now()
		bsm.logger.Printf("Cleaned up %d expired bypass entries", cleanupCount)
		return bsm.saveState()
	}

	return nil
}

// GetBypassStatistics returns statistics about bypass state
func (bsm *BypassStateManager) GetBypassStatistics() map[string]interface{} {
	bsm.mu.RLock()
	defer bsm.mu.RUnlock()

	stats := make(map[string]interface{})
	
	// Count by decision type
	decisionTypeCounts := make(map[BypassDecisionType]int)
	languageCounts := make(map[string]int)
	totalRecoveryAttempts := 0

	for _, entry := range bsm.data.Entries {
		decisionTypeCounts[entry.BypassDecisionType]++
		languageCounts[entry.Language]++
		totalRecoveryAttempts += entry.RecoveryAttempts
	}

	stats["total_bypassed"] = len(bsm.data.Entries)
	stats["by_decision_type"] = decisionTypeCounts
	stats["by_language"] = languageCounts
	stats["total_recovery_attempts"] = totalRecoveryAttempts
	stats["last_update"] = bsm.data.UpdateTime
	stats["state_file"] = bsm.stateFilePath

	return stats
}

// loadState loads bypass state from disk
func (bsm *BypassStateManager) loadState() error {
	file, err := os.Open(bsm.stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty state
			return nil
		}
		return fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	var data BypassStateData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode state file: %w", err)
	}

	bsm.data = &data
	bsm.logger.Printf("Loaded bypass state with %d entries from %s", 
		len(data.Entries), bsm.stateFilePath)

	return nil
}

// saveState saves bypass state to disk
func (bsm *BypassStateManager) saveState() error {
	// Create temporary file for atomic write
	tmpFile := bsm.stateFilePath + ".tmp"
	
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary state file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	
	if err := encoder.Encode(bsm.data); err != nil {
		file.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("failed to encode state data: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, bsm.stateFilePath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temporary state file: %w", err)
	}

	return nil
}

// findEntryLocked finds a bypass entry by server name (must be called with lock held)
func (bsm *BypassStateManager) findEntryLocked(serverName string) *BypassStateEntry {
	for _, entry := range bsm.data.Entries {
		if entry.ServerName == serverName {
			return entry
		}
	}
	return nil
}

// removeEntryLocked removes a bypass entry by server name (must be called with lock held)
func (bsm *BypassStateManager) removeEntryLocked(serverName string) bool {
	for i, entry := range bsm.data.Entries {
		if entry.ServerName == serverName {
			// Remove entry using slice operations
			bsm.data.Entries = append(bsm.data.Entries[:i], bsm.data.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// calculateCooldownDuration calculates cooldown duration based on recovery attempts
func (bsm *BypassStateManager) calculateCooldownDuration(attempts int) time.Duration {
	// Exponential backoff with max cap
	switch {
	case attempts == 0:
		return 5 * time.Minute
	case attempts == 1:
		return 10 * time.Minute
	case attempts == 2:
		return 20 * time.Minute
	case attempts == 3:
		return 40 * time.Minute
	case attempts >= 4:
		return 60 * time.Minute
	default:
		return 5 * time.Minute
	}
}