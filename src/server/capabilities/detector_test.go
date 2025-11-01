package capabilities

import (
	"encoding/json"
	"testing"

	"lsp-gateway/src/internal/types"
)

func TestLSPCapabilityDetector_ParseAndSupports_Standard(t *testing.T) {
	init := map[string]interface{}{"capabilities": map[string]interface{}{"workspaceSymbolProvider": true, "completionProvider": map[string]interface{}{"triggerCharacters": []string{"."}}, "definitionProvider": true, "referencesProvider": false}}
	raw, _ := json.Marshal(init)
	caps, err := ParseCapabilities(raw, "jedi-language-server")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !SupportsMethod(caps, types.MethodWorkspaceSymbol) {
		t.Fatalf("workspace/symbol supported")
	}
	if !SupportsMethod(caps, types.MethodTextDocumentCompletion) {
		t.Fatalf("completion supported when object")
	}
	if !SupportsMethod(caps, types.MethodTextDocumentDefinition) {
		t.Fatalf("definition supported")
	}
	if SupportsMethod(caps, types.MethodTextDocumentReferences) {
		t.Fatalf("references not supported")
	}
}

func TestLSPCapabilityDetector_JDTLSOverrides(t *testing.T) {
	init := map[string]interface{}{"capabilities": map[string]interface{}{}}
	raw, _ := json.Marshal(init)
	caps, err := ParseCapabilities(raw, "jdtls")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	methods := []string{types.MethodTextDocumentDefinition, types.MethodTextDocumentReferences, types.MethodTextDocumentHover, types.MethodTextDocumentDocumentSymbol, types.MethodTextDocumentCompletion}
	for _, m := range methods {
		if !SupportsMethod(caps, m) {
			t.Fatalf("jdtls should support %s", m)
		}
	}
}

func TestLSPCapabilityDetector_OmniSharpOverrides(t *testing.T) {
	init := map[string]interface{}{"capabilities": map[string]interface{}{}}
	raw, _ := json.Marshal(init)
	caps, err := ParseCapabilities(raw, "omnisharp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	methods := []string{types.MethodTextDocumentDefinition, types.MethodTextDocumentReferences, types.MethodTextDocumentHover, types.MethodTextDocumentDocumentSymbol, types.MethodTextDocumentCompletion}
	for _, m := range methods {
		if !SupportsMethod(caps, m) {
			t.Fatalf("omnisharp should support %s", m)
		}
	}
}
