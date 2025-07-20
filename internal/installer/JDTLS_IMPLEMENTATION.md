# JDTLS (Java Language Server) Installation Implementation

## Overview

This document describes the comprehensive implementation of the Eclipse JDT Language Server (jdtls) installation logic in the LSP Gateway project. The implementation handles the complex manual download and setup process required for jdtls, which differs significantly from package manager-based installations used by other language servers.

## Implementation Details

### Main Installation Method: `InstallJdtls()`

The primary installation method follows a structured 7-phase approach:

1. **Java Runtime Verification**
   - Uses existing `JavaDetector` from `internal/setup` package
   - Verifies Java 17+ is available and compatible
   - Checks for JDK vs JRE (warns if only JRE available)
   - Validates JAVA_HOME and development tools

2. **Installation Directory Setup**
   - Uses standard path: `~/.local/share/lsp-gateway/jdtls`
   - Creates directory structure if needed
   - Handles platform-specific paths correctly

3. **Existing Installation Check**
   - Detects if jdtls is already installed
   - Extracts version information from existing installation
   - Skips download if compatible version exists

4. **Download Phase**
   - Downloads from Eclipse JDT Language Server snapshots
   - Uses Go's standard `net/http` package
   - Implements proper timeout handling (10 minutes)
   - Creates temporary file for download
   - Includes User-Agent header for identification

5. **Extraction Phase**
   - Extracts tar.gz archive using Go standard library
   - Implements security checks for path traversal attacks
   - Handles directories, regular files, and symbolic links
   - Preserves file permissions and directory structure

6. **Executable Wrapper Creation**
   - Creates platform-specific wrapper scripts
   - Unix: Shell script (`jdtls`)
   - Windows: Batch file (`jdtls.bat`)
   - Includes all required JVM arguments and Eclipse configurations
   - Handles workspace directory setup

7. **Installation Verification**
   - Tests that the wrapper script can execute
   - Handles LSP server-specific behavior (timeout is expected)
   - Extracts version information from installation

## Key Features

### Robust Error Handling

- **Network Errors**: HTTP download failures, timeouts, server errors
- **File System Errors**: Permission issues, disk space, path problems
- **Archive Errors**: Corrupted downloads, extraction failures
- **Runtime Errors**: Java not found, incompatible versions, JRE vs JDK issues
- **Verification Errors**: Installation validation failures

### Security Considerations

- **Path Traversal Protection**: Validates extraction paths to prevent directory traversal
- **Temporary File Cleanup**: Removes downloaded archives after extraction
- **Proper File Permissions**: Sets appropriate permissions on extracted files
- **Input Validation**: Validates all paths and user inputs

### Platform Compatibility

- **Cross-Platform Paths**: Uses `filepath.Join()` for platform-specific paths
- **Executable Scripts**: Creates appropriate wrapper scripts for each platform
- **Config Directories**: Handles platform-specific jdtls config directories
  - Linux: `config_linux`
  - macOS: `config_mac`
  - Windows: `config_win`

### Performance Optimizations

- **Existing Installation Detection**: Avoids re-downloading if already installed
- **Streaming Downloads**: Uses `io.Copy()` for memory-efficient downloads
- **Proper Timeouts**: Configurable timeouts for network operations
- **Cleanup**: Automatic cleanup of temporary files

## Helper Methods

### Installation Directory Management
- `getJdtlsInstallDir()`: Returns standard installation directory
- `isJdtlsInstalled()`: Checks for existing installation
- `getInstalledJdtlsVersion()`: Extracts version from existing installation
- `getJdtlsExecutablePath()`: Returns platform-specific executable path
- `getJdtlsConfigDir()`: Returns platform-specific config directory

### Download and Extraction
- `getJdtlsDownloadURL()`: Returns download URL for latest jdtls
- `downloadJdtls()`: Downloads archive to temporary file
- `extractJdtls()`: Extracts tar.gz archive to installation directory

### Wrapper Script Creation
- `createJdtlsWrapper()`: Creates platform-specific executable wrapper
- `findJdtlsLauncherJar()`: Locates equinox launcher jar in plugins
- `verifyJdtlsInstallation()`: Verifies installation is functional

## Integration with Existing Codebase

### Leverages Existing Components
- **Java Detection**: Uses `internal/setup/JavaDetector` for runtime verification
- **Command Execution**: Uses `internal/platform/CommandExecutor` for cross-platform commands
- **Error Types**: Uses existing installer error types and constructors
- **Result Structures**: Returns standard `InstallResult` with comprehensive information

### Follows Established Patterns
- **Interface Compliance**: Implements `ServerInstaller` interface
- **Error Handling**: Uses project-standard error types and wrapping
- **Logging**: Provides detailed messages, warnings, and errors
- **Configuration**: Integrates with existing server registry and definitions

## Configuration

The jdtls installation integrates with the existing server registry:

```go
// Java language server (jdtls)
s.servers["jdtls"] = &ServerDefinition{
    Name:        "jdtls",
    DisplayName: "Eclipse JDT Language Server",
    Runtime:     "java",
    InstallCmd:  "manual download from Eclipse",
    VerifyCmd:   "java -jar jdt-language-server.jar --version",
    ConfigName:  "java-lsp",
    Languages:   []string{"java"},
    Extensions:  []string{".java"},
    MinRuntime:  "17.0.0",
}
```

## Wrapper Script Details

### Unix Shell Script
```bash
#!/bin/bash
exec "/path/to/java" \
  -Declipse.application=org.eclipse.jdt.ls.core.id1 \
  -Dosgi.bundles.defaultStartLevel=4 \
  -Declipse.product=org.eclipse.jdt.ls.core.product \
  -Dlog.protocol=true \
  -Dlog.level=ALL \
  -Xms1g \
  -Xmx2G \
  --add-modules=ALL-SYSTEM \
  --add-opens java.base/java.util=ALL-UNNAMED \
  --add-opens java.base/java.lang=ALL-UNNAMED \
  -jar "/path/to/launcher.jar" \
  -configuration "/path/to/config" \
  -data "${TMPDIR:-/tmp}/jdtls-workspace" \
  "$@"
```

### Windows Batch Script
```batch
@echo off
"C:\path\to\java.exe" ^
  -Declipse.application=org.eclipse.jdt.ls.core.id1 ^
  -Dosgi.bundles.defaultStartLevel=4 ^
  -Declipse.product=org.eclipse.jdt.ls.core.product ^
  -Dlog.protocol=true ^
  -Dlog.level=ALL ^
  -Xms1g ^
  -Xmx2G ^
  --add-modules=ALL-SYSTEM ^
  --add-opens java.base/java.util=ALL-UNNAMED ^
  --add-opens java.base/java.lang=ALL-UNNAMED ^
  -jar "C:\path\to\launcher.jar" ^
  -configuration "C:\path\to\config" ^
  -data "%TEMP%\jdtls-workspace" ^
  %*
```

## Future Enhancements

### Version Management
- Implement GitHub API integration for latest release detection
- Add support for specific version installation
- Implement version upgrade capabilities

### Enhanced Verification
- Add LSP protocol communication test
- Implement workspace initialization verification
- Add Java project compilation test

### Installation Options
- Add custom installation directory support
- Implement proxy support for corporate environments
- Add offline installation support

## Dependencies

The implementation uses only Go standard library packages and existing project components:

### Standard Library
- `archive/tar`: Archive extraction
- `compress/gzip`: Gzip decompression
- `net/http`: HTTP downloads
- `os`: File system operations
- `path/filepath`: Cross-platform path handling
- `context`: Timeout and cancellation
- `time`: Timing and timeouts

### Project Dependencies
- `lsp-gateway/internal/platform`: Command execution
- `lsp-gateway/internal/setup`: Java runtime detection

## Error Examples

The implementation provides detailed error information:

```go
// Runtime dependency error
NewRuntimeDependencyError("java", "17.0.0", "11.0.2", false)

// Network error
NewInstallerError(InstallerErrorTypeNetwork, "jdtls", "Failed to download jdtls archive", err)

// Installation error
NewInstallerError(InstallerErrorTypeInstallation, "jdtls", "Failed to extract jdtls archive", err)

// Verification error
NewInstallerError(InstallerErrorTypeVerification, "jdtls", "Installation verification failed", err)
```

## Testing

The implementation includes comprehensive test coverage:

- Helper method validation
- Path generation testing
- Version extraction testing
- Wrapper content validation
- Cross-platform compatibility testing

## Conclusion

This implementation provides a robust, secure, and comprehensive solution for jdtls installation that integrates seamlessly with the existing LSP Gateway architecture while handling the unique complexities of manual language server installation.