package registry

func GetClientCapabilities() map[string]interface{} {
	return map[string]interface{}{
		"workspace": map[string]interface{}{
			"applyEdit":              true,
			"workspaceEdit":          map[string]interface{}{"documentChanges": true},
			"didChangeConfiguration": map[string]interface{}{"dynamicRegistration": true},
			"didChangeWatchedFiles":  map[string]interface{}{"dynamicRegistration": true},
			"symbol":                 map[string]interface{}{"dynamicRegistration": true},
			"executeCommand":         map[string]interface{}{"dynamicRegistration": true},
			"configuration":          true,
			"workspaceFolders":       true,
		},
		"textDocument": map[string]interface{}{
			"publishDiagnostics": map[string]interface{}{
				"relatedInformation": true,
				"versionSupport":     false,
				"tagSupport":         map[string]interface{}{"valueSet": []int{1, 2}},
			},
			"synchronization": map[string]interface{}{
				"dynamicRegistration": true,
				"willSave":            true,
				"willSaveWaitUntil":   true,
				"didSave":             true,
			},
			"completion": map[string]interface{}{
				"dynamicRegistration": true,
				"contextSupport":      true,
				"completionItem": map[string]interface{}{
					"snippetSupport":          true,
					"commitCharactersSupport": true,
					"documentationFormat":     []string{"markdown", "plaintext"},
					"preselectSupport":        true,
				},
				"completionItemKind": map[string]interface{}{
					"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
				},
			},
			"hover": map[string]interface{}{
				"dynamicRegistration": true,
				"contentFormat":       []string{"markdown", "plaintext"},
			},
			"signatureHelp": map[string]interface{}{
				"dynamicRegistration": true,
				"signatureInformation": map[string]interface{}{
					"documentationFormat": []string{"markdown", "plaintext"},
				},
			},
			"definition": map[string]interface{}{
				"dynamicRegistration": true,
				"linkSupport":         true,
			},
			"references": map[string]interface{}{
				"dynamicRegistration": true,
			},
			"documentHighlight": map[string]interface{}{
				"dynamicRegistration": true,
			},
			"documentSymbol": map[string]interface{}{
				"dynamicRegistration":               true,
				"hierarchicalDocumentSymbolSupport": true,
				"symbolKind": map[string]interface{}{
					"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26},
				},
			},
			"codeAction": map[string]interface{}{
				"dynamicRegistration": true,
				"codeActionLiteralSupport": map[string]interface{}{
					"codeActionKind": map[string]interface{}{
						"valueSet": []string{"", "quickfix", "refactor", "refactor.extract", "refactor.inline", "refactor.rewrite", "source", "source.organizeImports"},
					},
				},
			},
			"formatting": map[string]interface{}{
				"dynamicRegistration": true,
			},
			"rangeFormatting": map[string]interface{}{
				"dynamicRegistration": true,
			},
			"onTypeFormatting": map[string]interface{}{
				"dynamicRegistration": true,
			},
			"rename": map[string]interface{}{
				"dynamicRegistration": true,
				"prepareSupport":      true,
			},
		},
	}
}
