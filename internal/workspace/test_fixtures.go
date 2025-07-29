package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// TestProjectFiles contains realistic project file templates for different programming languages
var TestProjectFiles = map[string]map[string]string{
	"go": {
		"main.go": `package main

import (
	"fmt"
	"log"
	"os"
)

// User represents a user in the system
type User struct {
	ID    int    ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

// UserService provides user-related operations
type UserService struct {
	users map[int]*User
}

// NewUserService creates a new user service
func NewUserService() *UserService {
	return &UserService{
		users: make(map[int]*User),
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(name, email string) *User {
	user := &User{
		ID:    len(s.users) + 1,
		Name:  name,
		Email: email,
	}
	s.users[user.ID] = user
	return user
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) (*User, error) {
	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}
	return user, nil
}

// ListUsers returns all users
func (s *UserService) ListUsers() []*User {
	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

func main() {
	service := NewUserService()
	
	// Create some test users
	user1 := service.CreateUser("John Doe", "john@example.com")
	user2 := service.CreateUser("Jane Smith", "jane@example.com")
	
	fmt.Printf("Created users: %+v, %+v\n", user1, user2)
	
	// List all users
	users := service.ListUsers()
	fmt.Printf("All users: %+v\n", users)
	
	// Get a specific user
	if user, err := service.GetUser(1); err != nil {
		log.Printf("Error getting user: %v", err)
		os.Exit(1)
	} else {
		fmt.Printf("Retrieved user: %+v\n", user)
	}
}`,
		"go.mod": `module test-workspace

go 1.21

require (
	github.com/gorilla/mux v1.8.0
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)`,
		"user_test.go": `package main

import (
	"testing"
)

func TestUserService_CreateUser(t *testing.T) {
	service := NewUserService()
	
	user := service.CreateUser("Test User", "test@example.com")
	
	if user.Name != "Test User" {
		t.Errorf("Expected name 'Test User', got '%s'", user.Name)
	}
	
	if user.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
	}
	
	if user.ID <= 0 {
		t.Errorf("Expected positive ID, got %d", user.ID)
	}
}

func TestUserService_GetUser(t *testing.T) {
	service := NewUserService()
	
	// Create a user
	createdUser := service.CreateUser("Test User", "test@example.com")
	
	// Retrieve the user
	retrievedUser, err := service.GetUser(createdUser.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if retrievedUser.ID != createdUser.ID {
		t.Errorf("Expected ID %d, got %d", createdUser.ID, retrievedUser.ID)
	}
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	service := NewUserService()
	
	_, err := service.GetUser(999)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestUserService_ListUsers(t *testing.T) {
	service := NewUserService()
	
	// Initially should be empty
	users := service.ListUsers()
	if len(users) != 0 {
		t.Errorf("Expected 0 users, got %d", len(users))
	}
	
	// Create some users
	service.CreateUser("User 1", "user1@example.com")
	service.CreateUser("User 2", "user2@example.com")
	
	users = service.ListUsers()
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}`,
	},
	
	"python": {
		"main.py": `#!/usr/bin/env python3
"""
Main module for the test workspace Python application.
"""

import json
import logging
from typing import Dict, List, Optional
from dataclasses import dataclass, asdict


@dataclass
class User:
    """Represents a user in the system."""
    id: int
    name: str
    email: str
    
    def to_dict(self) -> Dict:
        """Convert user to dictionary."""
        return asdict(self)
    
    @classmethod
    def from_dict(cls, data: Dict) -> 'User':
        """Create user from dictionary."""
        return cls(**data)


class UserService:
    """Service for managing users."""
    
    def __init__(self):
        """Initialize the user service."""
        self._users: Dict[int, User] = {}
        self._next_id = 1
        self.logger = logging.getLogger(__name__)
    
    def create_user(self, name: str, email: str) -> User:
        """Create a new user."""
        user = User(id=self._next_id, name=name, email=email)
        self._users[user.id] = user
        self._next_id += 1
        
        self.logger.info(f"Created user: {user.name} ({user.email})")
        return user
    
    def get_user(self, user_id: int) -> Optional[User]:
        """Get a user by ID."""
        user = self._users.get(user_id)
        if user:
            self.logger.debug(f"Retrieved user: {user.name}")
        else:
            self.logger.warning(f"User not found: {user_id}")
        return user
    
    def list_users(self) -> List[User]:
        """List all users."""
        users = list(self._users.values())
        self.logger.info(f"Listed {len(users)} users")
        return users
    
    def update_user(self, user_id: int, name: Optional[str] = None, 
                   email: Optional[str] = None) -> Optional[User]:
        """Update a user."""
        user = self._users.get(user_id)
        if not user:
            return None
        
        if name is not None:
            user.name = name
        if email is not None:
            user.email = email
        
        self.logger.info(f"Updated user: {user.name}")
        return user
    
    def delete_user(self, user_id: int) -> bool:
        """Delete a user."""
        if user_id in self._users:
            user = self._users.pop(user_id)
            self.logger.info(f"Deleted user: {user.name}")
            return True
        return False
    
    def save_to_file(self, filename: str) -> None:
        """Save users to a JSON file."""
        data = [user.to_dict() for user in self._users.values()]
        with open(filename, 'w') as f:
            json.dump(data, f, indent=2)
        self.logger.info(f"Saved {len(data)} users to {filename}")
    
    def load_from_file(self, filename: str) -> None:
        """Load users from a JSON file."""
        try:
            with open(filename, 'r') as f:
                data = json.load(f)
            
            self._users.clear()
            for user_data in data:
                user = User.from_dict(user_data)
                self._users[user.id] = user
                if user.id >= self._next_id:
                    self._next_id = user.id + 1
            
            self.logger.info(f"Loaded {len(data)} users from {filename}")
        except FileNotFoundError:
            self.logger.warning(f"File not found: {filename}")
        except json.JSONDecodeError as e:
            self.logger.error(f"Error decoding JSON from {filename}: {e}")


def setup_logging() -> None:
    """Setup logging configuration."""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )


def main() -> None:
    """Main function."""
    setup_logging()
    logger = logging.getLogger(__name__)
    
    logger.info("Starting Python test application")
    
    # Create user service
    service = UserService()
    
    # Create some test users
    user1 = service.create_user("Alice Johnson", "alice@example.com")
    user2 = service.create_user("Bob Wilson", "bob@example.com")
    user3 = service.create_user("Carol Brown", "carol@example.com")
    
    print(f"Created users:")
    for user in [user1, user2, user3]:
        print(f"  - {user.name} ({user.email})")
    
    # List all users
    all_users = service.list_users()
    print(f"\nAll users ({len(all_users)}):")
    for user in all_users:
        print(f"  {user.id}: {user.name} - {user.email}")
    
    # Update a user
    updated_user = service.update_user(1, name="Alice Smith")
    if updated_user:
        print(f"\nUpdated user: {updated_user.name}")
    
    # Get a specific user
    user = service.get_user(2)
    if user:
        print(f"\nRetrieved user: {user.name} ({user.email})")
    
    # Save to file
    service.save_to_file("users.json")
    print("\nUsers saved to users.json")
    
    logger.info("Python test application completed")


if __name__ == "__main__":
    main()`,
		"requirements.txt": `# Test workspace Python dependencies
requests==2.31.0
pytest==7.4.2
pytest-cov==4.1.0
black==23.7.0
flake8==6.0.0
mypy==1.5.1
dataclasses-json==0.5.14`,
		"test_main.py": `#!/usr/bin/env python3
"""
Test module for the main application.
"""

import unittest
import tempfile
import os
import json
from main import User, UserService


class TestUser(unittest.TestCase):
    """Test cases for User class."""
    
    def test_user_creation(self):
        """Test user creation."""
        user = User(id=1, name="Test User", email="test@example.com")
        self.assertEqual(user.id, 1)
        self.assertEqual(user.name, "Test User")
        self.assertEqual(user.email, "test@example.com")
    
    def test_user_to_dict(self):
        """Test user to dictionary conversion."""
        user = User(id=1, name="Test User", email="test@example.com")
        expected = {"id": 1, "name": "Test User", "email": "test@example.com"}
        self.assertEqual(user.to_dict(), expected)
    
    def test_user_from_dict(self):
        """Test user creation from dictionary."""
        data = {"id": 1, "name": "Test User", "email": "test@example.com"}
        user = User.from_dict(data)
        self.assertEqual(user.id, 1)
        self.assertEqual(user.name, "Test User")
        self.assertEqual(user.email, "test@example.com")


class TestUserService(unittest.TestCase):
    """Test cases for UserService class."""
    
    def setUp(self):
        """Set up test fixtures."""
        self.service = UserService()
    
    def test_create_user(self):
        """Test user creation."""
        user = self.service.create_user("Test User", "test@example.com")
        self.assertEqual(user.name, "Test User")
        self.assertEqual(user.email, "test@example.com")
        self.assertGreater(user.id, 0)
    
    def test_get_user(self):
        """Test user retrieval."""
        created_user = self.service.create_user("Test User", "test@example.com")
        retrieved_user = self.service.get_user(created_user.id)
        
        self.assertIsNotNone(retrieved_user)
        self.assertEqual(retrieved_user.id, created_user.id)
        self.assertEqual(retrieved_user.name, created_user.name)
    
    def test_get_user_not_found(self):
        """Test getting non-existent user."""
        user = self.service.get_user(999)
        self.assertIsNone(user)
    
    def test_list_users(self):
        """Test user listing."""
        # Initially empty
        users = self.service.list_users()
        self.assertEqual(len(users), 0)
        
        # Create some users
        self.service.create_user("User 1", "user1@example.com")
        self.service.create_user("User 2", "user2@example.com")
        
        users = self.service.list_users()
        self.assertEqual(len(users), 2)
    
    def test_update_user(self):
        """Test user update."""
        user = self.service.create_user("Original Name", "original@example.com")
        
        updated_user = self.service.update_user(user.id, name="Updated Name")
        self.assertIsNotNone(updated_user)
        self.assertEqual(updated_user.name, "Updated Name")
        self.assertEqual(updated_user.email, "original@example.com")
    
    def test_delete_user(self):
        """Test user deletion."""
        user = self.service.create_user("Test User", "test@example.com")
        
        # User should exist
        self.assertIsNotNone(self.service.get_user(user.id))
        
        # Delete user
        result = self.service.delete_user(user.id)
        self.assertTrue(result)
        
        # User should no longer exist
        self.assertIsNone(self.service.get_user(user.id))
    
    def test_save_and_load_file(self):
        """Test saving and loading users to/from file."""
        # Create some users
        user1 = self.service.create_user("User 1", "user1@example.com")
        user2 = self.service.create_user("User 2", "user2@example.com")
        
        # Save to temporary file
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.json') as f:
            temp_filename = f.name
        
        try:
            self.service.save_to_file(temp_filename)
            
            # Create new service and load
            new_service = UserService()
            new_service.load_from_file(temp_filename)
            
            # Verify users were loaded
            loaded_users = new_service.list_users()
            self.assertEqual(len(loaded_users), 2)
            
            loaded_user1 = new_service.get_user(user1.id)
            self.assertIsNotNone(loaded_user1)
            self.assertEqual(loaded_user1.name, user1.name)
            
        finally:
            if os.path.exists(temp_filename):
                os.unlink(temp_filename)


if __name__ == '__main__':
    unittest.main()`,
		"setup.py": `#!/usr/bin/env python3
"""
Setup script for the test workspace Python package.
"""

from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="test-workspace",
    version="0.1.0",
    author="Test Author",
    author_email="test@example.com",
    description="A test workspace Python application",
    long_description=long_description,
    long_description_content_type="text/markdown",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
    ],
    python_requires=">=3.8",
    install_requires=[
        "requests>=2.31.0",
    ],
    extras_require={
        "dev": [
            "pytest>=7.4.2",
            "pytest-cov>=4.1.0",
            "black>=23.7.0",
            "flake8>=6.0.0",
            "mypy>=1.5.1",
        ],
    },
    entry_points={
        "console_scripts": [
            "test-workspace=main:main",
        ],
    },
)`,
	},
	
	"typescript": {
		"src/main.ts": `#!/usr/bin/env node
/**
 * Main TypeScript module for the test workspace application.
 */

import * as fs from 'fs/promises';
import * as path from 'path';

// Interface definitions
interface User {
    id: number;
    name: string;
    email: string;
    createdAt: Date;
    updatedAt: Date;
}

interface CreateUserRequest {
    name: string;
    email: string;
}

interface UpdateUserRequest {
    name?: string;
    email?: string;
}

// User service class
class UserService {
    private users: Map<number, User> = new Map();
    private nextId: number = 1;

    /**
     * Create a new user
     */
    createUser(request: CreateUserRequest): User {
        const now = new Date();
        const user: User = {
            id: this.nextId++,
            name: request.name,
            email: request.email,
            createdAt: now,
            updatedAt: now
        };

        this.users.set(user.id, user);
        console.log(` + "`" + `Created user: ${user.name} (${user.email})` + "`" + `);
        return user;
    }

    /**
     * Get a user by ID
     */
    getUser(id: number): User | undefined {
        const user = this.users.get(id);
        if (user) {
            console.log(` + "`" + `Retrieved user: ${user.name}` + "`" + `);
        } else {
            console.warn(` + "`" + `User not found: ${id}` + "`" + `);
        }
        return user;
    }

    /**
     * List all users
     */
    listUsers(): User[] {
        const users = Array.from(this.users.values());
        console.log(` + "`" + `Listed ${users.length} users` + "`" + `);
        return users;
    }

    /**
     * Update a user
     */
    updateUser(id: number, request: UpdateUserRequest): User | undefined {
        const user = this.users.get(id);
        if (!user) {
            console.warn(` + "`" + `User not found for update: ${id}` + "`" + `);
            return undefined;
        }

        if (request.name !== undefined) {
            user.name = request.name;
        }
        if (request.email !== undefined) {
            user.email = request.email;
        }
        user.updatedAt = new Date();

        console.log(` + "`" + `Updated user: ${user.name}` + "`" + `);
        return user;
    }

    /**
     * Delete a user
     */
    deleteUser(id: number): boolean {
        const user = this.users.get(id);
        if (user) {
            this.users.delete(id);
            console.log(` + "`" + `Deleted user: ${user.name}` + "`" + `);
            return true;
        }
        console.warn(` + "`" + `User not found for deletion: ${id}` + "`" + `);
        return false;
    }

    /**
     * Save users to a JSON file
     */
    async saveToFile(filename: string): Promise<void> {
        const users = Array.from(this.users.values());
        const data = JSON.stringify(users, null, 2);
        
        await fs.writeFile(filename, data, 'utf-8');
        console.log(` + "`" + `Saved ${users.length} users to ${filename}` + "`" + `);
    }

    /**
     * Load users from a JSON file
     */
    async loadFromFile(filename: string): Promise<void> {
        try {
            const data = await fs.readFile(filename, 'utf-8');
            const users: User[] = JSON.parse(data);

            this.users.clear();
            this.nextId = 1;

            for (const userData of users) {
                const user: User = {
                    ...userData,
                    createdAt: new Date(userData.createdAt),
                    updatedAt: new Date(userData.updatedAt)
                };
                this.users.set(user.id, user);
                if (user.id >= this.nextId) {
                    this.nextId = user.id + 1;
                }
            }

            console.log(` + "`" + `Loaded ${users.length} users from ${filename}` + "`" + `);
        } catch (error) {
            if (error instanceof Error) {
                console.error(` + "`" + `Error loading users from ${filename}: ${error.message}` + "`" + `);
            }
        }
    }

    /**
     * Get user statistics
     */
    getStats(): { totalUsers: number; averageAge: number } {
        const users = Array.from(this.users.values());
        const now = new Date();
        
        const totalAge = users.reduce((sum, user) => {
            const ageMs = now.getTime() - user.createdAt.getTime();
            const ageDays = ageMs / (1000 * 60 * 60 * 24);
            return sum + ageDays;
        }, 0);

        return {
            totalUsers: users.length,
            averageAge: users.length > 0 ? totalAge / users.length : 0
        };
    }
}

// Utility functions
function displayUser(user: User): void {
    console.log(` + "`" + `  ${user.id}: ${user.name} - ${user.email} (created: ${user.createdAt.toISOString()})` + "`" + `);
}

function displayUsers(users: User[]): void {
    console.log(` + "`" + `\nUsers (${users.length}):` + "`" + `);
    users.forEach(displayUser);
}

// Main function
async function main(): Promise<void> {
    console.log('Starting TypeScript test application');

    // Create user service
    const service = new UserService();

    // Create some test users
    const user1 = service.createUser({ name: 'Alice Johnson', email: 'alice@example.com' });
    const user2 = service.createUser({ name: 'Bob Wilson', email: 'bob@example.com' });
    const user3 = service.createUser({ name: 'Carol Brown', email: 'carol@example.com' });

    console.log('\\nCreated users:');
    [user1, user2, user3].forEach(displayUser);

    // List all users
    const allUsers = service.listUsers();
    displayUsers(allUsers);

    // Update a user
    const updatedUser = service.updateUser(1, { name: 'Alice Smith' });
    if (updatedUser) {
        console.log(` + "`" + `\nUpdated user: ${updatedUser.name}` + "`" + `);
    }

    // Get a specific user
    const specificUser = service.getUser(2);
    if (specificUser) {
        console.log(` + "`" + `\nRetrieved user: ${specificUser.name} (${specificUser.email})` + "`" + `);
    }

    // Get statistics
    const stats = service.getStats();
    console.log(` + "`" + `\nStatistics: ${stats.totalUsers} users, average age: ${stats.averageAge.toFixed(2)} days` + "`" + `);

    // Save to file
    const filename = 'users.json';
    await service.saveToFile(filename);
    console.log(` + "`" + `\nUsers saved to ${filename}` + "`" + `);

    // Test loading (create new service and load)
    const newService = new UserService();
    await newService.loadFromFile(filename);
    const loadedUsers = newService.listUsers();
    console.log(` + "`" + `\nLoaded users verification: ${loadedUsers.length} users` + "`" + `);

    console.log('TypeScript test application completed');
}

// Error handling wrapper
async function runMain(): Promise<void> {
    try {
        await main();
    } catch (error) {
        console.error('Application error:', error);
        process.exit(1);
    }
}

// Run the application
if (require.main === module) {
    runMain();
}

export { User, UserService, CreateUserRequest, UpdateUserRequest };`,
		"package.json": `{
  "name": "test-workspace-ts",
  "version": "1.0.0",
  "description": "A test workspace TypeScript application",
  "main": "dist/main.js",
  "bin": {
    "test-workspace": "dist/main.js"
  },
  "scripts": {
    "build": "tsc",
    "start": "node dist/main.js",
    "dev": "ts-node src/main.ts",
    "test": "jest",
    "test:watch": "jest --watch",
    "test:coverage": "jest --coverage",
    "lint": "eslint src/**/*.ts",
    "lint:fix": "eslint src/**/*.ts --fix",
    "clean": "rm -rf dist",
    "prepare": "npm run build"
  },
  "keywords": [
    "typescript",
    "test",
    "workspace",
    "lsp"
  ],
  "author": "Test Author <test@example.com>",
  "license": "MIT",
  "devDependencies": {
    "@types/jest": "^29.5.5",
    "@types/node": "^20.6.0",
    "@typescript-eslint/eslint-plugin": "^6.7.0",
    "@typescript-eslint/parser": "^6.7.0",
    "eslint": "^8.49.0",
    "jest": "^29.7.0",
    "ts-jest": "^29.1.1",
    "ts-node": "^10.9.1",
    "typescript": "^5.2.2"
  },
  "dependencies": {},
  "engines": {
    "node": ">=16.0.0"
  }
}`,
		"tsconfig.json": `{
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
    "removeComments": false,
    "noImplicitAny": true,
    "noImplicitReturns": true,
    "noImplicitThis": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noPropertyAccessFromIndexSignature": true,
    "noUncheckedIndexedAccess": true
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
}`,
		"src/main.test.ts": `/**
 * Test file for the main TypeScript module.
 */

import { UserService, CreateUserRequest, UpdateUserRequest } from './main';
import * as fs from 'fs/promises';
import * as path from 'path';
import * as os from 'os';

describe('UserService', () => {
    let service: UserService;

    beforeEach(() => {
        service = new UserService();
    });

    describe('createUser', () => {
        it('should create a user with valid data', () => {
            const request: CreateUserRequest = {
                name: 'Test User',
                email: 'test@example.com'
            };

            const user = service.createUser(request);

            expect(user.name).toBe('Test User');
            expect(user.email).toBe('test@example.com');
            expect(user.id).toBeGreaterThan(0);
            expect(user.createdAt).toBeInstanceOf(Date);
            expect(user.updatedAt).toBeInstanceOf(Date);
        });

        it('should assign sequential IDs', () => {
            const user1 = service.createUser({ name: 'User 1', email: 'user1@example.com' });
            const user2 = service.createUser({ name: 'User 2', email: 'user2@example.com' });

            expect(user2.id).toBe(user1.id + 1);
        });
    });

    describe('getUser', () => {
        it('should return user when found', () => {
            const createdUser = service.createUser({ name: 'Test User', email: 'test@example.com' });
            const retrievedUser = service.getUser(createdUser.id);

            expect(retrievedUser).toBeDefined();
            expect(retrievedUser?.id).toBe(createdUser.id);
            expect(retrievedUser?.name).toBe(createdUser.name);
        });

        it('should return undefined when user not found', () => {
            const user = service.getUser(999);
            expect(user).toBeUndefined();
        });
    });

    describe('listUsers', () => {
        it('should return empty array initially', () => {
            const users = service.listUsers();
            expect(users).toHaveLength(0);
        });

        it('should return all created users', () => {
            service.createUser({ name: 'User 1', email: 'user1@example.com' });
            service.createUser({ name: 'User 2', email: 'user2@example.com' });

            const users = service.listUsers();
            expect(users).toHaveLength(2);
        });
    });

    describe('updateUser', () => {
        it('should update user name', () => {
            const user = service.createUser({ name: 'Original Name', email: 'test@example.com' });
            const originalUpdatedAt = user.updatedAt;

            // Wait a moment to ensure updatedAt changes
            setTimeout(() => {
                const updatedUser = service.updateUser(user.id, { name: 'Updated Name' });

                expect(updatedUser).toBeDefined();
                expect(updatedUser?.name).toBe('Updated Name');
                expect(updatedUser?.email).toBe('test@example.com');
                expect(updatedUser?.updatedAt.getTime()).toBeGreaterThan(originalUpdatedAt.getTime());
            }, 1);
        });

        it('should return undefined for non-existent user', () => {
            const result = service.updateUser(999, { name: 'New Name' });
            expect(result).toBeUndefined();
        });
    });

    describe('deleteUser', () => {
        it('should delete existing user', () => {
            const user = service.createUser({ name: 'Test User', email: 'test@example.com' });

            const result = service.deleteUser(user.id);
            expect(result).toBe(true);

            const retrievedUser = service.getUser(user.id);
            expect(retrievedUser).toBeUndefined();
        });

        it('should return false for non-existent user', () => {
            const result = service.deleteUser(999);
            expect(result).toBe(false);
        });
    });

    describe('file operations', () => {
        let tempFile: string;

        beforeEach(async () => {
            const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'user-service-test-'));
            tempFile = path.join(tempDir, 'users.json');
        });

        afterEach(async () => {
            try {
                await fs.unlink(tempFile);
                await fs.rmdir(path.dirname(tempFile));
            } catch (error) {
                // Ignore cleanup errors
            }
        });

        it('should save and load users', async () => {
            // Create some users
            const user1 = service.createUser({ name: 'User 1', email: 'user1@example.com' });
            const user2 = service.createUser({ name: 'User 2', email: 'user2@example.com' });

            // Save to file
            await service.saveToFile(tempFile);

            // Create new service and load
            const newService = new UserService();
            await newService.loadFromFile(tempFile);

            // Verify users were loaded
            const loadedUsers = newService.listUsers();
            expect(loadedUsers).toHaveLength(2);

            const loadedUser1 = newService.getUser(user1.id);
            expect(loadedUser1).toBeDefined();
            expect(loadedUser1?.name).toBe(user1.name);
            expect(loadedUser1?.email).toBe(user1.email);
        });

        it('should handle loading from non-existent file', async () => {
            await expect(service.loadFromFile('/non/existent/file.json')).resolves.not.toThrow();
            
            const users = service.listUsers();
            expect(users).toHaveLength(0);
        });
    });

    describe('getStats', () => {
        it('should return correct statistics', () => {
            // Create some users
            service.createUser({ name: 'User 1', email: 'user1@example.com' });
            service.createUser({ name: 'User 2', email: 'user2@example.com' });

            const stats = service.getStats();
            expect(stats.totalUsers).toBe(2);
            expect(stats.averageAge).toBeGreaterThanOrEqual(0);
        });

        it('should handle empty user list', () => {
            const stats = service.getStats();
            expect(stats.totalUsers).toBe(0);
            expect(stats.averageAge).toBe(0);
        });
    });
});`,
		"jest.config.js": `module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  roots: ['<rootDir>/src'],
  testMatch: ['**/__tests__/**/*.ts', '**/?(*.)+(spec|test).ts'],
  transform: {
    '^.+\\.ts$': 'ts-jest',
  },
  collectCoverageFrom: [
    'src/**/*.ts',
    '!src/**/*.test.ts',
    '!src/**/*.spec.ts',
  ],
  coverageDirectory: 'coverage',
  coverageReporters: ['text', 'lcov', 'html'],
  testTimeout: 10000,
};`,
	},
	
	"java": {
		"src/main/java/com/test/Main.java": `package com.test;

import com.test.model.User;
import com.test.service.UserService;
import com.test.controller.UserController;

import java.util.List;
import java.util.logging.Logger;
import java.util.logging.Level;

/**
 * Main class for the test workspace Java application.
 */
public class Main {
    private static final Logger LOGGER = Logger.getLogger(Main.class.getName());

    public static void main(String[] args) {
        LOGGER.info("Starting Java test application");

        try {
            // Create user service and controller
            UserService userService = new UserService();
            UserController userController = new UserController(userService);

            // Create some test users
            User user1 = userController.createUser("Alice Johnson", "alice@example.com");
            User user2 = userController.createUser("Bob Wilson", "bob@example.com");
            User user3 = userController.createUser("Carol Brown", "carol@example.com");

            System.out.println("Created users:");
            printUser(user1);
            printUser(user2);
            printUser(user3);

            // List all users
            List<User> allUsers = userController.getAllUsers();
            System.out.println("\\nAll users (" + allUsers.size() + "):");
            allUsers.forEach(Main::printUser);

            // Update a user
            User updatedUser = userController.updateUser(1L, "Alice Smith", null);
            if (updatedUser != null) {
                System.out.println("\\nUpdated user:");
                printUser(updatedUser);
            }

            // Get a specific user
            User specificUser = userController.getUser(2L);
            if (specificUser != null) {
                System.out.println("\\nRetrieved user:");
                printUser(specificUser);
            }

            // Get user statistics
            UserController.UserStats stats = userController.getUserStats();
            System.out.println("\\nStatistics:");
            System.out.println("  Total users: " + stats.getTotalUsers());
            System.out.println("  Average age: " + String.format("%.2f", stats.getAverageAgeDays()) + " days");

            LOGGER.info("Java test application completed successfully");

        } catch (Exception e) {
            LOGGER.log(Level.SEVERE, "Application error", e);
            System.exit(1);
        }
    }

    private static void printUser(User user) {
        System.out.println("  " + user.getId() + ": " + user.getName() + 
                          " - " + user.getEmail() + 
                          " (created: " + user.getCreatedAt() + ")");
    }
}`,
		"src/main/java/com/test/model/User.java": `package com.test.model;

import java.time.LocalDateTime;
import java.util.Objects;

/**
 * User entity representing a user in the system.
 */
public class User {
    private Long id;
    private String name;
    private String email;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;

    // Constructors
    public User() {
        this.createdAt = LocalDateTime.now();
        this.updatedAt = LocalDateTime.now();
    }

    public User(String name, String email) {
        this();
        this.name = name;
        this.email = email;
    }

    public User(Long id, String name, String email) {
        this(name, email);
        this.id = id;
    }

    // Getters and Setters
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
        this.updatedAt = LocalDateTime.now();
    }

    public String getEmail() {
        return email;
    }

    public void setEmail(String email) {
        this.email = email;
        this.updatedAt = LocalDateTime.now();
    }

    public LocalDateTime getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(LocalDateTime createdAt) {
        this.createdAt = createdAt;
    }

    public LocalDateTime getUpdatedAt() {
        return updatedAt;
    }

    public void setUpdatedAt(LocalDateTime updatedAt) {
        this.updatedAt = updatedAt;
    }

    // Business methods
    public void touch() {
        this.updatedAt = LocalDateTime.now();
    }

    public long getAgeDays() {
        return java.time.Duration.between(createdAt, LocalDateTime.now()).toDays();
    }

    // Object methods
    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        User user = (User) o;
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
                ", name='" + name + '\\'' +
                ", email='" + email + '\\'' +
                ", createdAt=" + createdAt +
                ", updatedAt=" + updatedAt +
                '}';
    }

    // Builder pattern
    public static class Builder {
        private Long id;
        private String name;
        private String email;
        private LocalDateTime createdAt;
        private LocalDateTime updatedAt;

        public Builder id(Long id) {
            this.id = id;
            return this;
        }

        public Builder name(String name) {
            this.name = name;
            return this;
        }

        public Builder email(String email) {
            this.email = email;
            return this;
        }

        public Builder createdAt(LocalDateTime createdAt) {
            this.createdAt = createdAt;
            return this;
        }

        public Builder updatedAt(LocalDateTime updatedAt) {
            this.updatedAt = updatedAt;
            return this;
        }

        public User build() {
            User user = new User();
            user.id = this.id;
            user.name = this.name;
            user.email = this.email;
            user.createdAt = this.createdAt != null ? this.createdAt : LocalDateTime.now();
            user.updatedAt = this.updatedAt != null ? this.updatedAt : LocalDateTime.now();
            return user;
        }
    }

    public static Builder builder() {
        return new Builder();
    }
}`,
		"src/main/java/com/test/service/UserService.java": `package com.test.service;

import com.test.model.User;

import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;
import java.util.logging.Logger;
import java.util.stream.Collectors;

/**
 * Service class for managing users.
 */
public class UserService {
    private static final Logger LOGGER = Logger.getLogger(UserService.class.getName());

    private final Map<Long, User> users = new ConcurrentHashMap<>();
    private final AtomicLong idGenerator = new AtomicLong(1);

    /**
     * Create a new user.
     */
    public User createUser(String name, String email) {
        validateUserInput(name, email);

        User user = new User(idGenerator.getAndIncrement(), name, email);
        users.put(user.getId(), user);

        LOGGER.info("Created user: " + user.getName() + " (" + user.getEmail() + ")");
        return user;
    }

    /**
     * Get a user by ID.
     */
    public Optional<User> getUser(Long id) {
        if (id == null) {
            throw new IllegalArgumentException("User ID cannot be null");
        }

        User user = users.get(id);
        if (user != null) {
            LOGGER.fine("Retrieved user: " + user.getName());
        } else {
            LOGGER.warning("User not found: " + id);
        }
        return Optional.ofNullable(user);
    }

    /**
     * List all users.
     */
    public List<User> listUsers() {
        List<User> userList = new ArrayList<>(users.values());
        userList.sort(Comparator.comparing(User::getId));
        
        LOGGER.info("Listed " + userList.size() + " users");
        return userList;
    }

    /**
     * Update a user.
     */
    public Optional<User> updateUser(Long id, String name, String email) {
        if (id == null) {
            throw new IllegalArgumentException("User ID cannot be null");
        }

        User user = users.get(id);
        if (user == null) {
            LOGGER.warning("User not found for update: " + id);
            return Optional.empty();
        }

        boolean updated = false;
        if (name != null && !name.trim().isEmpty()) {
            user.setName(name.trim());
            updated = true;
        }
        if (email != null && !email.trim().isEmpty()) {
            validateEmail(email);
            user.setEmail(email.trim());
            updated = true;
        }

        if (updated) {
            user.touch();
            LOGGER.info("Updated user: " + user.getName());
        }

        return Optional.of(user);
    }

    /**
     * Delete a user.
     */
    public boolean deleteUser(Long id) {
        if (id == null) {
            throw new IllegalArgumentException("User ID cannot be null");
        }

        User removedUser = users.remove(id);
        if (removedUser != null) {
            LOGGER.info("Deleted user: " + removedUser.getName());
            return true;
        } else {
            LOGGER.warning("User not found for deletion: " + id);
            return false;
        }
    }

    /**
     * Find users by name (case-insensitive partial match).
     */
    public List<User> findUsersByName(String namePattern) {
        if (namePattern == null || namePattern.trim().isEmpty()) {
            return Collections.emptyList();
        }

        String pattern = namePattern.toLowerCase().trim();
        return users.values().stream()
                .filter(user -> user.getName().toLowerCase().contains(pattern))
                .sorted(Comparator.comparing(User::getName))
                .collect(Collectors.toList());
    }

    /**
     * Find user by email (exact match).
     */
    public Optional<User> findUserByEmail(String email) {
        if (email == null || email.trim().isEmpty()) {
            return Optional.empty();
        }

        return users.values().stream()
                .filter(user -> user.getEmail().equalsIgnoreCase(email.trim()))
                .findFirst();
    }

    /**
     * Get user count.
     */
    public int getUserCount() {
        return users.size();
    }

    /**
     * Check if a user exists.
     */
    public boolean userExists(Long id) {
        return id != null && users.containsKey(id);
    }

    /**
     * Clear all users (for testing purposes).
     */
    public void clearAllUsers() {
        users.clear();
        idGenerator.set(1);
        LOGGER.info("Cleared all users");
    }

    /**
     * Get user statistics.
     */
    public UserStats getStats() {
        List<User> userList = listUsers();
        double averageAge = userList.stream()
                .mapToLong(User::getAgeDays)
                .average()
                .orElse(0.0);

        return new UserStats(userList.size(), averageAge);
    }

    // Validation methods
    private void validateUserInput(String name, String email) {
        if (name == null || name.trim().isEmpty()) {
            throw new IllegalArgumentException("User name cannot be null or empty");
        }
        if (email == null || email.trim().isEmpty()) {
            throw new IllegalArgumentException("User email cannot be null or empty");
        }
        validateEmail(email);
        
        // Check for duplicate email
        if (findUserByEmail(email).isPresent()) {
            throw new IllegalArgumentException("User with email " + email + " already exists");
        }
    }

    private void validateEmail(String email) {
        if (!email.contains("@") || !email.contains(".")) {
            throw new IllegalArgumentException("Invalid email format: " + email);
        }
    }

    // Statistics class
    public static class UserStats {
        private final int totalUsers;
        private final double averageAgeDays;

        public UserStats(int totalUsers, double averageAgeDays) {
            this.totalUsers = totalUsers;
            this.averageAgeDays = averageAgeDays;
        }

        public int getTotalUsers() {
            return totalUsers;
        }

        public double getAverageAgeDays() {
            return averageAgeDays;
        }

        @Override
        public String toString() {
            return "UserStats{" +
                    "totalUsers=" + totalUsers +
                    ", averageAgeDays=" + averageAgeDays +
                    '}';
        }
    }
}`,
		"src/main/java/com/test/controller/UserController.java": `package com.test.controller;

import com.test.model.User;
import com.test.service.UserService;

import java.util.List;
import java.util.Optional;
import java.util.logging.Logger;

/**
 * Controller class for handling user-related HTTP requests.
 * In a real application, this would be annotated with @RestController.
 */
public class UserController {
    private static final Logger LOGGER = Logger.getLogger(UserController.class.getName());

    private final UserService userService;

    public UserController(UserService userService) {
        this.userService = userService;
    }

    /**
     * Create a new user.
     */
    public User createUser(String name, String email) {
        try {
            User user = userService.createUser(name, email);
            LOGGER.info("API: Created user " + user.getId());
            return user;
        } catch (Exception e) {
            LOGGER.severe("API: Error creating user: " + e.getMessage());
            throw e;
        }
    }

    /**
     * Get a user by ID.
     */
    public User getUser(Long id) {
        try {
            Optional<User> user = userService.getUser(id);
            if (user.isPresent()) {
                LOGGER.info("API: Retrieved user " + id);
                return user.get();
            } else {
                LOGGER.warning("API: User not found " + id);
                return null;
            }
        } catch (Exception e) {
            LOGGER.severe("API: Error retrieving user " + id + ": " + e.getMessage());
            throw e;
        }
    }

    /**
     * Get all users.
     */
    public List<User> getAllUsers() {
        try {
            List<User> users = userService.listUsers();
            LOGGER.info("API: Retrieved " + users.size() + " users");
            return users;
        } catch (Exception e) {
            LOGGER.severe("API: Error retrieving users: " + e.getMessage());
            throw e;
        }
    }

    /**
     * Update a user.
     */
    public User updateUser(Long id, String name, String email) {
        try {
            Optional<User> user = userService.updateUser(id, name, email);
            if (user.isPresent()) {
                LOGGER.info("API: Updated user " + id);
                return user.get();
            } else {
                LOGGER.warning("API: User not found for update " + id);
                return null;
            }
        } catch (Exception e) {
            LOGGER.severe("API: Error updating user " + id + ": " + e.getMessage());
            throw e;
        }
    }

    /**
     * Delete a user.
     */
    public boolean deleteUser(Long id) {
        try {
            boolean deleted = userService.deleteUser(id);
            if (deleted) {
                LOGGER.info("API: Deleted user " + id);
            } else {
                LOGGER.warning("API: User not found for deletion " + id);
            }
            return deleted;
        } catch (Exception e) {
            LOGGER.severe("API: Error deleting user " + id + ": " + e.getMessage());
            throw e;
        }
    }

    /**
     * Search users by name.
     */
    public List<User> searchUsers(String namePattern) {
        try {
            List<User> users = userService.findUsersByName(namePattern);
            LOGGER.info("API: Found " + users.size() + " users matching '" + namePattern + "'");
            return users;
        } catch (Exception e) {
            LOGGER.severe("API: Error searching users: " + e.getMessage());
            throw e;
        }
    }

    /**
     * Find user by email.
     */
    public User findUserByEmail(String email) {
        try {
            Optional<User> user = userService.findUserByEmail(email);
            if (user.isPresent()) {
                LOGGER.info("API: Found user by email " + email);
                return user.get();
            } else {
                LOGGER.info("API: No user found with email " + email);
                return null;
            }
        } catch (Exception e) {
            LOGGER.severe("API: Error finding user by email: " + e.getMessage());
            throw e;
        }
    }

    /**
     * Get user statistics.
     */
    public UserStats getUserStats() {
        try {
            UserService.UserStats serviceStats = userService.getStats();
            UserStats stats = new UserStats(
                serviceStats.getTotalUsers(),
                serviceStats.getAverageAgeDays()
            );
            LOGGER.info("API: Retrieved user statistics");
            return stats;
        } catch (Exception e) {
            LOGGER.severe("API: Error retrieving user statistics: " + e.getMessage());
            throw e;
        }
    }

    /**
     * Check if user exists.
     */
    public boolean userExists(Long id) {
        try {
            boolean exists = userService.userExists(id);
            LOGGER.fine("API: User " + id + " exists: " + exists);
            return exists;
        } catch (Exception e) {
            LOGGER.severe("API: Error checking user existence: " + e.getMessage());
            throw e;
        }
    }

    /**
     * DTO class for user statistics.
     */
    public static class UserStats {
        private final int totalUsers;
        private final double averageAgeDays;

        public UserStats(int totalUsers, double averageAgeDays) {
            this.totalUsers = totalUsers;
            this.averageAgeDays = averageAgeDays;
        }

        public int getTotalUsers() {
            return totalUsers;
        }

        public double getAverageAgeDays() {
            return averageAgeDays;
        }

        @Override
        public String toString() {
            return "UserStats{" +
                    "totalUsers=" + totalUsers +
                    ", averageAgeDays=" + String.format("%.2f", averageAgeDays) +
                    '}';
        }
    }
}`,
		"src/test/java/com/test/UserServiceTest.java": `package com.test;

import com.test.model.User;
import com.test.service.UserService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Test class for UserService.
 */
@DisplayName("UserService Tests")
class UserServiceTest {

    private UserService userService;

    @BeforeEach
    void setUp() {
        userService = new UserService();
    }

    @Test
    @DisplayName("Should create user with valid data")
    void testCreateUser() {
        // Given
        String name = "John Doe";
        String email = "john.doe@example.com";

        // When
        User user = userService.createUser(name, email);

        // Then
        assertNotNull(user);
        assertEquals(name, user.getName());
        assertEquals(email, user.getEmail());
        assertNotNull(user.getId());
        assertNotNull(user.getCreatedAt());
        assertNotNull(user.getUpdatedAt());
    }

    @Test
    @DisplayName("Should throw exception for null name")
    void testCreateUserWithNullName() {
        // Given
        String email = "test@example.com";

        // When & Then
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser(null, email);
        });
    }

    @Test
    @DisplayName("Should throw exception for empty name")
    void testCreateUserWithEmptyName() {
        // Given
        String email = "test@example.com";

        // When & Then
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser("", email);
        });
    }

    @Test
    @DisplayName("Should throw exception for invalid email")
    void testCreateUserWithInvalidEmail() {
        // Given
        String name = "John Doe";
        String invalidEmail = "invalid-email";

        // When & Then
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser(name, invalidEmail);
        });
    }

    @Test
    @DisplayName("Should throw exception for duplicate email")
    void testCreateUserWithDuplicateEmail() {
        // Given
        String name1 = "John Doe";
        String name2 = "Jane Doe";
        String email = "same@example.com";

        // When
        userService.createUser(name1, email);

        // Then
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser(name2, email);
        });
    }

    @Test
    @DisplayName("Should get user by ID")
    void testGetUser() {
        // Given
        User createdUser = userService.createUser("John Doe", "john@example.com");

        // When
        Optional<User> retrievedUser = userService.getUser(createdUser.getId());

        // Then
        assertTrue(retrievedUser.isPresent());
        assertEquals(createdUser.getId(), retrievedUser.get().getId());
        assertEquals(createdUser.getName(), retrievedUser.get().getName());
        assertEquals(createdUser.getEmail(), retrievedUser.get().getEmail());
    }

    @Test
    @DisplayName("Should return empty optional for non-existent user")
    void testGetUserNotFound() {
        // When
        Optional<User> user = userService.getUser(999L);

        // Then
        assertFalse(user.isPresent());
    }

    @Test
    @DisplayName("Should throw exception for null ID")
    void testGetUserWithNullId() {
        // When & Then
        assertThrows(IllegalArgumentException.class, () -> {
            userService.getUser(null);
        });
    }

    @Test
    @DisplayName("Should list all users")
    void testListUsers() {
        // Given
        userService.createUser("User 1", "user1@example.com");
        userService.createUser("User 2", "user2@example.com");

        // When
        List<User> users = userService.listUsers();

        // Then
        assertEquals(2, users.size());
        // Verify sorting by ID
        assertTrue(users.get(0).getId() < users.get(1).getId());
    }

    @Test
    @DisplayName("Should return empty list when no users exist")
    void testListUsersEmpty() {
        // When
        List<User> users = userService.listUsers();

        // Then
        assertTrue(users.isEmpty());
    }

    @Test
    @DisplayName("Should update user name")
    void testUpdateUserName() {
        // Given
        User user = userService.createUser("Original Name", "user@example.com");
        String newName = "Updated Name";

        // When
        Optional<User> updatedUser = userService.updateUser(user.getId(), newName, null);

        // Then
        assertTrue(updatedUser.isPresent());
        assertEquals(newName, updatedUser.get().getName());
        assertEquals(user.getEmail(), updatedUser.get().getEmail());
    }

    @Test
    @DisplayName("Should update user email")
    void testUpdateUserEmail() {
        // Given
        User user = userService.createUser("John Doe", "old@example.com");
        String newEmail = "new@example.com";

        // When
        Optional<User> updatedUser = userService.updateUser(user.getId(), null, newEmail);

        // Then
        assertTrue(updatedUser.isPresent());
        assertEquals(user.getName(), updatedUser.get().getName());
        assertEquals(newEmail, updatedUser.get().getEmail());
    }

    @Test
    @DisplayName("Should return empty optional when updating non-existent user")
    void testUpdateUserNotFound() {
        // When
        Optional<User> result = userService.updateUser(999L, "New Name", null);

        // Then
        assertFalse(result.isPresent());
    }

    @Test
    @DisplayName("Should delete user")
    void testDeleteUser() {
        // Given
        User user = userService.createUser("John Doe", "john@example.com");

        // When
        boolean deleted = userService.deleteUser(user.getId());

        // Then
        assertTrue(deleted);
        assertFalse(userService.getUser(user.getId()).isPresent());
    }

    @Test
    @DisplayName("Should return false when deleting non-existent user")
    void testDeleteUserNotFound() {
        // When
        boolean deleted = userService.deleteUser(999L);

        // Then
        assertFalse(deleted);
    }

    @Test
    @DisplayName("Should find users by name pattern")
    void testFindUsersByName() {
        // Given
        userService.createUser("Alice Johnson", "alice@example.com");
        userService.createUser("Bob Johnson", "bob@example.com");
        userService.createUser("Charlie Smith", "charlie@example.com");

        // When
        List<User> johnsonUsers = userService.findUsersByName("Johnson");

        // Then
        assertEquals(2, johnsonUsers.size());
        assertTrue(johnsonUsers.stream().allMatch(u -> u.getName().contains("Johnson")));
    }

    @Test
    @DisplayName("Should find user by email")
    void testFindUserByEmail() {
        // Given
        User createdUser = userService.createUser("John Doe", "john@example.com");

        // When
        Optional<User> foundUser = userService.findUserByEmail("john@example.com");

        // Then
        assertTrue(foundUser.isPresent());
        assertEquals(createdUser.getId(), foundUser.get().getId());
    }

    @Test
    @DisplayName("Should return correct user count")
    void testGetUserCount() {
        // Given
        assertEquals(0, userService.getUserCount());

        userService.createUser("User 1", "user1@example.com");
        userService.createUser("User 2", "user2@example.com");

        // When
        int count = userService.getUserCount();

        // Then
        assertEquals(2, count);
    }

    @Test
    @DisplayName("Should check if user exists")
    void testUserExists() {
        // Given
        User user = userService.createUser("John Doe", "john@example.com");

        // When & Then
        assertTrue(userService.userExists(user.getId()));
        assertFalse(userService.userExists(999L));
    }

    @Test
    @DisplayName("Should clear all users")
    void testClearAllUsers() {
        // Given
        userService.createUser("User 1", "user1@example.com");
        userService.createUser("User 2", "user2@example.com");
        assertEquals(2, userService.getUserCount());

        // When
        userService.clearAllUsers();

        // Then
        assertEquals(0, userService.getUserCount());
        assertTrue(userService.listUsers().isEmpty());
    }

    @Test
    @DisplayName("Should get user statistics")
    void testGetStats() {
        // Given
        userService.createUser("User 1", "user1@example.com");
        userService.createUser("User 2", "user2@example.com");

        // When
        UserService.UserStats stats = userService.getStats();

        // Then
        assertEquals(2, stats.getTotalUsers());
        assertTrue(stats.getAverageAgeDays() >= 0);
    }
}`,
		"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.test</groupId>
    <artifactId>test-workspace-java</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <name>Test Workspace Java</name>
    <description>A test workspace Java application</description>
    <url>https://github.com/test/test-workspace-java</url>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
        <junit.version>5.10.0</junit.version>
        <maven.compiler.plugin.version>3.11.0</maven.compiler.plugin.version>
        <maven.surefire.plugin.version>3.1.2</maven.surefire.plugin.version>
        <maven.jar.plugin.version>3.3.0</maven.jar.plugin.version>
        <exec.maven.plugin.version>3.1.0</exec.maven.plugin.version>
    </properties>

    <dependencies>
        <!-- Test Dependencies -->
        <dependency>
            <groupId>org.junit.jupiter</groupId>
            <artifactId>junit-jupiter</artifactId>
            <version>\${junit.version}</version>
            <scope>test</scope>
        </dependency>
        
        <dependency>
            <groupId>org.junit.jupiter</groupId>
            <artifactId>junit-jupiter-engine</artifactId>
            <version>\${junit.version}</version>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <!-- Compiler Plugin -->
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>\${maven.compiler.plugin.version}</version>
                <configuration>
                    <source>\${maven.compiler.source}</source>
                    <target>\${maven.compiler.target}</target>
                </configuration>
            </plugin>

            <!-- Surefire Plugin for Tests -->
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-surefire-plugin</artifactId>
                <version>\${maven.surefire.plugin.version}</version>
                <configuration>
                    <includes>
                        <include>**/*Test.java</include>
                        <include>**/*Tests.java</include>
                    </includes>
                </configuration>
            </plugin>

            <!-- JAR Plugin -->
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-jar-plugin</artifactId>
                <version>\${maven.jar.plugin.version}</version>
                <configuration>
                    <archive>
                        <manifest>
                            <addClasspath>true</addClasspath>
                            <mainClass>com.test.Main</mainClass>
                        </manifest>
                    </archive>
                </configuration>
            </plugin>

            <!-- Exec Plugin for Running -->
            <plugin>
                <groupId>org.codehaus.mojo</groupId>
                <artifactId>exec-maven-plugin</artifactId>
                <version>\${exec.maven.plugin.version}</version>
                <configuration>
                    <mainClass>com.test.Main</mainClass>
                </configuration>
            </plugin>
        </plugins>
    </build>

    <profiles>
        <profile>
            <id>coverage</id>
            <build>
                <plugins>
                    <plugin>
                        <groupId>org.jacoco</groupId>
                        <artifactId>jacoco-maven-plugin</artifactId>
                        <version>0.8.10</version>
                        <executions>
                            <execution>
                                <goals>
                                    <goal>prepare-agent</goal>
                                </goals>
                            </execution>
                            <execution>
                                <id>report</id>
                                <phase>test</phase>
                                <goals>
                                    <goal>report</goal>
                                </goals>
                            </execution>
                        </executions>
                    </plugin>
                </plugins>
            </build>
        </profile>
    </profiles>
</project>`,
	},
}

// GetTestProjectFiles returns project files for a specific language
func GetTestProjectFiles(language string) map[string]string {
	if files, exists := TestProjectFiles[language]; exists {
		return files
	}
	
	// Return empty map for unsupported languages
	return make(map[string]string)
}



// GetSampleFileContent returns sample file content for testing specific scenarios
func GetSampleFileContent(language string, scenario string) string {
	scenarios := map[string]map[string]string{
		"go": {
			"simple_function": `package main

import "fmt"

// Add adds two integers and returns the result
func Add(a, b int) int {
	return a + b
}

func main() {
	result := Add(2, 3)
	fmt.Printf("Result: %d\n", result)
}`,
			"struct_definition": `package main

type Person struct {
	Name string
	Age  int
}

func NewPerson(name string, age int) *Person {
	return &Person{Name: name, Age: age}
}

func (p *Person) Greet() string {
	return fmt.Sprintf("Hello, I'm %s and I'm %d years old", p.Name, p.Age)
}`,
		},
		"python": {
			"simple_function": `def add(a: int, b: int) -> int:
    """Add two integers and return the result."""
    return a + b

def main():
    result = add(2, 3)
    print(f"Result: {result}")

if __name__ == "__main__":
    main()`,
			"class_definition": `class Person:
    """A simple Person class for testing."""
    
    def __init__(self, name: str, age: int):
        self.name = name
        self.age = age
    
    def greet(self) -> str:
        return f"Hello, I'm {self.name} and I'm {self.age} years old"
    
    def __str__(self) -> str:
        return f"Person(name='{self.name}', age={self.age})"`,
		},
		"typescript": {
			"simple_function": `function add(a: number, b: number): number {
    return a + b;
}

function main(): void {
    const result = add(2, 3);
    console.log(` + "`" + `Result: ${result}` + "`" + `);
}

main();`,
			"interface_definition": `interface Person {
    name: string;
    age: number;
}

class PersonImpl implements Person {
    constructor(public name: string, public age: number) {}
    
    greet(): string {
        return ` + "`" + `Hello, I'm ${this.name} and I'm ${this.age} years old` + "`" + `;
    }
}`,
		},
		"java": {
			"simple_class": `public class Calculator {
    public static int add(int a, int b) {
        return a + b;
    }
    
    public static void main(String[] args) {
        int result = add(2, 3);
        System.out.println("Result: " + result);
    }
}`,
			"interface_definition": `public interface Greeter {
    String greet();
}

public class Person implements Greeter {
    private final String name;
    private final int age;
    
    public Person(String name, int age) {
        this.name = name;
        this.age = age;
    }
    
    @Override
    public String greet() {
        return String.format("Hello, I'm %s and I'm %d years old", name, age);
    }
}`,
		},
	}
	
	if langScenarios, langExists := scenarios[language]; langExists {
		if content, scenarioExists := langScenarios[scenario]; scenarioExists {
			return content
		}
	}
	
	return ""
}

// CreateComplexProjectStructure creates a more complex project structure for advanced testing
func CreateComplexProjectStructure(baseDir string, languages []string) error {
	if err := CreateTestWorkspaceDirectories(baseDir, languages); err != nil {
		return err
	}
	
	// Create cross-language integration files
	for _, lang := range languages {
		langDir := filepath.Join(baseDir, lang)
		
		// Create multiple source files for each language
		scenarios := []string{"simple_function", "class_definition", "interface_definition"}
		for i, scenario := range scenarios {
			content := GetSampleFileContent(lang, scenario)
			if content != "" {
				filename := fmt.Sprintf("example_%d.%s", i+1, getFileExtension(lang))
				filePath := filepath.Join(langDir, "src", filename)
				
				// Ensure parent directory exists
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(filePath), err)
				}
				
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", filePath, err)
				}
			}
		}
	}
	
	return nil
}


// ValidateProjectFiles ensures all expected project files exist for a language
func ValidateProjectFiles(projectDir string, language string) error {
	expectedFiles := GetTestProjectFiles(language)
	
	for relativePath := range expectedFiles {
		fullPath := filepath.Join(projectDir, relativePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("expected project file does not exist: %s", fullPath)
		}
	}
	
	return nil
}

// GetLanguageFilePatterns returns file patterns typically associated with each language
func GetLanguageFilePatterns(language string) []string {
	patterns := map[string][]string{
		"go": {"*.go", "go.mod", "go.sum", "go.work"},
		"python": {"*.py", "*.pyi", "requirements.txt", "setup.py", "pyproject.toml"},
		"javascript": {"*.js", "*.jsx", "package.json", "package-lock.json"},
		"typescript": {"*.ts", "*.tsx", "tsconfig.json", "package.json"},
		"java": {"*.java", "pom.xml", "build.gradle", "*.properties"},
		"rust": {"*.rs", "Cargo.toml", "Cargo.lock"},
		"c": {"*.c", "*.h", "Makefile", "CMakeLists.txt"},
		"cpp": {"*.cpp", "*.hpp", "*.cc", "*.hh", "Makefile", "CMakeLists.txt"},
	}
	
	if filePatterns, exists := patterns[language]; exists {
		return filePatterns
	}
	return []string{}
}

// CreateMinimalProject creates a minimal but functional project for a language
func CreateMinimalProject(projectDir string, language string) error {
	// Ensure the project directory exists
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}
	
	// Get the minimal set of files for the language
	files := GetTestProjectFiles(language)
	if len(files) == 0 {
		return fmt.Errorf("no project files defined for language: %s", language)
	}
	
	// Create only the essential files (main file + config file)
	essentialFiles := getEssentialFiles(language, files)
	
	for relativePath, content := range essentialFiles {
		fullPath := filepath.Join(projectDir, relativePath)
		
		// Create parent directories
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
		}
		
		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}
	}
	
	return nil
}

// getEssentialFiles returns the minimum set of files needed for a functional project
func getEssentialFiles(language string, allFiles map[string]string) map[string]string {
	essential := make(map[string]string)
	
	// Define essential files for each language
	essentialPatterns := map[string][]string{
		"go":         {"main.go", "go.mod"},
		"python":     {"main.py", "requirements.txt"},
		"javascript": {"package.json"},
		"typescript": {"src/main.ts", "package.json", "tsconfig.json"},
		"java":       {"src/main/java/com/test/Main.java", "pom.xml"},
		"rust":       {"src/main.rs", "Cargo.toml"},
	}
	
	patterns, exists := essentialPatterns[language]
	if !exists {
		// Fallback: return first two files
		count := 0
		for path, content := range allFiles {
			essential[path] = content
			count++
			if count >= 2 {
				break
			}
		}
		return essential
	}
	
	// Find matching files
	for _, pattern := range patterns {
		if content, exists := allFiles[pattern]; exists {
			essential[pattern] = content
		}
	}
	
	return essential
}