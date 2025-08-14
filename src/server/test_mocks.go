package server

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/scip"
)

// LSPManagerInterface defines the methods that the HTTPGateway uses from LSPManager
type LSPManagerInterface interface {
	Start(ctx context.Context) error
	Stop() error
	ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
	GetClientStatus() interface{}
	GetCache() cache.SCIPCache
}

// MockSCIPCache provides a shared mock implementation for SCIP Cache testing
type MockSCIPCache struct {
	mock.Mock
	enabled bool
	metrics *cache.CacheMetrics
}

func NewMockSCIPCache() *MockSCIPCache {
	return &MockSCIPCache{
		enabled: true,
		metrics: &cache.CacheMetrics{
			HitCount:        10,
			MissCount:       5,
			ErrorCount:      0,
			EvictionCount:   1,
			TotalSize:       1024 * 1024,
			EntryCount:      50,
			AverageHitTime:  time.Millisecond * 5,
			AverageMissTime: time.Millisecond * 50,
		},
	}
}

func (m *MockSCIPCache) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSCIPCache) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSCIPCache) Lookup(method string, params interface{}) (interface{}, bool, error) {
	args := m.Called(method, params)
	return args.Get(0), args.Bool(1), args.Error(2)
}

func (m *MockSCIPCache) Store(method string, params interface{}, result interface{}) error {
	args := m.Called(method, params, result)
	return args.Error(0)
}

func (m *MockSCIPCache) InvalidateDocument(uri string) error {
	args := m.Called(uri)
	return args.Error(0)
}

func (m *MockSCIPCache) HealthCheck() (*cache.CacheMetrics, error) {
	args := m.Called()
	return args.Get(0).(*cache.CacheMetrics), args.Error(1)
}

func (m *MockSCIPCache) GetMetrics() *cache.CacheMetrics {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*cache.CacheMetrics)
}

func (m *MockSCIPCache) Clear() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockSCIPCache) SetMetrics(metrics *cache.CacheMetrics) {
	m.metrics = metrics
}

func (m *MockSCIPCache) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// Additional mock methods for complete interface
func (m *MockSCIPCache) IndexDocument(ctx context.Context, uri string, language string, symbols []types.SymbolInformation) error {
	args := m.Called(ctx, uri, language, symbols)
	return args.Error(0)
}

func (m *MockSCIPCache) QueryIndex(ctx context.Context, query *cache.IndexQuery) (*cache.IndexResult, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(*cache.IndexResult), args.Error(1)
}

func (m *MockSCIPCache) GetIndexStats() *cache.IndexStats {
	args := m.Called()
	return args.Get(0).(*cache.IndexStats)
}

func (m *MockSCIPCache) UpdateIndex(ctx context.Context, files []string) error {
	args := m.Called(ctx, files)
	return args.Error(0)
}

func (m *MockSCIPCache) SearchSymbols(ctx context.Context, pattern string, filePattern string, maxResults int) ([]interface{}, error) {
	args := m.Called(ctx, pattern, filePattern, maxResults)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockSCIPCache) SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	args := m.Called(ctx, symbolName, filePattern, maxResults)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockSCIPCache) SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error) {
	args := m.Called(ctx, symbolName, filePattern, maxResults)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockSCIPCache) GetSymbolInfo(ctx context.Context, symbolName string, filePattern string) (interface{}, error) {
	args := m.Called(ctx, symbolName, filePattern)
	return args.Get(0), args.Error(1)
}

func (m *MockSCIPCache) GetSCIPStorage() scip.SCIPDocumentStorage {
	args := m.Called()
	return args.Get(0).(scip.SCIPDocumentStorage)
}

// MockLSPManagerForGateway provides a mock LSP Manager specifically for HTTPGateway testing
type MockLSPManagerForGateway struct {
	mock.Mock
	scipCache cache.SCIPCache
	started   bool
}

func NewMockLSPManagerForGateway() *MockLSPManagerForGateway {
	return &MockLSPManagerForGateway{
		scipCache: NewMockSCIPCache(),
		started:   false,
	}
}

func (m *MockLSPManagerForGateway) Start(ctx context.Context) error {
	args := m.Called(ctx)
	m.started = true
	return args.Error(0)
}

func (m *MockLSPManagerForGateway) Stop() error {
	args := m.Called()
	m.started = false
	return args.Error(0)
}

func (m *MockLSPManagerForGateway) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	args := m.Called(ctx, method, params)
	return args.Get(0), args.Error(1)
}

func (m *MockLSPManagerForGateway) GetClientStatus() interface{} {
	args := m.Called()
	return args.Get(0)
}

func (m *MockLSPManagerForGateway) GetCache() cache.SCIPCache {
	return m.scipCache
}

func (m *MockLSPManagerForGateway) SetCache(cache cache.SCIPCache) {
	m.scipCache = cache
}
