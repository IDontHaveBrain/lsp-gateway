package server

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	errorspkg "lsp-gateway/src/internal/errors"
	"lsp-gateway/src/internal/errors/lsp"
	"lsp-gateway/src/internal/platform"
	"lsp-gateway/src/internal/project"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/aggregators"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/documents"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/server/watcher"
)

type LSPManager struct {
	clients             map[string]types.LSPClient
	clientErrors        map[string]error
	config              *config.Config
	ctx                 context.Context
	cancel              context.CancelFunc
	mu                  sync.RWMutex
	documentManager     *documents.DocumentManager
	workspaceAggregator aggregators.WorkspaceSymbolAggregator
	cacheIntegrator     *cache.CacheIntegrator
	scipCache           cache.SCIPCache
	projectInfo         *project.PackageInfo
	fileWatcher         *watcher.FileWatcher
	watcherMu           sync.Mutex
	indexLimiter        chan struct{}
}

func NewLSPManager(cfg *config.Config) (*LSPManager, error) {
	if cfg == nil {
		cfg = config.GetDefaultConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cacheIntegrator := cache.NewCacheIntegrator(cfg, common.LSPLogger)
	manager := &LSPManager{
		clients:             make(map[string]types.LSPClient),
		clientErrors:        make(map[string]error),
		config:              cfg,
		ctx:                 ctx,
		cancel:              cancel,
		documentManager:     documents.NewDocumentManager(),
		workspaceAggregator: aggregators.NewWorkspaceSymbolAggregator(),
		cacheIntegrator:     cacheIntegrator,
		scipCache:           cacheIntegrator.GetCache(),
		projectInfo:         nil,
	}
	limiterSize := 2
	if platform.IsWindows() {
		limiterSize = 1
	}
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

func (m *LSPManager) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if m.scipCache != nil && m.isCacheableMethod(method) {
		if result, found, err := m.scipCache.Lookup(method, params); err == nil && found {
			return result, nil
		}
	}
	uri, err := m.documentManager.ExtractURI(params)
	if err != nil {
		if method == types.MethodWorkspaceSymbol {
			m.mu.RLock()
			clients := make(map[string]interface{})
			for k, v := range m.clients {
				clients[k] = v
			}
			m.mu.RUnlock()
			result, err := m.workspaceAggregator.ProcessWorkspaceSymbol(ctx, clients, params)
			if err == nil && m.scipCache != nil {
				if m.isCacheableMethod(method) {
					_ = m.scipCache.Store(method, params, result)
				}
			}
			return result, err
		}
		return nil, fmt.Errorf("failed to extract URI from params: %w", err)
	}

	if method == types.MethodWorkspaceSymbol && uri == "" {
		m.mu.RLock()
		clients := make(map[string]interface{})
		for k, v := range m.clients {
			clients[k] = v
		}
		m.mu.RUnlock()
		result, err := m.workspaceAggregator.ProcessWorkspaceSymbol(ctx, clients, params)
		if err == nil && m.scipCache != nil {
			if m.isCacheableMethod(method) {
				_ = m.scipCache.Store(method, params, result)
			}
		}
		return result, err
	}

	language := m.documentManager.DetectLanguage(uri)
	if language == "" {
		return nil, fmt.Errorf("unsupported file type: %s", uri)
	}
	client, err := m.getClient(language)
	if err != nil {
		return nil, fmt.Errorf("no LSP client for language %s: %w", language, err)
	}
	if !client.Supports(method) {
		return nil, errorspkg.NewMethodNotSupportedError(language, method, lsp.GetMethodSuggestion(language, method))
	}
	if method != types.MethodWorkspaceSymbol {
		m.ensureDocumentOpen(client, uri, language, params)
	}
	result, err := m.sendRequestWithRetry(ctx, client, method, params, uri, language)
	if err == nil {
		if method == types.MethodTextDocumentReferences {
			if len(result) == 0 || string(result) == "null" {
				result = json.RawMessage("[]")
			}
		}
	}
	if err == nil && m.scipCache != nil && m.isCacheableMethod(method) {
		_ = m.scipCache.Store(method, params, result)
		if method == types.MethodTextDocumentDocumentSymbol {
			idxCtx, cancel := common.CreateContext(12 * time.Second)
			defer cancel()
			m.performSCIPIndexing(idxCtx, method, uri, language, params, result)
		} else {
			m.scheduleIndexing(method, uri, language, params, result)
		}
	}
	return result, err
}

func (m *LSPManager) sendRequestWithRetry(ctx context.Context, client types.LSPClient, method string, params interface{}, uri string, language string) (json.RawMessage, error) {
	maxRetries := 3
	baseDelay := 200 * time.Millisecond
	if platform.IsWindows() {
		baseDelay = 500 * time.Millisecond
		maxRetries = 4
	}
	var lastRes json.RawMessage
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		res, err := client.SendRequest(ctx, method, params)
		lastRes, lastErr = res, err
		if err != nil {
			return res, err
		}
		if isContentModifiedRPCError(res) {
			if uri == "" || attempt == maxRetries-1 {
				return res, nil
			}
			delay := time.Duration(attempt+1) * baseDelay
			time.Sleep(delay)
			m.ensureDocumentOpen(client, uri, language, params)
			continue
		}
		if !isNoViewsRPCError(res) {
			return res, nil
		}
		if uri == "" || attempt == maxRetries-1 {
			return res, nil
		}
		if attempt == 0 {
			common.LSPLogger.Debug("Encountered 'no views' error for %s, retrying...", uri)
		}
		m.ensureDocumentOpen(client, uri, language, params)
		delay := time.Duration(attempt+1) * baseDelay
		var b [1]byte
		_, _ = cryptoRand.Read(b[:])
		jitter := time.Duration(int(b[0])%100) * time.Millisecond
		time.Sleep(delay + jitter)
	}
	return lastRes, lastErr
}

func isNoViewsRPCError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return false
	}
	if e.Message == "" {
		return false
	}
	return strings.Contains(strings.ToLower(e.Message), "no views")
}

func isContentModifiedRPCError(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return false
	}
	if e.Message == "" {
		return false
	}
	if e.Code == -32801 {
		return true
	}
	return strings.Contains(strings.ToLower(e.Message), "content modified")
}

func (m *LSPManager) isCacheableMethod(method string) bool {
	cacheableMethods := map[string]bool{
		types.MethodTextDocumentDefinition:     true,
		types.MethodTextDocumentReferences:     true,
		types.MethodTextDocumentHover:          true,
		types.MethodTextDocumentDocumentSymbol: true,
		types.MethodWorkspaceSymbol:            true,
		types.MethodTextDocumentCompletion:     true,
	}

	return cacheableMethods[method]
}

func (m *LSPManager) InvalidateCache(uri string) error {
	if m.scipCache == nil {
		return nil
	}
	return m.scipCache.InvalidateDocument(uri)
}

func (m *LSPManager) GetCacheMetrics() interface{} {
	if m.scipCache == nil {
		return nil
	}

	cacheMetrics := m.scipCache.GetMetrics()
	indexStats := m.scipCache.GetIndexStats()

	return map[string]interface{}{
		"cache":      cacheMetrics,
		"scip_index": indexStats,
		"integrated": true,
	}
}

func (m *LSPManager) GetCache() cache.SCIPCache {
	return m.scipCache
}

func (m *LSPManager) SetCache(cache cache.SCIPCache) {
	m.scipCache = cache
	if cache != nil {
		common.LSPLogger.Debug("Cache injected into LSP manager")
	} else {
		common.LSPLogger.Debug("Cache removed from LSP manager")
	}
}

func (m *LSPManager) ensureDocumentOpen(client types.LSPClient, uri string, language string, params interface{}) {
	if m.documentManager.IsOpen(uri, language) {
		return
	}

	err := m.documentManager.EnsureOpen(client, uri, params)
	if err != nil {
		common.LSPLogger.Error("Failed to ensure document open for %s: %v", uri, err)
		return
	}

	m.documentManager.MarkOpen(uri, language, "", 0)
}

func (m *LSPManager) getScipStorage() scip.SCIPDocumentStorage {
	if m == nil || m.scipCache == nil {
		return nil
	}
	return m.scipCache.GetSCIPStorage()
}

func (m *LSPManager) GetConfiguredServers() map[string]*config.ServerConfig { return m.config.Servers }
