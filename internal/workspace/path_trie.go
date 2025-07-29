package workspace

import (
	"strings"
	"sync"
)

// PathTrieNode represents a node in the path trie for efficient project lookup
type PathTrieNode struct {
	children    map[string]*PathTrieNode
	subProject  *SubProject
	isEndOfPath bool
	mu          sync.RWMutex
}

// PathTrie implements efficient path-based project resolution using trie data structure
// Provides O(1) average case, O(log n) worst case performance for path lookups
type PathTrie struct {
	root *PathTrieNode
	mu   sync.RWMutex
}

// NewPathTrie creates a new PathTrie for efficient project path resolution
func NewPathTrie() *PathTrie {
	return &PathTrie{
		root: &PathTrieNode{
			children:    make(map[string]*PathTrieNode),
			subProject:  nil,
			isEndOfPath: false,
		},
	}
}

// Insert adds a project path to the trie for efficient lookup
// Uses path normalization and handles overlapping project boundaries
func (pt *PathTrie) Insert(projectPath string, subProject *SubProject) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Normalize path by removing trailing slashes and converting to absolute
	normalizedPath := normalizePath(projectPath)
	pathSegments := strings.Split(strings.Trim(normalizedPath, "/"), "/")
	
	if len(pathSegments) == 1 && pathSegments[0] == "" {
		// Root path case
		pathSegments = []string{}
	}

	current := pt.root
	
	// Traverse and create nodes as needed
	for _, segment := range pathSegments {
		if segment == "" {
			continue
		}
		
		current.mu.Lock()
		if current.children[segment] == nil {
			current.children[segment] = &PathTrieNode{
				children:    make(map[string]*PathTrieNode),
				subProject:  nil,
				isEndOfPath: false,
			}
		}
		next := current.children[segment]
		current.mu.Unlock()
		
		current = next
	}
	
	// Mark as end of path and store project
	current.mu.Lock()
	current.isEndOfPath = true
	current.subProject = subProject
	current.mu.Unlock()
}

// FindLongestMatch finds the longest path prefix match for efficient project resolution
// Returns the most specific (deepest) project that contains the given file path
func (pt *PathTrie) FindLongestMatch(filePath string) *SubProject {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	normalizedPath := normalizePath(filePath)
	pathSegments := strings.Split(strings.Trim(normalizedPath, "/"), "/")
	
	if len(pathSegments) == 1 && pathSegments[0] == "" {
		pathSegments = []string{}
	}

	var lastMatchedProject *SubProject
	current := pt.root
	
	// Check root level first
	current.mu.RLock()
	if current.isEndOfPath && current.subProject != nil {
		lastMatchedProject = current.subProject
	}
	current.mu.RUnlock()
	
	// Traverse path segments and find longest match
	for _, segment := range pathSegments {
		if segment == "" {
			continue
		}
		
		current.mu.RLock()
		next, exists := current.children[segment]
		current.mu.RUnlock()
		
		if !exists {
			break
		}
		
		current = next
		
		// Check if this node represents a project
		current.mu.RLock()
		if current.isEndOfPath && current.subProject != nil {
			lastMatchedProject = current.subProject
		}
		current.mu.RUnlock()
	}
	
	return lastMatchedProject
}

// Remove removes a project path from the trie
func (pt *PathTrie) Remove(projectPath string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	normalizedPath := normalizePath(projectPath)
	pathSegments := strings.Split(strings.Trim(normalizedPath, "/"), "/")
	
	if len(pathSegments) == 1 && pathSegments[0] == "" {
		pathSegments = []string{}
	}

	return pt.removeRecursive(pt.root, pathSegments, 0)
}

// removeRecursive recursively removes path segments and cleans up empty nodes
func (pt *PathTrie) removeRecursive(node *PathTrieNode, pathSegments []string, index int) bool {
	if index == len(pathSegments) {
		node.mu.Lock()
		defer node.mu.Unlock()
		
		if !node.isEndOfPath {
			return false
		}
		
		node.isEndOfPath = false
		node.subProject = nil
		
		// Return true if node can be deleted (no children and not end of path)
		return len(node.children) == 0
	}
	
	segment := pathSegments[index]
	if segment == "" {
		return pt.removeRecursive(node, pathSegments, index+1)
	}
	
	node.mu.RLock()
	child, exists := node.children[segment]
	node.mu.RUnlock()
	
	if !exists {
		return false
	}
	
	shouldDeleteChild := pt.removeRecursive(child, pathSegments, index+1)
	
	if shouldDeleteChild {
		node.mu.Lock()
		delete(node.children, segment)
		node.mu.Unlock()
	}
	
	// Check if current node can be deleted
	node.mu.RLock()
	canDelete := !node.isEndOfPath && len(node.children) == 0
	node.mu.RUnlock()
	
	return canDelete
}

// GetAllProjects returns all projects stored in the trie
func (pt *PathTrie) GetAllProjects() []*SubProject {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	
	var projects []*SubProject
	pt.collectProjects(pt.root, &projects)
	return projects
}

// collectProjects recursively collects all projects from the trie
func (pt *PathTrie) collectProjects(node *PathTrieNode, projects *[]*SubProject) {
	node.mu.RLock()
	
	if node.isEndOfPath && node.subProject != nil {
		*projects = append(*projects, node.subProject)
	}
	
	children := make(map[string]*PathTrieNode, len(node.children))
	for k, v := range node.children {
		children[k] = v
	}
	node.mu.RUnlock()
	
	for _, child := range children {
		pt.collectProjects(child, projects)
	}
}

// Clear removes all projects from the trie
func (pt *PathTrie) Clear() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	
	pt.root = &PathTrieNode{
		children:    make(map[string]*PathTrieNode),
		subProject:  nil,
		isEndOfPath: false,
	}
}

// Size returns the number of projects stored in the trie
func (pt *PathTrie) Size() int {
	return len(pt.GetAllProjects())
}

// Contains checks if a project path exists in the trie
func (pt *PathTrie) Contains(projectPath string) bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	normalizedPath := normalizePath(projectPath)
	pathSegments := strings.Split(strings.Trim(normalizedPath, "/"), "/")
	
	if len(pathSegments) == 1 && pathSegments[0] == "" {
		pathSegments = []string{}
	}

	current := pt.root
	
	for _, segment := range pathSegments {
		if segment == "" {
			continue
		}
		
		current.mu.RLock()
		next, exists := current.children[segment]
		current.mu.RUnlock()
		
		if !exists {
			return false
		}
		
		current = next
	}
	
	current.mu.RLock()
	defer current.mu.RUnlock()
	return current.isEndOfPath
}

// normalizePath normalizes file paths for consistent trie operations
func normalizePath(path string) string {
	// Remove file:// protocol if present
	if strings.HasPrefix(path, "file://") {
		path = path[7:]
	}
	
	// Clean path and ensure it starts with /
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	
	// Remove trailing slash except for root
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	
	return path
}