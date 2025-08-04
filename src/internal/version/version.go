package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
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

func GetBuildInfo() map[string]string {
	return map[string]string{
		"version":   Version,
		"gitCommit": GitCommit,
		"buildDate": BuildDate,
		"goVersion": GoVersion,
	}
}
