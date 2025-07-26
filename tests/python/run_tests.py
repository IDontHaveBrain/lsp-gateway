#!/usr/bin/env python3
"""
Test runner for Python unit tests in LSP Gateway project.

This script runs all Python unit tests and provides detailed reporting
to complement the Go-based integration tests.
"""

import unittest
import sys
import os
from pathlib import Path
from typing import List, Optional


def discover_and_run_tests(test_directory: Optional[str] = None, pattern: str = 'test_*.py', verbosity: int = 2) -> bool:
    """
    Discover and run Python unit tests.
    
    Args:
        test_directory: Directory to search for tests (defaults to current directory)
        pattern: Pattern to match test files
        verbosity: Test output verbosity level
        
    Returns:
        True if all tests passed, False otherwise
    """
    if test_directory is None:
        test_directory = os.path.dirname(os.path.abspath(__file__))
    
    # Discover tests
    loader = unittest.TestLoader()
    start_dir = test_directory
    suite = loader.discover(start_dir, pattern=pattern)
    
    # Count total tests
    total_tests = suite.countTestCases()
    print(f"Discovered {total_tests} test cases")
    print(f"Test directory: {test_directory}")
    print(f"Test pattern: {pattern}")
    print("-" * 70)
    
    # Run tests
    runner = unittest.TextTestRunner(
        verbosity=verbosity,
        buffer=True,  # Suppress stdout/stderr during tests
        failfast=False,  # Continue running tests after failures
    )
    
    result = runner.run(suite)
    
    # Print summary
    print("\n" + "=" * 70)
    print("TEST SUMMARY")
    print("=" * 70)
    print(f"Tests run: {result.testsRun}")
    print(f"Failures: {len(result.failures)}")
    print(f"Errors: {len(result.errors)}")
    print(f"Skipped: {len(result.skipped)}")
    
    if result.failures:
        print("\nFAILURES:")
        for test, traceback in result.failures:
            print(f"- {test}: {traceback.split('AssertionError:')[-1].strip()}")
    
    if result.errors:
        print("\nERRORS:")
        for test, traceback in result.errors:
            print(f"- {test}: {traceback.split('Exception:')[-1].strip()}")
    
    success = len(result.failures) == 0 and len(result.errors) == 0
    
    if success:
        print("\n✅ ALL TESTS PASSED!")
    else:
        print(f"\n❌ {len(result.failures) + len(result.errors)} TESTS FAILED")
    
    return success


def run_specific_test_module(module_name: str, verbosity: int = 2) -> bool:
    """
    Run tests from a specific module.
    
    Args:
        module_name: Name of the test module (e.g., 'test_project_detection')
        verbosity: Test output verbosity level
        
    Returns:
        True if all tests passed, False otherwise
    """
    try:
        # Import the test module
        if module_name.startswith('test_'):
            module_name = module_name[5:]  # Remove 'test_' prefix if present
        
        full_module_name = f"test_{module_name}"
        
        loader = unittest.TestLoader()
        suite = loader.loadTestsFromName(full_module_name)
        
        runner = unittest.TextTestRunner(verbosity=verbosity, buffer=True)
        result = runner.run(suite)
        
        return len(result.failures) == 0 and len(result.errors) == 0
        
    except ImportError as e:
        print(f"Error importing test module '{module_name}': {e}")
        return False


def run_test_class(class_name: str, verbosity: int = 2) -> bool:
    """
    Run tests from a specific test class.
    
    Args:
        class_name: Full test class name (e.g., 'test_project_detection.TestPythonProjectDetection')
        verbosity: Test output verbosity level
        
    Returns:
        True if all tests passed, False otherwise
    """
    try:
        loader = unittest.TestLoader()
        suite = loader.loadTestsFromName(class_name)
        
        runner = unittest.TextTestRunner(verbosity=verbosity, buffer=True)
        result = runner.run(suite)
        
        return len(result.failures) == 0 and len(result.errors) == 0
        
    except Exception as e:
        print(f"Error running test class '{class_name}': {e}")
        return False


def main():
    """Main entry point for test runner."""
    import argparse
    
    parser = argparse.ArgumentParser(
        description="Run Python unit tests for LSP Gateway",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s                                    # Run all tests
  %(prog)s --module project_detection         # Run specific module
  %(prog)s --class test_project_detection.TestPythonProjectDetection  # Run specific class
  %(prog)s --pattern "*lsp*"                  # Run tests matching pattern
  %(prog)s --verbose 1                        # Less verbose output
        """
    )
    
    parser.add_argument(
        '--module', '-m',
        help='Run tests from specific module (e.g., project_detection)'
    )
    
    parser.add_argument(
        '--class', '-c',
        dest='test_class',
        help='Run specific test class (e.g., test_project_detection.TestPythonProjectDetection)'
    )
    
    parser.add_argument(
        '--pattern', '-p',
        default='test_*.py',
        help='Pattern to match test files (default: test_*.py)'
    )
    
    parser.add_argument(
        '--directory', '-d',
        help='Directory to search for tests (default: current directory)'
    )
    
    parser.add_argument(
        '--verbose', '-v',
        type=int,
        choices=[0, 1, 2],
        default=2,
        help='Verbosity level (0=quiet, 1=normal, 2=verbose, default: 2)'
    )
    
    parser.add_argument(
        '--list', '-l',
        action='store_true',
        help='List available test modules and classes'
    )
    
    args = parser.parse_args()
    
    # Change to the directory containing this script
    script_dir = os.path.dirname(os.path.abspath(__file__))
    os.chdir(script_dir)
    
    # Add current directory to Python path
    if script_dir not in sys.path:
        sys.path.insert(0, script_dir)
    
    if args.list:
        # List available tests
        print("Available test modules:")
        for file in Path('.').glob('test_*.py'):
            if file.name != 'test_runner.py':
                print(f"  - {file.stem}")
        
        # Try to list test classes
        print("\nAvailable test classes:")
        try:
            for file in Path('.').glob('test_*.py'):
                if file.name == 'run_tests.py':
                    continue
                
                module_name = file.stem
                try:
                    module = __import__(module_name)
                    for attr_name in dir(module):
                        attr = getattr(module, attr_name)
                        if (isinstance(attr, type) and 
                            issubclass(attr, unittest.TestCase) and 
                            attr != unittest.TestCase):
                            print(f"  - {module_name}.{attr_name}")
                except ImportError:
                    continue
        except Exception as e:
            print(f"Error listing test classes: {e}")
        
        return 0
    
    success = False
    
    if args.test_class:
        print(f"Running test class: {args.test_class}")
        success = run_test_class(args.test_class, args.verbose)
    elif args.module:
        print(f"Running test module: {args.module}")
        success = run_specific_test_module(args.module, args.verbose)
    else:
        print("Running all Python unit tests...")
        success = discover_and_run_tests(
            test_directory=args.directory,
            pattern=args.pattern,
            verbosity=args.verbose
        )
    
    return 0 if success else 1


if __name__ == '__main__':
    sys.exit(main())