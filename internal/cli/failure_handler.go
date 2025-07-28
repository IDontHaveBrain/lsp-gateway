package cli

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"lsp-gateway/internal/gateway"
)

// FailureHandlerConfig configures failure handling behavior
type FailureHandlerConfig struct {
	Interactive         bool                                      `yaml:"interactive"`
	AutoBypassPolicies  map[gateway.FailureCategory]bool         `yaml:"auto_bypass_policies"`
	SeverityFilter     gateway.FailureSeverity                  `yaml:"severity_filter"`
	BatchThreshold     int                                       `yaml:"batch_threshold"`
	MaxRetryAttempts   int                                       `yaml:"max_retry_attempts"`
	RetryDelay         time.Duration                             `yaml:"retry_delay"`
	LogFailures        bool                                      `yaml:"log_failures"`
	LogLevel           string                                    `yaml:"log_level"`
}

// DefaultFailureHandlerConfig returns default failure handler configuration
func DefaultFailureHandlerConfig() *FailureHandlerConfig {
	return &FailureHandlerConfig{
		Interactive: true,
		AutoBypassPolicies: map[gateway.FailureCategory]bool{
			gateway.FailureCategoryStartup:       false,
			gateway.FailureCategoryRuntime:       false,
			gateway.FailureCategoryConfiguration: false,
			gateway.FailureCategoryTransport:     true,  // Auto-bypass transport failures
			gateway.FailureCategoryResource:      false,
		},
		SeverityFilter:   gateway.FailureSeverityLow,
		BatchThreshold:   3,
		MaxRetryAttempts: 3,
		RetryDelay:       2 * time.Second,
		LogFailures:      true,
		LogLevel:         "warn",
	}
}

// CLIFailureHandler handles LSP server failures in CLI context
type CLIFailureHandler struct {
	config        *FailureHandlerConfig
	notifier      *gateway.FailureNotifier
	logger        *log.Logger
	retryAttempts map[string]int  // Track retry attempts per server
	startTime     time.Time
}

// NewCLIFailureHandler creates a new CLI failure handler
func NewCLIFailureHandler(config *FailureHandlerConfig, logger *log.Logger) *CLIFailureHandler {
	if config == nil {
		config = DefaultFailureHandlerConfig()
	}

	// Configure notification based on CLI preferences
	notificationConfig := &gateway.NotificationConfig{
		Mode:               gateway.NotificationModeInteractive,
		AutoBypassPolicy:   config.AutoBypassPolicies,
		SeverityFilter:     config.SeverityFilter,
		BatchThreshold:     config.BatchThreshold,
		InteractivePrompts: config.Interactive,
		ShowDebugInfo:      false,
	}

	// Adjust notification mode based on environment
	if !config.Interactive || !isTerminalInteractive() {
		notificationConfig.Mode = gateway.NotificationModeNonInteractive
		notificationConfig.InteractivePrompts = false
	}

	// Use JSON mode if requested via environment
	if os.Getenv("LSP_GATEWAY_JSON_OUTPUT") == "true" {
		notificationConfig.Mode = gateway.NotificationModeJSON
	}

	return &CLIFailureHandler{
		config:        config,
		notifier:      gateway.NewFailureNotifier(notificationConfig),
		logger:        logger,
		retryAttempts: make(map[string]int),
		startTime:     time.Now(),
	}
}

// HandleServerStartupFailure handles failures during server startup
func (fh *CLIFailureHandler) HandleServerStartupFailure(
	serverName, language string, 
	err error, 
	recommendations []gateway.RecoveryRecommendation) error {

	if err == nil {
		return nil
	}

	// Create failure notification
	notification := gateway.CreateFailureNotification(
		serverName,
		language,
		gateway.FailureCategoryStartup,
		fh.determineSeverity(err),
		fmt.Sprintf("Failed to start LSP server: %s", err.Error()),
		err,
		recommendations,
	)

	// Add startup-specific context
	notification.Context = &gateway.FailureContext{
		ProjectPath: getCurrentProjectPath(),
		Metadata: map[string]string{
			"config_file":  getCurrentConfigFile(),
			"startup_time": time.Since(fh.startTime).String(),
		},
	}

	return fh.handleSingleFailure(notification)
}

// HandleRuntimeFailure handles failures during server runtime
func (fh *CLIFailureHandler) HandleRuntimeFailure(
	serverName, language string, 
	err error, 
	context map[string]interface{}) error {

	if err == nil {
		return nil
	}

	// Determine if this is a transport or runtime failure
	category := gateway.FailureCategoryRuntime
	if isTransportError(err) {
		category = gateway.FailureCategoryTransport
	}

	notification := gateway.CreateFailureNotification(
		serverName,
		language,
		category,
		fh.determineSeverity(err),
		fmt.Sprintf("Runtime failure: %s", err.Error()),
		err,
		fh.generateRuntimeRecommendations(category),
	)

	// Add runtime context
	if context != nil {
		notification.Context = &gateway.FailureContext{
			ServerName:    serverName,
			Language:      language,
			RequestMethod: getStringFromContext(context, "method"),
			Metadata: map[string]string{
				"request_uri": getStringFromContext(context, "uri"),
				"error_time":  time.Now().Format(time.RFC3339),
			},
		}
	}

	return fh.handleSingleFailure(notification)
}

// HandleConfigurationFailure handles configuration-related failures
func (fh *CLIFailureHandler) HandleConfigurationFailure(
	serverName, language string, 
	configError error) error {

	if configError == nil {
		return nil
	}

	notification := gateway.CreateFailureNotification(
		serverName,
		language,
		gateway.FailureCategoryConfiguration,
		gateway.FailureSeverityHigh,
		fmt.Sprintf("Configuration error: %s", configError.Error()),
		configError,
		fh.generateConfigurationRecommendations(),
	)

	notification.Context = &gateway.FailureContext{
		ServerName:  serverName,
		Language:    language,
		ProjectPath: getCurrentProjectPath(),
		Metadata: map[string]string{
			"config_file": getCurrentConfigFile(),
			"error_time":  time.Now().Format(time.RFC3339),
		},
	}

	return fh.handleSingleFailure(notification)
}

// HandleBatchFailures handles multiple failures at once
func (fh *CLIFailureHandler) HandleBatchFailures(failures []gateway.FailureNotification) error {
	if len(failures) == 0 {
		return nil
	}

	// Filter failures based on severity
	filteredFailures := fh.filterFailuresBySeverity(failures)
	if len(filteredFailures) == 0 {
		return nil
	}

	// Check if we should handle as batch
	if len(filteredFailures) >= fh.config.BatchThreshold {
		return fh.handleBatchFailures(filteredFailures)
	}

	// Handle individually if below batch threshold
	for _, failure := range filteredFailures {
		if err := fh.handleSingleFailure(failure); err != nil {
			return err
		}
	}

	return nil
}

// HandleGatewayStartupFailures handles failures during gateway startup
func (fh *CLIFailureHandler) HandleGatewayStartupFailures(failures []gateway.FailureNotification) error {
	if len(failures) == 0 {
		return nil
	}

	fh.logger.Printf("Gateway startup encountered %d server failures", len(failures))

	// Categorize failures
	criticalFailures := []gateway.FailureNotification{}
	bypassableFailures := []gateway.FailureNotification{}

	for _, failure := range failures {
		if failure.Severity == gateway.FailureSeverityCritical {
			criticalFailures = append(criticalFailures, failure)
		} else if failure.BypassAvailable {
			bypassableFailures = append(bypassableFailures, failure)
		}
	}

	// Handle critical failures first
	if len(criticalFailures) > 0 {
		fmt.Fprintf(os.Stderr, "\n🚨 Critical server failures detected during startup:\n")
		for _, failure := range criticalFailures {
			fmt.Fprintf(os.Stderr, "   • %s (%s): %s\n", 
				failure.ServerName, failure.Language, failure.Message)
		}

		if !fh.config.Interactive {
			return NewGatewayStartupError(fmt.Errorf("%d critical server failures", len(criticalFailures)))
		}

		fmt.Fprintf(os.Stderr, "\nGateway cannot start with critical failures. Options:\n")
		fmt.Fprintf(os.Stderr, "   1. Fix the issues and restart\n")
		fmt.Fprintf(os.Stderr, "   2. Continue without critical servers (not recommended)\n")
		fmt.Fprintf(os.Stderr, "   3. Exit and run diagnostics\n")

		// For now, exit on critical failures during startup
		return NewGatewayStartupError(fmt.Errorf("cannot start with %d critical server failures", len(criticalFailures)))
	}

	// Handle bypassable failures
	if len(bypassableFailures) > 0 {
		return fh.HandleBatchFailures(bypassableFailures)
	}

	return nil
}

// SetNonInteractiveMode configures the handler for non-interactive operation
func (fh *CLIFailureHandler) SetNonInteractiveMode(autoBypassAll bool) {
	fh.config.Interactive = false
	
	notificationConfig := &gateway.NotificationConfig{
		Mode:               gateway.NotificationModeNonInteractive,
		AutoBypassPolicy:   fh.config.AutoBypassPolicies,
		SeverityFilter:     fh.config.SeverityFilter,
		BatchThreshold:     fh.config.BatchThreshold,
		InteractivePrompts: false,
		ShowDebugInfo:      false,
	}

	if autoBypassAll {
		// Enable auto-bypass for all categories in non-interactive mode
		for category := range notificationConfig.AutoBypassPolicy {
			notificationConfig.AutoBypassPolicy[category] = true
		}
	}

	fh.notifier = gateway.NewFailureNotifier(notificationConfig)
}

// SetJSONMode configures the handler for JSON output
func (fh *CLIFailureHandler) SetJSONMode() {
	notificationConfig := &gateway.NotificationConfig{
		Mode:               gateway.NotificationModeJSON,
		AutoBypassPolicy:   fh.config.AutoBypassPolicies,
		SeverityFilter:     fh.config.SeverityFilter,
		BatchThreshold:     fh.config.BatchThreshold,
		InteractivePrompts: false,
		ShowDebugInfo:      true,
	}

	fh.notifier = gateway.NewFailureNotifier(notificationConfig)
}

// Private helper methods

func (fh *CLIFailureHandler) handleSingleFailure(notification gateway.FailureNotification) error {
	// Log the failure if configured
	if fh.config.LogFailures {
		fh.logFailure(notification)
	}

	// Check retry attempts
	serverKey := fmt.Sprintf("%s-%s", notification.ServerName, notification.Language)
	attempts := fh.retryAttempts[serverKey]

	if attempts >= fh.config.MaxRetryAttempts {
		// Force bypass after max retries
		notification.AutoBypassReason = fmt.Sprintf("Max retry attempts (%d) exceeded", fh.config.MaxRetryAttempts)
		notification.BypassAvailable = true
	}

	// Present notification to user
	response, err := fh.notifier.NotifyFailure(notification)
	if err != nil {
		return fmt.Errorf("failed to handle failure notification: %w", err)
	}

	// Execute user decision
	return fh.executeUserResponse(response, []gateway.FailureNotification{notification})
}

func (fh *CLIFailureHandler) handleBatchFailures(failures []gateway.FailureNotification) error {
	// Log batch failure
	if fh.config.LogFailures {
		fh.logger.Printf("Batch failure: %d servers failed", len(failures))
		for _, failure := range failures {
			fh.logFailure(failure)
		}
	}

	// Create batch notification
	batchNotification := gateway.CreateBatchFailureNotification(failures, extractCommonLanguage(failures))

	// Present batch notification to user
	response, err := fh.notifier.NotifyBatchFailures(batchNotification)
	if err != nil {
		return fmt.Errorf("failed to handle batch failure notification: %w", err)
	}

	// Execute user decision
	return fh.executeUserResponse(response, failures)
}

func (fh *CLIFailureHandler) executeUserResponse(response *gateway.UserResponse, failures []gateway.FailureNotification) error {
	switch response.Decision {
	case gateway.UserDecisionRetry, gateway.UserDecisionBatchRetry:
		return fh.executeRetry(response.ServerNames, failures)
		
	case gateway.UserDecisionBypass, gateway.UserDecisionBatchBypass:
		return fh.executeBypass(response.ServerNames, response.Reason, failures)
		
	case gateway.UserDecisionDebug:
		return fh.executeDebug(response.ServerNames, failures)
		
	case gateway.UserDecisionConfigure:
		return fh.executeConfigure()
		
	case gateway.UserDecisionSkip:
		return fh.executeSkip(response.ServerNames)
		
	case gateway.UserDecisionCancel:
		return fmt.Errorf("operation cancelled by user")
		
	default:
		fh.logger.Printf("Unknown user decision: %s, defaulting to skip", response.Decision.String())
		return fh.executeSkip(response.ServerNames)
	}
}

func (fh *CLIFailureHandler) executeRetry(serverNames []string, failures []gateway.FailureNotification) error {
	fmt.Fprintf(os.Stderr, "🔄 Retrying %d server(s)...\n", len(serverNames))

	for _, serverName := range serverNames {
		// Increment retry count
		serverKey := fmt.Sprintf("%s-%s", serverName, extractLanguageForServer(serverName, failures))
		fh.retryAttempts[serverKey]++

		fmt.Fprintf(os.Stderr, "   Retrying %s (attempt %d/%d)\n", 
			serverName, fh.retryAttempts[serverKey], fh.config.MaxRetryAttempts)
		
		// Add delay between retries
		if fh.config.RetryDelay > 0 {
			time.Sleep(fh.config.RetryDelay)
		}
	}

	// Note: Actual server restart logic would be implemented by the calling code
	// This method just handles the CLI interaction and tracking
	return nil
}

func (fh *CLIFailureHandler) executeBypass(serverNames []string, reason string, failures []gateway.FailureNotification) error {
	fmt.Fprintf(os.Stderr, "🔀 Bypassing %d server(s): %s\n", len(serverNames), reason)

	for _, serverName := range serverNames {
		fmt.Fprintf(os.Stderr, "   Bypassed: %s\n", serverName)
		
		// Reset retry count for bypassed servers
		serverKey := fmt.Sprintf("%s-%s", serverName, extractLanguageForServer(serverName, failures))
		delete(fh.retryAttempts, serverKey)
	}

	// Note: Actual bypass logic would be implemented by the calling code
	return nil
}

func (fh *CLIFailureHandler) executeDebug(serverNames []string, failures []gateway.FailureNotification) error {
	fmt.Fprintf(os.Stderr, "🔍 Debug information for %d server(s):\n\n", len(serverNames))

	for _, serverName := range serverNames {
		failure := findFailureByServerName(serverName, failures)
		if failure == nil {
			continue
		}

		fmt.Fprintf(os.Stderr, "📋 Server: %s (%s)\n", failure.ServerName, failure.Language)
		fmt.Fprintf(os.Stderr, "   Category: %s\n", failure.Category.String())
		fmt.Fprintf(os.Stderr, "   Severity: %s\n", failure.Severity.String())
		fmt.Fprintf(os.Stderr, "   Message: %s\n", failure.Message)
		if failure.Cause != nil {
			fmt.Fprintf(os.Stderr, "   Error: %s\n", failure.Cause.Error())
		}

		if failure.Context != nil {
			fmt.Fprintf(os.Stderr, "   Context:\n")
			if failure.Context.ProjectPath != "" {
				fmt.Fprintf(os.Stderr, "     Project: %s\n", failure.Context.ProjectPath)
			}
			if configFile, ok := failure.Context.Metadata["config_file"]; ok && configFile != "" {
				fmt.Fprintf(os.Stderr, "     Config: %s\n", configFile)
			}
			if failure.Context.RequestMethod != "" {
				fmt.Fprintf(os.Stderr, "     Method: %s\n", failure.Context.RequestMethod)
			}
		}

		fmt.Fprintf(os.Stderr, "   Recommendations:\n")
		for i, rec := range failure.Recommendations {
			fmt.Fprintf(os.Stderr, "     %d. %s\n", i+1, rec.Description)
			for _, cmd := range rec.Commands {
				fmt.Fprintf(os.Stderr, "        Command: lspg %s\n", cmd)
			}
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	return nil
}

func (fh *CLIFailureHandler) executeConfigure() error {
	fmt.Fprintf(os.Stderr, "⚙️  Launching configuration wizard...\n")
	
	// Note: This would trigger the configuration generation process
	// For now, just provide guidance
	fmt.Fprintf(os.Stderr, "\n💡 Configuration suggestions:\n")
	fmt.Fprintf(os.Stderr, "   1. Run: lspg config generate --auto-detect\n")
	fmt.Fprintf(os.Stderr, "   2. Run: lspg setup all\n")
	fmt.Fprintf(os.Stderr, "   3. Run: lspg diagnose\n")
	
	return nil
}

func (fh *CLIFailureHandler) executeSkip(serverNames []string) error {
	fmt.Fprintf(os.Stderr, "⏭️  Skipping %d server(s)\n", len(serverNames))
	
	for _, serverName := range serverNames {
		fmt.Fprintf(os.Stderr, "   Skipped: %s\n", serverName)
	}
	
	return nil
}

func (fh *CLIFailureHandler) determineSeverity(err error) gateway.FailureSeverity {
	if err == nil {
		return gateway.FailureSeverityLow
	}

	errStr := strings.ToLower(err.Error())
	
	// Critical indicators
	if strings.Contains(errStr, "panic") || 
	   strings.Contains(errStr, "fatal") ||
	   strings.Contains(errStr, "segmentation fault") ||
	   strings.Contains(errStr, "out of memory") {
		return gateway.FailureSeverityCritical
	}

	// High severity indicators
	if strings.Contains(errStr, "connection refused") ||
	   strings.Contains(errStr, "permission denied") ||
	   strings.Contains(errStr, "no such file") ||
	   strings.Contains(errStr, "timeout") {
		return gateway.FailureSeverityHigh
	}

	// Medium severity indicators
	if strings.Contains(errStr, "broken pipe") ||
	   strings.Contains(errStr, "connection reset") ||
	   strings.Contains(errStr, "invalid") {
		return gateway.FailureSeverityMedium
	}

	return gateway.FailureSeverityLow
}

func (fh *CLIFailureHandler) generateRuntimeRecommendations(category gateway.FailureCategory) []gateway.RecoveryRecommendation {
	switch category {
	case gateway.FailureCategoryTransport:
		return []gateway.RecoveryRecommendation{
			{
				Action:      "check_connectivity",
				Description: "Check network connectivity and port availability",
				Commands:    []string{"diagnose", "status servers"},
				Priority:    1,
			},
			{
				Action:      "restart_server",
				Description: "Restart the LSP server",
				Commands:    []string{"server --port 8081"},
				Priority:    2,
			},
		}
	default:
		return []gateway.RecoveryRecommendation{
			{
				Action:      "restart_server",
				Description: "Restart the failed server",
				Commands:    []string{"diagnose"},
				Priority:    1,
			},
		}
	}
}

func (fh *CLIFailureHandler) generateConfigurationRecommendations() []gateway.RecoveryRecommendation {
	return []gateway.RecoveryRecommendation{
		{
			Action:      "validate_config",
			Description: "Validate the current configuration",
			Commands:    []string{"config validate"},
			Priority:    1,
		},
		{
			Action:      "regenerate_config",
			Description: "Generate a new configuration",
			Commands:    []string{"config generate --auto-detect"},
			Priority:    2,
		},
		{
			Action:      "setup_all",
			Description: "Run complete setup process",
			Commands:    []string{"setup all"},
			Priority:    3,
		},
	}
}

func (fh *CLIFailureHandler) filterFailuresBySeverity(failures []gateway.FailureNotification) []gateway.FailureNotification {
	var filtered []gateway.FailureNotification
	for _, failure := range failures {
		if failure.Severity >= fh.config.SeverityFilter {
			filtered = append(filtered, failure)
		}
	}
	return filtered
}

func (fh *CLIFailureHandler) logFailure(notification gateway.FailureNotification) {
	logLevel := strings.ToLower(fh.config.LogLevel)
	
	message := fmt.Sprintf("Server failure: %s (%s) - %s: %s", 
		notification.ServerName, 
		notification.Language,
		notification.Category.String(),
		notification.Message)

	switch logLevel {
	case "error":
		if notification.Severity >= gateway.FailureSeverityHigh {
			fh.logger.Printf("ERROR: %s", message)
		}
	case "warn":
		if notification.Severity >= gateway.FailureSeverityMedium {
			fh.logger.Printf("WARN: %s", message)
		}
	case "info":
		fh.logger.Printf("INFO: %s", message)
	case "debug":
		fh.logger.Printf("DEBUG: %s [%v]", message, notification)
	}
}

// Helper functions


func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection") ||
		   strings.Contains(errStr, "transport") ||
		   strings.Contains(errStr, "network") ||
		   strings.Contains(errStr, "dial") ||
		   strings.Contains(errStr, "broken pipe")
}

func getCurrentProjectPath() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

func getCurrentConfigFile() string {
	// Check common config file locations
	configFiles := []string{
		"config.yaml",
		"lsp-gateway.yaml", 
		".lsp-gateway.yaml",
		"config/lsp-gateway.yaml",
	}
	
	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			return file
		}
	}
	
	return ""
}

func getStringFromContext(context map[string]interface{}, key string) string {
	if val, exists := context[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func extractCommonLanguage(failures []gateway.FailureNotification) string {
	if len(failures) == 0 {
		return ""
	}
	
	// If all failures are for the same language, return that language
	firstLang := failures[0].Language
	for _, failure := range failures {
		if failure.Language != firstLang {
			return "" // Mixed languages
		}
	}
	
	return firstLang
}

func extractLanguageForServer(serverName string, failures []gateway.FailureNotification) string {
	for _, failure := range failures {
		if failure.ServerName == serverName {
			return failure.Language
		}
	}
	return ""
}

func findFailureByServerName(serverName string, failures []gateway.FailureNotification) *gateway.FailureNotification {
	for _, failure := range failures {
		if failure.ServerName == serverName {
			return &failure
		}
	}
	return nil
}