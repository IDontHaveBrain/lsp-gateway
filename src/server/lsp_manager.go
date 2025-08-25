package server

import (
    "context"
    "os"
    "path/filepath"
    "runtime"
    "sync"

    "lsp-gateway/src/config"
    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/internal/project"
    "lsp-gateway/src/internal/types"
    "lsp-gateway/src/server/aggregators"
    "lsp-gateway/src/server/cache"
    "lsp-gateway/src/server/documents"
    "lsp-gateway/src/server/watcher"
)

type LSPManager struct {
    clients             map[string]types.LSPClient
    clientErrors        map[string]error
    config              *config.Config
    ctx                 context.Context
    cancel              context.CancelFunc
    mu                  sync.RWMutex
    documentManager     documents.DocumentManager
    workspaceAggregator aggregators.WorkspaceSymbolAggregator
    cacheIntegrator     *cache.CacheIntegrator
    scipCache           cache.SCIPCache
    projectInfo         *project.PackageInfo
    fileWatcher         *watcher.FileWatcher
    watcherMu           sync.Mutex
    indexLimiter        chan struct{}
}

func NewLSPManager(cfg *config.Config) (*LSPManager, error) {
    if cfg == nil { cfg = config.GetDefaultConfig() }
    ctx, cancel := context.WithCancel(context.Background())
    cacheIntegrator := cache.NewCacheIntegrator(cfg, common.LSPLogger)
    manager := &LSPManager{
        clients:             make(map[string]types.LSPClient),
        clientErrors:        make(map[string]error),
        config:              cfg,
        ctx:                 ctx,
        cancel:              cancel,
        documentManager:     documents.NewLSPDocumentManager(),
        workspaceAggregator: aggregators.NewWorkspaceSymbolAggregator(),
        cacheIntegrator:     cacheIntegrator,
        scipCache:           cacheIntegrator.GetCache(),
        projectInfo:         nil,
    }
    limiterSize := 2
    if runtime.GOOS == "windows" { limiterSize = 1 }
    manager.indexLimiter = make(chan struct{}, limiterSize)
    if wd, err := os.Getwd(); err == nil {
        language := manager.detectPrimaryLanguage(wd)
        if projectInfo, err := project.GetPackageInfo(wd, language); err == nil {
            manager.projectInfo = projectInfo
        } else {
            manager.projectInfo = &project.PackageInfo{Name: filepath.Base(wd), Version: "0.0.0", Language: language}
        }
    }
    return manager, nil
}

