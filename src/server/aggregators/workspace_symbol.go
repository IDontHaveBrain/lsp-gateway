package aggregators

import (
    "context"
    "fmt"
    "strings"
    "time"

    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/internal/types"
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
        result   []interface{}
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

            raw, err := c.SendRequest(clientCtx, types.MethodWorkspaceSymbol, params)

            // Always send a result, even if it's an error
            select {
            case resultCh <- func() clientResult {
                if err != nil {
                    return clientResult{language: lang, result: nil, err: err}
                }
                if raw == nil || string(raw) == "null" || len(raw) == 0 {
                    return clientResult{language: lang, result: []interface{}{}, err: nil}
                }
                if arr, convErr := jsonutil.Convert[[]interface{}](raw); convErr == nil {
                    return clientResult{language: lang, result: arr, err: nil}
                }
                if one, convErr := jsonutil.Convert[map[string]interface{}](raw); convErr == nil {
                    return clientResult{language: lang, result: []interface{}{one}, err: nil}
                }
                return clientResult{language: lang, result: nil, err: fmt.Errorf("failed to parse symbols")}
            }():
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

            allSymbols = append(allSymbols, result.result...)
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
