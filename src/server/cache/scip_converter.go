package cache

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
)

// ConversionContext holds context information for conversions
type ConversionContext struct {
	URI          string
	Language     string
	PackageName  string
	Version      string
	SymbolRoles  types.SymbolRole
	DefaultSize  int64
	TimestampNow time.Time
}

// NewConversionContext creates a new conversion context with defaults
func NewConversionContext(uri, language string) *ConversionContext {
	return &ConversionContext{
		URI:          uri,
		Language:     language,
		DefaultSize:  100,
		TimestampNow: time.Now(),
	}
}

// WithPackageInfo sets package information for symbol ID generation
func (c *ConversionContext) WithPackageInfo(packageName, version string) *ConversionContext {
	c.PackageName = packageName
	c.Version = version
	return c
}

// WithSymbolRoles sets the default symbol roles for occurrences
func (c *ConversionContext) WithSymbolRoles(roles types.SymbolRole) *ConversionContext {
	c.SymbolRoles = roles
	return c
}

// SCIPConverter provides generic conversion functions between LSP and SCIP types
type SCIPConverter struct {
	packageDetector func(string, string) (string, string) // Function to detect package info
}

// NewSCIPConverter creates a new converter with package detection function
func NewSCIPConverter(packageDetector func(string, string) (string, string)) *SCIPConverter {
	return &SCIPConverter{
		packageDetector: packageDetector,
	}
}

// =============================================================================
// Position and Range Conversions
// =============================================================================

// =============================================================================
// Symbol ID Generation
// =============================================================================

// GenerateLocationBasedSymbolID creates a symbol ID from location coordinates
func (c *SCIPConverter) GenerateLocationBasedSymbolID(uri string, line, character interface{}) string {
	return fmt.Sprintf("symbol_%s_%v_%v", uri, line, character)
}

// GeneratePackageBasedSymbolID creates a SCIP-format symbol ID
func (c *SCIPConverter) GeneratePackageBasedSymbolID(language, packageName, version, symbolDescriptor string) string {
	return fmt.Sprintf("scip-%s %s %s %s", language, packageName, version, symbolDescriptor)
}

// GenerateSymbolDescriptor creates a symbol descriptor from LSP symbol info
func (c *SCIPConverter) GenerateSymbolDescriptor(symbolName, containerName string) string {
	if containerName != "" {
		return containerName + "/" + symbolName
	}
	return symbolName
}

// =============================================================================
// Type Conversions
// =============================================================================

// ConvertLSPSymbolKindToSCIP converts LSP symbol kind to SCIP symbol kind
func (c *SCIPConverter) ConvertLSPSymbolKindToSCIP(kind types.SymbolKind) scip.SCIPSymbolKind {
	switch kind {
	case types.File:
		return scip.SCIPSymbolKindFile
	case types.Module:
		return scip.SCIPSymbolKindModule
	case types.Namespace:
		return scip.SCIPSymbolKindNamespace
	case types.Package:
		return scip.SCIPSymbolKindPackage
	case types.Class:
		return scip.SCIPSymbolKindClass
	case types.Method:
		return scip.SCIPSymbolKindMethod
	case types.Property:
		return scip.SCIPSymbolKindProperty
	case types.Field:
		return scip.SCIPSymbolKindField
	case types.Constructor:
		return scip.SCIPSymbolKindConstructor
	case types.Enum:
		return scip.SCIPSymbolKindEnum
	case types.Interface:
		return scip.SCIPSymbolKindInterface
	case types.Function:
		return scip.SCIPSymbolKindFunction
	case types.Variable:
		return scip.SCIPSymbolKindVariable
	case types.Constant:
		return scip.SCIPSymbolKindConstant
	default:
		return scip.SCIPSymbolKindUnknown
	}
}

// ConvertSCIPSymbolKindToLSP converts SCIP symbol kind back to LSP
func (c *SCIPConverter) ConvertSCIPSymbolKindToLSP(kind scip.SCIPSymbolKind) types.SymbolKind {
	switch kind {
	case scip.SCIPSymbolKindFile:
		return types.File
	case scip.SCIPSymbolKindModule:
		return types.Module
	case scip.SCIPSymbolKindNamespace:
		return types.Namespace
	case scip.SCIPSymbolKindPackage:
		return types.Package
	case scip.SCIPSymbolKindClass:
		return types.Class
	case scip.SCIPSymbolKindMethod:
		return types.Method
	case scip.SCIPSymbolKindProperty:
		return types.Property
	case scip.SCIPSymbolKindField:
		return types.Field
	case scip.SCIPSymbolKindConstructor:
		return types.Constructor
	case scip.SCIPSymbolKindEnum:
		return types.Enum
	case scip.SCIPSymbolKindInterface:
		return types.Interface
	case scip.SCIPSymbolKindFunction:
		return types.Function
	case scip.SCIPSymbolKindVariable:
		return types.Variable
	case scip.SCIPSymbolKindConstant:
		return types.Constant
	default:
		return types.Variable
	}
}

// ConvertLSPSymbolKindToSyntax converts LSP symbol kind to syntax kind for highlighting
func (c *SCIPConverter) ConvertLSPSymbolKindToSyntax(kind types.SymbolKind) types.SyntaxKind {
	switch kind {
	case types.Function, types.Method:
		return types.SyntaxKindIdentifierFunction
	case types.Class:
		return types.SyntaxKindIdentifierType
	case types.Variable:
		return types.SyntaxKindIdentifierLocal
	case types.Constant:
		return types.SyntaxKindIdentifierConstant
	case types.Module, types.Namespace:
		return types.SyntaxKindIdentifierNamespace
	default:
		return types.SyntaxKindUnspecified
	}
}

// ConvertSCIPSymbolKindToCompletionItem converts SCIP symbol kind to completion item kind
func (c *SCIPConverter) ConvertSCIPSymbolKindToCompletionItem(kind scip.SCIPSymbolKind) lsp.CompletionItemKind {
	switch kind {
	case scip.SCIPSymbolKindFunction:
		return lsp.FunctionComp
	case scip.SCIPSymbolKindMethod:
		return lsp.MethodComp
	case scip.SCIPSymbolKindClass:
		return lsp.ClassComp
	case scip.SCIPSymbolKindVariable:
		return lsp.VariableComp
	case scip.SCIPSymbolKindConstant:
		return lsp.ConstantComp
	case scip.SCIPSymbolKindModule:
		return lsp.ModuleComp
	case scip.SCIPSymbolKindInterface:
		return lsp.InterfaceComp
	default:
		return lsp.Text
	}
}

// =============================================================================
// SCIP Structure Creation
// =============================================================================

// CreateSCIPOccurrence creates a SCIP occurrence from common parameters
func (c *SCIPConverter) CreateSCIPOccurrence(ctx *ConversionContext, r types.Range, symbolID string, syntaxKind types.SyntaxKind) scip.SCIPOccurrence {
	return scip.SCIPOccurrence{
		Range:       r,
		Symbol:      symbolID,
		SymbolRoles: ctx.SymbolRoles,
		SyntaxKind:  syntaxKind,
	}
}

// CreateSCIPOccurrenceFromLocation creates a SCIP occurrence from types.Location
func (c *SCIPConverter) CreateSCIPOccurrenceFromLocation(ctx *ConversionContext, location types.Location, symbolID string, syntaxKind types.SyntaxKind) scip.SCIPOccurrence {
	return c.CreateSCIPOccurrence(ctx, location.Range, symbolID, syntaxKind)
}

// CreateSCIPSymbolInfo creates SCIP symbol information
func (c *SCIPConverter) CreateSCIPSymbolInfo(symbolID, displayName string, kind scip.SCIPSymbolKind, r types.Range, sel *types.Range) scip.SCIPSymbolInformation {
	return scip.SCIPSymbolInformation{
		Symbol:         symbolID,
		DisplayName:    displayName,
		Kind:           kind,
		Range:          r,
		SelectionRange: sel,
	}
}

// CreateSCIPDocument creates a SCIP document with common metadata
func (c *SCIPConverter) CreateSCIPDocument(ctx *ConversionContext) *scip.SCIPDocument {
	return &scip.SCIPDocument{
		URI:               ctx.URI,
		Language:          ctx.Language,
		LastModified:      ctx.TimestampNow,
		Size:              ctx.DefaultSize,
		Occurrences:       make([]scip.SCIPOccurrence, 0),
		SymbolInformation: make([]scip.SCIPSymbolInformation, 0),
	}
}

// =============================================================================
// High-Level Conversion Functions
// =============================================================================

// LSPSymbolConversionResult is the result of converting an LSP symbol to SCIP.
type LSPSymbolConversionResult struct {
	Occurrence scip.SCIPOccurrence
	SymbolInfo scip.SCIPSymbolInformation
	SymbolID   string
}

// ConvertLSPSymbolToSCIP converts a single LSP symbol to SCIP format
func (c *SCIPConverter) ConvertLSPSymbolToSCIP(ctx *ConversionContext, symbol types.SymbolInformation) LSPSymbolConversionResult {
	// Generate symbol ID
	var symbolID string
	if ctx.PackageName != "" && ctx.Version != "" {
		descriptor := c.GenerateSymbolDescriptor(symbol.Name, symbol.ContainerName)
		symbolID = c.GeneratePackageBasedSymbolID(ctx.Language, ctx.PackageName, ctx.Version, descriptor)
	} else {
		symbolID = c.GenerateLocationBasedSymbolID(ctx.URI, symbol.Location.Range.Start.Line, symbol.Location.Range.Start.Character)
	}

	// Convert symbol kind
	scipKind := c.ConvertLSPSymbolKindToSCIP(symbol.Kind)
	syntaxKind := c.ConvertLSPSymbolKindToSyntax(symbol.Kind)

	// Create occurrence (symbol.Location.Range is already types.Range)
	occurrence := c.CreateSCIPOccurrenceFromLocation(ctx, symbol.Location, symbolID, syntaxKind)

	// Handle selection range if present
	if symbol.SelectionRange != nil {
		// SelectionRange is already a types.Range
		occurrence.SelectionRange = symbol.SelectionRange
	}

	// Create symbol info with both full range and selection range
	symbolInfo := c.CreateSCIPSymbolInfo(symbolID, symbol.Name, scipKind, symbol.Location.Range, symbol.SelectionRange)

	return LSPSymbolConversionResult{
		Occurrence: occurrence,
		SymbolInfo: symbolInfo,
		SymbolID:   symbolID,
	}
}

// ConvertLocationsToSCIPDocument converts a list of locations to a SCIP document
func (c *SCIPConverter) ConvertLocationsToSCIPDocument(ctx *ConversionContext, locations []types.Location, symbolIDGenerator func(types.Location) string) (*scip.SCIPDocument, error) {
	if symbolIDGenerator == nil {
		symbolIDGenerator = func(loc types.Location) string {
			return c.GenerateLocationBasedSymbolID(loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
		}
	}

	scipDoc := c.CreateSCIPDocument(ctx)
	scipDoc.Size = int64(len(locations) * 50) // Rough estimate

	for _, location := range locations {
		symbolID := symbolIDGenerator(location)
		occurrence := c.CreateSCIPOccurrenceFromLocation(ctx, location, symbolID, types.SyntaxKindUnspecified)
		scipDoc.Occurrences = append(scipDoc.Occurrences, occurrence)
	}

	return scipDoc, nil
}

// ConvertLSPSymbolsToSCIPDocument converts LSP symbols to a complete SCIP document
func (c *SCIPConverter) ConvertLSPSymbolsToSCIPDocument(ctx *ConversionContext, symbols []types.SymbolInformation) (*scip.SCIPDocument, error) {
	scipDoc := c.CreateSCIPDocument(ctx)
	scipDoc.Size = int64(len(symbols) * 100) // Rough estimate

	// Detect package info if not already set
	if ctx.PackageName == "" && c.packageDetector != nil {
		ctx.PackageName, ctx.Version = c.packageDetector(ctx.URI, ctx.Language)
	}

	// Convert each symbol
	for _, symbol := range symbols {
		result := c.ConvertLSPSymbolToSCIP(ctx, symbol)
		scipDoc.Occurrences = append(scipDoc.Occurrences, result.Occurrence)
		scipDoc.SymbolInformation = append(scipDoc.SymbolInformation, result.SymbolInfo)
	}

	return scipDoc, nil
}

// =============================================================================
// Storage Helper Functions
// =============================================================================

// StoreSCIPDocument stores a SCIP document with error handling
func (c *SCIPConverter) StoreSCIPDocument(storage scip.SCIPDocumentStorage, doc *scip.SCIPDocument) error {
	if storage == nil {
		return fmt.Errorf("storage is nil")
	}
	if doc == nil {
		return fmt.Errorf("document is nil")
	}

	return storage.StoreDocument(context.Background(), doc)
}

// ValidateAndStore validates a SCIP document and stores it
func (c *SCIPConverter) ValidateAndStore(storage scip.SCIPDocumentStorage, doc *scip.SCIPDocument, operationType string) error {
	if doc == nil {
		return fmt.Errorf("cannot store nil document for %s", operationType)
	}
	if doc.URI == "" {
		return fmt.Errorf("document URI cannot be empty for %s", operationType)
	}

	if err := c.StoreSCIPDocument(storage, doc); err != nil {
		return fmt.Errorf("failed to store %s: %w", operationType, err)
	}

	return nil
}

// =============================================================================
// Utility Functions
// =============================================================================

// DetectLanguageFromURI detects language from file extension
func (c *SCIPConverter) DetectLanguageFromURI(uri string) string {
	path := utils.URIToFilePathCached(uri)
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := registry.GetLanguageByExtension(ext); ok {
		return lang.Name
	}
	return "unknown"
}

// CreateLocation creates a types.Location from URI and range
func (c *SCIPConverter) CreateLocation(uri string, r types.Range) types.Location {
	return types.Location{
		URI:   uri,
		Range: r,
	}
}
