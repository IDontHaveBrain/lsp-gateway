package types

import (
	"context"
	"encoding/json"
	"strings"
)

// LSPClient defines the unified interface for Language Server Protocol client operations.
// This interface consolidates all LSP client methods used throughout the application,
// including lifecycle management, request/notification handling, and capability checking.
type LSPClient interface {
	// Start initializes and starts the LSP server process.
	// Returns an error if the server fails to start or is already running.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the LSP server process.
	// Returns an error if the shutdown process fails.
	Stop() error

	// SendRequest sends a JSON-RPC request to the LSP server and waits for a response.
	// The method parameter specifies the LSP method name, params contains the request parameters,
	// and returns the raw JSON response or an error.
	SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)

	// SendNotification sends a JSON-RPC notification to the LSP server without expecting a response.
	// The method parameter specifies the LSP method name, params contains the notification parameters.
	// Returns an error if the notification fails to send.
	SendNotification(ctx context.Context, method string, params interface{}) error

	// IsActive returns true if the LSP server is currently running and responsive.
	IsActive() bool

	// Supports checks if the LSP server supports a specific method or capability.
	// Returns true if the method is supported, false otherwise.
	Supports(method string) bool
}

// SymbolRole represents bitset flags for different roles a symbol can have in source code.
// These values match the SCIP (SCIP Code Intelligence Protocol) specification exactly.
type SymbolRole int32

const (
	// Definition indicates this occurrence is the definition/declaration site of the symbol
	SymbolRoleDefinition SymbolRole = 0x1

	// Import indicates this occurrence is an import/include of the symbol
	SymbolRoleImport SymbolRole = 0x2

	// WriteAccess indicates this occurrence writes/modifies the symbol
	SymbolRoleWriteAccess SymbolRole = 0x4

	// ReadAccess indicates this occurrence reads/references the symbol
	SymbolRoleReadAccess SymbolRole = 0x8

	// Generated indicates this occurrence is in generated/synthetic code
	SymbolRoleGenerated SymbolRole = 0x10

	// Test indicates this occurrence is in test code
	SymbolRoleTest SymbolRole = 0x20

	// ForwardDefinition indicates this occurrence is a forward declaration
	SymbolRoleForwardDefinition SymbolRole = 0x40
)

// HasRole checks if the SymbolRole has the specified role bit set.
func (r SymbolRole) HasRole(role SymbolRole) bool {
	return (r & role) != 0
}

// AddRole adds the specified role bit to the SymbolRole.
func (r SymbolRole) AddRole(role SymbolRole) SymbolRole {
	return r | role
}

// RemoveRole removes the specified role bit from the SymbolRole.
func (r SymbolRole) RemoveRole(role SymbolRole) SymbolRole {
	return r &^ role
}

// String returns a human-readable representation of the SymbolRole.
func (r SymbolRole) String() string {
	var roles []string
	if r.HasRole(SymbolRoleDefinition) {
		roles = append(roles, "Definition")
	}
	if r.HasRole(SymbolRoleImport) {
		roles = append(roles, "Import")
	}
	if r.HasRole(SymbolRoleWriteAccess) {
		roles = append(roles, "WriteAccess")
	}
	if r.HasRole(SymbolRoleReadAccess) {
		roles = append(roles, "ReadAccess")
	}
	if r.HasRole(SymbolRoleGenerated) {
		roles = append(roles, "Generated")
	}
	if r.HasRole(SymbolRoleTest) {
		roles = append(roles, "Test")
	}
	if r.HasRole(SymbolRoleForwardDefinition) {
		roles = append(roles, "ForwardDefinition")
	}
	if len(roles) == 0 {
		return "None"
	}
	return strings.Join(roles, "|")
}

// SyntaxKind represents syntax highlighting categories following SCIP specification.
type SyntaxKind int32

const (
	SyntaxKindUnspecified SyntaxKind = iota
	SyntaxKindComment
	SyntaxKindKeyword
	SyntaxKindKeywordControlFlow
	SyntaxKindKeywordControlImport
	SyntaxKindKeywordControlConditional
	SyntaxKindKeywordControlException
	SyntaxKindKeywordControlLoop
	SyntaxKindKeywordControlReturn
	SyntaxKindKeywordFunction
	SyntaxKindKeywordStorage
	SyntaxKindKeywordStorageType
	SyntaxKindKeywordStorageModifier
	SyntaxKindKeywordOperator
	SyntaxKindIdentifierBuiltin
	SyntaxKindIdentifierNull
	SyntaxKindIdentifierConstant
	SyntaxKindIdentifierMutableGlobal
	SyntaxKindIdentifierParameter
	SyntaxKindIdentifierLocal
	SyntaxKindIdentifierShadowed
	SyntaxKindIdentifierNamespace
	SyntaxKindIdentifierModule
	SyntaxKindIdentifierFunction
	SyntaxKindIdentifierFunctionDefinition
	SyntaxKindIdentifierMacro
	SyntaxKindIdentifierMacroDefinition
	SyntaxKindIdentifierType
	SyntaxKindIdentifierBuiltinType
	SyntaxKindIdentifierAttribute
	SyntaxKindRegexEscape
	SyntaxKindRegexRepeated
	SyntaxKindRegexWildcard
	SyntaxKindRegexDelimiter
	SyntaxKindRegexJoin
	SyntaxKindStringLiteral
	SyntaxKindStringLiteralEscape
	SyntaxKindStringLiteralSpecial
	SyntaxKindStringLiteralKey
	SyntaxKindCharacterLiteral
	SyntaxKindNumericLiteral
	SyntaxKindBooleanLiteral
	SyntaxKindTag
	SyntaxKindTagAttribute
	SyntaxKindTagDelimiter
	SyntaxKindPunctuation
	SyntaxKindPunctuationDefinition
	SyntaxKindPunctuationBracket
	SyntaxKindPunctuationDelimiter
	SyntaxKindPunctuationSpecial
)

// DiagnosticSeverity represents the severity level of a diagnostic message.
type DiagnosticSeverity int32

const (
	DiagnosticSeverityUnspecified DiagnosticSeverity = iota
	DiagnosticSeverityError
	DiagnosticSeverityWarning
	DiagnosticSeverityInfo
	DiagnosticSeverityHint
)

// DiagnosticTag represents additional metadata for diagnostic messages.
type DiagnosticTag int32

const (
	DiagnosticTagUnspecified DiagnosticTag = iota
	DiagnosticTagUnnecessary
	DiagnosticTagDeprecated
)

// Position represents a text position with line and character information.
// Line and character are zero-based following LSP convention.
type Position struct {
	Line      int32 `json:"line"`
	Character int32 `json:"character"`
}

// Range represents a text range with start and end positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// SymbolOccurrence represents a single occurrence of a symbol in source code.
type SymbolOccurrence struct {
	// Range specifies the text range of this occurrence
	Range Range `json:"range"`

	// Symbol is the unique identifier for the symbol
	Symbol string `json:"symbol"`

	// SymbolRoles indicates the roles this symbol plays at this occurrence
	SymbolRoles SymbolRole `json:"symbol_roles"`

	// OverrideDocumentation can override symbol documentation for this specific occurrence
	OverrideDocumentation []string `json:"override_documentation,omitempty"`

	// SyntaxKind specifies the syntax highlighting category for this occurrence
	SyntaxKind SyntaxKind `json:"syntax_kind,omitempty"`

	// Diagnostics contains diagnostic messages associated with this occurrence
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Diagnostic represents a diagnostic message (error, warning, etc.) in source code.
type Diagnostic struct {
	// Severity indicates the severity level of this diagnostic
	Severity DiagnosticSeverity `json:"severity"`

	// Code is an optional diagnostic code
	Code string `json:"code,omitempty"`

	// CodeDescription provides additional information about the diagnostic code
	CodeDescription *CodeDescription `json:"code_description,omitempty"`

	// Source indicates the source of this diagnostic (e.g., "typescript", "eslint")
	Source string `json:"source,omitempty"`

	// Message is the diagnostic message text
	Message string `json:"message"`

	// Tags provide additional metadata about the diagnostic
	Tags []DiagnosticTag `json:"tags,omitempty"`

	// RelatedInformation provides additional diagnostic information
	RelatedInformation []DiagnosticRelatedInformation `json:"related_information,omitempty"`

	// Data can contain additional data for the diagnostic
	Data interface{} `json:"data,omitempty"`
}

// CodeDescription provides additional information about a diagnostic code.
type CodeDescription struct {
	// URI points to documentation for the diagnostic code
	URI string `json:"uri"`
}

// DiagnosticRelatedInformation provides additional context for a diagnostic.
type DiagnosticRelatedInformation struct {
	// Location specifies where this related information applies
	Location Location `json:"location"`

	// Message is the related information message
	Message string `json:"message"`
}

// Location represents a location in a source file.
type Location struct {
	// URI is the file URI
	URI string `json:"uri"`

	// Range is the text range in the file
	Range Range `json:"range"`
}

// SymbolKind represents the kind of a symbol in LSP
type SymbolKind int

const (
	File          SymbolKind = 1
	Module        SymbolKind = 2
	Namespace     SymbolKind = 3
	Package       SymbolKind = 4
	Class         SymbolKind = 5
	Method        SymbolKind = 6
	Property      SymbolKind = 7
	Field         SymbolKind = 8
	Constructor   SymbolKind = 9
	Enum          SymbolKind = 10
	Interface     SymbolKind = 11
	Function      SymbolKind = 12
	Variable      SymbolKind = 13
	Constant      SymbolKind = 14
	String        SymbolKind = 15
	Number        SymbolKind = 16
	Boolean       SymbolKind = 17
	Array         SymbolKind = 18
	Object        SymbolKind = 19
	Key           SymbolKind = 20
	Null          SymbolKind = 21
	EnumMember    SymbolKind = 22
	Struct        SymbolKind = 23
	Event         SymbolKind = 24
	Operator      SymbolKind = 25
	TypeParameter SymbolKind = 26
)

// SymbolInformation represents information about a symbol from LSP
type SymbolInformation struct {
	Name           string     `json:"name"`
	Kind           SymbolKind `json:"kind"`
	Tags           []int      `json:"tags,omitempty"`
	Deprecated     bool       `json:"deprecated,omitempty"`
	Location       Location   `json:"location"`
	ContainerName  string     `json:"containerName,omitempty"`
	SelectionRange *Range     `json:"-"` // Not part of LSP spec, used internally for DocumentSymbol conversion
}
