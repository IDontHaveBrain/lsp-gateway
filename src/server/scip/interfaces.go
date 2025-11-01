// Package scip contains types and helpers for SCIP integration.
package scip

import (
	"context"
	"time"

	"lsp-gateway/src/internal/types"
)

// SCIPDocument represents a SCIP document with occurrences and symbol information.
// This follows the SCIP protocol's occurrence-centric design where a document
// contains occurrences of symbols and metadata about those symbols.
type SCIPDocument struct {
	// URI is the unique identifier for this document
	URI string

	// Language specifies the programming language (go, python, typescript, etc.)
	Language string

	// Content contains the raw document content
	Content []byte

	// Occurrences contains all symbol occurrences in this document.
	// Each occurrence represents a specific location where a symbol appears.
	Occurrences []SCIPOccurrence

	// SymbolInformation contains metadata about symbols referenced in this document.
	// This includes documentation, relationships, and display information.
	SymbolInformation []SCIPSymbolInformation

	// LastModified tracks when this document was last updated
	LastModified time.Time

	// Size is the document size in bytes
	Size int64
}

// SCIPOccurrence represents a single occurrence of a symbol in source code.
// This is the core unit of SCIP's occurrence-centric design.
type SCIPOccurrence struct {
	// Range specifies the text range where this symbol occurrence appears
	Range types.Range

	// SelectionRange specifies the range of just the identifier itself (optional)
	// This is useful when Range includes surrounding syntax (like "func" keyword)
	SelectionRange *types.Range

	// Symbol is the unique SCIP symbol identifier (e.g., "go package main/", "go method main.Function")
	Symbol string

	// SymbolRoles indicates what roles this symbol plays at this occurrence
	// (definition, reference, write access, etc.)
	SymbolRoles types.SymbolRole

	// OverrideDocumentation can provide occurrence-specific documentation
	OverrideDocumentation []string

	// SyntaxKind specifies syntax highlighting category for this occurrence
	SyntaxKind types.SyntaxKind

	// Diagnostics contains any diagnostic messages associated with this occurrence
	Diagnostics []types.Diagnostic
}

// SCIPSymbolInformation contains metadata about a symbol.
// Unlike occurrences, this provides global information about the symbol itself.
type SCIPSymbolInformation struct {
	// Symbol is the unique SCIP symbol identifier
	Symbol string

	// Documentation provides symbol documentation (comments, docstrings, etc.)
	Documentation []string

	// Relationships describes how this symbol relates to other symbols
	Relationships []SCIPRelationship

	// Kind specifies the symbol kind (class, method, variable, etc.)
	Kind SCIPSymbolKind

	// DisplayName is the human-readable name for this symbol
	DisplayName string

	// SignatureDocumentation provides signature-specific documentation
	SignatureDocumentation SCIPSignatureDocumentation

	// Range is the full range of the symbol (definition span)
	Range types.Range

	// SelectionRange is the precise identifier span within Range
	SelectionRange *types.Range
}

// SCIPRelationship describes how one symbol relates to another.
// This enables features like "go to implementation" and "find type definition".
type SCIPRelationship struct {
	// Symbol is the target symbol this relationship points to
	Symbol string

	// IsReference indicates this is a reference relationship
	IsReference bool

	// IsImplementation indicates this symbol implements the target
	IsImplementation bool

	// IsTypeDefinition indicates this is a type definition relationship
	IsTypeDefinition bool

	// IsDefinition indicates this is the definition of the target
	IsDefinition bool
}

// SCIPSymbolKind represents the kind of a symbol (function, class, variable, etc.)
type SCIPSymbolKind int32

const (
	SCIPSymbolKindUnknown SCIPSymbolKind = iota
	SCIPSymbolKindFile
	SCIPSymbolKindModule
	SCIPSymbolKindNamespace
	SCIPSymbolKindPackage
	SCIPSymbolKindClass
	SCIPSymbolKindMethod
	SCIPSymbolKindProperty
	SCIPSymbolKindField
	SCIPSymbolKindConstructor
	SCIPSymbolKindEnum
	SCIPSymbolKindInterface
	SCIPSymbolKindFunction
	SCIPSymbolKindVariable
	SCIPSymbolKindConstant
	SCIPSymbolKindString
	SCIPSymbolKindNumber
	SCIPSymbolKindBoolean
	SCIPSymbolKindArray
	SCIPSymbolKindObject
	SCIPSymbolKindKey
	SCIPSymbolKindNull
	SCIPSymbolKindEnumMember
	SCIPSymbolKindStruct
	SCIPSymbolKindEvent
	SCIPSymbolKindOperator
	SCIPSymbolKindTypeParameter
)

// OccurrenceWithDocument pairs an occurrence with its document URI for fast lookup
type OccurrenceWithDocument struct {
	SCIPOccurrence
	DocumentURI string
}

// SCIPSignatureDocumentation provides detailed signature documentation
type SCIPSignatureDocumentation struct {
	// Text is the main documentation text
	Text string

	// Language specifies the language for syntax highlighting
	Language string

	// Parameters documents individual parameters
	Parameters []SCIPParameterDocumentation

	// Returns documents the return value
	Returns SCIPReturnDocumentation
}

// SCIPParameterDocumentation documents a function/method parameter
type SCIPParameterDocumentation struct {
	// Name is the parameter name
	Name string

	// Documentation describes the parameter
	Documentation string
}

// SCIPReturnDocumentation documents a function/method return value
type SCIPReturnDocumentation struct {
	// Documentation describes the return value
	Documentation string
}

// SCIPPackage represents a SCIP package descriptor
type SCIPPackage struct {
	// Manager specifies the package manager ("npm", "pip", "maven", etc.)
	Manager string

	// Name is the package name
	Name string

	// Version is the package version
	Version string
}

// SCIPDescriptor represents a SCIP symbol descriptor for structured symbol names
type SCIPDescriptor struct {
	// Name is the symbol name
	Name string

	// Disambiguator helps distinguish symbols with same name
	Disambiguator string

	// Suffix provides additional symbol information
	Suffix SCIPDescriptorSuffix
}

// SCIPDescriptorSuffix specifies the type of symbol
type SCIPDescriptorSuffix int32

const (
	SCIPDescriptorSuffixUnspecified SCIPDescriptorSuffix = iota
	SCIPDescriptorSuffixNamespace
	SCIPDescriptorSuffixType
	SCIPDescriptorSuffixTerm
	SCIPDescriptorSuffixMethod
	SCIPDescriptorSuffixTypeParameter
	SCIPDescriptorSuffixParameter
	SCIPDescriptorSuffixMeta
	SCIPDescriptorSuffixMacro
)

// SCIPIndex represents the top-level SCIP index containing all documents
type SCIPIndex struct {
	// Metadata contains index metadata
	Metadata SCIPMetadata

	// Documents contains all indexed documents
	Documents []SCIPDocument

	// ExternalSymbols contains references to symbols defined outside this index
	ExternalSymbols []SCIPSymbolInformation
}

// SCIPMetadata contains metadata about the SCIP index
type SCIPMetadata struct {
	// Version is the SCIP protocol version
	Version SCIPProtocolVersion

	// ToolInfo describes the tool that generated this index
	ToolInfo SCIPToolInfo

	// ProjectRoot is the root directory of the indexed project
	ProjectRoot string

	// TextDocumentEncoding specifies text encoding (typically UTF-8)
	TextDocumentEncoding string
}

// SCIPProtocolVersion represents the SCIP protocol version
type SCIPProtocolVersion int32

const (
	SCIPProtocolVersionUnspecified SCIPProtocolVersion = iota
	SCIPProtocolVersion1
)

// SCIPToolInfo describes the tool that generated the SCIP index
type SCIPToolInfo struct {
	// Name is the tool name (e.g., "lsp-gateway")
	Name string

	// Version is the tool version
	Version string

	// Arguments contains the command-line arguments used
	Arguments []string
}

// SCIPStorageStats provides storage statistics and health information
type SCIPStorageStats struct {
	// Memory usage statistics
	MemoryUsage int64
	DiskUsage   int64
	MemoryLimit int64
	HitRate     float64

	// Document statistics
	CachedDocuments  int
	TotalOccurrences int64
	TotalSymbols     int64
	TotalReferences  int64
	UniqueSymbols    int

	// Cache performance
	HotCacheSize  int
	CacheHits     int64
	CacheMisses   int64
	EvictionCount int64
}

// SCIPStorageConfig defines storage configuration
type SCIPStorageConfig struct {
	MemoryLimit        int64         // Memory limit in bytes (default: 256MB)
	DiskCacheDir       string        // Directory for disk storage
	CompressionType    string        // Compression type for disk storage
	CompactionInterval time.Duration // Compaction interval
	MaxDocumentAge     time.Duration // Max age for cached documents
	EnableMetrics      bool          // Enable metrics collection
}

// IndexStats provides statistics about the SCIP storage
type IndexStats struct {
	TotalDocuments   int
	TotalOccurrences int64
	TotalSymbols     int64
	MemoryUsage      int64
	HitRate          float64
}

// SCIPDocumentStorage interface defines the simplified storage operations for SCIP indexes.
// This interface focuses on core functionality with consistent method naming.
type SCIPDocumentStorage interface {
	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Document operations
	StoreDocument(ctx context.Context, doc *SCIPDocument) error
	GetDocument(ctx context.Context, uri string) (*SCIPDocument, error)
	RemoveDocument(ctx context.Context, uri string) error
	ListDocuments(ctx context.Context) ([]string, error)

	// Occurrence operations - Always return arrays
	GetDefinitions(ctx context.Context, symbolID string) ([]SCIPOccurrence, error)
	GetReferences(ctx context.Context, symbolID string) ([]SCIPOccurrence, error)
	GetOccurrences(ctx context.Context, symbolID string) ([]SCIPOccurrence, error)

	// Symbol operations
	GetSymbolInfo(ctx context.Context, symbolID string) (*SCIPSymbolInformation, error)
	SearchSymbols(ctx context.Context, query string, limit int) ([]SCIPSymbolInformation, error)

	// Batch operations
	AddOccurrences(ctx context.Context, uri string, occurrences []SCIPOccurrence) error

	// Index management
	GetIndexStats() IndexStats
	ClearIndex(ctx context.Context) error

	// Convenience: retrieve occurrences with document URIs
	GetReferencesWithDocuments(ctx context.Context, symbolID string) ([]OccurrenceWithDocument, error)
	GetDefinitionsWithDocuments(ctx context.Context, symbolID string) ([]OccurrenceWithDocument, error)
	// Optional: all occurrences with document URIs
	GetOccurrencesWithDocuments(ctx context.Context, symbolID string) ([]OccurrenceWithDocument, error)
}
