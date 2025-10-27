package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/filepattern"
	"lsp-gateway/src/utils/lspconv"
)

type defaultSymbolService struct {
	manager *LSPManager
}

func newDefaultSymbolService(manager *LSPManager) MCPSymbolService {
	return &defaultSymbolService{manager: manager}
}

func (s *defaultSymbolService) FindSymbols(ctx context.Context, query SymbolSearchQuery) (*SymbolSearchResult, error) {
	if query.Pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}
	if query.FilePattern == "" {
		query.FilePattern = "**/*"
	}
	if query.MaxResults <= 0 {
		query.MaxResults = constants.DefaultMaxResults
	}

	result := &SymbolSearchResult{
		Symbols:    []types.EnhancedSymbolInfo{},
		TotalCount: 0,
		Truncated:  false,
	}

	fallbackToLSP := s.manager.scipCache == nil
	if !fallbackToLSP && s.manager.scipCache != nil {
		if stats := s.manager.scipCache.GetIndexStats(); stats == nil || stats.Status == "disabled" || (stats.SymbolCount == 0 && stats.DocumentCount == 0) {
			fallbackToLSP = true
		}
	}

	if !fallbackToLSP && s.manager.scipCache != nil {
		scipResults, err := s.manager.scipCache.SearchSymbols(ctx, query.Pattern, query.FilePattern, query.MaxResults)
		if err == nil && len(scipResults) > 0 {
			symbols := make([]types.EnhancedSymbolInfo, 0, len(scipResults))
			for _, item := range scipResults {
				switch v := item.(type) {
				case map[string]interface{}:
					enhanced := s.parseEnhancedSymbolFromMap(ctx, v)
					if enhanced != nil {
						symbols = append(symbols, *enhanced)
					}
				case scip.SCIPSymbolInformation:
					if enhanced := s.parseEnhancedSymbolFromSCIP(ctx, v); enhanced != nil {
						symbols = append(symbols, *enhanced)
					}
				}
			}
			result.Symbols = symbols
			result.TotalCount = len(symbols)
			result.Truncated = len(scipResults) >= query.MaxResults
		}
	}

	if len(result.Symbols) == 0 {
		res, err := s.fallbackFindSymbolsWithLSP(ctx, query.Pattern, query.FilePattern, query.MaxResults)
		if err != nil {
			return nil, err
		}
		if res != nil {
			result.Symbols = res.Symbols
			result.TotalCount = res.TotalCount
			result.Truncated = res.Truncated
		}
	}

	if query.IncludeCode {
		for i := range result.Symbols {
			start := result.Symbols[i].LineNumber
			end := result.Symbols[i].EndLine
			if end < start {
				end = start
			}
			if code, err := extractCodeLines(result.Symbols[i].FilePath, start, end); err == nil {
				result.Symbols[i].Code = code
			}
		}
	}

	return result, nil
}

func (s *defaultSymbolService) FindReferences(ctx context.Context, query SymbolReferenceQuery) (*SymbolReferenceResult, error) {
	if query.Pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}
	if query.FilePattern == "" {
		query.FilePattern = "**/*"
	}
	if query.MaxResults <= 0 {
		query.MaxResults = constants.DefaultMaxResults
	}

	result, err := s.manager.SearchSymbolReferences(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(result.References) > query.MaxResults {
		result.References = result.References[:query.MaxResults]
		result.Truncated = true
	}

	return result, nil
}

func (s *defaultSymbolService) parseEnhancedSymbolFromMap(ctx context.Context, payload map[string]interface{}) *types.EnhancedSymbolInfo {
	var symbolInfo scip.SCIPSymbolInformation
	if info, ok := payload["symbolInfo"].(scip.SCIPSymbolInformation); ok {
		symbolInfo = info
	}

	var occurrence *scip.SCIPOccurrence
	if occ, ok := payload["occurrence"].(*scip.SCIPOccurrence); ok {
		occurrence = occ
	}

	var rng types.Range
	if r, ok := payload["range"].(types.Range); ok {
		rng = r
	} else if rm, ok := payload["range"].(map[string]interface{}); ok {
		if parsed, ok := lspconv.ParseRangeFromMap(rm); ok {
			rng = parsed
		}
	}

	filePath := ""
	if docURI, ok := payload["documentURI"].(string); ok && docURI != "" {
		filePath = utils.URIToFilePathCached(docURI)
	} else if fp, ok := payload["filePath"].(string); ok && fp != "" {
		filePath = utils.URIToFilePathCached(fp)
	}
	if filePath == "" {
		return nil
	}

	lineNumber := 0
	endLine := 0
	if occurrence != nil {
		lineNumber = int(occurrence.Range.Start.Line)
		endLine = int(occurrence.Range.End.Line)
	} else if (rng.Start.Line != 0 || rng.End.Line != 0) || (rng.Start.Character != 0 || rng.End.Character != 0) {
		lineNumber = int(rng.Start.Line)
		endLine = int(rng.End.Line)
	}
	if endLine < lineNumber {
		endLine = lineNumber
	}

	lspKind := s.convertSymbolKind(symbolInfo.Kind)
	doc := ""
	if len(symbolInfo.Documentation) > 0 {
		doc = strings.Join(symbolInfo.Documentation, "\n")
	}

	return &types.EnhancedSymbolInfo{
		SymbolInformation: types.SymbolInformation{
			Name: symbolInfo.DisplayName,
			Kind: lspKind,
			Location: types.Location{
				URI: utils.FilePathToURI(filePath),
				Range: types.Range{
					Start: types.Position{Line: int32(lineNumber), Character: 0},
					End:   types.Position{Line: int32(endLine), Character: 0},
				},
			},
		},
		FilePath:      filePath,
		LineNumber:    lineNumber,
		EndLine:       endLine,
		Documentation: doc,
	}
}

func (s *defaultSymbolService) parseEnhancedSymbolFromSCIP(ctx context.Context, info scip.SCIPSymbolInformation) *types.EnhancedSymbolInfo {
	filePath := ""
	lineNumber := int(info.Range.Start.Line)
	endLine := int(info.Range.End.Line)

	if storage := s.manager.scipCache.GetSCIPStorage(); storage != nil {
		if defs, _ := storage.GetDefinitionsWithDocuments(ctx, info.Symbol); len(defs) > 0 {
			filePath = utils.URIToFilePathCached(defs[0].DocumentURI)
			lineNumber = int(defs[0].Range.Start.Line)
			endLine = int(defs[0].Range.End.Line)
		} else if occs, _ := storage.GetOccurrencesWithDocuments(ctx, info.Symbol); len(occs) > 0 {
			filePath = utils.URIToFilePathCached(occs[0].DocumentURI)
			lineNumber = int(occs[0].Range.Start.Line)
			endLine = int(occs[0].Range.End.Line)
		} else if uris, err := storage.ListDocuments(ctx); err == nil {
			for _, uri := range uris {
				doc, derr := storage.GetDocument(ctx, uri)
				if derr != nil || doc == nil {
					continue
				}
				for _, si := range doc.SymbolInformation {
					if si.Symbol == info.Symbol {
						filePath = utils.URIToFilePathCached(uri)
						if si.Range.Start.Line != 0 || si.Range.End.Line != 0 || si.Range.Start.Character != 0 || si.Range.End.Character != 0 {
							lineNumber = int(si.Range.Start.Line)
							endLine = int(si.Range.End.Line)
						}
						break
					}
				}
				if filePath != "" {
					break
				}
			}
		}
	}
	if filePath == "" {
		return nil
	}

	doc := ""
	if len(info.Documentation) > 0 {
		doc = strings.Join(info.Documentation, "\n")
	}

	return &types.EnhancedSymbolInfo{
		SymbolInformation: types.SymbolInformation{
			Name: info.DisplayName,
			Kind: s.convertSymbolKind(info.Kind),
			Location: types.Location{
				URI: utils.FilePathToURI(filePath),
				Range: types.Range{
					Start: types.Position{Line: int32(lineNumber), Character: 0},
					End:   types.Position{Line: int32(endLine), Character: 0},
				},
			},
		},
		FilePath:      filePath,
		LineNumber:    lineNumber,
		EndLine:       endLine,
		Documentation: doc,
	}
}

func (s *defaultSymbolService) convertSymbolKind(kind scip.SCIPSymbolKind) types.SymbolKind {
	switch kind {
	case scip.SCIPSymbolKindClass:
		return types.Class
	case scip.SCIPSymbolKindMethod:
		return types.Method
	case scip.SCIPSymbolKindFunction:
		return types.Function
	case scip.SCIPSymbolKindNamespace:
		return types.Namespace
	case scip.SCIPSymbolKindModule:
		return types.Module
	case scip.SCIPSymbolKindInterface:
		return types.Interface
	case scip.SCIPSymbolKindEnum:
		return types.Enum
	case scip.SCIPSymbolKindField:
		return types.Field
	case scip.SCIPSymbolKindProperty:
		return types.Property
	case scip.SCIPSymbolKindConstructor:
		return types.Constructor
	case scip.SCIPSymbolKindConstant:
		return types.Constant
	case scip.SCIPSymbolKindStruct:
		return types.Struct
	case scip.SCIPSymbolKindVariable:
		return types.Variable
	default:
		return types.Variable
	}
}

func (s *defaultSymbolService) fallbackFindSymbolsWithLSP(ctx context.Context, pattern, filePattern string, maxResults int) (*types.SymbolPatternResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	s.manager.mu.RLock()
	clients := make(map[string]interface{}, len(s.manager.clients))
	for k, v := range s.manager.clients {
		clients[k] = v
	}
	s.manager.mu.RUnlock()

	symbols := make([]types.EnhancedSymbolInfo, 0, maxResults)
	for language, client := range clients {
		clientTyped, ok := client.(types.LSPClient)
		if !ok {
			continue
		}
		params := map[string]interface{}{
			"query": pattern,
		}

		raw, err := clientTyped.SendRequest(timeoutCtx, "workspace/symbol", params)
		if err != nil {
			common.LSPLogger.Debug("workspace/symbol request failed for %s: %v", language, err)
			continue
		}

		var items []interface{}
		if err := json.Unmarshal(raw, &items); err != nil {
			common.LSPLogger.Debug("workspace/symbol response decode failed for %s: %v", language, err)
			continue
		}

		for _, item := range items {
			symbol, ok := convertWorkspaceSymbol(item)
			if !ok {
				continue
			}

			if symbol.Location.URI == "" {
				continue
			}
			filePath := utils.URIToFilePathCached(symbol.Location.URI)
			if filePath == "" {
				continue
			}
			if !filepattern.Match(filePath, filePattern) {
				continue
			}

			symbols = append(symbols, types.EnhancedSymbolInfo{
				SymbolInformation: symbol,
				FilePath:          filePath,
				LineNumber:        int(symbol.Location.Range.Start.Line),
				EndLine:           int(symbol.Location.Range.End.Line),
			})

			if len(symbols) >= maxResults {
				break
			}
		}

		if len(symbols) >= maxResults {
			break
		}
	}

	sort.SliceStable(symbols, func(i, j int) bool {
		if symbols[i].FilePath == symbols[j].FilePath {
			return symbols[i].LineNumber < symbols[j].LineNumber
		}
		return symbols[i].FilePath < symbols[j].FilePath
	})

	return &types.SymbolPatternResult{
		Symbols:    symbols,
		TotalCount: len(symbols),
		Truncated:  len(symbols) >= maxResults,
	}, nil
}

func convertWorkspaceSymbol(item interface{}) (types.SymbolInformation, bool) {
	obj, ok := item.(map[string]interface{})
	if !ok {
		return types.SymbolInformation{}, false
	}

	name, _ := obj["name"].(string)
	kindFloat, _ := obj["kind"].(float64)
	locationObj, ok := obj["location"].(map[string]interface{})
	if !ok {
		return types.SymbolInformation{}, false
	}
	rangeObj, ok := locationObj["range"].(map[string]interface{})
	if !ok {
		return types.SymbolInformation{}, false
	}
	start, sOK := rangeObj["start"].(map[string]interface{})
	end, eOK := rangeObj["end"].(map[string]interface{})
	if !sOK || !eOK {
		return types.SymbolInformation{}, false
	}

	startLine, _ := start["line"].(float64)
	endLine, _ := end["line"].(float64)
	uri, _ := locationObj["uri"].(string)

	return types.SymbolInformation{
		Name: name,
		Kind: types.SymbolKind(kindFloat),
		Location: types.Location{
			URI: uri,
			Range: types.Range{
				Start: types.Position{Line: int32(startLine), Character: 0},
				End:   types.Position{Line: int32(endLine), Character: 0},
			},
		},
	}, true
}
