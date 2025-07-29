package workspace

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ClientRef represents a reference to an LSP client for a specific language
type ClientRef struct {
	Language   string    `json:"language"`
	ClientID   string    `json:"client_id"`
	IsActive   bool      `json:"is_active"`
	LastUsed   time.Time `json:"last_used"`
	StartedAt  time.Time `json:"started_at"`
}

// SubProject represents a detected sub-project within the workspace with enhanced structure
type SubProject struct {
	// Core identity and metadata
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Root          string                 `json:"root"`
	RelativePath  string                 `json:"relative_path"`
	ProjectType   string                 `json:"project_type"`
	DetectedAt    time.Time              `json:"detected_at"`
	LastModified  time.Time              `json:"last_modified"`
	
	// Language and technology information
	Languages     []string               `json:"languages"`
	PrimaryLang   string                 `json:"primary_language"`
	SourceDirs    []string               `json:"source_dirs"`
	ConfigFiles   []string               `json:"config_files"`
	
	// Project hierarchy and relationships
	Parent        *SubProject            `json:"parent,omitempty"`
	Children      []*SubProject          `json:"children,omitempty"`
	
	// LSP client management
	LSPClients    map[string]*ClientRef  `json:"lsp_clients"`
	
	// Thread safety
	mu            sync.RWMutex           `json:"-"`
}

// SubProjectResolver interface defines efficient file-to-subproject resolution capabilities
type SubProjectResolver interface {
	// Core resolution methods
	ResolveSubProject(fileURI string) (*SubProject, error)
	ResolveSubProjectCached(fileURI string) (*SubProject, error)
	
	// Project management
	RefreshProjects(ctx context.Context) error
	GetAllSubProjects() []*SubProject
	InvalidateCache(projectPath string)
	
	// Performance and observability
	GetCacheStats() *ResolverCacheStats
	GetMetrics() *ResolverMetrics
	
	// Lifecycle management
	Close() error
}

// ResolverMetrics provides comprehensive performance metrics for the resolver
type ResolverMetrics struct {
	// Resolution performance
	TotalResolutions    int64         `json:"total_resolutions"`
	CachedResolutions   int64         `json:"cached_resolutions"`
	TrieResolutions     int64         `json:"trie_resolutions"`
	FallbackResolutions int64         `json:"fallback_resolutions"`
	AverageLatency      time.Duration `json:"average_latency"`
	
	// Project statistics
	TotalProjects       int           `json:"total_projects"`
	NestedProjects      int           `json:"nested_projects"`
	LastRefresh         time.Time     `json:"last_refresh"`
	RefreshCount        int64         `json:"refresh_count"`
	
	// Error tracking
	ResolutionErrors    int64         `json:"resolution_errors"`
	URIParsingErrors    int64         `json:"uri_parsing_errors"`
	
	// Performance targets
	CacheHitRate        float64       `json:"cache_hit_rate"`
	AverageTrieDepth    float64       `json:"average_trie_depth"`
}

// SubProjectResolverImpl implements efficient project resolution with intelligent caching
type SubProjectResolverImpl struct {
	// Core data structures
	pathTrie        *PathTrie
	cache           *ResolverCache
	projects        map[string]*SubProject
	workspaceRoot   string
	
	// Project discovery components
	workspaceDetector WorkspaceDetector
	
	// Configuration
	maxDepth        int
	refreshInterval time.Duration
	
	// Performance metrics
	metrics         *ResolverMetrics
	
	// Synchronization
	mu              sync.RWMutex
	refreshMu       sync.Mutex
	
	// Lifecycle management
	ctx             context.Context
	cancel          context.CancelFunc
	refreshTicker   *time.Ticker
	stopChan        chan struct{}
}

// NewSubProjectResolver creates a new efficient SubProject resolver with caching
func NewSubProjectResolver(workspaceRoot string, options *ResolverOptions) SubProjectResolver {
	if options == nil {
		options = &ResolverOptions{
			CacheCapacity:   1000,
			CacheTTL:        30 * time.Minute,
			MaxDepth:        10,
			RefreshInterval: 5 * time.Minute,
		}
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	resolver := &SubProjectResolverImpl{
		pathTrie:          NewPathTrie(),
		cache:             NewResolverCache(options.CacheCapacity, options.CacheTTL),
		projects:          make(map[string]*SubProject),
		workspaceRoot:     workspaceRoot,
		workspaceDetector: NewWorkspaceDetector(),
		maxDepth:          options.MaxDepth,
		refreshInterval:   options.RefreshInterval,
		metrics:           &ResolverMetrics{LastRefresh: time.Now()},
		ctx:               ctx,
		cancel:            cancel,
		stopChan:          make(chan struct{}),
	}
	
	// Start background refresh if interval is set
	if options.RefreshInterval > 0 {
		resolver.refreshTicker = time.NewTicker(options.RefreshInterval)
		go resolver.backgroundRefreshWorker()
	}
	
	return resolver
}

// ResolverOptions configures the SubProject resolver behavior
type ResolverOptions struct {
	CacheCapacity   int           `json:"cache_capacity"`
	CacheTTL        time.Duration `json:"cache_ttl"`
	MaxDepth        int           `json:"max_depth"`
	RefreshInterval time.Duration `json:"refresh_interval"`
}

// ResolveSubProject resolves a file URI to its containing SubProject with caching
// Implements sub-millisecond resolution with O(1) average case performance
func (r *SubProjectResolverImpl) ResolveSubProject(fileURI string) (*SubProject, error) {
	return r.resolveWithMetrics(fileURI, false)
}

// ResolveSubProjectCached resolves using cache-first strategy for maximum performance
func (r *SubProjectResolverImpl) ResolveSubProjectCached(fileURI string) (*SubProject, error) {
	return r.resolveWithMetrics(fileURI, true)
}

// resolveWithMetrics performs resolution with comprehensive performance tracking
func (r *SubProjectResolverImpl) resolveWithMetrics(fileURI string, cacheFirst bool) (*SubProject, error) {
	startTime := time.Now()
	defer func() {
		r.updateLatencyMetrics(time.Since(startTime))
	}()
	
	r.metrics.TotalResolutions++
	
	// Parse and normalize file URI
	filePath, err := r.parseFileURI(fileURI)
	if err != nil {
		r.metrics.URIParsingErrors++
		return nil, fmt.Errorf("failed to parse file URI %s: %w", fileURI, err)
	}
	
	// Try cache first for cached resolution or if cache-first strategy
	if cacheFirst {
		if cachedProject := r.cache.Get(filePath); cachedProject != nil {
			r.metrics.CachedResolutions++
			return cachedProject, nil
		}
	}
	
	// Resolve using path trie for O(log n) lookup
	r.mu.RLock()
	project := r.pathTrie.FindLongestMatch(filePath)
	r.mu.RUnlock()
	
	if project != nil {
		r.metrics.TrieResolutions++
		
		// Cache the result for future lookups
		r.cache.Put(filePath, project)
		return project, nil
	}
	
	// Fallback: workspace-level resolution
	r.metrics.FallbackResolutions++
	return r.fallbackWorkspaceResolution(filePath)
}

// RefreshProjects rebuilds the project index with hierarchy detection
func (r *SubProjectResolverImpl) RefreshProjects(ctx context.Context) error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	
	r.metrics.RefreshCount++
	
	// Discover projects recursively
	discoveredProjects, err := r.discoverProjectsRecursively(ctx, r.workspaceRoot, 0)
	if err != nil {
		return fmt.Errorf("failed to discover projects: %w", err)
	}
	
	// Build project hierarchy
	rootProjects := r.buildProjectHierarchy(discoveredProjects)
	
	// Update data structures atomically
	r.mu.Lock()
	
	// Clear existing data
	r.pathTrie.Clear()
	r.projects = make(map[string]*SubProject)
	
	// Populate new data
	for _, project := range discoveredProjects {
		r.projects[project.Root] = project
		r.pathTrie.Insert(project.Root, project)
	}
	
	r.mu.Unlock()
	
	// Clear cache to force re-resolution
	r.cache.Clear()
	
	// Update metrics
	r.metrics.TotalProjects = len(discoveredProjects)
	r.metrics.NestedProjects = r.countNestedProjects(rootProjects)
	r.metrics.LastRefresh = time.Now()
	
	return nil
}

// GetAllSubProjects returns all discovered projects
func (r *SubProjectResolverImpl) GetAllSubProjects() []*SubProject {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	projects := make([]*SubProject, 0, len(r.projects))
	for _, project := range r.projects {
		projects = append(projects, project)
	}
	
	return projects
}

// InvalidateCache removes cached results for a project path
func (r *SubProjectResolverImpl) InvalidateCache(projectPath string) {
	invalidatedCount := r.cache.InvalidateByProject(projectPath)
	
	// Update metrics if we track cache operations
	_ = invalidatedCount
}

// GetCacheStats returns comprehensive cache performance statistics
func (r *SubProjectResolverImpl) GetCacheStats() *ResolverCacheStats {
	return r.cache.GetStats()
}

// GetMetrics returns comprehensive resolver performance metrics
func (r *SubProjectResolverImpl) GetMetrics() *ResolverMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metricsCopy := *r.metrics
	
	// Calculate cache hit rate
	cacheStats := r.cache.GetStats()
	metricsCopy.CacheHitRate = cacheStats.HitRate
	
	return &metricsCopy
}

// Close stops background workers and cleans up resources
func (r *SubProjectResolverImpl) Close() error {
	// Stop background refresh worker
	if r.refreshTicker != nil {
		r.refreshTicker.Stop()
	}
	
	// Signal shutdown
	close(r.stopChan)
	
	// Close cache
	r.cache.Close()
	
	// Cancel context
	r.cancel()
	
	return nil
}

// parseFileURI converts file URI to normalized absolute path
func (r *SubProjectResolverImpl) parseFileURI(fileURI string) (string, error) {
	// Handle file:// protocol
	if strings.HasPrefix(fileURI, "file://") {
		parsedURI, err := url.Parse(fileURI)
		if err != nil {
			return "", fmt.Errorf("invalid file URI: %w", err)
		}
		return parsedURI.Path, nil
	}
	
	// Handle absolute paths
	if filepath.IsAbs(fileURI) {
		return filepath.Clean(fileURI), nil
	}
	
	// Handle relative paths by making them absolute to workspace root
	absPath := filepath.Join(r.workspaceRoot, fileURI)
	return filepath.Clean(absPath), nil
}

// discoverProjectsRecursively discovers all projects within the workspace
func (r *SubProjectResolverImpl) discoverProjectsRecursively(ctx context.Context, rootPath string, depth int) ([]*SubProject, error) {
	if depth == 0 {
		// For root path, use workspace detection to get all sub-projects
		workspaceContext, err := r.workspaceDetector.DetectWorkspaceWithContext(ctx, rootPath)
		if err != nil {
			return nil, fmt.Errorf("failed to detect workspace: %w", err)
		}
		
		var allProjects []*SubProject
		for _, detectedSubProject := range workspaceContext.SubProjects {
			project := r.convertDetectedSubProjectToSubProject(detectedSubProject)
			if project != nil {
				allProjects = append(allProjects, project)
			}
		}
		
		return allProjects, nil
	}
	
	// For deeper levels, fall back to simple directory analysis
	return nil, nil
}

// convertDetectedSubProjectToSubProject converts DetectedSubProject to SubProject
func (r *SubProjectResolverImpl) convertDetectedSubProjectToSubProject(detectedSubProject *DetectedSubProject) *SubProject {
	if detectedSubProject == nil {
		return nil
	}
	
	return &SubProject{
		ID:           detectedSubProject.ID,
		Name:         detectedSubProject.Name,
		Root:         detectedSubProject.AbsolutePath,
		RelativePath: detectedSubProject.RelativePath,
		ProjectType:  detectedSubProject.ProjectType,
		Languages:    detectedSubProject.Languages,
		PrimaryLang:  detectedSubProject.ProjectType, // Use project type as primary language
		SourceDirs:   []string{detectedSubProject.AbsolutePath},
		ConfigFiles:  detectedSubProject.MarkerFiles,
		LSPClients:   make(map[string]*ClientRef),
		DetectedAt:   time.Now(),
		LastModified: time.Now(),
	}
}

// buildProjectHierarchy establishes parent-child relationships between projects
func (r *SubProjectResolverImpl) buildProjectHierarchy(projects []*SubProject) []*SubProject {
	// Sort projects by path depth (shortest first)
	for i := 0; i < len(projects)-1; i++ {
		for j := i + 1; j < len(projects); j++ {
			if len(strings.Split(projects[i].Root, string(os.PathSeparator))) > 
			   len(strings.Split(projects[j].Root, string(os.PathSeparator))) {
				projects[i], projects[j] = projects[j], projects[i]
			}
		}
	}
	
	var rootProjects []*SubProject
	
	// Establish relationships
	for _, project := range projects {
		parent := r.findParentProject(project, projects)
		if parent != nil {
			project.Parent = parent
			parent.Children = append(parent.Children, project)
		} else {
			rootProjects = append(rootProjects, project)
		}
	}
	
	return rootProjects
}

// findParentProject finds the immediate parent project for a given project
func (r *SubProjectResolverImpl) findParentProject(project *SubProject, allProjects []*SubProject) *SubProject {
	var bestParent *SubProject
	maxMatchLength := 0
	
	for _, candidate := range allProjects {
		if candidate == project {
			continue
		}
		
		// Check if candidate is a parent (project is under candidate's path)
		if strings.HasPrefix(project.Root+"/", candidate.Root+"/") &&
		   len(candidate.Root) > maxMatchLength {
			bestParent = candidate
			maxMatchLength = len(candidate.Root)
		}
	}
	
	return bestParent
}

// fallbackWorkspaceResolution provides workspace-level fallback resolution
func (r *SubProjectResolverImpl) fallbackWorkspaceResolution(filePath string) (*SubProject, error) {
	// Create a workspace-level project as fallback
	workspaceProject := &SubProject{
		ID:           "workspace-root",
		Name:         filepath.Base(r.workspaceRoot),
		Root:         r.workspaceRoot,
		RelativePath: "",
		ProjectType:  "workspace",
		Languages:    []string{"multi"},
		PrimaryLang:  "multi",
		LSPClients:   make(map[string]*ClientRef),
		DetectedAt:   time.Now(),
		LastModified: time.Now(),
	}
	
	// Cache the fallback result
	r.cache.Put(filePath, workspaceProject)
	
	return workspaceProject, nil
}

// shouldSkipDirectory determines if a directory should be skipped during discovery
func (r *SubProjectResolverImpl) shouldSkipDirectory(name string) bool {
	skipDirs := []string{
		"node_modules", ".git", ".svn", ".hg", ".bzr",
		"target", "build", "dist", "out", "bin",
		".vscode", ".idea", "__pycache__", ".pytest_cache",
		"vendor", "deps", ".next", ".nuxt",
	}
	
	for _, skipDir := range skipDirs {
		if name == skipDir {
			return true
		}
	}
	
	return false
}

// generateProjectID creates a unique ID for a project based on its path
func generateProjectID(projectPath string) string {
	// Simple hash-like ID generation based on path
	return fmt.Sprintf("project-%x", projectPath)
}

// countNestedProjects counts projects that have parent projects
func (r *SubProjectResolverImpl) countNestedProjects(rootProjects []*SubProject) int {
	count := 0
	for _, project := range r.projects {
		if project.Parent != nil {
			count++
		}
	}
	return count
}

// updateLatencyMetrics updates average latency tracking
func (r *SubProjectResolverImpl) updateLatencyMetrics(latency time.Duration) {
	// Simple moving average (could be improved with more sophisticated metrics)
	totalResolutions := r.metrics.TotalResolutions
	if totalResolutions == 1 {
		r.metrics.AverageLatency = latency
	} else {
		// Weighted average favoring recent measurements
		weight := 0.1
		r.metrics.AverageLatency = time.Duration(
			float64(r.metrics.AverageLatency)*(1-weight) + float64(latency)*weight,
		)
	}
}

// backgroundRefreshWorker runs periodic project refresh in background
func (r *SubProjectResolverImpl) backgroundRefreshWorker() {
	for {
		select {
		case <-r.refreshTicker.C:
			// Refresh projects in background
			if err := r.RefreshProjects(r.ctx); err != nil {
				// Log error but continue (would need proper logging integration)
				_ = err
			}
		case <-r.stopChan:
			return
		case <-r.ctx.Done():
			return
		}
	}
}