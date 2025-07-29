package workspace

import (
	"context"
	"fmt"
	"time"

	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// RoutingMetrics provides routing performance metrics (simple stub)
type RoutingMetrics struct {
	RequestCount     int64            `json:"request_count"`
	SuccessfulRoutes int64            `json:"successful_routes"`
	FailedRoutes     int64            `json:"failed_routes"`
	StrategyUsage    map[string]int64 `json:"strategy_usage"`
	LastUpdated      time.Time        `json:"last_updated"`
}

// RoutingErrorType represents types of routing errors
type RoutingErrorType int

const (
	RoutingErrorUnknown RoutingErrorType = iota
	RoutingErrorClientNotFound
	RoutingErrorTimeout
	RoutingErrorProjectNotResolved
)

// RoutingDecision represents routing decision (compatible stub)
type RoutingDecision struct {
	Method          string                    `json:"method"`
	FileURI         string                    `json:"file_uri"`
	Strategy        RoutingStrategy           `json:"-"`
	ProjectID       string                    `json:"project_id"`
	Language        string                    `json:"language"`
	RequestID       interface{}               `json:"request_id"`
	TargetProject   *SubProject              `json:"target_project"`
	PrimaryClient   transport.LSPClient       `json:"-"`
	FallbackClients []transport.LSPClient     `json:"-"`
	Context         map[string]interface{}    `json:"context"`
	Timeout         time.Duration             `json:"timeout"`
}

// RoutingError represents routing errors
type RoutingError struct {
	Message     string            `json:"message"`
	ErrorType   RoutingErrorType  `json:"error_type"`
	Context     map[string]interface{} `json:"context"`
	ProjectID   string            `json:"project_id"`
	FileURI     string            `json:"file_uri"`
	OriginalErr error             `json:"-"`
}

func (e *RoutingError) Error() string {
	return e.Message
}

// RoutingStrategy interface for routing strategies (stub)
type RoutingStrategy interface {
	Name() string
	Route(ctx context.Context, decision *RoutingDecision) (*JSONRPCResponse, error)
}

// SubProjectRequestRouter interface for complex routing (stub implementation)
type SubProjectRequestRouter interface {
	RouteRequest(ctx context.Context, request *JSONRPCRequest) (*RoutingDecision, error)
	HandleRoutingFailure(ctx context.Context, request *JSONRPCRequest, err error) (*JSONRPCResponse, error)
	GetRoutingMetrics() *RoutingMetrics
	SetResolver(resolver SubProjectResolver)
	SetClientManager(manager SubProjectClientManager)
	Shutdown(ctx context.Context) error
}

// Simple stub implementations
func NewSubProjectRequestRouter(logger *mcp.StructuredLogger) SubProjectRequestRouter {
	return &stubRequestRouter{
		logger: logger,
		metrics: &RoutingMetrics{
			StrategyUsage: make(map[string]int64),
			LastUpdated:   time.Now(),
		},
	}
}

// Stub request router implementation  
type stubRequestRouter struct {
	logger  *mcp.StructuredLogger
	metrics *RoutingMetrics
}

func (s *stubRequestRouter) RouteRequest(ctx context.Context, request *JSONRPCRequest) (*RoutingDecision, error) {
	// Always return error to fall back to simple routing
	return nil, fmt.Errorf("enhanced routing not implemented, using simple routing")
}

func (s *stubRequestRouter) HandleRoutingFailure(ctx context.Context, request *JSONRPCRequest, err error) (*JSONRPCResponse, error) {
	return nil, fmt.Errorf("fallback routing not implemented")
}

func (s *stubRequestRouter) GetRoutingMetrics() *RoutingMetrics {
	if s.metrics == nil {
		s.metrics = &RoutingMetrics{
			StrategyUsage: make(map[string]int64),
			LastUpdated:   time.Now(),
		}
	}
	return s.metrics
}

func (s *stubRequestRouter) SetResolver(resolver SubProjectResolver) {
	// Stub - no-op
}

func (s *stubRequestRouter) SetClientManager(manager SubProjectClientManager) {
	// Stub - no-op  
}

func (s *stubRequestRouter) Shutdown(ctx context.Context) error {
	return nil
}

// Simple stub routing strategy
type stubRoutingStrategy struct {
	name string
}

func (s *stubRoutingStrategy) Name() string {
	return s.name
}

func (s *stubRoutingStrategy) Route(ctx context.Context, decision *RoutingDecision) (*JSONRPCResponse, error) {
	return nil, fmt.Errorf("stub strategy %s not implemented", s.name)
}

// Helper constructor for stub strategy
func NewStubRoutingStrategy(name string) RoutingStrategy {
	return &stubRoutingStrategy{name: name}
}