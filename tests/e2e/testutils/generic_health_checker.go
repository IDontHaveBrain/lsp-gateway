package testutils

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GenericHealthChecker provides language-agnostic repository health checking
type GenericHealthChecker struct {
	workspaceDir   string
	repoURL        string
	languageConfig LanguageConfig
}

// NewGenericHealthChecker creates a new generic health checker
func NewGenericHealthChecker(workspaceDir, repoURL string, languageConfig LanguageConfig) *GenericHealthChecker {
	return &GenericHealthChecker{
		workspaceDir:   workspaceDir,
		repoURL:        repoURL,
		languageConfig: languageConfig,
	}
}

// CheckRepositoryHealth performs comprehensive repository health validation
func (ghc *GenericHealthChecker) CheckRepositoryHealth() error {
	repoPath := ghc.getRepositoryPath()
	
	// Check if repository directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository directory not found: %s", repoPath)
	}

	// Check git repository integrity (if .git exists)
	if err := ghc.checkGitIntegrity(repoPath); err != nil {
		log.Printf("[HealthChecker] Git integrity check warning: %v", err)
	}

	// Validate root markers if specified
	if err := ghc.validateRootMarkers(repoPath); err != nil {
		return fmt.Errorf("root marker validation failed: %w", err)
	}

	// Validate test paths exist
	if err := ghc.validateTestPaths(repoPath); err != nil {
		return fmt.Errorf("test path validation failed: %w", err)
	}

	// Check for expected file patterns
	if err := ghc.validateFilePatterns(repoPath); err != nil {
		return fmt.Errorf("file pattern validation failed: %w", err)
	}

	log.Printf("[HealthChecker] Repository health check passed for %s", ghc.languageConfig.Language)
	return nil
}

// CheckNetworkConnectivity validates network connectivity to the repository
func (ghc *GenericHealthChecker) CheckNetworkConnectivity() error {
	if ghc.repoURL == "" {
		return fmt.Errorf("repository URL not configured")
	}

	// Test basic network connectivity
	if err := ghc.testNetworkConnectivity(); err != nil {
		return fmt.Errorf("network connectivity test failed: %w", err)
	}

	// Test git repository accessibility
	if err := ghc.testGitConnectivity(); err != nil {
		return fmt.Errorf("git connectivity test failed: %w", err)
	}

	log.Printf("[HealthChecker] Network connectivity check passed for %s", ghc.repoURL)
	return nil
}

// Private helper methods

func (ghc *GenericHealthChecker) getRepositoryPath() string {
	if ghc.languageConfig.RepoSubDir != "" {
		return filepath.Join(ghc.workspaceDir, ghc.languageConfig.RepoSubDir)
	}
	
	repoName := filepath.Base(ghc.languageConfig.RepoURL)
	if strings.HasSuffix(repoName, ".git") {
		repoName = strings.TrimSuffix(repoName, ".git")
	}
	
	return filepath.Join(ghc.workspaceDir, repoName)
}

func (ghc *GenericHealthChecker) checkGitIntegrity(repoPath string) error {
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		log.Printf("[HealthChecker] .git directory not found, assuming extracted repository")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check git status
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = repoPath
	if _, err := statusCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git status check failed: %w", err)
	}

	// Perform git fsck for integrity
	fsckCmd := exec.CommandContext(ctx, "git", "fsck", "--quick")
	fsckCmd.Dir = repoPath
	if output, err := fsckCmd.CombinedOutput(); err != nil {
		log.Printf("[HealthChecker] Git fsck warning: %v, output: %s", err, string(output))
	}

	return nil
}

func (ghc *GenericHealthChecker) validateRootMarkers(repoPath string) error {
	if len(ghc.languageConfig.RootMarkers) == 0 {
		return nil // No root markers specified, skip validation
	}

	found := false
	for _, marker := range ghc.languageConfig.RootMarkers {
		markerPath := filepath.Join(repoPath, marker)
		if _, err := os.Stat(markerPath); err == nil {
			found = true
			log.Printf("[HealthChecker] Found root marker: %s", marker)
			break
		}
	}

	if !found {
		return fmt.Errorf("none of the expected root markers found: %v", ghc.languageConfig.RootMarkers)
	}

	return nil
}

func (ghc *GenericHealthChecker) validateTestPaths(repoPath string) error {
	if len(ghc.languageConfig.TestPaths) == 0 {
		return nil // No test paths specified, skip validation
	}

	validPaths := 0
	for _, testPath := range ghc.languageConfig.TestPaths {
		fullPath := filepath.Join(repoPath, testPath)
		if _, err := os.Stat(fullPath); err == nil {
			validPaths++
			log.Printf("[HealthChecker] Found test path: %s", testPath)
		} else {
			log.Printf("[HealthChecker] Test path not found (may be optional): %s", testPath)
		}
	}

	if validPaths == 0 {
		return fmt.Errorf("none of the test paths exist: %v", ghc.languageConfig.TestPaths)
	}

	return nil
}

func (ghc *GenericHealthChecker) validateFilePatterns(repoPath string) error {
	if len(ghc.languageConfig.FilePatterns) == 0 {
		return nil // No file patterns specified, skip validation
	}

	matchingFiles := 0
	for _, testPath := range ghc.languageConfig.TestPaths {
		searchPath := filepath.Join(repoPath, testPath)
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue // Skip non-existent paths
		}

		err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				// Skip excluded directories
				for _, excludePath := range ghc.languageConfig.ExcludePaths {
					if strings.Contains(path, excludePath) {
						return filepath.SkipDir
					}
				}
				return nil
			}

			// Check if file matches any pattern
			for _, pattern := range ghc.languageConfig.FilePatterns {
				matched, err := filepath.Match(pattern, info.Name())
				if err != nil {
					continue
				}
				if matched {
					matchingFiles++
					return nil // Found a match, no need to check other patterns
				}
			}

			return nil
		})

		if err != nil {
			log.Printf("[HealthChecker] Warning during file pattern validation: %v", err)
		}
	}

	if matchingFiles == 0 {
		return fmt.Errorf("no files matching patterns %v found in repository", ghc.languageConfig.FilePatterns)
	}

	log.Printf("[HealthChecker] Found %d files matching patterns %v", matchingFiles, ghc.languageConfig.FilePatterns)
	return nil
}

func (ghc *GenericHealthChecker) testNetworkConnectivity() error {
	host, port := ghc.extractHostAndPort()
	if host == "" {
		return fmt.Errorf("could not extract host information from URL: %s", ghc.repoURL)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if err != nil {
		return fmt.Errorf("connectivity test failed for %s:%s: %w", host, port, err)
	}
	conn.Close()

	return nil
}

func (ghc *GenericHealthChecker) testGitConnectivity() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", ghc.repoURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git ls-remote failed: %w, output: %s", err, string(output))
	}

	if len(output) == 0 {
		return fmt.Errorf("git ls-remote returned empty output for %s", ghc.repoURL)
	}

	return nil
}

func (ghc *GenericHealthChecker) extractHostAndPort() (string, string) {
	var host, port string

	if strings.HasPrefix(ghc.repoURL, "https://github.com") {
		host, port = "github.com", "443"
	} else if strings.HasPrefix(ghc.repoURL, "https://gitlab.com") {
		host, port = "gitlab.com", "443"
	} else if strings.HasPrefix(ghc.repoURL, "https://bitbucket.org") {
		host, port = "bitbucket.org", "443"
	} else if strings.HasPrefix(ghc.repoURL, "http://") {
		host = strings.TrimPrefix(ghc.repoURL, "http://")
		if idx := strings.Index(host, "/"); idx != -1 {
			host = host[:idx]
		}
		port = "80"
	} else if strings.HasPrefix(ghc.repoURL, "https://") {
		host = strings.TrimPrefix(ghc.repoURL, "https://")
		if idx := strings.Index(host, "/"); idx != -1 {
			host = host[:idx]
		}
		port = "443"
	}

	// Handle custom ports in URL
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		if len(parts) == 2 {
			host, port = parts[0], parts[1]
		}
	}

	return host, port
}

// RepositoryHealthReport provides detailed health information
type RepositoryHealthReport struct {
	RepositoryPath    string            `json:"repository_path"`
	Language          string            `json:"language"`
	RepoURL           string            `json:"repo_url"`
	GitIntegrity      bool              `json:"git_integrity"`
	RootMarkers       []string          `json:"root_markers_found"`
	TestPaths         []string          `json:"test_paths_found"`
	FileCount         int               `json:"matching_files_count"`
	NetworkReachable  bool              `json:"network_reachable"`
	GitAccessible     bool              `json:"git_accessible"`
	HealthScore       float64           `json:"health_score"`
	Issues            []string          `json:"issues"`
	Timestamp         time.Time         `json:"timestamp"`
}

// GenerateHealthReport creates a comprehensive health report
func (ghc *GenericHealthChecker) GenerateHealthReport() *RepositoryHealthReport {
	report := &RepositoryHealthReport{
		RepositoryPath:   ghc.getRepositoryPath(),
		Language:         ghc.languageConfig.Language,
		RepoURL:          ghc.repoURL,
		RootMarkers:      []string{},
		TestPaths:        []string{},
		Issues:           []string{},
		Timestamp:        time.Now(),
	}

	// Check git integrity
	if err := ghc.checkGitIntegrity(report.RepositoryPath); err != nil {
		report.GitIntegrity = false
		report.Issues = append(report.Issues, fmt.Sprintf("Git integrity: %v", err))
	} else {
		report.GitIntegrity = true
	}

	// Check root markers
	for _, marker := range ghc.languageConfig.RootMarkers {
		markerPath := filepath.Join(report.RepositoryPath, marker)
		if _, err := os.Stat(markerPath); err == nil {
			report.RootMarkers = append(report.RootMarkers, marker)
		}
	}

	// Check test paths
	for _, testPath := range ghc.languageConfig.TestPaths {
		fullPath := filepath.Join(report.RepositoryPath, testPath)
		if _, err := os.Stat(fullPath); err == nil {
			report.TestPaths = append(report.TestPaths, testPath)
		}
	}

	// Count matching files
	report.FileCount = ghc.countMatchingFiles(report.RepositoryPath)

	// Check network connectivity
	if err := ghc.testNetworkConnectivity(); err != nil {
		report.NetworkReachable = false
		report.Issues = append(report.Issues, fmt.Sprintf("Network connectivity: %v", err))
	} else {
		report.NetworkReachable = true
	}

	// Check git accessibility
	if err := ghc.testGitConnectivity(); err != nil {
		report.GitAccessible = false
		report.Issues = append(report.Issues, fmt.Sprintf("Git accessibility: %v", err))
	} else {
		report.GitAccessible = true
	}

	// Calculate health score
	report.HealthScore = ghc.calculateHealthScore(report)

	return report
}

func (ghc *GenericHealthChecker) countMatchingFiles(repoPath string) int {
	count := 0
	
	for _, testPath := range ghc.languageConfig.TestPaths {
		searchPath := filepath.Join(repoPath, testPath)
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}

			// Skip excluded paths
			for _, excludePath := range ghc.languageConfig.ExcludePaths {
				if strings.Contains(path, excludePath) {
					return nil
				}
			}

			// Check if file matches any pattern
			for _, pattern := range ghc.languageConfig.FilePatterns {
				matched, err := filepath.Match(pattern, info.Name())
				if err != nil {
					continue
				}
				if matched {
					count++
					return nil
				}
			}

			return nil
		})
	}

	return count
}

func (ghc *GenericHealthChecker) calculateHealthScore(report *RepositoryHealthReport) float64 {
	score := 0.0
	maxScore := 6.0

	// Repository exists (base requirement)
	if _, err := os.Stat(report.RepositoryPath); err == nil {
		score += 1.0
	}

	// Git integrity
	if report.GitIntegrity {
		score += 1.0
	}

	// Root markers (if specified)
	if len(ghc.languageConfig.RootMarkers) > 0 {
		if len(report.RootMarkers) > 0 {
			score += 1.0
		}
	} else {
		score += 1.0 // Full score if no root markers required
	}

	// Test paths
	if len(report.TestPaths) > 0 {
		score += 1.0
	}

	// Matching files
	if report.FileCount > 0 {
		score += 1.0
	}

	// Network accessibility
	if report.NetworkReachable && report.GitAccessible {
		score += 1.0
	}

	return (score / maxScore) * 100.0
}