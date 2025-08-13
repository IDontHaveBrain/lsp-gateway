package testutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"lsp-gateway/src/utils"
)

// CreateTempTestDir creates a temporary test directory with automatic cleanup
func CreateTempTestDir(t *testing.T, prefix string) string {
	tempDir, err := os.MkdirTemp("", prefix+"-*")
	require.NoError(t, err, "Failed to create temp directory")

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return tempDir
}

// WriteTestFile writes a file with proper error handling and cleanup tracking
func WriteTestFile(t *testing.T, path, content string) {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err, "Failed to create directory for file: %s", path)

	err = os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "Failed to write file: %s", path)
}

// CreateGoProject creates a Go project with go.mod and standard structure
func CreateGoProject(t *testing.T, tempDir, moduleName string) string {
	projectDir := filepath.Join(tempDir, "go-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err, "Failed to create Go project directory")

	// Create go.mod
	goModContent := "module " + moduleName + "\n\ngo 1.21\n\nrequire (\n\tgithub.com/gorilla/mux v1.8.0\n)\n"
	WriteTestFile(t, filepath.Join(projectDir, "go.mod"), goModContent)

	// Create main.go
	mainGoContent := `package main

import (
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
)

type Server struct {
	router *mux.Router
	port   string
}

func NewServer(port string) *Server {
	return &Server{
		router: mux.NewRouter(),
		port:   port,
	}
}

func (s *Server) Start() error {
	fmt.Printf("Starting server on port %s\n", s.port)
	return http.ListenAndServe(":"+s.port, s.router)
}

func (s *Server) HandleFunc(path string, handler http.HandlerFunc) {
	s.router.HandleFunc(path, handler)
}

func main() {
	server := NewServer("8080")
	server.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	if err := server.Start(); err != nil {
		panic(err)
	}
}
`
	WriteTestFile(t, filepath.Join(projectDir, "main.go"), mainGoContent)

	// Create handlers.go
	handlersContent := `package main

import (
	"encoding/json"
	"net/http"
)

type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

type UserHandler struct {
	users []User
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		users: make([]User, 0),
	}
}

func (uh *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uh.users)
}

func (uh *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	user.ID = len(uh.users) + 1
	uh.users = append(uh.users, user)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
`
	WriteTestFile(t, filepath.Join(projectDir, "handlers.go"), handlersContent)

	// Create test file
	testContent := `package main

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	server := NewServer("8080")
	if server == nil {
		t.Error("Expected server to be created")
	}
	
	if server.port != "8080" {
		t.Errorf("Expected port 8080, got %s", server.port)
	}
}

func TestNewUserHandler(t *testing.T) {
	handler := NewUserHandler()
	if handler == nil {
		t.Error("Expected handler to be created")
	}
	
	if len(handler.users) != 0 {
		t.Errorf("Expected empty users slice, got %d users", len(handler.users))
	}
}
`
	WriteTestFile(t, filepath.Join(projectDir, "main_test.go"), testContent)

	return projectDir
}

// CreatePythonProject creates a Python project with standard structure
func CreatePythonProject(t *testing.T, dir string) string {
	projectDir := filepath.Join(dir, "python-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err, "Failed to create Python project directory")

	// Create requirements.txt
	requirementsContent := `flask==2.3.3
requests==2.31.0
pytest==7.4.2
black==23.7.0
flake8==6.0.0
`
	WriteTestFile(t, filepath.Join(projectDir, "requirements.txt"), requirementsContent)

	// Create setup.py
	setupContent := `from setuptools import setup, find_packages

setup(
    name="test-python-project",
    version="1.0.0",
    description="Test Python project for LSP Gateway",
    author="Test Author",
    packages=find_packages(),
    install_requires=[
        "flask>=2.0.0",
        "requests>=2.25.0",
    ],
    python_requires=">=3.8",
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
    ],
)
`
	WriteTestFile(t, filepath.Join(projectDir, "setup.py"), setupContent)

	// Create main app.py
	appContent := `from flask import Flask, request, jsonify
import requests
from typing import List, Dict, Optional

app = Flask(__name__)

class User:
    def __init__(self, user_id: int, name: str, email: str):
        self.id = user_id
        self.name = name
        self.email = email
    
    def to_dict(self) -> Dict:
        return {
            "id": self.id,
            "name": self.name,
            "email": self.email
        }

class UserService:
    def __init__(self):
        self.users: List[User] = []
        self.next_id = 1
    
    def create_user(self, name: str, email: str) -> User:
        user = User(self.next_id, name, email)
        self.users.append(user)
        self.next_id += 1
        return user
    
    def get_user(self, user_id: int) -> Optional[User]:
        for user in self.users:
            if user.id == user_id:
                return user
        return None
    
    def get_all_users(self) -> List[User]:
        return self.users.copy()

user_service = UserService()

@app.route('/health', methods=['GET'])
def health_check():
    return jsonify({"status": "ok"})

@app.route('/users', methods=['GET'])
def get_users():
    users = user_service.get_all_users()
    return jsonify([user.to_dict() for user in users])

@app.route('/users', methods=['POST'])
def create_user():
    data = request.get_json()
    if not data or 'name' not in data or 'email' not in data:
        return jsonify({"error": "Name and email are required"}), 400
    
    user = user_service.create_user(data['name'], data['email'])
    return jsonify(user.to_dict()), 201

@app.route('/users/<int:user_id>', methods=['GET'])
def get_user(user_id: int):
    user = user_service.get_user(user_id)
    if not user:
        return jsonify({"error": "User not found"}), 404
    return jsonify(user.to_dict())

if __name__ == '__main__':
    app.run(debug=True, port=5000)
`
	WriteTestFile(t, filepath.Join(projectDir, "app.py"), appContent)

	// Create utils module
	utilsDir := filepath.Join(projectDir, "utils")
	err = os.MkdirAll(utilsDir, 0755)
	require.NoError(t, err, "Failed to create utils directory")

	WriteTestFile(t, filepath.Join(utilsDir, "__init__.py"), "")

	validatorContent := `import re
from typing import Union

def validate_email(email: str) -> bool:
    """Validate email format using regex."""
    pattern = r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
    return re.match(pattern, email) is not None

def validate_name(name: str) -> bool:
    """Validate name is not empty and has reasonable length."""
    return isinstance(name, str) and 1 <= len(name.strip()) <= 100

class ValidationError(Exception):
    """Custom exception for validation errors."""
    pass

def validate_user_data(data: dict) -> Union[dict, None]:
    """Validate user creation data."""
    if not isinstance(data, dict):
        raise ValidationError("Data must be a dictionary")
    
    name = data.get('name', '').strip()
    email = data.get('email', '').strip()
    
    if not validate_name(name):
        raise ValidationError("Invalid name")
    
    if not validate_email(email):
        raise ValidationError("Invalid email format")
    
    return {"name": name, "email": email}
`
	WriteTestFile(t, filepath.Join(utilsDir, "validators.py"), validatorContent)

	// Create test file
	testContent := `import unittest
from app import app, user_service, User, UserService
from utils.validators import validate_email, validate_name, ValidationError

class TestUserService(unittest.TestCase):
    def setUp(self):
        self.service = UserService()
    
    def test_create_user(self):
        user = self.service.create_user("John Doe", "john@example.com")
        self.assertIsInstance(user, User)
        self.assertEqual(user.name, "John Doe")
        self.assertEqual(user.email, "john@example.com")
        self.assertEqual(user.id, 1)
    
    def test_get_user(self):
        user = self.service.create_user("Jane Doe", "jane@example.com")
        found_user = self.service.get_user(user.id)
        self.assertEqual(found_user, user)
    
    def test_get_nonexistent_user(self):
        user = self.service.get_user(999)
        self.assertIsNone(user)

class TestValidators(unittest.TestCase):
    def test_validate_email(self):
        self.assertTrue(validate_email("test@example.com"))
        self.assertFalse(validate_email("invalid-email"))
    
    def test_validate_name(self):
        self.assertTrue(validate_name("John Doe"))
        self.assertFalse(validate_name(""))
        self.assertFalse(validate_name("   "))

if __name__ == '__main__':
    unittest.main()
`
	WriteTestFile(t, filepath.Join(projectDir, "test_app.py"), testContent)

	return projectDir
}

// CreateTypeScriptProject creates a TypeScript project with package.json and tsconfig
func CreateTypeScriptProject(t *testing.T, dir string) string {
	projectDir := filepath.Join(dir, "typescript-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err, "Failed to create TypeScript project directory")

	// Create package.json
	packageJsonContent := `{
  "name": "typescript-test-project",
  "version": "1.0.0",
  "description": "Test TypeScript project for LSP Gateway",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "dev": "ts-node src/index.ts",
    "test": "jest",
    "lint": "eslint src/**/*.ts",
    "start": "node dist/index.js"
  },
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5",
    "helmet": "^7.0.0"
  },
  "devDependencies": {
    "@types/express": "^4.17.17",
    "@types/cors": "^2.8.13",
    "@types/node": "^20.5.0",
    "@types/jest": "^29.5.4",
    "typescript": "^5.1.6",
    "ts-node": "^10.9.1",
    "jest": "^29.6.2",
    "ts-jest": "^29.1.1",
    "eslint": "^8.47.0",
    "@typescript-eslint/parser": "^6.4.1",
    "@typescript-eslint/eslint-plugin": "^6.4.1"
  },
  "keywords": ["typescript", "express", "test"],
  "author": "Test Author",
  "license": "MIT"
}
`
	WriteTestFile(t, filepath.Join(projectDir, "package.json"), packageJsonContent)

	// Create tsconfig.json
	tsconfigContent := `{
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
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "removeComments": true,
    "noImplicitAny": true,
    "noImplicitReturns": true,
    "noImplicitThis": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "experimentalDecorators": true,
    "emitDecoratorMetadata": true,
    "resolveJsonModule": true
  },
  "include": [
    "src/**/*"
  ],
  "exclude": [
    "node_modules",
    "dist",
    "**/*.test.ts"
  ]
}
`
	WriteTestFile(t, filepath.Join(projectDir, "tsconfig.json"), tsconfigContent)

	// Create src directory and files
	srcDir := filepath.Join(projectDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	require.NoError(t, err, "Failed to create src directory")

	// Create main index.ts
	indexContent := `import express, { Request, Response, NextFunction } from 'express';
import cors from 'cors';
import helmet from 'helmet';
import { UserController } from './controllers/UserController';
import { UserService } from './services/UserService';
import { ErrorHandler } from './middleware/ErrorHandler';

interface AppConfig {
  port: number;
  corsOrigin: string;
}

class App {
  private app: express.Application;
  private config: AppConfig;
  private userController: UserController;

  constructor(config: AppConfig) {
    this.app = express();
    this.config = config;
    this.userController = new UserController(new UserService());
    this.initializeMiddleware();
    this.initializeRoutes();
    this.initializeErrorHandling();
  }

  private initializeMiddleware(): void {
    this.app.use(helmet());
    this.app.use(cors({ origin: this.config.corsOrigin }));
    this.app.use(express.json());
    this.app.use(express.urlencoded({ extended: true }));
  }

  private initializeRoutes(): void {
    this.app.get('/health', (req: Request, res: Response) => {
      res.status(200).json({ status: 'ok', timestamp: new Date().toISOString() });
    });

    this.app.use('/api/users', this.userController.getRouter());
  }

  private initializeErrorHandling(): void {
    this.app.use(ErrorHandler.handle);
  }

  public listen(): void {
    this.app.listen(this.config.port, () => {
      console.log(` + "`Server running on port ${this.config.port}`" + `);
    });
  }

  public getApp(): express.Application {
    return this.app;
  }
}

const config: AppConfig = {
  port: parseInt(process.env.PORT || '3000'),
  corsOrigin: process.env.CORS_ORIGIN || '*'
};

const app = new App(config);
app.listen();

export { App };
`
	WriteTestFile(t, filepath.Join(srcDir, "index.ts"), indexContent)

	// Create models
	modelsDir := filepath.Join(srcDir, "models")
	err = os.MkdirAll(modelsDir, 0755)
	require.NoError(t, err, "Failed to create models directory")

	userModelContent := `export interface IUser {
  id: number;
  name: string;
  email: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateUserRequest {
  name: string;
  email: string;
}

export interface UpdateUserRequest {
  name?: string;
  email?: string;
}

export class User implements IUser {
  public id: number;
  public name: string;
  public email: string;
  public createdAt: Date;
  public updatedAt: Date;

  constructor(id: number, name: string, email: string) {
    this.id = id;
    this.name = name;
    this.email = email;
    this.createdAt = new Date();
    this.updatedAt = new Date();
  }

  public toJSON(): IUser {
    return {
      id: this.id,
      name: this.name,
      email: this.email,
      createdAt: this.createdAt,
      updatedAt: this.updatedAt
    };
  }

  public updateName(name: string): void {
    this.name = name;
    this.updatedAt = new Date();
  }

  public updateEmail(email: string): void {
    this.email = email;
    this.updatedAt = new Date();
  }
}
`
	WriteTestFile(t, filepath.Join(modelsDir, "User.ts"), userModelContent)

	// Create services
	servicesDir := filepath.Join(srcDir, "services")
	err = os.MkdirAll(servicesDir, 0755)
	require.NoError(t, err, "Failed to create services directory")

	userServiceContent := `import { User, CreateUserRequest, UpdateUserRequest } from '../models/User';

export class UserService {
  private users: User[] = [];
  private nextId: number = 1;

  public async createUser(request: CreateUserRequest): Promise<User> {
    const user = new User(this.nextId++, request.name, request.email);
    this.users.push(user);
    return user;
  }

  public async getUserById(id: number): Promise<User | undefined> {
    return this.users.find(user => user.id === id);
  }

  public async getAllUsers(): Promise<User[]> {
    return [...this.users];
  }

  public async updateUser(id: number, request: UpdateUserRequest): Promise<User | undefined> {
    const user = await this.getUserById(id);
    if (!user) {
      return undefined;
    }

    if (request.name) {
      user.updateName(request.name);
    }
    if (request.email) {
      user.updateEmail(request.email);
    }

    return user;
  }

  public async deleteUser(id: number): Promise<boolean> {
    const index = this.users.findIndex(user => user.id === id);
    if (index === -1) {
      return false;
    }

    this.users.splice(index, 1);
    return true;
  }

  public async getUserCount(): Promise<number> {
    return this.users.length;
  }
}
`
	WriteTestFile(t, filepath.Join(servicesDir, "UserService.ts"), userServiceContent)

	return projectDir
}

// CreateJavaProject creates a Java project with Maven pom.xml
func CreateJavaProject(t *testing.T, dir string) string {
	projectDir := filepath.Join(dir, "java-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err, "Failed to create Java project directory")

	// Create pom.xml
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.test</groupId>
    <artifactId>java-test-project</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <name>Java Test Project</name>
    <description>Test Java project for LSP Gateway testing</description>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
        <spring.boot.version>2.7.0</spring.boot.version>
        <junit.version>5.8.2</junit.version>
    </properties>

    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
            <version>${spring.boot.version}</version>
        </dependency>

        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-data-jpa</artifactId>
            <version>${spring.boot.version}</version>
        </dependency>

        <dependency>
            <groupId>com.fasterxml.jackson.core</groupId>
            <artifactId>jackson-databind</artifactId>
            <version>2.13.3</version>
        </dependency>

        <dependency>
            <groupId>org.junit.jupiter</groupId>
            <artifactId>junit-jupiter</artifactId>
            <version>${junit.version}</version>
            <scope>test</scope>
        </dependency>

        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-test</artifactId>
            <version>${spring.boot.version}</version>
            <scope>test</scope>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.10.1</version>
                <configuration>
                    <source>11</source>
                    <target>11</target>
                </configuration>
            </plugin>

            <plugin>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-maven-plugin</artifactId>
                <version>${spring.boot.version}</version>
            </plugin>
        </plugins>
    </build>
</project>
`
	WriteTestFile(t, filepath.Join(projectDir, "pom.xml"), pomContent)

	// Create source directories
	srcMainJava := filepath.Join(projectDir, "src", "main", "java", "com", "test")
	srcTestJava := filepath.Join(projectDir, "src", "test", "java", "com", "test")

	err = os.MkdirAll(srcMainJava, 0755)
	require.NoError(t, err, "Failed to create main Java source directory")
	err = os.MkdirAll(srcTestJava, 0755)
	require.NoError(t, err, "Failed to create test Java source directory")

	// Create Main.java
	mainJavaContent := `package com.test;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.web.servlet.config.annotation.CorsRegistry;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

@SpringBootApplication
public class Main {
    public static void main(String[] args) {
        SpringApplication.run(Main.class, args);
    }

    @Bean
    public WebMvcConfigurer corsConfigurer() {
        return new WebMvcConfigurer() {
            @Override
            public void addCorsMappings(CorsRegistry registry) {
                registry.addMapping("/**")
                        .allowedOrigins("*")
                        .allowedMethods("GET", "POST", "PUT", "DELETE")
                        .allowedHeaders("*");
            }
        };
    }
}
`
	WriteTestFile(t, filepath.Join(srcMainJava, "Main.java"), mainJavaContent)

	// Create model
	modelDir := filepath.Join(srcMainJava, "model")
	err = os.MkdirAll(modelDir, 0755)
	require.NoError(t, err, "Failed to create model directory")

	userModelContent := `package com.test.model;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.LocalDateTime;
import java.util.Objects;

public class User {
    @JsonProperty("id")
    private Long id;
    
    @JsonProperty("name")
    private String name;
    
    @JsonProperty("email")
    private String email;
    
    @JsonProperty("createdAt")
    private LocalDateTime createdAt;
    
    @JsonProperty("updatedAt")
    private LocalDateTime updatedAt;

    public User() {
        this.createdAt = LocalDateTime.now();
        this.updatedAt = LocalDateTime.now();
    }

    public User(Long id, String name, String email) {
        this();
        this.id = id;
        this.name = name;
        this.email = email;
    }

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
                ", name='" + name + '\'' +
                ", email='" + email + '\'' +
                ", createdAt=" + createdAt +
                ", updatedAt=" + updatedAt +
                '}';
    }
}
`
	WriteTestFile(t, filepath.Join(modelDir, "User.java"), userModelContent)

	// Create service
	serviceDir := filepath.Join(srcMainJava, "service")
	err = os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err, "Failed to create service directory")

	userServiceContent := `package com.test.service;

import com.test.model.User;
import org.springframework.stereotype.Service;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicLong;

@Service
public class UserService {
    private final Map<Long, User> users = new ConcurrentHashMap<>();
    private final AtomicLong idGenerator = new AtomicLong(1);

    public User createUser(String name, String email) {
        if (name == null || name.trim().isEmpty()) {
            throw new IllegalArgumentException("Name cannot be empty");
        }
        if (email == null || email.trim().isEmpty()) {
            throw new IllegalArgumentException("Email cannot be empty");
        }

        Long id = idGenerator.getAndIncrement();
        User user = new User(id, name.trim(), email.trim());
        users.put(id, user);
        return user;
    }

    public Optional<User> getUserById(Long id) {
        return Optional.ofNullable(users.get(id));
    }

    public List<User> getAllUsers() {
        return new ArrayList<>(users.values());
    }

    public Optional<User> updateUser(Long id, String name, String email) {
        User user = users.get(id);
        if (user == null) {
            return Optional.empty();
        }

        if (name != null && !name.trim().isEmpty()) {
            user.setName(name.trim());
        }
        if (email != null && !email.trim().isEmpty()) {
            user.setEmail(email.trim());
        }

        return Optional.of(user);
    }

    public boolean deleteUser(Long id) {
        return users.remove(id) != null;
    }

    public long getUserCount() {
        return users.size();
    }

    public void clearAllUsers() {
        users.clear();
    }
}
`
	WriteTestFile(t, filepath.Join(serviceDir, "UserService.java"), userServiceContent)

	// Create controller
	controllerDir := filepath.Join(srcMainJava, "controller")
	err = os.MkdirAll(controllerDir, 0755)
	require.NoError(t, err, "Failed to create controller directory")

	userControllerContent := `package com.test.controller;

import com.test.model.User;
import com.test.service.UserService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import java.util.List;
import java.util.Map;
import java.util.Optional;

@RestController
@RequestMapping("/api/users")
public class UserController {

    private final UserService userService;

    @Autowired
    public UserController(UserService userService) {
        this.userService = userService;
    }

    @GetMapping("/health")
    public ResponseEntity<Map<String, String>> health() {
        return ResponseEntity.ok(Map.of("status", "ok"));
    }

    @GetMapping
    public ResponseEntity<List<User>> getAllUsers() {
        List<User> users = userService.getAllUsers();
        return ResponseEntity.ok(users);
    }

    @GetMapping("/{id}")
    public ResponseEntity<User> getUserById(@PathVariable Long id) {
        Optional<User> user = userService.getUserById(id);
        return user.map(ResponseEntity::ok)
                  .orElse(ResponseEntity.notFound().build());
    }

    @PostMapping
    public ResponseEntity<User> createUser(@RequestBody Map<String, String> request) {
        try {
            String name = request.get("name");
            String email = request.get("email");
            User user = userService.createUser(name, email);
            return ResponseEntity.status(HttpStatus.CREATED).body(user);
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().build();
        }
    }

    @PutMapping("/{id}")
    public ResponseEntity<User> updateUser(@PathVariable Long id, @RequestBody Map<String, String> request) {
        String name = request.get("name");
        String email = request.get("email");
        Optional<User> user = userService.updateUser(id, name, email);
        return user.map(ResponseEntity::ok)
                  .orElse(ResponseEntity.notFound().build());
    }

    @DeleteMapping("/{id}")
    public ResponseEntity<Void> deleteUser(@PathVariable Long id) {
        boolean deleted = userService.deleteUser(id);
        return deleted ? ResponseEntity.noContent().build() 
                      : ResponseEntity.notFound().build();
    }

    @GetMapping("/count")
    public ResponseEntity<Map<String, Long>> getUserCount() {
        long count = userService.getUserCount();
        return ResponseEntity.ok(Map.of("count", count));
    }
}
`
	WriteTestFile(t, filepath.Join(controllerDir, "UserController.java"), userControllerContent)

	// Create test file
	userServiceTestContent := `package com.test;

import com.test.model.User;
import com.test.service.UserService;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.BeforeEach;
import static org.junit.jupiter.api.Assertions.*;
import java.util.Optional;

public class UserServiceTest {
    
    private UserService userService;

    @BeforeEach
    void setUp() {
        userService = new UserService();
    }

    @Test
    void testCreateUser() {
        User user = userService.createUser("John Doe", "john@example.com");
        
        assertNotNull(user);
        assertEquals("John Doe", user.getName());
        assertEquals("john@example.com", user.getEmail());
        assertNotNull(user.getId());
    }

    @Test
    void testGetUserById() {
        User created = userService.createUser("Jane Doe", "jane@example.com");
        Optional<User> found = userService.getUserById(created.getId());
        
        assertTrue(found.isPresent());
        assertEquals(created.getName(), found.get().getName());
        assertEquals(created.getEmail(), found.get().getEmail());
    }

    @Test
    void testGetNonExistentUser() {
        Optional<User> user = userService.getUserById(999L);
        assertFalse(user.isPresent());
    }

    @Test
    void testCreateUserWithEmptyName() {
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser("", "test@example.com");
        });
    }

    @Test
    void testCreateUserWithEmptyEmail() {
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser("Test User", "");
        });
    }

    @Test
    void testUpdateUser() {
        User created = userService.createUser("Original Name", "original@example.com");
        Optional<User> updated = userService.updateUser(created.getId(), "Updated Name", "updated@example.com");
        
        assertTrue(updated.isPresent());
        assertEquals("Updated Name", updated.get().getName());
        assertEquals("updated@example.com", updated.get().getEmail());
    }

    @Test
    void testDeleteUser() {
        User created = userService.createUser("Delete Me", "delete@example.com");
        boolean deleted = userService.deleteUser(created.getId());
        
        assertTrue(deleted);
        Optional<User> found = userService.getUserById(created.getId());
        assertFalse(found.isPresent());
    }

    @Test
    void testGetUserCount() {
        assertEquals(0, userService.getUserCount());
        
        userService.createUser("User 1", "user1@example.com");
        assertEquals(1, userService.getUserCount());
        
        userService.createUser("User 2", "user2@example.com");
        assertEquals(2, userService.getUserCount());
    }
}
`
	WriteTestFile(t, filepath.Join(srcTestJava, "UserServiceTest.java"), userServiceTestContent)

	return projectDir
}

// CreateRustProject creates a Rust project with Cargo.toml
func CreateRustProject(t *testing.T, dir string) string {
	projectDir := filepath.Join(dir, "rust-project")
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err, "Failed to create Rust project directory")

	// Create Cargo.toml
	cargoContent := `[package]
name = "rust-test-project"
version = "1.0.0"
edition = "2021"
authors = ["Test Author <test@example.com>"]
description = "Test Rust project for LSP Gateway testing"
license = "MIT"

[dependencies]
tokio = { version = "1.32", features = ["full"] }
axum = "0.6"
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
uuid = { version = "1.4", features = ["v4", "serde"] }
chrono = { version = "0.4", features = ["serde"] }
anyhow = "1.0"
thiserror = "1.0"

[dev-dependencies]
tokio-test = "0.4"
`
	WriteTestFile(t, filepath.Join(projectDir, "Cargo.toml"), cargoContent)

	// Create src directory
	srcDir := filepath.Join(projectDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	require.NoError(t, err, "Failed to create src directory")

	// Create main.rs
	mainContent := `use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::Json,
    routing::{get, post, put, delete},
    Router,
};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use tokio::net::TcpListener;
use uuid::Uuid;

mod models;
mod services;
mod handlers;

use models::User;
use services::UserService;
use handlers::UserHandler;

type AppState = Arc<Mutex<UserService>>;

#[derive(Clone)]
pub struct AppConfig {
    pub port: u16,
    pub host: String,
}

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            port: 3000,
            host: "127.0.0.1".to_string(),
        }
    }
}

pub async fn create_app(config: AppConfig) -> Router {
    let user_service = UserService::new();
    let app_state: AppState = Arc::new(Mutex::new(user_service));

    Router::new()
        .route("/health", get(health_check))
        .route("/api/users", get(UserHandler::get_all_users))
        .route("/api/users", post(UserHandler::create_user))
        .route("/api/users/:id", get(UserHandler::get_user_by_id))
        .route("/api/users/:id", put(UserHandler::update_user))
        .route("/api/users/:id", delete(UserHandler::delete_user))
        .with_state(app_state)
}

async fn health_check() -> Json<serde_json::Value> {
    Json(serde_json::json!({
        "status": "ok",
        "timestamp": chrono::Utc::now().to_rfc3339()
    }))
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let config = AppConfig::default();
    let app = create_app(config.clone()).await;

    let addr = format!("{}:{}", config.host, config.port);
    let listener = TcpListener::bind(&addr).await?;
    
    println!("Server running on http://{}", addr);
    
    axum::serve(listener, app).await?;
    
    Ok(())
}
`
	WriteTestFile(t, filepath.Join(srcDir, "main.rs"), mainContent)

	// Create models.rs
	modelsContent := `use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct User {
    pub id: Uuid,
    pub name: String,
    pub email: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

impl User {
    pub fn new(name: String, email: String) -> Self {
        let now = Utc::now();
        Self {
            id: Uuid::new_v4(),
            name,
            email,
            created_at: now,
            updated_at: now,
        }
    }

    pub fn update_name(&mut self, name: String) {
        self.name = name;
        self.updated_at = Utc::now();
    }

    pub fn update_email(&mut self, email: String) {
        self.email = email;
        self.updated_at = Utc::now();
    }
}

#[derive(Debug, Deserialize)]
pub struct CreateUserRequest {
    pub name: String,
    pub email: String,
}

#[derive(Debug, Deserialize)]
pub struct UpdateUserRequest {
    pub name: Option<String>,
    pub email: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct UserResponse {
    pub id: Uuid,
    pub name: String,
    pub email: String,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

impl From<User> for UserResponse {
    fn from(user: User) -> Self {
        Self {
            id: user.id,
            name: user.name,
            email: user.email,
            created_at: user.created_at,
            updated_at: user.updated_at,
        }
    }
}

#[derive(Debug, thiserror::Error)]
pub enum UserError {
    #[error("User not found")]
    NotFound,
    #[error("Invalid user data: {0}")]
    InvalidData(String),
    #[error("User already exists")]
    AlreadyExists,
}
`
	WriteTestFile(t, filepath.Join(srcDir, "models.rs"), modelsContent)

	// Create services.rs
	servicesContent := `use crate::models::{User, UserError, CreateUserRequest, UpdateUserRequest};
use std::collections::HashMap;
use uuid::Uuid;

#[derive(Debug)]
pub struct UserService {
    users: HashMap<Uuid, User>,
}

impl UserService {
    pub fn new() -> Self {
        Self {
            users: HashMap::new(),
        }
    }

    pub fn create_user(&mut self, request: CreateUserRequest) -> Result<User, UserError> {
        if request.name.trim().is_empty() {
            return Err(UserError::InvalidData("Name cannot be empty".to_string()));
        }
        
        if request.email.trim().is_empty() {
            return Err(UserError::InvalidData("Email cannot be empty".to_string()));
        }

        // Simple email validation
        if !request.email.contains('@') {
            return Err(UserError::InvalidData("Invalid email format".to_string()));
        }

        let user = User::new(request.name.trim().to_string(), request.email.trim().to_string());
        self.users.insert(user.id, user.clone());
        Ok(user)
    }

    pub fn get_user_by_id(&self, id: Uuid) -> Result<User, UserError> {
        self.users.get(&id).cloned().ok_or(UserError::NotFound)
    }

    pub fn get_all_users(&self) -> Vec<User> {
        self.users.values().cloned().collect()
    }

    pub fn update_user(&mut self, id: Uuid, request: UpdateUserRequest) -> Result<User, UserError> {
        let user = self.users.get_mut(&id).ok_or(UserError::NotFound)?;

        if let Some(name) = request.name {
            if name.trim().is_empty() {
                return Err(UserError::InvalidData("Name cannot be empty".to_string()));
            }
            user.update_name(name.trim().to_string());
        }

        if let Some(email) = request.email {
            if email.trim().is_empty() {
                return Err(UserError::InvalidData("Email cannot be empty".to_string()));
            }
            if !email.contains('@') {
                return Err(UserError::InvalidData("Invalid email format".to_string()));
            }
            user.update_email(email.trim().to_string());
        }

        Ok(user.clone())
    }

    pub fn delete_user(&mut self, id: Uuid) -> Result<(), UserError> {
        self.users.remove(&id).ok_or(UserError::NotFound)?;
        Ok(())
    }

    pub fn get_user_count(&self) -> usize {
        self.users.len()
    }

    pub fn clear_all_users(&mut self) {
        self.users.clear();
    }
}

impl Default for UserService {
    fn default() -> Self {
        Self::new()
    }
}
`
	WriteTestFile(t, filepath.Join(srcDir, "services.rs"), servicesContent)

	// Create handlers.rs
	handlersContent := `use crate::models::{CreateUserRequest, UpdateUserRequest, UserResponse, UserError};
use crate::services::UserService;
use crate::AppState;
use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::Json,
};
use serde_json::Value;
use uuid::Uuid;

pub struct UserHandler;

impl UserHandler {
    pub async fn get_all_users(State(state): State<AppState>) -> Result<Json<Vec<UserResponse>>, StatusCode> {
        let service = state.lock().map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
        let users = service.get_all_users();
        let responses: Vec<UserResponse> = users.into_iter().map(UserResponse::from).collect();
        Ok(Json(responses))
    }

    pub async fn get_user_by_id(
        State(state): State<AppState>,
        Path(id): Path<Uuid>,
    ) -> Result<Json<UserResponse>, StatusCode> {
        let service = state.lock().map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
        match service.get_user_by_id(id) {
            Ok(user) => Ok(Json(UserResponse::from(user))),
            Err(UserError::NotFound) => Err(StatusCode::NOT_FOUND),
            Err(_) => Err(StatusCode::INTERNAL_SERVER_ERROR),
        }
    }

    pub async fn create_user(
        State(state): State<AppState>,
        Json(request): Json<CreateUserRequest>,
    ) -> Result<(StatusCode, Json<UserResponse>), StatusCode> {
        let mut service = state.lock().map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
        match service.create_user(request) {
            Ok(user) => Ok((StatusCode::CREATED, Json(UserResponse::from(user)))),
            Err(UserError::InvalidData(_)) => Err(StatusCode::BAD_REQUEST),
            Err(_) => Err(StatusCode::INTERNAL_SERVER_ERROR),
        }
    }

    pub async fn update_user(
        State(state): State<AppState>,
        Path(id): Path<Uuid>,
        Json(request): Json<UpdateUserRequest>,
    ) -> Result<Json<UserResponse>, StatusCode> {
        let mut service = state.lock().map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
        match service.update_user(id, request) {
            Ok(user) => Ok(Json(UserResponse::from(user))),
            Err(UserError::NotFound) => Err(StatusCode::NOT_FOUND),
            Err(UserError::InvalidData(_)) => Err(StatusCode::BAD_REQUEST),
            Err(_) => Err(StatusCode::INTERNAL_SERVER_ERROR),
        }
    }

    pub async fn delete_user(
        State(state): State<AppState>,
        Path(id): Path<Uuid>,
    ) -> Result<StatusCode, StatusCode> {
        let mut service = state.lock().map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
        match service.delete_user(id) {
            Ok(()) => Ok(StatusCode::NO_CONTENT),
            Err(UserError::NotFound) => Err(StatusCode::NOT_FOUND),
            Err(_) => Err(StatusCode::INTERNAL_SERVER_ERROR),
        }
    }
}
`
	WriteTestFile(t, filepath.Join(srcDir, "handlers.rs"), handlersContent)

	// Create tests directory
	testsDir := filepath.Join(projectDir, "tests")
	err = os.MkdirAll(testsDir, 0755)
	require.NoError(t, err, "Failed to create tests directory")

	// Create integration test
	integrationTestContent := `use rust_test_project::models::{CreateUserRequest, UpdateUserRequest};
use rust_test_project::services::UserService;
use uuid::Uuid;

#[tokio::test]
async fn test_user_service_create_user() {
    let mut service = UserService::new();
    let request = CreateUserRequest {
        name: "John Doe".to_string(),
        email: "john@example.com".to_string(),
    };

    let user = service.create_user(request).unwrap();
    assert_eq!(user.name, "John Doe");
    assert_eq!(user.email, "john@example.com");
    assert_eq!(service.get_user_count(), 1);
}

#[tokio::test]
async fn test_user_service_get_user_by_id() {
    let mut service = UserService::new();
    let request = CreateUserRequest {
        name: "Jane Doe".to_string(),
        email: "jane@example.com".to_string(),
    };

    let created_user = service.create_user(request).unwrap();
    let found_user = service.get_user_by_id(created_user.id).unwrap();
    
    assert_eq!(created_user.id, found_user.id);
    assert_eq!(created_user.name, found_user.name);
    assert_eq!(created_user.email, found_user.email);
}

#[tokio::test]
async fn test_user_service_update_user() {
    let mut service = UserService::new();
    let request = CreateUserRequest {
        name: "Original Name".to_string(),
        email: "original@example.com".to_string(),
    };

    let created_user = service.create_user(request).unwrap();
    let update_request = UpdateUserRequest {
        name: Some("Updated Name".to_string()),
        email: Some("updated@example.com".to_string()),
    };

    let updated_user = service.update_user(created_user.id, update_request).unwrap();
    assert_eq!(updated_user.name, "Updated Name");
    assert_eq!(updated_user.email, "updated@example.com");
    assert!(updated_user.updated_at > updated_user.created_at);
}

#[tokio::test]
async fn test_user_service_delete_user() {
    let mut service = UserService::new();
    let request = CreateUserRequest {
        name: "Delete Me".to_string(),
        email: "delete@example.com".to_string(),
    };

    let created_user = service.create_user(request).unwrap();
    assert_eq!(service.get_user_count(), 1);

    service.delete_user(created_user.id).unwrap();
    assert_eq!(service.get_user_count(), 0);
    
    let result = service.get_user_by_id(created_user.id);
    assert!(result.is_err());
}

#[tokio::test]
async fn test_user_service_invalid_data() {
    let mut service = UserService::new();
    
    // Test empty name
    let request = CreateUserRequest {
        name: "".to_string(),
        email: "test@example.com".to_string(),
    };
    assert!(service.create_user(request).is_err());
    
    // Test empty email
    let request = CreateUserRequest {
        name: "Test User".to_string(),
        email: "".to_string(),
    };
    assert!(service.create_user(request).is_err());
    
    // Test invalid email
    let request = CreateUserRequest {
        name: "Test User".to_string(),
        email: "invalid-email".to_string(),
    };
    assert!(service.create_user(request).is_err());
}
`
	WriteTestFile(t, filepath.Join(testsDir, "integration_test.rs"), integrationTestContent)

	return projectDir
}

// GetFileURI returns the file URI for a given file path
func GetFileURI(filePath string) string {
	return utils.FilePathToURI(filePath)
}

// GetAbsolutePath returns the absolute path for a relative path within a directory
func GetAbsolutePath(baseDir, relativePath string) string {
	return filepath.Join(baseDir, relativePath)
}
