package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"lsp-gateway/src/internal/common"
)

// FileChangeEvent represents a file change event
type FileChangeEvent struct {
	Path      string
	Operation string // "write", "create", "remove", "rename"
	Timestamp time.Time
}

// FileWatcher watches for file system changes and triggers callbacks
type FileWatcher struct {
	watcher       *fsnotify.Watcher
	watchPaths    []string
	extensions    []string
	onChange      func([]FileChangeEvent)
	debounceDelay time.Duration

	// Debouncing
	pendingEvents map[string]*FileChangeEvent
	eventMutex    sync.Mutex
	debounceTimer *time.Timer

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(extensions []string, onChange func([]FileChangeEvent)) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FileWatcher{
		watcher:       watcher,
		watchPaths:    []string{},
		extensions:    extensions,
		onChange:      onChange,
		debounceDelay: 500 * time.Millisecond, // Default 500ms debounce
		pendingEvents: make(map[string]*FileChangeEvent),
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}

	return fw, nil
}

// AddPath adds a path to watch (can be file or directory)
func (fw *FileWatcher) AddPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Add to watcher
	if err := fw.watcher.Add(absPath); err != nil {
		return err
	}

	fw.watchPaths = append(fw.watchPaths, absPath)
	common.LSPLogger.Debug("FileWatcher: Added watch path: %s", absPath)

	// If it's a directory, walk and add subdirectories
	if err := fw.addSubdirectories(absPath); err != nil {
		common.LSPLogger.Warn("Failed to add subdirectories for %s: %v", absPath, err)
	}

	return nil
}

// addSubdirectories recursively adds subdirectories to watch
func (fw *FileWatcher) addSubdirectories(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip common directories that shouldn't be watched
		base := filepath.Base(path)
		if base == ".git" || base == "node_modules" || base == "vendor" ||
			base == ".vscode" || base == ".idea" || strings.HasPrefix(base, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Add directories to watcher
		if info.IsDir() && path != root {
			if err := fw.watcher.Add(path); err != nil {
				common.LSPLogger.Warn("Failed to watch directory %s: %v", path, err)
			}
		}

		return nil
	})
}

// Start begins watching for file changes
func (fw *FileWatcher) Start() {
	go fw.watchLoop()
}

// watchLoop is the main event processing loop
func (fw *FileWatcher) watchLoop() {
	defer close(fw.done)

	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Filter by extension
			if !fw.shouldProcess(event.Name) {
				continue
			}

			// Handle the event with debouncing
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			common.LSPLogger.Error("FileWatcher error: %v", err)
		}
	}
}

// shouldProcess checks if a file should be processed based on extension
func (fw *FileWatcher) shouldProcess(path string) bool {
	// Check if it's a directory operation
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		// Handle new directories
		if err := fw.addSubdirectories(path); err != nil {
			common.LSPLogger.Warn("Failed to add new directory %s: %v", path, err)
		}
		return false
	}

	// Check file extension
	ext := filepath.Ext(path)
	for _, validExt := range fw.extensions {
		if ext == validExt {
			return true
		}
	}
	return false
}

// handleEvent processes a file system event with debouncing
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	fw.eventMutex.Lock()
	defer fw.eventMutex.Unlock()

	// Determine operation type
	var operation string
	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		operation = "write"
	case event.Op&fsnotify.Create == fsnotify.Create:
		operation = "create"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		operation = "remove"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		operation = "rename"
	default:
		return // Ignore other operations
	}

	// Store/update pending event
	fw.pendingEvents[event.Name] = &FileChangeEvent{
		Path:      event.Name,
		Operation: operation,
		Timestamp: time.Now(),
	}

	// Reset debounce timer
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}

	fw.debounceTimer = time.AfterFunc(fw.debounceDelay, fw.flushEvents)
}

// flushEvents sends all pending events to the callback
func (fw *FileWatcher) flushEvents() {
	fw.eventMutex.Lock()
	defer fw.eventMutex.Unlock()

	if len(fw.pendingEvents) == 0 {
		return
	}

	// Collect events
	events := make([]FileChangeEvent, 0, len(fw.pendingEvents))
	for _, event := range fw.pendingEvents {
		events = append(events, *event)
	}

	// Clear pending events
	fw.pendingEvents = make(map[string]*FileChangeEvent)

	// Call the callback
	if fw.onChange != nil {
		common.LSPLogger.Debug("FileWatcher: Flushing %d file change events", len(events))
		go fw.onChange(events)
	}
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() error {
	fw.cancel()

	// Flush any remaining events
	fw.flushEvents()

	// Close the watcher
	err := fw.watcher.Close()

	// Wait for the watch loop to finish
	<-fw.done

	return err
}

// SetDebounceDelay sets the debounce delay for file events
func (fw *FileWatcher) SetDebounceDelay(delay time.Duration) {
	fw.debounceDelay = delay
}
