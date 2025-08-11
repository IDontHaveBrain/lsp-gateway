package types

import (
	"time"
)

// ClientConfig contains configuration for an LSP client
type ClientConfig struct {
	Command               string
	Args                  []string
	WorkingDir            string
	InitializationOptions interface{} // Optional initialization options from config
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
