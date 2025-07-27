package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// CLIErrorType represents the type of CLI error (local definition to avoid import cycle)
type CLIErrorType int

const (
	CLIErrorTypeConfig CLIErrorType = iota
	CLIErrorTypeInstallation
	CLIErrorTypeNetwork
	CLIErrorTypePermission
	CLIErrorTypeRuntime
	CLIErrorTypeServer
	CLIErrorTypeValidation
	CLIErrorTypeGeneral
)

// FormattedError represents an error with structured information for CLI display
type FormattedError struct {
	Type        CLIErrorType
	Message     string
	Cause       error
	Suggestions []string
	RelatedCmds []string
}

// Error implements the error interface
func (e *FormattedError) Error() string {
	if e == nil {
		return "❌ Error: unknown error (nil FormattedError)"
	}

	var parts []string

	switch e.Type {
	case CLIErrorTypeConfig:
		parts = append(parts, "⚙️  Configuration Error:")
	case CLIErrorTypeInstallation:
		parts = append(parts, "📦 Installation Error:")
	case CLIErrorTypeNetwork:
		parts = append(parts, "🌐 Network Error:")
	case CLIErrorTypePermission:
		parts = append(parts, "🔒 Permission Error:")
	case CLIErrorTypeRuntime:
		parts = append(parts, "⚡ Runtime Error:")
	case CLIErrorTypeServer:
		parts = append(parts, "🖥️  Server Error:")
	case CLIErrorTypeValidation:
		parts = append(parts, "✅ Validation Error:")
	default:
		parts = append(parts, "❌ Error:")
	}

	message := e.Message
	if message == "" {
		message = "unknown error"
	}
	parts = append(parts, message)

	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("\n   Cause: %v", e.Cause))
	}

	if len(e.Suggestions) > 0 {
		parts = append(parts, "\n\n💡 Try these solutions:")
		for i, suggestion := range e.Suggestions {
			parts = append(parts, fmt.Sprintf("   %d. %s", i+1, suggestion))
		}
	}

	if len(e.RelatedCmds) > 0 {
		parts = append(parts, "\n\n🔗 Related commands:")
		for _, cmd := range e.RelatedCmds {
			parts = append(parts, fmt.Sprintf("   lsp-gateway %s", cmd))
		}
	}

	return strings.Join(parts, " ")
}

// NotificationMode defines how notifications are presented to users
type NotificationMode int

const (
	NotificationModeInteractive NotificationMode = iota
	NotificationModeNonInteractive
	NotificationModeJSON
	NotificationModeSilent
)

// UserDecision represents a user's response to a failure notification
type UserDecision int

const (
	UserDecisionRetry UserDecision = iota
	UserDecisionBypass
	UserDecisionDebug
	UserDecisionSkip
	UserDecisionConfigure
	UserDecisionBatchBypass
	UserDecisionBatchRetry
	UserDecisionCancel
)

func (ud UserDecision) String() string {
	switch ud {
	case UserDecisionRetry:
		return "retry"
	case UserDecisionBypass:
		return "bypass"
	case UserDecisionDebug:
		return "debug"
	case UserDecisionSkip:
		return "skip"
	case UserDecisionConfigure:
		return "configure"
	case UserDecisionBatchBypass:
		return "batch_bypass"
	case UserDecisionBatchRetry:
		return "batch_retry"
	case UserDecisionCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

// FailureNotification contains all information needed to present a failure to the user
type FailureNotification struct {
	ServerName       string                   `json:"server_name"`
	Language         string                   `json:"language"`
	Category         FailureCategory          `json:"category"`
	Severity         FailureSeverity          `json:"severity"`
	Message          string                   `json:"message"`
	Cause            error                    `json:"cause,omitempty"`
	Context          *FailureContext          `json:"context,omitempty"`
	Recommendations  []RecoveryRecommendation `json:"recommendations"`
	BypassAvailable  bool                     `json:"bypass_available"`
	AutoBypassReason string                   `json:"auto_bypass_reason,omitempty"`
	Timestamp        time.Time                `json:"timestamp"`
}

// BatchFailureNotification groups multiple failures for batch processing
type BatchFailureNotification struct {
	Failures        []FailureNotification `json:"failures"`
	TotalCount      int                   `json:"total_count"`
	CriticalCount   int                   `json:"critical_count"`
	BypassableCount int                   `json:"bypassable_count"`
	Language        string                `json:"language,omitempty"`
	Timestamp       time.Time             `json:"timestamp"`
}

// UserResponse contains the user's decision and any additional parameters
type UserResponse struct {
	Decision    UserDecision `json:"decision"`
	ServerNames []string     `json:"server_names,omitempty"`
	Reason      string       `json:"reason,omitempty"`
	BatchMode   bool         `json:"batch_mode"`
}

// NotificationConfig controls notification behavior
type NotificationConfig struct {
	Mode             NotificationMode          `json:"mode"`
	AutoBypassPolicy map[FailureCategory]bool  `json:"auto_bypass_policy"`
	SeverityFilter   FailureSeverity           `json:"severity_filter"`
	BatchThreshold   int                       `json:"batch_threshold"`
	InteractivePrompts bool                    `json:"interactive_prompts"`
	ShowDebugInfo    bool                      `json:"show_debug_info"`
}

// FailureNotifier handles user notifications for LSP Gateway failures
type FailureNotifier struct {
	config *NotificationConfig
	reader *bufio.Reader
}

// NewFailureNotifier creates a new failure notifier with the given configuration
func NewFailureNotifier(config *NotificationConfig) *FailureNotifier {
	if config == nil {
		config = &NotificationConfig{
			Mode:             NotificationModeInteractive,
			AutoBypassPolicy: make(map[FailureCategory]bool),
			SeverityFilter:   FailureSeverityLow,
			BatchThreshold:   3,
			InteractivePrompts: true,
			ShowDebugInfo:    false,
		}
	}
	
	return &FailureNotifier{
		config: config,
		reader: bufio.NewReader(os.Stdin),
	}
}

// NotifyFailure presents a single failure to the user and returns their decision
func (fn *FailureNotifier) NotifyFailure(notification FailureNotification) (*UserResponse, error) {
	switch fn.config.Mode {
	case NotificationModeInteractive:
		return fn.handleInteractiveFailure(notification)
	case NotificationModeNonInteractive:
		return fn.handleNonInteractiveFailure(notification)
	case NotificationModeJSON:
		return fn.handleJSONFailure(notification)
	case NotificationModeSilent:
		return fn.handleSilentFailure(notification)
	default:
		return fn.handleInteractiveFailure(notification)
	}
}

// NotifyBatchFailures presents multiple failures to the user for batch processing
func (fn *FailureNotifier) NotifyBatchFailures(batch BatchFailureNotification) (*UserResponse, error) {
	switch fn.config.Mode {
	case NotificationModeInteractive:
		return fn.handleInteractiveBatch(batch)
	case NotificationModeNonInteractive:
		return fn.handleNonInteractiveBatch(batch)
	case NotificationModeJSON:
		return fn.handleJSONBatch(batch)
	case NotificationModeSilent:
		return fn.handleSilentBatch(batch)
	default:
		return fn.handleInteractiveBatch(batch)
	}
}

// handleInteractiveFailure presents a failure with interactive prompts
func (fn *FailureNotifier) handleInteractiveFailure(notification FailureNotification) (*UserResponse, error) {
	// Create formatted error for consistent formatting
	formattedError := fn.createFormattedError(notification)
	
	// Display the formatted error
	fmt.Fprintf(os.Stderr, "%s\n", formattedError.Error())
	
	if !fn.config.InteractivePrompts {
		return fn.handleNonInteractiveFailure(notification)
	}
	
	// Show available options
	fmt.Fprintf(os.Stderr, "\n🎯 Available Actions:\n")
	fmt.Fprintf(os.Stderr, "   [1] Retry - Attempt to restart the server\n")
	
	if notification.BypassAvailable {
		fmt.Fprintf(os.Stderr, "   [2] Bypass - Skip this server and continue\n")
	}
	
	fmt.Fprintf(os.Stderr, "   [3] Debug - Show detailed diagnostic information\n")
	fmt.Fprintf(os.Stderr, "   [4] Configure - Generate new configuration\n")
	fmt.Fprintf(os.Stderr, "   [5] Skip - Skip this server for now\n")
	fmt.Fprintf(os.Stderr, "   [0] Cancel - Exit\n")
	
	fmt.Fprintf(os.Stderr, "\nSelect an action [1]: ")
	
	// Read user input
	input, err := fn.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read user input: %w", err)
	}
	
	input = strings.TrimSpace(input)
	if input == "" {
		input = "1" // Default to retry
	}
	
	choice, err := strconv.Atoi(input)
	if err != nil {
		return &UserResponse{Decision: UserDecisionRetry}, nil
	}
	
	response := &UserResponse{
		ServerNames: []string{notification.ServerName},
		BatchMode:   false,
	}
	
	switch choice {
	case 1:
		response.Decision = UserDecisionRetry
	case 2:
		if notification.BypassAvailable {
			response.Decision = UserDecisionBypass
			response.Reason = fn.promptBypassReason()
		} else {
			response.Decision = UserDecisionRetry
		}
	case 3:
		response.Decision = UserDecisionDebug
	case 4:
		response.Decision = UserDecisionConfigure
	case 5:
		response.Decision = UserDecisionSkip
	case 0:
		response.Decision = UserDecisionCancel
	default:
		response.Decision = UserDecisionRetry
	}
	
	return response, nil
}

// handleInteractiveBatch presents multiple failures for batch processing
func (fn *FailureNotifier) handleInteractiveBatch(batch BatchFailureNotification) (*UserResponse, error) {
	fmt.Fprintf(os.Stderr, "🚨 Multiple Server Failures Detected\n\n")
	fmt.Fprintf(os.Stderr, "📊 Summary:\n")
	fmt.Fprintf(os.Stderr, "   Total Failures: %d\n", batch.TotalCount)
	fmt.Fprintf(os.Stderr, "   Critical: %d\n", batch.CriticalCount)
	fmt.Fprintf(os.Stderr, "   Bypassable: %d\n", batch.BypassableCount)
	
	if batch.Language != "" {
		fmt.Fprintf(os.Stderr, "   Language: %s\n", batch.Language)
	}
	
	fmt.Fprintf(os.Stderr, "\n📋 Failed Servers:\n")
	for i, failure := range batch.Failures {
		icon := fn.getSeverityIcon(failure.Severity)
		fmt.Fprintf(os.Stderr, "   %d. %s %s (%s): %s\n", 
			i+1, icon, failure.ServerName, failure.Category.String(), failure.Message)
	}
	
	if !fn.config.InteractivePrompts {
		return fn.handleNonInteractiveBatch(batch)
	}
	
	fmt.Fprintf(os.Stderr, "\n🎯 Batch Actions:\n")
	fmt.Fprintf(os.Stderr, "   [1] Retry All - Attempt to restart all servers\n")
	
	if batch.BypassableCount > 0 {
		fmt.Fprintf(os.Stderr, "   [2] Bypass All - Skip all failed servers\n")
		fmt.Fprintf(os.Stderr, "   [3] Bypass Critical - Skip only critical failures\n")
		fmt.Fprintf(os.Stderr, "   [4] Select Individual - Choose which servers to bypass\n")
	}
	
	fmt.Fprintf(os.Stderr, "   [5] Configure - Generate new configuration\n")
	fmt.Fprintf(os.Stderr, "   [0] Cancel - Exit\n")
	
	fmt.Fprintf(os.Stderr, "\nSelect a batch action [1]: ")
	
	input, err := fn.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read user input: %w", err)
	}
	
	input = strings.TrimSpace(input)
	if input == "" {
		input = "1"
	}
	
	choice, err := strconv.Atoi(input)
	if err != nil {
		return &UserResponse{Decision: UserDecisionBatchRetry, BatchMode: true}, nil
	}
	
	response := &UserResponse{BatchMode: true}
	
	switch choice {
	case 1:
		response.Decision = UserDecisionBatchRetry
		response.ServerNames = fn.extractServerNames(batch.Failures)
	case 2:
		if batch.BypassableCount > 0 {
			response.Decision = UserDecisionBatchBypass
			response.ServerNames = fn.extractBypassableServerNames(batch.Failures)
			response.Reason = fn.promptBypassReason()
		} else {
			response.Decision = UserDecisionBatchRetry
			response.ServerNames = fn.extractServerNames(batch.Failures)
		}
	case 3:
		response.Decision = UserDecisionBatchBypass
		response.ServerNames = fn.extractCriticalServerNames(batch.Failures)
		response.Reason = fn.promptBypassReason()
	case 4:
		return fn.handleIndividualSelection(batch)
	case 5:
		response.Decision = UserDecisionConfigure
	case 0:
		response.Decision = UserDecisionCancel
	default:
		response.Decision = UserDecisionBatchRetry
		response.ServerNames = fn.extractServerNames(batch.Failures)
	}
	
	return response, nil
}

// handleNonInteractiveFailure handles failures without user interaction
func (fn *FailureNotifier) handleNonInteractiveFailure(notification FailureNotification) (*UserResponse, error) {
	// Create formatted error for logging
	formattedError := fn.createFormattedError(notification)
	fmt.Fprintf(os.Stderr, "%s\n", formattedError.Error())
	
	// Apply auto-bypass policy
	if fn.shouldAutoBypass(notification) {
		return &UserResponse{
			Decision:    UserDecisionBypass,
			ServerNames: []string{notification.ServerName},
			Reason:      notification.AutoBypassReason,
			BatchMode:   false,
		}, nil
	}
	
	// Default to retry for non-critical failures
	if notification.Severity != FailureSeverityCritical {
		return &UserResponse{
			Decision:    UserDecisionRetry,
			ServerNames: []string{notification.ServerName},
			BatchMode:   false,
		}, nil
	}
	
	// Skip critical failures in non-interactive mode
	return &UserResponse{
		Decision:    UserDecisionSkip,
		ServerNames: []string{notification.ServerName},
		BatchMode:   false,
	}, nil
}

// handleNonInteractiveBatch handles batch failures without user interaction
func (fn *FailureNotifier) handleNonInteractiveBatch(batch BatchFailureNotification) (*UserResponse, error) {
	fmt.Fprintf(os.Stderr, "🚨 Batch failure detected: %d servers failed\n", batch.TotalCount)
	
	// Apply auto-bypass to eligible servers
	bypassableServers := []string{}
	retryableServers := []string{}
	
	for _, failure := range batch.Failures {
		if fn.shouldAutoBypass(failure) {
			bypassableServers = append(bypassableServers, failure.ServerName)
		} else if failure.Severity != FailureSeverityCritical {
			retryableServers = append(retryableServers, failure.ServerName)
		}
	}
	
	if len(bypassableServers) > 0 {
		return &UserResponse{
			Decision:    UserDecisionBatchBypass,
			ServerNames: bypassableServers,
			Reason:      "auto-bypass policy",
			BatchMode:   true,
		}, nil
	}
	
	if len(retryableServers) > 0 {
		return &UserResponse{
			Decision:    UserDecisionBatchRetry,
			ServerNames: retryableServers,
			BatchMode:   true,
		}, nil
	}
	
	return &UserResponse{
		Decision:  UserDecisionSkip,
		BatchMode: true,
	}, nil
}

// handleJSONFailure outputs failure information in JSON format
func (fn *FailureNotifier) handleJSONFailure(notification FailureNotification) (*UserResponse, error) {
	output := map[string]interface{}{
		"type":         "failure_notification",
		"notification": notification,
		"timestamp":    time.Now().UTC(),
	}
	
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal notification: %w", err)
	}
	
	fmt.Fprintf(os.Stdout, "%s\n", jsonData)
	
	// Return default action for JSON mode
	return &UserResponse{
		Decision:    UserDecisionRetry,
		ServerNames: []string{notification.ServerName},
		BatchMode:   false,
	}, nil
}

// handleJSONBatch outputs batch failure information in JSON format
func (fn *FailureNotifier) handleJSONBatch(batch BatchFailureNotification) (*UserResponse, error) {
	output := map[string]interface{}{
		"type":  "batch_failure_notification",
		"batch": batch,
		"timestamp": time.Now().UTC(),
	}
	
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch notification: %w", err)
	}
	
	fmt.Fprintf(os.Stdout, "%s\n", jsonData)
	
	return &UserResponse{
		Decision:    UserDecisionBatchRetry,
		ServerNames: fn.extractServerNames(batch.Failures),
		BatchMode:   true,
	}, nil
}

// handleSilentFailure handles failures in silent mode (logging only)
func (fn *FailureNotifier) handleSilentFailure(notification FailureNotification) (*UserResponse, error) {
	// Apply auto-bypass policy silently
	if fn.shouldAutoBypass(notification) {
		return &UserResponse{
			Decision:    UserDecisionBypass,
			ServerNames: []string{notification.ServerName},
			Reason:      notification.AutoBypassReason,
			BatchMode:   false,
		}, nil
	}
	
	return &UserResponse{
		Decision:    UserDecisionRetry,
		ServerNames: []string{notification.ServerName},
		BatchMode:   false,
	}, nil
}

// handleSilentBatch handles batch failures in silent mode
func (fn *FailureNotifier) handleSilentBatch(batch BatchFailureNotification) (*UserResponse, error) {
	bypassableServers := []string{}
	
	for _, failure := range batch.Failures {
		if fn.shouldAutoBypass(failure) {
			bypassableServers = append(bypassableServers, failure.ServerName)
		}
	}
	
	if len(bypassableServers) > 0 {
		return &UserResponse{
			Decision:    UserDecisionBatchBypass,
			ServerNames: bypassableServers,
			Reason:      "silent auto-bypass",
			BatchMode:   true,
		}, nil
	}
	
	return &UserResponse{
		Decision:    UserDecisionBatchRetry,
		ServerNames: fn.extractServerNames(batch.Failures),
		BatchMode:   true,
	}, nil
}

// createFormattedError converts a failure notification to a formatted error for consistent formatting
func (fn *FailureNotifier) createFormattedError(notification FailureNotification) *FormattedError {
	errorType := fn.mapFailureCategoryToErrorType(notification.Category)
	
	suggestions := []string{}
	relatedCmds := []string{}
	
	// Add recommendations as suggestions
	for _, rec := range notification.Recommendations {
		suggestions = append(suggestions, rec.Description)
		if len(rec.Commands) > 0 {
			relatedCmds = append(relatedCmds, rec.Commands...)
		}
	}
	
	// Add category-specific suggestions
	switch notification.Category {
	case FailureCategoryStartup:
		suggestions = append(suggestions, []string{
			"Check server installation: lsp-gateway status servers",
			"Verify runtime availability: lsp-gateway status runtimes",
			"Run diagnostics: lsp-gateway diagnose",
		}...)
		relatedCmds = append(relatedCmds, []string{
			"status servers",
			"status runtimes", 
			"diagnose",
			"install server " + notification.ServerName,
		}...)
		
	case FailureCategoryConfiguration:
		suggestions = append(suggestions, []string{
			"Validate configuration: lsp-gateway config validate",
			"Regenerate configuration: lsp-gateway config generate",
			"Run complete setup: lsp-gateway setup all",
		}...)
		relatedCmds = append(relatedCmds, []string{
			"config validate",
			"config generate",
			"setup all",
		}...)
		
	case FailureCategoryTransport:
		suggestions = append(suggestions, []string{
			"Check network connectivity",
			"Verify port availability: lsof -i :8080",
			"Try different transport method",
		}...)
		relatedCmds = append(relatedCmds, []string{
			"server --port <port>",
			"diagnose",
		}...)
		
	case FailureCategoryResource:
		suggestions = append(suggestions, []string{
			"Check system resources: htop or Activity Monitor",
			"Close unnecessary applications",
			"Increase system memory limits",
		}...)
		relatedCmds = append(relatedCmds, []string{
			"diagnose",
			"status",
		}...)
	}
	
	// Add bypass suggestion if available
	if notification.BypassAvailable {
		suggestions = append(suggestions, "Consider bypassing this server if issues persist")
	}
	
	message := fmt.Sprintf("%s server '%s' failed: %s", 
		strings.Title(notification.Category.String()), 
		notification.ServerName, 
		notification.Message)
	
	if notification.Language != "" {
		message = fmt.Sprintf("%s (language: %s)", message, notification.Language)
	}
	
	return &FormattedError{
		Type:        errorType,
		Message:     message,
		Cause:       notification.Cause,
		Suggestions: suggestions,
		RelatedCmds: relatedCmds,
	}
}

// mapFailureCategoryToErrorType maps failure categories to error types
func (fn *FailureNotifier) mapFailureCategoryToErrorType(category FailureCategory) CLIErrorType {
	switch category {
	case FailureCategoryStartup:
		return CLIErrorTypeServer
	case FailureCategoryRuntime:
		return CLIErrorTypeRuntime
	case FailureCategoryConfiguration:
		return CLIErrorTypeConfig
	case FailureCategoryTransport:
		return CLIErrorTypeNetwork
	case FailureCategoryResource:
		return CLIErrorTypeRuntime
	default:
		return CLIErrorTypeGeneral
	}
}

// getSeverityIcon returns an appropriate icon for the failure severity
func (fn *FailureNotifier) getSeverityIcon(severity FailureSeverity) string {
	switch severity {
	case FailureSeverityLow:
		return "⚠️"
	case FailureSeverityMedium:
		return "🟡"
	case FailureSeverityHigh:
		return "🟠"
	case FailureSeverityCritical:
		return "🔴"
	default:
		return "❓"
	}
}

// shouldAutoBypass determines if a failure should be automatically bypassed
func (fn *FailureNotifier) shouldAutoBypass(notification FailureNotification) bool {
	if !notification.BypassAvailable {
		return false
	}
	
	// Check auto-bypass policy
	if autoBypass, exists := fn.config.AutoBypassPolicy[notification.Category]; exists && autoBypass {
		return true
	}
	
	// Auto-bypass critical failures if configured
	if notification.Severity == FailureSeverityCritical && notification.AutoBypassReason != "" {
		return true
	}
	
	return false
}

// promptBypassReason prompts the user for a bypass reason
func (fn *FailureNotifier) promptBypassReason() string {
	fmt.Fprintf(os.Stderr, "\nEnter bypass reason (optional): ")
	reason, err := fn.reader.ReadString('\n')
	if err != nil {
		return "User bypass decision"
	}
	
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "User bypass decision"
	}
	
	return reason
}

// extractServerNames extracts all server names from failures
func (fn *FailureNotifier) extractServerNames(failures []FailureNotification) []string {
	names := make([]string, len(failures))
	for i, failure := range failures {
		names[i] = failure.ServerName
	}
	return names
}

// extractBypassableServerNames extracts server names for bypassable failures
func (fn *FailureNotifier) extractBypassableServerNames(failures []FailureNotification) []string {
	var names []string
	for _, failure := range failures {
		if failure.BypassAvailable {
			names = append(names, failure.ServerName)
		}
	}
	return names
}

// extractCriticalServerNames extracts server names for critical failures
func (fn *FailureNotifier) extractCriticalServerNames(failures []FailureNotification) []string {
	var names []string
	for _, failure := range failures {
		if failure.Severity == FailureSeverityCritical && failure.BypassAvailable {
			names = append(names, failure.ServerName)
		}
	}
	return names
}

// handleIndividualSelection allows users to select individual servers for bypass
func (fn *FailureNotifier) handleIndividualSelection(batch BatchFailureNotification) (*UserResponse, error) {
	fmt.Fprintf(os.Stderr, "\n🎯 Individual Server Selection:\n")
	
	bypassableFailures := []FailureNotification{}
	for _, failure := range batch.Failures {
		if failure.BypassAvailable {
			bypassableFailures = append(bypassableFailures, failure)
		}
	}
	
	if len(bypassableFailures) == 0 {
		fmt.Fprintf(os.Stderr, "No servers are bypassable. Returning to batch options.\n")
		return fn.handleInteractiveBatch(batch)
	}
	
	selectedServers := []string{}
	
	for i, failure := range bypassableFailures {
		icon := fn.getSeverityIcon(failure.Severity)
		fmt.Fprintf(os.Stderr, "\n%d. %s %s (%s)\n", 
			i+1, icon, failure.ServerName, failure.Category.String())
		fmt.Fprintf(os.Stderr, "   %s\n", failure.Message)
		fmt.Fprintf(os.Stderr, "   Bypass this server? [y/N]: ")
		
		input, err := fn.reader.ReadString('\n')
		if err != nil {
			continue
		}
		
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
			selectedServers = append(selectedServers, failure.ServerName)
		}
	}
	
	if len(selectedServers) == 0 {
		return &UserResponse{
			Decision:  UserDecisionRetry,
			BatchMode: true,
		}, nil
	}
	
	return &UserResponse{
		Decision:    UserDecisionBatchBypass,
		ServerNames: selectedServers,
		Reason:      fn.promptBypassReason(),
		BatchMode:   true,
	}, nil
}

// CreateFailureNotification creates a notification from failure information
func CreateFailureNotification(serverName, language string, category FailureCategory, 
	severity FailureSeverity, message string, cause error, 
	recommendations []RecoveryRecommendation) FailureNotification {
	
	return FailureNotification{
		ServerName:      serverName,
		Language:        language,
		Category:        category,
		Severity:        severity,
		Message:         message,
		Cause:           cause,
		Recommendations: recommendations,
		BypassAvailable: true,
		Timestamp:       time.Now(),
	}
}

// CreateBatchFailureNotification creates a batch notification from multiple failures
func CreateBatchFailureNotification(failures []FailureNotification, language string) BatchFailureNotification {
	criticalCount := 0
	bypassableCount := 0
	
	for _, failure := range failures {
		if failure.Severity == FailureSeverityCritical {
			criticalCount++
		}
		if failure.BypassAvailable {
			bypassableCount++
		}
	}
	
	return BatchFailureNotification{
		Failures:        failures,
		TotalCount:      len(failures),
		CriticalCount:   criticalCount,
		BypassableCount: bypassableCount,
		Language:        language,
		Timestamp:       time.Now(),
	}
}