"""
Python unit tests for LSP integration functionality.

This module tests Python-specific LSP integration logic that complements
the Go-based LSP Gateway's integration capabilities.
"""

import unittest
import json
import tempfile
import os
from pathlib import Path
from typing import Dict, List, Any, Optional, Union
from dataclasses import dataclass
from unittest.mock import Mock, patch, MagicMock
import asyncio


@dataclass
class LSPPosition:
    """LSP position information."""
    line: int
    character: int


@dataclass
class LSPRange:
    """LSP range information."""
    start: LSPPosition
    end: LSPPosition


@dataclass
class LSPLocation:
    """LSP location information."""
    uri: str
    range: LSPRange


class PythonLSPResponse:
    """Python LSP response handler and validator."""
    
    @staticmethod
    def validate_definition_response(response: Dict[str, Any]) -> bool:
        """Validate textDocument/definition response."""
        if isinstance(response, list):
            # Multiple definitions
            for item in response:
                if not PythonLSPResponse._validate_location(item):
                    return False
            return True
        else:
            # Single definition
            return PythonLSPResponse._validate_location(response)
    
    @staticmethod
    def validate_references_response(response: List[Dict[str, Any]]) -> bool:
        """Validate textDocument/references response."""
        if not isinstance(response, list):
            return False
        
        for ref in response:
            if not PythonLSPResponse._validate_location(ref):
                return False
        
        return True
    
    @staticmethod
    def validate_hover_response(response: Dict[str, Any]) -> bool:
        """Validate textDocument/hover response."""
        required_fields = ['contents']
        
        for field in required_fields:
            if field not in response:
                return False
        
        contents = response['contents']
        if isinstance(contents, dict):
            return 'kind' in contents and 'value' in contents
        elif isinstance(contents, list):
            return len(contents) > 0
        elif isinstance(contents, str):
            return len(contents) > 0
        
        return False
    
    @staticmethod
    def validate_completion_response(response: Dict[str, Any]) -> bool:
        """Validate textDocument/completion response."""
        if 'items' not in response:
            return False
        
        items = response['items']
        if not isinstance(items, list):
            return False
        
        for item in items:
            if not isinstance(item, dict) or 'label' not in item:
                return False
        
        return True
    
    @staticmethod
    def validate_document_symbols_response(response: List[Dict[str, Any]]) -> bool:
        """Validate textDocument/documentSymbol response."""
        if not isinstance(response, list):
            return False
        
        for symbol in response:
            if not PythonLSPResponse._validate_document_symbol(symbol):
                return False
        
        return True
    
    @staticmethod
    def validate_workspace_symbols_response(response: List[Dict[str, Any]]) -> bool:
        """Validate workspace/symbol response."""
        if not isinstance(response, list):
            return False
        
        for symbol in response:
            required_fields = ['name', 'kind', 'location']
            for field in required_fields:
                if field not in symbol:
                    return False
            
            if not PythonLSPResponse._validate_location(symbol['location']):
                return False
        
        return True
    
    @staticmethod
    def _validate_location(location: Dict[str, Any]) -> bool:
        """Validate LSP location structure."""
        required_fields = ['uri', 'range']
        
        for field in required_fields:
            if field not in location:
                return False
        
        range_obj = location['range']
        if not isinstance(range_obj, dict):
            return False
        
        for pos_key in ['start', 'end']:
            if pos_key not in range_obj:
                return False
            
            pos = range_obj[pos_key]
            if not isinstance(pos, dict):
                return False
            
            if 'line' not in pos or 'character' not in pos:
                return False
            
            if not isinstance(pos['line'], int) or not isinstance(pos['character'], int):
                return False
        
        return True
    
    @staticmethod
    def _validate_document_symbol(symbol: Dict[str, Any]) -> bool:
        """Validate document symbol structure."""
        required_fields = ['name', 'kind', 'range', 'selectionRange']
        
        for field in required_fields:
            if field not in symbol:
                return False
        
        # Validate ranges
        for range_key in ['range', 'selectionRange']:
            if not PythonLSPResponse._validate_range(symbol[range_key]):
                return False
        
        # Validate children if present
        if 'children' in symbol:
            if not isinstance(symbol['children'], list):
                return False
            
            for child in symbol['children']:
                if not PythonLSPResponse._validate_document_symbol(child):
                    return False
        
        return True
    
    @staticmethod
    def _validate_range(range_obj: Dict[str, Any]) -> bool:
        """Validate LSP range structure."""
        if not isinstance(range_obj, dict):
            return False
        
        for pos_key in ['start', 'end']:
            if pos_key not in range_obj:
                return False
            
            pos = range_obj[pos_key]
            if not isinstance(pos, dict):
                return False
            
            if 'line' not in pos or 'character' not in pos:
                return False
            
            if not isinstance(pos['line'], int) or not isinstance(pos['character'], int):
                return False
        
        return True


class PythonSymbolAnalyzer:
    """Analyze Python code for LSP symbol information."""
    
    def __init__(self):
        self.symbol_kinds = {
            'class': 5,
            'method': 6,
            'function': 12,
            'variable': 13,
            'constant': 14,
            'property': 7,
            'field': 8,
            'constructor': 9,
            'enum': 10,
            'module': 2,
        }
    
    def extract_symbols_from_code(self, code: str, uri: str) -> List[Dict[str, Any]]:
        """Extract symbols from Python code."""
        symbols = []
        lines = code.split('\n')
        
        for line_num, line in enumerate(lines):
            line_stripped = line.strip()
            
            # Class definitions
            if line_stripped.startswith('class '):
                class_name = self._extract_class_name(line_stripped)
                if class_name:
                    symbols.append(self._create_symbol(
                        name=class_name,
                        kind=self.symbol_kinds['class'],
                        line=line_num,
                        character=line.find('class'),
                        uri=uri
                    ))
            
            # Function/method definitions
            elif line_stripped.startswith('def '):
                func_name = self._extract_function_name(line_stripped)
                if func_name:
                    # Determine if it's a method (indented) or function
                    is_method = line.startswith('    ') or line.startswith('\t')
                    kind = self.symbol_kinds['method'] if is_method else self.symbol_kinds['function']
                    
                    symbols.append(self._create_symbol(
                        name=func_name,
                        kind=kind,
                        line=line_num,
                        character=line.find('def'),
                        uri=uri
                    ))
            
            # Async function definitions
            elif line_stripped.startswith('async def '):
                func_name = self._extract_function_name(line_stripped.replace('async ', ''))
                if func_name:
                    is_method = line.startswith('    ') or line.startswith('\t')
                    kind = self.symbol_kinds['method'] if is_method else self.symbol_kinds['function']
                    
                    symbols.append(self._create_symbol(
                        name=f"async {func_name}",
                        kind=kind,
                        line=line_num,
                        character=line.find('async def'),
                        uri=uri
                    ))
            
            # Variable assignments at module level
            elif '=' in line_stripped and not line.startswith(' ') and not line.startswith('\t'):
                var_name = self._extract_variable_name(line_stripped)
                if var_name and var_name.isupper():  # Constants
                    symbols.append(self._create_symbol(
                        name=var_name,
                        kind=self.symbol_kinds['constant'],
                        line=line_num,
                        character=line.find(var_name),
                        uri=uri
                    ))
                elif var_name:
                    symbols.append(self._create_symbol(
                        name=var_name,
                        kind=self.symbol_kinds['variable'],
                        line=line_num,
                        character=line.find(var_name),
                        uri=uri
                    ))
        
        return symbols
    
    def _extract_class_name(self, line: str) -> Optional[str]:
        """Extract class name from class definition line."""
        import re
        match = re.match(r'class\s+(\w+)', line)
        return match.group(1) if match else None
    
    def _extract_function_name(self, line: str) -> Optional[str]:
        """Extract function name from function definition line."""
        import re
        match = re.match(r'def\s+(\w+)', line)
        return match.group(1) if match else None
    
    def _extract_variable_name(self, line: str) -> Optional[str]:
        """Extract variable name from assignment line."""
        import re
        match = re.match(r'(\w+)\s*=', line)
        return match.group(1) if match else None
    
    def _create_symbol(self, name: str, kind: int, line: int, character: int, uri: str) -> Dict[str, Any]:
        """Create LSP symbol structure."""
        return {
            'name': name,
            'kind': kind,
            'location': {
                'uri': uri,
                'range': {
                    'start': {'line': line, 'character': character},
                    'end': {'line': line, 'character': character + len(name)}
                }
            }
        }


class TestPythonLSPIntegration(unittest.TestCase):
    """Test Python LSP integration functionality."""
    
    def setUp(self):
        """Set up test fixtures."""
        self.response_validator = PythonLSPResponse()
        self.symbol_analyzer = PythonSymbolAnalyzer()
        
        # Sample Python code for testing
        self.sample_python_code = '''"""Sample Python module for testing."""
import os
import sys
from typing import List, Dict, Optional

# Module-level constant
API_VERSION = "1.0.0"
DEBUG_MODE = False

# Module-level variable
default_config = {"timeout": 30, "retries": 3}

class UserService:
    """Service for managing users."""
    
    def __init__(self, config: Dict):
        self.config = config
        self._cache = {}
    
    def get_user(self, user_id: int) -> Optional[Dict]:
        """Get user by ID."""
        if user_id in self._cache:
            return self._cache[user_id]
        
        # Simulate database lookup
        user = {"id": user_id, "name": f"User {user_id}"}
        self._cache[user_id] = user
        return user
    
    async def create_user(self, user_data: Dict) -> Dict:
        """Create a new user asynchronously."""
        # Validate user data
        if not user_data.get("name"):
            raise ValueError("Name is required")
        
        # Simulate async database operation
        user_id = len(self._cache) + 1
        user = {"id": user_id, **user_data}
        self._cache[user_id] = user
        return user
    
    @property
    def user_count(self) -> int:
        """Get total number of users."""
        return len(self._cache)

def validate_email(email: str) -> bool:
    """Validate email address format."""
    import re
    pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$'
    return bool(re.match(pattern, email))

async def process_users(users: List[Dict]) -> List[Dict]:
    """Process multiple users asynchronously."""
    results = []
    for user in users:
        if validate_email(user.get("email", "")):
            results.append(user)
    return results
'''
    
    def test_definition_response_validation(self):
        """Test validation of textDocument/definition responses."""
        
        # Valid single definition response
        valid_single = {
            "uri": "file:///workspace/test.py",
            "range": {
                "start": {"line": 10, "character": 4},
                "end": {"line": 10, "character": 15}
            }
        }
        self.assertTrue(self.response_validator.validate_definition_response(valid_single))
        
        # Valid multiple definitions response
        valid_multiple = [
            {
                "uri": "file:///workspace/test.py",
                "range": {
                    "start": {"line": 10, "character": 4},
                    "end": {"line": 10, "character": 15}
                }
            },
            {
                "uri": "file:///workspace/utils.py",
                "range": {
                    "start": {"line": 5, "character": 0},
                    "end": {"line": 5, "character": 11}
                }
            }
        ]
        self.assertTrue(self.response_validator.validate_definition_response(valid_multiple))
        
        # Invalid response - missing required fields
        invalid_response = {"uri": "file:///test.py"}  # Missing range
        self.assertFalse(self.response_validator.validate_definition_response(invalid_response))
    
    def test_references_response_validation(self):
        """Test validation of textDocument/references responses."""
        
        # Valid references response
        valid_references = [
            {
                "uri": "file:///workspace/test.py",
                "range": {
                    "start": {"line": 15, "character": 8},
                    "end": {"line": 15, "character": 20}
                }
            },
            {
                "uri": "file:///workspace/main.py",  
                "range": {
                    "start": {"line": 25, "character": 12},
                    "end": {"line": 25, "character": 24}
                }
            }
        ]
        self.assertTrue(self.response_validator.validate_references_response(valid_references))
        
        # Empty references (valid)
        empty_references = []
        self.assertTrue(self.response_validator.validate_references_response(empty_references))
        
        # Invalid - not a list
        invalid_references = {"error": "not found"}
        self.assertFalse(self.response_validator.validate_references_response(invalid_references))
    
    def test_hover_response_validation(self):
        """Test validation of textDocument/hover responses."""
        
        # Valid hover with markdown content
        valid_hover_markdown = {
            "contents": {
                "kind": "markdown",
                "value": "**UserService.get_user**\n\nGet user by ID.\n\n**Parameters:**\n- user_id: int"
            },
            "range": {
                "start": {"line": 20, "character": 8},
                "end": {"line": 20, "character": 16}
            }
        }
        self.assertTrue(self.response_validator.validate_hover_response(valid_hover_markdown))
        
        # Valid hover with plain text content
        valid_hover_text = {
            "contents": "def get_user(user_id: int) -> Optional[Dict]"
        }
        self.assertTrue(self.response_validator.validate_hover_response(valid_hover_text))
        
        # Valid hover with list content
        valid_hover_list = {
            "contents": [
                "def get_user(user_id: int) -> Optional[Dict]",
                "Get user by ID from cache or database"
            ]
        }
        self.assertTrue(self.response_validator.validate_hover_response(valid_hover_list))
        
        # Invalid hover - missing contents
        invalid_hover = {"range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 0}}}
        self.assertFalse(self.response_validator.validate_hover_response(invalid_hover))
    
    def test_completion_response_validation(self):
        """Test validation of textDocument/completion responses."""
        
        # Valid completion response
        valid_completion = {
            "items": [
                {
                    "label": "get_user",
                    "kind": 3,
                    "detail": "def get_user(user_id: int) -> Optional[Dict]",
                    "documentation": "Get user by ID from cache or database"
                },
                {
                    "label": "create_user", 
                    "kind": 3,
                    "detail": "async def create_user(user_data: Dict) -> Dict",
                    "documentation": "Create a new user asynchronously"
                },
                {
                    "label": "user_count",
                    "kind": 10,
                    "detail": "property user_count: int",
                    "documentation": "Get total number of users"
                }
            ]
        }
        self.assertTrue(self.response_validator.validate_completion_response(valid_completion))
        
        # Empty completion (valid)
        empty_completion = {"items": []}
        self.assertTrue(self.response_validator.validate_completion_response(empty_completion))
        
        # Invalid completion - missing items
        invalid_completion = {"completions": []}
        self.assertFalse(self.response_validator.validate_completion_response(invalid_completion))
        
        # Invalid completion - items not a list
        invalid_completion_type = {"items": "not a list"}
        self.assertFalse(self.response_validator.validate_completion_response(invalid_completion_type))
    
    def test_document_symbols_response_validation(self):
        """Test validation of textDocument/documentSymbol responses."""
        
        # Valid document symbols response
        valid_symbols = [
            {
                "name": "UserService",
                "kind": 5,  # Class
                "range": {
                    "start": {"line": 10, "character": 0},
                    "end": {"line": 50, "character": 0}
                },
                "selectionRange": {
                    "start": {"line": 10, "character": 6},
                    "end": {"line": 10, "character": 17}
                },
                "children": [
                    {
                        "name": "__init__",
                        "kind": 9,  # Constructor
                        "range": {
                            "start": {"line": 13, "character": 4},
                            "end": {"line": 16, "character": 0}
                        },
                        "selectionRange": {
                            "start": {"line": 13, "character": 8},
                            "end": {"line": 13, "character": 16}
                        }
                    },
                    {
                        "name": "get_user",
                        "kind": 6,  # Method
                        "range": {
                            "start": {"line": 18, "character": 4},
                            "end": {"line": 28, "character": 0}
                        },
                        "selectionRange": {
                            "start": {"line": 18, "character": 8},
                            "end": {"line": 18, "character": 16}
                        }
                    }
                ]
            },
            {
                "name": "validate_email",
                "kind": 12,  # Function
                "range": {
                    "start": {"line": 52, "character": 0},
                    "end": {"line": 56, "character": 0}
                },
                "selectionRange": {
                    "start": {"line": 52, "character": 4},
                    "end": {"line": 52, "character": 18}
                }
            }
        ]
        self.assertTrue(self.response_validator.validate_document_symbols_response(valid_symbols))
        
        # Empty symbols (valid)
        empty_symbols = []
        self.assertTrue(self.response_validator.validate_document_symbols_response(empty_symbols))
        
        # Invalid symbols - not a list
        invalid_symbols = {"symbols": []}
        self.assertFalse(self.response_validator.validate_document_symbols_response(invalid_symbols))
    
    def test_workspace_symbols_response_validation(self):
        """Test validation of workspace/symbol responses."""
        
        # Valid workspace symbols response
        valid_workspace_symbols = [
            {
                "name": "UserService",
                "kind": 5,
                "location": {
                    "uri": "file:///workspace/services/user.py",
                    "range": {
                        "start": {"line": 10, "character": 6},
                        "end": {"line": 10, "character": 17}
                    }
                }
            },
            {
                "name": "validate_email",
                "kind": 12,
                "location": {
                    "uri": "file:///workspace/utils/validation.py",
                    "range": {
                        "start": {"line": 5, "character": 4},
                        "end": {"line": 5, "character": 18}
                    }
                }
            }
        ]
        self.assertTrue(self.response_validator.validate_workspace_symbols_response(valid_workspace_symbols))
        
        # Empty workspace symbols (valid)
        empty_workspace_symbols = []
        self.assertTrue(self.response_validator.validate_workspace_symbols_response(empty_workspace_symbols))
        
        # Invalid workspace symbols - missing location
        invalid_workspace_symbols = [
            {
                "name": "UserService",
                "kind": 5
                # Missing location
            }
        ]
        self.assertFalse(self.response_validator.validate_workspace_symbols_response(invalid_workspace_symbols))
    
    def test_python_symbol_extraction(self):
        """Test Python symbol extraction from code."""
        symbols = self.symbol_analyzer.extract_symbols_from_code(
            self.sample_python_code,
            "file:///workspace/test.py"
        )
        
        # Check that we found the expected symbols
        symbol_names = [symbol['name'] for symbol in symbols]
        
        # Constants
        self.assertIn('API_VERSION', symbol_names)
        self.assertIn('DEBUG_MODE', symbol_names)
        
        # Variables
        self.assertIn('default_config', symbol_names)
        
        # Classes
        self.assertIn('UserService', symbol_names)
        
        # Functions
        self.assertIn('validate_email', symbol_names)
        self.assertIn('async process_users', symbol_names)
        
        # Methods (should be detected as methods based on indentation)
        method_symbols = [s for s in symbols if s['kind'] == 6]  # Method kind
        method_names = [s['name'] for s in method_symbols]
        self.assertIn('get_user', method_names)
        self.assertIn('async create_user', method_names)
        
        # Verify symbol structure
        class_symbol = next((s for s in symbols if s['name'] == 'UserService'), None)
        self.assertIsNotNone(class_symbol)
        self.assertEqual(class_symbol['kind'], 5)  # Class kind
        self.assertIn('location', class_symbol)
        self.assertIn('uri', class_symbol['location'])
        self.assertIn('range', class_symbol['location'])
    
    def test_python_type_hint_analysis(self):
        """Test analysis of Python type hints for LSP completion."""
        
        def extract_type_hints(code: str) -> Dict[str, str]:
            """Extract type hints from Python code."""
            import re
            type_hints = {}
            
            # Function signatures with type hints
            func_pattern = r'def\s+(\w+)\s*\([^)]*\)\s*->\s*([^:]+):'
            for match in re.finditer(func_pattern, code):
                func_name, return_type = match.groups()
                type_hints[func_name] = return_type.strip()
            
            # Variable type annotations
            var_pattern = r'(\w+)\s*:\s*([^=\n]+)(?:\s*=|$)'
            for match in re.finditer(var_pattern, code):
                var_name, var_type = match.groups()
                type_hints[var_name] = var_type.strip()
            
            return type_hints
        
        type_hints = extract_type_hints(self.sample_python_code)
        
        # Check extracted type hints
        self.assertIn('get_user', type_hints)
        self.assertIn('Optional[Dict]', type_hints['get_user'])
        
        self.assertIn('create_user', type_hints)
        self.assertIn('Dict', type_hints['create_user'])
        
        self.assertIn('validate_email', type_hints)
        self.assertIn('bool', type_hints['validate_email'])
        
        self.assertIn('process_users', type_hints)
        self.assertIn('List[Dict]', type_hints['process_users'])
    
    def test_python_import_analysis(self):
        """Test analysis of Python imports for LSP functionality."""
        
        def extract_imports(code: str) -> Dict[str, List[str]]:
            """Extract import information from Python code."""
            import re
            imports = {
                'standard_library': [],
                'third_party': [],
                'relative': [],
                'from_imports': {}
            }
            
            lines = code.split('\n')
            for line in lines:
                line = line.strip()
                
                # Standard imports
                import_match = re.match(r'import\s+(.+)', line)
                if import_match:
                    modules = [m.strip() for m in import_match.group(1).split(',')]
                    for module in modules:
                        if module in ['os', 'sys', 're', 'json', 'datetime']:
                            imports['standard_library'].append(module)
                        else:
                            imports['third_party'].append(module)
                
                # From imports
                from_match = re.match(r'from\s+(.+?)\s+import\s+(.+)', line)
                if from_match:
                    module, items = from_match.groups()
                    items_list = [item.strip() for item in items.split(',')]
                    
                    if module.startswith('.'):
                        imports['relative'].append(module)
                    
                    imports['from_imports'][module] = items_list
            
            return imports
        
        imports = extract_imports(self.sample_python_code)
        
        # Check standard library imports
        self.assertIn('os', imports['standard_library'])
        self.assertIn('sys', imports['standard_library'])
        
        # Check from imports
        self.assertIn('typing', imports['from_imports'])
        typing_imports = imports['from_imports']['typing']
        self.assertIn('List', typing_imports)
        self.assertIn('Dict', typing_imports)
        self.assertIn('Optional', typing_imports)
    
    def test_python_error_detection(self):
        """Test Python syntax error detection for LSP diagnostics."""
        
        def detect_syntax_errors(code: str) -> List[Dict[str, Any]]:
            """Detect syntax errors in Python code."""
            import ast
            import sys
            from io import StringIO
            
            errors = []
            
            try:
                ast.parse(code)
            except SyntaxError as e:
                errors.append({
                    'line': e.lineno - 1 if e.lineno else 0,
                    'character': e.offset - 1 if e.offset else 0,
                    'message': str(e),
                    'severity': 1,  # Error severity
                    'source': 'python'
                })
            except Exception as e:
                errors.append({
                    'line': 0,
                    'character': 0,
                    'message': f"Parse error: {str(e)}",
                    'severity': 1,
                    'source': 'python'
                })
            
            return errors
        
        # Valid code should have no errors
        errors = detect_syntax_errors(self.sample_python_code)
        self.assertEqual(len(errors), 0)
        
        # Invalid code should detect errors
        invalid_code = '''
def invalid_function(
    # Missing closing parenthesis and colon
    pass
'''
        errors = detect_syntax_errors(invalid_code)
        self.assertGreater(len(errors), 0)
        self.assertEqual(errors[0]['severity'], 1)  # Error severity
        self.assertIn('source', errors[0])


class TestPythonLSPRequests(unittest.TestCase):
    """Test Python LSP request generation and handling."""
    
    def test_definition_request_generation(self):
        """Test generation of textDocument/definition requests."""
        
        def create_definition_request(uri: str, line: int, character: int) -> Dict[str, Any]:
            """Create LSP definition request."""
            return {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "textDocument/definition",
                "params": {
                    "textDocument": {"uri": uri},
                    "position": {"line": line, "character": character}
                }
            }
        
        request = create_definition_request("file:///workspace/test.py", 25, 10)
        
        self.assertEqual(request["method"], "textDocument/definition")
        self.assertEqual(request["params"]["textDocument"]["uri"], "file:///workspace/test.py")
        self.assertEqual(request["params"]["position"]["line"], 25)
        self.assertEqual(request["params"]["position"]["character"], 10)
    
    def test_completion_request_generation(self):
        """Test generation of textDocument/completion requests."""
        
        def create_completion_request(uri: str, line: int, character: int, trigger_character: Optional[str] = None) -> Dict[str, Any]:
            """Create LSP completion request."""
            params = {
                "textDocument": {"uri": uri},
                "position": {"line": line, "character": character}
            }
            
            if trigger_character:
                params["context"] = {
                    "triggerKind": 2,  # TriggerCharacter
                    "triggerCharacter": trigger_character
                }
            else:
                params["context"] = {"triggerKind": 1}  # Invoked
            
            return {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "textDocument/completion",
                "params": params
            }
        
        # Normal completion request
        request = create_completion_request("file:///workspace/test.py", 30, 15)
        self.assertEqual(request["method"], "textDocument/completion")
        self.assertEqual(request["params"]["context"]["triggerKind"], 1)
        
        # Completion with trigger character
        dot_request = create_completion_request("file:///workspace/test.py", 30, 15, ".")
        self.assertEqual(dot_request["params"]["context"]["triggerKind"], 2)
        self.assertEqual(dot_request["params"]["context"]["triggerCharacter"], ".")
    
    def test_hover_request_generation(self):
        """Test generation of textDocument/hover requests."""
        
        def create_hover_request(uri: str, line: int, character: int) -> Dict[str, Any]:
            """Create LSP hover request."""
            return {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "textDocument/hover",
                "params": {
                    "textDocument": {"uri": uri},
                    "position": {"line": line, "character": character}
                }
            }
        
        request = create_hover_request("file:///workspace/test.py", 20, 8)
        
        self.assertEqual(request["method"], "textDocument/hover")
        self.assertIn("textDocument", request["params"])
        self.assertIn("position", request["params"])
    
    def test_workspace_symbol_request_generation(self):
        """Test generation of workspace/symbol requests."""
        
        def create_workspace_symbol_request(query: str) -> Dict[str, Any]:
            """Create LSP workspace symbol request."""
            return {
                "jsonrpc": "2.0",
                "id": 4,
                "method": "workspace/symbol",
                "params": {"query": query}
            }
        
        request = create_workspace_symbol_request("UserService")
        
        self.assertEqual(request["method"], "workspace/symbol")
        self.assertEqual(request["params"]["query"], "UserService")
        
        # Empty query should also be valid
        empty_request = create_workspace_symbol_request("")
        self.assertEqual(empty_request["params"]["query"], "")


if __name__ == '__main__':
    # Run all tests
    unittest.main(verbosity=2)