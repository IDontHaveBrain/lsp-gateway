package workspace

import (
	"fmt"
	"time"
)

type RequestClassifier struct {
	supportedMethods map[string]*MethodInfo
}

type MethodInfo struct {
	Name                string                    `json:"name"`
	RequiresFileURI     bool                      `json:"requires_file_uri"`
	RequiresPosition    bool                      `json:"requires_position"`
	SupportsWorkspace   bool                      `json:"supports_workspace"`
	DefaultTimeout      time.Duration             `json:"default_timeout"`
	URIExtractorFunc    func(params interface{}) (string, error) `json:"-"`
	ParameterStructure  map[string]interface{}    `json:"parameter_structure"`
	Description         string                    `json:"description"`
}

func NewRequestClassifier() *RequestClassifier {
	classifier := &RequestClassifier{
		supportedMethods: make(map[string]*MethodInfo),
	}

	classifier.initializeSupportedMethods()
	return classifier
}

func (rc *RequestClassifier) initializeSupportedMethods() {
	rc.supportedMethods = map[string]*MethodInfo{
		"textDocument/definition": {
			Name:               "textDocument/definition",
			RequiresFileURI:    true,
			RequiresPosition:   true,
			SupportsWorkspace:  false,
			DefaultTimeout:     5 * time.Second,
			ParameterStructure: map[string]interface{}{
				"textDocument": map[string]string{"uri": "string"},
				"position":     map[string]interface{}{"line": "number", "character": "number"},
			},
			Description: "Navigate to the definition of a symbol at a specific position",
		},
		"textDocument/references": {
			Name:               "textDocument/references",
			RequiresFileURI:    true,
			RequiresPosition:   true,
			SupportsWorkspace:  false,
			DefaultTimeout:     10 * time.Second,
			ParameterStructure: map[string]interface{}{
				"textDocument": map[string]string{"uri": "string"},
				"position":     map[string]interface{}{"line": "number", "character": "number"},
				"context":      map[string]interface{}{"includeDeclaration": "boolean"},
			},
			Description: "Find all references to a symbol at a specific position",
		},
		"textDocument/hover": {
			Name:               "textDocument/hover",
			RequiresFileURI:    true,
			RequiresPosition:   true,
			SupportsWorkspace:  false,
			DefaultTimeout:     3 * time.Second,
			ParameterStructure: map[string]interface{}{
				"textDocument": map[string]string{"uri": "string"},
				"position":     map[string]interface{}{"line": "number", "character": "number"},
			},
			Description: "Get hover information for a symbol at a specific position",
		},
		"textDocument/documentSymbol": {
			Name:               "textDocument/documentSymbol",
			RequiresFileURI:    true,
			RequiresPosition:   false,
			SupportsWorkspace:  false,
			DefaultTimeout:     8 * time.Second,
			ParameterStructure: map[string]interface{}{
				"textDocument": map[string]string{"uri": "string"},
			},
			Description: "Get all symbols in a document",
		},
		"textDocument/completion": {
			Name:               "textDocument/completion",
			RequiresFileURI:    true,
			RequiresPosition:   true,
			SupportsWorkspace:  false,
			DefaultTimeout:     2 * time.Second,
			ParameterStructure: map[string]interface{}{
				"textDocument": map[string]string{"uri": "string"},
				"position":     map[string]interface{}{"line": "number", "character": "number"},
				"context":      map[string]interface{}{"triggerKind": "number", "triggerCharacter": "string"},
			},
			Description: "Provide code completion suggestions at a specific position",
		},
		"workspace/symbol": {
			Name:               "workspace/symbol",
			RequiresFileURI:    false,
			RequiresPosition:   false,
			SupportsWorkspace:  true,
			DefaultTimeout:     15 * time.Second,
			ParameterStructure: map[string]interface{}{
				"query": "string",
			},
			Description: "Search for symbols in the workspace",
		},
	}
}

func (rc *RequestClassifier) ClassifyRequest(method string, params interface{}) (*RequestClassification, error) {
	methodInfo, exists := rc.supportedMethods[method]
	if !exists {
		return nil, fmt.Errorf("unsupported LSP method: %s", method)
	}

	classification := &RequestClassification{
		Method:             method,
		MethodInfo:         methodInfo,
		IsSupported:        true,
		RequiresFileURI:    methodInfo.RequiresFileURI,
		RequiresPosition:   methodInfo.RequiresPosition,
		SupportsWorkspace:  methodInfo.SupportsWorkspace,
		EstimatedTimeout:   methodInfo.DefaultTimeout,
		ValidationErrors:   []string{},
	}

	if err := rc.validateParameters(methodInfo, params); err != nil {
		classification.ValidationErrors = append(classification.ValidationErrors, err.Error())
		classification.IsValid = false
	} else {
		classification.IsValid = true
	}

	return classification, nil
}

type RequestClassification struct {
	Method             string        `json:"method"`
	MethodInfo         *MethodInfo   `json:"method_info"`
	IsSupported        bool          `json:"is_supported"`
	IsValid            bool          `json:"is_valid"`
	RequiresFileURI    bool          `json:"requires_file_uri"`
	RequiresPosition   bool          `json:"requires_position"`
	SupportsWorkspace  bool          `json:"supports_workspace"`
	EstimatedTimeout   time.Duration `json:"estimated_timeout"`
	ValidationErrors   []string      `json:"validation_errors"`
}

func (rc *RequestClassifier) validateParameters(methodInfo *MethodInfo, params interface{}) error {
	if params == nil {
		if methodInfo.RequiresFileURI || methodInfo.RequiresPosition {
			return fmt.Errorf("parameters required for method %s", methodInfo.Name)
		}
		return nil
	}

	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid parameter format for method %s: expected map, got %T", methodInfo.Name, params)
	}

	if methodInfo.RequiresFileURI {
		if err := rc.validateTextDocumentURI(paramsMap); err != nil {
			return fmt.Errorf("textDocument URI validation failed for %s: %w", methodInfo.Name, err)
		}
	}

	if methodInfo.RequiresPosition {
		if err := rc.validatePosition(paramsMap); err != nil {
			return fmt.Errorf("position validation failed for %s: %w", methodInfo.Name, err)
		}
	}

	switch methodInfo.Name {
	case "textDocument/references":
		return rc.validateReferenceContext(paramsMap)
	case "textDocument/completion":
		return rc.validateCompletionContext(paramsMap)
	case "workspace/symbol":
		return rc.validateWorkspaceSymbolQuery(paramsMap)
	}

	return nil
}

func (rc *RequestClassifier) validateTextDocumentURI(params map[string]interface{}) error {
	textDoc, exists := params["textDocument"]
	if !exists {
		return fmt.Errorf("missing textDocument parameter")
	}

	textDocMap, ok := textDoc.(map[string]interface{})
	if !ok {
		return fmt.Errorf("textDocument parameter must be an object")
	}

	uri, exists := textDocMap["uri"]
	if !exists {
		return fmt.Errorf("missing textDocument.uri")
	}

	uriStr, ok := uri.(string)
	if !ok {
		return fmt.Errorf("textDocument.uri must be a string")
	}

	if uriStr == "" {
		return fmt.Errorf("textDocument.uri cannot be empty")
	}

	return nil
}

func (rc *RequestClassifier) validatePosition(params map[string]interface{}) error {
	position, exists := params["position"]
	if !exists {
		return fmt.Errorf("missing position parameter")
	}

	positionMap, ok := position.(map[string]interface{})
	if !ok {
		return fmt.Errorf("position parameter must be an object")
	}

	line, exists := positionMap["line"]
	if !exists {
		return fmt.Errorf("missing position.line")
	}

	if _, ok := line.(float64); !ok {
		return fmt.Errorf("position.line must be a number")
	}

	character, exists := positionMap["character"]
	if !exists {
		return fmt.Errorf("missing position.character")
	}

	if _, ok := character.(float64); !ok {
		return fmt.Errorf("position.character must be a number")
	}

	return nil
}

func (rc *RequestClassifier) validateReferenceContext(params map[string]interface{}) error {
	context, exists := params["context"]
	if !exists {
		return nil
	}

	contextMap, ok := context.(map[string]interface{})
	if !ok {
		return fmt.Errorf("reference context must be an object")
	}

	if includeDecl, exists := contextMap["includeDeclaration"]; exists {
		if _, ok := includeDecl.(bool); !ok {
			return fmt.Errorf("includeDeclaration must be a boolean")
		}
	}

	return nil
}

func (rc *RequestClassifier) validateCompletionContext(params map[string]interface{}) error {
	context, exists := params["context"]
	if !exists {
		return nil
	}

	contextMap, ok := context.(map[string]interface{})
	if !ok {
		return fmt.Errorf("completion context must be an object")
	}

	if triggerKind, exists := contextMap["triggerKind"]; exists {
		if _, ok := triggerKind.(float64); !ok {
			return fmt.Errorf("triggerKind must be a number")
		}
	}

	if triggerChar, exists := contextMap["triggerCharacter"]; exists {
		if _, ok := triggerChar.(string); !ok {
			return fmt.Errorf("triggerCharacter must be a string")
		}
	}

	return nil
}

func (rc *RequestClassifier) validateWorkspaceSymbolQuery(params map[string]interface{}) error {
	query, exists := params["query"]
	if !exists {
		return fmt.Errorf("missing query parameter for workspace/symbol")
	}

	if _, ok := query.(string); !ok {
		return fmt.Errorf("query parameter must be a string")
	}

	return nil
}

func (rc *RequestClassifier) IsMethodSupported(method string) bool {
	_, exists := rc.supportedMethods[method]
	return exists
}

func (rc *RequestClassifier) GetSupportedMethods() []string {
	methods := make([]string, 0, len(rc.supportedMethods))
	for method := range rc.supportedMethods {
		methods = append(methods, method)
	}
	return methods
}

func (rc *RequestClassifier) GetMethodInfo(method string) (*MethodInfo, bool) {
	info, exists := rc.supportedMethods[method]
	return info, exists
}

func (rc *RequestClassifier) GetMethodsByCapability(requiresFileURI, requiresPosition, supportsWorkspace bool) []string {
	var methods []string
	for method, info := range rc.supportedMethods {
		matches := true
		if requiresFileURI && !info.RequiresFileURI {
			matches = false
		}
		if requiresPosition && !info.RequiresPosition {
			matches = false
		}
		if supportsWorkspace && !info.SupportsWorkspace {
			matches = false
		}
		if matches {
			methods = append(methods, method)
		}
	}
	return methods
}

func (rc *RequestClassifier) EstimateComplexity(method string, params interface{}) (string, error) {
	if !rc.IsMethodSupported(method) {
		return "unknown", fmt.Errorf("unsupported method: %s", method)
	}

	switch method {
	case "textDocument/hover":
		return "low", nil
	case "textDocument/completion":
		return "low", nil
	case "textDocument/definition":
		return "medium", nil
	case "textDocument/documentSymbol":
		return "medium", nil
	case "textDocument/references":
		return "high", nil
	case "workspace/symbol":
		return "high", nil
	default:
		return "medium", nil
	}
}