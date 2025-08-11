package errors

import (
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/errors"
	"lsp-gateway/src/internal/registry"
)

type ErrorTranslator interface {
	TranslateAndLogError(serverName, line string, context []string) bool
	GetMethodSuggestion(serverName, method string) string
}

type LSPErrorTranslator struct{}

func NewLSPErrorTranslator() *LSPErrorTranslator {
	return &LSPErrorTranslator{}
}

func (t *LSPErrorTranslator) TranslateAndLogError(serverName, line string, context []string) bool {
	if strings.Contains(line, "KeyError") {
		hasWorkspaceSymbol := strings.Contains(strings.Join(context, " "), "workspace") ||
			strings.Contains(strings.Join(context, " "), "symbol")

		if hasWorkspaceSymbol || serverName == "pylsp" {
			common.LSPLogger.Warn("LSP %s: Server doesn't support workspace/symbol feature. %s",
				serverName,
				t.GetMethodSuggestion(serverName, "workspace/symbol"))
			return true
		}
	}

	if strings.Contains(line, "Method not found") || strings.Contains(line, "MethodNotFound") {
		method := t.extractMethodFromError(line)
		if method != "" {
			common.LSPLogger.Warn("LSP %s: Method '%s' not supported. %s",
				serverName,
				method,
				t.GetMethodSuggestion(serverName, method))
			return true
		}
	}

	if strings.Contains(line, "not supported") || strings.Contains(line, "unsupported") {
		common.LSPLogger.Warn("LSP %s: Feature not supported by this server. Consider checking server capabilities or using an alternative server.", serverName)
		return true
	}

	return false
}

func (t *LSPErrorTranslator) GetMethodSuggestion(serverName, method string) string {
	// Try to get language by server name, fallback to treating serverName as language name
	langInfo, exists := registry.GetLanguageByName(serverName)
	if !exists {
		// Check if this is a known server command and find the associated language
		for _, lang := range registry.GetSupportedLanguages() {
			if lang.DefaultCommand == serverName {
				langInfo = &lang
				exists = true
				break
			}
		}
	}

	if exists {
		errorPatterns := langInfo.GetErrorPatterns()
		for _, pattern := range errorPatterns {
			if strings.Contains(method, pattern.Pattern) {
				return pattern.Message
			}
		}
	}

	return "Check your LSP server documentation for supported features or consider alternative servers."
}

func (t *LSPErrorTranslator) extractMethodFromError(errorLine string) string {
	patterns := []string{
		"workspace/symbol",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/completion",
		"textDocument/documentSymbol",
	}

	for _, pattern := range patterns {
		if strings.Contains(errorLine, pattern) {
			return pattern
		}
	}

	return ""
}

// CreateUnifiedError creates a unified error from LSP error translation
func (t *LSPErrorTranslator) CreateUnifiedError(serverName, line string, context []string) error {
	if strings.Contains(line, "KeyError") {
		hasWorkspaceSymbol := strings.Contains(strings.Join(context, " "), "workspace") ||
			strings.Contains(strings.Join(context, " "), "symbol")

		if hasWorkspaceSymbol || serverName == "pylsp" {
			suggestion := t.GetMethodSuggestion(serverName, "workspace/symbol")
			return errors.NewMethodNotSupportedError(serverName, "workspace/symbol", suggestion)
		}
	}

	if strings.Contains(line, "Method not found") || strings.Contains(line, "MethodNotFound") {
		method := t.extractMethodFromError(line)
		if method != "" {
			suggestion := t.GetMethodSuggestion(serverName, method)
			return errors.NewMethodNotSupportedError(serverName, method, suggestion)
		}
		return errors.NewLSPError(errors.MethodNotFound, "Method not found", map[string]string{
			"server": serverName,
			"line":   line,
		})
	}

	if strings.Contains(line, "not supported") || strings.Contains(line, "unsupported") {
		return errors.NewLSPError(errors.UnsupportedMethod, "Feature not supported", map[string]string{
			"server": serverName,
			"line":   line,
		})
	}

	// Default to generic LSP error
	return errors.NewLSPError(errors.InternalError, line, map[string]string{
		"server": serverName,
	})
}

// TranslateToUnifiedError translates any error to the appropriate unified error type
func (t *LSPErrorTranslator) TranslateToUnifiedError(serverName string, err error) error {
	if err == nil {
		return nil
	}

	// Check if already a unified error type
	if errors.IsConnectionError(err) || errors.IsValidationError(err) ||
		errors.IsTimeoutError(err) || errors.IsMethodNotSupportedError(err) {
		return err
	}

	errMsg := err.Error()

	// Classify and wrap the error appropriately
	if errors.IsConnectionError(err) || strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "process") {
		return errors.NewConnectionError(serverName, err)
	}

	if errors.IsTimeoutError(err) {
		return errors.NewTimeoutError("lsp_operation", serverName, 0, err)
	}

	if strings.Contains(errMsg, "not supported") || strings.Contains(errMsg, "method not found") {
		method := t.extractMethodFromError(errMsg)
		suggestion := t.GetMethodSuggestion(serverName, method)
		return errors.NewMethodNotSupportedError(serverName, method, suggestion)
	}

	if strings.Contains(errMsg, "validation") || strings.Contains(errMsg, "parameter") {
		return errors.NewValidationError("unknown", errMsg)
	}

	// Default to generic LSP error
	return errors.NewLSPError(errors.InternalError, errMsg, map[string]string{
		"server": serverName,
	})
}
