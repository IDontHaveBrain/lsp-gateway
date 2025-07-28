package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

type HttpConnectionTestSuite struct {
	suite.Suite
	httpClient    *testutils.HttpClient
	gatewayCmd    *exec.Cmd
	gatewayPort   int
	configPath    string
	tempDir       string
	projectRoot   string
	testTimeout   time.Duration
}

func (suite *HttpConnectionTestSuite) SetupSuite() {
	suite.testTimeout = 45 * time.Second
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err)
	
	suite.tempDir, err = os.MkdirTemp("", "http-connection-test-*")
	suite.Require().NoError(err)
	
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err)
	
	suite.createTestConfig()
}

func (suite *HttpConnectionTestSuite) SetupTest() {
	config := testutils.HttpClientConfig{
		BaseURL:         fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:         10 * time.Second,
		MaxRetries:      3,
		RetryDelay:      500 * time.Millisecond,
		EnableLogging:   true,
		EnableRecording: true,
		WorkspaceID:     "test-workspace",
		ProjectPath:     suite.tempDir,
	}
	suite.httpClient = testutils.NewHttpClient(config)
}

func (suite *HttpConnectionTestSuite) TearDownTest() {
	if suite.httpClient != nil {
		suite.httpClient.Close()
	}
}

func (suite *HttpConnectionTestSuite) TearDownSuite() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *HttpConnectionTestSuite) createTestConfig() {
	configContent := fmt.Sprintf(`
servers:
- name: test-lsp
  languages:
  - go
  command: echo
  args: ["test"]
  transport: stdio
  root_markers:
  - go.mod
  priority: 1
  weight: 1.0
port: %d
timeout: 30s
max_concurrent_requests: 50
`, suite.gatewayPort)

	var err error
	suite.configPath, _, err = testutils.CreateTempConfig(configContent)
	suite.Require().NoError(err)
}

func (suite *HttpConnectionTestSuite) startGatewayServer() {
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lsp-gateway")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot
	
	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err)
	
	time.Sleep(2 * time.Second)
}

func (suite *HttpConnectionTestSuite) TestBasicConnectionLifecycle() {
	suite.T().Run("ServerStartupAndHealthCheck", func(t *testing.T) {
		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.Eventually(func() bool {
			err := suite.httpClient.HealthCheck(ctx)
			return err == nil
		}, 15*time.Second, 500*time.Millisecond, "Health check should eventually succeed")

		err := suite.httpClient.HealthCheck(ctx)
		suite.NoError(err, "Health check should succeed after server startup")

		metrics := suite.httpClient.GetMetrics()
		suite.Greater(metrics.TotalRequests, 0, "Should have recorded requests")
		suite.Greater(metrics.SuccessfulReqs, 0, "Should have successful requests")
	})

	suite.T().Run("ConnectionValidation", func(t *testing.T) {
		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		time.Sleep(3 * time.Second)

		err := suite.httpClient.ValidateConnection(ctx)
		suite.NoError(err, "Connection validation should succeed")

		recordings := suite.httpClient.GetRecordings()
		suite.Greater(len(recordings), 0, "Should have recorded requests")

		for _, recording := range recordings {
			suite.Contains(recording.Headers, "Content-Type", "Should have content type header")
			suite.Equal("application/json", recording.Headers["Content-Type"])
			suite.Contains(recording.Headers, "User-Agent", "Should have user agent header")
		}
	})
}

func (suite *HttpConnectionTestSuite) TestConnectionTimeouts() {
	suite.T().Run("ConnectionTimeoutHandling", func(t *testing.T) {
		shortTimeoutConfig := testutils.HttpClientConfig{
			BaseURL:    fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:    100 * time.Millisecond,
			MaxRetries: 1,
			RetryDelay: 50 * time.Millisecond,
		}
		shortTimeoutClient := testutils.NewHttpClient(shortTimeoutConfig)
		defer shortTimeoutClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := shortTimeoutClient.HealthCheck(ctx)
		suite.Error(err, "Should timeout when server is not running")

		metrics := shortTimeoutClient.GetMetrics()
		suite.Greater(metrics.ConnectionErrors, 0, "Should record connection errors")
	})

	suite.T().Run("RequestTimeoutWithRunningServer", func(t *testing.T) {
		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		time.Sleep(3 * time.Second)

		extremelyShortConfig := testutils.HttpClientConfig{
			BaseURL:    fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:    1 * time.Millisecond,
			MaxRetries: 1,
			RetryDelay: 1 * time.Millisecond,
		}
		extremelyShortClient := testutils.NewHttpClient(extremelyShortConfig)
		defer extremelyShortClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := extremelyShortClient.HealthCheck(ctx)
		suite.Error(err, "Should timeout with extremely short timeout")

		metrics := extremelyShortClient.GetMetrics()
		suite.Greater(metrics.TimeoutErrors, 0, "Should record timeout errors")
	})
}

func (suite *HttpConnectionTestSuite) TestInvalidConnections() {
	suite.T().Run("InvalidPortConnection", func(t *testing.T) {
		invalidPort, err := testutils.FindAvailablePort()
		suite.Require().NoError(err)

		invalidConfig := testutils.HttpClientConfig{
			BaseURL:    fmt.Sprintf("http://localhost:%d", invalidPort),
			Timeout:    2 * time.Second,
			MaxRetries: 1,
			RetryDelay: 100 * time.Millisecond,
		}
		invalidClient := testutils.NewHttpClient(invalidConfig)
		defer invalidClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = invalidClient.HealthCheck(ctx)
		suite.Error(err, "Should fail to connect to invalid port")

		metrics := invalidClient.GetMetrics()
		suite.Greater(metrics.ConnectionErrors, 0, "Should record connection errors")
		suite.Equal(0, metrics.SuccessfulReqs, "Should have no successful requests")
	})

	suite.T().Run("InvalidURLConnection", func(t *testing.T) {
		invalidConfig := testutils.HttpClientConfig{
			BaseURL:    "http://invalid-hostname-that-does-not-exist:8080",
			Timeout:    2 * time.Second,
			MaxRetries: 1,
			RetryDelay: 100 * time.Millisecond,
		}
		invalidClient := testutils.NewHttpClient(invalidConfig)
		defer invalidClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := invalidClient.HealthCheck(ctx)
		suite.Error(err, "Should fail to connect to invalid hostname")

		metrics := invalidClient.GetMetrics()
		suite.Greater(metrics.ConnectionErrors, 0, "Should record connection errors")
	})
}

func (suite *HttpConnectionTestSuite) TestConnectionPooling() {
	suite.T().Run("ConnectionPoolingAndReuse", func(t *testing.T) {
		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		time.Sleep(3 * time.Second)

		poolConfig := testutils.HttpClientConfig{
			BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:            10 * time.Second,
			MaxRetries:         2,
			RetryDelay:         100 * time.Millisecond,
			ConnectionPoolSize: 5,
			KeepAlive:          30 * time.Second,
		}
		poolClient := testutils.NewHttpClient(poolConfig)
		defer poolClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		for i := 0; i < 10; i++ {
			err := poolClient.HealthCheck(ctx)
			suite.NoError(err, fmt.Sprintf("Health check %d should succeed", i))
		}

		metrics := poolClient.GetMetrics()
		suite.Equal(10, metrics.TotalRequests, "Should have 10 total requests")
		suite.Equal(10, metrics.SuccessfulReqs, "Should have 10 successful requests")
		suite.Greater(metrics.AverageLatency, time.Duration(0), "Should have recorded latency")
	})

	suite.T().Run("ConcurrentConnectionHandling", func(t *testing.T) {
		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		time.Sleep(3 * time.Second)

		concurrentConfig := testutils.HttpClientConfig{
			BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:            15 * time.Second,
			MaxRetries:         2,
			RetryDelay:         100 * time.Millisecond,
			ConnectionPoolSize: 10,
			KeepAlive:          30 * time.Second,
		}
		concurrentClient := testutils.NewHttpClient(concurrentConfig)
		defer concurrentClient.Close()

		numWorkers := 5
		requestsPerWorker := 10
		var wg sync.WaitGroup
		errorChan := make(chan error, numWorkers*requestsPerWorker)

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < requestsPerWorker; j++ {
					err := concurrentClient.HealthCheck(ctx)
					if err != nil {
						errorChan <- err
					}
				}
			}()
		}

		wg.Wait()
		close(errorChan)

		errorCount := 0
		for err := range errorChan {
			suite.T().Logf("Concurrent request error: %v", err)
			errorCount++
		}

		metrics := concurrentClient.GetMetrics()
		suite.T().Logf("Concurrent test metrics: Total: %d, Successful: %d, Failed: %d, Avg Latency: %v",
			metrics.TotalRequests, metrics.SuccessfulReqs, metrics.FailedRequests, metrics.AverageLatency)

		suite.LessOrEqual(errorCount, 5, "Should have minimal errors in concurrent requests")
		suite.GreaterOrEqual(metrics.SuccessfulReqs, 40, "Should have majority of requests succeed")
	})
}

func (suite *HttpConnectionTestSuite) TestServerLifecycleHandling() {
	suite.T().Run("ServerRestartRecovery", func(t *testing.T) {
		suite.startGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		time.Sleep(2 * time.Second)

		err := suite.httpClient.HealthCheck(ctx)
		suite.NoError(err, "Health check should succeed initially")

		if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
			suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
			suite.gatewayCmd.Wait()
		}

		time.Sleep(1 * time.Second)

		err = suite.httpClient.HealthCheck(ctx)
		suite.Error(err, "Health check should fail after server shutdown")

		suite.startGatewayServer()
		time.Sleep(3 * time.Second)

		suite.Eventually(func() bool {
			err := suite.httpClient.HealthCheck(ctx)
			return err == nil
		}, 10*time.Second, 500*time.Millisecond, "Health check should recover after server restart")

		metrics := suite.httpClient.GetMetrics()
		suite.Greater(metrics.ConnectionErrors, 0, "Should have recorded connection errors during downtime")
		suite.Greater(metrics.SuccessfulReqs, 0, "Should have successful requests after recovery")
	})

	suite.T().Run("GracefulShutdownHandling", func(t *testing.T) {
		suite.startGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		time.Sleep(2 * time.Second)

		err := suite.httpClient.HealthCheck(ctx)
		suite.NoError(err, "Health check should succeed before shutdown")

		if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
			suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
			
			done := make(chan error, 1)
			go func() {
				done <- suite.gatewayCmd.Wait()
			}()

			select {
			case <-done:
				suite.T().Log("Server shutdown gracefully")
			case <-time.After(10 * time.Second):
				suite.T().Log("Server shutdown timeout, force killing")
				suite.gatewayCmd.Process.Kill()
			}
		}

		time.Sleep(1 * time.Second)

		err = suite.httpClient.HealthCheck(ctx)
		suite.Error(err, "Health check should fail after graceful shutdown")
	})
}

func (suite *HttpConnectionTestSuite) TestConnectionErrorScenarios() {
	suite.T().Run("InvalidJSONRPCRequests", func(t *testing.T) {
		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		time.Sleep(3 * time.Second)

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		_, err := suite.httpClient.WorkspaceSymbol(ctx, "")
		suite.Error(err, "Empty workspace symbol query should result in error")

		invalidPosition := testutils.Position{Line: -1, Character: -1}
		_, err = suite.httpClient.Definition(ctx, "file:///nonexistent.go", invalidPosition)
		suite.Error(err, "Invalid position should result in error")

		metrics := suite.httpClient.GetMetrics()
		suite.Greater(metrics.FailedRequests, 0, "Should record failed requests")
	})

	suite.T().Run("MalformedResponseHandling", func(t *testing.T) {
		malformedConfig := testutils.HttpClientConfig{
			BaseURL:         fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:         5 * time.Second,
			MaxRetries:      1,
			RetryDelay:      100 * time.Millisecond,
			MaxResponseSize: 100,
		}
		malformedClient := testutils.NewHttpClient(malformedConfig)
		defer malformedClient.Close()

		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		time.Sleep(3 * time.Second)

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		err := malformedClient.HealthCheck(ctx)
		suite.NoError(err, "Health check should work with size limit")

		_, err = malformedClient.WorkspaceSymbol(ctx, "test")
		if err != nil {
			suite.T().Logf("Expected potential error with response size limit: %v", err)
		}
	})
}

func (suite *HttpConnectionTestSuite) TestPerformanceUnderLoad() {
	suite.T().Run("PerformanceUnderLoad", func(t *testing.T) {
		suite.startGatewayServer()
		defer func() {
			if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
				suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
				suite.gatewayCmd.Wait()
			}
		}()

		time.Sleep(3 * time.Second)

		loadConfig := testutils.HttpClientConfig{
			BaseURL:            fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:            20 * time.Second,
			MaxRetries:         2,
			RetryDelay:         100 * time.Millisecond,
			ConnectionPoolSize: 20,
			KeepAlive:          60 * time.Second,
		}
		loadClient := testutils.NewHttpClient(loadConfig)
		defer loadClient.Close()

		numWorkers := 10
		requestsPerWorker := 20
		var wg sync.WaitGroup
		errorChan := make(chan error, numWorkers*requestsPerWorker)
		latencies := make(chan time.Duration, numWorkers*requestsPerWorker)

		ctx, cancel := context.WithTimeout(context.Background(), 2*suite.testTimeout)
		defer cancel()

		startTime := time.Now()

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < requestsPerWorker; j++ {
					requestStart := time.Now()
					err := loadClient.HealthCheck(ctx)
					requestLatency := time.Since(requestStart)
					latencies <- requestLatency
					if err != nil {
						errorChan <- err
					}
				}
			}()
		}

		wg.Wait()
		totalDuration := time.Since(startTime)
		close(errorChan)
		close(latencies)

		errorCount := 0
		for err := range errorChan {
			suite.T().Logf("Load test error: %v", err)
			errorCount++
		}

		var totalLatency time.Duration
		var maxLatency time.Duration
		var minLatency time.Duration = time.Hour
		latencyCount := 0

		for latency := range latencies {
			totalLatency += latency
			latencyCount++
			if latency > maxLatency {
				maxLatency = latency
			}
			if latency < minLatency {
				minLatency = latency
			}
		}

		avgLatency := totalLatency / time.Duration(latencyCount)
		throughput := float64(numWorkers*requestsPerWorker) / totalDuration.Seconds()

		suite.T().Logf("Load test results:")
		suite.T().Logf("  Total requests: %d", numWorkers*requestsPerWorker)
		suite.T().Logf("  Total duration: %v", totalDuration)
		suite.T().Logf("  Throughput: %.2f req/s", throughput)
		suite.T().Logf("  Error count: %d", errorCount)
		suite.T().Logf("  Avg latency: %v", avgLatency)
		suite.T().Logf("  Min latency: %v", minLatency)
		suite.T().Logf("  Max latency: %v", maxLatency)

		suite.LessOrEqual(errorCount, 10, "Should have minimal errors under load")
		suite.Greater(throughput, 10.0, "Should maintain reasonable throughput")
		suite.Less(avgLatency, 2*time.Second, "Average latency should be reasonable")

		metrics := loadClient.GetMetrics()
		suite.Equal(numWorkers*requestsPerWorker, metrics.TotalRequests, "Should match expected request count")
		suite.GreaterOrEqual(metrics.SuccessfulReqs, numWorkers*requestsPerWorker-errorCount, "Successful requests should match")
	})
}

func TestHttpConnectionTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP connection tests in short mode")
	}
	suite.Run(t, new(HttpConnectionTestSuite))
}