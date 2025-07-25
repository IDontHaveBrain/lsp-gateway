# LSP Gateway Troubleshooting Guide

This document provides solutions to common issues encountered when developing and running LSP Gateway.

## Quick Diagnostic Commands

Start with these commands to identify issues:

```bash
# Overall system health check
./bin/lsp-gateway health

# Comprehensive diagnostics
./bin/lsp-gateway diagnose

# Check server status
./bin/lsp-gateway status

# Verify installation
./bin/lsp-gateway verify
```

## Common Issues

### 1. Configuration Problems

**Symptoms**: Server won't start, invalid configuration errors

```bash
# Validate configuration
./bin/lsp-gateway config validate

# Check available templates
./bin/lsp-gateway config templates

# Regenerate configuration
./bin/lsp-gateway config generate --force

# Detailed validation with verbose output
./bin/lsp-gateway config validate --verbose
```

### 2. Language Server Issues

**Symptoms**: Language features not working, server connection failures

```bash
# Check language server status
./bin/lsp-gateway status

# Verify language server installation
./bin/lsp-gateway verify

# Reinstall problematic server
./bin/lsp-gateway install <server-name> --force

# Check runtime detection
./bin/lsp-gateway detect --verbose
```

**Common Language Server Problems:**
- **gopls**: Check Go version (requires 1.18+)
- **pylsp**: Verify Python environment and pip installation
- **typescript-language-server**: Check Node.js version (requires 16+)
- **jdtls**: Verify Java SDK installation (requires JDK 17+)

### 3. Performance Issues

**Symptoms**: Slow responses, high memory usage, timeouts

```bash
# Run performance diagnostics
./bin/lsp-gateway performance

# Check circuit breaker status
./bin/lsp-gateway diagnose --verbose

# Run performance tests
make test-integration

# Monitor memory usage
htop # or top on macOS
```

**Performance Optimization:**
- **Circuit Breaker**: Default 10 errors/30s timeout - adjust in config
- **Connection Pool**: Check pool utilization in diagnostics
- **Memory**: 3GB max limit, 1GB growth threshold
- **Response Time**: 5s max - check server load

### 4. Connection Issues

**Symptoms**: Connection refused, transport errors, protocol failures

```bash
# Check transport layer health
./bin/lsp-gateway health

# Verify network connectivity
./bin/lsp-gateway diagnose --network

# Reset connection pools
pkill lsp-gateway && ./bin/lsp-gateway server --config config.yaml

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

### 6. MCP Integration Issues

**Symptoms**: AI assistant can't connect, MCP protocol errors

```bash
# Start MCP server in debug mode
./bin/lsp-gateway mcp --config config.yaml --debug

# Check MCP transport options
./bin/lsp-gateway mcp --help

# Test MCP connection
# For stdio: Check if process starts correctly
# For tcp: Check if port is available and accessible
```

## Advanced Troubleshooting

### Circuit Breaker States

Monitor circuit breaker status in diagnostics:
- **Closed**: Normal operation
- **Open**: Failing, requests blocked
- **Half-Open**: Testing recovery

### Configuration Debugging

Check configuration hierarchy:
```bash
# Show effective configuration
./bin/lsp-gateway config show

# Validate specific template
./bin/lsp-gateway config templates --show enterprise

# Test configuration generation
./bin/lsp-gateway config generate --dry-run
```

### Log Analysis

Enable verbose logging:
```bash
# Start with debug logging
./bin/lsp-gateway server --config config.yaml --debug --verbose

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

# CI-style testing
./scripts/ci/run-tests.sh unit
```

## Environment-Specific Issues

### macOS

```bash
# Permission issues
xattr -d com.apple.quarantine ./bin/lsp-gateway

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
2. **Run full diagnostics** with `./bin/lsp-gateway diagnose --verbose`
3. **Check the logs** for specific error messages
4. **Test with minimal configuration** using basic templates
5. **Verify system requirements** (Go 1.24+, Node.js 22+)

## Configuration Templates

For complex setups, start with these templates:
- `single-language.yaml` - Minimal single language setup
- `basic.yaml` - Simple multi-language configuration
- `enterprise.yaml` - Production-ready configuration
- `development.yaml` - Development-optimized settings

Use `./bin/lsp-gateway config templates` to see all available templates.