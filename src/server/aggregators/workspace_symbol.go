package aggregators

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server/errors"
)

// LSPClient interface for LSP communication required by aggregators
type LSPClient interface {
	Supports(method string) bool
	SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
}

// WorkspaceSymbolAggregator handles workspace symbol queries across multiple LSP clients
type WorkspaceSymbolAggregator interface {
	ProcessWorkspaceSymbol(ctx context.Context, clients map[string]interface{}, params interface{}) (interface{}, error)
}

// LSPWorkspaceSymbolAggregator implements workspace symbol aggregation
type LSPWorkspaceSymbolAggregator struct {
	errorTranslator errors.ErrorTranslator
}

// NewWorkspaceSymbolAggregator creates a new workspace symbol aggregator
func NewWorkspaceSymbolAggregator() *LSPWorkspaceSymbolAggregator {
	return &LSPWorkspaceSymbolAggregator{
		errorTranslator: errors.NewLSPErrorTranslator(),
	}
}

// ProcessWorkspaceSymbol processes workspace/symbol requests across all clients
// For multi-repo support, collects results from all active clients and merges them
func (w *LSPWorkspaceSymbolAggregator) ProcessWorkspaceSymbol(ctx context.Context, clients map[string]interface{}, params interface{}) (interface{}, error) {
	clientList := make([]LSPClient, 0, len(clients))
	languages := make([]string, 0, len(clients))
	unsupportedClients := make([]string, 0)

	for lang, clientInterface := range clients {
		// Type assert to LSPClient interface
		client, ok := clientInterface.(LSPClient)
		if !ok {
			unsupportedClients = append(unsupportedClients, lang)
			continue
		}

		// Only include clients that support workspace/symbol
		if client.Supports("workspace/symbol") {
			clientList = append(clientList, client)
			languages = append(languages, lang)
		} else {
			unsupportedClients = append(unsupportedClients, lang)
		}
	}

	// Log which servers are being skipped for workspace/symbol
	if len(unsupportedClients) > 0 {
		common.LSPLogger.Debug("Skipping servers that don't support workspace/symbol: %v", unsupportedClients)
	}

	if len(clientList) == 0 {
		if len(unsupportedClients) > 0 {
			return nil, fmt.Errorf("no LSP servers support workspace/symbol. Unsupported servers: %v. %s",
				unsupportedClients,
				w.errorTranslator.GetMethodSuggestion(unsupportedClients[0], "workspace/symbol"))
		}
		return nil, fmt.Errorf("no active LSP clients available")
	}

	// Use channels to collect results from all clients in parallel
	type clientResult struct {
		language string
		result   json.RawMessage
		err      error
	}

	resultCh := make(chan clientResult, len(clientList))

	// Query all clients in parallel with individual timeouts
	for i, client := range clientList {
		go func(lang string, c LSPClient) {
			// Create a timeout context for this individual client request
			// Use longer timeout for workspace/symbol as it can be slow on large codebases
			clientCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()

			result, err := c.SendRequest(clientCtx, "workspace/symbol", params)

			// Always send a result, even if it's an error
			select {
			case resultCh <- clientResult{
				language: lang,
				result:   result,
				err:      err,
			}:
			case <-clientCtx.Done():
				// Context cancelled, send timeout error
				resultCh <- clientResult{
					language: lang,
					result:   nil,
					err:      fmt.Errorf("client request timeout"),
				}
			}
		}(languages[i], client)
	}

	// Collect results from all clients with overall timeout
	var allSymbols []interface{}
	var errorsList []string
	successCount := 0

	// Overall timeout for collecting all responses (longer than individual client timeout)
	overallTimeout := time.After(25 * time.Second)
	responsesReceived := 0

	for responsesReceived < len(clientList) {
		select {
		case result := <-resultCh:
			responsesReceived++

			if result.err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", result.language, result.err))
				continue
			}

			// Parse the result - handle both array and object responses
			var symbols []interface{}

			// First try to unmarshal as array (standard LSP response)
			if err := json.Unmarshal(result.result, &symbols); err != nil {
				// If that fails, try to unmarshal as a single object and wrap it in an array
				var singleSymbol interface{}
				if err2 := json.Unmarshal(result.result, &singleSymbol); err2 != nil {
					errorsList = append(errorsList, fmt.Sprintf("%s: failed to parse symbols as array or object: %v", result.language, err))
					continue
				}
				// Wrap single object in array
				symbols = []interface{}{singleSymbol}
			}

			// Add all symbols from this client
			allSymbols = append(allSymbols, symbols...)
			successCount++

		case <-overallTimeout:
			// Overall timeout reached - return what we have
			common.LSPLogger.Warn("Overall timeout reached while collecting workspace symbols, returning partial results (%d/%d clients responded)", responsesReceived, len(clientList))
			goto collectDone

		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while collecting workspace symbols")
		}
	}

collectDone:

	// If no clients succeeded, return error
	if successCount == 0 {
		return nil, fmt.Errorf("no workspace symbol results found - all clients failed: %s", strings.Join(errorsList, "; "))
	}

	// Log errors but don't fail if some clients succeeded
	if len(errorsList) > 0 {
		common.LSPLogger.Warn("Some LSP clients failed during workspace/symbol query: %s", strings.Join(errorsList, "; "))
	}

	// Return combined results from all successful clients
	return allSymbols, nil
}
