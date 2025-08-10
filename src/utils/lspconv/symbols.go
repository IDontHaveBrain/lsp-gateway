package lspconv

import (
    "encoding/json"

    "lsp-gateway/src/internal/models/lsp"
    "lsp-gateway/src/internal/types"
    "lsp-gateway/src/utils/jsonutil"
)

// ParseWorkspaceSymbols normalizes workspace/symbol results into []types.SymbolInformation
func ParseWorkspaceSymbols(result interface{}) []types.SymbolInformation {
    if result == nil {
        return nil
    }
    switch v := result.(type) {
    case []types.SymbolInformation:
        return v
    case []interface{}:
        out := make([]types.SymbolInformation, 0, len(v))
        for _, it := range v {
            if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
                out = append(out, si)
            }
        }
        return out
    case json.RawMessage:
        var arr []types.SymbolInformation
        if err := json.Unmarshal(v, &arr); err == nil {
            return arr
        }
        var raw []interface{}
        if err := json.Unmarshal(v, &raw); err == nil {
            out := make([]types.SymbolInformation, 0, len(raw))
            for _, it := range raw {
                if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
                    out = append(out, si)
                }
            }
            return out
        }
        return nil
    default:
        return nil
    }
}

// ParseDocumentSymbols converts documentSymbol responses to []lsp.DocumentSymbol (pointers)
func ParseDocumentSymbols(result interface{}) []*lsp.DocumentSymbol {
    switch v := result.(type) {
    case []lsp.DocumentSymbol:
        out := make([]*lsp.DocumentSymbol, 0, len(v))
        for i := range v {
            out = append(out, &v[i])
        }
        return out
    case []*lsp.DocumentSymbol:
        return v
    case []interface{}:
        out := make([]*lsp.DocumentSymbol, 0, len(v))
        for _, it := range v {
            if ds, err := jsonutil.Convert[lsp.DocumentSymbol](it); err == nil && ds.Name != "" {
                // capture new var to take address safely
                dsc := ds
                out = append(out, &dsc)
            }
        }
        return out
    case json.RawMessage:
        var arr []lsp.DocumentSymbol
        if err := json.Unmarshal(v, &arr); err == nil {
            out := make([]*lsp.DocumentSymbol, 0, len(arr))
            for i := range arr {
                out = append(out, &arr[i])
            }
            return out
        }
        var raw []interface{}
        if err := json.Unmarshal(v, &raw); err == nil {
            out := make([]*lsp.DocumentSymbol, 0, len(raw))
            for _, it := range raw {
                if ds, err := jsonutil.Convert[lsp.DocumentSymbol](it); err == nil && ds.Name != "" {
                    dsc := ds
                    out = append(out, &dsc)
                }
            }
            return out
        }
        return nil
    default:
        return nil
    }
}

// ParseDocumentSymbolsToSymbolInformation converts documentSymbol responses to []types.SymbolInformation
// using the provided defaultURI to populate locations.
func ParseDocumentSymbolsToSymbolInformation(result interface{}, defaultURI string) []types.SymbolInformation {
    // First try to get DocumentSymbols, then map to SymbolInformation
    docSyms := ParseDocumentSymbols(result)
    if len(docSyms) > 0 {
        out := make([]types.SymbolInformation, 0, len(docSyms))
        for _, ds := range docSyms {
            if ds == nil {
                continue
            }
            si := types.SymbolInformation{
                Name: ds.Name,
                Kind: ds.Kind,
                Location: types.Location{
                    URI:   defaultURI,
                    Range: ds.Range,
                },
            }
            // preserve selection range via pointer
            sel := ds.SelectionRange
            si.SelectionRange = &sel
            out = append(out, si)
        }
        return out
    }

    // Otherwise, attempt to parse directly as SymbolInformation array
    switch v := result.(type) {
    case []types.SymbolInformation:
        return v
    case []interface{}:
        out := make([]types.SymbolInformation, 0, len(v))
        for _, it := range v {
            if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
                out = append(out, si)
            }
        }
        return out
    case json.RawMessage:
        var arr []types.SymbolInformation
        if err := json.Unmarshal(v, &arr); err == nil {
            return arr
        }
        var raw []interface{}
        if err := json.Unmarshal(v, &raw); err == nil {
            out := make([]types.SymbolInformation, 0, len(raw))
            for _, it := range raw {
                if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
                    out = append(out, si)
                }
            }
            return out
        }
    }
    return nil
}

