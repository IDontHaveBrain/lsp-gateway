# Setup Error Handling and Logging Framework

This package provides a comprehensive error handling and logging framework specifically designed for the embedded auto-setup system. It focuses on providing excellent user experience through clear error messages, actionable guidance, and robust progress tracking.

## Overview

The framework consists of several key components:

- **Platform Errors** (`internal/platform/errors.go`) - Platform-specific error types with rich context
- **Setup Errors** (`internal/setup/errors.go`) - Installation, validation, and command execution errors
- **Setup Logging** (`internal/setup/logging.go`) - Structured logging with user-friendly output
- **Error Utilities** (`internal/setup/error_utils.go`) - Error handling patterns and utilities
- **Examples** (`internal/setup/example.go`) - Usage patterns and best practices

## Key Features

### 1. Rich Error Context
- Machine-readable error codes for automation
- User-friendly messages with actionable guidance
- Automatic recovery suggestions and steps
- Error categorization and severity levels

### 2. Structured Logging
- Dual output streams (technical logs + user messages)
- Progress tracking for long-running operations
- Metrics collection and reporting
- JSON and human-readable formats

### 3. Automatic Error Recovery
- Retry mechanisms with exponential backoff
- Automatic recovery handlers for common issues
- Permission and dependency resolution

### 4. User Experience Focus
- Clear, non-technical error messages
- Step-by-step recovery instructions
- Progress indicators with ETA calculations
- Success/warning/error indicators with symbols

## Quick Start

### Basic Logging Setup

```go
import "lsp-gateway/internal/setup"

// Create logger
logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
    Level:              setup.LogLevelInfo,
    Component:          "auto-setup",
    EnableUserMessages: true,
    EnableProgressTracking: true,
})
defer logger.LogSummary()

// Create error handler
errorHandler := setup.NewErrorHandler(logger)

// Log user-facing messages
logger.UserInfo("Starting installation process")
logger.UserSuccess("Installation completed successfully")
logger.UserWarn("Configuration needs attention")
logger.UserError("Installation failed")
```

### Error Handling

```go
// Create operation context
ctx := &setup.ErrorContext{
    Operation: "install-go-runtime",
    Component: "go",
    StartTime: time.Now(),
    Logger:    logger,
    Metadata:  map[string]interface{}{"version": "1.21.0"},
}

// Handle errors with automatic recovery
if err := someOperation(); err != nil {
    errorHandler.HandleError(ctx, err)
    return
}
```

### Progress Tracking

```go
// Create progress tracker
progress := setup.NewProgressInfo("download-files", 100)

for i := int64(0); i <= 100; i += 10 {
    progress.Update(i, fmt.Sprintf("file_%d.txt", i))
    logger.UserProgress("Downloading files", progress)
    time.Sleep(100 * time.Millisecond)
}
```

## Error Types

### Platform Errors

Located in `internal/platform/errors.go`:

```go
// Create platform-specific errors
err := platform.NewUnsupportedPlatformError("unsupported-os")
err := platform.NewInsufficientPermissionsError("write", "/restricted/path")
err := platform.NewPackageManagerError("apt", "install", originalErr)
```

### Installation Errors

```go
// Runtime installation errors
err := setup.NewRuntimeNotFoundError("go").
    WithVersion("1.21.0").
    WithAlternatives(alternatives)

// Server installation errors
err := setup.NewServerInstallFailedError("gopls", originalErr).
    WithTargetPath("/usr/local/bin/gopls")
```

### Validation Errors

```go
// Configuration validation
err := setup.NewConfigValidationFailedError("port", "invalid", "must be 1-65535").
    WithValidValues([]string{"8080", "3000", "9000"})
```

### Command Execution Errors

```go
// Command execution with rich context
err := setup.NewCommandError(setup.ErrCodeCommandFailed, "git", []string{"clone", "repo"}, 128).
    WithDuration(5*time.Second).
    WithOutput(stdout, stderr)
```

## Error Codes

All errors include machine-readable codes for automation:

### Platform Codes
- `PLATFORM_UNSUPPORTED` - Unsupported operating system
- `PERMISSIONS_INSUFFICIENT` - Permission denied
- `PACKAGE_MANAGER_FAILED` - Package manager error

### Setup Codes
- `RUNTIME_NOT_FOUND` - Required runtime missing
- `SERVER_INSTALL_FAILED` - Language server installation failed
- `CONFIG_VALIDATION_FAILED` - Configuration validation error
- `COMMAND_FAILED` - Command execution failed
- `DOWNLOAD_FAILED` - File download failed

## Logging Levels

The framework supports multiple logging levels:

- `LogLevelTrace` - Detailed debugging information
- `LogLevelDebug` - Development debugging
- `LogLevelInfo` - General information
- `LogLevelWarn` - Warning conditions
- `LogLevelError` - Error conditions
- `LogLevelFatal` - Fatal errors (exits program)
- `LogLevelUser` - Special level for user messages

## Configuration Options

### Logger Configuration

```go
config := &setup.SetupLoggerConfig{
    Level:                  setup.LogLevelInfo,
    Component:              "component-name",
    EnableJSON:             false,  // Human-readable by default
    EnableUserMessages:     true,   // Enable user-facing output
    EnableProgressTracking: true,   // Enable progress indicators
    EnableMetrics:          true,   // Collect operation metrics
    Output:                 os.Stderr,  // Technical logs
    UserOutput:             os.Stdout,  // User messages
    VerboseMode:            false,  // Detailed output
    QuietMode:              false,  // Minimal output
    SessionID:              "auto-generated",
}
```

## Advanced Features

### Retry Operations

```go
// Automatic retry with exponential backoff
err := errorHandler.RetryOperation(
    context.Background(),
    operationFunc,
    setup.ErrCodeDownloadFailed,
    logger,
)
```

### Command Wrapping

```go
// Execute commands with comprehensive error handling
cmd := exec.Command("go", "version")
if err := errorHandler.WrapCommand(ctx, cmd); err != nil {
    errorHandler.HandleError(ctx, err)
}
```

### Environment Validation

```go
// Validate system requirements
requirements := map[string]interface{}{
    "disk_space":  int64(1024 * 1024 * 1024), // 1GB
    "permissions": []string{"/usr/local", "/opt"},
    "commands":    []string{"git", "curl"},
    "network":     []string{"https://golang.org"},
}

if errors := errorHandler.ValidateEnvironment(ctx, requirements); len(errors) > 0 {
    // Handle validation failures
}
```

### Metrics Collection

```go
// Update custom metrics
logger.UpdateMetrics(map[string]interface{}{
    "files_processed":   10,
    "bytes_downloaded":  1024*1024,
    "success_rate":      0.95,
})

// Get current metrics
metrics := logger.GetMetrics()
fmt.Printf("Processed %d files, %d errors\n", 
    metrics.FilesProcessed, metrics.ErrorsEncountered)
```

## Integration Patterns

### With CLI Commands

```go
func setupCommand(cmd *cobra.Command, args []string) error {
    logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
        Component:          "cli-setup",
        EnableUserMessages: true,
        QuietMode:          quiet, // From CLI flag
        VerboseMode:        verbose, // From CLI flag
    })
    
    errorHandler := setup.NewErrorHandler(logger)
    
    ctx := &setup.ErrorContext{
        Operation: "cli-setup",
        Component: "lsp-gateway",
        Logger:    logger,
    }
    
    // Perform setup operations...
    
    return nil
}
```

### With Existing Error Handling

```go
// Wrap existing errors
if err := existingFunction(); err != nil {
    wrappedErr := setup.WrapWithContext(err, "operation", "component", metadata)
    return errorHandler.HandleError(ctx, wrappedErr)
}
```

## Error Recovery

The framework includes automatic recovery for common issues:

- **Permission Issues**: Automatic directory creation, permission fixes
- **Missing Commands**: Package manager installation attempts
- **Network Issues**: Retry with exponential backoff
- **Package Manager Issues**: Cache updates, repository refresh

### Custom Recovery Handlers

```go
// Add custom recovery handler
errorHandler.RegisterHandler(myErrorCode, func(ctx context.Context, err error, logger *SetupLogger) error {
    // Custom recovery logic
    return nil // Return nil if recovered, error if not
})
```

## Best Practices

### 1. Use Appropriate Error Types
- Platform errors for OS/system issues
- Installation errors for component installation
- Validation errors for configuration issues
- Command errors for external command execution

### 2. Provide Rich Context
- Always include operation and component information
- Add relevant metadata (versions, paths, etc.)
- Use progress tracking for long operations

### 3. User Experience
- Prefer user-friendly messages over technical details
- Provide actionable recovery steps
- Use symbols (✓, ⚠, ✗) for visual clarity

### 4. Error Handling Flow
```go
// 1. Create context
ctx := &setup.ErrorContext{...}

// 2. Perform operation
if err := operation(); err != nil {
    // 3. Handle with framework
    return errorHandler.HandleError(ctx, err)
}

// 4. Log success
logger.UserSuccess("Operation completed")
```

### 5. Testing
- Test error scenarios thoroughly
- Verify user message clarity
- Test automatic recovery mechanisms
- Validate metrics collection

## Migration Guide

### From Existing Error Handling

1. **Replace simple errors**:
   ```go
   // Before
   return fmt.Errorf("failed to install %s: %v", component, err)
   
   // After
   return setup.NewInstallationError(
       setup.ErrCodeServerInstallFailed, 
       component, 
       "Installation failed"
   ).WithCause(err)
   ```

2. **Add user-friendly logging**:
   ```go
   // Before
   log.Printf("Installing %s...\n", component)
   
   // After
   logger.UserInfo(fmt.Sprintf("Installing %s...", component))
   ```

3. **Add progress tracking**:
   ```go
   // Before
   for i, item := range items {
       processItem(item)
   }
   
   // After
   progress := setup.NewProgressInfo("process-items", int64(len(items)))
   for i, item := range items {
       progress.Update(int64(i), item.Name)
       logger.UserProgress("Processing items", progress)
       processItem(item)
   }
   ```

## Performance Considerations

- Logging operations are thread-safe but may impact performance in high-frequency scenarios
- JSON logging is slower than human-readable format
- Metrics collection adds minimal overhead
- Progress tracking updates should be throttled for very fast operations

## Examples

See `internal/setup/example.go` for comprehensive usage examples including:
- Complete installation workflows
- Error scenario demonstrations
- Progress tracking examples
- Metrics collection patterns
- Integration with existing code

## Testing

Run the test suite:
```bash
go test ./internal/setup -v
```

The test suite includes:
- Unit tests for all error types
- Logger functionality tests
- Error handler tests
- Performance benchmarks
- Integration test examples