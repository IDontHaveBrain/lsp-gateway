package types

import (
	"time"
)

type InstallOptions struct {
	Version        string
	Force          bool
	SkipVerify     bool
	Timeout        time.Duration
	PackageManager string
	Platform       string
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

type InstallResult struct {
	Success  bool
	Runtime  string
	Version  string
	Path     string
	Method   string
	Duration time.Duration
	Errors   []string
	Warnings []string
	Messages []string
	Details  map[string]interface{}
}

type VerificationResult struct {
	Installed       bool
	Compatible      bool
	Version         string
	Path            string
	Runtime         string
	Issues          []Issue
	Details         map[string]interface{}
	Metadata        map[string]interface{}
	EnvironmentVars map[string]string
	Recommendations []string
	WorkingDir      string
	AdditionalPaths []string
	VerifiedAt      time.Time
	Duration        time.Duration
}

type Issue struct {
	Severity    IssueSeverity
	Category    IssueCategory
	Title       string
	Description string
	Solution    string
	Details     map[string]interface{}
}

type IssueCategory string

const (
	IssueCategoryInstallation  IssueCategory = "installation"
	IssueCategoryVersion       IssueCategory = "version"
	IssueCategoryPath          IssueCategory = "path"
	IssueCategoryEnvironment   IssueCategory = "environment"
	IssueCategoryPermissions   IssueCategory = "permissions"
	IssueCategoryDependencies  IssueCategory = "dependencies"
	IssueCategoryConfiguration IssueCategory = "configuration"
	IssueCategoryCorruption    IssueCategory = "corruption"
	IssueCategoryExecution     IssueCategory = "execution"
)

type IssueSeverity string

const (
	IssueSeverityInfo     IssueSeverity = "info"
	IssueSeverityLow      IssueSeverity = "low"
	IssueSeverityMedium   IssueSeverity = "medium"
	IssueSeverityHigh     IssueSeverity = "high"
	IssueSeverityCritical IssueSeverity = "critical"
)

type RuntimeDefinition struct {
	Name               string
	DisplayName        string
	MinVersion         string
	RecommendedVersion string
	InstallMethods     map[string]InstallMethod
	VerificationCmd    []string
	VersionCommand     []string
	EnvVars            map[string]string
	Dependencies       []string
	PostInstall        []string
}

type ServerDefinition struct {
	Name              string
	DisplayName       string
	Runtime           string
	MinVersion        string
	MinRuntimeVersion string
	InstallCmd        []string
	VerifyCmd         []string
	ConfigKey         string
	Description       string
	Homepage          string
	Languages         []string
	Extensions        []string
	InstallMethods    []InstallMethod
	VersionCommand    []string
}

type InstallMethod struct {
	Name          string
	Platform      string
	Method        string
	Commands      []string
	Description   string
	Requirements  []string
	PreRequisites []string
	Verification  []string
	PostInstall   []string
}

type DependencyValidationResult struct {
	Server            string
	Valid             bool
	RuntimeRequired   string
	RuntimeInstalled  bool
	RuntimeVersion    string
	RuntimeCompatible bool
	Issues            []Issue
	CanInstall        bool
	MissingRuntimes   []string
	VersionIssues     []VersionIssue
	Recommendations   []string
	ValidatedAt       time.Time
	Duration          time.Duration
}

type VersionIssue struct {
	Component        string
	RequiredVersion  string
	InstalledVersion string
	Severity         IssueSeverity
}

type VersionValidationResult struct {
	Valid            bool
	RequiredVersion  string
	InstalledVersion string
	Issues           []Issue
}
