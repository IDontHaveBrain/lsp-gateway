package common

import (
	"fmt"
	"os"

	"lsp-gateway/internal/config"
)

type ConfigManager struct{}

func NewConfigManager() *ConfigManager {
	return &ConfigManager{}
}

func (cm *ConfigManager) LoadAndValidateConfig(configPath string) (*config.GatewayConfig, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, &ConfigError{
			Type:    ConfigErrorTypeNotFound,
			Message: fmt.Sprintf(ERROR_CONFIG_NOT_FOUND, configPath),
			Path:    configPath,
			Suggestions: []string{
				fmt.Sprintf(SUGGESTION_CREATE_CONFIG, configPath),
				"Run complete setup: lsp-gateway setup all",
				"Use interactive setup: lsp-gateway setup wizard",
			},
		}
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if os.IsPermission(err) {
			return nil, &ConfigError{
				Type:    ConfigErrorTypePermission,
				Message: fmt.Sprintf("Permission denied reading configuration file: %s", configPath),
				Path:    configPath,
				Cause:   err,
				Suggestions: []string{
					fmt.Sprintf("Check file permissions: ls -la %s", configPath),
					fmt.Sprintf("Fix permissions: chmod 644 %s", configPath),
				},
			}
		}

		return nil, &ConfigError{
			Type:    ConfigErrorTypeInvalid,
			Message: fmt.Sprintf(ERROR_CONFIG_LOAD_FAILED, err),
			Path:    configPath,
			Cause:   err,
			Suggestions: []string{
				"Check YAML syntax",
				"Validate config: lsp-gateway config validate",
				"Regenerate config: lsp-gateway config generate",
			},
		}
	}

	if err := config.ValidateConfig(cfg); err != nil {
		return nil, &ConfigError{
			Type:    ConfigErrorTypeValidation,
			Message: fmt.Sprintf("Configuration validation failed: %v", err),
			Path:    configPath,
			Cause:   err,
			Suggestions: []string{
				"Fix validation issues",
				"Regenerate config: lsp-gateway config generate",
				"Use setup wizard: lsp-gateway setup wizard",
			},
		}
	}

	return cfg, nil
}

func (cm *ConfigManager) OverridePortIfSpecified(cfg *config.GatewayConfig, port, defaultPort int) {
	if port != defaultPort {
		cfg.Port = port
	}
}

type ConfigErrorType int

const (
	ConfigErrorTypeNotFound ConfigErrorType = iota
	ConfigErrorTypePermission
	ConfigErrorTypeInvalid
	ConfigErrorTypeValidation
)

type ConfigError struct {
	Type        ConfigErrorType
	Message     string
	Path        string
	Cause       error
	Suggestions []string
}

func (e *ConfigError) Error() string {
	var prefix string
	switch e.Type {
	case ConfigErrorTypeNotFound:
		prefix = "Configuration Not Found"
	case ConfigErrorTypePermission:
		prefix = "Configuration Permission Error"
	case ConfigErrorTypeInvalid:
		prefix = "Invalid Configuration"
	case ConfigErrorTypeValidation:
		prefix = "Configuration Validation Error"
	default:
		prefix = "Configuration Error"
	}

	message := prefix + ": " + e.Message

	if e.Cause != nil {
		message += " (cause: " + e.Cause.Error() + ")"
	}

	return message
}

func (e *ConfigError) Unwrap() error {
	return e.Cause
}

func (e *ConfigError) GetSuggestions() []string {
	return e.Suggestions
}
