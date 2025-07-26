package project

import (
	"encoding/json"
	"testing"
	"time"

	"lsp-gateway/internal/project/types"
)

func TestProjectAnalysisResult_IsSuccessful(t *testing.T) {
	tests := []struct {
		name     string
		status   AnalysisStatus
		expected bool
	}{
		{
			name:     "success status returns true",
			status:   AnalysisStatusSuccess,
			expected: true,
		},
		{
			name:     "failed status returns false",
			status:   AnalysisStatusFailed,
			expected: false,
		},
		{
			name:     "partial status returns false",
			status:   AnalysisStatusPartial,
			expected: false,
		},
		{
			name:     "timeout status returns false",
			status:   AnalysisStatusTimeout,
			expected: false,
		},
		{
			name:     "in_progress status returns false",
			status:   AnalysisStatusInProgress,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			par := &ProjectAnalysisResult{Status: tt.status}
			if got := par.IsSuccessful(); got != tt.expected {
				t.Errorf("IsSuccessful() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectAnalysisResult_HasErrors(t *testing.T) {
	tests := []struct {
		name              string
		errors            []AnalysisError
		validationErrors  []string
		expected          bool
	}{
		{
			name:     "no errors returns false",
			errors:   []AnalysisError{},
			expected: false,
		},
		{
			name: "has analysis errors returns true",
			errors: []AnalysisError{
				{Message: "test error", Type: "parsing", Phase: "detection"},
			},
			expected: true,
		},
		{
			name:             "has validation errors returns true",
			validationErrors: []string{"validation failed"},
			expected:         true,
		},
		{
			name: "has both types of errors returns true",
			errors: []AnalysisError{
				{Message: "test error", Type: "parsing", Phase: "detection"},
			},
			validationErrors: []string{"validation failed"},
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			par := &ProjectAnalysisResult{
				Errors:           tt.errors,
				ValidationErrors: tt.validationErrors,
			}
			if got := par.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectAnalysisResult_HasWarnings(t *testing.T) {
	tests := []struct {
		name               string
		warnings           []AnalysisWarning
		validationWarnings []string
		expected           bool
	}{
		{
			name:     "no warnings returns false",
			warnings: []AnalysisWarning{},
			expected: false,
		},
		{
			name: "has analysis warnings returns true",
			warnings: []AnalysisWarning{
				{Message: "test warning", Type: "performance", Phase: "detection"},
			},
			expected: true,
		},
		{
			name:               "has validation warnings returns true",
			validationWarnings: []string{"validation warning"},
			expected:           true,
		},
		{
			name: "has both types of warnings returns true",
			warnings: []AnalysisWarning{
				{Message: "test warning", Type: "performance", Phase: "detection"},
			},
			validationWarnings: []string{"validation warning"},
			expected:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			par := &ProjectAnalysisResult{
				Warnings:           tt.warnings,
				ValidationWarnings: tt.validationWarnings,
			}
			if got := par.HasWarnings(); got != tt.expected {
				t.Errorf("HasWarnings() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectAnalysisResult_GetConfidenceLevel(t *testing.T) {
	tests := []struct {
		name           string
		detectionScore float64
		expected       ConfidenceLevel
	}{
		{
			name:           "high confidence (0.9)",
			detectionScore: 0.9,
			expected:       ConfidenceLevelHigh,
		},
		{
			name:           "high confidence threshold (0.8)",
			detectionScore: 0.8,
			expected:       ConfidenceLevelHigh,
		},
		{
			name:           "medium confidence (0.7)",
			detectionScore: 0.7,
			expected:       ConfidenceLevelMedium,
		},
		{
			name:           "medium confidence threshold (0.5)",
			detectionScore: 0.5,
			expected:       ConfidenceLevelMedium,
		},
		{
			name:           "low confidence (0.3)",
			detectionScore: 0.3,
			expected:       ConfidenceLevelLow,
		},
		{
			name:           "low confidence threshold (0.2)",
			detectionScore: 0.2,
			expected:       ConfidenceLevelLow,
		},
		{
			name:           "no confidence (0.1)",
			detectionScore: 0.1,
			expected:       ConfidenceLevelNone,
		},
		{
			name:           "zero confidence",
			detectionScore: 0.0,
			expected:       ConfidenceLevelNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			par := &ProjectAnalysisResult{DetectionScore: tt.detectionScore}
			if got := par.GetConfidenceLevel(); got != tt.expected {
				t.Errorf("GetConfidenceLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectAnalysisResult_IsLargeProject(t *testing.T) {
	tests := []struct {
		name       string
		totalFiles int
		expected   bool
	}{
		{
			name:       "small project (500 files)",
			totalFiles: 500,
			expected:   false,
		},
		{
			name:       "threshold project (1000 files)",
			totalFiles: 1000,
			expected:   false,
		},
		{
			name:       "large project (1500 files)",
			totalFiles: 1500,
			expected:   true,
		},
		{
			name:       "very large project (3000 files)",
			totalFiles: 3000,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			par := &ProjectAnalysisResult{
				ProjectSize: ProjectSize{TotalFiles: tt.totalFiles},
			}
			if got := par.IsLargeProject(); got != tt.expected {
				t.Errorf("IsLargeProject() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectAnalysisResult_IsHugeProject(t *testing.T) {
	tests := []struct {
		name       string
		totalFiles int
		expected   bool
	}{
		{
			name:       "small project (500 files)",
			totalFiles: 500,
			expected:   false,
		},
		{
			name:       "large project (3000 files)",
			totalFiles: 3000,
			expected:   false,
		},
		{
			name:       "threshold project (5000 files)",
			totalFiles: 5000,
			expected:   false,
		},
		{
			name:       "huge project (6000 files)",
			totalFiles: 6000,
			expected:   true,
		},
		{
			name:       "very huge project (10000 files)",
			totalFiles: 10000,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			par := &ProjectAnalysisResult{
				ProjectSize: ProjectSize{TotalFiles: tt.totalFiles},
			}
			if got := par.IsHugeProject(); got != tt.expected {
				t.Errorf("IsHugeProject() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectAnalysisResult_GetSuggestedTimeout(t *testing.T) {
	tests := []struct {
		name       string
		totalFiles int
		expected   time.Duration
	}{
		{
			name:       "small project (500 files)",
			totalFiles: 500,
			expected:   DefaultAnalysisTimeout,
		},
		{
			name:       "large project (1500 files)",
			totalFiles: 1500,
			expected:   DefaultAnalysisTimeout * 2,
		},
		{
			name:       "huge project (6000 files)",
			totalFiles: 6000,
			expected:   DefaultAnalysisTimeout * 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			par := &ProjectAnalysisResult{
				ProjectSize: ProjectSize{TotalFiles: tt.totalFiles},
			}
			if got := par.GetSuggestedTimeout(); got != tt.expected {
				t.Errorf("GetSuggestedTimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProjectAnalysisResult_JSON(t *testing.T) {
	now := time.Now()
	par := &ProjectAnalysisResult{
		Path:           "/test/project",
		NormalizedPath: "/test/project",
		Timestamp:      now,
		Duration:       5 * time.Second,
		Status:         AnalysisStatusSuccess,
		DetectionScore: 0.85,
		ProjectSize: ProjectSize{
			TotalFiles:  1500,
			SourceFiles: 1200,
			TestFiles:   300,
		},
		RequiredServers: []string{"gopls", "pylsp"},
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	jsonData, err := par.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() failed: %v", err)
	}

	decoded, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("FromJSON() failed: %v", err)
	}

	if decoded.Path != par.Path {
		t.Errorf("Path mismatch: got %v, want %v", decoded.Path, par.Path)
	}
	if decoded.Status != par.Status {
		t.Errorf("Status mismatch: got %v, want %v", decoded.Status, par.Status)
	}
	if decoded.DetectionScore != par.DetectionScore {
		t.Errorf("DetectionScore mismatch: got %v, want %v", decoded.DetectionScore, par.DetectionScore)
	}
	if decoded.ProjectSize.TotalFiles != par.ProjectSize.TotalFiles {
		t.Errorf("TotalFiles mismatch: got %v, want %v", decoded.ProjectSize.TotalFiles, par.ProjectSize.TotalFiles)
	}
	if len(decoded.RequiredServers) != len(par.RequiredServers) {
		t.Errorf("RequiredServers length mismatch: got %v, want %v", len(decoded.RequiredServers), len(par.RequiredServers))
	}
}

func TestAnalysisError_Construction(t *testing.T) {
	now := time.Now()
	err := AnalysisError{
		Type:        "parsing",
		Phase:       "detection",
		Message:     "Failed to parse configuration",
		Path:        "/test/config.yaml",
		Details:     "Invalid YAML syntax on line 5",
		Severity:    ErrorSeverityHigh,
		Recoverable: true,
		Suggestions: []string{"Check YAML syntax", "Validate configuration"},
		Metadata: map[string]interface{}{
			"line": 5,
			"col":  10,
		},
		Timestamp: now,
	}

	if err.Type != "parsing" {
		t.Errorf("Type mismatch: got %v, want %v", err.Type, "parsing")
	}
	if err.Severity != ErrorSeverityHigh {
		t.Errorf("Severity mismatch: got %v, want %v", err.Severity, ErrorSeverityHigh)
	}
	if !err.Recoverable {
		t.Error("Expected error to be recoverable")
	}
	if len(err.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(err.Suggestions))
	}

	jsonData, err2 := json.Marshal(err)
	if err2 != nil {
		t.Fatalf("JSON marshaling failed: %v", err2)
	}

	var decoded AnalysisError
	if err3 := json.Unmarshal(jsonData, &decoded); err3 != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err3)
	}

	if decoded.Type != err.Type {
		t.Errorf("JSON roundtrip failed for Type: got %v, want %v", decoded.Type, err.Type)
	}
}

func TestAnalysisWarning_Construction(t *testing.T) {
	now := time.Now()
	warning := AnalysisWarning{
		Type:        "performance",
		Phase:       "validation",
		Message:     "Large project detected",
		Path:        "/test/project",
		Details:     "Project has over 5000 files",
		Severity:    WarningSeverityMedium,
		Suggestions: []string{"Consider breaking into smaller modules"},
		Metadata: map[string]interface{}{
			"file_count": 5500,
		},
		Timestamp: now,
	}

	if warning.Type != "performance" {
		t.Errorf("Type mismatch: got %v, want %v", warning.Type, "performance")
	}
	if warning.Severity != WarningSeverityMedium {
		t.Errorf("Severity mismatch: got %v, want %v", warning.Severity, WarningSeverityMedium)
	}
	if len(warning.Suggestions) != 1 {
		t.Errorf("Expected 1 suggestion, got %d", len(warning.Suggestions))
	}

	jsonData, err := json.Marshal(warning)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var decoded AnalysisWarning
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if decoded.Type != warning.Type {
		t.Errorf("JSON roundtrip failed for Type: got %v, want %v", decoded.Type, warning.Type)
	}
}

func TestPerformanceMetrics_Calculation(t *testing.T) {
	metrics := PerformanceMetrics{
		AnalysisTime:         5 * time.Second,
		DetectionTime:        2 * time.Second,
		ValidationTime:       1 * time.Second,
		ConfigGenerationTime: 500 * time.Millisecond,
		FilesScanned:         1500,
		DirsTraversed:        50,
		MemoryUsageBytes:     1024 * 1024 * 100, // 100 MB
		CacheHits:            850,
		CacheMisses:          150,
	}

	if metrics.AnalysisTime != 5*time.Second {
		t.Errorf("AnalysisTime mismatch: got %v, want %v", metrics.AnalysisTime, 5*time.Second)
	}
	if metrics.FilesScanned != 1500 {
		t.Errorf("FilesScanned mismatch: got %v, want %v", metrics.FilesScanned, 1500)
	}
	if metrics.MemoryUsageBytes != 1024*1024*100 {
		t.Errorf("MemoryUsageBytes mismatch: got %v, want %v", metrics.MemoryUsageBytes, 1024*1024*100)
	}

	totalCacheOps := metrics.CacheHits + metrics.CacheMisses
	expectedTotal := 1000
	if totalCacheOps != expectedTotal {
		t.Errorf("Total cache operations mismatch: got %v, want %v", totalCacheOps, expectedTotal)
	}

	hitRate := float64(metrics.CacheHits) / float64(totalCacheOps)
	expectedHitRate := 0.85
	if hitRate != expectedHitRate {
		t.Errorf("Cache hit rate mismatch: got %v, want %v", hitRate, expectedHitRate)
	}
}

func TestDetectedFramework_ConfidenceScoring(t *testing.T) {
	tests := []struct {
		name       string
		framework  DetectedFramework
		minConf    float64
		maxConf    float64
	}{
		{
			name: "high confidence framework",
			framework: DetectedFramework{
				Name:        "React",
				Version:     "18.2.0",
				Type:        FrameworkTypeWeb,
				Confidence:  0.95,
				ConfigFiles: []string{"package.json", "tsconfig.json"},
				Metadata: map[string]interface{}{
					"jsx_files": 45,
				},
			},
			minConf: 0.8,
			maxConf: 1.0,
		},
		{
			name: "medium confidence framework",
			framework: DetectedFramework{
				Name:       "Express",
				Type:       FrameworkTypeAPI,
				Confidence: 0.65,
				Metadata: map[string]interface{}{
					"server_files": 3,
				},
			},
			minConf: 0.5,
			maxConf: 0.8,
		},
		{
			name: "low confidence framework",
			framework: DetectedFramework{
				Name:       "Unknown Framework",
				Type:       FrameworkTypeUnknown,
				Confidence: 0.3,
			},
			minConf: 0.0,
			maxConf: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.framework.Confidence < tt.minConf || tt.framework.Confidence > tt.maxConf {
				t.Errorf("Confidence %v not in range [%v, %v]", tt.framework.Confidence, tt.minConf, tt.maxConf)
			}

			jsonData, err := json.Marshal(tt.framework)
			if err != nil {
				t.Fatalf("JSON marshaling failed: %v", err)
			}

			var decoded DetectedFramework
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("JSON unmarshaling failed: %v", err)
			}

			if decoded.Name != tt.framework.Name {
				t.Errorf("Name mismatch after JSON roundtrip: got %v, want %v", decoded.Name, tt.framework.Name)
			}
			if decoded.Confidence != tt.framework.Confidence {
				t.Errorf("Confidence mismatch after JSON roundtrip: got %v, want %v", decoded.Confidence, tt.framework.Confidence)
			}
		})
	}
}

func TestRecommendation_Structure(t *testing.T) {
	rec := Recommendation{
		Type:        "optimization",
		Priority:    RecommendationPriorityHigh,
		Title:       "Optimize Build Process",
		Description: "Consider using a faster build tool to reduce compilation time",
		Actions:     []string{"Evaluate Vite", "Configure parallel builds", "Enable caching"},
		Benefits:    []string{"Faster development cycle", "Reduced CI time"},
		Metadata: map[string]interface{}{
			"current_build_time": "45s",
			"estimated_savings":  "60%",
		},
	}

	if rec.Priority != RecommendationPriorityHigh {
		t.Errorf("Priority mismatch: got %v, want %v", rec.Priority, RecommendationPriorityHigh)
	}
	if len(rec.Actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(rec.Actions))
	}
	if len(rec.Benefits) != 2 {
		t.Errorf("Expected 2 benefits, got %d", len(rec.Benefits))
	}

	jsonData, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var decoded Recommendation
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if decoded.Title != rec.Title {
		t.Errorf("Title mismatch after JSON roundtrip: got %v, want %v", decoded.Title, rec.Title)
	}
}

func TestLanguageDetectionResult_Structure(t *testing.T) {
	result := types.LanguageDetectionResult{
		Language:    "go",
		Confidence:  0.9,
		Version:     "1.21",
		MarkerFiles: []string{"go.mod", "go.sum"},
		ConfigFiles: []string{".golangci.yml"},
		SourceDirs:  []string{"cmd", "internal", "pkg"},
		TestDirs:    []string{"test", "tests"},
		Dependencies: map[string]string{
			"github.com/gin-gonic/gin": "v1.9.1",
		},
		BuildFiles:      []string{"Makefile"},
		RequiredServers: []string{"gopls"},
		Metadata: map[string]interface{}{
			"module_name": "example.com/myproject",
		},
	}

	if result.Language != "go" {
		t.Errorf("Language mismatch: got %v, want %v", result.Language, "go")
	}
	if result.Confidence != 0.9 {
		t.Errorf("Confidence mismatch: got %v, want %v", result.Confidence, 0.9)
	}
	if len(result.MarkerFiles) != 2 {
		t.Errorf("Expected 2 marker files, got %d", len(result.MarkerFiles))
	}
	if len(result.RequiredServers) != 1 {
		t.Errorf("Expected 1 required server, got %d", len(result.RequiredServers))
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var decoded types.LanguageDetectionResult
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if decoded.Language != result.Language {
		t.Errorf("Language mismatch after JSON roundtrip: got %v, want %v", decoded.Language, result.Language)
	}
}

func TestProjectSize_Metrics(t *testing.T) {
	size := types.ProjectSize{
		TotalFiles:     1500,
		SourceFiles:    1200,
		TestFiles:      200,
		ConfigFiles:    100,
		TotalSizeBytes: 1024 * 1024 * 50, // 50 MB
	}

	if size.TotalFiles != 1500 {
		t.Errorf("TotalFiles mismatch: got %v, want %v", size.TotalFiles, 1500)
	}

	calculatedTotal := size.SourceFiles + size.TestFiles + size.ConfigFiles
	if calculatedTotal != 1500 {
		t.Errorf("Calculated total files mismatch: got %v, want %v", calculatedTotal, 1500)
	}

	if size.TotalSizeBytes != 1024*1024*50 {
		t.Errorf("TotalSizeBytes mismatch: got %v, want %v", size.TotalSizeBytes, 1024*1024*50)
	}

	jsonData, err := json.Marshal(size)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var decoded types.ProjectSize
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if decoded.TotalFiles != size.TotalFiles {
		t.Errorf("TotalFiles mismatch after JSON roundtrip: got %v, want %v", decoded.TotalFiles, size.TotalFiles)
	}
}

func TestLanguageInfo_Comprehensive(t *testing.T) {
	info := types.LanguageInfo{
		Name:           "go",
		DisplayName:    "Go",
		Version:        "1.21.0",
		MinVersion:     "1.19.0",
		MaxVersion:     "1.22.0",
		BuildTools:     []string{"go build", "make"},
		PackageManager: "go mod",
		TestFrameworks: []string{"testing", "testify"},
		LintTools:      []string{"golangci-lint", "staticcheck"},
		FormatTools:    []string{"gofmt", "goimports"},
		LSPServers:     []string{"gopls"},
		FileExtensions: []string{".go"},
		Capabilities:   []string{"compilation", "static_analysis", "testing"},
		Metadata: map[string]interface{}{
			"concurrent": true,
			"compiled":   true,
		},
	}

	if info.Name != "go" {
		t.Errorf("Name mismatch: got %v, want %v", info.Name, "go")
	}
	if len(info.LSPServers) != 1 {
		t.Errorf("Expected 1 LSP server, got %d", len(info.LSPServers))
	}
	if len(info.Capabilities) != 3 {
		t.Errorf("Expected 3 capabilities, got %d", len(info.Capabilities))
	}

	jsonData, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var decoded types.LanguageInfo
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if decoded.DisplayName != info.DisplayName {
		t.Errorf("DisplayName mismatch after JSON roundtrip: got %v, want %v", decoded.DisplayName, info.DisplayName)
	}
}

func TestMavenInfo_Structure(t *testing.T) {
	maven := types.MavenInfo{
		GroupId:    "com.example",
		ArtifactId: "my-app",
		Version:    "1.0.0",
		Packaging:  "jar",
		Dependencies: map[string]string{
			"org.springframework:spring-boot-starter": "2.7.0",
		},
		Plugins: []string{"maven-compiler-plugin", "maven-surefire-plugin"},
	}

	if maven.GroupId != "com.example" {
		t.Errorf("GroupId mismatch: got %v, want %v", maven.GroupId, "com.example")
	}
	if len(maven.Plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(maven.Plugins))
	}

	jsonData, err := json.Marshal(maven)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var decoded types.MavenInfo
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if decoded.ArtifactId != maven.ArtifactId {
		t.Errorf("ArtifactId mismatch after JSON roundtrip: got %v, want %v", decoded.ArtifactId, maven.ArtifactId)
	}
}

func TestGradleInfo_Structure(t *testing.T) {
	gradle := types.GradleInfo{
		GroupId: "com.example",
		Version: "1.0.0",
		Dependencies: map[string]string{
			"org.springframework.boot:spring-boot-starter": "2.7.0",
		},
		Plugins: []string{"java", "spring-boot"},
		Tasks:   []string{"build", "test", "bootJar"},
	}

	if gradle.GroupId != "com.example" {
		t.Errorf("GroupId mismatch: got %v, want %v", gradle.GroupId, "com.example")
	}
	if len(gradle.Tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(gradle.Tasks))
	}

	jsonData, err := json.Marshal(gradle)
	if err != nil {
		t.Fatalf("JSON marshaling failed: %v", err)
	}

	var decoded types.GradleInfo
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("JSON unmarshaling failed: %v", err)
	}

	if decoded.Version != gradle.Version {
		t.Errorf("Version mismatch after JSON roundtrip: got %v, want %v", decoded.Version, gradle.Version)
	}
}