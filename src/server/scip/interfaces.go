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
}

// SCIPCacheManager interface for cache-specific operations
type SCIPCacheManager interface {
	Evict(key string) error
	EvictLRU(count int) error
	Clear() error
	GetSize() int64
	GetStats() map[string]interface{}
}

// SCIPSymbolGenerator provides methods for generating and parsing SCIP symbol identifiers.
// SCIP symbols follow the format: <scheme> ' ' <package> ' ' (<descriptor>)+
// Example: "go github.com/owner/repo v1.0.0 `main.go`/Function#"
type SCIPSymbolGenerator interface {
	// GenerateSCIPSymbol creates a SCIP symbol ID from components
	// Format: <scheme> ' ' <package> ' ' (<descriptor>)+
	GenerateSCIPSymbol(scheme, packageName, version string, descriptors []SCIPDescriptor) (string, error)

	// ParseSCIPSymbol parses a SCIP symbol ID into its components
	// Returns scheme, package name, version, and descriptors
	ParseSCIPSymbol(symbolID string) (scheme, packageName, version string, descriptors []SCIPDescriptor, err error)

	// FormatSymbolID formats symbol components into SCIP string format
	FormatSymbolID(scheme, packageName, version string, descriptors []SCIPDescriptor) string

	// CreateLocalSymbol creates local symbol IDs for document-scoped symbols
	// Local symbols are relative to a specific document
	CreateLocalSymbol(documentURI string, name string, suffix SCIPDescriptorSuffix) string

	// CreatePackageSymbol creates a symbol for a package/module
	CreatePackageSymbol(scheme, packageName, version string) string
}

// SCIPSymbolUtility provides utility methods for working with SCIP symbols
type SCIPSymbolUtility interface {
	// IsLocalSymbol checks if a symbol is local to a document (no package)
	IsLocalSymbol(symbolID string) bool

	// IsExternalSymbol checks if a symbol is defined outside the current index
	IsExternalSymbol(symbolID string, currentPackage string) bool

	// GetSymbolPackage extracts package information from a symbol ID
	GetSymbolPackage(symbolID string) (*SCIPPackage, error)

	// GetSymbolDisplayName extracts the human-readable display name from a symbol
	GetSymbolDisplayName(symbolID string) string

	// ValidateSCIPSymbol validates that a symbol ID follows SCIP format
	ValidateSCIPSymbol(symbolID string) error

	// GetSymbolScheme extracts the scheme (language) from a symbol ID
	GetSymbolScheme(symbolID string) string

	// GetSymbolDescriptors extracts descriptors from a symbol ID
	GetSymbolDescriptors(symbolID string) ([]SCIPDescriptor, error)

	// CompareSymbols compares two symbols for equality, ignoring version differences
	CompareSymbols(symbol1, symbol2 string) bool
}

// SCIPDescriptorBuilder provides methods for building SCIP descriptors
type SCIPDescriptorBuilder interface {
	// NewDescriptor creates a descriptor with name, disambiguator, and suffix
	NewDescriptor(name, disambiguator string, suffix SCIPDescriptorSuffix) SCIPDescriptor

	// BuildMethodDescriptor creates a method descriptor with parameter information
	// Parameters are encoded in the disambiguator for overload resolution
	BuildMethodDescriptor(name string, parameters []string, returnType string) SCIPDescriptor

	// BuildTypeDescriptor creates a type descriptor (class, struct, interface, etc.)
	BuildTypeDescriptor(typeName string, typeKind SCIPSymbolKind) SCIPDescriptor

	// BuildNamespaceDescriptor creates a namespace/module descriptor
	BuildNamespaceDescriptor(namespaceName string) SCIPDescriptor

	// BuildFieldDescriptor creates a field/property descriptor
	BuildFieldDescriptor(fieldName string, isStatic bool) SCIPDescriptor

	// BuildParameterDescriptor creates a parameter descriptor
	BuildParameterDescriptor(paramName string, position int) SCIPDescriptor

	// BuildLocalDescriptor creates a descriptor for local variables
	BuildLocalDescriptor(varName string, scopeID string) SCIPDescriptor

	// FormatDescriptor formats a descriptor into its string representation
	FormatDescriptor(descriptor SCIPDescriptor) string

	// ParseDescriptor parses a descriptor string back into a SCIPDescriptor
	ParseDescriptor(descriptorStr string) (SCIPDescriptor, error)
}

// SCIPExternalSymbolManager manages external symbols (symbols defined outside the current index)
type SCIPExternalSymbolManager interface {
	// RegisterExternalSymbol adds an external symbol to the index
	RegisterExternalSymbol(symbolInfo *SCIPSymbolInformation) error

	// ResolveExternalSymbol resolves external symbol information
	ResolveExternalSymbol(ctx context.Context, symbolID string) (*SCIPSymbolInformation, error)

	// GetExternalSymbols returns all external symbols in the index
	GetExternalSymbols(ctx context.Context) ([]SCIPSymbolInformation, error)

	// GetExternalSymbolsByPackage returns external symbols from a specific package
	GetExternalSymbolsByPackage(ctx context.Context, packageName string) ([]SCIPSymbolInformation, error)

	// IsExternalSymbolCached checks if an external symbol is already cached
	IsExternalSymbolCached(symbolID string) bool

	// UpdateExternalSymbol updates information for an external symbol
	UpdateExternalSymbol(symbolInfo *SCIPSymbolInformation) error

	// RemoveExternalSymbol removes an external symbol from the index
	RemoveExternalSymbol(symbolID string) error

	// GetExternalDependencies returns packages that have external symbols
	GetExternalDependencies(ctx context.Context) ([]SCIPPackage, error)
}

// SCIPRelationshipManager manages symbol relationships and cross-references
type SCIPRelationshipManager interface {
	// AddRelationship adds a relationship between two symbols
	AddRelationship(fromSymbol, toSymbol string, relationshipType SCIPRelationshipType) error

	// GetRelationships returns all relationships for a symbol
	GetRelationships(ctx context.Context, symbolID string) ([]SCIPRelationship, error)

	// GetImplementedBy finds symbols that implement the given symbol
	GetImplementedBy(ctx context.Context, symbolID string) ([]string, error)

	// GetOverrides finds symbols that override the given symbol
	GetOverrides(ctx context.Context, symbolID string) ([]string, error)

	// GetImplements finds symbols that this symbol implements
	GetImplements(ctx context.Context, symbolID string) ([]string, error)

	// GetExtends finds symbols that this symbol extends/inherits from
	GetExtends(ctx context.Context, symbolID string) ([]string, error)

	// GetUsages finds all usage relationships for a symbol
	GetUsages(ctx context.Context, symbolID string) ([]string, error)

	// RemoveRelationship removes a specific relationship
	RemoveRelationship(fromSymbol, toSymbol string, relationshipType SCIPRelationshipType) error

	// GetInverseRelationships finds symbols that have relationships pointing to this symbol
	GetInverseRelationships(ctx context.Context, symbolID string) ([]SCIPRelationship, error)
}

// SCIPRelationshipType represents different types of symbol relationships
type SCIPRelationshipType int32

const (
	SCIPRelationshipTypeUnknown SCIPRelationshipType = iota
	SCIPRelationshipTypeReference
	SCIPRelationshipTypeDefinition
	SCIPRelationshipTypeImplementation
	SCIPRelationshipTypeTypeDefinition
	SCIPRelationshipTypeOverride
	SCIPRelationshipTypeExtends
	SCIPRelationshipTypeUsage
	SCIPRelationshipTypeCall
	SCIPRelationshipTypeInstantiation
)

// SCIPSymbolComponents represents the parsed components of a SCIP symbol
type SCIPSymbolComponents struct {
	// Scheme is the language scheme (go, python, typescript, etc.)
	Scheme string

	// Package information
	Package SCIPPackage

	// Descriptors form the symbol path
	Descriptors []SCIPDescriptor

	// IsLocal indicates if this is a local symbol
	IsLocal bool

	// DocumentURI for local symbols
	DocumentURI string
}

// SCIPSymbolFactory provides factory methods for creating common symbol types
type SCIPSymbolFactory interface {
	// CreateFunctionSymbol creates a function symbol
	CreateFunctionSymbol(packageName, version, functionName string, parameters []string) (string, error)

	// CreateClassSymbol creates a class symbol
	CreateClassSymbol(packageName, version, className string) (string, error)

	// CreateMethodSymbol creates a method symbol
	CreateMethodSymbol(packageName, version, className, methodName string, parameters []string) (string, error)

	// CreateVariableSymbol creates a variable symbol
	CreateVariableSymbol(packageName, version, variableName string, isGlobal bool) (string, error)

	// CreateInterfaceSymbol creates an interface symbol
	CreateInterfaceSymbol(packageName, version, interfaceName string) (string, error)

	// CreateEnumSymbol creates an enum symbol
	CreateEnumSymbol(packageName, version, enumName string) (string, error)

	// CreateModuleSymbol creates a module/namespace symbol
	CreateModuleSymbol(packageName, version, moduleName string) (string, error)

	// CreateTypeAliasSymbol creates a type alias symbol
	CreateTypeAliasSymbol(packageName, version, aliasName string) (string, error)
}

// SCIPSymbolIndexer provides methods for indexing and searching symbols efficiently
type SCIPSymbolIndexer interface {
	// IndexSymbol adds a symbol to the search index
	IndexSymbol(symbolInfo *SCIPSymbolInformation) error

	// ReindexDocument reindexes all symbols in a document
	ReindexDocument(ctx context.Context, documentURI string) error

	// SearchSymbolsByPattern searches symbols using pattern matching
	SearchSymbolsByPattern(ctx context.Context, pattern string, limit int) ([]SCIPSymbolInformation, error)

	// SearchSymbolsByKind searches symbols by their kind (function, class, etc.)
	SearchSymbolsByKind(ctx context.Context, kind SCIPSymbolKind, limit int) ([]SCIPSymbolInformation, error)

	// GetSymbolCompletions returns symbol completions for a given prefix
	GetSymbolCompletions(ctx context.Context, prefix string, limit int) ([]SCIPSymbolInformation, error)

	// GetRelatedSymbols finds symbols related to a given symbol
	GetRelatedSymbols(ctx context.Context, symbolID string) ([]SCIPSymbolInformation, error)

	// RebuildIndex rebuilds the entire symbol index
	RebuildIndex(ctx context.Context) error

	// GetIndexStats returns statistics about the symbol index
	GetIndexStats(ctx context.Context) (*SCIPIndexStats, error)
}

// SCIPIndexStats provides statistics about the symbol index
type SCIPIndexStats struct {
	// Total number of indexed symbols
	TotalSymbols int64

	// Number of symbols by kind
	SymbolsByKind map[SCIPSymbolKind]int64

	// Number of external symbols
	ExternalSymbols int64

	// Number of relationships
	TotalRelationships int64

	// Index size in bytes
	IndexSize int64

	// Last index update time
	LastUpdated time.Time
}
