package version

// Package version provides centralized version information for LSP Gateway.
// These variables are injected at build time via -ldflags.
// Default values are used when building without make.

var (
	Version   = "dev"        // Application version (injected from package.json)
	GitCommit = "unknown"    // Git commit hash
	GitBranch = "unknown"    // Git branch name
	BuildTime = "unknown"    // Build timestamp
	BuildUser = "unknown"    // Build user
)