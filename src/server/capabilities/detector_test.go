package capabilities

import (
	"encoding/json"
	"testing"

	"lsp-gateway/src/internal/types"
)

func TestLSPCapabilityDetector_ParseAndSupports_Standard(t *testing.T) {
	det := NewLSPCapabilityDetector()
	init := map[string]interface{}{"capabilities": map[string]interface{}{"workspaceSymbolProvider": true, "completionProvider": map[string]interface{}{"triggerCharacters": []string{"."}}, "definitionProvider": true, "referencesProvider": false}}
	raw, _ := json.Marshal(init)
	caps, err := det.ParseCapabilities(raw, "jedi-language-server")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !det.SupportsMethod(caps, types.MethodWorkspaceSymbol) {
		t.Fatalf("workspace/symbol supported")
	}
	if !det.SupportsMethod(caps, types.MethodTextDocumentCompletion) {
		t.Fatalf("completion supported when object")
	}
	if !det.SupportsMethod(caps, types.MethodTextDocumentDefinition) {
		t.Fatalf("definition supported")
	}
	if det.SupportsMethod(caps, types.MethodTextDocumentReferences) {
		t.Fatalf("references not supported")
	}
}

func TestLSPCapabilityDetector_JDTLSOverrides(t *testing.T) {
	det := NewLSPCapabilityDetector()
	init := map[string]interface{}{"capabilities": map[string]interface{}{}}
	raw, _ := json.Marshal(init)
	caps, err := det.ParseCapabilities(raw, "jdtls")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	methods := []string{types.MethodTextDocumentDefinition, types.MethodTextDocumentReferences, types.MethodTextDocumentHover, types.MethodTextDocumentDocumentSymbol, types.MethodTextDocumentCompletion}
	for _, m := range methods {
		if !det.SupportsMethod(caps, m) {
			t.Fatalf("jdtls should support %s", m)
		}
	}
}
