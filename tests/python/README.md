# Python Unit Tests for LSP Gateway

This directory contains comprehensive Python unit tests that complement the Go-based integration tests in the LSP Gateway project.

## Overview

The LSP Gateway is a Go-based project that provides LSP (Language Server Protocol) integration for Python development. While the main integration testing is done through Go E2E tests, these Python unit tests focus on **Python-specific functionality** including:

- Python project detection and analysis
- Python configuration parsing (pyproject.toml, requirements.txt)
- LSP response validation and processing
- Python code analysis and symbol extraction
- Framework detection (Django, FastAPI, Flask, etc.)
- Dependency analysis and version compatibility

## Test Structure

### Test Modules

1. **`test_project_detection.py`** - Tests Python project detection logic
   - Django project detection and configuration parsing
   - FastAPI project detection with modern Python features
   - Machine learning project detection with scientific libraries
   - Virtual environment detection and analysis
   - Dependency analysis from requirements.txt and pyproject.toml
   - Framework detection patterns

2. **`test_lsp_integration.py`** - Tests LSP protocol integration
   - LSP response validation for all 6 supported methods
   - Python symbol extraction and analysis
   - Type hint analysis for LSP completion
   - Import analysis for LSP functionality
   - Python syntax error detection
   - LSP request generation and handling

3. **`test_python_utilities.py`** - Tests Python utility functions
   - File path and URI conversion utilities
   - Python executable and version detection
   - Project root detection
   - Site-packages path detection
   - Python version compatibility checking
   - Modern syntax feature detection
   - Dependency analysis from various configuration formats

## Running Tests

### Prerequisites

- Python 3.8 or higher
- Standard library modules (no external dependencies required)

### Quick Start

```bash
# Run all tests
python tests/python/run_tests.py

# Run with less verbose output
python tests/python/run_tests.py --verbose 1

# Run specific test module
python tests/python/run_tests.py --module project_detection

# Run specific test class
python tests/python/run_tests.py --class test_lsp_integration.TestPythonLSPIntegration

# List available tests
python tests/python/run_tests.py --list
```

### Alternative Test Execution

```bash
# Using unittest directly
cd tests/python
python -m unittest discover -v

# Run specific test file
python -m unittest test_project_detection -v

# Run specific test class
python -m unittest test_project_detection.PythonProjectDetectionTests -v

# Run specific test method
python -m unittest test_project_detection.PythonProjectDetectionTests.test_django_project_detection -v
```

### Integration with Go Tests

These Python tests complement the existing Go integration tests:

```bash
# Run Go integration tests
make test

# Run Python unit tests
python tests/python/run_tests.py

# Run both for comprehensive testing
make test && python tests/python/run_tests.py
```

## Test Philosophy

Following the project's **"streamlined, essential-only"** testing approach:

- **Focused Testing**: Tests only essential Python-specific functionality
- **Real-World Scenarios**: Uses realistic project structures and configurations
- **No Over-Engineering**: Simple, direct test implementations without complex frameworks
- **Complementary Coverage**: Fills gaps not covered by Go integration tests

## Test Categories

### Unit Tests (Fast - <30s)
```bash
python tests/python/run_tests.py
```

Tests core Python logic:
- Configuration parsing
- Project detection algorithms
- Utility functions
- LSP response validation

### Integration Tests (Within unit tests)
Some tests create temporary project structures to test:
- Multi-file project analysis
- Real configuration file parsing
- Framework detection across multiple files

## Test Data and Fixtures

Tests use realistic Python project structures:

- **Django Projects**: Complete Django setup with models, views, settings
- **FastAPI Projects**: Modern async Python with type hints and Pydantic
- **ML Projects**: Scientific Python with NumPy, Pandas, Scikit-learn
- **Configuration Files**: Real pyproject.toml, requirements.txt examples

## Expected Test Results

When all tests pass, you should see:

```
Discovered X test cases
Test directory: /path/to/tests/python
Test pattern: test_*.py
======================================================================
...
======================================================================
TEST SUMMARY
======================================================================
Tests run: X
Failures: 0
Errors: 0
Skipped: 0

✅ ALL TESTS PASSED!
```

## Troubleshooting

### Common Issues

1. **Import Errors**: Ensure you're running from the correct directory
   ```bash
   cd tests/python
   python run_tests.py
   ```

2. **Python Version**: Tests require Python 3.8+
   ```bash
   python --version  # Should be 3.8+
   ```

3. **Path Issues**: Use absolute paths if needed
   ```bash
   python /full/path/to/tests/python/run_tests.py
   ```

### Test Failures

If tests fail:

1. Check the detailed error output
2. Verify your Python environment
3. Ensure no conflicting Python packages
4. Check file permissions for temporary directory creation

## Integration with LSP Gateway

These tests validate Python-specific functionality that the Go LSP Gateway relies on:

- **Project Detection**: Ensures accurate Python project identification
- **Configuration Parsing**: Validates parsing of Python configuration files
- **LSP Protocol**: Confirms proper LSP response handling for Python
- **Framework Support**: Tests framework-specific feature detection

## Development Guidelines

When adding new Python unit tests:

1. **Follow Naming Convention**: `test_*.py` for modules, `Test*` for classes
2. **Use Realistic Data**: Create realistic project structures and configurations
3. **Keep Tests Independent**: Each test should be runnable in isolation
4. **Document Complex Logic**: Add docstrings for complex test scenarios
5. **Update This README**: Document new test modules and their purpose

## Performance Benchmarks

Expected test execution times:
- **All tests**: < 30 seconds
- **Project detection tests**: < 10 seconds
- **LSP integration tests**: < 15 seconds
- **Utility tests**: < 5 seconds

## Relationship to Go Tests

| Go Tests (Integration) | Python Tests (Unit) |
|----------------------|-------------------|
| Test LSP Gateway → Python LSP Server | Test Python-specific parsing logic |
| Test complete workflows | Test individual functions |
| Test protocol compliance | Test response validation |
| Test multi-language integration | Test Python project detection |
| Test performance at scale | Test algorithm correctness |

Both test suites are essential for comprehensive validation of Python support in the LSP Gateway.