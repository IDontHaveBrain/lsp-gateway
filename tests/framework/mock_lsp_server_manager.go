package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/transport"
)

// MockServerConfig provides comprehensive configuration for mock LSP server behavior
type MockServerConfig struct {
	// Protocol compliance configuration
	ProtocolVersion    string
	Capabilities       ServerCapabilities
	SupportedMethods   []string

	// Failure simulation configuration
	GlobalFailureRate     float64
	MethodFailureRates    map[string]float64
	TimeoutSimulation     TimeoutConfig
	MemoryPressure        MemoryPressureConfig
	CrashSimulation       CrashConfig
	CorruptionSimulation  CorruptionConfig
	CircuitBreakerConfig  CircuitBreakerConfig

	// Response behavior configuration
	ResponseDelays        ResponseDelayConfig
	ResponseGeneration    ResponseGenerationConfig
	HealthBehavior        HealthBehaviorConfig
	CapacityLimits        CapacityLimitsConfig

	// Content and workspace simulation
	WorkspaceContent      map[string]string // file URI -> content
	SymbolDatabase        SymbolDatabase
	ProjectStructure      ProjectStructure
}

// ServerCapabilities represents LSP server capabilities
type ServerCapabilities struct {
	TextDocumentSync                 interface{} `json:"textDocumentSync,omitempty"`
	CompletionProvider               interface{} `json:"completionProvider,omitempty"`
	HoverProvider                    bool        `json:"hoverProvider,omitempty"`
	SignatureHelpProvider            interface{} `json:"signatureHelpProvider,omitempty"`
	DefinitionProvider               bool        `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider           bool        `json:"typeDefinitionProvider,omitempty"`
	ImplementationProvider           bool        `json:"implementationProvider,omitempty"`
	ReferencesProvider               bool        `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider        bool        `json:"documentHighlightProvider,omitempty"`
	DocumentSymbolProvider           bool        `json:"documentSymbolProvider,omitempty"`
	CodeActionProvider               interface{} `json:"codeActionProvider,omitempty"`
	CodeLensProvider                 interface{} `json:"codeLensProvider,omitempty"`
	DocumentLinkProvider             interface{} `json:"documentLinkProvider,omitempty"`
	ColorProvider                    bool        `json:"colorProvider,omitempty"`
	DocumentFormattingProvider       bool        `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider  bool        `json:"documentRangeFormattingProvider,omitempty"`
	DocumentOnTypeFormattingProvider interface{} `json:"documentOnTypeFormattingProvider,omitempty"`
	RenameProvider                   interface{} `json:"renameProvider,omitempty"`
	FoldingRangeProvider             bool        `json:"foldingRangeProvider,omitempty"`
	ExecuteCommandProvider           interface{} `json:"executeCommandProvider,omitempty"`
	SelectionRangeProvider           bool        `json:"selectionRangeProvider,omitempty"`
	WorkspaceSymbolProvider          bool        `json:"workspaceSymbolProvider,omitempty"`
	Workspace                        interface{} `json:"workspace,omitempty"`
	SemanticTokensProvider           interface{} `json:"semanticTokensProvider,omitempty"`
	InlayHintProvider                interface{} `json:"inlayHintProvider,omitempty"`
	DiagnosticProvider               interface{} `json:"diagnosticProvider,omitempty"`
}

// TimeoutConfig configures timeout simulation
type TimeoutConfig struct {
	Enabled          bool
	TimeoutRate      float64
	MinTimeout       time.Duration
	MaxTimeout       time.Duration
	MethodTimeouts   map[string]time.Duration
}

// MemoryPressureConfig configures memory pressure simulation
type MemoryPressureConfig struct {
	Enabled             bool
	OOMRate             float64
	MemoryLeakRate      float64
	MaxMemoryUsageMB    float64
	GCPressureThreshold float64
}

// CrashConfig configures server crash simulation
type CrashConfig struct {
	Enabled          bool
	CrashRate        float64
	RandomCrashes    bool
	CrashAfterCount  int64
	RecoveryTime     time.Duration
}

// CorruptionConfig configures response corruption simulation
type CorruptionConfig struct {
	Enabled         bool
	CorruptionRate  float64
	PartialResponse bool
	MalformedJSON   bool
	FieldCorruption bool
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	Enabled              bool
	FailureThreshold     int
	SuccessThreshold     int
	Timeout              time.Duration
	TriggerConditions    []string
}

// ResponseDelayConfig configures response timing
type ResponseDelayConfig struct {
	Enabled       bool
	BaseDelay     time.Duration
	RandomJitter  time.Duration
	MethodDelays  map[string]time.Duration
	LoadBasedDelay LoadBasedDelayConfig
}

// LoadBasedDelayConfig configures load-based response delays
type LoadBasedDelayConfig struct {
	Enabled                bool
	LowLoadDelay          time.Duration
	HighLoadDelay         time.Duration
	LoadThreshold         int64
}

// ResponseGenerationConfig configures response generation behavior
type ResponseGenerationConfig struct {
	Realistic              bool
	LanguageSpecific       bool
	ContentAware           bool
	CrossReferenceEnabled  bool
	DocumentationEnabled   bool
	DiagnosticsEnabled     bool
}

// HealthBehaviorConfig configures health check behavior
type HealthBehaviorConfig struct {
	Enabled                bool
	HealthScoreVariation   float64
	DegradationSimulation  bool
	RecoverySimulation     bool
	MaintenanceMode        bool
}

// CapacityLimitsConfig configures server capacity limits
type CapacityLimitsConfig struct {
	MaxConcurrentRequests  int
	MaxQueueSize           int
	MaxMemoryUsageMB       float64
	RequestRateLimit       int // requests per second
	ConnectionLimit        int
}

// SymbolDatabase represents a database of symbols for realistic responses
type SymbolDatabase struct {
	Symbols     map[string][]SymbolInfo
	References  map[string][]ReferenceInfo
	Definitions map[string]DefinitionInfo
}

// SymbolInfo represents information about a symbol
type SymbolInfo struct {
	Name          string
	Kind          int
	ContainerName string
	Location      LocationInfo
	Documentation string
	Signature     string
	Detail        string
}

// ReferenceInfo represents a reference to a symbol
type ReferenceInfo struct {
	URI      string
	Range    RangeInfo
	Context  ReferenceContext
}

// DefinitionInfo represents a definition location
type DefinitionInfo struct {
	URI   string
	Range RangeInfo
}

// LocationInfo represents a location in a document
type LocationInfo struct {
	URI   string
	Range RangeInfo
}

// RangeInfo represents a range in a document
type RangeInfo struct {
	Start PositionInfo
	End   PositionInfo
}

// PositionInfo represents a position in a document
type PositionInfo struct {
	Line      int
	Character int
}

// ReferenceContext provides context for references
type ReferenceContext struct {
	IncludeDeclaration bool
}

// ProjectStructure represents the structure of a project
type ProjectStructure struct {
	RootURI       string
	Files         map[string]FileInfo
	Dependencies  []string
	BuildSystem   string
	LanguageInfo  LanguageInfo
}

// FileInfo represents information about a file
type FileInfo struct {
	URI         string
	Content     string
	Language    string
	Version     int
	LastModified time.Time
}

// LanguageInfo represents language-specific information
type LanguageInfo struct {
	Name              string
	FileExtensions    []string
	CommentStyle      CommentStyle
	Keywords          []string
	BuiltinTypes      []string
	StandardLibrary   []string
}

// CommentStyle represents comment syntax for a language
type CommentStyle struct {
	SingleLine  string
	MultiStart  string
	MultiEnd    string
}

// MockLSPServer represents a mock LSP server for testing
type MockLSPServer struct {
	Language     string
	ServerID     string
	Command      string
	Args         []string
	Port         int
	IsRunning    bool
	StartTime    time.Time
	RequestCount int64
	ErrorCount   int64
	ResponseTime time.Duration
	ProcessID    int
	WorkingDir   string
	LogFile      string

	// Enhanced configuration
	Config *MockServerConfig

	// Mock behavior configuration (legacy, kept for compatibility)
	ShouldFail        bool
	FailureRate       float64
	ResponseDelay     time.Duration
	MaxConcurrentReqs int
	MemoryUsageMB     float64
	CPUUsagePercent   float64
	HealthScore       float64

	// Advanced state tracking
	CurrentLoad         int64
	CircuitBreakerState CircuitBreakerState
	MemoryUsageHistory  []float64
	CrashCount          int64
	LastCrashTime       time.Time

	// Protocol compliance tracking
	Initialized         bool
	ClientCapabilities  map[string]interface{}
	ServerCapabilities  ServerCapabilities

	// Call tracking
	StartCalls            []context.Context
	StopCalls             []interface{}
	SendRequestCalls      []MockRequestCall
	SendNotificationCalls []MockNotificationCall
	IsActiveCalls         []interface{}
	GetHealthCalls        []interface{}
	GetMetricsCalls       []interface{}

	// Function fields for behavior customization
	StartFunc            func(ctx context.Context) error
	StopFunc             func() error
	SendRequestFunc      func(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	SendNotificationFunc func(ctx context.Context, method string, params interface{}) error
	IsActiveFunc         func() bool
	GetHealthFunc        func() *ServerHealth
	GetMetricsFunc       func() *ServerMetrics

	mu sync.RWMutex
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState struct {
	State        string // "CLOSED", "OPEN", "HALF_OPEN"
	FailureCount int
	SuccessCount int
	LastFailure  time.Time
	NextRetry    time.Time
}

// MockRequestCall tracks request calls
type MockRequestCall struct {
	Method  string
	Params  interface{}
	Time    time.Time
	Context context.Context
}

// MockNotificationCall tracks notification calls
type MockNotificationCall struct {
	Method  string
	Params  interface{}
	Time    time.Time
	Context context.Context
}

// ServerHealth represents server health status
type ServerHealth struct {
	IsHealthy    bool
	Score        float64
	LastCheck    time.Time
	ResponseTime time.Duration
	ErrorRate    float64
	MemoryUsage  float64
	CPUUsage     float64
	RequestLoad  int64
	Issues       []string
	Warnings     []string
}

// ServerMetrics represents server performance metrics
type ServerMetrics struct {
	RequestCount      int64
	SuccessCount      int64
	ErrorCount        int64
	AverageResponse   time.Duration
	TotalUptime       time.Duration
	MemoryUsageMB     float64
	CPUUsagePercent   float64
	ActiveConnections int
	QueuedRequests    int
	CacheHitRatio     float64
}

// MockLSPServerManager manages multiple mock LSP servers
type MockLSPServerManager struct {
	TempDir     string
	Servers     map[string]*MockLSPServer
	PortManager *PortManager

	// Call tracking
	CreateServerCalls  []string
	StartServerCalls   []ServerStartCall
	StopServerCalls    []string
	GetServerCalls     []string
	GetAllServersCalls []interface{}

	// Function fields for behavior customization
	CreateServerFunc func(language string) (*MockLSPServer, error)
	StartServerFunc  func(language string, ctx context.Context) error
	StopServerFunc   func(language string) error
	GetServerFunc    func(language string) (*MockLSPServer, bool)

	mu sync.RWMutex
}

// ServerStartCall tracks server start calls
type ServerStartCall struct {
	Language string
	Context  context.Context
	Time     time.Time
}

// PortManager manages port allocation for mock servers
type PortManager struct {
	allocatedPorts map[int]bool
	nextPort       int
	mu             sync.Mutex
}

// NewMockLSPServerManager creates a new mock LSP server manager
func NewMockLSPServerManager(tempDir string) *MockLSPServerManager {
	return &MockLSPServerManager{
		TempDir:     tempDir,
		Servers:     make(map[string]*MockLSPServer),
		PortManager: NewPortManager(9000), // Start from port 9000 for tests

		CreateServerCalls:  make([]string, 0),
		StartServerCalls:   make([]ServerStartCall, 0),
		StopServerCalls:    make([]string, 0),
		GetServerCalls:     make([]string, 0),
		GetAllServersCalls: make([]interface{}, 0),
	}
}

// NewPortManager creates a new port manager
func NewPortManager(startPort int) *PortManager {
	return &PortManager{
		allocatedPorts: make(map[int]bool),
		nextPort:       startPort,
	}
}

// AllocatePort allocates the next available port
func (pm *PortManager) AllocatePort() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for pm.allocatedPorts[pm.nextPort] {
		pm.nextPort++
	}

	port := pm.nextPort
	pm.allocatedPorts[port] = true
	pm.nextPort++

	return port
}

// ReleasePort releases a previously allocated port
func (pm *PortManager) ReleasePort(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.allocatedPorts, port)
}

// Initialize initializes the server manager
func (m *MockLSPServerManager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create logs directory
	logsDir := filepath.Join(m.TempDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	return nil
}

// CreateServer creates a new mock LSP server for the specified language
func (m *MockLSPServerManager) CreateServer(language string) (*MockLSPServer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateServerCalls = append(m.CreateServerCalls, language)

	if m.CreateServerFunc != nil {
		return m.CreateServerFunc(language)
	}

	// Check if server already exists
	if server, exists := m.Servers[language]; exists {
		return server, nil
	}

	// Create new server with enhanced configuration
	config := DefaultMockServerConfig(language)
	server := &MockLSPServer{
		Language:   language,
		ServerID:   fmt.Sprintf("mock-%s-server-%d", language, time.Now().UnixNano()),
		Command:    m.getCommandForLanguage(language),
		Args:       m.getArgsForLanguage(language),
		Port:       m.PortManager.AllocatePort(),
		WorkingDir: m.TempDir,
		LogFile:    filepath.Join(m.TempDir, "logs", fmt.Sprintf("%s-server.log", language)),

		// Enhanced configuration
		Config: config,

		// Default configuration (legacy, kept for compatibility)
		FailureRate:       0.0,
		ResponseDelay:     time.Millisecond * 50,
		MaxConcurrentReqs: 100,
		MemoryUsageMB:     64.0,
		CPUUsagePercent:   5.0,
		HealthScore:       1.0,

		// Advanced state initialization
		CurrentLoad:         0,
		CircuitBreakerState: CircuitBreakerState{State: "CLOSED", FailureCount: 0, SuccessCount: 0},
		MemoryUsageHistory:  make([]float64, 0, 100),
		CrashCount:          0,
		LastCrashTime:       time.Time{},

		// Protocol compliance initialization
		Initialized:        false,
		ClientCapabilities: make(map[string]interface{}),
		ServerCapabilities: config.Capabilities,

		// Initialize call tracking
		StartCalls:            make([]context.Context, 0),
		StopCalls:             make([]interface{}, 0),
		SendRequestCalls:      make([]MockRequestCall, 0),
		SendNotificationCalls: make([]MockNotificationCall, 0),
		IsActiveCalls:         make([]interface{}, 0),
		GetHealthCalls:        make([]interface{}, 0),
		GetMetricsCalls:       make([]interface{}, 0),
	}

	m.Servers[language] = server

	return server, nil
}

// StartServer starts a mock LSP server
func (m *MockLSPServerManager) StartServer(language string, ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartServerCalls = append(m.StartServerCalls, ServerStartCall{
		Language: language,
		Context:  ctx,
		Time:     time.Now(),
	})

	if m.StartServerFunc != nil {
		return m.StartServerFunc(language, ctx)
	}

	server, exists := m.Servers[language]
	if !exists {
		return fmt.Errorf("server for language %s not found", language)
	}

	return server.Start(ctx)
}

// StopServer stops a mock LSP server
func (m *MockLSPServerManager) StopServer(language string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StopServerCalls = append(m.StopServerCalls, language)

	if m.StopServerFunc != nil {
		return m.StopServerFunc(language)
	}

	server, exists := m.Servers[language]
	if !exists {
		return fmt.Errorf("server for language %s not found", language)
	}

	return server.Stop()
}

// GetServer retrieves a mock LSP server
func (m *MockLSPServerManager) GetServer(language string) (*MockLSPServer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetServerCalls = append(m.GetServerCalls, language)

	if m.GetServerFunc != nil {
		return m.GetServerFunc(language)
	}

	server, exists := m.Servers[language]
	return server, exists
}

// GetAllServers returns all mock LSP servers
func (m *MockLSPServerManager) GetAllServers() map[string]*MockLSPServer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetAllServersCalls = append(m.GetAllServersCalls, struct{}{})

	// Return a copy to avoid race conditions
	result := make(map[string]*MockLSPServer)
	for k, v := range m.Servers {
		result[k] = v
	}

	return result
}

// StopAllServers stops all running servers
func (m *MockLSPServerManager) StopAllServers() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	for language, server := range m.Servers {
		if err := server.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop %s server: %w", language, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping servers: %v", errors)
	}

	return nil
}

// Reset resets all call tracking and servers
func (m *MockLSPServerManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all servers
	for _, server := range m.Servers {
		server.Stop()
		m.PortManager.ReleasePort(server.Port)
	}

	// Clear state
	m.Servers = make(map[string]*MockLSPServer)
	m.CreateServerCalls = make([]string, 0)
	m.StartServerCalls = make([]ServerStartCall, 0)
	m.StopServerCalls = make([]string, 0)
	m.GetServerCalls = make([]string, 0)
	m.GetAllServersCalls = make([]interface{}, 0)

	// Reset function fields
	m.CreateServerFunc = nil
	m.StartServerFunc = nil
	m.StopServerFunc = nil
	m.GetServerFunc = nil
}

// getCommandForLanguage returns the mock command for a language
func (m *MockLSPServerManager) getCommandForLanguage(language string) string {
	commands := map[string]string{
		"go":         "gopls",
		"python":     "python",
		"javascript": "typescript-language-server",
		"typescript": "typescript-language-server",
		"java":       "java",
		"rust":       "rust-analyzer",
		"cpp":        "clangd",
		"csharp":     "omnisharp",
		"php":        "intelephense",
		"ruby":       "solargraph",
	}

	if cmd, exists := commands[language]; exists {
		return cmd
	}

	return fmt.Sprintf("mock-%s-lsp", language)
}

// getArgsForLanguage returns the mock arguments for a language
func (m *MockLSPServerManager) getArgsForLanguage(language string) []string {
	args := map[string][]string{
		"go":         {},
		"python":     {"-m", "pylsp"},
		"javascript": {"--stdio"},
		"typescript": {"--stdio"},
		"java":       {"-jar", "/mock/jdtls.jar"},
		"rust":       {},
		"cpp":        {"--background-index"},
		"csharp":     {"--stdio"},
		"php":        {"--stdio"},
		"ruby":       {"stdio"},
	}

	if argList, exists := args[language]; exists {
		return argList
	}

	return []string{"--stdio"}
}

// MockLSPServer methods

// Start starts the mock LSP server
func (s *MockLSPServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.StartCalls = append(s.StartCalls, ctx)

	if s.StartFunc != nil {
		return s.StartFunc(ctx)
	}

	if s.IsRunning {
		return fmt.Errorf("server %s is already running", s.Language)
	}

	s.IsRunning = true
	s.StartTime = time.Now()
	s.ProcessID = os.Getpid() // Mock process ID

	// Create log file
	if err := os.MkdirAll(filepath.Dir(s.LogFile), 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.Create(s.LogFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	fmt.Fprintf(logFile, "Mock %s LSP server started at %s\n", s.Language, s.StartTime.Format(time.RFC3339))

	return nil
}

// Stop stops the mock LSP server
func (s *MockLSPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.StopCalls = append(s.StopCalls, struct{}{})

	if s.StopFunc != nil {
		return s.StopFunc()
	}

	if !s.IsRunning {
		return nil // Already stopped
	}

	s.IsRunning = false

	// Append to log file
	if logFile, err := os.OpenFile(s.LogFile, os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		defer logFile.Close()
		fmt.Fprintf(logFile, "Mock %s LSP server stopped at %s\n", s.Language, time.Now().Format(time.RFC3339))
	}

	return nil
}

// SendRequest simulates sending a request to the LSP server with enhanced protocol compliance and failure simulation
func (s *MockLSPServer) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.SendRequestCalls = append(s.SendRequestCalls, MockRequestCall{
		Method:  method,
		Params:  params,
		Time:    time.Now(),
		Context: ctx,
	})

	if s.SendRequestFunc != nil {
		return s.SendRequestFunc(ctx, method, params)
	}

	// Enhanced pre-request checks
	if err := s.performPreRequestChecks(ctx, method); err != nil {
		return nil, err
	}

	// Update load tracking
	atomic.AddInt64(&s.CurrentLoad, 1)
	defer atomic.AddInt64(&s.CurrentLoad, -1)

	// Simulate response timing with enhanced delay calculation
	delay := s.calculateResponseDelay(method)
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Enhanced failure simulation
	if shouldSimulateFailure := s.shouldSimulateFailure(method); shouldSimulateFailure {
		return s.simulateFailure(method)
	}

	// Protocol compliance validation
	if !s.isMethodSupported(method) {
		return nil, s.createLSPError(-32601, fmt.Sprintf("Method not found: %s", method))
	}

	// Generate enhanced response
	response, err := s.generateEnhancedResponse(method, params)
	if err != nil {
		s.ErrorCount++
		return nil, err
	}

	// Simulate response corruption if configured
	if s.shouldCorruptResponse() {
		return s.corruptResponse(response)
	}

	s.RequestCount++
	s.updateCircuitBreakerState(true)
	s.updateMemoryUsage()

	return response, nil
}

// performPreRequestChecks performs enhanced pre-request validation
func (s *MockLSPServer) performPreRequestChecks(ctx context.Context, method string) error {
	// Check if server is running
	if !s.IsRunning {
		return fmt.Errorf("server is not running")
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if server has crashed and needs recovery
	if s.Config.CrashSimulation.Enabled && s.shouldSimulateCrash() {
		return s.simulateCrash()
	}

	// Check capacity limits
	if s.Config.CapacityLimits.MaxConcurrentRequests > 0 && 
	   atomic.LoadInt64(&s.CurrentLoad) >= int64(s.Config.CapacityLimits.MaxConcurrentRequests) {
		return s.createLSPError(-32000, "Server overloaded")
	}

	// Check memory pressure
	if s.Config.MemoryPressure.Enabled && s.shouldSimulateMemoryPressure() {
		return s.simulateMemoryPressure()
	}

	// Check timeout simulation
	if s.Config.TimeoutSimulation.Enabled && s.shouldSimulateTimeout(method) {
		return s.simulateTimeout(ctx, method)
	}

	// Check circuit breaker state
	if s.Config.CircuitBreakerConfig.Enabled && s.CircuitBreakerState.State == "OPEN" {
		if time.Now().Before(s.CircuitBreakerState.NextRetry) {
			return s.createLSPError(-32000, "Circuit breaker is open")
		}
		// Transition to half-open
		s.CircuitBreakerState.State = "HALF_OPEN"
	}

	return nil
}

// calculateResponseDelay calculates the response delay based on configuration and current load
func (s *MockLSPServer) calculateResponseDelay(method string) time.Duration {
	if !s.Config.ResponseDelays.Enabled {
		return s.ResponseDelay // fallback to legacy configuration
	}

	baseDelay := s.Config.ResponseDelays.BaseDelay

	// Method-specific delay
	if methodDelay, exists := s.Config.ResponseDelays.MethodDelays[method]; exists {
		baseDelay = methodDelay
	}

	// Add random jitter
	if s.Config.ResponseDelays.RandomJitter > 0 {
		jitter := time.Duration(rand.Int63n(int64(s.Config.ResponseDelays.RandomJitter)))
		baseDelay += jitter
	}

	// Load-based delay adjustment
	if s.Config.ResponseDelays.LoadBasedDelay.Enabled {
		currentLoad := atomic.LoadInt64(&s.CurrentLoad)
		if currentLoad > s.Config.ResponseDelays.LoadBasedDelay.LoadThreshold {
			loadMultiplier := float64(currentLoad) / float64(s.Config.ResponseDelays.LoadBasedDelay.LoadThreshold)
			additionalDelay := time.Duration(float64(s.Config.ResponseDelays.LoadBasedDelay.HighLoadDelay-s.Config.ResponseDelays.LoadBasedDelay.LowLoadDelay) * loadMultiplier)
			baseDelay += additionalDelay
		}
	}

	return baseDelay
}

// shouldSimulateFailure determines if a failure should be simulated for a method
func (s *MockLSPServer) shouldSimulateFailure(method string) bool {
	if s.ShouldFail {
		return true
	}

	// Check method-specific failure rate
	if methodRate, exists := s.Config.MethodFailureRates[method]; exists {
		return rand.Float64() < methodRate
	}

	// Check global failure rate
	if s.Config.GlobalFailureRate > 0 {
		return rand.Float64() < s.Config.GlobalFailureRate
	}

	// Legacy failure rate check
	if s.FailureRate > 0 {
		return rand.Float64() < s.FailureRate
	}

	return false
}

// simulateFailure simulates various types of failures
func (s *MockLSPServer) simulateFailure(method string) (json.RawMessage, error) {
	s.ErrorCount++
	s.updateCircuitBreakerState(false)

	// Different types of failures
	failureTypes := []string{"server_error", "invalid_params", "internal_error", "method_not_found"}
	failureType := failureTypes[rand.Intn(len(failureTypes))]

	switch failureType {
	case "server_error":
		return nil, s.createLSPError(-32000, fmt.Sprintf("Server error processing %s", method))
	case "invalid_params":
		return nil, s.createLSPError(-32602, "Invalid params")
	case "internal_error":
		return nil, s.createLSPError(-32603, "Internal error")
	case "method_not_found":
		return nil, s.createLSPError(-32601, fmt.Sprintf("Method not found: %s", method))
	default:
		return nil, fmt.Errorf("simulated server error for method %s", method)
	}
}

// shouldSimulateCrash determines if a server crash should be simulated
func (s *MockLSPServer) shouldSimulateCrash() bool {
	if !s.Config.CrashSimulation.Enabled {
		return false
	}

	if s.Config.CrashSimulation.RandomCrashes && rand.Float64() < s.Config.CrashSimulation.CrashRate {
		return true
	}

	if s.Config.CrashSimulation.CrashAfterCount > 0 && s.RequestCount >= s.Config.CrashSimulation.CrashAfterCount {
		return true
	}

	return false
}

// simulateCrash simulates a server crash
func (s *MockLSPServer) simulateCrash() error {
	s.CrashCount++
	s.LastCrashTime = time.Now()
	s.IsRunning = false

	// Log the crash
	if logFile, err := os.OpenFile(s.LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err == nil {
		defer logFile.Close()
		fmt.Fprintf(logFile, "CRASH: Mock %s LSP server crashed at %s (crash #%d)\n", 
			s.Language, time.Now().Format(time.RFC3339), s.CrashCount)
	}

	// Schedule recovery if configured
	if s.Config.CrashSimulation.RecoveryTime > 0 {
		go func() {
			time.Sleep(s.Config.CrashSimulation.RecoveryTime)
			s.mu.Lock()
			s.IsRunning = true
			s.mu.Unlock()
			
			// Log recovery
			if logFile, err := os.OpenFile(s.LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err == nil {
				defer logFile.Close()
				fmt.Fprintf(logFile, "RECOVERY: Mock %s LSP server recovered at %s\n", 
					s.Language, time.Now().Format(time.RFC3339))
			}
		}()
	}

	return fmt.Errorf("server crashed - simulated failure")
}

// shouldSimulateMemoryPressure determines if memory pressure should be simulated
func (s *MockLSPServer) shouldSimulateMemoryPressure() bool {
	if !s.Config.MemoryPressure.Enabled {
		return false
	}

	if rand.Float64() < s.Config.MemoryPressure.OOMRate {
		return true
	}

	if s.MemoryUsageMB > s.Config.MemoryPressure.MaxMemoryUsageMB {
		return true
	}

	return false
}

// simulateMemoryPressure simulates memory pressure conditions
func (s *MockLSPServer) simulateMemoryPressure() error {
	if rand.Float64() < s.Config.MemoryPressure.OOMRate {
		return fmt.Errorf("out of memory error - simulated OOM condition")
	}

	return s.createLSPError(-32000, "Server experiencing memory pressure")
}

// shouldSimulateTimeout determines if a timeout should be simulated
func (s *MockLSPServer) shouldSimulateTimeout(method string) bool {
	if !s.Config.TimeoutSimulation.Enabled {
		return false
	}

	if rand.Float64() < s.Config.TimeoutSimulation.TimeoutRate {
		return true
	}

	// Check method-specific timeouts
	if _, exists := s.Config.TimeoutSimulation.MethodTimeouts[method]; exists {
		return rand.Float64() < 0.1 // 10% chance for method-specific timeouts
	}

	return false
}

// simulateTimeout simulates request timeouts
func (s *MockLSPServer) simulateTimeout(ctx context.Context, method string) error {
	timeoutDuration := s.Config.TimeoutSimulation.MinTimeout + 
		time.Duration(rand.Int63n(int64(s.Config.TimeoutSimulation.MaxTimeout-s.Config.TimeoutSimulation.MinTimeout)))

	if methodTimeout, exists := s.Config.TimeoutSimulation.MethodTimeouts[method]; exists {
		timeoutDuration = methodTimeout
	}

	select {
	case <-time.After(timeoutDuration):
		return context.DeadlineExceeded
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isMethodSupported checks if a method is supported by the server
func (s *MockLSPServer) isMethodSupported(method string) bool {
	for _, supportedMethod := range s.Config.SupportedMethods {
		if supportedMethod == method {
			return true
		}
	}
	return false
}

// shouldCorruptResponse determines if a response should be corrupted
func (s *MockLSPServer) shouldCorruptResponse() bool {
	return s.Config.CorruptionSimulation.Enabled && 
	       rand.Float64() < s.Config.CorruptionSimulation.CorruptionRate
}

// corruptResponse corrupts a response in various ways
func (s *MockLSPServer) corruptResponse(response json.RawMessage) (json.RawMessage, error) {
	if s.Config.CorruptionSimulation.MalformedJSON {
		// Return malformed JSON
		return json.RawMessage(`{"corrupted": "malformed json`), nil
	}

	if s.Config.CorruptionSimulation.PartialResponse {
		// Return partial response
		if len(response) > 10 {
			return response[:len(response)/2], nil
		}
	}

	if s.Config.CorruptionSimulation.FieldCorruption {
		// Corrupt specific fields in the response
		var responseObj map[string]interface{}
		if err := json.Unmarshal(response, &responseObj); err == nil {
			// Add corrupted fields
			responseObj["corrupted_field"] = "corrupted_value"
			responseObj["timestamp"] = "invalid_timestamp"
			if corrupted, err := json.Marshal(responseObj); err == nil {
				return corrupted, nil
			}
		}
	}

	return response, nil
}

// updateCircuitBreakerState updates the circuit breaker state based on request outcome
func (s *MockLSPServer) updateCircuitBreakerState(success bool) {
	if !s.Config.CircuitBreakerConfig.Enabled {
		return
	}

	if success {
		s.CircuitBreakerState.SuccessCount++
		s.CircuitBreakerState.FailureCount = 0 // Reset failure count on success

		// Transition from HALF_OPEN to CLOSED if enough successes
		if s.CircuitBreakerState.State == "HALF_OPEN" && 
		   s.CircuitBreakerState.SuccessCount >= s.Config.CircuitBreakerConfig.SuccessThreshold {
			s.CircuitBreakerState.State = "CLOSED"
			s.CircuitBreakerState.SuccessCount = 0
		}
	} else {
		s.CircuitBreakerState.FailureCount++
		s.CircuitBreakerState.LastFailure = time.Now()

		// Transition to OPEN if failure threshold exceeded
		if s.CircuitBreakerState.FailureCount >= s.Config.CircuitBreakerConfig.FailureThreshold {
			s.CircuitBreakerState.State = "OPEN"
			s.CircuitBreakerState.NextRetry = time.Now().Add(s.Config.CircuitBreakerConfig.Timeout)
			s.CircuitBreakerState.FailureCount = 0
		}
	}
}

// updateMemoryUsage updates memory usage tracking
func (s *MockLSPServer) updateMemoryUsage() {
	// Simulate memory usage fluctuation
	baseUsage := s.MemoryUsageMB
	variationRange := baseUsage * 0.1 // 10% variation
	variationAmount := (rand.Float64() - 0.5) * 2 * variationRange
	newUsage := baseUsage + variationAmount

	if newUsage < 0 {
		newUsage = baseUsage
	}

	s.MemoryUsageMB = newUsage

	// Add to history (keep last 100 measurements)
	s.MemoryUsageHistory = append(s.MemoryUsageHistory, newUsage)
	if len(s.MemoryUsageHistory) > 100 {
		s.MemoryUsageHistory = s.MemoryUsageHistory[1:]
	}

	// Simulate memory leak if configured
	if s.Config.MemoryPressure.Enabled && s.Config.MemoryPressure.MemoryLeakRate > 0 {
		if rand.Float64() < s.Config.MemoryPressure.MemoryLeakRate {
			s.MemoryUsageMB += 0.1 // Small memory leak increment
		}
	}
}

// createLSPError creates a properly formatted LSP error
func (s *MockLSPServer) createLSPError(code int, message string) error {
	errorResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	errorJSON, _ := json.Marshal(errorResponse)
	return fmt.Errorf("%s", string(errorJSON))
}

// SendNotification simulates sending a notification to the LSP server (implements transport.LSPClient)
func (s *MockLSPServer) SendNotification(ctx context.Context, method string, params interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.SendNotificationCalls = append(s.SendNotificationCalls, MockNotificationCall{
		Method:  method,
		Params:  params,
		Time:    time.Now(),
		Context: ctx,
	})

	if s.SendNotificationFunc != nil {
		return s.SendNotificationFunc(ctx, method, params)
	}

	if !s.IsRunning {
		return fmt.Errorf("server is not running")
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate processing delay for notifications
	if s.ResponseDelay > 0 {
		time.Sleep(s.ResponseDelay / 2) // Shorter delay for notifications
	}

	// Notifications don't have responses, just track the call
	return nil
}

// IsActive checks if the server is active (implements transport.LSPClient)
func (s *MockLSPServer) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.IsActiveCalls = append(s.IsActiveCalls, struct{}{})

	if s.IsActiveFunc != nil {
		return s.IsActiveFunc()
	}

	return s.IsRunning
}

// GetHealth returns the server health status
func (s *MockLSPServer) GetHealth() *ServerHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.GetHealthCalls = append(s.GetHealthCalls, struct{}{})

	if s.GetHealthFunc != nil {
		return s.GetHealthFunc()
	}

	errorRate := 0.0
	if s.RequestCount > 0 {
		errorRate = float64(s.ErrorCount) / float64(s.RequestCount)
	}

	return &ServerHealth{
		IsHealthy:    s.IsRunning && s.HealthScore > 0.5,
		Score:        s.HealthScore,
		LastCheck:    time.Now(),
		ResponseTime: s.ResponseTime,
		ErrorRate:    errorRate,
		MemoryUsage:  s.MemoryUsageMB,
		CPUUsage:     s.CPUUsagePercent,
		RequestLoad:  s.RequestCount,
		Issues:       []string{},
		Warnings:     []string{},
	}
}

// GetMetrics returns server performance metrics
func (s *MockLSPServer) GetMetrics() *ServerMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.GetMetricsCalls = append(s.GetMetricsCalls, struct{}{})

	if s.GetMetricsFunc != nil {
		return s.GetMetricsFunc()
	}

	uptime := time.Duration(0)
	if s.IsRunning {
		uptime = time.Since(s.StartTime)
	}

	successCount := s.RequestCount - s.ErrorCount

	return &ServerMetrics{
		RequestCount:      s.RequestCount,
		SuccessCount:      successCount,
		ErrorCount:        s.ErrorCount,
		AverageResponse:   s.ResponseTime,
		TotalUptime:       uptime,
		MemoryUsageMB:     s.MemoryUsageMB,
		CPUUsagePercent:   s.CPUUsagePercent,
		ActiveConnections: 1,    // Mock value
		QueuedRequests:    0,    // Mock value
		CacheHitRatio:     0.85, // Mock value
	}
}

// GetCommand returns the server command
func (s *MockLSPServer) GetCommand() string {
	return s.Command
}

// GetArgs returns the server arguments
func (s *MockLSPServer) GetArgs() []string {
	return s.Args
}

// SetFailureRate sets the failure rate for requests
func (s *MockLSPServer) SetFailureRate(rate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailureRate = rate
}

// SetResponseDelay sets the response delay
func (s *MockLSPServer) SetResponseDelay(delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ResponseDelay = delay
}

// generateEnhancedResponse generates enhanced mock responses with realistic protocol compliance
func (s *MockLSPServer) generateEnhancedResponse(method string, params interface{}) (json.RawMessage, error) {
	// Use enhanced response generation if configured
	if s.Config.ResponseGeneration.Realistic {
		return s.generateRealisticResponse(method, params)
	}

	// Fall back to basic response generation
	response := s.generateMockResponse(method, params)
	rawMessage, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return json.RawMessage(rawMessage), nil
}

// generateRealisticResponse generates realistic LSP responses based on the protocol specification
func (s *MockLSPServer) generateRealisticResponse(method string, params interface{}) (json.RawMessage, error) {
	var response interface{}
	var err error

	switch method {
	// Lifecycle methods
	case "initialize":
		response = s.generateInitializeResponse(params)
	case "initialized":
		response = nil // initialized is a notification, no response
	case "shutdown":
		response = nil
	case "exit":
		response = nil // exit is a notification, no response

	// Document synchronization methods
	case "textDocument/didOpen", "textDocument/didChange", "textDocument/didSave", "textDocument/didClose":
		response = nil // These are notifications, no response

	// Language feature methods
	case "textDocument/completion":
		response = s.generateCompletionResponse(params)
	case "textDocument/hover":
		response = s.generateHoverResponse(params)
	case "textDocument/signatureHelp":
		response = s.generateSignatureHelpResponse(params)
	case "textDocument/definition":
		response = s.generateDefinitionResponse(params)
	case "textDocument/typeDefinition":
		response = s.generateTypeDefinitionResponse(params)
	case "textDocument/implementation":
		response = s.generateImplementationResponse(params)
	case "textDocument/references":
		response = s.generateReferencesResponse(params)
	case "textDocument/documentHighlight":
		response = s.generateDocumentHighlightResponse(params)
	case "textDocument/documentSymbol":
		response = s.generateDocumentSymbolResponse(params)
	case "textDocument/codeAction":
		response = s.generateCodeActionResponse(params)
	case "textDocument/codeLens":
		response = s.generateCodeLensResponse(params)
	case "textDocument/documentLink":
		response = s.generateDocumentLinkResponse(params)
	case "textDocument/colorPresentation":
		response = s.generateColorPresentationResponse(params)
	case "textDocument/formatting":
		response = s.generateFormattingResponse(params)
	case "textDocument/rangeFormatting":
		response = s.generateRangeFormattingResponse(params)
	case "textDocument/onTypeFormatting":
		response = s.generateOnTypeFormattingResponse(params)
	case "textDocument/rename":
		response = s.generateRenameResponse(params)
	case "textDocument/prepareRename":
		response = s.generatePrepareRenameResponse(params)
	case "textDocument/foldingRange":
		response = s.generateFoldingRangeResponse(params)
	case "textDocument/selectionRange":
		response = s.generateSelectionRangeResponse(params)
	case "textDocument/semanticTokens/full":
		response = s.generateSemanticTokensResponse(params)
	case "textDocument/semanticTokens/range":
		response = s.generateSemanticTokensRangeResponse(params)
	case "textDocument/inlayHint":
		response = s.generateInlayHintResponse(params)
	case "textDocument/diagnostic":
		response = s.generateDiagnosticResponse(params)

	// Workspace methods
	case "workspace/symbol":
		response = s.generateWorkspaceSymbolResponse(params)
	case "workspace/executeCommand":
		response = s.generateExecuteCommandResponse(params)
	case "workspace/applyEdit":
		response = s.generateApplyEditResponse(params)
	case "workspace/didChangeConfiguration", "workspace/didChangeWatchedFiles", "workspace/didCreateFiles", "workspace/didDeleteFiles", "workspace/didRenameFiles":
		response = nil // These are notifications, no response
	case "workspace/willCreateFiles", "workspace/willDeleteFiles", "workspace/willRenameFiles":
		response = s.generateWorkspaceEditResponse(params)
	case "workspace/diagnostic":
		response = s.generateWorkspaceDiagnosticResponse(params)

	// Window methods
	case "window/showMessage", "window/logMessage":
		response = nil // These are notifications, no response
	case "window/showMessageRequest":
		response = s.generateShowMessageRequestResponse(params)
	case "window/workDoneProgress/create":
		response = nil

	default:
		return nil, s.createLSPError(-32601, fmt.Sprintf("Method not found: %s", method))
	}

	if response == nil {
		return json.RawMessage("null"), nil
	}

	rawMessage, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return json.RawMessage(rawMessage), nil
}

// generateMockResponse generates mock responses based on LSP method (legacy method for compatibility)
func (s *MockLSPServer) generateMockResponse(method string, params interface{}) interface{} {
	switch method {
	case "textDocument/definition":
		return map[string]interface{}{
			"uri":   "file:///mock/definition.go",
			"range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 5}},
		}
	case "textDocument/references":
		return []map[string]interface{}{
			{"uri": "file:///mock/ref1.go", "range": map[string]interface{}{"start": map[string]int{"line": 5, "character": 10}}},
			{"uri": "file:///mock/ref2.go", "range": map[string]interface{}{"start": map[string]int{"line": 15, "character": 3}}},
		}
	case "textDocument/hover":
		return map[string]interface{}{
			"contents": fmt.Sprintf("Mock hover information for %s", s.Language),
		}
	case "textDocument/documentSymbol":
		return []map[string]interface{}{
			{"name": "MockFunction", "kind": 12, "range": map[string]interface{}{"start": map[string]int{"line": 1, "character": 0}}},
			{"name": "MockStruct", "kind": 23, "range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 0}}},
		}
	case "workspace/symbol":
		return []map[string]interface{}{
			{"name": "GlobalFunction", "kind": 12, "location": map[string]interface{}{"uri": "file:///mock/global.go"}},
		}
	default:
		return map[string]interface{}{
			"result": fmt.Sprintf("Mock response for method %s in %s", method, s.Language),
		}
	}
}

// DefaultMockServerConfig creates a default configuration for a mock LSP server
func DefaultMockServerConfig(language string) *MockServerConfig {
	return &MockServerConfig{
		// Protocol compliance configuration
		ProtocolVersion:  "3.17",
		Capabilities:     getDefaultCapabilities(language),
		SupportedMethods: getDefaultSupportedMethods(language),

		// Basic failure simulation configuration
		GlobalFailureRate:  0.0,
		MethodFailureRates: make(map[string]float64),

		// Timeout simulation configuration
		TimeoutSimulation: TimeoutConfig{
			Enabled:        false,
			TimeoutRate:    0.0,
			MinTimeout:     time.Millisecond * 100,
			MaxTimeout:     time.Second * 5,
			MethodTimeouts: make(map[string]time.Duration),
		},

		// Memory pressure configuration
		MemoryPressure: MemoryPressureConfig{
			Enabled:             false,
			OOMRate:             0.0,
			MemoryLeakRate:      0.0,
			MaxMemoryUsageMB:    512.0,
			GCPressureThreshold: 0.8,
		},

		// Crash simulation configuration
		CrashSimulation: CrashConfig{
			Enabled:          false,
			CrashRate:        0.0,
			RandomCrashes:    false,
			CrashAfterCount:  0,
			RecoveryTime:     time.Second * 10,
		},

		// Response corruption configuration
		CorruptionSimulation: CorruptionConfig{
			Enabled:         false,
			CorruptionRate:  0.0,
			PartialResponse: false,
			MalformedJSON:   false,
			FieldCorruption: false,
		},

		// Circuit breaker configuration
		CircuitBreakerConfig: CircuitBreakerConfig{
			Enabled:           false,
			FailureThreshold:  5,
			SuccessThreshold:  3,
			Timeout:           time.Second * 30,
			TriggerConditions: []string{},
		},

		// Response behavior configuration
		ResponseDelays: ResponseDelayConfig{
			Enabled:      false,
			BaseDelay:    time.Millisecond * 50,
			RandomJitter: time.Millisecond * 10,
			MethodDelays: make(map[string]time.Duration),
			LoadBasedDelay: LoadBasedDelayConfig{
				Enabled:       false,
				LowLoadDelay:  time.Millisecond * 50,
				HighLoadDelay: time.Millisecond * 200,
				LoadThreshold: 10,
			},
		},

		// Response generation configuration
		ResponseGeneration: ResponseGenerationConfig{
			Realistic:              false,
			LanguageSpecific:       true,
			ContentAware:           false,
			CrossReferenceEnabled:  false,
			DocumentationEnabled:   false,
			DiagnosticsEnabled:     false,
		},

		// Health behavior configuration
		HealthBehavior: HealthBehaviorConfig{
			Enabled:               false,
			HealthScoreVariation:  0.1,
			DegradationSimulation: false,
			RecoverySimulation:    false,
			MaintenanceMode:       false,
		},

		// Capacity limits configuration
		CapacityLimits: CapacityLimitsConfig{
			MaxConcurrentRequests: 100,
			MaxQueueSize:          1000,
			MaxMemoryUsageMB:      256.0,
			RequestRateLimit:      100,
			ConnectionLimit:       50,
		},

		// Content and workspace simulation
		WorkspaceContent: make(map[string]string),
		SymbolDatabase: SymbolDatabase{
			Symbols:     make(map[string][]SymbolInfo),
			References:  make(map[string][]ReferenceInfo),
			Definitions: make(map[string]DefinitionInfo),
		},
		ProjectStructure: ProjectStructure{
			RootURI:      "file:///mock/project",
			Files:        make(map[string]FileInfo),
			Dependencies: []string{},
			BuildSystem:  "mock",
			LanguageInfo: getDefaultLanguageInfo(language),
		},
	}
}

// getDefaultCapabilities returns default LSP server capabilities for a language
func getDefaultCapabilities(language string) ServerCapabilities {
	return ServerCapabilities{
		TextDocumentSync:       2, // Incremental
		HoverProvider:          true,
		DefinitionProvider:     true,
		ReferencesProvider:     true,
		DocumentSymbolProvider: true,
		WorkspaceSymbolProvider: true,
	}
}

// getDefaultSupportedMethods returns default supported LSP methods for a language
func getDefaultSupportedMethods(language string) []string {
	return []string{
		"initialize",
		"initialized",
		"shutdown",
		"exit",
		"textDocument/didOpen",
		"textDocument/didChange",
		"textDocument/didSave",
		"textDocument/didClose",
		"textDocument/hover",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/documentSymbol",
		"workspace/symbol",
	}
}

// getDefaultLanguageInfo returns default language information
func getDefaultLanguageInfo(language string) LanguageInfo {
	languageConfigs := map[string]LanguageInfo{
		"go": {
			Name:            "Go",
			FileExtensions:  []string{".go"},
			CommentStyle:    CommentStyle{SingleLine: "//", MultiStart: "/*", MultiEnd: "*/"},
			Keywords:        []string{"func", "var", "const", "type", "package", "import"},
			BuiltinTypes:    []string{"string", "int", "bool", "float64"},
			StandardLibrary: []string{"fmt", "os", "io", "net/http"},
		},
		"python": {
			Name:            "Python",
			FileExtensions:  []string{".py"},
			CommentStyle:    CommentStyle{SingleLine: "#", MultiStart: "\"\"\"", MultiEnd: "\"\"\""},
			Keywords:        []string{"def", "class", "import", "from", "if", "for"},
			BuiltinTypes:    []string{"str", "int", "bool", "float", "list", "dict"},
			StandardLibrary: []string{"os", "sys", "json", "requests"},
		},
		"javascript": {
			Name:            "JavaScript",
			FileExtensions:  []string{".js"},
			CommentStyle:    CommentStyle{SingleLine: "//", MultiStart: "/*", MultiEnd: "*/"},
			Keywords:        []string{"function", "var", "let", "const", "class"},
			BuiltinTypes:    []string{"string", "number", "boolean", "object", "undefined"},
			StandardLibrary: []string{"JSON", "Math", "Date", "Array"},
		},
		"typescript": {
			Name:            "TypeScript",
			FileExtensions:  []string{".ts", ".tsx"},
			CommentStyle:    CommentStyle{SingleLine: "//", MultiStart: "/*", MultiEnd: "*/"},
			Keywords:        []string{"function", "var", "let", "const", "class", "interface", "type"},
			BuiltinTypes:    []string{"string", "number", "boolean", "object", "undefined", "any"},
			StandardLibrary: []string{"JSON", "Math", "Date", "Array"},
		},
	}

	if config, exists := languageConfigs[language]; exists {
		return config
	}

	// Default language info for unknown languages
	return LanguageInfo{
		Name:            strings.Title(language),
		FileExtensions:  []string{"." + language},
		CommentStyle:    CommentStyle{SingleLine: "//", MultiStart: "/*", MultiEnd: "*/"},
		Keywords:        []string{},
		BuiltinTypes:    []string{},
		StandardLibrary: []string{},
	}
}

// SetFailureMethods sets method-specific failure rates
func (s *MockLSPServer) SetFailureMethods(methods map[string]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Config == nil {
		s.Config = DefaultMockServerConfig(s.Language)
	}
	s.Config.MethodFailureRates = make(map[string]float64)
	for method, rate := range methods {
		s.Config.MethodFailureRates[method] = rate
	}
}

// SetMaxConcurrentRequests sets the maximum concurrent requests limit
func (s *MockLSPServer) SetMaxConcurrentRequests(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MaxConcurrentReqs = limit
	if s.Config != nil {
		s.Config.CapacityLimits.MaxConcurrentRequests = limit
	}
}

// GetFailureRate returns the current global failure rate
func (s *MockLSPServer) GetFailureRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.FailureRate
}

// GetResponseDelay returns the current response delay
func (s *MockLSPServer) GetResponseDelay() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ResponseDelay
}

// GetMaxConcurrentRequests returns the current concurrent requests limit
func (s *MockLSPServer) GetMaxConcurrentRequests() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MaxConcurrentReqs
}

// GetFailureMethods returns the current method-specific failure rates
func (s *MockLSPServer) GetFailureMethods() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Config == nil || s.Config.MethodFailureRates == nil {
		return make(map[string]float64)
	}
	// Return a copy to avoid race conditions
	result := make(map[string]float64)
	for method, rate := range s.Config.MethodFailureRates {
		result[method] = rate
	}
	return result
}

// EnableBasicFailureSimulation enables basic failure simulation with simple defaults
func (s *MockLSPServer) EnableBasicFailureSimulation(globalRate float64, responseDelay time.Duration, maxConcurrent int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.FailureRate = globalRate
	s.ResponseDelay = responseDelay
	s.MaxConcurrentReqs = maxConcurrent
	
	if s.Config == nil {
		s.Config = DefaultMockServerConfig(s.Language)
	}
	s.Config.GlobalFailureRate = globalRate
	s.Config.ResponseDelays.Enabled = responseDelay > 0
	s.Config.ResponseDelays.BaseDelay = responseDelay
	s.Config.CapacityLimits.MaxConcurrentRequests = maxConcurrent
}

// DisableFailureSimulation disables all failure simulation
func (s *MockLSPServer) DisableFailureSimulation() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.FailureRate = 0.0
	s.ShouldFail = false
	s.ResponseDelay = 0
	
	if s.Config != nil {
		s.Config.GlobalFailureRate = 0.0
		s.Config.MethodFailureRates = make(map[string]float64)
		s.Config.ResponseDelays.Enabled = false
		s.Config.TimeoutSimulation.Enabled = false
		s.Config.CrashSimulation.Enabled = false
		s.Config.CorruptionSimulation.Enabled = false
		s.Config.MemoryPressure.Enabled = false
	}
}

// Response generation methods (stub implementations for basic functionality)

func (s *MockLSPServer) generateInitializeResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"capabilities": s.ServerCapabilities,
		"serverInfo": map[string]interface{}{
			"name":    fmt.Sprintf("Mock %s LSP Server", s.Language),
			"version": "1.0.0",
		},
	}
}

func (s *MockLSPServer) generateCompletionResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"isIncomplete": false,
		"items": []map[string]interface{}{
			{"label": "MockCompletion1", "kind": 1, "detail": "Mock completion item"},
			{"label": "MockCompletion2", "kind": 6, "detail": "Mock method completion"},
		},
	}
}

func (s *MockLSPServer) generateHoverResponse(params interface{}) interface{} {
	return s.generateMockResponse("textDocument/hover", params)
}

func (s *MockLSPServer) generateSignatureHelpResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"signatures": []map[string]interface{}{
			{"label": "mockFunction(param1: string, param2: number)", "documentation": "Mock function signature"},
		},
		"activeSignature": 0,
		"activeParameter": 0,
	}
}

func (s *MockLSPServer) generateDefinitionResponse(params interface{}) interface{} {
	return s.generateMockResponse("textDocument/definition", params)
}

func (s *MockLSPServer) generateTypeDefinitionResponse(params interface{}) interface{} {
	return s.generateMockResponse("textDocument/definition", params)
}

func (s *MockLSPServer) generateImplementationResponse(params interface{}) interface{} {
	return s.generateMockResponse("textDocument/definition", params)
}

func (s *MockLSPServer) generateReferencesResponse(params interface{}) interface{} {
	return s.generateMockResponse("textDocument/references", params)
}

func (s *MockLSPServer) generateDocumentHighlightResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"range": map[string]interface{}{"start": map[string]int{"line": 5, "character": 10}}, "kind": 1},
	}
}

func (s *MockLSPServer) generateDocumentSymbolResponse(params interface{}) interface{} {
	return s.generateMockResponse("textDocument/documentSymbol", params)
}

func (s *MockLSPServer) generateCodeActionResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"title": "Mock Code Action", "kind": "quickfix"},
	}
}

func (s *MockLSPServer) generateCodeLensResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"range": map[string]interface{}{"start": map[string]int{"line": 1, "character": 0}}, "command": map[string]interface{}{"title": "Mock Lens", "command": "mock.command"}},
	}
}

func (s *MockLSPServer) generateDocumentLinkResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"range": map[string]interface{}{"start": map[string]int{"line": 1, "character": 0}}, "target": "file:///mock/link.go"},
	}
}

func (s *MockLSPServer) generateColorPresentationResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"label": "#ff0000"},
	}
}

func (s *MockLSPServer) generateFormattingResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"range": map[string]interface{}{"start": map[string]int{"line": 0, "character": 0}}, "newText": "formatted code"},
	}
}

func (s *MockLSPServer) generateRangeFormattingResponse(params interface{}) interface{} {
	return s.generateFormattingResponse(params)
}

func (s *MockLSPServer) generateOnTypeFormattingResponse(params interface{}) interface{} {
	return s.generateFormattingResponse(params)
}

func (s *MockLSPServer) generateRenameResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"changes": map[string]interface{}{
			"file:///mock/file.go": []map[string]interface{}{
				{"range": map[string]interface{}{"start": map[string]int{"line": 1, "character": 0}}, "newText": "newName"},
			},
		},
	}
}

func (s *MockLSPServer) generatePrepareRenameResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"range":       map[string]interface{}{"start": map[string]int{"line": 1, "character": 0}},
		"placeholder": "currentName",
	}
}

func (s *MockLSPServer) generateFoldingRangeResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"startLine": 1, "endLine": 10, "kind": "region"},
	}
}

func (s *MockLSPServer) generateSelectionRangeResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"range": map[string]interface{}{"start": map[string]int{"line": 1, "character": 0}}},
	}
}

func (s *MockLSPServer) generateSemanticTokensResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"data": []int{1, 0, 5, 1, 0},
	}
}

func (s *MockLSPServer) generateSemanticTokensRangeResponse(params interface{}) interface{} {
	return s.generateSemanticTokensResponse(params)
}

func (s *MockLSPServer) generateInlayHintResponse(params interface{}) interface{} {
	return []map[string]interface{}{
		{"position": map[string]int{"line": 1, "character": 10}, "label": "hint", "kind": 1},
	}
}

func (s *MockLSPServer) generateDiagnosticResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"items": []map[string]interface{}{
			{"range": map[string]interface{}{"start": map[string]int{"line": 1, "character": 0}}, "message": "Mock diagnostic", "severity": 1},
		},
	}
}

func (s *MockLSPServer) generateWorkspaceSymbolResponse(params interface{}) interface{} {
	return s.generateMockResponse("workspace/symbol", params)
}

func (s *MockLSPServer) generateExecuteCommandResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"result": "Mock command executed successfully",
	}
}

func (s *MockLSPServer) generateApplyEditResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"applied": true,
	}
}

func (s *MockLSPServer) generateWorkspaceEditResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"changes": map[string]interface{}{},
	}
}

func (s *MockLSPServer) generateWorkspaceDiagnosticResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"items": []interface{}{},
	}
}

func (s *MockLSPServer) generateShowMessageRequestResponse(params interface{}) interface{} {
	return map[string]interface{}{
		"title": "OK",
	}
}
