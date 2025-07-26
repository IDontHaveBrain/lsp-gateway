package config

import (
	"os"
	"path/filepath"
	"testing"

	"lsp-gateway/internal/config"
	"lsp-gateway/tests/integration/config/helpers"

	"github.com/stretchr/testify/suite"
)

// ConfigurationLoadingTestSuite provides comprehensive integration tests for configuration loading
type ConfigurationLoadingTestSuite struct {
	suite.Suite
	testHelper   *helpers.ConfigTestHelper
	tempDir      string
	cleanup      []func()
	testProjects map[string]string
}

// SetupTest initializes the test environment
func (suite *ConfigurationLoadingTestSuite) SetupTest() {
	tempDir, err := os.MkdirTemp("", "config-loading-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir

	suite.testHelper = helpers.NewConfigTestHelper(tempDir)
	suite.cleanup = []func(){}
	suite.testProjects = make(map[string]string)

	// Register cleanup for temp directory
	suite.cleanup = append(suite.cleanup, func() {
		_ = os.RemoveAll(tempDir)
	})
}

// TearDownTest cleans up test resources
func (suite *ConfigurationLoadingTestSuite) TearDownTest() {
	for _, cleanupFunc := range suite.cleanup {
		cleanupFunc()
	}
}

// TestMultiLanguageConfigLoading tests loading of multi-language configurations
func (suite *ConfigurationLoadingTestSuite) TestMultiLanguageConfigLoading() {
	// Create comprehensive multi-language project
	projectStructure := map[string]string{
		// Go services
		"services/auth/go.mod":      "module auth-service\n\ngo 1.21\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.1\n\tgorm.io/gorm v1.25.5\n)",
		"services/auth/main.go":     "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() {\n\tr := gin.Default()\n\tr.Run(\":8080\")\n}",
		"services/auth/handlers.go": "package main\n\ntype AuthHandler struct{}\n\nfunc (h *AuthHandler) Login() {}",
		"services/auth/models.go":   "package main\n\ntype User struct {\n\tID   uint   `json:\"id\"`\n\tName string `json:\"name\"`\n}",

		// Python services
		"services/ml/pyproject.toml": `[build-system]
requires = ["setuptools>=65.0", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "ml-service"
version = "1.0.0"
dependencies = [
    "fastapi>=0.100.0",
    "uvicorn>=0.23.0",
    "pandas>=2.0.0",
    "scikit-learn>=1.3.0"
]`,
		"services/ml/src/main.py":          "from fastapi import FastAPI\n\napp = FastAPI()\n\n@app.get('/')\ndef read_root():\n    return {'service': 'ml-service'}",
		"services/ml/src/models.py":        "import pandas as pd\nfrom sklearn.base import BaseEstimator\n\nclass MLModel(BaseEstimator):\n    def fit(self, X, y): pass\n    def predict(self, X): pass",
		"services/ml/src/preprocessing.py": "import pandas as pd\n\ndef preprocess_data(df: pd.DataFrame) -> pd.DataFrame:\n    return df.dropna()",

		// Java services
		"services/notification/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>notification-service</artifactId>
    <version>1.0.0-SNAPSHOT</version>
    <packaging>jar</packaging>
    
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
            <version>3.1.0</version>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-data-jpa</artifactId>
            <version>3.1.0</version>
        </dependency>
    </dependencies>
</project>`,
		"services/notification/src/main/java/com/example/NotificationApplication.java": `package com.example;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class NotificationApplication {
    public static void main(String[] args) {
        SpringApplication.run(NotificationApplication.class, args);
    }
}`,
		"services/notification/src/main/java/com/example/controller/NotificationController.java": `package com.example.controller;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class NotificationController {
    @GetMapping("/health")
    public String health() {
        return "OK";
    }
}`,

		// TypeScript frontend
		"frontend/web/package.json": `{
  "name": "web-frontend",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "@types/react": "^18.2.0",
    "typescript": "^5.0.0",
    "vite": "^4.4.0",
    "@vitejs/plugin-react": "^4.0.0"
  },
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  }
}`,
		"frontend/web/tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}`,
		"frontend/web/src/App.tsx": `import React from 'react';
import { useState, useEffect } from 'react';

interface User {
  id: number;
  name: string;
}

const App: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);

  useEffect(() => {
    fetchUsers();
  }, []);

  const fetchUsers = async () => {
    try {
      const response = await fetch('/api/users');
      const data = await response.json();
      setUsers(data);
    } catch (error) {
      console.error('Failed to fetch users:', error);
    }
  };

  return (
    <div className="app">
      <h1>Multi-Language Application</h1>
      <div>
        {users.map(user => (
          <div key={user.id}>{user.name}</div>
        ))}
      </div>
    </div>
  );
};

export default App;`,

		// Configuration files
		"docker-compose.yml": `version: '3.8'
services:
  auth:
    build: ./services/auth
    ports:
      - "8081:8080"
    environment:
      - DATABASE_URL=postgres://user:pass@db:5432/auth
  
  ml:
    build: ./services/ml
    ports:
      - "8082:8000"
    environment:
      - MODEL_PATH=/app/models
  
  notification:
    build: ./services/notification
    ports:
      - "8083:8080"
    environment:
      - SPRING_PROFILES_ACTIVE=production
  
  web:
    build: ./frontend/web
    ports:
      - "3000:3000"
    environment:
      - VITE_API_URL=http://localhost:8080

  db:
    image: postgres:15
    environment:
      - POSTGRES_DB=myapp
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass`,

		"Makefile": `# Multi-language project Makefile
.PHONY: build test clean docker-up docker-down

build: build-go build-python build-java build-frontend

build-go:
	@echo "Building Go services..."
	cd services/auth && go build -o ../../bin/auth ./...

build-python:
	@echo "Building Python services..."
	cd services/ml && pip install -e .

build-java:
	@echo "Building Java services..."
	cd services/notification && mvn clean package

build-frontend:
	@echo "Building TypeScript frontend..."
	cd frontend/web && npm install && npm run build

test: test-go test-python test-java test-frontend

test-go:
	cd services/auth && go test ./...

test-python:
	cd services/ml && python -m pytest

test-java:
	cd services/notification && mvn test

test-frontend:
	cd frontend/web && npm test

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

clean:
	rm -rf bin/
	cd services/auth && go clean
	cd services/ml && rm -rf build/ dist/
	cd services/notification && mvn clean
	cd frontend/web && rm -rf dist/ node_modules/`,

		"README.md": `# Multi-Language Microservices Application

This is a comprehensive multi-language application demonstrating integration between:

- **Go**: Authentication service using Gin and GORM
- **Python**: Machine learning service using FastAPI and scikit-learn  
- **Java**: Notification service using Spring Boot
- **TypeScript**: Web frontend using React and Vite

## Architecture

The application follows a microservices architecture with each service built in its optimal language.

## Development

See the Makefile for build and test commands.`,
	}

	projectPath := suite.testHelper.CreateTestProject("comprehensive-multi-lang", projectStructure)
	suite.testProjects["comprehensive-multi-lang"] = projectPath

	// Test configuration loading scenarios
	suite.Run("LoadBasicMultiLanguageConfig", func() {
		config, err := suite.testHelper.LoadConfigForProject(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(config)

		// Validate multi-language detection
		suite.GreaterOrEqual(len(config.ProjectInfo.LanguageContexts), 4, "Should detect at least 4 languages (Go, Python, Java, TypeScript)")
		suite.Equal(config.ProjectInfo.ProjectType, "monorepo", "Should detect as monorepo project type")

		// Validate specific languages
		languageNames := make([]string, len(config.ProjectInfo.LanguageContexts))
		for i, lang := range config.ProjectInfo.LanguageContexts {
			languageNames[i] = lang.Language
		}

		suite.Contains(languageNames, "go", "Should detect Go language")
		suite.Contains(languageNames, "python", "Should detect Python language")
		suite.Contains(languageNames, "java", "Should detect Java language")
		suite.Contains(languageNames, "typescript", "Should detect TypeScript language")
	})

	suite.Run("LoadConfigWithPerformanceSettings", func() {
		// Create configuration with performance optimizations
		perfConfig := &config.GatewayConfig{
			Servers: []config.ServerConfig{
				{
					Name:      "go-lsp",
					Languages: []string{"go"},
					Command:   "gopls",
					Transport: "stdio",
				},
			},
			PerformanceConfig: &config.PerformanceConfiguration{
				Enabled:      true,
				Profile:      "development",
				AutoTuning:   true,
			},
		}

		configPath := filepath.Join(projectPath, "lsp-gateway-perf.yaml")
		err := suite.testHelper.SaveEnhancedConfig(perfConfig, configPath)
		suite.Require().NoError(err)

		// Load and validate performance configuration
		loadedConfig, err := suite.testHelper.LoadEnhancedConfig(configPath)
		suite.Require().NoError(err)
		suite.NotNil(loadedConfig.PerformanceConfig)

		// Validate performance settings
		suite.True(loadedConfig.PerformanceConfig.Enabled)
		suite.Equal("development", loadedConfig.PerformanceConfig.Profile)
		suite.True(loadedConfig.PerformanceConfig.AutoTuning)
	})

	suite.Run("LoadConfigWithOptimizationStrategies", func() {
		// Test configuration with different optimization strategies
		strategies := []string{"development", "production", "large-project", "memory-optimized"}

		for _, strategy := range strategies {
			config := &config.GatewayConfig{
				PerformanceConfig: &config.PerformanceConfiguration{
					Profile: strategy,
				},
			}

			configPath := filepath.Join(projectPath, "lsp-gateway-"+strategy+".yaml")
			err := suite.testHelper.SaveEnhancedConfig(config, configPath)
			suite.Require().NoError(err)

			// Load and validate optimization configuration
			loadedConfig, err := suite.testHelper.LoadEnhancedConfig(configPath)
			suite.Require().NoError(err)
			suite.NotNil(loadedConfig.PerformanceConfig)

			suite.Equal(strategy, loadedConfig.PerformanceConfig.Profile, "Strategy should match")
		}
	})

	suite.Run("LoadTemplateBasedConfig", func() {
		// Test loading configuration from templates
		templateTypes := []string{"monorepo", "microservices", "full-stack", "enterprise"}

		for _, templateType := range templateTypes {
			templateConfig, err := suite.testHelper.LoadConfigTemplate(templateType)
			suite.Require().NoError(err)
			suite.NotNil(templateConfig)

			// Validate template structure
			suite.NotEmpty(templateConfig.Name, "Template should have a name")
			suite.NotEmpty(templateConfig.Description, "Template should have a description")
			suite.NotEmpty(templateConfig.ProjectTypes, "Template should specify project types")
			suite.NotEmpty(templateConfig.Languages, "Template should specify languages")
		}
	})

	suite.Run("LoadEnvironmentSpecificConfig", func() {
		// Test environment-specific configuration loading
		environments := []string{"development", "staging", "production", "testing"}

		for _, env := range environments {
			envConfig := &config.GatewayConfig{
				PerformanceConfig: &config.PerformanceConfiguration{
					Profile:      env,
					Enabled:         env != "development",
				},
			}

			configPath := filepath.Join(projectPath, "lsp-gateway-"+env+".yaml")
			err := suite.testHelper.SaveEnhancedConfig(envConfig, configPath)
			suite.Require().NoError(err)

			// Load and validate environment configuration
			loadedConfig, err := suite.testHelper.LoadEnhancedConfig(configPath)
			suite.Require().NoError(err)

			suite.Equal(env, loadedConfig.PerformanceConfig.Profile)
		}
	})
}

// TestComplexConfigurationScenarios tests complex configuration loading scenarios
func (suite *ConfigurationLoadingTestSuite) TestComplexConfigurationScenarios() {
	suite.Run("LoadNestedWorkspaceConfig", func() {
		// Create complex nested workspace structure
		nestedStructure := map[string]string{
			// Platform core
			"platform/core/go.work": `go 1.21

use (
	./auth
	./gateway
	./shared
)`,
			"platform/core/auth/go.mod":     "module platform/core/auth\n\ngo 1.21",
			"platform/core/auth/main.go":    "package main\n\nfunc main() {}",
			"platform/core/gateway/go.mod":  "module platform/core/gateway\n\ngo 1.21",
			"platform/core/gateway/main.go": "package main\n\nfunc main() {}",
			"platform/core/shared/go.mod":   "module platform/core/shared\n\ngo 1.21",
			"platform/core/shared/utils.go": "package shared\n\nfunc Utils() {}",

			// Services in different languages
			"services/user-management/pyproject.toml": `[build-system]
requires = ["setuptools", "wheel"]

[project]
name = "user-management"
version = "1.0.0"`,
			"services/user-management/src/main.py":   "def main(): pass",
			"services/user-management/src/models.py": "class User: pass",

			"services/analytics/package.json": `{
  "name": "analytics-service",
  "version": "1.0.0",
  "dependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  }
}`,
			"services/analytics/tsconfig.json":    `{"compilerOptions": {"target": "ES2020"}}`,
			"services/analytics/src/index.ts":     "console.log('Analytics service');",
			"services/analytics/src/processor.ts": "export class DataProcessor {}",

			// Frontend applications
			"apps/dashboard/package.json": `{
  "name": "dashboard",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.0.0",
    "typescript": "^5.0.0"
  }
}`,
			"apps/dashboard/tsconfig.json": `{"compilerOptions": {"target": "ES2020", "jsx": "react-jsx"}}`,
			"apps/dashboard/src/App.tsx":   "import React from 'react';\n\nconst App = () => <div>Dashboard</div>;\n\nexport default App;",

			"apps/mobile/package.json": `{
  "name": "mobile-app", 
  "version": "1.0.0",
  "dependencies": {
    "react-native": "^0.72.0",
    "typescript": "^5.0.0"
  }
}`,
			"apps/mobile/tsconfig.json": `{"compilerOptions": {"target": "ES2020", "jsx": "react-native"}}`,
			"apps/mobile/src/App.tsx":   "import React from 'react';\n\nconst App = () => null;\n\nexport default App;",
		}

		projectPath := suite.testHelper.CreateTestProject("nested-workspace", nestedStructure)

		// Load configuration for nested workspace
		config, err := suite.testHelper.LoadConfigForProject(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(config)

		// Validate workspace detection
		suite.Equal("workspace", config.ProjectInfo.ProjectType, "Should detect as workspace project")

		// Validate multi-language detection across nested structure
		suite.GreaterOrEqual(len(config.ProjectInfo.LanguageContexts), 2, "Should detect multiple languages")

		// Validate workspace roots are correctly identified
		suite.NotEmpty(config.ProjectInfo.WorkspaceRoot, "Should identify workspace root")
	})

	suite.Run("LoadMonorepoWithSubprojects", func() {
		// Create monorepo with independent subprojects
		monorepoStructure := map[string]string{
			// Root configuration
			"package.json": `{
  "name": "monorepo",
  "version": "1.0.0",
  "workspaces": [
    "packages/*",
    "apps/*"
  ],
  "devDependencies": {
    "lerna": "^7.0.0"
  }
}`,
			"lerna.json": `{
  "version": "1.0.0",
  "npmClient": "npm",
  "packages": [
    "packages/*",
    "apps/*"
  ]
}`,

			// Shared packages
			"packages/shared-types/package.json": `{
  "name": "@monorepo/shared-types",
  "version": "1.0.0",
  "dependencies": {
    "typescript": "^5.0.0"
  }
}`,
			"packages/shared-types/tsconfig.json": `{"compilerOptions": {"target": "ES2020"}}`,
			"packages/shared-types/src/index.ts":  "export interface User { id: number; name: string; }",

			"packages/shared-utils/package.json": `{
  "name": "@monorepo/shared-utils", 
  "version": "1.0.0",
  "dependencies": {
    "typescript": "^5.0.0"
  }
}`,
			"packages/shared-utils/tsconfig.json": `{"compilerOptions": {"target": "ES2020"}}`,
			"packages/shared-utils/src/index.ts":  "export const formatDate = (date: Date) => date.toISOString();",

			// Applications
			"apps/web/package.json": `{
  "name": "web-app",
  "version": "1.0.0", 
  "dependencies": {
    "react": "^18.0.0",
    "typescript": "^5.0.0",
    "@monorepo/shared-types": "1.0.0",
    "@monorepo/shared-utils": "1.0.0"
  }
}`,
			"apps/web/tsconfig.json": `{"compilerOptions": {"target": "ES2020", "jsx": "react-jsx"}}`,
			"apps/web/src/App.tsx":   "import React from 'react';\nimport { User } from '@monorepo/shared-types';\n\nconst App = () => <div>Web App</div>;\n\nexport default App;",

			"apps/api/package.json": `{
  "name": "api-server",
  "version": "1.0.0",
  "dependencies": {
    "typescript": "^5.0.0",
    "express": "^4.18.0",
    "@monorepo/shared-types": "1.0.0"
  }
}`,
			"apps/api/tsconfig.json": `{"compilerOptions": {"target": "ES2020"}}`,
			"apps/api/src/server.ts": "import express from 'express';\nimport { User } from '@monorepo/shared-types';\n\nconst app = express();",

			// Go services (separate from Node.js monorepo)
			"services/auth/go.mod":  "module monorepo/services/auth\n\ngo 1.21",
			"services/auth/main.go": "package main\n\nfunc main() {}",

			"services/data/go.mod":  "module monorepo/services/data\n\ngo 1.21",
			"services/data/main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("complex-monorepo", monorepoStructure)

		// Load configuration
		config, err := suite.testHelper.LoadConfigForProject(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(config)

		// Validate monorepo detection
		suite.Equal("monorepo", config.ProjectInfo.ProjectType, "Should detect as monorepo")

		// Validate language detection across monorepo
		languageNames := make([]string, len(config.ProjectInfo.LanguageContexts))
		for i, lang := range config.ProjectInfo.LanguageContexts {
			languageNames[i] = lang.Language
		}

		suite.Contains(languageNames, "typescript", "Should detect TypeScript")
		suite.Contains(languageNames, "go", "Should detect Go")

		// Validate TypeScript workspace detection
		for _, lang := range config.ProjectInfo.LanguageContexts {
			if lang.Language == "typescript" {
				suite.Greater(lang.FileCount, 5, "TypeScript should have multiple files across packages and apps")
			}
		}
	})
}

// Run the test suite
func TestConfigurationLoadingTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationLoadingTestSuite))
}
