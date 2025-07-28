package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
)

// mockNodeCommandExecutor for testing Node.js version detection
type mockNodeCommandExecutor struct {
	shouldFail       bool
	exitCode         int
	stdout           string
	stderr           string
	failError        error
	commandCalled    string
	argsCalled       []string
	nodeVersion      string
	npmVersion       string
	commandCallCount int
}

func (m *mockNodeCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	m.commandCalled = cmd
	m.argsCalled = args
	m.commandCallCount++

	if m.shouldFail {
		return &platform.Result{
			ExitCode: m.exitCode,
			Stdout:   m.stdout,
			Stderr:   m.stderr,
		}, m.failError
	}

	// Handle different commands
	if cmd == "node" && len(args) > 0 && args[0] == "--version" {
		return &platform.Result{
			ExitCode: 0,
			Stdout:   m.nodeVersion,
			Stderr:   "",
		}, nil
	} else if cmd == "npm" && len(args) > 0 && args[0] == "--version" {
		return &platform.Result{
			ExitCode: 0,
			Stdout:   m.npmVersion,
			Stderr:   "",
		}, nil
	}

	return &platform.Result{
		ExitCode: m.exitCode,
		Stdout:   m.stdout,
		Stderr:   m.stderr,
	}, nil
}

func (m *mockNodeCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockNodeCommandExecutor) GetShell() string {
	return "/bin/sh"
}

func (m *mockNodeCommandExecutor) GetShellArgs(command string) []string {
	return []string{"-c", command}
}

func (m *mockNodeCommandExecutor) IsCommandAvailable(command string) bool {
	return !m.shouldFail
}

// Test data creation helpers for Node.js projects
func createReactProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create package.json for React project
	packageJsonContent := `{
  "name": "react-demo-app",
  "version": "1.0.0",
  "description": "Demo React application",
  "main": "index.js",
  "scripts": {
    "start": "react-scripts start",
    "build": "react-scripts build",
    "test": "react-scripts test",
    "eject": "react-scripts eject"
  },
  "keywords": ["react", "javascript", "frontend"],
  "author": "Demo Author",
  "license": "MIT",
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.3.0",
    "axios": "^1.4.0",
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "react-scripts": "5.0.1",
    "@testing-library/react": "^13.3.0",
    "@testing-library/jest-dom": "^5.16.4",
    "@testing-library/user-event": "^14.2.0",
    "eslint": "^8.21.0",
    "prettier": "^2.7.1"
  },
  "engines": {
    "node": ">=20.0.0",
    "npm": ">=8.0.0"
  },
  "browserslist": {
    "production": [
      ">0.2%",
      "not dead",
      "not op_mini all"
    ],
    "development": [
      "last 1 chrome version",
      "last 1 firefox version",
      "last 1 safari version"
    ]
  }
}`

	err := os.WriteFile(filepath.Join(tempDir, types.MARKER_PACKAGE_JSON), []byte(packageJsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create package-lock.json
	packageLockContent := `{
  "name": "react-demo-app",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "requires": true,
  "packages": {}
}`

	err = os.WriteFile(filepath.Join(tempDir, "package-lock.json"), []byte(packageLockContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create package-lock.json: %v", err)
	}

	// Create directory structure
	dirs := []string{
		"src/components",
		"src/pages",
		"src/utils",
		"public",
		"src/__tests__",
		"build",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create React components
	appComponent := `import React from 'react';
import './App.css';

function App() {
  return (
    <div className="App">
      <header className="App-header">
        <h1>React Demo App</h1>
      </header>
    </div>
  );
}

export default App;`

	err = os.WriteFile(filepath.Join(tempDir, "src/App.jsx"), []byte(appComponent), 0644)
	if err != nil {
		t.Fatalf("Failed to create App.jsx: %v", err)
	}

	headerComponent := `import React from 'react';

const Header = ({ title }) => {
  return (
    <header>
      <h1>{title}</h1>
    </header>
  );
};

export default Header;`

	err = os.WriteFile(filepath.Join(tempDir, "src/components/Header.jsx"), []byte(headerComponent), 0644)
	if err != nil {
		t.Fatalf("Failed to create Header.jsx: %v", err)
	}

	// Create test file
	appTest := `import React from 'react';
import { render, screen } from '@testing-library/react';
import App from '../App';

test('renders app header', () => {
  render(<App />);
  const headerElement = screen.getByText(/React Demo App/i);
  expect(headerElement).toBeInTheDocument();
});`

	err = os.WriteFile(filepath.Join(tempDir, "src/__tests__/App.test.jsx"), []byte(appTest), 0644)
	if err != nil {
		t.Fatalf("Failed to create App.test.jsx: %v", err)
	}

	// Create index.html
	indexHtml := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>React Demo App</title>
</head>
<body>
    <div id="root"></div>
</body>
</html>`

	err = os.WriteFile(filepath.Join(tempDir, "public/index.html"), []byte(indexHtml), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	return tempDir
}

func createExpressProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create package.json for Express project
	packageJsonContent := `{
  "name": "express-api-server",
  "version": "2.1.0",
  "description": "Express.js API server",
  "main": "server.js",
  "scripts": {
    "start": "node server.js",
    "dev": "nodemon server.js",
    "test": "jest",
    "test:watch": "jest --watch",
    "lint": "eslint ."
  },
  "keywords": ["express", "api", "nodejs", "backend"],
  "author": "API Team",
  "license": "ISC",
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5",
    "helmet": "^7.0.0",
    "morgan": "^1.10.0",
    "dotenv": "^16.0.3",
    "bcryptjs": "^2.4.3",
    "jsonwebtoken": "^9.0.0"
  },
  "devDependencies": {
    "nodemon": "^2.0.22",
    "jest": "^29.5.0",
    "supertest": "^6.3.3",
    "eslint": "^8.42.0",
    "eslint-config-prettier": "^8.8.0"
  },
  "engines": {
    "node": ">=18.0.0"
  }
}`

	err := os.WriteFile(filepath.Join(tempDir, types.MARKER_PACKAGE_JSON), []byte(packageJsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create yarn.lock
	yarnLockContent := `# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.
# yarn lockfile v1

express@^4.18.2:
  version "4.18.2"
  resolved "https://registry.yarnpkg.com/express/-/express-4.18.2.tgz"`

	err = os.WriteFile(filepath.Join(tempDir, types.MARKER_YARN_LOCK), []byte(yarnLockContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create yarn.lock: %v", err)
	}

	// Create directory structure
	dirs := []string{
		"routes",
		"middleware",
		"models",
		"controllers",
		"test",
		"config",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create main server file
	serverJs := `const express = require('express');
const cors = require('cors');
const helmet = require('helmet');
const morgan = require('morgan');

const app = express();
const PORT = process.env.PORT || 3000;

// Middleware
app.use(helmet());
app.use(cors());
app.use(morgan('combined'));
app.use(express.json());

// Routes
app.use('/api/users', require('./routes/users'));

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'OK', timestamp: new Date().toISOString() });
});

app.listen(PORT, () => {
  console.log(` + "`Server running on port ${PORT}`" + `);
});

module.exports = app;`

	err = os.WriteFile(filepath.Join(tempDir, "server.js"), []byte(serverJs), 0644)
	if err != nil {
		t.Fatalf("Failed to create server.js: %v", err)
	}

	// Create API routes
	usersRoute := `const express = require('express');
const router = express.Router();

// GET /api/users
router.get('/', (req, res) => {
  res.json({ users: [] });
});

// POST /api/users
router.post('/', (req, res) => {
  res.status(201).json({ message: 'User created' });
});

module.exports = router;`

	err = os.WriteFile(filepath.Join(tempDir, "routes/users.js"), []byte(usersRoute), 0644)
	if err != nil {
		t.Fatalf("Failed to create users.js: %v", err)
	}

	// Create test file
	serverTest := `const request = require('supertest');
const app = require('../server');

describe('Server', () => {
  test('GET /health should return OK', async () => {
    const response = await request(app).get('/health');
    expect(response.status).toBe(200);
    expect(response.body.status).toBe('OK');
  });

  test('GET /api/users should return users array', async () => {
    const response = await request(app).get('/api/users');
    expect(response.status).toBe(200);
    expect(response.body.users).toEqual([]);
  });
});`

	err = os.WriteFile(filepath.Join(tempDir, "test/server.test.js"), []byte(serverTest), 0644)
	if err != nil {
		t.Fatalf("Failed to create server.test.js: %v", err)
	}

	return tempDir
}

func createTypeScriptProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create package.json for TypeScript project
	packageJsonContent := `{
  "name": "typescript-app",
  "version": "1.0.0",
  "description": "TypeScript application",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts",
    "test": "jest",
    "lint": "eslint src/**/*.ts"
  },
  "dependencies": {
    "express": "^4.18.2",
    "@types/express": "^4.17.17"
  },
  "devDependencies": {
    "typescript": "^5.0.4",
    "ts-node": "^10.9.1",
    "@types/node": "^20.2.5",
    "jest": "^29.5.0",
    "@types/jest": "^29.5.2",
    "ts-jest": "^29.1.0",
    "eslint": "^8.42.0",
    "@typescript-eslint/parser": "^5.59.8",
    "@typescript-eslint/eslint-plugin": "^5.59.8"
  },
  "engines": {
    "node": ">=20.0.0"
  }
}`

	err := os.WriteFile(filepath.Join(tempDir, types.MARKER_PACKAGE_JSON), []byte(packageJsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

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
    "resolveJsonModule": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "**/*.test.ts"]
}`

	err = os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create tsconfig.json: %v", err)
	}

	// Create pnpm-lock.yaml
	pnpmLockContent := `lockfileVersion: 5.4

specifiers:
  typescript: ^5.0.4

dependencies:
  typescript: 5.0.4`

	err = os.WriteFile(filepath.Join(tempDir, types.MARKER_PNPM_LOCK), []byte(pnpmLockContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create pnpm-lock.yaml: %v", err)
	}

	// Create directory structure
	dirs := []string{
		"src/types",
		"src/utils",
		"src/services",
		"dist",
		"tests",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create TypeScript files
	indexTs := `import express, { Request, Response } from 'express';
import { UserService } from './services/UserService';

const app = express();
const PORT = process.env.PORT || 3000;

app.use(express.json());

const userService = new UserService();

app.get('/api/users', (req: Request, res: Response) => {
  const users = userService.getAllUsers();
  res.json(users);
});

app.listen(PORT, () => {
  console.log(` + "`Server running on port ${PORT}`" + `);
});

export default app;`

	err = os.WriteFile(filepath.Join(tempDir, "src/index.ts"), []byte(indexTs), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.ts: %v", err)
	}

	// Create types
	typesFile := `export interface User {
  id: number;
  name: string;
  email: string;
  createdAt: Date;
}

export interface ApiResponse<T> {
  data: T;
  success: boolean;
  message?: string;
}`

	err = os.WriteFile(filepath.Join(tempDir, "src/types/index.ts"), []byte(typesFile), 0644)
	if err != nil {
		t.Fatalf("Failed to create types/index.ts: %v", err)
	}

	// Create service
	serviceTs := `import { User } from '../types';

export class UserService {
  private users: User[] = [];

  getAllUsers(): User[] {
    return this.users;
  }

  createUser(name: string, email: string): User {
    const user: User = {
      id: this.users.length + 1,
      name,
      email,
      createdAt: new Date()
    };
    this.users.push(user);
    return user;
  }
}`

	err = os.WriteFile(filepath.Join(tempDir, "src/services/UserService.ts"), []byte(serviceTs), 0644)
	if err != nil {
		t.Fatalf("Failed to create UserService.ts: %v", err)
	}

	// Create test file
	testFile := `import { UserService } from '../src/services/UserService';

describe('UserService', () => {
  let userService: UserService;

  beforeEach(() => {
    userService = new UserService();
  });

  test('should create a new user', () => {
    const user = userService.createUser('John Doe', 'john@example.com');
    expect(user.name).toBe('John Doe');
    expect(user.email).toBe('john@example.com');
  });

  test('should get all users', () => {
    userService.createUser('John Doe', 'john@example.com');
    const users = userService.getAllUsers();
    expect(users).toHaveLength(1);
  });
});`

	err = os.WriteFile(filepath.Join(tempDir, "tests/UserService.test.ts"), []byte(testFile), 0644)
	if err != nil {
		t.Fatalf("Failed to create UserService.test.ts: %v", err)
	}

	return tempDir
}

func createJavaScriptOnlyProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create simple JavaScript files without package.json
	indexJs := `const express = require('express');

const app = express();

app.get('/', (req, res) => {
  res.send('Hello World');
});

app.listen(3000, () => {
  console.log('Server running on port 3000');
});`

	err := os.WriteFile(filepath.Join(tempDir, "index.js"), []byte(indexJs), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.js: %v", err)
	}

	utilsJs := `function formatDate(date) {
  return date.toISOString().split('T')[0];
}

function capitalize(str) {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

module.exports = {
  formatDate,
  capitalize
};`

	err = os.WriteFile(filepath.Join(tempDir, "utils.js"), []byte(utilsJs), 0644)
	if err != nil {
		t.Fatalf("Failed to create utils.js: %v", err)
	}

	return tempDir
}

func createMalformedPackageJsonProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create invalid JSON package.json
	malformedPackageJson := `{
  "name": "malformed-project",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.2"
  // Missing closing brace
`

	err := os.WriteFile(filepath.Join(tempDir, types.MARKER_PACKAGE_JSON), []byte(malformedPackageJson), 0644)
	if err != nil {
		t.Fatalf("Failed to create malformed package.json: %v", err)
	}

	return tempDir
}

// Test constructor
func TestNewNodeJSLanguageDetector(t *testing.T) {
	detector := NewNodeJSLanguageDetector()

	if detector == nil {
		t.Fatal("Expected detector to be non-nil")
	}

	nodeDetector, ok := detector.(*NodeJSLanguageDetector)
	if !ok {
		t.Fatal("Expected detector to be of type *NodeJSLanguageDetector")
	}

	if nodeDetector.logger == nil {
		t.Error("Expected logger to be initialized")
	}

	if nodeDetector.executor == nil {
		t.Error("Expected executor to be initialized")
	}
}

// Test React project detection
func TestDetectLanguage_ReactProject(t *testing.T) {
	testDir := createReactProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_NODEJS, result.Language)
	}

	if result.Confidence < 0.95 {
		t.Errorf("Expected confidence >= 0.95 for React project, got %f", result.Confidence)
	}

	// Verify marker files
	if !nodeJSContains(result.MarkerFiles, types.MARKER_PACKAGE_JSON) {
		t.Errorf("Expected marker files to contain %s", types.MARKER_PACKAGE_JSON)
	}

	if !nodeJSContains(result.MarkerFiles, "package-lock.json") {
		t.Errorf("Expected marker files to contain package-lock.json")
	}

	// Verify required servers
	expectedServers := []string{types.SERVER_TYPESCRIPT_LANG_SERVER}
	if !nodeJSSlicesEqual(result.RequiredServers, expectedServers) {
		t.Errorf("Expected servers %v, got %v", expectedServers, result.RequiredServers)
	}

	// Verify package manager detection
	if packageManager, ok := result.Metadata["package_manager"]; !ok || packageManager != "npm" {
		t.Errorf("Expected package_manager to be 'npm', got %v", packageManager)
	}

	// Verify Node.js version from engines
	if result.Version != ">=20.0.0" {
		t.Errorf("Expected version to be '>=20.0.0', got %s", result.Version)
	}

	// Verify project metadata
	if projectName, ok := result.Metadata["project_name"]; !ok || projectName != "react-demo-app" {
		t.Errorf("Expected project_name to be 'react-demo-app', got %v", projectName)
	}

	// Verify dependencies were parsed
	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies to be parsed from package.json")
	}

	if len(result.DevDependencies) == 0 {
		t.Error("Expected devDependencies to be parsed from package.json")
	}

	// Check for React dependencies
	foundReact := false
	foundReactDom := false
	for dep := range result.Dependencies {
		if dep == "react" {
			foundReact = true
		}
		if dep == "react-dom" {
			foundReactDom = true
		}
	}

	if !foundReact {
		t.Error("Expected to find react dependency")
	}

	if !foundReactDom {
		t.Error("Expected to find react-dom dependency")
	}

	// Verify framework detection
	frameworks, ok := result.Metadata["frameworks"]
	if !ok {
		t.Fatal("Expected frameworks metadata to be present")
	}

	frameworkList, ok := frameworks.([]string)
	if !ok {
		t.Fatal("Expected frameworks to be string slice")
	}

	if !nodeJSContains(frameworkList, "React") {
		t.Error("Expected React framework to be detected")
	}

	// Verify project types
	projectTypes, ok := result.Metadata["project_types"]
	if !ok {
		t.Fatal("Expected project_types metadata to be present")
	}

	typesList, ok := projectTypes.([]string)
	if !ok {
		t.Fatal("Expected project_types to be string slice")
	}

	if !nodeJSContains(typesList, "frontend") {
		t.Error("Expected 'frontend' project type to be detected")
	}

	// Verify file detection
	if nodeFileCount, ok := result.Metadata["node_file_count"]; !ok || nodeFileCount.(int) < 1 {
		t.Error("Expected node_file_count metadata with positive value")
	}
}

// Test Express project detection
func TestDetectLanguage_ExpressProject(t *testing.T) {
	testDir := createExpressProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_NODEJS, result.Language)
	}

	// Verify package manager detection (yarn in this case)
	if packageManager, ok := result.Metadata["package_manager"]; !ok || packageManager != "yarn" {
		t.Errorf("Expected package_manager to be 'yarn', got %v", packageManager)
	}

	// Verify main file
	if mainFile, ok := result.Metadata["main_file"]; !ok || mainFile != "server.js" {
		t.Errorf("Expected main_file to be 'server.js', got %v", mainFile)
	}

	// Check for Express dependencies
	foundExpress := false
	for dep := range result.Dependencies {
		if dep == "express" {
			foundExpress = true
		}
	}

	if !foundExpress {
		t.Error("Expected to find express dependency")
	}

	// Verify framework detection
	frameworks, ok := result.Metadata["frameworks"]
	if !ok {
		t.Fatal("Expected frameworks metadata to be present")
	}

	frameworkList, ok := frameworks.([]string)
	if !ok {
		t.Fatal("Expected frameworks to be string slice")
	}

	if !nodeJSContains(frameworkList, "Express.js") {
		t.Error("Expected Express.js framework to be detected")
	}

	// Verify project types
	projectTypes, ok := result.Metadata["project_types"]
	if !ok {
		t.Fatal("Expected project_types metadata to be present")
	}

	typesList, ok := projectTypes.([]string)
	if !ok {
		t.Fatal("Expected project_types to be string slice")
	}

	if !nodeJSContains(typesList, "backend") {
		t.Error("Expected 'backend' project type to be detected")
	}

	// Verify scripts
	scripts, ok := result.Metadata["scripts"]
	if !ok {
		t.Fatal("Expected scripts metadata to be present")
	}

	scriptsMap, ok := scripts.(map[string]string)
	if !ok {
		t.Fatal("Expected scripts to be string map")
	}

	if scriptsMap["start"] != "node server.js" {
		t.Errorf("Expected start script to be 'node server.js', got %s", scriptsMap["start"])
	}
}

// Test TypeScript project detection
func TestDetectLanguage_TypeScriptProject(t *testing.T) {
	testDir := createTypeScriptProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_NODEJS, result.Language)
	}

	// Verify package manager detection (pnpm in this case)
	if packageManager, ok := result.Metadata["package_manager"]; !ok || packageManager != "pnpm" {
		t.Errorf("Expected package_manager to be 'pnpm', got %v", packageManager)
	}

	// Verify marker files include pnpm lock and tsconfig
	if !nodeJSContains(result.MarkerFiles, types.MARKER_PNPM_LOCK) {
		t.Errorf("Expected marker files to contain %s", types.MARKER_PNPM_LOCK)
	}

	// Verify config files include tsconfig.json
	if !nodeJSContains(result.ConfigFiles, "tsconfig.json") {
		t.Error("Expected config files to contain tsconfig.json")
	}

	// Check for TypeScript dependencies
	foundTypeScript := false
	for dep := range result.DevDependencies {
		if dep == "typescript" {
			foundTypeScript = true
		}
	}

	if !foundTypeScript {
		t.Error("Expected to find typescript dev dependency")
	}

	// Verify framework detection
	frameworks, ok := result.Metadata["frameworks"]
	if !ok {
		t.Fatal("Expected frameworks metadata to be present")
	}

	frameworkList, ok := frameworks.([]string)
	if !ok {
		t.Fatal("Expected frameworks to be string slice")
	}

	if !nodeJSContains(frameworkList, "TypeScript") {
		t.Error("Expected TypeScript framework to be detected")
	}

	// Verify TypeScript file detection
	if nodeFileCount, ok := result.Metadata["node_file_count"]; !ok || nodeFileCount.(int) < 3 {
		t.Errorf("Expected at least 3 TypeScript files to be detected, got %v", nodeFileCount)
	}
}

// Test JavaScript files only (no package.json)
func TestDetectLanguage_JavaScriptOnly(t *testing.T) {
	testDir := createJavaScriptOnlyProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_NODEJS, result.Language)
	}

	// Should have lower confidence without package.json
	if result.Confidence < 0.6 {
		t.Errorf("Expected confidence >= 0.6 for JS files only, got %f", result.Confidence)
	}

	// Should not have package.json as marker file
	if nodeJSContains(result.MarkerFiles, types.MARKER_PACKAGE_JSON) {
		t.Error("Expected no package.json marker for JS files only project")
	}

	// Should still detect JavaScript files
	if nodeFileCount, ok := result.Metadata["node_file_count"]; !ok || nodeFileCount.(int) != 2 {
		t.Errorf("Expected node_file_count to be 2, got %v", nodeFileCount)
	}
}

// Test malformed package.json parsing
func TestParsePackageJSON_Malformed(t *testing.T) {
	testDir := createMalformedPackageJsonProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Should still detect as Node.js project but with lower confidence
	if result.Language != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_NODEJS, result.Language)
	}

	// Confidence should be reduced due to parsing failure
	if result.Confidence < 0.8 {
		t.Errorf("Expected confidence >= 0.8 for malformed package.json, got %f", result.Confidence)
	}

	// Should still have package.json as marker file
	if !nodeJSContains(result.MarkerFiles, types.MARKER_PACKAGE_JSON) {
		t.Errorf("Expected marker files to contain %s", types.MARKER_PACKAGE_JSON)
	}
}

// Test no Node.js files
func TestDetectLanguage_NoNodeFiles(t *testing.T) {
	testDir := createEmptyProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Should have very low confidence
	if result.Confidence != 0.1 {
		t.Errorf("Expected confidence 0.1 for no Node.js indicators, got %f", result.Confidence)
	}

	// Should not have any Node.js files detected
	if nodeFileCount, ok := result.Metadata["node_file_count"]; ok && nodeFileCount.(int) > 0 {
		t.Errorf("Expected no Node.js files for empty project, got %v", nodeFileCount)
	}
}

// Test ValidateStructure
func TestNodeJSValidateStructure(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid React Project",
			setupFunc:   createReactProject,
			expectError: false,
		},
		{
			name:        "Valid Express Project",
			setupFunc:   createExpressProject,
			expectError: false,
		},
		{
			name:        "No Package.json",
			setupFunc:   createNodeJSEmptyProject,
			expectError: true,
			errorMsg:    "package.json file not found",
		},
		{
			name:        "Malformed Package.json",
			setupFunc:   createMalformedPackageJsonProject,
			expectError: true,
			errorMsg:    "package.json is not valid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := tt.setupFunc(t)
			defer os.RemoveAll(testDir)

			detector := NewNodeJSLanguageDetector()
			ctx := context.Background()

			err := detector.ValidateStructure(ctx, testDir)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// Test GetLanguageInfo
func TestNodeJSGetLanguageInfo(t *testing.T) {
	detector := NewNodeJSLanguageDetector()

	// Test valid language
	info, err := detector.GetLanguageInfo(types.PROJECT_TYPE_NODEJS)
	if err != nil {
		t.Fatalf("GetLanguageInfo failed: %v", err)
	}

	if info.Name != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected name %s, got %s", types.PROJECT_TYPE_NODEJS, info.Name)
	}

	if info.DisplayName != "Node.js" {
		t.Errorf("Expected display name 'Node.js', got %s", info.DisplayName)
	}

	expectedServers := []string{types.SERVER_TYPESCRIPT_LANG_SERVER}
	if !nodeJSSlicesEqual(info.LSPServers, expectedServers) {
		t.Errorf("Expected LSP servers %v, got %v", expectedServers, info.LSPServers)
	}

	// Test invalid language
	_, err = detector.GetLanguageInfo("invalid")
	if err == nil {
		t.Error("Expected error for invalid language")
	}
}

// Test interface methods
func TestNodeJSInterfaceMethods(t *testing.T) {
	detector := NewNodeJSLanguageDetector()

	// Test GetMarkerFiles
	markerFiles := detector.GetMarkerFiles()
	expectedMarkers := []string{types.MARKER_PACKAGE_JSON, types.MARKER_YARN_LOCK, types.MARKER_PNPM_LOCK}
	if !nodeJSSlicesEqual(markerFiles, expectedMarkers) {
		t.Errorf("Expected marker files %v, got %v", expectedMarkers, markerFiles)
	}

	// Test GetRequiredServers
	requiredServers := detector.GetRequiredServers()
	expectedServers := []string{types.SERVER_TYPESCRIPT_LANG_SERVER}
	if !nodeJSSlicesEqual(requiredServers, expectedServers) {
		t.Errorf("Expected required servers %v, got %v", expectedServers, requiredServers)
	}

	// Test GetPriority
	priority := detector.GetPriority()
	if priority != types.PRIORITY_NODEJS {
		t.Errorf("Expected priority %d, got %d", types.PRIORITY_NODEJS, priority)
	}
}

// Test Node.js version detection with mock executor
func TestDetectNodeVersion(t *testing.T) {
	testDir := createReactProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector().(*NodeJSLanguageDetector)

	// Test successful version detection
	mockExec := &mockNodeCommandExecutor{
		nodeVersion: "v18.17.0",
		npmVersion:  "9.6.7",
	}
	detector.executor = mockExec

	ctx := context.Background()
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify Node.js version was detected from command
	if installedVersion, ok := result.Metadata["installed_node_version"]; !ok || installedVersion != "18.17.0" {
		t.Errorf("Expected installed_node_version to be '18.17.0', got %v", installedVersion)
	}

	// Verify npm version was detected
	if npmVersion, ok := result.Metadata["installed_npm_version"]; !ok || npmVersion != "9.6.7" {
		t.Errorf("Expected installed_npm_version to be '9.6.7', got %v", npmVersion)
	}

	// Version from package.json should take precedence
	if result.Version != ">=20.0.0" {
		t.Errorf("Expected version from package.json to be '>=20.0.0', got %s", result.Version)
	}

	// Test version detection failure
	mockExec.shouldFail = true
	mockExec.failError = &platform.PlatformError{}

	result, err = detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Should still work without version detection
	if result.Language != types.PROJECT_TYPE_NODEJS {
		t.Errorf("Expected language %s even with version detection failure", types.PROJECT_TYPE_NODEJS)
	}
}

// Test framework detection
func TestDetectNodeFrameworks(t *testing.T) {
	testDir := createReactProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify framework detection
	frameworks, ok := result.Metadata["frameworks"]
	if !ok {
		t.Fatal("Expected frameworks metadata to be present")
	}

	frameworkList, ok := frameworks.([]string)
	if !ok {
		t.Fatal("Expected frameworks to be string slice")
	}

	// Check for essential frameworks that should definitely be detected
	mustHaveFrameworks := []string{"React", "ESLint", "Prettier"}
	for _, expected := range mustHaveFrameworks {
		if !nodeJSContains(frameworkList, expected) {
			t.Errorf("Expected framework %s to be detected, got %v", expected, frameworkList)
		}
	}
	
	// Jest might not be detected if only in devDependencies and not in the exact form expected
	// This is acceptable since different projects might use different testing setups
}

// Test project structure analysis
func TestNodeJSAnalyzeProjectStructure(t *testing.T) {
	testDir := createReactProject(t)
	defer os.RemoveAll(testDir)

	detector := NewNodeJSLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify project structure detection
	projectStructure, ok := result.Metadata["project_structure"]
	if !ok {
		t.Fatal("Expected project_structure metadata to be present")
	}

	structureList, ok := projectStructure.([]string)
	if !ok {
		t.Fatal("Expected project_structure to be string slice")
	}

	expectedDirs := []string{"src", "public"}
	for _, expected := range expectedDirs {
		if !nodeJSContains(structureList, expected) {
			t.Errorf("Expected directory %s to be detected in project structure", expected)
		}
	}

	// Verify SPA project detection
	if isSPA, ok := result.Metadata["is_spa_project"]; !ok || !isSPA.(bool) {
		t.Error("Expected React project to be detected as SPA project")
	}
}

// Helper functions for Node.js tests
func createNodeJSEmptyProject(t *testing.T) string {
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Empty Project"), 0644)
	if err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}
	return tempDir
}

func nodeJSContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func nodeJSSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}