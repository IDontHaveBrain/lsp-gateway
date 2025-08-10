package aggregators

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/internal/types"
    "lsp-gateway/src/server/errors"
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

// ProcessWorkspaceSymbol processes workspace/symbol requests across all clients
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

		// Only include clients that support workspace/symbol
		if client.Supports(types.MethodWorkspaceSymbol) {
			clientList = append(clientList, client)
			languages = append(languages, lang)
		} else {
			unsupportedClients = append(unsupportedClients, lang)
		}
	}

	if len(clientList) == 0 {
		if len(unsupportedClients) > 0 {
			return nil, fmt.Errorf("no LSP servers support workspace/symbol. Unsupported servers: %v. %s",
				unsupportedClients,
				w.errorTranslator.GetMethodSuggestion(unsupportedClients[0], types.MethodWorkspaceSymbol))
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
		go func(lang string, c types.LSPClient) {
			// Create a timeout context for this individual client request
			// Use longer timeout for workspace/symbol as it can be slow on large codebases
            clientCtx, cancel := common.WithTimeout(ctx, 20*time.Second)
			defer cancel()

			result, err := c.SendRequest(clientCtx, types.MethodWorkspaceSymbol, params)

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
	allSymbols := make([]interface{}, 0)
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

			// Parse the result - handle null, empty, array, and object responses
			var symbols []interface{}

			// Check if result is null or empty
			if len(result.result) == 0 || string(result.result) == "null" {
				// Empty or null result - no symbols found, which is valid
				common.LSPLogger.Debug("No symbols returned from %s (null/empty response)", result.language)
				successCount++
				continue
			}

			// First try to unmarshal as array (standard LSP response)
			if err := json.Unmarshal(result.result, &symbols); err != nil {
				// If that fails, try to unmarshal as a single object and wrap it in an array
				var singleSymbol interface{}
				if err2 := json.Unmarshal(result.result, &singleSymbol); err2 != nil {
					// Log the actual response for debugging Windows CI issues
					responsePreview := string(result.result)
					if len(responsePreview) > 100 {
						responsePreview = responsePreview[:100] + "..."
					}
					errorsList = append(errorsList, fmt.Sprintf("%s: failed to parse symbols (response: %s): %v", result.language, responsePreview, err))
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

	return allSymbols, nil
}
