package scip

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

// convertRangeToInt32Array converts a SCIP Range to []int32 format
func convertRangeToInt32Array(r *scip.Range) []int32 {
	if r == nil {
		return nil
	}
	return []int32{
		int32(r.Start.Line),
		int32(r.Start.Character),
		int32(r.End.Line),
		int32(r.End.Character),
	}
}

// generateTestProject creates a synthetic test project for the specified language
func (suite *SCIPTestSuite) generateTestProject(language string) (*TestProject, error) {
	switch language {
	case "go":
		return suite.generateGoTestProject()
	case "typescript":
		return suite.generateTypeScriptTestProject()
	case "python":
		return suite.generatePythonTestProject()
	case "java":
		return suite.generateJavaTestProject()
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

// generateGoTestProject creates a comprehensive Go test project
func (suite *SCIPTestSuite) generateGoTestProject() (*TestProject, error) {
	project := &TestProject{
		Name:     "go-test-project",
		Language: "go",
		Files:    make(map[string]string),
		Symbols:  make([]*TestSymbol, 0),
		References: make([]*TestReference, 0),
	}
	
	// Main package file with various symbol types
	mainGoContent := `package main

import (
	"fmt"
	"context"
	"time"
)

// User represents a user in the system
type User struct {
	ID       int64     ` + "`json:\"id\"`" + `
	Name     string    ` + "`json:\"name\"`" + `
	Email    string    ` + "`json:\"email\"`" + `
	Created  time.Time ` + "`json:\"created\"`" + `
}

// UserService provides user management functionality
type UserService interface {
	GetUser(ctx context.Context, id int64) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id int64) error
}

// DefaultUserService implements UserService
type DefaultUserService struct {
	users map[int64]*User
}

// NewUserService creates a new user service
func NewUserService() *DefaultUserService {
	return &DefaultUserService{
		users: make(map[int64]*User),
	}
}

// GetUser retrieves a user by ID
func (s *DefaultUserService) GetUser(ctx context.Context, id int64) (*User, error) {
	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found: %d", id)
	}
	return user, nil
}

// CreateUser creates a new user
func (s *DefaultUserService) CreateUser(ctx context.Context, user *User) error {
	if user.ID == 0 {
		user.ID = time.Now().Unix()
	}
	user.Created = time.Now()
	s.users[user.ID] = user
	return nil
}

// UpdateUser updates an existing user
func (s *DefaultUserService) UpdateUser(ctx context.Context, user *User) error {
	if _, exists := s.users[user.ID]; !exists {
		return fmt.Errorf("user not found: %d", user.ID)
	}
	s.users[user.ID] = user
	return nil
}

// DeleteUser removes a user
func (s *DefaultUserService) DeleteUser(ctx context.Context, id int64) error {
	delete(s.users, id)
	return nil
}

func main() {
	service := NewUserService()
	
	user := &User{
		Name:  "John Doe",
		Email: "john.doe@example.com",
	}
	
	ctx := context.Background()
	if err := service.CreateUser(ctx, user); err != nil {
		panic(err)
	}
	
	fmt.Printf("Created user: %+v\n", user)
}
`

	// Test file
	mainTestContent := `package main

import (
	"context"
	"testing"
	"time"
)

func TestUserService_CreateUser(t *testing.T) {
	service := NewUserService()
	ctx := context.Background()
	
	user := &User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	
	err := service.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	
	if user.ID == 0 {
		t.Error("User ID should be set after creation")
	}
	
	if user.Created.IsZero() {
		t.Error("User Created time should be set")
	}
}

func TestUserService_GetUser(t *testing.T) {
	service := NewUserService()
	ctx := context.Background()
	
	originalUser := &User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	
	err := service.CreateUser(ctx, originalUser)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	
	retrievedUser, err := service.GetUser(ctx, originalUser.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	
	if retrievedUser.Name != originalUser.Name {
		t.Errorf("Expected name %s, got %s", originalUser.Name, retrievedUser.Name)
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	service := NewUserService()
	ctx := context.Background()
	
	user := &User{
		Name:  "Original Name",
		Email: "original@example.com",
	}
	
	err := service.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	
	user.Name = "Updated Name"
	user.Email = "updated@example.com"
	
	err = service.UpdateUser(ctx, user)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	
	updatedUser, err := service.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	
	if updatedUser.Name != "Updated Name" {
		t.Errorf("Expected updated name, got %s", updatedUser.Name)
	}
}

func TestUserService_DeleteUser(t *testing.T) {
	service := NewUserService()
	ctx := context.Background()
	
	user := &User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	
	err := service.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	
	err = service.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	
	_, err = service.GetUser(ctx, user.ID)
	if err == nil {
		t.Error("Expected error when getting deleted user")
	}
}
`

	// Go module file
	goModContent := `module github.com/test/go-test-project

go 1.21

require (
	// Add any required dependencies here
)
`

	project.Files["main.go"] = mainGoContent
	project.Files["main_test.go"] = mainTestContent
	project.Files["go.mod"] = goModContent
	
	// Define expected symbols for testing
	project.Symbols = []*TestSymbol{
		{
			Name: "User",
			Kind: scip.SymbolInformation_Type,
			URI:  "file://main.go",
			Range: &scip.Range{
				Start: scip.Position{Line: 8, Character: 5},
				End:   scip.Position{Line: 8, Character: 9},
			},
		},
		{
			Name: "UserService",
			Kind: scip.SymbolInformation_Interface,
			URI:  "file://main.go",
			Range: &scip.Range{
				Start: scip.Position{Line: 15, Character: 5},
				End:   scip.Position{Line: 15, Character: 16},
			},
		},
		{
			Name: "DefaultUserService",
			Kind: scip.SymbolInformation_Type,
			URI:  "file://main.go",
			Range: &scip.Range{
				Start: scip.Position{Line: 22, Character: 5},
				End:   scip.Position{Line: 22, Character: 21},
			},
		},
		{
			Name: "NewUserService",
			Kind: scip.SymbolInformation_Function,
			URI:  "file://main.go",
			Range: &scip.Range{
				Start: scip.Position{Line: 27, Character: 5},
				End:   scip.Position{Line: 27, Character: 18},
			},
		},
		{
			Name: "GetUser",
			Kind: scip.SymbolInformation_Method,
			URI:  "file://main.go",
			Range: &scip.Range{
				Start: scip.Position{Line: 33, Character: 26},
				End:   scip.Position{Line: 33, Character: 33},
			},
		},
	}
	
	// Define expected references
	project.References = []*TestReference{
		{
			Symbol: "User",
			URI:    "file://main.go",
			Range: &scip.Range{
				Start: scip.Position{Line: 16, Character: 33},
				End:   scip.Position{Line: 16, Character: 37},
			},
			Role: int32(scip.SymbolRole_Definition),
		},
		{
			Symbol: "NewUserService",
			URI:    "file://main.go",
			Range: &scip.Range{
				Start: scip.Position{Line: 65, Character: 12},
				End:   scip.Position{Line: 65, Character: 25},
			},
			Role: int32(scip.SymbolRole_ForwardDefinition),
		},
	}
	
	return project, nil
}

// generateTypeScriptTestProject creates a comprehensive TypeScript test project
func (suite *SCIPTestSuite) generateTypeScriptTestProject() (*TestProject, error) {
	project := &TestProject{
		Name:     "typescript-test-project",
		Language: "typescript",
		Files:    make(map[string]string),
		Symbols:  make([]*TestSymbol, 0),
		References: make([]*TestReference, 0),
	}
	
	// Main TypeScript file
	indexTsContent := `// User management system
export interface User {
  id: number;
  name: string;
  email: string;
  created: Date;
}

export interface UserRepository {
  getUser(id: number): Promise<User | null>;
  createUser(user: Omit<User, 'id' | 'created'>): Promise<User>;
  updateUser(id: number, user: Partial<User>): Promise<User>;
  deleteUser(id: number): Promise<void>;
}

export class InMemoryUserRepository implements UserRepository {
  private users: Map<number, User> = new Map();
  private nextId: number = 1;

  async getUser(id: number): Promise<User | null> {
    return this.users.get(id) || null;
  }

  async createUser(userData: Omit<User, 'id' | 'created'>): Promise<User> {
    const user: User = {
      id: this.nextId++,
      name: userData.name,
      email: userData.email,
      created: new Date(),
    };
    
    this.users.set(user.id, user);
    return user;
  }

  async updateUser(id: number, updates: Partial<User>): Promise<User> {
    const existingUser = this.users.get(id);
    if (!existingUser) {
      throw new Error("User with id " + id + " not found");
    }
    
    const updatedUser = { ...existingUser, ...updates };
    this.users.set(id, updatedUser);
    return updatedUser;
  }

  async deleteUser(id: number): Promise<void> {
    this.users.delete(id);
  }
}

export class UserService {
  constructor(private repository: UserRepository) {}

  async getUserById(id: number): Promise<User | null> {
    return this.repository.getUser(id);
  }

  async createNewUser(name: string, email: string): Promise<User> {
    if (!name || !email) {
      throw new Error('Name and email are required');
    }
    
    return this.repository.createUser({ name, email });
  }

  async updateUserProfile(id: number, updates: Partial<User>): Promise<User> {
    return this.repository.updateUser(id, updates);
  }

  async removeUser(id: number): Promise<void> {
    return this.repository.deleteUser(id);
  }
}

// Usage example
async function main() {
  const repository = new InMemoryUserRepository();
  const userService = new UserService(repository);
  
  try {
    const user = await userService.createNewUser('John Doe', 'john@example.com');
    console.log('Created user:', user);
    
    const retrievedUser = await userService.getUserById(user.id);
    console.log('Retrieved user:', retrievedUser);
    
    const updatedUser = await userService.updateUserProfile(user.id, { 
      name: 'John Smith' 
    });
    console.log('Updated user:', updatedUser);
    
    await userService.removeUser(user.id);
    console.log('User removed');
  } catch (error) {
    console.error('Error:', error);
  }
}

main().catch(console.error);
`

	// Test file
	userTestContent := `import { User, UserRepository, InMemoryUserRepository, UserService } from './index';

describe('UserService', () => {
  let repository: UserRepository;
  let userService: UserService;

  beforeEach(() => {
    repository = new InMemoryUserRepository();
    userService = new UserService(repository);
  });

  describe('createNewUser', () => {
    it('should create a user with valid data', async () => {
      const user = await userService.createNewUser('Test User', 'test@example.com');
      
      expect(user.id).toBeDefined();
      expect(user.name).toBe('Test User');
      expect(user.email).toBe('test@example.com');
      expect(user.created).toBeInstanceOf(Date);
    });

    it('should throw error for invalid data', async () => {
      await expect(userService.createNewUser('', 'test@example.com'))
        .rejects.toThrow('Name and email are required');
      
      await expect(userService.createNewUser('Test', ''))
        .rejects.toThrow('Name and email are required');
    });
  });

  describe('getUserById', () => {
    it('should return user when found', async () => {
      const createdUser = await userService.createNewUser('Test User', 'test@example.com');
      const foundUser = await userService.getUserById(createdUser.id);
      
      expect(foundUser).not.toBeNull();
      expect(foundUser!.id).toBe(createdUser.id);
      expect(foundUser!.name).toBe('Test User');
    });

    it('should return null when user not found', async () => {
      const foundUser = await userService.getUserById(999);
      expect(foundUser).toBeNull();
    });
  });

  describe('updateUserProfile', () => {
    it('should update user data', async () => {
      const createdUser = await userService.createNewUser('Original Name', 'original@example.com');
      
      const updatedUser = await userService.updateUserProfile(createdUser.id, {
        name: 'Updated Name',
        email: 'updated@example.com'
      });
      
      expect(updatedUser.name).toBe('Updated Name');
      expect(updatedUser.email).toBe('updated@example.com');
      expect(updatedUser.id).toBe(createdUser.id);
    });

    it('should throw error for non-existent user', async () => {
      await expect(userService.updateUserProfile(999, { name: 'New Name' }))
        .rejects.toThrow('User with id 999 not found');
    });
  });

  describe('removeUser', () => {
    it('should remove user successfully', async () => {
      const createdUser = await userService.createNewUser('Test User', 'test@example.com');
      
      await userService.removeUser(createdUser.id);
      
      const foundUser = await userService.getUserById(createdUser.id);
      expect(foundUser).toBeNull();
    });
  });
});
`

	// Package.json
	packageJsonContent := `{
  "name": "typescript-test-project",
  "version": "1.0.0",
  "description": "TypeScript test project for SCIP integration testing",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "test": "jest",
    "dev": "ts-node index.ts"
  },
  "dependencies": {
    "typescript": "^5.0.0"
  },
  "devDependencies": {
    "@types/jest": "^29.0.0",
    "@types/node": "^20.0.0",
    "jest": "^29.0.0",
    "ts-jest": "^29.0.0",
    "ts-node": "^10.0.0"
  }
}
`

	// TypeScript config
	tsconfigContent := `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "outDir": "./dist",
    "rootDir": "./",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  },
  "include": ["**/*.ts"],
  "exclude": ["node_modules", "dist"]
}
`

	project.Files["index.ts"] = indexTsContent
	project.Files["user.test.ts"] = userTestContent
	project.Files["package.json"] = packageJsonContent
	project.Files["tsconfig.json"] = tsconfigContent
	
	// Define expected symbols
	project.Symbols = []*TestSymbol{
		{
			Name: "User",
			Kind: scip.SymbolInformation_Interface,
			URI:  "file://index.ts",
			Range: &scip.Range{
				Start: scip.Position{Line: 1, Character: 17},
				End:   scip.Position{Line: 1, Character: 21},
			},
		},
		{
			Name: "UserRepository",
			Kind: scip.SymbolInformation_Interface,
			URI:  "file://index.ts",
			Range: &scip.Range{
				Start: scip.Position{Line: 8, Character: 17},
				End:   scip.Position{Line: 8, Character: 31},
			},
		},
		{
			Name: "UserService",
			Kind: scip.SymbolInformation_Class,
			URI:  "file://index.ts",
			Range: &scip.Range{
				Start: scip.Position{Line: 46, Character: 13},
				End:   scip.Position{Line: 46, Character: 24},
			},
		},
	}
	
	return project, nil
}

// generatePythonTestProject creates a comprehensive Python test project
func (suite *SCIPTestSuite) generatePythonTestProject() (*TestProject, error) {
	project := &TestProject{
		Name:     "python-test-project",
		Language: "python",
		Files:    make(map[string]string),
		Symbols:  make([]*TestSymbol, 0),
		References: make([]*TestReference, 0),
	}
	
	// Main Python module
	userPyContent := `"""User management system for testing SCIP integration."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime
from typing import Dict, Optional, List
import json


@dataclass
class User:
    """Represents a user in the system."""
    id: int
    name: str
    email: str
    created: datetime
    
    def to_dict(self) -> dict:
        """Convert user to dictionary representation."""
        return {
            'id': self.id,
            'name': self.name,
            'email': self.email,
            'created': self.created.isoformat()
        }
    
    @classmethod
    def from_dict(cls, data: dict) -> 'User':
        """Create user from dictionary representation."""
        return cls(
            id=data['id'],
            name=data['name'],
            email=data['email'],
            created=datetime.fromisoformat(data['created'])
        )


class UserRepository(ABC):
    """Abstract base class for user data access."""
    
    @abstractmethod
    def get_user(self, user_id: int) -> Optional[User]:
        """Retrieve a user by ID."""
        pass
    
    @abstractmethod
    def create_user(self, name: str, email: str) -> User:
        """Create a new user."""
        pass
    
    @abstractmethod
    def update_user(self, user_id: int, **kwargs) -> User:
        """Update an existing user."""
        pass
    
    @abstractmethod
    def delete_user(self, user_id: int) -> bool:
        """Delete a user by ID."""
        pass
    
    @abstractmethod
    def list_users(self) -> List[User]:
        """List all users."""
        pass


class InMemoryUserRepository(UserRepository):
    """In-memory implementation of UserRepository."""
    
    def __init__(self):
        self._users: Dict[int, User] = {}
        self._next_id = 1
    
    def get_user(self, user_id: int) -> Optional[User]:
        """Retrieve a user by ID."""
        return self._users.get(user_id)
    
    def create_user(self, name: str, email: str) -> User:
        """Create a new user."""
        user = User(
            id=self._next_id,
            name=name,
            email=email,
            created=datetime.now()
        )
        self._users[user.id] = user
        self._next_id += 1
        return user
    
    def update_user(self, user_id: int, **kwargs) -> User:
        """Update an existing user."""
        user = self._users.get(user_id)
        if not user:
            raise ValueError(f"User with ID {user_id} not found")
        
        # Update user attributes
        for key, value in kwargs.items():
            if hasattr(user, key):
                setattr(user, key, value)
        
        return user
    
    def delete_user(self, user_id: int) -> bool:
        """Delete a user by ID."""
        if user_id in self._users:
            del self._users[user_id]
            return True
        return False
    
    def list_users(self) -> List[User]:
        """List all users."""
        return list(self._users.values())


class UserService:
    """Service class for user management business logic."""
    
    def __init__(self, repository: UserRepository):
        self.repository = repository
    
    def get_user_by_id(self, user_id: int) -> Optional[User]:
        """Get a user by ID with validation."""
        if user_id <= 0:
            raise ValueError("User ID must be positive")
        return self.repository.get_user(user_id)
    
    def create_new_user(self, name: str, email: str) -> User:
        """Create a new user with validation."""
        if not name or not name.strip():
            raise ValueError("Name cannot be empty")
        if not email or '@' not in email:
            raise ValueError("Invalid email address")
        
        return self.repository.create_user(name.strip(), email.strip())
    
    def update_user_profile(self, user_id: int, **updates) -> User:
        """Update user profile with validation."""
        if user_id <= 0:
            raise ValueError("User ID must be positive")
        
        # Validate updates
        if 'name' in updates and not updates['name'].strip():
            raise ValueError("Name cannot be empty")
        if 'email' in updates and '@' not in updates['email']:
            raise ValueError("Invalid email address")
        
        return self.repository.update_user(user_id, **updates)
    
    def remove_user(self, user_id: int) -> bool:
        """Remove a user from the system."""
        if user_id <= 0:
            raise ValueError("User ID must be positive")
        return self.repository.delete_user(user_id)
    
    def get_all_users(self) -> List[User]:
        """Get all users in the system."""
        return self.repository.list_users()
    
    def search_users_by_name(self, name_query: str) -> List[User]:
        """Search users by name."""
        if not name_query or not name_query.strip():
            return []
        
        query = name_query.lower().strip()
        all_users = self.repository.list_users()
        
        return [
            user for user in all_users
            if query in user.name.lower()
        ]
    
    def export_users_to_json(self) -> str:
        """Export all users to JSON format."""
        users = self.repository.list_users()
        user_data = [user.to_dict() for user in users]
        return json.dumps(user_data, indent=2)


def main():
    """Example usage of the user management system."""
    repository = InMemoryUserRepository()
    service = UserService(repository)
    
    try:
        # Create some users
        user1 = service.create_new_user("Alice Johnson", "alice@example.com")
        user2 = service.create_new_user("Bob Smith", "bob@example.com")
        user3 = service.create_new_user("Charlie Brown", "charlie@example.com")
        
        print(f"Created users: {[u.name for u in [user1, user2, user3]]}")
        
        # Update a user
        updated_user = service.update_user_profile(user1.id, name="Alice Johnson-Smith")
        print(f"Updated user: {updated_user.name}")
        
        # Search users
        search_results = service.search_users_by_name("alice")
        print(f"Search results: {[u.name for u in search_results]}")
        
        # Export to JSON
        json_export = service.export_users_to_json()
        print(f"JSON export: {json_export}")
        
        # Remove a user
        removed = service.remove_user(user2.id)
        print(f"User removed: {removed}")
        
        # List remaining users
        remaining_users = service.get_all_users()
        print(f"Remaining users: {[u.name for u in remaining_users]}")
        
    except ValueError as e:
        print(f"Error: {e}")


if __name__ == "__main__":
    main()
`

	// Test file
	testUserContent := `"""Tests for the user management system."""

import pytest
from datetime import datetime
from user import User, InMemoryUserRepository, UserService


class TestUser:
    """Test cases for the User class."""
    
    def test_user_creation(self):
        """Test user object creation."""
        now = datetime.now()
        user = User(1, "Test User", "test@example.com", now)
        
        assert user.id == 1
        assert user.name == "Test User"
        assert user.email == "test@example.com"
        assert user.created == now
    
    def test_user_to_dict(self):
        """Test user to dictionary conversion."""
        now = datetime.now()
        user = User(1, "Test User", "test@example.com", now)
        
        user_dict = user.to_dict()
        
        assert user_dict['id'] == 1
        assert user_dict['name'] == "Test User"
        assert user_dict['email'] == "test@example.com"
        assert user_dict['created'] == now.isoformat()
    
    def test_user_from_dict(self):
        """Test user creation from dictionary."""
        now = datetime.now()
        data = {
            'id': 1,
            'name': 'Test User',
            'email': 'test@example.com',
            'created': now.isoformat()
        }
        
        user = User.from_dict(data)
        
        assert user.id == 1
        assert user.name == "Test User"
        assert user.email == "test@example.com"
        assert user.created == now


class TestInMemoryUserRepository:
    """Test cases for InMemoryUserRepository."""
    
    def setup_method(self):
        """Set up test fixtures."""
        self.repository = InMemoryUserRepository()
    
    def test_create_user(self):
        """Test user creation."""
        user = self.repository.create_user("Test User", "test@example.com")
        
        assert user.id == 1
        assert user.name == "Test User"
        assert user.email == "test@example.com"
        assert isinstance(user.created, datetime)
    
    def test_get_user(self):
        """Test user retrieval."""
        created_user = self.repository.create_user("Test User", "test@example.com")
        retrieved_user = self.repository.get_user(created_user.id)
        
        assert retrieved_user is not None
        assert retrieved_user.id == created_user.id
        assert retrieved_user.name == created_user.name
    
    def test_get_nonexistent_user(self):
        """Test retrieving non-existent user."""
        user = self.repository.get_user(999)
        assert user is None
    
    def test_update_user(self):
        """Test user update."""
        created_user = self.repository.create_user("Original Name", "original@example.com")
        
        updated_user = self.repository.update_user(
            created_user.id,
            name="Updated Name",
            email="updated@example.com"
        )
        
        assert updated_user.name == "Updated Name"
        assert updated_user.email == "updated@example.com"
        assert updated_user.id == created_user.id
    
    def test_update_nonexistent_user(self):
        """Test updating non-existent user."""
        with pytest.raises(ValueError, match="User with ID 999 not found"):
            self.repository.update_user(999, name="New Name")
    
    def test_delete_user(self):
        """Test user deletion."""
        created_user = self.repository.create_user("Test User", "test@example.com")
        
        deleted = self.repository.delete_user(created_user.id)
        assert deleted is True
        
        retrieved_user = self.repository.get_user(created_user.id)
        assert retrieved_user is None
    
    def test_delete_nonexistent_user(self):
        """Test deleting non-existent user."""
        deleted = self.repository.delete_user(999)
        assert deleted is False
    
    def test_list_users(self):
        """Test listing all users."""
        user1 = self.repository.create_user("User 1", "user1@example.com")
        user2 = self.repository.create_user("User 2", "user2@example.com")
        
        users = self.repository.list_users()
        
        assert len(users) == 2
        assert user1 in users
        assert user2 in users


class TestUserService:
    """Test cases for UserService."""
    
    def setup_method(self):
        """Set up test fixtures."""
        self.repository = InMemoryUserRepository()
        self.service = UserService(self.repository)
    
    def test_create_new_user_valid(self):
        """Test creating new user with valid data."""
        user = self.service.create_new_user("Test User", "test@example.com")
        
        assert user.name == "Test User"
        assert user.email == "test@example.com"
    
    def test_create_new_user_invalid_name(self):
        """Test creating user with invalid name."""
        with pytest.raises(ValueError, match="Name cannot be empty"):
            self.service.create_new_user("", "test@example.com")
        
        with pytest.raises(ValueError, match="Name cannot be empty"):
            self.service.create_new_user("   ", "test@example.com")
    
    def test_create_new_user_invalid_email(self):
        """Test creating user with invalid email."""
        with pytest.raises(ValueError, match="Invalid email address"):
            self.service.create_new_user("Test User", "invalid-email")
        
        with pytest.raises(ValueError, match="Invalid email address"):
            self.service.create_new_user("Test User", "")
    
    def test_get_user_by_id_valid(self):
        """Test getting user by valid ID."""
        created_user = self.service.create_new_user("Test User", "test@example.com")
        retrieved_user = self.service.get_user_by_id(created_user.id)
        
        assert retrieved_user is not None
        assert retrieved_user.id == created_user.id
    
    def test_get_user_by_id_invalid(self):
        """Test getting user by invalid ID."""
        with pytest.raises(ValueError, match="User ID must be positive"):
            self.service.get_user_by_id(0)
        
        with pytest.raises(ValueError, match="User ID must be positive"):
            self.service.get_user_by_id(-1)
    
    def test_search_users_by_name(self):
        """Test searching users by name."""
        user1 = self.service.create_new_user("Alice Johnson", "alice@example.com")
        user2 = self.service.create_new_user("Bob Alice", "bob@example.com")
        user3 = self.service.create_new_user("Charlie Brown", "charlie@example.com")
        
        results = self.service.search_users_by_name("alice")
        
        assert len(results) == 2
        assert user1 in results
        assert user2 in results
        assert user3 not in results
    
    def test_export_users_to_json(self):
        """Test exporting users to JSON."""
        user1 = self.service.create_new_user("User 1", "user1@example.com")
        user2 = self.service.create_new_user("User 2", "user2@example.com")
        
        json_export = self.service.export_users_to_json()
        
        assert isinstance(json_export, str)
        assert '"name": "User 1"' in json_export
        assert '"name": "User 2"' in json_export
`

	// Requirements file
	requirementsContent := `# Python requirements for SCIP testing
pytest>=7.0.0
pytest-cov>=4.0.0
black>=22.0.0
flake8>=5.0.0
mypy>=1.0.0
`

	project.Files["user.py"] = userPyContent
	project.Files["test_user.py"] = testUserContent
	project.Files["requirements.txt"] = requirementsContent
	
	// Define expected symbols
	project.Symbols = []*TestSymbol{
		{
			Name: "User",
			Kind: scip.SymbolInformation_Class,
			URI:  "file://user.py",
			Range: &scip.Range{
				Start: scip.Position{Line: 10, Character: 6},
				End:   scip.Position{Line: 10, Character: 10},
			},
		},
		{
			Name: "UserRepository",
			Kind: scip.SymbolInformation_Class,
			URI:  "file://user.py",
			Range: &scip.Range{
				Start: scip.Position{Line: 35, Character: 6},
				End:   scip.Position{Line: 35, Character: 20},
			},
		},
		{
			Name: "UserService",
			Kind: scip.SymbolInformation_Class,
			URI:  "file://user.py",
			Range: &scip.Range{
				Start: scip.Position{Line: 107, Character: 6},
				End:   scip.Position{Line: 107, Character: 17},
			},
		},
	}
	
	return project, nil
}

// generateJavaTestProject creates a comprehensive Java test project
func (suite *SCIPTestSuite) generateJavaTestProject() (*TestProject, error) {
	project := &TestProject{
		Name:     "java-test-project",
		Language: "java",
		Files:    make(map[string]string),
		Symbols:  make([]*TestSymbol, 0),
		References: make([]*TestReference, 0),
	}
	
	// Main Java file - User entity
	userJavaContent := `package com.example.user;

import java.time.LocalDateTime;
import java.util.Objects;

/**
 * Represents a user in the system.
 */
public class User {
    private Long id;
    private String name;
    private String email;
    private LocalDateTime created;
    
    public User() {
        this.created = LocalDateTime.now();
    }
    
    public User(Long id, String name, String email) {
        this.id = id;
        this.name = name;
        this.email = email;
        this.created = LocalDateTime.now();
    }
    
    public User(String name, String email) {
        this(null, name, email);
    }
    
    // Getters and setters
    public Long getId() {
        return id;
    }
    
    public void setId(Long id) {
        this.id = id;
    }
    
    public String getName() {
        return name;
    }
    
    public void setName(String name) {
        this.name = name;
    }
    
    public String getEmail() {
        return email;
    }
    
    public void setEmail(String email) {
        this.email = email;
    }
    
    public LocalDateTime getCreated() {
        return created;
    }
    
    public void setCreated(LocalDateTime created) {
        this.created = created;
    }
    
    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (obj == null || getClass() != obj.getClass()) return false;
        User user = (User) obj;
        return Objects.equals(id, user.id) &&
               Objects.equals(name, user.name) &&
               Objects.equals(email, user.email);
    }
    
    @Override
    public int hashCode() {
        return Objects.hash(id, name, email);
    }
    
    @Override
    public String toString() {
        return "User{" +
                "id=" + id +
                ", name='" + name + '\'' +
                ", email='" + email + '\'' +
                ", created=" + created +
                '}';
    }
}
`

	// Repository interface
	userRepositoryContent := `package com.example.user;

import java.util.List;
import java.util.Optional;

/**
 * Repository interface for User data access.
 */
public interface UserRepository {
    
    /**
     * Find a user by ID.
     */
    Optional<User> findById(Long id);
    
    /**
     * Find a user by email.
     */
    Optional<User> findByEmail(String email);
    
    /**
     * Find users by name containing the given string.
     */
    List<User> findByNameContaining(String name);
    
    /**
     * Get all users.
     */
    List<User> findAll();
    
    /**
     * Save a user.
     */
    User save(User user);
    
    /**
     * Delete a user by ID.
     */
    void deleteById(Long id);
    
    /**
     * Check if a user exists by ID.
     */
    boolean existsById(Long id);
    
    /**
     * Count total number of users.
     */
    long count();
}
`

	// In-memory repository implementation
	inMemoryUserRepositoryContent := `package com.example.user;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;

/**
 * In-memory implementation of UserRepository for testing.
 */
public class InMemoryUserRepository implements UserRepository {
    
    private final Map<Long, User> users = new ConcurrentHashMap<>();
    private final AtomicLong idGenerator = new AtomicLong(1);
    
    @Override
    public Optional<User> findById(Long id) {
        return Optional.ofNullable(users.get(id));
    }
    
    @Override
    public Optional<User> findByEmail(String email) {
        return users.values().stream()
                .filter(user -> Objects.equals(user.getEmail(), email))
                .findFirst();
    }
    
    @Override
    public List<User> findByNameContaining(String name) {
        if (name == null || name.trim().isEmpty()) {
            return new ArrayList<>();
        }
        
        String searchTerm = name.toLowerCase().trim();
        return users.values().stream()
                .filter(user -> user.getName() != null && 
                               user.getName().toLowerCase().contains(searchTerm))
                .collect(Collectors.toList());
    }
    
    @Override
    public List<User> findAll() {
        return new ArrayList<>(users.values());
    }
    
    @Override
    public User save(User user) {
        if (user == null) {
            throw new IllegalArgumentException("User cannot be null");
        }
        
        if (user.getId() == null) {
            user.setId(idGenerator.getAndIncrement());
        }
        
        users.put(user.getId(), user);
        return user;
    }
    
    @Override
    public void deleteById(Long id) {
        users.remove(id);
    }
    
    @Override
    public boolean existsById(Long id) {
        return users.containsKey(id);
    }
    
    @Override
    public long count() {
        return users.size();
    }
}
`

	// User service
	userServiceContent := `package com.example.user;

import java.util.List;
import java.util.Optional;

/**
 * Service class for user management business logic.
 */
public class UserService {
    
    private final UserRepository userRepository;
    
    public UserService(UserRepository userRepository) {
        this.userRepository = Objects.requireNonNull(userRepository, 
                                                    "UserRepository cannot be null");
    }
    
    /**
     * Create a new user.
     */
    public User createUser(String name, String email) {
        validateUserInput(name, email);
        
        // Check if user with email already exists
        Optional<User> existingUser = userRepository.findByEmail(email);
        if (existingUser.isPresent()) {
            throw new IllegalArgumentException("User with email " + email + " already exists");
        }
        
        User user = new User(name, email);
        return userRepository.save(user);
    }
    
    /**
     * Get user by ID.
     */
    public Optional<User> getUserById(Long id) {
        if (id == null || id <= 0) {
            throw new IllegalArgumentException("User ID must be positive");
        }
        return userRepository.findById(id);
    }
    
    /**
     * Get user by email.
     */
    public Optional<User> getUserByEmail(String email) {
        if (email == null || email.trim().isEmpty()) {
            throw new IllegalArgumentException("Email cannot be null or empty");
        }
        return userRepository.findByEmail(email.trim());
    }
    
    /**
     * Update user information.
     */
    public User updateUser(Long id, String name, String email) {
        if (id == null || id <= 0) {
            throw new IllegalArgumentException("User ID must be positive");
        }
        
        validateUserInput(name, email);
        
        Optional<User> existingUser = userRepository.findById(id);
        if (!existingUser.isPresent()) {
            throw new IllegalArgumentException("User with ID " + id + " not found");
        }
        
        // Check if email is already taken by another user
        Optional<User> userWithEmail = userRepository.findByEmail(email);
        if (userWithEmail.isPresent() && !userWithEmail.get().getId().equals(id)) {
            throw new IllegalArgumentException("Email " + email + " is already taken");
        }
        
        User user = existingUser.get();
        user.setName(name);
        user.setEmail(email);
        
        return userRepository.save(user);
    }
    
    /**
     * Delete user by ID.
     */
    public boolean deleteUser(Long id) {
        if (id == null || id <= 0) {
            throw new IllegalArgumentException("User ID must be positive");
        }
        
        if (!userRepository.existsById(id)) {
            return false;
        }
        
        userRepository.deleteById(id);
        return true;
    }
    
    /**
     * Search users by name.
     */
    public List<User> searchUsersByName(String name) {
        return userRepository.findByNameContaining(name);
    }
    
    /**
     * Get all users.
     */
    public List<User> getAllUsers() {
        return userRepository.findAll();
    }
    
    /**
     * Get user count.
     */
    public long getUserCount() {
        return userRepository.count();
    }
    
    private void validateUserInput(String name, String email) {
        if (name == null || name.trim().isEmpty()) {
            throw new IllegalArgumentException("Name cannot be null or empty");
        }
        
        if (email == null || email.trim().isEmpty()) {
            throw new IllegalArgumentException("Email cannot be null or empty");
        }
        
        if (!isValidEmail(email)) {
            throw new IllegalArgumentException("Invalid email format");
        }
    }
    
    private boolean isValidEmail(String email) {
        return email.contains("@") && email.contains(".");
    }
}
`

	// Test file
	userServiceTestContent := `package com.example.user;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;
import static org.junit.jupiter.api.Assertions.*;

import java.util.List;
import java.util.Optional;

/**
 * Test class for UserService.
 */
class UserServiceTest {
    
    private UserRepository userRepository;
    private UserService userService;
    
    @BeforeEach
    void setUp() {
        userRepository = new InMemoryUserRepository();
        userService = new UserService(userRepository);
    }
    
    @Test
    @DisplayName("Should create user with valid data")
    void testCreateUser_ValidData() {
        // Arrange
        String name = "John Doe";
        String email = "john.doe@example.com";
        
        // Act
        User createdUser = userService.createUser(name, email);
        
        // Assert
        assertNotNull(createdUser);
        assertNotNull(createdUser.getId());
        assertEquals(name, createdUser.getName());
        assertEquals(email, createdUser.getEmail());
        assertNotNull(createdUser.getCreated());
    }
    
    @Test
    @DisplayName("Should throw exception for invalid name")
    void testCreateUser_InvalidName() {
        // Arrange & Act & Assert
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.createUser("", "john@example.com"));
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.createUser(null, "john@example.com"));
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.createUser("   ", "john@example.com"));
    }
    
    @Test
    @DisplayName("Should throw exception for invalid email")
    void testCreateUser_InvalidEmail() {
        // Arrange & Act & Assert
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.createUser("John", ""));
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.createUser("John", null));
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.createUser("John", "invalid-email"));
    }
    
    @Test
    @DisplayName("Should throw exception for duplicate email")
    void testCreateUser_DuplicateEmail() {
        // Arrange
        String email = "john@example.com";
        userService.createUser("John Doe", email);
        
        // Act & Assert
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.createUser("Jane Doe", email));
    }
    
    @Test
    @DisplayName("Should get user by ID")
    void testGetUserById() {
        // Arrange
        User createdUser = userService.createUser("John Doe", "john@example.com");
        
        // Act
        Optional<User> foundUser = userService.getUserById(createdUser.getId());
        
        // Assert
        assertTrue(foundUser.isPresent());
        assertEquals(createdUser.getId(), foundUser.get().getId());
        assertEquals(createdUser.getName(), foundUser.get().getName());
    }
    
    @Test
    @DisplayName("Should return empty for non-existent user ID")
    void testGetUserById_NotFound() {
        // Act
        Optional<User> foundUser = userService.getUserById(999L);
        
        // Assert
        assertFalse(foundUser.isPresent());
    }
    
    @Test
    @DisplayName("Should throw exception for invalid user ID")
    void testGetUserById_InvalidId() {
        // Act & Assert
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.getUserById(null));
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.getUserById(0L));
        assertThrows(IllegalArgumentException.class, 
                    () -> userService.getUserById(-1L));
    }
    
    @Test
    @DisplayName("Should update user successfully")
    void testUpdateUser() {
        // Arrange
        User createdUser = userService.createUser("John Doe", "john@example.com");
        String newName = "John Smith";
        String newEmail = "john.smith@example.com";
        
        // Act
        User updatedUser = userService.updateUser(createdUser.getId(), newName, newEmail);
        
        // Assert
        assertEquals(createdUser.getId(), updatedUser.getId());
        assertEquals(newName, updatedUser.getName());
        assertEquals(newEmail, updatedUser.getEmail());
    }
    
    @Test
    @DisplayName("Should delete user successfully")
    void testDeleteUser() {
        // Arrange
        User createdUser = userService.createUser("John Doe", "john@example.com");
        
        // Act
        boolean deleted = userService.deleteUser(createdUser.getId());
        
        // Assert
        assertTrue(deleted);
        Optional<User> foundUser = userService.getUserById(createdUser.getId());
        assertFalse(foundUser.isPresent());
    }
    
    @Test
    @DisplayName("Should return false when deleting non-existent user")
    void testDeleteUser_NotFound() {
        // Act
        boolean deleted = userService.deleteUser(999L);
        
        // Assert
        assertFalse(deleted);
    }
    
    @Test
    @DisplayName("Should search users by name")
    void testSearchUsersByName() {
        // Arrange
        userService.createUser("John Doe", "john@example.com");
        userService.createUser("Jane Doe", "jane@example.com");
        userService.createUser("Bob Smith", "bob@example.com");
        
        // Act
        List<User> searchResults = userService.searchUsersByName("Doe");
        
        // Assert
        assertEquals(2, searchResults.size());
        assertTrue(searchResults.stream().anyMatch(u -> u.getName().equals("John Doe")));
        assertTrue(searchResults.stream().anyMatch(u -> u.getName().equals("Jane Doe")));
    }
    
    @Test
    @DisplayName("Should get all users")
    void testGetAllUsers() {
        // Arrange
        userService.createUser("John Doe", "john@example.com");
        userService.createUser("Jane Smith", "jane@example.com");
        
        // Act
        List<User> allUsers = userService.getAllUsers();
        
        // Assert
        assertEquals(2, allUsers.size());
    }
    
    @Test
    @DisplayName("Should get user count")
    void testGetUserCount() {
        // Arrange
        assertEquals(0, userService.getUserCount());
        
        userService.createUser("John Doe", "john@example.com");
        userService.createUser("Jane Smith", "jane@example.com");
        
        // Act & Assert
        assertEquals(2, userService.getUserCount());
    }
}
`

	// Build file (Maven pom.xml)
	pomXmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    
    <groupId>com.example</groupId>
    <artifactId>java-test-project</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>
    
    <name>Java Test Project</name>
    <description>Java test project for SCIP integration testing</description>
    
    <properties>
        <maven.compiler.source>17</maven.compiler.source>
        <maven.compiler.target>17</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
        <junit.version>5.9.2</junit.version>
    </properties>
    
    <dependencies>
        <dependency>
            <groupId>org.junit.jupiter</groupId>
            <artifactId>junit-jupiter</artifactId>
            <version>5.9.2</version>
            <scope>test</scope>
        </dependency>
    </dependencies>
    
    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.11.0</version>
                <configuration>
                    <source>17</source>
                    <target>17</target>
                </configuration>
            </plugin>
            
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-surefire-plugin</artifactId>
                <version>3.0.0-M9</version>
            </plugin>
        </plugins>
    </build>
</project>
`

	project.Files["src/main/java/com/example/user/User.java"] = userJavaContent
	project.Files["src/main/java/com/example/user/UserRepository.java"] = userRepositoryContent
	project.Files["src/main/java/com/example/user/InMemoryUserRepository.java"] = inMemoryUserRepositoryContent
	project.Files["src/main/java/com/example/user/UserService.java"] = userServiceContent
	project.Files["src/test/java/com/example/user/UserServiceTest.java"] = userServiceTestContent
	project.Files["pom.xml"] = pomXmlContent
	
	// Define expected symbols
	project.Symbols = []*TestSymbol{
		{
			Name: "User",
			Kind: scip.SymbolInformation_Class,
			URI:  "file://src/main/java/com/example/user/User.java",
			Range: &scip.Range{
				Start: scip.Position{Line: 8, Character: 13},
				End:   scip.Position{Line: 8, Character: 17},
			},
		},
		{
			Name: "UserRepository",
			Kind: scip.SymbolInformation_Interface,
			URI:  "file://src/main/java/com/example/user/UserRepository.java",
			Range: &scip.Range{
				Start: scip.Position{Line: 8, Character: 17},
				End:   scip.Position{Line: 8, Character: 31},
			},
		},
		{
			Name: "UserService",
			Kind: scip.SymbolInformation_Class,
			URI:  "file://src/main/java/com/example/user/UserService.java",
			Range: &scip.Range{
				Start: scip.Position{Line: 8, Character: 13},
				End:   scip.Position{Line: 8, Character: 24},
			},
		},
	}
	
	return project, nil
}

// generateCrossLanguageProject creates a project with cross-language references
func (suite *SCIPTestSuite) generateCrossLanguageProject() error {
	// This would create a project that demonstrates cross-language references
	// For example, a Go service that calls a Python script, or a TypeScript frontend
	// that communicates with a Java backend
	
	// For now, we'll create a simple multi-language project structure
	crossLangProject := &TestProject{
		Name:         "cross-language-project",
		Language:     "multi",
		Files:        make(map[string]string),
		Dependencies: []string{"go-test-project", "typescript-test-project"},
		Symbols:      make([]*TestSymbol, 0),
		References:   make([]*TestReference, 0),
	}
	
	// API Gateway in Go that calls TypeScript services
	apiGatewayContent := `package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
)

type APIGateway struct {
	typescriptServiceURL string
}

func NewAPIGateway(tsServiceURL string) *APIGateway {
	return &APIGateway{
		typescriptServiceURL: tsServiceURL,
	}
}

// CallTypeScriptService demonstrates cross-language communication
func (gw *APIGateway) CallTypeScriptService(userID int) (map[string]interface{}, error) {
	// This would normally make an HTTP call to a TypeScript service
	// For testing purposes, we simulate calling a Node.js script
	cmd := exec.Command("node", "user-service.js", fmt.Sprintf("%d", userID))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to call TypeScript service: %w", err)
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse TypeScript response: %w", err)
	}
	
	return result, nil
}
`

	// TypeScript service that the Go service calls
	tsServiceContent := `// user-service.js - TypeScript service called by Go
const args = process.argv.slice(2);
const userID = parseInt(args[0] || '1');

interface User {
  id: number;
  name: string;
  email: string;
  created: string;
}

// Simulate user data retrieval
function getUser(id: number): User {
  return {
    id,
    name: "User " + id,
    email: "user" + id + "@example.com",
    created: new Date().toISOString()
  };
}

// Cross-language communication: output JSON for Go service
const user = getUser(userID);
console.log(JSON.stringify(user));
`

	crossLangProject.Files["api-gateway.go"] = apiGatewayContent
	crossLangProject.Files["user-service.js"] = tsServiceContent
	
	suite.testProjects["cross-language"] = crossLangProject
	
	return nil
}

// generateSCIPIndex generates a SCIP index file for a test project
func (suite *SCIPTestSuite) generateSCIPIndex(project *TestProject) (string, error) {
	// Create a synthetic SCIP index based on the project symbols and references
	scipIndex := &scip.Index{
		Metadata: &scip.Metadata{
			Version:     scip.ProtocolVersion_UnspecifiedProtocolVersion,
			ToolInfo:    &scip.ToolInfo{Name: "scip-test-generator", Version: "1.0.0"},
			ProjectRoot: fmt.Sprintf("file://%s", project.Name),
		},
		Documents: make([]*scip.Document, 0),
	}
	
	// Create documents for each file in the project
	for filename, content := range project.Files {
		if !strings.HasSuffix(filename, getLanguageExtension(project.Language)) {
			continue // Skip non-source files
		}
		
		scipDoc := &scip.Document{
			RelativePath: filename,
			Language:     project.Language,
			Text:         content,
			Symbols:      make([]*scip.SymbolInformation, 0),
			Occurrences:  make([]*scip.Occurrence, 0),
		}
		
		// Add symbols from the test project
		for _, symbol := range project.Symbols {
			if symbol.URI == fmt.Sprintf("file://%s", filename) {
				scipSymbol := &scip.SymbolInformation{
					Symbol:        symbol.Name,
					DisplayName:   symbol.Name,
					Kind:          symbol.Kind,
				}
				
				scipDoc.Symbols = append(scipDoc.Symbols, scipSymbol)
				
				// Add definition occurrence
				if symbol.Definition != nil {
					occurrence := &scip.Occurrence{
						Range:       convertRangeToInt32Array(symbol.Definition),
						Symbol:      symbol.Name,
						SymbolRoles: int32(scip.SymbolRole_Definition),
					}
					scipDoc.Occurrences = append(scipDoc.Occurrences, occurrence)
				}
			}
		}
		
		// Add references from the test project
		for _, ref := range project.References {
			if ref.URI == fmt.Sprintf("file://%s", filename) {
				occurrence := &scip.Occurrence{
					Range:       convertRangeToInt32Array(ref.Range),
					Symbol:      ref.Symbol,
					SymbolRoles: ref.Role,
				}
				scipDoc.Occurrences = append(scipDoc.Occurrences, occurrence)
			}
		}
		
		scipIndex.Documents = append(scipIndex.Documents, scipDoc)
	}
	
	// Write SCIP index to file
	indexPath := filepath.Join(suite.testDataDir, "scip_indices", 
		fmt.Sprintf("%s.scip", project.Name))
	
	if err := writeSCIPIndex(scipIndex, indexPath); err != nil {
		return "", fmt.Errorf("failed to write SCIP index: %w", err)
	}
	
	return indexPath, nil
}

// getLanguageExtension returns the primary file extension for a language
func getLanguageExtension(language string) string {
	switch language {
	case "go":
		return ".go"
	case "typescript":
		return ".ts"
	case "python":
		return ".py"
	case "java":
		return ".java"
	default:
		return ""
	}
}

// writeSCIPIndex writes a SCIP index to a file
func writeSCIPIndex(index *scip.Index, filePath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create SCIP file: %w", err)
	}
	defer file.Close()
	
	// Write SCIP index (simplified - would use proper SCIP serialization)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(index); err != nil {
		return fmt.Errorf("failed to encode SCIP index: %w", err)
	}
	
	return nil
}