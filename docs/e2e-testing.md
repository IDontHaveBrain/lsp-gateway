# E2E Testing Guide

LSP Gateway의 End-to-End 테스트 가이드입니다. 실제 사용 시나리오를 기반으로 한 포괄적인 테스트 전략을 제공합니다.

## E2E 테스트 철학

LSP Gateway는 dual-protocol (HTTP JSON-RPC + MCP) 아키텍처를 가지므로, E2E 테스트는 다음 원칙을 따릅니다:

- **실제 워크플로우 중심**: 개발자가 실제로 사용하는 시나리오 테스트
- **Protocol 독립적**: HTTP와 MCP 두 프로토콜 모두 동일한 결과 보장
- **Language Server 통합**: 실제 언어 서버와의 완전한 통합 테스트
- **Real Codebase 검증**: Kubernetes, Django, VS Code 등 실제 프로젝트 대상 테스트

## 핵심 E2E 테스트 시나리오

### 1. 기본 설정 및 시작 테스트
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

### 2. HTTP JSON-RPC Protocol 테스트
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

### 3. MCP Protocol 테스트
```bash
# Start MCP server
./bin/lsp-gateway mcp --config config.yaml &
MCP_PID=$!

# Test via claude-desktop or MCP client
# (MCP protocol uses stdio/JSON-RPC over stdin/stdout)

# Cleanup  
kill $MCP_PID
```

### 4. Multi-Language Server 테스트
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

## 실제 코드베이스 테스트

### Repository-based 테스트 실행
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

## 성능 및 부하 테스트

### Concurrent Request 테스트
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

### Memory 및 Circuit Breaker 테스트
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

## 자동화된 E2E 테스트 실행

### Make Targets를 통한 E2E 테스트
```bash
# Quick E2E validation (1분)
make test-simple-quick

# Full LSP validation (5분)
make test-lsp-validation

# Integration tests with performance (10분)
make test-integration

# JDTLS specific E2E tests (10분)
make test-jdtls-integration
```

### CI/CD 통합을 위한 스크립트
```bash
#!/bin/bash
# e2e-ci.sh - CI/CD pipeline용 E2E 테스트

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

## 디버깅 및 문제해결

### E2E 테스트 실패시 진단
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

### 로그 분석
```bash
# Server logs with debug level
./bin/lsp-gateway server --config config.yaml --verbose

# MCP server logs
./bin/lsp-gateway mcp --config config.yaml --verbose

# Circuit breaker status in logs
grep -i "circuit" server.log
grep -i "backoff" server.log
```

## E2E 테스트 확장

### 새로운 Language Server 추가시 E2E 테스트
1. Language runtime 설치 테스트
2. LSP server 설치 및 검증 테스트  
3. 기본 LSP 메소드 동작 테스트 (hover, definition, references)
4. 실제 코드베이스 대상 통합 테스트
5. 성능 및 메모리 사용량 테스트

### 새로운 Transport Method 추가시 E2E 테스트
1. Transport 연결 설정 테스트
2. Request/Response 순환 테스트
3. Circuit breaker 동작 테스트
4. 동시 연결 처리 테스트
5. 장애 복구 테스트

이 E2E 테스트 가이드를 통해 LSP Gateway의 모든 주요 기능과 실제 사용 시나리오를 포괄적으로 검증할 수 있습니다.