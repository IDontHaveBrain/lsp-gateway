package gateway

import (
	"context"
	"net/http"

	"lsp-gateway/internal/transport"
)

// GatewayInterface defines the common interface for both traditional and project-aware gateways
type GatewayInterface interface {
	// Start starts the gateway and initializes all LSP clients
	Start(ctx context.Context) error

	// Stop stops the gateway and cleans up all LSP clients
	Stop() error

	// HandleJSONRPC handles incoming JSON-RPC HTTP requests
	HandleJSONRPC(w http.ResponseWriter, r *http.Request)

	// GetClient retrieves an LSP client by server name
	GetClient(serverName string) (transport.LSPClient, bool)
}

// ProjectAwareGatewayInterface extends the basic gateway interface with project-aware operations
type ProjectAwareGatewayInterface interface {
	GatewayInterface

	// GetWorkspaceClient retrieves an LSP client for a specific workspace and language
	GetWorkspaceClient(workspaceID, language string) (transport.LSPClient, error)

	// GetWorkspaceContext retrieves or creates workspace context for a file URI
	GetWorkspaceContext(fileURI string) (WorkspaceContext, error)

	// GetAllWorkspaces returns information about all active workspaces
	GetAllWorkspaces() []WorkspaceContext

	// CleanupWorkspace manually cleans up a specific workspace
	CleanupWorkspace(workspaceID string) error

	// IsProjectAware indicates that this gateway supports project-aware operations
	IsProjectAware() bool
}

// GatewayType represents the type of gateway to create
type GatewayType int

const (
	// GatewayTypeTraditional creates a traditional gateway without project awareness
	GatewayTypeTraditional GatewayType = iota

	// GatewayTypeProjectAware creates a project-aware gateway with workspace management
	GatewayTypeProjectAware
)

// CreateGatewayOptions contains options for gateway creation
type CreateGatewayOptions struct {
	Type                  GatewayType
	Config                interface{} // *config.GatewayConfig
	ForceProjectAware     bool
	FallbackToTraditional bool
}

// GatewayCreationResult contains the result of gateway creation
type GatewayCreationResult struct {
	Gateway        GatewayInterface
	Type           GatewayType
	IsProjectAware bool
	Error          error
}

// Ensure both gateway types implement the interface at compile time
var (
	_ GatewayInterface             = (*Gateway)(nil)
	_ GatewayInterface             = (*ProjectAwareGateway)(nil)
	_ ProjectAwareGatewayInterface = (*ProjectAwareGateway)(nil)
)
