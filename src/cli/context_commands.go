package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	clicommon "lsp-gateway/src/cli/common"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/lspconv"
)

type contextNode struct {
	Name      string         `json:"name,omitempty"`
	Signature string         `json:"signature,omitempty"`
	StartL    int            `json:"start_line"`
	StartC    int            `json:"start_char"`
	EndL      int            `json:"end_line"`
	EndC      int            `json:"end_char"`
	Kind      string         `json:"kind"`
	Children  []*contextNode `json:"children,omitempty"`
}

type SymbolInfo struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Signature  string   `json:"signature,omitempty"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	References []string `json:"references,omitempty"`
}

type FileRelation struct {
	File         string   `json:"file"`
	Imports      []string `json:"imports,omitempty"`
	ImportedBy   []string `json:"imported_by,omitempty"`
	References   []string `json:"references,omitempty"`
	ReferencedBy []string `json:"referenced_by,omitempty"`
	Symbols      int      `json:"symbols_count"`
}

type DependencyInfo struct {
	File        string         `json:"file"`
	Exports     []ExportSymbol `json:"exports"`
	SymbolCount int            `json:"symbol_count"`
}

type ExportSymbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// GenerateContextSignatureMap creates a .txt signature map from indexed data
func GenerateContextSignatureMap(configPath, outputPath string) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	if cmdCtx.Cache == nil {
		return fmt.Errorf("cache unavailable")
	}

	if _, err := cmdCtx.CheckCacheHealth(); err != nil {
		return fmt.Errorf("cache health check failed: %w", err)
	}

	storage := cmdCtx.Cache.GetSCIPStorage()
	if storage == nil {
		return fmt.Errorf("scip storage unavailable")
	}

	// Ensure output dir exists
	if outputPath == "" {
		outputPath = "context-signature-map.txt"
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil && filepath.Dir(outputPath) != "." {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Collect entries grouped by file
	grouped := make(map[string][]*contextNode)

	// List and iterate documents
	uris, err := storage.ListDocuments(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	totalSymbols := 0

	for _, uri := range uris {
		doc, err := storage.GetDocument(context.Background(), uri)
		if err != nil || doc == nil {
			continue
		}

		file := utils.URIToFilePathCached(uri)
		if file == "" {
			file = uri
		}

		for _, si := range doc.SymbolInformation {
			name := strings.TrimSpace(si.DisplayName)
			sig := strings.TrimSpace(si.SignatureDocumentation.Text)
			if name == "" && sig == "" {
				continue
			}
			nd := &contextNode{
				Name:      name,
				Signature: sig,
				StartL:    int(si.Range.Start.Line),
				StartC:    int(si.Range.Start.Character),
				EndL:      int(si.Range.End.Line),
				EndC:      int(si.Range.End.Character),
				Kind:      lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase),
				Children:  nil,
			}
			grouped[file] = append(grouped[file], nd)
			totalSymbols++
		}
	}

	if totalSymbols == 0 {
		return fmt.Errorf("no indexed symbols found; run 'lsp-gateway cache index' first")
	}

	// Sort files and entries
	files := make([]string, 0, len(grouped))
	for f := range grouped {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, f := range files {
		list := grouped[f]
		sort.Slice(list, func(i, j int) bool {
			if list[i].StartL == list[j].StartL {
				if list[i].StartC == list[j].StartC {
					if list[i].EndL == list[j].EndL {
						if list[i].EndC == list[j].EndC {
							return list[i].Name < list[j].Name
						}
						return list[i].EndC < list[j].EndC
					}
					return list[i].EndL < list[j].EndL
				}
				return list[i].StartC < list[j].StartC
			}
			return list[i].StartL < list[j].StartL
		})
		roots := buildTree(list)
		grouped[f] = roots
	}

	// Write output
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	w := bufio.NewWriter(out)
	defer w.Flush()

	// Header
	fmt.Fprintf(w, "# context signature map\n")
	fmt.Fprintf(w, "# generated: %s\n\n", time.Now().Format(time.RFC3339))

	for _, f := range files {
		fmt.Fprintf(w, "FILE: %s\n", f)
		roots := grouped[f]
		writeNodes(w, roots, 0)
		fmt.Fprintln(w)
	}

	common.CLILogger.Info("Signature map written: %s (%d symbols)", outputPath, totalSymbols)
	return nil
}

func runContextCmd(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runContextMapCmd(cmd *cobra.Command, args []string) error {
	if formatJSON {
		return GenerateContextSignatureMapJSON(configPath, outPath)
	}
	return GenerateContextSignatureMap(configPath, outPath)
}

func runContextRelatedCmd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide at least one file path")
	}
	return FindRelatedFiles(configPath, args, formatJSON, maxDepth)
}

func runContextSymbolsCmd(cmd *cobra.Command, args []string) error {
	files := targetFiles
	if len(args) > 0 {
		files = args
	}
	return ExtractSymbols(configPath, files, formatJSON, includeRefs)
}

func runContextDependenciesCmd(cmd *cobra.Command, args []string) error {
	files := targetFiles
	if len(args) > 0 {
		files = args
	}
	return AnalyzeDependencies(configPath, files, formatJSON)
}

func containsRange(p, c *contextNode) bool {
	if c.StartL < p.StartL || (c.StartL == p.StartL && c.StartC < p.StartC) {
		return false
	}
	if c.EndL > p.EndL || (c.EndL == p.EndL && c.EndC > p.EndC) {
		return false
	}
	return true
}

func buildTree(nodes []*contextNode) []*contextNode {
	var roots []*contextNode
	var stack []*contextNode
	for _, n := range nodes {
		for len(stack) > 0 && !containsRange(stack[len(stack)-1], n) {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
		}
		stack = append(stack, n)
	}
	return roots
}

func writeNodes(w *bufio.Writer, nodes []*contextNode, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, n := range nodes {
		label := n.Name
		if n.Signature != "" {
			fmt.Fprintf(w, "%s- %s: %s\n", indent, label, n.Signature)
		} else {
			fmt.Fprintf(w, "%s- %s\n", indent, label)
		}
		if len(n.Children) > 0 {
			writeNodes(w, n.Children, depth+1)
		}
	}
}

// parseDocumentSymbolsResult normalizes an LSP documentSymbol result into []lsp.DocumentSymbol
func parseDocumentSymbolsResult(result interface{}) ([]lsp.DocumentSymbol, error) {
	if result == nil {
		return nil, nil
	}
	switch v := result.(type) {
	case []lsp.DocumentSymbol:
		return v, nil
	case []*lsp.DocumentSymbol:
		out := make([]lsp.DocumentSymbol, 0, len(v))
		for _, p := range v {
			if p != nil {
				out = append(out, *p)
			}
		}
		return out, nil
	case json.RawMessage:
		if len(v) == 0 || string(v) == "null" {
			return nil, nil
		}
		var ds []lsp.DocumentSymbol
		if err := json.Unmarshal(v, &ds); err == nil {
			return ds, nil
		}
		var si []types.SymbolInformation
		if err := json.Unmarshal(v, &si); err == nil {
			out := make([]lsp.DocumentSymbol, 0, len(si))
			for _, s := range si {
				out = append(out, lsp.DocumentSymbol{
					Name:           s.Name,
					Kind:           s.Kind,
					Range:          s.Location.Range,
					SelectionRange: s.Location.Range,
				})
			}
			return out, nil
		}
		return nil, fmt.Errorf("unable to parse document symbols result")
	case []byte:
		return parseDocumentSymbolsResult(json.RawMessage(v))
	case string:
		return parseDocumentSymbolsResult(json.RawMessage([]byte(v)))
	default:
		return nil, fmt.Errorf("unsupported document symbols result type: %T", result)
	}
}

// parseReferencesResult normalizes an LSP references result into []types.Location
func parseReferencesResult(result interface{}) ([]types.Location, error) {
	if result == nil {
		return nil, nil
	}
	switch v := result.(type) {
	case []types.Location:
		return v, nil
	case []*types.Location:
		out := make([]types.Location, 0, len(v))
		for _, p := range v {
			if p != nil {
				out = append(out, *p)
			}
		}
		return out, nil
	case json.RawMessage:
		if len(v) == 0 || string(v) == "null" {
			return nil, nil
		}
		var locs []types.Location
		if err := json.Unmarshal(v, &locs); err == nil {
			return locs, nil
		}
		return nil, fmt.Errorf("unable to parse references result")
	case []byte:
		return parseReferencesResult(json.RawMessage(v))
	case string:
		return parseReferencesResult(json.RawMessage([]byte(v)))
	default:
		return nil, fmt.Errorf("unsupported references result type: %T", result)
	}
}

// GenerateContextSignatureMapJSON creates a JSON signature map from indexed data
func GenerateContextSignatureMapJSON(configPath, outputPath string) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	if cmdCtx.Cache == nil {
		return fmt.Errorf("cache unavailable")
	}

	storage := cmdCtx.Cache.GetSCIPStorage()
	if storage == nil {
		return fmt.Errorf("scip storage unavailable")
	}

	type JSONOutput struct {
		Generated string                    `json:"generated"`
		Files     map[string][]*contextNode `json:"files"`
		Total     int                       `json:"total_symbols"`
	}

	output := JSONOutput{
		Generated: time.Now().Format(time.RFC3339),
		Files:     make(map[string][]*contextNode),
	}

	uris, err := storage.ListDocuments(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	for _, uri := range uris {
		doc, err := storage.GetDocument(context.Background(), uri)
		if err != nil || doc == nil {
			continue
		}

		file := utils.URIToFilePathCached(uri)
		var nodes []*contextNode

		for _, si := range doc.SymbolInformation {
			name := strings.TrimSpace(si.DisplayName)
			sig := strings.TrimSpace(si.SignatureDocumentation.Text)
			if name == "" && sig == "" {
				continue
			}
			nd := &contextNode{
				Name:      name,
				Signature: sig,
				StartL:    int(si.Range.Start.Line),
				StartC:    int(si.Range.Start.Character),
				EndL:      int(si.Range.End.Line),
				EndC:      int(si.Range.End.Character),
				Kind:      lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase),
			}
			nodes = append(nodes, nd)
			output.Total++
		}

		if len(nodes) > 0 {
			sort.Slice(nodes, func(i, j int) bool {
				if nodes[i].StartL != nodes[j].StartL {
					return nodes[i].StartL < nodes[j].StartL
				}
				return nodes[i].StartC < nodes[j].StartC
			})
			output.Files[file] = buildTree(nodes)
		}
	}

	if output.Total == 0 {
		return fmt.Errorf("no indexed symbols found; run 'lsp-gateway cache index' first")
	}

	if outputPath == "" {
		outputPath = "context-signature-map.json"
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	common.CLILogger.Info("JSON signature map written: %s (%d symbols)", outputPath, output.Total)
	return nil
}

// FindRelatedFiles finds files related to the given input files
func FindRelatedFiles(configPath string, inputFiles []string, formatJSON bool, maxDepth int) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}

	relatedFiles := make(map[string]*FileRelation)
	visited := make(map[string]bool)

	var targetFiles []string
	for _, f := range inputFiles {
		absPath, err := filepath.Abs(f)
		if err == nil {
			targetFiles = append(targetFiles, absPath)
			visited[absPath] = true
		}
	}

	for depth := 0; depth < maxDepth; depth++ {
		var newFiles []string
		for _, file := range targetFiles {
			uri := utils.FilePathToURICached(file)

			symbolCount := 0
			refSet := make(map[string]bool)
			refBySet := make(map[string]bool)

			if storage != nil {
				if doc, err := storage.GetDocument(context.Background(), uri); err == nil && doc != nil {
					symbolCount = len(doc.SymbolInformation)

					defSymbols := make(map[string]bool)
					for _, occ := range doc.Occurrences {
						if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
							defSymbols[occ.Symbol] = true
						}
						if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) || occ.SymbolRoles.HasRole(types.SymbolRoleImport) || occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
							if defs, err := storage.GetDefinitionsWithDocuments(context.Background(), occ.Symbol); err == nil {
								for _, d := range defs {
									defURI := utils.URIToFilePathCached(d.DocumentURI)
									if defURI != "" && defURI != file {
										refSet[defURI] = true
									}
								}
							}
						}
					}

					for sym := range defSymbols {
						if refs, err := storage.GetReferencesWithDocuments(context.Background(), sym); err == nil {
							for _, r := range refs {
								refURI := utils.URIToFilePathCached(r.DocumentURI)
								if refURI != "" && refURI != file {
									refBySet[refURI] = true
									if !visited[refURI] {
										newFiles = append(newFiles, refURI)
										visited[refURI] = true
									}
								}
							}
						}
					}
				}
			} else {
				// Fallback to LSP if no storage available
				result, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{
					TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				})
				if err == nil && result != nil {
					if symbols, err := parseDocumentSymbolsResult(result); err == nil && len(symbols) > 0 {
						symbolCount = countSymbols(symbols)
					}
				}

				if content, err := os.ReadFile(file); err == nil {
					lines := strings.Split(string(content), "\n")
					for lineNum := range lines {
						refsResult, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentReferences, &lsp.ReferenceParams{
							TextDocumentPositionParams: lsp.TextDocumentPositionParams{
								TextDocument: lsp.TextDocumentIdentifier{URI: uri},
								Position:     types.Position{Line: int32(lineNum), Character: 0},
							},
							Context: lsp.ReferenceContext{IncludeDeclaration: true},
						})
						if err == nil && refsResult != nil {
							if refs, err := parseReferencesResult(refsResult); err == nil {
								for _, ref := range refs {
									refFile := utils.URIToFilePathCached(ref.URI)
									if refFile != file && !visited[refFile] {
										newFiles = append(newFiles, refFile)
										visited[refFile] = true
										refSet[refFile] = true
										refBySet[refFile] = true
									}
								}
							}
						}
					}
				}
			}

			if relation, ok := relatedFiles[file]; ok {
				relation.Symbols += symbolCount
			} else {
				relatedFiles[file] = &FileRelation{File: file, Symbols: symbolCount}
			}

			if rel := relatedFiles[file]; rel != nil {
				for f := range refSet {
					rel.References = append(rel.References, f)
				}
				for f := range refBySet {
					rel.ReferencedBy = append(rel.ReferencedBy, f)
				}
			}
		}

		if len(newFiles) == 0 {
			break
		}
		targetFiles = newFiles
	}

	if formatJSON {
		data, err := json.MarshalIndent(relatedFiles, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Println("# Related Files Analysis")
		fmt.Printf("# Found %d related files\n\n", len(relatedFiles))
		for _, rel := range relatedFiles {
			fmt.Printf("File: %s\n", rel.File)
			if rel.Symbols > 0 {
				fmt.Printf("  Symbols: %d\n", rel.Symbols)
			}
			if len(rel.References) > 0 {
				fmt.Printf("  References: %s\n", strings.Join(rel.References, ", "))
			}
			if len(rel.ReferencedBy) > 0 {
				fmt.Printf("  Referenced by: %s\n", strings.Join(rel.ReferencedBy, ", "))
			}
			fmt.Println()
		}
	}

	return nil
}

// ExtractSymbols extracts symbols from specified files
func ExtractSymbols(configPath string, files []string, formatJSON bool, includeRefs bool) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	var symbols []SymbolInfo

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}

	if len(files) == 0 && storage != nil {
		if uris, _ := storage.ListDocuments(context.Background()); len(uris) > 0 {
			for _, uri := range uris {
				files = append(files, utils.URIToFilePathCached(uri))
			}
		}
	}

	for _, file := range files {
		absPath, _ := filepath.Abs(file)
		uri := utils.FilePathToURICached(absPath)

		if storage != nil {
			if doc, err := storage.GetDocument(context.Background(), uri); err == nil && doc != nil {
				for _, si := range doc.SymbolInformation {
					info := SymbolInfo{
						Name:      si.DisplayName,
						Kind:      lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase),
						File:      absPath,
						Line:      int(si.Range.Start.Line),
						Signature: si.SignatureDocumentation.Text,
					}
					if includeRefs {
						if refs, err := storage.GetReferencesWithDocuments(context.Background(), si.Symbol); err == nil {
							for _, r := range refs {
								refFile := utils.URIToFilePathCached(r.DocumentURI)
								info.References = append(info.References, fmt.Sprintf("%s:%d", refFile, r.Range.Start.Line))
							}
						}
					}
					symbols = append(symbols, info)
				}
				continue
			}
		}

		// Fallback to LSP
		result, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
		if err != nil {
			continue
		}
		var extractSymbols func([]lsp.DocumentSymbol)
		extractSymbols = func(syms []lsp.DocumentSymbol) {
			for _, sym := range syms {
				info := SymbolInfo{
					Name:      sym.Name,
					Kind:      lspconv.LSPSymbolKindToString(sym.Kind, lspconv.StyleLowercase),
					File:      absPath,
					Line:      int(sym.Range.Start.Line),
					Signature: sym.Detail,
				}
				if includeRefs {
					refsResult, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentReferences, &lsp.ReferenceParams{
						TextDocumentPositionParams: lsp.TextDocumentPositionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: sym.Range.Start},
						Context:                    lsp.ReferenceContext{IncludeDeclaration: false},
					})
					if err == nil && refsResult != nil {
						if refs, err := parseReferencesResult(refsResult); err == nil {
							for _, ref := range refs {
								refFile := utils.URIToFilePathCached(ref.URI)
								info.References = append(info.References, fmt.Sprintf("%s:%d", refFile, ref.Range.Start.Line))
							}
						}
					}
				}
				symbols = append(symbols, info)
				if len(sym.Children) > 0 {
					children := make([]lsp.DocumentSymbol, 0, len(sym.Children))
					for _, child := range sym.Children {
						if child != nil {
							children = append(children, *child)
						}
					}
					extractSymbols(children)
				}
			}
		}
		if result != nil {
			if docSymbols, err := parseDocumentSymbolsResult(result); err == nil && len(docSymbols) > 0 {
				extractSymbols(docSymbols)
			}
		}
	}

	// Output results
	if formatJSON {
		data, err := json.MarshalIndent(symbols, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("# Extracted Symbols (%d total)\n\n", len(symbols))

		currentFile := ""
		for _, sym := range symbols {
			if sym.File != currentFile {
				currentFile = sym.File
				fmt.Printf("\nFile: %s\n", currentFile)
			}
			fmt.Printf("  %s %s (line %d)", sym.Kind, sym.Name, sym.Line)
			if sym.Signature != "" {
				fmt.Printf(": %s", sym.Signature)
			}
			if len(sym.References) > 0 {
				fmt.Printf(" [%d refs]", len(sym.References))
			}
			fmt.Println()
		}
	}

	return nil
}

// Helper function to count symbols recursively
func countSymbols(symbols []lsp.DocumentSymbol) int {
	count := len(symbols)
	for _, sym := range symbols {
		if len(sym.Children) > 0 {
			// Convert pointers to values
			children := make([]lsp.DocumentSymbol, 0, len(sym.Children))
			for _, child := range sym.Children {
				if child != nil {
					children = append(children, *child)
				}
			}
			count += countSymbols(children)
		}
	}
	return count
}

// AnalyzeDependencies analyzes dependencies for specified files using only LSP data
func AnalyzeDependencies(configPath string, files []string, formatJSON bool) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	dependencies := make(map[string]*DependencyInfo)

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}

	if len(files) == 0 && storage != nil {
		if uris, _ := storage.ListDocuments(context.Background()); len(uris) > 0 {
			for _, uri := range uris {
				files = append(files, utils.URIToFilePathCached(uri))
			}
		}
	}

	for _, file := range files {
		absPath, _ := filepath.Abs(file)
		uri := utils.FilePathToURICached(absPath)

		dep := &DependencyInfo{File: absPath, Exports: []ExportSymbol{}, SymbolCount: 0}

		if storage != nil {
			if doc, err := storage.GetDocument(context.Background(), uri); err == nil && doc != nil {
				dep.SymbolCount = len(doc.SymbolInformation)
				// Determine exported symbols by definition in this file and uppercase heuristic
				defSymbols := make(map[string]bool)
				for _, occ := range doc.Occurrences {
					if occ.SymbolRoles&types.SymbolRoleDefinition != 0 {
						defSymbols[occ.Symbol] = true
					}
				}
				for _, si := range doc.SymbolInformation {
					if !defSymbols[si.Symbol] {
						continue
					}
					name := si.DisplayName
					if name != "" && unicode.IsUpper(rune(name[0])) {
						dep.Exports = append(dep.Exports, ExportSymbol{Name: name, Kind: lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase)})
					}
				}
				dependencies[absPath] = dep
				continue
			}
		}

		// Fallback to LSP
		result, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
		if err == nil && result != nil {
			var processSymbols func([]lsp.DocumentSymbol)
			processSymbols = func(syms []lsp.DocumentSymbol) {
				for _, sym := range syms {
					dep.SymbolCount++
					if sym.Name != "" && unicode.IsUpper(rune(sym.Name[0])) {
						dep.Exports = append(dep.Exports, ExportSymbol{Name: sym.Name, Kind: lspconv.LSPSymbolKindToString(sym.Kind, lspconv.StyleLowercase)})
					}
					if len(sym.Children) > 0 {
						children := make([]lsp.DocumentSymbol, 0, len(sym.Children))
						for _, child := range sym.Children {
							if child != nil {
								children = append(children, *child)
							}
						}
						processSymbols(children)
					}
				}
			}
			if docSymbols, err := parseDocumentSymbolsResult(result); err == nil && len(docSymbols) > 0 {
				processSymbols(docSymbols)
			}
		}
		dependencies[absPath] = dep
	}

	if formatJSON {
		data, err := json.MarshalIndent(dependencies, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("# Dependencies Analysis (%d files)\n\n", len(dependencies))
		for file, dep := range dependencies {
			relPath, _ := filepath.Rel(workingDir, file)
			fmt.Printf("File: %s\n", relPath)
			fmt.Printf("  Total Symbols: %d\n", dep.SymbolCount)
			if len(dep.Exports) > 0 {
				fmt.Printf("  Exported Symbols (%d):\n", len(dep.Exports))
				for _, exp := range dep.Exports {
					fmt.Printf("    - %s %s\n", exp.Kind, exp.Name)
				}
			}
			fmt.Println()
		}
	}

	return nil
}
