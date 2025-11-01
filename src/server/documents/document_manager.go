package documents

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/errors"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/jsonutil"
)

type DocumentState struct {
	URI      string
	Language string
	OpenedBy map[string]bool
}

func (ds *DocumentState) IsOpenForLanguage(language string) bool {
	return ds.OpenedBy[language]
}

func (ds *DocumentState) OpenForLanguage(language string) {
	if ds.OpenedBy == nil {
		ds.OpenedBy = make(map[string]bool)
	}
	ds.OpenedBy[language] = true
}

func (ds *DocumentState) CloseForLanguage(language string) {
	delete(ds.OpenedBy, language)
}

func (ds *DocumentState) IsClosed() bool {
	return len(ds.OpenedBy) == 0
}

type DocumentManager struct {
	documents map[string]*DocumentState
	mu        sync.RWMutex
}

func NewDocumentManager() *DocumentManager {
	return &DocumentManager{
		documents: make(map[string]*DocumentState),
	}
}

func (dm *DocumentManager) DetectLanguage(uri string) string {
	path := utils.URIToFilePathCached(uri)
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := registry.GetLanguageByExtension(ext); ok {
		return lang.Name
	}
	return ""
}

func (dm *DocumentManager) ExtractURI(params interface{}) (string, error) {
	if params == nil {
		return "", errors.NewValidationError("params", "no parameters provided")
	}

	var paramsMap map[string]interface{}

	if m, ok := params.(map[string]interface{}); ok {
		paramsMap = m
	} else {
		converted, err := jsonutil.Convert[map[string]interface{}](params)
		if err != nil {
			return "", errors.WrapWithContext("failed to convert params to map", err)
		}
		paramsMap = converted
	}

	if textDoc, ok := paramsMap["textDocument"].(map[string]interface{}); ok {
		if uri, ok := textDoc["uri"].(string); ok {
			return uri, nil
		}
	}

	if uri, ok := paramsMap["uri"].(string); ok {
		return uri, nil
	}

	if _, isWorkspaceSymbol := paramsMap["query"]; isWorkspaceSymbol {
		return "", nil
	}

	return "", errors.NewValidationError("parameter", "no URI found in parameters")
}

func (dm *DocumentManager) EnsureOpen(client types.LSPClient, uri string, params interface{}) error {
	var fileContent string
	language := dm.DetectLanguage(uri)

	if strings.HasPrefix(uri, "file://") {
		filePath := utils.URIToFilePathCached(uri)
		if data, err := common.SafeReadFile(filePath); err == nil {
			fileContent = string(data)
		} else {
			common.LSPLogger.Error("Failed to read file content for %s: %v", uri, err)
			fileContent = ""
		}
	} else {
		common.LSPLogger.Warn("URI does not start with file://: %s", uri)
	}

	didOpenParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": language,
			"version":    1,
			"text":       fileContent,
		},
	}

	err := client.SendNotification(context.Background(), types.MethodTextDocumentDidOpen, didOpenParams)
	if err != nil {
		common.LSPLogger.Error("Failed to send didOpen notification for %s: %v", uri, err)
		return errors.WrapWithContext("failed to send didOpen notification", err)
	}

	time.Sleep(constants.GetDocumentAnalysisDelay(language))

	return nil
}

func (dm *DocumentManager) IsOpen(uri string, language string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	doc, exists := dm.documents[uri]
	if !exists {
		return false
	}
	return doc.IsOpenForLanguage(language)
}

func (dm *DocumentManager) MarkOpen(uri string, language string, content string, version int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, exists := dm.documents[uri]
	if !exists {
		doc = &DocumentState{
			URI:      uri,
			Language: language,
			OpenedBy: make(map[string]bool),
		}
		dm.documents[uri] = doc
	}
	doc.OpenForLanguage(language)
}

func (dm *DocumentManager) MarkClosed(uri string, language string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	doc, exists := dm.documents[uri]
	if exists {
		doc.CloseForLanguage(language)
		if doc.IsClosed() {
			delete(dm.documents, uri)
		}
	}
}

func (dm *DocumentManager) Clear() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.documents = make(map[string]*DocumentState)
}
