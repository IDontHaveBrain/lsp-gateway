package indexing

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// NewPathFilter creates a new path filter with ignore patterns
func NewPathFilter(ignorePatterns []string) (*PathFilter, error) {
	filter := &PathFilter{
		includeTree:    NewPathPrefixTree(),
		excludeTree:    NewPathPrefixTree(),
		ignorePatterns: make([]*IgnorePattern, 0, len(ignorePatterns)),
	}

	// Compile ignore patterns
	for _, pattern := range ignorePatterns {
		compiledPattern, err := compileIgnorePattern(pattern)
		if err != nil {
			log.Printf("PathFilter: Failed to compile ignore pattern '%s': %v", pattern, err)
			continue
		}
		filter.ignorePatterns = append(filter.ignorePatterns, compiledPattern)

		// Add to exclude tree for fast prefix matching
		filter.excludeTree.Insert(pattern, compiledPattern)
	}

	log.Printf("PathFilter: Compiled %d ignore patterns", len(filter.ignorePatterns))
	return filter, nil
}

// ShouldIgnore checks if a path should be ignored based on patterns
func (pf *PathFilter) ShouldIgnore(path string) bool {
	startTime := time.Now()
	defer func() {
		pf.updateMatchTime(time.Since(startTime))
	}()

	// Normalize path
	normalizedPath := filepath.Clean(path)

	// Quick prefix tree lookup first
	if pf.excludeTree.HasPrefix(normalizedPath) {
		atomic.AddInt64(&pf.matchCount, 1)
		return true
	}

	// Check each ignore pattern
	for _, pattern := range pf.ignorePatterns {
		if pf.matchesPattern(normalizedPath, pattern) {
			atomic.AddInt64(&pf.matchCount, 1)
			return true
		}
	}

	return false
}

// matchesPattern checks if a path matches an ignore pattern
func (pf *PathFilter) matchesPattern(path string, pattern *IgnorePattern) bool {
	// Handle negation
	if pattern.Negated {
		// Negated patterns include files that would otherwise be ignored
		return false
	}

	// Directory-only patterns
	if pattern.IsDirectory && !strings.HasSuffix(path, "/") {
		return false
	}

	// Glob pattern matching
	if pattern.IsGlob {
		matched, err := filepath.Match(pattern.Pattern, filepath.Base(path))
		if err != nil {
			return false
		}
		return matched
	}

	// Regex pattern matching
	if pattern.CompiledRegex != nil {
		if regex, ok := pattern.CompiledRegex.(*regexp.Regexp); ok {
			return regex.MatchString(path)
		}
	}

	// Simple string matching
	return strings.Contains(path, pattern.Pattern)
}

// updateMatchTime updates average match time statistics
func (pf *PathFilter) updateMatchTime(duration time.Duration) {
	if pf.avgMatchTime == 0 {
		pf.avgMatchTime = duration
	} else {
		alpha := 0.1
		pf.avgMatchTime = time.Duration(float64(pf.avgMatchTime)*(1-alpha) + float64(duration)*alpha)
	}
}

// GetStats returns path filter statistics
func (pf *PathFilter) GetStats() map[string]interface{} {
	pf.mutex.RLock()
	defer pf.mutex.RUnlock()

	return map[string]interface{}{
		"total_matches":     atomic.LoadInt64(&pf.matchCount),
		"avg_match_time_ns": int64(pf.avgMatchTime),
		"pattern_count":     len(pf.ignorePatterns),
		"tree_size":         pf.excludeTree.Size(),
	}
}

// compileIgnorePattern compiles an ignore pattern string into a structured pattern
func compileIgnorePattern(pattern string) (*IgnorePattern, error) {
	if pattern == "" {
		return nil, fmt.Errorf("empty pattern")
	}

	// Clean the pattern
	cleanPattern := strings.TrimSpace(pattern)

	compiledPattern := &IgnorePattern{
		Pattern: cleanPattern,
	}

	// Check for negation
	if strings.HasPrefix(cleanPattern, "!") {
		compiledPattern.Negated = true
		cleanPattern = cleanPattern[1:]
	}

	// Check for directory-only pattern
	if strings.HasSuffix(cleanPattern, "/") {
		compiledPattern.IsDirectory = true
		cleanPattern = strings.TrimSuffix(cleanPattern, "/")
	}

	// Check if it's a glob pattern
	if strings.ContainsAny(cleanPattern, "*?[]") {
		compiledPattern.IsGlob = true
	} else {
		// Try to compile as regex for more complex patterns
		if strings.ContainsAny(cleanPattern, "^$(){}|+\\") {
			regex, err := regexp.Compile(cleanPattern)
			if err == nil {
				compiledPattern.CompiledRegex = regex
				compiledPattern.Regex = cleanPattern
			}
		}
	}

	return compiledPattern, nil
}

// NewPathPrefixTree creates a new path prefix tree
func NewPathPrefixTree() *PathPrefixTree {
	return &PathPrefixTree{
		root: &PrefixNode{
			children: make(map[string]*PrefixNode),
		},
	}
}

// Insert adds a pattern to the prefix tree
func (ppt *PathPrefixTree) Insert(path string, pattern *IgnorePattern) {
	ppt.mutex.Lock()
	defer ppt.mutex.Unlock()

	// Split path into segments
	segments := strings.Split(filepath.Clean(path), string(filepath.Separator))

	current := ppt.root
	depth := 0

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if current.children[segment] == nil {
			current.children[segment] = &PrefixNode{
				segment:  segment,
				children: make(map[string]*PrefixNode),
				depth:    depth,
			}
		}

		current = current.children[segment]
		depth++
	}

	// Mark as pattern and store the compiled pattern
	current.isPattern = true
	current.pattern = pattern

	ppt.size++
}

// HasPrefix checks if any patterns match the given path prefix
func (ppt *PathPrefixTree) HasPrefix(path string) bool {
	ppt.mutex.RLock()
	defer ppt.mutex.RUnlock()

	// Split path into segments
	segments := strings.Split(filepath.Clean(path), string(filepath.Separator))

	current := ppt.root

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		// Check if current node is a pattern
		if current.isPattern {
			return true
		}

		// Check for wildcard matches
		for childSegment, child := range current.children {
			if matchesWildcard(childSegment, segment) {
				if child.isPattern {
					return true
				}
			}
		}

		// Move to specific child
		if current.children[segment] == nil {
			return false
		}

		current = current.children[segment]
	}

	// Check if final node is a pattern
	return current.isPattern
}

// matchesWildcard checks if a segment matches a wildcard pattern
func matchesWildcard(pattern, segment string) bool {
	if pattern == segment {
		return true
	}

	// Simple wildcard matching for common cases
	if strings.Contains(pattern, "*") {
		matched, err := filepath.Match(pattern, segment)
		return err == nil && matched
	}

	return false
}

// Size returns the number of patterns in the tree
func (ppt *PathPrefixTree) Size() int {
	ppt.mutex.RLock()
	defer ppt.mutex.RUnlock()
	return ppt.size
}

// Clear clears the prefix tree
func (ppt *PathPrefixTree) Clear() {
	ppt.mutex.Lock()
	defer ppt.mutex.Unlock()

	ppt.root = &PrefixNode{
		children: make(map[string]*PrefixNode),
	}
	ppt.size = 0
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(maxMemoryUsage int64, cpuThreshold float64) *ResourceMonitor {
	return &ResourceMonitor{
		maxMemoryUsage:  maxMemoryUsage,
		memoryThreshold: int64(float64(maxMemoryUsage) * 0.8), // 80% of max
		cpuThreshold:    cpuThreshold,
		monitorInterval: 30 * time.Second,
	}
}

// Start begins resource monitoring
func (rm *ResourceMonitor) Start() {
	rm.mutex.Lock()
	if rm.monitoring {
		rm.mutex.Unlock()
		return
	}
	rm.monitoring = true
	rm.mutex.Unlock()

	go rm.monitorLoop()
	log.Println("ResourceMonitor: Started resource monitoring")
}

// Stop stops resource monitoring
func (rm *ResourceMonitor) Stop() {
	rm.mutex.Lock()
	rm.monitoring = false
	rm.mutex.Unlock()

	log.Println("ResourceMonitor: Stopped resource monitoring")
}

// monitorLoop runs the resource monitoring loop
func (rm *ResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(rm.monitorInterval)
	defer ticker.Stop()

	for {
		rm.mutex.RLock()
		monitoring := rm.monitoring
		rm.mutex.RUnlock()

		if !monitoring {
			return
		}

		select {
		case <-ticker.C:
			rm.checkResources()
		}
	}
}

// checkResources checks current resource usage
func (rm *ResourceMonitor) checkResources() {
	// Check memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	currentMemory := int64(memStats.Alloc)

	rm.mutex.Lock()
	rm.currentMemoryUsage = currentMemory
	if currentMemory > rm.peakMemoryUsage {
		rm.peakMemoryUsage = currentMemory
	}
	rm.lastMonitorTime = time.Now()
	rm.mutex.Unlock()

	// Check if memory usage exceeds threshold
	if currentMemory > rm.memoryThreshold {
		alert := ResourceAlert{
			Type:      "memory",
			Severity:  "warning",
			Current:   float64(currentMemory),
			Threshold: float64(rm.memoryThreshold),
			Message: fmt.Sprintf("Memory usage %.1f MB exceeds threshold %.1f MB",
				float64(currentMemory)/(1024*1024), float64(rm.memoryThreshold)/(1024*1024)),
			Timestamp: time.Now(),
		}

		if currentMemory > rm.maxMemoryUsage {
			alert.Severity = "critical"
		}

		rm.handleAlert(alert)
	}

	// TODO: Add CPU monitoring using a cross-platform solution
	// For now, we'll skip CPU monitoring as it requires platform-specific code
}

// handleAlert handles resource usage alerts
func (rm *ResourceMonitor) handleAlert(alert ResourceAlert) {
	log.Printf("ResourceMonitor: %s alert - %s", strings.ToUpper(alert.Severity), alert.Message)

	if rm.alertCallback != nil {
		rm.alertCallback(alert)
	}

	// Take action based on alert type and severity
	switch alert.Type {
	case "memory":
		if alert.Severity == "critical" {
			// Force garbage collection
			runtime.GC()
			log.Println("ResourceMonitor: Forced garbage collection due to critical memory usage")
		}
	}
}

// SetAlertCallback sets a callback function for resource alerts
func (rm *ResourceMonitor) SetAlertCallback(callback func(ResourceAlert)) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.alertCallback = callback
}

// GetStats returns resource monitor statistics
func (rm *ResourceMonitor) GetStats() map[string]interface{} {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	return map[string]interface{}{
		"current_memory_mb":   float64(rm.currentMemoryUsage) / (1024 * 1024),
		"peak_memory_mb":      float64(rm.peakMemoryUsage) / (1024 * 1024),
		"memory_threshold_mb": float64(rm.memoryThreshold) / (1024 * 1024),
		"max_memory_mb":       float64(rm.maxMemoryUsage) / (1024 * 1024),
		"monitoring":          rm.monitoring,
		"last_check":          rm.lastMonitorTime.Format(time.RFC3339),
		"avg_cpu_usage":       rm.avgCPUUsage,
	}
}

// ForceGarbageCollection forces garbage collection and reports memory reduction
func (rm *ResourceMonitor) ForceGarbageCollection() (int64, int64) {
	var before, after runtime.MemStats

	runtime.ReadMemStats(&before)
	runtime.GC()
	runtime.ReadMemStats(&after)

	beforeMB := int64(before.Alloc)
	afterMB := int64(after.Alloc)

	rm.mutex.Lock()
	rm.currentMemoryUsage = afterMB
	rm.mutex.Unlock()

	log.Printf("ResourceMonitor: GC completed - freed %.1f MB (%.1f MB -> %.1f MB)",
		float64(beforeMB-afterMB)/(1024*1024),
		float64(beforeMB)/(1024*1024),
		float64(afterMB)/(1024*1024))

	return beforeMB, afterMB
}

// IsMemoryUsageHigh returns true if memory usage is above threshold
func (rm *ResourceMonitor) IsMemoryUsageHigh() bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.currentMemoryUsage > rm.memoryThreshold
}

// GetCurrentMemoryUsage returns current memory usage in bytes
func (rm *ResourceMonitor) GetCurrentMemoryUsage() int64 {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.currentMemoryUsage
}

// GetPeakMemoryUsage returns peak memory usage in bytes
func (rm *ResourceMonitor) GetPeakMemoryUsage() int64 {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return rm.peakMemoryUsage
}

// SetMemoryThreshold sets a new memory threshold
func (rm *ResourceMonitor) SetMemoryThreshold(threshold int64) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.memoryThreshold = threshold
}

// SetMonitorInterval sets the monitoring interval
func (rm *ResourceMonitor) SetMonitorInterval(interval time.Duration) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.monitorInterval = interval
}

// Helper functions for pattern matching optimization

// OptimizePatterns optimizes ignore patterns for better performance
func OptimizePatterns(patterns []string) []string {
	// Remove duplicates
	seen := make(map[string]bool)
	optimized := make([]string, 0, len(patterns))

	for _, pattern := range patterns {
		if !seen[pattern] {
			seen[pattern] = true
			optimized = append(optimized, pattern)
		}
	}

	// Sort patterns by specificity (more specific patterns first)
	// This helps with early termination in matching
	for i := 0; i < len(optimized)-1; i++ {
		for j := i + 1; j < len(optimized); j++ {
			if isMoreSpecific(optimized[j], optimized[i]) {
				optimized[i], optimized[j] = optimized[j], optimized[i]
			}
		}
	}

	return optimized
}

// isMoreSpecific returns true if pattern a is more specific than pattern b
func isMoreSpecific(a, b string) bool {
	// Exact matches are more specific than wildcards
	aHasWildcard := strings.ContainsAny(a, "*?[]")
	bHasWildcard := strings.ContainsAny(b, "*?[]")

	if !aHasWildcard && bHasWildcard {
		return true
	}
	if aHasWildcard && !bHasWildcard {
		return false
	}

	// Longer patterns are generally more specific
	return len(a) > len(b)
}

// BenchmarkPathMatching provides performance benchmarking for path matching
func BenchmarkPathMatching(filter *PathFilter, testPaths []string) map[string]interface{} {
	if len(testPaths) == 0 {
		return map[string]interface{}{"error": "no test paths provided"}
	}

	startTime := time.Now()
	matches := 0

	for _, path := range testPaths {
		if filter.ShouldIgnore(path) {
			matches++
		}
	}

	totalTime := time.Since(startTime)
	avgTime := totalTime / time.Duration(len(testPaths))

	return map[string]interface{}{
		"total_paths":          len(testPaths),
		"matches":              matches,
		"total_time_ms":        totalTime.Milliseconds(),
		"avg_time_per_path_ns": avgTime.Nanoseconds(),
		"paths_per_second":     float64(len(testPaths)) / totalTime.Seconds(),
	}
}

// GenerateTestPaths generates test paths for performance benchmarking
func GenerateTestPaths(count int) []string {
	paths := make([]string, count)

	// Common path patterns for testing
	patterns := []string{
		"src/main.go",
		"tests/unit_test.go",
		"node_modules/package/index.js",
		".git/objects/abc123",
		"build/output.bin",
		"docs/README.md",
		"config/app.yaml",
		"scripts/deploy.sh",
		"vendor/lib/library.a",
		".vscode/settings.json",
	}

	for i := 0; i < count; i++ {
		base := patterns[i%len(patterns)]
		// Add some variation
		paths[i] = fmt.Sprintf("project%d/%s", i%10, base)
	}

	return paths
}

// CreateOptimizedFilter creates an optimized path filter with performance tuning
func CreateOptimizedFilter(patterns []string) (*PathFilter, error) {
	// Optimize patterns first
	optimizedPatterns := OptimizePatterns(patterns)

	filter, err := NewPathFilter(optimizedPatterns)
	if err != nil {
		return nil, err
	}

	log.Printf("PathFilter: Optimized %d patterns down to %d patterns",
		len(patterns), len(optimizedPatterns))

	return filter, nil
}
