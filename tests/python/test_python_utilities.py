"""
Python unit tests for Python utility functions and helpers.

This module tests utility functions that support Python language server integration
and complement the Go-based LSP Gateway's Python functionality.
"""

import unittest
import tempfile
import os
import json
import subprocess
import sys
from pathlib import Path
from typing import Dict, List, Any, Optional, Union, Tuple
from unittest.mock import Mock, patch, MagicMock
import re


class PythonPathUtils:
    """Utilities for Python path and environment management."""
    
    @staticmethod
    def normalize_file_uri(file_path: str) -> str:
        """Convert file path to LSP file URI format."""
        if file_path.startswith('file://'):
            return file_path
        
        # Convert to absolute path
        abs_path = os.path.abspath(file_path)
        
        # Convert to URI format
        if os.name == 'nt':  # Windows
            # Convert backslashes to forward slashes and handle drive letters
            abs_path = abs_path.replace('\\', '/')
            if abs_path[1] == ':':
                abs_path = abs_path[0].upper() + abs_path[1:]
            return f"file:///{abs_path}"
        else:  # Unix-like systems
            return f"file://{abs_path}"
    
    @staticmethod
    def uri_to_file_path(uri: str) -> str:
        """Convert LSP file URI to local file path."""
        if not uri.startswith('file://'):
            return uri
        
        path = uri[7:]  # Remove 'file://' prefix
        
        if os.name == 'nt':  # Windows
            # Handle Windows paths
            if path.startswith('/') and len(path) > 1 and path[2] == ':':
                path = path[1:]  # Remove leading slash for Windows drive letters
            path = path.replace('/', '\\')
        
        return path
    
    @staticmethod
    def find_python_executable() -> str:
        """Find the Python executable path."""
        # Try common Python executable names
        python_names = ['python3', 'python', 'py']
        
        for name in python_names:
            try:
                result = subprocess.run([name, '--version'], 
                                      capture_output=True, text=True, timeout=5)
                if result.returncode == 0 and 'Python' in result.stdout:
                    return name
            except (subprocess.TimeoutExpired, FileNotFoundError):
                continue
        
        # Fallback to sys.executable
        return sys.executable
    
    @staticmethod
    def get_python_version(python_executable: str = None) -> Optional[str]:
        """Get Python version string."""
        if python_executable is None:
            python_executable = PythonPathUtils.find_python_executable()
        
        try:
            result = subprocess.run([python_executable, '--version'], 
                                  capture_output=True, text=True, timeout=5)
            if result.returncode == 0:
                # Extract version from output like "Python 3.11.5"
                match = re.search(r'Python (\d+\.\d+\.\d+)', result.stdout)
                return match.group(1) if match else None
        except (subprocess.TimeoutExpired, FileNotFoundError):
            pass
        
        return None
    
    @staticmethod
    def find_project_root(start_path: str) -> Optional[str]:
        """Find Python project root by looking for common project files."""
        project_indicators = [
            'pyproject.toml',
            'setup.py',
            'requirements.txt',
            'Pipfile',
            'poetry.lock',
            'setup.cfg',
            'tox.ini',
            '.git'
        ]
        
        current_path = Path(start_path).resolve()
        
        while current_path != current_path.parent:
            for indicator in project_indicators:
                if (current_path / indicator).exists():
                    return str(current_path)
            
            current_path = current_path.parent
        
        return None
    
    @staticmethod
    def get_site_packages_paths(python_executable: str = None) -> List[str]:
        """Get site-packages paths for Python installation."""
        if python_executable is None:
            python_executable = PythonPathUtils.find_python_executable()
        
        try:
            code = "import site; print('\\n'.join(site.getsitepackages()))"
            result = subprocess.run([python_executable, '-c', code], 
                                  capture_output=True, text=True, timeout=10)
            if result.returncode == 0:
                return [path.strip() for path in result.stdout.strip().split('\n') if path.strip()]
        except (subprocess.TimeoutExpired, FileNotFoundError):
            pass
        
        return []


class PythonVersionDetector:
    """Detect and analyze Python version compatibility."""
    
    def __init__(self):
        self.version_features = {
            (3, 8): ['positional_only_parameters', 'assignment_expressions'],
            (3, 9): ['dict_merge_operators', 'generic_types'],
            (3, 10): ['pattern_matching', 'parenthesized_context_managers'],
            (3, 11): ['exception_groups', 'tomllib'],
            (3, 12): ['type_parameter_syntax', 'f_string_improvements'],
        }
    
    def parse_version_string(self, version_str: str) -> Optional[Tuple[int, int, int]]:
        """Parse version string into tuple."""
        match = re.match(r'(\d+)\.(\d+)\.(\d+)', version_str)
        if match:
            return tuple(map(int, match.groups()))
        return None
    
    def get_supported_features(self, version: Union[str, Tuple[int, int, int]]) -> List[str]:
        """Get list of supported Python features for version."""
        if isinstance(version, str):
            version_tuple = self.parse_version_string(version)
            if not version_tuple:
                return []
        else:
            version_tuple = version
        
        features = []
        for (major, minor), feature_list in self.version_features.items():
            if version_tuple[:2] >= (major, minor):
                features.extend(feature_list)
        
        return features
    
    def is_version_compatible(self, required_version: str, actual_version: str) -> bool:
        """Check if actual version meets required version."""
        req_tuple = self.parse_version_string(required_version)
        act_tuple = self.parse_version_string(actual_version)
        
        if not req_tuple or not act_tuple:
            return False
        
        return act_tuple >= req_tuple
    
    def detect_modern_syntax(self, code: str) -> Dict[str, bool]:
        """Detect modern Python syntax features in code."""
        features = {
            'f_strings': bool(re.search(r'f["\']', code)),
            'type_hints': bool(re.search(r':\s*[A-Z]', code)),
            'dataclasses': '@dataclass' in code,
            'async_await': bool(re.search(r'\basync\s+def\b', code)),
            'match_statements': bool(re.search(r'\bmatch\s+\w+:', code)),
            'assignment_expressions': bool(re.search(r':=', code)),
            'positional_only': bool(re.search(r'def\s+\w+\([^)]*/', code)),
            'union_types': bool(re.search(r'\|\s*\w+', code)),
        }
        
        return features


class PythonDependencyAnalyzer:
    """Analyze Python dependencies and package information."""
    
    def __init__(self):
        self.framework_patterns = {
            'django': [
                r'from django',
                r'import django',
                r'DJANGO_SETTINGS_MODULE',
                r'manage\.py',
            ],
            'fastapi': [
                r'from fastapi',
                r'import fastapi',
                r'FastAPI\(',
                r'@app\.(get|post|put|delete)',
            ],
            'flask': [
                r'from flask',
                r'import flask',
                r'Flask\(',
                r'@app\.route',
            ],
            'pytest': [
                r'import pytest',
                r'def test_',
                r'@pytest\.',
                r'pytest\.',
            ],
            'numpy': [
                r'import numpy',
                r'np\.',
                r'numpy\.',
            ],
            'pandas': [
                r'import pandas',
                r'pd\.',
                r'pandas\.',
                r'DataFrame',
            ],
            'sklearn': [
                r'from sklearn',
                r'import sklearn',
                r'scikit.learn',
            ],
            'tensorflow': [
                r'import tensorflow',
                r'tf\.',
                r'tensorflow\.',
            ],
            'torch': [
                r'import torch',
                r'torch\.',
                r'from torch',
            ],
        }
    
    def detect_frameworks_in_code(self, code: str) -> List[str]:
        """Detect frameworks used in Python code."""
        detected = []
        
        for framework, patterns in self.framework_patterns.items():
            for pattern in patterns:
                if re.search(pattern, code, re.IGNORECASE):
                    detected.append(framework)
                    break
        
        return detected
    
    def parse_requirements_txt(self, content: str) -> Dict[str, Any]:
        """Parse requirements.txt content."""
        dependencies = {}
        dev_dependencies = {}
        options = {}
        
        for line in content.split('\n'):
            line = line.strip()
            
            # Skip empty lines and comments
            if not line or line.startswith('#'):
                continue
            
            # Handle pip options
            if line.startswith('-'):
                if line.startswith('--'):
                    if '=' in line:
                        option, value = line.split('=', 1)
                        options[option] = value
                    else:
                        options[line] = True
                elif line.startswith('-e '):
                    # Editable installs
                    dependencies['_editable'] = dependencies.get('_editable', [])
                    dependencies['_editable'].append(line[3:])
                continue
            
            # Parse dependency specification
            dep_info = self._parse_dependency_spec(line)
            if dep_info:
                dependencies[dep_info['name']] = dep_info
        
        return {
            'dependencies': dependencies,
            'options': options
        }
    
    def _parse_dependency_spec(self, spec: str) -> Optional[Dict[str, Any]]:
        """Parse a single dependency specification."""
        # Handle extras like requests[security]
        extras_match = re.match(r'([^[]+)(\[([^\]]+)\])?(.*)$', spec)
        if not extras_match:
            return None
        
        name_part = extras_match.group(1)
        extras = extras_match.group(3)
        version_part = extras_match.group(4)
        
        # Parse version constraints
        version_operators = ['==', '>=', '<=', '>', '<', '~=', '!=']
        constraints = []
        
        if version_part:
            for op in version_operators:
                if op in version_part:
                    parts = version_part.split(op)
                    if len(parts) == 2:
                        constraints.append({
                            'operator': op,
                            'version': parts[1].strip(' ,')
                        })
        
        return {
            'name': name_part.strip(),
            'extras': extras.split(',') if extras else [],
            'version_constraints': constraints,
            'raw_spec': spec
        }
    
    def analyze_pyproject_toml(self, content: str) -> Dict[str, Any]:
        """Analyze pyproject.toml for project information."""
        # Simple TOML-like parsing for testing
        result = {
            'project_name': None,
            'version': None,
            'python_requires': None,
            'dependencies': {},
            'dev_dependencies': {},
            'build_system': {},
            'tools': {}
        }
        
        lines = content.split('\n')
        current_section = None
        
        for line in lines:
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            
            # Section headers
            if line.startswith('[') and line.endswith(']'):
                current_section = line[1:-1]
                continue
            
            # Key-value pairs
            if '=' in line and current_section:
                key, value = line.split('=', 1)
                key = key.strip()
                value = value.strip().strip('"\'')
                
                if current_section == 'tool.poetry':
                    if key == 'name':
                        result['project_name'] = value
                    elif key == 'version':
                        result['version'] = value
                elif current_section == 'tool.poetry.dependencies':
                    result['dependencies'][key] = value
                elif 'dev' in current_section.lower():
                    result['dev_dependencies'][key] = value
                elif current_section == 'build-system':
                    result['build_system'][key] = value
        
        return result


class TestPythonPathUtils(unittest.TestCase):
    """Test Python path and URI utilities."""
    
    def test_normalize_file_uri(self):
        """Test file path to URI normalization."""
        # Unix paths
        if os.name != 'nt':
            uri = PythonPathUtils.normalize_file_uri('/home/user/project/main.py')
            self.assertTrue(uri.startswith('file://'))
            self.assertIn('/home/user/project/main.py', uri)
        
        # Already normalized URI
        existing_uri = 'file:///workspace/test.py'
        normalized = PythonPathUtils.normalize_file_uri(existing_uri)
        self.assertEqual(normalized, existing_uri)
        
        # Relative path should be converted to absolute
        temp_file = tempfile.NamedTemporaryFile(delete=False)
        try:
            relative_path = os.path.basename(temp_file.name)
            uri = PythonPathUtils.normalize_file_uri(relative_path)
            self.assertTrue(uri.startswith('file://'))
            self.assertIn(temp_file.name, uri)
        finally:
            os.unlink(temp_file.name)
    
    def test_uri_to_file_path(self):
        """Test URI to file path conversion."""
        # Unix URI
        if os.name != 'nt':
            file_path = PythonPathUtils.uri_to_file_path('file:///home/user/test.py')
            self.assertEqual(file_path, '/home/user/test.py')
        
        # Non-URI path should pass through unchanged
        regular_path = '/home/user/test.py'
        converted = PythonPathUtils.uri_to_file_path(regular_path)
        self.assertEqual(converted, regular_path)
    
    def test_find_python_executable(self):
        """Test Python executable detection."""
        python_exe = PythonPathUtils.find_python_executable()
        self.assertIsNotNone(python_exe)
        self.assertTrue(len(python_exe) > 0)
        
        # Should be able to run the found executable
        try:
            result = subprocess.run([python_exe, '--version'], 
                                  capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode, 0)
            self.assertIn('Python', result.stdout)
        except (subprocess.TimeoutExpired, FileNotFoundError):
            self.fail("Found Python executable is not functional")
    
    def test_get_python_version(self):
        """Test Python version detection."""
        version = PythonPathUtils.get_python_version()
        self.assertIsNotNone(version)
        
        # Should be a valid version string
        version_pattern = r'^\d+\.\d+\.\d+$'
        self.assertRegex(version, version_pattern)
        
        # Should be Python 3.x
        major_version = int(version.split('.')[0])
        self.assertGreaterEqual(major_version, 3)
    
    def test_find_project_root(self):
        """Test project root detection."""
        # Create temporary project structure
        with tempfile.TemporaryDirectory() as temp_dir:
            project_dir = Path(temp_dir) / 'project'
            src_dir = project_dir / 'src'
            src_dir.mkdir(parents=True)
            
            # Create project indicator file
            (project_dir / 'pyproject.toml').write_text('[tool.poetry]\nname = "test"')
            
            # Should find project root from subdirectory
            root = PythonPathUtils.find_project_root(str(src_dir))
            self.assertEqual(root, str(project_dir))
            
            # Should find project root from project directory itself
            root = PythonPathUtils.find_project_root(str(project_dir))
            self.assertEqual(root, str(project_dir))
    
    def test_get_site_packages_paths(self):
        """Test site-packages path detection."""
        paths = PythonPathUtils.get_site_packages_paths()
        self.assertIsInstance(paths, list)
        
        # Should have at least one path
        if paths:  # May be empty in some environments
            for path in paths:
                self.assertTrue(os.path.isabs(path))
                # Path should exist or be a reasonable site-packages location
                self.assertTrue('site-packages' in path or 'dist-packages' in path or os.path.exists(path))


class TestPythonVersionDetector(unittest.TestCase):
    """Test Python version detection and compatibility."""
    
    def setUp(self):
        """Set up test fixtures."""
        self.detector = PythonVersionDetector()
    
    def test_parse_version_string(self):
        """Test version string parsing."""
        # Valid version strings
        self.assertEqual(self.detector.parse_version_string('3.11.5'), (3, 11, 5))
        self.assertEqual(self.detector.parse_version_string('3.9.0'), (3, 9, 0))
        self.assertEqual(self.detector.parse_version_string('2.7.18'), (2, 7, 18))
        
        # Invalid version strings
        self.assertIsNone(self.detector.parse_version_string('invalid'))
        self.assertIsNone(self.detector.parse_version_string('3.11'))
        self.assertIsNone(self.detector.parse_version_string(''))
    
    def test_get_supported_features(self):
        """Test feature detection for Python versions."""
        # Python 3.8 features
        features_38 = self.detector.get_supported_features('3.8.10')
        self.assertIn('positional_only_parameters', features_38)
        self.assertIn('assignment_expressions', features_38)
        
        # Python 3.10 should include all previous features
        features_310 = self.detector.get_supported_features('3.10.0')
        self.assertIn('positional_only_parameters', features_310)
        self.assertIn('pattern_matching', features_310)
        
        # Python 3.12 should include the most features
        features_312 = self.detector.get_supported_features('3.12.0')
        self.assertIn('type_parameter_syntax', features_312)
        self.assertIn('f_string_improvements', features_312)
    
    def test_is_version_compatible(self):
        """Test version compatibility checking."""
        # Compatible versions
        self.assertTrue(self.detector.is_version_compatible('3.8.0', '3.11.5'))
        self.assertTrue(self.detector.is_version_compatible('3.9.0', '3.9.0'))
        self.assertTrue(self.detector.is_version_compatible('3.10.0', '3.12.1'))
        
        # Incompatible versions
        self.assertFalse(self.detector.is_version_compatible('3.11.0', '3.10.5'))
        self.assertFalse(self.detector.is_version_compatible('3.12.0', '3.8.10'))
    
    def test_detect_modern_syntax(self):
        """Test modern Python syntax detection."""
        modern_code = '''
from typing import List, Dict
from dataclasses import dataclass

@dataclass
class User:
    name: str
    age: int

async def process_users(users: List[User]) -> Dict[str, User]:
    result = {}
    for user in users:
        if (age := user.age) >= 18:  # Assignment expression
            result[f"user_{user.name}"] = user  # f-string
    return result

def greet(name: str, /, greeting: str = "Hello") -> str:  # Positional-only parameter
    return f"{greeting}, {name}!"

def handle_response(response):
    match response.status:  # Match statement
        case 200:
            return "Success"
        case 404:
            return "Not Found"
        case _:
            return "Unknown"
'''
        
        features = self.detector.detect_modern_syntax(modern_code)
        
        self.assertTrue(features['f_strings'])
        self.assertTrue(features['type_hints'])
        self.assertTrue(features['dataclasses'])
        self.assertTrue(features['async_await'])
        self.assertTrue(features['match_statements'])
        self.assertTrue(features['assignment_expressions'])
        self.assertTrue(features['positional_only'])
        
        # Test code without modern features
        old_code = '''
def add_numbers(a, b):
    return a + b

class Calculator:
    def multiply(self, x, y):
        return x * y
'''
        
        old_features = self.detector.detect_modern_syntax(old_code)
        self.assertFalse(old_features['f_strings'])
        self.assertFalse(old_features['type_hints'])
        self.assertFalse(old_features['dataclasses'])


class TestPythonDependencyAnalyzer(unittest.TestCase):
    """Test Python dependency analysis."""
    
    def setUp(self):
        """Set up test fixtures."""
        self.analyzer = PythonDependencyAnalyzer()
    
    def test_detect_frameworks_in_code(self):
        """Test framework detection in Python code."""
        django_code = '''
from django.db import models
from django.http import HttpResponse

class User(models.Model):
    name = models.CharField(max_length=100)

def index(request):
    return HttpResponse("Hello Django")
'''
        
        frameworks = self.analyzer.detect_frameworks_in_code(django_code)
        self.assertIn('django', frameworks)
        
        fastapi_code = '''
from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello FastAPI"}
'''
        
        frameworks = self.analyzer.detect_frameworks_in_code(fastapi_code)
        self.assertIn('fastapi', frameworks)
        
        # Multiple frameworks
        mixed_code = '''
import pytest
import numpy as np
from sklearn.model_selection import train_test_split

def test_model():
    data = np.array([1, 2, 3, 4, 5])
    train, test = train_test_split(data)
    assert len(train) > 0
'''
        
        frameworks = self.analyzer.detect_frameworks_in_code(mixed_code)
        self.assertIn('pytest', frameworks)
        self.assertIn('numpy', frameworks)
        self.assertIn('sklearn', frameworks)
    
    def test_parse_requirements_txt(self):
        """Test requirements.txt parsing."""
        requirements_content = '''
# Core dependencies
Django==4.2.0
requests>=2.28.0,<3.0.0
python-decouple~=3.8
click

# Development
pytest>=7.0.0
black[d]>=23.0.0

# Optional
-e git+https://github.com/example/repo.git#egg=example
--index-url https://pypi.org/simple/
'''
        
        result = self.analyzer.parse_requirements_txt(requirements_content)
        
        dependencies = result['dependencies']
        self.assertIn('Django', dependencies)
        self.assertIn('requests', dependencies)
        self.assertIn('python-decouple', dependencies)
        self.assertIn('click', dependencies)
        self.assertIn('pytest', dependencies)
        self.assertIn('black', dependencies)
        
        # Check version constraints
        django_dep = dependencies['Django']
        self.assertEqual(django_dep['name'], 'Django')
        self.assertEqual(len(django_dep['version_constraints']), 1)
        self.assertEqual(django_dep['version_constraints'][0]['operator'], '==')
        self.assertEqual(django_dep['version_constraints'][0]['version'], '4.2.0')
        
        # Check extras
        black_dep = dependencies['black']
        self.assertEqual(black_dep['extras'], ['d'])
        
        # Check editable installs
        self.assertIn('_editable', dependencies)
        self.assertIn('git+https://github.com/example/repo.git#egg=example', dependencies['_editable'])
        
        # Check options
        options = result['options']
        self.assertIn('--index-url', options)
        self.assertEqual(options['--index-url'], 'https://pypi.org/simple/')
    
    def test_analyze_pyproject_toml(self):
        """Test pyproject.toml analysis."""
        pyproject_content = '''
[build-system]
requires = ["poetry-core>=1.0.0"]
build-backend = "poetry.core.masonry.api"

[tool.poetry] 
name = "my-project"
version = "1.0.0"
description = "A sample project"

[tool.poetry.dependencies]
python = "^3.11"
fastapi = "^0.104.0"
pydantic = "^2.4.0"

[tool.poetry.group.dev.dependencies]
pytest = "^7.4.0"
mypy = "^1.5.0"
black = "^23.0.0"
'''
        
        result = self.analyzer.analyze_pyproject_toml(pyproject_content)
        
        self.assertEqual(result['project_name'], 'my-project')
        self.assertEqual(result['version'], '1.0.0')
        
        dependencies = result['dependencies']
        self.assertIn('python', dependencies)
        self.assertIn('fastapi', dependencies)
        self.assertIn('pydantic', dependencies)
        
        dev_dependencies = result['dev_dependencies']
        self.assertIn('pytest', dev_dependencies)
        self.assertIn('mypy', dev_dependencies)
        self.assertIn('black', dev_dependencies)
        
        build_system = result['build_system']
        self.assertIn('requires', build_system)
        self.assertIn('build-backend', build_system)


class TestPythonCodeAnalysis(unittest.TestCase):
    """Test Python code analysis utilities."""
    
    def test_extract_imports(self):
        """Test import extraction from Python code."""
        
        def extract_all_imports(code: str) -> Dict[str, List[str]]:
            """Extract all imports from Python code."""
            imports = {
                'standard': [],
                'third_party': [],
                'local': [],
                'from_imports': {}
            }
            
            lines = code.split('\n')
            for line in lines:
                line = line.strip()
                
                # Regular imports
                if line.startswith('import '):
                    modules = line[7:].split(',')
                    for module in modules:
                        module = module.strip()
                        if module in ['os', 'sys', 're', 'json', 'datetime', 'pathlib', 'typing']:
                            imports['standard'].append(module)
                        elif '.' in module:
                            imports['local'].append(module)
                        else:
                            imports['third_party'].append(module)
                
                # From imports
                elif line.startswith('from '):
                    parts = line.split(' import ')
                    if len(parts) == 2:
                        module = parts[0][5:].strip()
                        items = [item.strip() for item in parts[1].split(',')]
                        imports['from_imports'][module] = items
                        
                        if module.startswith('.'):
                            imports['local'].append(module)
                        elif module in ['os', 'sys', 're', 'json', 'datetime', 'pathlib', 'typing']:
                            imports['standard'].append(module)
                        else:
                            imports['third_party'].append(module)
            
            return imports
        
        sample_code = '''
import os
import sys
from typing import List, Dict, Optional
from pathlib import Path
import requests
import numpy as np
from django.db import models
from .local_module import helper
from ..parent_module import utils
'''
        
        imports = extract_all_imports(sample_code)
        
        # Check standard library imports
        std_imports = imports['standard'] + imports['from_imports'].get('typing', [])
        self.assertIn('os', std_imports)
        self.assertIn('sys', std_imports)
        self.assertIn('typing', std_imports)
        self.assertIn('pathlib', std_imports)
        
        # Check third-party imports
        third_party = imports['third_party'] + list(imports['from_imports'].keys())
        self.assertIn('requests', third_party)
        self.assertIn('numpy', third_party)
        self.assertIn('django.db', third_party)
        
        # Check local imports
        self.assertIn('.local_module', imports['local'])
        self.assertIn('..parent_module', imports['local'])
    
    def test_function_signature_analysis(self):
        """Test function signature extraction and analysis."""
        
        def extract_function_signatures(code: str) -> List[Dict[str, Any]]:
            """Extract function signatures from Python code."""
            import re
            
            signatures = []
            
            # Pattern for function definitions with type hints
            func_pattern = r'(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)\s*(?:->\s*([^:]+))?:'
            
            for match in re.finditer(func_pattern, code, re.MULTILINE):
                is_async = 'async' in match.group(0)
                func_name = match.group(1)
                params_str = match.group(2)
                return_type = match.group(3).strip() if match.group(3) else None
                
                # Parse parameters
                parameters = []
                if params_str.strip():
                    for param in params_str.split(','):
                        param = param.strip()
                        if param:
                            param_info = {'name': param}
                            
                            # Check for type hints
                            if ':' in param:
                                name_part, type_part = param.split(':', 1)
                                param_info['name'] = name_part.strip()
                                param_info['type'] = type_part.split('=')[0].strip()
                                
                                # Check for default value
                                if '=' in type_part:
                                    param_info['default'] = type_part.split('=', 1)[1].strip()
                            elif '=' in param:
                                name_part, default_part = param.split('=', 1)
                                param_info['name'] = name_part.strip()
                                param_info['default'] = default_part.strip()
                            
                            parameters.append(param_info)
                
                signatures.append({
                    'name': func_name,
                    'is_async': is_async,
                    'parameters': parameters,
                    'return_type': return_type
                })
            
            return signatures
        
        function_code = '''
def simple_function():
    pass

def typed_function(name: str, age: int = 25) -> str:
    return f"Hello, {name}!"

async def async_function(data: List[Dict[str, Any]]) -> Optional[Dict]:
    return None

def complex_function(
    required_param: str,
    optional_param: int = 10,
    *args,
    **kwargs
) -> Tuple[str, int]:
    return required_param, optional_param
'''
        
        signatures = extract_function_signatures(function_code)
        
        # Check function names
        func_names = [sig['name'] for sig in signatures]
        self.assertIn('simple_function', func_names)
        self.assertIn('typed_function', func_names)
        self.assertIn('async_function', func_names)
        self.assertIn('complex_function', func_names)
        
        # Check async detection
        async_func = next((sig for sig in signatures if sig['name'] == 'async_function'), None)
        self.assertIsNotNone(async_func)
        self.assertTrue(async_func['is_async'])
        
        # Check parameter parsing
        typed_func = next((sig for sig in signatures if sig['name'] == 'typed_function'), None)
        self.assertIsNotNone(typed_func)
        self.assertEqual(len(typed_func['parameters']), 2)
        
        name_param = typed_func['parameters'][0]
        self.assertEqual(name_param['name'], 'name')
        self.assertEqual(name_param['type'], 'str')
        self.assertNotIn('default', name_param)
        
        age_param = typed_func['parameters'][1]
        self.assertEqual(age_param['name'], 'age')
        self.assertEqual(age_param['type'], 'int')
        self.assertEqual(age_param['default'], '25')
        
        # Check return type
        self.assertEqual(typed_func['return_type'], 'str')


if __name__ == '__main__':
    # Run all tests
    unittest.main(verbosity=2)