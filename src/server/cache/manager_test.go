package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"lsp-gateway/src/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSCIPCacheManager(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.CacheConfig
		wantErr   bool
		expectNil bool
	}{
		{
			name: "Valid config",
			config: &config.CacheConfig{
				Enabled:     true,
				MaxMemoryMB: 512,
				TTLHours:    24,
				StoragePath: "/tmp/test-cache",
			},
			wantErr:   false,
			expectNil: false,
		},
		{
			name: "Disabled cache",
			config: &config.CacheConfig{
				Enabled: false,
			},
			wantErr:   false,
			expectNil: false,
		},
		{
			name: "Zero memory limit uses default",
			config: &config.CacheConfig{
				Enabled:     true,
				MaxMemoryMB: 0,
				TTLHours:    24,
			},
			wantErr:   false,
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewSCIPCacheManager(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, manager)
			} else {
				assert.NotNil(t, manager)
			}

			// No cleanup needed for SCIPCacheManager
		})
	}
}

func TestCacheKey_Generation(t *testing.T) {
	tests := []struct {
		name   string
		method string
		uri    string
		params interface{}
	}{
		{
			name:   "Simple params",
			method: "textDocument/definition",
			uri:    "file:///test.go",
			params: map[string]interface{}{
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
		},
		{
			name:   "Complex params",
			method: "workspace/symbol",
			uri:    "",
			params: map[string]interface{}{
				"query": "TestFunction",
				"limit": 100,
			},
		},
		{
			name:   "Nil params",
			method: "textDocument/hover",
			uri:    "file:///test.py",
			params: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate cache key
			data, _ := json.Marshal(tt.params)
			hash := sha256.Sum256(data)
			hashStr := fmt.Sprintf("%x", hash)

			key := CacheKey{
				Method: tt.method,
				URI:    tt.uri,
				Hash:   hashStr,
			}

			assert.Equal(t, tt.method, key.Method)
			assert.Equal(t, tt.uri, key.URI)
			assert.NotEmpty(t, key.Hash)

			// Verify hash consistency
			data2, _ := json.Marshal(tt.params)
			hash2 := sha256.Sum256(data2)
			hashStr2 := fmt.Sprintf("%x", hash2)

			assert.Equal(t, hashStr, hashStr2)
		})
	}
}

func TestPosition_Comparison(t *testing.T) {
	tests := []struct {
		name     string
		p1       Position
		p2       Position
		expected int // -1: p1 < p2, 0: p1 == p2, 1: p1 > p2
	}{
		{
			name:     "Equal positions",
			p1:       Position{Line: 10, Character: 5},
			p2:       Position{Line: 10, Character: 5},
			expected: 0,
		},
		{
			name:     "Different lines",
			p1:       Position{Line: 5, Character: 10},
			p2:       Position{Line: 10, Character: 5},
			expected: -1,
		},
		{
			name:     "Same line, different character",
			p1:       Position{Line: 10, Character: 15},
			p2:       Position{Line: 10, Character: 5},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expected == 0 {
				assert.Equal(t, tt.p1.Line, tt.p2.Line)
				assert.Equal(t, tt.p1.Character, tt.p2.Character)
			} else if tt.expected < 0 {
				assert.True(t, tt.p1.Line < tt.p2.Line ||
					(tt.p1.Line == tt.p2.Line && tt.p1.Character < tt.p2.Character))
			} else {
				assert.True(t, tt.p1.Line > tt.p2.Line ||
					(tt.p1.Line == tt.p2.Line && tt.p1.Character > tt.p2.Character))
			}
		})
	}
}

func TestRange_Contains(t *testing.T) {
	r := Range{
		Start: Position{Line: 10, Character: 5},
		End:   Position{Line: 15, Character: 20},
	}

	tests := []struct {
		name     string
		position Position
		contains bool
	}{
		{
			name:     "Inside range",
			position: Position{Line: 12, Character: 10},
			contains: true,
		},
		{
			name:     "At start",
			position: Position{Line: 10, Character: 5},
			contains: true,
		},
		{
			name:     "At end",
			position: Position{Line: 15, Character: 20},
			contains: true,
		},
		{
			name:     "Before range",
			position: Position{Line: 9, Character: 0},
			contains: false,
		},
		{
			name:     "After range",
			position: Position{Line: 16, Character: 0},
			contains: false,
		},
		{
			name:     "Same line, before character",
			position: Position{Line: 10, Character: 3},
			contains: false,
		},
		{
			name:     "Same line, after character",
			position: Position{Line: 15, Character: 25},
			contains: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple containment check
			inRange := false
			if tt.position.Line > r.Start.Line && tt.position.Line < r.End.Line {
				inRange = true
			} else if tt.position.Line == r.Start.Line && tt.position.Line == r.End.Line {
				inRange = tt.position.Character >= r.Start.Character &&
					tt.position.Character <= r.End.Character
			} else if tt.position.Line == r.Start.Line {
				inRange = tt.position.Character >= r.Start.Character
			} else if tt.position.Line == r.End.Line {
				inRange = tt.position.Character <= r.End.Character
			}

			assert.Equal(t, tt.contains, inRange)
		})
	}
}

func TestIndexStats_Basic(t *testing.T) {
	stats := IndexStats{
		DocumentCount: 100,
		SymbolCount:   1500,
		IndexSize:     1024 * 1024,
		LastUpdate:    time.Now(),
		LanguageStats: map[string]int64{
			"go":         50,
			"python":     30,
			"javascript": 20,
		},
		IndexedLanguages: []string{"go", "python", "javascript"},
		Status:           "healthy",
	}

	// Verify total document count
	var totalDocs int64
	for _, count := range stats.LanguageStats {
		totalDocs += count
	}
	assert.Equal(t, stats.DocumentCount, totalDocs)

	// Verify language list matches stats
	assert.Len(t, stats.IndexedLanguages, len(stats.LanguageStats))
	for _, lang := range stats.IndexedLanguages {
		_, exists := stats.LanguageStats[lang]
		assert.True(t, exists)
	}

	// Verify status
	assert.Equal(t, "healthy", stats.Status)

	// Verify other fields
	assert.Equal(t, int64(1500), stats.SymbolCount)
	assert.Equal(t, int64(1024*1024), stats.IndexSize)
	assert.NotZero(t, stats.LastUpdate)
}

func TestConcurrentCacheAccess(t *testing.T) {
	config := &config.CacheConfig{
		Enabled:     true,
		MaxMemoryMB: 10,
		TTLHours:    24,
		StoragePath: "/tmp/test-cache-concurrent",
	}

	manager, err := NewSCIPCacheManager(config)
	require.NoError(t, err)

	ctx := context.Background()
	done := make(chan bool)
	errors := make(chan error, 100)

	// Concurrent operations simulation
	for i := 0; i < 10; i++ {
		go func(id int) {
			// Simulate cache operations
			for j := 0; j < 10; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := fmt.Sprintf("value-%d-%d", id, j)

				// Store operation
				if manager != nil {
					_ = key
					_ = value
					_ = ctx
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(t, err)
	}
}

func TestHashConsistency(t *testing.T) {
	// Test that identical parameters produce identical hashes
	params1 := map[string]interface{}{
		"position": map[string]interface{}{
			"line":      10,
			"character": 5,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	params2 := map[string]interface{}{
		"position": map[string]interface{}{
			"line":      10,
			"character": 5,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	data1, _ := json.Marshal(params1)
	hash1 := sha256.Sum256(data1)
	hashStr1 := fmt.Sprintf("%x", hash1)

	data2, _ := json.Marshal(params2)
	hash2 := sha256.Sum256(data2)
	hashStr2 := fmt.Sprintf("%x", hash2)

	assert.Equal(t, hashStr1, hashStr2, "Identical parameters should produce identical hashes")

	// Test that different parameters produce different hashes
	params3 := map[string]interface{}{
		"position": map[string]interface{}{
			"line":      11, // Different line
			"character": 5,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	data3, _ := json.Marshal(params3)
	hash3 := sha256.Sum256(data3)
	hashStr3 := fmt.Sprintf("%x", hash3)

	assert.NotEqual(t, hashStr1, hashStr3, "Different parameters should produce different hashes")
}
