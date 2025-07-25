package gateway

import (
	"context"
	"encoding/json"
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
	TargetServers      []*RoutingServerInstance    `json:"target_servers"`
	RoutingStrategy    string               `json:"routing_strategy"`
	RequestContext     *RequestContext      `json:"request_context"`
	ResponseAggregator ResponseAggregator   `json:"-"` // Not serializable due to interface
	Timeout            time.Duration        `json:"timeout"`
	Priority           int                  `json:"priority"`
	CreatedAt          time.Time            `json:"created_at"`
	DecisionID         string               `json:"decision_id"`
	Client             transport.LSPClient  `json:"-"` // LSP client for communication
	ServerName         string               `json:"server_name"`
	ServerConfig       interface{}          `json:"server_config,omitempty"`
	Weight             float64              `json:"weight,omitempty"`
	Strategy           RoutingStrategy      `json:"-"` // Strategy interface
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// RequestContext contains comprehensive context information about the LSP request
type RequestContext struct {
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
	Method     string          `json:"method"`
	Params     interface{}     `json:"params,omitempty"`
	ID         interface{}     `json:"id,omitempty"`
	URI        string          `json:"uri,omitempty"`
	Language   string          `json:"language,omitempty"`
	Context    *RequestContext `json:"context,omitempty"`
	JSONRPC    string          `json:"jsonrpc"`
	Timestamp  time.Time       `json:"timestamp"`
	RequestID  string          `json:"request_id"`
}

// AggregatedResponse contains the result of aggregating multiple LSP server responses
type AggregatedResponse struct {
	PrimaryResponse    interface{}            `json:"primary_response"`
	PrimaryResult      interface{}            `json:"primary_result"`
	SecondaryResponses []interface{}          `json:"secondary_responses,omitempty"`
	SecondaryResults   []ServerResponse       `json:"secondary_results,omitempty"`
	AggregatedResult   interface{}            `json:"aggregated_result"`
	ResponseSources    []string               `json:"response_sources"`
	ProcessingTime     time.Duration          `json:"processing_time"`
	AggregationMethod  string                 `json:"aggregation_method"`
	SuccessCount       int                    `json:"success_count"`
	ErrorCount         int                    `json:"error_count"`
	Warnings           []string               `json:"warnings,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	ServerCount        int                    `json:"server_count"`
	Strategy           RoutingStrategyType    `json:"strategy"`
}

// RoutingServerInstance represents a language server instance for routing decisions
type RoutingServerInstance struct {
	Name        string             `json:"name"`
	Language    string             `json:"language"`
	Performance *ServerPerformance `json:"performance,omitempty"`
	Available   bool               `json:"available"`
	LoadScore   float64            `json:"load_score"`
	Config      *config.ServerConfig `json:"config,omitempty"`
	Client      transport.LSPClient `json:"-"` // Not serializable
	LastUsed    time.Time          `json:"last_used"`
	Priority    int                `json:"priority"`
	Weight      float64            `json:"weight"`
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


// Routing strategy interface and implementations

// RoutingStrategy defines how requests should be routed to language servers
type RoutingStrategy interface {
	Route(request *LSPRequest, availableServers []*RoutingServerInstance) (*RoutingDecision, error)
	Name() string
	Description() string
}

// SingleServerStrategy routes to a single best server
type SingleServerStrategy struct{}

func (sss *SingleServerStrategy) Route(request *LSPRequest, availableServers []*RoutingServerInstance) (*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no available servers for request")
	}

	// Find the best server based on language match and performance
	bestServer := selectBestServer(availableServers, request.Context)
	if bestServer == nil {
		return nil, fmt.Errorf("no suitable server found for request")
	}

	aggregator := getAggregatorForMethod(request.Method)
	
	return &RoutingDecision{
		TargetServers:      []*RoutingServerInstance{bestServer},
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

func (mss *MultiServerStrategy) Route(request *LSPRequest, availableServers []*RoutingServerInstance) (*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no available servers for request")
	}

	// Select multiple suitable servers
	suitableServers := selectSuitableServers(availableServers, request.Context)
	if len(suitableServers) == 0 {
		return nil, fmt.Errorf("no suitable servers found for request")
	}

	aggregator := getAggregatorForMethod(request.Method)
	
	return &RoutingDecision{
		TargetServers:      suitableServers,
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

// SingleTargetWithFallbackStrategy routes to primary server with fallback options
type SingleTargetWithFallbackStrategy struct{}

func (stfs *SingleTargetWithFallbackStrategy) Route(request *LSPRequest, availableServers []*RoutingServerInstance) (*RoutingDecision, error) {
	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no available servers for single target with fallback routing")
	}

	// Sort servers by priority and health status
	sortedServers := make([]*RoutingServerInstance, len(availableServers))
	copy(sortedServers, availableServers)
	
	// Simple priority-based sorting (higher priority first)
	for i := 0; i < len(sortedServers); i++ {
		for j := i + 1; j < len(sortedServers); j++ {
			if sortedServers[i].Priority < sortedServers[j].Priority {
				sortedServers[i], sortedServers[j] = sortedServers[j], sortedServers[i]
			}
		}
	}

	// Try servers in priority order for fallback capability
	for _, server := range sortedServers {
		if server.Available {
			return &RoutingDecision{
				TargetServers:      []*RoutingServerInstance{server},
				RoutingStrategy:    "single_target_with_fallback",
				RequestContext:     request.Context,
				ResponseAggregator: nil, // Single target doesn't need aggregation
				Timeout:            30 * time.Second,
				Priority:           server.Priority,
				CreatedAt:          time.Now(),
				DecisionID:         generateDecisionID(),
			}, nil
		}
	}

	// If no available servers found, return error
	return nil, fmt.Errorf("no available servers found for single target with fallback routing")
}

func (stfs *SingleTargetWithFallbackStrategy) Name() string {
	return "single_target_with_fallback"
}

func (stfs *SingleTargetWithFallbackStrategy) Description() string {
	return "Routes requests to the highest priority healthy server with automatic fallback to lower priority servers"
}





// Strategy instances for use in routing
var (
	SingleTargetWithFallback = &SingleTargetWithFallbackStrategy{}
	BroadcastAggregate       = &BroadcastAggregateStrategy{}
	MultiTargetParallel      = &MultiTargetStrategy{}
	PrimaryWithEnhancement   = &SingleTargetWithFallbackStrategy{} // Using fallback for now
	LoadBalanced             = &LoadBalancedStrategy{}
)


// Helper functions for context creation and server selection

// CreateRequestContextFromURI creates a RequestContext from a file URI and workspace information
func CreateRequestContextFromURI(uri string, workspaceRoot string, projectType string) *RequestContext {
	language := detectLanguageFromURI(uri)
	
	return &RequestContext{
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

// CreateWorkspaceRequestContext creates a RequestContext for workspace-wide operations
func CreateWorkspaceRequestContext(workspaceRoot string, projectType string, languages []string) *RequestContext {
	return &RequestContext{
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
		request.Context = CreateRequestContextFromURI(request.URI, "", "")
	}

	return nil
}

// Server selection utilities

func selectBestServer(servers []*RoutingServerInstance, context *RequestContext) *RoutingServerInstance {
	var bestServer *RoutingServerInstance
	bestScore := -1.0

	for _, server := range servers {
		if !server.Available {
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

func selectSuitableServers(servers []*RoutingServerInstance, context *RequestContext) []*RoutingServerInstance {
	var suitable []*RoutingServerInstance

	for _, server := range servers {
		if !server.Available {
			continue
		}

		if isServerSuitableForContext(server, context) {
			suitable = append(suitable, server)
		}
	}

	return suitable
}

func selectLeastLoadedServer(servers []*RoutingServerInstance, context *RequestContext) *RoutingServerInstance {
	var bestServer *RoutingServerInstance
	lowestLoad := float64(1000000) // High initial value

	for _, server := range servers {
		if !server.Available {
			continue
		}

		if !isServerSuitableForContext(server, context) {
			continue
		}

		if server.LoadScore < lowestLoad {
			lowestLoad = server.LoadScore
			bestServer = server
		}
	}

	return bestServer
}

func calculateServerScore(server *RoutingServerInstance, context *RequestContext) float64 {
	score := 0.0

	// Language match (most important)
	if server.Language == context.Language {
		score += 100.0
	} else if supportsLanguage(server, context.Language) {
		score += 50.0
	}

	// Performance metrics
	if server.Performance != nil {
		score += server.Performance.SuccessRate * 20.0
		
		// Lower response time is better
		if server.Performance.AverageResponseTime > 0 {
			responseTimeScore := 1000.0 / float64(server.Performance.AverageResponseTime.Milliseconds())
			score += responseTimeScore
		}

		// Circuit breaker penalty
		if server.Performance.CircuitBreakerOpen {
			score -= 50.0
		}
	}

	// Load score (lower is better)
	score -= server.LoadScore

	// Priority and weight
	score += float64(server.Priority) * 10.0
	score *= server.Weight

	return score
}

func isServerSuitableForContext(server *RoutingServerInstance, context *RequestContext) bool {
	// Check language support
	if server.Language == context.Language {
		return true
	}

	if supportsLanguage(server, context.Language) {
		return true
	}

	// Check if server is in the supported servers list
	for _, supportedServer := range context.SupportedServers {
		if server.Name == supportedServer {
			return true
		}
	}

	return false
}

func supportsLanguage(server *RoutingServerInstance, language string) bool {
	if server.Config == nil {
		return false
	}

	for _, lang := range server.Config.Languages {
		if lang == language {
			return true
		}
	}

	return false
}

// Response aggregation utilities

// TODO: Update to use response_aggregator.go registry system
func getAggregatorForMethod(method string) ResponseAggregator {
	// Temporary stub - needs to be updated to work with response_aggregator.go
	return nil
}

func mergeDefinitionLocations(definitions []interface{}) interface{} {
	// Implementation would merge definition locations, removing duplicates
	// This is a simplified version - real implementation would handle LSP Location types
	merged := make([]interface{}, 0)
	seen := make(map[string]bool)

	for _, def := range definitions {
		if def == nil {
			continue
		}
		
		// Convert to JSON string for deduplication (simplified approach)
		if jsonBytes, err := json.Marshal(def); err == nil {
			key := string(jsonBytes)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, def)
			}
		}
	}

	return merged
}

func mergeReferenceLocations(references []interface{}) interface{} {
	// Similar to definition merging but for references
	merged := make([]interface{}, 0)
	seen := make(map[string]bool)

	for _, ref := range references {
		if ref == nil {
			continue
		}
		
		if jsonBytes, err := json.Marshal(ref); err == nil {
			key := string(jsonBytes)
			if !seen[key] {
				seen[key] = true
				merged = append(merged, ref)
			}
		}
	}

	return merged
}

func mergeSymbols(symbols []interface{}) interface{} {
	// Merge symbols from multiple servers, organizing by relevance
	merged := make([]interface{}, 0)

	for _, symbolSet := range symbols {
		if symbolSet == nil {
			continue
		}
		
		// Add all symbols - in real implementation would sort by relevance
		merged = append(merged, symbolSet)
	}

	return merged
}

func isBetterHoverResponse(current, best interface{}) bool {
	if best == nil {
		return true
	}

	// Simplified comparison - real implementation would compare content richness
	currentJSON, _ := json.Marshal(current)
	bestJSON, _ := json.Marshal(best)

	return len(currentJSON) > len(bestJSON)
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
		return "python"
	case ".js", ".mjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".cs":
		return "csharp"
	default:
		return "unknown"
	}
}

func getSupportedServersForLanguage(language string) []string {
	switch language {
	case "go":
		return []string{"gopls"}
	case "python":
		return []string{"pylsp", "pyright"}
	case "javascript", "typescript":
		return []string{"typescript-language-server", "tsserver"}
	case "java":
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
func (rc *RequestContext) IsFileBasedRequest() bool {
	return rc.FileURI != "" && rc.RequestType == "file_based"
}

// IsWorkspaceRequest returns true if the request is workspace-wide
func (rc *RequestContext) IsWorkspaceRequest() bool {
	return rc.RequestType == "workspace"
}

// GetFileExtension returns the file extension from the URI
func (rc *RequestContext) GetFileExtension() string {
	if rc.FileURI == "" {
		return ""
	}
	return filepath.Ext(rc.FileURI)
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