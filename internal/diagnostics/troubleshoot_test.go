package diagnostics

import (
	"lsp-gateway/internal/types"
	"testing"
)

func TestNewTroubleshootingGuide(t *testing.T) {
	guide := NewTroubleshootingGuide()
	if guide == nil {
		t.Fatal("NewTroubleshootingGuide returned nil")
	}

	commonIssues := guide.GetCommonIssues()
	if len(commonIssues) == 0 {
		t.Error("No common issues were initialized")
	}

	expectedIssues := []string{
		"runtime_not_found",
		"version_incompatible",
		"server_not_found",
		"config_invalid",
		"permission_denied",
		"network_unreachable",
		"python_pip_missing",
		"nodejs_npm_missing",
		"java_home_missing",
		"lsp_communication_failed",
		"path_misconfigured",
		"installation_corrupted",
	}

	issueMap := make(map[string]bool)
	for _, issue := range commonIssues {
		issueMap[issue.ID] = true
	}

	for _, expectedID := range expectedIssues {
		if !issueMap[expectedID] {
			t.Errorf("Expected issue '%s' not found in common issues", expectedID)
		}
	}
}

func TestAnalyzeIssue(t *testing.T) {
	guide := NewTroubleshootingGuide()

	issue := types.Issue{
		Severity:    types.IssueSeverityHigh,
		Category:    types.IssueCategoryExecution,
		Title:       "Go Runtime Not Found",
		Description: "go: command not found when trying to execute Go programs",
		Solution:    "Install Go runtime using package manager",
		Details:     map[string]interface{}{"component": "go"},
	}

	result, err := guide.AnalyzeIssue(issue)
	if err != nil {
		t.Fatalf("AnalyzeIssue failed: %v", err)
	}

	if result == nil {
		t.Fatal("AnalyzeIssue returned nil result")
	}

	if len(result.PossibleCauses) == 0 {
		t.Error("No possible causes identified")
	}

	if len(result.Solutions) == 0 {
		t.Error("No solutions generated")
	}

	if result.EstimatedTime == "" {
		t.Error("No estimated time provided")
	}
}

func TestGetRecommendations(t *testing.T) {
	guide := NewTroubleshootingGuide()

	healthReport := &HealthReport{
		Issues: []DiagnosticIssue{
			{
				Issue: types.Issue{
					Severity:    types.IssueSeverityHigh,
					Category:    types.IssueCategoryConfiguration,
					Title:       "Config Missing",
					Description: "Configuration file not found",
					Solution:    "Generate configuration file",
				},
				Component: "config",
			},
		},
	}

	recommendations, err := guide.GetRecommendations(healthReport)
	if err != nil {
		t.Fatalf("GetRecommendations failed: %v", err)
	}

	if len(recommendations) == 0 {
		t.Error("No recommendations generated")
	}

	for _, rec := range recommendations {
		if rec.Priority == RecommendationPriorityCritical {
			break
		}
	}
}

func TestAutoFix(t *testing.T) {
	guide := NewTroubleshootingGuide()

	issue := types.Issue{
		Severity:    types.IssueSeverityMedium,
		Category:    types.IssueCategoryConfiguration,
		Title:       "Config Missing",
		Description: "Configuration file not found",
		Solution:    "Generate configuration file",
		Details:     map[string]interface{}{"component": "config"},
	}

	result, err := guide.AutoFix(issue)
	if err != nil {
		t.Fatalf("AutoFix failed: %v", err)
	}

	if result == nil {
		t.Fatal("AutoFix returned nil result")
	}

	if !result.Success && len(result.Applied) == 0 && len(result.Skipped) == 0 {
		t.Error("AutoFix should have attempted to fix or provide skip reason")
	}

	if result.Duration == "" {
		t.Error("No duration provided for auto-fix")
	}
}

func TestGetCommonIssues(t *testing.T) {
	guide := NewTroubleshootingGuide()

	issues := guide.GetCommonIssues()
	if len(issues) == 0 {
		t.Error("No common issues returned")
	}

	for _, issue := range issues {
		if issue.ID == "" {
			t.Error("Issue missing ID")
		}
		if issue.Title == "" {
			t.Error("Issue missing Title")
		}
		if issue.Description == "" {
			t.Error("Issue missing Description")
		}
		if len(issue.Solutions) == 0 {
			t.Error("Issue missing Solutions")
		}
	}
}

func TestIssueMatching(t *testing.T) {
	guide := NewTroubleshootingGuide()

	issue := types.Issue{
		Severity:    types.IssueSeverityHigh,
		Category:    types.IssueCategoryExecution,
		Title:       "Runtime Not Found",
		Description: "go: command not found when trying to execute Go programs",
		Details:     map[string]interface{}{"component": "go"},
	}

	commonIssue, found := guide.findMatchingCommonIssue(issue)
	if !found {
		t.Error("Should have found matching common issue")
	}

	if commonIssue.ID != "runtime_not_found" {
		t.Errorf("Expected runtime_not_found, got %s", commonIssue.ID)
	}
}
