package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/platform"
	"lsp-gateway/src/internal/project"
	"lsp-gateway/src/internal/security"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/aggregators/base"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/client"
)

const (
	langJava             = "java"
	langKotlin           = "kotlin"
	langRust             = "rust"
	langPython           = "python"
	serverPyrightLS      = "pyright-langserver"
	serverBasedPyrightLS = "basedpyright-langserver"
	serverJediLS         = "jedi-language-server"
	pmUVX                = "uvx"
	namePyright          = "pyright"
	nameBasedPyright     = "basedpyright"
)

type ClientStatus struct {
	Active    bool
	Error     error
	Available bool
}

type serverConfigWrapper struct {
	language string
	config   *config.ServerConfig
}

func (w *serverConfigWrapper) Start(ctx context.Context) error { return nil }
func (w *serverConfigWrapper) Stop() error                     { return nil }
func (w *serverConfigWrapper) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return nil, nil
}
func (w *serverConfigWrapper) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}
func (w *serverConfigWrapper) Supports(method string) bool { return false }
func (w *serverConfigWrapper) IsActive() bool              { return false }

func (m *LSPManager) Start(ctx context.Context) error {
	common.LSPLogger.Debug("[LSPManager.Start] Starting LSP manager, scipCache=%v", m.cacheIntegrator.IsEnabled())

	if err := m.cacheIntegrator.StartCache(ctx); err != nil {
		return fmt.Errorf("unexpected cache start error: %w", err)
	}
	m.scipCache = m.cacheIntegrator.GetCache()

	languagesToStart := make([]string, 0)
	serversToStart := make(map[string]*config.ServerConfig)

	if wd, err := os.Getwd(); err == nil {
		for lang, cfg := range m.config.Servers {
			if cfg != nil && cfg.WorkingDir != "" {
				serversToStart[lang] = cfg
				languagesToStart = append(languagesToStart, lang)
			}
		}
		detected, derr := project.DetectLanguages(wd)
		if derr == nil && len(detected) > 0 {
			detectedSet := make(map[string]bool, len(detected))
			for _, lang := range detected {
				detectedSet[lang] = true
			}
			for lang, cfg := range m.config.Servers {
				if detectedSet[lang] {
					serversToStart[lang] = cfg
					languagesToStart = append(languagesToStart, lang)
				}
			}
			common.LSPLogger.Info("Detected workspace languages: %v", detected)
			common.LSPLogger.Info("Starting LSP servers for detected and pinned languages: %v", languagesToStart)
		} else {
			if derr != nil {
				common.LSPLogger.Warn("Language detection failed: %v", derr)
			}
			if len(serversToStart) == 0 {
				common.LSPLogger.Warn("No languages detected in workspace and no pinned servers; skipping LSP server startup")
			} else {
				common.LSPLogger.Info("Starting servers explicitly pinned to working directories: %v", languagesToStart)
			}
		}
	} else {
		common.LSPLogger.Warn("Failed to get working directory; skipping LSP server startup")
	}

	if len(serversToStart) == 0 {
		if m.config.Cache != nil && m.config.Cache.BackgroundIndex {
			if err := m.startFileWatcher(); err != nil {
				common.LSPLogger.Warn("Failed to start file watcher: %v", err)
			}
		}
		return nil
	}

	timeoutMgr := base.NewTimeoutManager().ForOperation(base.OperationInitialize)
	overallTimeout := timeoutMgr.GetOverallTimeout(languagesToStart)
	common.LSPLogger.Debug("[LSPManager.Start] Using overall collection timeout of %v for %d servers", overallTimeout, len(serversToStart))

	aggregator := base.NewParallelAggregator[*config.ServerConfig, error](0, overallTimeout)

	serverConfigs := make(map[string]types.LSPClient)
	for lang, cfg := range serversToStart {
		serverConfigs[lang] = &serverConfigWrapper{language: lang, config: cfg}
	}

	executor := func(ctx context.Context, client types.LSPClient, _ *config.ServerConfig) (error, error) {
		wrapper := client.(*serverConfigWrapper)
		err := m.startClientWithTimeout(ctx, wrapper.language, wrapper.config)
		return err, err
	}

	_, errors := aggregator.ExecuteWithLanguageTimeouts(ctx, serverConfigs, nil, executor, timeoutMgr.GetTimeout)

	completed := len(serversToStart) - len(errors)

	m.mu.Lock()
	for _, err := range errors {
		errStr := err.Error()
		if colonIndex := strings.Index(errStr, ": "); colonIndex > 0 {
			language := errStr[:colonIndex]
			actualErr := fmt.Errorf("%s", errStr[colonIndex+2:])
			m.clientErrors[language] = actualErr
			common.LSPLogger.Error("Failed to start %s client: %v", language, actualErr)
		}
	}
	m.mu.Unlock()

	if len(serversToStart) > 0 && completed == 0 {
		return fmt.Errorf("no LSP clients started: %d/%d failed", len(errors), len(serversToStart))
	}

	if len(errors) > 0 {
		for _, err := range errors {
			errStr := err.Error()
			if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "Overall timeout reached") {
				common.LSPLogger.Warn("Timeout reached, %d/%d clients started", completed, len(serversToStart))
				return nil
			}
		}
	}

	select {
	case <-ctx.Done():
		common.LSPLogger.Warn("Context cancelled, %d/%d clients started", completed, len(serversToStart))
		return nil
	default:
	}

	if m.scipCache != nil && m.config.Cache != nil && m.config.Cache.BackgroundIndex {
		if cacheManager, ok := m.scipCache.(*cache.SCIPCacheManager); ok {
			stats := cacheManager.GetIndexStats()
			if stats != nil && (stats.SymbolCount > 0 || stats.ReferenceCount > 0 || stats.DocumentCount > 0) {
				common.LSPLogger.Debug("LSP Manager: Cache already populated with %d symbols, %d references, %d documents - skipping background indexing",
					stats.SymbolCount, stats.ReferenceCount, stats.DocumentCount)
			} else {
				go func() {
					time.Sleep(constants.GetBackgroundIndexingDelay())
					recheckStats := cacheManager.GetIndexStats()
					if recheckStats != nil && (recheckStats.SymbolCount > 0 || recheckStats.ReferenceCount > 0 || recheckStats.DocumentCount > 0) {
						common.LSPLogger.Debug("LSP Manager: Cache was populated while waiting - skipping background indexing")
						return
					}
					wd, err := os.Getwd()
					if err != nil {
						common.LSPLogger.Warn("Failed to get working directory for indexing: %v", err)
						return
					}
					common.LSPLogger.Debug("LSP Manager: Performing background workspace indexing")
					indexCtx, cancel := common.CreateContext(5 * time.Minute)
					defer cancel()
					if err := cacheManager.PerformWorkspaceIndexing(indexCtx, wd, m); err != nil {
						common.LSPLogger.Warn("Failed to perform workspace indexing: %v", err)
					}
				}()
			}
		}
	}

	if m.config.Cache != nil && m.config.Cache.BackgroundIndex {
		if err := m.startFileWatcher(); err != nil {
			common.LSPLogger.Warn("Failed to start file watcher: %v", err)
		}
	}

	return nil
}

func (m *LSPManager) Stop() error {
	m.cancel()

	if m.fileWatcher != nil {
		if err := m.fileWatcher.Stop(); err != nil {
			common.LSPLogger.Warn("Failed to stop file watcher: %v", err)
		}
	}

	if err := m.cacheIntegrator.StopCache(); err != nil {
		common.LSPLogger.Warn("Failed to stop SCIP cache: %v", err)
	}

	m.mu.Lock()
	clients := make(map[string]types.LSPClient)
	for k, v := range m.clients {
		clients[k] = v
	}
	m.clients = make(map[string]types.LSPClient)
	m.mu.Unlock()

	if len(clients) == 0 {
		return nil
	}

	individualTimeout := constants.ProcessShutdownTimeout * 3
	overallTimeout := individualTimeout + 5*time.Second

	aggregator := base.NewParallelAggregator[struct{}, error](individualTimeout, overallTimeout)
	ctx := context.Background()
	results, errors := aggregator.Execute(ctx, clients, struct{}{}, func(ctx context.Context, client types.LSPClient, _ struct{}) (error, error) {
		err := client.Stop()
		return err, err
	})

	_ = results
	if len(errors) > 0 {
		common.LSPLogger.Warn("One or more clients did not stop cleanly: %d errors", len(errors))
		return errors[0]
	}
	return nil
}

func (m *LSPManager) GetClientStatus() map[string]ClientStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]ClientStatus)
	for language := range m.config.Servers {
		if client, exists := m.clients[language]; exists {
			if activeClient, ok := client.(interface{ IsActive() bool }); ok {
				status[language] = ClientStatus{Active: activeClient.IsActive(), Available: true, Error: nil}
			} else {
				status[language] = ClientStatus{Active: true, Available: true, Error: nil}
			}
		} else {
			if err, hasError := m.clientErrors[language]; hasError {
				status[language] = ClientStatus{Active: false, Available: false, Error: err}
			} else {
				status[language] = ClientStatus{Active: false, Available: false, Error: fmt.Errorf("client not started")}
			}
		}
	}
	return status
}

func (m *LSPManager) CheckServerAvailability() map[string]ClientStatus {
	result := make(map[string]ClientStatus)
	for language, cfg := range m.config.Servers {
		resolved := m.resolveCommandPath(language, cfg.Command)
		if err := security.ValidateCommand(resolved, cfg.Args); err != nil {
			result[language] = ClientStatus{Active: false, Available: false, Error: fmt.Errorf("invalid command: %w", err)}
			continue
		}
		if _, err := exec.LookPath(resolved); err != nil {
			result[language] = ClientStatus{Active: false, Available: false, Error: fmt.Errorf("command not found: %s", resolved)}
			continue
		}
		result[language] = ClientStatus{Active: false, Available: true, Error: nil}
	}
	m.mu.RLock()
	for lang, client := range m.clients {
		if _, ok := result[lang]; !ok {
			continue
		}
		if activeClient, ok2 := client.(interface{ IsActive() bool }); ok2 {
			cs := result[lang]
			cs.Active = activeClient.IsActive()
			result[lang] = cs
		}
	}
	m.mu.RUnlock()
	return result
}

func (m *LSPManager) getClientActiveWaitIterations(language string) int {
	maxWaitTime := constants.GetInitializeTimeout(language)
	maxIterations := int(maxWaitTime.Seconds() * 10)
	if maxIterations < 30 {
		maxIterations = 30
	}
	return maxIterations
}

func (m *LSPManager) startClientWithTimeout(ctx context.Context, language string, cfg *config.ServerConfig) error {
	if language == langPython {
		candidates := m.buildPythonCommandCandidates(cfg)
		var errs []error
		for _, cand := range candidates {
			if err := m.tryStartCandidate(ctx, language, cfg, cand); err == nil {
				if cand.command != "" && cfg != nil && cand.command != cfg.Command {
					common.LSPLogger.Info("Started %s client using fallback command %s %v", language, cand.command, cand.args)
				}
				return nil
			} else {
				errs = append(errs, fmt.Errorf("%s %v: %w", cand.command, cand.args, err))
				common.LSPLogger.Warn("Python LSP candidate '%s %v' failed: %v", cand.command, cand.args, err)
			}
		}
		if len(errs) == 0 {
			return fmt.Errorf("%s: no Python LSP candidates available", language)
		}
		return fmt.Errorf("%s: %w", language, errors.Join(errs...))
	}

	resolvedCommand := m.resolveCommandPath(language, cfg.Command)
	argsToUse := cfg.Args

	if language == langKotlin {
		base := filepath.Base(resolvedCommand)
		isJetBrains := strings.Contains(base, "kotlin-lsp")
		isFWCD := strings.Contains(base, "kotlin-language-server")
		if _, err := exec.LookPath(resolvedCommand); err != nil {
			if alt := m.resolveCommandPath(language, "kotlin-lsp"); alt != "" {
				if _, e := exec.LookPath(alt); e == nil {
					resolvedCommand = alt
					isJetBrains = true
					isFWCD = false
				}
			}
			if !isJetBrains {
				if alt := m.resolveCommandPath(language, "kotlin-language-server"); alt != "" {
					if _, e := exec.LookPath(alt); e == nil {
						resolvedCommand = alt
						isJetBrains = false
						isFWCD = true
					}
				}
			}
			if !isJetBrains && !isFWCD {
				if p, e := exec.LookPath("kotlin-lsp"); e == nil {
					resolvedCommand = p
					isJetBrains = true
				} else if p2, e2 := exec.LookPath("kotlin-language-server"); e2 == nil {
					resolvedCommand = p2
					isFWCD = true
				}
			}
		}
		if platform.IsWindows() || isFWCD {
		} else if isJetBrains || !platform.IsWindows() {
			addr := "127.0.0.1:9999"
			if ln, e := net.Listen("tcp", addr); e == nil {
				_ = ln.Close()
			} else {
				if ln2, e2 := net.Listen("tcp", "127.0.0.1:0"); e2 == nil {
					if tcp, ok := ln2.Addr().(*net.TCPAddr); ok {
						addr = fmt.Sprintf("127.0.0.1:%d", tcp.Port)
					}
					_ = ln2.Close()
				}
			}
			argsToUse = []string{"--socket", addr}
			common.LSPLogger.Info("Launching JetBrains %s LSP in socket mode at %s", language, addr)
			clientConfig := types.ClientConfig{Command: resolvedCommand, Args: argsToUse, WorkingDir: cfg.WorkingDir, InitializationOptions: cfg.InitializationOptions}
			if err := security.ValidateCommand(resolvedCommand, argsToUse); err != nil {
				return fmt.Errorf("%s: invalid LSP server command: %w", language, err)
			}
			socketClient, err := client.NewSocketClient(clientConfig, language, addr, m.documentManager)
			if err == nil {
				if errStart := socketClient.Start(ctx); errStart == nil {
					m.mu.Lock()
					m.clients[language] = socketClient
					m.mu.Unlock()
					if activeClient, ok := socketClient.(interface{ IsActive() bool }); ok {
						maxWaitIterations := m.getClientActiveWaitIterations(language)
						for i := 0; i < maxWaitIterations; i++ {
							if activeClient.IsActive() {
								break
							}
							time.Sleep(100 * time.Millisecond)
						}
					}
					return nil
				} else {
					common.LSPLogger.Warn("Kotlin socket mode failed to start: %v; falling back to stdio", errStart)
					_ = socketClient.Stop()
				}
			} else {
				common.LSPLogger.Warn("Failed to create Kotlin socket client: %v; falling back to stdio", err)
			}

			common.LSPLogger.Info("Attempting Kotlin LSP stdio mode as fallback")
			stdioArgs := []string{"--stdio"}
			stdioCfg := types.ClientConfig{Command: resolvedCommand, Args: stdioArgs, WorkingDir: cfg.WorkingDir, InitializationOptions: cfg.InitializationOptions}
			if err := security.ValidateCommand(resolvedCommand, stdioArgs); err == nil {
				if stdClient, e2 := client.NewStdioClient(stdioCfg, language, m.documentManager); e2 == nil {
					if startErr := stdClient.Start(ctx); startErr == nil {
						m.mu.Lock()
						m.clients[language] = stdClient
						m.mu.Unlock()
						return nil
					} else {
						common.LSPLogger.Warn("Kotlin stdio mode (JetBrains) failed to start: %v", startErr)
					}
				} else {
					common.LSPLogger.Warn("Failed to create Kotlin stdio client (JetBrains): %v", e2)
				}
			}

			if p2, e2 := exec.LookPath("kotlin-language-server"); e2 == nil {
				altCfg := types.ClientConfig{Command: p2, Args: []string{}, WorkingDir: cfg.WorkingDir, InitializationOptions: cfg.InitializationOptions}
				if err := security.ValidateCommand(p2, nil); err == nil {
					if fwcdClient, e3 := client.NewStdioClient(altCfg, language, m.documentManager); e3 == nil {
						if startErr := fwcdClient.Start(ctx); startErr == nil {
							common.LSPLogger.Info("Using fwcd kotlin-language-server via stdio as fallback")
							m.mu.Lock()
							m.clients[language] = fwcdClient
							m.mu.Unlock()
							return nil
						} else {
							common.LSPLogger.Error("Failed to start fwcd kotlin-language-server: %v", startErr)
						}
					} else {
						common.LSPLogger.Error("Failed to create fwcd kotlin-language-server client: %v", e3)
					}
				}
			}

			return fmt.Errorf("failed to start Kotlin LSP in socket or stdio mode")
		}
	}

	if err := m.tryStartCandidate(ctx, language, cfg, commandCandidate{command: resolvedCommand, args: argsToUse, resolved: true}); err != nil {
		return fmt.Errorf("%s: %w", language, err)
	}
	return nil
}

type commandCandidate struct {
	command  string
	args     []string
	resolved bool
}

func (m *LSPManager) buildPythonCommandCandidates(cfg *config.ServerConfig) []commandCandidate {
	seen := make(map[string]bool)
	var candidates []commandCandidate

	add := func(cmd string, args []string) {
		if cmd == "" {
			return
		}
		key := cmd + "|" + strings.Join(args, "\x00")
		if seen[key] {
			return
		}
		seen[key] = true
		copiedArgs := append([]string(nil), args...)
		candidates = append(candidates, commandCandidate{command: cmd, args: copiedArgs})
	}

	if cfg != nil && cfg.Command != "" {
		add(cfg.Command, cfg.Args)
	}

	add(serverBasedPyrightLS, []string{"--stdio"})
	add(serverPyrightLS, []string{"--stdio"})
	add("pylsp", nil)
	add(serverJediLS, []string{"--stdio"})
	add("uvx", []string{"--from", "basedpyright", serverBasedPyrightLS, "--", "--stdio"})
	add("uvx", []string{"--from", "pyright", serverPyrightLS, "--", "--stdio"})

	return candidates
}

func (m *LSPManager) tryStartCandidate(ctx context.Context, language string, cfg *config.ServerConfig, cand commandCandidate) error {
	commandName := cand.command
	if commandName == "" {
		return fmt.Errorf("empty command")
	}

	resolvedCommand := commandName
	if !cand.resolved {
		resolvedCommand = m.resolveCommandPath(language, commandName)
	}

	argsToUse := cand.args
	if argsToUse == nil && cfg != nil {
		argsToUse = cfg.Args
	}

	if err := security.ValidateCommand(resolvedCommand, argsToUse); err != nil {
		return fmt.Errorf("invalid LSP server command for %s: %w", language, err)
	}
	if _, err := exec.LookPath(resolvedCommand); err != nil {
		return fmt.Errorf("LSP server command not found: %s", resolvedCommand)
	}

	clientConfig := types.ClientConfig{
		Command:               resolvedCommand,
		Args:                  append([]string(nil), argsToUse...),
		WorkingDir:            cfg.WorkingDir,
		InitializationOptions: cfg.InitializationOptions,
	}

	lspClient, err := client.NewStdioClient(clientConfig, language, m.documentManager)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := lspClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start client: %w", err)
	}

	if activeClient, ok := lspClient.(interface{ IsActive() bool }); ok {
		maxWaitIterations := m.getClientActiveWaitIterations(language)
		for i := 0; i < maxWaitIterations; i++ {
			select {
			case <-ctx.Done():
				_ = lspClient.Stop()
				return fmt.Errorf("context cancelled while waiting for client to become active")
			default:
				if activeClient.IsActive() {
					m.mu.Lock()
					m.clients[language] = lspClient
					m.mu.Unlock()
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		_ = lspClient.Stop()
		return fmt.Errorf("client did not become active within timeout")
	}

	m.mu.Lock()
	m.clients[language] = lspClient
	m.mu.Unlock()
	return nil
}

func (m *LSPManager) getClient(language string) (types.LSPClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, exists := m.clients[language]
	if !exists {
		return nil, fmt.Errorf("no client for language: %s", language)
	}
	return client, nil
}

func (m *LSPManager) GetClient(language string) (types.LSPClient, error) {
	return m.getClient(language)
}

func (m *LSPManager) detectPrimaryLanguage(workingDir string) string {
	projectMarkers := []struct {
		files    []string
		language string
	}{
		{[]string{"go.mod", "go.work"}, "go"},
		{[]string{"package.json", "tsconfig.json"}, "typescript"},
		{[]string{"package.json"}, "javascript"},
		{[]string{"pyproject.toml", "setup.py", "requirements.txt"}, "python"},
		{[]string{"pom.xml"}, "java"},
		{[]string{"build.gradle.kts"}, "kotlin"},
		{[]string{"build.gradle"}, "java"},
		{[]string{"Cargo.toml", "Cargo.lock"}, "rust"},
	}
	for _, marker := range projectMarkers {
		for _, file := range marker.files {
			full := filepath.Join(workingDir, file)
			if common.FileExists(full) {
				if file == "build.gradle" {
					if data, err := os.ReadFile(full); err == nil {
						lc := strings.ToLower(string(data))
						if strings.Contains(lc, "org.jetbrains.kotlin") || strings.Contains(lc, "apply plugin: \"kotlin") || strings.Contains(lc, "apply plugin: 'kotlin") || strings.Contains(lc, "kotlin-stdlib") {
							return "kotlin"
						}
					}
				}
				return marker.language
			}
		}
	}
	if files, err := os.ReadDir(workingDir); err == nil {
		langCounts := make(map[string]int)
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			ext := filepath.Ext(file.Name())
			switch ext {
			case ".go":
				langCounts["go"]++
			case ".py":
				langCounts["python"]++
			case ".js", ".mjs":
				langCounts["javascript"]++
			case ".ts":
				langCounts["typescript"]++
			case ".java":
				langCounts["java"]++
			case ".rs":
				langCounts["rust"]++
			}
		}
		maxCount := 0
		var primaryLang string
		for lang, count := range langCounts {
			if count > maxCount {
				maxCount = count
				primaryLang = lang
			}
		}
		if primaryLang != "" {
			return primaryLang
		}
	}
	return "unknown"
}
