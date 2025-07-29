package core

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"lsp-gateway/internal/setup"
)

// MemoryManager handles memory allocation, monitoring, and cleanup for the shared core
type MemoryManager struct {
	// Configuration
	maxMemoryBytes    int64
	gcThreshold       float64  // Trigger GC when memory usage exceeds this percentage
	compactionThreshold float64 // Trigger compaction when memory usage exceeds this percentage
	
	// State tracking
	mu                sync.RWMutex
	currentUsage      int64
	peakUsage         int64
	allocationCount   int64
	deallocationCount int64
	gcCount           int64
	
	// Component references
	cache             SCIPCache
	registry          LSPServerRegistry
	projectMgr        ProjectManager
	
	// Monitoring
	logger            *setup.SetupLogger
	monitoringActive  bool
	stopCh            chan struct{}
	
	// Memory pools for frequently allocated objects
	symbolPool        sync.Pool
	referencePool     sync.Pool
	locationPool      sync.Pool
	
	// Cleanup strategies
	cleanupStrategies []CleanupStrategy
}

// CleanupStrategy defines an interface for memory cleanup strategies
type CleanupStrategy interface {
	Name() string
	CanCleanup(usage MemoryUsage) bool
	Cleanup(ctx context.Context) (int64, error)
	Priority() int
}

// MemoryUsage provides current memory usage information
type MemoryUsage struct {
	CurrentBytes      int64     `json:"current_bytes"`
	MaxBytes          int64     `json:"max_bytes"`
	UsagePercentage   float64   `json:"usage_percentage"`
	PeakBytes         int64     `json:"peak_bytes"`
	AllocatedObjects  int64     `json:"allocated_objects"`
	DeallocatedObjects int64    `json:"deallocated_objects"`
	GCCount           int64     `json:"gc_count"`
	LastGC            time.Time `json:"last_gc"`
	
	// Component-specific usage
	CacheUsage        int64     `json:"cache_usage"`
	RegistryUsage     int64     `json:"registry_usage"`
	ProjectsUsage     int64     `json:"projects_usage"`
}

// NewMemoryManager creates a new memory manager
func NewMemoryManager(maxMemoryBytes int64, cache SCIPCache, registry LSPServerRegistry, projectMgr ProjectManager) *MemoryManager {
	mm := &MemoryManager{
		maxMemoryBytes:      maxMemoryBytes,
		gcThreshold:         0.75,  // 75%
		compactionThreshold: 0.85,  // 85%
		cache:               cache,
		registry:            registry,
		projectMgr:          projectMgr,
		logger:              setup.NewSetupLogger(nil),
		stopCh:              make(chan struct{}),
		cleanupStrategies:   make([]CleanupStrategy, 0),
	}
	
	// Initialize object pools
	mm.initializePools()
	
	// Register default cleanup strategies
	mm.registerDefaultStrategies()
	
	return mm
}

// Start begins memory monitoring and management
func (mm *MemoryManager) Start(ctx context.Context) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	if mm.monitoringActive {
		return fmt.Errorf("memory manager already started")
	}
	
	mm.logger.Info("Starting memory manager")
	mm.monitoringActive = true
	
	// Start monitoring goroutine
	go mm.monitoringLoop()
	
	return nil
}

// Stop stops memory monitoring
func (mm *MemoryManager) Stop() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	if !mm.monitoringActive {
		return nil
	}
	
	mm.logger.Info("Stopping memory manager")
	close(mm.stopCh)
	mm.monitoringActive = false
	
	return nil
}

// GetUsage returns current memory usage information
func (mm *MemoryManager) GetUsage() MemoryUsage {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	usage := MemoryUsage{
		CurrentBytes:       int64(m.Alloc),
		MaxBytes:          mm.maxMemoryBytes,
		UsagePercentage:   float64(m.Alloc) / float64(mm.maxMemoryBytes) * 100,
		PeakBytes:         mm.peakUsage,
		AllocatedObjects:  mm.allocationCount,
		DeallocatedObjects: mm.deallocationCount,
		GCCount:           mm.gcCount,
		LastGC:            time.Unix(0, int64(m.LastGC)),
		
		// Component-specific usage
		CacheUsage:        mm.cache.GetMemoryUsage(),
		RegistryUsage:     mm.estimateRegistryUsage(),
		ProjectsUsage:     mm.estimateProjectsUsage(),
	}
	
	// Update peak usage
	if usage.CurrentBytes > mm.peakUsage {
		mm.peakUsage = usage.CurrentBytes
	}
	
	return usage
}

// ForceGC forces garbage collection
func (mm *MemoryManager) ForceGC() {
	mm.logger.Debug("Forcing garbage collection")
	
	runtime.GC()
	
	mm.mu.Lock()
	mm.gcCount++
	mm.mu.Unlock()
	
	mm.logger.WithField("gc_count", mm.gcCount).Debug("Garbage collection completed")
}

// ForceCleanup forces memory cleanup using all available strategies
func (mm *MemoryManager) ForceCleanup(ctx context.Context) (int64, error) {
	mm.logger.Info("Forcing memory cleanup")
	
	usage := mm.GetUsage()
	totalFreed := int64(0)
	
	// Execute cleanup strategies in priority order
	for _, strategy := range mm.cleanupStrategies {
		if strategy.CanCleanup(usage) {
			freed, err := strategy.Cleanup(ctx)
			if err != nil {
				mm.logger.WithError(err).WithField("strategy", strategy.Name()).
					Error("Cleanup strategy failed")
				continue
			}
			
			totalFreed += freed
			mm.logger.WithField("strategy", strategy.Name()).
				WithField("freed_bytes", freed).
				Debug("Cleanup strategy executed")
		}
	}
	
	// Force GC after cleanup
	mm.ForceGC()
	
	mm.logger.WithField("total_freed", totalFreed).Info("Memory cleanup completed")
	return totalFreed, nil
}

// Object pool methods for memory efficiency

func (mm *MemoryManager) GetSymbol() *Symbol {
	return mm.symbolPool.Get().(*Symbol)
}

func (mm *MemoryManager) PutSymbol(s *Symbol) {
	// Reset symbol before returning to pool
	*s = Symbol{}
	mm.symbolPool.Put(s)
}

func (mm *MemoryManager) GetReference() *Reference {
	return mm.referencePool.Get().(*Reference)
}

func (mm *MemoryManager) PutReference(r *Reference) {
	// Reset reference before returning to pool
	*r = Reference{}
	mm.referencePool.Put(r)
}

func (mm *MemoryManager) GetLocation() *Location {
	return mm.locationPool.Get().(*Location)
}

func (mm *MemoryManager) PutLocation(l *Location) {
	// Reset location before returning to pool
	*l = Location{}
	mm.locationPool.Put(l)
}

// Private methods

func (mm *MemoryManager) initializePools() {
	mm.symbolPool = sync.Pool{
		New: func() interface{} {
			return &Symbol{}
		},
	}
	
	mm.referencePool = sync.Pool{
		New: func() interface{} {
			return &Reference{}
		},
	}
	
	mm.locationPool = sync.Pool{
		New: func() interface{} {
			return &Location{}
		},
	}
}

func (mm *MemoryManager) registerDefaultStrategies() {
	// Register cleanup strategies in priority order (highest priority first)
	mm.cleanupStrategies = []CleanupStrategy{
		NewCacheEvictionStrategy(mm.cache),
		NewIdleWorkspaceCleanupStrategy(mm.projectMgr),
		NewIdleServerCleanupStrategy(mm.registry),
		NewObjectPoolCleanupStrategy(mm),
	}
}

func (mm *MemoryManager) monitoringLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-mm.stopCh:
			return
		case <-ticker.C:
			mm.checkMemoryPressure()
		}
	}
}

func (mm *MemoryManager) checkMemoryPressure() {
	usage := mm.GetUsage()
	
	// Log memory usage periodically
	mm.logger.WithFields(map[string]interface{}{
		"current_mb":      usage.CurrentBytes / (1024 * 1024),
		"max_mb":          usage.MaxBytes / (1024 * 1024),
		"usage_percent":   usage.UsagePercentage,
		"cache_mb":        usage.CacheUsage / (1024 * 1024),
	}).Debug("Memory usage report")
	
	// Check if we need to take action
	if usage.UsagePercentage > mm.compactionThreshold*100 {
		mm.logger.WithField("usage_percent", usage.UsagePercentage).
			Warn("Memory usage exceeds compaction threshold, forcing cleanup")
		
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if _, err := mm.ForceCleanup(ctx); err != nil {
			mm.logger.WithError(err).Error("Failed to cleanup memory under pressure")
		}
	} else if usage.UsagePercentage > mm.gcThreshold*100 {
		mm.logger.WithField("usage_percent", usage.UsagePercentage).
			Debug("Memory usage exceeds GC threshold, forcing garbage collection")
		
		mm.ForceGC()
	}
}

func (mm *MemoryManager) estimateRegistryUsage() int64 {
	// Rough estimation based on number of servers
	servers := mm.registry.GetAllServers()
	serverCount := 0
	
	for _, workspaceServers := range servers {
		serverCount += len(workspaceServers)
	}
	
	// Estimate ~1MB per server (rough approximation)
	return int64(serverCount) * 1024 * 1024
}

func (mm *MemoryManager) estimateProjectsUsage() int64 {
	// Rough estimation based on number of workspaces
	workspaces := mm.projectMgr.GetAllWorkspaces()
	
	// Estimate ~500KB per workspace (rough approximation)
	return int64(len(workspaces)) * 512 * 1024
}

// Cleanup strategy implementations

// CacheEvictionStrategy removes old cache entries
type CacheEvictionStrategy struct {
	cache SCIPCache
}

func NewCacheEvictionStrategy(cache SCIPCache) *CacheEvictionStrategy {
	return &CacheEvictionStrategy{cache: cache}
}

func (s *CacheEvictionStrategy) Name() string {
	return "cache_eviction"
}

func (s *CacheEvictionStrategy) CanCleanup(usage MemoryUsage) bool {
	return usage.CacheUsage > 0
}

func (s *CacheEvictionStrategy) Cleanup(ctx context.Context) (int64, error) {
	beforeUsage := s.cache.GetMemoryUsage()
	
	if err := s.cache.Compact(); err != nil {
		return 0, err
	}
	
	afterUsage := s.cache.GetMemoryUsage()
	freed := beforeUsage - afterUsage
	
	return freed, nil
}

func (s *CacheEvictionStrategy) Priority() int {
	return 100 // Highest priority
}

// IdleWorkspaceCleanupStrategy removes idle workspaces
type IdleWorkspaceCleanupStrategy struct {
	projectMgr ProjectManager
}

func NewIdleWorkspaceCleanupStrategy(projectMgr ProjectManager) *IdleWorkspaceCleanupStrategy {
	return &IdleWorkspaceCleanupStrategy{projectMgr: projectMgr}
}

func (s *IdleWorkspaceCleanupStrategy) Name() string {
	return "idle_workspace_cleanup"
}

func (s *IdleWorkspaceCleanupStrategy) CanCleanup(usage MemoryUsage) bool {
	workspaces := s.projectMgr.GetAllWorkspaces()
	return len(workspaces) > 0
}

func (s *IdleWorkspaceCleanupStrategy) Cleanup(ctx context.Context) (int64, error) {
	workspaces := s.projectMgr.GetAllWorkspaces()
	now := time.Now()
	maxIdleTime := 30 * time.Minute
	
	var removedWorkspaces []string
	for workspaceID, workspace := range workspaces {
		if now.Sub(workspace.LastAccessedAt) > maxIdleTime {
			if err := s.projectMgr.RemoveWorkspace(workspaceID); err == nil {
				removedWorkspaces = append(removedWorkspaces, workspaceID)
			}
		}
	}
	
	// Estimate freed memory (~500KB per workspace)
	freed := int64(len(removedWorkspaces)) * 512 * 1024
	
	return freed, nil
}

func (s *IdleWorkspaceCleanupStrategy) Priority() int {
	return 80
}

// IdleServerCleanupStrategy removes idle server connections
type IdleServerCleanupStrategy struct {
	registry LSPServerRegistry
}

func NewIdleServerCleanupStrategy(registry LSPServerRegistry) *IdleServerCleanupStrategy {
	return &IdleServerCleanupStrategy{registry: registry}
}

func (s *IdleServerCleanupStrategy) Name() string {
	return "idle_server_cleanup"
}

func (s *IdleServerCleanupStrategy) CanCleanup(usage MemoryUsage) bool {
	servers := s.registry.GetAllServers()
	return len(servers) > 0
}

func (s *IdleServerCleanupStrategy) Cleanup(ctx context.Context) (int64, error) {
	// This would require more sophisticated tracking of server usage
	// For now, we'll just return 0 as a placeholder
	return 0, nil
}

func (s *IdleServerCleanupStrategy) Priority() int {
	return 60
}

// ObjectPoolCleanupStrategy clears object pools
type ObjectPoolCleanupStrategy struct {
	memoryMgr *MemoryManager
}

func NewObjectPoolCleanupStrategy(memoryMgr *MemoryManager) *ObjectPoolCleanupStrategy {
	return &ObjectPoolCleanupStrategy{memoryMgr: memoryMgr}
}

func (s *ObjectPoolCleanupStrategy) Name() string {
	return "object_pool_cleanup"
}

func (s *ObjectPoolCleanupStrategy) CanCleanup(usage MemoryUsage) bool {
	return true // Can always clean up pools
}

func (s *ObjectPoolCleanupStrategy) Cleanup(ctx context.Context) (int64, error) {
	// Clear object pools to free up memory
	s.memoryMgr.symbolPool = sync.Pool{
		New: func() interface{} {
			return &Symbol{}
		},
	}
	
	s.memoryMgr.referencePool = sync.Pool{
		New: func() interface{} {
			return &Reference{}
		},
	}
	
	s.memoryMgr.locationPool = sync.Pool{
		New: func() interface{} {
			return &Location{}
		},
	}
	
	// Estimate freed memory (rough approximation)
	return 1024 * 1024, nil // ~1MB
}

func (s *ObjectPoolCleanupStrategy) Priority() int {
	return 20 // Lowest priority
}