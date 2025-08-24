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
		// Try Location[]
		var locs []types.Location
		if err := json.Unmarshal(v, &locs); err == nil && len(locs) > 0 {
			valid := make([]types.Location, 0, len(locs))
			for _, l := range locs {
				if l.URI != "" {
					valid = append(valid, l)
				}
			}
			if len(valid) > 0 {
				return valid
			}
		}
		// Try Location single
		var single types.Location
		if err := json.Unmarshal(v, &single); err == nil && single.URI != "" {
			return []types.Location{single}
		}
		// Try LocationLink[]: { targetUri, targetRange, targetSelectionRange }
		type locationLink struct {
			TargetURI            string      `json:"targetUri"`
			TargetRange          types.Range `json:"targetRange"`
			TargetSelectionRange types.Range `json:"targetSelectionRange"`
		}
		var links []locationLink
		if err := json.Unmarshal(v, &links); err == nil && len(links) > 0 {
			out := make([]types.Location, 0, len(links))
			for _, l := range links {
				rng := l.TargetRange
				// Fallback to selection range if full range is empty
				if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
					rng = l.TargetSelectionRange
				}
				if l.TargetURI != "" {
					out = append(out, types.Location{URI: l.TargetURI, Range: rng})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
		// Try LocationLink single
		var link locationLink
		if err := json.Unmarshal(v, &link); err == nil && link.TargetURI != "" {
			rng := link.TargetRange
			if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
				rng = link.TargetSelectionRange
			}
			return []types.Location{{URI: link.TargetURI, Range: rng}}
		}
		// Try generic array and map shapes
		var raw []interface{}
		if err := json.Unmarshal(v, &raw); err == nil {
			out := make([]types.Location, 0, len(raw))
			for _, item := range raw {
				// First try to coerce to Location directly
				if loc, err := jsonutil.Convert[types.Location](item); err == nil && loc.URI != "" {
					out = append(out, loc)
					continue
				}
				// Then try LocationLink-like map
				if m, ok := item.(map[string]interface{}); ok {
					if loc, ok2 := locationFromLinkMap(m); ok2 {
						out = append(out, loc)
					}
				}
			}
			return out
		}
		return nil
	case []interface{}:
		out := make([]types.Location, 0, len(v))
		for _, item := range v {
			// Try direct Location
			if loc, err := jsonutil.Convert[types.Location](item); err == nil && loc.URI != "" {
				out = append(out, loc)
				continue
			}
			// Try LocationLink-like map
			if m, ok := item.(map[string]interface{}); ok {
				if loc, ok2 := locationFromLinkMap(m); ok2 {
					out = append(out, loc)
					continue
				}
			}
		}
		return out
	case map[string]interface{}:
		// Try direct Location
		if loc, err := jsonutil.Convert[types.Location](v); err == nil && loc.URI != "" {
			return []types.Location{loc}
		}
		// Try LocationLink-like map
		if loc, ok := locationFromLinkMap(v); ok {
			return []types.Location{loc}
		}
		return nil
	default:
		return nil
	}
}

// locationFromLinkMap tries to build a Location from a map with LocationLink keys.
func locationFromLinkMap(m map[string]interface{}) (types.Location, bool) {
	var out types.Location
	uri, _ := m["targetUri"].(string)
	if uri == "" {
		return out, false
	}
	var rng types.Range
	if r, ok := m["targetRange"].(map[string]interface{}); ok {
		if pr, ok := ParseRangeFromMap(r); ok {
			rng = pr
		}
	}
	// Fallback to targetSelectionRange if targetRange was empty
	if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
		if r, ok := m["targetSelectionRange"].(map[string]interface{}); ok {
			if pr, ok := ParseRangeFromMap(r); ok {
				rng = pr
			}
		}
	}
	if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
		return out, false
	}
	out = types.Location{URI: uri, Range: rng}
	return out, true
}
