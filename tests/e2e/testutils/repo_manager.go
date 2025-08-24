package testutils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

// SetupRepository clones and prepares a test repository with comprehensive error handling
func (rm *RepoManager) SetupRepository(language string) (string, error) {
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Setting up repository for language: %s\n", language)
	}
	
	repos := GetTestRepositories()
	repo, exists := repos[language]
	if !exists {
		availableLanguages := make([]string, 0, len(repos))
		for lang := range repos {
			availableLanguages = append(availableLanguages, lang)
		}
		return "", fmt.Errorf("no test repository defined for language: %s (available: %v)", language, availableLanguages)
	}

	repoDir := filepath.Join(rm.baseDir, fmt.Sprintf("%s-%s", language, repo.Name))
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Repository setup - Language: %s, Repo: %s, URL: %s, Commit: %s, Dir: %s\n", language, repo.Name, repo.URL, repo.CommitHash, repoDir)
	}

	// Check if already cloned
	if _, err := os.Stat(repoDir); err == nil {
		if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
			log.Printf("[INFO] Repository already exists, updating to correct commit: %s\n", repoDir)
		}
		
		// Repository already exists, just update to correct commit
		if err := rm.checkoutCommit(repoDir, repo.CommitHash); err != nil {
			return "", fmt.Errorf("failed to checkout commit %s in existing repository %s for language %s: %w", repo.CommitHash, repo.URL, language, err)
		}
		
		// Verify repository health after checkout
		if err := rm.verifyRepositoryHealth(language, repoDir, repo); err != nil {
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[WARN] Repository health check failed after checkout, will re-clone: %v\n", err)
			}
			// Remove corrupted repository and proceed with fresh clone
			if removeErr := os.RemoveAll(repoDir); removeErr != nil {
				if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
					log.Printf("[ERROR] Failed to remove corrupted repository: %v\n", removeErr)
				}
			}
		} else {
			rm.repos[language] = repo
			
			// Verify repository registration after existing repo checkout
			if err := rm.verifyRepositoryRegistration(language, repoDir); err != nil {
				if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
					log.Printf("[ERROR] Repository registration verification failed after checkout: %v\n", err)
				}
				return "", fmt.Errorf("repository registration verification failed for %s after checkout: %w", language, err)
			}
			
			return repoDir, nil
		}
	}

	// Clone repository
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Cloning repository - URL: %s, Dir: %s\n", repo.URL, repoDir)
	}
	if err := rm.cloneRepository(repo.URL, repoDir, repo.CommitHash); err != nil {
		return "", fmt.Errorf("failed to clone repository %s for language %s to directory %s: %w", repo.URL, language, repoDir, err)
	}

	// Checkout specific commit
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Checking out specific commit: %s in %s\n", repo.CommitHash, repoDir)
	}
	if err := rm.checkoutCommit(repoDir, repo.CommitHash); err != nil {
		return "", fmt.Errorf("failed to checkout commit %s in repository %s for language %s: %w", repo.CommitHash, repo.URL, language, err)
	}

	// Verify repository health after setup
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Verifying repository health after setup - Language: %s, Dir: %s\n", language, repoDir)
	}
	if err := rm.verifyRepositoryHealth(language, repoDir, repo); err != nil {
		return "", fmt.Errorf("repository health check failed for %s repository %s: %w", language, repo.URL, err)
	}

	// Language-specific preparation
	switch language {
	case "java":
		if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
			log.Printf("[INFO] Preparing Java project in %s\n", repoDir)
		}
		// Try to generate Eclipse project files for JDTLS to index reliably
		// Prefer Gradle wrapper if present; fall back to Maven wrapper
		rm.prepareJavaProject(repoDir)
	}

	rm.repos[language] = repo
	
	// Verify repository registration
	if err := rm.verifyRepositoryRegistration(language, repoDir); err != nil {
		return "", fmt.Errorf("repository registration verification failed for %s: %w", language, err)
	}
	
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Repository setup completed successfully - Language: %s, Dir: %s, Registered: %t\n", language, repoDir, rm.IsRepositoryRegistered(language))
	}
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
 
// GitOperationFunc represents a git operation that can be retried
type GitOperationFunc func() ([]byte, error)

// retryGitOperation retries a git operation up to 3 times with exponential backoff
func (rm *RepoManager) retryGitOperation(operation GitOperationFunc, operationName string) error {
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
			log.Printf("[INFO] Attempting git operation: %s (attempt %d/%d)\n", operationName, attempt, maxRetries)
		}
		
		output, err := operation()
		if err == nil {
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[INFO] Git operation succeeded: %s (attempt %d)\n", operationName, attempt)
			}
			return nil
		}

		if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
			log.Printf("[ERROR] Git operation failed: %s (attempt %d) - %v, output: %s\n", operationName, attempt, err, string(output))
		}

		if attempt < maxRetries {
			delay := time.Duration(attempt) * baseDelay // exponential backoff: 1s, 2s, 3s
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[INFO] Retrying git operation after delay: %s (delay: %v)\n", operationName, delay)
			}
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("git operation failed after %d attempts: %s", maxRetries, operationName)
}

// cloneRepository clones a git repository efficiently with retry logic and robust fallback
func (rm *RepoManager) cloneRepository(url, targetDir, commit string) error {
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Starting repository clone - URL: %s, Dir: %s, Commit: %s\n", url, targetDir, commit)
	}

	isSHA := regexp.MustCompile(`^[0-9a-f]{7,40}$`).MatchString(commit)
	
	// Strategy 1: Shallow clone for tags/branches
	if !isSHA {
		branch := strings.TrimSpace(commit)
		if branch != "" {
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[INFO] Attempting shallow clone for branch/tag: %s from %s\n", branch, url)
			}
			
			err := rm.retryGitOperation(func() ([]byte, error) {
				cmd := exec.Command("git", "clone", "--depth", "1", "--filter=blob:none", "--branch", branch, url, targetDir)
				return cmd.CombinedOutput()
			}, fmt.Sprintf("shallow clone branch %s from %s", branch, url))
			
			if err == nil {
				if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
					log.Printf("[INFO] Shallow clone succeeded for branch: %s from %s\n", branch, url)
				}
				return nil
			}
			
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[WARN] Shallow clone failed for branch %s from %s, falling back to full clone: %v\n", branch, url, err)
			}
		}
	}

	// Strategy 2: Optimized fetch for specific SHA commits
	if isSHA {
		if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
			log.Printf("[INFO] Attempting optimized fetch for SHA commit: %s from %s\n", commit, url)
		}
		
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
		}
		
		cmds := []struct {
			args []string
			desc string
		}{
			{[]string{"git", "init"}, "initialize git repository"},
			{[]string{"git", "remote", "add", "origin", url}, "add remote origin"},
			{[]string{"git", "fetch", "--depth", "1", "--filter=blob:none", "origin", commit}, "fetch specific commit"},
			{[]string{"git", "checkout", "FETCH_HEAD"}, "checkout fetched commit"},
		}
		
		for _, cmdInfo := range cmds {
			err := rm.retryGitOperation(func() ([]byte, error) {
				cmd := exec.Command(cmdInfo.args[0], cmdInfo.args[1:]...)
				cmd.Dir = targetDir
				return cmd.CombinedOutput()
			}, fmt.Sprintf("%s for commit %s from %s", cmdInfo.desc, commit, url))
			
			if err != nil {
				if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
					log.Printf("[WARN] Optimized fetch failed for commit %s from %s at step %s, falling back to full clone: %v\n", commit, url, cmdInfo.desc, err)
				}
				break
			}
		}
		
		// Check if optimized fetch succeeded
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[INFO] Optimized fetch succeeded for commit: %s from %s\n", commit, url)
			}
			return nil
		}
		
		// Clean up failed attempt
		if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
			log.Printf("[WARN] Cleaning up failed optimized fetch attempt: %s\n", targetDir)
		}
		if cleanupErr := os.RemoveAll(targetDir); cleanupErr != nil {
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[ERROR] Failed to cleanup after failed fetch: %v\n", cleanupErr)
			}
		}
	}

	// Strategy 3: Full clone as final fallback
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Attempting full clone as fallback: %s\n", url)
	}
	
	err := rm.retryGitOperation(func() ([]byte, error) {
		cmd := exec.Command("git", "clone", url, targetDir)
		return cmd.CombinedOutput()
	}, fmt.Sprintf("full clone from %s", url))
	
	if err != nil {
		return fmt.Errorf("all clone strategies failed for repository %s: %w", url, err)
	}
	
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Full clone succeeded: %s\n", url)
	}
	return nil
}

// checkoutCommit checks out a specific commit or tag with retry logic
func (rm *RepoManager) checkoutCommit(repoDir, commitHash string) error {
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Checking out commit: %s in %s\n", commitHash, repoDir)
	}
	
	err := rm.retryGitOperation(func() ([]byte, error) {
		cmd := exec.Command("git", "checkout", commitHash)
		cmd.Dir = repoDir
		return cmd.CombinedOutput()
	}, fmt.Sprintf("checkout commit %s in %s", commitHash, repoDir))
	
	if err != nil {
		return fmt.Errorf("failed to checkout commit %s in repository %s: %w", commitHash, repoDir, err)
	}
	
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Successfully checked out commit: %s in %s\n", commitHash, repoDir)
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

// verifyRepositoryHealth performs comprehensive health checks on a cloned repository
func (rm *RepoManager) verifyRepositoryHealth(language, repoDir string, repo *TestRepository) error {
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Starting repository health check - Language: %s, Dir: %s\n", language, repoDir)
	}
	
	// Check 1: Verify .git directory exists
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("repository is not a valid git repository (missing .git directory): %s", repoDir)
	}
	
	// Check 2: Verify repository is not empty
	if isEmpty, err := rm.isDirectoryEmpty(repoDir); err != nil {
		return fmt.Errorf("failed to check if repository directory is empty: %w", err)
	} else if isEmpty {
		return fmt.Errorf("repository directory is empty: %s", repoDir)
	}
	
	// Check 3: Verify all expected test files exist
	missingFiles := []string{}
	for i, testFile := range repo.TestFiles {
		fullPath := filepath.Join(repoDir, testFile.Path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, fmt.Sprintf("%s (test file %d)", testFile.Path, i))
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
				log.Printf("[WARN] Expected test file missing: %s (full path: %s, test index: %d)\n", testFile.Path, fullPath, i)
			}
		} else {
			if os.Getenv("LSP_GATEWAY_DEBUG") == "true" && os.Getenv("LSP_GATEWAY_VERBOSE") == "true" {
				log.Printf("[DEBUG] Test file found: %s (full path: %s, test index: %d)\n", testFile.Path, fullPath, i)
			}
		}
	}
	
	if len(missingFiles) > 0 {
		return fmt.Errorf("repository %s for language %s is missing %d expected test files: %v", repoDir, language, len(missingFiles), missingFiles)
	}
	
	// Check 4: Language-specific health checks
	switch language {
	case "go":
		if err := rm.verifyGoRepository(repoDir); err != nil {
			return fmt.Errorf("Go repository health check failed: %w", err)
		}
	case "python":
		if err := rm.verifyPythonRepository(repoDir); err != nil {
			return fmt.Errorf("Python repository health check failed: %w", err)
		}
	case "java":
		if err := rm.verifyJavaRepository(repoDir); err != nil {
			return fmt.Errorf("Java repository health check failed: %w", err)
		}
	case "typescript", "javascript":
		if err := rm.verifyNodeRepository(repoDir); err != nil {
			return fmt.Errorf("%s repository health check failed: %w", language, err)
		}
	}
	
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Repository health check passed - Language: %s, Dir: %s, Test files: %d\n", language, repoDir, len(repo.TestFiles))
	}
	return nil
}

// isDirectoryEmpty checks if a directory is empty (excluding .git)
func (rm *RepoManager) isDirectoryEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	
	// Consider directory empty if it only contains .git
	for _, entry := range entries {
		if entry.Name() != ".git" {
			return false, nil
		}
	}
	return true, nil
}

// verifyGoRepository performs Go-specific health checks
func (rm *RepoManager) verifyGoRepository(repoDir string) error {
	goModPath := filepath.Join(repoDir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("Go repository missing go.mod file")
	}
	return nil
}

// verifyPythonRepository performs Python-specific health checks
func (rm *RepoManager) verifyPythonRepository(repoDir string) error {
	// Check for common Python project indicators
	pythonIndicators := []string{"setup.py", "pyproject.toml", "requirements.txt", "Pipfile"}
	for _, indicator := range pythonIndicators {
		if _, err := os.Stat(filepath.Join(repoDir, indicator)); err == nil {
			return nil // Found at least one Python project indicator
		}
	}
	// If no standard indicators, just check for .py files
	if hasExtension, err := rm.hasFilesWithExtension(repoDir, ".py"); err != nil {
		return fmt.Errorf("failed to check for Python files: %w", err)
	} else if !hasExtension {
		return fmt.Errorf("Python repository contains no .py files")
	}
	return nil
}

// verifyJavaRepository performs Java-specific health checks
func (rm *RepoManager) verifyJavaRepository(repoDir string) error {
	// Check for Java build files
	javaIndicators := []string{"pom.xml", "build.gradle", "build.gradle.kts"}
	for _, indicator := range javaIndicators {
		if _, err := os.Stat(filepath.Join(repoDir, indicator)); err == nil {
			return nil // Found Java build file
		}
	}
	return fmt.Errorf("Java repository missing build configuration (pom.xml, build.gradle, etc.)")
}

// verifyNodeRepository performs Node.js/TypeScript-specific health checks
func (rm *RepoManager) verifyNodeRepository(repoDir string) error {
	packageJsonPath := filepath.Join(repoDir, "package.json")
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		return fmt.Errorf("Node.js/TypeScript repository missing package.json file")
	}
	return nil
}

// hasFilesWithExtension checks if the directory contains files with the given extension
func (rm *RepoManager) hasFilesWithExtension(dir, ext string) (bool, error) {
	return rm.walkDirForExtension(dir, ext)
}

// walkDirForExtension recursively walks directory looking for files with extension
func (rm *RepoManager) walkDirForExtension(dir, ext string) (bool, error) {
	found := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ext) {
			found = true
			return filepath.SkipAll // Stop walking once we find a match
		}
		return nil
	})
	return found, err
}

// VerifyRepositoryRegistration checks if a repository is properly registered and accessible
func (rm *RepoManager) VerifyRepositoryRegistration(language string) error {
	repoPath, err := rm.GetRepositoryPath(language)
	if err != nil {
		return fmt.Errorf("repository path not accessible: %w", err)
	}
	return rm.verifyRepositoryRegistration(language, repoPath)
}

// verifyRepositoryRegistration performs comprehensive verification of repository registration
func (rm *RepoManager) verifyRepositoryRegistration(language, expectedPath string) error {
	// Check 1: Repository is stored in the repos map
	if !rm.IsRepositoryRegistered(language) {
		return fmt.Errorf("repository not found in internal registry for language: %s", language)
	}
	
	// Check 2: Repository path matches expected path
	actualPath, err := rm.GetRepositoryPath(language)
	if err != nil {
		return fmt.Errorf("repository path retrieval failed: %w", err)
	}
	
	if actualPath != expectedPath {
		return fmt.Errorf("repository path mismatch - expected: %s, actual: %s", expectedPath, actualPath)
	}
	
	// Check 3: Repository is accessible via GetTestFile
	testFile, err := rm.GetTestFile(language, 0)
	if err != nil {
		return fmt.Errorf("test file access failed: %w", err)
	}
	
	// Check 4: File URI generation works
	_, err = rm.GetFileURI(language, 0)
	if err != nil {
		return fmt.Errorf("file URI generation failed: %w", err)
	}
	
	// Check 5: File existence verification works
	err = rm.VerifyFileExists(language, 0)
	if err != nil {
		return fmt.Errorf("file existence verification failed: %w", err)
	}
	
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] Repository registration verification passed - Language: %s, Path: %s, TestFile: %s\n", language, actualPath, testFile.Path)
	}
	
	return nil
}

// IsRepositoryRegistered checks if a repository is registered in the internal map
func (rm *RepoManager) IsRepositoryRegistered(language string) bool {
	_, exists := rm.repos[language]
	return exists
}

// GetRegisteredLanguages returns all currently registered languages
func (rm *RepoManager) GetRegisteredLanguages() []string {
	languages := make([]string, 0, len(rm.repos))
	for lang := range rm.repos {
		languages = append(languages, lang)
	}
	return languages
}

// ValidateAllRegistrations verifies that all registered repositories are accessible
func (rm *RepoManager) ValidateAllRegistrations() error {
	languages := rm.GetRegisteredLanguages()
	if len(languages) == 0 {
		return fmt.Errorf("no repositories registered")
	}
	
	var errors []string
	for _, lang := range languages {
		if err := rm.VerifyRepositoryRegistration(lang); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", lang, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("repository validation failed for %d languages: %v", len(errors), errors)
	}
	
	if os.Getenv("LSP_GATEWAY_DEBUG") == "true" {
		log.Printf("[INFO] All repository registrations validated successfully - Languages: %v\n", languages)
	}
	
	return nil
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
