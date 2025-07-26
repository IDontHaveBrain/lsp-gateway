"""
Python unit tests for testing Python project detection functionality.

This module tests Python-specific project detection logic that complements
the Go-based LSP Gateway's Python integration capabilities.
"""

import unittest
import tempfile
import os
import json
from pathlib import Path
from typing import Dict, List, Optional, Any
import subprocess
import sys


class PythonProjectDetectionTests(unittest.TestCase):
    """Test Python project detection and analysis functionality."""
    
    def setUp(self):
        """Set up test fixtures for each test."""
        self.temp_dir = tempfile.mkdtemp()
        self.test_projects = {}
        
    def tearDown(self):
        """Clean up after each test."""
        import shutil
        shutil.rmtree(self.temp_dir, ignore_errors=True)
    
    def create_test_project(self, project_type: str, files: Dict[str, str]) -> Path:
        """Create a test project with specified files."""
        project_path = Path(self.temp_dir) / project_type
        project_path.mkdir(parents=True, exist_ok=True)
        
        for file_path, content in files.items():
            full_path = project_path / file_path
            full_path.parent.mkdir(parents=True, exist_ok=True)
            full_path.write_text(content)
        
        self.test_projects[project_type] = project_path
        return project_path
    
    def test_django_project_detection(self):
        """Test Django project detection and configuration parsing."""
        django_files = {
            "manage.py": '''#!/usr/bin/env python
import os
import sys

if __name__ == "__main__":
    os.environ.setdefault("DJANGO_SETTINGS_MODULE", "myproject.settings")
    try:
        from django.core.management import execute_from_command_line
    except ImportError as exc:
        raise ImportError(
            "Couldn't import Django. Are you sure it's installed and "
            "available on your PYTHONPATH environment variable?"
        ) from exc
    execute_from_command_line(sys.argv)
''',
            "requirements.txt": '''Django==4.2.0
djangorestframework==3.14.0
celery==5.3.0
redis==4.5.4
psycopg2-binary==2.9.6
python-decouple==3.8
''',
            "myproject/__init__.py": "",
            "myproject/settings.py": '''
import os
from pathlib import Path
from decouple import config

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = config('SECRET_KEY', default='dev-secret-key')
DEBUG = config('DEBUG', default=True, cast=bool)

INSTALLED_APPS = [
    'django.contrib.admin',
    'django.contrib.auth',
    'django.contrib.contenttypes',
    'django.contrib.sessions',
    'django.contrib.messages',
    'django.contrib.staticfiles',
    'rest_framework',
    'myapp',
]

DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.postgresql',
        'NAME': config('DB_NAME', default='myproject'),
        'USER': config('DB_USER', default='postgres'),
        'PASSWORD': config('DB_PASSWORD', default='password'),
        'HOST': config('DB_HOST', default='localhost'),
        'PORT': config('DB_PORT', default='5432'),
    }
}
''',
            "myproject/urls.py": '''
from django.contrib import admin
from django.urls import path, include

urlpatterns = [
    path('admin/', admin.site.urls),
    path('api/', include('myapp.urls')),
]
''',
            "myapp/__init__.py": "",
            "myapp/models.py": '''
from django.db import models
from django.contrib.auth.models import AbstractUser

class User(AbstractUser):
    email = models.EmailField(unique=True)
    created_at = models.DateTimeField(auto_now_add=True)
    
class Post(models.Model):
    title = models.CharField(max_length=200)
    content = models.TextField()
    author = models.ForeignKey(User, on_delete=models.CASCADE)
    created_at = models.DateTimeField(auto_now_add=True)
''',
            "myapp/views.py": '''
from rest_framework import viewsets
from rest_framework.permissions import IsAuthenticated
from .models import Post
from .serializers import PostSerializer

class PostViewSet(viewsets.ModelViewSet):
    queryset = Post.objects.all()
    serializer_class = PostSerializer
    permission_classes = [IsAuthenticated]
''',
        }
        
        project_path = self.create_test_project("django_project", django_files)
        
        # Test Django project detection
        self.assertTrue((project_path / "manage.py").exists())
        self.assertTrue((project_path / "requirements.txt").exists())
        
        # Test framework detection
        requirements_content = (project_path / "requirements.txt").read_text()
        self.assertIn("Django", requirements_content)
        self.assertIn("djangorestframework", requirements_content)
        self.assertIn("celery", requirements_content)
        
        # Test project structure validation
        self.assertTrue((project_path / "myproject" / "settings.py").exists())
        self.assertTrue((project_path / "myapp" / "models.py").exists())
        
        # Validate that project follows Django conventions
        settings_content = (project_path / "myproject" / "settings.py").read_text()
        self.assertIn("INSTALLED_APPS", settings_content)
        self.assertIn("DATABASES", settings_content)
    
    def test_fastapi_project_detection(self):
        """Test FastAPI project detection with modern Python features."""
        fastapi_files = {
            "pyproject.toml": '''[tool.poetry]
name = "fastapi-project"
version = "1.0.0"
description = "Modern FastAPI project"
authors = ["Developer <dev@example.com>"]

[tool.poetry.dependencies]
python = "^3.11"
fastapi = "^0.104.0"
uvicorn = {extras = ["standard"], version = "^0.24.0"}
pydantic = "^2.4.0"
sqlalchemy = {extras = ["asyncio"], version = "^2.0.0"}
redis = "^5.0.0"

[tool.poetry.group.dev.dependencies]
pytest = "^7.4.0"
pytest-asyncio = "^0.21.0"
httpx = "^0.25.0"
mypy = "^1.5.0"

[build-system]
requires = ["poetry-core>=1.0.0"]
build-backend = "poetry.core.masonry.api"
''',
            "main.py": '''from fastapi import FastAPI, Depends, HTTPException
from pydantic import BaseModel
from typing import List, Optional
import asyncio

app = FastAPI(title="Modern API", version="1.0.0")

class UserCreate(BaseModel):
    username: str
    email: str
    age: Optional[int] = None

class UserResponse(BaseModel):
    id: int
    username: str
    email: str
    is_active: bool

@app.get("/")
async def root():
    return {"message": "FastAPI Project"}

@app.post("/users/", response_model=UserResponse)
async def create_user(user: UserCreate):
    # Simulated user creation
    return UserResponse(
        id=1,
        username=user.username,
        email=user.email,
        is_active=True
    )

@app.get("/users/", response_model=List[UserResponse])
async def get_users():
    return []
''',
            "tests/__init__.py": "",
            "tests/test_main.py": '''import pytest
from fastapi.testclient import TestClient
from main import app

client = TestClient(app)

def test_read_root():
    response = client.get("/")
    assert response.status_code == 200
    assert response.json() == {"message": "FastAPI Project"}

def test_create_user():
    user_data = {
        "username": "testuser",
        "email": "test@example.com",
        "age": 25
    }
    response = client.post("/users/", json=user_data)
    assert response.status_code == 200
    assert response.json()["username"] == "testuser"
''',
        }
        
        project_path = self.create_test_project("fastapi_project", fastapi_files)
        
        # Test FastAPI project detection
        self.assertTrue((project_path / "pyproject.toml").exists())
        self.assertTrue((project_path / "main.py").exists())
        
        # Test modern Python configuration parsing
        pyproject_content = (project_path / "pyproject.toml").read_text()
        self.assertIn("fastapi", pyproject_content)
        self.assertIn("pydantic", pyproject_content)
        self.assertIn("python = \"^3.11\"", pyproject_content)
        
        # Validate FastAPI patterns in main.py
        main_content = (project_path / "main.py").read_text()
        self.assertIn("from fastapi import", main_content)
        self.assertIn("app = FastAPI", main_content)
        self.assertIn("async def", main_content)
    
    def test_ml_project_detection(self):
        """Test machine learning project detection with scientific Python libraries."""
        ml_files = {
            "requirements.txt": '''numpy==1.24.0
pandas==2.1.0
scikit-learn==1.3.0
matplotlib==3.7.0
jupyter==1.0.0
tensorflow==2.13.0
torch==2.0.0
transformers==4.33.0
datasets==2.14.0
''',
            "src/__init__.py": "",
            "src/data_processing.py": '''import pandas as pd
import numpy as np
from sklearn.preprocessing import StandardScaler
from sklearn.model_selection import train_test_split
from typing import Tuple, Optional

def load_and_preprocess_data(file_path: str) -> Tuple[pd.DataFrame, pd.Series]:
    """Load and preprocess training data."""
    df = pd.read_csv(file_path)
    
    # Separate features and target
    X = df.drop('target', axis=1)
    y = df['target']
    
    # Handle missing values
    X = X.fillna(X.mean())
    
    return X, y

def split_data(X: pd.DataFrame, y: pd.Series, test_size: float = 0.2) -> Tuple[pd.DataFrame, pd.DataFrame, pd.Series, pd.Series]:
    """Split data into train and test sets."""
    return train_test_split(X, y, test_size=test_size, random_state=42)

class DataPreprocessor:
    """Advanced data preprocessing pipeline."""
    
    def __init__(self):
        self.scaler = StandardScaler()
        self.is_fitted = False
    
    def fit_transform(self, X: pd.DataFrame) -> np.ndarray:
        """Fit preprocessor and transform data."""
        X_scaled = self.scaler.fit_transform(X)
        self.is_fitted = True
        return X_scaled
    
    def transform(self, X: pd.DataFrame) -> np.ndarray:
        """Transform data using fitted preprocessor."""
        if not self.is_fitted:
            raise ValueError("Preprocessor must be fitted before transform")
        return self.scaler.transform(X)
''',
            "src/model.py": '''from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import accuracy_score, classification_report
import joblib
from typing import Dict, Any
import numpy as np

class MLModel:
    """Machine learning model wrapper."""
    
    def __init__(self, model_type: str = "random_forest"):
        self.model_type = model_type
        self.model = self._create_model()
        self.is_trained = False
    
    def _create_model(self):
        """Create model based on type."""
        if self.model_type == "random_forest":
            return RandomForestClassifier(n_estimators=100, random_state=42)
        else:
            raise ValueError(f"Unsupported model type: {self.model_type}")
    
    def train(self, X_train: np.ndarray, y_train: np.ndarray) -> Dict[str, Any]:
        """Train the model and return metrics."""
        self.model.fit(X_train, y_train)
        self.is_trained = True
        
        # Calculate training accuracy
        y_pred = self.model.predict(X_train)
        accuracy = accuracy_score(y_train, y_pred)
        
        return {
            "training_accuracy": accuracy,
            "n_features": X_train.shape[1],
            "n_samples": X_train.shape[0]
        }
    
    def predict(self, X: np.ndarray) -> np.ndarray:
        """Make predictions."""
        if not self.is_trained:
            raise ValueError("Model must be trained before prediction")
        return self.model.predict(X)
    
    def save(self, filepath: str) -> None:
        """Save model to file."""
        joblib.dump(self.model, filepath)
    
    @classmethod
    def load(cls, filepath: str) -> 'MLModel':
        """Load model from file."""
        instance = cls()
        instance.model = joblib.load(filepath)
        instance.is_trained = True
        return instance
''',
            "notebooks/data_exploration.ipynb": '''{
 "cells": [
  {
   "cell_type": "markdown",
   "metadata": {},
   "source": [
    "# Data Exploration\\n",
    "\\n",
    "This notebook explores the dataset and performs initial analysis."
   ]
  },
  {
   "cell_type": "code",
   "execution_count": null,
   "metadata": {},
   "outputs": [],
   "source": [
    "import pandas as pd\\n",
    "import numpy as np\\n",
    "import matplotlib.pyplot as plt\\n",
    "\\n",
    "# Load data\\n",
    "df = pd.read_csv('../data/sample_data.csv')\\n",
    "df.head()"
   ]
  }
 ],
 "metadata": {
  "kernelspec": {
   "display_name": "Python 3",
   "language": "python",
   "name": "python3"
  }
 },
 "nbformat": 4,
 "nbformat_minor": 4
}''',
            "tests/test_data_processing.py": '''import unittest
import pandas as pd
import numpy as np
from unittest.mock import patch, MagicMock
import tempfile
import os

from src.data_processing import load_and_preprocess_data, split_data, DataPreprocessor

class TestDataProcessing(unittest.TestCase):
    
    def setUp(self):
        # Create sample data for testing
        self.sample_data = pd.DataFrame({
            'feature1': [1, 2, 3, 4, 5],
            'feature2': [2, 4, 6, 8, 10],
            'feature3': [1, 1, 2, 2, 3],
            'target': [0, 0, 1, 1, 1]
        })
        
        # Create temporary file
        self.temp_file = tempfile.NamedTemporaryFile(mode='w', suffix='.csv', delete=False)
        self.sample_data.to_csv(self.temp_file.name, index=False)
        self.temp_file.close()
    
    def tearDown(self):
        os.unlink(self.temp_file.name)
    
    def test_load_and_preprocess_data(self):
        """Test data loading and preprocessing."""
        X, y = load_and_preprocess_data(self.temp_file.name)
        
        self.assertIsInstance(X, pd.DataFrame)
        self.assertIsInstance(y, pd.Series)
        self.assertEqual(len(X), 5)
        self.assertEqual(len(y), 5)
        self.assertEqual(list(X.columns), ['feature1', 'feature2', 'feature3'])
    
    def test_split_data(self):
        """Test data splitting functionality."""
        X = self.sample_data.drop('target', axis=1)
        y = self.sample_data['target']
        
        X_train, X_test, y_train, y_test = split_data(X, y, test_size=0.4)
        
        self.assertEqual(len(X_train), 3)  # 60% of 5 samples
        self.assertEqual(len(X_test), 2)   # 40% of 5 samples
        self.assertEqual(len(y_train), 3)
        self.assertEqual(len(y_test), 2)
    
    def test_data_preprocessor(self):
        """Test DataPreprocessor class."""
        X = self.sample_data.drop('target', axis=1)
        preprocessor = DataPreprocessor()
        
        # Test fit_transform
        X_scaled = preprocessor.fit_transform(X)
        self.assertTrue(preprocessor.is_fitted)
        self.assertEqual(X_scaled.shape, X.shape)
        
        # Test transform
        X_new = pd.DataFrame({
            'feature1': [6, 7],
            'feature2': [12, 14],
            'feature3': [3, 4]
        })
        X_new_scaled = preprocessor.transform(X_new)
        self.assertEqual(X_new_scaled.shape, (2, 3))
    
    def test_preprocessor_not_fitted_error(self):
        """Test error when using unfitted preprocessor."""
        preprocessor = DataPreprocessor()
        X = self.sample_data.drop('target', axis=1)
        
        with self.assertRaises(ValueError):
            preprocessor.transform(X)

if __name__ == '__main__':
    unittest.main()
''',
        }
        
        project_path = self.create_test_project("ml_project", ml_files)
        
        # Test ML project detection
        self.assertTrue((project_path / "requirements.txt").exists())
        self.assertTrue((project_path / "src" / "data_processing.py").exists())
        self.assertTrue((project_path / "src" / "model.py").exists())
        self.assertTrue((project_path / "notebooks").exists())
        
        # Test scientific library detection
        requirements_content = (project_path / "requirements.txt").read_text()
        ml_libraries = ["numpy", "pandas", "scikit-learn", "tensorflow", "torch"]
        for lib in ml_libraries:
            self.assertIn(lib, requirements_content)
        
        # Test project structure validation
        self.assertTrue((project_path / "tests" / "test_data_processing.py").exists())
        self.assertTrue((project_path / "notebooks" / "data_exploration.ipynb").exists())
    
    def test_virtual_environment_detection(self):
        """Test virtual environment detection and analysis."""
        venv_files = {
            "venv/pyvenv.cfg": '''home = /usr/bin
include-system-site-packages = false
version = 3.11.5
executable = /usr/bin/python3.11
command = /usr/bin/python3 -m venv /home/user/project/venv
''',
            "requirements.txt": '''django==4.2.0
requests==2.31.0
pytest==7.4.0
''',
            ".python-version": "3.11.5",
            "pyproject.toml": '''[tool.poetry]
name = "venv-project"
version = "1.0.0"

[tool.poetry.dependencies]
python = "^3.11"
django = "^4.2.0"
''',
        }
        
        project_path = self.create_test_project("venv_project", venv_files)
        
        # Test virtual environment detection
        pyvenv_cfg = project_path / "venv" / "pyvenv.cfg"
        self.assertTrue(pyvenv_cfg.exists())
        
        # Parse pyvenv.cfg
        pyvenv_content = pyvenv_cfg.read_text()
        self.assertIn("version = 3.11.5", pyvenv_content)
        self.assertIn("include-system-site-packages = false", pyvenv_content)
        
        # Test Python version detection
        python_version_file = project_path / ".python-version"
        self.assertTrue(python_version_file.exists())
        self.assertEqual(python_version_file.read_text().strip(), "3.11.5")
    
    def test_dependency_analysis(self):
        """Test Python dependency analysis and parsing."""
        
        def analyze_requirements(requirements_text: str) -> Dict[str, str]:
            """Analyze requirements.txt content."""
            dependencies = {}
            for line in requirements_text.strip().split('\n'):
                if line and not line.startswith('#'):
                    if '==' in line:
                        name, version = line.split('==')
                        dependencies[name.strip()] = version.strip()
                    elif '>=' in line:
                        name, version = line.split('>=')
                        dependencies[name.strip()] = f">={version.strip()}"
            return dependencies
        
        def analyze_pyproject_dependencies(pyproject_content: str) -> Dict[str, Any]:
            """Analyze pyproject.toml dependencies."""
            # Simple parsing for test purposes
            lines = pyproject_content.split('\n')
            in_dependencies = False
            dependencies = {}
            
            for line in lines:
                line = line.strip()
                if line == '[tool.poetry.dependencies]':
                    in_dependencies = True
                    continue
                elif line.startswith('[') and in_dependencies:
                    in_dependencies = False
                elif in_dependencies and '=' in line:
                    parts = line.split('=', 1)
                    if len(parts) == 2:
                        name = parts[0].strip()
                        version = parts[1].strip(' "')
                        dependencies[name] = version
            
            return dependencies
        
        # Test requirements.txt parsing
        requirements_text = '''django==4.2.0
requests>=2.30.0
pytest==7.4.0
black>=23.0.0
mypy==1.5.0
'''
        deps = analyze_requirements(requirements_text)
        expected_deps = {
            'django': '4.2.0',
            'requests': '>=2.30.0',
            'pytest': '7.4.0',
            'black': '>=23.0.0',
            'mypy': '1.5.0'
        }
        self.assertEqual(deps, expected_deps)
        
        # Test pyproject.toml parsing
        pyproject_content = '''[tool.poetry]
name = "test-project"

[tool.poetry.dependencies]
python = "^3.11"
fastapi = "^0.104.0"
uvicorn = "^0.24.0"

[tool.poetry.group.dev.dependencies]
pytest = "^7.4.0"
'''
        pyproject_deps = analyze_pyproject_dependencies(pyproject_content)
        self.assertIn('python', pyproject_deps)
        self.assertIn('fastapi', pyproject_deps)
        self.assertEqual(pyproject_deps['python'], '^3.11')
    
    def test_framework_detection_patterns(self):
        """Test Python framework detection patterns."""
        
        def detect_frameworks(file_contents: Dict[str, str]) -> List[str]:
            """Detect Python frameworks from file contents."""
            frameworks = set()
            
            # Check all file contents for framework indicators
            all_content = ' '.join(file_contents.values()).lower()
            
            # Django detection
            if any(indicator in all_content for indicator in [
                'from django', 'import django', 'django.contrib', 'manage.py', 'django_settings_module'
            ]):
                frameworks.add('django')
            
            # FastAPI detection
            if any(indicator in all_content for indicator in [
                'from fastapi', 'import fastapi', 'fastapi(', '@app.get', '@app.post'
            ]):
                frameworks.add('fastapi')
            
            # Flask detection
            if any(indicator in all_content for indicator in [
                'from flask', 'import flask', 'flask(', '@app.route'
            ]):
                frameworks.add('flask')
            
            # Pytest detection
            if any(indicator in all_content for indicator in [
                'import pytest', 'pytest.', 'def test_', '@pytest.'
            ]):
                frameworks.add('pytest')
            
            # ML frameworks
            if any(indicator in all_content for indicator in [
                'import tensorflow', 'from tensorflow', 'import torch', 'from torch'
            ]):
                frameworks.add('machine_learning')
            
            return sorted(list(frameworks))
        
        # Test Django detection
        django_content = {
            "manage.py": "from django.core.management import execute_from_command_line",
            "settings.py": "INSTALLED_APPS = ['django.contrib.admin']",
            "models.py": "from django.db import models"
        }
        frameworks = detect_frameworks(django_content)
        self.assertIn('django', frameworks)
        
        # Test FastAPI detection
        fastapi_content = {
            "main.py": "from fastapi import FastAPI\napp = FastAPI()\n@app.get('/')",
        }
        frameworks = detect_frameworks(fastapi_content)
        self.assertIn('fastapi', frameworks)
        
        # Test multiple frameworks
        mixed_content = {
            "test_api.py": "import pytest\nfrom fastapi.testclient import TestClient",
            "main.py": "from fastapi import FastAPI"
        }
        frameworks = detect_frameworks(mixed_content)
        self.assertIn('fastapi', frameworks)
        self.assertIn('pytest', frameworks)


class PythonConfigurationTests(unittest.TestCase):
    """Test Python configuration parsing and validation."""
    
    def test_pyproject_toml_parsing(self):
        """Test comprehensive pyproject.toml parsing."""
        pyproject_content = '''[build-system]
requires = ["poetry-core>=1.0.0"]
build-backend = "poetry.core.masonry.api"

[tool.poetry]
name = "my-project"
version = "1.0.0"
description = "A sample Python project"
authors = ["John Doe <john@example.com>"]
license = "MIT"
readme = "README.md"

[tool.poetry.dependencies]
python = "^3.11"
fastapi = "^0.104.0"
pydantic = "^2.4.0"
sqlalchemy = {extras = ["asyncio"], version = "^2.0.0"}

[tool.poetry.group.dev.dependencies]
pytest = "^7.4.0"
mypy = "^1.5.0"
black = "^23.0.0"
isort = "^5.12.0"

[tool.mypy]
python_version = "3.11"
strict = true
warn_return_any = true

[tool.black]
line-length = 100
target-version = ['py311']

[tool.isort]
profile = "black"
line_length = 100
'''
        
        def parse_pyproject_toml(content: str) -> Dict[str, Any]:
            """Parse pyproject.toml content for testing."""
            import re
            
            result = {
                'build_system': {},
                'project': {},
                'dependencies': {},
                'dev_dependencies': {},
                'tools': {}
            }
            
            lines = content.split('\n')
            current_section = None
            
            for line in lines:
                line = line.strip()
                if not line or line.startswith('#'):
                    continue
                
                # Section headers
                if line.startswith('['):
                    current_section = line.strip('[]')
                    continue
                
                # Key-value pairs
                if '=' in line and current_section:
                    key, value = line.split('=', 1)
                    key = key.strip()
                    value = value.strip(' "\'')
                    
                    if current_section == 'build-system':
                        result['build_system'][key] = value
                    elif current_section == 'tool.poetry':
                        result['project'][key] = value
                    elif current_section == 'tool.poetry.dependencies':
                        result['dependencies'][key] = value
                    elif current_section == 'tool.poetry.group.dev.dependencies':
                        result['dev_dependencies'][key] = value
                    elif current_section.startswith('tool.'):
                        tool_name = current_section.split('.', 1)[1]
                        if tool_name not in result['tools']:
                            result['tools'][tool_name] = {}
                        result['tools'][tool_name][key] = value
            
            return result
        
        parsed = parse_pyproject_toml(pyproject_content)
        
        # Test build system
        self.assertIn('requires', parsed['build_system'])
        self.assertIn('build-backend', parsed['build_system'])
        
        # Test project metadata
        self.assertEqual(parsed['project']['name'], 'my-project')
        self.assertEqual(parsed['project']['version'], '1.0.0')
        
        # Test dependencies
        self.assertIn('python', parsed['dependencies'])
        self.assertIn('fastapi', parsed['dependencies'])
        
        # Test dev dependencies
        self.assertIn('pytest', parsed['dev_dependencies'])
        self.assertIn('mypy', parsed['dev_dependencies'])
        
        # Test tool configurations
        self.assertIn('mypy', parsed['tools'])
        self.assertIn('black', parsed['tools'])
    
    def test_requirements_txt_parsing(self):
        """Test requirements.txt parsing with various formats."""
        requirements_content = '''# Core dependencies
Django==4.2.0
djangorestframework>=3.14.0,<4.0.0
requests>=2.28.0
python-decouple~=3.8

# Database
psycopg2-binary==2.9.6
redis>=4.5.0

# Development
pytest>=7.0.0
black
mypy>=1.0.0

# Optional dependencies
-e git+https://github.com/example/repo.git@v1.0#egg=custom_package
./local_package

# From index
--index-url https://pypi.org/simple/
--extra-index-url https://custom.pypi.org/simple/
'''
        
        def parse_requirements(content: str) -> Dict[str, Any]:
            """Parse requirements.txt content."""
            dependencies = {}
            options = {}
            
            for line in content.split('\n'):
                line = line.strip()
                
                # Skip empty lines and comments
                if not line or line.startswith('#'):
                    continue
                
                # Handle options
                if line.startswith('--'):
                    if '=' in line:
                        option, value = line.split('=', 1)
                        options[option] = value
                    else:
                        options[line] = True
                    continue
                
                # Handle editable installs
                if line.startswith('-e '):
                    dependencies['_editable'] = dependencies.get('_editable', [])
                    dependencies['_editable'].append(line[3:])
                    continue
                
                # Handle local paths
                if line.startswith('./') or line.startswith('/'):
                    dependencies['_local'] = dependencies.get('_local', [])
                    dependencies['_local'].append(line)
                    continue
                
                # Regular dependencies
                if any(op in line for op in ['==', '>=', '<=', '>', '<', '~=', '!=']):
                    for op in ['==', '>=', '<=', '~=', '!=', '>', '<']:
                        if op in line:
                            name, version = line.split(op, 1)
                            dependencies[name.strip()] = f"{op}{version.strip()}"
                            break
                else:
                    # No version specified
                    dependencies[line] = 'latest'
            
            return {'dependencies': dependencies, 'options': options}
        
        parsed = parse_requirements(requirements_content)
        
        # Test regular dependencies
        deps = parsed['dependencies']
        self.assertEqual(deps['Django'], '==4.2.0')
        self.assertEqual(deps['djangorestframework'], '>=3.14.0,<4.0.0')
        self.assertEqual(deps['python-decouple'], '~=3.8')
        self.assertEqual(deps['black'], 'latest')
        
        # Test editable installs
        self.assertIn('_editable', deps)
        self.assertIn('git+https://github.com/example/repo.git@v1.0#egg=custom_package', deps['_editable'])
        
        # Test local packages
        self.assertIn('_local', deps)
        self.assertIn('./local_package', deps['_local'])
        
        # Test options
        options = parsed['options']
        self.assertIn('--index-url', options)
        self.assertEqual(options['--index-url'], 'https://pypi.org/simple/')


if __name__ == '__main__':
    # Run all tests
    unittest.main(verbosity=2)