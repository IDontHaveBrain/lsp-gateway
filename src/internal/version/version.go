// Package version exposes build and version metadata.
package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "0.0.22"
	GitCommit = "unknown"
	BuildDate = "unknown"
	GoVersion = runtime.Version()
)

func GetVersion() string {
	return Version
}

func GetFullVersionInfo() string {
	return fmt.Sprintf("lsp-gateway %s (commit: %s, built: %s, go: %s)",
		Version, GitCommit, BuildDate, GoVersion)
}
