package types

import (
	"time"
)

// TransportMode defines how to communicate with the LSP server
type TransportMode string

const (
	TransportStdio  TransportMode = "stdio"
	TransportSocket TransportMode = "socket"
)

// ClientConfig contains configuration for an LSP client
type ClientConfig struct {
	Command               string
	Args                  []string
	WorkingDir            string
	InitializationOptions interface{} // Optional initialization options from config

	// Transport configuration
	Transport TransportMode
	Host      string // For socket mode
	Port      int    // For socket mode
}

type ServerInstallOptions struct {
	Version             string
	Force               bool
	SkipVerify          bool
	SkipDependencyCheck bool
	Timeout             time.Duration
	Platform            string
	InstallMethod       string
	WorkingDir          string
}
