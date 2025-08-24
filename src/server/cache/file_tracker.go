package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/utils"
)

// FileMetadata stores metadata about indexed files
type FileMetadata struct {
	URI          string    `json:"uri"`
	Path         string    `json:"path"`
	LastModified time.Time `json:"last_modified"`
	LastIndexed  time.Time `json:"last_indexed"`
	Size         int64     `json:"size"`
	Language     string    `json:"language"`
}

// FileChangeTracker tracks file modifications for incremental indexing
type FileChangeTracker struct {
	metadata map[string]*FileMetadata // key: file URI
	mu       sync.RWMutex
	dirty    bool // indicates if metadata has changed since last save
}

// NewFileChangeTracker creates a new file change tracker
func NewFileChangeTracker() *FileChangeTracker {
	return &FileChangeTracker{
		metadata: make(map[string]*FileMetadata),
	}
}

// GetFileMetadata retrieves metadata for a file
func (t *FileChangeTracker) GetFileMetadata(uri string) (*FileMetadata, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	meta, exists := t.metadata[uri]
	return meta, exists
}

// UpdateFileMetadata updates or adds metadata for a file
func (t *FileChangeTracker) UpdateFileMetadata(uri string, path string, modTime time.Time, size int64, language string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.metadata[uri] = &FileMetadata{
		URI:          uri,
		Path:         path,
		LastModified: modTime,
		LastIndexed:  time.Now(),
		Size:         size,
		Language:     language,
	}
	t.dirty = true
}

// IsFileChanged checks if a file has been modified since last indexing
func (t *FileChangeTracker) IsFileChanged(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // File doesn't exist, not changed
		}
		return false, err
	}

	// Convert to URI for lookup
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	uri := utils.FilePathToURI(absPath)

	t.mu.RLock()
	defer t.mu.RUnlock()

	meta, exists := t.metadata[uri]
	if !exists {
		// File not indexed before, consider it as new/changed
		return true, nil
	}

	// Check if modification time is newer than last indexed time
	return fileInfo.ModTime().After(meta.LastIndexed), nil
}

// GetChangedFiles scans a list of files and returns those that have changed
func (t *FileChangeTracker) GetChangedFiles(ctx context.Context, files []string) ([]string, []string, []string, error) {
	var newFiles, modifiedFiles, unchangedFiles []string

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		default:
		}

		absPath, err := filepath.Abs(file)
		if err != nil {
			common.LSPLogger.Warn("Failed to get absolute path for %s: %v", file, err)
			continue
		}

		fileInfo, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip non-existent files
			}
			common.LSPLogger.Warn("Failed to stat file %s: %v", file, err)
			continue
		}

		uri := utils.FilePathToURI(absPath)

		t.mu.RLock()
		meta, exists := t.metadata[uri]
		t.mu.RUnlock()

		if !exists {
			// New file
			newFiles = append(newFiles, file)
		} else if fileInfo.ModTime().After(meta.LastIndexed) || fileInfo.Size() != meta.Size {
			// Modified file (changed time or size)
			modifiedFiles = append(modifiedFiles, file)
		} else {
			// Unchanged file
			unchangedFiles = append(unchangedFiles, file)
		}
	}

	return newFiles, modifiedFiles, unchangedFiles, nil
}

// GetDeletedFiles returns files that were indexed but no longer exist
func (t *FileChangeTracker) GetDeletedFiles(currentFiles map[string]bool) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var deletedFiles []string
	for uri, meta := range t.metadata {
		if !currentFiles[meta.Path] {
			// Check if file still exists
			if _, err := os.Stat(meta.Path); os.IsNotExist(err) {
				deletedFiles = append(deletedFiles, uri)
			}
		}
	}
	return deletedFiles
}

// RemoveFileMetadata removes metadata for deleted files
func (t *FileChangeTracker) RemoveFileMetadata(uris []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, uri := range uris {
		delete(t.metadata, uri)
	}
	t.dirty = true
}

// SaveToFile persists metadata to a JSON file
func (t *FileChangeTracker) SaveToFile(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.dirty {
		return nil // No changes to save
	}

	data, err := json.MarshalIndent(t.metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file first for atomicity
	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Rename to final path
	if err := os.Rename(tempFile, path); err != nil {
		if rmErr := os.Remove(tempFile); rmErr != nil && !os.IsNotExist(rmErr) {
			common.LSPLogger.Warn("Failed to remove temp metadata file: %v", rmErr)
		}
		return fmt.Errorf("failed to rename metadata file: %w", err)
	}

	t.dirty = false
	return nil
}

// LoadFromFile loads metadata from a JSON file
func (t *FileChangeTracker) LoadFromFile(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start fresh
			t.metadata = make(map[string]*FileMetadata)
			return nil
		}
		return fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata map[string]*FileMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	t.metadata = metadata
	t.dirty = false
	return nil
}

// Clear removes all metadata
func (t *FileChangeTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.metadata = make(map[string]*FileMetadata)
	t.dirty = true
}

// GetIndexedFileCount returns the number of indexed files
func (t *FileChangeTracker) GetIndexedFileCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.metadata)
}

// GetAllMetadata returns a copy of all file metadata
func (t *FileChangeTracker) GetAllMetadata() map[string]*FileMetadata {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*FileMetadata, len(t.metadata))
	for k, v := range t.metadata {
		// Create a copy to avoid external modifications
		copy := *v
		result[k] = &copy
	}
	return result
}
