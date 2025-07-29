// SubProject Router Architecture Design
// **Depth 2** - Core Routing Interfaces and Architecture Design
// 
// This file contains the comprehensive design for SubProjectRouter interfaces,
// data structures, and request routing architecture that orchestrates 
// sub-project resolution and LSP client management.

package design

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// =============================================================================
// CORE INTERFACES
// =============================================================================

// SubProjectRouter defines the primary routing interface for project-aware LSP routing
type SubProjectRouter interface {
	// Core routing methods for the 6 supported LSP operations
	RouteRequest(ctx context.Context, method string, params json.RawMessage) (interface{}, error)
	RouteDefinition(ctx context.Context, params *LSPTextDocumentPositionParams) (*LSPLocation, error)
	RouteReferences(ctx context.Context, params *LSPReferenceParams) ([]LSPLocation, error)
	RouteHover(ctx context.Context, params *LSPTextDocumentPositionParams) (*LSPHover, error)
	RouteDocumentSymbol(ctx context.Context, params *LSPDocumentSymbolParams) ([]LSPDocumentSymbol, error)
	RouteWorkspaceSymbol(ctx context.Context, params *LSPWorkspaceSymbolParams) ([]LSPSymbolInformation, error)
	RouteCompletion(ctx context.Context, params *LSPCompletionParams) (*LSPCompletionList, error)
	
	// Advanced routing capabilities
	RouteBatch(ctx context.Context, requests []*LSPRequest) ([]*LSPResponse, error)
	RouteWithFallback(ctx context.Context, request *LSPRequest, fallbackStrategies []FallbackStrategy) (*LSPResponse, error)
	
	// Project resolution and management
	ResolveSubProject(ctx context.Context, fileURI string) (*SubProject, error)
	GetProjectHierarchy(ctx context.Context, fileURI string) (*ProjectHierarchy, error)
	RefreshProjects(ctx context.Context) error
	
	// Client management
	GetClientForProject(ctx context.Context, project *SubProject, language string) (LSPClient, error)
	GetAvailableClients(ctx context.Context, project *SubProject) ([]ClientInfo, error)
	ValidateClientHealth(ctx context.Context, client LSPClient) (*ClientHealthStatus, error)
	
	// Lifecycle management
	Initialize(config *RouterConfig) error
	Start(ctx context.Context) error
	Shutdown() error
	Reload(config *RouterConfig) error
	
	// Observability and debugging
	GetMetrics() *RouterMetrics
	GetDebugInfo() *RouterDebugInfo
	EnableDebugMode(enabled bool)
}

// RouterConfig defines comprehensive configuration for the SubProjectRouter
type RouterConfig interface {
	// Workspace configuration
	GetWorkspaceRoot() string
	GetWorkspaceRoots() []string
	GetProjectDiscoveryDepth() int
	
	// Project configuration
	GetProjects() []*SubProject
	GetProjectOverrides() map[string]*ProjectOverride
	GetExcludedPaths() []string
	GetIncludedPatterns() []string
	
	// Client configuration
	GetClientConfigs() map[string]*ClientConfig
	GetGlobalClientSettings() *GlobalClientSettings
	GetLanguageSpecificSettings() map[string]*LanguageSettings
	
	// Routing configuration
	GetRoutingRules() []*RoutingRule
	GetFallbackStrategies() []*FallbackStrategy
	GetLoadBalancingConfig() *LoadBalancingConfig
	
	// Performance configuration
	GetCacheConfig() *CacheConfig
	GetTimeoutConfig() *TimeoutConfig
	GetConcurrencyLimits() *ConcurrencyLimits
	
	// Observability configuration
	GetMetricsConfig() *MetricsConfig
	GetLoggingConfig() *LoggingConfig
	GetTracingConfig() *TracingConfig
}

// =============================================================================
// DATA STRUCTURES
// =============================================================================

// SubProject represents a detected sub-project within the workspace
type SubProject struct {
	// Identity and metadata
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Root          string                 `json:"root"`
	RelativePath  string                 `json:"relative_path"`
	ProjectType   ProjectType            `json:"project_type"`
	DetectedAt    time.Time              `json:"detected_at"`
	LastModified  time.Time              `json:"last_modified"`
	
	// Language and technology stack
	Languages     []LanguageInfo         `json:"languages"`
	PrimaryLang   string                 `json:"primary_language"`
	BuildSystem   BuildSystemInfo        `json:"build_system"`
	Dependencies  []DependencyInfo       `json:"dependencies"`
	
	// File organization
	SourceDirs    []string               `json:"source_dirs"`
	TestDirs      []string               `json:"test_dirs"`
	ConfigFiles   []string               `json:"config_files"`
	IgnoreFiles   []string               `json:"ignore_files"`
	
	// Hierarchy and relationships
	Parent        *SubProject            `json:"parent,omitempty"`
	Children      []*SubProject          `json:"children,omitempty"`
	Dependencies  []*ProjectDependency   `json:"project_dependencies,omitempty"`
	
	// LSP configuration
	LSPClients    map[string]*ClientRef  `json:"lsp_clients"`
	ClientPrefs   *ClientPreferences     `json:"client_preferences"`
	
	// Performance and caching
	CacheEnabled  bool                   `json:"cache_enabled"`
	IndexStatus   IndexStatus            `json:"index_status"`
	Metrics       *ProjectMetrics        `json:"metrics,omitempty"`
	
	// Thread safety
	mu            sync.RWMutex           `json:"-"`
}

// ProjectHierarchy represents the hierarchical structure of projects
type ProjectHierarchy struct {
	Root         *SubProject            `json:"root"`
	AllProjects  map[string]*SubProject `json:"all_projects"`
	ByLanguage   map[string][]*SubProject `json:"by_language"`
	ByType       map[ProjectType][]*SubProject `json:"by_type"`
	Dependencies *DependencyGraph       `json:"dependencies"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// RoutingDecision represents the result of request routing analysis
type RoutingDecision struct {
	// Core routing information
	Method        string                 `json:"method"`
	TargetProject *SubProject           `json:"target_project"`
	TargetClient  LSPClient             `json:"target_client"`
	Strategy      RoutingStrategy       `json:"strategy"`
	
	// Request processing
	RequestContext *RequestContext      `json:"request_context"`
	ProcessingMode ProcessingMode       `json:"processing_mode"`
	Timeout        time.Duration        `json:"timeout"`
	
	// Fallback and alternatives
	FallbackOptions []*FallbackOption   `json:"fallback_options"`
	AlternativeRoutes []*AlternativeRoute `json:"alternative_routes"`
	
	// Metadata and observability
	RoutingLatency time.Duration        `json:"routing_latency"`
	DecisionFactors []DecisionFactor    `json:"decision_factors"`
	DebugInfo      map[string]interface{} `json:"debug_info,omitempty"`
}

// =============================================================================
// REQUEST PROCESSING PIPELINE
// =============================================================================

// RequestProcessor defines the interface for request processing pipeline stages
type RequestProcessor interface {
	Process(ctx context.Context, request *LSPRequest) (*LSPRequest, error)
	Name() string
	Priority() int
}

// RequestPipeline orchestrates the complete request processing workflow
type RequestPipeline struct {
	// Processing stages
	validators    []RequestValidator
	preprocessors []RequestPreprocessor
	resolvers     []ProjectResolver
	routers       []RequestRouter
	postprocessors []RequestPostprocessor
	
	// Configuration and metrics
	config        *PipelineConfig
	metrics       *PipelineMetrics
	logger        Logger
	
	// Thread safety
	mu            sync.RWMutex
}

// Core processing stages
type RequestValidator interface {
	Validate(ctx context.Context, request *LSPRequest) error
}

type RequestPreprocessor interface {
	Preprocess(ctx context.Context, request *LSPRequest) (*LSPRequest, error)
}

type ProjectResolver interface {
	Resolve(ctx context.Context, request *LSPRequest) (*SubProject, error)
}

type RequestRouter interface {
	Route(ctx context.Context, request *LSPRequest, project *SubProject) (*RoutingDecision, error)
}

type RequestPostprocessor interface {
	Postprocess(ctx context.Context, response *LSPResponse) (*LSPResponse, error)
}

// =============================================================================
// ROUTING STRATEGIES
// =============================================================================

// RoutingStrategy defines how requests are routed to LSP clients
type RoutingStrategy interface {
	Name() string
	Route(ctx context.Context, request *LSPRequest, candidates []LSPClient) (*RoutingDecision, error)
	CanHandle(method string) bool
	Priority() int
}

// Built-in routing strategies
type SingleTargetStrategy struct {
	selectBest    ClientSelector
	fallback      FallbackStrategy
}

type LoadBalancedStrategy struct {
	balancer      LoadBalancer
	healthChecker HealthChecker
}

type ParallelAggregateStrategy struct {
	aggregator    ResponseAggregator
	timeout       time.Duration
}

type PrimaryWithFallbackStrategy struct {
	primarySelector   ClientSelector
	fallbackSelectors []ClientSelector
	maxRetries        int
}

// =============================================================================
// CLIENT MANAGEMENT
// =============================================================================

// LSPClient defines the interface for Language Server Protocol clients
type LSPClient interface {
	// Core LSP operations
	SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
	SendNotification(ctx context.Context, method string, params interface{}) error
	
	// Lifecycle management
	Initialize(ctx context.Context, params *InitializeParams) error
	Shutdown(ctx context.Context) error
	IsHealthy() bool
	
	// Configuration and metadata
	GetLanguage() string
	GetCapabilities() *ServerCapabilities
	GetProject() *SubProject
	GetConfig() *ClientConfig
	
	// Performance and observability
	GetMetrics() *ClientMetrics
	GetStatus() *ClientStatus
}

// ClientManager orchestrates LSP client lifecycle and selection
type ClientManager interface {
	// Client lifecycle
	CreateClient(ctx context.Context, config *ClientConfig, project *SubProject) (LSPClient, error)
	GetClient(clientID string) (LSPClient, bool)
	RemoveClient(clientID string) error
	
	// Client selection and management
	SelectBestClient(ctx context.Context, request *LSPRequest, project *SubProject) (LSPClient, error)
	GetAvailableClients(project *SubProject, language string) ([]LSPClient, error)
	ValidateClients(ctx context.Context) ([]*ClientValidationResult, error)
	
	// Health and monitoring
	MonitorHealth(ctx context.Context) error
	GetHealthStatus() map[string]*ClientHealthStatus
	CleanupStaleClients(ctx context.Context) error
}

// =============================================================================
// ERROR HANDLING AND FALLBACK
// =============================================================================

// RouterError defines comprehensive error handling for routing operations
type RouterError struct {
	Type        RouterErrorType        `json:"type"`
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Context     *ErrorContext          `json:"context,omitempty"`
	Cause       error                  `json:"cause,omitempty"`
	Recoverable bool                   `json:"recoverable"`
	Timestamp   time.Time              `json:"timestamp"`
}

type RouterErrorType string

const (
	ErrorTypeProjectResolution RouterErrorType = "project_resolution"
	ErrorTypeClientSelection   RouterErrorType = "client_selection"
	ErrorTypeClientCommunication RouterErrorType = "client_communication"
	ErrorTypeTimeout          RouterErrorType = "timeout"
	ErrorTypeConfiguration    RouterErrorType = "configuration"
	ErrorTypeValidation       RouterErrorType = "validation"
	ErrorTypeInternal         RouterErrorType = "internal"
)

// FallbackStrategy defines strategies for handling routing failures
type FallbackStrategy interface {
	Name() string
	CanHandle(err error) bool
	Execute(ctx context.Context, request *LSPRequest, originalError error) (*LSPResponse, error)
	Priority() int
}

// Built-in fallback strategies
type WorkspaceLevelFallback struct {
	workspaceClient LSPClient
}

type ReadOnlyModeFallback struct {
	staticResponses map[string]interface{}
}

type CachedResponseFallback struct {
	cache ResponseCache
	ttl   time.Duration
}

// =============================================================================
// METRICS AND OBSERVABILITY
// =============================================================================

// RouterMetrics provides comprehensive metrics for routing operations
type RouterMetrics struct {
	// Request metrics
	RequestCount        int64                    `json:"request_count"`
	RequestLatency      *LatencyMetrics         `json:"request_latency"`
	RequestsByMethod    map[string]int64        `json:"requests_by_method"`
	RequestsByProject   map[string]int64        `json:"requests_by_project"`
	
	// Routing metrics
	RoutingDecisions    int64                   `json:"routing_decisions"`
	RoutingLatency      *LatencyMetrics         `json:"routing_latency"`
	SuccessfulRoutes    int64                   `json:"successful_routes"`
	FailedRoutes        int64                   `json:"failed_routes"`
	
	// Client metrics
	ActiveClients       int                     `json:"active_clients"`
	ClientUtilization   map[string]float64      `json:"client_utilization"`
	ClientErrors        map[string]int64        `json:"client_errors"`
	
	// Project metrics
	ProjectCount        int                     `json:"project_count"`
	ProjectResolutions  int64                   `json:"project_resolutions"`
	ResolutionCache     *CacheMetrics           `json:"resolution_cache"`
	
	// Performance metrics
	MemoryUsage         *MemoryMetrics          `json:"memory_usage"`
	GoroutineCount      int                     `json:"goroutine_count"`
	
	// Timestamp
	CollectedAt         time.Time               `json:"collected_at"`
}

// =============================================================================
// INTEGRATION ARCHITECTURE
// =============================================================================

// GatewayIntegration defines integration points with existing gateway
type GatewayIntegration struct {
	// Handler integration at /home/skawn/work/lsp-gateway/internal/gateway/handlers.go:889
	HandlerIntegration *HandlerIntegration
	
	// Middleware integration
	Middlewares []Middleware
	
	// Configuration integration
	ConfigIntegration *ConfigIntegration
	
	// Event system integration
	EventSystem EventSystem
}

// HandlerIntegration defines how SubProjectRouter integrates with existing handlers
type HandlerIntegration struct {
	// Request interception points
	PreRouting  []RequestInterceptor
	PostRouting []ResponseInterceptor
	
	// Gateway method overrides
	MethodOverrides map[string]MethodHandler
	
	// Legacy compatibility
	LegacyFallback LegacyHandler
}

// Middleware defines cross-cutting concerns
type Middleware interface {
	Name() string
	Process(ctx context.Context, request *LSPRequest, next MiddlewareFunc) (*LSPResponse, error)
	Priority() int
}

type MiddlewareFunc func(ctx context.Context, request *LSPRequest) (*LSPResponse, error)

// Built-in middleware implementations
type LoggingMiddleware struct {
	logger Logger
	config *LoggingConfig
}

type MetricsMiddleware struct {
	collector MetricsCollector
	config    *MetricsConfig
}

type TracingMiddleware struct {
	tracer Tracer
	config *TracingConfig
}

type AuthenticationMiddleware struct {
	authenticator Authenticator
	config        *AuthConfig
}

type RateLimitingMiddleware struct {
	limiter RateLimiter
	config  *RateLimitConfig
}

// =============================================================================
// CONFIGURATION SCHEMA
// =============================================================================

// RouterConfigSchema defines the complete configuration structure
type RouterConfigSchema struct {
	// Router configuration
	Router RouterSettings `yaml:"router" json:"router"`
	
	// Workspace configuration
	Workspace WorkspaceSettings `yaml:"workspace" json:"workspace"`
	
	// Project configuration
	Projects ProjectSettings `yaml:"projects" json:"projects"`
	
	// Client configuration
	Clients ClientSettings `yaml:"clients" json:"clients"`
	
	// Performance configuration
	Performance PerformanceSettings `yaml:"performance" json:"performance"`
	
	// Observability configuration
	Observability ObservabilitySettings `yaml:"observability" json:"observability"`
}

// Example configuration structures
type RouterSettings struct {
	DefaultStrategy    string                    `yaml:"default_strategy" json:"default_strategy"`
	FallbackStrategies []string                  `yaml:"fallback_strategies" json:"fallback_strategies"`
	RoutingRules       []*RoutingRuleConfig      `yaml:"routing_rules" json:"routing_rules"`
	MethodStrategies   map[string]string         `yaml:"method_strategies" json:"method_strategies"`
}

type WorkspaceSettings struct {
	Roots              []string                  `yaml:"roots" json:"roots"`
	ExcludePaths       []string                  `yaml:"exclude_paths" json:"exclude_paths"`
	IncludePatterns    []string                  `yaml:"include_patterns" json:"include_patterns"`
	DiscoveryDepth     int                      `yaml:"discovery_depth" json:"discovery_depth"`
	WatchForChanges    bool                     `yaml:"watch_for_changes" json:"watch_for_changes"`
}

type ProjectSettings struct {
	AutoDiscovery      bool                     `yaml:"auto_discovery" json:"auto_discovery"`
	DetectionRules     []*DetectionRuleConfig   `yaml:"detection_rules" json:"detection_rules"`
	Overrides          map[string]*ProjectOverride `yaml:"overrides" json:"overrides"`
	HierarchyDepth     int                      `yaml:"hierarchy_depth" json:"hierarchy_depth"`
}

// =============================================================================
// CONCURRENCY AND THREADING
// =============================================================================

// ConcurrencyManager handles thread-safe routing operations
type ConcurrencyManager struct {
	// Resource pools
	requestPool    *RequestPool
	responsePool   *ResponsePool
	clientPool     *ClientPool
	
	// Synchronization primitives
	routingMutex   sync.RWMutex
	clientMutex    sync.RWMutex
	projectMutex   sync.RWMutex
	
	// Concurrency limits
	maxConcurrentRequests int
	maxConcurrentClients  int
	
	// Request queuing
	requestQueue   chan *QueuedRequest
	workerPool     []*RequestWorker
	
	// Deadlock prevention
	timeouts       *TimeoutManager
	circuitBreaker *CircuitBreaker
}

// RequestPool manages request object pooling for memory efficiency
type RequestPool struct {
	pool sync.Pool
}

// ResponsePool manages response object pooling
type ResponsePool struct {
	pool sync.Pool
}

// ClientPool manages LSP client connection pooling
type ClientPool struct {
	clients map[string]LSPClient
	mutex   sync.RWMutex
	maxSize int
}

// =============================================================================
// IMPLEMENTATION NOTES
// =============================================================================

/*
THREAD-SAFETY CONSIDERATIONS:
- All routing operations use appropriate locking mechanisms
- Request processing is designed for concurrent execution
- Client management includes connection pooling and lifecycle management
- Project resolution caching is thread-safe with optimistic locking

INTEGRATION WITH EXISTING GATEWAY:
- Hooks into existing handlers.go:889 for request interception
- Maintains backward compatibility with current routing logic
- Extends SmartRouter functionality with project-aware capabilities
- Leverages existing transport layer and client management

ERROR HANDLING STRATEGY:
- Comprehensive error taxonomy with recovery strategies
- Fallback routing with multiple levels (project -> workspace -> static)
- Circuit breaker pattern for client health management
- Timeout and retry logic with exponential backoff

PERFORMANCE OPTIMIZATIONS:
- Memory pooling for request/response objects
- Lazy loading of project hierarchies
- Intelligent caching with TTL and invalidation
- Connection pooling for LSP clients
- Concurrent request processing with proper resource isolation

OBSERVABILITY FRAMEWORK:
- Comprehensive metrics collection and reporting
- Distributed tracing support for request flows
- Debug logging with configurable verbosity
- Health check endpoints for monitoring
- Performance profiling integration

CONFIGURATION MANAGEMENT:
- Hierarchical configuration with environment overrides
- Hot-reloading support for runtime configuration changes
- Validation and schema enforcement
- Migration support for configuration upgrades
*/