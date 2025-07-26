package framework

import (
	"fmt"
)

// Content generation methods for different languages and project types

// Go content generators

func (g *TestProjectGenerator) generateGoMod(projectName string) string {
	return fmt.Sprintf(`module %s

go 1.19

require (
	github.com/gin-gonic/gin v1.9.1
	gorm.io/gorm v1.25.0
	gorm.io/driver/sqlite v1.5.0
)
`, projectName)
}

func (g *TestProjectGenerator) generateGoMain() string {
	return `package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Hello, World!")
	log.Println("Application started successfully")
}
`
}

func (g *TestProjectGenerator) generateGoServerMain() string {
	return `package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	
	r.GET("/api/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"users": []string{"user1", "user2"}})
	})
	
	log.Println("Server starting on :8080")
	r.Run(":8080")
}
`
}

func (g *TestProjectGenerator) generateGoUtils() string {
	return `package utils

import (
	"crypto/md5"
	"fmt"
	"strings"
)

// StringToMD5 converts a string to its MD5 hash
func StringToMD5(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// CamelToSnake converts camelCase to snake_case
func CamelToSnake(str string) string {
	var result strings.Builder
	for i, r := range str {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// Contains checks if slice contains item
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
`
}

func (g *TestProjectGenerator) generateGoService() string {
	return `package internal

import (
	"errors"
	"fmt"
)

type Service struct {
	name string
}

func NewService(name string) *Service {
	return &Service{name: name}
}

func (s *Service) Process(data string) (string, error) {
	if data == "" {
		return "", errors.New("data cannot be empty")
	}
	
	return fmt.Sprintf("Processed by %s: %s", s.name, data), nil
}

func (s *Service) GetName() string {
	return s.name
}
`
}

func (g *TestProjectGenerator) generateGoUserModel() string {
	return `package models

import (
	"time"
	"gorm.io/gorm"
)

type User struct {
	ID        uint           ` + "`gorm:\"primarykey\" json:\"id\"`" + `
	CreatedAt time.Time      ` + "`json:\"created_at\"`" + `
	UpdatedAt time.Time      ` + "`json:\"updated_at\"`" + `
	DeletedAt gorm.DeletedAt ` + "`gorm:\"index\" json:\"deleted_at,omitempty\"`" + `
	
	Name     string ` + "`gorm:\"not null\" json:\"name\"`" + `
	Email    string ` + "`gorm:\"uniqueIndex;not null\" json:\"email\"`" + `
	Password string ` + "`gorm:\"not null\" json:\"-\"`" + `
	IsActive bool   ` + "`gorm:\"default:true\" json:\"is_active\"`" + `
}

func (u *User) TableName() string {
	return "users"
}
`
}

func (g *TestProjectGenerator) generateGoUserService() string {
	return `package service

import (
	"errors"
	"fmt"
	"../models"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) CreateUser(user *models.User) error {
	if user.Name == "" {
		return errors.New("name is required")
	}
	if user.Email == "" {
		return errors.New("email is required")
	}
	
	return s.db.Create(user).Error
}

func (s *UserService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	err := s.db.First(&user, id).Error
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (s *UserService) GetAllUsers() ([]models.User, error) {
	var users []models.User
	err := s.db.Find(&users).Error
	return users, err
}

func (s *UserService) UpdateUser(user *models.User) error {
	return s.db.Save(user).Error
}

func (s *UserService) DeleteUser(id uint) error {
	return s.db.Delete(&models.User{}, id).Error
}
`
}

func (g *TestProjectGenerator) generateGoUserHandler() string {
	return `package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"../models"
	"../service"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.userService.CreateUser(&user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}
	
	user, err := h.userService.GetUserByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetAllUsers(c *gin.Context) {
	users, err := h.userService.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, users)
}
`
}

func (g *TestProjectGenerator) generateGoDatabase() string {
	return `package database

import (
	"log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	var err error
	DB, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		return err
	}
	
	log.Println("Database connection established")
	return nil
}

func GetDB() *gorm.DB {
	return DB
}

func CloseDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
`
}

func (g *TestProjectGenerator) generateGoRoutes() string {
	return `package api

import (
	"github.com/gin-gonic/gin"
	"../internal/handlers"
	"../internal/service"
	"../internal/database"
)

func SetupRoutes() *gin.Engine {
	r := gin.Default()
	
	// Initialize services
	userService := service.NewUserService(database.GetDB())
	userHandler := handlers.NewUserHandler(userService)
	
	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})
	
	// API routes
	api := r.Group("/api/v1")
	{
		users := api.Group("/users")
		{
			users.POST("/", userHandler.CreateUser)
			users.GET("/", userHandler.GetAllUsers)
			users.GET("/:id", userHandler.GetUser)
		}
	}
	
	return r
}
`
}

// Python content generators

func (g *TestProjectGenerator) generatePythonSetup(projectName string) string {
	return fmt.Sprintf(`from setuptools import setup, find_packages

setup(
    name="%s",
    version="1.0.0",
    description="A test Python project",
    author="Test Author",
    author_email="test@example.com",
    packages=find_packages(),
    install_requires=[
        "flask>=2.0.0",
        "requests>=2.25.0",
        "sqlalchemy>=1.4.0",
        "pytest>=6.0.0",
    ],
    python_requires=">=3.8",
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
    ],
)
`, projectName)
}

func (g *TestProjectGenerator) generatePythonPyproject(projectName string) string {
	return fmt.Sprintf(`[build-system]
requires = ["setuptools>=61.0", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "%s"
version = "1.0.0"
description = "A test Python project"
authors = [
    {name = "Test Author", email = "test@example.com"},
]
dependencies = [
    "flask>=2.0.0",
    "requests>=2.25.0",
    "sqlalchemy>=1.4.0",
    "pydantic>=1.8.0",
]
requires-python = ">=3.8"
classifiers = [
    "Development Status :: 3 - Alpha",
    "Intended Audience :: Developers",
    "License :: OSI Approved :: MIT License",
    "Programming Language :: Python :: 3.8",
    "Programming Language :: Python :: 3.9",
    "Programming Language :: Python :: 3.10",
]

[project.optional-dependencies]
test = [
    "pytest>=6.0.0",
    "pytest-cov>=2.12.0",
    "black>=21.0.0",
    "flake8>=3.9.0",
]
dev = [
    "pre-commit>=2.15.0",
    "mypy>=0.910",
]

[tool.black]
line-length = 88
target-version = ['py38']

[tool.mypy]
python_version = "3.8"
warn_return_any = true
warn_unused_configs = true
`, projectName)
}

func (g *TestProjectGenerator) generatePythonRequirements() string {
	return `flask==2.3.2
requests==2.31.0
sqlalchemy==2.0.19
pydantic==2.0.3
pytest==7.4.0
pytest-cov==4.1.0
black==23.7.0
flake8==6.0.0
mypy==1.4.1
`
}

func (g *TestProjectGenerator) generatePythonMain() string {
	return `#!/usr/bin/env python3
"""
Main entry point for the Python application.
"""

import logging
import sys
from typing import Optional

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def main(args: Optional[list] = None) -> int:
    """Main function."""
    if args is None:
        args = sys.argv[1:]
    
    logger.info("Application starting...")
    
    try:
        print("Hello, World!")
        logger.info("Application completed successfully")
        return 0
    except Exception as e:
        logger.error(f"Application failed: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
`
}

func (g *TestProjectGenerator) generatePythonUtils() string {
	return `"""
Utility functions for the Python application.
"""

import hashlib
import re
from typing import List, Optional, Any, Dict


def string_to_md5(text: str) -> str:
    """Convert a string to its MD5 hash."""
    return hashlib.md5(text.encode()).hexdigest()


def camel_to_snake(name: str) -> str:
    """Convert camelCase to snake_case."""
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()


def snake_to_camel(name: str) -> str:
    """Convert snake_case to camelCase."""
    components = name.split('_')
    return components[0] + ''.join(x.capitalize() for x in components[1:])


def contains(items: List[Any], item: Any) -> bool:
    """Check if list contains item."""
    return item in items


def safe_get(dictionary: Dict[str, Any], key: str, default: Any = None) -> Any:
    """Safely get value from dictionary."""
    return dictionary.get(key, default)


def validate_email(email: str) -> bool:
    """Validate email format."""
    pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
    return re.match(pattern, email) is not None


def chunk_list(lst: List[Any], chunk_size: int) -> List[List[Any]]:
    """Split list into chunks of specified size."""
    return [lst[i:i + chunk_size] for i in range(0, len(lst), chunk_size)]
`
}

func (g *TestProjectGenerator) generatePythonModels() string {
	return `"""
Data models for the Python application.
"""

from dataclasses import dataclass, field
from datetime import datetime
from typing import Optional, List
from sqlalchemy import Column, Integer, String, Boolean, DateTime
from sqlalchemy.ext.declarative import declarative_base

Base = declarative_base()


@dataclass
class UserData:
    """User data class."""
    name: str
    email: str
    is_active: bool = True
    created_at: datetime = field(default_factory=datetime.now)
    id: Optional[int] = None


class User(Base):
    """User SQLAlchemy model."""
    __tablename__ = 'users'
    
    id = Column(Integer, primary_key=True, autoincrement=True)
    name = Column(String(100), nullable=False)
    email = Column(String(255), unique=True, nullable=False)
    is_active = Column(Boolean, default=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    
    def to_dict(self) -> dict:
        """Convert model to dictionary."""
        return {
            'id': self.id,
            'name': self.name,
            'email': self.email,
            'is_active': self.is_active,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None,
        }
    
    @classmethod
    def from_dict(cls, data: dict) -> 'User':
        """Create model from dictionary."""
        return cls(
            name=data['name'],
            email=data['email'],
            is_active=data.get('is_active', True)
        )
`
}

func (g *TestProjectGenerator) generatePythonUserModel() string {
	return `"""
User model and related functionality.
"""

from datetime import datetime
from typing import Optional, Dict, Any
from pydantic import BaseModel, EmailStr, validator
from sqlalchemy import Column, Integer, String, Boolean, DateTime
from sqlalchemy.ext.declarative import declarative_base

Base = declarative_base()


class UserCreate(BaseModel):
    """User creation schema."""
    name: str
    email: EmailStr
    password: str
    is_active: bool = True
    
    @validator('name')
    def validate_name(cls, v):
        if len(v.strip()) < 2:
            raise ValueError('Name must be at least 2 characters long')
        return v.strip()
    
    @validator('password')
    def validate_password(cls, v):
        if len(v) < 6:
            raise ValueError('Password must be at least 6 characters long')
        return v


class UserUpdate(BaseModel):
    """User update schema."""
    name: Optional[str] = None
    email: Optional[EmailStr] = None
    is_active: Optional[bool] = None


class UserResponse(BaseModel):
    """User response schema."""
    id: int
    name: str
    email: EmailStr
    is_active: bool
    created_at: datetime
    updated_at: datetime
    
    class Config:
        orm_mode = True


class User(Base):
    """User database model."""
    __tablename__ = 'users'
    
    id = Column(Integer, primary_key=True, autoincrement=True)
    name = Column(String(100), nullable=False)
    email = Column(String(255), unique=True, nullable=False)
    password_hash = Column(String(255), nullable=False)
    is_active = Column(Boolean, default=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary."""
        return {
            'id': self.id,
            'name': self.name,
            'email': self.email,
            'is_active': self.is_active,
            'created_at': self.created_at,
            'updated_at': self.updated_at,
        }
    
    def __repr__(self) -> str:
        return f"<User(id={self.id}, name='{self.name}', email='{self.email}')>"
`
}

func (g *TestProjectGenerator) generatePythonUserService() string {
	return `"""
User service layer.
"""

from typing import List, Optional, Dict, Any
from sqlalchemy.orm import Session
from sqlalchemy.exc import IntegrityError
import bcrypt

from .models.user import User, UserCreate, UserUpdate, UserResponse


class UserService:
    """Service for user operations."""
    
    def __init__(self, db_session: Session):
        self.db = db_session
    
    def create_user(self, user_data: UserCreate) -> UserResponse:
        """Create a new user."""
        # Hash password
        password_hash = bcrypt.hashpw(
            user_data.password.encode('utf-8'), 
            bcrypt.gensalt()
        ).decode('utf-8')
        
        db_user = User(
            name=user_data.name,
            email=user_data.email,
            password_hash=password_hash,
            is_active=user_data.is_active
        )
        
        try:
            self.db.add(db_user)
            self.db.commit()
            self.db.refresh(db_user)
            return UserResponse.from_orm(db_user)
        except IntegrityError:
            self.db.rollback()
            raise ValueError(f"User with email {user_data.email} already exists")
    
    def get_user_by_id(self, user_id: int) -> Optional[UserResponse]:
        """Get user by ID."""
        db_user = self.db.query(User).filter(User.id == user_id).first()
        if db_user:
            return UserResponse.from_orm(db_user)
        return None
    
    def get_user_by_email(self, email: str) -> Optional[UserResponse]:
        """Get user by email."""
        db_user = self.db.query(User).filter(User.email == email).first()
        if db_user:
            return UserResponse.from_orm(db_user)
        return None
    
    def get_all_users(self, skip: int = 0, limit: int = 100) -> List[UserResponse]:
        """Get all users with pagination."""
        db_users = self.db.query(User).offset(skip).limit(limit).all()
        return [UserResponse.from_orm(user) for user in db_users]
    
    def update_user(self, user_id: int, user_data: UserUpdate) -> Optional[UserResponse]:
        """Update user."""
        db_user = self.db.query(User).filter(User.id == user_id).first()
        if not db_user:
            return None
        
        update_data = user_data.dict(exclude_unset=True)
        for field, value in update_data.items():
            setattr(db_user, field, value)
        
        try:
            self.db.commit()
            self.db.refresh(db_user)
            return UserResponse.from_orm(db_user)
        except IntegrityError:
            self.db.rollback()
            raise ValueError("Email already exists")
    
    def delete_user(self, user_id: int) -> bool:
        """Delete user."""
        db_user = self.db.query(User).filter(User.id == user_id).first()
        if not db_user:
            return False
        
        self.db.delete(db_user)
        self.db.commit()
        return True
    
    def verify_password(self, user_id: int, password: str) -> bool:
        """Verify user password."""
        db_user = self.db.query(User).filter(User.id == user_id).first()
        if not db_user:
            return False
        
        return bcrypt.checkpw(
            password.encode('utf-8'),
            db_user.password_hash.encode('utf-8')
        )
`
}

func (g *TestProjectGenerator) generatePythonRoutes() string {
	return `"""
API routes for the Python application.
"""

from flask import Flask, request, jsonify
from flask.views import MethodView
from typing import Dict, Any, Optional
import logging

from ..services.user_service import UserService
from ..models.user import UserCreate, UserUpdate
from ..database.connection import get_db_session

logger = logging.getLogger(__name__)


class UserAPI(MethodView):
    """User API endpoints."""
    
    def __init__(self):
        self.user_service = UserService(get_db_session())
    
    def get(self, user_id: Optional[int] = None) -> Dict[str, Any]:
        """Get user(s)."""
        if user_id is None:
            # Get all users
            skip = request.args.get('skip', 0, type=int)
            limit = request.args.get('limit', 100, type=int)
            users = self.user_service.get_all_users(skip=skip, limit=limit)
            return {
                'users': [user.dict() for user in users],
                'total': len(users)
            }
        else:
            # Get specific user
            user = self.user_service.get_user_by_id(user_id)
            if not user:
                return {'error': 'User not found'}, 404
            return user.dict()
    
    def post(self) -> Dict[str, Any]:
        """Create user."""
        try:
            data = request.get_json()
            if not data:
                return {'error': 'No data provided'}, 400
            
            user_data = UserCreate(**data)
            user = self.user_service.create_user(user_data)
            logger.info(f"Created user with ID: {user.id}")
            return user.dict(), 201
        except ValueError as e:
            return {'error': str(e)}, 400
        except Exception as e:
            logger.error(f"Error creating user: {e}")
            return {'error': 'Internal server error'}, 500
    
    def put(self, user_id: int) -> Dict[str, Any]:
        """Update user."""
        try:
            data = request.get_json()
            if not data:
                return {'error': 'No data provided'}, 400
            
            user_data = UserUpdate(**data)
            user = self.user_service.update_user(user_id, user_data)
            if not user:
                return {'error': 'User not found'}, 404
            
            logger.info(f"Updated user with ID: {user_id}")
            return user.dict()
        except ValueError as e:
            return {'error': str(e)}, 400
        except Exception as e:
            logger.error(f"Error updating user: {e}")
            return {'error': 'Internal server error'}, 500
    
    def delete(self, user_id: int) -> Dict[str, Any]:
        """Delete user."""
        try:
            success = self.user_service.delete_user(user_id)
            if not success:
                return {'error': 'User not found'}, 404
            
            logger.info(f"Deleted user with ID: {user_id}")
            return {'message': 'User deleted successfully'}
        except Exception as e:
            logger.error(f"Error deleting user: {e}")
            return {'error': 'Internal server error'}, 500


def create_app() -> Flask:
    """Create Flask application."""
    app = Flask(__name__)
    
    # Health check endpoint
    @app.route('/health', methods=['GET'])
    def health_check():
        return {'status': 'healthy', 'service': 'python-api'}
    
    # Register user API
    user_view = UserAPI.as_view('user_api')
    app.add_url_rule('/api/users', defaults={'user_id': None}, 
                     view_func=user_view, methods=['GET'])
    app.add_url_rule('/api/users', view_func=user_view, methods=['POST'])
    app.add_url_rule('/api/users/<int:user_id>', view_func=user_view,
                     methods=['GET', 'PUT', 'DELETE'])
    
    return app


if __name__ == '__main__':
    app = create_app()
    app.run(debug=True, host='0.0.0.0', port=5000)
`
}

func (g *TestProjectGenerator) generatePythonDatabase() string {
	return `"""
Database connection and session management.
"""

import os
from typing import Generator
from sqlalchemy import create_engine, MetaData
from sqlalchemy.orm import sessionmaker, Session
from sqlalchemy.pool import StaticPool
import logging

from ..models.user import Base

logger = logging.getLogger(__name__)

# Database configuration
DATABASE_URL = os.getenv('DATABASE_URL', 'sqlite:///./test.db')

# Create engine
engine = create_engine(
    DATABASE_URL,
    poolclass=StaticPool,
    connect_args={"check_same_thread": False} if 'sqlite' in DATABASE_URL else {},
    echo=os.getenv('DB_ECHO', 'false').lower() == 'true'
)

# Create session factory
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


def create_tables() -> None:
    """Create all database tables."""
    try:
        Base.metadata.create_all(bind=engine)
        logger.info("Database tables created successfully")
    except Exception as e:
        logger.error(f"Error creating tables: {e}")
        raise


def drop_tables() -> None:
    """Drop all database tables."""
    try:
        Base.metadata.drop_all(bind=engine)
        logger.info("Database tables dropped successfully")
    except Exception as e:
        logger.error(f"Error dropping tables: {e}")
        raise


def get_db_session() -> Generator[Session, None, None]:
    """Get database session."""
    db = SessionLocal()
    try:
        yield db
    except Exception as e:
        logger.error(f"Database session error: {e}")
        db.rollback()
        raise
    finally:
        db.close()


def init_db() -> None:
    """Initialize database."""
    create_tables()
    logger.info("Database initialized successfully")


def close_db() -> None:
    """Close database connections."""
    engine.dispose()
    logger.info("Database connections closed")


# Health check function
def check_db_health() -> bool:
    """Check database health."""
    try:
        with engine.connect() as conn:
            conn.execute("SELECT 1")
        return True
    except Exception as e:
        logger.error(f"Database health check failed: {e}")
        return False
`
}

// TypeScript content generators

func (g *TestProjectGenerator) generatePackageJson(projectName, language string) string {
	if language == "typescript" {
		return fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "description": "A test TypeScript project",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts",
    "test": "jest",
    "lint": "eslint src/**/*.ts",
    "format": "prettier --write src/**/*.ts"
  },
  "dependencies": {
    "express": "^4.18.2",
    "axios": "^1.4.0",
    "cors": "^2.8.5",
    "helmet": "^7.0.0",
    "dotenv": "^16.3.1"
  },
  "devDependencies": {
    "typescript": "^5.1.6",
    "@types/node": "^20.4.2",
    "@types/express": "^4.17.17",
    "@types/cors": "^2.8.13",
    "ts-node": "^10.9.1",
    "jest": "^29.6.1",
    "@types/jest": "^29.5.3",
    "ts-jest": "^29.1.1",
    "eslint": "^8.45.0",
    "@typescript-eslint/parser": "^6.2.0",
    "@typescript-eslint/eslint-plugin": "^6.2.0",
    "prettier": "^3.0.0"
  },
  "keywords": ["typescript", "node", "api"],
  "author": "Test Author",
  "license": "MIT"
}`, projectName)
	}

	return fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "description": "A test JavaScript project",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "test": "jest",
    "lint": "eslint .",
    "format": "prettier --write ."
  },
  "dependencies": {
    "express": "^4.18.2",
    "axios": "^1.4.0",
    "cors": "^2.8.5"
  },
  "devDependencies": {
    "jest": "^29.6.1",
    "eslint": "^8.45.0",
    "prettier": "^3.0.0"
  },
  "keywords": ["javascript", "node", "api"],
  "author": "Test Author",
  "license": "MIT"
}`, projectName)
}

func (g *TestProjectGenerator) generateTsConfig() string {
	return `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "removeComments": true,
    "noImplicitAny": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true
  },
  "include": [
    "src/**/*"
  ],
  "exclude": [
    "node_modules",
    "dist",
    "**/*.test.ts",
    "**/*.spec.ts"
  ]
}`
}

func (g *TestProjectGenerator) generateTypeScriptIndex() string {
	return `import express, { Application, Request, Response } from 'express';
import cors from 'cors';
import helmet from 'helmet';
import dotenv from 'dotenv';

// Load environment variables
dotenv.config();

const app: Application = express();
const port = process.env.PORT || 3000;

// Middleware
app.use(helmet());
app.use(cors());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

// Health check endpoint
app.get('/health', (req: Request, res: Response) => {
  res.status(200).json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    service: 'typescript-api'
  });
});

// Sample API endpoint
app.get('/api/hello', (req: Request, res: Response) => {
  const name = req.query.name as string || 'World';
  res.json({
    message: ` + "`Hello, ${name}!`" + `,
    timestamp: new Date().toISOString()
  });
});

// Error handling middleware
app.use((err: Error, req: Request, res: Response, next: any) => {
  console.error('Error:', err.message);
  res.status(500).json({
    error: 'Internal Server Error',
    message: process.env.NODE_ENV === 'development' ? err.message : 'Something went wrong'
  });
});

// 404 handler
app.use('*', (req: Request, res: Response) => {
  res.status(404).json({
    error: 'Not Found',
    message: ` + "`Route ${req.originalUrl} not found`" + `
  });
});

// Start server
app.listen(port, () => {
  console.log(` + "`Server is running on port ${port}`" + `);
  console.log(` + "`Environment: ${process.env.NODE_ENV || 'development'}`" + `);
});

export default app;
`
}

func (g *TestProjectGenerator) generateTypeScriptUtils() string {
	return `/**
 * Utility functions for the TypeScript application.
 */

import crypto from 'crypto';

/**
 * Convert string to MD5 hash
 */
export function stringToMd5(text: string): string {
  return crypto.createHash('md5').update(text).digest('hex');
}

/**
 * Convert camelCase to snake_case
 */
export function camelToSnake(str: string): string {
  return str.replace(/[A-Z]/g, letter => ` + "`_${letter.toLowerCase()}`" + `);
}

/**
 * Convert snake_case to camelCase
 */
export function snakeToCamel(str: string): string {
  return str.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
}

/**
 * Check if array contains item
 */
export function contains<T>(array: T[], item: T): boolean {
  return array.includes(item);
}

/**
 * Safe get from object with default value
 */
export function safeGet<T>(obj: Record<string, any>, key: string, defaultValue: T): T {
  return obj[key] !== undefined ? obj[key] : defaultValue;
}

/**
 * Validate email format
 */
export function validateEmail(email: string): boolean {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return emailRegex.test(email);
}

/**
 * Generate random string
 */
export function generateRandomString(length: number): string {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  return result;
}

/**
 * Chunk array into smaller arrays
 */
export function chunkArray<T>(array: T[], chunkSize: number): T[][] {
  const chunks: T[][] = [];
  for (let i = 0; i < array.length; i += chunkSize) {
    chunks.push(array.slice(i, i + chunkSize));
  }
  return chunks;
}

/**
 * Debounce function
 */
export function debounce<T extends (...args: any[]) => any>(
  func: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timeoutId: NodeJS.Timeout;
  
  return (...args: Parameters<T>) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => func.apply(null, args), delay);
  };
}

/**
 * Sleep/delay function
 */
export function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}
`
}

// Continue with more content generators...
// This file is getting quite long, so I'll break it here and continue in another file or add the remaining generators as needed.









// Additional helper methods are in the main test_project_generator.go file

