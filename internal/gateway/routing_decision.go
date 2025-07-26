package gateway

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)


// Core routing decision types

// RoutingDecision represents a complete routing decision with all required context
type RoutingDecision struct {
	TargetServers      []*RoutingServerInstance `json:"target_servers"`
	RoutingStrategy    string                   `json:"routing_strategy"`
	RequestContext     *RequestContext          `json:"request_context"`
	ResponseAggregator ResponseAggregator       `json:"-"` // Not serializable due to interface
	Timeout            time.Duration            `json:"timeout"`
	Priority           int                      `json:"priority"`
	CreatedAt          time.Time                `json:"created_at"`
	DecisionID         string                   `json:"decision_id"`

	// Additional fields for enhanced routing
	ServerName   string                 `json:"server_name,omitempty"`
	ServerConfig *config.ServerConfig   `json:"server_config,omitempty"`
	Client       transport.LSPClient    `json:"-"` // Not serializable due to interface
	Weight       float64                `json:"weight,omitempty"`
	Strategy     RoutingStrategy        `json:"-"` // Not serializable due to interface
	Metadata     map[string]interface{} `json:"metadata,omitempty"`

	// Fields referenced in routing_strategies.go
	PrimaryServer   *RoutingServerInstance   `json:"primary_server,omitempty"`
	FallbackServers []*RoutingServerInstance `json:"fallback_servers,omitempty"`
	RequestID       interface{}              `json:"request_id,omitempty"`
	Method          string                   `json:"method,omitempty"`
	Language        string                   `json:"language,omitempty"`
}

// RoutingRequestContext contains basic context information for routing decisions
type RoutingRequestContext struct {
	FileURI              string            `json:"file_uri"`
	Language             string            `json:"language"`
	RequestType          string            `json:"request_type"`
	WorkspaceRoot        string            `json:"workspace_root"`
	ProjectType          string            `json:"project_type"`
	CrossLanguageContext bool              `json:"cross_language_context"`
	RequiresAggregation  bool              `json:"requires_aggregation"`
	SupportedServers     []string          `json:"supported_servers"`
	AdditionalContext    map[string]string `json:"additional_context,omitempty"`
	WorkspaceID          string            `json:"workspace_id,omitempty"`
}

// LSPRequest represents an enhanced LSP request with routing context
type LSPRequest struct {
	Method    string          `json:"method"`
	Params    interface{}     `json:"params,omitempty"`
	ID        interface{}     `json:"id,omitempty"`
	URI       string          `json:"uri,omitempty"`
	Language  string          `json:"language,omitempty"`
	Context   *RequestContext `json:"context,omitempty"`
	JSONRPC   string          `json:"jsonrpc"`
	Timestamp time.Time       `json:"timestamp"`
	RequestID string          `json:"request_id"`
}

// AggregatedResponse contains the result of aggregating multiple LSP server responses
type AggregatedResponse struct {
	PrimaryResponse    interface{}         `json:"primary_response"`
	SecondaryResponses []interface{}       `json:"secondary_responses,omitempty"`
	AggregatedResult   interface{}         `json:"aggregated_result"`
	ResponseSources    []string            `json:"response_sources"`
	ProcessingTime     time.Duration       `json:"processing_time"`
	AggregationMethod  string              `json:"aggregation_method"`
	SuccessCount       int                 `json:"success_count"`
	ErrorCount         int                 `json:"error_count"`
	Warnings           []string            `json:"warnings,omitempty"`
	PrimaryResult      interface{}         `json:"primary_result"`
	Metadata           interface{}         `json:"metadata,omitempty"`
	ServerCount        int                 `json:"server_count"`
	Strategy           RoutingStrategyType `json:"strategy"`
}

// RoutingServerInstance represents a language server instance for routing decisions
type RoutingServerInstance struct {
	Name        string               `json:"name"`
	Language    string               `json:"language"`
	Performance *ServerPerformance   `json:"performance,omitempty"`
	Available   bool                 `json:"available"`
	LoadScore   float64              `json:"load_score"`
	Config      *config.ServerConfig `json:"config,omitempty"`
	Client      transport.LSPClient  `json:"-"` // Not serializable
	LastUsed    time.Time            `json:"last_used"`
	Priority    int                  `json:"priority"`
	Weight      float64              `json:"weight"`
}

// ServerPerformance tracks performance metrics for server instances
type ServerPerformance struct {
	AverageResponseTime time.Duration `json:"average_response_time"`
	RequestCount        int64         `json:"request_count"`
	ErrorCount          int64         `json:"error_count"`
	SuccessRate         float64       `json:"success_rate"`
	LastErrorTime       time.Time     `json:"last_error_time,omitempty"`
	CircuitBreakerOpen  bool          `json:"circuit_breaker_open"`
	MemoryUsage         int64         `json:"memory_usage,omitempty"`
	CPUUsage            float64       `json:"cpu_usage,omitempty"`
}

// Global strategy instances
var (
	PrimaryWithEnhancementStrategyInstance = &PrimaryWithEnhancementStrategy{}
)

// Response aggregation interfaces and implementations

// ServerResponse represents a response from a specific server
type ServerResponse struct {
	ServerName   string        `json:"server_name"`
	Response     interface{}   `json:"response"`
	Result       interface{}   `json:"result"`
	Error        error         `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
	ResponseTime time.Duration `json:"response_time"`
	Success      bool          `json:"success"`
}

// SymbolsAggregator aggregates document and workspace symbol responses
type SymbolsAggregator struct{}

func (sa *SymbolsAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	var allSymbols []interface{}
	successCount := 0
	errorCount := 0
	var primary interface{}

	for _, resp := range responses {
		if resp != nil {
			successCount++
			
			if primary == nil {
				primary = resp
			}
			
			allSymbols = append(allSymbols, resp)
		} else {
			errorCount++
		}
	}

	// Merge symbols and organize by relevance
	aggregated := mergeSymbols(allSymbols)

	return &AggregatedResponse{
		PrimaryResponse:    primary,
		SecondaryResponses: allSymbols[1:],
		AggregatedResult:   aggregated,
		ResponseSources:    sources,
		AggregationMethod:  "symbols_merge",
		SuccessCount:       successCount,
		ErrorCount:         errorCount,
	}, nil
}

func (sa *SymbolsAggregator) GetAggregationType() string {
	return "symbols"
}

func (sa *SymbolsAggregator) SupportedMethods() []string {
	return []string{"textDocument/documentSymbol", "workspace/symbol"}
}

// FirstSuccessAggregator returns the first successful response
type FirstSuccessAggregator struct{}

func (fsa *FirstSuccessAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	for i, resp := range responses {
		if resp != nil {
			sourceName := ""
			if i < len(sources) {
				sourceName = sources[i]
			}
			return &AggregatedResponse{
				PrimaryResponse:   resp,
				AggregatedResult:  resp,
				ResponseSources:   []string{sourceName},
				AggregationMethod: "first_success",
				SuccessCount:      1,
				ErrorCount:        len(responses) - 1,
			}, nil
		}
	}

	return &AggregatedResponse{
		AggregationMethod: "first_success",
		SuccessCount:      0,
		ErrorCount:        len(responses),
	}, fmt.Errorf("no successful responses")
}

func (fsa *FirstSuccessAggregator) GetAggregationType() string {
	return "first_success"
}

func (fsa *FirstSuccessAggregator) SupportedMethods() []string {
	return []string{"*"} // Supports all methods as fallback
}

// Routing strategy interface and implementations

// RoutingStrategy defines how requests should be routed to language servers
type RoutingStrategy interface {
	Route(request *LSPRequest, availableServers []*ServerInstance) (*RoutingDecision, error)
	Name() string
	Description() string
}

// SingleServerStrategy routes to a single best server
type SingleServerStrategy struct{}

func (sss *SingleServerStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) (*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no available servers for request")
	}

	// Find the best server based on language match and performance
	bestServer := selectBestServer(availableServers, request.Context)
	if bestServer == nil {
		return nil, fmt.Errorf("no suitable server found for request")
	}

	aggregator := getAggregatorForMethod(request.Method)

	// Convert ServerInstance to RoutingServerInstance for routing decision
	routingServer := convertToRoutingServerInstance(bestServer)

	return &RoutingDecision{
		TargetServers:      []*RoutingServerInstance{routingServer},
		RoutingStrategy:    "single_server",
		RequestContext:     request.Context,
		ResponseAggregator: aggregator,
		Timeout:            30 * time.Second,
		Priority:           1,
		CreatedAt:          time.Now(),
		DecisionID:         generateDecisionID(),
	}, nil
}

func (sss *SingleServerStrategy) Name() string {
	return "single_server"
}

func (sss *SingleServerStrategy) Description() string {
	return "Routes requests to the single best matching server"
}

// MultiServerStrategy routes to multiple servers and aggregates responses
type MultiServerStrategy struct{}

func (mss *MultiServerStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) (*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no available servers for request")
	}

	// Select multiple suitable servers
	suitableServers := selectSuitableServers(availableServers, request.Context)
	if len(suitableServers) == 0 {
		return nil, fmt.Errorf("no suitable servers found for request")
	}

	aggregator := getAggregatorForMethod(request.Method)

	// Convert ServerInstances to RoutingServerInstances
	routingServers := make([]*RoutingServerInstance, len(suitableServers))
	for i, server := range suitableServers {
		routingServers[i] = convertToRoutingServerInstance(server)
	}

	return &RoutingDecision{
		TargetServers:      routingServers,
		RoutingStrategy:    "multi_server",
		RequestContext:     request.Context,
		ResponseAggregator: aggregator,
		Timeout:            45 * time.Second, // Longer timeout for multiple servers
		Priority:           2,
		CreatedAt:          time.Now(),
		DecisionID:         generateDecisionID(),
	}, nil
}

func (mss *MultiServerStrategy) Name() string {
	return "multi_server"
}

func (mss *MultiServerStrategy) Description() string {
	return "Routes requests to multiple servers and aggregates responses"
}

// PrimaryWithEnhancementStrategy routes to a primary server with optional enhancement from secondary servers
type PrimaryWithEnhancementStrategy struct{}

func (pwes *PrimaryWithEnhancementStrategy) Route(request *LSPRequest, availableServers []*ServerInstance) (*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no available servers for request")
	}

	// Find the best primary server
	primaryServer := selectBestServer(availableServers, request.Context)
	if primaryServer == nil {
		return nil, fmt.Errorf("no suitable primary server found for request")
	}

	aggregator := getAggregatorForMethod(request.Method)

	// Convert ServerInstance to RoutingServerInstance
	routingServer := convertToRoutingServerInstance(primaryServer)

	return &RoutingDecision{
		TargetServers:      []*RoutingServerInstance{routingServer},
		RoutingStrategy:    "primary_with_enhancement",
		RequestContext:     request.Context,
		ResponseAggregator: aggregator,
		Timeout:            35 * time.Second, // Slightly longer timeout for potential enhancement
		Priority:           1,
		CreatedAt:          time.Now(),
		DecisionID:         generateDecisionID(),
	}, nil
}

func (pwes *PrimaryWithEnhancementStrategy) Name() string {
	return "primary_with_enhancement"
}

func (pwes *PrimaryWithEnhancementStrategy) Description() string {
	return "Routes to primary server with optional enhancement from secondary servers"
}

// Helper functions for context creation and server selection

// CreateRequestContextFromURI creates a RoutingRequestContext from a file URI and workspace information
func CreateRequestContextFromURI(uri string, workspaceRoot string, projectType string) *RoutingRequestContext {
	language := detectLanguageFromURI(uri)

	return &RoutingRequestContext{
		FileURI:              uri,
		Language:             language,
		RequestType:          "file_based",
		WorkspaceRoot:        workspaceRoot,
		ProjectType:          projectType,
		CrossLanguageContext: false,
		RequiresAggregation:  false,
		SupportedServers:     getSupportedServersForLanguage(language),
		AdditionalContext:    make(map[string]string),
	}
}

// CreateWorkspaceRequestContext creates a RoutingRequestContext for workspace-wide operations
func CreateWorkspaceRequestContext(workspaceRoot string, projectType string, languages []string) *RoutingRequestContext {
	return &RoutingRequestContext{
		FileURI:              "",
		Language:             determineMainLanguage(languages),
		RequestType:          "workspace",
		WorkspaceRoot:        workspaceRoot,
		ProjectType:          projectType,
		CrossLanguageContext: len(languages) > 1,
		RequiresAggregation:  len(languages) > 1,
		SupportedServers:     getSupportedServersForLanguages(languages),
		AdditionalContext:    map[string]string{"languages": strings.Join(languages, ",")},
	}
}

// ValidateLSPRequest validates an LSP request before routing
func ValidateLSPRequest(request *LSPRequest) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if request.Method == "" {
		return fmt.Errorf("method cannot be empty")
	}

	if request.JSONRPC != "2.0" {
		return fmt.Errorf("invalid JSON-RPC version: %s", request.JSONRPC)
	}

	// Validate context for certain methods
	if requiresURI(request.Method) && request.Context != nil && request.Context.FileURI == "" {
		return fmt.Errorf("method %s requires a file URI", request.Method)
	}

	return nil
}

// PreprocessLSPRequest enhances a request with additional context
func PreprocessLSPRequest(request *LSPRequest) error {
	if request.Timestamp.IsZero() {
		request.Timestamp = time.Now()
	}

	if request.RequestID == "" {
		request.RequestID = generateRequestID()
	}

	if request.JSONRPC == "" {
		request.JSONRPC = "2.0"
	}

	// Extract URI from params if not set in context
	if request.URI == "" {
		request.URI = extractURIFromParams(request.Params)
	}

	// Create or enhance context
	if request.Context == nil && request.URI != "" {
		routingCtx := CreateRequestContextFromURI(request.URI, "", "")
		request.Context = convertRoutingToRequestContext(routingCtx)
	}

	return nil
}

// Server selection utilities

func selectBestServer(servers []*ServerInstance, context *RequestContext) *ServerInstance {
	var bestServer *ServerInstance
	bestScore := -1.0

	for _, server := range servers {
		if !server.IsHealthy() {
			continue
		}

		score := calculateServerScore(server, context)
		if score > bestScore {
			bestScore = score
			bestServer = server
		}
	}

	return bestServer
}

func selectSuitableServers(servers []*ServerInstance, context *RequestContext) []*ServerInstance {
	var suitable []*ServerInstance

	for _, server := range servers {
		if !server.IsHealthy() {
			continue
		}

		if isServerSuitableForContext(server, context) {
			suitable = append(suitable, server)
		}
	}

	return suitable
}

func selectLeastLoadedServer(servers []*ServerInstance, context *RequestContext) *ServerInstance {
	var bestServer *ServerInstance
	lowestLoad := float64(1000000) // High initial value

	for _, server := range servers {
		if !server.IsHealthy() {
			continue
		}

		if !isServerSuitableForContext(server, context) {
			continue
		}

		// Calculate load score based on metrics
		metrics := server.GetMetrics()
		loadScore := float64(metrics.ActiveConnections) + metrics.ErrorRate
		
		if loadScore < lowestLoad {
			lowestLoad = loadScore
			bestServer = server
		}
	}

	return bestServer
}

func calculateServerScore(server *ServerInstance, context *RequestContext) float64 {
	score := 0.0

	// Language match (most important)
	if supportsLanguage(server, context.Language) {
		score += 100.0
	}

	// Performance metrics
	metrics := server.GetMetrics()
	if metrics != nil {
		successRate := float64(metrics.SuccessCount) / float64(metrics.RequestCount)
		if metrics.RequestCount == 0 {
			successRate = 1.0 // Default for new servers
		}
		score += successRate * 20.0
		
		// Lower response time is better
		if metrics.RequestCount > 0 {
			avgResponseTime := float64(metrics.TotalResponseTime.Milliseconds()) / float64(metrics.RequestCount)
			if avgResponseTime > 0 {
				responseTimeScore := 1000.0 / avgResponseTime
				score += responseTimeScore
			}
		}

		// Error rate penalty
		score -= metrics.ErrorRate * 30.0

		// Load score (lower is better) - calculate from metrics
		loadScore := float64(metrics.ActiveConnections) + metrics.ErrorRate
		score -= loadScore
	}

	// Priority and weight from config
	score += float64(server.GetConfig().Priority) * 10.0

	return score
}

func isServerSuitableForContext(server *ServerInstance, context *RequestContext) bool {
	// Check language support
	if supportsLanguage(server, context.Language) {
		return true
	}

	// Check if server is in the supported servers list
	for _, supportedServer := range context.SupportedServers {
		if server.GetConfig().Name == supportedServer {
			return true
		}
	}

	return false
}

func supportsLanguage(server *ServerInstance, language string) bool {
	config := server.GetConfig()
	if config == nil {
		return false
	}

	for _, lang := range config.Languages {
		if lang == language {
			return true
		}
	}

	return false
}

// Response aggregation utilities

// TODO: Update to use response_aggregator.go registry system
func getAggregatorForMethod(method string) ResponseAggregator {
	switch method {
	case LSP_METHOD_DOCUMENT_SYMBOL, LSP_METHOD_WORKSPACE_SYMBOL:
		return &SymbolsAggregator{}
	default:
		return &FirstSuccessAggregator{}
	}
}





// Utility functions

func detectLanguageFromURI(uri string) string {
	if uri == "" {
		return ""
	}

	// Remove file:// prefix if present
	cleanURI := strings.TrimPrefix(uri, "file://")

	// Handle URL decoding
	if parsedURL, err := url.Parse(uri); err == nil {
		cleanURI = parsedURL.Path
	}

	ext := strings.ToLower(filepath.Ext(cleanURI))

	switch ext {
	case ".go":
		return "go"
	case ".py":
		return LANG_PYTHON
	case ".js", ".mjs":
		return LANG_JAVASCRIPT
	case ".ts":
		return LANG_TYPESCRIPT
	case ".java":
		return LANG_JAVA
	case ".rs":
		return LANG_RUST
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".cs":
		return "csharp"
	default:
		return StateStringUnknown
	}
}

func getSupportedServersForLanguage(language string) []string {
	switch language {
	case "go":
		return []string{"gopls"}
	case LANG_PYTHON:
		return []string{"pylsp", "pyright"}
	case LANG_JAVASCRIPT, LANG_TYPESCRIPT:
		return []string{"typescript-language-server", "tsserver"}
	case LANG_JAVA:
		return []string{"jdtls"}
	case "rust":
		return []string{"rust-analyzer"}
	default:
		return []string{}
	}
}

func getSupportedServersForLanguages(languages []string) []string {
	serverSet := make(map[string]bool)

	for _, lang := range languages {
		servers := getSupportedServersForLanguage(lang)
		for _, server := range servers {
			serverSet[server] = true
		}
	}

	result := make([]string, 0, len(serverSet))
	for server := range serverSet {
		result = append(result, server)
	}

	return result
}

func determineMainLanguage(languages []string) string {
	if len(languages) == 0 {
		return ""
	}

	// Return first language as main - could be enhanced with priority logic
	return languages[0]
}

func requiresURI(method string) bool {
	return strings.HasPrefix(method, "textDocument/")
}

func extractURIFromParams(params interface{}) string {
	if params == nil {
		return ""
	}

	// Convert to map for easier access
	if paramMap, ok := params.(map[string]interface{}); ok {
		// Check textDocument.uri
		if textDoc, exists := paramMap["textDocument"]; exists {
			if textDocMap, ok := textDoc.(map[string]interface{}); ok {
				if uri, exists := textDocMap["uri"]; exists {
					if uriStr, ok := uri.(string); ok {
						return uriStr
					}
				}
			}
		}

		// Check direct uri field
		if uri, exists := paramMap["uri"]; exists {
			if uriStr, ok := uri.(string); ok {
				return uriStr
			}
		}
	}

	return ""
}

func generateDecisionID() string {
	return fmt.Sprintf("decision_%d", time.Now().UnixNano())
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// Enhanced routing decision methods

// IsValid checks if a routing decision is valid and ready for execution
func (rd *RoutingDecision) IsValid() bool {
	return len(rd.TargetServers) > 0 &&
		rd.RequestContext != nil &&
		rd.ResponseAggregator != nil &&
		rd.Timeout > 0
}

// GetPrimaryServer returns the highest priority server from the target servers
func (rd *RoutingDecision) GetPrimaryServer() *RoutingServerInstance {
	if len(rd.TargetServers) == 0 {
		return nil
	}

	primary := rd.TargetServers[0]
	for _, server := range rd.TargetServers[1:] {
		if server.Priority > primary.Priority {
			primary = server
		}
	}

	return primary
}

// GetServerNames returns the names of all target servers
func (rd *RoutingDecision) GetServerNames() []string {
	names := make([]string, len(rd.TargetServers))
	for i, server := range rd.TargetServers {
		names[i] = server.Name
	}
	return names
}

// SupportsAggregation returns true if the decision supports response aggregation
func (rd *RoutingDecision) SupportsAggregation() bool {
	return len(rd.TargetServers) > 1 && rd.ResponseAggregator != nil
}

// Enhanced request context methods

// IsFileBasedRequest returns true if the request is for a specific file
func (rc *RoutingRequestContext) IsFileBasedRequest() bool {
	return rc.FileURI != "" && rc.RequestType == "file_based"
}

// IsWorkspaceRequest returns true if the request is workspace-wide
func (rc *RoutingRequestContext) IsWorkspaceRequest() bool {
	return rc.RequestType == "workspace"
}

// GetFileExtension returns the file extension from the URI
func (rc *RoutingRequestContext) GetFileExtension() string {
	if rc.FileURI == "" {
		return ""
	}
	return filepath.Ext(rc.FileURI)
}

// Helper functions for type conversion

// convertToRoutingServerInstance converts a ServerInstance to RoutingServerInstance
func convertToRoutingServerInstance(server *ServerInstance) *RoutingServerInstance {
	config := server.GetConfig()
	language := ""
	if len(config.Languages) > 0 {
		language = config.Languages[0] // Use first language as primary
	}

	return &RoutingServerInstance{
		Name:        config.Name,
		Language:    language,
		Available:   server.IsHealthy(),
		LoadScore:   0.0, // Could be calculated from metrics if needed
		Config:      config,
		Client:      server.GetClient(),
		LastUsed:    time.Now(),
		Priority:    config.Priority,
		Weight:      1.0, // Default weight
		Performance: convertToServerPerformance(server.GetMetrics()),
	}
}

// convertToServerPerformance converts metrics to ServerPerformance
func convertToServerPerformance(metrics *SimpleServerMetrics) *ServerPerformance {
	if metrics == nil {
		return nil
	}

	var avgResponseTime time.Duration
	if metrics.RequestCount > 0 {
		avgResponseTime = time.Duration(metrics.TotalResponseTime.Nanoseconds() / metrics.RequestCount)
	}

	successRate := float64(1.0)
	if metrics.RequestCount > 0 {
		successRate = float64(metrics.SuccessCount) / float64(metrics.RequestCount)
	}

	return &ServerPerformance{
		AverageResponseTime: avgResponseTime,
		RequestCount:        metrics.RequestCount,
		ErrorCount:          metrics.FailureCount,
		SuccessRate:         successRate,
		CircuitBreakerOpen:  false, // Would need circuit breaker state
	}
}

// convertRoutingToRequestContext converts RoutingRequestContext to RequestContext
func convertRoutingToRequestContext(routingCtx *RoutingRequestContext) *RequestContext {
	if routingCtx == nil {
		return nil
	}

	return &RequestContext{
		FileURI:              routingCtx.FileURI,
		Language:             routingCtx.Language,
		RequestType:          routingCtx.RequestType,
		WorkspaceRoot:        routingCtx.WorkspaceRoot,
		ProjectType:          routingCtx.ProjectType,
		CrossLanguageContext: routingCtx.CrossLanguageContext,
		RequiresAggregation:  routingCtx.RequiresAggregation,
		SupportedServers:     routingCtx.SupportedServers,
		AdditionalContext:    convertStringMapToInterface(routingCtx.AdditionalContext),
		WorkspaceID:          routingCtx.WorkspaceID,
	}
}

// mergeSymbols merges symbol responses from multiple servers
func mergeSymbols(symbols []interface{}) interface{} {
	if len(symbols) == 0 {
		return []interface{}{}
	}

	// For now, return all symbols combined
	// In a more sophisticated implementation, this could deduplicate and organize
	var merged []interface{}
	for _, symbolSet := range symbols {
		if symbolList, ok := symbolSet.([]interface{}); ok {
			merged = append(merged, symbolList...)
		} else {
			merged = append(merged, symbolSet)
		}
	}

	return merged
}

// Enhanced server instance methods

// UpdatePerformanceMetrics updates the performance metrics for the server
func (si *RoutingServerInstance) UpdatePerformanceMetrics(responseTime time.Duration, success bool) {
	if si.Performance == nil {
		si.Performance = &ServerPerformance{}
	}

	si.Performance.RequestCount++

	if success {
		// Update average response time (simple moving average approximation)
		if si.Performance.RequestCount == 1 {
			si.Performance.AverageResponseTime = responseTime
		} else {
			// Weighted average with more weight on recent responses
			weight := 0.1
			si.Performance.AverageResponseTime = time.Duration(
				float64(si.Performance.AverageResponseTime)*(1-weight) +
					float64(responseTime)*weight,
			)
		}

		si.Performance.SuccessRate = float64(si.Performance.RequestCount-si.Performance.ErrorCount) / float64(si.Performance.RequestCount)
	} else {
		si.Performance.ErrorCount++
		si.Performance.LastErrorTime = time.Now()
		si.Performance.SuccessRate = float64(si.Performance.RequestCount-si.Performance.ErrorCount) / float64(si.Performance.RequestCount)
	}

	si.LastUsed = time.Now()
}

// IsHealthy returns true if the server is considered healthy
func (si *RoutingServerInstance) IsHealthy() bool {
	if !si.Available {
		return false
	}

	if si.Performance == nil {
		return true // No performance data yet, assume healthy
	}

	return !si.Performance.CircuitBreakerOpen &&
		si.Performance.SuccessRate >= 0.8 &&
		time.Since(si.Performance.LastErrorTime) > time.Minute*5
}

// convertStringMapToInterface converts map[string]string to map[string]interface{}
func convertStringMapToInterface(stringMap map[string]string) map[string]interface{} {
	if stringMap == nil {
		return nil
	}
	result := make(map[string]interface{})
	for k, v := range stringMap {
		result[k] = v
	}
	return result
}
