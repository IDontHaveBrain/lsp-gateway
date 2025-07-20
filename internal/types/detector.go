package types

import (
	"context"
	"time"
)

type RuntimeDetector interface {
	DetectGo(ctx context.Context) (*RuntimeInfo, error)

	DetectPython(ctx context.Context) (*RuntimeInfo, error)

	DetectNodejs(ctx context.Context) (*RuntimeInfo, error)

	DetectJava(ctx context.Context) (*RuntimeInfo, error)

	DetectAll(ctx context.Context) (*DetectionReport, error)

	SetLogger(logger interface{})

	SetTimeout(timeout time.Duration)
}

type RuntimeInfo struct {
	Name       string                 // Runtime name (go, python, nodejs, java)
	Version    string                 // Detected version
	Path       string                 // Path to the runtime executable
	Installed  bool                   // Whether the runtime is installed
	Compatible bool                   // Whether the version is compatible with requirements
	Issues     []string               // Any issues found during detection (simplified)
	Metadata   map[string]interface{} // Additional runtime-specific metadata
}

type DetectionReport struct {
	Platform     string                  // Detected platform as string
	Architecture string                  // Detected architecture as string
	Runtimes     map[string]*RuntimeInfo // Detected runtimes
	Summary      *DetectionSummary       // Summary of detection results
	Issues       []string                // Global detection issues
	Timestamp    time.Time               // When detection was performed
	Duration     time.Duration           // Time taken for detection
}

type DetectionSummary struct {
	TotalRuntimes      int     // Total runtimes checked
	InstalledRuntimes  int     // Number of installed runtimes
	CompatibleRuntimes int     // Number of compatible runtimes
	IssuesFound        int     // Number of issues found
	ReadinessScore     float64 // Overall readiness score (0-100)
}

type SetupLogger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Debug(msg string)
	WithField(key string, value interface{}) SetupLogger
	WithFields(fields map[string]interface{}) SetupLogger
	WithError(err error) SetupLogger
	WithOperation(op string) SetupLogger
}
