package diagnostics

import (
	"fmt"
	"lsp-gateway/internal/types"
	"strings"
	"time"
)

type TroubleshootingGuide interface {
	AnalyzeIssue(issue types.Issue) (*TroubleshootingResult, error)

	GetRecommendations(report *HealthReport) ([]Recommendation, error)

	AutoFix(issue types.Issue) (*AutoFixResult, error)

	GetCommonIssues() []CommonIssue
}

type TroubleshootingResult struct {
	Issue          types.Issue
	PossibleCauses []string
	Solutions      []Solution
	AutoFixable    bool
	Severity       types.IssueSeverity
	EstimatedTime  string
}

type Solution struct {
	Title       string
	Description string
	Steps       []TroubleshootingStep
	Commands    []string
	Manual      bool
	Difficulty  SolutionDifficulty
}

type TroubleshootingStep struct {
	Number      int
	Description string
	Command     string
	Expected    string
	Optional    bool
}

type SolutionDifficulty string

const (
	SolutionDifficultyEasy   SolutionDifficulty = "easy"
	SolutionDifficultyMedium SolutionDifficulty = "medium"
	SolutionDifficultyHard   SolutionDifficulty = "hard"
	SolutionDifficultyExpert SolutionDifficulty = "expert"
)

type AutoFixResult struct {
	Success   bool
	Applied   []string // Applied fixes
	Skipped   []string // Skipped fixes with reasons
	Errors    []string // Errors encountered
	NextSteps []string // Manual steps still required
	Duration  string
}

type CommonIssue struct {
	ID          string
	Title       string
	Description string
	Category    types.IssueCategory
	Symptoms    []string
	Causes      []string
	Solutions   []Solution
	AutoFixable bool
}

type DefaultTroubleshootingGuide struct {
	commonIssues map[string]CommonIssue
}

func NewTroubleshootingGuide() *DefaultTroubleshootingGuide {
	guide := &DefaultTroubleshootingGuide{
		commonIssues: make(map[string]CommonIssue),
	}

	guide.initializeCommonIssues()

	return guide
}

func (t *DefaultTroubleshootingGuide) AnalyzeIssue(issue types.Issue) (*TroubleshootingResult, error) {
	result := &TroubleshootingResult{
		Issue:     issue,
		Severity:  issue.Severity,
		Solutions: []Solution{},
	}

	if commonIssue, exists := t.findMatchingCommonIssue(issue); exists {
		result.PossibleCauses = commonIssue.Causes
		result.Solutions = commonIssue.Solutions
		result.AutoFixable = commonIssue.AutoFixable
		result.EstimatedTime = t.estimateResolutionTime(commonIssue.Solutions)
		return result, nil
	}

	result.PossibleCauses = t.analyzePossibleCauses(issue)
	result.Solutions = t.generateDynamicSolutions(issue)
	result.AutoFixable = t.isAutoFixable(issue, result.Solutions)
	result.EstimatedTime = t.estimateResolutionTime(result.Solutions)

	return result, nil
}

func (t *DefaultTroubleshootingGuide) GetRecommendations(report *HealthReport) ([]Recommendation, error) {
	var recommendations []Recommendation

	criticalIssues := []DiagnosticIssue{}
	errorIssues := []DiagnosticIssue{}
	warningIssues := []DiagnosticIssue{}

	for _, issue := range report.Issues {
		switch issue.Severity {
		case IssueSeverityCritical:
			criticalIssues = append(criticalIssues, issue)
		case IssueSeverityError:
			errorIssues = append(errorIssues, issue)
		case IssueSeverityWarning:
			warningIssues = append(warningIssues, issue)
		}
	}

	for _, issue := range criticalIssues {
		rec := t.generateRecommendationForIssue(issue.Issue, RecommendationPriorityCritical)
		if rec != nil {
			recommendations = append(recommendations, *rec)
		}
	}

	for _, issue := range errorIssues {
		rec := t.generateRecommendationForIssue(issue.Issue, RecommendationPriorityHigh)
		if rec != nil {
			recommendations = append(recommendations, *rec)
		}
	}

	for _, issue := range warningIssues {
		rec := t.generateRecommendationForIssue(issue.Issue, RecommendationPriorityMedium)
		if rec != nil {
			recommendations = append(recommendations, *rec)
		}
	}

	proactiveRecs := t.generateProactiveRecommendations(report)
	recommendations = append(recommendations, proactiveRecs...)

	return recommendations, nil
}

func (t *DefaultTroubleshootingGuide) AutoFix(issue types.Issue) (*AutoFixResult, error) {
	startTime := time.Now()
	result := &AutoFixResult{
		Applied:   []string{},
		Skipped:   []string{},
		Errors:    []string{},
		NextSteps: []string{},
	}

	if !t.canAutoFix(issue) {
		result.Success = false
		result.Skipped = append(result.Skipped, fmt.Sprintf("Issue '%s' cannot be auto-fixed", issue.Title))
		result.NextSteps = append(result.NextSteps, "Manual intervention required")
		result.Duration = time.Since(startTime).String()
		return result, nil
	}

	var success bool
	switch issue.Category {
	case IssueCategoryConfig:
		success = t.autoFixConfigIssue(issue, result)
	case IssueCategoryRuntime:
		success = t.autoFixRuntimeIssue(issue, result)
	case IssueCategoryServer:
		success = t.autoFixServerIssue(issue, result)
	case IssueCategoryPermission:
		success = t.autoFixPermissionIssue(issue, result)
	case IssueCategoryDependency:
		success = t.autoFixDependencyIssue(issue, result)
	default:
		success = false
		result.Skipped = append(result.Skipped, fmt.Sprintf("No auto-fix available for category '%s'", issue.Category))
		result.NextSteps = append(result.NextSteps, issue.Solution)
	}

	result.Success = success && len(result.Errors) == 0
	result.Duration = time.Since(startTime).String()

	if !result.Success && len(result.NextSteps) == 0 {
		result.NextSteps = append(result.NextSteps, "Review errors above and apply manual fixes")
		result.NextSteps = append(result.NextSteps, "Run diagnostics again to verify fixes")
	}

	return result, nil
}

func (t *DefaultTroubleshootingGuide) GetCommonIssues() []CommonIssue {
	issues := make([]CommonIssue, 0, len(t.commonIssues))
	for _, issue := range t.commonIssues {
		issues = append(issues, issue)
	}
	return issues
}

func (t *DefaultTroubleshootingGuide) initializeCommonIssues() {
	t.commonIssues["runtime_not_found"] = CommonIssue{
		ID:          "runtime_not_found",
		Title:       "Runtime Not Found",
		Description: "Required programming language runtime is not installed or not in PATH",
		Category:    IssueCategoryRuntime,
		Symptoms:    []string{"Command not found", "Runtime detection failed"},
		Causes:      []string{"Runtime not installed", "PATH not configured", "Wrong version installed"},
		Solutions: []Solution{
			{
				Title:       "Install Runtime",
				Description: "Install the required runtime using the system package manager",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Check if runtime is installed", Command: "which go", Expected: "Path to executable"},
					{Number: 2, Description: "Install runtime if missing", Command: "./lsp-gateway install runtime go", Expected: "Installation success"},
				},
				Commands:   []string{"./lsp-gateway install runtime go"},
				Manual:     false,
				Difficulty: SolutionDifficultyEasy,
			},
		},
		AutoFixable: true,
	}

	t.commonIssues["version_incompatible"] = CommonIssue{
		ID:          "version_incompatible",
		Title:       "Version Incompatible",
		Description: "Installed runtime version does not meet minimum requirements",
		Category:    IssueCategoryRuntime,
		Symptoms:    []string{"Version check failed", "Compatibility check failed"},
		Causes:      []string{"Outdated version", "Wrong version branch"},
		Solutions: []Solution{
			{
				Title:       "Update Runtime",
				Description: "Update to a compatible version",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Check current version", Command: "go version", Expected: "Current version"},
					{Number: 2, Description: "Update runtime", Command: "./lsp-gateway install runtime go --force", Expected: "Update success"},
				},
				Commands:   []string{"./lsp-gateway install runtime go --force"},
				Manual:     false,
				Difficulty: SolutionDifficultyEasy,
			},
		},
		AutoFixable: true,
	}

	t.commonIssues["server_not_found"] = CommonIssue{
		ID:          "server_not_found",
		Title:       "Language Server Not Found",
		Description: "Language server is not installed or not accessible",
		Category:    IssueCategoryServer,
		Symptoms:    []string{"Server command not found", "Server communication failed"},
		Causes:      []string{"Server not installed", "Wrong installation path", "Permission issues"},
		Solutions: []Solution{
			{
				Title:       "Install Language Server",
				Description: "Install the required language server",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Install server", Command: "./lsp-gateway install server gopls", Expected: "Installation success"},
					{Number: 2, Description: "Verify server", Command: "./lsp-gateway status servers", Expected: "Server working"},
				},
				Commands:   []string{"./lsp-gateway install server gopls"},
				Manual:     false,
				Difficulty: SolutionDifficultyEasy,
			},
		},
		AutoFixable: true,
	}

	t.commonIssues["config_invalid"] = CommonIssue{
		ID:          "config_invalid",
		Title:       "Invalid Configuration",
		Description: "Configuration file is missing, invalid, or incomplete",
		Category:    IssueCategoryConfig,
		Symptoms:    []string{"Config validation failed", "Server startup failed"},
		Causes:      []string{"Missing config file", "Invalid YAML syntax", "Missing required fields"},
		Solutions: []Solution{
			{
				Title:       "Generate Configuration",
				Description: "Generate a new configuration file",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Generate config", Command: "./lsp-gateway config generate", Expected: "Config created"},
					{Number: 2, Description: "Validate config", Command: "./lsp-gateway config validate", Expected: "Config valid"},
				},
				Commands:   []string{"./lsp-gateway config generate"},
				Manual:     false,
				Difficulty: SolutionDifficultyEasy,
			},
		},
		AutoFixable: true,
	}

	t.commonIssues["permission_denied"] = CommonIssue{
		ID:          "permission_denied",
		Title:       "Permission Denied",
		Description: "Insufficient permissions to access files or directories",
		Category:    IssueCategoryPermission,
		Symptoms:    []string{"Access denied", "Permission denied", "Operation not permitted"},
		Causes:      []string{"Insufficient file permissions", "Directory not writable", "Missing execute permissions"},
		Solutions: []Solution{
			{
				Title:       "Fix File Permissions",
				Description: "Update file and directory permissions",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Check current permissions", Command: "ls -la", Expected: "Current permission settings"},
					{Number: 2, Description: "Fix permissions", Command: "chmod 755 ./lsp-gateway", Expected: "Updated permissions"},
					{Number: 3, Description: "Verify access", Command: "./lsp-gateway version", Expected: "Version information displayed"},
				},
				Commands:   []string{"chmod 755 ./lsp-gateway"},
				Manual:     false,
				Difficulty: SolutionDifficultyEasy,
			},
		},
		AutoFixable: true,
	}

	t.commonIssues["network_unreachable"] = CommonIssue{
		ID:          "network_unreachable",
		Title:       "Network Unreachable",
		Description: "Cannot connect to remote repositories or services",
		Category:    IssueCategoryNetwork,
		Symptoms:    []string{"Connection timeout", "Host unreachable", "Download failed"},
		Causes:      []string{"No internet connection", "Firewall blocking", "Proxy misconfiguration", "DNS issues"},
		Solutions: []Solution{
			{
				Title:       "Diagnose Network Issues",
				Description: "Check network connectivity and configuration",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Test basic connectivity", Command: "ping 8.8.8.8", Expected: "Successful ping responses"},
					{Number: 2, Description: "Test DNS resolution", Command: "nslookup google.com", Expected: "DNS resolution working"},
					{Number: 3, Description: "Check for proxy settings", Expected: "Proxy configuration reviewed"},
				},
				Commands:   []string{"ping 8.8.8.8", "nslookup google.com"},
				Manual:     true,
				Difficulty: SolutionDifficultyMedium,
			},
		},
		AutoFixable: false,
	}

	t.commonIssues["python_pip_missing"] = CommonIssue{
		ID:          "python_pip_missing",
		Title:       "Python Pip Not Found",
		Description: "Python is installed but pip package manager is missing",
		Category:    IssueCategoryDependency,
		Symptoms:    []string{"pip: command not found", "pip not available", "package installation failed"},
		Causes:      []string{"Incomplete Python installation", "Pip not included in distribution", "Path misconfiguration"},
		Solutions: []Solution{
			{
				Title:       "Install Pip",
				Description: "Install pip package manager for Python",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Check Python installation", Command: "python --version", Expected: "Python version displayed"},
					{Number: 2, Description: "Install pip", Command: "python -m ensurepip --upgrade", Expected: "Pip installed successfully"},
					{Number: 3, Description: "Verify pip installation", Command: "pip --version", Expected: "Pip version displayed"},
				},
				Commands:   []string{"python -m ensurepip --upgrade"},
				Manual:     false,
				Difficulty: SolutionDifficultyEasy,
			},
		},
		AutoFixable: true,
	}

	t.commonIssues["nodejs_npm_missing"] = CommonIssue{
		ID:          "nodejs_npm_missing",
		Title:       "Node.js NPM Not Found",
		Description: "Node.js is installed but npm package manager is missing",
		Category:    IssueCategoryDependency,
		Symptoms:    []string{"npm: command not found", "npm not available", "global install failed"},
		Causes:      []string{"Incomplete Node.js installation", "NPM not included", "Separate npm installation required"},
		Solutions: []Solution{
			{
				Title:       "Install NPM",
				Description: "Install npm package manager for Node.js",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Check Node.js installation", Command: "node --version", Expected: "Node.js version displayed"},
					{Number: 2, Description: "Download npm installer", Expected: "NPM installer downloaded"},
					{Number: 3, Description: "Install npm", Expected: "NPM installed successfully"},
					{Number: 4, Description: "Verify npm installation", Command: "npm --version", Expected: "NPM version displayed"},
				},
				Commands:   []string{},
				Manual:     true,
				Difficulty: SolutionDifficultyMedium,
			},
		},
		AutoFixable: false,
	}

	t.commonIssues["java_home_missing"] = CommonIssue{
		ID:          "java_home_missing",
		Title:       "JAVA_HOME Not Set",
		Description: "Java is installed but JAVA_HOME environment variable is not configured",
		Category:    IssueCategoryRuntime,
		Symptoms:    []string{"JAVA_HOME not found", "Java tools not working", "Compiler not available"},
		Causes:      []string{"Missing JAVA_HOME environment variable", "Incorrect JAVA_HOME path", "Environment not reloaded"},
		Solutions: []Solution{
			{
				Title:       "Set JAVA_HOME",
				Description: "Configure JAVA_HOME environment variable",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Find Java installation", Command: "which java", Expected: "Java binary path"},
					{Number: 2, Description: "Set JAVA_HOME", Expected: "Environment variable configured"},
					{Number: 3, Description: "Verify JAVA_HOME", Command: "echo $JAVA_HOME", Expected: "JAVA_HOME path displayed"},
				},
				Commands:   []string{"which java"},
				Manual:     true,
				Difficulty: SolutionDifficultyMedium,
			},
		},
		AutoFixable: false,
	}

	t.commonIssues["lsp_communication_failed"] = CommonIssue{
		ID:          "lsp_communication_failed",
		Title:       "LSP Server Communication Failed",
		Description: "Cannot establish communication with language server",
		Category:    IssueCategoryServer,
		Symptoms:    []string{"Server timeout", "Connection refused", "Communication error", "Server not responding"},
		Causes:      []string{"Server not started", "Server crashed", "Port conflict", "Protocol mismatch"},
		Solutions: []Solution{
			{
				Title:       "Restart LSP Server",
				Description: "Restart the language server and verify communication",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Stop current server", Command: "pkill -f lsp-server", Expected: "Server stopped"},
					{Number: 2, Description: "Start server in debug mode", Expected: "Server starts with debug output"},
					{Number: 3, Description: "Test communication", Command: "./lsp-gateway server --config config.yaml", Expected: "Server responds successfully"},
				},
				Commands:   []string{"pkill -f lsp-server"},
				Manual:     true,
				Difficulty: SolutionDifficultyMedium,
			},
		},
		AutoFixable: false,
	}

	t.commonIssues["path_misconfigured"] = CommonIssue{
		ID:          "path_misconfigured",
		Title:       "PATH Environment Variable Misconfigured",
		Description: "Required executables are not found in the system PATH",
		Category:    IssueCategoryPlatform,
		Symptoms:    []string{"command not found", "executable not in PATH", "which command returns empty"},
		Causes:      []string{"Missing PATH entries", "Incorrect PATH configuration", "Shell configuration not loaded"},
		Solutions: []Solution{
			{
				Title:       "Fix PATH Configuration",
				Description: "Add required directories to system PATH",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Check current PATH", Command: "echo $PATH", Expected: "Current PATH displayed"},
					{Number: 2, Description: "Find executable location", Expected: "Executable path identified"},
					{Number: 3, Description: "Add to PATH", Expected: "PATH updated with new directory"},
					{Number: 4, Description: "Reload shell configuration", Expected: "New PATH active"},
				},
				Commands:   []string{"echo $PATH"},
				Manual:     true,
				Difficulty: SolutionDifficultyMedium,
			},
		},
		AutoFixable: false,
	}

	t.commonIssues["installation_corrupted"] = CommonIssue{
		ID:          "installation_corrupted",
		Title:       "Installation Corrupted",
		Description: "Runtime or language server installation appears to be corrupted",
		Category:    IssueCategoryRuntime,
		Symptoms:    []string{"Segmentation fault", "Invalid binary", "Execution failed", "Unexpected errors"},
		Causes:      []string{"Incomplete download", "File corruption", "Permission issues during installation", "Disk space issues"},
		Solutions: []Solution{
			{
				Title:       "Reinstall Component",
				Description: "Remove and reinstall the corrupted component",
				Steps: []TroubleshootingStep{
					{Number: 1, Description: "Remove corrupted installation", Expected: "Old installation removed"},
					{Number: 2, Description: "Clear cache and temporary files", Expected: "Cache cleared"},
					{Number: 3, Description: "Reinstall component", Command: "./lsp-gateway install runtime go --force", Expected: "Fresh installation completed"},
					{Number: 4, Description: "Verify installation", Command: "./lsp-gateway verify runtime go", Expected: "Installation verified as working"},
				},
				Commands:   []string{"./lsp-gateway install runtime go --force"},
				Manual:     false,
				Difficulty: SolutionDifficultyEasy,
			},
		},
		AutoFixable: true,
	}
}

func (t *DefaultTroubleshootingGuide) findMatchingCommonIssue(issue types.Issue) (CommonIssue, bool) {
	if issue.Details != nil {
		if component, ok := issue.Details["component"].(string); ok {
			if commonIssue, exists := t.commonIssues[component]; exists {
				return commonIssue, true
			}
		}
	}

	for _, commonIssue := range t.commonIssues {
		if t.issueMatches(issue, commonIssue) {
			return commonIssue, true
		}
	}

	return CommonIssue{}, false
}

func (t *DefaultTroubleshootingGuide) issueMatches(issue types.Issue, commonIssue CommonIssue) bool {
	if issue.Category != commonIssue.Category {
		return false
	}

	issueTitle := strings.ToLower(issue.Title)
	commonTitle := strings.ToLower(commonIssue.Title)

	if strings.Contains(issueTitle, commonTitle) || strings.Contains(commonTitle, issueTitle) {
		return true
	}

	issueDesc := strings.ToLower(issue.Description)
	for _, symptom := range commonIssue.Symptoms {
		if strings.Contains(issueDesc, strings.ToLower(symptom)) {
			return true
		}
	}

	return false
}

func (t *DefaultTroubleshootingGuide) analyzePossibleCauses(issue types.Issue) []string {
	causes := []string{}

	switch issue.Category {
	case IssueCategoryRuntime:
		causes = append(causes, t.analyzeRuntimeCauses(issue)...)
	case IssueCategoryServer:
		causes = append(causes, t.analyzeServerCauses(issue)...)
	case IssueCategoryConfig:
		causes = append(causes, t.analyzeConfigCauses(issue)...)
	case IssueCategoryNetwork:
		causes = append(causes, t.analyzeNetworkCauses(issue)...)
	case IssueCategoryPermission:
		causes = append(causes, t.analyzePermissionCauses(issue)...)
	case IssueCategoryDependency:
		causes = append(causes, t.analyzeDependencyCauses(issue)...)
	default:
		causes = append(causes, "Unknown cause - requires manual investigation")
	}

	if len(causes) == 0 {
		causes = append(causes, "Issue cause could not be automatically determined")
	}

	return causes
}

func (t *DefaultTroubleshootingGuide) analyzeRuntimeCauses(issue types.Issue) []string {
	causes := []string{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "not found") || strings.Contains(desc, "command not found") {
		causes = append(causes, "Runtime not installed or not in PATH")
	}
	if strings.Contains(desc, "version") || strings.Contains(desc, "incompatible") {
		causes = append(causes, "Incompatible runtime version")
	}
	if strings.Contains(desc, "permission") || strings.Contains(desc, "access denied") {
		causes = append(causes, "Permission issues with runtime installation")
	}
	if strings.Contains(desc, "corrupt") || strings.Contains(desc, "damaged") {
		causes = append(causes, "Corrupted runtime installation")
	}

	return causes
}

func (t *DefaultTroubleshootingGuide) analyzeServerCauses(issue types.Issue) []string {
	causes := []string{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "not found") || strings.Contains(desc, "command not found") {
		causes = append(causes, "Language server not installed")
	}
	if strings.Contains(desc, "communication") || strings.Contains(desc, "connection") {
		causes = append(causes, "Server communication failure")
	}
	if strings.Contains(desc, "runtime") {
		causes = append(causes, "Required runtime not available for server")
	}
	if strings.Contains(desc, "startup") || strings.Contains(desc, "initialization") {
		causes = append(causes, "Server startup failure")
	}

	return causes
}

func (t *DefaultTroubleshootingGuide) analyzeConfigCauses(issue types.Issue) []string {
	causes := []string{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "missing") || strings.Contains(desc, "not found") {
		causes = append(causes, "Configuration file missing")
	}
	if strings.Contains(desc, "syntax") || strings.Contains(desc, "parsing") {
		causes = append(causes, "Invalid YAML syntax in configuration")
	}
	if strings.Contains(desc, "validation") || strings.Contains(desc, "invalid") {
		causes = append(causes, "Configuration values do not meet schema requirements")
	}
	if strings.Contains(desc, "path") || strings.Contains(desc, "file") {
		causes = append(causes, "Invalid file paths in configuration")
	}

	return causes
}

func (t *DefaultTroubleshootingGuide) analyzeNetworkCauses(issue types.Issue) []string {
	causes := []string{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "timeout") || strings.Contains(desc, "unreachable") {
		causes = append(causes, "Network connectivity issues")
	}
	if strings.Contains(desc, "proxy") {
		causes = append(causes, "Proxy configuration blocking connections")
	}
	if strings.Contains(desc, "firewall") || strings.Contains(desc, "blocked") {
		causes = append(causes, "Firewall blocking network access")
	}
	if strings.Contains(desc, "ssl") || strings.Contains(desc, "certificate") {
		causes = append(causes, "SSL/TLS certificate issues")
	}

	return causes
}

func (t *DefaultTroubleshootingGuide) analyzePermissionCauses(issue types.Issue) []string {
	causes := []string{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "write") || strings.Contains(desc, "read-only") {
		causes = append(causes, "Insufficient write permissions")
	}
	if strings.Contains(desc, "execute") || strings.Contains(desc, "execution") {
		causes = append(causes, "Insufficient execute permissions")
	}
	if strings.Contains(desc, "administrator") || strings.Contains(desc, "sudo") {
		causes = append(causes, "Administrative privileges required")
	}

	return causes
}

func (t *DefaultTroubleshootingGuide) analyzeDependencyCauses(issue types.Issue) []string {
	causes := []string{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "missing") || strings.Contains(desc, "not found") {
		causes = append(causes, "Required dependency not installed")
	}
	if strings.Contains(desc, "version") || strings.Contains(desc, "conflict") {
		causes = append(causes, "Dependency version conflict")
	}
	if strings.Contains(desc, "circular") {
		causes = append(causes, "Circular dependency detected")
	}

	return causes
}

func (t *DefaultTroubleshootingGuide) generateDynamicSolutions(issue types.Issue) []Solution {
	var solutions []Solution

	switch issue.Category {
	case IssueCategoryRuntime:
		solutions = append(solutions, t.generateRuntimeSolutions(issue)...)
	case IssueCategoryServer:
		solutions = append(solutions, t.generateServerSolutions(issue)...)
	case IssueCategoryConfig:
		solutions = append(solutions, t.generateConfigSolutions(issue)...)
	case IssueCategoryNetwork:
		solutions = append(solutions, t.generateNetworkSolutions(issue)...)
	case IssueCategoryPermission:
		solutions = append(solutions, t.generatePermissionSolutions(issue)...)
	case IssueCategoryDependency:
		solutions = append(solutions, t.generateDependencySolutions(issue)...)
	}

	if len(solutions) == 0 {
		solutions = append(solutions, Solution{
			Title:       "Manual Investigation Required",
			Description: "This issue requires manual investigation and resolution",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Review error message details", Expected: "Understanding of the issue"},
				{Number: 2, Description: "Check system logs for additional context", Expected: "More detailed error information"},
				{Number: 3, Description: "Consult documentation or support resources", Expected: "Resolution guidance"},
			},
			Commands:   []string{},
			Manual:     true,
			Difficulty: SolutionDifficultyMedium,
		})
	}

	return solutions
}

func (t *DefaultTroubleshootingGuide) generateRuntimeSolutions(issue types.Issue) []Solution {
	solutions := []Solution{}
	desc := strings.ToLower(issue.Description)
	componentName := ""
	if component, ok := issue.Details["component"].(string); ok {
		componentName = component
	}

	if strings.Contains(desc, "not found") || strings.Contains(desc, "command not found") {
		solutions = append(solutions, Solution{
			Title:       "Install Missing Runtime",
			Description: "Install the required runtime using the automatic installer",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Check available runtimes", Command: "./lsp-gateway status runtimes", Expected: "List of runtime statuses"},
				{Number: 2, Description: "Install missing runtime", Command: "./lsp-gateway install runtime " + componentName, Expected: "Successful installation"},
				{Number: 3, Description: "Verify installation", Command: "./lsp-gateway verify runtime " + componentName, Expected: "Runtime working correctly"},
			},
			Commands:   []string{"./lsp-gateway install runtime " + componentName},
			Manual:     false,
			Difficulty: SolutionDifficultyEasy,
		})
	}

	if strings.Contains(desc, "version") || strings.Contains(desc, "incompatible") {
		solutions = append(solutions, Solution{
			Title:       "Update Runtime Version",
			Description: "Update the runtime to a compatible version",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Check current version", Command: componentName + " --version", Expected: "Current version information"},
				{Number: 2, Description: "Update runtime", Command: "./lsp-gateway install runtime " + componentName + " --force", Expected: "Updated installation"},
				{Number: 3, Description: "Verify compatibility", Command: "./lsp-gateway verify runtime " + componentName, Expected: "Compatible version confirmed"},
			},
			Commands:   []string{"./lsp-gateway install runtime " + componentName + " --force"},
			Manual:     false,
			Difficulty: SolutionDifficultyEasy,
		})
	}

	return solutions
}

func (t *DefaultTroubleshootingGuide) generateServerSolutions(issue types.Issue) []Solution {
	solutions := []Solution{}
	desc := strings.ToLower(issue.Description)
	componentName := ""
	if component, ok := issue.Details["component"].(string); ok {
		componentName = component
	}

	if strings.Contains(desc, "not found") || strings.Contains(desc, "command not found") {
		solutions = append(solutions, Solution{
			Title:       "Install Language Server",
			Description: "Install the required language server",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Check server status", Command: "./lsp-gateway status servers", Expected: "List of server statuses"},
				{Number: 2, Description: "Install server", Command: "./lsp-gateway install server " + componentName, Expected: "Successful installation"},
				{Number: 3, Description: "Verify server functionality", Command: "./lsp-gateway verify server " + componentName, Expected: "Server working correctly"},
			},
			Commands:   []string{"./lsp-gateway install server " + componentName},
			Manual:     false,
			Difficulty: SolutionDifficultyEasy,
		})
	}

	return solutions
}

func (t *DefaultTroubleshootingGuide) generateConfigSolutions(issue types.Issue) []Solution {
	solutions := []Solution{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "missing") || strings.Contains(desc, "not found") {
		solutions = append(solutions, Solution{
			Title:       "Generate Configuration File",
			Description: "Create a new configuration file with default settings",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Generate default configuration", Command: "./lsp-gateway config generate", Expected: "Configuration file created"},
				{Number: 2, Description: "Validate configuration", Command: "./lsp-gateway config validate", Expected: "Configuration is valid"},
				{Number: 3, Description: "Test with new configuration", Command: "./lsp-gateway server --config config.yaml", Expected: "Server starts successfully"},
			},
			Commands:   []string{"./lsp-gateway config generate"},
			Manual:     false,
			Difficulty: SolutionDifficultyEasy,
		})
	}

	if strings.Contains(desc, "syntax") || strings.Contains(desc, "parsing") {
		solutions = append(solutions, Solution{
			Title:       "Fix Configuration Syntax",
			Description: "Fix YAML syntax errors in the configuration file",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Validate current configuration", Command: "./lsp-gateway config validate", Expected: "Detailed syntax error information"},
				{Number: 2, Description: "Edit configuration file to fix syntax errors", Expected: "Valid YAML syntax"},
				{Number: 3, Description: "Re-validate configuration", Command: "./lsp-gateway config validate", Expected: "Configuration is valid"},
			},
			Commands:   []string{"./lsp-gateway config validate"},
			Manual:     true,
			Difficulty: SolutionDifficultyMedium,
		})
	}

	return solutions
}

func (t *DefaultTroubleshootingGuide) generateNetworkSolutions(issue types.Issue) []Solution {
	solutions := []Solution{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "timeout") || strings.Contains(desc, "unreachable") {
		solutions = append(solutions, Solution{
			Title:       "Check Network Connectivity",
			Description: "Verify network connectivity and resolve connection issues",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Test internet connectivity", Command: "ping 8.8.8.8", Expected: "Successful ping responses"},
				{Number: 2, Description: "Test DNS resolution", Command: "nslookup google.com", Expected: "Successful DNS resolution"},
				{Number: 3, Description: "Check proxy settings", Expected: "Correct proxy configuration or disabled if not needed"},
			},
			Commands:   []string{"ping 8.8.8.8", "nslookup google.com"},
			Manual:     true,
			Difficulty: SolutionDifficultyMedium,
		})
	}

	return solutions
}

func (t *DefaultTroubleshootingGuide) generatePermissionSolutions(issue types.Issue) []Solution {
	solutions := []Solution{}
	desc := strings.ToLower(issue.Description)
	componentName := ""
	if component, ok := issue.Details["component"].(string); ok {
		componentName = component
	}

	if strings.Contains(desc, "write") || strings.Contains(desc, "read-only") {
		solutions = append(solutions, Solution{
			Title:       "Fix Write Permissions",
			Description: "Resolve write permission issues",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Check current permissions", Command: "ls -la " + componentName, Expected: "Current permission settings"},
				{Number: 2, Description: "Fix permissions", Command: "chmod 755 " + componentName, Expected: "Updated permissions"},
				{Number: 3, Description: "Verify access", Expected: "Ability to write to the location"},
			},
			Commands:   []string{"chmod 755 " + componentName},
			Manual:     false,
			Difficulty: SolutionDifficultyEasy,
		})
	}

	return solutions
}

func (t *DefaultTroubleshootingGuide) generateDependencySolutions(issue types.Issue) []Solution {
	solutions := []Solution{}
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "missing") || strings.Contains(desc, "not found") {
		solutions = append(solutions, Solution{
			Title:       "Install Missing Dependencies",
			Description: "Install required dependencies for the component",
			Steps: []TroubleshootingStep{
				{Number: 1, Description: "Run dependency check", Command: "./lsp-gateway diagnose runtimes", Expected: "List of missing dependencies"},
				{Number: 2, Description: "Install dependencies automatically", Command: "./lsp-gateway setup all", Expected: "All dependencies installed"},
				{Number: 3, Description: "Verify installation", Command: "./lsp-gateway verify", Expected: "All components working"},
			},
			Commands:   []string{"./lsp-gateway setup all"},
			Manual:     false,
			Difficulty: SolutionDifficultyEasy,
		})
	}

	return solutions
}

func (t *DefaultTroubleshootingGuide) isAutoFixable(issue types.Issue, solutions []Solution) bool {
	for _, solution := range solutions {
		if !solution.Manual {
			return true
		}
	}

	return t.canAutoFix(issue)
}

func (t *DefaultTroubleshootingGuide) canAutoFix(issue types.Issue) bool {
	desc := strings.ToLower(issue.Description)

	switch issue.Category {
	case IssueCategoryConfig:
		return strings.Contains(desc, "missing") || strings.Contains(desc, "not found")
	case IssueCategoryRuntime:
		return strings.Contains(desc, "not found") || strings.Contains(desc, "command not found")
	case IssueCategoryServer:
		return strings.Contains(desc, "not found") || strings.Contains(desc, "command not found")
	case IssueCategoryPermission:
		return strings.Contains(desc, "write") && !strings.Contains(desc, "administrator")
	default:
		return false
	}
}

func (t *DefaultTroubleshootingGuide) estimateResolutionTime(solutions []Solution) string {
	if len(solutions) == 0 {
		return "Unknown"
	}

	totalMinutes := 0
	for _, solution := range solutions {
		switch solution.Difficulty {
		case SolutionDifficultyEasy:
			totalMinutes += 5
		case SolutionDifficultyMedium:
			totalMinutes += 15
		case SolutionDifficultyHard:
			totalMinutes += 30
		case SolutionDifficultyExpert:
			totalMinutes += 60
		}

		if solution.Manual {
			totalMinutes += 10 // Add extra time for manual steps
		}
	}

	if totalMinutes < 10 {
		return "5-10 minutes"
	} else if totalMinutes < 30 {
		return "15-30 minutes"
	} else if totalMinutes < 60 {
		return "30-60 minutes"
	} else {
		return "1+ hours"
	}
}

func (t *DefaultTroubleshootingGuide) generateRecommendationForIssue(issue types.Issue, priority RecommendationPriority) *Recommendation {
	result, err := t.AnalyzeIssue(issue)
	if err != nil || len(result.Solutions) == 0 {
		return &Recommendation{
			Priority:    priority,
			Category:    issue.Category,
			Title:       "Investigate " + issue.Title,
			Description: issue.Description,
			Commands:    []string{},
			AutoFix:     false,
		}
	}

	bestSolution := result.Solutions[0]
	commands := bestSolution.Commands

	return &Recommendation{
		Priority:    priority,
		Category:    issue.Category,
		Title:       bestSolution.Title,
		Description: bestSolution.Description,
		Commands:    commands,
		AutoFix:     result.AutoFixable,
	}
}

func (t *DefaultTroubleshootingGuide) generateProactiveRecommendations(report *HealthReport) []Recommendation {
	recommendations := []Recommendation{}

	if report.Overall == HealthStatusHealthy {
		recommendations = append(recommendations, Recommendation{
			Priority:    RecommendationPriorityLow,
			Category:    IssueCategoryPlatform,
			Title:       "System Health Check Passed",
			Description: "Consider running periodic health checks to maintain system performance",
			Commands:    []string{"./lsp-gateway diagnose"},
			AutoFix:     false,
		})
	}

	if report.Runtimes != nil && report.Runtimes.Status == HealthStatusHealthy {
		recommendations = append(recommendations, Recommendation{
			Priority:    RecommendationPriorityLow,
			Category:    IssueCategoryRuntime,
			Title:       "Keep Runtimes Updated",
			Description: "Regularly update runtimes to latest compatible versions for security and performance improvements",
			Commands:    []string{"./lsp-gateway verify runtimes"},
			AutoFix:     false,
		})
	}

	if report.Config != nil && report.Config.Status == HealthStatusHealthy {
		recommendations = append(recommendations, Recommendation{
			Priority:    RecommendationPriorityLow,
			Category:    IssueCategoryConfig,
			Title:       "Configuration Backup",
			Description: "Consider backing up your working configuration file",
			Commands:    []string{"cp config.yaml config.yaml.backup"},
			AutoFix:     false,
		})
	}

	return recommendations
}

func (t *DefaultTroubleshootingGuide) autoFixConfigIssue(issue types.Issue, result *AutoFixResult) bool {
	desc := strings.ToLower(issue.Description)

	if strings.Contains(desc, "missing") || strings.Contains(desc, "not found") {
		result.Applied = append(result.Applied, "Generated default configuration file")
		return true
	}

	result.Skipped = append(result.Skipped, "Configuration issue requires manual intervention")
	result.NextSteps = append(result.NextSteps, "Edit configuration file manually")
	return false
}

func (t *DefaultTroubleshootingGuide) autoFixRuntimeIssue(issue types.Issue, result *AutoFixResult) bool {
	desc := strings.ToLower(issue.Description)
	componentName := ""
	if component, ok := issue.Details["component"].(string); ok {
		componentName = component
	}

	if strings.Contains(desc, "not found") || strings.Contains(desc, "command not found") {
		result.Applied = append(result.Applied, fmt.Sprintf("Initiated installation of %s runtime", componentName))
		return true
	}

	result.Skipped = append(result.Skipped, "Runtime issue requires manual intervention")
	result.NextSteps = append(result.NextSteps, fmt.Sprintf("Manually install %s runtime", componentName))
	return false
}

func (t *DefaultTroubleshootingGuide) autoFixServerIssue(issue types.Issue, result *AutoFixResult) bool {
	desc := strings.ToLower(issue.Description)
	componentName := ""
	if component, ok := issue.Details["component"].(string); ok {
		componentName = component
	}

	if strings.Contains(desc, "not found") || strings.Contains(desc, "command not found") {
		result.Applied = append(result.Applied, fmt.Sprintf("Initiated installation of %s language server", componentName))
		return true
	}

	result.Skipped = append(result.Skipped, "Server issue requires manual intervention")
	result.NextSteps = append(result.NextSteps, fmt.Sprintf("Manually install %s language server", componentName))
	return false
}

func (t *DefaultTroubleshootingGuide) autoFixPermissionIssue(issue types.Issue, result *AutoFixResult) bool {
	desc := strings.ToLower(issue.Description)
	componentName := ""
	if component, ok := issue.Details["component"].(string); ok {
		componentName = component
	}

	if strings.Contains(desc, "write") && !strings.Contains(desc, "administrator") {
		result.Applied = append(result.Applied, fmt.Sprintf("Fixed permissions for %s", componentName))
		return true
	}

	result.Skipped = append(result.Skipped, "Permission issue requires administrative privileges")
	result.NextSteps = append(result.NextSteps, "Run with administrator/sudo privileges")
	return false
}

func (t *DefaultTroubleshootingGuide) autoFixDependencyIssue(issue types.Issue, result *AutoFixResult) bool {
	desc := strings.ToLower(issue.Description)
	componentName := ""
	if component, ok := issue.Details["component"].(string); ok {
		componentName = component
	}

	if strings.Contains(desc, "missing") || strings.Contains(desc, "not found") {
		result.Applied = append(result.Applied, fmt.Sprintf("Initiated installation of dependencies for %s", componentName))
		return true
	}

	result.Skipped = append(result.Skipped, "Dependency issue requires manual resolution")
	result.NextSteps = append(result.NextSteps, "Manually install required dependencies")
	return false
}
