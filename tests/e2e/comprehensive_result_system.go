package e2e_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ComprehensiveResultSystem integrates all result management components
type ComprehensiveResultSystem struct {
	mu                   sync.RWMutex
	config               *SystemConfig
	logger               *log.Logger
	
	// Core components
	persistenceManager   *ResultPersistenceManager
	historicalTracker    *HistoricalTracker
	reportingSystem      *ReportingSystem
	
	// Enhanced formatters
	formatters           map[ReportFormat]ReportFormatter
	
	// Integration state
	ctx                  context.Context
	cancel               context.CancelFunc
	startTime            time.Time
	sessionActive        bool
	
	// Performance monitoring
	metricsCollector     *SystemMetricsCollector
	
	// Cleanup management
	cleanupFunctions     []func() error
}

// SystemConfig defines comprehensive configuration for the result system
type SystemConfig struct {
	// Storage configuration
	StorageDirectory        string
	DatabasePath            string
	EnableCompression       bool
	CompressionLevel        int
	
	// Historical tracking configuration
	EnableHistoricalTracking bool
	AnalysisWindowDays      int
	TrendAnalysisWindow     int
	RegressionThreshold     float64
	BaselineUpdateInterval  time.Duration
	
	// Reporting configuration
	OutputDirectory         string
	EnableRealtimeReporting bool
	ReportFormats          []ReportFormat
	MetricsCollectionInterval time.Duration
	ProgressReportingInterval time.Duration
	DetailLevel            DetailLevel
	
	// Performance thresholds
	PerformanceThresholds  *PerformanceThresholds
	
	// Retention and archival
	RetentionDays          int
	ArchivalDays           int
	EnableArchival         bool
	
	// Caching configuration
	CacheSize              int
	CacheTTL               time.Duration
	
	// Integration features
	EnableBackgroundAnalysis bool
	AnalysisInterval        time.Duration
	AlertingEnabled         bool
	
	// Advanced formatting options
	PDFConfig              *PDFConfig
	MarkdownConfig         *MarkdownConfig
	TSVConfig              *TSVConfig
	XMLConfig              *XMLConfig
}

// SystemMetricsCollector collects system-wide metrics
type SystemMetricsCollector struct {
	mu                     sync.RWMutex
	enabled                bool
	collectionInterval     time.Duration
	lastCollection         time.Time
	
	// Collected metrics
	systemMetrics          *SystemResourceMetrics
	persistenceMetrics     *PersistenceMetrics
	reportingMetrics       *ReportingPerformanceMetrics
	historicalMetrics      *HistoricalAnalysisMetrics
	
	// Performance tracking
	operationDurations     map[string][]time.Duration
	errorCounts            map[string]int64
	successCounts          map[string]int64
}

// PersistenceMetrics tracks persistence system performance
type PersistenceMetrics struct {
	TotalRecordsStored     int64
	TotalRecordsRetrieved  int64
	TotalSearchesPerformed int64
	AverageStorageTime     time.Duration
	AverageRetrievalTime   time.Duration
	AverageSearchTime      time.Duration
	StorageErrors          int64
	RetrievalErrors        int64
	SearchErrors           int64
	DatabaseSize           int64
	FileSystemUsage        int64
	CompressionRatio       float64
	CacheHitRate          float64
	IndexingTime          time.Duration
	MaintenanceTime       time.Duration
	BackupTime            time.Duration
}

// ReportingPerformanceMetrics tracks reporting system performance
type ReportingPerformanceMetrics struct {
	ReportsGenerated      int64
	ReportsByFormat       map[ReportFormat]int64
	AverageGenerationTime time.Duration
	GenerationTimeByFormat map[ReportFormat]time.Duration
	GenerationErrors      int64
	ErrorsByFormat        map[ReportFormat]int64
	TotalReportSize       int64
	ReportSizeByFormat    map[ReportFormat]int64
	FormattingTime        time.Duration
	OutputTime            time.Duration
	CompressionTime       time.Duration
}

// HistoricalAnalysisMetrics tracks historical analysis performance
type HistoricalAnalysisMetrics struct {
	TrendAnalysesPerformed     int64
	RegressionsDetected        int64
	BaselinesUpdated          int64
	AnomaliesDetected         int64
	AverageAnalysisTime       time.Duration
	AnalysisTimeByType        map[string]time.Duration
	AnalysisErrors            int64
	ErrorsByAnalysisType      map[string]int64
	CacheHitRate             float64
	ModelAccuracy            float64
	PredictionAccuracy       float64
	FalsePositiveRate        float64
	BackgroundTasksCompleted int64
	BackgroundTasksFailed    int64
}

// SessionSummary provides a summary of a testing session
type SessionSummary struct {
	SessionID             string
	StartTime             time.Time
	EndTime               time.Time
	Duration              time.Duration
	TotalTests            int64
	PassedTests           int64
	FailedTests           int64
	SkippedTests          int64
	SuccessRate           float64
	TestsPerSecond        float64
	
	// Storage metrics
	RecordsStored         int64
	StorageTime           time.Duration
	CompressionRatio      float64
	
	// Analysis metrics
	TrendAnalyses         int64
	RegressionsDetected   int64
	AnomaliesFound        int64
	BaselinesUpdated      int64
	
	// Reporting metrics
	ReportsGenerated      int64
	ReportingTime         time.Duration
	ReportFormats         []ReportFormat
	
	// System metrics
	PeakMemoryUsage       float64
	TotalGCTime           time.Duration
	MaxGoroutines         int
	
	// Quality metrics
	OverallQualityScore   float64
	PerformanceScore      float64
	CoverageScore         float64
	
	// Issues and recommendations
	CriticalIssues        []string
	Recommendations       []string
}

// NewComprehensiveResultSystem creates a new comprehensive result system
func NewComprehensiveResultSystem(config *SystemConfig) (*ComprehensiveResultSystem, error) {
	if config == nil {
		config = getDefaultSystemConfig()
	}

	if err := validateSystemConfig(config); err != nil {
		return nil, fmt.Errorf("invalid system configuration: %w", err)
	}

	logger := log.New(os.Stdout, "[E2E-ComprehensiveSystem] ", log.LstdFlags|log.Lshortfile)
	ctx, cancel := context.WithCancel(context.Background())

	// Create directories
	dirs := []string{
		config.StorageDirectory,
		config.OutputDirectory,
		filepath.Join(config.StorageDirectory, "historical"),
		filepath.Join(config.StorageDirectory, "archive"),
		filepath.Join(config.OutputDirectory, "reports"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	system := &ComprehensiveResultSystem{
		config:           config,
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
		startTime:        time.Now(),
		formatters:       make(map[ReportFormat]ReportFormatter),
		cleanupFunctions: make([]func() error, 0),
	}

	// Initialize components
	if err := system.initializeComponents(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	// Initialize metrics collector
	system.metricsCollector = &SystemMetricsCollector{
		enabled:            true,
		collectionInterval: config.MetricsCollectionInterval,
		operationDurations: make(map[string][]time.Duration),
		errorCounts:        make(map[string]int64),
		successCounts:      make(map[string]int64),
		persistenceMetrics: &PersistenceMetrics{
			ReportsByFormat:       make(map[ReportFormat]int64),
			GenerationTimeByFormat: make(map[ReportFormat]time.Duration),
			ErrorsByFormat:        make(map[ReportFormat]int64),
			ReportSizeByFormat:    make(map[ReportFormat]int64),
		},
		reportingMetrics: &ReportingPerformanceMetrics{
			ReportsByFormat:       make(map[ReportFormat]int64),
			GenerationTimeByFormat: make(map[ReportFormat]time.Duration),
			ErrorsByFormat:        make(map[ReportFormat]int64),
			ReportSizeByFormat:    make(map[ReportFormat]int64),
		},
		historicalMetrics: &HistoricalAnalysisMetrics{
			AnalysisTimeByType:   make(map[string]time.Duration),
			ErrorsByAnalysisType: make(map[string]int64),
		},
	}

	// Start background services
	go system.startBackgroundServices()

	logger.Printf("Comprehensive result system initialized")
	return system, nil
}

// initializeComponents initializes all system components
func (crs *ComprehensiveResultSystem) initializeComponents() error {
	// Initialize persistence manager
	persistenceConfig := &PersistenceConfig{
		StorageDirectory:  crs.config.StorageDirectory,
		DatabasePath:      crs.config.DatabasePath,
		EnableCompression: crs.config.EnableCompression,
		CompressionLevel:  crs.config.CompressionLevel,
		EnableArchival:    crs.config.EnableArchival,
		ArchivalDays:      crs.config.ArchivalDays,
		EnableSearch:      true,
		BatchSize:         10,
		RetentionDays:     crs.config.RetentionDays,
		BackupEnabled:     true,
		BackupInterval:    24 * time.Hour,
		IndexingEnabled:   true,
	}

	var err error
	crs.persistenceManager, err = NewResultPersistenceManager(persistenceConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize persistence manager: %w", err)
	}

	// Initialize historical tracker
	if crs.config.EnableHistoricalTracking {
		historicalConfig := &HistoricalConfig{
			AnalysisWindowDays:      crs.config.AnalysisWindowDays,
			TrendAnalysisWindow:     crs.config.TrendAnalysisWindow,
			RegressionThreshold:     crs.config.RegressionThreshold,
			BaselineUpdateInterval:  crs.config.BaselineUpdateInterval,
			CacheSize:              crs.config.CacheSize,
			CacheTTL:               crs.config.CacheTTL,
			EnableBackgroundAnalysis: crs.config.EnableBackgroundAnalysis,
			AnalysisInterval:        crs.config.AnalysisInterval,
			AlertingEnabled:         crs.config.AlertingEnabled,
		}

		crs.historicalTracker, err = NewHistoricalTracker(historicalConfig, crs.persistenceManager)
		if err != nil {
			return fmt.Errorf("failed to initialize historical tracker: %w", err)
		}
	}

	// Initialize reporting system
	reportingConfig := &ReportingConfig{
		OutputDirectory:           crs.config.OutputDirectory,
		EnableRealtimeReporting:   crs.config.EnableRealtimeReporting,
		EnableHistoricalTracking:  crs.config.EnableHistoricalTracking,
		ReportFormats:            crs.config.ReportFormats,
		MetricsCollectionInterval: crs.config.MetricsCollectionInterval,
		ProgressReportingInterval: crs.config.ProgressReportingInterval,
		RetentionDays:            crs.config.RetentionDays,
		DetailLevel:              crs.config.DetailLevel,
		PerformanceThresholds:    crs.config.PerformanceThresholds,
	}

	crs.reportingSystem, err = NewReportingSystem(reportingConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize reporting system: %w", err)
	}

	// Initialize enhanced formatters
	crs.initializeEnhancedFormatters()

	return nil
}

// initializeEnhancedFormatters initializes all available formatters
func (crs *ComprehensiveResultSystem) initializeEnhancedFormatters() {
	// Standard formatters
	crs.formatters[FormatConsole] = &ConsoleFormatter{}
	crs.formatters[FormatJSON] = &JSONFormatter{}
	crs.formatters[FormatHTML] = &HTMLFormatter{}
	crs.formatters[FormatCSV] = &CSVFormatter{}

	// Enhanced formatters
	crs.formatters["pdf"] = NewPDFFormatter(crs.config.PDFConfig)
	crs.formatters["markdown"] = NewMarkdownFormatter(crs.config.MarkdownConfig)
	crs.formatters["tsv"] = NewTSVFormatter(crs.config.TSVConfig)
	crs.formatters[FormatXML] = NewXMLFormatter(crs.config.XMLConfig)
}

// StartSession begins a new comprehensive testing session
func (crs *ComprehensiveResultSystem) StartSession(sessionID string, testCount int64) error {
	crs.mu.Lock()
	defer crs.mu.Unlock()

	if crs.sessionActive {
		return fmt.Errorf("session already active")
	}

	crs.startTime = time.Now()
	crs.sessionActive = true

	// Start reporting session
	if err := crs.reportingSystem.StartSession(testCount); err != nil {
		return fmt.Errorf("failed to start reporting session: %w", err)
	}

	// Initialize metrics collection
	crs.metricsCollector.lastCollection = time.Now()

	crs.logger.Printf("Started comprehensive testing session: %s with %d tests", sessionID, testCount)
	return nil
}

// RecordTestResult records a comprehensive test result
func (crs *ComprehensiveResultSystem) RecordTestResult(result *TestResult) error {
	startTime := time.Now()

	// Record in reporting system
	crs.reportingSystem.RecordTestResult(result)

	// Create test run record for persistence
	testRunRecord := &TestRunRecord{
		RunID:           fmt.Sprintf("run_%d", time.Now().Unix()),
		Timestamp:       result.StartTime,
		Duration:        result.Duration,
		Success:         result.Success,
		TotalTests:      1,
		PassedTests:     0,
		FailedTests:     0,
		SkippedTests:    0,
		TestResults:     []*TestResult{result},
		Environment:     crs.extractEnvironment(result),
		Notes:           crs.generateResultNotes(result),
		Tags:            crs.extractTags(result),
		Metadata:        crs.extractMetadata(result),
	}

	if result.Success {
		testRunRecord.PassedTests = 1
	} else {
		testRunRecord.FailedTests = 1
	}

	// Store in persistence manager
	if err := crs.persistenceManager.StoreTestRun(testRunRecord); err != nil {
		crs.metricsCollector.errorCounts["storage"]++
		return fmt.Errorf("failed to store test result: %w", err)
	}

	// Record metrics
	duration := time.Since(startTime)
	crs.metricsCollector.operationDurations["record_result"] = append(
		crs.metricsCollector.operationDurations["record_result"], duration)
	crs.metricsCollector.successCounts["record_result"]++

	return nil
}

// RecordMCPMetrics records MCP client metrics
func (crs *ComprehensiveResultSystem) RecordMCPMetrics(mockClient interface{}) {
	// Cast to appropriate type and record metrics
	crs.reportingSystem.RecordMCPMetrics(mockClient)
}

// RecordSystemMetrics records system resource metrics
func (crs *ComprehensiveResultSystem) RecordSystemMetrics() {
	crs.reportingSystem.RecordSystemMetrics()
	
	// Update our metrics collector
	crs.metricsCollector.mu.Lock()
	crs.metricsCollector.lastCollection = time.Now()
	crs.metricsCollector.mu.Unlock()
}

// PerformHistoricalAnalysis performs comprehensive historical analysis
func (crs *ComprehensiveResultSystem) PerformHistoricalAnalysis(testName, scenario string) (*ComprehensiveAnalysisResult, error) {
	if crs.historicalTracker == nil {
		return nil, fmt.Errorf("historical tracking not enabled")
	}

	startTime := time.Now()

	result := &ComprehensiveAnalysisResult{
		AnalysisID:   fmt.Sprintf("analysis_%d", time.Now().Unix()),
		TestName:     testName,
		Scenario:     scenario,
		AnalysisTime: startTime,
	}

	// Perform trend analysis
	trendAnalysis, err := crs.historicalTracker.AnalyzeHistoricalTrends(testName, scenario, crs.config.AnalysisWindowDays)
	if err != nil {
		crs.logger.Printf("Warning: Failed to perform trend analysis: %v", err)
	} else {
		result.TrendAnalysis = trendAnalysis
	}

	// Detect regressions
	regressions, err := crs.historicalTracker.DetectRegressions(testName, scenario)
	if err != nil {
		crs.logger.Printf("Warning: Failed to detect regressions: %v", err)
	} else {
		result.Regressions = regressions
	}

	// Get performance profile
	profile, err := crs.historicalTracker.GetPerformanceProfile(testName, scenario, "")
	if err != nil {
		crs.logger.Printf("Warning: Failed to get performance profile: %v", err)
	} else {
		result.PerformanceProfile = profile
	}

	// Calculate analysis duration
	result.AnalysisDuration = time.Since(startTime)

	// Record metrics
	crs.metricsCollector.historicalMetrics.TrendAnalysesPerformed++
	crs.metricsCollector.historicalMetrics.RegressionsDetected += int64(len(regressions))
	crs.metricsCollector.historicalMetrics.AverageAnalysisTime = result.AnalysisDuration

	return result, nil
}

// GenerateComprehensiveReport generates a comprehensive report with all available data
func (crs *ComprehensiveResultSystem) GenerateComprehensiveReport() (*ComprehensiveReportResult, error) {
	startTime := time.Now()

	// Generate base report
	baseReport, err := crs.reportingSystem.GenerateReport()
	if err != nil {
		return nil, fmt.Errorf("failed to generate base report: %w", err)
	}

	// Enhance with historical data if available
	if crs.historicalTracker != nil {
		crs.enhanceReportWithHistoricalData(baseReport)
	}

	// Generate reports in all configured formats
	reportResults := make(map[ReportFormat]*ReportOutput)
	var totalReportSize int64

	for format := range crs.formatters {
		formatter := crs.formatters[format]
		
		formatStartTime := time.Now()
		data, err := formatter.Format(baseReport)
		formatDuration := time.Since(formatStartTime)

		if err != nil {
			crs.logger.Printf("Warning: Failed to format report as %s: %v", format, err)
			crs.metricsCollector.reportingMetrics.ErrorsByFormat[format]++
			continue
		}

		// Save report to file
		filename := fmt.Sprintf("comprehensive-report-%d.%s", time.Now().Unix(), formatter.Extension())
		filepath := filepath.Join(crs.config.OutputDirectory, "reports", filename)

		if err := os.WriteFile(filepath, data, 0644); err != nil {
			crs.logger.Printf("Warning: Failed to save %s report: %v", format, err)
			continue
		}

		reportOutput := &ReportOutput{
			Format:       format,
			FilePath:     filepath,
			FileSize:     int64(len(data)),
			GeneratedAt:  time.Now(),
			GenerationTime: formatDuration,
			ContentType:  formatter.ContentType(),
		}

		reportResults[format] = reportOutput
		totalReportSize += reportOutput.FileSize

		// Record metrics
		crs.metricsCollector.reportingMetrics.ReportsByFormat[format]++
		crs.metricsCollector.reportingMetrics.GenerationTimeByFormat[format] = formatDuration
		crs.metricsCollector.reportingMetrics.ReportSizeByFormat[format] += reportOutput.FileSize
	}

	totalDuration := time.Since(startTime)

	// Update metrics
	crs.metricsCollector.reportingMetrics.ReportsGenerated++
	crs.metricsCollector.reportingMetrics.AverageGenerationTime = totalDuration
	crs.metricsCollector.reportingMetrics.TotalReportSize += totalReportSize

	result := &ComprehensiveReportResult{
		BaseReport:        baseReport,
		ReportOutputs:     reportResults,
		GenerationTime:    totalDuration,
		TotalSize:         totalReportSize,
		FormatsGenerated:  len(reportResults),
		GeneratedAt:       time.Now(),
	}

	crs.logger.Printf("Generated comprehensive report in %d formats, total size: %d bytes, duration: %v", 
		len(reportResults), totalReportSize, totalDuration)

	return result, nil
}

// EndSession ends the current testing session and generates final reports
func (crs *ComprehensiveResultSystem) EndSession() (*SessionSummary, error) {
	crs.mu.Lock()
	defer crs.mu.Unlock()

	if !crs.sessionActive {
		return nil, fmt.Errorf("no active session")
	}

	endTime := time.Now()
	sessionDuration := endTime.Sub(crs.startTime)

	// Flush any pending data
	if err := crs.persistenceManager.FlushWriteBuffer(); err != nil {
		crs.logger.Printf("Warning: Failed to flush write buffer: %v", err)
	}

	// Generate final comprehensive report
	reportResult, err := crs.GenerateComprehensiveReport()
	if err != nil {
		crs.logger.Printf("Warning: Failed to generate final report: %v", err)
	}

	// Create session summary
	summary := &SessionSummary{
		SessionID:           fmt.Sprintf("session_%d", crs.startTime.Unix()),
		StartTime:           crs.startTime,
		EndTime:             endTime,
		Duration:            sessionDuration,
		PeakMemoryUsage:     crs.getSystemMetric("peak_memory", 0),
		MaxGoroutines:       int(crs.getSystemMetric("max_goroutines", 0)),
		OverallQualityScore: crs.calculateQualityScore(reportResult),
		PerformanceScore:    crs.calculatePerformanceScore(reportResult),
		CoverageScore:       crs.calculateCoverageScore(reportResult),
	}

	// Populate test metrics from reporting system
	if reportResult != nil && reportResult.BaseReport != nil && reportResult.BaseReport.TestMetrics != nil {
		metrics := reportResult.BaseReport.TestMetrics
		summary.TotalTests = metrics.TotalTests
		summary.PassedTests = metrics.PassedTests
		summary.FailedTests = metrics.FailedTests
		summary.SkippedTests = metrics.SkippedTests
		summary.SuccessRate = metrics.SuccessRate
		summary.TestsPerSecond = metrics.TestsPerSecond
	}

	// Populate storage metrics
	summary.RecordsStored = crs.metricsCollector.persistenceMetrics.TotalRecordsStored
	summary.StorageTime = crs.metricsCollector.persistenceMetrics.AverageStorageTime
	summary.CompressionRatio = crs.metricsCollector.persistenceMetrics.CompressionRatio

	// Populate analysis metrics
	if crs.historicalTracker != nil {
		summary.TrendAnalyses = crs.metricsCollector.historicalMetrics.TrendAnalysesPerformed
		summary.RegressionsDetected = crs.metricsCollector.historicalMetrics.RegressionsDetected
		summary.AnomaliesFound = crs.metricsCollector.historicalMetrics.AnomaliesDetected
		summary.BaselinesUpdated = crs.metricsCollector.historicalMetrics.BaselinesUpdated
	}

	// Populate reporting metrics
	if reportResult != nil {
		summary.ReportsGenerated = int64(reportResult.FormatsGenerated)
		summary.ReportingTime = reportResult.GenerationTime
		summary.ReportFormats = make([]ReportFormat, 0, len(reportResult.ReportOutputs))
		for format := range reportResult.ReportOutputs {
			summary.ReportFormats = append(summary.ReportFormats, format)
		}
	}

	// Extract issues and recommendations
	if reportResult != nil && reportResult.BaseReport != nil {
		if reportResult.BaseReport.ExecutiveSummary != nil {
			summary.CriticalIssues = reportResult.BaseReport.ExecutiveSummary.CriticalIssues
			summary.Recommendations = reportResult.BaseReport.ExecutiveSummary.RecommendedActions
		}
	}

	crs.sessionActive = false

	crs.logger.Printf("Session ended. Duration: %v, Tests: %d, Success Rate: %.2f%%", 
		sessionDuration, summary.TotalTests, summary.SuccessRate)

	return summary, nil
}

// SearchHistoricalResults searches historical test results
func (crs *ComprehensiveResultSystem) SearchHistoricalResults(query *SearchQuery) (*SearchResult, error) {
	if crs.persistenceManager == nil {
		return nil, fmt.Errorf("persistence manager not available")
	}

	startTime := time.Now()
	result, err := crs.persistenceManager.SearchTestRuns(query)
	searchDuration := time.Since(startTime)

	// Record metrics
	crs.metricsCollector.persistenceMetrics.TotalSearchesPerformed++
	crs.metricsCollector.persistenceMetrics.AverageSearchTime = searchDuration

	if err != nil {
		crs.metricsCollector.persistenceMetrics.SearchErrors++
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return result, nil
}

// GetSystemHealth returns comprehensive system health information
func (crs *ComprehensiveResultSystem) GetSystemHealth() *SystemHealthReport {
	health := &SystemHealthReport{
		Timestamp:       time.Now(),
		OverallStatus:   "healthy",
		Components:      make(map[string]*ComponentHealth),
		Metrics:         crs.getSystemMetrics(),
		Recommendations: make([]string, 0),
	}

	// Check persistence manager health
	persistenceHealth := &ComponentHealth{Component: "PersistenceManager", Status: "healthy"}
	if crs.persistenceManager != nil {
		// Add persistence-specific health checks
		persistenceHealth.Metrics = map[string]float64{
			"records_stored": float64(crs.metricsCollector.persistenceMetrics.TotalRecordsStored),
			"storage_errors": float64(crs.metricsCollector.persistenceMetrics.StorageErrors),
			"cache_hit_rate": crs.metricsCollector.persistenceMetrics.CacheHitRate,
		}
		
		if crs.metricsCollector.persistenceMetrics.StorageErrors > 10 {
			persistenceHealth.Status = "degraded"
			persistenceHealth.Issues = append(persistenceHealth.Issues, "High storage error rate")
		}
	} else {
		persistenceHealth.Status = "unavailable"
		persistenceHealth.Issues = append(persistenceHealth.Issues, "Persistence manager not initialized")
	}
	health.Components["persistence"] = persistenceHealth

	// Check historical tracker health
	historicalHealth := &ComponentHealth{Component: "HistoricalTracker", Status: "healthy"}
	if crs.historicalTracker != nil {
		historicalHealth.Metrics = map[string]float64{
			"analyses_performed": float64(crs.metricsCollector.historicalMetrics.TrendAnalysesPerformed),
			"regressions_detected": float64(crs.metricsCollector.historicalMetrics.RegressionsDetected),
			"model_accuracy": crs.metricsCollector.historicalMetrics.ModelAccuracy,
		}
		
		if crs.metricsCollector.historicalMetrics.AnalysisErrors > 5 {
			historicalHealth.Status = "degraded"
			historicalHealth.Issues = append(historicalHealth.Issues, "Analysis errors detected")
		}
	} else {
		historicalHealth.Status = "disabled"
		historicalHealth.Issues = append(historicalHealth.Issues, "Historical tracking disabled")
	}
	health.Components["historical"] = historicalHealth

	// Check reporting system health
	reportingHealth := &ComponentHealth{Component: "ReportingSystem", Status: "healthy"}
	if crs.reportingSystem != nil {
		reportingHealth.Metrics = map[string]float64{
			"reports_generated": float64(crs.metricsCollector.reportingMetrics.ReportsGenerated),
			"generation_errors": float64(crs.metricsCollector.reportingMetrics.GenerationErrors),
			"average_generation_time": float64(crs.metricsCollector.reportingMetrics.AverageGenerationTime.Milliseconds()),
		}
		
		if crs.metricsCollector.reportingMetrics.GenerationErrors > 3 {
			reportingHealth.Status = "degraded"
			reportingHealth.Issues = append(reportingHealth.Issues, "Report generation errors")
		}
	}
	health.Components["reporting"] = reportingHealth

	// Determine overall status
	degradedCount := 0
	unavailableCount := 0
	for _, component := range health.Components {
		switch component.Status {
		case "degraded":
			degradedCount++
		case "unavailable":
			unavailableCount++
		}
	}

	if unavailableCount > 0 {
		health.OverallStatus = "unhealthy"
	} else if degradedCount > 1 {
		health.OverallStatus = "degraded"
	}

	// Generate recommendations
	if health.OverallStatus != "healthy" {
		health.Recommendations = append(health.Recommendations, "Review component health issues")
		health.Recommendations = append(health.Recommendations, "Check system logs for detailed error information")
	}

	return health
}

// Helper methods

func (crs *ComprehensiveResultSystem) startBackgroundServices() {
	ticker := time.NewTicker(crs.config.MetricsCollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-crs.ctx.Done():
			return
		case <-ticker.C:
			crs.performBackgroundMaintenance()
		}
	}
}

func (crs *ComprehensiveResultSystem) performBackgroundMaintenance() {
	// Collect system metrics
	crs.RecordSystemMetrics()

	// Update performance metrics
	crs.updatePerformanceMetrics()

	// Perform cleanup if needed
	crs.performCleanup()
}

func (crs *ComprehensiveResultSystem) updatePerformanceMetrics() {
	// Update operation duration averages
	for operation, durations := range crs.metricsCollector.operationDurations {
		if len(durations) > 100 {
			// Keep only recent measurements
			crs.metricsCollector.operationDurations[operation] = durations[len(durations)-50:]
		}
	}
}

func (crs *ComprehensiveResultSystem) performCleanup() {
	// Placeholder for cleanup operations
	// In a real implementation, this would:
	// - Clean up old cache entries
	// - Archive old data
	// - Rotate logs
	// - Update statistics
}

func (crs *ComprehensiveResultSystem) extractEnvironment(result *TestResult) string {
	if result.Metadata != nil {
		if env, ok := result.Metadata["environment"]; ok {
			if envStr, ok := env.(string); ok {
				return envStr
			}
		}
	}
	return "default"
}

func (crs *ComprehensiveResultSystem) generateResultNotes(result *TestResult) string {
	notes := fmt.Sprintf("Test: %s, Scenario: %s, Language: %s", result.TestName, result.Scenario, result.Language)
	if len(result.Errors) > 0 {
		notes += fmt.Sprintf(", Errors: %d", len(result.Errors))
	}
	if len(result.Warnings) > 0 {
		notes += fmt.Sprintf(", Warnings: %d", len(result.Warnings))
	}
	return notes
}

func (crs *ComprehensiveResultSystem) extractTags(result *TestResult) []string {
	tags := []string{result.Scenario, result.Language}
	if result.Success {
		tags = append(tags, "success")
	} else {
		tags = append(tags, "failure")
	}
	if result.TimeoutOccurred {
		tags = append(tags, "timeout")
	}
	return tags
}

func (crs *ComprehensiveResultSystem) extractMetadata(result *TestResult) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["test_name"] = result.TestName
	metadata["scenario"] = result.Scenario
	metadata["language"] = result.Language
	metadata["duration_ms"] = result.Duration.Milliseconds()
	metadata["retry_count"] = result.RetryCount
	
	if result.Metrics != nil {
		metadata["request_count"] = result.Metrics.RequestCount
		metadata["throughput"] = result.Metrics.ThroughputPerSecond
		metadata["error_rate"] = result.Metrics.ErrorRate
	}
	
	return metadata
}

func (crs *ComprehensiveResultSystem) enhanceReportWithHistoricalData(report *ComprehensiveReport) {
	if crs.historicalTracker == nil {
		return
	}

	// Add historical comparison for key metrics
	// This would involve complex historical analysis
	// For now, we'll add a placeholder enhancement
	if report.RawData == nil {
		report.RawData = make(map[string]interface{})
	}
	report.RawData["historical_enhancement"] = "enabled"
}

func (crs *ComprehensiveResultSystem) getSystemMetric(metricName string, defaultValue float64) float64 {
	// Placeholder implementation
	// In a real system, this would retrieve actual system metrics
	return defaultValue
}

func (crs *ComprehensiveResultSystem) calculateQualityScore(reportResult *ComprehensiveReportResult) float64 {
	if reportResult == nil || reportResult.BaseReport == nil || reportResult.BaseReport.ExecutiveSummary == nil {
		return 0.0
	}
	return reportResult.BaseReport.ExecutiveSummary.QualityScore
}

func (crs *ComprehensiveResultSystem) calculatePerformanceScore(reportResult *ComprehensiveReportResult) float64 {
	if reportResult == nil || reportResult.BaseReport == nil || reportResult.BaseReport.ExecutiveSummary == nil {
		return 0.0
	}
	return reportResult.BaseReport.ExecutiveSummary.PerformanceScore
}

func (crs *ComprehensiveResultSystem) calculateCoverageScore(reportResult *ComprehensiveReportResult) float64 {
	if reportResult == nil || reportResult.BaseReport == nil || reportResult.BaseReport.ExecutiveSummary == nil {
		return 0.0
	}
	return reportResult.BaseReport.ExecutiveSummary.CoverageScore
}

func (crs *ComprehensiveResultSystem) getSystemMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})
	
	if crs.metricsCollector != nil {
		metrics["persistence"] = crs.metricsCollector.persistenceMetrics
		metrics["reporting"] = crs.metricsCollector.reportingMetrics
		metrics["historical"] = crs.metricsCollector.historicalMetrics
		metrics["operation_counts"] = map[string]interface{}{
			"successes": crs.metricsCollector.successCounts,
			"errors":    crs.metricsCollector.errorCounts,
		}
	}
	
	return metrics
}

// Cleanup performs comprehensive cleanup of all components
func (crs *ComprehensiveResultSystem) Cleanup() error {
	crs.mu.Lock()
	defer crs.mu.Unlock()

	crs.logger.Printf("Starting comprehensive system cleanup")

	// Cancel context
	if crs.cancel != nil {
		crs.cancel()
	}

	var errors []error

	// Cleanup persistence manager
	if crs.persistenceManager != nil {
		if err := crs.persistenceManager.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("persistence manager cleanup: %w", err))
		}
	}

	// Cleanup historical tracker
	if crs.historicalTracker != nil {
		if err := crs.historicalTracker.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("historical tracker cleanup: %w", err))
		}
	}

	// Cleanup reporting system
	if crs.reportingSystem != nil {
		if err := crs.reportingSystem.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("reporting system cleanup: %w", err))
		}
	}

	// Execute additional cleanup functions
	for _, cleanup := range crs.cleanupFunctions {
		if err := cleanup(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	crs.logger.Printf("Comprehensive system cleanup completed")
	return nil
}

// Supporting types for the comprehensive system

// ComprehensiveAnalysisResult contains results from comprehensive analysis
type ComprehensiveAnalysisResult struct {
	AnalysisID         string
	TestName           string
	Scenario           string
	AnalysisTime       time.Time
	AnalysisDuration   time.Duration
	TrendAnalysis      *HistoricalTrendAnalysis
	Regressions        []*RegressionEvent
	PerformanceProfile *PerformanceProfile
	Anomalies          []*AnomalyEvent
	Recommendations    []string
}

// ComprehensiveReportResult contains results from comprehensive report generation
type ComprehensiveReportResult struct {
	BaseReport        *ComprehensiveReport
	ReportOutputs     map[ReportFormat]*ReportOutput
	GenerationTime    time.Duration
	TotalSize         int64
	FormatsGenerated  int
	GeneratedAt       time.Time
}

// ReportOutput represents a generated report in a specific format
type ReportOutput struct {
	Format         ReportFormat
	FilePath       string
	FileSize       int64
	GeneratedAt    time.Time
	GenerationTime time.Duration
	ContentType    string
}

// SystemHealthReport provides comprehensive system health information
type SystemHealthReport struct {
	Timestamp       time.Time
	OverallStatus   string
	Components      map[string]*ComponentHealth
	Metrics         map[string]interface{}
	Recommendations []string
}

// ComponentHealth represents the health of a system component
type ComponentHealth struct {
	Component   string
	Status      string
	Issues      []string
	Metrics     map[string]float64
	LastChecked time.Time
}

// Configuration helper functions

func getDefaultSystemConfig() *SystemConfig {
	return &SystemConfig{
		StorageDirectory:        "./e2e-comprehensive-results",
		DatabasePath:            "",
		EnableCompression:       true,
		CompressionLevel:        6,
		EnableHistoricalTracking: true,
		AnalysisWindowDays:      30,
		TrendAnalysisWindow:     100,
		RegressionThreshold:     20.0,
		BaselineUpdateInterval:  24 * time.Hour,
		OutputDirectory:         "./e2e-comprehensive-reports",
		EnableRealtimeReporting: true,
		ReportFormats:          []ReportFormat{FormatConsole, FormatJSON, FormatHTML, "markdown", "pdf"},
		MetricsCollectionInterval: 30 * time.Second,
		ProgressReportingInterval: 10 * time.Second,
		DetailLevel:             DetailLevelStandard,
		RetentionDays:           365,
		ArchivalDays:           90,
		EnableArchival:         true,
		CacheSize:              1000,
		CacheTTL:               1 * time.Hour,
		EnableBackgroundAnalysis: true,
		AnalysisInterval:        1 * time.Hour,
		AlertingEnabled:         true,
		PerformanceThresholds: &PerformanceThresholds{
			MaxAverageResponseTime: 5 * time.Second,
			MinThroughputPerSecond: 100.0,
			MaxErrorRatePercent:    5.0,
			MaxMemoryUsageMB:       4096,
			MaxCPUUsagePercent:     80.0,
			MaxDurationSeconds:     3600,
		},
	}
}

func validateSystemConfig(config *SystemConfig) error {
	if config.StorageDirectory == "" {
		return fmt.Errorf("storage directory cannot be empty")
	}
	if config.OutputDirectory == "" {
		return fmt.Errorf("output directory cannot be empty")
	}
	if config.AnalysisWindowDays <= 0 {
		return fmt.Errorf("analysis window days must be positive")
	}
	if config.TrendAnalysisWindow <= 0 {
		return fmt.Errorf("trend analysis window must be positive")
	}
	if config.RegressionThreshold <= 0 {
		return fmt.Errorf("regression threshold must be positive")
	}
	if len(config.ReportFormats) == 0 {
		return fmt.Errorf("at least one report format must be specified")
	}
	return nil
}