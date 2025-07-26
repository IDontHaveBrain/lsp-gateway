package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/indexing"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type WorkspaceAwareJSONRPCRequest struct {
	JSONRPCRequest
	WorkspaceID string `json:"workspace_id,omitempty"`
	ProjectPath string `json:"project_path,omitempty"`
}

type WorkspaceContext interface {
	GetID() string
	GetRootPath() string
	GetProjectType() string
	GetProjectName() string
	GetLanguages() []string
	IsActive() bool
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RequestContext provides context information for LSP requests
type RequestContext struct {
	ProjectType          string                 `json:"project_type,omitempty"`
	ServerName           string                 `json:"server_name,omitempty"`
	Language             string                 `json:"language,omitempty"`
	WorkspaceID          string                 `json:"workspace_id,omitempty"`
	FileURI              string                 `json:"file_uri,omitempty"`
	RequestType          string                 `json:"request_type,omitempty"`
	WorkspaceRoot        string                 `json:"workspace_root,omitempty"`
	CrossLanguageContext bool                   `json:"cross_language_context,omitempty"`
	RequiresAggregation  bool                   `json:"requires_aggregation,omitempty"`
	SupportedServers     []string               `json:"supported_servers,omitempty"`
	AdditionalContext    map[string]interface{} `json:"additional_context,omitempty"`
}

// IsWorkspaceRequest checks if this is a workspace-level request
func (rc *RequestContext) IsWorkspaceRequest() bool {
	return rc.WorkspaceID != ""
}

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

const (
	JSONRPCVersion = "2.0"
)

const (
	HTTPContentTypeJSON = "application/json"
	HTTPMethodPOST      = "POST"
	URIPrefixFile       = "file://"
	PathJSONRPC         = "/jsonrpc"
)

const (
	LoggerComponentGateway = "gateway"
	LoggerFieldServerName  = "server_name"
	TimestampFormatISO8601 = "2006-01-02T15:04:05Z07:00"
)

const (
	LSPMethodHover           = "textDocument/hover"
	LSPMethodDefinition      = "textDocument/definition"
	LSPMethodReferences      = "textDocument/references"
	LSPMethodDocumentSymbol  = "textDocument/documentSymbol"
	LSPMethodWorkspaceSymbol = "workspace/symbol"
)

const (
	LSPMethodInitialize              = "initialize"
	LSPMethodInitialized             = "initialized"
	LSPMethodShutdown                = "shutdown"
	LSPMethodExit                    = "exit"
	LSPMethodWorkspaceExecuteCommand = "workspace/executeCommand"
)

const (
	ERROR_INVALID_REQUEST   = "Invalid JSON-RPC request"
	ERROR_INTERNAL          = "Internal server error"
	ERROR_SERVER_NOT_FOUND  = "server %s not found"
	FORMAT_INVALID_JSON_RPC = "invalid JSON-RPC version: %s"
)

const (
	LoggerFieldWorkspaceID = "workspace_id"
	LoggerFieldProjectPath = "project_path"
	LoggerFieldProjectType = "project_type"
	LoggerFieldProjectName = "project_name"
	URIPrefixWorkspace     = "workspace://"
)

// RequestTracker tracks concurrent requests for performance monitoring
type RequestTracker struct {
	activeRequests map[string]*ConcurrentRequestInfo
	mu             sync.RWMutex
}

// ConcurrentRequestInfo holds information about a concurrent request
type ConcurrentRequestInfo struct {
	RequestID   string
	Method      string
	Language    string
	ServerCount int
	StartTime   time.Time
	Status      string
}

// NewRequestTracker creates a new request tracker
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		activeRequests: make(map[string]*ConcurrentRequestInfo),
	}
}

// TrackRequest starts tracking a concurrent request
func (rt *RequestTracker) TrackRequest(requestID, method, language string, serverCount int) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.activeRequests[requestID] = &ConcurrentRequestInfo{
		RequestID:   requestID,
		Method:      method,
		Language:    language,
		ServerCount: serverCount,
		StartTime:   time.Now(),
		Status:      "active",
	}
}

// CompleteRequest marks a request as completed
func (rt *RequestTracker) CompleteRequest(requestID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if info, exists := rt.activeRequests[requestID]; exists {
		info.Status = "completed"
		// Keep completed requests for a short time for metrics
		go func() {
			time.Sleep(5 * time.Minute)
			rt.mu.Lock()
			delete(rt.activeRequests, requestID)
			rt.mu.Unlock()
		}()
	}
}

// GetActiveRequestCount returns the number of active concurrent requests
func (rt *RequestTracker) GetActiveRequestCount() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	count := 0
	for _, info := range rt.activeRequests {
		if info.Status == "active" {
			count++
		}
	}
	return count
}

type Router struct {
	langToServer map[string]string
	extToLang    map[string]string
	mu           sync.RWMutex
}

func NewRouter() *Router {
	return &Router{
		langToServer: make(map[string]string),
		extToLang:    make(map[string]string),
	}
}

func (r *Router) RegisterServer(serverName string, languages []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, lang := range languages {
		r.langToServer[lang] = serverName

		extensions := getExtensionsForLanguage(lang)
		for _, ext := range extensions {
			r.extToLang[ext] = lang
		}
	}
}

func (r *Router) RouteRequest(uri string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filePath := uri
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "", fmt.Errorf("cannot determine file type from URI: %s", uri)
	}

	ext = strings.TrimPrefix(ext, ".")

	lang, exists := r.extToLang[ext]
	if !exists {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	server, exists := r.langToServer[lang]
	if !exists {
		return "", fmt.Errorf("no server configured for language: %s", lang)
	}

	return server, nil
}

func (r *Router) GetSupportedLanguages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var languages []string
	for lang := range r.langToServer {
		languages = append(languages, lang)
	}
	return languages
}

func (r *Router) GetSupportedExtensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var extensions []string
	for ext := range r.extToLang {
		extensions = append(extensions, ext)
	}
	return extensions
}

func (r *Router) GetServerByLanguage(language string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.langToServer[language]
	return server, exists
}

func (r *Router) GetLanguageByExtension(extension string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	extension = strings.TrimPrefix(strings.ToLower(extension), ".")
	lang, exists := r.extToLang[extension]
	return lang, exists
}

func getExtensionsForLanguage(language string) []string {
	extensions := map[string][]string{
		"go": {"go", "mod", "sum", "work"},

		"python": {"py", "pyi", "pyx", "pyz", "pyw", "pyc", "pyo", "pyd"},

		"typescript": {"ts", "tsx", "mts", "cts"},

		"javascript": {"js", "jsx", "mjs", "cjs", "es", "es6", "es2015", "es2017", "es2018", "es2019", "es2020", "es2021", "es2022"},

		"java": {"java", "class", "jar", "war", "ear", "jsp", "jspx"},

		"c": {"c", "h", "i"},

		"cpp": {"cpp", "cxx", "cc", "c++", "hpp", "hxx", "h++", "hh", "ipp", "ixx", "txx", "tpp", "tcc"},

		"rust": {"rs", "rlib"},

		"ruby": {"rb", "rbw", "rake", "gemspec", "podspec", "thor", "irb"},

		"php": {"php", "php3", "php4", "php5", "php7", "php8", "phtml", "phar"},

		"swift": {"swift", "swiftmodule", "swiftdoc", "swiftsourceinfo"},

		"kotlin": {"kt", "kts", "ktm"},

		"scala": {"scala", "sc", "sbt"},

		"csharp": {"cs", "csx", "csproj", "sln", "vb", "vbproj"},

		"fsharp": {"fs", "fsi", "fsx", "fsscript", "fsproj"},

		"html":         {"html", "htm", "xhtml", "shtml", "svg"},
		"css":          {"css", "scss", "sass", "less", "styl", "stylus"},
		"json":         {"json", "jsonc", "json5"},
		"xml":          {"xml", "xsd", "xsl", "xslt", "wsdl", "soap", "rss", "atom"},
		"yaml":         {"yaml", "yml"},
		"toml":         {"toml"},
		"ini":          {"ini", "cfg", "conf", "config"},
		"markdown":     {"md", "markdown", "mdown", "mkdn", "mkd", "mdx"},
		"sql":          {"sql", "mysql", "pgsql", "plsql", "tsql", "sqlite", "ddl", "dml"},
		"shell":        {"sh", "bash", "zsh", "fish", "csh", "tcsh", "ksh", "ash", "dash"},
		"powershell":   {"ps1", "psm1", "psd1", "ps1xml", "pssc", "psrc", "cdxml"},
		"dockerfile":   {"dockerfile", "dockerignore"},
		"lua":          {"lua", "luac"},
		"perl":         {"pl", "pm", "pod", "t", "psgi"},
		"r":            {"r", "R", "rdata", "rds", "rda"},
		"matlab":       {"m", "mat", "fig", "mlx", "mex", "p", "mlapp"},
		"octave":       {"m", "oct"},
		"haskell":      {"hs", "lhs", "cabal"},
		"elm":          {"elm"},
		"clojure":      {"clj", "cljs", "cljc", "edn"},
		"erlang":       {"erl", "hrl", "escript"},
		"elixir":       {"ex", "exs"},
		"dart":         {"dart"},
		"vim":          {"vim", "vimrc"},
		"latex":        {"tex", "latex", "ltx", "dtx", "sty", "cls", "bib", "bst"},
		"makefile":     {"makefile", "mk", "mak"},
		"cmake":        {"cmake", "txt"}, // CMakeLists.txt
		"gradle":       {"gradle", "properties"},
		"groovy":       {"groovy", "gvy", "gy", "gsh"},
		"protobuf":     {"proto"},
		"graphql":      {"graphql", "gql"},
		"solidity":     {"sol"},
		"assembly":     {"asm", "s", "a"},
		"cobol":        {"cbl", "cob", "cpy"},
		"fortran":      {"f", "f77", "f90", "f95", "f03", "f08", "for", "ftn", "fpp"},
		"pascal":       {"pas", "pp", "inc"},
		"ada":          {"ada", "adb", "ads", "ali"},
		"prolog":       {"pl", "pro", "P"},
		"lisp":         {"lisp", "lsp", "l", "cl", "fasl"},
		"scheme":       {"scm", "ss", "sch", "rkt"},
		"smalltalk":    {"st", "cs"},
		"tcl":          {"tcl", "tk", "itcl", "itk"},
		"verilog":      {"v", "vh", "sv", "svh"},
		"vhdl":         {"vhd", "vhdl"},
		"zig":          {"zig"},
		"nim":          {"nim", "nims", "nimble"},
		"crystal":      {"cr"},
		"d":            {"d", "di"},
		"ocaml":        {"ml", "mli", "mll", "mly"},
		"reason":       {"re", "rei"},
		"purescript":   {"purs"},
		"idris":        {"idr", "lidr"},
		"agda":         {"agda"},
		"lean":         {"lean"},
		"coq":          {"v"},
		"isabelle":     {"thy"},
		"nix":          {"nix"},
		"dhall":        {"dhall"},
		"julia":        {"jl"},
		"moonscript":   {"moon"},
		"coffeescript": {"coffee", "litcoffee"},
		"livescript":   {"ls"},
		"pug":          {"pug", "jade"},
		"stylus":       {"styl"},
		"handlebars":   {"hbs", "handlebars"},
		"mustache":     {"mustache"},
		"twig":         {"twig"},
		"smarty":       {"tpl"},
		"velocity":     {"vm"},
		"freemarker":   {"ftl"},
		"thymeleaf":    {"html"},
		"razor":        {"cshtml", "vbhtml"},
		"erb":          {"erb"},
		"haml":         {"haml"},
		"slim":         {"slim"},
		"actionscript": {"as", "mxml"},
		"flex":         {"as", "mxml"},
		"cuda":         {"cu", "cuh"},
		"opencl":       {"cl"},
		"glsl":         {"glsl", "vert", "frag", "geom", "tesc", "tese", "comp"},
		"hlsl":         {"hlsl", "fx", "fxh"},
		"autohotkey":   {"ahk"},
		"autoit":       {"au3"},
		"batch":        {"bat", "cmd"},
		"applescript":  {"applescript", "scpt"},
		"vbscript":     {"vbs"},
		"jscript":      {"js"},
		"qml":          {"qml"},
		"gdscript":     {"gd"},
		"angelscript":  {"as"},
		"squirrel":     {"nut"},
		"red":          {"red", "reds"},
		"rebol":        {"r", "reb"},
		"factor":       {"factor"},
		"forth":        {"fth", "4th"},
		"postscript":   {"ps", "eps"},
		"povray":       {"pov"},
		"maxscript":    {"ms"},
		"mel":          {"mel"},
		"lsl":          {"lsl"},
		"pike":         {"pike"},
		"io":           {"io"},
		"boo":          {"boo"},
		"nemerle":      {"n"},
		"fantom":       {"fan"},
		"monkey":       {"monkey"},
		"cobra":        {"cobra"},
		"bro":          {"bro"},
		"chapel":       {"chpl"},
		"x10":          {"x10"},
		"ceylon":       {"ceylon"},
		"ooc":          {"ooc"},
		"vala":         {"vala", "vapi"},
		"genie":        {"gs"},
		"oxygene":      {"oxygene"},
		"delphi":       {"pas", "pp", "inc"},
		"modelica":     {"mo"},
		"mathematica":  {"m", "nb", "cdf"},
		"maple":        {"mpl"},
		"gap":          {"g", "gap"},
		"sage":         {"sage"},
		"magma":        {"m"},
		"mupad":        {"mu"},
		"maxima":       {"mac"},
		"scilab":       {"sci", "sce"},
		"labview":      {"vi"},
		"simulink":     {"mdl", "slx"},
		"abap":         {"abap"},
		"apex":         {"cls", "trigger"},
		"apl":          {"apl"},
		"awk":          {"awk"},
		"brainfuck":    {"bf", "b"},
		"befunge":      {"bf"},
		"whitespace":   {"ws"},
		"chef":         {"chef"},
		"piet":         {"piet"},
		"lolcode":      {"lol"},
		"malbolge":     {"mb"},
		"intercal":     {"i"},
		"unlambda":     {"unl"},
		"befunge93":    {"bf"},
		"grass":        {"grass"},
		"ook":          {"ook"},
		"zero":         {"0"},
		"one":          {"1"},
		"two":          {"2"},
		"three":        {"3"},
		"four":         {"4"},
		"five":         {"5"},
		"six":          {"6"},
		"seven":        {"7"},
		"eight":        {"8"},
		"nine":         {"9"},
	}

	return extensions[language]
}

type Gateway struct {
	Config  *config.GatewayConfig
	Clients map[string]transport.LSPClient
	Router  *Router
	Logger  *mcp.StructuredLogger
	Mu      sync.RWMutex

	// SmartRouter integration
	smartRouter      *SmartRouterImpl
	workspaceManager *WorkspaceManager
	projectRouter    *ProjectAwareRouter

	// SCIP-aware Smart Router integration
	scipStore           indexing.SCIPStore
	scipSmartRouter     SCIPSmartRouter
	scipRoutingProvider SCIPRoutingProvider
	adaptiveMetrics     *AdaptiveRoutingOptimizer
	enableSCIPRouting   bool

	// Performance monitoring
	performanceCache  PerformanceCache
	requestClassifier RequestClassifier
	// Multi-server management
	multiServerManager      *MultiServerManager
	enableConcurrentServers bool
	aggregatorRegistry      *AggregatorRegistry
	requestTracker          *RequestTracker

	// Configuration
	enableSmartRouting bool
	routingStrategies  map[string]RoutingStrategy

	// Enhancement components
	enableEnhancements bool
	health_monitor     *HealthMonitor
}

func NewGateway(config *config.GatewayConfig) (*Gateway, error) {
	logConfig := &mcp.LoggerConfig{
		Level:              mcp.LogLevelInfo,
		Component:          LoggerComponentGateway,
		EnableJSON:         false,
		EnableStackTrace:   false,
		EnableCaller:       true,
		EnableMetrics:      false,
		Output:             nil, // Uses default (stderr)
		IncludeTimestamp:   true,
		TimestampFormat:    TimestampFormatISO8601,
		MaxStackTraceDepth: 10,
		EnableAsyncLogging: false,
		AsyncBufferSize:    1000,
	}
	logger := mcp.NewStructuredLogger(logConfig)

	// Initialize enhanced components
	router := NewRouter()
	workspaceManager := NewWorkspaceManager(config, router, logger)
	projectRouter := NewProjectAwareRouter(router, workspaceManager, logger)

	// Initialize performance monitoring components
	performanceCache := NewPerformanceCache(logger)
	requestClassifier := NewRequestClassifier()
	health_monitor := NewHealthMonitor(10 * time.Second)
	// Initialize multi-server components
	aggregatorRegistry := NewAggregatorRegistry(logger)
	requestTracker := NewRequestTracker()

	// Initialize MultiServerManager if configuration supports it
	var multiServerManager *MultiServerManager
	enableConcurrentServers := false
	if config.EnableConcurrentServers && config.LanguagePools != nil {
		multiServerManager = NewMultiServerManager(config, nil)
		if err := multiServerManager.Initialize(); err != nil {
			logger.Warnf("Failed to initialize MultiServerManager: %v", err)
		} else {
			enableConcurrentServers = true
			logger.Info("MultiServerManager initialized successfully")
		}
	}

	// Create SmartRouter with all components
	smartRouter := NewSmartRouter(projectRouter, config, workspaceManager, logger)

	// Check for SmartRouter configuration
	enableSmartRouting := config.EnableSmartRouting
	enableEnhancements := config.EnableEnhancements

	// Initialize SCIP-aware components if enabled
	var scipStore indexing.SCIPStore
	var scipSmartRouter SCIPSmartRouter
	var scipRoutingProvider SCIPRoutingProvider
	var adaptiveMetrics *AdaptiveRoutingOptimizer
	enableSCIPRouting := false

	if config.EnableSCIPRouting && config.SCIPConfig != nil {
		// Initialize SCIP store and client from indexing package
		var scipClient *indexing.SCIPClient
		var scipMapper *indexing.LSPSCIPMapper

		// Initialize SCIP store if configured
		if config.PerformanceConfig != nil && config.PerformanceConfig.SCIP != nil &&
			config.PerformanceConfig.SCIP.Enabled && config.PerformanceConfig.SCIP.CacheConfig.Enabled {
			scipConfig := &indexing.SCIPConfig{
				CacheConfig: indexing.CacheConfig{
					Enabled: config.PerformanceConfig.SCIP.CacheConfig.Enabled,
					MaxSize: int(config.PerformanceConfig.SCIP.CacheConfig.MaxSize),
					TTL:     config.PerformanceConfig.SCIP.CacheConfig.TTL,
				},
			}
			scipStore = indexing.NewRealSCIPStore(scipConfig)
			logger.Infof("SCIP store initialized successfully")
		}

		// Initialize SCIP client if store is available
		if scipStore != nil {
			scipConfig := &indexing.SCIPConfig{}
			var err error
			scipClient, err = indexing.NewSCIPClient(scipConfig)
			if err != nil {
				logger.Errorf("Failed to create SCIP client: %v", err)
				scipClient = nil
			}

			// Initialize LSP-SCIP mapper
			scipMapper = indexing.NewLSPSCIPMapper(scipStore, scipConfig)
			logger.Infof("SCIP client and mapper initialized successfully")
		}

		// Initialize SCIP routing provider if components are available
		if scipStore != nil {
			// Convert config types
			providerConfig := convertToSCIPProviderConfig(config.SCIPConfig.ProviderConfig)
			provider, err := NewSCIPRoutingProvider(scipStore, scipClient, scipMapper, providerConfig, logger)
			if err != nil {
				logger.Warnf("Failed to initialize SCIP routing provider: %v", err)
			} else {
				scipRoutingProvider = provider
				logger.Infof("SCIP routing provider initialized successfully")
			}
		}

		// Initialize SCIP smart router if base components are available
		if scipStore != nil && smartRouter != nil {
			// Convert config types
			routerConfig := convertToSCIPRoutingConfig(config.SCIPConfig.RouterConfig)
			scipRouter, err := NewSCIPSmartRouter(smartRouter, scipStore, routerConfig, logger)
			if err != nil {
				logger.Warnf("Failed to initialize SCIP smart router: %v", err)
			} else {
				scipSmartRouter = scipRouter
				enableSCIPRouting = true
				logger.Infof("SCIP smart router initialized successfully")
			}
		}

		// Initialize adaptive routing optimizer if SCIP routing is enabled
		if enableSCIPRouting {
			// Convert config types
			optimizationConfig := convertToAdaptiveOptimizationConfig(config.SCIPConfig.OptimizationConfig)
			optimizer := NewAdaptiveRoutingOptimizer(optimizationConfig, logger)
			adaptiveMetrics = optimizer
			logger.Infof("Adaptive routing optimizer initialized successfully")
		}
	}
	gateway := &Gateway{
		Config:  config,
		Clients: make(map[string]transport.LSPClient),
		Router:  router,
		Logger:  logger,

		// SmartRouter components
		smartRouter:      smartRouter,
		workspaceManager: workspaceManager,
		projectRouter:    projectRouter,

		// SCIP-aware Smart Router integration
		scipStore:           scipStore,
		scipSmartRouter:     scipSmartRouter,
		scipRoutingProvider: scipRoutingProvider,
		adaptiveMetrics:     adaptiveMetrics,
		enableSCIPRouting:   enableSCIPRouting,

		// Performance monitoring
		performanceCache:  performanceCache,
		requestClassifier: requestClassifier,
		health_monitor:    health_monitor,

		// Multi-server management
		multiServerManager:      multiServerManager,
		enableConcurrentServers: enableConcurrentServers,
		aggregatorRegistry:      aggregatorRegistry,
		requestTracker:          requestTracker,
		// Configuration
		enableSmartRouting: enableSmartRouting,
		routingStrategies:  make(map[string]RoutingStrategy),
		enableEnhancements: enableEnhancements,
	}

	logger.Infof("Initializing %d LSP server clients", len(config.Servers))
	for _, serverConfig := range config.Servers {
		serverLogger := logger.WithField(LoggerFieldServerName, serverConfig.Name)

		serverLogger.Debugf("Creating LSP client: command=%s, transport=%s",
			serverConfig.Command, serverConfig.Transport)

		client, err := transport.NewLSPClient(transport.ClientConfig{
			Command:   serverConfig.Command,
			Args:      serverConfig.Args,
			Transport: serverConfig.Transport,
		})
		if err != nil {
			serverLogger.WithError(err).Error("Failed to create LSP client")
			return nil, fmt.Errorf("failed to create client for %s: %w", serverConfig.Name, err)
		}

		// Validate that the LSP server command exists
		serverLogger.Debug("Validating LSP server command exists")
		if _, err := exec.LookPath(serverConfig.Command); err != nil {
			serverLogger.WithError(err).Error("LSP server command not found")
			return nil, fmt.Errorf("LSP server command not found for %s: %s", serverConfig.Name, serverConfig.Command)
		}

		gateway.Clients[serverConfig.Name] = client
		gateway.Router.RegisterServer(serverConfig.Name, serverConfig.Languages)

		serverLogger.WithField("languages", serverConfig.Languages).
			Info("LSP client registered successfully")
	}

	return gateway, nil
}

func (g *Gateway) Start(ctx context.Context) error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if g.Logger != nil {
		g.Logger.Infof("Starting gateway with %d LSP server clients", len(g.Clients))
	}

	// Start clients asynchronously to improve startup performance
	var wg sync.WaitGroup
	errorCh := make(chan error, len(g.Clients))

	for name, client := range g.Clients {
		wg.Add(1)
		go func(clientName string, lspClient transport.LSPClient) {
			defer wg.Done()

			var clientLogger *mcp.StructuredLogger
			if g.Logger != nil {
				clientLogger = g.Logger.WithField(LoggerFieldServerName, clientName)
				clientLogger.Debug("Starting LSP client asynchronously")
			}

			if err := lspClient.Start(ctx); err != nil {
				if clientLogger != nil {
					clientLogger.WithError(err).Error("Failed to start LSP client")
				}
				errorCh <- fmt.Errorf("failed to start client %s: %w", clientName, err)
				return
			}

			if clientLogger != nil {
				clientLogger.Info("LSP client started successfully")
			}
		}(name, client)
	}

	// Wait for all clients to start with a timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All clients started successfully
		if g.Logger != nil {
			g.Logger.Info("Gateway started successfully - all LSP clients ready")
		}
	case err := <-errorCh:
		// At least one client failed to start
		if g.Logger != nil {
			g.Logger.Warnf("Gateway started with client errors: %v", err)
		}
		// Don't return error - gateway can still function with partial clients
	case <-time.After(2 * time.Second):
		// Timeout waiting for all clients - proceed anyway
		if g.Logger != nil {
			g.Logger.Warn("Gateway startup timeout - proceeding with available clients")
		}
	}

	// Start MultiServerManager if enabled
	if g.multiServerManager != nil {
		if err := g.multiServerManager.Start(); err != nil {
			if g.Logger != nil {
				g.Logger.Warnf("Failed to start MultiServerManager: %v", err)
			}
			g.enableConcurrentServers = false
		} else if g.Logger != nil {
			g.Logger.Info("MultiServerManager started successfully")
		}
	}

	if g.Logger != nil {
		g.Logger.Info("Gateway started successfully")
	}
	return nil
}

func (g *Gateway) Stop() error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if g.Logger != nil {
		g.Logger.Info("Stopping gateway and all LSP clients")
	}

	var errors []error
	for name, client := range g.Clients {
		var clientLogger *mcp.StructuredLogger
		if g.Logger != nil {
			clientLogger = g.Logger.WithField(LoggerFieldServerName, name)
			clientLogger.Debug("Stopping LSP client")
		}

		if err := client.Stop(); err != nil {
			if clientLogger != nil {
				clientLogger.WithError(err).Error("Failed to stop LSP client")
			}
			errors = append(errors, fmt.Errorf("failed to stop client %s: %w", name, err))
		} else {
			if clientLogger != nil {
				clientLogger.Info("LSP client stopped successfully")
			}
		}
	}

	// Stop MultiServerManager if enabled
	if g.multiServerManager != nil {
		if err := g.multiServerManager.Stop(); err != nil {
			if g.Logger != nil {
				g.Logger.Warnf("Failed to stop MultiServerManager: %v", err)
			}
			errors = append(errors, fmt.Errorf("failed to stop MultiServerManager: %w", err))
		} else if g.Logger != nil {
			g.Logger.Info("MultiServerManager stopped successfully")
		}
	}

	if g.Logger != nil {
		g.Logger.Info("Gateway stopped successfully")
	}

	// If there were any errors, return the first one (maintains backwards compatibility)
	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

func (g *Gateway) GetClient(serverName string) (transport.LSPClient, bool) {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	client, exists := g.Clients[serverName]
	return client, exists
}

// extractLanguageFromURI extracts the language from a file URI
func (g *Gateway) extractLanguageFromURI(uri string) (string, error) {
	if uri == "" {
		return "", fmt.Errorf("empty URI provided")
	}

	// Remove URI prefixes
	filePath := uri
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	} else if strings.HasPrefix(uri, URIPrefixWorkspace) {
		filePath = strings.TrimPrefix(uri, URIPrefixWorkspace)
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "", fmt.Errorf("cannot determine file type from URI: %s", uri)
	}

	ext = strings.TrimPrefix(ext, ".")

	// Use router to get language from extension
	language, exists := g.Router.GetLanguageByExtension(ext)
	if !exists {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	return language, nil
}

func (g *Gateway) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requestLogger := g.initializeRequestLogger(r)

	if !g.validateHTTPMethod(w, r, requestLogger) {
		return
	}

	w.Header().Set("Content-Type", HTTPContentTypeJSON)

	req, ok := g.parseAndValidateJSONRPC(w, r, requestLogger)
	if !ok {
		return
	}

	if requestLogger != nil {
		requestLogger = requestLogger.WithField("lsp_method", req.Method)
	}

	serverName, ok := g.handleRequestRouting(w, req, requestLogger)
	if !ok {
		return
	}

	if requestLogger != nil {
		requestLogger = requestLogger.WithField(LoggerFieldServerName, serverName)
		requestLogger.Debug("Routed request to LSP server")
	}

	// Enhanced request processing with multi-server support
	g.processLSPRequest(w, r, req, serverName, requestLogger, startTime)
}

// processLSPRequest processes LSP requests with support for concurrent multi-server operations
// processLSPRequest processes LSP requests with support for SCIP-aware routing and concurrent multi-server operations
func (g *Gateway) processLSPRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, serverName string, logger *mcp.StructuredLogger, startTime time.Time) {
	// Check if SCIP-aware routing is enabled and should be used for this request
	if g.enableSCIPRouting && g.scipSmartRouter != nil && g.shouldUseSCIPRouting(req.Method) {
		g.processSCIPAwareRequest(w, r, req, serverName, logger, startTime)
		return
	}

	// Determine if request should use concurrent servers
	if g.shouldUseConcurrentServers(req.Method) {
		g.processConcurrentLSPRequest(w, r, req, logger, startTime)
		return
	}

	// Fallback to single server processing
	g.processSingleServerRequest(w, r, req, serverName, logger, startTime)
}

// shouldUseConcurrentServers determines if a method benefits from concurrent processing
func (g *Gateway) shouldUseConcurrentServers(method string) bool {
	if !g.enableConcurrentServers || g.multiServerManager == nil {
		return false
	}

	concurrentMethods := map[string]bool{
		LSP_METHOD_REFERENCES:       true,  // Aggregate from multiple servers
		LSP_METHOD_WORKSPACE_SYMBOL: true,  // Search across all servers
		LSP_METHOD_DOCUMENT_SYMBOL:  false, // Single server sufficient
		LSP_METHOD_DEFINITION:       false, // Single server sufficient
		LSP_METHOD_HOVER:            false, // Single server sufficient
	}

	return concurrentMethods[method]
}

// shouldUseSCIPRouting determines if SCIP routing should be used for a method
func (g *Gateway) shouldUseSCIPRouting(method string) bool {
	if !g.enableSCIPRouting || g.scipSmartRouter == nil {
		return false
	}

	// SCIP routing is beneficial for symbol-related queries
	scipBeneficialMethods := map[string]bool{
		LSP_METHOD_DEFINITION:       true, // Symbol definitions work well with SCIP
		LSP_METHOD_REFERENCES:       true, // Reference finding benefits from SCIP cache
		LSP_METHOD_DOCUMENT_SYMBOL:  true, // Document symbols can be cached
		LSP_METHOD_WORKSPACE_SYMBOL: true, // Workspace symbol search benefits from indexing
		LSP_METHOD_HOVER:            true, // Hover information can be cached
	}

	return scipBeneficialMethods[method]
}

// processSCIPAwareRequest processes a request using SCIP-aware routing with intelligent fallback
func (g *Gateway) processSCIPAwareRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, serverName string, logger *mcp.StructuredLogger, startTime time.Time) {
	// Create LSP request for SCIP routing
	lspRequest := &LSPRequest{
		Method:  req.Method,
		Params:  req.Params,
		URI:     g.extractURIFromParamsSimple(req.Params),
		Context: g.createRequestContext(req, serverName),
	}

	// Get SCIP routing decision
	decision, err := g.scipSmartRouter.RouteWithSCIPAwareness(lspRequest)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("SCIP routing decision failed, falling back to standard routing")
		}
		g.processSingleServerRequest(w, r, req, serverName, logger, startTime)
		return
	}

	// Track routing decision for optimization
	if g.adaptiveMetrics != nil {
		trackedDecision := g.createTrackedDecision(decision, req, startTime)
		g.adaptiveMetrics.TrackRoutingDecision(trackedDecision)
	}

	// Process based on routing decision
	if decision.UseSCIP && decision.SCIPResult != nil && decision.SCIPResult.Found {
		// Use SCIP result
		g.processSCIPResponse(w, req, decision, logger, startTime)
	} else if decision.FallbackToLSP {
		// Fallback to LSP server
		if logger != nil {
			logger.Debugf("SCIP routing recommends LSP fallback: %s", decision.DecisionReason)
		}
		g.processSingleServerRequest(w, r, req, serverName, logger, startTime)
	} else {
		// Use routing decision's recommended approach
		if decision.RoutingDecision != nil {
			g.processRoutingDecision(w, r, req, decision.RoutingDecision, logger, startTime)
		} else {
			// Final fallback to standard processing
			g.processSingleServerRequest(w, r, req, serverName, logger, startTime)
		}
	}
}

// processSCIPResponse processes a successful SCIP query result
func (g *Gateway) processSCIPResponse(w http.ResponseWriter, req JSONRPCRequest, decision *SCIPRoutingDecision, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.Debugf("Processing SCIP result for %s (confidence: %.2f, cache_hit: %v)",
			req.Method, decision.SCIPConfidence, decision.SCIPResult.CacheHit)
	}

	// Create JSON-RPC response from SCIP result
	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  decision.SCIPResult.Response,
	}

	// Add performance headers if available
	if decision.SCIPResult.QueryTime > 0 {
		w.Header().Set("X-Query-Time", decision.SCIPResult.QueryTime.String())
	}
	w.Header().Set("X-Source", "SCIP")
	w.Header().Set("X-Cache-Hit", fmt.Sprintf("%v", decision.SCIPResult.CacheHit))
	w.Header().Set("X-Confidence", fmt.Sprintf("%.2f", decision.SCIPConfidence))

	// Send response
	w.Header().Set("Content-Type", HTTPContentTypeJSON)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to encode SCIP response")
		}
		g.writeError(w, req.ID, InternalError, "Failed to encode response", fmt.Errorf("failed to encode response"))
		return
	}

	// Log performance metrics
	if logger != nil {
		totalTime := time.Since(startTime)
		logger.WithFields(map[string]interface{}{
			"method":        req.Method,
			"source":        "SCIP",
			"cache_hit":     decision.SCIPResult.CacheHit,
			"confidence":    decision.SCIPConfidence,
			"query_time":    decision.SCIPResult.QueryTime,
			"total_time":    totalTime,
			"decision_time": decision.DecisionLatency,
		}).Info("SCIP request completed")
	}
}

// extractURIFromParamsSimple extracts file URI from LSP request parameters (simple version)
func (g *Gateway) extractURIFromParamsSimple(params interface{}) string {
	if params == nil {
		return ""
	}

	// Handle different parameter structures
	switch p := params.(type) {
	case map[string]interface{}:
		// TextDocumentPositionParams or TextDocumentParams
		if textDoc, ok := p["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDoc["uri"].(string); ok {
				return uri
			}
		}
		// WorkspaceSymbolParams or other params
		if uri, ok := p["uri"].(string); ok {
			return uri
		}
	}

	return ""
}

// createRequestContext creates request context for SCIP routing
func (g *Gateway) createRequestContext(req JSONRPCRequest, serverName string) *RequestContext {
	return &RequestContext{
		ProjectType: "unknown", // Could be enhanced with project detection
		ServerName:  serverName,
		Language:    "", // Could be extracted from URI
		WorkspaceID: "", // Could be extracted from workspace detection
	}
}

// createTrackedDecision creates a tracked decision for performance optimization
func (g *Gateway) createTrackedDecision(decision *SCIPRoutingDecision, req JSONRPCRequest, startTime time.Time) *TrackedDecision {
	return &TrackedDecision{
		ID:              fmt.Sprintf("%v_%d", req.ID, time.Now().UnixNano()),
		Timestamp:       startTime,
		Method:          req.Method,
		Strategy:        decision.Strategy,
		Source:          g.getSourceFromDecision(decision),
		Confidence:      decision.Confidence,
		ExpectedLatency: decision.ExpectedLatency,
		QueryAnalysis:   decision.QueryAnalysis,
		DecisionReason:  decision.DecisionReason,
		SCIPQueried:     decision.SCIPQueried,
		UsedFallback:    decision.FallbackToLSP,
		ActualLatency:   0,     // Will be updated when request completes
		Success:         false, // Will be updated when request completes
		AccuracyScore:   float64(decision.SCIPConfidence),
		CacheHit:        decision.SCIPResult != nil && decision.SCIPResult.CacheHit,
		ComplexityLevel: decision.QueryAnalysis.Complexity,
	}
}

// getSourceFromDecision determines the routing source from a SCIP decision
func (g *Gateway) getSourceFromDecision(decision *SCIPRoutingDecision) RoutingSource {
	if decision.UseSCIP {
		return SourceSCIPCache
	}
	return SourceLSPServer
}

// processRoutingDecision processes a routing decision from SCIP smart router
func (g *Gateway) processRoutingDecision(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, decision *RoutingDecision, logger *mcp.StructuredLogger, startTime time.Time) {
	// This would implement processing based on the routing decision
	// For now, fall back to standard processing
	if logger != nil {
		logger.Debug("Processing routing decision - falling back to standard processing")
	}
	// Get server name from decision or fall back to original routing
	serverName, err := g.routeRequest(req)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to route request after routing decision")
		}
		g.writeError(w, req.ID, InternalError, "Failed to route request", fmt.Errorf("failed to route request"))
		return
	}

	g.processSingleServerRequest(w, r, req, serverName, logger, startTime)
}

// getMaxServersForMethod returns the maximum number of servers to use for a method
func (g *Gateway) getMaxServersForMethod(method string) int {
	maxServers := map[string]int{
		LSP_METHOD_REFERENCES:       3, // Use up to 3 servers for references
		LSP_METHOD_WORKSPACE_SYMBOL: 5, // Use up to 5 servers for symbol search
	}

	if max, exists := maxServers[method]; exists {
		return max
	}
	return 1
}

// processConcurrentLSPRequest handles concurrent requests to multiple servers
func (g *Gateway) processConcurrentLSPRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.Debug("Processing concurrent LSP request with multiple servers")
	}

	// Extract language from URI
	uri, err := g.extractURI(req)
	if err != nil {
		g.writeError(w, req.ID, InvalidParams, "Invalid parameters", err)
		return
	}

	language, err := g.extractLanguageFromURI(uri)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to extract language from URI, falling back to single server")
		}
		// Fallback to single server processing
		serverName, err := g.routeRequest(req)
		if err != nil {
			g.writeError(w, req.ID, MethodNotFound, "Method not found", err)
			return
		}
		g.processSingleServerRequest(w, r, req, serverName, logger, startTime)
		return
	}

	// Get multiple servers for concurrent processing
	maxServers := g.getMaxServersForMethod(req.Method)
	servers, err := g.multiServerManager.GetServersForConcurrentRequestWithType(language, req.Method, maxServers)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to get servers for concurrent request, falling back to single server")
		}
		// Fallback to single server processing
		g.handleConcurrentRequestError(w, r, req, language, err, logger, startTime)
		return
	}

	if len(servers) == 0 {
		g.writeError(w, req.ID, InternalError, "No servers available", fmt.Errorf("no servers available for language %s", language))
		return
	}

	// Generate request ID for tracking
	requestID := fmt.Sprintf("concurrent_%d_%s", time.Now().UnixNano(), req.Method)
	g.requestTracker.TrackRequest(requestID, req.Method, language, len(servers))

	// Execute concurrent requests
	responses, err := g.executeConcurrentRequests(r.Context(), servers, req.Method, req.Params)
	if err != nil {
		g.requestTracker.CompleteRequest(requestID)
		if logger != nil {
			logger.WithError(err).Error("Concurrent requests failed")
		}
		g.handleConcurrentRequestError(w, r, req, language, err, logger, startTime)
		return
	}

	// Aggregate responses
	aggregatedResponse, err := g.aggregateResponses(req.Method, responses, servers)
	if err != nil {
		g.requestTracker.CompleteRequest(requestID)
		if logger != nil {
			logger.WithError(err).Error("Response aggregation failed")
		}
		g.writeError(w, req.ID, InternalError, "Aggregation failed", err)
		return
	}

	// Send response
	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  aggregatedResponse.MergedResponse,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to encode concurrent response")
		}
		g.writeError(w, req.ID, InternalError, "Internal error", err)
		return
	}

	g.requestTracker.CompleteRequest(requestID)
	duration := time.Since(startTime)

	if logger != nil {
		logger.WithFields(map[string]interface{}{
			"duration":     duration.String(),
			"server_count": len(servers),
			"method":       req.Method,
			"language":     language,
		}).Info("Concurrent request processed successfully")
	}
}

// executeConcurrentRequests executes requests concurrently across multiple servers
func (g *Gateway) executeConcurrentRequests(ctx context.Context, servers []*ServerInstance, method string, params interface{}) ([]interface{}, error) {
	responses := make([]interface{}, len(servers))
	errors := make([]error, len(servers))
	var wg sync.WaitGroup

	// Create context with timeout
	timeout, err := time.ParseDuration(g.Config.Timeout)
	if err != nil {
		timeout = 30 * time.Second // Default timeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for i, server := range servers {
		wg.Add(1)
		go func(index int, srv *ServerInstance) {
			defer wg.Done()

			select {
			case <-timeoutCtx.Done():
				errors[index] = timeoutCtx.Err()
				return
			default:
				response, err := srv.SendRequest(timeoutCtx, method, params)
				responses[index] = response
				errors[index] = err
			}
		}(i, server)
	}

	wg.Wait()

	// Filter successful responses
	var successfulResponses []interface{}
	for i, err := range errors {
		if err == nil && responses[i] != nil {
			successfulResponses = append(successfulResponses, responses[i])
		}
	}

	if len(successfulResponses) == 0 {
		return nil, fmt.Errorf("all concurrent requests failed")
	}

	return successfulResponses, nil
}

// aggregateResponses aggregates responses from multiple servers
func (g *Gateway) aggregateResponses(method string, responses []interface{}, servers []*ServerInstance) (*AggregationResult, error) {
	sources := make([]string, len(servers))
	for i, server := range servers {
		sources[i] = server.config.Name
	}

	return g.aggregatorRegistry.AggregateResponses(method, responses, sources)
}

// handleConcurrentRequestError handles errors in concurrent requests with fallback
func (g *Gateway) handleConcurrentRequestError(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, language string, originalErr error, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.WithError(originalErr).Warn("Concurrent request failed, falling back to single server")
	}

	// Fallback to single server
	serverName, err := g.routeRequest(req)
	if err != nil {
		g.writeError(w, req.ID, MethodNotFound, "Method not found", fmt.Errorf("concurrent request failed: %v, fallback routing failed: %v", originalErr, err))
		return
	}

	g.processSingleServerRequest(w, r, req, serverName, logger, startTime)
}

func (g *Gateway) routeRequest(req JSONRPCRequest) (string, error) {
	uri, err := g.extractURI(req)
	if err != nil {
		return "", err
	}

	switch req.Method {
	case LSPMethodInitialize, LSPMethodInitialized, LSPMethodShutdown, LSPMethodExit, LSPMethodWorkspaceSymbol, LSPMethodWorkspaceExecuteCommand:
		return uri, nil
	default:
		return g.Router.RouteRequest(uri)
	}
}

func (g *Gateway) extractURI(req JSONRPCRequest) (string, error) {
	if g.isServerManagementMethod(req.Method) {
		return g.getAnyAvailableServer()
	}

	return g.extractURIFromParams(req)
}

func (g *Gateway) isServerManagementMethod(method string) bool {
	switch method {
	case LSPMethodInitialize, LSPMethodInitialized, LSPMethodShutdown, LSPMethodExit,
		LSPMethodWorkspaceSymbol, LSPMethodWorkspaceExecuteCommand:
		return true
	default:
		return false
	}
}

func (g *Gateway) getAnyAvailableServer() (string, error) {
	g.Mu.RLock()
	defer g.Mu.RUnlock()

	for serverName := range g.Clients {
		return serverName, nil
	}
	return "", fmt.Errorf("no servers available")
}

func (g *Gateway) extractURIFromParams(req JSONRPCRequest) (string, error) {
	if req.Params == nil {
		return "", fmt.Errorf("missing parameters for method %s", req.Method)
	}

	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid parameters format for method %s", req.Method)
	}

	if uri, found := g.extractURIFromTextDocument(paramsMap); found {
		return uri, nil
	}

	if uri, found := g.extractURIFromDirectParam(paramsMap); found {
		return uri, nil
	}

	return "", fmt.Errorf("could not extract URI from parameters for method %s", req.Method)
}

func (g *Gateway) extractURIFromTextDocument(paramsMap map[string]interface{}) (string, bool) {
	textDoc, exists := paramsMap["textDocument"]
	if !exists {
		return "", false
	}

	textDocMap, ok := textDoc.(map[string]interface{})
	if !ok {
		return "", false
	}

	uri, exists := textDocMap["uri"]
	if !exists {
		return "", false
	}

	uriStr, ok := uri.(string)
	if !ok {
		return "", false
	}

	return uriStr, true
}

func (g *Gateway) extractURIFromDirectParam(paramsMap map[string]interface{}) (string, bool) {
	uri, exists := paramsMap["uri"]
	if exists {
		if uriStr, ok := uri.(string); ok {
			return uriStr, true
		}
	}
	return "", false
}

func (g *Gateway) initializeRequestLogger(r *http.Request) *mcp.StructuredLogger {
	requestID := "req_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	var requestLogger *mcp.StructuredLogger
	if g.Logger != nil {
		requestLogger = g.Logger.WithRequestID(requestID)
		requestLogger.WithFields(map[string]interface{}{
			"method":      r.Method,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		}).Info("Received HTTP request")
	}
	return requestLogger
}

func (g *Gateway) validateHTTPMethod(w http.ResponseWriter, r *http.Request, logger *mcp.StructuredLogger) bool {
	if r.Method != http.MethodPost {
		if logger != nil {
			logger.Warn("Invalid HTTP method, rejecting request")
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func (g *Gateway) parseAndValidateJSONRPC(w http.ResponseWriter, r *http.Request, logger *mcp.StructuredLogger) (JSONRPCRequest, bool) {
	var req JSONRPCRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to parse JSON-RPC request")
		}
		g.writeError(w, nil, ParseError, "Parse error", err)
		return req, false
	}

	if req.JSONRPC != JSONRPCVersion {
		if logger != nil {
			logger.WithField("jsonrpc_version", req.JSONRPC).Error("Invalid JSON-RPC version")
		}
		g.writeError(w, req.ID, InvalidRequest, ERROR_INVALID_REQUEST,
			fmt.Errorf(FORMAT_INVALID_JSON_RPC, req.JSONRPC))
		return req, false
	}

	if req.Method == "" {
		if logger != nil {
			logger.Error("Missing method field in JSON-RPC request")
		}
		g.writeError(w, req.ID, InvalidRequest, ERROR_INVALID_REQUEST,
			fmt.Errorf("missing method field"))
		return req, false
	}

	return req, true
}

func (g *Gateway) handleRequestRouting(w http.ResponseWriter, req JSONRPCRequest, logger *mcp.StructuredLogger) (string, bool) {
	serverName, err := g.routeRequest(req)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to route request to LSP server")
		}
		g.writeError(w, req.ID, MethodNotFound, "Method not found", err)
		return "", false
	}
	return serverName, true
}

// processSingleServerRequest processes a request using a single server (backward compatibility)
func (g *Gateway) processSingleServerRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, serverName string, logger *mcp.StructuredLogger, startTime time.Time) {
	client, exists := g.GetClient(serverName)
	if !exists {
		if logger != nil {
			logger.Error("LSP server not found")
		}
		g.writeError(w, req.ID, InternalError, ERROR_INTERNAL,
			fmt.Errorf(ERROR_SERVER_NOT_FOUND, serverName))
		return
	}

	if !client.IsActive() {
		if logger != nil {
			logger.Error("LSP server is not active")
		}
		g.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("server %s is not active", serverName))
		return
	}

	if req.ID == nil {
		g.handleNotification(w, r, req, client, logger, startTime)
		return
	}

	g.handleRequest(w, r, req, client, logger, startTime)
}

func (g *Gateway) handleNotification(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, client transport.LSPClient, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.Debug("Processing LSP notification (no response expected)")
	}

	err := client.SendNotification(r.Context(), req.Method, req.Params)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to send LSP notification")
		}
		g.writeError(w, req.ID, InternalError, "Internal error", err)
		return
	}

	duration := time.Since(startTime)
	if logger != nil {
		logger.WithField("duration", duration.String()).Info("LSP notification processed successfully")
	}
	w.WriteHeader(http.StatusOK)
}

func (g *Gateway) handleRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, client transport.LSPClient, logger *mcp.StructuredLogger, startTime time.Time) {
	// SCIP cache-first routing: Check SCIP cache before LSP call
	if g.enableSCIPRouting && g.scipStore != nil {
		if logger != nil {
			logger.Debug("Checking SCIP cache for request")
		}

		// Query SCIP cache
		scipResult := g.scipStore.Query(req.Method, req.Params)
		if scipResult.Found {
			if logger != nil {
				logger.WithFields(map[string]interface{}{
					"cache_hit":  scipResult.CacheHit,
					"query_time": scipResult.QueryTime.String(),
					"confidence": scipResult.Confidence,
				}).Info("SCIP cache hit - returning cached response")
			}

			// Return cached response directly
			response := JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      req.ID,
				Result:  scipResult.Response,
			}

			if err := json.NewEncoder(w).Encode(response); err != nil {
				if logger != nil {
					logger.WithError(err).Error("Failed to encode cached JSON response")
				}
				g.writeError(w, req.ID, InternalError, "Internal error",
					fmt.Errorf("failed to encode cached response: %w", err))
				return
			}

			// Log cache hit metrics
			duration := time.Since(startTime)
			if logger != nil {
				logger.WithFields(map[string]interface{}{
					"duration":  duration.String(),
					"source":    "scip_cache",
					"cache_hit": true,
				}).Info("Cached request processed successfully")
			}
			return
		}

		if logger != nil {
			logger.Debug("SCIP cache miss - falling back to LSP server")
		}
	}

	// Original LSP server call (fallback or when SCIP is disabled)
	if logger != nil {
		logger.Debug("Sending request to LSP server")
	}

	result, err := client.SendRequest(r.Context(), req.Method, req.Params)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("LSP server request failed")
		}

		// Check if error is due to context deadline exceeded
		if err == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded") {
			g.writeError(w, req.ID, InternalError, "Request timeout: context deadline exceeded", err)
		} else {
			g.writeError(w, req.ID, InternalError, "Internal error", err)
		}
		return
	}

	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}

	// Cache the successful LSP response in SCIP store
	if g.enableSCIPRouting && g.scipStore != nil {
		// Convert result to json.RawMessage for caching
		resultBytes, err := json.Marshal(result)
		if err == nil {
			if cacheErr := g.scipStore.CacheResponse(req.Method, req.Params, json.RawMessage(resultBytes)); cacheErr != nil {
				if logger != nil {
					logger.WithError(cacheErr).Warn("Failed to cache LSP response in SCIP store")
				}
			} else {
				if logger != nil {
					logger.Debug("Successfully cached LSP response in SCIP store")
				}
			}
		} else {
			if logger != nil {
				logger.WithError(err).Warn("Failed to marshal LSP result for SCIP caching")
			}
		}
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to encode JSON response")
		}
		g.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("failed to encode response: %w", err))
		return
	}

	duration := time.Since(startTime)
	responseData, err := json.Marshal(response)
	responseSize := 0
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("Failed to marshal response for logging metrics")
		}
		// Estimate response size based on result length if marshaling fails
		if response.Result != nil {
			responseSize = len(fmt.Sprintf("%v", response.Result))
		}
	} else {
		responseSize = len(responseData)
	}

	if logger != nil {
		logger.WithFields(map[string]interface{}{
			"duration":      duration.String(),
			"response_size": responseSize,
			"source":        "lsp_server",
			"cache_hit":     false,
		}).Info("Request processed successfully")
	}
}

func (g *Gateway) writeError(w http.ResponseWriter, id interface{}, code int, message string, err error) {
	var data interface{}
	if err != nil {
		data = err.Error()
	}

	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Enhanced helper functions for robust file system operations

// Enhanced request processing methods for SmartRouter integration

// processAggregatedRequest handles requests that need aggregated responses from multiple servers
func (g *Gateway) processAggregatedRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.Debug("Processing aggregated request with multiple servers")
	}

	// Extract URI and language for SmartRouter
	uri, err := g.extractURI(req)
	if err != nil {
		g.writeError(w, req.ID, InvalidParams, "Invalid parameters", err)
		return
	}

	// Create LSPRequest for SmartRouter
	lspRequest := &LSPRequest{
		Method:  req.Method,
		Params:  req.Params,
		URI:     uri,
		Context: convertRoutingToRequestContext(CreateRequestContextFromURI(uri, "", "")),
	}

	// Use SmartRouter to aggregate responses
	aggregatedResponse, err := g.smartRouter.AggregateBroadcast(lspRequest)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Error("Aggregated request failed")
		}
		g.writeError(w, req.ID, InternalError, "Aggregation failed", err)
		return
	}

	// Convert aggregated response to JSON-RPC response
	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  aggregatedResponse.PrimaryResponse,
	}

	// Add aggregation metadata if available
	if len(aggregatedResponse.ResponseSources) > 0 {
		if resultMap, ok := response.Result.(map[string]interface{}); ok {
			resultMap["_sources"] = aggregatedResponse.ResponseSources
			resultMap["_aggregation_method"] = aggregatedResponse.AggregationMethod
			response.Result = resultMap
		}
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger != nil {
			logger.WithError(err).Error("Failed to encode aggregated response")
		}
		g.writeError(w, req.ID, InternalError, "Internal error", err)
		return
	}

	duration := time.Since(startTime)
	if logger != nil {
		logger.WithFields(map[string]interface{}{
			"duration":           duration.String(),
			"source_count":       len(aggregatedResponse.ResponseSources),
			"success_count":      aggregatedResponse.SuccessCount,
			"error_count":        aggregatedResponse.ErrorCount,
			"aggregation_method": aggregatedResponse.AggregationMethod,
		}).Info("Aggregated request processed successfully")
	}
}

// processEnhancedRequest handles requests with primary and enhancement servers
func (g *Gateway) processEnhancedRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, primaryServerName string, logger *mcp.StructuredLogger, startTime time.Time) {
	if logger != nil {
		logger.Debug("Processing enhanced request with primary and enhancement servers")
	}

	// For now, fallback to single server processing
	// Enhancement logic can be implemented later based on specific requirements
	g.processSingleServerRequestWithClient(w, r, req, primaryServerName, logger, startTime)
}

// processSingleServerRequestWithSmartRouter processes request using SmartRouter's client selection
func (g *Gateway) processSingleServerRequestWithSmartRouter(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, serverName string, logger *mcp.StructuredLogger, startTime time.Time) bool {
	// Try to get client through workspace manager if available
	if g.workspaceManager != nil {
		uri, err := g.extractURI(req)
		if err == nil {
			workspace, err := g.workspaceManager.GetOrCreateWorkspace(uri)
			if err == nil {
				language := ""
				if uri != "" {
					if lang, err := g.extractLanguageFromURI(uri); err == nil {
						language = lang
					}
				}

				if language != "" {
					client, err := workspace.getOrCreateLanguageClient(language, g.Config, g.Logger)
					if err == nil {
						g.processRequestWithClient(w, r, req, client, logger, startTime)
						return true
					}
				}
			}
		}
	}

	// Fallback to traditional client selection
	return false
}

// processSingleServerRequestWithClient processes request with a specific client
func (g *Gateway) processSingleServerRequestWithClient(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, serverName string, logger *mcp.StructuredLogger, startTime time.Time) {
	client, exists := g.GetClient(serverName)
	if !exists {
		if logger != nil {
			logger.Error("LSP server not found")
		}
		g.writeError(w, req.ID, InternalError, ERROR_INTERNAL,
			fmt.Errorf(ERROR_SERVER_NOT_FOUND, serverName))
		return
	}

	g.processRequestWithClient(w, r, req, client, logger, startTime)
}

// processRequestWithClient processes a request with the given client
func (g *Gateway) processRequestWithClient(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, client transport.LSPClient, logger *mcp.StructuredLogger, startTime time.Time) {
	if !client.IsActive() {
		if logger != nil {
			logger.Error("LSP server is not active")
		}
		g.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("server is not active"))
		return
	}

	if req.ID == nil {
		g.handleNotification(w, r, req, client, logger, startTime)
		return
	}

	g.handleRequest(w, r, req, client, logger, startTime)
}

// GetSmartRouter returns the SmartRouter instance for external access
func (g *Gateway) GetSmartRouter() *SmartRouterImpl {
	return g.smartRouter
}

// GetWorkspaceManager returns the WorkspaceManager instance for external access
func (g *Gateway) GetWorkspaceManager() *WorkspaceManager {
	return g.workspaceManager
}

// GetProjectRouter returns the ProjectAwareRouter instance for external access
func (g *Gateway) GetProjectRouter() *ProjectAwareRouter {
	return g.projectRouter
}

// SetRoutingStrategy sets a routing strategy for a specific LSP method
func (g *Gateway) SetRoutingStrategy(method string, strategy RoutingStrategy) {
	if g.smartRouter != nil {
		g.smartRouter.SetRoutingStrategy(method, RoutingStrategyType(strategy.Name()))
	}

	g.Mu.Lock()
	defer g.Mu.Unlock()
	g.routingStrategies[method] = strategy
}

// GetRoutingStrategy gets the routing strategy for a specific LSP method
func (g *Gateway) GetRoutingStrategy(method string) RoutingStrategy {
	if g.smartRouter != nil {
		return g.smartRouter.GetRoutingStrategy(method)
	}

	g.Mu.RLock()
	defer g.Mu.RUnlock()
	if strategy, exists := g.routingStrategies[method]; exists {
		return strategy
	}
	// Return nil as default when no strategy is set
	return nil
}

// GetRoutingMetrics returns current routing performance metrics
func (g *Gateway) GetRoutingMetrics() *RoutingMetrics {
	if g.smartRouter != nil {
		return g.smartRouter.GetRoutingMetrics()
	}
	return nil
}

// IsSmartRoutingEnabled returns whether smart routing is enabled
func (g *Gateway) IsSmartRoutingEnabled() bool {
	return g.enableSmartRouting
}

// EnableSmartRouting enables or disables smart routing
func (g *Gateway) EnableSmartRouting(enable bool) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	g.enableSmartRouting = enable

	if g.Logger != nil {
		g.Logger.Infof("Smart routing %s", map[bool]string{true: "enabled", false: "disabled"}[enable])
	}
}

// Multi-server management methods

// GetMultiServerManager returns the MultiServerManager instance
func (g *Gateway) GetMultiServerManager() *MultiServerManager {
	return g.multiServerManager
}

// IsConcurrentServersEnabled returns whether concurrent servers are enabled
func (g *Gateway) IsConcurrentServersEnabled() bool {
	return g.enableConcurrentServers
}

// EnableConcurrentServers enables or disables concurrent servers
func (g *Gateway) EnableConcurrentServers(enable bool) {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if !enable || g.multiServerManager == nil {
		g.enableConcurrentServers = false
	} else {
		g.enableConcurrentServers = enable
	}

	if g.Logger != nil {
		g.Logger.Infof("Concurrent servers %s", map[bool]string{true: "enabled", false: "disabled"}[g.enableConcurrentServers])
	}
}

// GetConcurrentRequestMetrics returns metrics about concurrent requests
func (g *Gateway) GetConcurrentRequestMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	if g.requestTracker != nil {
		metrics["active_concurrent_requests"] = g.requestTracker.GetActiveRequestCount()
	}

	if g.multiServerManager != nil {
		if managerMetrics := g.multiServerManager.GetMetrics(); managerMetrics != nil {
			metrics["manager_metrics"] = managerMetrics
		}
	}

	if g.aggregatorRegistry != nil {
		metrics["aggregation_enabled"] = true
	} else {
		metrics["aggregation_enabled"] = false
	}

	return metrics
}

// GetMethodConcurrencyInfo returns information about which methods use concurrent processing
func (g *Gateway) GetMethodConcurrencyInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// Define concurrent methods and their max servers
	concurrentMethods := map[string]bool{
		LSP_METHOD_REFERENCES:       true,
		LSP_METHOD_WORKSPACE_SYMBOL: true,
		LSP_METHOD_DOCUMENT_SYMBOL:  false,
		LSP_METHOD_DEFINITION:       false,
		LSP_METHOD_HOVER:            false,
	}

	maxServers := map[string]int{
		LSP_METHOD_REFERENCES:       3,
		LSP_METHOD_WORKSPACE_SYMBOL: 5,
	}

	for method, enabled := range concurrentMethods {
		methodInfo := map[string]interface{}{
			"concurrent_enabled": enabled,
			"max_servers":        1,
		}

		if max, exists := maxServers[method]; exists {
			methodInfo["max_servers"] = max
		}

		info[method] = methodInfo
	}

	return info
}

// RestartMultiServerManager restarts the multi-server manager
func (g *Gateway) RestartMultiServerManager() error {
	g.Mu.Lock()
	defer g.Mu.Unlock()

	if g.multiServerManager == nil {
		return fmt.Errorf("MultiServerManager is not initialized")
	}

	if g.Logger != nil {
		g.Logger.Info("Restarting MultiServerManager")
	}

	// Stop current manager
	if err := g.multiServerManager.Stop(); err != nil {
		if g.Logger != nil {
			g.Logger.WithError(err).Warn("Error stopping MultiServerManager during restart")
		}
	}

	// Reinitialize
	if err := g.multiServerManager.Initialize(); err != nil {
		g.enableConcurrentServers = false
		return fmt.Errorf("failed to reinitialize MultiServerManager: %w", err)
	}

	// Start again
	if err := g.multiServerManager.Start(); err != nil {
		g.enableConcurrentServers = false
		return fmt.Errorf("failed to restart MultiServerManager: %w", err)
	}

	if g.Logger != nil {
		g.Logger.Info("MultiServerManager restarted successfully")
	}

	return nil
}

// GetServerHealthStatus returns health status of all servers
func (g *Gateway) GetServerHealthStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// Traditional clients
	g.Mu.RLock()
	traditionalClients := make(map[string]bool)
	for name, client := range g.Clients {
		traditionalClients[name] = client.IsActive()
	}
	g.Mu.RUnlock()

	status["traditional_clients"] = traditionalClients

	// Multi-server manager status
	if g.multiServerManager != nil {
		if managerMetrics := g.multiServerManager.GetMetrics(); managerMetrics != nil {
			status["multi_server_metrics"] = managerMetrics
		}
		status["multi_server_enabled"] = g.enableConcurrentServers
	} else {
		status["multi_server_enabled"] = false
	}

	return status
}

// Conversion functions for config types
func convertToSCIPProviderConfig(cfg *config.SCIPProviderConfig) *SCIPProviderConfig {
	if cfg == nil {
		return nil
	}

	// Parse min acceptable quality
	quality := QualityLow // default
	switch cfg.MinAcceptableQuality {
	case "high":
		quality = QualityHigh
	case "medium":
		quality = QualityMedium
	case "low":
		quality = QualityLow
	}

	return &SCIPProviderConfig{
		HealthCheckInterval:  cfg.HealthCheckInterval,
		FallbackEnabled:      cfg.FallbackEnabled,
		FallbackTimeout:      cfg.FallbackTimeout,
		MinAcceptableQuality: quality,
		EnableMetricsLogging: cfg.EnableMetricsLogging,
		EnableHealthLogging:  cfg.EnableHealthLogging,
		// Leave nested configs nil for now - can be expanded if needed
		PerformanceTargets:      nil,
		RoutingHealthThresholds: nil,
		CacheSettings:           nil,
	}
}

func convertToSCIPRoutingConfig(cfg *config.SCIPRoutingConfig) *SCIPRoutingConfig {
	if cfg == nil {
		return nil
	}

	// Convert method strategies
	methodStrategies := make(map[string]SCIPRoutingStrategyType)
	for method, strategy := range cfg.MethodStrategies {
		methodStrategies[method] = SCIPRoutingStrategyType(strategy)
	}

	// Convert method confidence thresholds
	methodConfidenceThresholds := make(map[string]ConfidenceLevel)
	for method, threshold := range cfg.MethodConfidenceThresholds {
		methodConfidenceThresholds[method] = ConfidenceLevel(threshold)
	}

	return &SCIPRoutingConfig{
		DefaultStrategy:            SCIPRoutingStrategyType(cfg.DefaultStrategy),
		ConfidenceThreshold:        ConfidenceLevel(cfg.ConfidenceThreshold),
		MaxSCIPLatency:             cfg.MaxSCIPLatency,
		EnableFallback:             cfg.EnableFallback,
		EnableAdaptiveLearning:     cfg.EnableAdaptiveLearning,
		CachePrewarmEnabled:        cfg.CachePrewarmEnabled,
		MethodStrategies:           methodStrategies,
		MethodConfidenceThresholds: methodConfidenceThresholds,
		OptimizationInterval:       cfg.OptimizationInterval,
		PerformanceWindowSize:      cfg.PerformanceWindowSize,
	}
}

func convertToAdaptiveOptimizationConfig(cfg *config.AdaptiveOptimizationConfig) *AdaptiveOptimizationConfig {
	if cfg == nil {
		return nil
	}

	return &AdaptiveOptimizationConfig{
		OptimizationInterval:     cfg.OptimizationInterval,
		MinDataPoints:            cfg.MinDataPoints,
		ConfidenceThreshold:      cfg.ConfidenceThreshold,
		PerformanceThreshold:     cfg.PerformanceThreshold,
		LearningRate:             cfg.LearningRate,
		AdaptationRate:           cfg.AdaptationRate,
		ExplorationRate:          cfg.ExplorationRate,
		MaxThresholdAdjustment:   cfg.MaxThresholdAdjustment,
		ThresholdDecayRate:       cfg.ThresholdDecayRate,
		StrategyEvaluationWindow: cfg.StrategyEvaluationWindow,
		MinStrategyConfidence:    cfg.MinStrategyConfidence,
		CacheWarmingEnabled:      cfg.CacheWarmingEnabled,
		WarmingTriggerThreshold:  cfg.WarmingTriggerThreshold,
		MaxWarmingOperations:     cfg.MaxWarmingOperations,
	}
}
