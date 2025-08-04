package types

import (
	"fmt"
	"time"
)

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

// MCPMode represents the operational mode of the MCP server
type MCPMode string

const (
	// MCPModeLSP provides direct 1:1 LSP method mapping (current behavior)
	// Tools return raw LSP responses with minimal processing
	MCPModeLSP MCPMode = "lsp"

	// MCPModeEnhanced provides AI-optimized tools with processed data
	// Tools return structured, context-rich responses optimized for LLMs
	MCPModeEnhanced MCPMode = "enhanced"
)

// DefaultMCPMode is the default mode (maintains backward compatibility)
const DefaultMCPMode = MCPModeLSP

// ValidModes returns all valid MCP modes
func ValidModes() []MCPMode {
	return []MCPMode{MCPModeLSP, MCPModeEnhanced}
}

// IsValid checks if a mode string is valid
func (m MCPMode) IsValid() bool {
	for _, valid := range ValidModes() {
		if m == valid {
			return true
		}
	}
	return false
}

// String returns the string representation
func (m MCPMode) String() string {
	return string(m)
}

// ParseMCPMode parses a string into MCPMode with validation
func ParseMCPMode(s string) (MCPMode, error) {
	if s == "" {
		return DefaultMCPMode, nil
	}
	mode := MCPMode(s)
	if !mode.IsValid() {
		return "", fmt.Errorf("invalid MCP mode '%s', valid modes: %v", s, ValidModes())
	}
	return mode, nil
}

// GetDescription returns a human-readable description of the mode
func (m MCPMode) GetDescription() string {
	switch m {
	case MCPModeLSP:
		return "Direct LSP mapping - returns raw LSP responses with minimal processing"
	case MCPModeEnhanced:
		return "AI-optimized tools - returns structured, context-rich responses for LLMs"
	default:
		return "Unknown mode"
	}
}
