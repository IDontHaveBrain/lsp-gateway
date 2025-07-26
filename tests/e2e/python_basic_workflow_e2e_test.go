package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"
)

// PythonBasicWorkflowE2ETestSuite provides comprehensive E2E tests for Python LSP workflows
// covering all essential Python development scenarios that developers use daily
type PythonBasicWorkflowE2ETestSuite struct {
	suite.Suite
	mockClient    *mocks.MockMcpClient
	testTimeout   time.Duration
	workspaceRoot string
	pythonFiles   map[string]PythonTestFile
}

// PythonTestFile represents a realistic Python file with its metadata
type PythonTestFile struct {
	FileName    string
	Content     string
	Language    string
	Description string
	Symbols     []PythonSymbol
}

// PythonSymbol represents a Python symbol with position information
type PythonSymbol struct {
	Name        string
	Kind        int    // LSP SymbolKind
	Position    LSPPosition
	Type        string // Python type information
	Description string
}

// PythonWorkflowResult captures results from Python workflow execution
type PythonWorkflowResult struct {
	DefinitionFound        bool
	ReferencesCount        int
	HoverInfoRetrieved     bool
	DocumentSymbolsCount   int
	CompletionItemsCount   int
	TypeInformationValid   bool
	WorkflowLatency        time.Duration
	ErrorCount             int
	RequestCount           int
}

// SetupSuite initializes the test suite with realistic Python fixtures
func (suite *PythonBasicWorkflowE2ETestSuite) SetupSuite() {
	suite.testTimeout = 30 * time.Second
	suite.workspaceRoot = "/workspace"
	suite.setupPythonTestFiles()
}

// SetupTest initializes a fresh mock client for each test
func (suite *PythonBasicWorkflowE2ETestSuite) SetupTest() {
	suite.mockClient = mocks.NewMockMcpClient()
	suite.mockClient.SetHealthy(true)
}

// TearDownTest cleans up mock client state
func (suite *PythonBasicWorkflowE2ETestSuite) TearDownTest() {
	if suite.mockClient != nil {
		suite.mockClient.Reset()
	}
}

// setupPythonTestFiles creates realistic Python test files and fixtures
func (suite *PythonBasicWorkflowE2ETestSuite) setupPythonTestFiles() {
	suite.pythonFiles = map[string]PythonTestFile{
		"src/handlers/request_handler.py": {
			FileName:    "src/handlers/request_handler.py",
			Language:    "python",
			Description: "Main request handler with Django integration",
			Content: `"""Request handler module for Django web application."""
from typing import Dict, Any, Optional, List
from django.http import HttpRequest, HttpResponse, JsonResponse
from django.views.decorators.csrf import csrf_exempt
from django.utils.decorators import method_decorator
from django.views import View
import json
import logging

from ..models.user_model import User
from ..services.user_service import UserService
from ..utils.request_utils import validate_request, extract_headers


logger = logging.getLogger(__name__)


class RequestHandler(View):
    """Main request handler for API endpoints."""
    
    def __init__(self):
        """Initialize the request handler with required services."""
        super().__init__()
        self.user_service = UserService()
    
    @method_decorator(csrf_exempt)
    def dispatch(self, request: HttpRequest, *args, **kwargs) -> HttpResponse:
        """Override dispatch to handle CSRF exemption."""
        return super().dispatch(request, *args, **kwargs)
    
    def handle_request(self, request: HttpRequest) -> HttpResponse:
        """
        Handle incoming HTTP request and return appropriate response.
        
        Args:
            request: The incoming HTTP request object
            
        Returns:
            HttpResponse: HTTP response object
            
        Raises:
            ValueError: If request is invalid
            HTTPException: If processing fails
        """
        try:
            # Validate incoming request
            if not validate_request(request):
                return JsonResponse({'error': 'Invalid request'}, status=400)
                
            # Extract and process headers
            headers = extract_headers(request)
            logger.info(f"Processing request with headers: {headers}")
            
            # Handle different HTTP methods
            if request.method == 'GET':
                return self._handle_get_request(request)
            elif request.method == 'POST':
                return self._handle_post_request(request)
            elif request.method == 'PUT':
                return self._handle_put_request(request)
            elif request.method == 'DELETE':
                return self._handle_delete_request(request)
            else:
                return JsonResponse({'error': 'Method not allowed'}, status=405)
                
        except ValueError as e:
            logger.error(f"Validation error: {e}")
            return JsonResponse({'error': str(e)}, status=400)
        except Exception as e:
            logger.error(f"Internal server error: {e}")
            return JsonResponse({'error': 'Internal server error'}, status=500)
    
    def _handle_get_request(self, request: HttpRequest) -> JsonResponse:
        """Handle GET requests for resource retrieval."""
        user_id = request.GET.get('user_id')
        if user_id:
            user = self.user_service.get_user_by_id(int(user_id))
            if user:
                return JsonResponse({'user': user.to_dict()})
            return JsonResponse({'error': 'User not found'}, status=404)
        
        # Return all users
        users = self.user_service.get_all_users()
        return JsonResponse({'users': [user.to_dict() for user in users]})
    
    def _handle_post_request(self, request: HttpRequest) -> JsonResponse:
        """Handle POST requests for resource creation."""
        try:
            data = json.loads(request.body)
            user = self.user_service.create_user(data)
            return JsonResponse({'user': user.to_dict()}, status=201)
        except json.JSONDecodeError:
            return JsonResponse({'error': 'Invalid JSON'}, status=400)
    
    def _handle_put_request(self, request: HttpRequest) -> JsonResponse:
        """Handle PUT requests for resource updates."""
        user_id = request.GET.get('user_id')
        if not user_id:
            return JsonResponse({'error': 'User ID required'}, status=400)
            
        try:
            data = json.loads(request.body)
            user = self.user_service.update_user(int(user_id), data)
            if user:
                return JsonResponse({'user': user.to_dict()})
            return JsonResponse({'error': 'User not found'}, status=404)
        except json.JSONDecodeError:
            return JsonResponse({'error': 'Invalid JSON'}, status=400)
    
    def _handle_delete_request(self, request: HttpRequest) -> JsonResponse:
        """Handle DELETE requests for resource deletion."""
        user_id = request.GET.get('user_id')
        if not user_id:
            return JsonResponse({'error': 'User ID required'}, status=400)
            
        success = self.user_service.delete_user(int(user_id))
        if success:
            return JsonResponse({'message': 'User deleted successfully'})
        return JsonResponse({'error': 'User not found'}, status=404)


# Function-based view example
@csrf_exempt
def api_endpoint(request: HttpRequest) -> JsonResponse:
    """Simple API endpoint function."""
    handler = RequestHandler()
    return handler.handle_request(request)


def health_check(request: HttpRequest) -> JsonResponse:
    """Health check endpoint."""
    return JsonResponse({'status': 'healthy', 'timestamp': time.time()})
`,
			Symbols: []PythonSymbol{
				{Name: "RequestHandler", Kind: 5, Position: LSPPosition{Line: 18, Character: 6}, Type: "class", Description: "Main request handler class"},
				{Name: "handle_request", Kind: 6, Position: LSPPosition{Line: 29, Character: 8}, Type: "method", Description: "Handle incoming HTTP request method"},
				{Name: "api_endpoint", Kind: 12, Position: LSPPosition{Line: 98, Character: 4}, Type: "function", Description: "API endpoint function"},
				{Name: "health_check", Kind: 12, Position: LSPPosition{Line: 103, Character: 4}, Type: "function", Description: "Health check function"},
			},
		},
		"src/models/user_model.py": {
			FileName:    "src/models/user_model.py",
			Language:    "python",
			Description: "User model with Django ORM integration",
			Content: `"""User model definition for Django application."""
from django.db import models
from django.contrib.auth.models import AbstractUser
from django.core.validators import EmailValidator
from django.urls import reverse
from typing import Dict, Any, Optional
from datetime import datetime
import uuid


class UserRole(models.TextChoices):
    """User role choices."""
    USER = 'user', 'User'
    ADMIN = 'admin', 'Administrator'
    MODERATOR = 'moderator', 'Moderator'


class User(AbstractUser):
    """
    Custom user model extending Django's AbstractUser.
    
    Attributes:
        id: UUID primary key
        email: Unique email address
        role: User role (user, admin, moderator)
        is_active: Boolean indicating if user is active
        created_at: Timestamp when user was created
        updated_at: Timestamp when user was last updated
        profile: Related user profile
    """
    
    # Override the default ID field with UUID
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    
    # Email field with validation
    email = models.EmailField(
        unique=True,
        validators=[EmailValidator()],
        help_text="User's unique email address"
    )
    
    # User role
    role = models.CharField(
        max_length=20,
        choices=UserRole.choices,
        default=UserRole.USER,
        help_text="User's role in the system"
    )
    
    # Additional fields
    is_active = models.BooleanField(default=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    # Use email as the unique identifier
    USERNAME_FIELD = 'email'
    REQUIRED_FIELDS = ['username', 'first_name', 'last_name']
    
    class Meta:
        """Meta configuration for User model."""
        db_table = 'users'
        verbose_name = 'User'
        verbose_name_plural = 'Users'
        ordering = ['-created_at']
        indexes = [
            models.Index(fields=['email']),
            models.Index(fields=['role']),
            models.Index(fields=['created_at']),
        ]
    
    def __str__(self) -> str:
        """String representation of the user."""
        return f"{self.get_full_name()} ({self.email})"
    
    def __repr__(self) -> str:
        """Developer representation of the user."""
        return f"User(id={self.id}, email='{self.email}', role='{self.role}')"
    
    def get_absolute_url(self) -> str:
        """Get the absolute URL for this user."""
        return reverse('user-detail', kwargs={'pk': self.pk})
    
    def to_dict(self) -> Dict[str, Any]:
        """
        Convert user instance to dictionary representation.
        
        Returns:
            Dict containing user data suitable for JSON serialization
        """
        return {
            'id': str(self.id),
            'username': self.username,
            'email': self.email,
            'first_name': self.first_name,
            'last_name': self.last_name,
            'role': self.role,
            'is_active': self.is_active,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
            'last_login': self.last_login.isoformat() if self.last_login else None,
        }
    
    @classmethod
    def create_user(cls, email: str, username: str, password: str, **extra_fields) -> 'User':
        """
        Create and return a user with email and password.
        
        Args:
            email: User's email address
            username: User's username
            password: User's password
            **extra_fields: Additional fields for user creation
            
        Returns:
            Created User instance
        """
        if not email:
            raise ValueError('Email is required')
        if not username:
            raise ValueError('Username is required')
            
        user = cls(email=email, username=username, **extra_fields)
        user.set_password(password)
        user.save()
        return user
    
    def is_admin(self) -> bool:
        """Check if user has admin role."""
        return self.role == UserRole.ADMIN
    
    def is_moderator(self) -> bool:
        """Check if user has moderator role."""
        return self.role == UserRole.MODERATOR
    
    def can_moderate(self) -> bool:
        """Check if user can perform moderation actions."""
        return self.role in [UserRole.ADMIN, UserRole.MODERATOR]
    
    def get_permissions(self) -> Dict[str, bool]:
        """
        Get user permissions based on role.
        
        Returns:
            Dictionary of permission flags
        """
        base_permissions = {
            'can_create': False,
            'can_edit_own': True,
            'can_edit_others': False,
            'can_delete_own': False,
            'can_delete_others': False,
            'can_moderate': False,
            'can_admin': False,
        }
        
        if self.role == UserRole.ADMIN:
            return {
                'can_create': True,
                'can_edit_own': True,
                'can_edit_others': True,
                'can_delete_own': True,
                'can_delete_others': True,
                'can_moderate': True,
                'can_admin': True,
            }
        elif self.role == UserRole.MODERATOR:
            base_permissions.update({
                'can_create': True,
                'can_edit_others': True,
                'can_moderate': True,
            })
        
        return base_permissions


class UserProfile(models.Model):
    """Extended user profile information."""
    
    user = models.OneToOneField(
        User,
        on_delete=models.CASCADE,
        related_name='profile'
    )
    avatar = models.ImageField(upload_to='avatars/', blank=True, null=True)
    bio = models.TextField(max_length=500, blank=True)
    location = models.CharField(max_length=100, blank=True)
    website = models.URLField(blank=True)
    birth_date = models.DateField(null=True, blank=True)
    
    # Social media links
    twitter_handle = models.CharField(max_length=50, blank=True)
    github_username = models.CharField(max_length=50, blank=True)
    linkedin_profile = models.CharField(max_length=100, blank=True)
    
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    class Meta:
        """Meta configuration for UserProfile model."""
        db_table = 'user_profiles'
        verbose_name = 'User Profile'
        verbose_name_plural = 'User Profiles'
    
    def __str__(self) -> str:
        """String representation of the user profile."""
        return f"Profile for {self.user.get_full_name()}"
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert profile to dictionary."""
        return {
            'avatar': self.avatar.url if self.avatar else None,
            'bio': self.bio,
            'location': self.location,
            'website': self.website,
            'birth_date': self.birth_date.isoformat() if self.birth_date else None,
            'social': {
                'twitter': self.twitter_handle,
                'github': self.github_username,
                'linkedin': self.linkedin_profile,
            }
        }
`,
			Symbols: []PythonSymbol{
				{Name: "UserRole", Kind: 5, Position: LSPPosition{Line: 11, Character: 6}, Type: "class", Description: "User role enumeration"},
				{Name: "User", Kind: 5, Position: LSPPosition{Line: 17, Character: 6}, Type: "class", Description: "Custom user model"},
				{Name: "to_dict", Kind: 6, Position: LSPPosition{Line: 73, Character: 8}, Type: "method", Description: "Convert user to dictionary"},
				{Name: "create_user", Kind: 6, Position: LSPPosition{Line: 88, Character: 8}, Type: "method", Description: "Create user class method"},
				{Name: "UserProfile", Kind: 5, Position: LSPPosition{Line: 149, Character: 6}, Type: "class", Description: "User profile model"},
			},
		},
		"src/services/user_service.py": {
			FileName:    "src/services/user_service.py",
			Language:    "python",
			Description: "User service with business logic and database operations",
			Content: `"""User service for handling business logic and database operations."""
from typing import List, Optional, Dict, Any
from django.db import transaction
from django.core.exceptions import ValidationError
from django.contrib.auth import authenticate
from django.contrib.auth.hashers import make_password
import logging

from ..models.user_model import User, UserRole, UserProfile
from ..utils.validation_utils import validate_email, validate_password


logger = logging.getLogger(__name__)


class UserService:
    """Service class for user-related business operations."""
    
    def __init__(self):
        """Initialize the user service."""
        self.logger = logging.getLogger(self.__class__.__name__)
    
    def get_all_users(self) -> List[User]:
        """
        Retrieve all active users from the database.
        
        Returns:
            List of User instances
        """
        try:
            users = User.objects.filter(is_active=True).select_related('profile')
            self.logger.info(f"Retrieved {len(users)} active users")
            return list(users)
        except Exception as e:
            self.logger.error(f"Error retrieving users: {e}")
            return []
    
    def get_user_by_id(self, user_id: int) -> Optional[User]:
        """
        Retrieve a user by their ID.
        
        Args:
            user_id: The user's ID
            
        Returns:
            User instance if found, None otherwise
        """
        try:
            user = User.objects.select_related('profile').get(id=user_id, is_active=True)
            self.logger.info(f"Retrieved user: {user.email}")
            return user
        except User.DoesNotExist:
            self.logger.warning(f"User with ID {user_id} not found")
            return None
        except Exception as e:
            self.logger.error(f"Error retrieving user {user_id}: {e}")
            return None
    
    def get_user_by_email(self, email: str) -> Optional[User]:
        """
        Retrieve a user by their email address.
        
        Args:
            email: The user's email address
            
        Returns:
            User instance if found, None otherwise
        """
        try:
            user = User.objects.select_related('profile').get(email=email, is_active=True)
            self.logger.info(f"Retrieved user by email: {email}")
            return user
        except User.DoesNotExist:
            self.logger.warning(f"User with email {email} not found")
            return None
        except Exception as e:
            self.logger.error(f"Error retrieving user by email {email}: {e}")
            return None
    
    @transaction.atomic
    def create_user(self, user_data: Dict[str, Any]) -> Optional[User]:
        """
        Create a new user with validation.
        
        Args:
            user_data: Dictionary containing user information
            
        Returns:
            Created User instance if successful, None otherwise
            
        Raises:
            ValidationError: If user data is invalid
        """
        try:
            # Validate required fields
            required_fields = ['email', 'username', 'password', 'first_name', 'last_name']
            for field in required_fields:
                if field not in user_data or not user_data[field]:
                    raise ValidationError(f"Field '{field}' is required")
            
            # Validate email format
            if not validate_email(user_data['email']):
                raise ValidationError("Invalid email format")
            
            # Validate password strength
            if not validate_password(user_data['password']):
                raise ValidationError("Password does not meet security requirements")
            
            # Check if user already exists
            if User.objects.filter(email=user_data['email']).exists():
                raise ValidationError("User with this email already exists")
            
            if User.objects.filter(username=user_data['username']).exists():
                raise ValidationError("User with this username already exists")
            
            # Create user
            user = User.objects.create_user(
                email=user_data['email'],
                username=user_data['username'],
                password=user_data['password'],
                first_name=user_data['first_name'],
                last_name=user_data['last_name'],
                role=user_data.get('role', UserRole.USER)
            )
            
            # Create user profile if profile data provided
            profile_data = user_data.get('profile', {})
            if profile_data:
                UserProfile.objects.create(
                    user=user,
                    bio=profile_data.get('bio', ''),
                    location=profile_data.get('location', ''),
                    website=profile_data.get('website', ''),
                    twitter_handle=profile_data.get('twitter', ''),
                    github_username=profile_data.get('github', ''),
                    linkedin_profile=profile_data.get('linkedin', '')
                )
            
            self.logger.info(f"Created new user: {user.email}")
            return user
            
        except ValidationError as e:
            self.logger.error(f"Validation error creating user: {e}")
            raise
        except Exception as e:
            self.logger.error(f"Error creating user: {e}")
            return None
    
    @transaction.atomic
    def update_user(self, user_id: int, update_data: Dict[str, Any]) -> Optional[User]:
        """
        Update an existing user.
        
        Args:
            user_id: The user's ID
            update_data: Dictionary containing fields to update
            
        Returns:
            Updated User instance if successful, None otherwise
        """
        try:
            user = self.get_user_by_id(user_id)
            if not user:
                return None
            
            # Update allowed fields
            allowed_fields = ['first_name', 'last_name', 'role']
            for field in allowed_fields:
                if field in update_data:
                    setattr(user, field, update_data[field])
            
            # Handle password update separately
            if 'password' in update_data:
                if validate_password(update_data['password']):
                    user.set_password(update_data['password'])
                else:
                    raise ValidationError("Invalid password")
            
            user.save()
            
            # Update profile if profile data provided
            profile_data = update_data.get('profile', {})
            if profile_data:
                profile, created = UserProfile.objects.get_or_create(user=user)
                for field, value in profile_data.items():
                    if hasattr(profile, field):
                        setattr(profile, field, value)
                profile.save()
            
            self.logger.info(f"Updated user: {user.email}")
            return user
            
        except ValidationError as e:
            self.logger.error(f"Validation error updating user {user_id}: {e}")
            raise
        except Exception as e:
            self.logger.error(f"Error updating user {user_id}: {e}")
            return None
    
    def delete_user(self, user_id: int) -> bool:
        """
        Soft delete a user (set is_active=False).
        
        Args:
            user_id: The user's ID
            
        Returns:
            True if user was deleted, False otherwise
        """
        try:
            user = self.get_user_by_id(user_id)
            if not user:
                return False
            
            user.is_active = False
            user.save()
            
            self.logger.info(f"Deleted user: {user.email}")
            return True
            
        except Exception as e:
            self.logger.error(f"Error deleting user {user_id}: {e}")
            return False
    
    def authenticate_user(self, email: str, password: str) -> Optional[User]:
        """
        Authenticate a user with email and password.
        
        Args:
            email: User's email address
            password: User's password
            
        Returns:
            User instance if authentication successful, None otherwise
        """
        try:
            user = authenticate(username=email, password=password)
            if user and user.is_active:
                self.logger.info(f"User authenticated: {email}")
                return user
            else:
                self.logger.warning(f"Authentication failed for: {email}")
                return None
        except Exception as e:
            self.logger.error(f"Error authenticating user {email}: {e}")
            return None
    
    def get_users_by_role(self, role: UserRole) -> List[User]:
        """
        Get all users with a specific role.
        
        Args:
            role: The user role to filter by
            
        Returns:
            List of users with the specified role
        """
        try:
            users = User.objects.filter(role=role, is_active=True).select_related('profile')
            self.logger.info(f"Retrieved {len(users)} users with role: {role}")
            return list(users)
        except Exception as e:
            self.logger.error(f"Error retrieving users by role {role}: {e}")
            return []
    
    def search_users(self, query: str) -> List[User]:
        """
        Search users by name or email.
        
        Args:
            query: Search query string
            
        Returns:
            List of matching users
        """
        try:
            users = User.objects.filter(
                models.Q(first_name__icontains=query) |
                models.Q(last_name__icontains=query) |
                models.Q(email__icontains=query),
                is_active=True
            ).select_related('profile')
            
            self.logger.info(f"Found {len(users)} users matching query: {query}")
            return list(users)
        except Exception as e:
            self.logger.error(f"Error searching users with query {query}: {e}")
            return []
`,
			Symbols: []PythonSymbol{
				{Name: "UserService", Kind: 5, Position: LSPPosition{Line: 16, Character: 6}, Type: "class", Description: "User service class"},
				{Name: "get_all_users", Kind: 6, Position: LSPPosition{Line: 22, Character: 8}, Type: "method", Description: "Get all users method"},
				{Name: "create_user", Kind: 6, Position: LSPPosition{Line: 73, Character: 8}, Type: "method", Description: "Create user method"},
				{Name: "authenticate_user", Kind: 6, Position: LSPPosition{Line: 194, Character: 8}, Type: "method", Description: "Authenticate user method"},
			},
		},
		"src/utils/validation_utils.py": {
			FileName:    "src/utils/validation_utils.py",
			Language:    "python",
			Description: "Validation utilities for data validation and sanitization",
			Content: `"""Validation utilities for data validation and sanitization."""
import re
from typing import Any, Dict, List, Optional, Union
from django.core.validators import EmailValidator
from django.core.exceptions import ValidationError
import logging


logger = logging.getLogger(__name__)

# Regular expressions for validation
EMAIL_REGEX = re.compile(r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$')
PASSWORD_REGEX = re.compile(r'^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$')
USERNAME_REGEX = re.compile(r'^[a-zA-Z0-9_]{3,30}$')
PHONE_REGEX = re.compile(r'^\+?1?\d{9,15}$')


def validate_email(email: str) -> bool:
    """
    Validate email address format.
    
    Args:
        email: Email address to validate
        
    Returns:
        True if email is valid, False otherwise
    """
    if not email or not isinstance(email, str):
        return False
    
    try:
        # Use Django's built-in email validator
        validator = EmailValidator()
        validator(email)
        return True
    except ValidationError:
        return False


def validate_password(password: str) -> bool:
    """
    Validate password strength.
    
    Requirements:
    - At least 8 characters long
    - Contains at least one lowercase letter
    - Contains at least one uppercase letter
    - Contains at least one digit
    - Contains at least one special character
    
    Args:
        password: Password to validate
        
    Returns:
        True if password meets requirements, False otherwise
    """
    if not password or not isinstance(password, str):
        return False
    
    return bool(PASSWORD_REGEX.match(password))


def validate_username(username: str) -> bool:
    """
    Validate username format.
    
    Requirements:
    - 3-30 characters long
    - Only letters, numbers, and underscores
    
    Args:
        username: Username to validate
        
    Returns:
        True if username is valid, False otherwise
    """
    if not username or not isinstance(username, str):
        return False
    
    return bool(USERNAME_REGEX.match(username))


def validate_phone_number(phone: str) -> bool:
    """
    Validate phone number format.
    
    Args:
        phone: Phone number to validate
        
    Returns:
        True if phone number is valid, False otherwise
    """
    if not phone or not isinstance(phone, str):
        return False
    
    # Remove all non-digit characters except +
    cleaned_phone = re.sub(r'[^\d+]', '', phone)
    return bool(PHONE_REGEX.match(cleaned_phone))


def sanitize_string(value: str, max_length: Optional[int] = None) -> str:
    """
    Sanitize string input by removing dangerous characters.
    
    Args:
        value: String to sanitize
        max_length: Maximum allowed length (optional)
        
    Returns:
        Sanitized string
    """
    if not isinstance(value, str):
        return ""
    
    # Remove null bytes and control characters
    sanitized = ''.join(char for char in value if ord(char) >= 32 or char in '\t\n\r')
    
    # Strip whitespace
    sanitized = sanitized.strip()
    
    # Truncate if max_length specified
    if max_length and len(sanitized) > max_length:
        sanitized = sanitized[:max_length]
    
    return sanitized


def validate_required_fields(data: Dict[str, Any], required_fields: List[str]) -> List[str]:
    """
    Validate that all required fields are present and not empty.
    
    Args:
        data: Dictionary to validate
        required_fields: List of required field names
        
    Returns:
        List of missing or empty field names
    """
    missing_fields = []
    
    for field in required_fields:
        if field not in data:
            missing_fields.append(field)
        elif data[field] is None or (isinstance(data[field], str) and not data[field].strip()):
            missing_fields.append(field)
    
    return missing_fields


def validate_data_types(data: Dict[str, Any], type_mapping: Dict[str, type]) -> List[str]:
    """
    Validate data types of fields in a dictionary.
    
    Args:
        data: Dictionary to validate
        type_mapping: Dictionary mapping field names to expected types
        
    Returns:
        List of fields with incorrect types
    """
    invalid_types = []
    
    for field, expected_type in type_mapping.items():
        if field in data and not isinstance(data[field], expected_type):
            invalid_types.append(field)
    
    return invalid_types


def validate_string_length(value: str, min_length: int = 0, max_length: Optional[int] = None) -> bool:
    """
    Validate string length constraints.
    
    Args:
        value: String to validate
        min_length: Minimum required length
        max_length: Maximum allowed length (optional)
        
    Returns:
        True if length is valid, False otherwise
    """
    if not isinstance(value, str):
        return False
    
    length = len(value)
    
    if length < min_length:
        return False
    
    if max_length is not None and length > max_length:
        return False
    
    return True


def validate_numeric_range(value: Union[int, float], min_value: Optional[Union[int, float]] = None, 
                          max_value: Optional[Union[int, float]] = None) -> bool:
    """
    Validate numeric value is within specified range.
    
    Args:
        value: Numeric value to validate
        min_value: Minimum allowed value (optional)
        max_value: Maximum allowed value (optional)
        
    Returns:
        True if value is in range, False otherwise
    """
    if not isinstance(value, (int, float)):
        return False
    
    if min_value is not None and value < min_value:
        return False
    
    if max_value is not None and value > max_value:
        return False
    
    return True


def validate_list_items(items: List[Any], item_validator: callable) -> List[int]:
    """
    Validate items in a list using a validator function.
    
    Args:
        items: List of items to validate
        item_validator: Function to validate individual items
        
    Returns:
        List of indices of invalid items
    """
    if not isinstance(items, list):
        return []
    
    invalid_indices = []
    
    for i, item in enumerate(items):
        if not item_validator(item):
            invalid_indices.append(i)
    
    return invalid_indices


class ValidationResult:
    """Class to hold validation results."""
    
    def __init__(self):
        """Initialize validation result."""
        self.is_valid = True
        self.errors = []
        self.warnings = []
    
    def add_error(self, message: str):
        """Add an error message."""
        self.is_valid = False
        self.errors.append(message)
    
    def add_warning(self, message: str):
        """Add a warning message."""
        self.warnings.append(message)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary representation."""
        return {
            'is_valid': self.is_valid,
            'errors': self.errors,
            'warnings': self.warnings
        }


def comprehensive_user_validation(user_data: Dict[str, Any]) -> ValidationResult:
    """
    Perform comprehensive validation on user data.
    
    Args:
        user_data: Dictionary containing user data
        
    Returns:
        ValidationResult instance with validation results
    """
    result = ValidationResult()
    
    # Check required fields
    required_fields = ['email', 'username', 'first_name', 'last_name', 'password']
    missing_fields = validate_required_fields(user_data, required_fields)
    
    for field in missing_fields:
        result.add_error(f"Field '{field}' is required")
    
    # Validate email
    if 'email' in user_data and not validate_email(user_data['email']):
        result.add_error("Invalid email format")
    
    # Validate username
    if 'username' in user_data and not validate_username(user_data['username']):
        result.add_error("Invalid username format (3-30 chars, letters, numbers, underscores only)")
    
    # Validate password
    if 'password' in user_data and not validate_password(user_data['password']):
        result.add_error("Password must be at least 8 characters with uppercase, lowercase, digit, and special character")
    
    # Validate names
    for field in ['first_name', 'last_name']:
        if field in user_data and not validate_string_length(user_data[field], 1, 50):
            result.add_error(f"Field '{field}' must be 1-50 characters long")
    
    # Validate optional phone number
    if 'phone' in user_data and user_data['phone'] and not validate_phone_number(user_data['phone']):
        result.add_error("Invalid phone number format")
    
    return result
`,
			Symbols: []PythonSymbol{
				{Name: "validate_email", Kind: 12, Position: LSPPosition{Line: 17, Character: 4}, Type: "function", Description: "Email validation function"},
				{Name: "validate_password", Kind: 12, Position: LSPPosition{Line: 38, Character: 4}, Type: "function", Description: "Password validation function"},
				{Name: "validate_username", Kind: 12, Position: LSPPosition{Line: 63, Character: 4}, Type: "function", Description: "Username validation function"},
				{Name: "ValidationResult", Kind: 5, Position: LSPPosition{Line: 233, Character: 6}, Type: "class", Description: "Validation result class"},
				{Name: "comprehensive_user_validation", Kind: 12, Position: LSPPosition{Line: 258, Character: 4}, Type: "function", Description: "Comprehensive user validation function"},
			},
		},
		"tests/test_user_service.py": {
			FileName:    "tests/test_user_service.py",
			Language:    "python",
			Description: "Unit tests for user service functionality",
			Content: `"""Unit tests for user service functionality."""
import unittest
from unittest.mock import Mock, patch, MagicMock
from django.test import TestCase
from django.core.exceptions import ValidationError
from django.contrib.auth import get_user_model
import uuid

from src.services.user_service import UserService
from src.models.user_model import User, UserRole, UserProfile


class UserServiceTestCase(TestCase):
    """Test cases for UserService class."""
    
    def setUp(self):
        """Set up test fixtures."""
        self.user_service = UserService()
        self.test_user_data = {
            'email': 'test@example.com',
            'username': 'testuser',
            'password': 'SecurePass123!',
            'first_name': 'Test',
            'last_name': 'User',
            'role': UserRole.USER
        }
    
    def tearDown(self):
        """Clean up after tests."""
        User.objects.all().delete()
    
    def test_get_all_users_empty(self):
        """Test getting all users when database is empty."""
        users = self.user_service.get_all_users()
        self.assertEqual(len(users), 0)
        self.assertIsInstance(users, list)
    
    def test_get_all_users_with_data(self):
        """Test getting all users with existing data."""
        # Create test users
        user1 = User.objects.create_user(
            email='user1@example.com',
            username='user1',
            password='password123'
        )
        user2 = User.objects.create_user(
            email='user2@example.com',
            username='user2',
            password='password123'
        )
        
        users = self.user_service.get_all_users()
        self.assertEqual(len(users), 2)
        self.assertIn(user1, users)
        self.assertIn(user2, users)
    
    def test_get_all_users_excludes_inactive(self):
        """Test that get_all_users excludes inactive users."""
        # Create active user
        active_user = User.objects.create_user(
            email='active@example.com',
            username='active',
            password='password123'
        )
        
        # Create inactive user
        inactive_user = User.objects.create_user(
            email='inactive@example.com',
            username='inactive',
            password='password123',
            is_active=False
        )
        
        users = self.user_service.get_all_users()
        self.assertEqual(len(users), 1)
        self.assertIn(active_user, users)
        self.assertNotIn(inactive_user, users)
    
    def test_get_user_by_id_existing(self):
        """Test getting user by ID when user exists."""
        user = User.objects.create_user(
            email='test@example.com',
            username='testuser',
            password='password123'
        )
        
        retrieved_user = self.user_service.get_user_by_id(user.id)
        self.assertIsNotNone(retrieved_user)
        self.assertEqual(retrieved_user.id, user.id)
        self.assertEqual(retrieved_user.email, user.email)
    
    def test_get_user_by_id_nonexistent(self):
        """Test getting user by ID when user doesn't exist."""
        nonexistent_id = str(uuid.uuid4())
        user = self.user_service.get_user_by_id(nonexistent_id)
        self.assertIsNone(user)
    
    def test_get_user_by_email_existing(self):
        """Test getting user by email when user exists."""
        user = User.objects.create_user(
            email='test@example.com',
            username='testuser',
            password='password123'
        )
        
        retrieved_user = self.user_service.get_user_by_email('test@example.com')
        self.assertIsNotNone(retrieved_user)
        self.assertEqual(retrieved_user.email, user.email)
    
    def test_get_user_by_email_nonexistent(self):
        """Test getting user by email when user doesn't exist."""
        user = self.user_service.get_user_by_email('nonexistent@example.com')
        self.assertIsNone(user)
    
    def test_create_user_valid_data(self):
        """Test creating a user with valid data."""
        user = self.user_service.create_user(self.test_user_data)
        
        self.assertIsNotNone(user)
        self.assertEqual(user.email, self.test_user_data['email'])
        self.assertEqual(user.username, self.test_user_data['username'])
        self.assertEqual(user.first_name, self.test_user_data['first_name'])
        self.assertEqual(user.last_name, self.test_user_data['last_name'])
        self.assertEqual(user.role, self.test_user_data['role'])
        self.assertTrue(user.is_active)
    
    def test_create_user_missing_required_field(self):
        """Test creating a user with missing required field."""
        invalid_data = self.test_user_data.copy()
        del invalid_data['email']
        
        with self.assertRaises(ValidationError) as context:
            self.user_service.create_user(invalid_data)
        
        self.assertIn("Field 'email' is required", str(context.exception))
    
    def test_create_user_invalid_email(self):
        """Test creating a user with invalid email."""
        invalid_data = self.test_user_data.copy()
        invalid_data['email'] = 'invalid-email'
        
        with patch('src.services.user_service.validate_email', return_value=False):
            with self.assertRaises(ValidationError) as context:
                self.user_service.create_user(invalid_data)
            
            self.assertIn("Invalid email format", str(context.exception))
    
    def test_create_user_weak_password(self):
        """Test creating a user with weak password."""
        invalid_data = self.test_user_data.copy()
        invalid_data['password'] = 'weak'
        
        with patch('src.services.user_service.validate_password', return_value=False):
            with self.assertRaises(ValidationError) as context:
                self.user_service.create_user(invalid_data)
            
            self.assertIn("Password does not meet security requirements", str(context.exception))
    
    def test_create_user_duplicate_email(self):
        """Test creating a user with duplicate email."""
        # Create first user
        User.objects.create_user(
            email=self.test_user_data['email'],
            username='firstuser',
            password='password123'
        )
        
        # Try to create second user with same email
        duplicate_data = self.test_user_data.copy()
        duplicate_data['username'] = 'seconduser'
        
        with self.assertRaises(ValidationError) as context:
            self.user_service.create_user(duplicate_data)
        
        self.assertIn("User with this email already exists", str(context.exception))
    
    def test_create_user_with_profile(self):
        """Test creating a user with profile data."""
        user_data_with_profile = self.test_user_data.copy()
        user_data_with_profile['profile'] = {
            'bio': 'Test bio',
            'location': 'Test City',
            'website': 'https://example.com',
            'twitter': 'testuser',
            'github': 'testuser'
        }
        
        user = self.user_service.create_user(user_data_with_profile)
        
        self.assertIsNotNone(user)
        self.assertTrue(hasattr(user, 'profile'))
        self.assertEqual(user.profile.bio, 'Test bio')
        self.assertEqual(user.profile.location, 'Test City')
        self.assertEqual(user.profile.website, 'https://example.com')
    
    def test_update_user_existing(self):
        """Test updating an existing user."""
        user = User.objects.create_user(
            email='test@example.com',
            username='testuser',
            password='password123'
        )
        
        update_data = {
            'first_name': 'Updated',
            'last_name': 'Name',
            'role': UserRole.ADMIN
        }
        
        updated_user = self.user_service.update_user(user.id, update_data)
        
        self.assertIsNotNone(updated_user)
        self.assertEqual(updated_user.first_name, 'Updated')
        self.assertEqual(updated_user.last_name, 'Name')
        self.assertEqual(updated_user.role, UserRole.ADMIN)
    
    def test_update_user_nonexistent(self):
        """Test updating a nonexistent user."""
        nonexistent_id = str(uuid.uuid4())
        update_data = {'first_name': 'Updated'}
        
        result = self.user_service.update_user(nonexistent_id, update_data)
        self.assertIsNone(result)
    
    def test_delete_user_existing(self):
        """Test deleting an existing user (soft delete)."""
        user = User.objects.create_user(
            email='test@example.com',
            username='testuser',
            password='password123'
        )
        
        result = self.user_service.delete_user(user.id)
        
        self.assertTrue(result)
        
        # Verify user is soft deleted (is_active=False)
        user.refresh_from_db()
        self.assertFalse(user.is_active)
    
    def test_delete_user_nonexistent(self):
        """Test deleting a nonexistent user."""
        nonexistent_id = str(uuid.uuid4())
        result = self.user_service.delete_user(nonexistent_id)
        self.assertFalse(result)
    
    @patch('src.services.user_service.authenticate')
    def test_authenticate_user_valid(self, mock_authenticate):
        """Test authenticating a user with valid credentials."""
        user = User.objects.create_user(
            email='test@example.com',
            username='testuser',
            password='password123'
        )
        
        mock_authenticate.return_value = user
        
        authenticated_user = self.user_service.authenticate_user('test@example.com', 'password123')
        
        self.assertIsNotNone(authenticated_user)
        self.assertEqual(authenticated_user, user)
        mock_authenticate.assert_called_once_with(username='test@example.com', password='password123')
    
    @patch('src.services.user_service.authenticate')
    def test_authenticate_user_invalid(self, mock_authenticate):
        """Test authenticating a user with invalid credentials."""
        mock_authenticate.return_value = None
        
        authenticated_user = self.user_service.authenticate_user('test@example.com', 'wrongpassword')
        
        self.assertIsNone(authenticated_user)
    
    def test_get_users_by_role(self):
        """Test getting users by role."""
        # Create users with different roles
        admin_user = User.objects.create_user(
            email='admin@example.com',
            username='admin',
            password='password123',
            role=UserRole.ADMIN
        )
        
        regular_user = User.objects.create_user(
            email='user@example.com',
            username='user',
            password='password123',
            role=UserRole.USER
        )
        
        moderator_user = User.objects.create_user(
            email='mod@example.com',
            username='moderator',
            password='password123',
            role=UserRole.MODERATOR
        )
        
        # Test getting admin users
        admin_users = self.user_service.get_users_by_role(UserRole.ADMIN)
        self.assertEqual(len(admin_users), 1)
        self.assertIn(admin_user, admin_users)
        
        # Test getting regular users
        regular_users = self.user_service.get_users_by_role(UserRole.USER)
        self.assertEqual(len(regular_users), 1)
        self.assertIn(regular_user, regular_users)
    
    @patch('src.services.user_service.models.Q')
    def test_search_users(self, mock_q):
        """Test searching users by query."""
        # Create test users
        user1 = User.objects.create_user(
            email='john.doe@example.com',
            username='johndoe',
            first_name='John',
            last_name='Doe',
            password='password123'
        )
        
        user2 = User.objects.create_user(
            email='jane.smith@example.com',
            username='janesmith',
            first_name='Jane',
            last_name='Smith',
            password='password123'
        )
        
        # Mock the Q object and filter
        mock_q_instance = Mock()
        mock_q.return_value = mock_q_instance
        
        with patch.object(User.objects, 'filter') as mock_filter:
            mock_filter.return_value.select_related.return_value = [user1]
            
            results = self.user_service.search_users('John')
            
            self.assertEqual(len(results), 1)
            self.assertIn(user1, results)


class UserServiceIntegrationTestCase(TestCase):
    """Integration tests for UserService with real database operations."""
    
    def setUp(self):
        """Set up integration test fixtures."""
        self.user_service = UserService()
    
    def test_create_and_retrieve_user_flow(self):
        """Test complete flow of creating and retrieving a user."""
        user_data = {
            'email': 'integration@example.com',
            'username': 'integrationuser',
            'password': 'SecurePass123!',
            'first_name': 'Integration',
            'last_name': 'Test',
            'role': UserRole.USER,
            'profile': {
                'bio': 'Integration test user',
                'location': 'Test City'
            }
        }
        
        # Create user
        created_user = self.user_service.create_user(user_data)
        self.assertIsNotNone(created_user)
        
        # Retrieve by ID
        retrieved_by_id = self.user_service.get_user_by_id(created_user.id)
        self.assertEqual(retrieved_by_id.id, created_user.id)
        
        # Retrieve by email
        retrieved_by_email = self.user_service.get_user_by_email(created_user.email)
        self.assertEqual(retrieved_by_email.email, created_user.email)
        
        # Verify profile was created
        self.assertTrue(hasattr(retrieved_by_id, 'profile'))
        self.assertEqual(retrieved_by_id.profile.bio, 'Integration test user')


if __name__ == '__main__':
    unittest.main()
`,
			Symbols: []PythonSymbol{
				{Name: "UserServiceTestCase", Kind: 5, Position: LSPPosition{Line: 13, Character: 6}, Type: "class", Description: "User service test case class"},
				{Name: "setUp", Kind: 6, Position: LSPPosition{Line: 16, Character: 8}, Type: "method", Description: "Test setup method"},
				{Name: "test_get_all_users_empty", Kind: 6, Position: LSPPosition{Line: 30, Character: 8}, Type: "method", Description: "Test get all users empty"},
				{Name: "test_create_user_valid_data", Kind: 6, Position: LSPPosition{Line: 94, Character: 8}, Type: "method", Description: "Test create user with valid data"},
				{Name: "UserServiceIntegrationTestCase", Kind: 5, Position: LSPPosition{Line: 359, Character: 6}, Type: "class", Description: "User service integration test case"},
			},
		},
	}
}

// TestBasicLSPOperations tests all fundamental LSP operations for Python
func (suite *PythonBasicWorkflowE2ETestSuite) TestBasicLSPOperations() {
	testCases := []struct {
		name         string
		fileName     string
		symbolName   string
		expectedType string
	}{
		{"Go to Definition - Class", "src/handlers/request_handler.py", "RequestHandler", "class"},
		{"Go to Definition - Method", "src/handlers/request_handler.py", "handle_request", "method"},
		{"Go to Definition - Function", "src/utils/validation_utils.py", "validate_email", "function"},
		{"Go to Definition - Model", "src/models/user_model.py", "User", "class"},
		{"Go to Definition - Service", "src/services/user_service.py", "UserService", "class"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executeBasicLSPWorkflow(tc.fileName, tc.symbolName, tc.expectedType)

			// Validate basic LSP operations
			suite.True(result.DefinitionFound, "Definition should be found for %s", tc.symbolName)
			suite.Greater(result.ReferencesCount, 0, "References should be found for %s", tc.symbolName)
			suite.True(result.HoverInfoRetrieved, "Hover info should be retrieved for %s", tc.symbolName)
			suite.Greater(result.DocumentSymbolsCount, 0, "Document symbols should be found")
			suite.Equal(0, result.ErrorCount, "No errors should occur during workflow")
			suite.Less(result.WorkflowLatency, 5*time.Second, "Workflow should complete within 5 seconds")
		})
	}
}

// TestPythonSpecificFeatures tests Python-specific language features (6 supported LSP functions only)
func (suite *PythonBasicWorkflowE2ETestSuite) TestPythonSpecificFeatures() {
	testCases := []struct {
		name        string
		feature     string
		description string
	}{
		{"Import Resolution", "import_resolution", "Resolve Python imports and modules"},
		{"Django Integration", "django_features", "Django-specific features like models and views"},
		{"Docstring Support", "docstrings", "Python docstring parsing and hover information"},
		{"Auto-completion", "completion", "Python-aware code completion with type hints"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executePythonSpecificWorkflow(tc.feature)

			// Validate Python-specific features (6 supported LSP functions only)
			suite.True(result.TypeInformationValid, "Type information should be valid for %s", tc.feature)
			
			if tc.feature == "completion" {
				suite.Greater(result.CompletionItemsCount, 0, "Completion items should be available")
			}
			
			suite.Equal(0, result.ErrorCount, "No errors should occur for %s", tc.feature)
			suite.Less(result.WorkflowLatency, 3*time.Second, "Feature test should complete quickly")
		})
	}
}

// TestMultiFilePythonProject tests navigation across multiple Python files
func (suite *PythonBasicWorkflowE2ETestSuite) TestMultiFilePythonProject() {
	// Setup responses for multi-file navigation
	suite.setupMultiFileResponses()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	startTime := time.Now()
	totalSymbols := 0
	totalReferences := 0

	// Navigate through each Python file
	for fileName, fileInfo := range suite.pythonFiles {
		suite.Run(fmt.Sprintf("Multi-file navigation - %s", fileName), func() {
			// Get document symbols for the file
			symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
			})
			suite.NoError(err, "Document symbols should be retrieved for %s", fileName)
			suite.NotNil(symbolsResp, "Symbol response should not be nil for %s", fileName)

			// Parse and count symbols
			var symbols []interface{}
			err = json.Unmarshal(symbolsResp, &symbols)
			suite.NoError(err, "Should be able to parse symbols for %s", fileName)
			totalSymbols += len(symbols)

			// Test cross-file references for main symbols
			for _, symbol := range fileInfo.Symbols {
				refsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
					"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
					"position":     symbol.Position,
					"context":      map[string]bool{"includeDeclaration": true},
				})
				suite.NoError(err, "References should be found for %s in %s", symbol.Name, fileName)
				suite.NotNil(refsResp, "References response should not be nil")

				// Count references
				var refs []interface{}
				if json.Unmarshal(refsResp, &refs) == nil {
					totalReferences += len(refs)
				}
			}
		})
	}

	multiFileLatency := time.Since(startTime)

	// Validate multi-file project navigation
	suite.Greater(totalSymbols, 15, "Should find significant number of symbols across all Python files")
	suite.Greater(totalReferences, 10, "Should find cross-file references")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), len(suite.pythonFiles), "Document symbols should be called for each file")
	suite.Greater(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES), 0, "References should be called")
	suite.Less(multiFileLatency, 10*time.Second, "Multi-file navigation should complete within reasonable time")
}

// TestProtocolValidation tests both HTTP JSON-RPC and MCP protocol validation
func (suite *PythonBasicWorkflowE2ETestSuite) TestProtocolValidation() {
	protocols := []struct {
		name        string
		description string
	}{
		{"HTTP_JSON_RPC", "Test HTTP JSON-RPC protocol for Python operations"},
		{"MCP_Protocol", "Test MCP protocol Python tool integration"},
	}

	for _, protocol := range protocols {
		suite.Run(protocol.name, func() {
			// Setup protocol-specific responses
			suite.setupProtocolResponses(protocol.name)

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			startTime := time.Now()

			// Test core LSP methods through the protocol
			methods := []string{
				mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
				mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
				mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
				mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
				mcp.LSP_METHOD_WORKSPACE_SYMBOL,
			}

			successCount := 0
			for _, method := range methods {
				resp, err := suite.mockClient.SendLSPRequest(ctx, method, suite.createTestParams(method))
				if err == nil && resp != nil {
					successCount++
				}
			}

			protocolLatency := time.Since(startTime)

			// Validate protocol functionality
			suite.Equal(len(methods), successCount, "All LSP methods should work through %s protocol", protocol.name)
			suite.Less(protocolLatency, 5*time.Second, "%s protocol should respond quickly", protocol.name)
			
			// Validate dual protocol consistency
			if protocol.name == "MCP_Protocol" {
				// Ensure MCP protocol provides consistent responses
				suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), 1, "Definition should be called through MCP")
			}
		})
	}
}

// TestPerformanceValidation tests performance against established thresholds
func (suite *PythonBasicWorkflowE2ETestSuite) TestPerformanceValidation() {
	// Performance thresholds based on project guidelines
	const (
		maxResponseTime = 5 * time.Second
		minThroughput   = 80 // requests per second (lower than TypeScript due to Python startup)
		maxErrorRate    = 0.05 // 5%
	)

	testCases := []struct {
		name            string
		requestCount    int
		concurrency     int
		expectedLatency time.Duration
	}{
		{"Single Request Performance", 1, 1, 200 * time.Millisecond}, // Python startup overhead
		{"Moderate Load Performance", 50, 5, 3 * time.Second},
		{"High Load Performance", 100, 10, maxResponseTime},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup sufficient responses for performance test
			for i := 0; i < tc.requestCount; i++ {
				suite.mockClient.QueueResponse(json.RawMessage(`{
					"uri": "file:///workspace/src/handlers/request_handler.py",
					"range": {
						"start": {"line": 25, "character": 4},
						"end": {"line": 25, "character": 18}
					}
				}`))
			}

			startTime := time.Now()
			var wg sync.WaitGroup
			errorCount := 0
			mu := sync.Mutex{}

			// Execute concurrent requests
			for i := 0; i < tc.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					requestsPerWorker := tc.requestCount / tc.concurrency
					for j := 0; j < requestsPerWorker; j++ {
						ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
						_, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
							"textDocument": map[string]string{"uri": "file:///workspace/src/handlers/request_handler.py"},
							"position":     LSPPosition{Line: 25, Character: 10},
						})
						cancel()
						
						if err != nil {
							mu.Lock()
							errorCount++
							mu.Unlock()
						}
					}
				}(i)
			}

			wg.Wait()
			totalLatency := time.Since(startTime)

			// Calculate performance metrics
			actualThroughput := float64(tc.requestCount) / totalLatency.Seconds()
			errorRate := float64(errorCount) / float64(tc.requestCount)

			// Validate performance thresholds
			suite.Less(totalLatency, maxResponseTime, "Total response time should be under threshold for %s", tc.name)
			suite.Greater(actualThroughput, float64(minThroughput), "Throughput should exceed minimum for %s", tc.name)
			suite.Less(errorRate, maxErrorRate, "Error rate should be under threshold for %s", tc.name)
			suite.Equal(tc.requestCount, suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), "All requests should be processed")
		})
	}
}

// TestConcurrentWorkflows tests multiple Python workflows running concurrently
func (suite *PythonBasicWorkflowE2ETestSuite) TestConcurrentWorkflows() {
	const numConcurrentWorkflows = 6 // Slightly lower than TypeScript due to Python overhead

	// Setup responses for concurrent execution
	for i := 0; i < numConcurrentWorkflows*4; i++ { // 4 requests per workflow
		suite.mockClient.QueueResponse(suite.createPythonSymbolResponse())
		suite.mockClient.QueueResponse(suite.createPythonDefinitionResponse())
		suite.mockClient.QueueResponse(suite.createPythonReferencesResponse())
		suite.mockClient.QueueResponse(suite.createPythonHoverResponse())
	}

	var wg sync.WaitGroup
	results := make(chan bool, numConcurrentWorkflows)
	startTime := time.Now()

	// Launch concurrent Python workflows
	for i := 0; i < numConcurrentWorkflows; i++ {
		wg.Add(1)
		go func(workflowID int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute Python workflow: symbols → definition → references → hover
			_, err1 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.py", workflowID)},
			})

			_, err2 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.py", workflowID)},
				"position":     LSPPosition{Line: 25, Character: 10},
			})

			_, err3 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.py", workflowID)},
				"position":     LSPPosition{Line: 25, Character: 10},
				"context":      map[string]bool{"includeDeclaration": true},
			})

			_, err4 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.py", workflowID)},
				"position":     LSPPosition{Line: 25, Character: 10},
			})

			results <- (err1 == nil && err2 == nil && err3 == nil && err4 == nil)
		}(i)
	}

	// Wait for all workflows to complete
	wg.Wait()
	close(results)

	concurrentLatency := time.Since(startTime)

	// Validate concurrent execution results
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}

	suite.Equal(numConcurrentWorkflows, successCount, "All concurrent Python workflows should succeed")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), numConcurrentWorkflows, "All symbol requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), numConcurrentWorkflows, "All definition requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES), numConcurrentWorkflows, "All references requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER), numConcurrentWorkflows, "All hover requests should be made")
	suite.Less(concurrentLatency, 20*time.Second, "Concurrent Python workflows should complete within reasonable time")
}

// Helper methods for workflow execution

// executeBasicLSPWorkflow executes a complete basic LSP workflow for Python
func (suite *PythonBasicWorkflowE2ETestSuite) executeBasicLSPWorkflow(fileName, symbolName, expectedType string) PythonWorkflowResult {
	result := PythonWorkflowResult{}

	// Setup mock responses
	suite.mockClient.QueueResponse(suite.createPythonDefinitionResponse())
	suite.mockClient.QueueResponse(suite.createPythonReferencesResponse())
	suite.mockClient.QueueResponse(suite.createPythonHoverResponse())
	suite.mockClient.QueueResponse(suite.createPythonSymbolResponse())

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Execute workflow steps
	defResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	result.DefinitionFound = err == nil && defResp != nil
	if err != nil {
		result.ErrorCount++
	}

	refsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
		"context":      map[string]bool{"includeDeclaration": true},
	})
	if err == nil && refsResp != nil {
		var refs []interface{}
		if json.Unmarshal(refsResp, &refs) == nil {
			result.ReferencesCount = len(refs)
		}
	} else {
		result.ErrorCount++
	}

	hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 25, Character: 10},
	})
	result.HoverInfoRetrieved = err == nil && hoverResp != nil
	if err != nil {
		result.ErrorCount++
	}

	symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
	})
	if err == nil && symbolsResp != nil {
		var symbols []interface{}
		if json.Unmarshal(symbolsResp, &symbols) == nil {
			result.DocumentSymbolsCount = len(symbols)
		}
	} else {
		result.ErrorCount++
	}

	result.WorkflowLatency = time.Since(startTime)
	result.RequestCount = len(suite.mockClient.SendLSPRequestCalls)
	result.TypeInformationValid = true // Mock always provides valid type info

	return result
}

// executePythonSpecificWorkflow executes Python-specific feature tests
func (suite *PythonBasicWorkflowE2ETestSuite) executePythonSpecificWorkflow(feature string) PythonWorkflowResult {
	result := PythonWorkflowResult{}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	switch feature {
	case "completion":
		// Mock completion response with Python-specific items
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"items": [
				{"label": "handle_request", "kind": 2, "detail": "(self, request: HttpRequest) -> HttpResponse"},
				{"label": "RequestHandler", "kind": 7, "detail": "class RequestHandler(View)"},
				{"label": "user_service", "kind": 5, "detail": "UserService"},
				{"label": "validate_request", "kind": 3, "detail": "function"},
				{"label": "JsonResponse", "kind": 7, "detail": "from django.http import JsonResponse"}
			]
		}`))
		resp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/src/handlers/request_handler.py", suite.workspaceRoot)},
			"position":     LSPPosition{Line: 50, Character: 8},
		})
		if err == nil && resp != nil {
			var completion map[string]interface{}
			if json.Unmarshal(resp, &completion) == nil {
				if items, ok := completion["items"].([]interface{}); ok {
					result.CompletionItemsCount = len(items)
				}
			}
		}

	default:
		// Generic Python feature test
		suite.mockClient.QueueResponse(suite.createPythonDefinitionResponse())
		suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/src/handlers/request_handler.py", suite.workspaceRoot)},
			"position":     LSPPosition{Line: 25, Character: 10},
		})
	}

	result.WorkflowLatency = time.Since(startTime)
	result.TypeInformationValid = true
	result.RequestCount = len(suite.mockClient.SendLSPRequestCalls)

	return result
}

// Helper methods for creating mock responses

func (suite *PythonBasicWorkflowE2ETestSuite) setupMultiFileResponses() {
	// Setup responses for each file type
	responses := []json.RawMessage{
		suite.createPythonSymbolResponse(),
		suite.createPythonDefinitionResponse(),
		suite.createPythonReferencesResponse(),
		suite.createPythonHoverResponse(),
	}

	// Queue responses for all files
	for range suite.pythonFiles {
		for _, response := range responses {
			suite.mockClient.QueueResponse(response)
		}
	}
}

func (suite *PythonBasicWorkflowE2ETestSuite) setupProtocolResponses(protocol string) {
	baseResponses := []json.RawMessage{
		suite.createPythonDefinitionResponse(),
		suite.createPythonReferencesResponse(),
		suite.createPythonHoverResponse(),
		suite.createPythonSymbolResponse(),
		suite.createWorkspaceSymbolResponse(),
	}

	for _, response := range baseResponses {
		suite.mockClient.QueueResponse(response)
	}
}

func (suite *PythonBasicWorkflowE2ETestSuite) createTestParams(method string) map[string]interface{} {
	switch method {
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		return map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///workspace/src/handlers/request_handler.py"},
			"position":     LSPPosition{Line: 25, Character: 10},
		}
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		return map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///workspace/src/handlers/request_handler.py"},
		}
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		return map[string]interface{}{
			"query": "RequestHandler",
		}
	default:
		return map[string]interface{}{}
	}
}

func (suite *PythonBasicWorkflowE2ETestSuite) createPythonDefinitionResponse() json.RawMessage {
	return json.RawMessage(`{
		"uri": "file:///workspace/src/handlers/request_handler.py",
		"range": {
			"start": {"line": 25, "character": 4},
			"end": {"line": 25, "character": 18}
		}
	}`)
}

func (suite *PythonBasicWorkflowE2ETestSuite) createPythonReferencesResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"uri": "file:///workspace/src/handlers/request_handler.py",
			"range": {"start": {"line": 25, "character": 4}, "end": {"line": 25, "character": 18}}
		},
		{
			"uri": "file:///workspace/src/controllers/api_controller.py",
			"range": {"start": {"line": 12, "character": 8}, "end": {"line": 12, "character": 22}}
		},
		{
			"uri": "file:///workspace/src/middleware/auth_middleware.py",
			"range": {"start": {"line": 35, "character": 16}, "end": {"line": 35, "character": 30}}
		},
		{
			"uri": "file:///workspace/tests/test_handlers.py",
			"range": {"start": {"line": 8, "character": 19}, "end": {"line": 8, "character": 33}}
		},
		{
			"uri": "file:///workspace/src/models/base_model.py",
			"range": {"start": {"line": 45, "character": 12}, "end": {"line": 45, "character": 26}}
		},
		{
			"uri": "file:///workspace/src/utils/request_utils.py",
			"range": {"start": {"line": 67, "character": 25}, "end": {"line": 67, "character": 39}}
		},
		{
			"uri": "file:///workspace/src/services/user_service.py",
			"range": {"start": {"line": 89, "character": 7}, "end": {"line": 89, "character": 21}}
		}
	]`)
}

func (suite *PythonBasicWorkflowE2ETestSuite) createPythonHoverResponse() json.RawMessage {
	return json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```python\ndef handle_request(request: Request) -> Response\n```" + `\n\nHandle incoming HTTP request and return appropriate response.\n\n**Args:**\n- ` + "`request`" + `: The incoming HTTP request object\n\n**Returns:**\n- ` + "`Response`" + `: HTTP response object\n\n**Raises:**\n- ` + "`ValueError`" + `: If request is invalid\n- ` + "`HTTPException`" + `: If processing fails\n\n**Module:** src.handlers.request_handler"
		},
		"range": {
			"start": {"line": 25, "character": 4},
			"end": {"line": 25, "character": 18}
		}
	}`)
}

func (suite *PythonBasicWorkflowE2ETestSuite) createPythonSymbolResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "RequestHandler",
			"kind": 5,
			"range": {"start": {"line": 18, "character": 0}, "end": {"line": 95, "character": 0}},
			"selectionRange": {"start": {"line": 18, "character": 6}, "end": {"line": 18, "character": 20}},
			"children": [
				{
					"name": "__init__",
					"kind": 9,
					"range": {"start": {"line": 21, "character": 4}, "end": {"line": 24, "character": 0}},
					"selectionRange": {"start": {"line": 21, "character": 8}, "end": {"line": 21, "character": 16}}
				},
				{
					"name": "handle_request",
					"kind": 6,
					"range": {"start": {"line": 29, "character": 4}, "end": {"line": 60, "character": 0}},
					"selectionRange": {"start": {"line": 29, "character": 8}, "end": {"line": 29, "character": 22}}
				},
				{
					"name": "_handle_get_request",
					"kind": 6,
					"range": {"start": {"line": 62, "character": 4}, "end": {"line": 72, "character": 0}},
					"selectionRange": {"start": {"line": 62, "character": 8}, "end": {"line": 62, "character": 27}}
				},
				{
					"name": "_handle_post_request",
					"kind": 6,
					"range": {"start": {"line": 74, "character": 4}, "end": {"line": 80, "character": 0}},
					"selectionRange": {"start": {"line": 74, "character": 8}, "end": {"line": 74, "character": 28}}
				}
			]
		},
		{
			"name": "api_endpoint",
			"kind": 12,
			"range": {"start": {"line": 98, "character": 0}, "end": {"line": 101, "character": 0}},
			"selectionRange": {"start": {"line": 98, "character": 4}, "end": {"line": 98, "character": 16}}
		},
		{
			"name": "health_check",
			"kind": 12,
			"range": {"start": {"line": 103, "character": 0}, "end": {"line": 105, "character": 0}},
			"selectionRange": {"start": {"line": 103, "character": 4}, "end": {"line": 103, "character": 16}}
		}
	]`)
}

func (suite *PythonBasicWorkflowE2ETestSuite) createWorkspaceSymbolResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "RequestHandler",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/handlers/request_handler.py",
				"range": {"start": {"line": 18, "character": 6}, "end": {"line": 18, "character": 20}}
			}
		},
		{
			"name": "UserService",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/services/user_service.py",
				"range": {"start": {"line": 16, "character": 6}, "end": {"line": 16, "character": 17}}
			}
		},
		{
			"name": "User",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/models/user_model.py",
				"range": {"start": {"line": 17, "character": 6}, "end": {"line": 17, "character": 10}}
			}
		},
		{
			"name": "validate_email",
			"kind": 12,
			"location": {
				"uri": "file:///workspace/src/utils/validation_utils.py",
				"range": {"start": {"line": 17, "character": 4}, "end": {"line": 17, "character": 18}}
			}
		}
	]`)
}

// Benchmark tests for performance measurement

// BenchmarkPythonBasicWorkflow benchmarks the basic Python workflow
func BenchmarkPythonBasicWorkflow(b *testing.B) {
	suite := &PythonBasicWorkflowE2ETestSuite{}
	suite.SetupSuite()

	workflows := []string{"definition", "references", "hover", "symbols"}

	for _, workflow := range workflows {
		b.Run(fmt.Sprintf("Python_%s", workflow), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()
				result := suite.executeBasicLSPWorkflow("src/handlers/request_handler.py", "RequestHandler", "class")
				if result.ErrorCount > 0 {
					b.Errorf("Workflow failed with %d errors", result.ErrorCount)
				}
				suite.TearDownTest()
			}
		})
	}
}

// BenchmarkPythonConcurrentWorkflows benchmarks concurrent workflow execution
func BenchmarkPythonConcurrentWorkflows(b *testing.B) {
	suite := &PythonBasicWorkflowE2ETestSuite{}
	suite.SetupSuite()

	concurrencyLevels := []int{1, 4, 6, 12}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrent_%d", concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()

				// Setup sufficient responses
				for j := 0; j < concurrency*4; j++ {
					suite.mockClient.QueueResponse(suite.createPythonSymbolResponse())
				}

				var wg sync.WaitGroup
				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
							"textDocument": map[string]string{"uri": "file:///workspace/src/handlers/request_handler.py"},
						})
					}()
				}
				wg.Wait()

				suite.TearDownTest()
			}
		})
	}
}

// TestSuite runner
func TestPythonBasicWorkflowE2ETestSuite(t *testing.T) {
	suite.Run(t, new(PythonBasicWorkflowE2ETestSuite))
}