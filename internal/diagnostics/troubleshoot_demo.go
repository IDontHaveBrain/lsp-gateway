package diagnostics

import (
	"fmt"
	"lsp-gateway/internal/types"
	"time"
)

func DemoTroubleshootingGuide() {
	fmt.Println("=== LSP Gateway Troubleshooting Guide Demo ===")
	fmt.Println()

	guide := NewTroubleshootingGuide()

	fmt.Println("1. Common Issues Database:")
	commonIssues := guide.GetCommonIssues()
	fmt.Printf("   - Loaded %d common issues\n", len(commonIssues))

	for i, issue := range commonIssues {
		if i < 3 { // Show first 3 for brevity
			fmt.Printf("   - %s (%s)\n", issue.Title, issue.Category)
		}
	}
	fmt.Println("   ... and more")
	fmt.Println()

	fmt.Println("2. Issue Analysis Example:")
	sampleIssue := types.Issue{
		Severity:    types.IssueSeverityHigh,
		Category:    types.IssueCategoryExecution,
		Title:       "Go Runtime Not Found",
		Description: "go: command not found when trying to execute Go programs",
		Solution:    "Install Go runtime using package manager",
		Details:     map[string]interface{}{"component": "go"},
	}

	fmt.Printf("   Analyzing issue: %s\n", sampleIssue.Title)
	result, err := guide.AnalyzeIssue(sampleIssue)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   - Possible causes: %d\n", len(result.PossibleCauses))
	for _, cause := range result.PossibleCauses {
		fmt.Printf("     • %s\n", cause)
	}

	fmt.Printf("   - Solutions available: %d\n", len(result.Solutions))
	fmt.Printf("   - Auto-fixable: %v\n", result.AutoFixable)
	fmt.Printf("   - Estimated time: %s\n\n", result.EstimatedTime)

	fmt.Println("3. Auto-Fix Demo:")
	fmt.Printf("   Attempting auto-fix for: %s\n", sampleIssue.Title)

	autoFixResult, err := guide.AutoFix(sampleIssue)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   - Success: %v\n", autoFixResult.Success)
	fmt.Printf("   - Applied fixes: %d\n", len(autoFixResult.Applied))
	for _, fix := range autoFixResult.Applied {
		fmt.Printf("     • %s\n", fix)
	}
	fmt.Printf("   - Next steps: %d\n", len(autoFixResult.NextSteps))
	fmt.Printf("   - Duration: %s\n\n", autoFixResult.Duration)

	fmt.Println("4. Health Report Recommendations:")
	healthReport := &HealthReport{
		Timestamp: time.Now(),
		Overall:   HealthStatusDegraded,
		Issues: []DiagnosticIssue{
			{
				Issue: types.Issue{
					Severity:    types.IssueSeverityCritical,
					Category:    types.IssueCategoryExecution,
					Title:       "Runtime Missing",
					Description: "Critical runtime not installed",
				},
				Component: "runtime",
			},
			{
				Issue: types.Issue{
					Severity:    types.IssueSeverityMedium,
					Category:    types.IssueCategoryConfiguration,
					Title:       "Config Warning",
					Description: "Configuration file has warnings",
				},
				Component: "config",
			},
		},
	}

	recommendations, err := guide.GetRecommendations(healthReport)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return
	}

	fmt.Printf("   Generated %d recommendations:\n", len(recommendations))
	for i, rec := range recommendations {
		if i < 3 { // Show first 3
			fmt.Printf("   %d. %s (%s priority)\n", i+1, rec.Title, rec.Priority)
			fmt.Printf("      Description: %s\n", rec.Description)
			if len(rec.Commands) > 0 {
				fmt.Printf("      Command: %s\n", rec.Commands[0])
			}
		}
	}

	fmt.Println("\n5. Troubleshooting Statistics:")
	demoSystemStats(guide)

	fmt.Println("\n=== Demo Complete ===")
}

func demoSystemStats(guide *DefaultTroubleshootingGuide) {
	commonIssues := guide.GetCommonIssues()

	categoryCount := make(map[types.IssueCategory]int)
	autoFixableCount := 0

	for _, issue := range commonIssues {
		categoryCount[issue.Category]++
		if issue.AutoFixable {
			autoFixableCount++
		}
	}

	stats := map[string]interface{}{
		"total_common_issues": len(commonIssues),
		"auto_fixable_issues": autoFixableCount,
		"categories": map[string]int{
			"installation":  categoryCount[types.IssueCategoryInstallation],
			"version":       categoryCount[types.IssueCategoryVersion],
			"path":          categoryCount[types.IssueCategoryPath],
			"environment":   categoryCount[types.IssueCategoryEnvironment],
			"permissions":   categoryCount[types.IssueCategoryPermissions],
			"dependencies":  categoryCount[types.IssueCategoryDependencies],
			"configuration": categoryCount[types.IssueCategoryConfiguration],
			"corruption":    categoryCount[types.IssueCategoryCorruption],
			"execution":     categoryCount[types.IssueCategoryExecution],
		},
	}

	fmt.Printf("   - Total issues in database: %v\n", stats["total_common_issues"])
	fmt.Printf("   - Auto-fixable issues: %v\n", stats["auto_fixable_issues"])
	fmt.Printf("   - Coverage by category:\n")

	categories := stats["categories"].(map[string]int)
	for category, count := range categories {
		if count > 0 {
			fmt.Printf("     • %s: %d issues\n", category, count)
		}
	}
}
