package e2e_test

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ResultPersistenceManager handles comprehensive result storage and retrieval
type ResultPersistenceManager struct {
	mu               sync.RWMutex
	config           *PersistenceConfig
	logger           *log.Logger
	
	// Database components
	db               *sql.DB
	dbPath           string
	
	// File storage components
	storageDir       string
	indexDir         string
	archiveDir       string
	
	// Compression and archival
	compressionLevel int
	archivalEnabled  bool
	archivalDays     int
	
	// Performance optimization
	batchSize        int
	writeBuffer      []*TestRunRecord
	writeBufferMu    sync.Mutex
	
	// Search and indexing
	searchIndex      *SearchIndex
	
	// Cleanup management
	cleanupFunctions []func() error
}

// PersistenceConfig defines configuration for result persistence
type PersistenceConfig struct {
	StorageDirectory    string
	DatabasePath        string
	EnableCompression   bool
	CompressionLevel    int
	EnableArchival      bool
	ArchivalDays        int
	EnableSearch        bool
	BatchSize           int
	MaxStorageSize      int64  // bytes
	RetentionDays       int
	BackupEnabled       bool
	BackupInterval      time.Duration
	IndexingEnabled     bool
}

// TestRunRecord represents a complete test run stored in the database
type TestRunRecord struct {
	ID                  int64                  `json:"id" db:"id"`
	RunID               string                 `json:"run_id" db:"run_id"`
	Timestamp           time.Time              `json:"timestamp" db:"timestamp"`
	Version             string                 `json:"version" db:"version"`
	Branch              string                 `json:"branch" db:"branch"`
	Commit              string                 `json:"commit" db:"commit"`
	Environment         string                 `json:"environment" db:"environment"`
	Duration            time.Duration          `json:"duration" db:"duration"`
	Success             bool                   `json:"success" db:"success"`
	TotalTests          int64                  `json:"total_tests" db:"total_tests"`
	PassedTests         int64                  `json:"passed_tests" db:"passed_tests"`
	FailedTests         int64                  `json:"failed_tests" db:"failed_tests"`
	SkippedTests        int64                  `json:"skipped_tests" db:"skipped_tests"`
	TestResults         []*TestResult          `json:"test_results"`
	TestMetrics         *TestExecutionMetrics  `json:"test_metrics"`
	SystemMetrics       *SystemResourceMetrics `json:"system_metrics"`
	MCPMetrics          *MCPClientMetrics      `json:"mcp_metrics"`
	PerformanceData     *PerformanceMetrics    `json:"performance_data"`
	ComprehensiveReport *ComprehensiveReport   `json:"comprehensive_report"`
	Notes               string                 `json:"notes" db:"notes"`
	Tags                []string               `json:"tags"`
	Metadata            map[string]interface{} `json:"metadata"`
	StoragePath         string                 `json:"storage_path" db:"storage_path"`
	Compressed          bool                   `json:"compressed" db:"compressed"`
	Archived            bool                   `json:"archived" db:"archived"`
	CreatedAt           time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at" db:"updated_at"`
}

// SearchIndex provides fast search capabilities over stored results
type SearchIndex struct {
	mu          sync.RWMutex
	termIndex   map[string][]int64  // term -> record IDs
	metadataIndex map[string]map[string][]int64  // field -> value -> record IDs
	timeIndex   []TimeRangeEntry
	tagIndex    map[string][]int64  // tag -> record IDs
	enabled     bool
}

// TimeRangeEntry represents a time-based index entry
type TimeRangeEntry struct {
	RecordID  int64
	Timestamp time.Time
	Duration  time.Duration
}

// SearchQuery represents a search query for stored results
type SearchQuery struct {
	Terms           []string
	StartTime       *time.Time
	EndTime         *time.Time
	Success         *bool
	MinDuration     *time.Duration
	MaxDuration     *time.Duration
	Environment     string
	Branch          string
	Version         string
	Tags            []string
	Metadata        map[string]interface{}
	Limit           int
	Offset          int
	OrderBy         string
	OrderDirection  string  // "ASC" or "DESC"
}

// SearchResult represents search results
type SearchResult struct {
	Records      []*TestRunRecord
	TotalCount   int64
	SearchTime   time.Duration
	Query        *SearchQuery
}

// NewResultPersistenceManager creates a new result persistence manager
func NewResultPersistenceManager(config *PersistenceConfig) (*ResultPersistenceManager, error) {
	if config == nil {
		config = getDefaultPersistenceConfig()
	}

	if err := validatePersistenceConfig(config); err != nil {
		return nil, fmt.Errorf("invalid persistence configuration: %w", err)
	}

	logger := log.New(os.Stdout, "[E2E-Persistence] ", log.LstdFlags|log.Lshortfile)

	// Create directories
	if err := os.MkdirAll(config.StorageDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	storageDir := config.StorageDirectory
	indexDir := filepath.Join(storageDir, "indexes")
	archiveDir := filepath.Join(storageDir, "archive")

	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}

	if config.EnableArchival {
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create archive directory: %w", err)
		}
	}

	manager := &ResultPersistenceManager{
		config:           config,
		logger:           logger,
		storageDir:       storageDir,
		indexDir:         indexDir,
		archiveDir:       archiveDir,
		compressionLevel: config.CompressionLevel,
		archivalEnabled:  config.EnableArchival,
		archivalDays:     config.ArchivalDays,
		batchSize:        config.BatchSize,
		writeBuffer:      make([]*TestRunRecord, 0, config.BatchSize),
		cleanupFunctions: make([]func() error, 0),
	}

	// Initialize database
	if err := manager.initializeDatabase(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize search index
	if config.EnableSearch {
		manager.searchIndex = &SearchIndex{
			termIndex:     make(map[string][]int64),
			metadataIndex: make(map[string]map[string][]int64),
			timeIndex:     make([]TimeRangeEntry, 0),
			tagIndex:      make(map[string][]int64),
			enabled:       true,
		}
		if err := manager.loadSearchIndex(); err != nil {
			logger.Printf("Warning: Failed to load search index: %v", err)
		}
	}

	// Start background tasks
	go manager.startBackgroundTasks()

	logger.Printf("Result persistence manager initialized with storage: %s", storageDir)
	return manager, nil
}

// StoreTestRun stores a complete test run with all associated data
func (rpm *ResultPersistenceManager) StoreTestRun(record *TestRunRecord) error {
	if record == nil {
		return fmt.Errorf("test run record is nil")
	}

	rpm.writeBufferMu.Lock()
	defer rpm.writeBufferMu.Unlock()

	// Add to write buffer
	rpm.writeBuffer = append(rpm.writeBuffer, record)

	// Flush buffer if it reaches batch size
	if len(rpm.writeBuffer) >= rpm.batchSize {
		return rpm.flushWriteBuffer()
	}

	return nil
}

// FlushWriteBuffer forces flushing of the write buffer
func (rpm *ResultPersistenceManager) FlushWriteBuffer() error {
	rpm.writeBufferMu.Lock()
	defer rpm.writeBufferMu.Unlock()
	return rpm.flushWriteBuffer()
}

// flushWriteBuffer (internal) flushes the write buffer to storage
func (rpm *ResultPersistenceManager) flushWriteBuffer() error {
	if len(rpm.writeBuffer) == 0 {
		return nil
	}

	tx, err := rpm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, record := range rpm.writeBuffer {
		if err := rpm.storeTestRunRecord(tx, record); err != nil {
			return fmt.Errorf("failed to store test run record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	rpm.logger.Printf("Flushed %d test run records to storage", len(rpm.writeBuffer))
	rpm.writeBuffer = rpm.writeBuffer[:0] // Clear buffer

	return nil
}

// storeTestRunRecord stores a single test run record
func (rpm *ResultPersistenceManager) storeTestRunRecord(tx *sql.Tx, record *TestRunRecord) error {
	// Generate unique filename for detailed data storage
	filename := fmt.Sprintf("testrun_%s_%d.json", record.RunID, record.Timestamp.Unix())
	if rpm.config.EnableCompression {
		filename += ".gz"
		record.Compressed = true
	}

	storagePath := filepath.Join(rpm.storageDir, filename)
	record.StoragePath = storagePath

	// Store detailed data to file
	if err := rpm.storeTestRunFile(record, storagePath); err != nil {
		return fmt.Errorf("failed to store test run file: %w", err)
	}

	// Store metadata to database
	now := time.Now()
	record.CreatedAt = now
	record.UpdatedAt = now

	query := `
		INSERT INTO test_runs (
			run_id, timestamp, version, branch, commit, environment,
			duration, success, total_tests, passed_tests, failed_tests, skipped_tests,
			notes, storage_path, compressed, archived, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := tx.Exec(query,
		record.RunID, record.Timestamp, record.Version, record.Branch, record.Commit, record.Environment,
		record.Duration, record.Success, record.TotalTests, record.PassedTests, record.FailedTests, record.SkippedTests,
		record.Notes, record.StoragePath, record.Compressed, record.Archived, record.CreatedAt, record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert test run record: %w", err)
	}

	recordID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}
	record.ID = recordID

	// Store tags
	if len(record.Tags) > 0 {
		if err := rpm.storeTags(tx, recordID, record.Tags); err != nil {
			return fmt.Errorf("failed to store tags: %w", err)
		}
	}

	// Store metadata
	if len(record.Metadata) > 0 {
		if err := rpm.storeMetadata(tx, recordID, record.Metadata); err != nil {
			return fmt.Errorf("failed to store metadata: %w", err)
		}
	}

	// Update search index
	if rpm.searchIndex != nil && rpm.searchIndex.enabled {
		rpm.updateSearchIndex(record)
	}

	return nil
}

// storeTestRunFile stores the detailed test run data to a file
func (rpm *ResultPersistenceManager) storeTestRunFile(record *TestRunRecord, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	var writer io.Writer = file

	// Apply compression if enabled
	if rpm.config.EnableCompression {
		gzWriter, err := gzip.NewWriterLevel(file, rpm.compressionLevel)
		if err != nil {
			return fmt.Errorf("failed to create gzip writer: %w", err)
		}
		defer gzWriter.Close()
		writer = gzWriter
	}

	// Encode as JSON
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(record); err != nil {
		return fmt.Errorf("failed to encode test run record: %w", err)
	}

	return nil
}

// StoreTags stores tags for a test run record
func (rpm *ResultPersistenceManager) storeTags(tx *sql.Tx, recordID int64, tags []string) error {
	query := `INSERT INTO test_run_tags (test_run_id, tag) VALUES (?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare tag statement: %w", err)
	}
	defer stmt.Close()

	for _, tag := range tags {
		if _, err := stmt.Exec(recordID, tag); err != nil {
			return fmt.Errorf("failed to insert tag: %w", err)
		}
	}

	return nil
}

// storeMetadata stores metadata for a test run record
func (rpm *ResultPersistenceManager) storeMetadata(tx *sql.Tx, recordID int64, metadata map[string]interface{}) error {
	query := `INSERT INTO test_run_metadata (test_run_id, key, value) VALUES (?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare metadata statement: %w", err)
	}
	defer stmt.Close()

	for key, value := range metadata {
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata value: %w", err)
		}

		if _, err := stmt.Exec(recordID, key, string(valueJSON)); err != nil {
			return fmt.Errorf("failed to insert metadata: %w", err)
		}
	}

	return nil
}

// RetrieveTestRun retrieves a test run by ID
func (rpm *ResultPersistenceManager) RetrieveTestRun(id int64) (*TestRunRecord, error) {
	rpm.mu.RLock()
	defer rpm.mu.RUnlock()

	query := `
		SELECT id, run_id, timestamp, version, branch, commit, environment,
		       duration, success, total_tests, passed_tests, failed_tests, skipped_tests,
		       notes, storage_path, compressed, archived, created_at, updated_at
		FROM test_runs WHERE id = ?
	`

	var record TestRunRecord
	err := rpm.db.QueryRow(query, id).Scan(
		&record.ID, &record.RunID, &record.Timestamp, &record.Version, &record.Branch, &record.Commit, &record.Environment,
		&record.Duration, &record.Success, &record.TotalTests, &record.PassedTests, &record.FailedTests, &record.SkippedTests,
		&record.Notes, &record.StoragePath, &record.Compressed, &record.Archived, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("test run not found: %d", id)
		}
		return nil, fmt.Errorf("failed to query test run: %w", err)
	}

	// Load detailed data from file
	if err := rpm.loadTestRunFile(&record); err != nil {
		return nil, fmt.Errorf("failed to load test run file: %w", err)
	}

	// Load tags and metadata
	if err := rpm.loadTagsAndMetadata(&record); err != nil {
		return nil, fmt.Errorf("failed to load tags and metadata: %w", err)
	}

	return &record, nil
}

// loadTestRunFile loads detailed test run data from file
func (rpm *ResultPersistenceManager) loadTestRunFile(record *TestRunRecord) error {
	file, err := os.Open(record.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var reader io.Reader = file

	// Handle decompression if needed
	if record.Compressed {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Decode JSON
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(record); err != nil {
		return fmt.Errorf("failed to decode test run record: %w", err)
	}

	return nil
}

// loadTagsAndMetadata loads tags and metadata for a test run record
func (rpm *ResultPersistenceManager) loadTagsAndMetadata(record *TestRunRecord) error {
	// Load tags
	tagQuery := `SELECT tag FROM test_run_tags WHERE test_run_id = ?`
	tagRows, err := rpm.db.Query(tagQuery, record.ID)
	if err != nil {
		return fmt.Errorf("failed to query tags: %w", err)
	}
	defer tagRows.Close()

	record.Tags = make([]string, 0)
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil {
			return fmt.Errorf("failed to scan tag: %w", err)
		}
		record.Tags = append(record.Tags, tag)
	}

	// Load metadata
	metadataQuery := `SELECT key, value FROM test_run_metadata WHERE test_run_id = ?`
	metadataRows, err := rpm.db.Query(metadataQuery, record.ID)
	if err != nil {
		return fmt.Errorf("failed to query metadata: %w", err)
	}
	defer metadataRows.Close()

	record.Metadata = make(map[string]interface{})
	for metadataRows.Next() {
		var key, valueJSON string
		if err := metadataRows.Scan(&key, &valueJSON); err != nil {
			return fmt.Errorf("failed to scan metadata: %w", err)
		}

		var value interface{}
		if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
			return fmt.Errorf("failed to unmarshal metadata value: %w", err)
		}

		record.Metadata[key] = value
	}

	return nil
}

// SearchTestRuns searches for test runs based on query criteria
func (rpm *ResultPersistenceManager) SearchTestRuns(query *SearchQuery) (*SearchResult, error) {
	startTime := time.Now()

	rpm.mu.RLock()
	defer rpm.mu.RUnlock()

	// Build SQL query
	sqlQuery, args, err := rpm.buildSearchQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to build search query: %w", err)
	}

	// Execute query
	rows, err := rpm.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var records []*TestRunRecord
	for rows.Next() {
		var record TestRunRecord
		err := rows.Scan(
			&record.ID, &record.RunID, &record.Timestamp, &record.Version, &record.Branch, &record.Commit, &record.Environment,
			&record.Duration, &record.Success, &record.TotalTests, &record.PassedTests, &record.FailedTests, &record.SkippedTests,
			&record.Notes, &record.StoragePath, &record.Compressed, &record.Archived, &record.CreatedAt, &record.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		records = append(records, &record)
	}

	// Get total count
	countQuery := strings.Replace(sqlQuery, "SELECT id, run_id, timestamp, version, branch, commit, environment, duration, success, total_tests, passed_tests, failed_tests, skipped_tests, notes, storage_path, compressed, archived, created_at, updated_at", "SELECT COUNT(*)", 1)
	// Remove ORDER BY and LIMIT clauses for count
	if strings.Contains(countQuery, "ORDER BY") {
		countQuery = countQuery[:strings.Index(countQuery, "ORDER BY")]
	}

	var totalCount int64
	if err := rpm.db.QueryRow(countQuery, args[:len(args)-2]...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	searchTime := time.Since(startTime)

	return &SearchResult{
		Records:    records,
		TotalCount: totalCount,
		SearchTime: searchTime,
		Query:      query,
	}, nil
}

// buildSearchQuery builds SQL query from search parameters
func (rpm *ResultPersistenceManager) buildSearchQuery(query *SearchQuery) (string, []interface{}, error) {
	baseQuery := `
		SELECT id, run_id, timestamp, version, branch, commit, environment,
		       duration, success, total_tests, passed_tests, failed_tests, skipped_tests,
		       notes, storage_path, compressed, archived, created_at, updated_at
		FROM test_runs
	`

	var conditions []string
	var args []interface{}

	// Time range conditions
	if query.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *query.StartTime)
	}
	if query.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *query.EndTime)
	}

	// Success condition
	if query.Success != nil {
		conditions = append(conditions, "success = ?")
		args = append(args, *query.Success)
	}

	// Duration conditions
	if query.MinDuration != nil {
		conditions = append(conditions, "duration >= ?")
		args = append(args, *query.MinDuration)
	}
	if query.MaxDuration != nil {
		conditions = append(conditions, "duration <= ?")
		args = append(args, *query.MaxDuration)
	}

	// String field conditions
	if query.Environment != "" {
		conditions = append(conditions, "environment = ?")
		args = append(args, query.Environment)
	}
	if query.Branch != "" {
		conditions = append(conditions, "branch = ?")
		args = append(args, query.Branch)
	}
	if query.Version != "" {
		conditions = append(conditions, "version = ?")
		args = append(args, query.Version)
	}

	// Text search in notes
	if len(query.Terms) > 0 {
		termConditions := make([]string, len(query.Terms))
		for i, term := range query.Terms {
			termConditions[i] = "notes LIKE ?"
			args = append(args, "%"+term+"%")
		}
		conditions = append(conditions, "("+strings.Join(termConditions, " OR ")+")")
	}

	// Tag conditions
	if len(query.Tags) > 0 {
		tagConditions := make([]string, len(query.Tags))
		for i, tag := range query.Tags {
			tagConditions[i] = "id IN (SELECT test_run_id FROM test_run_tags WHERE tag = ?)"
			args = append(args, tag)
		}
		conditions = append(conditions, "("+strings.Join(tagConditions, " AND ")+")")
	}

	// Build WHERE clause
	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ORDER BY
	orderBy := "timestamp"
	orderDirection := "DESC"
	if query.OrderBy != "" {
		orderBy = query.OrderBy
	}
	if query.OrderDirection != "" {
		orderDirection = query.OrderDirection
	}
	baseQuery += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDirection)

	// Add LIMIT and OFFSET
	limit := 50
	if query.Limit > 0 {
		limit = query.Limit
	}
	baseQuery += " LIMIT ?"
	args = append(args, limit)

	if query.Offset > 0 {
		baseQuery += " OFFSET ?"
		args = append(args, query.Offset)
	}

	return baseQuery, args, nil
}

// GetHistoricalData retrieves historical test run data for analysis
func (rpm *ResultPersistenceManager) GetHistoricalData(days int, environment string) ([]*TestRunRecord, error) {
	query := &SearchQuery{
		StartTime:   &time.Time{},
		Environment: environment,
		OrderBy:     "timestamp",
		OrderDirection: "ASC",
		Limit:       1000,
	}

	startTime := time.Now().AddDate(0, 0, -days)
	query.StartTime = &startTime

	result, err := rpm.SearchTestRuns(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical data: %w", err)
	}

	return result.Records, nil
}

// initializeDatabase initializes the SQLite database schema
func (rpm *ResultPersistenceManager) initializeDatabase() error {
	dbPath := rpm.config.DatabasePath
	if dbPath == "" {
		dbPath = filepath.Join(rpm.storageDir, "test_results.db")
	}
	rpm.dbPath = dbPath

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	rpm.db = db

	// Create tables
	if err := rpm.createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Create indexes
	if err := rpm.createIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// createTables creates the database schema
func (rpm *ResultPersistenceManager) createTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS test_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id TEXT NOT NULL UNIQUE,
			timestamp DATETIME NOT NULL,
			version TEXT,
			branch TEXT,
			commit TEXT,
			environment TEXT,
			duration INTEGER,
			success BOOLEAN NOT NULL,
			total_tests INTEGER,
			passed_tests INTEGER,
			failed_tests INTEGER,
			skipped_tests INTEGER,
			notes TEXT,
			storage_path TEXT NOT NULL,
			compressed BOOLEAN DEFAULT FALSE,
			archived BOOLEAN DEFAULT FALSE,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		
		`CREATE TABLE IF NOT EXISTS test_run_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			test_run_id INTEGER NOT NULL,
			tag TEXT NOT NULL,
			FOREIGN KEY (test_run_id) REFERENCES test_runs(id) ON DELETE CASCADE,
			UNIQUE(test_run_id, tag)
		)`,
		
		`CREATE TABLE IF NOT EXISTS test_run_metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			test_run_id INTEGER NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			FOREIGN KEY (test_run_id) REFERENCES test_runs(id) ON DELETE CASCADE,
			UNIQUE(test_run_id, key)
		)`,
		
		`CREATE TABLE IF NOT EXISTS performance_baselines (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			test_name TEXT NOT NULL,
			scenario TEXT,
			environment TEXT,
			baseline_duration INTEGER,
			baseline_throughput REAL,
			baseline_memory REAL,
			sample_count INTEGER,
			confidence REAL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(test_name, scenario, environment)
		)`,
	}

	for _, table := range tables {
		if _, err := rpm.db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// createIndexes creates database indexes for performance
func (rpm *ResultPersistenceManager) createIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_test_runs_timestamp ON test_runs(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_test_runs_success ON test_runs(success)",
		"CREATE INDEX IF NOT EXISTS idx_test_runs_environment ON test_runs(environment)",
		"CREATE INDEX IF NOT EXISTS idx_test_runs_branch ON test_runs(branch)",
		"CREATE INDEX IF NOT EXISTS idx_test_runs_version ON test_runs(version)",
		"CREATE INDEX IF NOT EXISTS idx_test_runs_duration ON test_runs(duration)",
		"CREATE INDEX IF NOT EXISTS idx_test_run_tags_tag ON test_run_tags(tag)",
		"CREATE INDEX IF NOT EXISTS idx_test_run_metadata_key ON test_run_metadata(key)",
		"CREATE INDEX IF NOT EXISTS idx_performance_baselines_test_name ON performance_baselines(test_name)",
	}

	for _, index := range indexes {
		if _, err := rpm.db.Exec(index); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// updateSearchIndex updates the in-memory search index
func (rpm *ResultPersistenceManager) updateSearchIndex(record *TestRunRecord) {
	if rpm.searchIndex == nil {
		return
	}

	rpm.searchIndex.mu.Lock()
	defer rpm.searchIndex.mu.Unlock()

	// Index time range
	rpm.searchIndex.timeIndex = append(rpm.searchIndex.timeIndex, TimeRangeEntry{
		RecordID:  record.ID,
		Timestamp: record.Timestamp,
		Duration:  record.Duration,
	})

	// Index tags
	for _, tag := range record.Tags {
		rpm.searchIndex.tagIndex[tag] = append(rpm.searchIndex.tagIndex[tag], record.ID)
	}

	// Index metadata
	for key, value := range record.Metadata {
		if rpm.searchIndex.metadataIndex[key] == nil {
			rpm.searchIndex.metadataIndex[key] = make(map[string][]int64)
		}
		valueStr := fmt.Sprintf("%v", value)
		rpm.searchIndex.metadataIndex[key][valueStr] = append(rpm.searchIndex.metadataIndex[key][valueStr], record.ID)
	}

	// Index searchable terms from notes
	if record.Notes != "" {
		terms := strings.Fields(strings.ToLower(record.Notes))
		for _, term := range terms {
			rpm.searchIndex.termIndex[term] = append(rpm.searchIndex.termIndex[term], record.ID)
		}
	}
}

// loadSearchIndex loads the search index from disk
func (rpm *ResultPersistenceManager) loadSearchIndex() error {
	indexPath := filepath.Join(rpm.indexDir, "search_index.json")
	
	file, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Index doesn't exist, rebuild it
			return rpm.rebuildSearchIndex()
		}
		return fmt.Errorf("failed to open search index: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(rpm.searchIndex); err != nil {
		return fmt.Errorf("failed to decode search index: %w", err)
	}

	return nil
}

// rebuildSearchIndex rebuilds the search index from database
func (rpm *ResultPersistenceManager) rebuildSearchIndex() error {
	rpm.logger.Printf("Rebuilding search index...")

	query := `SELECT id, timestamp, duration, notes FROM test_runs ORDER BY timestamp`
	rows, err := rpm.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query test runs for indexing: %w", err)
	}
	defer rows.Close()

	rpm.searchIndex.mu.Lock()
	defer rpm.searchIndex.mu.Unlock()

	// Clear existing index
	rpm.searchIndex.termIndex = make(map[string][]int64)
	rpm.searchIndex.timeIndex = make([]TimeRangeEntry, 0)

	for rows.Next() {
		var id int64
		var timestamp time.Time
		var duration time.Duration
		var notes string

		if err := rows.Scan(&id, &timestamp, &duration, &notes); err != nil {
			return fmt.Errorf("failed to scan row for indexing: %w", err)
		}

		// Index time range
		rpm.searchIndex.timeIndex = append(rpm.searchIndex.timeIndex, TimeRangeEntry{
			RecordID:  id,
			Timestamp: timestamp,
			Duration:  duration,
		})

		// Index terms from notes
		if notes != "" {
			terms := strings.Fields(strings.ToLower(notes))
			for _, term := range terms {
				rpm.searchIndex.termIndex[term] = append(rpm.searchIndex.termIndex[term], id)
			}
		}
	}

	// Index tags and metadata
	if err := rpm.indexTagsAndMetadata(); err != nil {
		return fmt.Errorf("failed to index tags and metadata: %w", err)
	}

	rpm.logger.Printf("Search index rebuilt with %d records", len(rpm.searchIndex.timeIndex))
	return nil
}

// indexTagsAndMetadata indexes tags and metadata for search
func (rpm *ResultPersistenceManager) indexTagsAndMetadata() error {
	// Index tags
	tagQuery := `SELECT test_run_id, tag FROM test_run_tags`
	tagRows, err := rpm.db.Query(tagQuery)
	if err != nil {
		return fmt.Errorf("failed to query tags for indexing: %w", err)
	}
	defer tagRows.Close()

	for tagRows.Next() {
		var recordID int64
		var tag string
		if err := tagRows.Scan(&recordID, &tag); err != nil {
			return fmt.Errorf("failed to scan tag for indexing: %w", err)
		}
		rpm.searchIndex.tagIndex[tag] = append(rpm.searchIndex.tagIndex[tag], recordID)
	}

	// Index metadata
	metadataQuery := `SELECT test_run_id, key, value FROM test_run_metadata`
	metadataRows, err := rpm.db.Query(metadataQuery)
	if err != nil {
		return fmt.Errorf("failed to query metadata for indexing: %w", err)
	}
	defer metadataRows.Close()

	for metadataRows.Next() {
		var recordID int64
		var key, valueJSON string
		if err := metadataRows.Scan(&recordID, &key, &valueJSON); err != nil {
			return fmt.Errorf("failed to scan metadata for indexing: %w", err)
		}

		var value interface{}
		if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
			continue // Skip invalid JSON
		}

		if rpm.searchIndex.metadataIndex[key] == nil {
			rpm.searchIndex.metadataIndex[key] = make(map[string][]int64)
		}
		valueStr := fmt.Sprintf("%v", value)
		rpm.searchIndex.metadataIndex[key][valueStr] = append(rpm.searchIndex.metadataIndex[key][valueStr], recordID)
	}

	return nil
}

// startBackgroundTasks starts background maintenance tasks
func (rpm *ResultPersistenceManager) startBackgroundTasks() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Perform periodic maintenance
			if err := rpm.performMaintenance(); err != nil {
				rpm.logger.Printf("Maintenance error: %v", err)
			}
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (rpm *ResultPersistenceManager) performMaintenance() error {
	// Flush write buffer
	if err := rpm.FlushWriteBuffer(); err != nil {
		return fmt.Errorf("failed to flush write buffer: %w", err)
	}

	// Archive old records if enabled
	if rpm.archivalEnabled && rpm.archivalDays > 0 {
		if err := rpm.archiveOldRecords(); err != nil {
			return fmt.Errorf("failed to archive old records: %w", err)
		}
	}

	// Clean up old records if retention is enabled
	if rpm.config.RetentionDays > 0 {
		if err := rpm.cleanupOldRecords(); err != nil {
			return fmt.Errorf("failed to cleanup old records: %w", err)
		}
	}

	// Vacuum database
	if _, err := rpm.db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	return nil
}

// archiveOldRecords moves old records to archive storage
func (rpm *ResultPersistenceManager) archiveOldRecords() error {
	cutoffTime := time.Now().AddDate(0, 0, -rpm.archivalDays)
	
	query := `SELECT id, storage_path FROM test_runs WHERE timestamp < ? AND archived = FALSE`
	rows, err := rpm.db.Query(query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to query records for archival: %w", err)
	}
	defer rows.Close()

	var recordsArchived int
	for rows.Next() {
		var id int64
		var storagePath string
		if err := rows.Scan(&id, &storagePath); err != nil {
			continue
		}

		// Move file to archive directory
		archivePath := filepath.Join(rpm.archiveDir, filepath.Base(storagePath))
		if err := os.Rename(storagePath, archivePath); err != nil {
			continue
		}

		// Update database record
		updateQuery := `UPDATE test_runs SET archived = TRUE, storage_path = ? WHERE id = ?`
		if _, err := rpm.db.Exec(updateQuery, archivePath, id); err != nil {
			continue
		}

		recordsArchived++
	}

	if recordsArchived > 0 {
		rpm.logger.Printf("Archived %d old records", recordsArchived)
	}

	return nil
}

// cleanupOldRecords removes old records beyond retention period
func (rpm *ResultPersistenceManager) cleanupOldRecords() error {
	cutoffTime := time.Now().AddDate(0, 0, -rpm.config.RetentionDays)
	
	// Get records to delete
	query := `SELECT id, storage_path FROM test_runs WHERE timestamp < ?`
	rows, err := rpm.db.Query(query, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to query records for cleanup: %w", err)
	}
	defer rows.Close()

	var recordsDeleted int
	for rows.Next() {
		var id int64
		var storagePath string
		if err := rows.Scan(&id, &storagePath); err != nil {
			continue
		}

		// Delete file
		if err := os.Remove(storagePath); err != nil && !os.IsNotExist(err) {
			rpm.logger.Printf("Warning: Failed to delete file %s: %v", storagePath, err)
		}

		recordsDeleted++
	}

	// Delete database records
	deleteQuery := `DELETE FROM test_runs WHERE timestamp < ?`
	if _, err := rpm.db.Exec(deleteQuery, cutoffTime); err != nil {
		return fmt.Errorf("failed to delete old records: %w", err)
	}

	if recordsDeleted > 0 {
		rpm.logger.Printf("Cleaned up %d old records", recordsDeleted)
	}

	return nil
}

// Cleanup performs cleanup of the persistence manager
func (rpm *ResultPersistenceManager) Cleanup() error {
	rpm.mu.Lock()
	defer rpm.mu.Unlock()

	rpm.logger.Printf("Starting result persistence manager cleanup")

	// Flush any remaining data
	if err := rpm.FlushWriteBuffer(); err != nil {
		rpm.logger.Printf("Warning: Failed to flush write buffer during cleanup: %v", err)
	}

	// Save search index
	if rpm.searchIndex != nil {
		if err := rpm.saveSearchIndex(); err != nil {
			rpm.logger.Printf("Warning: Failed to save search index: %v", err)
		}
	}

	// Close database
	if rpm.db != nil {
		if err := rpm.db.Close(); err != nil {
			rpm.logger.Printf("Warning: Failed to close database: %v", err)
		}
	}

	// Execute cleanup functions
	var errors []error
	for _, cleanup := range rpm.cleanupFunctions {
		if err := cleanup(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	rpm.logger.Printf("Result persistence manager cleanup completed")
	return nil
}

// saveSearchIndex saves the search index to disk
func (rpm *ResultPersistenceManager) saveSearchIndex() error {
	if rpm.searchIndex == nil {
		return nil
	}

	indexPath := filepath.Join(rpm.indexDir, "search_index.json")
	
	file, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create search index file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rpm.searchIndex); err != nil {
		return fmt.Errorf("failed to encode search index: %w", err)
	}

	return nil
}

// Helper functions

func getDefaultPersistenceConfig() *PersistenceConfig {
	return &PersistenceConfig{
		StorageDirectory:  "./e2e-results",
		DatabasePath:      "",
		EnableCompression: true,
		CompressionLevel:  6,
		EnableArchival:    true,
		ArchivalDays:      90,
		EnableSearch:      true,
		BatchSize:         10,
		MaxStorageSize:    10 * 1024 * 1024 * 1024, // 10GB
		RetentionDays:     365,
		BackupEnabled:     true,
		BackupInterval:    24 * time.Hour,
		IndexingEnabled:   true,
	}
}

func validatePersistenceConfig(config *PersistenceConfig) error {
	if config.StorageDirectory == "" {
		return fmt.Errorf("storage directory cannot be empty")
	}
	if config.CompressionLevel < 1 || config.CompressionLevel > 9 {
		return fmt.Errorf("compression level must be between 1 and 9")
	}
	if config.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}
	if config.RetentionDays < 0 {
		return fmt.Errorf("retention days cannot be negative")
	}
	return nil
}