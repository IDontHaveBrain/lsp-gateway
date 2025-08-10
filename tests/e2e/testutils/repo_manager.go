package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"lsp-gateway/src/utils"
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
					DefinitionPos: Position{Line: 46, Character: 5},  // type Router struct definition (Line 47, 0-based = 46)
					ReferencePos:  Position{Line: 24, Character: 10}, // NewRouter usage of Router (Line 25, 0-based = 24)
					HoverPos:      Position{Line: 46, Character: 10}, // Router struct name
					CompletionPos: Position{Line: 25, Character: 15}, // Inside return statement for completion
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
					DefinitionPos: Position{Line: 61, Character: 4}, // def get function definition (Line 62, 0-based = 61)
					ReferencePos:  Position{Line: 13, Character: 4}, // request function reference
					HoverPos:      Position{Line: 61, Character: 8}, // get function name
					CompletionPos: Position{Line: 65, Character: 4}, // inside get function for completion
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
					DefinitionPos: Position{Line: 101, Character: 9},  // function is definition (Line 102, 0-based = 101)
					ReferencePos:  Position{Line: 737, Character: 15}, // export default is usage (Line 738, 0-based = 737)
					HoverPos:      Position{Line: 101, Character: 12}, // is function name
					CompletionPos: Position{Line: 103, Character: 4},  // inside is function for completion
					SymbolQuery:   "is",
				},
			},
		},
		"javascript": {
			Language:   "javascript",
			Name:       "ramda",
			URL:        "https://github.com/ramda/ramda.git",
			CommitHash: "v0.31.3", // stable release tag (lightweight JavaScript functional library)
			TestFiles: []TestFile{
				{
					Path:          "source/compose.js",               // Pure JavaScript functional programming utilities
					DefinitionPos: Position{Line: 23, Character: 23}, // export default compose function definition (Line 24, 0-based = 23)
					ReferencePos:  Position{Line: 26, Character: 2},  // pipe function reference usage (Line 27, 0-based = 26)
					HoverPos:      Position{Line: 23, Character: 28}, // compose function name
					CompletionPos: Position{Line: 25, Character: 4},  // inside compose function for completion
					SymbolQuery:   "compose",
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
					DefinitionPos: Position{Line: 29, Character: 13}, // public class PetClinicApplication (Line 30, 0-based = 29)
					ReferencePos:  Position{Line: 32, Character: 16}, // SpringApplication.run(PetClinicApplication.class) usage (Line 33, 0-based = 32)
					HoverPos:      Position{Line: 27, Character: 1},  // @SpringBootApplication annotation (Line 28, 0-based = 27)
					CompletionPos: Position{Line: 32, Character: 20}, // inside main method for completion
					SymbolQuery:   "PetClinicApplication",
				},
			},
		},
		"rust": {
			Language:   "rust",
			Name:       "itoa",
			URL:        "https://github.com/dtolnay/itoa.git",
			CommitHash: "1.0.15", // stable small crate
			TestFiles: []TestFile{
				{
					Path:          "src/lib.rs",
					DefinitionPos: Position{Line: 62, Character: 12},
					ReferencePos:  Position{Line: 69, Character: 8}, // "Buffer::new()" usage in default impl
					HoverPos:      Position{Line: 97, Character: 11},
					CompletionPos: Position{Line: 89, Character: 8},
					SymbolQuery:   "Buffer",
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

	// Convert to file URI using proper URI conversion that handles Windows paths
	// This ensures proper handling of drive letters and path separators on Windows
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Use the utils.FilePathToURI function which properly handles Windows paths
	// Note: We need to import the utils package for this
	return utils.FilePathToURI(absPath), nil
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
