# E2E Testing Guide

This is the End-to-End testing guide for LSP Gateway. It provides a comprehensive testing strategy based on real-world usage scenarios.

## E2E Testing Philosophy

Since LSP Gateway has a dual-protocol (HTTP JSON-RPC + MCP) architecture, E2E testing follows these principles:

- **Real Workflow-Centric**: Testing scenarios that developers actually use
- **Protocol-Independent**: Ensuring identical results across both HTTP and MCP protocols
- **Language Server Integration**: Complete integration testing with actual language servers
- **Real Codebase Validation**: Testing against actual projects like Kubernetes, Django, VS Code

## Core E2E Test Scenarios

### 1. Basic Setup and Startup Testing
```bash
# Complete setup and server start
make clean && make local
./bin/lsp-gateway setup all
./bin/lsp-gateway server --config config.yaml &
SERVER_PID=$!

# Health check
curl -f http://localhost:8080/health || exit 1

# Cleanup
kill $SERVER_PID
```

### 2. HTTP JSON-RPC Protocol Testing
```bash
# Go to definition test
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "textDocument/definition",
    "params": {
      "textDocument": {"uri": "file:///path/to/main.go"},
      "position": {"line": 10, "character": 5}
    }
  }' | jq '.result'

# References test
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "textDocument/references",
    "params": {
      "textDocument": {"uri": "file:///path/to/main.go"},
      "position": {"line": 15, "character": 8},
      "context": {"includeDeclaration": true}
    }
  }' | jq '.result'
```

### 3. MCP Protocol Testing
```bash
# Start MCP server
./bin/lsp-gateway mcp --config config.yaml &
MCP_PID=$!

# Test via claude-desktop or MCP client
# (MCP protocol uses stdio/JSON-RPC over stdin/stdout)

# Cleanup  
kill $MCP_PID
```

### 4. Multi-Language Server Testing
```bash
# Test all supported languages
for lang in go python typescript java; do
  echo "Testing $lang language server..."
  
  # Create test project
  mkdir -p test-$lang
  cd test-$lang
  
  case $lang in
    go) echo 'package main\nfunc main() {}' > main.go ;;
    python) echo 'def main():\n    pass' > main.py ;;
    typescript) echo 'function main() {}' > main.ts ;;
    java) echo 'public class Main { public static void main(String[] args) {} }' > Main.java ;;
  esac
  
  # Test LSP functionality
  curl -X POST http://localhost:8080/jsonrpc \
    -H "Content-Type: application/json" \
    -d "{
      \"jsonrpc\": \"2.0\",
      \"id\": 1,
      \"method\": \"textDocument/hover\",
      \"params\": {
        \"textDocument\": {\"uri\": \"file://$(pwd)/main.*\"},
        \"position\": {\"line\": 0, \"character\": 5}
      }
    }" | jq '.result'
  
  cd ..
  rm -rf test-$lang
done
```

## Real Codebase Testing

### Repository-based Test Execution
```bash
# Setup test repositories (requires network)
make setup-simple-repos

# Run comprehensive validation
make test-lsp-repos

# Test specific repository
./bin/lsp-gateway server --config config.yaml &
SERVER_PID=$!

# Test against Kubernetes codebase
cd test-repos/kubernetes
for file in $(find . -name "*.go" | head -5); do
  echo "Testing $file..."
  curl -X POST http://localhost:8080/jsonrpc \
    -H "Content-Type: application/json" \
    -d "{
      \"jsonrpc\": \"2.0\",
      \"id\": 1,
      \"method\": \"textDocument/documentSymbol\",
      \"params\": {
        \"textDocument\": {\"uri\": \"file://$(realpath $file)\"}
      }
    }" | jq '.result | length'
done

kill $SERVER_PID
```

## Performance and Load Testing

### Concurrent Request Testing
```bash
# Start server
./bin/lsp-gateway server --config config.yaml &
SERVER_PID=$!

# Test concurrent requests
for i in {1..10}; do
  curl -X POST http://localhost:8080/jsonrpc \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": '$i',
      "method": "textDocument/hover",
      "params": {
        "textDocument": {"uri": "file:///path/to/file.go"},
        "position": {"line": 5, "character": 10}
      }
    }' &
done

wait
kill $SERVER_PID
```

### Memory and Circuit Breaker Testing
```bash
# Start server with resource monitoring
./bin/lsp-gateway server --config config.yaml &
SERVER_PID=$!

# Monitor memory usage
while kill -0 $SERVER_PID 2>/dev/null; do
  ps -p $SERVER_PID -o pid,vsz,rss,comm
  sleep 5
done &
MONITOR_PID=$!

# Generate load to trigger circuit breaker
for i in {1..100}; do
  curl -X POST http://localhost:8080/jsonrpc \
    -H "Content-Type: application/json" \
    -d '{
      "jsonrpc": "2.0",
      "id": '$i',
      "method": "workspace/symbol",
      "params": {"query": "nonexistent"}
    }' &
  
  if (( i % 10 == 0 )); then
    wait
  fi
done

kill $MONITOR_PID $SERVER_PID
```

## Automated E2E Test Execution

### E2E Testing via Make Targets
```bash
# Quick E2E validation (1 minute)
make test-simple-quick

# Full LSP validation (5 minutes)
make test-lsp-validation

# Integration tests with performance (10 minutes)
make test-integration

# JDTLS specific E2E tests (10 minutes)
make test-jdtls-integration
```

### CI/CD Integration Script
```bash
#!/bin/bash
# e2e-ci.sh - E2E testing for CI/CD pipeline

set -e

echo "Starting E2E tests..."

# 1. Build and setup
make clean && make local
./bin/lsp-gateway setup all

# 2. Start server in background
./bin/lsp-gateway server --config config.yaml &
SERVER_PID=$!

# 3. Wait for server to be ready
sleep 5
curl -f http://localhost:8080/health || exit 1

# 4. Run core E2E tests
make test-simple-quick

# 5. Protocol-specific tests
echo "Testing HTTP JSON-RPC protocol..."
curl -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{"textDocument":{"uri":"file:///tmp/test.go"},"position":{"line":0,"character":0}}}' \
  | jq '.result' > /dev/null

# 6. Multi-language validation
for lang in go python typescript; do
  echo "Validating $lang language server..."
  ./bin/lsp-gateway status server $lang-lsp || exit 1
done

# 7. Performance check
echo "Running performance validation..."
time make test-lsp-validation-short

# 8. Cleanup
kill $SERVER_PID
echo "E2E tests completed successfully!"
```

## Debugging and Troubleshooting

### E2E Test Failure Diagnosis
```bash
# 1. System diagnostics
./bin/lsp-gateway diagnose

# 2. Server status check
./bin/lsp-gateway status

# 3. Runtime verification
./bin/lsp-gateway verify runtime all

# 4. Configuration validation
./bin/lsp-gateway config validate

# 5. Health monitoring
./scripts/health-check.sh

# 6. Manual protocol test
curl -v -X POST http://localhost:8080/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"ping"}'
```

### Log Analysis
```bash
# Server logs with debug level
./bin/lsp-gateway server --config config.yaml --verbose

# MCP server logs
./bin/lsp-gateway mcp --config config.yaml --verbose

# Circuit breaker status in logs
grep -i "circuit" server.log
grep -i "backoff" server.log
```

## E2E Test Extensions

### E2E Testing When Adding New Language Servers
1. Language runtime installation testing
2. LSP server installation and verification testing  
3. Basic LSP method operation testing (hover, definition, references)
4. Integration testing against real codebases
5. Performance and memory usage testing

### E2E Testing When Adding New Transport Methods
1. Transport connection setup testing
2. Request/Response cycle testing
3. Circuit breaker operation testing
4. Concurrent connection handling testing
5. Failure recovery testing

This E2E testing guide enables comprehensive validation of all major LSP Gateway features and real-world usage scenarios.
