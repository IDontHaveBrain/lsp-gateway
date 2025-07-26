package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"lsp-gateway/internal/gateway"
)

func main() {
	logger := log.New(os.Stdout, "[BASIC-SCANNER-TEST] ", log.LstdFlags)

	if err := runBasicScannerTest(logger); err != nil {
		logger.Printf("Test failed: %v", err)
		os.Exit(1)
	}

	logger.Printf("Basic scanner test completed successfully!")
}

func runBasicScannerTest(logger *log.Logger) error {
	logger.Printf("Starting basic ProjectLanguageScanner test...")

	// Create temporary test project
	tempDir, err := createBasicTestProject()
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.Printf("Warning: failed to clean up temp directory %s: %v", tempDir, err)
		}
	}()

	logger.Printf("Created test project at: %s", tempDir)

	// Test basic scanner functionality
	scanner := gateway.NewProjectLanguageScanner()

	// Configure scanner
	scanner.SetMaxDepth(3)
	scanner.SetMaxFiles(1000)
	scanner.SetCacheEnabled(true)

	// Validate configuration
	if err := scanner.ValidateConfiguration(); err != nil {
		return fmt.Errorf("scanner validation failed: %w", err)
	}

	logger.Printf("Scanner configuration validated")

	// Get supported languages
	languages := scanner.GetSupportedLanguages()
	logger.Printf("Supported languages (%d): %v", len(languages), languages)

	// Scan the test project
	projectInfo, err := scanner.ScanProject(tempDir)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	// Print detailed results
	logger.Printf("\n=== PROJECT SCAN RESULTS ===")
	logger.Printf("Root Path: %s", projectInfo.RootPath)
	logger.Printf("Project Type: %s", projectInfo.ProjectType)
	logger.Printf("Dominant Language: %s", projectInfo.DominantLanguage)
	logger.Printf("Total Files: %d", projectInfo.TotalFileCount)
	logger.Printf("Scan Duration: %v", projectInfo.ScanDuration)
	logger.Printf("Scan Depth: %d", projectInfo.ScanDepth)
	logger.Printf("Languages Found: %d", len(projectInfo.Languages))

	logger.Printf("\n=== LANGUAGE DETAILS ===")
	for lang, ctx := range projectInfo.Languages {
		logger.Printf("Language: %s", lang)
		logger.Printf("  File Count: %d", ctx.FileCount)
		logger.Printf("  Test Files: %d", ctx.TestFileCount)
		logger.Printf("  Priority: %d", ctx.Priority)
		logger.Printf("  Confidence: %.2f", ctx.Confidence)
		logger.Printf("  Root Path: %s", ctx.RootPath)

		if ctx.Framework != "" {
			logger.Printf("  Framework: %s", ctx.Framework)
		}
		if ctx.Version != "" {
			logger.Printf("  Version: %s", ctx.Version)
		}
		if ctx.LSPServerName != "" {
			logger.Printf("  LSP Server: %s", ctx.LSPServerName)
		}

		logger.Printf("  Build Files: %v", ctx.BuildFiles)
		logger.Printf("  Config Files: %v", ctx.ConfigFiles)
		logger.Printf("  Source Paths: %v", ctx.SourcePaths)
		logger.Printf("  Test Paths: %v", ctx.TestPaths)
		logger.Printf("  File Extensions: %v", ctx.FileExtensions)
		logger.Printf("")
	}

	// Test project info utility methods
	logger.Printf("=== PROJECT INFO UTILITIES ===")

	primaryLangs := projectInfo.GetPrimaryLanguages()
	secondaryLangs := projectInfo.GetSecondaryLanguages()

	logger.Printf("Primary Languages: %v", primaryLangs)
	logger.Printf("Secondary Languages: %v", secondaryLangs)
	logger.Printf("Is Polyglot: %t", projectInfo.IsPolyglot())

	// Test language-specific methods
	for lang := range projectInfo.Languages {
		hasLang := projectInfo.HasLanguage(lang)
		workspaceRoot := projectInfo.GetWorkspaceRoot(lang)
		langCtx := projectInfo.GetLanguageContext(lang)

		logger.Printf("Language %s:", lang)
		logger.Printf("  Has Language: %t", hasLang)
		logger.Printf("  Workspace Root: %s", workspaceRoot)
		logger.Printf("  Context Valid: %t", langCtx != nil)
	}

	// Test validation
	if err := projectInfo.Validate(); err != nil {
		return fmt.Errorf("project info validation failed: %w", err)
	}

	logger.Printf("Project info validation passed")

	// Test performance metrics
	perfMetrics := scanner.GetPerformanceMetrics()
	if perfMetrics != nil {
		logger.Printf("\n=== PERFORMANCE METRICS ===")
		if config, ok := perfMetrics["configuration"].(map[string]interface{}); ok {
			for key, value := range config {
				logger.Printf("  %s: %v", key, value)
			}
		}
	}

	// Test cache stats
	cacheStats := scanner.GetCacheStats()
	if cacheStats != nil {
		logger.Printf("\n=== CACHE STATISTICS ===")
		logger.Printf("  Hit Ratio: %.2f", cacheStats.HitRatio)
		logger.Printf("  Total Entries: %d", cacheStats.TotalEntries)
		logger.Printf("  Hit Count: %d", cacheStats.HitCount)
		logger.Printf("  Miss Count: %d", cacheStats.MissCount)
	}

	logger.Printf("\n=== ALL TESTS COMPLETED SUCCESSFULLY ===")
	return nil
}

func createBasicTestProject() (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "basic-lang-test-*")
	if err != nil {
		return "", err
	}

	// Create test files for different languages
	files := map[string]string{
		// Go files
		"main.go": `package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello from Go!")
}`,
		"go.mod": `module multi-lang-test

go 1.21

require (
	github.com/gorilla/mux v1.8.0
)`,
		"go.sum": `github.com/gorilla/mux v1.8.0 h1:i40aqfkR1h2SlN9hojwV5ZA91wcXFOvdyBDW8k6dVfg=
github.com/gorilla/mux v1.8.0/go.mod h1:DVbg23sWSpFRCP0SfiEN6jmj59UnW/n46BH5rLB71So=`,

		// Python files
		"api/app.py": `#!/usr/bin/env python3
"""
Multi-language test Python API
"""

from flask import Flask, jsonify
import os

app = Flask(__name__)

@app.route("/")
def hello():
    return jsonify({"message": "Hello from Python!"})

@app.route("/health")
def health():
    return jsonify({"status": "healthy"})

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 5000))
    app.run(host="0.0.0.0", port=port, debug=True)`,

		"requirements.txt": `Flask==2.3.2
requests==2.31.0
pytest==7.4.0`,

		"setup.py": `from setuptools import setup, find_packages

setup(
    name="multi-lang-test-api",
    version="1.0.0",
    packages=find_packages(),
    install_requires=[
        "Flask>=2.0.0",
        "requests>=2.25.0",
    ],
    python_requires=">=3.8",
)`,

		"pyproject.toml": `[build-system]
requires = ["setuptools>=61.0", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "multi-lang-test-api"
version = "1.0.0"
description = "Multi-language test Python API"
dependencies = [
    "Flask>=2.0.0",
    "requests>=2.25.0",
]`,

		// TypeScript/JavaScript files
		"frontend/src/app.ts": `interface Config {
    apiUrl: string;
    version: string;
}

interface User {
    id: number;
    name: string;
    email: string;
}

class AppService {
    private config: Config;
    
    constructor(config: Config) {
        this.config = config;
    }
    
    async fetchUsers(): Promise<User[]> {
        const response = await fetch(` + "`${this.config.apiUrl}/users`" + `);
        return response.json();
    }
    
    greet(): string {
        return "Hello from TypeScript!";
    }
}

export { AppService, Config, User };`,

		"frontend/package.json": `{
    "name": "multi-lang-test-frontend",
    "version": "1.0.0",
    "description": "Multi-language test frontend",
    "main": "dist/app.js",
    "scripts": {
        "build": "tsc",
        "dev": "tsc --watch",
        "test": "jest"
    },
    "dependencies": {
        "typescript": "^5.0.0",
        "@types/node": "^20.0.0"
    },
    "devDependencies": {
        "jest": "^29.0.0",
        "@types/jest": "^29.0.0"
    }
}`,

		"frontend/tsconfig.json": `{
    "compilerOptions": {
        "target": "ES2020",
        "module": "commonjs",
        "lib": ["ES2020", "DOM"],
        "outDir": "./dist",
        "rootDir": "./src",
        "strict": true,
        "esModuleInterop": true,
        "skipLibCheck": true,
        "forceConsistentCasingInFileNames": true,
        "declaration": true,
        "declarationMap": true,
        "sourceMap": true
    },
    "include": ["src/**/*"],
    "exclude": ["node_modules", "dist"]
}`,

		// Java files
		"backend/src/main/java/Main.java": `import java.util.*;
import java.io.*;

public class Main {
    private static final String VERSION = "1.0.0";
    
    public static void main(String[] args) {
        System.out.println("Hello from Java!");
        
        AppService service = new AppService();
        service.start();
    }
}

class AppService {
    private Map<String, String> config;
    
    public AppService() {
        this.config = new HashMap<>();
        this.config.put("version", "1.0.0");
        this.config.put("environment", "development");
    }
    
    public void start() {
        System.out.println("Java service started");
        System.out.println("Version: " + config.get("version"));
    }
    
    public String getGreeting() {
        return "Hello from Java AppService!";
    }
}`,

		"backend/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    
    <groupId>com.example</groupId>
    <artifactId>multi-lang-test-backend</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>
    
    <name>Multi-Language Test Backend</name>
    <description>Java backend for multi-language test</description>
    
    <properties>
        <maven.compiler.source>17</maven.compiler.source>
        <maven.compiler.target>17</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>
    
    <dependencies>
        <dependency>
            <groupId>junit</groupId>
            <artifactId>junit</artifactId>
            <version>4.13.2</version>
            <scope>test</scope>
        </dependency>
    </dependencies>
</project>`,

		// Rust files (additional language)
		"tools/src/main.rs": `use std::collections::HashMap;
use std::env;

fn main() {
    println!("Hello from Rust!");
    
    let service = AppService::new();
    service.run();
}

struct AppService {
    config: HashMap<String, String>,
}

impl AppService {
    fn new() -> Self {
        let mut config = HashMap::new();
        config.insert("name".to_string(), "multi-lang-test".to_string());
        config.insert("version".to_string(), "1.0.0".to_string());
        
        AppService { config }
    }
    
    fn run(&self) {
        println!("Rust service running");
        println!("Config: {:?}", self.config);
    }
    
    fn greet(&self) -> String {
        "Hello from Rust AppService!".to_string()
    }
}`,

		"tools/Cargo.toml": `[package]
name = "multi-lang-test-tools"
version = "1.0.0"
edition = "2021"
description = "Rust tools for multi-language test"

[dependencies]
serde = { version = "1.0", features = ["derive"] }
tokio = { version = "1.0", features = ["full"] }`,

		// Configuration and documentation files
		"README.md": `# Multi-Language Test Project

This is a comprehensive test project demonstrating multi-language LSP Gateway functionality.

## Languages Included

### Go (Backend Service)
- **Files**: main.go, go.mod, go.sum
- **Purpose**: HTTP server and main application logic
- **LSP Server**: gopls

### Python (API Service)
- **Files**: api/app.py, requirements.txt, setup.py, pyproject.toml
- **Purpose**: Flask-based REST API
- **LSP Server**: python-lsp-server (pylsp)

### TypeScript/JavaScript (Frontend)
- **Files**: frontend/src/app.ts, frontend/package.json, frontend/tsconfig.json
- **Purpose**: Frontend application service
- **LSP Server**: typescript-language-server

### Java (Backend Service)
- **Files**: backend/src/main/java/Main.java, backend/pom.xml
- **Purpose**: Enterprise backend service
- **LSP Server**: Eclipse JDT Language Server (jdtls)

### Rust (Tools)
- **Files**: tools/src/main.rs, tools/Cargo.toml
- **Purpose**: System tools and utilities
- **LSP Server**: rust-analyzer

## Project Structure

` + "```" + `
multi-lang-test/
├── main.go              # Go HTTP server
├── go.mod               # Go module definition
├── go.sum               # Go dependencies
├── api/
│   └── app.py           # Python Flask API
├── requirements.txt     # Python dependencies
├── setup.py             # Python package setup
├── pyproject.toml       # Python project config
├── frontend/
│   ├── src/
│   │   └── app.ts       # TypeScript application
│   ├── package.json     # Node.js dependencies
│   └── tsconfig.json    # TypeScript config
├── backend/
│   ├── src/main/java/
│   │   └── Main.java    # Java application
│   └── pom.xml          # Maven configuration
├── tools/
│   ├── src/
│   │   └── main.rs      # Rust tools
│   └── Cargo.toml       # Rust package config
└── README.md            # This file
` + "```" + `

## Testing

This project is designed to test:
- Multi-language project detection
- Language-specific root discovery
- Framework detection
- LSP server recommendations  
- Cross-language symbol search
- Intelligent request routing

## Expected Detection Results

The LSP Gateway should detect this as a **polyglot monorepo** with:
- **Primary Languages**: Go, Python, TypeScript, Java, Rust
- **Project Type**: monorepo (multiple languages with clear separation)
- **Frameworks**: Flask (Python), TypeScript compiler (TypeScript)
- **Build Systems**: Go modules, Python setuptools, npm/TypeScript, Maven, Cargo`,

		"docker-compose.yml": `version: '3.8'

services:
  go-backend:
    build:
      context: .
      dockerfile: Dockerfile.go
    ports:
      - "8080:8080"
    environment:
      - ENV=production
  
  python-api:
    build:
      context: ./api
      dockerfile: Dockerfile
    ports:
      - "5000:5000"
    environment:
      - FLASK_ENV=production
  
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
    environment:
      - NODE_ENV=production`,

		".gitignore": `# Compiled binaries
/bin/
*.exe
*.exe~
*.dll
*.so
*.dylib

# Go
*.test
*.out
go.work

# Python
__pycache__/
*.py[cod]
*\$py.class
*.so
.Python
build/
develop-eggs/
dist/
downloads/
eggs/
.eggs/
lib/
lib64/
parts/
sdist/
var/
wheels/
*.egg-info/
.installed.cfg
*.egg

# Node.js/TypeScript
node_modules/
npm-debug.log*
yarn-debug.log*
yarn-error.log*
.npm
.eslintcache
*.tsbuildinfo
dist/

# Java
target/
*.class
*.log
*.ctxt
.mtj.tmp/
*.jar
*.war
*.nar
*.ear
*.zip
*.tar.gz
*.rar

# Rust
target/
Cargo.lock

# IDEs
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db`,
	}

	// Create all files and directories
	for filePath, content := range files {
		fullPath := filepath.Join(tempDir, filePath)

		// Create directory if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
		}
	}

	return tempDir, nil
}
