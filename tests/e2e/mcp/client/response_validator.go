package client

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mcp "lsp-gateway/tests/mcp"
)

type ResponseValidator struct {
	logger             *TestLogger
	schemaValidator    *SchemaValidator
	protocolValidator  *ProtocolValidator
	contentValidator   *ContentValidator
	metrics            *ValidationMetrics
	
	validationRules    []ResponseValidationRule
	enabled            bool
	strictMode         bool
	mu                 sync.RWMutex
}

type SchemaValidator struct {
	logger      *TestLogger
	schemas     map[string]interface{}
	cacheHits   int64
	cacheMisses int64
	mu          sync.RWMutex
}

type ProtocolValidator struct {
	logger             *TestLogger
	supportedVersions  []string
	requiredFields     map[string][]string
	mu                 sync.RWMutex
}

type ContentValidator struct {
	logger           *TestLogger
	maxContentSize   int
	allowedTypes     map[string]bool
	validationCache  map[string]bool
	mu               sync.RWMutex
}

type ResponseValidationRule struct {
	Name        string
	Description string
	Validator   func(*mcp.MCPResponse) *ValidationResult
	Severity    ValidationSeverity
	Enabled     bool
}

type ValidationResult struct {
	Valid        bool
	Errors       []string
	Warnings     []string
	Suggestions  []string
	Context      map[string]interface{}
}

type ErrorCategory int

const (
	ErrorCategoryProtocol ErrorCategory = iota
	ErrorCategorySchema
	ErrorCategoryContent
	ErrorCategoryStructure
	ErrorCategorySemantic
	ErrorCategoryPerformance
)

func (e ErrorCategory) String() string {
	switch e {
	case ErrorCategoryProtocol:
		return "Protocol"
	case ErrorCategorySchema:
		return "Schema"
	case ErrorCategoryContent:
		return "Content"
	case ErrorCategoryStructure:
		return "Structure"
	case ErrorCategorySemantic:
		return "Semantic"
	case ErrorCategoryPerformance:
		return "Performance"
	default:
		return "Unknown"
	}
}

type StructuredValidationError struct {
	Category    ErrorCategory
	Code        string
	Message     string
	Details     string
	Context     map[string]interface{}
	Suggestions []string
	Retryable   bool
	Timestamp   time.Time
}

func (e *StructuredValidationError) Error() string {
	return fmt.Sprintf("[%s:%s] %s", e.Category, e.Code, e.Message)
}

const (
	MaxContentSize        = 50 * 1024 * 1024 // 50MB
	ValidationCacheSize   = 1000
	ValidationTimeout     = 5 * time.Second
)

func NewResponseValidator(logger *TestLogger) *ResponseValidator {
	rv := &ResponseValidator{
		logger:      logger,
		metrics:     &ValidationMetrics{
			ErrorsByRule: make(map[string]int64),
		},
		enabled:     true,
		strictMode:  true,
	}
	
	rv.schemaValidator = NewSchemaValidator(logger)
	rv.protocolValidator = NewProtocolValidator(logger)
	rv.contentValidator = NewContentValidator(logger)
	
	rv.validationRules = []ResponseValidationRule{
		{
			Name:        "JSONRPCCompliance",
			Description: "Validates JSON-RPC 2.0 protocol compliance",
			Validator:   rv.validateJSONRPCCompliance,
			Severity:    ValidationError,
			Enabled:     true,
		},
		{
			Name:        "ResponseStructure",
			Description: "Validates response structure and required fields",
			Validator:   rv.validateResponseStructure,
			Severity:    ValidationError,
			Enabled:     true,
		},
		{
			Name:        "ContentIntegrity",
			Description: "Validates response content integrity and format",
			Validator:   rv.validateContentIntegrity,
			Severity:    ValidationWarning,
			Enabled:     true,
		},
		{
			Name:        "ErrorFormat",
			Description: "Validates error response format and structure",
			Validator:   rv.validateErrorFormat,
			Severity:    ValidationError,
			Enabled:     true,
		},
		{
			Name:        "ToolResultSchema",
			Description: "Validates tool result schema compliance",
			Validator:   rv.validateToolResultSchema,
			Severity:    ValidationWarning,
			Enabled:     true,
		},
		{
			Name:        "ContentSize",
			Description: "Validates response content size limits",
			Validator:   rv.validateContentSize,
			Severity:    ValidationWarning,
			Enabled:     true,
		},
	}
	
	return rv
}

func (rv *ResponseValidator) ValidateResponse(response *mcp.MCPResponse) error {
	if !rv.enabled {
		return nil
	}
	
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		atomic.AddInt64(&rv.metrics.TotalValidations, 1)
		rv.logger.Debug("Response validation completed", "duration", duration)
	}()
	
	validationErrors := make([]*StructuredValidationError, 0)
	validationWarnings := make([]*StructuredValidationError, 0)
	
	for _, rule := range rv.validationRules {
		if !rule.Enabled {
			continue
		}
		
		result := rule.Validator(response)
		if result == nil {
			continue
		}
		
		if !result.Valid {
			for _, errMsg := range result.Errors {
				structuredErr := &StructuredValidationError{
					Category:    rv.categorizeValidationError(rule.Name, errMsg),
					Code:        rule.Name,
					Message:     errMsg,
					Context:     result.Context,
					Suggestions: result.Suggestions,
					Retryable:   false,
					Timestamp:   time.Now(),
				}
				
				if rule.Severity == ValidationError {
					validationErrors = append(validationErrors, structuredErr)
				} else {
					validationWarnings = append(validationWarnings, structuredErr)
				}
			}
			
			for _, warnMsg := range result.Warnings {
				structuredWarn := &StructuredValidationError{
					Category:    rv.categorizeValidationError(rule.Name, warnMsg),
					Code:        rule.Name,
					Message:     warnMsg,
					Context:     result.Context,
					Suggestions: result.Suggestions,
					Retryable:   true,
					Timestamp:   time.Now(),
				}
				validationWarnings = append(validationWarnings, structuredWarn)
			}
			
			rv.recordRuleViolation(rule.Name)
		}
	}
	
	if len(validationWarnings) > 0 {
		atomic.AddInt64(&rv.metrics.ValidationWarnings, 1)
		rv.logger.Warn("Response validation warnings", "count", len(validationWarnings), "warnings", validationWarnings)
	}
	
	if len(validationErrors) > 0 {
		atomic.AddInt64(&rv.metrics.ValidationErrors, 1)
		
		if rv.strictMode {
			errorMessages := make([]string, len(validationErrors))
			for i, err := range validationErrors {
				errorMessages[i] = err.Error()
			}
			return fmt.Errorf("response validation failed: %v", errorMessages)
		}
		
		rv.logger.Error("Response validation errors (non-strict mode)", "count", len(validationErrors), "errors", validationErrors)
	}
	
	return nil
}

func (rv *ResponseValidator) validateJSONRPCCompliance(response *mcp.MCPResponse) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Context:  make(map[string]interface{}),
	}
	
	if response.JSONRPC != mcp.JSONRPCVersion {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("invalid JSON-RPC version: expected %s, got %s", mcp.JSONRPCVersion, response.JSONRPC))
		result.Suggestions = append(result.Suggestions, fmt.Sprintf("Set JSONRPC field to %s", mcp.JSONRPCVersion))
	}
	
	if response.ID == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "response missing required ID field")
		result.Suggestions = append(result.Suggestions, "Include ID field matching the original request")
	}
	
	if response.Result == nil && response.Error == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "response must have either result or error field")
		result.Suggestions = append(result.Suggestions, "Include either a result or error field in the response")
	}
	
	if response.Result != nil && response.Error != nil {
		result.Valid = false
		result.Errors = append(result.Errors, "response cannot have both result and error fields")
		result.Suggestions = append(result.Suggestions, "Include only one of result or error field")
	}
	
	return result
}

func (rv *ResponseValidator) validateResponseStructure(response *mcp.MCPResponse) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Context:  make(map[string]interface{}),
	}
	
	if response.ID != nil {
		switch id := response.ID.(type) {
		case string:
			if len(id) == 0 {
				result.Valid = false
				result.Errors = append(result.Errors, "empty string response ID")
			}
		case float64:
			if id < 0 {
				result.Warnings = append(result.Warnings, "negative numeric response ID")
			}
		default:
			result.Warnings = append(result.Warnings, fmt.Sprintf("unusual response ID type: %T", id))
		}
	}
	
	return result
}

func (rv *ResponseValidator) validateContentIntegrity(response *mcp.MCPResponse) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Context:  make(map[string]interface{}),
	}
	
	if response.Result != nil {
		if err := rv.contentValidator.ValidateContent(response.Result); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("result content validation failed: %v", err))
		}
	}
	
	return result
}

func (rv *ResponseValidator) validateErrorFormat(response *mcp.MCPResponse) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Context:  make(map[string]interface{}),
	}
	
	if response.Error != nil {
		if response.Error.Code == 0 {
			result.Warnings = append(result.Warnings, "error code is zero, should be a valid JSON-RPC error code")
		}
		
		if response.Error.Message == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "error message is empty")
			result.Suggestions = append(result.Suggestions, "Provide a descriptive error message")
		}
		
		if response.Error.Code < -32768 || response.Error.Code > -32000 {
			if response.Error.Code < -32099 || response.Error.Code > -32000 {
				result.Warnings = append(result.Warnings, "error code outside standard JSON-RPC range")
			}
		}
	}
	
	return result
}

func (rv *ResponseValidator) validateToolResultSchema(response *mcp.MCPResponse) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Context:  make(map[string]interface{}),
	}
	
	if response.Result != nil {
		if err := rv.schemaValidator.ValidateToolResult(response.Result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("tool result schema validation: %v", err))
		}
	}
	
	return result
}

func (rv *ResponseValidator) validateContentSize(response *mcp.MCPResponse) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
		Context:  make(map[string]interface{}),
	}
	
	if response.Result != nil {
		resultBytes, err := json.Marshal(response.Result)
		if err != nil {
			result.Warnings = append(result.Warnings, "failed to marshal result for size check")
			return result
		}
		
		size := len(resultBytes)
		result.Context["resultSize"] = size
		
		if size > MaxContentSize {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("result size %d exceeds maximum %d", size, MaxContentSize))
			result.Suggestions = append(result.Suggestions, "Reduce result content size or implement pagination")
		} else if size > MaxContentSize/2 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("result size %d is large, consider optimization", size))
		}
	}
	
	return result
}

func (rv *ResponseValidator) categorizeValidationError(ruleName, errorMessage string) ErrorCategory {
	if strings.Contains(strings.ToLower(ruleName), "protocol") || strings.Contains(strings.ToLower(ruleName), "jsonrpc") {
		return ErrorCategoryProtocol
	}
	if strings.Contains(strings.ToLower(ruleName), "schema") {
		return ErrorCategorySchema
	}
	if strings.Contains(strings.ToLower(ruleName), "content") {
		return ErrorCategoryContent
	}
	if strings.Contains(strings.ToLower(ruleName), "structure") {
		return ErrorCategoryStructure
	}
	if strings.Contains(strings.ToLower(errorMessage), "size") || strings.Contains(strings.ToLower(errorMessage), "performance") {
		return ErrorCategoryPerformance
	}
	return ErrorCategorySemantic
}

func (rv *ResponseValidator) recordRuleViolation(ruleName string) {
	rv.metrics.mu.Lock()
	defer rv.metrics.mu.Unlock()
	rv.metrics.ErrorsByRule[ruleName]++
}

func (rv *ResponseValidator) GetMetrics() *ValidationMetrics {
	rv.metrics.mu.RLock()
	defer rv.metrics.mu.RUnlock()
	
	errorsByRule := make(map[string]int64)
	for rule, count := range rv.metrics.ErrorsByRule {
		errorsByRule[rule] = count
	}
	
	return &ValidationMetrics{
		TotalValidations:   atomic.LoadInt64(&rv.metrics.TotalValidations),
		ValidationErrors:   atomic.LoadInt64(&rv.metrics.ValidationErrors),
		ValidationWarnings: atomic.LoadInt64(&rv.metrics.ValidationWarnings),
		ErrorsByRule:       errorsByRule,
	}
}

func (rv *ResponseValidator) SetEnabled(enabled bool) {
	rv.mu.Lock()
	defer rv.mu.Unlock()
	rv.enabled = enabled
}

func (rv *ResponseValidator) SetStrictMode(strict bool) {
	rv.mu.Lock()
	defer rv.mu.Unlock()
	rv.strictMode = strict
}

func (rv *ResponseValidator) EnableRule(ruleName string) error {
	rv.mu.Lock()
	defer rv.mu.Unlock()
	
	for i, rule := range rv.validationRules {
		if rule.Name == ruleName {
			rv.validationRules[i].Enabled = true
			return nil
		}
	}
	return fmt.Errorf("validation rule %s not found", ruleName)
}

func (rv *ResponseValidator) DisableRule(ruleName string) error {
	rv.mu.Lock()
	defer rv.mu.Unlock()
	
	for i, rule := range rv.validationRules {
		if rule.Name == ruleName {
			rv.validationRules[i].Enabled = false
			return nil
		}
	}
	return fmt.Errorf("validation rule %s not found", ruleName)
}

func NewSchemaValidator(logger *TestLogger) *SchemaValidator {
	return &SchemaValidator{
		logger:  logger,
		schemas: make(map[string]interface{}),
	}
}

func (sv *SchemaValidator) ValidateToolResult(result interface{}) error {
	if result == nil {
		return fmt.Errorf("nil result")
	}
	
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	
	var toolResult mcp.ToolResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		atomic.AddInt64(&sv.cacheMisses, 1)
		return fmt.Errorf("result does not match ToolResult schema: %w", err)
	}
	
	if len(toolResult.Content) == 0 {
		return fmt.Errorf("tool result missing content")
	}
	
	for i, content := range toolResult.Content {
		if content.Type == "" {
			return fmt.Errorf("content block %d missing type", i)
		}
		if content.Type == "text" && content.Text == "" {
			return fmt.Errorf("text content block %d missing text field", i)
		}
	}
	
	atomic.AddInt64(&sv.cacheHits, 1)
	return nil
}

func NewProtocolValidator(logger *TestLogger) *ProtocolValidator {
	return &ProtocolValidator{
		logger:            logger,
		supportedVersions: []string{mcp.JSONRPCVersion},
		requiredFields: map[string][]string{
			"response": {"jsonrpc", "id"},
			"error":    {"code", "message"},
		},
	}
}

func NewContentValidator(logger *TestLogger) *ContentValidator {
	return &ContentValidator{
		logger:         logger,
		maxContentSize: MaxContentSize,
		allowedTypes: map[string]bool{
			"text":   true,
			"image":  true,
			"data":   true,
			"json":   true,
			"binary": true,
		},
		validationCache: make(map[string]bool),
	}
}

func (cv *ContentValidator) ValidateContent(content interface{}) error {
	if content == nil {
		return fmt.Errorf("content is nil")
	}
	
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	
	if len(contentBytes) > cv.maxContentSize {
		return fmt.Errorf("content size %d exceeds maximum %d", len(contentBytes), cv.maxContentSize)
	}
	
	contentType := cv.detectContentType(content)
	if !cv.allowedTypes[contentType] {
		return fmt.Errorf("content type %s not allowed", contentType)
	}
	
	return nil
}

func (cv *ContentValidator) detectContentType(content interface{}) string {
	switch content.(type) {
	case string:
		return "text"
	case map[string]interface{}:
		return "json"
	case []byte:
		return "binary"
	case []interface{}:
		return "json"
	default:
		if reflect.TypeOf(content).Kind() == reflect.Struct {
			return "json"
		}
		return "data"
	}
}