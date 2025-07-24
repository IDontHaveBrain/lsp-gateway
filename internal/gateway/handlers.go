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
	ERROR_SERVER_NOT_FOUND  = "Server %s not found"
	FORMAT_INVALID_JSON_RPC = "Invalid JSON-RPC version: %s"
)

const (
	LoggerFieldWorkspaceID   = "workspace_id"
	LoggerFieldProjectPath   = "project_path"
	LoggerFieldProjectType   = "project_type"
	LoggerFieldProjectName   = "project_name"
	URIPrefixWorkspace       = "workspace://"
)

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

	gateway := &Gateway{
		Config:  config,
		Clients: make(map[string]transport.LSPClient),
		Router:  NewRouter(),
		Logger:  logger,
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

	g.processLSPRequest(w, r, req, serverName, requestLogger, startTime)
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

func (g *Gateway) processLSPRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, serverName string, logger *mcp.StructuredLogger, startTime time.Time) {
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

func findProjectRoot(startPath string) string {
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

func fileExists(path string) bool {
	_, err := filepath.Abs(path)
	return err == nil
}

func isDirectory(path string) bool {
	return filepath.Ext(path) == ""
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
