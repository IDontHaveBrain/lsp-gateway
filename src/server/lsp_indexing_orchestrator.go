package server

import (
    "fmt"
    "os"
    "time"

    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/internal/constants"
    "lsp-gateway/src/internal/types"
    "lsp-gateway/src/server/cache"
    "lsp-gateway/src/server/watcher"
    "lsp-gateway/src/utils"
)

func (m *LSPManager) scheduleIndexing(method, uri, language string, params, result interface{}) {
    go func() {
        m.indexLimiter <- struct{}{}
        defer func() { <-m.indexLimiter }()
        timeout := 10 * time.Second
        switch method {
        case types.MethodWorkspaceSymbol:
            timeout = 20 * time.Second
        case types.MethodTextDocumentReferences:
            timeout = 15 * time.Second
        case types.MethodTextDocumentDocumentSymbol:
            timeout = 12 * time.Second
        case types.MethodTextDocumentDefinition, types.MethodTextDocumentCompletion, types.MethodTextDocumentHover:
            timeout = 10 * time.Second
        }
        timeout = constants.AdjustDurationForWindows(timeout, 1.5)
        idxCtx, cancel := common.CreateContext(timeout)
        defer cancel()
        m.performSCIPIndexing(idxCtx, method, uri, language, params, result)
    }()
}

func (m *LSPManager) startFileWatcher() error {
    m.watcherMu.Lock()
    defer m.watcherMu.Unlock()
    extensions := constants.GetAllSupportedExtensions()
    fw, err := watcher.NewFileWatcher(extensions, m.handleFileChanges)
    if err != nil { return fmt.Errorf("failed to create file watcher: %w", err) }
    wd, err := os.Getwd()
    if err != nil { return fmt.Errorf("failed to get working directory: %w", err) }
    if err := fw.AddPath(wd); err != nil { return fmt.Errorf("failed to add watch path: %w", err) }
    fw.Start(); m.fileWatcher = fw
    common.LSPLogger.Info("File watcher started for real-time change detection")
    return nil
}

func (m *LSPManager) handleFileChanges(events []watcher.FileChangeEvent) {
    if len(events) == 0 { return }
    if m.scipCache == nil { return }
    var modifiedFiles []string
    var deletedFiles []string
    for _, event := range events {
        switch event.Operation {
        case "write", "create":
            modifiedFiles = append(modifiedFiles, event.Path)
        case "remove":
            deletedFiles = append(deletedFiles, event.Path)
        }
    }
    common.LSPLogger.Debug("File changes detected: %d modified, %d deleted", len(modifiedFiles), len(deletedFiles))
    for _, path := range deletedFiles {
        uri := utils.FilePathToURI(path)
        if err := m.scipCache.InvalidateDocument(uri); err != nil {
            common.LSPLogger.Warn("Failed to invalidate deleted document %s: %v", uri, err)
        }
    }
    if len(modifiedFiles) > 0 { go m.performIncrementalReindex(modifiedFiles) }
}

func (m *LSPManager) performIncrementalReindex(files []string) {
    m.indexLimiter <- struct{}{}
    defer func() { <-m.indexLimiter }()
    cacheManager, ok := m.scipCache.(*cache.SCIPCacheManager)
    if !ok {
        common.LSPLogger.Warn("Cache manager doesn't support incremental indexing")
        return
    }
    ctx, cancel := common.CreateContext(2 * time.Minute)
    defer cancel()
    wd, err := os.Getwd()
    if err != nil {
        common.LSPLogger.Error("Failed to get working directory: %v", err)
        return
    }
    common.LSPLogger.Info("Performing incremental reindex for %d changed files", len(files))
    if err := cacheManager.PerformIncrementalIndexing(ctx, wd, m); err != nil {
        common.LSPLogger.Error("Incremental reindexing failed: %v", err)
    } else {
        common.LSPLogger.Info("Incremental reindex completed successfully")
        if stats := cacheManager.GetIndexStats(); stats != nil {
            common.LSPLogger.Debug("Cache stats after reindex: %d symbols, %d documents", stats.SymbolCount, stats.DocumentCount)
        }
    }
}
