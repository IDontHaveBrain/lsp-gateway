package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TestRepository defines a test repository configuration
type TestRepository struct {
	Language   string
	Name       string
	URL        string
	CommitHash string
	TestFiles  []TestFile
}

// TestFile defines files and positions for LSP testing
type TestFile struct {
	Path          string
	DefinitionPos Position
	ReferencePos  Position
	HoverPos      Position
	CompletionPos Position
	SymbolQuery   string
}

// Position represents line/character position (0-based)
type Position struct {
	Line      int
	Character int
}

// RepoManager manages test repositories
type RepoManager struct {
	baseDir string
	repos   map[string]*TestRepository
}

// NewRepoManager creates a new repository manager
func NewRepoManager(baseDir string) *RepoManager {
	return &RepoManager{
		baseDir: baseDir,
		repos:   make(map[string]*TestRepository),
	}
}

// GetTestRepositories returns predefined test repositories for each language
func GetTestRepositories() map[string]*TestRepository {
	return map[string]*TestRepository{
		"go": {
			Language:   "go",
			Name:       "gorilla-mux",
			URL:        "https://github.com/gorilla/mux.git",
			CommitHash: "v1.8.0", // stable release tag
			TestFiles: []TestFile{
				{
					Path:          "mux.go",
					DefinitionPos: Position{Line: 18, Character: 5},  // Router struct definition
					ReferencePos:  Position{Line: 65, Character: 10}, // Router usage in NewRouter
					HoverPos:      Position{Line: 18, Character: 10}, // Router struct name
					CompletionPos: Position{Line: 100, Character: 5}, // Inside function for completion
					SymbolQuery:   "Router",
				},
			},
		},
		"python": {
			Language:   "python",
			Name:       "requests",
			URL:        "https://github.com/psf/requests.git",
			CommitHash: "v2.28.1", // stable release tag (v2.28.2 has checkout issues)
			TestFiles: []TestFile{
				{
					Path:          "requests/api.py",
					DefinitionPos: Position{Line: 15, Character: 4},  // get function definition
					ReferencePos:  Position{Line: 25, Character: 10}, // function usage
					HoverPos:      Position{Line: 15, Character: 8},  // function name
					CompletionPos: Position{Line: 30, Character: 4},  // inside function
					SymbolQuery:   "get",
				},
			},
		},
		"typescript": {
			Language:   "typescript",
			Name:       "is",
			URL:        "https://github.com/sindresorhus/is.git",
			CommitHash: "v5.4.1", // stable release tag
			TestFiles: []TestFile{
				{
					Path:          "source/index.ts",
					DefinitionPos: Position{Line: 50, Character: 8},  // is object definition
					ReferencePos:  Position{Line: 100, Character: 5}, // is usage
					HoverPos:      Position{Line: 50, Character: 12}, // type annotation
					CompletionPos: Position{Line: 200, Character: 3}, // completion point
					SymbolQuery:   "is",
				},
			},
		},
		"java": {
			Language:   "java",
			Name:       "spring-petclinic",
			URL:        "https://github.com/spring-projects/spring-petclinic.git",
			CommitHash: "main", // using main branch since v2.7.3 tag doesn't exist
			TestFiles: []TestFile{
				{
					Path:          "src/main/java/org/springframework/samples/petclinic/PetClinicApplication.java",
					DefinitionPos: Position{Line: 29, Character: 13}, // PetClinicApplication class name
					ReferencePos:  Position{Line: 32, Character: 16}, // SpringApplication.run usage
					HoverPos:      Position{Line: 28, Character: 5},  // @SpringBootApplication annotation
					CompletionPos: Position{Line: 33, Character: 8},  // inside main method
					SymbolQuery:   "PetClinicApplication",
				},
			},
		},
	}
}

// SetupRepository clones and prepares a test repository
func (rm *RepoManager) SetupRepository(language string) (string, error) {
	repos := GetTestRepositories()
	repo, exists := repos[language]
	if !exists {
		return "", fmt.Errorf("no test repository defined for language: %s", language)
	}

	repoDir := filepath.Join(rm.baseDir, fmt.Sprintf("%s-%s", language, repo.Name))

	// Check if already cloned
	if _, err := os.Stat(repoDir); err == nil {
		// Repository already exists, just update to correct commit
		if err := rm.checkoutCommit(repoDir, repo.CommitHash); err != nil {
			return "", fmt.Errorf("failed to checkout commit: %w", err)
		}
		rm.repos[language] = repo
		return repoDir, nil
	}

	// Clone repository
	if err := rm.cloneRepository(repo.URL, repoDir); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout specific commit
	if err := rm.checkoutCommit(repoDir, repo.CommitHash); err != nil {
		return "", fmt.Errorf("failed to checkout commit: %w", err)
	}

	rm.repos[language] = repo
	return repoDir, nil
}

// GetTestFile returns test file configuration for a language
func (rm *RepoManager) GetTestFile(language string, index int) (*TestFile, error) {
	repo, exists := rm.repos[language]
	if !exists {
		return nil, fmt.Errorf("repository not set up for language: %s", language)
	}

	if index >= len(repo.TestFiles) {
		return nil, fmt.Errorf("test file index %d out of range for language %s", index, language)
	}

	return &repo.TestFiles[index], nil
}

// GetRepositoryPath returns the path to a cloned repository
func (rm *RepoManager) GetRepositoryPath(language string) (string, error) {
	repos := GetTestRepositories()
	repo, exists := repos[language]
	if !exists {
		return "", fmt.Errorf("no test repository defined for language: %s", language)
	}

	repoDir := filepath.Join(rm.baseDir, fmt.Sprintf("%s-%s", language, repo.Name))
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return "", fmt.Errorf("repository not cloned for language: %s", language)
	}

	return repoDir, nil
}

// Cleanup removes all cloned repositories
func (rm *RepoManager) Cleanup() error {
	if rm.baseDir != "" {
		return os.RemoveAll(rm.baseDir)
	}
	return nil
}

// cloneRepository clones a git repository
func (rm *RepoManager) cloneRepository(url, targetDir string) error {
	cmd := exec.Command("git", "clone", url, targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}
	return nil
}

// checkoutCommit checks out a specific commit or tag
func (rm *RepoManager) checkoutCommit(repoDir, commitHash string) error {
	cmd := exec.Command("git", "checkout", commitHash)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout failed: %w, output: %s", err, string(output))
	}
	return nil
}

// GetFileURI returns the file URI for a test file
func (rm *RepoManager) GetFileURI(language string, testFileIndex int) (string, error) {
	repoPath, err := rm.GetRepositoryPath(language)
	if err != nil {
		return "", err
	}

	testFile, err := rm.GetTestFile(language, testFileIndex)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(repoPath, testFile.Path)

	// Convert to file URI
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}

	return "file://" + fullPath, nil
}

// VerifyFileExists checks if a test file exists
func (rm *RepoManager) VerifyFileExists(language string, testFileIndex int) error {
	repoPath, err := rm.GetRepositoryPath(language)
	if err != nil {
		return err
	}

	testFile, err := rm.GetTestFile(language, testFileIndex)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(repoPath, testFile.Path)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("test file does not exist: %s", fullPath)
	}

	return nil
}
