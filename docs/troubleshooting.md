# LSP Gateway Troubleshooting Guide

This document provides solutions to common issues encountered when developing and running LSP Gateway.

## Quick Diagnostic Commands

Start with these commands to identify issues:

```bash
# Overall system health check
./bin/lspg health

# Comprehensive diagnostics
./bin/lspg diagnose

# Enhanced diagnostic capabilities
./bin/lspg diagnose runtimes     # Runtime detection and validation
./bin/lspg diagnose routing      # Request routing and LSP connections
./bin/lspg diagnose performance  # Performance metrics and SCIP cache

# Check server status
./bin/lspg status
./bin/lspg status runtimes       # Runtime-specific status

# Verify installation
./bin/lspg verify
./bin/lspg verify runtime <lang> # Verify specific language runtime

# System information
./bin/lspg version               # Version and build information
```

## Common Issues

### 1. Configuration Problems

**Symptoms**: Server won't start, invalid configuration errors, template issues

```bash
# Validate configuration
./bin/lspg config validate
./bin/lspg config validate --verbose    # Detailed validation output

# Show current effective configuration
./bin/lspg config show                  # Display merged configuration
./bin/lspg config show --format json    # JSON format output

# Configuration generation and templates
./bin/lspg config generate --force      # Regenerate configuration
./bin/lspg config templates             # List available templates

# Advanced configuration management
./bin/lspg config migrate               # Migrate old configuration format
./bin/lspg config optimize              # Optimize config for performance

# Project detection and setup
./bin/lspg setup detect                 # Detect project languages
./bin/lspg setup template <name>        # Apply specific template
./bin/lspg setup multi-language         # Multi-language project setup
```

### 2. Language Server Issues

**Symptoms**: Language features not working, server connection failures

```bash
# Check language server status
./bin/lspg status

# Verify language server installation
./bin/lspg verify

# Reinstall problematic server
./bin/lspg install <server-name> --force

# Check runtime detection
./bin/lspg detect --verbose
```

**Common Language Server Problems:**
- **gopls**: Check Go version (requires 1.18+)
- **pylsp**: Verify Python environment and pip installation
- **typescript-language-server**: Check Node.js version (requires 20+)
- **jdtls**: Verify Java SDK installation (requires JDK 17+)

### 3. Performance Issues

**Symptoms**: Slow responses, high memory usage, timeouts, poor SCIP cache performance

```bash
# Run performance diagnostics
./bin/lspg performance
./bin/lspg diagnose performance         # Enhanced performance metrics

# SCIP cache diagnostics
./bin/lspg diagnose --verbose           # Includes cache hit rates
./bin/lspg status                       # Shows cache utilization

# Check circuit breaker status
./bin/lspg diagnose --verbose

# Run performance tests
make test-integration

# Monitor system resources
htop # or top on macOS
```

**Performance Optimization:**
- **Circuit Breaker**: Default 10 errors/30s timeout - adjust in config
- **Connection Pool**: Check pool utilization in diagnostics
- **Memory**: 3GB max limit, 1GB growth threshold
- **Response Time**: 5s max - check server load

**SCIP Cache Troubleshooting:**
- **Low Cache Hit Rate (<85%)**: Check file watching, ensure source files aren't changing rapidly
- **High Memory Usage**: Cache uses 65-75MB additional memory - normal behavior
- **Cache Invalidation Issues**: Restart server to rebuild cache, check file permissions
- **Performance Regression**: Verify 60-87% response time improvement with cache enabled

### 4. Connection Issues

**Symptoms**: Connection refused, transport errors, protocol failures

```bash
# Check transport layer health
./bin/lspg health

# Verify network connectivity
./bin/lspg diagnose --network

# Reset connection pools
pkill lspg && ./bin/lspg server --config config.yaml

# Check port availability (HTTP mode)
lsof -i :8080  # or netstat -an | grep 8080
```

### 5. Build and Development Issues

**Symptoms**: Build failures, test failures, lint errors

```bash
# Clean rebuild
make clean && make local

# Check Go version
go version  # requires 1.24+

# Update dependencies
make deps && make tidy

# Run quality checks
make quality  # format + lint + security

# Quick test feedback
make test-simple-quick
```

### 6. Setup and Installation Problems

**Symptoms**: Runtime detection failures, language server installation issues, multi-language setup problems

```bash
# Enhanced setup diagnostics
./bin/lspg setup detect                 # Detect project languages and requirements
./bin/lspg diagnose runtimes            # Validate runtime installations

# Runtime-specific verification
./bin/lspg verify runtime go            # Verify Go runtime
./bin/lspg verify runtime python        # Verify Python runtime
./bin/lspg verify runtime node          # Verify Node.js runtime
./bin/lspg verify runtime java          # Verify Java runtime

# Template-based setup
./bin/lspg setup template go-advanced   # Apply Go template
./bin/lspg setup template python-django # Apply Django template
./bin/lspg setup multi-language         # Multi-language project setup

# Troubleshoot setup issues
./bin/lspg setup all --verbose          # Verbose setup output
./bin/lspg install <server> --force     # Force reinstall language server
```

**Common Setup Problems:**

**Runtime Detection Failures:**
- **Go**: Check `go version` (requires 1.24+), verify GOPATH/GOROOT
- **Python**: Verify `python3` and `pip3` availability, check virtual environments
- **Node.js**: Check `node --version` (requires 20+), verify npm/yarn access
- **Java**: Verify JDK installation (requires 17+), check JAVA_HOME

**Language Server Installation Issues:**
- **Permission Errors**: Check npm/pip permissions, use `--force` flag
- **Network Issues**: Verify internet connectivity, check proxy settings
- **Version Conflicts**: Use `install <server> --force` to override existing installations
- **Missing Dependencies**: Run `setup all` to install missing runtimes

**Multi-Language Project Problems:**
- **Template Conflicts**: Use project-specific templates instead of generic ones
- **Resource Conflicts**: Check memory and CPU limits in configuration
- **Path Issues**: Verify language-specific PATH configuration in templates

### 7. MCP Integration Issues

**Symptoms**: AI assistant can't connect, MCP protocol errors, tool failures

```bash
# Start MCP server in debug mode
./bin/lspg mcp --config config.yaml --debug --verbose

# Check MCP transport options and configuration
./bin/lspg mcp --help
./bin/lspg config show               # Verify MCP configuration

# Diagnose MCP protocol issues
./bin/lspg diagnose routing          # Check MCP to LSP routing
./bin/lspg status                    # Verify MCP server status

# Test MCP connection by transport type
# For stdio: Check if process starts correctly
echo '{"jsonrpc":"2.0","method":"initialize","id":1}' | ./bin/lspg mcp --config config.yaml

# For tcp: Check if port is available and accessible
./bin/lspg mcp --config config.yaml --transport tcp --port 3000
lsof -i :3000  # Verify port binding
```

**Common MCP Protocol Problems:**
- **STDIO Transport**: Process startup issues, stdin/stdout redirection conflicts
- **TCP Transport**: Port binding failures, firewall blocking, connection timeouts  
- **Tool Failures**: LSP method not supported, gateway routing errors
- **Protocol Errors**: JSON-RPC format issues, missing method implementations
- **AI Assistant Integration**: Capability mismatch, transport configuration errors

**MCP Protocol Debugging:**
- **Enable Verbose Logging**: Use `--debug --verbose` for detailed protocol traces
- **Test Individual Tools**: Verify each LSP feature works via HTTP before MCP
- **Check Transport Layer**: Ensure chosen transport (stdio/tcp) works correctly
- **Validate Protocol Messages**: Check JSON-RPC 2.0 format compliance

## Advanced Troubleshooting

### Circuit Breaker States

Monitor circuit breaker status in diagnostics:
- **Closed**: Normal operation
- **Open**: Failing, requests blocked
- **Half-Open**: Testing recovery

### Configuration Debugging

Check configuration hierarchy and troubleshoot config issues:
```bash
# Show effective configuration
./bin/lspg config show
./bin/lspg config show --format json    # JSON format for analysis

# Validate specific template
./bin/lspg config templates --show enterprise
./bin/lspg config templates             # List all available templates

# Test configuration generation
./bin/lspg config generate --dry-run    # Test without writing files
./bin/lspg config generate --force      # Force regeneration

# Advanced configuration management
./bin/lspg config migrate               # Migrate legacy configurations
./bin/lspg config optimize              # Optimize for current setup
./bin/lspg config validate --verbose    # Detailed validation output

# Project-specific configuration
./bin/lspg setup detect                 # Auto-detect project requirements
./bin/lspg setup template <name>        # Apply template to current project
```

### SCIP Cache Diagnostics

Monitor and troubleshoot SCIP intelligent caching performance:

```bash
# SCIP cache performance diagnostics
./bin/lspg diagnose performance         # Cache hit rates and performance metrics
./bin/lspg status                       # Cache utilization and memory usage

# Cache management operations
# Cache is managed automatically - manual operations for troubleshooting only
rm -rf ~/.cache/lsp-gateway/scip               # Reset cache (restart required)

# Performance baseline creation
./bin/lspg diagnose performance --baseline  # Create performance baseline
./bin/lspg diagnose performance --compare   # Compare with baseline

# Cache monitoring commands
./bin/lspg diagnose --verbose           # Detailed cache statistics
```

**SCIP Cache Issues and Solutions:**

**Low Cache Hit Rate (<85%)**:
- **Cause**: Rapidly changing source files, file watching issues
- **Solution**: Check file permissions, reduce file modification frequency during testing
- **Diagnosis**: Monitor cache hit rates with `diagnose performance`

**High Memory Usage (>100MB cache)**:
- **Cause**: Large codebase, many indexed symbols
- **Normal**: 65-75MB additional memory usage is expected
- **Solution**: Monitor with system tools, restart if memory growth is excessive

**Cache Invalidation Problems**:
- **Symptoms**: Stale responses, outdated symbol information
- **Solution**: Restart LSP Gateway to rebuild cache, check file watching
- **Prevention**: Ensure proper file permissions for cache directory

**Performance Regression**:
- **Expected**: 60-87% response time improvement with cache
- **Diagnosis**: Compare performance with and without cache using baselines
- **Solution**: Verify SCIP indexing is working, check for cache corruption

### Log Analysis

Enable verbose logging:
```bash
# Start with debug logging
./bin/lspg server --config config.yaml --debug --verbose

# Check system logs
journalctl -u lsp-gateway  # systemd systems
tail -f /var/log/lsp-gateway.log  # if configured
```

### Testing Issues

Isolate test failures:
```bash
# Unit tests only
make test-unit

# Specific test categories
go test -v ./tests/unit/...
go test -v ./tests/integration/...
```

## Environment-Specific Issues

### macOS

```bash
# Permission issues
xattr -d com.apple.quarantine ./bin/lspg

# Homebrew conflicts
brew uninstall conflicting-package
```

### Linux

```bash
# Missing dependencies
sudo apt-get install build-essential  # Ubuntu/Debian
sudo yum groupinstall "Development Tools"  # RHEL/CentOS

# SELinux issues
sestatus  # check if enabled
sudo setsebool -P httpd_can_network_connect 1
```

### Windows

```bash
# PowerShell execution policy
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser

# Windows Defender exclusions
# Add project directory to exclusions
```

## Getting Help

1. **Enable verbose logging** with `--verbose --debug` flags
2. **Run comprehensive diagnostics**:
   ```bash
   ./bin/lspg diagnose --verbose          # Full system diagnostics
   ./bin/lspg diagnose runtimes           # Runtime-specific issues
   ./bin/lspg diagnose performance        # Performance and SCIP cache
   ./bin/lspg diagnose routing            # Request routing and connections
   ```
3. **Check system status and configuration**:
   ```bash
   ./bin/lspg status runtimes             # Runtime status
   ./bin/lspg config show                 # Effective configuration
   ./bin/lspg version                     # Version and build info
   ```
4. **Test with minimal configuration** using basic templates
5. **Verify system requirements** (Go 1.24+, Node.js 20+)
6. **Enable shell completion** for easier command usage:
   ```bash
   ./bin/lspg completion bash >> ~/.bashrc  # Bash completion
   ./bin/lspg completion zsh >> ~/.zshrc    # Zsh completion
   ```

## Configuration Templates

For complex setups, start with these templates:
- `single-language.yaml` - Minimal single language setup
- `basic.yaml` - Simple multi-language configuration
- `enterprise.yaml` - Production-ready configuration
- `development.yaml` - Development-optimized settings

Use `./bin/lspg config templates` to see all available templates.