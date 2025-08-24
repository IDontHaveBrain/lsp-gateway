package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

type SymbolRef struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	DefFile string `json:"def_file"`
	DefLine int    `json:"def_line"`
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
type fileRefSets struct {
	refSet   map[string]bool
	refBySet map[string]bool
}

func collectRelationsForFile(storage scip.SCIPDocumentStorage, file string, visited map[string]bool) (int, fileRefSets, []string) {
	uri := utils.FilePathToURICached(file)
	symbolCount := 0
	refs := fileRefSets{refSet: make(map[string]bool), refBySet: make(map[string]bool)}
	var newFiles []string
	if storage == nil {
		return 0, refs, newFiles
	}
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
							refs.refSet[defURI] = true
						}
					}
				}
			}
		}
		for sym := range defSymbols {
			if theRefs, err := storage.GetReferencesWithDocuments(context.Background(), sym); err == nil {
				for _, r := range theRefs {
					refURI := utils.URIToFilePathCached(r.DocumentURI)
					if refURI != "" && refURI != file {
						refs.refBySet[refURI] = true
						if !visited[refURI] {
							newFiles = append(newFiles, refURI)
							visited[refURI] = true
						}
					}
				}
			}
		}
	}
	return symbolCount, refs, newFiles
}
func updateRelation(rel *FileRelation, symbolCount int, refs fileRefSets) {
	rel.Symbols += symbolCount
	for f := range refs.refSet {
		rel.References = append(rel.References, f)
	}
	for f := range refs.refBySet {
		rel.ReferencedBy = append(rel.ReferencedBy, f)
	}
}
func printRelated(relatedFiles map[string]*FileRelation, formatJSON bool) error {
	if formatJSON {
		data, err := json.MarshalIndent(relatedFiles, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
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
	return nil
}

// GenerateContextSignatureMap creates a .txt signature map from indexed data
func GenerateContextSignatureMap(configPath, outputPath string) error {
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
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
	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil && filepath.Dir(outputPath) != "." {
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
	defer func() { _ = out.Close() }()

	w := bufio.NewWriter(out)
	defer func() { _ = w.Flush() }()

	// Header
	_, _ = fmt.Fprintf(w, "# context signature map\n")
	_, _ = fmt.Fprintf(w, "# generated: %s\n\n", time.Now().Format(time.RFC3339))

	for _, f := range files {
		_, _ = fmt.Fprintf(w, "FILE: %s\n", f)
		roots := grouped[f]
		writeNodes(w, roots, 0)
		_, _ = fmt.Fprintln(w)
	}

	common.CLILogger.Info("Signature map written: %s (%d symbols)", outputPath, totalSymbols)
	return nil
}

func runContextCmd(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runContextMapCmd(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("please provide exactly one file path")
	}
	return PrintReferencedFilesCode(configPath, args[0])
}

func PrintReferencedFilesCode(configPath string, inputFile string) error {
	var (
		cmdCtx *clicommon.CommandContext
		err    error
	)
	cmdCtx, err = clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	absPath, err := filepath.Abs(inputFile)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	if st, err := os.Stat(absPath); err != nil || st.IsDir() {
		return fmt.Errorf("file not found or is a directory: %s", absPath)
	}

	uri := utils.FilePathToURICached(absPath)
	// Determine project root for filtering (prefer repo root with go.mod/.git)
	workspaceRoot := ""
	fileExists := func(p string) bool {
		if st, err := os.Stat(p); err == nil && st.IsDir() || (err == nil && !st.IsDir()) {
			return true
		}
		return false
	}
	root := filepath.Dir(absPath)
	for {
		if fileExists(filepath.Join(root, ".git")) || fileExists(filepath.Join(root, "go.mod")) {
			workspaceRoot = root
			break
		}
		parent := filepath.Dir(root)
		if parent == root { // reached filesystem root
			break
		}
		root = parent
	}
	if workspaceRoot == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceRoot = wd
		}
	}

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}
	if storage == nil {
		return fmt.Errorf("index unavailable; run 'lsp-gateway cache index' and try again")
	}

	// Heuristic: trivial header range (likely module jump) — first line small span
	isTrivialHeaderRange := func(r types.Range) bool {
		if r.End.Line < r.Start.Line {
			return true
		}
		if r.Start.Line > 1 || r.End.Line > 1 {
			return false
		}
		if r.End.Line == r.Start.Line {
			if (r.End.Character - r.Start.Character) <= 16 {
				return true
			}
		}
		return false
	}

	// Check if the target file has a symbol with the given display name in index
	hasSymbolByName := func(path, name string) bool {
		if name == "" {
			return false
		}
		defURI := utils.FilePathToURICached(path)
		if defURI == "" {
			return false
		}
		if ddoc, err := storage.GetDocument(context.Background(), defURI); err == nil && ddoc != nil {
			for _, si := range ddoc.SymbolInformation {
				if strings.EqualFold(si.DisplayName, name) {
					return true
				}
			}
		}
		return false
	}

	shouldDropTrivialHeader := func(path string, r types.Range, name string) bool {
		return isTrivialHeaderRange(r) && !hasSymbolByName(path, name)
	}

	// Generic single-line check to trigger LSP refinement when needed
	isSingleLineRange := func(r types.Range) bool {
		return r.Start.Line == r.End.Line
	}

	// Skip noisy/non-source files from results
	isSkippablePath := func(path string) bool {
		p := filepath.ToSlash(path)
		if strings.HasSuffix(p, "_test.go") {
			return true
		}
		if strings.Contains(p, "/tests/") {
			return true
		}
		return false
	}

	type snippet struct {
		file   string
		name   string
		startL int
		startC int
		endL   int
		endC   int
	}

	// Collect referenced symbol definitions -> snippets
	doc, err := storage.GetDocument(context.Background(), uri)
	if err != nil || doc == nil {
		return fmt.Errorf("document not indexed: %s", absPath)
	}

	// Map: file -> list of snippets
	snippetMap := make(map[string][]snippet)
	// Dedup by (file, symbol)
	seen := make(map[string]bool)
	// Track symbols actually referenced in this document
	referenced := make(map[string]bool)

	// Range helpers and language-agnostic expansion to full symbol range
	rangeContains := func(outer, inner types.Range) bool {
		if inner.Start.Line < outer.Start.Line || (inner.Start.Line == outer.Start.Line && inner.Start.Character < outer.Start.Character) {
			return false
		}
		if inner.End.Line > outer.End.Line || (inner.End.Line == outer.End.Line && inner.End.Character > outer.End.Character) {
			return false
		}
		return true
	}
	rangeSize := func(r types.Range) int64 {
		return int64(r.End.Line-r.Start.Line)*100000 + int64(r.End.Character-r.Start.Character)
	}
	rangeIntersects := func(a, b types.Range) bool {
		if a.End.Line < b.Start.Line || (a.End.Line == b.Start.Line && a.End.Character <= b.Start.Character) {
			return false
		}
		if b.End.Line < a.Start.Line || (b.End.Line == a.Start.Line && b.End.Character <= a.Start.Character) {
			return false
		}
		return true
	}

	// Note: do not parse code; rely solely on LSP responses and indexed data

	nameMatches := func(a, b string) bool {
		if a == "" || b == "" {
			return false
		}
		al := strings.ToLower(a)
		bl := strings.ToLower(b)
		if al == bl {
			return true
		}
		// tolerate signatures like name(...)
		if strings.HasPrefix(al, bl) || strings.HasPrefix(bl, al) {
			return true
		}
		return false
	}
	expandRangeToSymbol := func(defPath string, base types.Range, name string) types.Range {
		best := base
		defURI := utils.FilePathToURICached(defPath)
		// Try SCIP index ranges first
		if storage != nil && defURI != "" {
			if ddoc, err := storage.GetDocument(context.Background(), defURI); err == nil && ddoc != nil {
				matchBest := best
				var insideBase []types.Range
				var containsBase []types.Range
				containerSet := false
				var containerBest types.Range
				for _, si := range ddoc.SymbolInformation {
					r := si.Range
					if nameMatches(si.DisplayName, name) {
						if rangeContains(best, r) || rangeIntersects(r, best) {
							insideBase = append(insideBase, r)
						} else if rangeContains(r, best) {
							containsBase = append(containsBase, r)
						}
					}
					if rangeContains(r, best) {
						if !containerSet || rangeSize(r) < rangeSize(containerBest) {
							containerBest = r
							containerSet = true
						}
					}
				}
				if len(insideBase) > 0 {
					// choose largest candidate inside base (likely full method rather than selection)
					chosen := insideBase[0]
					for _, r := range insideBase[1:] {
						if rangeSize(r) > rangeSize(chosen) {
							chosen = r
						}
					}
					matchBest = chosen
				} else if len(containsBase) > 0 {
					// choose smallest candidate that still contains the base
					chosen := containsBase[0]
					for _, r := range containsBase[1:] {
						if rangeSize(r) < rangeSize(chosen) {
							chosen = r
						}
					}
					matchBest = chosen
				} else if containerSet && rangeSize(containerBest) > rangeSize(best) {
					matchBest = containerBest
				}
				best = matchBest
			}
		}

		// If still trivial header but index has a symbol with this name, snap to it
		if isSingleLineRange(best) && name != "" && storage != nil && defURI != "" {
			if ddoc, err := storage.GetDocument(context.Background(), defURI); err == nil && ddoc != nil {
				for _, si := range ddoc.SymbolInformation {
					if strings.EqualFold(si.DisplayName, name) {
						best = si.Range
						break
					}
				}
			}
		}
		// Do not attempt to expand by parsing source code; keep ranges from LSP/SCIP only
		return best
	}

	for _, occ := range doc.Occurrences {
		if occ.Symbol == "" {
			continue
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) || occ.SymbolRoles.HasRole(types.SymbolRoleGenerated) {
			continue
		}
		referenced[occ.Symbol] = true

		defs, err := storage.GetDefinitionsWithDocuments(context.Background(), occ.Symbol)
		if err == nil && len(defs) > 0 {
			symInfo, _ := storage.GetSymbolInfo(context.Background(), occ.Symbol)
			if symInfo != nil {
				if symInfo.Kind == scip.SCIPSymbolKindModule || symInfo.Kind == scip.SCIPSymbolKindFile || symInfo.Kind == scip.SCIPSymbolKindPackage || symInfo.Kind == scip.SCIPSymbolKindNamespace {
					continue
				}
			}
			// Prefer the largest full range per definition path for this symbol
			bestByPath := make(map[string]types.Range)
			name := ""
			if symInfo != nil && symInfo.DisplayName != "" {
				name = symInfo.DisplayName
			}
			for _, d := range defs {
				defPath := utils.URIToFilePathCached(d.DocumentURI)
				if defPath == "" || defPath == absPath {
					continue
				}
				if isSkippablePath(defPath) {
					continue
				}
				if workspaceRoot != "" {
					rp, _ := filepath.Abs(defPath)
					rr, _ := filepath.Abs(workspaceRoot)
					if !strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr {
						continue
					}
				}
				cand := expandRangeToSymbol(defPath, d.Range, name)
				cur, ok := bestByPath[defPath]
				if !ok || rangeSize(cand) > rangeSize(cur) {
					bestByPath[defPath] = cand
				}
			}
			for defPath, rng := range bestByPath {
				key := defPath + "::" + occ.Symbol
				if seen[key] {
					continue
				}
				if shouldDropTrivialHeader(defPath, rng, name) {
					continue
				}
				seen[key] = true
				snippetMap[defPath] = append(snippetMap[defPath], snippet{
					file:   defPath,
					name:   name,
					startL: int(rng.Start.Line),
					startC: int(rng.Start.Character),
					endL:   int(rng.End.Line),
					endC:   int(rng.End.Character),
				})
			}
			continue
		}

	}

	for _, si := range doc.SymbolInformation {
		if si.Symbol == "" {
			continue
		}
		if !referenced[si.Symbol] {
			continue
		}
		defs, err := storage.GetDefinitionsWithDocuments(context.Background(), si.Symbol)
		if err != nil || len(defs) == 0 {
			continue
		}
		for _, d := range defs {
			defPath := utils.URIToFilePathCached(d.DocumentURI)
			if defPath == "" || defPath == absPath {
				continue
			}
			if isSkippablePath(defPath) {
				continue
			}
			if workspaceRoot != "" {
				rp, _ := filepath.Abs(defPath)
				rr, _ := filepath.Abs(workspaceRoot)
				if !strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr {
					continue
				}
			}
			key := defPath + "::" + si.Symbol
			if seen[key] {
				continue
			}
			seen[key] = true
			rng := si.Range
			if rng.Start.Line == 0 && rng.End.Line == 0 {
				rng = d.Range
			}
			name := si.DisplayName
			if name == "" {
				name = si.Symbol
			}
			rng = expandRangeToSymbol(defPath, rng, name)
			if shouldDropTrivialHeader(defPath, rng, name) {
				continue
			}
			sn := snippet{
				file:   defPath,
				name:   name,
				startL: int(rng.Start.Line),
				startC: int(rng.Start.Character),
				endL:   int(rng.End.Line),
				endC:   int(rng.End.Character),
			}
			snippetMap[defPath] = append(snippetMap[defPath], sn)
		}
	}

	// defer empty-check until after LSP fallback scan

	// Defer computing files until after potential LSP fallback augmentation
	var files []string

	// Helper: merge overlapping/adjacent snippets per file
	merge := func(list []snippet) []snippet {
		sort.Slice(list, func(i, j int) bool {
			if list[i].startL == list[j].startL {
				return list[i].startC < list[j].startC
			}
			return list[i].startL < list[j].startL
		})
		var out []snippet
		for _, s := range list {
			if len(out) == 0 {
				out = append(out, s)
				continue
			}
			last := &out[len(out)-1]
			// Overlap check
			if s.startL < last.endL || (s.startL == last.endL && s.startC <= last.endC) {
				// extend end if needed
				if s.endL > last.endL || (s.endL == last.endL && s.endC > last.endC) {
					last.endL = s.endL
					last.endC = s.endC
				}
				continue
			}
			out = append(out, s)
		}
		return out
	}

	// Helper: extract full lines covering the range (print entire code lines)
	extract := func(content []byte, s snippet) string {
		lines := strings.Split(string(content), "\n")
		if s.startL < 0 {
			s.startL = 0
		}
		if s.endL >= len(lines) {
			s.endL = len(lines) - 1
		}
		var b strings.Builder
		for ln := s.startL; ln <= s.endL && ln < len(lines); ln++ {
			b.WriteString(lines[ln])
			if ln < s.endL {
				b.WriteByte('\n')
			}
		}
		return b.String()
	}

	// Fallback LSP scan: walk tokens and resolve definitions when storage misses
	// No token refinement; keep full ranges

	_ = func(path, name string, r types.Range) {
		if path == "" || path == absPath {
			return
		}
		if isSkippablePath(path) {
			return
		}
		if workspaceRoot != "" {
			rp, _ := filepath.Abs(path)
			rr, _ := filepath.Abs(workspaceRoot)
			if !strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr {
				return
			}
		}
		// Expand to full symbol range when possible
		r = expandRangeToSymbol(path, r, name)
		if shouldDropTrivialHeader(path, r, name) {
			return
		}

		key := path + "::" + name + fmt.Sprintf("@%d:%d-%d:%d", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
		if seen[key] {
			return
		}
		seen[key] = true
		snippetMap[path] = append(snippetMap[path], snippet{
			file:   path,
			name:   name,
			startL: int(r.Start.Line),
			startC: int(r.Start.Character),
			endL:   int(r.End.Line),
			endC:   int(r.End.Character),
		})
	}

	// If still empty after LSP fallback, report error
	if len(snippetMap) == 0 {
		return fmt.Errorf("no referenced code found for: %s", absPath)
	}

	// Compute files now (after augmentation) and print snippets grouped by file
	files = make([]string, 0, len(snippetMap))
	for f := range snippetMap {
		files = append(files, f)
	}
	sort.Strings(files)

	// Print snippets grouped by file, using indexed content
	for _, f := range files {
		if isSkippablePath(f) {
			continue
		}
		// Skip stdlib dirs
		if strings.Contains(f, string(os.PathSeparator)+".g"+string(os.PathSeparator)+"go"+string(os.PathSeparator)+"src"+string(os.PathSeparator)) {
			continue
		}
		if workspaceRoot != "" {
			rp, _ := filepath.Abs(f)
			rr, _ := filepath.Abs(workspaceRoot)
			if !strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr {
				continue
			}
		}
		merged := merge(snippetMap[f])
		fmt.Printf("\n# FILE: %s\n", f)
		// Always read from disk to ensure latest content and full lines
		data, derr := os.ReadFile(f)
		if derr != nil {
			fmt.Printf("# (unreadable)\n")
			continue
		}
		for _, sn := range merged {
			fmt.Printf("# SNIPPET %s [%d:%d-%d:%d]\n", sn.name, sn.startL, sn.startC, sn.endL, sn.endC)
			text := extract(data, sn)
			fmt.Print(text)
			fmt.Print("\n")
		}
	}

	return nil
}

func parseDefinitionResult(result interface{}) ([]types.Location, error) {
	if result == nil {
		return nil, nil
	}
	switch v := result.(type) {
	case types.Location:
		return []types.Location{v}, nil
	case *types.Location:
		if v == nil {
			return nil, nil
		}
		return []types.Location{*v}, nil
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
		var arr []types.Location
		if err := json.Unmarshal(v, &arr); err == nil {
			return arr, nil
		}
		var single types.Location
		if err := json.Unmarshal(v, &single); err == nil {
			return []types.Location{single}, nil
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
				// Prefer full targetRange; use selection if full is absent
				rng := l.TargetRange
				if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
					rng = l.TargetSelectionRange
				}
				out = append(out, types.Location{URI: l.TargetURI, Range: rng})
			}
			return out, nil
		}
		// Try single LocationLink
		var link locationLink
		if err := json.Unmarshal(v, &link); err == nil && link.TargetURI != "" {
			rng := link.TargetRange
			if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
				rng = link.TargetSelectionRange
			}
			return []types.Location{{URI: link.TargetURI, Range: rng}}, nil
		}
		return nil, fmt.Errorf("unable to parse definition result")
	case []byte:
		return parseDefinitionResult(json.RawMessage(v))
	case string:
		return parseDefinitionResult(json.RawMessage([]byte(v)))
	default:
		return nil, fmt.Errorf("unsupported definition result type: %T", result)
	}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func extractFileDefsRefsWithContext(cmdCtx *clicommon.CommandContext, file string, formatJSON bool) error {
	absPath, _ := filepath.Abs(file)
	uri := utils.FilePathToURICached(absPath)

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}

	if storage == nil {
		return fmt.Errorf("index unavailable; run 'lsp-gateway cache index' and try again")
	}

	doc, _ := storage.GetDocument(context.Background(), uri)
	if doc == nil {
		return fmt.Errorf("document not indexed: %s", absPath)
	}

	defs := make([]SymbolInfo, 0, len(doc.SymbolInformation))
	for _, si := range doc.SymbolInformation {
		defs = append(defs, SymbolInfo{
			Name:      si.DisplayName,
			Kind:      lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase),
			File:      absPath,
			Line:      int(si.Range.Start.Line),
			Signature: si.SignatureDocumentation.Text,
		})
	}

	refs := []SymbolRef{}
	seen := make(map[string]bool)
	parseLocationSymbol := func(sym string) (string, int, bool) {
		const prefix = "symbol_"
		if !strings.HasPrefix(sym, prefix) {
			return "", 0, false
		}
		rest := sym[len(prefix):]
		i2 := strings.LastIndex(rest, "_")
		if i2 <= 0 {
			return "", 0, false
		}
		i1 := strings.LastIndex(rest[:i2], "_")
		if i1 <= 0 {
			return "", 0, false
		}
		uriPart := rest[:i1]
		linePart := rest[i1+1 : i2]
		defPath := utils.URIToFilePathCached(uriPart)
		if defPath == "" {
			defPath = uriPart
		}
		defLine := 0
		allDigits := true
		for _, ch := range linePart {
			if ch < '0' || ch > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			if n, err := strconv.Atoi(linePart); err == nil {
				defLine = n
			}
		}
		return defPath, defLine, true
	}
	for _, occ := range doc.Occurrences {
		if occ.Symbol == "" {
			continue
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) || occ.SymbolRoles.HasRole(types.SymbolRoleGenerated) {
			continue
		}
		if seen[occ.Symbol] {
			continue
		}
		symInfo, _ := storage.GetSymbolInfo(context.Background(), occ.Symbol)
		if symInfo != nil {
			if symInfo.Kind == scip.SCIPSymbolKindModule || symInfo.Kind == scip.SCIPSymbolKindFile || symInfo.Kind == scip.SCIPSymbolKindPackage || symInfo.Kind == scip.SCIPSymbolKindNamespace {
				continue
			}
		}
		defsWithDocs, err := storage.GetDefinitionsWithDocuments(context.Background(), occ.Symbol)
		var defPath string
		var defLine int
		if err == nil && len(defsWithDocs) > 0 {
			for _, d := range defsWithDocs {
				p := utils.URIToFilePathCached(d.DocumentURI)
				if p == "" {
					continue
				}
				defPath = p
				defLine = int(d.Range.Start.Line)
				break
			}
		}
		if defPath == "" {
			if pth, ln, ok := parseLocationSymbol(occ.Symbol); ok {
				defPath = pth
				defLine = ln
			}
		}
		if defPath == "" {
			continue
		}
		name := occ.Symbol
		kind := ""
		if symInfo != nil {
			if symInfo.DisplayName != "" {
				name = symInfo.DisplayName
			}
			kind = lspconv.SCIPSymbolKindToString(symInfo.Kind, lspconv.StyleLowercase)
		}
		refs = append(refs, SymbolRef{Name: name, Kind: kind, DefFile: defPath, DefLine: defLine})
		seen[occ.Symbol] = true
	}

	if formatJSON {
		payload := map[string]interface{}{
			"file": absPath,
			"defs": defs,
			"refs": refs,
		}
		data, jerr := json.MarshalIndent(payload, "", "  ")
		if jerr != nil {
			return fmt.Errorf("failed to marshal JSON: %w", jerr)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("File: %s\n", absPath)
	fmt.Println("Defs:")
	for _, d := range defs {
		if d.Signature != "" {
			fmt.Printf("  %s %s (line %d): %s\n", d.Kind, d.Name, d.Line, d.Signature)
		} else {
			fmt.Printf("  %s %s (line %d)\n", d.Kind, d.Name, d.Line)
		}
	}
	fmt.Println("Refs:")
	for _, r := range refs {
		if r.Kind != "" {
			fmt.Printf("  %s %s -> %s:%d\n", r.Kind, r.Name, r.DefFile, r.DefLine)
		} else {
			fmt.Printf("  %s -> %s:%d\n", r.Name, r.DefFile, r.DefLine)
		}
	}
	return nil
}

func ExtractFileDefsRefs(configPath string, file string, formatJSON bool) error {
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()
	return extractFileDefsRefsWithContext(cmdCtx, file, formatJSON)
}

func runContextSymbolsCmd(cmd *cobra.Command, args []string) error {
	files := targetFiles
	if len(args) > 0 {
		files = args
	}
	if len(files) == 0 {
		cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
		if err != nil {
			return err
		}
		defer cmdCtx.Cleanup()

		var storage scip.SCIPDocumentStorage
		if cmdCtx.Cache != nil {
			storage = cmdCtx.Cache.GetSCIPStorage()
		}
		if storage == nil {
			return fmt.Errorf("index unavailable; run 'lsp-gateway cache index' and try again")
		}
		uris, err := storage.ListDocuments(context.Background())
		if err != nil {
			return err
		}
		allFiles := make([]string, 0, len(uris))
		for _, uri := range uris {
			p := utils.URIToFilePathCached(uri)
			if p == "" {
				p = uri
			}
			allFiles = append(allFiles, p)
		}
		sort.Strings(allFiles)
		for _, f := range allFiles {
			if err := extractFileDefsRefsWithContext(cmdCtx, f, formatJSON); err != nil {
				continue
			}
		}
		return nil
	}
	if len(files) == 1 {
		return ExtractFileDefsRefs(configPath, files[0], formatJSON)
	}
	return ExtractSymbols(configPath, files, formatJSON, includeRefs)
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
			_, _ = fmt.Fprintf(w, "%s- %s: %s\n", indent, label, n.Signature)
		} else {
			_, _ = fmt.Fprintf(w, "%s- %s\n", indent, label)
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
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
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

	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	common.CLILogger.Info("JSON signature map written: %s (%d symbols)", outputPath, output.Total)
	return nil
}

// FindRelatedFiles finds files related to the given input files
func FindRelatedFiles(configPath string, inputFiles []string, formatJSON bool, maxDepth int) error {
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
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
			symbolCount, refs, newly := collectRelationsForFile(storage, file, visited)
			if relation, ok := relatedFiles[file]; ok {
				relation.Symbols += symbolCount
			} else {
				relatedFiles[file] = &FileRelation{File: file, Symbols: symbolCount}
			}
			if rel := relatedFiles[file]; rel != nil {
				updateRelation(rel, symbolCount, refs)
			}
			newFiles = append(newFiles, newly...)
		}
		if len(newFiles) == 0 {
			break
		}
		targetFiles = newFiles
	}
	if err := printRelated(relatedFiles, formatJSON); err != nil {
		return err
	}

	return nil
}

// ExtractSymbols extracts symbols from specified files
func ExtractSymbols(configPath string, files []string, formatJSON bool, includeRefs bool) error {
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
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
	cmdCtx, err := clicommon.NewCacheOnlyContext(configPath, 3*time.Minute)
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
