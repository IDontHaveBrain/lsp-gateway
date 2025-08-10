package lspconv

import (
    "encoding/json"
    "lsp-gateway/src/internal/types"
    "lsp-gateway/src/utils/jsonutil"
)

// ParseLocations converts various LSP location result shapes to a []types.Location
func ParseLocations(result interface{}) []types.Location {
    switch v := result.(type) {
    case nil:
        return nil
    case types.Location:
        return []types.Location{v}
    case []types.Location:
        return v
    case json.RawMessage:
        var locs []types.Location
        if err := json.Unmarshal(v, &locs); err == nil {
            return locs
        }
        var raw []interface{}
        if err := json.Unmarshal(v, &raw); err == nil {
            out := make([]types.Location, 0, len(raw))
            for _, item := range raw {
                if loc, err := jsonutil.Convert[types.Location](item); err == nil && loc.URI != "" {
                    out = append(out, loc)
                }
            }
            return out
        }
        return nil
    case []interface{}:
        out := make([]types.Location, 0, len(v))
        for _, item := range v {
            if loc, err := jsonutil.Convert[types.Location](item); err == nil && loc.URI != "" {
                out = append(out, loc)
            }
        }
        return out
    case map[string]interface{}:
        if loc, err := jsonutil.Convert[types.Location](v); err == nil && loc.URI != "" {
            return []types.Location{loc}
        }
        return nil
    default:
        return nil
    }
}

