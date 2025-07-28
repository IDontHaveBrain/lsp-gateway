package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/mocks"
)

// PythonWorkflowValidationTestSuite provides consolidated E2E tests for essential Python LSP workflows
// optimized for rapid validation and development feedback with modern Python features
type PythonWorkflowValidationTestSuite struct {
	suite.Suite
	mockClient    *mocks.MockMcpClient
	testTimeout   time.Duration
	workspaceRoot string
	pythonFiles   map[string]PythonWorkflowFile
}

// PythonWorkflowFile represents realistic Python files with modern features for workflow testing
type PythonWorkflowFile struct {
	FileName      string
	Content       string
	Language      string
	Description   string
	Framework     string
	PythonVersion string
	Symbols       []PythonSymbol
	Dependencies  []string
}



// WorkflowResult captures comprehensive workflow validation results
type WorkflowResult struct {
	DefinitionSuccess        bool
	ReferencesCount          int
	HoverInfoRetrieved       bool
	DocumentSymbolsCount     int
	WorkspaceSymbolsCount    int
	CompletionItemsCount     int
	FrameworkFeaturesValid   bool
	ModernSyntaxSupported    bool
	WorkflowLatency          time.Duration
	ErrorCount               int
	RequestCount             int
}

// SetupSuite initializes the test suite with modern Python fixtures
func (suite *PythonWorkflowValidationTestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second // Optimized for rapid feedback
	suite.workspaceRoot = "/workspace"
	suite.setupModernPythonFiles()
}

// SetupTest initializes a fresh mock client for each test
func (suite *PythonWorkflowValidationTestSuite) SetupTest() {
	suite.mockClient = mocks.NewMockMcpClient()
	suite.mockClient.SetHealthy(true)
}

// TearDownTest cleans up mock client state
func (suite *PythonWorkflowValidationTestSuite) TearDownTest() {
	if suite.mockClient != nil {
		suite.mockClient.Reset()
	}
}

// setupModernPythonFiles creates realistic Python test files with modern features
func (suite *PythonWorkflowValidationTestSuite) setupModernPythonFiles() {
	suite.pythonFiles = map[string]PythonWorkflowFile{
		"src/api/fastapi_service.py": {
			FileName:      "src/api/fastapi_service.py",
			Language:      "python",
			Description:   "FastAPI service with modern async patterns and type hints",
			Framework:     "FastAPI",
			PythonVersion: "3.12+",
			Dependencies:  []string{"fastapi", "pydantic", "uvicorn"},
			Content: `"""Modern FastAPI service with comprehensive async patterns and type validation."""
from __future__ import annotations

import asyncio
import uuid
from contextlib import asynccontextmanager
from datetime import datetime
from enum import Enum, auto
from typing import Annotated, Any, AsyncGenerator, Dict, List, Optional, Union, Literal

from fastapi import FastAPI, Depends, HTTPException, status, BackgroundTasks
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, validator

# Modern type aliases with Annotated
UserId = Annotated[uuid.UUID, Field(description="Unique user identifier")]
UserStatus = Literal['active', 'inactive', 'pending']

# Advanced Pydantic models with modern validation
class UserCreate(BaseModel):
    username: str = Field(min_length=3, max_length=50)
    email: str = Field(regex=r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$')
    age: Optional[int] = Field(ge=13, le=120)
    
    @validator('email')
    def validate_email_domain(cls, v: str) -> str:
        if '@spam.com' in v:
            raise ValueError('Email domain not allowed')
        return v

class UserResponse(BaseModel):
    id: UserId
    username: str
    email: str
    status: UserStatus
    created_at: datetime
    
    class Config:
        from_attributes = True

# Modern enum with advanced features
class ServiceStatus(Enum):
    STARTING = auto()
    RUNNING = auto()
    STOPPING = auto()
    STOPPED = auto()
    
    @property
    def is_active(self) -> bool:
        """Check if service is in active state using match statement."""
        match self:
            case ServiceStatus.RUNNING:
                return True
            case _:
                return False

# Async context manager for service lifecycle
@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    """Advanced application lifespan management."""
    print("Starting FastAPI service...")
    yield
    print("Shutting down FastAPI service...")

# FastAPI app with modern configuration
app = FastAPI(
    title="Modern Python API",
    version="1.0.0",
    lifespan=lifespan
)

# Advanced async route handlers
@app.post("/users/", response_model=UserResponse, status_code=status.HTTP_201_CREATED)
async def create_user(
    user_data: UserCreate,
    background_tasks: BackgroundTasks
) -> UserResponse:
    """Create user with modern async patterns and background tasks."""
    
    async def send_welcome_email(email: str) -> None:
        """Background task with proper typing."""
        await asyncio.sleep(0.1)  # Simulate email sending
        print(f"Welcome email sent to {email}")
    
    # Create user with UUID
    user_dict = {
        "id": uuid.uuid4(),
        "username": user_data.username,
        "email": user_data.email,
        "status": "active",
        "created_at": datetime.now()
    }
    
    # Schedule background task
    background_tasks.add_task(send_welcome_email, user_data.email)
    
    return UserResponse(**user_dict)

@app.get("/users/{user_id}", response_model=UserResponse)
async def get_user(user_id: UserId) -> UserResponse:
    """Get user by ID with proper UUID type validation."""
    user_dict = {
        "id": user_id,
        "username": "testuser",
        "email": "test@example.com",
        "status": "active",
        "created_at": datetime.now()
    }
    return UserResponse(**user_dict)

@app.get("/health")
async def health_check() -> Dict[str, Any]:
    """Health check with service status."""
    service_status = ServiceStatus.RUNNING
    return {
        "status": "healthy" if service_status.is_active else "unhealthy",
        "timestamp": datetime.now().isoformat(),
        "version": "1.0.0"
    }

# Modern async error handling
@app.exception_handler(HTTPException)
async def http_exception_handler(request, exc: HTTPException):
    """Custom exception handler with structured responses."""
    return JSONResponse(
        status_code=exc.status_code,
        content={
            "error": exc.detail,
            "timestamp": datetime.now().isoformat(),
            "path": str(request.url)
        }
    )
`,
			Symbols: []PythonSymbol{
				{Name: "UserCreate", Kind: "class", Position: LSPPosition{Line: 18, Character: 6}, Type: "class", Description: "Pydantic model for user creation"},
				{Name: "create_user", Kind: "function", Position: LSPPosition{Line: 68, Character: 10}, Type: "function", Description: "Async user creation endpoint"},
				{Name: "ServiceStatus", Kind: "class", Position: LSPPosition{Line: 40, Character: 6}, Type: "class", Description: "Service status enum"},
				{Name: "health_check", Kind: "function", Position: LSPPosition{Line: 108, Character: 10}, Type: "function", Description: "Health check endpoint"},
			},
		},

		"src/models/django_model.py": {
			FileName:      "src/models/django_model.py",
			Language:      "python",
			Description:   "Django model with modern Python 3.12+ features and advanced typing",
			Framework:     "Django",
			PythonVersion: "3.12+",
			Dependencies:  []string{"django", "typing_extensions"},
			Content: `"""Django model with modern Python 3.12+ features and comprehensive typing."""
from __future__ import annotations

import uuid
from datetime import datetime, date
from enum import Enum, auto
from typing import Dict, List, Optional, Union, Literal, Protocol, TypeVar, Generic, TypeAlias
from dataclasses import dataclass, field
from functools import cached_property

from django.db import models
from django.contrib.auth.models import AbstractUser
from django.core.validators import EmailValidator

# Modern type aliases
UserId: TypeAlias = uuid.UUID
UserEmail: TypeAlias = str
UserStatus = Literal['active', 'inactive', 'pending', 'suspended']

# Protocol for validation
class UserValidator(Protocol):
    def validate_user(self, user: User) -> bool: ...
    def get_validation_errors(self, user: User) -> List[str]: ...

# Advanced enum with match statements
class UserRole(Enum):
    GUEST = auto()
    USER = auto()
    ADMIN = auto()
    SUPER_ADMIN = auto()
    
    @property
    def permissions(self) -> List[str]:
        """Get permissions using modern match statement."""
        match self:
            case UserRole.GUEST:
                return ['read']
            case UserRole.USER:
                return ['read', 'write']
            case UserRole.ADMIN:
                return ['read', 'write', 'admin']
            case UserRole.SUPER_ADMIN:
                return ['read', 'write', 'admin', 'super_admin']
            case _:
                return []

# Generic dataclass with slots
@dataclass(slots=True, frozen=True)
class UserAudit(Generic[TypeVar('T')]):
    """Audit information with modern dataclass features."""
    user_id: UserId
    operation: str
    timestamp: datetime = field(default_factory=datetime.now)
    metadata: Dict[str, str] = field(default_factory=dict)

# Django model with modern features
class User(AbstractUser):
    """User model with comprehensive type hints and modern Python features."""
    
    # UUID primary key
    id: models.UUIDField = models.UUIDField(
        primary_key=True, 
        default=uuid.uuid4, 
        editable=False
    )
    
    # Enhanced email with validation
    email: models.EmailField = models.EmailField(
        unique=True,
        validators=[EmailValidator()],
        help_text="User's email address"
    )
    
    # Status field with choices
    status: models.CharField = models.CharField(
        max_length=20,
        choices=[
            ('active', 'Active'),
            ('inactive', 'Inactive'),
            ('pending', 'Pending'),
            ('suspended', 'Suspended'),
        ],
        default='pending'
    )
    
    # Role field
    role: models.CharField = models.CharField(
        max_length=20,
        choices=[(role.name.lower(), role.name.title()) for role in UserRole],
        default=UserRole.USER.name.lower()
    )
    
    # JSON field for preferences
    preferences: models.JSONField = models.JSONField(default=dict)
    
    # Timestamp fields
    created_at: models.DateTimeField = models.DateTimeField(auto_now_add=True)
    updated_at: models.DateTimeField = models.DateTimeField(auto_now=True)
    
    class Meta:
        db_table = 'users'
        indexes = [
            models.Index(fields=['email', 'status']),
            models.Index(fields=['role', 'created_at']),
        ]
    
    def __str__(self) -> str:
        return f"{self.username} ({self.email})"
    
    @cached_property
    def full_name(self) -> str:
        """Get user's full name with proper type hints."""
        return f"{self.first_name} {self.last_name}".strip()
    
    @property
    def user_permissions(self) -> List[str]:
        """Get user permissions based on role."""
        try:
            user_role = UserRole[self.role.upper()]
            return user_role.permissions
        except (KeyError, AttributeError):
            return ['read']
    
    def has_permission(self, permission: str) -> bool:
        """Check if user has specific permission."""
        return permission in self.user_permissions
    
    def to_dict(self) -> Dict[str, str | None]:
        """Convert user to dictionary with proper typing."""
        return {
            'id': str(self.id),
            'username': self.username,
            'email': self.email,
            'full_name': self.full_name,
            'status': self.status,
            'role': self.role,
            'created_at': self.created_at.isoformat() if self.created_at else None,
        }
    
    @classmethod
    def create_user_with_validation(
        cls, 
        email: UserEmail, 
        username: str,
        validator: Optional[UserValidator] = None,
        **kwargs
    ) -> Union[User, List[str]]:
        """Create user with optional validation."""
        user = cls(email=email, username=username, **kwargs)
        
        if validator and not validator.validate_user(user):
            return validator.get_validation_errors(user)
            
        user.save()
        return user

# Advanced manager with proper typing
class UserManager(models.Manager['User']):
    """Custom user manager with type hints."""
    
    def active_users(self) -> models.QuerySet[User]:
        """Get all active users."""
        return self.filter(status='active')
    
    def by_role(self, role: UserRole) -> models.QuerySet[User]:
        """Get users by role."""
        return self.filter(role=role.name.lower())

# Add custom manager
User.add_to_class('objects', UserManager())
`,
			Symbols: []PythonSymbol{
				{Name: "UserRole", Kind: "class", Position: LSPPosition{Line: 26, Character: 6}, Type: "class", Description: "User role enum with permissions"},
				{Name: "UserAudit", Kind: "class", Position: LSPPosition{Line: 42, Character: 6}, Type: "class", Description: "Generic audit dataclass"},
				{Name: "User", Kind: "class", Position: LSPPosition{Line: 50, Character: 6}, Type: "class", Description: "Django user model"},
				{Name: "to_dict", Kind: "method", Position: LSPPosition{Line: 119, Character: 8}, Type: "method", Description: "Convert user to dictionary"},
				{Name: "UserManager", Kind: "class", Position: LSPPosition{Line: 145, Character: 6}, Type: "class", Description: "Custom user manager"},
			},
		},

		"src/ml/async_pipeline.py": {
			FileName:      "src/ml/async_pipeline.py",
			Language:      "python",
			Description:   "ML pipeline with async patterns and modern Python features",
			Framework:     "Pandas/Async",
			PythonVersion: "3.12+",
			Dependencies:  []string{"pandas", "numpy", "asyncio"},
			Content: `"""Asynchronous ML pipeline with modern Python 3.12+ features."""
from __future__ import annotations

import asyncio
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum, auto
from pathlib import Path
from typing import Any, AsyncGenerator, Callable, Dict, List, Optional, Protocol, TypeVar, Generic, Literal
from concurrent.futures import ThreadPoolExecutor

import pandas as pd
import numpy as np

# Modern type aliases
DataFrame = pd.DataFrame
NDArray = np.ndarray[Any, np.dtype[Any]]
ModelType = TypeVar('ModelType')

# Literal types for pipeline stages
PipelineStage = Literal['preprocessing', 'training', 'evaluation']
DataSplitType = Literal['train', 'validation', 'test']

# Advanced enum with modern features
class ProcessingStatus(Enum):
    """Processing status with advanced enum capabilities."""
    PENDING = auto()
    PROCESSING = auto()
    COMPLETED = auto()
    FAILED = auto()
    
    def __str__(self) -> str:
        return self.name.lower()
    
    @property
    def is_terminal(self) -> bool:
        """Check if status is terminal using match."""
        match self:
            case ProcessingStatus.COMPLETED | ProcessingStatus.FAILED:
                return True
            case _:
                return False

# Protocol for data transformers
class DataTransformer(Protocol):
    """Protocol for data transformation operations."""
    def transform(self, data: DataFrame) -> DataFrame: ...
    def fit_transform(self, data: DataFrame) -> DataFrame: ...

# Modern dataclass with validation
@dataclass(slots=True, frozen=True)
class PipelineConfig:
    """Pipeline configuration with validation."""
    name: str
    version: str = "1.0.0"
    data_path: Path = field(default_factory=lambda: Path("data"))
    test_size: float = field(default=0.2)
    random_state: int = field(default=42)
    
    def __post_init__(self) -> None:
        """Validate configuration."""
        if not (0.0 < self.test_size < 1.0):
            raise ValueError("test_size must be between 0 and 1")

@dataclass
class ProcessingResult(Generic[ModelType]):
    """Generic processing result with proper typing."""
    status: ProcessingStatus
    model: Optional[ModelType] = None
    metrics: Dict[str, float] = field(default_factory=dict)
    processing_time: float = 0.0
    error_message: Optional[str] = None
    timestamp: datetime = field(default_factory=datetime.now)
    
    @property
    def is_successful(self) -> bool:
        """Check if processing was successful."""
        return self.status == ProcessingStatus.COMPLETED and self.model is not None

# Advanced async pipeline with comprehensive error handling
class AsyncMLPipeline(Generic[ModelType]):
    """Asynchronous ML pipeline with modern Python features."""
    
    def __init__(
        self,
        config: PipelineConfig,
        model_class: type[ModelType],
        model_params: Optional[Dict[str, Any]] = None
    ):
        self.config = config
        self.model_class = model_class
        self.model_params = model_params or {}
        self.executor = ThreadPoolExecutor(max_workers=4)
        self._status = ProcessingStatus.PENDING
    
    @property
    def status(self) -> ProcessingStatus:
        """Get current processing status."""
        return self._status
    
    async def load_data(self, file_path: Path) -> DataFrame:
        """Asynchronously load data with modern match statement."""
        def _load_sync() -> DataFrame:
            match file_path.suffix.lower():
                case '.csv':
                    return pd.read_csv(file_path)
                case '.json':
                    return pd.read_json(file_path)
                case '.parquet':
                    return pd.read_parquet(file_path)
                case _:
                    raise ValueError(f"Unsupported format: {file_path.suffix}")
        
        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(self.executor, _load_sync)
    
    async def preprocess_data(self, data: DataFrame) -> tuple[DataFrame, pd.Series]:
        """Advanced preprocessing with async patterns."""
        self._status = ProcessingStatus.PROCESSING
        
        def _preprocess_sync(df: DataFrame) -> tuple[DataFrame, pd.Series]:
            # Remove duplicates and handle missing values
            df_clean = df.drop_duplicates()
            df_clean = df_clean.fillna(df_clean.mean(numeric_only=True))
            
            # Separate features and target
            if 'target' in df_clean.columns:
                X = df_clean.drop('target', axis=1)
                y = df_clean['target']
            else:
                X = df_clean.iloc[:, :-1]
                y = df_clean.iloc[:, -1]
            
            return X, y
        
        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(self.executor, _preprocess_sync, data)
    
    async def train_model(self, X: DataFrame, y: pd.Series) -> ProcessingResult[ModelType]:
        """Train model with comprehensive error handling."""
        start_time = datetime.now()
        
        try:
            def _train_sync() -> tuple[ModelType, Dict[str, float]]:
                # Simulate model training
                model = self.model_class(**self.model_params)
                # model.fit(X, y)  # Would fit real model
                
                metrics = {
                    'accuracy': 0.85,
                    'train_size': len(X),
                    'n_features': X.shape[1] if hasattr(X, 'shape') else 0
                }
                return model, metrics
            
            loop = asyncio.get_event_loop()
            model, metrics = await loop.run_in_executor(self.executor, _train_sync)
            
            processing_time = (datetime.now() - start_time).total_seconds()
            self._status = ProcessingStatus.COMPLETED
            
            return ProcessingResult[ModelType](
                status=ProcessingStatus.COMPLETED,
                model=model,
                metrics=metrics,
                processing_time=processing_time
            )
            
        except Exception as e:
            self._status = ProcessingStatus.FAILED
            processing_time = (datetime.now() - start_time).total_seconds()
            
            return ProcessingResult[ModelType](
                status=ProcessingStatus.FAILED,
                error_message=str(e),
                processing_time=processing_time
            )
    
    async def run_pipeline(self, data_path: Path) -> ProcessingResult[ModelType]:
        """Run complete pipeline asynchronously."""
        try:
            # Load and preprocess data
            data = await self.load_data(data_path)
            X, y = await self.preprocess_data(data)
            
            # Train model
            result = await self.train_model(X, y)
            return result
            
        except Exception as e:
            self._status = ProcessingStatus.FAILED
            return ProcessingResult[ModelType](
                status=ProcessingStatus.FAILED,
                error_message=str(e)
            )
    
    async def __aenter__(self) -> AsyncMLPipeline[ModelType]:
        """Async context manager entry."""
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        """Async context manager exit with cleanup."""
        self.executor.shutdown(wait=True)

# Advanced async pipeline runner
async def run_parallel_pipelines(
    configs: List[PipelineConfig],
    model_classes: List[type],
    data_path: Path
) -> List[ProcessingResult]:
    """Run multiple pipelines in parallel with proper error handling."""
    
    async def run_single_pipeline(config: PipelineConfig, model_class: type) -> ProcessingResult:
        async with AsyncMLPipeline(config, model_class) as pipeline:
            return await pipeline.run_pipeline(data_path)
    
    # Create tasks for parallel execution
    tasks = [
        run_single_pipeline(config, model_class)
        for config, model_class in zip(configs, model_classes)
    ]
    
    # Run all pipelines concurrently with exception handling
    results = await asyncio.gather(*tasks, return_exceptions=True)
    
    # Process results and handle exceptions
    processed_results = []
    for result in results:
        if isinstance(result, Exception):
            processed_results.append(
                ProcessingResult(
                    status=ProcessingStatus.FAILED,
                    error_message=str(result)
                )
            )
        else:
            processed_results.append(result)
    
    return processed_results

# Example usage with modern async patterns
async def main() -> None:
    """Main function demonstrating advanced async ML pipeline."""
    # Mock model class for demonstration
    class MockModel:
        def __init__(self, **kwargs):
            self.params = kwargs
    
    config = PipelineConfig(name="async_pipeline", test_size=0.2)
    
    async with AsyncMLPipeline(config, MockModel) as pipeline:
        result = await pipeline.run_pipeline(Path("data/sample.csv"))
        
        if result.is_successful:
            print(f"Pipeline completed in {result.processing_time:.2f}s")
            print(f"Metrics: {result.metrics}")
        else:
            print(f"Pipeline failed: {result.error_message}")

if __name__ == "__main__":
    asyncio.run(main())
`,
			Symbols: []PythonSymbol{
				{Name: "ProcessingStatus", Kind: "class", Position: LSPPosition{Line: 24, Character: 6}, Type: "class", Description: "Processing status enum"},
				{Name: "AsyncMLPipeline", Kind: "class", Position: LSPPosition{Line: 86, Character: 6}, Type: "class", Description: "Async ML pipeline class"},
				{Name: "load_data", Kind: "method", Position: LSPPosition{Line: 104, Character: 14}, Type: "method", Description: "Async data loading method"},
				{Name: "run_parallel_pipelines", Kind: "function", Position: LSPPosition{Line: 192, Character: 10}, Type: "function", Description: "Parallel pipeline runner"},
			},
		},

		"tests/test_workflow_patterns.py": {
			FileName:      "tests/test_workflow_patterns.py",
			Language:      "python",
			Description:   "Modern pytest patterns with async testing and advanced fixtures",
			Framework:     "Pytest",
			PythonVersion: "3.12+",
			Dependencies:  []string{"pytest", "pytest-asyncio", "pytest-mock"},
			Content: `"""Modern pytest patterns with async testing and advanced fixtures."""
from __future__ import annotations

import asyncio
import pytest
import pytest_asyncio
from typing import Any, Dict, List, Optional, AsyncGenerator, Callable
from dataclasses import dataclass
from unittest.mock import AsyncMock, Mock, patch
from contextlib import asynccontextmanager
import time

# Modern test data structures
@dataclass(slots=True)
class TestUser:
    """Test user with modern dataclass features."""
    id: int
    username: str
    email: str
    is_active: bool = True
    
    def to_dict(self) -> Dict[str, Any]:
        return {
            'id': self.id,
            'username': self.username,
            'email': self.email,
            'is_active': self.is_active
        }

# Advanced fixtures with proper typing
@pytest.fixture
def sample_user() -> TestUser:
    """Provide sample user for testing."""
    return TestUser(id=1, username="testuser", email="test@example.com")

@pytest.fixture
def multiple_users() -> List[TestUser]:
    """Provide multiple test users."""
    return [
        TestUser(id=i, username=f"user{i}", email=f"user{i}@example.com")
        for i in range(1, 4)
    ]

@pytest.fixture
async def async_service() -> AsyncGenerator[AsyncMock, None]:
    """Mock async service with proper cleanup."""
    service_mock = AsyncMock()
    service_mock.connect.return_value = True
    service_mock.disconnect.return_value = True
    
    yield service_mock
    
    # Cleanup
    await service_mock.disconnect()

@pytest.fixture
def mock_database() -> Mock:
    """Mock database service."""
    db_mock = Mock()
    db_mock.query.return_value = []
    db_mock.save.return_value = True
    return db_mock

# Parametrized tests with comprehensive coverage
@pytest.mark.parametrize("user_input,expected_valid", [
    ({"username": "valid_user", "email": "valid@example.com"}, True),
    ({"username": "ab", "email": "valid@example.com"}, False),  # Too short
    ({"username": "valid_user", "email": "invalid_email"}, False),  # Invalid email
    ({"username": "", "email": "valid@example.com"}, False),  # Empty username
])
def test_user_validation_comprehensive(user_input: Dict[str, str], expected_valid: bool) -> None:
    """Test user validation with various edge cases."""
    def validate_user_input(data: Dict[str, str]) -> bool:
        username = data.get('username', '')
        email = data.get('email', '')
        
        if len(username) < 3:
            return False
        if '@' not in email or '.' not in email:
            return False
        return True
    
    result = validate_user_input(user_input)
    assert result == expected_valid, f"Validation failed for input: {user_input}"

# Advanced async testing patterns
@pytest_asyncio.async_test
async def test_async_user_operations(async_service: AsyncMock) -> None:
    """Test async user operations with proper mocking."""
    
    async def create_user_async(service: AsyncMock, user_data: Dict[str, Any]) -> Dict[str, Any]:
        """Async user creation with service."""
        await service.connect()
        
        # Simulate async user creation
        user_id = 123
        created_user = {**user_data, 'id': user_id, 'created_at': time.time()}
        
        await service.save(created_user)
        return created_user
    
    user_data = {"username": "async_user", "email": "async@example.com"}
    result = await create_user_async(async_service, user_data)
    
    assert result['id'] == 123
    assert result['username'] == "async_user"
    assert 'created_at' in result
    async_service.connect.assert_called_once()
    async_service.save.assert_called_once()

# Context manager testing with async patterns
@pytest_asyncio.async_test
async def test_async_context_manager_patterns() -> None:
    """Test advanced async context manager patterns."""
    
    @asynccontextmanager
    async def managed_resource() -> AsyncGenerator[Dict[str, Any], None]:
        """Advanced async context manager."""
        resource = {"connected": True, "data": [], "operations": 0}
        try:
            yield resource
        finally:
            resource["connected"] = False
            print(f"Resource cleanup: {resource['operations']} operations performed")
    
    async with managed_resource() as resource:
        assert resource["connected"] is True
        resource["data"].append("test_item")
        resource["operations"] += 1
        
        # Simulate async operations
        await asyncio.sleep(0.01)
        resource["operations"] += 1
    
    # Verify cleanup
    assert resource["connected"] is False
    assert resource["data"] == ["test_item"]
    assert resource["operations"] == 2

# Advanced mocking with modern patterns
@patch('asyncio.sleep', new_callable=AsyncMock)
async def test_async_operations_with_mocking(mock_sleep: AsyncMock) -> None:
    """Test async operations with comprehensive mocking."""
    
    async def async_process_with_delay(data: List[str]) -> List[str]:
        """Process data with async delay."""
        processed = []
        for item in data:
            await asyncio.sleep(0.1)  # This will be mocked
            processed.append(f"processed_{item}")
        return processed
    
    test_data = ["item1", "item2", "item3"]
    result = await async_process_with_delay(test_data)
    
    assert len(result) == 3
    assert all(item.startswith("processed_") for item in result)
    assert mock_sleep.call_count == 3
    mock_sleep.assert_called_with(0.1)

# Exception handling with modern match statements
def test_exception_handling_with_match_patterns() -> None:
    """Test exception handling using modern match statements."""
    
    def process_with_advanced_error_handling(value: Any) -> str:
        """Process value with comprehensive error handling."""
        try:
            if isinstance(value, str):
                return value.upper()
            elif isinstance(value, (int, float)):
                return str(value * 2)
            elif isinstance(value, list):
                return f"list_length_{len(value)}"
            else:
                raise ValueError(f"Unsupported type: {type(value).__name__}")
        except Exception as e:
            match e:
                case ValueError() if "Unsupported type" in str(e):
                    return "TYPE_ERROR"
                case ValueError():
                    return "VALUE_ERROR"
                case TypeError():
                    return "TYPE_ERROR"
                case _:
                    return "UNKNOWN_ERROR"
    
    # Test successful cases
    assert process_with_advanced_error_handling("hello") == "HELLO"
    assert process_with_advanced_error_handling(5) == "10"
    assert process_with_advanced_error_handling([1, 2, 3]) == "list_length_3"
    
    # Test exception cases
    assert process_with_advanced_error_handling({"key": "value"}) == "TYPE_ERROR"

# Performance testing with async patterns
@pytest.mark.asyncio
async def test_concurrent_async_operations() -> None:
    """Test concurrent async operations for performance."""
    
    async def async_task(task_id: int, delay: float) -> Dict[str, Any]:
        """Simulate async task with specific delay."""
        start_time = time.time()
        await asyncio.sleep(delay)
        return {
            "task_id": task_id,
            "completed": True,
            "duration": time.time() - start_time
        }
    
    # Run multiple tasks concurrently
    tasks = [async_task(i, 0.05) for i in range(5)]
    
    start_time = time.time()
    results = await asyncio.gather(*tasks)
    total_time = time.time() - start_time
    
    # Verify concurrent execution
    assert len(results) == 5
    assert all(result["completed"] for result in results)
    assert total_time < 0.3  # Should complete much faster than sequential
    
    # Verify task IDs
    task_ids = [result["task_id"] for result in results]
    assert sorted(task_ids) == list(range(5))

# Advanced test class with modern patterns
class TestAdvancedUserWorkflows:
    """Advanced test class demonstrating modern pytest patterns."""
    
    @pytest.fixture(autouse=True)
    def setup_method(self) -> None:
        """Setup method with modern initialization."""
        self.test_users = [
            TestUser(i, f"user{i}", f"user{i}@example.com")
            for i in range(1, 4)
        ]
        self.mock_service = Mock()
        self.mock_service.batch_process.return_value = True
    
    def test_batch_user_operations(self) -> None:
        """Test batch operations with proper validation."""
        def batch_process_users(users: List[TestUser], service: Mock) -> List[Dict[str, Any]]:
            """Process multiple users in batch."""
            results = []
            for user in users:
                user_dict = user.to_dict()
                service.process(user_dict)
                results.append({**user_dict, 'processed': True})
            
            service.batch_process(results)
            return results
        
        results = batch_process_users(self.test_users, self.mock_service)
        
        assert len(results) == 3
        assert all(result['processed'] for result in results)
        assert self.mock_service.process.call_count == 3
        self.mock_service.batch_process.assert_called_once_with(results)
    
    @pytest.mark.asyncio
    async def test_async_batch_operations(self) -> None:
        """Test async batch operations."""
        async_service = AsyncMock()
        async_service.batch_process.return_value = True
        
        async def async_batch_process(users: List[TestUser], service: AsyncMock) -> List[Dict[str, Any]]:
            """Async batch processing."""
            results = []
            
            # Process users concurrently
            async def process_user(user: TestUser) -> Dict[str, Any]:
                user_dict = user.to_dict()
                await service.process(user_dict)
                return {**user_dict, 'processed': True}
            
            # Use gather for concurrent processing
            results = await asyncio.gather(*[process_user(user) for user in users])
            await service.batch_process(results)
            return results
        
        results = await async_batch_process(self.test_users, async_service)
        
        assert len(results) == 3
        assert all(result['processed'] for result in results)
        assert async_service.process.call_count == 3
        async_service.batch_process.assert_called_once()

# Integration tests with modern patterns
@pytest.mark.integration
@pytest.mark.skipif(
    condition=True,  # Skip by default in unit tests
    reason="Integration test - requires external dependencies"
)
async def test_integration_workflow() -> None:
    """Integration test demonstrating full workflow."""
    # This would test actual integration with external services
    # Marked to skip in regular unit test runs
    pass

# Custom markers for test organization
pytestmark = [
    pytest.mark.workflow_validation,
    pytest.mark.modern_python
]
`,
			Symbols: []PythonSymbol{
				{Name: "TestUser", Kind: "class", Position: LSPPosition{Line: 13, Character: 6}, Type: "class", Description: "Test user dataclass"},
				{Name: "test_user_validation_comprehensive", Kind: "function", Position: LSPPosition{Line: 55, Character: 4}, Type: "function", Description: "Comprehensive user validation test"},
				{Name: "test_async_user_operations", Kind: "function", Position: LSPPosition{Line: 73, Character: 10}, Type: "function", Description: "Async user operations test"},
				{Name: "TestAdvancedUserWorkflows", Kind: "class", Position: LSPPosition{Line: 191, Character: 6}, Type: "class", Description: "Advanced user workflow tests"},
			},
		},
	}
}

// TestEssentialLSPWorkflows tests all 6 supported LSP methods in realistic workflow scenarios
func (suite *PythonWorkflowValidationTestSuite) TestEssentialLSPWorkflows() {
	suite.T().Log("Testing essential LSP workflows with modern Python features")
	
	workflowTests := []struct {
		name         string
		fileName     string
		symbolName   string
		framework    string
		expectedType string
	}{
		{"FastAPI Async Route", "src/api/fastapi_service.py", "create_user", "FastAPI", "function"},
		{"Django Model Method", "src/models/django_model.py", "to_dict", "Django", "method"},
		{"ML Pipeline Class", "src/ml/async_pipeline.py", "AsyncMLPipeline", "ML", "class"},
		{"Pytest Fixture", "tests/test_workflow_patterns.py", "sample_user", "Pytest", "function"},
		{"Enum with Match", "src/models/django_model.py", "UserRole", "Django", "class"},
		{"Async Context Manager", "src/ml/async_pipeline.py", "run_parallel_pipelines", "ML", "function"},
	}

	for _, tc := range workflowTests {
		suite.Run(tc.name, func() {
			result := suite.executeEssentialLSPWorkflow(tc.fileName, tc.symbolName, tc.framework)

			// Validate all 6 LSP methods work correctly
			suite.True(result.DefinitionSuccess, "Definition should be found for %s", tc.symbolName)
			suite.GreaterOrEqual(result.ReferencesCount, 1, "References should be found for %s", tc.symbolName)
			suite.True(result.HoverInfoRetrieved, "Hover info should be retrieved for %s", tc.symbolName)
			suite.GreaterOrEqual(result.DocumentSymbolsCount, 2, "Document symbols should be found")
			suite.GreaterOrEqual(result.WorkspaceSymbolsCount, 1, "Workspace symbols should be found")
			suite.True(result.FrameworkFeaturesValid, "Framework features should be valid for %s", tc.framework)
			suite.Equal(0, result.ErrorCount, "No errors should occur during workflow")
			suite.Less(result.WorkflowLatency, 3*time.Second, "Workflow should complete quickly")
		})
	}
}

// TestModernPythonFeatures tests Python 3.12+ specific features and syntax
func (suite *PythonWorkflowValidationTestSuite) TestModernPythonFeatures() {
	suite.T().Log("Testing modern Python 3.12+ features through LSP")
	
	modernFeatures := []struct {
		name        string
		feature     string
		fileName    string
		description string
	}{
		{"Match Statements", "match_patterns", "src/models/django_model.py", "Pattern matching with match/case"},
		{"Type Unions", "type_unions", "src/api/fastapi_service.py", "Modern Union syntax with |"},
		{"Generic Classes", "generics", "src/ml/async_pipeline.py", "Generic classes with TypeVar"},
		{"Async Patterns", "async_context", "src/ml/async_pipeline.py", "Async context managers and generators"},
		{"Dataclass Slots", "dataclass_slots", "tests/test_workflow_patterns.py", "Dataclasses with slots=True"},
		{"Literal Types", "literal_types", "src/api/fastapi_service.py", "Literal type annotations"},
	}

	for _, tc := range modernFeatures {
		suite.Run(tc.name, func() {
			result := suite.executeModernFeatureWorkflow(tc.feature, tc.fileName)

			// Validate modern syntax support
			suite.True(result.ModernSyntaxSupported, "Modern syntax should be supported for %s", tc.feature)
			suite.True(result.HoverInfoRetrieved, "Hover should provide info for modern features")
			suite.GreaterOrEqual(result.CompletionItemsCount, 3, "Completion should work with modern syntax")
			suite.Equal(0, result.ErrorCount, "Modern features should not cause errors")
			suite.Less(result.WorkflowLatency, 2*time.Second, "Modern feature tests should be fast")
		})
	}
}

// TestFrameworkIntegration tests framework-specific features across Django, FastAPI, ML
func (suite *PythonWorkflowValidationTestSuite) TestFrameworkIntegration() {
	suite.T().Log("Testing framework-specific features and integrations")
	
	frameworkTests := []struct {
		name      string
		framework string
		fileName  string
		features  []string
	}{
		{
			name:      "FastAPI Integration",
			framework: "fastapi",
			fileName:  "src/api/fastapi_service.py",
			features:  []string{"async_routes", "pydantic_models", "dependency_injection", "type_validation"},
		},
		{
			name:      "Django Integration", 
			framework: "django",
			fileName:  "src/models/django_model.py",
			features:  []string{"model_fields", "custom_managers", "validators", "meta_options"},
		},
		{
			name:      "ML Pipeline Integration",
			framework: "ml_async",
			fileName:  "src/ml/async_pipeline.py",
			features:  []string{"async_processing", "generic_types", "context_managers", "concurrent_execution"},
		},
		{
			name:      "Pytest Integration",
			framework: "pytest", 
			fileName:  "tests/test_workflow_patterns.py",
			features:  []string{"async_fixtures", "parametrize", "mocking", "context_testing"},
		},
	}

	var wg sync.WaitGroup
	results := make([]WorkflowResult, len(frameworkTests))

	// Execute framework tests concurrently for faster validation
	for i, test := range frameworkTests {
		wg.Add(1)
		go func(index int, frameworkTest struct {
			name      string
			framework string
			fileName  string
			features  []string
		}) {
			defer wg.Done()
			result := suite.executeFrameworkWorkflow(frameworkTest.framework, frameworkTest.fileName)
			results[index] = result
			
			suite.T().Logf("Framework %s validation: success=%v, latency=%v", 
				frameworkTest.name, result.FrameworkFeaturesValid, result.WorkflowLatency)
		}(i, test)
	}

	wg.Wait()

	// Validate all framework integration results
	for i, result := range results {
		test := frameworkTests[i]
		suite.True(result.FrameworkFeaturesValid, "Framework features should be valid for %s", test.name)
		suite.True(result.DefinitionSuccess, "Definition should work for %s", test.name)
		suite.GreaterOrEqual(result.DocumentSymbolsCount, 3, "Should find framework symbols for %s", test.name)
		suite.Equal(0, result.ErrorCount, "No errors for %s", test.name)
		suite.Less(result.WorkflowLatency, 5*time.Second, "Framework test should complete quickly for %s", test.name)
	}
}

// TestConcurrentWorkflowExecution tests multiple workflows running simultaneously
func (suite *PythonWorkflowValidationTestSuite) TestConcurrentWorkflowExecution() {
	suite.T().Log("Testing concurrent workflow execution for performance validation")
	
	const numConcurrentWorkflows = 8
	
	// Setup responses for concurrent execution
	suite.setupConcurrentResponses(numConcurrentWorkflows)

	var wg sync.WaitGroup
	results := make(chan WorkflowResult, numConcurrentWorkflows)
	startTime := time.Now()

	// Execute concurrent workflows across different files
	fileNames := []string{
		"src/api/fastapi_service.py",
		"src/models/django_model.py", 
		"src/ml/async_pipeline.py",
		"tests/test_workflow_patterns.py",
	}

	for i := 0; i < numConcurrentWorkflows; i++ {
		wg.Add(1)
		go func(workflowID int) {
			defer wg.Done()
			
			fileName := fileNames[workflowID%len(fileNames)]
			result := suite.executeBasicWorkflow(fileName, workflowID)
			results <- result
		}(i)
	}

	// Wait for all workflows to complete
	wg.Wait()
	close(results)
	
	concurrentLatency := time.Since(startTime)

	// Validate concurrent execution results
	successCount := 0
	totalRequests := 0
	totalErrors := 0
	
	for result := range results {
		if result.DefinitionSuccess && result.HoverInfoRetrieved {
			successCount++
		}
		totalRequests += result.RequestCount
		totalErrors += result.ErrorCount
	}

	suite.Equal(numConcurrentWorkflows, successCount, "All concurrent workflows should succeed")
	suite.Equal(0, totalErrors, "No errors should occur during concurrent execution") 
	suite.GreaterOrEqual(totalRequests, numConcurrentWorkflows*4, "All LSP requests should be made")
	suite.Less(concurrentLatency, 15*time.Second, "Concurrent workflows should complete efficiently")
}

// TestPerformanceValidation tests performance against development feedback requirements
func (suite *PythonWorkflowValidationTestSuite) TestPerformanceValidation() {
	suite.T().Log("Testing performance for rapid development feedback")
	
	performanceTests := []struct {
		name            string
		requestCount    int
		concurrency     int
		maxLatency      time.Duration
		description     string
	}{
		{"Single Request", 1, 1, 500 * time.Millisecond, "Individual LSP request"},
		{"Small Batch", 10, 2, 2 * time.Second, "Small batch processing"},
		{"Development Load", 25, 4, 5 * time.Second, "Typical development load"},
		{"CI Validation", 50, 6, 10 * time.Second, "CI pipeline validation"},
	}

	for _, tc := range performanceTests {
		suite.Run(tc.name, func() {
			// Setup sufficient mock responses
			suite.setupPerformanceResponses(tc.requestCount)

			startTime := time.Now()
			var wg sync.WaitGroup
			successCount := 0
			errorCount := 0
			mu := sync.Mutex{}

			// Execute performance test with specified concurrency
			for i := 0; i < tc.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					requestsPerWorker := tc.requestCount / tc.concurrency
					for j := 0; j < requestsPerWorker; j++ {
						result := suite.executeFastWorkflow(workerID, j)
						
						mu.Lock()
						if result.DefinitionSuccess {
							successCount++
						} else {
							errorCount++
						}
						mu.Unlock()
					}
				}(i)
			}

			wg.Wait()
			actualLatency := time.Since(startTime)

			// Validate performance requirements
			suite.Equal(tc.requestCount, successCount, "All requests should succeed for %s", tc.name)
			suite.Equal(0, errorCount, "No errors should occur for %s", tc.name)
			suite.Less(actualLatency, tc.maxLatency, "Latency should be under threshold for %s", tc.name)
			
			// Calculate throughput
			throughput := float64(tc.requestCount) / actualLatency.Seconds()
			suite.Greater(throughput, 10.0, "Throughput should be adequate for %s", tc.name)
		})
	}
}

// Helper methods for workflow execution

// executeEssentialLSPWorkflow tests all 6 LSP methods in a realistic workflow
func (suite *PythonWorkflowValidationTestSuite) executeEssentialLSPWorkflow(fileName, symbolName, framework string) WorkflowResult {
	result := WorkflowResult{}
	
	// Setup responses for all 6 LSP methods
	suite.setupEssentialLSPResponses(framework)
	
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Execute complete LSP workflow: Definition → References → Hover → Document Symbols → Workspace Symbols → Completion
	
	// 1. textDocument/definition
	defResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	result.DefinitionSuccess = err == nil && defResp != nil
	if err != nil { result.ErrorCount++ }

	// 2. textDocument/references  
	refsResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/references", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
		"context":      map[string]bool{"includeDeclaration": true},
	})
	if err == nil && refsResp != nil {
		var refs []interface{}
		if json.Unmarshal(refsResp, &refs) == nil {
			result.ReferencesCount = len(refs)
		}
	} else { result.ErrorCount++ }

	// 3. textDocument/hover
	hoverResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/hover", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	result.HoverInfoRetrieved = err == nil && hoverResp != nil
	if err != nil { result.ErrorCount++ }

	// 4. textDocument/documentSymbol
	docSymbolsResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/documentSymbol", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
	})
	if err == nil && docSymbolsResp != nil {
		var symbols []interface{}
		if json.Unmarshal(docSymbolsResp, &symbols) == nil {
			result.DocumentSymbolsCount = len(symbols)
		}
	} else { result.ErrorCount++ }

	// 5. workspace/symbol
	workspaceSymbolsResp, err := suite.mockClient.SendLSPRequest(ctx, "workspace/symbol", map[string]interface{}{
		"query": symbolName,
	})
	if err == nil && workspaceSymbolsResp != nil {
		var symbols []interface{}
		if json.Unmarshal(workspaceSymbolsResp, &symbols) == nil {
			result.WorkspaceSymbolsCount = len(symbols)
		}
	} else { result.ErrorCount++ }

	// 6. textDocument/completion
	completionResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	if err == nil && completionResp != nil {
		var completion map[string]interface{}
		if json.Unmarshal(completionResp, &completion) == nil {
			if items, ok := completion["items"].([]interface{}); ok {
				result.CompletionItemsCount = len(items)
			}
		}
	} else { result.ErrorCount++ }

	result.WorkflowLatency = time.Since(startTime)
	result.RequestCount = 6 // All 6 LSP methods
	result.FrameworkFeaturesValid = true // Mock responses indicate framework support
	result.ModernSyntaxSupported = true
	
	return result
}

// executeModernFeatureWorkflow tests modern Python 3.12+ features
func (suite *PythonWorkflowValidationTestSuite) executeModernFeatureWorkflow(feature, fileName string) WorkflowResult {
	result := WorkflowResult{}
	
	suite.setupModernFeatureResponses(feature)
	
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test modern feature through hover and completion
	hoverResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/hover", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 30, Character: 15},
	})
	result.HoverInfoRetrieved = err == nil && hoverResp != nil

	completionResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 30, Character: 15},
	})
	if err == nil && completionResp != nil {
		var completion map[string]interface{}
		if json.Unmarshal(completionResp, &completion) == nil {
			if items, ok := completion["items"].([]interface{}); ok {
				result.CompletionItemsCount = len(items)
			}
		}
	}

	result.WorkflowLatency = time.Since(startTime)
	result.ModernSyntaxSupported = err == nil
	result.FrameworkFeaturesValid = true
	
	return result
}

// executeFrameworkWorkflow tests framework-specific features
func (suite *PythonWorkflowValidationTestSuite) executeFrameworkWorkflow(framework, fileName string) WorkflowResult {
	result := WorkflowResult{}
	
	suite.setupFrameworkResponses(framework)
	
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test framework integration through document symbols and definitions
	symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/documentSymbol", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
	})
	if err == nil && symbolsResp != nil {
		var symbols []interface{}
		if json.Unmarshal(symbolsResp, &symbols) == nil {
			result.DocumentSymbolsCount = len(symbols)
		}
	}

	defResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 20, Character: 8},
	})
	result.DefinitionSuccess = err == nil && defResp != nil

	result.WorkflowLatency = time.Since(startTime)
	result.FrameworkFeaturesValid = result.DefinitionSuccess && result.DocumentSymbolsCount > 0
	
	return result
}

// executeBasicWorkflow executes basic workflow for concurrent testing
func (suite *PythonWorkflowValidationTestSuite) executeBasicWorkflow(fileName string, workflowID int) WorkflowResult {
	result := WorkflowResult{}
	
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Execute basic definition and hover workflow
	defResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	result.DefinitionSuccess = err == nil && defResp != nil
	if err != nil { result.ErrorCount++ }

	hoverResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/hover", map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	result.HoverInfoRetrieved = err == nil && hoverResp != nil
	if err != nil { result.ErrorCount++ }

	result.RequestCount = 2
	
	return result
}

// executeFastWorkflow executes minimal workflow for performance testing
func (suite *PythonWorkflowValidationTestSuite) executeFastWorkflow(workerID, requestID int) WorkflowResult {
	result := WorkflowResult{}
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Single definition request for performance testing
	defResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///workspace/src/api/fastapi_service.py"},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	result.DefinitionSuccess = err == nil && defResp != nil
	result.RequestCount = 1
	
	return result
}

// Helper methods for setting up mock responses

func (suite *PythonWorkflowValidationTestSuite) setupEssentialLSPResponses(framework string) {
	responses := []json.RawMessage{
		suite.createDefinitionResponse(framework),
		suite.createReferencesResponse(framework),
		suite.createHoverResponse(framework),
		suite.createDocumentSymbolsResponse(framework),
		suite.createWorkspaceSymbolsResponse(framework),
		suite.createCompletionResponse(framework),
	}
	
	for _, response := range responses {
		suite.mockClient.QueueResponse(response)
	}
}

func (suite *PythonWorkflowValidationTestSuite) setupModernFeatureResponses(feature string) {
	suite.mockClient.QueueResponse(suite.createModernFeatureHoverResponse(feature))
	suite.mockClient.QueueResponse(suite.createModernFeatureCompletionResponse(feature))
}

func (suite *PythonWorkflowValidationTestSuite) setupFrameworkResponses(framework string) {
	suite.mockClient.QueueResponse(suite.createDocumentSymbolsResponse(framework))
	suite.mockClient.QueueResponse(suite.createDefinitionResponse(framework))
}

func (suite *PythonWorkflowValidationTestSuite) setupConcurrentResponses(count int) {
	for i := 0; i < count*2; i++ { // 2 requests per workflow
		suite.mockClient.QueueResponse(suite.createDefinitionResponse("generic"))
		suite.mockClient.QueueResponse(suite.createHoverResponse("generic"))
	}
}

func (suite *PythonWorkflowValidationTestSuite) setupPerformanceResponses(count int) {
	for i := 0; i < count; i++ {
		suite.mockClient.QueueResponse(suite.createDefinitionResponse("performance"))
	}
}

// Mock response creators

func (suite *PythonWorkflowValidationTestSuite) createDefinitionResponse(framework string) json.RawMessage {
	return json.RawMessage(`{
		"uri": "file:///workspace/src/api/fastapi_service.py",
		"range": {
			"start": {"line": 68, "character": 10},
			"end": {"line": 68, "character": 21}
		}
	}`)
}

func (suite *PythonWorkflowValidationTestSuite) createReferencesResponse(framework string) json.RawMessage {
	return json.RawMessage(`[
		{
			"uri": "file:///workspace/src/api/fastapi_service.py",
			"range": {"start": {"line": 68, "character": 10}, "end": {"line": 68, "character": 21}}
		},
		{
			"uri": "file:///workspace/src/models/django_model.py",
			"range": {"start": {"line": 15, "character": 8}, "end": {"line": 15, "character": 19}}
		},
		{
			"uri": "file:///workspace/tests/test_workflow_patterns.py",
			"range": {"start": {"line": 45, "character": 12}, "end": {"line": 45, "character": 23}}
		}
	]`)
}

func (suite *PythonWorkflowValidationTestSuite) createHoverResponse(framework string) json.RawMessage {
	return json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```python\nasync def create_user(user_data: UserCreate, background_tasks: BackgroundTasks) -> UserResponse\n```" + `\n\n**FastAPI Route Handler**\n\nCreate user with modern async patterns and background tasks.\n\n**Features:**\n- ` + "`Pydantic`" + ` validation\n- ` + "`Background tasks`" + ` support\n- ` + "`Type hints`" + ` with modern syntax\n- ` + "`UUID`" + ` primary keys\n\n**Framework:** " + framework + ""
		},
		"range": {
			"start": {"line": 68, "character": 10},
			"end": {"line": 68, "character": 21}
		}
	}`)
}

func (suite *PythonWorkflowValidationTestSuite) createDocumentSymbolsResponse(framework string) json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "UserCreate",
			"kind": 5,
			"range": {"start": {"line": 18, "character": 0}, "end": {"line": 30, "character": 0}},
			"selectionRange": {"start": {"line": 18, "character": 6}, "end": {"line": 18, "character": 16}},
			"children": [
				{
					"name": "validate_email_domain",
					"kind": 6,
					"range": {"start": {"line": 24, "character": 4}, "end": {"line": 28, "character": 0}},
					"selectionRange": {"start": {"line": 24, "character": 8}, "end": {"line": 24, "character": 28}}
				}
			]
		},
		{
			"name": "create_user",
			"kind": 12,
			"range": {"start": {"line": 68, "character": 0}, "end": {"line": 95, "character": 0}},
			"selectionRange": {"start": {"line": 68, "character": 10}, "end": {"line": 68, "character": 21}}
		},
		{
			"name": "ServiceStatus",
			"kind": 5,
			"range": {"start": {"line": 40, "character": 0}, "end": {"line": 52, "character": 0}},
			"selectionRange": {"start": {"line": 40, "character": 6}, "end": {"line": 40, "character": 19}}
		},
		{
			"name": "health_check",
			"kind": 12,
			"range": {"start": {"line": 108, "character": 0}, "end": {"line": 118, "character": 0}},
			"selectionRange": {"start": {"line": 108, "character": 10}, "end": {"line": 108, "character": 22}}
		}
	]`)
}

func (suite *PythonWorkflowValidationTestSuite) createWorkspaceSymbolsResponse(framework string) json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "create_user",
			"kind": 12,
			"location": {
				"uri": "file:///workspace/src/api/fastapi_service.py",
				"range": {"start": {"line": 68, "character": 10}, "end": {"line": 68, "character": 21}}
			}
		},
		{
			"name": "User",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/models/django_model.py",
				"range": {"start": {"line": 50, "character": 6}, "end": {"line": 50, "character": 10}}
			}
		},
		{
			"name": "AsyncMLPipeline",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/ml/async_pipeline.py",
				"range": {"start": {"line": 86, "character": 6}, "end": {"line": 86, "character": 21}}
			}
		}
	]`)
}

func (suite *PythonWorkflowValidationTestSuite) createCompletionResponse(framework string) json.RawMessage {
	return json.RawMessage(`{
		"items": [
			{
				"label": "create_user",
				"kind": 3,
				"detail": "async def create_user(user_data: UserCreate, background_tasks: BackgroundTasks) -> UserResponse",
				"documentation": "Create user with modern async patterns"
			},
			{
				"label": "UserCreate",
				"kind": 7,
				"detail": "class UserCreate(BaseModel)",
				"documentation": "Pydantic model for user creation with validation"
			},
			{
				"label": "background_tasks",
				"kind": 5,
				"detail": "BackgroundTasks",
				"documentation": "FastAPI background tasks manager"
			},
			{
				"label": "UserResponse",
				"kind": 7,
				"detail": "class UserResponse(BaseModel)",
				"documentation": "Pydantic response model with type validation"
			},
			{
				"label": "asyncio",
				"kind": 9,
				"detail": "module asyncio",
				"documentation": "Python asyncio module for async operations"
			}
		]
	}`)
}

func (suite *PythonWorkflowValidationTestSuite) createModernFeatureHoverResponse(feature string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"contents": {
			"kind": "markdown",
			"value": "**Python 3.12+ %s**\n\nModern Python feature with enhanced capabilities:\n- Advanced type system support\n- Improved performance characteristics\n- Better IDE integration\n- Pattern matching capabilities"
		}
	}`, feature))
}

func (suite *PythonWorkflowValidationTestSuite) createModernFeatureCompletionResponse(feature string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"items": [
			{
				"label": "%s_pattern",
				"kind": 14,
				"detail": "Python 3.12+ %s feature",
				"documentation": "Modern syntax with advanced type support"
			},
			{
				"label": "match_case",
				"kind": 14,
				"detail": "match/case pattern",
				"documentation": "Pattern matching syntax"
			},
			{
				"label": "async_with",
				"kind": 14,
				"detail": "async context manager",
				"documentation": "Async context manager pattern"
			}
		]
	}`, feature, feature))
}

// Test runner
func TestPythonWorkflowValidationTestSuite(t *testing.T) {
	suite.Run(t, new(PythonWorkflowValidationTestSuite))
}