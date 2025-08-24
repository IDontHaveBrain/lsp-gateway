// Package aggregators provides aggregation logic for LSP results.
package aggregators

import (
	"context"
	"fmt"
	"time"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/aggregators/base"
	"lsp-gateway/src/server/errors"
	"lsp-gateway/src/utils/jsonutil"
)

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

// ProcessWorkspaceSymbol processes types.MethodWorkspaceSymbol requests across all clients
// For multi-repo support, collects results from all active clients and merges them
func (w *LSPWorkspaceSymbolAggregator) ProcessWorkspaceSymbol(ctx context.Context, clients map[string]interface{}, params interface{}) (interface{}, error) {
	clientList := make([]types.LSPClient, 0, len(clients))
	languages := make([]string, 0, len(clients))
	unsupportedClients := make([]string, 0)

	for lang, clientInterface := range clients {
		// Type assert to LSPClient interface
		client, ok := clientInterface.(types.LSPClient)
		if !ok {
			unsupportedClients = append(unsupportedClients, lang)
			continue
		}

		// Only include clients that support types.MethodWorkspaceSymbol
		if client.Supports(types.MethodWorkspaceSymbol) {
			clientList = append(clientList, client)
			languages = append(languages, lang)
		} else {
			unsupportedClients = append(unsupportedClients, lang)
		}
	}

	if len(clientList) == 0 {
		if len(unsupportedClients) > 0 {
			return nil, fmt.Errorf("no LSP servers support %s. Unsupported servers: %v. %s",
				types.MethodWorkspaceSymbol,
				unsupportedClients,
				w.errorTranslator.GetMethodSuggestion(unsupportedClients[0], types.MethodWorkspaceSymbol))
		}
		return nil, fmt.Errorf("no active LSP clients available")
	}

	// Convert clients map to proper type for framework
	clientMap := make(map[string]types.LSPClient)
	for i, client := range clientList {
		clientMap[languages[i]] = client
	}

	// Create timeout manager for workspace symbol operations
	timeoutManager := base.NewTimeoutManager().ForOperation(base.OperationRequest)

	// Calculate overall timeout based on languages
	overallTimeout := timeoutManager.GetOverallTimeout(languages)
	// Add 25% buffer for overall timeout
	overallTimeout = time.Duration(float64(overallTimeout) * 1.25)

	// Create aggregator with dynamic timeouts
	aggregator := base.NewParallelAggregator[interface{}, []interface{}](0, overallTimeout)

	// Define executor function to handle LSP request and JSON parsing
	executor := func(ctx context.Context, client types.LSPClient, params interface{}) ([]interface{}, error) {
		raw, err := client.SendRequest(ctx, types.MethodWorkspaceSymbol, params)
		if err != nil {
			return nil, err
		}

		// Handle null/empty responses
		if raw == nil || string(raw) == "null" || len(raw) == 0 {
			return []interface{}{}, nil
		}

		// Try parsing as array first
		if arr, convErr := jsonutil.Convert[[]interface{}](raw); convErr == nil {
			return arr, nil
		}

		// Try parsing as single object
		if one, convErr := jsonutil.Convert[map[string]interface{}](raw); convErr == nil {
			return []interface{}{one}, nil
		}

		return nil, fmt.Errorf("failed to parse symbols")
	}

	// Define timeout function for language-specific timeouts
	timeoutFunc := func(language string) time.Duration {
		return timeoutManager.GetTimeout(language)
	}

	// Execute parallel requests with language-specific timeouts and partial success support
	results, _ := aggregator.ExecuteWithLanguageTimeouts(ctx, clientMap, params, executor, timeoutFunc)

	// Use SliceMerger to combine all results
	merger := base.NewSliceMerger[interface{}](nil)
	allSymbols := merger.Merge(results)

	return allSymbols, nil
}
