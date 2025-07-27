package indexing

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

// Test configuration helpers
func createTestConfig() *SCIPConfig {
	return &SCIPConfig{
		CacheConfig: CacheConfig{
			Enabled: true,
			MaxSize: 1000,
			TTL:     time.Hour,
		},
		Logging: LoggingConfig{
			LogQueries:         false,
			LogCacheOperations: false,
			LogIndexOperations: false,
		},
		Performance: PerformanceConfig{
			QueryTimeout:         time.Minute,
			MaxConcurrentQueries: 10,
			IndexLoadTimeout:     time.Minute * 5,
		},
	}
}

func createInvalidConfig() *SCIPConfig {
	return &SCIPConfig{
		Performance: PerformanceConfig{
			QueryTimeout:         -1,
			MaxConcurrentQueries: 0,
			IndexLoadTimeout:     0,
		},
		CacheConfig: CacheConfig{
			MaxSize: -1,
			TTL:     -time.Hour,
		},
	}
}

// Test SCIP data helpers
func createTestSCIPIndex() *scip.Index {
	return &scip.Index{
		Metadata: &scip.Metadata{
			Version: scip.ProtocolVersion_UnspecifiedProtocolVersion,
			ToolInfo: &scip.ToolInfo{
				Name:    "test-tool",
				Version: "1.0.0",
			},
			ProjectRoot: "/test/project",
		},
		Documents: []*scip.Document{
			{
				RelativePath: "main.go",
				Language:     "go",
				Text:         "package main\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}",
				Symbols: []*scip.SymbolInformation{
					{
						Symbol:        "main.main",
						DisplayName:   "main",
						Kind:          scip.SymbolInformation_Function,
						Documentation: []string{"Main function"},
					},
				},
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{2, 5, 2, 9},
						Symbol:      "main.main",
						SymbolRoles: int32(scip.SymbolRole_Definition),
						SyntaxKind:  scip.SyntaxKind_UnspecifiedSyntaxKind,
					},
				},
			},
			{
				RelativePath: "utils.go",
				Language:     "go",
				Text:         "package main\n\nfunc helper() string {\n    return \"help\"\n}",
				Symbols: []*scip.SymbolInformation{
					{
						Symbol:      "main.helper",
						DisplayName: "helper",
						Kind:        scip.SymbolInformation_Function,
					},
				},
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{2, 5, 2, 11},
						Symbol:      "main.helper",
						SymbolRoles: int32(scip.SymbolRole_Definition),
						SyntaxKind:  scip.SyntaxKind_UnspecifiedSyntaxKind,
					},
				},
			},
		},
	}
}

func createEmptySCIPIndex() *scip.Index {
	return &scip.Index{
		Metadata:  &scip.Metadata{},
		Documents: []*scip.Document{},
	}
}

func createSCIPFile(t *testing.T, index *scip.Index) string {
	t.Helper()
	
	data, err := proto.Marshal(index)
	if err != nil {
		t.Fatalf("Failed to marshal SCIP index: %v", err)
	}

	tmpFile, err := os.CreateTemp("", "test_scip_*.scip")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatalf("Failed to write SCIP data: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}

// TestNewSCIPClient tests client creation and configuration validation
func TestNewSCIPClient(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := createTestConfig()
		client, err := NewSCIPClient(config)
		
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		
		if client == nil {
			t.Fatal("Expected client to be created")
		}
		
		if client.config != config {
			t.Error("Expected config to be set")
		}
		
		if client.indices == nil {
			t.Error("Expected indices map to be initialized")
		}
		
		if client.symbolIndex == nil {
			t.Error("Expected symbolIndex map to be initialized")
		}
		
		if client.documentIndex == nil {
			t.Error("Expected documentIndex map to be initialized")
		}
		
		if client.stats == nil {
			t.Error("Expected stats to be initialized")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		client, err := NewSCIPClient(nil)
		
		if err == nil {
			t.Fatal("Expected error for nil config")
		}
		
		if client != nil {
			t.Error("Expected nil client for nil config")
		}
		
		if !strings.Contains(err.Error(), "config cannot be nil") {
			t.Errorf("Expected 'config cannot be nil' error, got: %v", err)
		}
	})

	t.Run("InvalidConfig", func(t *testing.T) {
		config := createInvalidConfig()
		client, err := NewSCIPClient(config)
		
		if err == nil {
			t.Fatal("Expected error for invalid config")
		}
		
		if client != nil {
			t.Error("Expected nil client for invalid config")
		}
		
		if !strings.Contains(err.Error(), "invalid configuration") {
			t.Errorf("Expected 'invalid configuration' error, got: %v", err)
		}
	})
}

// TestLoadIndex tests SCIP index loading functionality
func TestLoadIndex(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("ValidIndex", func(t *testing.T) {
		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		err := client.LoadIndex(indexPath)
		if err != nil {
			t.Fatalf("Expected no error loading valid index, got: %v", err)
		}

		// Check that statistics were updated
		if client.stats.IndexLoads != 1 {
			t.Errorf("Expected 1 index load, got: %d", client.stats.IndexLoads)
		}

		if client.stats.TotalDocuments != 2 {
			t.Errorf("Expected 2 documents, got: %d", client.stats.TotalDocuments)
		}

		if client.stats.TotalSymbols != 2 {
			t.Errorf("Expected 2 symbols, got: %d", client.stats.TotalSymbols)
		}

		// Check that data was loaded into indices
		if len(client.indices) != 1 {
			t.Errorf("Expected 1 loaded index, got: %d", len(client.indices))
		}

		if len(client.documentIndex) != 2 {
			t.Errorf("Expected 2 documents in index, got: %d", len(client.documentIndex))
		}

		if len(client.symbolIndex) != 2 {
			t.Errorf("Expected 2 symbols in index, got: %d", len(client.symbolIndex))
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		err := client.LoadIndex("")
		if err == nil {
			t.Fatal("Expected error for empty path")
		}

		if !strings.Contains(err.Error(), "index path cannot be empty") {
			t.Errorf("Expected 'index path cannot be empty' error, got: %v", err)
		}
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		err := client.LoadIndex("/non/existent/file.scip")
		if err == nil {
			t.Fatal("Expected error for non-existent file")
		}

		if !strings.Contains(err.Error(), "failed to stat index file") {
			t.Errorf("Expected 'failed to stat index file' error, got: %v", err)
		}
	})

	t.Run("DirectoryInsteadOfFile", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "test_dir")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		err = client.LoadIndex(tmpDir)
		if err == nil {
			t.Fatal("Expected error for directory path")
		}

		if !strings.Contains(err.Error(), "is a directory, not a file") {
			t.Errorf("Expected 'is a directory, not a file' error, got: %v", err)
		}
	})

	t.Run("InvalidSCIPFile", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "invalid_*.scip")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write invalid protobuf data
		if _, err := tmpFile.WriteString("invalid scip data"); err != nil {
			t.Fatalf("Failed to write invalid data: %v", err)
		}
		tmpFile.Close()

		err = client.LoadIndex(tmpFile.Name())
		if err == nil {
			t.Fatal("Expected error for invalid SCIP file")
		}

		if !strings.Contains(err.Error(), "failed to parse SCIP file") {
			t.Errorf("Expected 'failed to parse SCIP file' error, got: %v", err)
		}
	})
}

// TestParseSCIPFile tests SCIP data parsing functionality
func TestParseSCIPFile(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("ValidSCIPData", func(t *testing.T) {
		scipIndex := createTestSCIPIndex()
		data, err := proto.Marshal(scipIndex)
		if err != nil {
			t.Fatalf("Failed to marshal SCIP index: %v", err)
		}

		reader := bytes.NewReader(data)
		err = client.ParseSCIPFile(reader, "/test/path.scip", int64(len(data)))
		
		if err != nil {
			t.Fatalf("Expected no error parsing valid SCIP data, got: %v", err)
		}

		// Check that data was parsed correctly
		if len(client.indices) != 1 {
			t.Errorf("Expected 1 parsed index, got: %d", len(client.indices))
		}

		indexData := client.indices["/test/path.scip"]
		if indexData == nil {
			t.Fatal("Expected index data to be stored")
		}

		if indexData.Metadata.Version != scip.ProtocolVersion_UnspecifiedProtocolVersion {
			t.Errorf("Expected version UnspecifiedProtocolVersion, got: %v", indexData.Metadata.Version)
		}

		if len(indexData.Documents) != 2 {
			t.Errorf("Expected 2 documents, got: %d", len(indexData.Documents))
		}
	})

	t.Run("EmptySCIPData", func(t *testing.T) {
		scipIndex := createEmptySCIPIndex()
		data, err := proto.Marshal(scipIndex)
		if err != nil {
			t.Fatalf("Failed to marshal empty SCIP index: %v", err)
		}

		reader := bytes.NewReader(data)
		err = client.ParseSCIPFile(reader, "/test/empty.scip", int64(len(data)))
		
		if err != nil {
			t.Fatalf("Expected no error parsing empty SCIP data, got: %v", err)
		}

		// Check that empty data was handled correctly
		indexData := client.indices["/test/empty.scip"]
		if indexData == nil {
			t.Fatal("Expected index data to be stored")
		}

		if len(indexData.Documents) != 0 {
			t.Errorf("Expected 0 documents for empty index, got: %d", len(indexData.Documents))
		}
	})

	t.Run("InvalidProtobufData", func(t *testing.T) {
		reader := strings.NewReader("invalid protobuf data")
		err = client.ParseSCIPFile(reader, "/test/invalid.scip", 20)
		
		if err == nil {
			t.Fatal("Expected error for invalid protobuf data")
		}

		if !strings.Contains(err.Error(), "SCIP unmarshaling failed") {
			t.Errorf("Expected 'SCIP unmarshaling failed' error, got: %v", err)
		}
	})
}

// TestGetDocumentData tests document retrieval functionality
func TestGetDocumentData(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Load test data
	scipIndex := createTestSCIPIndex()
	indexPath := createSCIPFile(t, scipIndex)
	defer os.Remove(indexPath)

	if err := client.LoadIndex(indexPath); err != nil {
		t.Fatalf("Failed to load test index: %v", err)
	}

	t.Run("ExistingDocument", func(t *testing.T) {
		docData, err := client.GetDocumentData("main.go")
		if err != nil {
			t.Fatalf("Expected no error getting existing document, got: %v", err)
		}

		if docData.URI != "main.go" {
			t.Errorf("Expected URI 'main.go', got: %s", docData.URI)
		}

		if docData.Language != "go" {
			t.Errorf("Expected language 'go', got: %s", docData.Language)
		}

		if len(docData.Symbols) != 1 {
			t.Errorf("Expected 1 symbol, got: %d", len(docData.Symbols))
		}

		if len(docData.Occurrences) != 1 {
			t.Errorf("Expected 1 occurrence, got: %d", len(docData.Occurrences))
		}
	})

	t.Run("NonExistentDocument", func(t *testing.T) {
		_, err := client.GetDocumentData("non_existent.go")
		if err == nil {
			t.Fatal("Expected error for non-existent document")
		}

		if !strings.Contains(err.Error(), "document not found") {
			t.Errorf("Expected 'document not found' error, got: %v", err)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		_, err := client.GetDocumentData("")
		if err == nil {
			t.Fatal("Expected error for empty path")
		}

		if !strings.Contains(err.Error(), "file path cannot be empty") {
			t.Errorf("Expected 'file path cannot be empty' error, got: %v", err)
		}
	})

	t.Run("PathVariations", func(t *testing.T) {
		// Test that relative path variations work
		docData, err := client.GetDocumentData("./main.go")
		if err != nil {
			t.Fatalf("Expected no error for relative path variation, got: %v", err)
		}

		if docData.URI != "main.go" {
			t.Errorf("Expected URI 'main.go', got: %s", docData.URI)
		}
	})
}

// TestFindSymbolsByPrefix tests symbol search functionality
func TestFindSymbolsByPrefix(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Load test data
	scipIndex := createTestSCIPIndex()
	indexPath := createSCIPFile(t, scipIndex)
	defer os.Remove(indexPath)

	if err := client.LoadIndex(indexPath); err != nil {
		t.Fatalf("Failed to load test index: %v", err)
	}

	t.Run("ExistingPrefix", func(t *testing.T) {
		symbols, err := client.FindSymbolsByPrefix("main")
		if err != nil {
			t.Fatalf("Expected no error finding symbols, got: %v", err)
		}

		if len(symbols) != 2 {
			t.Errorf("Expected 2 symbols with prefix 'main', got: %d", len(symbols))
		}

		// Check that results are sorted
		if len(symbols) >= 2 && symbols[0].Symbol > symbols[1].Symbol {
			t.Error("Expected symbols to be sorted by symbol name")
		}
	})

	t.Run("PartialPrefix", func(t *testing.T) {
		symbols, err := client.FindSymbolsByPrefix("help")
		if err != nil {
			t.Fatalf("Expected no error finding symbols, got: %v", err)
		}

		if len(symbols) != 1 {
			t.Errorf("Expected 1 symbol with prefix 'help', got: %d", len(symbols))
		}

		if len(symbols) > 0 && symbols[0].Symbol != "main.helper" {
			t.Errorf("Expected symbol 'main.helper', got: %s", symbols[0].Symbol)
		}
	})

	t.Run("NonExistentPrefix", func(t *testing.T) {
		symbols, err := client.FindSymbolsByPrefix("nonexistent")
		if err != nil {
			t.Fatalf("Expected no error for non-existent prefix, got: %v", err)
		}

		if len(symbols) != 0 {
			t.Errorf("Expected 0 symbols for non-existent prefix, got: %d", len(symbols))
		}
	})

	t.Run("EmptyPrefix", func(t *testing.T) {
		_, err := client.FindSymbolsByPrefix("")
		if err == nil {
			t.Fatal("Expected error for empty prefix")
		}

		if !strings.Contains(err.Error(), "prefix cannot be empty") {
			t.Errorf("Expected 'prefix cannot be empty' error, got: %v", err)
		}
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		symbols, err := client.FindSymbolsByPrefix("MAIN")
		if err != nil {
			t.Fatalf("Expected no error for uppercase prefix, got: %v", err)
		}

		if len(symbols) != 2 {
			t.Errorf("Expected 2 symbols for case-insensitive search, got: %d", len(symbols))
		}
	})
}

// TestGetSymbolByName tests exact symbol lookup functionality
func TestGetSymbolByName(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Load test data
	scipIndex := createTestSCIPIndex()
	indexPath := createSCIPFile(t, scipIndex)
	defer os.Remove(indexPath)

	if err := client.LoadIndex(indexPath); err != nil {
		t.Fatalf("Failed to load test index: %v", err)
	}

	t.Run("ExistingSymbol", func(t *testing.T) {
		symbol, err := client.GetSymbolByName("main.main")
		if err != nil {
			t.Fatalf("Expected no error getting existing symbol, got: %v", err)
		}

		if symbol.Symbol != "main.main" {
			t.Errorf("Expected symbol 'main.main', got: %s", symbol.Symbol)
		}

		if symbol.DisplayName != "main" {
			t.Errorf("Expected display name 'main', got: %s", symbol.DisplayName)
		}

		if symbol.Kind != scip.SymbolInformation_Function {
			t.Errorf("Expected kind Function, got: %v", symbol.Kind)
		}
	})

	t.Run("NonExistentSymbol", func(t *testing.T) {
		_, err := client.GetSymbolByName("non.existent")
		if err == nil {
			t.Fatal("Expected error for non-existent symbol")
		}

		if !strings.Contains(err.Error(), "symbol not found") {
			t.Errorf("Expected 'symbol not found' error, got: %v", err)
		}
	})

	t.Run("EmptySymbolName", func(t *testing.T) {
		_, err := client.GetSymbolByName("")
		if err == nil {
			t.Fatal("Expected error for empty symbol name")
		}

		if !strings.Contains(err.Error(), "symbol name cannot be empty") {
			t.Errorf("Expected 'symbol name cannot be empty' error, got: %v", err)
		}
	})
}

// TestClientStats tests statistics tracking functionality
func TestClientStats(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("InitialStats", func(t *testing.T) {
		stats := client.GetStats()
		if stats.IndexLoads != 0 {
			t.Errorf("Expected 0 initial index loads, got: %d", stats.IndexLoads)
		}

		if stats.TotalDocuments != 0 {
			t.Errorf("Expected 0 initial documents, got: %d", stats.TotalDocuments)
		}

		if stats.QueryCount != 0 {
			t.Errorf("Expected 0 initial queries, got: %d", stats.QueryCount)
		}
	})

	t.Run("StatsAfterLoading", func(t *testing.T) {
		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		if err := client.LoadIndex(indexPath); err != nil {
			t.Fatalf("Failed to load test index: %v", err)
		}

		stats := client.GetStats()
		if stats.IndexLoads != 1 {
			t.Errorf("Expected 1 index load, got: %d", stats.IndexLoads)
		}

		if stats.TotalDocuments != 2 {
			t.Errorf("Expected 2 documents, got: %d", stats.TotalDocuments)
		}

		if stats.MemoryUsage <= 0 {
			t.Errorf("Expected positive memory usage, got: %d", stats.MemoryUsage)
		}
	})

	t.Run("StatsAfterQueries", func(t *testing.T) {
		// Create a fresh client for this test
		freshClient, err := NewSCIPClient(createTestConfig())
		if err != nil {
			t.Fatalf("Failed to create fresh client: %v", err)
		}

		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		if err := freshClient.LoadIndex(indexPath); err != nil {
			t.Fatalf("Failed to load test index: %v", err)
		}

		// Perform some queries
		freshClient.GetDocumentData("main.go")
		freshClient.FindSymbolsByPrefix("main")
		freshClient.GetSymbolByName("main.main")

		stats := freshClient.GetStats()
		// Note: GetSymbolByName doesn't increment query count in current implementation
		if stats.QueryCount != 2 {
			t.Errorf("Expected 2 queries (GetDocumentData + FindSymbolsByPrefix), got: %d", stats.QueryCount)
		}

		if stats.AverageQueryTime <= 0 {
			t.Errorf("Expected positive average query time, got: %v", stats.AverageQueryTime)
		}
	})
}

// TestClientHealth tests health checking functionality
func TestClientHealth(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("UnhealthyWithoutIndices", func(t *testing.T) {
		if client.IsHealthy() {
			t.Error("Expected client to be unhealthy without loaded indices")
		}
	})

	t.Run("HealthyWithLoadedIndex", func(t *testing.T) {
		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		if err := client.LoadIndex(indexPath); err != nil {
			t.Fatalf("Failed to load test index: %v", err)
		}

		if !client.IsHealthy() {
			t.Error("Expected client to be healthy with loaded index")
		}
	})

	t.Run("UnhealthyWithHighErrorRate", func(t *testing.T) {
		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		if err := client.LoadIndex(indexPath); err != nil {
			t.Fatalf("Failed to load test index: %v", err)
		}

		// Simulate high error rate by incrementing error count directly
		client.stats.QueryCount = 10
		client.stats.ErrorCount = 2 // 20% error rate

		if client.IsHealthy() {
			t.Error("Expected client to be unhealthy with high error rate")
		}
	})
}

// TestClientCleanup tests resource cleanup functionality
func TestClientCleanup(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Load some data
	scipIndex := createTestSCIPIndex()
	indexPath := createSCIPFile(t, scipIndex)
	defer os.Remove(indexPath)

	if err := client.LoadIndex(indexPath); err != nil {
		t.Fatalf("Failed to load test index: %v", err)
	}

	// Verify data is loaded
	if len(client.indices) == 0 {
		t.Fatal("Expected indices to be loaded")
	}

	if len(client.symbolIndex) == 0 {
		t.Fatal("Expected symbol index to be loaded")
	}

	if len(client.documentIndex) == 0 {
		t.Fatal("Expected document index to be loaded")
	}

	// Close the client
	if err := client.Close(); err != nil {
		t.Fatalf("Expected no error closing client, got: %v", err)
	}

	// Verify data is cleaned up
	if len(client.indices) != 0 {
		t.Errorf("Expected indices to be cleared, got: %d", len(client.indices))
	}

	if len(client.symbolIndex) != 0 {
		t.Errorf("Expected symbol index to be cleared, got: %d", len(client.symbolIndex))
	}

	if len(client.documentIndex) != 0 {
		t.Errorf("Expected document index to be cleared, got: %d", len(client.documentIndex))
	}
}

// TestReloadIndex tests index reloading functionality
func TestReloadIndex(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	scipIndex := createTestSCIPIndex()
	indexPath := createSCIPFile(t, scipIndex)
	defer os.Remove(indexPath)

	// Load initial index
	if err := client.LoadIndex(indexPath); err != nil {
		t.Fatalf("Failed to load initial index: %v", err)
	}

	initialLoadCount := client.stats.IndexLoads

	// Reload the same index
	if err := client.ReloadIndex(indexPath); err != nil {
		t.Fatalf("Failed to reload index: %v", err)
	}

	// Verify that load count increased
	if client.stats.IndexLoads != initialLoadCount+1 {
		t.Errorf("Expected load count to increase by 1, got: %d", client.stats.IndexLoads-initialLoadCount)
	}

	// Verify data is still accessible
	if _, err := client.GetDocumentData("main.go"); err != nil {
		t.Errorf("Expected to access document after reload, got: %v", err)
	}
}

// TestGetLoadedIndices tests loaded indices retrieval
func TestGetLoadedIndices(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Run("EmptyIndices", func(t *testing.T) {
		indices := client.GetLoadedIndices()
		if len(indices) != 0 {
			t.Errorf("Expected 0 loaded indices, got: %d", len(indices))
		}
	})

	t.Run("LoadedIndices", func(t *testing.T) {
		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		if err := client.LoadIndex(indexPath); err != nil {
			t.Fatalf("Failed to load test index: %v", err)
		}

		indices := client.GetLoadedIndices()
		if len(indices) != 1 {
			t.Errorf("Expected 1 loaded index, got: %d", len(indices))
		}

		// Verify that returned data is a copy
		for key, indexData := range indices {
			if indexData == client.indices[key] {
				t.Error("Expected returned index data to be a copy, not the original")
			}
		}
	})
}

// TestSCIPClientAdapter tests the adapter functionality
func TestSCIPClientAdapter(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	adapter := NewSCIPClientAdapter(client)

	t.Run("AdapterCreation", func(t *testing.T) {
		if adapter == nil {
			t.Fatal("Expected adapter to be created")
		}

		if adapter.client != client {
			t.Error("Expected adapter to reference the original client")
		}

		if adapter.cache == nil {
			t.Error("Expected adapter cache to be initialized")
		}
	})

	t.Run("LoadIndexThroughAdapter", func(t *testing.T) {
		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		if err := adapter.LoadIndex(indexPath); err != nil {
			t.Fatalf("Failed to load index through adapter: %v", err)
		}

		// Verify that the underlying client was updated
		if len(client.indices) != 1 {
			t.Errorf("Expected 1 index in underlying client, got: %d", len(client.indices))
		}
	})

	t.Run("AdapterStats", func(t *testing.T) {
		// Create a fresh client and adapter for this test
		freshClient, err := NewSCIPClient(createTestConfig())
		if err != nil {
			t.Fatalf("Failed to create fresh client: %v", err)
		}
		freshAdapter := NewSCIPClientAdapter(freshClient)

		scipIndex := createTestSCIPIndex()
		indexPath := createSCIPFile(t, scipIndex)
		defer os.Remove(indexPath)

		if err := freshAdapter.LoadIndex(indexPath); err != nil {
			t.Fatalf("Failed to load index through adapter: %v", err)
		}

		stats := freshAdapter.GetStats()
		if stats.IndexesLoaded != 1 {
			t.Errorf("Expected 1 loaded index in stats, got: %d", stats.IndexesLoaded)
		}

		if stats.MemoryUsage <= 0 {
			t.Errorf("Expected positive memory usage in adapter stats, got: %d", stats.MemoryUsage)
		}
	})

	t.Run("AdapterClose", func(t *testing.T) {
		if err := adapter.Close(); err != nil {
			t.Fatalf("Failed to close adapter: %v", err)
		}

		// Verify that cache was cleared
		if len(adapter.cache) != 0 {
			t.Errorf("Expected adapter cache to be cleared, got: %d items", len(adapter.cache))
		}
	})
}

// TestValidateSCIPConfig tests configuration validation
func TestValidateSCIPConfig(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := createTestConfig()
		if err := validateSCIPConfig(config); err != nil {
			t.Errorf("Expected valid config to pass validation, got: %v", err)
		}
	})

	t.Run("InvalidQueryTimeout", func(t *testing.T) {
		config := createTestConfig()
		config.Performance.QueryTimeout = -1
		
		if err := validateSCIPConfig(config); err == nil {
			t.Fatal("Expected error for negative query timeout")
		} else if !strings.Contains(err.Error(), "query timeout must be positive") {
			t.Errorf("Expected 'query timeout must be positive' error, got: %v", err)
		}
	})

	t.Run("InvalidMaxConcurrentQueries", func(t *testing.T) {
		config := createTestConfig()
		config.Performance.MaxConcurrentQueries = 0
		
		if err := validateSCIPConfig(config); err == nil {
			t.Fatal("Expected error for zero max concurrent queries")
		} else if !strings.Contains(err.Error(), "max concurrent queries must be positive") {
			t.Errorf("Expected 'max concurrent queries must be positive' error, got: %v", err)
		}
	})

	t.Run("InvalidIndexLoadTimeout", func(t *testing.T) {
		config := createTestConfig()
		config.Performance.IndexLoadTimeout = -1
		
		if err := validateSCIPConfig(config); err == nil {
			t.Fatal("Expected error for negative index load timeout")
		} else if !strings.Contains(err.Error(), "index load timeout must be positive") {
			t.Errorf("Expected 'index load timeout must be positive' error, got: %v", err)
		}
	})

	t.Run("InvalidCacheMaxSize", func(t *testing.T) {
		config := createTestConfig()
		config.CacheConfig.MaxSize = -1
		
		if err := validateSCIPConfig(config); err == nil {
			t.Fatal("Expected error for negative cache max size")
		} else if !strings.Contains(err.Error(), "cache max size cannot be negative") {
			t.Errorf("Expected 'cache max size cannot be negative' error, got: %v", err)
		}
	})

	t.Run("InvalidCacheTTL", func(t *testing.T) {
		config := createTestConfig()
		config.CacheConfig.TTL = -time.Hour
		
		if err := validateSCIPConfig(config); err == nil {
			t.Fatal("Expected error for negative cache TTL")
		} else if !strings.Contains(err.Error(), "cache TTL cannot be negative") {
			t.Errorf("Expected 'cache TTL cannot be negative' error, got: %v", err)
		}
	})
}

// TestConcurrentOperations tests thread safety
func TestConcurrentOperations(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	scipIndex := createTestSCIPIndex()
	indexPath := createSCIPFile(t, scipIndex)
	defer os.Remove(indexPath)

	if err := client.LoadIndex(indexPath); err != nil {
		t.Fatalf("Failed to load test index: %v", err)
	}

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			
			// Perform various read operations
			client.GetDocumentData("main.go")
			client.FindSymbolsByPrefix("main")
			client.GetSymbolByName("main.main")
			client.GetStats()
			client.IsHealthy()
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(time.Second * 5):
			t.Fatal("Timeout waiting for concurrent operations to complete")
		}
	}
}

// TestMemoryEstimation tests memory usage estimation
func TestMemoryEstimation(t *testing.T) {
	config := createTestConfig()
	client, err := NewSCIPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	initialMemory := client.estimateMemoryUsage()
	if initialMemory <= 0 {
		t.Errorf("Expected positive initial memory estimate, got: %d", initialMemory)
	}

	scipIndex := createTestSCIPIndex()
	indexPath := createSCIPFile(t, scipIndex)
	defer os.Remove(indexPath)

	if err := client.LoadIndex(indexPath); err != nil {
		t.Fatalf("Failed to load test index: %v", err)
	}

	loadedMemory := client.estimateMemoryUsage()
	if loadedMemory <= initialMemory {
		t.Errorf("Expected memory usage to increase after loading index, initial: %d, loaded: %d", initialMemory, loadedMemory)
	}
}