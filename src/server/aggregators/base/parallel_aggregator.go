package base

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
)

// result represents the outcome of a parallel client operation
type result[TResponse any] struct {
	language string
	response TResponse
	err      error
}

// ParallelAggregator provides a generic framework for executing parallel operations
// across multiple LSP clients with timeout handling and error aggregation.
type ParallelAggregator[TRequest, TResponse any] struct {
	individualTimeout time.Duration
	overallTimeout    time.Duration
}

// NewParallelAggregator creates a new ParallelAggregator with specified timeouts
func NewParallelAggregator[TRequest, TResponse any](individualTimeout, overallTimeout time.Duration) *ParallelAggregator[TRequest, TResponse] {
	return &ParallelAggregator[TRequest, TResponse]{
		individualTimeout: individualTimeout,
		overallTimeout:    overallTimeout,
	}
}

// Execute runs parallel operations across LSP clients and aggregates results.
// Returns a map of successful responses by language and any errors encountered.
// Supports graceful degradation - partial success is allowed.
func (a *ParallelAggregator[TRequest, TResponse]) Execute(
	ctx context.Context,
	clients map[string]types.LSPClient,
	request TRequest,
	executor func(context.Context, types.LSPClient, TRequest) (TResponse, error),
) (map[string]TResponse, []error) {
	if len(clients) == 0 {
		return make(map[string]TResponse), []error{fmt.Errorf("no clients provided")}
	}

	// Create channel for result collection
	resultCh := make(chan result[TResponse], len(clients))

	// Launch parallel goroutines for each client
	for language, client := range clients {
		go func(lang string, c types.LSPClient) {
			// Create individual timeout context for this client request
			clientCtx, cancel := common.WithTimeout(ctx, a.individualTimeout)
			defer cancel()

			// Execute the operation
			response, err := executor(clientCtx, c, request)

			// Always send a result, even if it's an error
			select {
			case resultCh <- result[TResponse]{
				language: lang,
				response: response,
				err:      err,
			}:
			case <-clientCtx.Done():
				// Context cancelled, send timeout error
				var zeroResponse TResponse
				resultCh <- result[TResponse]{
					language: lang,
					response: zeroResponse,
					err:      fmt.Errorf("client request timeout"),
				}
			}
		}(language, client)
	}

	// Collect results with overall timeout
	results := make(map[string]TResponse)
	var errorsList []string
	responsesReceived := 0
	respondedClients := make(map[string]bool)

	// Set up overall timeout
	overallTimeout := time.After(a.overallTimeout)

	for responsesReceived < len(clients) {
		select {
		case result := <-resultCh:
			responsesReceived++
			respondedClients[result.language] = true

			if result.err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", result.language, result.err))
				continue
			}

			results[result.language] = result.response

		case <-overallTimeout:
			// Overall timeout reached - add errors for clients that didn't respond
			common.LSPLogger.Warn("Overall timeout reached while collecting parallel results, returning partial results (%d/%d clients responded)", responsesReceived, len(clients))
			
			// Add errors for clients that didn't respond
			for language := range clients {
				if !respondedClients[language] {
					errorsList = append(errorsList, fmt.Sprintf("%s: overall timeout - no response received", language))
				}
			}
			goto collectDone

		case <-ctx.Done():
			// Context cancelled
			var contextErrors []error
			contextErrors = append(contextErrors, fmt.Errorf("context cancelled while collecting parallel results"))
			return results, contextErrors
		}
	}

collectDone:
	// Convert error strings to error slice
	var errors []error
	for _, errStr := range errorsList {
		errors = append(errors, fmt.Errorf("%s", errStr))
	}

	// Log errors but don't fail if some clients succeeded
	if len(errorsList) > 0 {
		common.LSPLogger.Warn("Some LSP clients failed during parallel operation: %s", strings.Join(errorsList, "; "))
	}

	return results, errors
}

// ExecuteWithLanguageTimeouts runs parallel operations with language-specific individual timeouts.
// The timeoutFunc is called for each language to determine its individual timeout.
func (a *ParallelAggregator[TRequest, TResponse]) ExecuteWithLanguageTimeouts(
	ctx context.Context,
	clients map[string]types.LSPClient,
	request TRequest,
	executor func(context.Context, types.LSPClient, TRequest) (TResponse, error),
	timeoutFunc func(string) time.Duration,
) (map[string]TResponse, []error) {
	if len(clients) == 0 {
		return make(map[string]TResponse), []error{fmt.Errorf("no clients provided")}
	}

	// Create channel for result collection
	resultCh := make(chan result[TResponse], len(clients))

	// Launch parallel goroutines for each client
	for language, client := range clients {
		go func(lang string, c types.LSPClient) {
			// Use language-specific timeout
			individualTimeout := timeoutFunc(lang)
			clientCtx, cancel := common.WithTimeout(ctx, individualTimeout)
			defer cancel()

			// Execute the operation
			response, err := executor(clientCtx, c, request)

			// Always send a result, even if it's an error
			select {
			case resultCh <- result[TResponse]{
				language: lang,
				response: response,
				err:      err,
			}:
			case <-clientCtx.Done():
				// Context cancelled, send timeout error
				var zeroResponse TResponse
				resultCh <- result[TResponse]{
					language: lang,
					response: zeroResponse,
					err:      fmt.Errorf("client request timeout"),
				}
			}
		}(language, client)
	}

	// Collect results with overall timeout
	results := make(map[string]TResponse)
	var errorsList []string
	responsesReceived := 0
	respondedClients := make(map[string]bool)

	// Set up overall timeout
	overallTimeout := time.After(a.overallTimeout)

	for responsesReceived < len(clients) {
		select {
		case result := <-resultCh:
			responsesReceived++
			respondedClients[result.language] = true

			if result.err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", result.language, result.err))
				continue
			}

			results[result.language] = result.response

		case <-overallTimeout:
			// Overall timeout reached - add errors for clients that didn't respond
			common.LSPLogger.Warn("Overall timeout reached while collecting parallel results, returning partial results (%d/%d clients responded)", responsesReceived, len(clients))
			
			// Add errors for clients that didn't respond
			for language := range clients {
				if !respondedClients[language] {
					errorsList = append(errorsList, fmt.Sprintf("%s: overall timeout - no response received", language))
				}
			}
			goto collectDone

		case <-ctx.Done():
			// Context cancelled
			var contextErrors []error
			contextErrors = append(contextErrors, fmt.Errorf("context cancelled while collecting parallel results"))
			return results, contextErrors
		}
	}

collectDone:
	// Convert error strings to error slice
	var errors []error
	for _, errStr := range errorsList {
		errors = append(errors, fmt.Errorf("%s", errStr))
	}

	// Log errors but don't fail if some clients succeeded
	if len(errorsList) > 0 {
		common.LSPLogger.Warn("Some LSP clients failed during parallel operation: %s", strings.Join(errorsList, "; "))
	}

	return results, errors
}

// ExecuteAll runs operations across all clients and only succeeds if ALL clients succeed.
// Returns error if any client fails. Use this for operations requiring complete success.
func (a *ParallelAggregator[TRequest, TResponse]) ExecuteAll(
	ctx context.Context,
	clients map[string]types.LSPClient,
	request TRequest,
	executor func(context.Context, types.LSPClient, TRequest) (TResponse, error),
) (map[string]TResponse, error) {
	results, errors := a.Execute(ctx, clients, request, executor)

	// If any errors occurred, fail completely
	if len(errors) > 0 {
		var errorMessages []string
		for _, err := range errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return nil, fmt.Errorf("parallel operation failed - all clients must succeed: %s", strings.Join(errorMessages, "; "))
	}

	return results, nil
}

// ExecuteAtLeastOne runs operations across clients and succeeds if at least one client succeeds.
// Returns error only if ALL clients fail.
func (a *ParallelAggregator[TRequest, TResponse]) ExecuteAtLeastOne(
	ctx context.Context,
	clients map[string]types.LSPClient,
	request TRequest,
	executor func(context.Context, types.LSPClient, TRequest) (TResponse, error),
) (map[string]TResponse, error) {
	results, errors := a.Execute(ctx, clients, request, executor)

	// If no successful results, return error
	if len(results) == 0 {
		var errorMessages []string
		for _, err := range errors {
			errorMessages = append(errorMessages, err.Error())
		}
		return nil, fmt.Errorf("no parallel operation results found - all clients failed: %s", strings.Join(errorMessages, "; "))
	}

	return results, nil
}
