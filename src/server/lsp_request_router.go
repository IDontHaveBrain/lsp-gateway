package server

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	errorspkg "lsp-gateway/src/internal/errors"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/errors"
	"strings"
)

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
	language := m.documentManager.DetectLanguage(uri)
	if language == "" {
		return nil, fmt.Errorf("unsupported file type: %s", uri)
	}
	client, err := m.getClient(language)
	if err != nil {
		return nil, fmt.Errorf("no LSP client for language %s: %w", language, err)
	}
	if !client.Supports(method) {
		errorTranslator := errors.NewLSPErrorTranslator()
		return nil, errorspkg.NewMethodNotSupportedError(language, method, errorTranslator.GetMethodSuggestion(language, method))
	}
	if method != types.MethodWorkspaceSymbol {
		m.ensureDocumentOpen(client, uri, params)
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
            idxCtx, cancel := common.CreateContext(constants.AdjustDurationForWindows(12*time.Second, 1.5))
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
	if runtime.GOOS == osWindows {
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
			m.ensureDocumentOpen(client, uri, params)
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
		m.ensureDocumentOpen(client, uri, params)
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
