package lsp

import (
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/errors"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
)

func TranslateAndLogError(serverName, line string, context []string) bool {
	if strings.Contains(line, "KeyError") {
		hasWorkspaceSymbol := strings.Contains(strings.Join(context, " "), "workspace") ||
			strings.Contains(strings.Join(context, " "), "symbol")

		if hasWorkspaceSymbol || serverName == "pylsp" {
			common.LSPLogger.Warn("LSP %s: Server doesn't support workspace/symbol feature. %s",
				serverName,
				GetMethodSuggestion(serverName, types.MethodWorkspaceSymbol))
			return true
		}
	}

	if strings.Contains(line, "Method not found") || strings.Contains(line, "MethodNotFound") {
		method := extractMethodFromError(line)
		if method != "" {
			common.LSPLogger.Warn("LSP %s: Method '%s' not supported. %s",
				serverName,
				method,
				GetMethodSuggestion(serverName, method))
			return true
		}
	}

	if strings.Contains(line, "not supported") || strings.Contains(line, "unsupported") {
		common.LSPLogger.Warn("LSP %s: Feature not supported by this server. Consider checking server capabilities or using an alternative server.", serverName)
		return true
	}

	return false
}

func GetMethodSuggestion(serverName, method string) string {
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

func extractMethodFromError(errorLine string) string {
	patterns := []string{
		types.MethodWorkspaceSymbol,
		types.MethodTextDocumentDefinition,
		types.MethodTextDocumentReferences,
		types.MethodTextDocumentHover,
		types.MethodTextDocumentCompletion,
		types.MethodTextDocumentDocumentSymbol,
	}

	for _, pattern := range patterns {
		if strings.Contains(errorLine, pattern) {
			return pattern
		}
	}

	return ""
}

// CreateUnifiedError creates a unified error from LSP error translation
func CreateUnifiedError(serverName, line string, context []string) error {
	if strings.Contains(line, "KeyError") {
		hasWorkspaceSymbol := strings.Contains(strings.Join(context, " "), "workspace") ||
			strings.Contains(strings.Join(context, " "), "symbol")

		if hasWorkspaceSymbol || serverName == "pylsp" {
			suggestion := GetMethodSuggestion(serverName, types.MethodWorkspaceSymbol)
			return errors.NewMethodNotSupportedError(serverName, types.MethodWorkspaceSymbol, suggestion)
		}
	}

	if strings.Contains(line, "Method not found") || strings.Contains(line, "MethodNotFound") {
		method := extractMethodFromError(line)
		if method != "" {
			suggestion := GetMethodSuggestion(serverName, method)
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
func TranslateToUnifiedError(serverName string, err error) error {
	if err == nil {
		return nil
	}

	// Classify the error using centralized classification
	classification := errors.ClassifyError(err)

	// If already classified as a known type, check if it needs wrapping
	switch classification {
	case errors.ClassConnection:
		// Already a connection error or classified as one
		if errors.IsConnectionError(err) {
			return err
		}
		return errors.NewConnectionError(serverName, err)

	case errors.ClassTimeout:
		// Already a timeout error or classified as one
		if errors.IsTimeoutError(err) {
			return err
		}
		return errors.NewTimeoutError("lsp_operation", serverName, 0, err)

	case errors.ClassMethodNotSupported:
		// Already a method not supported error or classified as one
		if errors.IsMethodNotSupportedError(err) {
			return err
		}
		method := extractMethodFromError(err.Error())
		suggestion := GetMethodSuggestion(serverName, method)
		return errors.NewMethodNotSupportedError(serverName, method, suggestion)

	case errors.ClassValidation:
		// Already a validation error or classified as one
		if errors.IsValidationError(err) {
			return err
		}
		return errors.NewValidationError("unknown", err.Error())

	case errors.ClassProcess:
		// Process errors are related to connection issues
		return errors.NewProcessError(serverName, "", "process", err)

	case errors.ClassProtocol:
		// Protocol errors are already LSPError types
		if errors.IsProtocolError(err) {
			return err
		}
		return errors.NewLSPError(errors.InternalError, err.Error(), map[string]string{
			"server": serverName,
		})

	case errors.ClassCancellation:
		// Return as-is for cancellation
		return err

	default:
		// Unknown classification - wrap as generic LSP error
		return errors.NewLSPError(errors.InternalError, err.Error(), map[string]string{
			"server": serverName,
		})
	}
}
