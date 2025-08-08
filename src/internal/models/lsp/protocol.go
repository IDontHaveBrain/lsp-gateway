package lsp

import (
	"lsp-gateway/src/internal/types"
)

type Hover struct {
	Contents interface{}  `json:"contents"`
	Range    *types.Range `json:"range,omitempty"`
}

type SymbolInformation struct {
	Name           string           `json:"name"`
	Kind           types.SymbolKind `json:"kind"`
	Tags           []int            `json:"tags,omitempty"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Location       types.Location   `json:"location"`
	ContainerName  string           `json:"containerName,omitempty"`
	SelectionRange *types.Range     `json:"-"` // Not part of LSP spec, used internally for DocumentSymbol conversion
}

type DocumentSymbol struct {
	Name           string            `json:"name"`
	Detail         string            `json:"detail,omitempty"`
	Kind           types.SymbolKind  `json:"kind"`
	Tags           []int             `json:"tags,omitempty"`
	Deprecated     bool              `json:"deprecated,omitempty"`
	Range          types.Range       `json:"range"`
	SelectionRange types.Range       `json:"selectionRange"`
	Children       []*DocumentSymbol `json:"children,omitempty"`
}

type CompletionItemKind int

const (
	Text              CompletionItemKind = 1
	MethodComp        CompletionItemKind = 2
	FunctionComp      CompletionItemKind = 3
	ConstructorComp   CompletionItemKind = 4
	FieldComp         CompletionItemKind = 5
	VariableComp      CompletionItemKind = 6
	ClassComp         CompletionItemKind = 7
	InterfaceComp     CompletionItemKind = 8
	ModuleComp        CompletionItemKind = 9
	PropertyComp      CompletionItemKind = 10
	UnitComp          CompletionItemKind = 11
	ValueComp         CompletionItemKind = 12
	EnumComp          CompletionItemKind = 13
	KeywordComp       CompletionItemKind = 14
	SnippetComp       CompletionItemKind = 15
	ColorComp         CompletionItemKind = 16
	FileComp          CompletionItemKind = 17
	ReferenceComp     CompletionItemKind = 18
	FolderComp        CompletionItemKind = 19
	EnumMemberComp    CompletionItemKind = 20
	ConstantComp      CompletionItemKind = 21
	StructComp        CompletionItemKind = 22
	EventComp         CompletionItemKind = 23
	OperatorComp      CompletionItemKind = 24
	TypeParameterComp CompletionItemKind = 25
)

type CompletionItem struct {
	Label               string             `json:"label"`
	Kind                CompletionItemKind `json:"kind,omitempty"`
	Tags                []int              `json:"tags,omitempty"`
	Detail              string             `json:"detail,omitempty"`
	Documentation       interface{}        `json:"documentation,omitempty"`
	Deprecated          bool               `json:"deprecated,omitempty"`
	Preselect           bool               `json:"preselect,omitempty"`
	SortText            string             `json:"sortText,omitempty"`
	FilterText          string             `json:"filterText,omitempty"`
	InsertText          string             `json:"insertText,omitempty"`
	InsertTextFormat    int                `json:"insertTextFormat,omitempty"`
	InsertTextMode      int                `json:"insertTextMode,omitempty"`
	AdditionalTextEdits []interface{}      `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string           `json:"commitCharacters,omitempty"`
	Command             interface{}        `json:"command,omitempty"`
	Data                interface{}        `json:"data,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool              `json:"isIncomplete"`
	Items        []*CompletionItem `json:"items"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     types.Position         `json:"position"`
}

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type HoverParams struct {
	TextDocumentPositionParams
}

type CompletionParams struct {
	TextDocumentPositionParams
	Context *CompletionContext `json:"context,omitempty"`
}

type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

type DefinitionParams struct {
	TextDocumentPositionParams
}
