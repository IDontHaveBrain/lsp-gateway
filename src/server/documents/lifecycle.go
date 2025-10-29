package documents

import (
	"sync"
	"time"
)

type DocumentState struct {
	URI         string
	Language    string
	Version     int
	OpenedBy    map[string]bool
	Content     string
	LastAccess  time.Time
}

func (ds *DocumentState) IsOpenForLanguage(language string) bool {
	return ds.OpenedBy[language]
}

func (ds *DocumentState) OpenForLanguage(language string) {
	if ds.OpenedBy == nil {
		ds.OpenedBy = make(map[string]bool)
	}
	ds.OpenedBy[language] = true
	ds.LastAccess = time.Now()
}

func (ds *DocumentState) CloseForLanguage(language string) {
	delete(ds.OpenedBy, language)
	ds.LastAccess = time.Now()
}

func (ds *DocumentState) IsClosed() bool {
	return len(ds.OpenedBy) == 0
}

type DocumentLifecycleManager struct {
	mu        sync.RWMutex
	documents map[string]*DocumentState
}

func NewDocumentLifecycleManager() *DocumentLifecycleManager {
	return &DocumentLifecycleManager{
		documents: make(map[string]*DocumentState),
	}
}

func (m *DocumentLifecycleManager) IsOpen(uri string, language string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, exists := m.documents[uri]
	if !exists {
		return false
	}
	return doc.IsOpenForLanguage(language)
}

func (m *DocumentLifecycleManager) MarkOpen(uri string, language string, content string, version int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[uri]
	if !exists {
		doc = &DocumentState{
			URI:      uri,
			Language: language,
			Version:  version,
			Content:  content,
			OpenedBy: make(map[string]bool),
		}
		m.documents[uri] = doc
	}
	doc.OpenForLanguage(language)
}

func (m *DocumentLifecycleManager) MarkClosed(uri string, language string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[uri]
	if exists {
		doc.CloseForLanguage(language)
		if doc.IsClosed() {
			delete(m.documents, uri)
		}
	}
}

func (m *DocumentLifecycleManager) UpdateContent(uri string, content string, version int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, exists := m.documents[uri]
	if exists {
		doc.Content = content
		doc.Version = version
		doc.LastAccess = time.Now()
	}
}

func (m *DocumentLifecycleManager) GetDocument(uri string) (*DocumentState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, exists := m.documents[uri]
	return doc, exists
}

func (m *DocumentLifecycleManager) GetAllDocuments() []*DocumentState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	docs := make([]*DocumentState, 0, len(m.documents))
	for _, doc := range m.documents {
		docs = append(docs, doc)
	}
	return docs
}

func (m *DocumentLifecycleManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.documents = make(map[string]*DocumentState)
}
