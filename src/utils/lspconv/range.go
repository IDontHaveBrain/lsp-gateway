package lspconv

import (
	"lsp-gateway/src/internal/types"
)

// ParseRangeFromMap converts a map[string]interface{} with LSP range shape into types.Range
func ParseRangeFromMap(m map[string]interface{}) (types.Range, bool) {
	var r types.Range
	if m == nil {
		return r, false
	}
	if s, ok := m["start"].(map[string]interface{}); ok {
		if v, ok := s["line"].(float64); ok {
			r.Start.Line = int32(v)
		}
		if v, ok := s["character"].(float64); ok {
			r.Start.Character = int32(v)
		}
	}
	if e, ok := m["end"].(map[string]interface{}); ok {
		if v, ok := e["line"].(float64); ok {
			r.End.Line = int32(v)
		}
		if v, ok := e["character"].(float64); ok {
			r.End.Character = int32(v)
		}
	}
	return r, true
}
