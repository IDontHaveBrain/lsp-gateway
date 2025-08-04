package errors

import (
	"strings"

	"lsp-gateway/src/internal/common"
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
	switch {
	case serverName == "python" && method == "workspace/symbol":
		return "Ensure 'pylsp' is installed via 'pip install python-lsp-server' for better workspace symbol support."
	case serverName == "python" && strings.Contains(method, "semanticTokens"):
		return "Ensure 'pylsp' is installed via 'pip install python-lsp-server' for semantic token support."
	case serverName == "pylsp" && method == "workspace/symbol":
		return "pylsp supports workspace/symbol. Ensure 'pylsp' is installed via 'pip install python-lsp-server'."
	case serverName == "pylsp" && strings.Contains(method, "semanticTokens"):
		return "pylsp has semantic token support. Ensure 'pylsp' is installed via 'pip install python-lsp-server'.'"
	default:
		return "Check your LSP server documentation for supported features or consider alternative servers."
	}
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
