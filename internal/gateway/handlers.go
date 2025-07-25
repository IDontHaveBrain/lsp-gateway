package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/config"
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

	gateway := &Gateway{
		Config:  config,
		Clients: make(map[string]transport.LSPClient),
		Router:  router,
		Logger:  logger,

		// SmartRouter components
		smartRouter:      smartRouter,
		workspaceManager: workspaceManager,
		projectRouter:    projectRouter,

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
func (g *Gateway) processLSPRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, serverName string, logger *mcp.StructuredLogger, startTime time.Time) {
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

func extractProjectContextFromURI(uri string) (string, string, error) {
	if uri == "" {
		return "", "", fmt.Errorf("empty URI provided")
	}

	var filePath string
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	} else if strings.HasPrefix(uri, URIPrefixWorkspace) {
		filePath = strings.TrimPrefix(uri, URIPrefixWorkspace)
	} else {
		filePath = uri
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute path for %s: %w", filePath, err)
	}

	projectRoot := findProjectRoot(absPath)
	if projectRoot == "" {
		projectRoot = filepath.Dir(absPath)
	}

	workspaceID := generateWorkspaceID(projectRoot)
	return workspaceID, projectRoot, nil
}

// ProjectRootResult represents enhanced project root detection results
type ProjectRootResult struct {
	MainRoot        string                      `json:"main_root"`           // Primary project root
	LanguageRoots   map[string]string           `json:"language_roots"`      // Language -> specific root path
	DetectedMarkers []string                    `json:"detected_markers"`    // All markers found
	ProjectType     string                      `json:"project_type"`        // Type of project detected
	IsNested        bool                        `json:"is_nested"`           // Whether nested projects exist
	Confidence      float64                     `json:"confidence"`          // Detection confidence (0.0-1.0)
	ScanDepth       int                         `json:"scan_depth"`          // Maximum depth scanned
	Languages       map[string]*LanguageContext `json:"languages,omitempty"` // Language contexts if available
}

// Enhanced multi-language root detection with comprehensive analysis
func findProjectRootMultiLanguage(startPath string) (*ProjectRootResult, error) {
	const maxScanDepth = 10

	if startPath == "" {
		return nil, fmt.Errorf("empty start path provided")
	}

	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Initialize scanner for comprehensive analysis
	scanner := NewProjectLanguageScanner()
	scanner.SetMaxDepth(3) // Limit depth for performance

	result := &ProjectRootResult{
		LanguageRoots:   make(map[string]string),
		DetectedMarkers: []string{},
		Languages:       make(map[string]*LanguageContext),
		ScanDepth:       0,
	}

	// Define language-specific marker groups
	languageMarkers := map[string][]string{
		"go":         {"go.mod", "go.sum", "go.work"},
		"python":     {"pyproject.toml", "setup.py", "requirements.txt", "Pipfile", "setup.cfg"},
		"javascript": {"package.json", "yarn.lock", "package-lock.json", "pnpm-lock.yaml"},
		"typescript": {"tsconfig.json", "package.json"},
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle"},
		"rust":       {"Cargo.toml", "Cargo.lock"},
		"cpp":        {"CMakeLists.txt", "Makefile", "configure.ac", "meson.build"},
		"csharp":     {"*.csproj", "*.sln", "Directory.Build.props"},
		"ruby":       {"Gemfile", "Gemfile.lock", "*.gemspec", "Rakefile"},
		"php":        {"composer.json", "composer.lock"},
	}

	// Version control markers
	vcsMarkers := []string{".git", ".hg", ".svn", ".bzr"}

	currentDir := absPath
	if !isDirectory(currentDir) {
		currentDir = filepath.Dir(currentDir)
	}

	foundRoots := make(map[string]string) // level -> root path
	languageRootMap := make(map[string]string)
	allMarkers := []string{}
	var primaryRoot string
	var maxConfidence float64
	depth := 0

	// Scan upward through directory hierarchy
	for depth < maxScanDepth {
		levelMarkers := []string{}
		levelConfidence := 0.0
		foundLanguages := []string{}

		// Check for VCS markers (high confidence indicators)
		for _, vcsMarker := range vcsMarkers {
			markerPath := filepath.Join(currentDir, vcsMarker)
			if fileExists(markerPath) {
				levelMarkers = append(levelMarkers, vcsMarker)
				levelConfidence += 0.3
				foundRoots[fmt.Sprintf("vcs_%d", depth)] = currentDir
			}
		}

		// Check language-specific markers
		for language, markers := range languageMarkers {
			languageFound := false
			for _, marker := range markers {
				var markerPath string
				if strings.Contains(marker, "*") {
					// Handle wildcard patterns
					matches, err := filepath.Glob(filepath.Join(currentDir, marker))
					if err == nil && len(matches) > 0 {
						markerPath = matches[0]
					}
				} else {
					markerPath = filepath.Join(currentDir, marker)
				}

				if markerPath != "" && fileExists(markerPath) {
					levelMarkers = append(levelMarkers, marker)
					if !languageFound {
						foundLanguages = append(foundLanguages, language)
						languageFound = true
						levelConfidence += 0.4

						// Store language-specific root
						if _, exists := languageRootMap[language]; !exists {
							languageRootMap[language] = currentDir
						}
					}
				}
			}
		}

		// Store level information if markers found
		if len(levelMarkers) > 0 {
			allMarkers = append(allMarkers, levelMarkers...)
			foundRoots[fmt.Sprintf("level_%d", depth)] = currentDir

			// Update primary root based on confidence
			if levelConfidence > maxConfidence {
				maxConfidence = levelConfidence
				primaryRoot = currentDir
			}
		}

		// Check if we've reached filesystem root
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
		depth++
	}

	// Determine primary root if not set by confidence
	if primaryRoot == "" && len(foundRoots) > 0 {
		// Prefer the deepest root with markers (closest to start path)
		minDepth := maxScanDepth
		for key, root := range foundRoots {
			if strings.HasPrefix(key, "level_") {
				if levelDepth := depth - len(strings.Split(key, "_")); levelDepth < minDepth {
					minDepth = levelDepth
					primaryRoot = root
				}
			}
		}

		// Fallback to VCS root if no level root found
		if primaryRoot == "" {
			for key, root := range foundRoots {
				if strings.HasPrefix(key, "vcs_") {
					primaryRoot = root
					break
				}
			}
		}
	}

	// Calculate overall confidence
	overallConfidence := maxConfidence
	if len(languageRootMap) > 1 {
		overallConfidence += 0.2 // Bonus for multi-language detection
	}
	if len(allMarkers) > 3 {
		overallConfidence += 0.1 // Bonus for comprehensive markers
	}
	if overallConfidence > 1.0 {
		overallConfidence = 1.0
	}

	// Detect nested projects
	isNested := len(languageRootMap) > 1 && len(foundRoots) > 1

	// Determine project type
	projectType := determineProjectType(languageRootMap, foundRoots, allMarkers)

	// Perform comprehensive scan if we have a good root
	if primaryRoot != "" && overallConfidence > 0.5 {
		if info, err := scanner.ScanProjectComprehensive(primaryRoot); err == nil {
			result.Languages = info.Languages
			result.ProjectType = info.ProjectType

			// Update language roots from comprehensive scan
			for lang, ctx := range info.Languages {
				if ctx.RootPath != "" {
					languageRootMap[lang] = ctx.RootPath
				}
			}
		}
	}

	// Populate result
	result.MainRoot = primaryRoot
	result.LanguageRoots = languageRootMap
	result.DetectedMarkers = removeDuplicateStrings(allMarkers)
	result.IsNested = isNested
	result.Confidence = overallConfidence
	result.ScanDepth = depth

	if result.ProjectType == "" {
		result.ProjectType = projectType
	}

	return result, nil
}

// Backward compatibility wrapper - returns simple string result
func findProjectRoot(startPath string) string {
	result, err := findProjectRootMultiLanguage(startPath)
	if err != nil || result == nil || result.MainRoot == "" {
		// Fallback to simple detection
		return findProjectRootSimple(startPath)
	}
	return result.MainRoot
}

// Simple fallback detection for backward compatibility
func findProjectRootSimple(startPath string) string {
	projectMarkers := []string{
		"go.mod", "go.sum",
		"package.json", "tsconfig.json", "yarn.lock", "package-lock.json",
		"pyproject.toml", "setup.py", "requirements.txt", "Pipfile",
		"pom.xml", "build.gradle", "build.gradle.kts", "build.xml",
		".git", ".hg", ".svn",
		"Cargo.toml", "Makefile", "CMakeLists.txt",
	}

	currentDir := startPath
	if !isDirectory(currentDir) {
		currentDir = filepath.Dir(currentDir)
	}

	for {
		for _, marker := range projectMarkers {
			markerPath := filepath.Join(currentDir, marker)
			if fileExists(markerPath) {
				return currentDir
			}
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}

	return ""
}

func generateWorkspaceID(projectRoot string) string {
	projectName := filepath.Base(projectRoot)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	return fmt.Sprintf("ws_%s_%s", projectName, timestamp[:8])
}

// Enhanced helper functions for robust file system operations

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Additional helper functions for enhanced project root detection

// determineProjectType analyzes found markers and language distribution to determine project type
func determineProjectType(languageRoots map[string]string, foundRoots map[string]string, markers []string) string {
	numLanguages := len(languageRoots)
	numRoots := len(foundRoots)

	if numLanguages == 0 {
		return "empty"
	}

	if numLanguages == 1 {
		return "single-language"
	}

	// Check for common patterns based on markers
	hasDocker := containsMarker(markers, "docker")
	hasWorkspace := containsMarker(markers, "go.work") || hasWorkspacePackageJson(foundRoots)
	hasMultipleBuildSystems := countBuildSystems(markers) >= 2

	// Frontend-backend pattern detection
	hasFrontend := hasLanguage(languageRoots, "javascript", "typescript")
	hasBackend := hasLanguage(languageRoots, "go", "python", "java", "rust")

	if hasFrontend && hasBackend && numLanguages <= 3 {
		return "frontend-backend"
	}

	// Monorepo pattern (multiple languages with workspace setup)
	if hasWorkspace && numLanguages >= 2 {
		return "monorepo"
	}

	// Microservices pattern (multiple languages with separate build systems)
	if hasMultipleBuildSystems && numLanguages >= 3 && (hasDocker || numRoots > 2) {
		return "microservices"
	}

	// Polyglot pattern (multiple languages intermixed)
	if numLanguages >= 2 && numRoots <= 2 {
		return "polyglot"
	}

	// Multi-language (catch-all for multiple languages)
	return "multi-language"
}

// containsMarker checks if any marker contains the specified pattern
func containsMarker(markers []string, pattern string) bool {
	for _, marker := range markers {
		if strings.Contains(strings.ToLower(marker), strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// hasWorkspacePackageJson checks if any root contains a workspace-configured package.json
func hasWorkspacePackageJson(roots map[string]string) bool {
	for _, root := range roots {
		packageJsonPath := filepath.Join(root, "package.json")
		if fileExists(packageJsonPath) {
			// In a real implementation, we'd read and parse the file to check for workspaces
			// For now, assume it might be a workspace if it exists at a high level
			return true
		}
	}
	return false
}

// countBuildSystems counts distinct build systems in markers
func countBuildSystems(markers []string) int {
	buildSystems := make(map[string]bool)
	buildSystemMarkers := map[string]string{
		"go.mod":         "go",
		"package.json":   "npm",
		"pom.xml":        "maven",
		"build.gradle":   "gradle",
		"Cargo.toml":     "cargo",
		"Makefile":       "make",
		"CMakeLists.txt": "cmake",
		"setup.py":       "python",
	}

	for _, marker := range markers {
		if system, exists := buildSystemMarkers[marker]; exists {
			buildSystems[system] = true
		}
	}

	return len(buildSystems)
}

// hasLanguage checks if any of the specified languages exist in the language roots
func hasLanguage(languageRoots map[string]string, languages ...string) bool {
	for _, lang := range languages {
		if _, exists := languageRoots[lang]; exists {
			return true
		}
	}
	return false
}

// removeDuplicateStrings removes duplicate strings from a slice
func removeDuplicateStrings(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// Language-specific root detection functions

// findLanguageSpecificRoot finds the most specific root for a given language
func findLanguageSpecificRoot(startPath, language string) string {
	result, err := findProjectRootMultiLanguage(startPath)
	if err != nil || result == nil {
		return ""
	}

	if root, exists := result.LanguageRoots[language]; exists {
		return root
	}

	return result.MainRoot
}

// detectNestedProjects identifies nested projects within a root directory
func detectNestedProjects(rootPath string) map[string]string {
	nestedProjects := make(map[string]string)

	// Walk directory tree looking for project markers at different levels
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		if info.IsDir() {
			// Check if this directory contains project markers
			if hasProjectMarkers(path) && path != rootPath {
				relPath, _ := filepath.Rel(rootPath, path)
				projectType := detectProjectTypeForPath(path)
				nestedProjects[relPath] = projectType
			}

			// Skip deep nesting and common ignore directories
			depth := strings.Count(strings.TrimPrefix(path, rootPath), string(filepath.Separator))
			if depth > 3 || shouldSkipDirectory(info.Name()) {
				return filepath.SkipDir
			}
		}

		return nil
	})

	if err != nil {
		return make(map[string]string)
	}

	return nestedProjects
}

// hasProjectMarkers checks if a directory contains project markers
func hasProjectMarkers(dirPath string) bool {
	markers := []string{
		"go.mod", "package.json", "pom.xml", "Cargo.toml",
		"pyproject.toml", "setup.py", "build.gradle", "CMakeLists.txt",
	}

	for _, marker := range markers {
		if fileExists(filepath.Join(dirPath, marker)) {
			return true
		}
	}

	return false
}

// detectProjectTypeForPath detects project type for a specific path
func detectProjectTypeForPath(path string) string {
	// Simple detection based on primary marker found
	if fileExists(filepath.Join(path, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(path, "package.json")) {
		return "javascript"
	}
	if fileExists(filepath.Join(path, "pom.xml")) {
		return "java"
	}
	if fileExists(filepath.Join(path, "Cargo.toml")) {
		return "rust"
	}
	if fileExists(filepath.Join(path, "pyproject.toml")) || fileExists(filepath.Join(path, "setup.py")) {
		return "python"
	}

	return "unknown"
}

// shouldSkipDirectory determines if a directory should be skipped during scanning
func shouldSkipDirectory(dirName string) bool {
	skipDirs := []string{
		"node_modules", "venv", ".venv", "env", ".env",
		".git", ".svn", ".hg", "__pycache__", ".pytest_cache",
		"target", "build", "dist", "out", ".idea", ".vscode",
		"vendor", "coverage", ".gradle", ".m2",
	}

	lowerName := strings.ToLower(dirName)
	for _, skip := range skipDirs {
		if lowerName == skip || strings.HasPrefix(lowerName, ".") && len(dirName) > 1 {
			return true
		}
	}

	return false
}

// calculateRootConfidence calculates confidence score for a root path
func calculateRootConfidence(rootPath string, markers []string) float64 {
	confidence := 0.0

	// Base confidence from markers
	confidence += float64(len(markers)) * 0.1

	// VCS presence adds confidence
	vcsMarkers := []string{".git", ".hg", ".svn"}
	for _, vcs := range vcsMarkers {
		if fileExists(filepath.Join(rootPath, vcs)) {
			confidence += 0.3
			break
		}
	}

	// Build system markers add confidence
	buildMarkers := []string{"go.mod", "package.json", "pom.xml", "Cargo.toml"}
	buildCount := 0
	for _, marker := range buildMarkers {
		if fileExists(filepath.Join(rootPath, marker)) {
			buildCount++
		}
	}
	confidence += float64(buildCount) * 0.2

	// Configuration files add confidence
	configMarkers := []string{"tsconfig.json", "pyproject.toml", "build.gradle"}
	for _, marker := range configMarkers {
		if fileExists(filepath.Join(rootPath, marker)) {
			confidence += 0.1
		}
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// isMonorepoRoot determines if a path represents a monorepo root
func isMonorepoRoot(rootPath string) bool {
	// Check for workspace indicators
	workspaceIndicators := []string{
		"go.work",
		"lerna.json",
		"nx.json",
		"rush.json",
	}

	for _, indicator := range workspaceIndicators {
		if fileExists(filepath.Join(rootPath, indicator)) {
			return true
		}
	}

	// Check for package.json with workspaces
	packageJsonPath := filepath.Join(rootPath, "package.json")
	if fileExists(packageJsonPath) {
		// In a real implementation, we'd parse the JSON and check for workspaces field
		// For now, assume it's a monorepo if we find multiple package.json files in subdirs
		subDirPackages := 0
		entries, err := os.ReadDir(rootPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() && !shouldSkipDirectory(entry.Name()) {
					subPackagePath := filepath.Join(rootPath, entry.Name(), "package.json")
					if fileExists(subPackagePath) {
						subDirPackages++
					}
				}
			}
		}
		return subDirPackages >= 2
	}

	return false
}

// getProjectBoundaries identifies logical project boundaries within a directory tree
func getProjectBoundaries(startPath string) []string {
	var boundaries []string

	// Start from the path and work upward
	currentPath := startPath
	if !isDirectory(currentPath) {
		currentPath = filepath.Dir(currentPath)
	}

	maxDepth := 10
	depth := 0

	for depth < maxDepth {
		// Check if current path represents a project boundary
		if hasProjectMarkers(currentPath) {
			boundaries = append(boundaries, currentPath)
		}

		// Check if it's a VCS boundary
		vcsMarkers := []string{".git", ".hg", ".svn"}
		for _, vcs := range vcsMarkers {
			if fileExists(filepath.Join(currentPath, vcs)) {
				boundaries = append(boundaries, currentPath)
				break
			}
		}

		// Move up one level
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			break
		}
		currentPath = parent
		depth++
	}

	return removeDuplicateStrings(boundaries)
}

// Integration functions for MultiLanguageProjectInfo

// ToMultiLanguageProjectInfo converts ProjectRootResult to MultiLanguageProjectInfo
func (r *ProjectRootResult) ToMultiLanguageProjectInfo() *MultiLanguageProjectInfo {
	if r == nil {
		return nil
	}

	info := &MultiLanguageProjectInfo{
		RootPath:       r.MainRoot,
		ProjectType:    r.ProjectType,
		Languages:      r.Languages,
		WorkspaceRoots: r.LanguageRoots,
		BuildFiles:     []string{},
		ConfigFiles:    []string{},
		TotalFileCount: 0,
		ScanDepth:      r.ScanDepth,
		DetectedAt:     time.Now(),
		Metadata: map[string]interface{}{
			"detection_confidence": r.Confidence,
			"is_nested":            r.IsNested,
			"detected_markers":     r.DetectedMarkers,
		},
	}

	// Determine dominant language
	if len(r.Languages) > 0 {
		var maxPriority int
		for lang, ctx := range r.Languages {
			if ctx.Priority > maxPriority {
				maxPriority = ctx.Priority
				info.DominantLanguage = lang
			}
		}
	}

	// Collect build and config files from language contexts
	buildFiles := make(map[string]bool)
	configFiles := make(map[string]bool)
	totalFiles := 0

	for _, ctx := range r.Languages {
		for _, file := range ctx.BuildFiles {
			buildFiles[file] = true
		}
		for _, file := range ctx.ConfigFiles {
			configFiles[file] = true
		}
		totalFiles += ctx.FileCount + ctx.TestFileCount
	}

	// Convert to slices
	for file := range buildFiles {
		info.BuildFiles = append(info.BuildFiles, file)
	}
	for file := range configFiles {
		info.ConfigFiles = append(info.ConfigFiles, file)
	}

	info.TotalFileCount = totalFiles

	return info
}

// integrateRootDetectionWithScanner integrates root detection with the project scanner
func integrateRootDetectionWithScanner(startPath string) (*MultiLanguageProjectInfo, error) {
	// First, perform enhanced root detection
	rootResult, err := findProjectRootMultiLanguage(startPath)
	if err != nil {
		return nil, fmt.Errorf("root detection failed: %w", err)
	}

	if rootResult.MainRoot == "" {
		return nil, fmt.Errorf("no project root found for path: %s", startPath)
	}

	// If we already have comprehensive language information, use it
	if len(rootResult.Languages) > 0 {
		return rootResult.ToMultiLanguageProjectInfo(), nil
	}

	// Otherwise, perform comprehensive scanning
	scanner := NewProjectLanguageScanner()
	info, err := scanner.ScanProjectComprehensive(rootResult.MainRoot)
	if err != nil {
		return nil, fmt.Errorf("comprehensive scan failed: %w", err)
	}

	// Merge root detection results into scan results
	info.Metadata["root_detection"] = map[string]interface{}{
		"confidence":       rootResult.Confidence,
		"is_nested":        rootResult.IsNested,
		"detected_markers": rootResult.DetectedMarkers,
		"scan_depth":       rootResult.ScanDepth,
	}

	// Update workspace roots from root detection
	for lang, root := range rootResult.LanguageRoots {
		info.WorkspaceRoots[lang] = root
	}

	return info, nil
}

func (g *Gateway) enrichRequestWithWorkspaceContext(req JSONRPCRequest, workspaceID string) WorkspaceAwareJSONRPCRequest {
	workspaceReq := WorkspaceAwareJSONRPCRequest{
		JSONRPCRequest: req,
		WorkspaceID:    workspaceID,
	}

	uri, err := g.extractURIFromParams(req)
	if err == nil {
		_, projectPath, err := extractProjectContextFromURI(uri)
		if err == nil {
			workspaceReq.ProjectPath = projectPath
		}
	}

	return workspaceReq
}

func (g *Gateway) validateWorkspaceRequest(req JSONRPCRequest, workspace WorkspaceContext) error {
	if workspace == nil {
		return fmt.Errorf("workspace context is nil")
	}

	if !workspace.IsActive() {
		return fmt.Errorf("workspace %s is not active", workspace.GetID())
	}

	uri, err := g.extractURIFromParams(req)
	if err != nil {
		return fmt.Errorf("failed to extract URI from request: %w", err)
	}

	var filePath string
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	} else {
		filePath = uri
	}

	if !strings.HasPrefix(filePath, workspace.GetRootPath()) {
		return fmt.Errorf("file %s is not within workspace root %s", filePath, workspace.GetRootPath())
	}

	return nil
}

func getWorkspaceSpecificServerName(workspace WorkspaceContext, language string) string {
	if workspace == nil {
		return ""
	}

	projectType := workspace.GetProjectType()
	workspaceID := workspace.GetID()

	switch projectType {
	case "go":
		return fmt.Sprintf("go-lsp-%s", workspaceID)
	case "python":
		return fmt.Sprintf("python-lsp-%s", workspaceID)
	case "typescript", "javascript":
		return fmt.Sprintf("typescript-lsp-%s", workspaceID)
	case "java":
		return fmt.Sprintf("java-lsp-%s", workspaceID)
	default:
		return fmt.Sprintf("%s-lsp-%s", language, workspaceID)
	}
}

func logRequestWithWorkspaceContext(logger *mcp.StructuredLogger, workspace WorkspaceContext, method string) *mcp.StructuredLogger {
	if logger == nil {
		return nil
	}

	fields := map[string]interface{}{
		"lsp_method": method,
	}

	if workspace != nil {
		fields[LoggerFieldWorkspaceID] = workspace.GetID()
		fields[LoggerFieldProjectPath] = workspace.GetRootPath()
		fields[LoggerFieldProjectType] = workspace.GetProjectType()
		fields[LoggerFieldProjectName] = workspace.GetProjectName()
	}

	return logger.WithFields(fields)
}

func (g *Gateway) enrichRequestParamsWithWorkspaceInfo(params interface{}, workspace WorkspaceContext) interface{} {
	if params == nil || workspace == nil {
		return params
	}

	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return params
	}

	enrichedParams := make(map[string]interface{})
	for k, v := range paramsMap {
		enrichedParams[k] = v
	}

	enrichedParams["workspaceRoot"] = workspace.GetRootPath()
	enrichedParams["workspaceID"] = workspace.GetID()
	enrichedParams["projectType"] = workspace.GetProjectType()

	return enrichedParams
}

func (g *Gateway) extractWorkspaceURIs(req JSONRPCRequest) []string {
	var uris []string

	if req.Params == nil {
		return uris
	}

	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return uris
	}

	if uri, found := g.extractURIFromTextDocument(paramsMap); found {
		uris = append(uris, uri)
	}

	if uri, found := g.extractURIFromDirectParam(paramsMap); found {
		uris = append(uris, uri)
	}

	if workspaceFolders, exists := paramsMap["workspaceFolders"]; exists {
		if folders, ok := workspaceFolders.([]interface{}); ok {
			for _, folder := range folders {
				if folderMap, ok := folder.(map[string]interface{}); ok {
					if uri, exists := folderMap["uri"]; exists {
						if uriStr, ok := uri.(string); ok {
							uris = append(uris, uriStr)
						}
					}
				}
			}
		}
	}

	return uris
}

func (g *Gateway) isWorkspaceMethod(method string) bool {
	workspaceMethods := []string{
		LSPMethodWorkspaceSymbol,
		LSPMethodWorkspaceExecuteCommand,
		"workspace/didChangeWorkspaceFolders",
		"workspace/didChangeConfiguration",
		"workspace/didChangeWatchedFiles",
	}

	for _, wsMethod := range workspaceMethods {
		if method == wsMethod {
			return true
		}
	}

	return false
}

func (g *Gateway) createWorkspaceAwareResponse(response JSONRPCResponse, workspace WorkspaceContext) JSONRPCResponse {
	if workspace == nil {
		return response
	}

	if response.Result != nil {
		if resultMap, ok := response.Result.(map[string]interface{}); ok {
			enrichedResult := make(map[string]interface{})
			for k, v := range resultMap {
				enrichedResult[k] = v
			}
			enrichedResult["workspaceContext"] = map[string]interface{}{
				"id":          workspace.GetID(),
				"rootPath":    workspace.GetRootPath(),
				"projectType": workspace.GetProjectType(),
				"projectName": workspace.GetProjectName(),
			}
			response.Result = enrichedResult
		}
	}

	return response
}

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

	language := ""
	if uri != "" {
		if lang, err := g.extractLanguageFromURI(uri); err == nil {
			language = lang
		}
	}

	// Create LSPRequest for SmartRouter
	requestContext := &RequestContext{
		FileURI:     uri,
		Language:    language,
		RequestType: "file_based",
	}
	lspRequest := &LSPRequest{
		Method:    req.Method,
		Params:    req.Params,
		URI:       uri,
		Context:   requestContext,
		JSONRPC:   "2.0",
		Timestamp: time.Now(),
		RequestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
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
		Result:  aggregatedResponse.PrimaryResult,
	}

	// Add metadata if available
	if aggregatedResponse.Metadata != nil {
		if resultMap, ok := response.Result.(map[string]interface{}); ok {
			resultMap["_aggregation_metadata"] = aggregatedResponse.Metadata
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
			"duration":     duration.String(),
			"server_count": aggregatedResponse.ServerCount,
			"strategy":     string(aggregatedResponse.Strategy),
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
		g.smartRouter.SetRoutingStrategy(method, strategy)
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
	return SingleTargetWithFallback
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
