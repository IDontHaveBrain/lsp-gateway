# Test Guide

This guide covers the streamlined testing strategies for LSP Gateway, focusing on essential functionality with minimal overhead.

## Testing Philosophy

LSP Gateway uses a **simplified, essential-only** testing approach:
- **Unit Tests**: Core logic only - no integration testing disguised as unit tests
- **E2E Tests**: Real-world usage scenarios only - no enterprise-scale simulation
- **Simplified Infrastructure**: Basic mocks and fixtures without over-engineering
- **Focus**: Essential functionality that supports local development workflows

## Quick Start

### Unit Tests (Fast - <60s)
```bash
make test-unit                # Fast unit tests only
go test ./internal/...        # Direct unit test execution
```

### E2E Tests (Essential Scenarios)
```bash
# Quick E2E Tests (<5 minutes)
make test-simple-quick        # Quick E2E validation (1min)
make test-e2e-quick          # Quick E2E test suite (5min)
make test-lsp-validation-short # Short LSP validation (2min)

# Standard E2E Tests (5-15 minutes)
make test-lsp-validation      # Full LSP validation (5min)
make test-e2e-setup-cli       # Setup CLI E2E tests (5min)
make test-e2e-workflow        # E2E workflow tests (15min)

# Comprehensive E2E Tests (15-30 minutes)
make test-e2e-full           # Full E2E test suite (30min)
make test-e2e-advanced       # Advanced E2E scenarios (20min)
```

### Language-Specific Tests (10-15 minutes)
```bash
make test-e2e-java          # Java E2E tests with JDTLS (15min)
make test-e2e-python        # Python E2E tests with pylsp (10min)
make test-e2e-typescript    # TypeScript E2E tests (10min)
make test-e2e-go            # Go E2E tests with gopls (10min)
```

### Real Language Server Integration (10-15 minutes)
```bash
make test-java-real         # Java real JDTLS integration (15min)
make test-python-real       # Python real pylsp integration (10min)
make test-typescript-real   # TypeScript real server integration (10min)
make test-jdtls-integration # Comprehensive JDTLS testing (15min)
```

### MCP Protocol Tests (5-15 minutes)
```bash
make test-e2e-mcp          # Comprehensive MCP E2E tests (10min)
make test-mcp-stdio        # MCP STDIO protocol tests (5min)
make test-mcp-tcp          # MCP TCP protocol tests (5min)
make test-mcp-tools        # MCP tools E2E tests (10min)
make test-mcp-scip         # SCIP-enhanced MCP tests (15min)
```

### NPM Integration Tests (5-10 minutes)
```bash
make test-npm-cli          # NPM CLI E2E tests (10min)
make test-npm-mcp          # Comprehensive NPM-MCP tests (10min)
make test-npm-mcp-quick    # Quick NPM-MCP tests (5min)
make test-npm-mcp-js       # NPM-MCP JavaScript tests only (5min)
make test-npm-mcp-go       # NPM-MCP Go integration tests (5min)
```

### Circuit Breaker Tests (5-10 minutes)
```bash
make test-circuit-breaker              # Circuit breaker E2E tests (5min)
make test-circuit-breaker-comprehensive # Comprehensive scenarios (10min)
```

## Test Categories

### Unit Tests
Simplified unit tests cover only essential logic:
- **Core Gateway Logic** (`internal/gateway/`): Request routing and protocol handling
- **Configuration System** (`internal/config/`): Config validation and loading  
- **Transport Layer** (`internal/transport/`): Basic connection management
- **Project Detection** (`internal/project/`): Language detection core functionality

**Location**: Co-located with source code (`*_test.go` files)
**Approach**: Standard Go testing, no complex test suites or frameworks

### E2E Tests
Essential E2E scenarios only:
- **Basic LSP Workflow**: Definition, references, hover for real development scenarios
- **Multi-Language Support**: Go, Python, TypeScript integration with actual language servers
- **Protocol Validation**: HTTP JSON-RPC and MCP protocol basics
- **Setup CLI Testing**: Comprehensive binary execution tests for setup automation
- **Simple Error Handling**: Basic failure recovery without complex simulation

**Location**: `tests/e2e/` and `tests/integration/` directories
**Approach**: Real language server integration, no synthetic project generation

### Setup CLI E2E Tests
Comprehensive binary execution tests for setup automation:
- **Real Binary Testing**: Execute actual compiled binary with various command combinations
- **JSON Output Validation**: Parse and validate complex JSON response structures  
- **Command Coverage**: Tests setup all, detect, wizard, template commands with real scenarios
- **Error Handling**: Validates proper error responses for invalid inputs and edge cases
- **Project Integration**: Tests with real project structures (Go, Python, etc.)

**Location**: `tests/e2e/setup_cli_e2e_test.go`
**Approach**: Direct binary execution with comprehensive JSON parsing and validation

## Test Commands

### Development Workflows

#### Quick Development Cycle (<5 minutes)
```bash
# Fastest validation for immediate feedback
make test-unit && make test-simple-quick

# Quick language-specific development
make test-unit && make test-e2e-python      # Python development
make test-unit && make test-e2e-typescript  # TypeScript development
make test-unit && make test-e2e-go          # Go development

# Quick MCP development
make test-unit && make test-mcp-stdio
```

#### Pre-commit Validation (5-10 minutes)
```bash
# Standard pre-commit checks
make test-unit && make test-lsp-validation-short

# Language-specific pre-commit
make test-unit && make test-e2e-java         # Java development
make test-unit && make test-python-real      # Python with real server
make test-unit && make test-typescript-real  # TypeScript with real server

# MCP feature development
make test-unit && make test-e2e-mcp

# NPM integration development
make test-unit && make test-npm-mcp-quick
```

#### Full Validation Before PR (15-30 minutes)
```bash
# Comprehensive validation
make test-unit && make test-lsp-validation && make test-e2e-advanced

# Language-specific comprehensive testing
make test-unit && make test-java-real && make test-jdtls-integration

# MCP comprehensive validation
make test-unit && make test-e2e-mcp && make test-mcp-scip

# NPM integration comprehensive testing
make test-unit && make test-npm-mcp && make test-npm-cli

# Full end-to-end validation (30+ minutes)
make test-unit && make test-e2e-full
```

#### Specialized Development Workflows
```bash
# Circuit breaker development
make test-unit && make test-circuit-breaker-comprehensive

# SCIP caching development
make test-unit && make test-mcp-scip

# Setup CLI development
make test-unit && make test-e2e-setup-cli

# NPM-specific development
make test-npm-mcp-js      # JavaScript focus
make test-npm-mcp-go      # Go integration focus
```

### Test Infrastructure Validation
```bash
# Validate all test infrastructure
make test-unit           # Validate core testing framework
make test-simple-quick   # Validate basic E2E infrastructure
make test-e2e-quick     # Validate comprehensive E2E framework

# Validate specific test infrastructure components
go test -v ./tests/unit/...        # Unit test infrastructure
go test -v ./tests/integration/... # Integration test infrastructure
go test -v ./tests/e2e/...         # E2E test infrastructure
```

### Complete Test Command Reference

#### Unit Tests (<60 seconds)
```bash
make test-unit               # Fast unit tests only
go test ./internal/...       # Direct unit test execution
```

#### Quick Development Tests (<5 minutes)
```bash
make test-simple-quick       # Quick E2E validation (1min)
make test-e2e-quick         # Quick E2E test suite (5min)
make test-lsp-validation-short # Short LSP validation (2min)
make test-npm-mcp-quick     # Quick NPM-MCP tests (5min)
make test-mcp-stdio         # MCP STDIO protocol tests (5min)
make test-mcp-tcp           # MCP TCP protocol tests (5min)
make test-circuit-breaker   # Circuit breaker E2E tests (5min)
```

#### Standard E2E Tests (5-15 minutes)
```bash
make test-lsp-validation    # Full LSP validation (5min)
make test-e2e-setup-cli     # Setup CLI E2E tests (5min)
make test-integration       # Basic integration tests (5min)
make test-e2e-workflow      # E2E workflow tests (15min)
make test-e2e-python        # Python E2E tests (10min)
make test-e2e-typescript    # TypeScript E2E tests (10min)
make test-e2e-go            # Go E2E tests (10min)
make test-python-real       # Python real pylsp integration (10min)
make test-typescript-real   # TypeScript real server integration (10min)
make test-e2e-mcp          # Comprehensive MCP E2E tests (10min)
make test-mcp-tools        # MCP tools E2E tests (10min)
make test-npm-cli          # NPM CLI E2E tests (10min)
make test-npm-mcp          # Comprehensive NPM-MCP tests (10min)
make test-circuit-breaker-comprehensive # Comprehensive scenarios (10min)
```

#### Language-Specific Tests (10-15 minutes)
```bash
make test-e2e-java         # Java E2E tests with JDTLS (15min)
make test-java-real        # Java real JDTLS integration (15min)
make test-jdtls-integration # Comprehensive JDTLS testing (15min)
make test-mcp-scip         # SCIP-enhanced MCP tests (15min)
```

#### Comprehensive Test Suites (20-30 minutes)
```bash
make test-e2e-advanced     # Advanced E2E scenarios (20min)
make test-e2e-full         # Full E2E test suite (30min)
```

#### Specialized NPM-MCP Tests (5-10 minutes)
```bash
make test-npm-mcp-js       # NPM-MCP JavaScript tests only (5min)
make test-npm-mcp-go       # NPM-MCP Go integration tests (5min)
```

## Test Timing Reference

### Quick Tests (<5 minutes)
Ideal for immediate feedback during development:
- **Unit Tests**: <60 seconds (all internal packages)
- **Quick E2E**: 1-5 minutes (basic validation scenarios)
- **Quick MCP**: 5 minutes (STDIO/TCP protocol basics)
- **Quick NPM**: 5 minutes (basic npm integration)
- **Circuit Breaker**: 5 minutes (basic failure scenarios)

### Standard Tests (5-15 minutes)
Suitable for pre-commit validation:
- **LSP Validation**: 5 minutes (full protocol validation)
- **Language-Specific**: 10-15 minutes (per language with real servers)
- **MCP Comprehensive**: 10 minutes (full MCP tool testing)
- **NPM Integration**: 10 minutes (comprehensive npm scenarios)
- **E2E Workflow**: 15 minutes (complete development workflows)

### Comprehensive Tests (15-30+ minutes)
For thorough validation before releases:
- **Advanced E2E**: 20 minutes (complex scenarios)
- **Full E2E Suite**: 30+ minutes (complete test coverage)
- **Java JDTLS**: 15 minutes (comprehensive Java integration)
- **SCIP-Enhanced**: 15 minutes (caching and performance validation)

### Parallel Execution Optimization
```bash
# Run tests in parallel for faster CI
go test -v -timeout 10m -parallel 4 ./...

# Language-specific parallel testing
go test -v -timeout 15m -parallel 2 -run "TestJava|TestPython" ./tests/e2e/...
```

## Performance Thresholds

Essential performance requirements:
- **Response Time**: <5 seconds
- **Throughput**: >100 requests/second  
- **Error Rate**: <5%
- **Memory Usage**: <3GB total, <1GB growth
- **Test Suite Performance**: Unit tests <60s, E2E <30min

## Test Infrastructure

### Test Organization (55+ Test Files)
The test suite is organized into focused categories:
- **Unit Tests**: `internal/**/*_test.go` - 35+ files co-located with source code
- **Integration Tests**: `tests/integration/` - 8+ files for LSP protocol validation
- **E2E Tests**: `tests/e2e/` - 12+ files for comprehensive end-to-end scenarios
- **Specialized Tests**: Language-specific, MCP protocol, NPM integration, circuit breaker tests

### Mock Infrastructure (8+ Mock Objects)
Streamlined mock implementations for essential testing:
- **Mock LSP Server**: Simplified 87-line implementation for basic protocol testing
- **Mock MCP Client**: Reduced 50-line implementation for MCP protocol validation
- **Mock Transport**: Basic connection simulation without over-engineering
- **Mock Project Detector**: Simple project structure simulation
- **Mock Configuration**: Basic config loading and validation mocks
- **Mock Circuit Breaker**: Essential failure simulation without complexity
- **Mock NPM Client**: Basic NPM integration testing support
- **Mock SCIP Cache**: Simple cache behavior simulation

### Test Framework Approach
- **Unit Tests**: Standard Go testing without complex test suites
- **E2E Tests**: Direct integration with real language servers when possible
- **Protocol Testing**: Real MCP STDIO/TCP protocol validation
- **NPM Integration**: Real npm command execution and validation
- **Performance Testing**: Essential performance thresholds without enterprise complexity
- **Assertions**: Standard Go testing assertions with focused error checking

### Test Data and Fixtures
- **Real Projects**: Simple temporary directories with actual project files
- **Language Server Responses**: Realistic LSP response fixtures in JSON format
- **MCP Protocol Messages**: Real MCP request/response examples
- **NPM Package Examples**: Minimal test packages for npm integration validation
- **Configuration Templates**: Test configurations for various language setups
- **SCIP Index Samples**: Basic SCIP cache examples for testing

### Real Server Integration
The test suite includes integration with actual language servers:
- **JDTLS (Java)**: Real Eclipse JDT Language Server integration
- **pylsp (Python)**: Real Python LSP server integration
- **TypeScript Language Server**: Real tsserver integration
- **gopls (Go)**: Real Go language server integration
- **Setup Validation**: Real binary execution with actual language server detection

## Development Guidelines

### Writing Unit Tests
- Test **core business logic only** - skip getters, setters, and trivial functions
- Use **standard Go testing** - no complex test frameworks or suites
- Focus on **error conditions** and **edge cases** that matter
- Keep tests **fast (<60s total)** and **independent**
- Avoid testing implementation details - test behavior

### Writing E2E Tests
- Test **actual developer workflows** only
- Use **real language servers** when possible
- Test **essential LSP methods** - definition, hover, references
- Keep scenarios **simple and realistic**
- Avoid performance testing unless critical

### Writing Language-Specific Tests
- **Focus on real language servers** - JDTLS, pylsp, tsserver, gopls
- **Test common development scenarios** - code navigation, symbol search
- **Validate language-specific features** - Java classpath, Python virtual environments
- **Keep test projects minimal** - simple, focused examples
- **Test both mock and real server scenarios** for flexibility

### Writing MCP Protocol Tests
- **Test both STDIO and TCP transports** for complete coverage
- **Validate MCP tool functionality** - ensure tools work as expected
- **Test SCIP integration** - verify caching performance benefits
- **Keep protocol tests focused** - test essential MCP operations only
- **Use real MCP client scenarios** - simulate actual AI assistant usage

### Writing NPM Integration Tests
- **Test real npm command execution** - validate actual npm integration
- **Cover both JavaScript and Go scenarios** for complete coverage
- **Test package installation and configuration** - end-to-end npm workflows
- **Validate CLI integration** - ensure npm and Go CLI work together
- **Keep npm tests isolated** - avoid dependencies on external packages

### Writing Setup CLI E2E Tests
- Test **real binary execution** with actual command combinations
- Validate **JSON output structure** and content accuracy
- Cover **error scenarios** and edge cases with proper response validation
- Use **real project structures** for authentic testing environments
- Focus on **command coverage** - setup all, detect, wizard, template scenarios
- Ensure **cross-platform compatibility** where applicable

### Test Maintenance
- **Delete tests** that don't add value or test trivial functionality  
- **Simplify over-engineered tests** - prefer standard Go testing patterns
- **Remove redundant test infrastructure** - avoid custom frameworks
- **Keep test data minimal** - use simple fixtures and real temporary files
- **Update tests only when behavior changes** - not implementation details

## Troubleshooting

### Test Failures
```bash
# System diagnostics
./bin/lsp-gateway diagnose
./bin/lsp-gateway verify

# Verbose test execution
go test -v -run TestSpecificScenario ./tests/
```

### Common Issues

#### General Issues
- **Configuration errors**: `./bin/lsp-gateway config validate`
- **Language server issues**: `./bin/lsp-gateway install <server> --force`
- **Performance issues**: `./bin/lsp-gateway performance`

#### Language-Specific Test Failures
```bash
# Java/JDTLS issues
./bin/lsp-gateway install jdtls --force
./bin/lsp-gateway diagnose --language java

# Python/pylsp issues
./bin/lsp-gateway install pylsp --force
./bin/lsp-gateway diagnose --language python

# TypeScript server issues
./bin/lsp-gateway install typescript-language-server --force
./bin/lsp-gateway diagnose --language typescript

# Go/gopls issues
./bin/lsp-gateway install gopls --force
./bin/lsp-gateway diagnose --language go
```

#### MCP Protocol Test Issues
```bash
# MCP server connectivity
./bin/lsp-gateway mcp --config config.yaml --debug

# MCP STDIO protocol testing
go test -v -run TestMCPStdioProtocol ./tests/e2e/mcp_protocol_e2e_test.go

# MCP tools validation
go test -v -run TestMCPTools ./tests/e2e/mcp_tools_e2e_test.go
```

#### NPM Integration Test Issues
```bash
# NPM CLI validation
./scripts/test-npm-mcp.sh --debug

# NPM package installation issues
npm install -g @lsp-gateway/cli

# NPM-MCP integration debugging
go test -v -run TestNpmCliE2ETestSuite ./tests/e2e/npm_cli_e2e_test.go
```

#### Circuit Breaker Test Issues
```bash
# Circuit breaker behavior validation
go test -v -run TestCircuitBreakerE2ESuite ./tests/e2e/...

# Circuit breaker configuration
./bin/lsp-gateway config validate --circuit-breaker
```

## CI/CD Integration

### Basic CI Configuration
```bash
# Standard CI pipeline
go test -v -timeout 10m -parallel 4 ./...

# JSON output for reporting
go test -json ./... > test_results.json
```

### Staged CI Testing Strategy
```bash
# Stage 1: Fast feedback (<5 minutes)
make test-unit
make test-simple-quick

# Stage 2: Core validation (10-15 minutes)
make test-lsp-validation
make test-e2e-quick
make test-mcp-stdio

# Stage 3: Language-specific testing (parallel, 15-20 minutes)
make test-e2e-python &
make test-e2e-typescript &
make test-e2e-go &
wait

# Stage 4: Comprehensive validation (20-30 minutes)
make test-java-real
make test-e2e-mcp
make test-npm-mcp

# Stage 5: Full validation (30+ minutes - nightly)
make test-e2e-full
make test-e2e-advanced
```

### Language-Specific CI Testing
```bash
# Java-focused CI pipeline
make test-unit && make test-e2e-java && make test-jdtls-integration

# Python-focused CI pipeline
make test-unit && make test-e2e-python && make test-python-real

# MCP-focused CI pipeline
make test-unit && make test-e2e-mcp && make test-mcp-scip

# NPM integration CI pipeline
make test-unit && make test-npm-cli && make test-npm-mcp
```

### Failure Recovery and Debugging
```bash
# Detailed test output for CI debugging
go test -v -timeout 15m -run TestSpecificFailure ./tests/e2e/...

# Test artifacts collection
go test -json -v ./... | tee test_results.json

# Generate test coverage reports
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Test Infrastructure Capabilities

### Comprehensive Test Coverage (55+ Test Files)
The current test suite provides extensive coverage across all core functionality:

**Unit Tests (35+ files)**:
- Gateway routing and protocol handling
- Configuration loading and validation
- Transport layer (STDIO/TCP) with circuit breakers
- Project detection and language identification
- SCIP cache integration and performance
- MCP protocol message handling
- NPM integration components

**Integration Tests (8+ files)**:
- Real language server communication (JDTLS, pylsp, tsserver, gopls)
- Protocol validation (HTTP JSON-RPC, MCP STDIO/TCP)
- Multi-language project detection
- Circuit breaker behavior validation
- Configuration template processing

**E2E Tests (12+ files)**:
- Complete developer workflows (definition, references, hover)
- Language-specific scenarios (Java, Python, TypeScript, Go)
- MCP tool functionality and SCIP integration
- NPM CLI and integration testing
- Setup automation and binary execution
- Advanced circuit breaker scenarios

### Performance Testing Infrastructure
- **Response Time Validation**: LSP request/response timing
- **Cache Performance**: SCIP cache hit rates and response improvements
- **Memory Usage Monitoring**: Memory growth and garbage collection
- **Throughput Testing**: Concurrent request handling
- **Circuit Breaker Performance**: Failure detection and recovery timing

### Real Server Integration Testing
The test suite includes comprehensive real language server testing:
- **Java**: Complete JDTLS integration with real Eclipse JDT Language Server
- **Python**: Full pylsp integration with real Python Language Server
- **TypeScript**: Real TypeScript Language Server integration
- **Go**: Complete gopls integration with real Go Language Server
- **Multi-language**: Cross-language project detection and routing

## Current Test Infrastructure (2025)

The test suite balances comprehensive coverage with maintenance simplicity:

### Current Capabilities
- **30+ Test Commands**: Covering all development scenarios from quick validation to comprehensive testing
- **Real Server Integration**: Actual language server testing for production-ready validation
- **MCP Protocol Coverage**: Complete Model Context Protocol testing including SCIP integration
- **NPM Integration**: Full npm CLI and package integration testing
- **Circuit Breaker Testing**: Comprehensive failure scenario validation
- **Performance Validation**: Essential performance thresholds without enterprise complexity

### Maintained Simplicity
- **Standard Go Testing**: No complex test frameworks or over-engineered infrastructure
- **Focused Mock Objects**: 8+ simplified mocks covering essential functionality only
- **Essential Fixtures**: Minimal test data focusing on realistic scenarios
- **Streamlined Organization**: Clear test categories with logical command groupings

### Testing Philosophy Alignment
- **Essential Coverage**: Tests focus on core LSP Gateway functionality for local development
- **Developer Experience**: Quick feedback loops with <60 second unit tests
- **Real Scenarios**: E2E tests validate actual developer workflows
- **Maintenance Focus**: Simple patterns that are easy to understand and maintain

This approach provides comprehensive testing coverage while maintaining the essential-only philosophy, ensuring LSP Gateway functions reliably as a local development tool without unnecessary complexity.