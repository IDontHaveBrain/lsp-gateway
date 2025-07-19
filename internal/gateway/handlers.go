package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON-RPC error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// JSON-RPC protocol constants
const (
	JSONRPCVersion = "2.0"
)

// LSP method constants
const (
	LSPMethodHover           = "textDocument/hover"
	LSPMethodDefinition      = "textDocument/definition"
	LSPMethodReferences      = "textDocument/references"
	LSPMethodDocumentSymbol  = "textDocument/documentSymbol"
	LSPMethodWorkspaceSymbol = "workspace/symbol"
)

// LSP lifecycle method constants
const (
	LSPMethodInitialize              = "initialize"
	LSPMethodInitialized             = "initialized"
	LSPMethodShutdown                = "shutdown"
	LSPMethodExit                    = "exit"
	LSPMethodWorkspaceExecuteCommand = "workspace/executeCommand"
)

// Router handles request routing to appropriate LSP servers
type Router struct {
	langToServer map[string]string
	extToLang    map[string]string
	mu           sync.RWMutex
}

// NewRouter creates a new Router instance
func NewRouter() *Router {
	return &Router{
		langToServer: make(map[string]string),
		extToLang:    make(map[string]string),
	}
}

// RegisterServer registers a server for specific languages
func (r *Router) RegisterServer(serverName string, languages []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, lang := range languages {
		r.langToServer[lang] = serverName

		// Map common file extensions to languages
		extensions := getExtensionsForLanguage(lang)
		for _, ext := range extensions {
			r.extToLang[ext] = lang
		}
	}
}

// RouteRequest determines which server should handle a request based on file URI
func (r *Router) RouteRequest(uri string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Handle file:// URIs
	filePath := uri
	if strings.HasPrefix(uri, "file://") {
		filePath = strings.TrimPrefix(uri, "file://")
	}

	// Extract file extension from URI
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "", fmt.Errorf("cannot determine file type from URI: %s", uri)
	}

	// Remove leading dot from extension
	ext = strings.TrimPrefix(ext, ".")

	// Find language for extension
	lang, exists := r.extToLang[ext]
	if !exists {
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Find server for language
	server, exists := r.langToServer[lang]
	if !exists {
		return "", fmt.Errorf("no server configured for language: %s", lang)
	}

	return server, nil
}

// GetSupportedLanguages returns a list of all supported languages
func (r *Router) GetSupportedLanguages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var languages []string
	for lang := range r.langToServer {
		languages = append(languages, lang)
	}
	return languages
}

// GetSupportedExtensions returns a list of all supported file extensions
func (r *Router) GetSupportedExtensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var extensions []string
	for ext := range r.extToLang {
		extensions = append(extensions, ext)
	}
	return extensions
}

// GetServerByLanguage returns the server name for a given language
func (r *Router) GetServerByLanguage(language string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.langToServer[language]
	return server, exists
}

// GetLanguageByExtension returns the language for a given file extension
func (r *Router) GetLanguageByExtension(extension string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Remove leading dot if present
	extension = strings.TrimPrefix(strings.ToLower(extension), ".")
	lang, exists := r.extToLang[extension]
	return lang, exists
}

// getExtensionsForLanguage returns common file extensions for a language
func getExtensionsForLanguage(language string) []string {
	extensions := map[string][]string{
		// Go programming language
		"go": {"go", "mod", "sum", "work"},

		// Python programming language
		"python": {"py", "pyi", "pyx", "pyz", "pyw", "pyc", "pyo", "pyd"},

		// TypeScript programming language
		"typescript": {"ts", "tsx", "mts", "cts"},

		// JavaScript programming language
		"javascript": {"js", "jsx", "mjs", "cjs", "es", "es6", "es2015", "es2017", "es2018", "es2019", "es2020", "es2021", "es2022"},

		// Java programming language
		"java": {"java", "class", "jar", "war", "ear", "jsp", "jspx"},

		// C programming language
		"c": {"c", "h", "i"},

		// C++ programming language
		"cpp": {"cpp", "cxx", "cc", "c++", "hpp", "hxx", "h++", "hh", "ipp", "ixx", "txx", "tpp", "tcc"},

		// Rust programming language
		"rust": {"rs", "rlib"},

		// Ruby programming language
		"ruby": {"rb", "rbw", "rake", "gemspec", "podspec", "thor", "irb"},

		// PHP programming language
		"php": {"php", "php3", "php4", "php5", "php7", "php8", "phtml", "phar"},

		// Swift programming language
		"swift": {"swift", "swiftmodule", "swiftdoc", "swiftsourceinfo"},

		// Kotlin programming language
		"kotlin": {"kt", "kts", "ktm"},

		// Scala programming language
		"scala": {"scala", "sc", "sbt"},

		// C# programming language
		"csharp": {"cs", "csx", "csproj", "sln", "vb", "vbproj"},

		// F# programming language
		"fsharp": {"fs", "fsi", "fsx", "fsscript", "fsproj"},

		// Additional languages
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

// Gateway manages LSP server connections and request routing
type Gateway struct {
	config  *config.GatewayConfig
	clients map[string]transport.LSPClient
	router  *Router
	mu      sync.RWMutex
}

// NewGateway creates a new Gateway instance
func NewGateway(config *config.GatewayConfig) (*Gateway, error) {
	gateway := &Gateway{
		config:  config,
		clients: make(map[string]transport.LSPClient),
		router:  NewRouter(),
	}

	// Initialize LSP clients
	for _, serverConfig := range config.Servers {
		client, err := transport.NewLSPClient(transport.ClientConfig{
			Command:   serverConfig.Command,
			Args:      serverConfig.Args,
			Transport: serverConfig.Transport,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create client for %s: %w", serverConfig.Name, err)
		}

		gateway.clients[serverConfig.Name] = client
		gateway.router.RegisterServer(serverConfig.Name, serverConfig.Languages)
	}

	return gateway, nil
}

// Start initializes all LSP server connections
func (g *Gateway) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for name, client := range g.clients {
		if err := client.Start(ctx); err != nil {
			return fmt.Errorf("failed to start client %s: %w", name, err)
		}
	}

	return nil
}

// Stop shuts down all LSP server connections
func (g *Gateway) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for name, client := range g.clients {
		if err := client.Stop(); err != nil {
			return fmt.Errorf("failed to stop client %s: %w", name, err)
		}
	}

	return nil
}

// GetClient returns the LSP client for a given server name
func (g *Gateway) GetClient(serverName string) (transport.LSPClient, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	client, exists := g.clients[serverName]
	return client, exists
}

// HandleJSONRPC handles JSON-RPC requests over HTTP
func (g *Gateway) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Parse JSON-RPC request
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeError(w, nil, ParseError, "Parse error", err)
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != JSONRPCVersion {
		g.writeError(w, req.ID, InvalidRequest, "Invalid request",
			fmt.Errorf("invalid JSON-RPC version: %s", req.JSONRPC))
		return
	}

	// Validate method
	if req.Method == "" {
		g.writeError(w, req.ID, InvalidRequest, "Invalid request",
			fmt.Errorf("missing method field"))
		return
	}

	// Route request to appropriate LSP server
	serverName, err := g.routeRequest(req)
	if err != nil {
		g.writeError(w, req.ID, MethodNotFound, "Method not found", err)
		return
	}

	// Get LSP client
	client, exists := g.GetClient(serverName)
	if !exists {
		g.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("server not found: %s", serverName))
		return
	}

	// Check if client is active
	if !client.IsActive() {
		g.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("server %s is not active", serverName))
		return
	}

	// Handle notifications (requests without ID)
	if req.ID == nil {
		err := client.SendNotification(r.Context(), req.Method, req.Params)
		if err != nil {
			g.writeError(w, req.ID, InternalError, "Internal error", err)
			return
		}

		// Notifications don't return responses
		w.WriteHeader(http.StatusOK)
		return
	}

	// Forward request to LSP server
	result, err := client.SendRequest(r.Context(), req.Method, req.Params)
	if err != nil {
		g.writeError(w, req.ID, InternalError, "Internal error", err)
		return
	}

	// Send successful response
	response := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		g.writeError(w, req.ID, InternalError, "Internal error",
			fmt.Errorf("failed to encode response: %w", err))
		return
	}
}

// routeRequest determines which server should handle the request
func (g *Gateway) routeRequest(req JSONRPCRequest) (string, error) {
	// Extract URI from request parameters
	uri, err := g.extractURI(req)
	if err != nil {
		return "", err
	}

	// For special methods, extractURI returns server name directly
	// For file-based methods, extractURI returns URI, so we need to route it
	switch req.Method {
	case LSPMethodInitialize, LSPMethodInitialized, LSPMethodShutdown, LSPMethodExit, LSPMethodWorkspaceSymbol, LSPMethodWorkspaceExecuteCommand:
		// extractURI already returned the server name for these methods
		return uri, nil
	default:
		// For other methods, route the URI through the router
		return g.router.RouteRequest(uri)
	}
}

// extractURI extracts the file URI from request parameters based on LSP method
func (g *Gateway) extractURI(req JSONRPCRequest) (string, error) {
	// Handle special methods that don't require URI
	switch req.Method {
	case LSPMethodInitialize, LSPMethodInitialized, LSPMethodShutdown, LSPMethodExit:
		// These methods don't require specific file routing
		// Route to the first available server
		g.mu.RLock()
		defer g.mu.RUnlock()

		for serverName := range g.clients {
			return serverName, nil
		}
		return "", fmt.Errorf("no servers available")

	case "workspace/symbol", "workspace/executeCommand":
		// Workspace methods - route to first available server
		g.mu.RLock()
		defer g.mu.RUnlock()

		for serverName := range g.clients {
			return serverName, nil
		}
		return "", fmt.Errorf("no servers available")
	}

	// Extract URI from parameters based on method type
	if req.Params == nil {
		return "", fmt.Errorf("missing parameters for method %s", req.Method)
	}

	// Convert params to map for easier access
	paramsMap, ok := req.Params.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid parameters format for method %s", req.Method)
	}

	// Try to extract URI from textDocument parameter
	if textDoc, exists := paramsMap["textDocument"]; exists {
		if textDocMap, ok := textDoc.(map[string]interface{}); ok {
			if uri, exists := textDocMap["uri"]; exists {
				if uriStr, ok := uri.(string); ok {
					return uriStr, nil
				}
			}
		}
	}

	// Try to extract URI from uri parameter (for some methods)
	if uri, exists := paramsMap["uri"]; exists {
		if uriStr, ok := uri.(string); ok {
			return uriStr, nil
		}
	}

	// Try to extract URI from textDocument.uri in different structures
	if params, exists := paramsMap["textDocument"]; exists {
		if doc, ok := params.(map[string]interface{}); ok {
			if uri, exists := doc["uri"]; exists {
				if uriStr, ok := uri.(string); ok {
					return uriStr, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not extract URI from parameters for method %s", req.Method)
}

// writeError writes a JSON-RPC error response
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

	// Always return 200 OK for JSON-RPC (errors are in the response body)
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// If we can't encode the error response, log it and return a simple HTTP error
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
