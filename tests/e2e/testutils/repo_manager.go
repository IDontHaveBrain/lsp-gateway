package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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
					DefinitionPos: Position{Line: 0, Character: 7},   // pipe import - should navigate to pipe.js definition
					ReferencePos:  Position{Line: 33, Character: 9},  // pipe usage - should find references including import
					HoverPos:      Position{Line: 29, Character: 25}, // compose function name
					CompletionPos: Position{Line: 31, Character: 4},  // inside compose function for completion
					SymbolQuery:   "compose",
				},
			},
		},
		"java": {
			Language:   "java",
			Name:       "spring-petclinic",
			URL:        "https://github.com/spring-projects/spring-petclinic.git",
			CommitHash: "30aab0ae764ad845b5eedd76028756835fec771f", // pinned for stable line positions
			TestFiles: []TestFile{
				{
					Path:          "src/main/java/org/springframework/samples/petclinic/PetClinicApplication.java",
					DefinitionPos: Position{Line: 28, Character: 20}, // PetClinicRuntimeHints in @ImportRuntimeHints(...)
					ReferencePos:  Position{Line: 28, Character: 20}, // same token to find definition+usages
					HoverPos:      Position{Line: 27, Character: 1},  // @SpringBootApplication annotation (Line 28, 0-based = 27)
					CompletionPos: Position{Line: 32, Character: 30}, // inside main method
					SymbolQuery:   "PetClinicRuntimeHints",
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
					DefinitionPos: Position{Line: 69, Character: 8},  // "Buffer" in Buffer::new() call -> navigate to struct definition
					ReferencePos:  Position{Line: 62, Character: 11}, // "Buffer" struct definition -> find all references across files
					HoverPos:      Position{Line: 96, Character: 29}, // "Integer" in generic constraint -> show trait information
					CompletionPos: Position{Line: 97, Character: 10}, // After "i." -> show available trait methods
					SymbolQuery:   "Buffer",
				},
			},
		},
		"csharp": {
			Language:   "csharp",
			Name:       "Newtonsoft.Json",
			URL:        "https://github.com/JamesNK/Newtonsoft.Json.git",
			CommitHash: "13.0.3", // stable release tag
			TestFiles: []TestFile{
				{
					Path:          "Src/Newtonsoft.Json/JsonConvert.cs",
					DefinitionPos: Position{Line: 900, Character: 25},
					ReferencePos:  Position{Line: 527, Character: 35},
					HoverPos:      Position{Line: 528, Character: 35},
					CompletionPos: Position{Line: 530, Character: 25},
					SymbolQuery:   "JsonConvert",
				},
			},
		},
		"kotlin": {
			Language:   "kotlin",
			Name:       "kotlin-result",
			URL:        "https://github.com/michaelbull/kotlin-result.git",
			CommitHash: "1.1.18", // Stable release - minimal Result type library (pure Kotlin)
			TestFiles: []TestFile{
				{
					Path:          "kotlin-result/src/commonMain/kotlin/com/github/michaelbull/result/Result.kt",
					DefinitionPos: Position{Line: 34, Character: 50}, // 'Result' usage in Ok extends clause -> definition jumps to sealed class
					ReferencePos:  Position{Line: 34, Character: 50}, // Same 'Result' usage ensures non-empty cross-file references
					HoverPos:      Position{Line: 21, Character: 30}, // 'of' function name hover (has KDoc/deprecation)
					CompletionPos: Position{Line: 35, Character: 4},  // Inside Ok class for completion
					SymbolQuery:   "combine",
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
	if err := rm.cloneRepository(repo.URL, repoDir, repo.CommitHash); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout specific commit
	if err := rm.checkoutCommit(repoDir, repo.CommitHash); err != nil {
		return "", fmt.Errorf("failed to checkout commit: %w", err)
	}

	// Language-specific preparation
	switch language {
	case "java":
		// Try to generate Eclipse project files for JDTLS to index reliably
		// Prefer Gradle wrapper if present; fall back to Maven wrapper
		rm.prepareJavaProject(repoDir)
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
 
// cloneRepository clones a git repository efficiently
func (rm *RepoManager) cloneRepository(url, targetDir, commit string) error {
	isSHA := regexp.MustCompile(`^[0-9a-f]{7,40}$`).MatchString(commit)
	if !isSHA {
		branch := strings.TrimSpace(commit)
		if branch != "" {
			cmd := exec.Command("git", "clone", "--depth", "1", "--filter=blob:none", "--branch", branch, url, targetDir)
			if output, err := cmd.CombinedOutput(); err == nil {
				_ = output
				return nil
			}
			// fallthrough to full clone on failure
		}
	}
	if isSHA {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return err
		}
		cmds := [][]string{
			{"git", "init"},
			{"git", "remote", "add", "origin", url},
			{"git", "fetch", "--depth", "1", "--filter=blob:none", "origin", commit},
			{"git", "checkout", "FETCH_HEAD"},
		}
		for _, args := range cmds {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = targetDir
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%s failed: %w, output: %s", strings.Join(args, " "), err, string(output))
			}
		}
		return nil
	}
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

// prepareJavaProject attempts to set up Eclipse project metadata to help jdtls
func (rm *RepoManager) prepareJavaProject(repoDir string) {
	if strings.TrimSpace(os.Getenv("SKIP_JAVA_PREPARE")) == "1" || strings.ToLower(strings.TrimSpace(os.Getenv("SKIP_JAVA_PREPARE"))) == "true" {
		return
	}
	// Try Gradle eclipse
	try := func(name string, args ...string) bool {
		cmd := exec.Command(name, args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err == nil {
			_ = out
			return true
		}
		return false
	}
	// Make gradlew executable if present
	_ = os.Chmod(filepath.Join(repoDir, "gradlew"), 0755)
	if try("./gradlew", "-q", "eclipse") || try("gradlew", "-q", "eclipse") || try("./gradlew", "eclipse") {
		return
	}
	// Fallback to Maven wrapper generating eclipse files if plugin is available
	_ = os.Chmod(filepath.Join(repoDir, "mvnw"), 0755)
	if try("./mvnw", "-q", "-DskipTests", "eclipse:eclipse") || try("mvn", "-q", "-DskipTests", "eclipse:eclipse") {
		return
	}
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
