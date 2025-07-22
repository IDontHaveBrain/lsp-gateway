package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

// RequestGenerator provides utilities for generating realistic LSP requests
type RequestGenerator struct {
	baseURL   string
	client    *http.Client
	requestID int64
	languages []string
	fileURIs  []string
	positions []Position
}

// Position represents a position in a document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a range in a document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// RequestPattern defines a pattern for generating requests
type RequestPattern struct {
	Method       string
	Weight       int // Relative frequency of this request type
	ParamsFunc   func(gen *RequestGenerator) interface{}
	ValidateFunc func(response *JSONRPCResponse) error
}

// NewRequestGenerator creates a new request generator
func NewRequestGenerator(baseURL string) *RequestGenerator {
	return &RequestGenerator{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		languages: []string{"go", "python", "typescript", "javascript", "java"},
		fileURIs: []string{
			"file:///test.go",
			"file:///main.py",
			"file:///app.ts",
			"file:///index.js",
			"file:///Application.java",
		},
		positions: []Position{
			{Line: 0, Character: 0},
			{Line: 5, Character: 10},
			{Line: 10, Character: 5},
			{Line: 15, Character: 20},
			{Line: 20, Character: 0},
		},
	}
}

// GetDefaultRequestPatterns returns common LSP request patterns
func (rg *RequestGenerator) GetDefaultRequestPatterns() []RequestPattern {
	return []RequestPattern{
		{
			Method:       "textDocument/definition",
			Weight:       30,
			ParamsFunc:   rg.generateDefinitionParams,
			ValidateFunc: rg.validateDefinitionResponse,
		},
		{
			Method:       "textDocument/hover",
			Weight:       25,
			ParamsFunc:   rg.generateHoverParams,
			ValidateFunc: rg.validateHoverResponse,
		},
		{
			Method:       "textDocument/references",
			Weight:       20,
			ParamsFunc:   rg.generateReferencesParams,
			ValidateFunc: rg.validateReferencesResponse,
		},
		{
			Method:       "textDocument/documentSymbol",
			Weight:       15,
			ParamsFunc:   rg.generateDocumentSymbolParams,
			ValidateFunc: rg.validateDocumentSymbolResponse,
		},
		{
			Method:       "workspace/symbol",
			Weight:       10,
			ParamsFunc:   rg.generateWorkspaceSymbolParams,
			ValidateFunc: rg.validateWorkspaceSymbolResponse,
		},
	}
}

// generateDefinitionParams creates parameters for textDocument/definition requests
func (rg *RequestGenerator) generateDefinitionParams(gen *RequestGenerator) interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": rg.randomFileURI(),
		},
		"position": rg.randomPosition(),
	}
}

// generateHoverParams creates parameters for textDocument/hover requests
func (rg *RequestGenerator) generateHoverParams(gen *RequestGenerator) interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": rg.randomFileURI(),
		},
		"position": rg.randomPosition(),
	}
}

// generateReferencesParams creates parameters for textDocument/references requests
func (rg *RequestGenerator) generateReferencesParams(gen *RequestGenerator) interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": rg.randomFileURI(),
		},
		"position": rg.randomPosition(),
		"context": map[string]interface{}{
			"includeDeclaration": rand.Intn(2) == 1,
		},
	}
}

// generateDocumentSymbolParams creates parameters for textDocument/documentSymbol requests
func (rg *RequestGenerator) generateDocumentSymbolParams(gen *RequestGenerator) interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": rg.randomFileURI(),
		},
	}
}

// generateWorkspaceSymbolParams creates parameters for workspace/symbol requests
func (rg *RequestGenerator) generateWorkspaceSymbolParams(gen *RequestGenerator) interface{} {
	queries := []string{"test", "main", "function", "class", "method", "variable"}
	return map[string]interface{}{
		"query": queries[rand.Intn(len(queries))],
	}
}

// randomFileURI returns a random file URI
func (rg *RequestGenerator) randomFileURI() string {
	return rg.fileURIs[rand.Intn(len(rg.fileURIs))]
}

// randomPosition returns a random position
func (rg *RequestGenerator) randomPosition() Position {
	return rg.positions[rand.Intn(len(rg.positions))]
}

// nextRequestID generates a unique request ID
func (rg *RequestGenerator) nextRequestID() int64 {
	return atomic.AddInt64(&rg.requestID, 1)
}

// GenerateRequest creates a random request based on patterns
func (rg *RequestGenerator) GenerateRequest(patterns []RequestPattern) *JSONRPCRequest {
	pattern := rg.selectPattern(patterns)

	return &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      rg.nextRequestID(),
		Method:  pattern.Method,
		Params:  pattern.ParamsFunc(rg),
	}
}

// selectPattern selects a pattern based on weights
func (rg *RequestGenerator) selectPattern(patterns []RequestPattern) RequestPattern {
	totalWeight := 0
	for _, p := range patterns {
		totalWeight += p.Weight
	}

	target := rand.Intn(totalWeight)
	current := 0

	for _, p := range patterns {
		current += p.Weight
		if current > target {
			return p
		}
	}

	return patterns[0] // Fallback
}

// SendRequest sends a request and returns the response
func (rg *RequestGenerator) SendRequest(ctx context.Context, request *JSONRPCRequest) (*JSONRPCResponse, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rg.baseURL+"/jsonrpc", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := rg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the request
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// SendRequestBatch sends multiple requests and returns their responses
func (rg *RequestGenerator) SendRequestBatch(ctx context.Context, requests []*JSONRPCRequest) ([]*JSONRPCResponse, error) {
	responses := make([]*JSONRPCResponse, len(requests))
	errors := make([]error, len(requests))

	// Use a simple sequential approach for now
	// Could be enhanced with concurrency if needed
	for i, req := range requests {
		resp, err := rg.SendRequest(ctx, req)
		responses[i] = resp
		errors[i] = err
	}

	// Return first error encountered, if any
	for _, err := range errors {
		if err != nil {
			return responses, err
		}
	}

	return responses, nil
}

// Validation functions for different response types

func (rg *RequestGenerator) validateDefinitionResponse(response *JSONRPCResponse) error {
	if response.Error != nil {
		return nil // Errors are acceptable in testing
	}

	if response.Result == nil {
		return fmt.Errorf("definition response missing result")
	}

	// Basic validation - could be enhanced
	return nil
}

func (rg *RequestGenerator) validateHoverResponse(response *JSONRPCResponse) error {
	if response.Error != nil {
		return nil // Errors are acceptable in testing
	}

	if response.Result == nil {
		return fmt.Errorf("hover response missing result")
	}

	resultMap, ok := response.Result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("hover result is not an object")
	}

	if _, hasContents := resultMap["contents"]; !hasContents {
		return fmt.Errorf("hover response missing contents")
	}

	return nil
}

func (rg *RequestGenerator) validateReferencesResponse(response *JSONRPCResponse) error {
	if response.Error != nil {
		return nil // Errors are acceptable in testing
	}

	if response.Result == nil {
		return fmt.Errorf("references response missing result")
	}

	// Result should be an array
	if _, ok := response.Result.([]interface{}); !ok {
		return fmt.Errorf("references result is not an array")
	}

	return nil
}

func (rg *RequestGenerator) validateDocumentSymbolResponse(response *JSONRPCResponse) error {
	if response.Error != nil {
		return nil // Errors are acceptable in testing
	}

	if response.Result == nil {
		return fmt.Errorf("document symbol response missing result")
	}

	// Result should be an array
	if _, ok := response.Result.([]interface{}); !ok {
		return fmt.Errorf("document symbol result is not an array")
	}

	return nil
}

func (rg *RequestGenerator) validateWorkspaceSymbolResponse(response *JSONRPCResponse) error {
	if response.Error != nil {
		return nil // Errors are acceptable in testing
	}

	if response.Result == nil {
		return fmt.Errorf("workspace symbol response missing result")
	}

	// Result should be an array
	if _, ok := response.Result.([]interface{}); !ok {
		return fmt.Errorf("workspace symbol result is not an array")
	}

	return nil
}

// RequestSequence represents a sequence of related requests
type RequestSequence struct {
	Name     string
	Requests []*JSONRPCRequest
	Delays   []time.Duration // Delay before each request
}

// GenerateRealisticSequence creates a sequence that mimics real IDE usage
func (rg *RequestGenerator) GenerateRealisticSequence() *RequestSequence {
	fileURI := rg.randomFileURI()
	position := rg.randomPosition()

	// Simulate a user opening a file, hovering, then going to definition
	requests := []*JSONRPCRequest{
		{
			JSONRPC: "2.0",
			ID:      rg.nextRequestID(),
			Method:  "textDocument/documentSymbol",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
			},
		},
		{
			JSONRPC: "2.0",
			ID:      rg.nextRequestID(),
			Method:  "textDocument/hover",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
				"position": position,
			},
		},
		{
			JSONRPC: "2.0",
			ID:      rg.nextRequestID(),
			Method:  "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
				"position": position,
			},
		},
		{
			JSONRPC: "2.0",
			ID:      rg.nextRequestID(),
			Method:  "textDocument/references",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
				"position": position,
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			},
		},
	}

	delays := []time.Duration{
		0,                      // No delay for first request
		500 * time.Millisecond, // User hovers after seeing symbols
		200 * time.Millisecond, // Quick go-to-definition
		300 * time.Millisecond, // Then find references
	}

	return &RequestSequence{
		Name:     "realistic_ide_usage",
		Requests: requests,
		Delays:   delays,
	}
}

// SendSequence sends a sequence of requests with appropriate delays
func (rg *RequestGenerator) SendSequence(ctx context.Context, sequence *RequestSequence) ([]*JSONRPCResponse, error) {
	responses := make([]*JSONRPCResponse, len(sequence.Requests))

	for i, req := range sequence.Requests {
		// Apply delay if specified
		if i < len(sequence.Delays) && sequence.Delays[i] > 0 {
			select {
			case <-ctx.Done():
				return responses, ctx.Err()
			case <-time.After(sequence.Delays[i]):
			}
		}

		resp, err := rg.SendRequest(ctx, req)
		if err != nil {
			return responses, fmt.Errorf("request %d in sequence failed: %w", i, err)
		}

		responses[i] = resp
	}

	return responses, nil
}

// BenchmarkConfig configures request generation for benchmarking
type BenchmarkConfig struct {
	RequestCount    int
	ConcurrentUsers int
	PatternWeights  map[string]int
	IncludeErrors   bool
	ValidationLevel ValidationLevel
}

// ValidationLevel defines how strict response validation should be
type ValidationLevel int

const (
	ValidationNone ValidationLevel = iota
	ValidationBasic
	ValidationStrict
)

// GenerateBenchmarkRequests creates a set of requests for benchmarking
func (rg *RequestGenerator) GenerateBenchmarkRequests(config *BenchmarkConfig) []*JSONRPCRequest {
	patterns := rg.GetDefaultRequestPatterns()

	// Adjust weights based on config
	if config.PatternWeights != nil {
		for i := range patterns {
			if weight, exists := config.PatternWeights[patterns[i].Method]; exists {
				patterns[i].Weight = weight
			}
		}
	}

	requests := make([]*JSONRPCRequest, config.RequestCount)
	for i := 0; i < config.RequestCount; i++ {
		requests[i] = rg.GenerateRequest(patterns)
	}

	return requests
}
