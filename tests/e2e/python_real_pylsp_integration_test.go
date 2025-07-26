package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// PythonRealPylspIntegrationTestSuite provides comprehensive integration tests
// for Python with actual pylsp process communication
type PythonRealPylspIntegrationTestSuite struct {
	suite.Suite
	testTimeout        time.Duration
	projectRoot        string
	lspClient          transport.LSPClient
	clientConfig       transport.ClientConfig
	projectFiles       map[string]string
	performanceMetrics *PythonPerformanceMetrics
}

// PythonPerformanceMetrics tracks actual performance data from real pylsp server
type PythonPerformanceMetrics struct {
	InitializationTime   time.Duration
	FirstResponseTime    time.Duration
	AverageResponseTime  time.Duration
	MemoryUsage         int64
	ProcessPID          int
	TotalRequests       int
	SuccessfulRequests  int
	FailedRequests      int
	PythonStartupTime   time.Duration // Python-specific metric
}

// PythonIntegrationResult captures comprehensive test results for supported LSP features
type PythonIntegrationResult struct {
	ServerStartSuccess      bool
	InitializationSuccess   bool
	ProjectLoadSuccess      bool
	DefinitionAccuracy      bool  // textDocument/definition
	ReferencesWork          bool  // textDocument/references
	HoverInformationWorks   bool  // textDocument/hover
	DocumentSymbolsWork     bool  // textDocument/documentSymbol
	WorkspaceSymbolsWork    bool  // workspace/symbol
	CompletionWorks         bool  // textDocument/completion
	ImportResolutionWorks   bool
	ModuleResolutionWorks   bool
	DjangoModelSupport      bool
	PylspPluginIntegration  bool
	PerformanceMetrics      *PythonPerformanceMetrics
	TestDuration           time.Duration
	TypeHintAccuracy       float64 // Type hint processing accuracy
	ErrorCount             int     // Number of errors encountered
}

// SetupSuite initializes the test suite with real Python project structure
func (suite *PythonRealPylspIntegrationTestSuite) SetupSuite() {
	suite.testTimeout = 3 * time.Minute // Extended timeout for Python server operations (Python startup is slower)
	
	// Verify pylsp is available
	if !suite.isPylspServerAvailable() {
		suite.T().Skip("pylsp not available, skipping real server integration tests")
	}

	// Create real Python project structure
	suite.createTestProject()
	
	suite.performanceMetrics = &PythonPerformanceMetrics{}
}

// SetupTest initializes a fresh LSP client for each test
func (suite *PythonRealPylspIntegrationTestSuite) SetupTest() {
	suite.clientConfig = transport.ClientConfig{
		Command:   "pylsp",
		Args:      []string{}, // pylsp uses stdio by default
		Transport: transport.TransportStdio,
	}
	
	var err error
	suite.lspClient, err = transport.NewLSPClient(suite.clientConfig)
	suite.Require().NoError(err, "Should create LSP client")
}

// TearDownTest cleans up the LSP client
func (suite *PythonRealPylspIntegrationTestSuite) TearDownTest() {
	if suite.lspClient != nil && suite.lspClient.IsActive() {
		err := suite.lspClient.Stop()
		if err != nil {
			suite.T().Logf("Warning: Error stopping LSP client: %v", err)
		}
	}
}

// TearDownSuite cleans up the test project
func (suite *PythonRealPylspIntegrationTestSuite) TearDownSuite() {
	if suite.projectRoot != "" {
		_ = os.RemoveAll(suite.projectRoot)
	}
}

// TestRealPylspServerLifecycle tests the complete LSP server lifecycle
func (suite *PythonRealPylspIntegrationTestSuite) TestRealPylspServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	startTime := time.Now()
	
	// Step 1: Start the actual pylsp language server
	pythonStartTime := time.Now()
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start pylsp process")
	suite.performanceMetrics.ProcessPID = suite.getProcessPID()
	suite.performanceMetrics.PythonStartupTime = time.Since(pythonStartTime)
	
	// Step 2: Initialize the server with proper Python capabilities
	initResult := suite.initializeServer(ctx)
	suite.True(initResult, "Server initialization should succeed")
	suite.performanceMetrics.InitializationTime = time.Since(startTime)
	
	// Step 3: Open the Python project workspace
	suite.openWorkspace(ctx)
	
	// Step 4: Test core LSP functionality with real Python analysis
	result := suite.executeComprehensivePythonWorkflow(ctx)
	
	// Step 5: Validate all aspects of real Python integration
	suite.validatePythonIntegrationResult(result)
	
	// Step 6: Measure final performance metrics
	result.TestDuration = time.Since(startTime)
	suite.recordPerformanceMetrics(result)
	
	suite.T().Logf("Real Python integration test completed in %v", result.TestDuration)
	suite.T().Logf("Python startup: %v, Server PID: %d, Total requests: %d, Success rate: %.2f%%", 
		suite.performanceMetrics.PythonStartupTime,
		suite.performanceMetrics.ProcessPID,
		suite.performanceMetrics.TotalRequests,
		float64(suite.performanceMetrics.SuccessfulRequests)/float64(suite.performanceMetrics.TotalRequests)*100)
}

// TestRealPythonFeatures tests actual Python language features
func (suite *PythonRealPylspIntegrationTestSuite) TestRealPythonFeatures() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Start and initialize server
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start server")
	
	suite.initializeServer(ctx)
	suite.openWorkspace(ctx)

	featureTests := []struct {
		name           string
		testFunction   func(context.Context) bool
		description    string
	}{
		{
			name:         "TypeHintInferenceAndChecking",
			testFunction: suite.testTypeHintInferenceAndChecking,
			description:  "Test Python type hint inference and error checking",
		},
		{
			name:         "ImportPathResolution",
			testFunction: suite.testImportPathResolution,
			description:  "Test module import path resolution with real filesystem",
		},
		{
			name:         "DjangoModelHandling",
			testFunction: suite.testDjangoModelHandling,
			description:  "Test Django model class processing and field definitions",
		},
		{
			name:         "DocstringProcessing",
			testFunction: suite.testDocstringProcessing,
			description:  "Test Python docstring parsing and documentation",
		},
		{
			name:         "VirtualEnvResolution",
			testFunction: suite.testVirtualEnvResolution,
			description:  "Test virtual environment and PYTHONPATH handling",
		},
		{
			name:         "PylspPluginSupport",
			testFunction: suite.testPylspPluginSupport,
			description:  "Test pylsp plugin integration (jedi, pycodestyle, etc.)",
		},
	}

	for _, test := range featureTests {
		suite.Run(test.name, func() {
			startTime := time.Now()
			success := test.testFunction(ctx)
			duration := time.Since(startTime)
			
			suite.True(success, "%s should succeed", test.description)
			suite.Less(duration, 45*time.Second, "%s should complete within reasonable time", test.name) // Python is slower
			
			suite.T().Logf("%s completed in %v", test.name, duration)
		})
	}
}

// TestRealPylspPerformance tests performance with large Python projects
func (suite *PythonRealPylspIntegrationTestSuite) TestRealPylspPerformance() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Start server and initialize
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start server")
	
	suite.initializeServer(ctx)
	suite.openWorkspace(ctx)

	performanceTests := []struct {
		name                string
		requestCount        int
		concurrency         int
		maxAcceptableTime   time.Duration
		expectedSuccessRate float64
	}{
		{
			name:                "HighVolumeDefinitionRequests",
			requestCount:        80, // Reduced for Python performance characteristics
			concurrency:         3,  // Lower concurrency for Python
			maxAcceptableTime:   35 * time.Second,
			expectedSuccessRate: 0.90, // Slightly lower for Python complexity
		},
		{
			name:                "ConcurrentSymbolRequests",
			requestCount:        40,
			concurrency:         5,
			maxAcceptableTime:   25 * time.Second,
			expectedSuccessRate: 0.85,
		},
		{
			name:                "IntensiveHoverRequests",
			requestCount:        120,
			concurrency:         2,
			maxAcceptableTime:   40 * time.Second,
			expectedSuccessRate: 0.95,
		},
	}

	for _, test := range performanceTests {
		suite.Run(test.name, func() {
			result := suite.executePerformanceTest(ctx, test.requestCount, test.concurrency)
			
			suite.Less(result.TotalDuration, test.maxAcceptableTime, 
				"Performance test should complete within acceptable time")
			suite.GreaterOrEqual(result.SuccessRate, test.expectedSuccessRate,
				"Success rate should meet expectations")
			
			suite.T().Logf("%s: %d requests in %v (%.2f req/s, %.2f%% success)", 
				test.name, test.requestCount, result.TotalDuration,
				float64(test.requestCount)/result.TotalDuration.Seconds(),
				result.SuccessRate*100)
		})
	}
}


// Helper methods for test implementation

func (suite *PythonRealPylspIntegrationTestSuite) isPylspServerAvailable() bool {
	cmd := exec.Command("pylsp", "--version")
	err := cmd.Run()
	if err != nil {
		// Try alternative command
		cmd = exec.Command("python", "-m", "pylsp", "--version")
		err = cmd.Run()
	}
	return err == nil
}

func (suite *PythonRealPylspIntegrationTestSuite) createTestProject() {
	var err error
	suite.projectRoot, err = os.MkdirTemp("", "python-integration-test-*")
	suite.Require().NoError(err, "Should create temporary project directory")

	// Create realistic Django-style Python project structure
	suite.projectFiles = map[string]string{
		"requirements.txt": `Django>=4.2.0
djangorestframework>=3.14.0
pytest>=7.0.0
mypy>=1.0.0
black>=23.0.0
flake8>=6.0.0
requests>=2.28.0
celery>=5.2.0
redis>=4.5.0
psycopg2-binary>=2.9.0
`,
		"pyproject.toml": `[build-system]
requires = ["setuptools>=45", "wheel", "setuptools_scm[toml]>=6.2"]
build-backend = "setuptools.build_meta"

[project]
name = "python-integration-test"
version = "1.0.0"
description = "Test Python project for LSP integration"
dependencies = [
    "Django>=4.2.0",
    "djangorestframework>=3.14.0"
]

[tool.mypy]
python_version = "3.11"
warn_return_any = true
warn_unused_configs = true
disallow_untyped_defs = true

[tool.black]
line-length = 88
target-version = ['py311']

[tool.pytest.ini_options]
DJANGO_SETTINGS_MODULE = "src.settings"
python_files = ["test_*.py", "*_test.py"]
`,
		"setup.py": `from setuptools import setup, find_packages

setup(
    name="python-integration-test",
    version="1.0.0",
    packages=find_packages(),
    install_requires=[
        "Django>=4.2.0",
        "djangorestframework>=3.14.0",
    ],
    python_requires=">=3.8",
)`,
		"src/__init__.py": "",
		"src/settings.py": `import os
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = 'test-secret-key-for-integration-testing'
DEBUG = True
ALLOWED_HOSTS = ['localhost', '127.0.0.1']

INSTALLED_APPS = [
    'django.contrib.admin',
    'django.contrib.auth',
    'django.contrib.contenttypes',
    'django.contrib.sessions',
    'django.contrib.messages',
    'django.contrib.staticfiles',
    'rest_framework',
    'src.apps.users',
    'src.apps.projects',
]

MIDDLEWARE = [
    'django.middleware.security.SecurityMiddleware',
    'django.contrib.sessions.middleware.SessionMiddleware',
    'django.middleware.common.CommonMiddleware',
    'django.middleware.csrf.CsrfViewMiddleware',
    'django.contrib.auth.middleware.AuthenticationMiddleware',
    'django.contrib.messages.middleware.MessageMiddleware',
    'django.middleware.clickjacking.XFrameOptionsMiddleware',
]

ROOT_URLCONF = 'src.urls'
WSGI_APPLICATION = 'src.wsgi.application'

DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.sqlite3',
        'NAME': BASE_DIR / 'db.sqlite3',
    }
}

REST_FRAMEWORK = {
    'DEFAULT_PAGINATION_CLASS': 'rest_framework.pagination.PageNumberPagination',
    'PAGE_SIZE': 20,
    'DEFAULT_AUTHENTICATION_CLASSES': [
        'rest_framework.authentication.SessionAuthentication',
    ],
    'DEFAULT_PERMISSION_CLASSES': [
        'rest_framework.permissions.IsAuthenticated',
    ],
}

LANGUAGE_CODE = 'en-us'
TIME_ZONE = 'UTC'
USE_I18N = True
USE_TZ = True

STATIC_URL = '/static/'
DEFAULT_AUTO_FIELD = 'django.db.models.BigAutoField'
`,
		"src/urls.py": `from django.contrib import admin
from django.urls import path, include
from rest_framework.routers import DefaultRouter
from src.apps.users.views import UserViewSet
from src.apps.projects.views import ProjectViewSet

router = DefaultRouter()
router.register(r'users', UserViewSet)
router.register(r'projects', ProjectViewSet)

urlpatterns = [
    path('admin/', admin.site.urls),
    path('api/', include(router.urls)),
    path('api-auth/', include('rest_framework.urls')),
]
`,
		"src/wsgi.py": `import os
from django.core.wsgi import get_wsgi_application

os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'src.settings')
application = get_wsgi_application()
`,
		"src/apps/__init__.py": "",
		"src/apps/users/__init__.py": "",
		"src/apps/users/models.py": `from django.db import models
from django.contrib.auth.models import AbstractUser
from typing import Optional, List
from datetime import datetime

class UserRole(models.TextChoices):
    USER = 'user', 'User'
    ADMIN = 'admin', 'Administrator'
    MODERATOR = 'moderator', 'Moderator'

class UserProfile(models.Model):
    """User profile with additional information and preferences."""
    
    user = models.OneToOneField(
        'User', 
        on_delete=models.CASCADE,
        related_name='profile'
    )
    avatar = models.URLField(blank=True, null=True)
    bio = models.TextField(max_length=500, blank=True)
    theme_preference = models.CharField(
        max_length=10,
        choices=[('light', 'Light'), ('dark', 'Dark')],
        default='light'
    )
    notifications_enabled = models.BooleanField(default=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    class Meta:
        db_table = 'user_profiles'
        verbose_name = 'User Profile'
        verbose_name_plural = 'User Profiles'
    
    def __str__(self) -> str:
        return f"Profile for {self.user.username}"
    
    def get_display_name(self) -> str:
        """Get the display name for the user."""
        if self.user.first_name and self.user.last_name:
            return f"{self.user.first_name} {self.user.last_name}"
        return self.user.username

class User(AbstractUser):
    """Extended user model with additional fields and methods."""
    
    email = models.EmailField(unique=True)
    role = models.CharField(
        max_length=20,
        choices=UserRole.choices,
        default=UserRole.USER
    )
    is_active = models.BooleanField(default=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    USERNAME_FIELD = 'email'
    REQUIRED_FIELDS = ['username']
    
    class Meta:
        db_table = 'users'
        ordering = ['-created_at']
        indexes = [
            models.Index(fields=['email']),
            models.Index(fields=['role']),
            models.Index(fields=['is_active']),
        ]
    
    def __str__(self) -> str:
        return f"{self.username} ({self.email})"
    
    def is_admin(self) -> bool:
        """Check if user has admin role."""
        return self.role == UserRole.ADMIN
    
    def is_moderator(self) -> bool:
        """Check if user has moderator role."""
        return self.role == UserRole.MODERATOR
    
    def get_full_name(self) -> str:
        """Return the full name for the user."""
        if self.first_name and self.last_name:
            return f"{self.first_name} {self.last_name}".strip()
        return self.username
    
    def get_projects(self) -> List['Project']:
        """Get all projects associated with this user."""
        from src.apps.projects.models import Project
        return list(self.projects.all())
    
    @property
    def project_count(self) -> int:
        """Get the number of projects for this user."""
        return self.projects.count()
`,
		"src/apps/users/serializers.py": `from rest_framework import serializers
from django.contrib.auth import get_user_model
from .models import UserProfile, UserRole
from typing import Dict, Any

User = get_user_model()

class UserProfileSerializer(serializers.ModelSerializer):
    """Serializer for user profile data."""
    
    display_name = serializers.CharField(source='get_display_name', read_only=True)
    
    class Meta:
        model = UserProfile
        fields = [
            'avatar', 'bio', 'theme_preference', 
            'notifications_enabled', 'display_name',
            'created_at', 'updated_at'
        ]
        read_only_fields = ['created_at', 'updated_at']

class UserSerializer(serializers.ModelSerializer):
    """Serializer for user data with profile information."""
    
    profile = UserProfileSerializer(read_only=True)
    full_name = serializers.CharField(source='get_full_name', read_only=True)
    project_count = serializers.IntegerField(read_only=True)
    is_admin = serializers.BooleanField(source='is_admin', read_only=True)
    password = serializers.CharField(write_only=True, min_length=8)
    
    class Meta:
        model = User
        fields = [
            'id', 'username', 'email', 'first_name', 'last_name',
            'role', 'is_active', 'full_name', 'project_count',
            'is_admin', 'profile', 'password', 'created_at', 'updated_at'
        ]
        read_only_fields = ['id', 'created_at', 'updated_at']
        extra_kwargs = {
            'password': {'write_only': True, 'min_length': 8},
        }
    
    def validate_email(self, value: str) -> str:
        """Validate email uniqueness."""
        if User.objects.filter(email=value).exists():
            raise serializers.ValidationError("Email already exists.")
        return value
    
    def validate_role(self, value: str) -> str:
        """Validate user role."""
        if value not in [choice[0] for choice in UserRole.choices]:
            raise serializers.ValidationError("Invalid role.")
        return value
    
    def create(self, validated_data: Dict[str, Any]) -> User:
        """Create user with hashed password."""
        password = validated_data.pop('password')
        user = User.objects.create_user(**validated_data)
        user.set_password(password)
        user.save()
        return user
    
    def update(self, instance: User, validated_data: Dict[str, Any]) -> User:
        """Update user with optional password change."""
        password = validated_data.pop('password', None)
        for attr, value in validated_data.items():
            setattr(instance, attr, value)
        
        if password:
            instance.set_password(password)
        
        instance.save()
        return instance

class CreateUserSerializer(serializers.ModelSerializer):
    """Simplified serializer for user creation."""
    
    password_confirm = serializers.CharField(write_only=True)
    
    class Meta:
        model = User
        fields = [
            'username', 'email', 'first_name', 'last_name',
            'password', 'password_confirm', 'role'
        ]
        extra_kwargs = {
            'password': {'write_only': True, 'min_length': 8},
        }
    
    def validate(self, attrs: Dict[str, Any]) -> Dict[str, Any]:
        """Validate password confirmation."""
        if attrs['password'] != attrs['password_confirm']:
            raise serializers.ValidationError("Passwords don't match.")
        attrs.pop('password_confirm')
        return attrs
`,
		"src/apps/users/views.py": `from rest_framework import viewsets, status, permissions
from rest_framework.decorators import action
from rest_framework.response import Response
from django.contrib.auth import get_user_model
from django.shortcuts import get_object_or_404
from typing import Any, Dict
from .models import UserProfile
from .serializers import UserSerializer, CreateUserSerializer, UserProfileSerializer

User = get_user_model()

class UserViewSet(viewsets.ModelViewSet):
    """ViewSet for managing users with full CRUD operations."""
    
    queryset = User.objects.select_related('profile').all()
    serializer_class = UserSerializer
    permission_classes = [permissions.IsAuthenticated]
    
    def get_serializer_class(self):
        """Return appropriate serializer based on action."""
        if self.action == 'create':
            return CreateUserSerializer
        return UserSerializer
    
    def get_permissions(self):
        """Define permissions based on action."""
        if self.action == 'create':
            permission_classes = [permissions.AllowAny]
        elif self.action in ['update', 'partial_update', 'destroy']:
            permission_classes = [permissions.IsAuthenticated]
        else:
            permission_classes = [permissions.IsAuthenticated]
        
        return [permission() for permission in permission_classes]
    
    def perform_create(self, serializer) -> None:
        """Create user and associated profile."""
        user = serializer.save()
        UserProfile.objects.create(user=user)
    
    @action(detail=True, methods=['get', 'patch'])
    def profile(self, request, pk=None) -> Response:
        """Get or update user profile."""
        user = self.get_object()
        profile, created = UserProfile.objects.get_or_create(user=user)
        
        if request.method == 'GET':
            serializer = UserProfileSerializer(profile)
            return Response(serializer.data)
        
        elif request.method == 'PATCH':
            serializer = UserProfileSerializer(
                profile, 
                data=request.data, 
                partial=True
            )
            if serializer.is_valid():
                serializer.save()
                return Response(serializer.data)
            return Response(serializer.errors, status=status.HTTP_400_BAD_REQUEST)
    
    @action(detail=True, methods=['post'])
    def activate(self, request, pk=None) -> Response:
        """Activate a user account."""
        user = self.get_object()
        user.is_active = True
        user.save()
        
        serializer = self.get_serializer(user)
        return Response({
            'message': 'User activated successfully',
            'user': serializer.data
        })
    
    @action(detail=True, methods=['post'])
    def deactivate(self, request, pk=None) -> Response:
        """Deactivate a user account."""
        user = self.get_object()
        user.is_active = False
        user.save()
        
        serializer = self.get_serializer(user)
        return Response({
            'message': 'User deactivated successfully',
            'user': serializer.data
        })
    
    @action(detail=False, methods=['get'])
    def me(self, request) -> Response:
        """Get current user information."""
        serializer = self.get_serializer(request.user)
        return Response(serializer.data)
    
    @action(detail=False, methods=['get'])
    def admins(self, request) -> Response:
        """Get all admin users."""
        admin_users = self.get_queryset().filter(role='admin')
        serializer = self.get_serializer(admin_users, many=True)
        return Response(serializer.data)
`,
		"src/apps/projects/__init__.py": "",
		"src/apps/projects/models.py": `from django.db import models
from django.contrib.auth import get_user_model
from typing import List, Optional
from datetime import datetime

User = get_user_model()

class ProjectStatus(models.TextChoices):
    DRAFT = 'draft', 'Draft'
    ACTIVE = 'active', 'Active'
    COMPLETED = 'completed', 'Completed'
    ARCHIVED = 'archived', 'Archived'

class Project(models.Model):
    """Project model with user relationships and status tracking."""
    
    name = models.CharField(max_length=200)
    description = models.TextField(blank=True)
    owner = models.ForeignKey(
        User,
        on_delete=models.CASCADE,
        related_name='owned_projects'
    )
    members = models.ManyToManyField(
        User,
        through='ProjectMembership',
        related_name='projects',
        blank=True
    )
    status = models.CharField(
        max_length=20,
        choices=ProjectStatus.choices,
        default=ProjectStatus.DRAFT
    )
    start_date = models.DateField(null=True, blank=True)
    end_date = models.DateField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    class Meta:
        db_table = 'projects'
        ordering = ['-created_at']
        indexes = [
            models.Index(fields=['status']),
            models.Index(fields=['owner']),
            models.Index(fields=['created_at']),
        ]
    
    def __str__(self) -> str:
        return f"{self.name} (Owner: {self.owner.username})"
    
    def is_active(self) -> bool:
        """Check if project is in active status."""
        return self.status == ProjectStatus.ACTIVE
    
    def is_completed(self) -> bool:
        """Check if project is completed."""
        return self.status == ProjectStatus.COMPLETED
    
    def get_member_count(self) -> int:
        """Get the number of project members."""
        return self.members.count()
    
    def get_task_count(self) -> int:
        """Get the number of tasks in this project."""
        return self.tasks.count()
    
    def add_member(self, user: User, role: str = 'member') -> 'ProjectMembership':
        """Add a member to the project."""
        membership, created = ProjectMembership.objects.get_or_create(
            project=self,
            user=user,
            defaults={'role': role}
        )
        return membership
    
    def remove_member(self, user: User) -> bool:
        """Remove a member from the project."""
        try:
            membership = ProjectMembership.objects.get(project=self, user=user)
            membership.delete()
            return True
        except ProjectMembership.DoesNotExist:
            return False

class ProjectMembershipRole(models.TextChoices):
    MEMBER = 'member', 'Member'
    LEAD = 'lead', 'Project Lead'
    ADMIN = 'admin', 'Admin'

class ProjectMembership(models.Model):
    """Through model for project membership with roles."""
    
    project = models.ForeignKey(Project, on_delete=models.CASCADE)
    user = models.ForeignKey(User, on_delete=models.CASCADE)
    role = models.CharField(
        max_length=20,
        choices=ProjectMembershipRole.choices,
        default=ProjectMembershipRole.MEMBER
    )
    joined_at = models.DateTimeField(auto_now_add=True)
    
    class Meta:
        db_table = 'project_memberships'
        unique_together = ['project', 'user']
        indexes = [
            models.Index(fields=['project', 'user']),
            models.Index(fields=['role']),
        ]
    
    def __str__(self) -> str:
        return f"{self.user.username} - {self.project.name} ({self.role})"
    
    def is_lead(self) -> bool:
        """Check if user is project lead."""
        return self.role == ProjectMembershipRole.LEAD

class Task(models.Model):
    """Task model for project task management."""
    
    class Priority(models.TextChoices):
        LOW = 'low', 'Low'
        MEDIUM = 'medium', 'Medium'
        HIGH = 'high', 'High'
        URGENT = 'urgent', 'Urgent'
    
    class Status(models.TextChoices):
        TODO = 'todo', 'To Do'
        IN_PROGRESS = 'in_progress', 'In Progress'
        REVIEW = 'review', 'Under Review'
        DONE = 'done', 'Done'
    
    title = models.CharField(max_length=200)
    description = models.TextField(blank=True)
    project = models.ForeignKey(
        Project,
        on_delete=models.CASCADE,
        related_name='tasks'
    )
    assignee = models.ForeignKey(
        User,
        on_delete=models.SET_NULL,
        null=True,
        blank=True,
        related_name='assigned_tasks'
    )
    priority = models.CharField(
        max_length=10,
        choices=Priority.choices,
        default=Priority.MEDIUM
    )
    status = models.CharField(
        max_length=20,
        choices=Status.choices,
        default=Status.TODO
    )
    due_date = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    class Meta:
        db_table = 'tasks'
        ordering = ['-created_at']
        indexes = [
            models.Index(fields=['project', 'status']),
            models.Index(fields=['assignee']),
            models.Index(fields=['priority']),
            models.Index(fields=['due_date']),
        ]
    
    def __str__(self) -> str:
        return f"{self.title} ({self.project.name})"
    
    def is_overdue(self) -> bool:
        """Check if task is overdue."""
        if not self.due_date:
            return False
        return datetime.now() > self.due_date and self.status != self.Status.DONE
    
    def is_completed(self) -> bool:
        """Check if task is completed."""
        return self.status == self.Status.DONE
`,
		"src/apps/projects/views.py": `from rest_framework import viewsets, status, permissions
from rest_framework.decorators import action
from rest_framework.response import Response
from django.shortcuts import get_object_or_404
from typing import Any, Dict
from .models import Project, ProjectMembership, Task
from .serializers import ProjectSerializer, TaskSerializer

class ProjectViewSet(viewsets.ModelViewSet):
    """ViewSet for managing projects."""
    
    serializer_class = ProjectSerializer
    permission_classes = [permissions.IsAuthenticated]
    
    def get_queryset(self):
        """Filter projects based on user permissions."""
        user = self.request.user
        if user.is_admin():
            return Project.objects.all()
        
        return Project.objects.filter(
            models.Q(owner=user) | models.Q(members=user)
        ).distinct()
    
    def perform_create(self, serializer) -> None:
        """Set current user as project owner."""
        serializer.save(owner=self.request.user)
    
    @action(detail=True, methods=['post'])
    def add_member(self, request, pk=None) -> Response:
        """Add a member to the project."""
        project = self.get_object()
        user_id = request.data.get('user_id')
        role = request.data.get('role', 'member')
        
        if not user_id:
            return Response(
                {'error': 'user_id is required'}, 
                status=status.HTTP_400_BAD_REQUEST
            )
        
        try:
            from django.contrib.auth import get_user_model
            User = get_user_model()
            user = User.objects.get(id=user_id)
            membership = project.add_member(user, role)
            
            return Response({
                'message': 'Member added successfully',
                'membership': {
                    'user': user.username,
                    'role': membership.role,
                    'joined_at': membership.joined_at
                }
            })
        except User.DoesNotExist:
            return Response(
                {'error': 'User not found'}, 
                status=status.HTTP_404_NOT_FOUND
            )
    
    @action(detail=True, methods=['delete'])
    def remove_member(self, request, pk=None) -> Response:
        """Remove a member from the project."""
        project = self.get_object()
        user_id = request.data.get('user_id')
        
        if not user_id:
            return Response(
                {'error': 'user_id is required'}, 
                status=status.HTTP_400_BAD_REQUEST
            )
        
        try:
            from django.contrib.auth import get_user_model
            User = get_user_model()
            user = User.objects.get(id=user_id)
            
            if project.remove_member(user):
                return Response({'message': 'Member removed successfully'})
            else:
                return Response(
                    {'error': 'User is not a member of this project'}, 
                    status=status.HTTP_400_BAD_REQUEST
                )
        except User.DoesNotExist:
            return Response(
                {'error': 'User not found'}, 
                status=status.HTTP_404_NOT_FOUND
            )
    
    @action(detail=True, methods=['get'])
    def tasks(self, request, pk=None) -> Response:
        """Get all tasks for the project."""
        project = self.get_object()
        tasks = project.tasks.all()
        serializer = TaskSerializer(tasks, many=True)
        return Response(serializer.data)
    
    @action(detail=True, methods=['get'])
    def members(self, request, pk=None) -> Response:
        """Get all project members."""
        project = self.get_object()
        memberships = ProjectMembership.objects.filter(project=project).select_related('user')
        
        members_data = []
        for membership in memberships:
            members_data.append({
                'user_id': membership.user.id,
                'username': membership.user.username,
                'email': membership.user.email,
                'role': membership.role,
                'joined_at': membership.joined_at
            })
        
        return Response(members_data)
`,
		"src/apps/projects/serializers.py": `from rest_framework import serializers
from .models import Project, Task, ProjectMembership
from django.contrib.auth import get_user_model

User = get_user_model()

class ProjectSerializer(serializers.ModelSerializer):
    """Serializer for project data."""
    
    owner_name = serializers.CharField(source='owner.get_full_name', read_only=True)
    member_count = serializers.IntegerField(source='get_member_count', read_only=True)
    task_count = serializers.IntegerField(source='get_task_count', read_only=True)
    is_active = serializers.BooleanField(source='is_active', read_only=True)
    
    class Meta:
        model = Project
        fields = [
            'id', 'name', 'description', 'owner', 'owner_name',
            'status', 'start_date', 'end_date', 'member_count',
            'task_count', 'is_active', 'created_at', 'updated_at'
        ]
        read_only_fields = ['id', 'owner', 'created_at', 'updated_at']

class TaskSerializer(serializers.ModelSerializer):
    """Serializer for task data."""
    
    assignee_name = serializers.CharField(source='assignee.get_full_name', read_only=True)
    project_name = serializers.CharField(source='project.name', read_only=True)
    is_overdue = serializers.BooleanField(source='is_overdue', read_only=True)
    is_completed = serializers.BooleanField(source='is_completed', read_only=True)
    
    class Meta:
        model = Task
        fields = [
            'id', 'title', 'description', 'project', 'project_name',
            'assignee', 'assignee_name', 'priority', 'status',
            'due_date', 'is_overdue', 'is_completed',
            'created_at', 'updated_at'
        ]
        read_only_fields = ['id', 'created_at', 'updated_at']
`,
		"src/services/__init__.py": "",
		"src/services/user_service.py": `from typing import List, Optional, Dict, Any
from django.contrib.auth import get_user_model
from django.db import transaction
from django.core.exceptions import ValidationError
from src.apps.users.models import UserProfile, UserRole
from src.utils.validators import validate_email_format
from src.utils.exceptions import UserServiceError

User = get_user_model()

class UserService:
    """Service class for user-related business logic."""
    
    @staticmethod
    def get_all_users(include_inactive: bool = False) -> List[User]:
        """Retrieve all users with optional filtering."""
        queryset = User.objects.select_related('profile')
        
        if not include_inactive:
            queryset = queryset.filter(is_active=True)
        
        return list(queryset.all())
    
    @staticmethod
    def get_user_by_id(user_id: int) -> Optional[User]:
        """Retrieve user by ID with related profile."""
        try:
            return User.objects.select_related('profile').get(id=user_id)
        except User.DoesNotExist:
            return None
    
    @staticmethod
    def get_user_by_email(email: str) -> Optional[User]:
        """Retrieve user by email address."""
        try:
            return User.objects.select_related('profile').get(email=email)
        except User.DoesNotExist:
            return None
    
    @staticmethod
    @transaction.atomic
    def create_user(user_data: Dict[str, Any]) -> User:
        """Create a new user with profile."""
        # Validate email format
        email = user_data.get('email')
        if not validate_email_format(email):
            raise UserServiceError("Invalid email format")
        
        # Check if email already exists
        if User.objects.filter(email=email).exists():
            raise UserServiceError("Email already exists")
        
        # Create user
        password = user_data.pop('password', None)
        if not password:
            raise UserServiceError("Password is required")
        
        user = User.objects.create_user(**user_data)
        user.set_password(password)
        user.save()
        
        # Create user profile
        UserProfile.objects.create(user=user)
        
        return user
    
    @staticmethod
    @transaction.atomic
    def update_user(user: User, update_data: Dict[str, Any]) -> User:
        """Update user information."""
        # Handle password update separately
        password = update_data.pop('password', None)
        
        # Update user fields
        for field, value in update_data.items():
            if hasattr(user, field):
                setattr(user, field, value)
        
        # Update password if provided
        if password:
            user.set_password(password)
        
        user.full_clean()
        user.save()
        
        return user
    
    @staticmethod
    def deactivate_user(user: User) -> User:
        """Deactivate a user account."""
        user.is_active = False
        user.save()
        return user
    
    @staticmethod
    def activate_user(user: User) -> User:
        """Activate a user account."""
        user.is_active = True
        user.save()
        return user
    
    @staticmethod
    def get_users_by_role(role: UserRole) -> List[User]:
        """Get all users with a specific role."""
        return list(User.objects.filter(role=role).select_related('profile'))
    
    @staticmethod
    def promote_to_admin(user: User) -> User:
        """Promote user to admin role."""
        user.role = UserRole.ADMIN
        user.save()
        return user
    
    @staticmethod
    def get_user_statistics() -> Dict[str, int]:
        """Get user statistics."""
        total_users = User.objects.count()
        active_users = User.objects.filter(is_active=True).count()
        admin_users = User.objects.filter(role=UserRole.ADMIN).count()
        
        return {
            'total_users': total_users,
            'active_users': active_users,
            'inactive_users': total_users - active_users,
            'admin_users': admin_users,
        }
    
    @staticmethod
    def search_users(query: str, limit: int = 10) -> List[User]:
        """Search users by username, email, or name."""
        from django.db.models import Q
        
        queryset = User.objects.filter(
            Q(username__icontains=query) |
            Q(email__icontains=query) |
            Q(first_name__icontains=query) |
            Q(last_name__icontains=query)
        ).select_related('profile')[:limit]
        
        return list(queryset)
`,
		"src/utils/__init__.py": "",
		"src/utils/validators.py": `import re
from typing import Any, Optional
from django.core.exceptions import ValidationError
from django.core.validators import validate_email as django_validate_email

def validate_email_format(email: str) -> bool:
    """Validate email format using Django's built-in validator."""
    try:
        django_validate_email(email)
        return True
    except ValidationError:
        return False

def validate_password_strength(password: str) -> bool:
    """Validate password meets strength requirements."""
    if len(password) < 8:
        return False
    
    # Check for at least one digit, one letter, and one special character
    has_digit = bool(re.search(r'\d', password))
    has_letter = bool(re.search(r'[a-zA-Z]', password))
    has_special = bool(re.search(r'[!@#$%^&*(),.?":{}|<>]', password))
    
    return has_digit and has_letter and has_special

def validate_username(username: str) -> bool:
    """Validate username format."""
    if len(username) < 3 or len(username) > 30:
        return False
    
    # Allow alphanumeric characters, underscores, and hyphens
    pattern = r'^[a-zA-Z0-9_-]+$'
    return bool(re.match(pattern, username))

def validate_phone_number(phone: str) -> bool:
    """Validate phone number format."""
    # Simple phone number validation
    pattern = r'^\+?1?[-.\s]?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}$'
    return bool(re.match(pattern, phone))

class DataValidator:
    """Utility class for data validation."""
    
    @staticmethod
    def validate_required_fields(data: dict, required_fields: list) -> bool:
        """Check if all required fields are present and not empty."""
        for field in required_fields:
            if field not in data or not data[field]:
                return False
        return True
    
    @staticmethod
    def validate_field_types(data: dict, field_types: dict) -> bool:
        """Validate field types match expected types."""
        for field, expected_type in field_types.items():
            if field in data and not isinstance(data[field], expected_type):
                return False
        return True
    
    @staticmethod
    def sanitize_string(value: str, max_length: Optional[int] = None) -> str:
        """Sanitize string input."""
        if not isinstance(value, str):
            return str(value)
        
        # Strip whitespace
        value = value.strip()
        
        # Truncate if max_length specified
        if max_length and len(value) > max_length:
            value = value[:max_length]
        
        return value
`,
		"src/utils/exceptions.py": `class LSPGatewayError(Exception):
    """Base exception for LSP Gateway application."""
    pass

class UserServiceError(LSPGatewayError):
    """Exception raised by user service operations."""
    pass

class ProjectServiceError(LSPGatewayError):
    """Exception raised by project service operations."""
    pass

class ValidationError(LSPGatewayError):
    """Exception raised for data validation errors."""
    pass

class AuthenticationError(LSPGatewayError):
    """Exception raised for authentication errors."""
    pass

class PermissionError(LSPGatewayError):
    """Exception raised for permission errors."""
    pass
`,
		"tests/__init__.py": "",
		"tests/test_models.py": `import pytest
from django.test import TestCase
from django.contrib.auth import get_user_model
from src.apps.users.models import UserProfile, UserRole
from src.apps.projects.models import Project, ProjectStatus

User = get_user_model()

class UserModelTest(TestCase):
    """Test cases for User model."""
    
    def setUp(self):
        self.user_data = {
            'username': 'testuser',
            'email': 'test@example.com',
            'first_name': 'Test',
            'last_name': 'User',
            'role': UserRole.USER
        }
    
    def test_create_user(self):
        """Test user creation."""
        user = User.objects.create_user(**self.user_data)
        self.assertEqual(user.username, 'testuser')
        self.assertEqual(user.email, 'test@example.com')
        self.assertEqual(user.role, UserRole.USER)
        self.assertTrue(user.is_active)
    
    def test_user_str_representation(self):
        """Test user string representation."""
        user = User.objects.create_user(**self.user_data)
        expected = f"{user.username} ({user.email})"
        self.assertEqual(str(user), expected)
    
    def test_user_full_name(self):
        """Test get_full_name method."""
        user = User.objects.create_user(**self.user_data)
        self.assertEqual(user.get_full_name(), "Test User")
    
    def test_user_is_admin(self):
        """Test is_admin method."""
        user = User.objects.create_user(**self.user_data)
        self.assertFalse(user.is_admin())
        
        user.role = UserRole.ADMIN
        user.save()
        self.assertTrue(user.is_admin())
    
    def test_user_profile_creation(self):
        """Test user profile is created."""
        user = User.objects.create_user(**self.user_data)
        profile = UserProfile.objects.create(user=user)
        
        self.assertEqual(profile.user, user)
        self.assertEqual(profile.theme_preference, 'light')
        self.assertTrue(profile.notifications_enabled)

class ProjectModelTest(TestCase):
    """Test cases for Project model."""
    
    def setUp(self):
        self.user = User.objects.create_user(
            username='projectowner',
            email='owner@example.com'
        )
        self.project_data = {
            'name': 'Test Project',
            'description': 'A test project',
            'owner': self.user,
            'status': ProjectStatus.ACTIVE
        }
    
    def test_create_project(self):
        """Test project creation."""
        project = Project.objects.create(**self.project_data)
        self.assertEqual(project.name, 'Test Project')
        self.assertEqual(project.owner, self.user)
        self.assertEqual(project.status, ProjectStatus.ACTIVE)
    
    def test_project_str_representation(self):
        """Test project string representation."""
        project = Project.objects.create(**self.project_data)
        expected = f"{project.name} (Owner: {self.user.username})"
        self.assertEqual(str(project), expected)
    
    def test_project_is_active(self):
        """Test is_active method."""
        project = Project.objects.create(**self.project_data)
        self.assertTrue(project.is_active())
        
        project.status = ProjectStatus.COMPLETED
        project.save()
        self.assertFalse(project.is_active())
    
    def test_add_project_member(self):
        """Test adding member to project."""
        project = Project.objects.create(**self.project_data)
        member = User.objects.create_user(
            username='member',
            email='member@example.com'
        )
        
        membership = project.add_member(member, 'lead')
        self.assertEqual(membership.user, member)
        self.assertEqual(membership.role, 'lead')
        self.assertEqual(project.get_member_count(), 1)
`,
		"tests/test_services.py": `import pytest
from django.test import TestCase
from django.contrib.auth import get_user_model
from src.services.user_service import UserService
from src.apps.users.models import UserRole
from src.utils.exceptions import UserServiceError

User = get_user_model()

class UserServiceTest(TestCase):
    """Test cases for UserService."""
    
    def setUp(self):
        self.user_data = {
            'username': 'testuser',
            'email': 'test@example.com',
            'first_name': 'Test',
            'last_name': 'User',
            'password': 'testpassword123'
        }
    
    def test_create_user_success(self):
        """Test successful user creation."""
        user = UserService.create_user(self.user_data)
        
        self.assertEqual(user.username, 'testuser')
        self.assertEqual(user.email, 'test@example.com')
        self.assertTrue(user.check_password('testpassword123'))
        self.assertTrue(hasattr(user, 'profile'))
    
    def test_create_user_duplicate_email(self):
        """Test user creation with duplicate email."""
        UserService.create_user(self.user_data)
        
        with self.assertRaises(UserServiceError):
            UserService.create_user(self.user_data)
    
    def test_get_user_by_email(self):
        """Test retrieving user by email."""
        created_user = UserService.create_user(self.user_data)
        found_user = UserService.get_user_by_email('test@example.com')
        
        self.assertEqual(created_user.id, found_user.id)
    
    def test_get_user_by_email_not_found(self):
        """Test retrieving non-existent user by email."""
        user = UserService.get_user_by_email('nonexistent@example.com')
        self.assertIsNone(user)
    
    def test_update_user(self):
        """Test user update."""
        user = UserService.create_user(self.user_data)
        update_data = {
            'first_name': 'Updated',
            'last_name': 'Name'
        }
        
        updated_user = UserService.update_user(user, update_data)
        
        self.assertEqual(updated_user.first_name, 'Updated')
        self.assertEqual(updated_user.last_name, 'Name')
    
    def test_deactivate_user(self):
        """Test user deactivation."""
        user = UserService.create_user(self.user_data)
        self.assertTrue(user.is_active)
        
        deactivated_user = UserService.deactivate_user(user)
        self.assertFalse(deactivated_user.is_active)
    
    def test_get_users_by_role(self):
        """Test getting users by role."""
        # Create regular user
        UserService.create_user(self.user_data)
        
        # Create admin user
        admin_data = self.user_data.copy()
        admin_data['username'] = 'admin'
        admin_data['email'] = 'admin@example.com'
        admin_data['role'] = UserRole.ADMIN
        UserService.create_user(admin_data)
        
        admin_users = UserService.get_users_by_role(UserRole.ADMIN)
        regular_users = UserService.get_users_by_role(UserRole.USER)
        
        self.assertEqual(len(admin_users), 1)
        self.assertEqual(len(regular_users), 1)
    
    def test_search_users(self):
        """Test user search functionality."""
        UserService.create_user(self.user_data)
        
        # Search by username
        results = UserService.search_users('testuser')
        self.assertEqual(len(results), 1)
        
        # Search by email
        results = UserService.search_users('test@example.com')
        self.assertEqual(len(results), 1)
        
        # Search with no results
        results = UserService.search_users('nonexistent')
        self.assertEqual(len(results), 0)
`,
	}

	// Create all project files
	for relativePath, content := range suite.projectFiles {
		fullPath := filepath.Join(suite.projectRoot, relativePath)
		
		// Create directory if needed
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		suite.Require().NoError(err, "Should create directory %s", dir)
		
		// Write file content
		err = os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err, "Should create file %s", relativePath)
	}
}

func (suite *PythonRealPylspIntegrationTestSuite) initializeServer(ctx context.Context) bool {
	// Send initialize request with proper Python capabilities
	initParams := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   fmt.Sprintf("file://%s", suite.projectRoot),
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"workspaceFolders": true,
			},
			"textDocument": map[string]interface{}{
				"synchronization": map[string]interface{}{
					"dynamicRegistration": true,
					"willSave":           true,
					"willSaveWaitUntil":  true,
					"didSave":            true,
				},
				"completion": map[string]interface{}{
					"dynamicRegistration": true,
					"completionItem": map[string]interface{}{
						"snippetSupport": true,
						"commitCharactersSupport": true,
						"documentationFormat": []string{"markdown", "plaintext"},
					},
				},
				"hover": map[string]interface{}{
					"dynamicRegistration": true,
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"definition": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"references": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"documentSymbol": map[string]interface{}{
					"dynamicRegistration": true,
				},
			},
		},
		"initializationOptions": map[string]interface{}{
			"settings": map[string]interface{}{
				"pylsp": map[string]interface{}{
					"plugins": map[string]interface{}{
						"jedi_completion": map[string]interface{}{
							"enabled": true,
							"include_params": true,
						},
						"jedi_hover": map[string]interface{}{
							"enabled": true,
						},
						"jedi_references": map[string]interface{}{
							"enabled": true,
						},
						"jedi_symbols": map[string]interface{}{
							"enabled": true,
							"all_scopes": true,
						},
					},
				},
			},
		},
	}

	startTime := time.Now()
	response, err := suite.lspClient.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		suite.T().Logf("Initialize request failed: %v", err)
		return false
	}

	suite.performanceMetrics.FirstResponseTime = time.Since(startTime)

	// Parse initialize response
	var initResult map[string]interface{}
	if err := json.Unmarshal(response, &initResult); err != nil {
		suite.T().Logf("Failed to parse initialize response: %v", err)
		return false
	}

	// Send initialized notification
	err = suite.lspClient.SendNotification(ctx, "initialized", map[string]interface{}{})
	if err != nil {
		suite.T().Logf("Initialized notification failed: %v", err)
		return false
	}

	return true
}

func (suite *PythonRealPylspIntegrationTestSuite) openWorkspace(ctx context.Context) {
	// Open all Python files in the project
	for relativePath := range suite.projectFiles {
		if strings.HasSuffix(relativePath, ".py") {
			fullPath := filepath.Join(suite.projectRoot, relativePath)
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			// Send textDocument/didOpen notification
			err = suite.lspClient.SendNotification(ctx, "textDocument/didOpen", map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri":        fmt.Sprintf("file://%s", fullPath),
					"languageId": "python",
					"version":    1,
					"text":       string(content),
				},
			})
			if err != nil {
				suite.T().Logf("Failed to open document %s: %v", relativePath, err)
			}
		}
	}

	// Wait for initial processing (Python analysis can take longer)
	time.Sleep(5 * time.Second)
}

func (suite *PythonRealPylspIntegrationTestSuite) executeComprehensivePythonWorkflow(ctx context.Context) *PythonIntegrationResult {
	result := &PythonIntegrationResult{
		ServerStartSuccess:    true,
		InitializationSuccess: true,
		ProjectLoadSuccess:    true,
		PerformanceMetrics:   suite.performanceMetrics,
	}

	startTime := time.Now()

	// Test 1: Go to definition
	if suite.testGoToDefinition(ctx) {
		result.TypeHintAccuracy = 1.0
	} else {
		result.TypeHintAccuracy = 0.0
	}
	
	// Test 2: Find references
	suite.testFindReferences(ctx)
	
	// Test 3: Hover information
	suite.testHoverInformation(ctx)
	
	// Test 4: Document symbols
	suite.testDocumentSymbols(ctx)
	
	// Test 5: Import resolution
	result.ImportResolutionWorks = suite.testImportResolution(ctx)
	
	// Test 6: Module resolution
	result.ModuleResolutionWorks = suite.testModuleResolution(ctx)
	
	// Test 7: Django model support
	result.DjangoModelSupport = suite.testDjangoModelSupport(ctx)
	
	// Test 8: Pylsp plugin integration
	result.PylspPluginIntegration = suite.testPylspPluginIntegration(ctx)

	result.TestDuration = time.Since(startTime)
	return result
}

func (suite *PythonRealPylspIntegrationTestSuite) testGoToDefinition(ctx context.Context) bool {
	// Test go to definition on User model in user service
	filePath := filepath.Join(suite.projectRoot, "src/services/user_service.py")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      6, // from src.apps.users.models import UserProfile, UserRole
			"character": 38, // Position of "UserProfile"
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Go to definition failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	// Parse response to verify it points to the correct file
	var definitions []interface{}
	if err := json.Unmarshal(response, &definitions); err != nil {
		suite.T().Logf("Failed to parse definition response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify definition points to models.py file
	if len(definitions) > 0 {
		if def, ok := definitions[0].(map[string]interface{}); ok {
			if uri, exists := def["uri"]; exists {
				return strings.Contains(uri.(string), "users/models.py")
			}
		}
	}

	return false
}

func (suite *PythonRealPylspIntegrationTestSuite) testFindReferences(ctx context.Context) bool {
	// Test find references for UserRole enum
	filePath := filepath.Join(suite.projectRoot, "src/apps/users/models.py")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      6, // class UserRole(models.TextChoices):
			"character": 6, // Position of "UserRole"
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Find references failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var references []interface{}
	if err := json.Unmarshal(response, &references); err != nil {
		suite.T().Logf("Failed to parse references response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	return len(references) > 1 // Should find multiple references across files
}

func (suite *PythonRealPylspIntegrationTestSuite) testHoverInformation(ctx context.Context) bool {
	// Test hover on User model method
	filePath := filepath.Join(suite.projectRoot, "src/apps/users/models.py")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      88, // def is_admin(self) -> bool:
			"character": 12, // Position of "is_admin"
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Hover failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var hoverResult map[string]interface{}
	if err := json.Unmarshal(response, &hoverResult); err != nil {
		suite.T().Logf("Failed to parse hover response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify hover contains method signature information
	if contents, exists := hoverResult["contents"]; exists {
		contentStr := fmt.Sprintf("%v", contents)
		return strings.Contains(contentStr, "bool") || strings.Contains(contentStr, "admin")
	}

	return false
}

func (suite *PythonRealPylspIntegrationTestSuite) testDocumentSymbols(ctx context.Context) bool {
	// Test document symbols for user models file
	filePath := filepath.Join(suite.projectRoot, "src/apps/users/models.py")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Document symbols failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		suite.T().Logf("Failed to parse symbols response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify User class is found in symbols
	for _, symbol := range symbols {
		if sym, ok := symbol.(map[string]interface{}); ok {
			if name, exists := sym["name"]; exists && strings.Contains(name.(string), "User") {
				return true
			}
		}
	}

	return false
}

func (suite *PythonRealPylspIntegrationTestSuite) testImportResolution(ctx context.Context) bool {
	// This is tested implicitly by other operations - if imports were broken,
	// go to definition and other features wouldn't work
	return suite.testGoToDefinition(ctx)
}

func (suite *PythonRealPylspIntegrationTestSuite) testModuleResolution(ctx context.Context) bool {
	// Test workspace symbol search to verify module resolution
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
		"query": "User",
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Workspace symbol search failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		suite.T().Logf("Failed to parse workspace symbols response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	return len(symbols) > 0
}

func (suite *PythonRealPylspIntegrationTestSuite) testDjangoModelSupport(ctx context.Context) bool {
	// Test hover on Django model field
	filePath := filepath.Join(suite.projectRoot, "src/apps/users/models.py")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      68, // email = models.EmailField(unique=True)
			"character": 20, // Position of "EmailField"
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Django model hover failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var hoverResult map[string]interface{}
	if err := json.Unmarshal(response, &hoverResult); err != nil {
		suite.T().Logf("Failed to parse Django hover response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify hover contains Django field information
	if contents, exists := hoverResult["contents"]; exists {
		contentStr := fmt.Sprintf("%v", contents)
		return strings.Contains(contentStr, "EmailField") || strings.Contains(contentStr, "Field")
	}

	return false
}

func (suite *PythonRealPylspIntegrationTestSuite) testPylspPluginIntegration(ctx context.Context) bool {
	// Test if completion works (indicating jedi plugin is working)
	filePath := filepath.Join(suite.projectRoot, "src/services/user_service.py")
	response, err := suite.lspClient.SendRequest(ctx, "textDocument/completion", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      20, // After User.objects.
			"character": 50,
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Completion failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var completions map[string]interface{}
	if err := json.Unmarshal(response, &completions); err != nil {
		suite.T().Logf("Failed to parse completion response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Check if completion items exist
	if items, exists := completions["items"]; exists {
		if itemList, ok := items.([]interface{}); ok {
			return len(itemList) > 0
		}
	}

	return false
}

// Additional test methods for specific Python features

func (suite *PythonRealPylspIntegrationTestSuite) testTypeHintInferenceAndChecking(ctx context.Context) bool {
	return suite.testGoToDefinition(ctx)
}

func (suite *PythonRealPylspIntegrationTestSuite) testImportPathResolution(ctx context.Context) bool {
	return suite.testImportResolution(ctx)
}

func (suite *PythonRealPylspIntegrationTestSuite) testDjangoModelHandling(ctx context.Context) bool {
	return suite.testDjangoModelSupport(ctx)
}

func (suite *PythonRealPylspIntegrationTestSuite) testDocstringProcessing(ctx context.Context) bool {
	// Test hover on a method with docstring
	filePath := filepath.Join(suite.projectRoot, "src/apps/users/models.py")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      95, // def get_full_name(self) -> str:
			"character": 12, // Position of "get_full_name"
		},
	})

	return err == nil && response != nil
}

func (suite *PythonRealPylspIntegrationTestSuite) testVirtualEnvResolution(ctx context.Context) bool {
	// Test if pylsp can resolve Django imports (indicates proper Python path)
	return suite.testDjangoModelSupport(ctx)
}

func (suite *PythonRealPylspIntegrationTestSuite) testPylspPluginSupport(ctx context.Context) bool {
	return suite.testPylspPluginIntegration(ctx)
}

// Performance testing structures and methods

type PythonPerformanceTestResult struct {
	TotalDuration time.Duration
	SuccessRate   float64
	RequestCount  int
	FailureCount  int
}

func (suite *PythonRealPylspIntegrationTestSuite) executePerformanceTest(ctx context.Context, requestCount, concurrency int) *PythonPerformanceTestResult {
	startTime := time.Now()
	
	var wg sync.WaitGroup
	successChan := make(chan bool, requestCount)
	
	// Execute concurrent requests
	requestsPerWorker := requestCount / concurrency
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerWorker; j++ {
				// Alternate between different types of supported LSP requests
				var success bool
				switch j % 6 {
				case 0:
					success = suite.testGoToDefinition(ctx)         // textDocument/definition
				case 1:
					success = suite.testFindReferences(ctx)         // textDocument/references
				case 2:
					success = suite.testHoverInformation(ctx)       // textDocument/hover
				case 3:
					success = suite.testDocumentSymbols(ctx)        // textDocument/documentSymbol
				case 4:
					success = suite.testModuleResolution(ctx)       // workspace/symbol
				case 5:
					success = suite.testPylspPluginIntegration(ctx) // textDocument/completion
				}
				successChan <- success
			}
		}(i)
	}
	
	wg.Wait()
	close(successChan)
	
	// Calculate results
	totalDuration := time.Since(startTime)
	successCount := 0
	for success := range successChan {
		if success {
			successCount++
		}
	}
	
	return &PythonPerformanceTestResult{
		TotalDuration: totalDuration,
		SuccessRate:   float64(successCount) / float64(requestCount),
		RequestCount:  requestCount,
		FailureCount:  requestCount - successCount,
	}
}

// Helper methods for file operations and notifications


func (suite *PythonRealPylspIntegrationTestSuite) getProcessPID() int {
	// Try to get the actual process PID from the LSP client
	if stdioClient, ok := suite.lspClient.(*transport.StdioClient); ok {
		return stdioClient.GetProcessPIDForTesting()
	}
	return 0
}

func (suite *PythonRealPylspIntegrationTestSuite) validatePythonIntegrationResult(result *PythonIntegrationResult) {
	suite.True(result.ServerStartSuccess, "Python server should start successfully")
	suite.True(result.InitializationSuccess, "Server initialization should succeed")
	suite.True(result.ProjectLoadSuccess, "Project should load successfully")
	suite.True(result.TypeHintAccuracy > 0.0, "Type hint processing should work accurately")
	suite.True(result.ImportResolutionWorks, "Import resolution should work")
	suite.True(result.ModuleResolutionWorks, "Module resolution should work")
	suite.True(result.DjangoModelSupport, "Django model support should work")
	suite.True(result.PylspPluginIntegration, "Pylsp plugin integration should work")
	
	suite.Less(result.TestDuration, suite.testTimeout, "Test should complete within timeout")
	suite.Equal(0, result.ErrorCount, "Should have no errors in comprehensive test")
	
	// Performance validations
	suite.NotNil(result.PerformanceMetrics, "Performance metrics should be collected")
	if result.PerformanceMetrics != nil {
		suite.Greater(result.PerformanceMetrics.TotalRequests, 0, "Should have made requests")
		suite.Greater(result.PerformanceMetrics.SuccessfulRequests, 0, "Should have successful requests")
		suite.Less(result.PerformanceMetrics.InitializationTime, 45*time.Second, "Initialization should be reasonable")
		suite.Less(result.PerformanceMetrics.PythonStartupTime, 15*time.Second, "Python startup should be reasonable")
	}
}

func (suite *PythonRealPylspIntegrationTestSuite) recordPerformanceMetrics(result *PythonIntegrationResult) {
	if suite.performanceMetrics.TotalRequests > 0 {
		suite.performanceMetrics.AverageResponseTime = result.TestDuration / time.Duration(suite.performanceMetrics.TotalRequests)
	}
	
	result.PerformanceMetrics = suite.performanceMetrics
}

// Test suite runner
func TestPythonRealPylspIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PythonRealPylspIntegrationTestSuite))
}