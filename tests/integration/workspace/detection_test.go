package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/workspace"
	"lsp-gateway/tests/integration/workspace/helpers"
)

type WorkspaceDetectionSuite struct {
	suite.Suite
	tempDir   string
	detector  workspace.WorkspaceDetector
	helper    *helpers.WorkspaceTestHelper
}

func (suite *WorkspaceDetectionSuite) SetupSuite() {
	tempDir, err := os.MkdirTemp("", "workspace-detection-test-*")
	require.NoError(suite.T(), err)
	
	suite.tempDir = tempDir
	suite.detector = workspace.NewWorkspaceDetector()
	suite.helper = helpers.NewWorkspaceTestHelper(tempDir)
}

func (suite *WorkspaceDetectionSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *WorkspaceDetectionSuite) TestDetectGoWorkspace() {
	// Create Go workspace with multiple modules
	workspacePath := suite.helper.CreateGoWorkspace("go-workspace", map[string]string{
		"go.work": `go 1.24

use (
	./service-a
	./service-b
	./libs/shared
)
`,
		"service-a/go.mod": `module example.com/service-a

go 1.24
`,
		"service-a/main.go": `package main

import "fmt"

func main() {
	fmt.Println("Service A")
}
`,
		"service-b/go.mod": `module example.com/service-b

go 1.24
`,
		"service-b/main.go": `package main

import "fmt"

func main() {
	fmt.Println("Service B")
}
`,
		"libs/shared/go.mod": `module example.com/shared

go 1.24
`,
		"libs/shared/utils.go": `package shared

func CommonFunc() string {
	return "shared utility"
}
`,
	})

	context, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := suite.detector.DetectWorkspaceWithContext(context, workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Validate workspace structure
	assert.Equal(suite.T(), workspacePath, result.Root)
	assert.Equal(suite.T(), types.PROJECT_TYPE_GO, result.ProjectType)
	assert.Contains(suite.T(), result.Languages, types.PROJECT_TYPE_GO)
	assert.Len(suite.T(), result.SubProjects, 3)

	// Validate sub-projects
	subProjectNames := make([]string, len(result.SubProjects))
	for i, sp := range result.SubProjects {
		subProjectNames[i] = sp.Name
		assert.Equal(suite.T(), types.PROJECT_TYPE_GO, sp.ProjectType)
		assert.Contains(suite.T(), sp.Languages, types.PROJECT_TYPE_GO)
		assert.NotEmpty(suite.T(), sp.ID)
		assert.NotEmpty(suite.T(), sp.AbsolutePath)
	}

	assert.ElementsMatch(suite.T(), []string{"service-a", "service-b", "shared"}, subProjectNames)

	// Validate workspace ID and hash generation
	assert.NotEmpty(suite.T(), result.ID)
	assert.NotEmpty(suite.T(), result.Hash)
}

func (suite *WorkspaceDetectionSuite) TestDetectMonorepoStructure() {
	// Create a monorepo with multiple languages
	monorepoPath := suite.helper.CreateMonorepo("fullstack-monorepo", map[string]string{
		"package.json": `{
  "name": "fullstack-monorepo",
  "workspaces": ["apps/*", "packages/*"],
  "scripts": {
    "build": "turbo run build"
  }
}`,
		"turbo.json": `{
  "pipeline": {
    "build": {}
  }
}`,
		"apps/web/package.json": `{
  "name": "@company/web",
  "dependencies": {
    "react": "^18.0.0",
    "next": "^13.0.0"
  }
}`,
		"apps/web/tsconfig.json": `{
  "compilerOptions": {
    "target": "es2017"
  }
}`,
		"apps/web/src/page.tsx": `export default function Home() {
  return <div>Home</div>
}`,
		"apps/api/package.json": `{
  "name": "@company/api",
  "dependencies": {
    "express": "^4.18.0"
  }
}`,
		"apps/api/src/server.js": `const express = require('express');
const app = express();
app.listen(3000);`,
		"packages/ui/package.json": `{
  "name": "@company/ui",
  "dependencies": {
    "react": "^18.0.0"
  }
}`,
		"packages/ui/tsconfig.json": `{
  "compilerOptions": {
    "target": "es2017"
  }
}`,
		"packages/ui/src/Button.tsx": `export const Button = () => <button>Click me</button>`,
		"services/user-service/go.mod": `module company.com/user-service

go 1.24
`,
		"services/user-service/main.go": `package main

func main() {}
`,
		"libs/python-utils/pyproject.toml": `[project]
name = "python-utils"
version = "0.1.0"
`,
		"libs/python-utils/src/utils.py": `def helper_function():
    return "utility"
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(monorepoPath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Validate mixed language detection
	assert.Equal(suite.T(), types.PROJECT_TYPE_MIXED, result.ProjectType)
	assert.ElementsMatch(suite.T(), 
		[]string{types.PROJECT_TYPE_NODEJS, types.PROJECT_TYPE_TYPESCRIPT, types.PROJECT_TYPE_GO, types.PROJECT_TYPE_PYTHON}, 
		result.Languages)

	// Should detect multiple sub-projects
	assert.Greater(suite.T(), len(result.SubProjects), 3)

	// Validate specific sub-projects exist
	subProjectPaths := make([]string, len(result.SubProjects))
	for i, sp := range result.SubProjects {
		subProjectPaths[i] = sp.RelativePath
	}

	expectedPaths := []string{"apps/web", "apps/api", "packages/ui", "services/user-service", "libs/python-utils"}
	for _, expectedPath := range expectedPaths {
		assert.Contains(suite.T(), subProjectPaths, expectedPath, "Expected sub-project %s not found", expectedPath)
	}
}

func (suite *WorkspaceDetectionSuite) TestDetectMicroservicesArchitecture() {
	// Create microservices structure
	microservicesPath := suite.helper.CreateMicroservicesProject("microservices-project", map[string]string{
		"docker-compose.yml": `version: '3.8'
services:
  user-service:
    build: ./services/user-service
    ports:
      - "3001:3000"
  order-service:
    build: ./services/order-service
    ports:
      - "3002:3000"
  payment-service:
    build: ./services/payment-service
    ports:
      - "3003:3000"
  api-gateway:
    build: ./gateway
    ports:
      - "8080:8080"
`,
		"services/user-service/go.mod": `module microservices/user-service

go 1.24
`,
		"services/user-service/main.go": `package main

func main() {}
`,
		"services/user-service/Dockerfile": `FROM golang:1.24-alpine
WORKDIR /app
COPY . .
RUN go build -o main .
CMD ["./main"]`,
		"services/order-service/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.microservices</groupId>
    <artifactId>order-service</artifactId>
    <version>1.0.0</version>
</project>`,
		"services/order-service/src/main/java/OrderService.java": `public class OrderService {
    public static void main(String[] args) {}
}`,
		"services/payment-service/package.json": `{
  "name": "payment-service",
  "dependencies": {
    "express": "^4.18.0"
  }
}`,
		"services/payment-service/src/index.js": `const express = require('express');
const app = express();
app.listen(3000);`,
		"gateway/go.mod": `module microservices/gateway

go 1.24
`,
		"gateway/main.go": `package main

func main() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(microservicesPath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Validate microservices detection
	assert.Equal(suite.T(), types.PROJECT_TYPE_MIXED, result.ProjectType)
	assert.ElementsMatch(suite.T(), 
		[]string{types.PROJECT_TYPE_GO, types.PROJECT_TYPE_JAVA, types.PROJECT_TYPE_NODEJS}, 
		result.Languages)

	// Should detect service sub-projects
	serviceSubProjects := make([]string, 0)
	for _, sp := range result.SubProjects {
		if filepath.Dir(sp.RelativePath) == "services" || sp.RelativePath == "gateway" {
			serviceSubProjects = append(serviceSubProjects, sp.Name)
		}
	}

	expectedServices := []string{"user-service", "order-service", "payment-service", "gateway"}
	assert.ElementsMatch(suite.T(), expectedServices, serviceSubProjects)
}

func (suite *WorkspaceDetectionSuite) TestDetectPythonMLProject() {
	// Create Python ML project structure
	mlProjectPath := suite.helper.CreatePythonMLProject("ml-project", map[string]string{
		"pyproject.toml": `[project]
name = "ml-project"
version = "0.1.0"
dependencies = [
    "scikit-learn>=1.0.0",
    "tensorflow>=2.10.0",
    "mlflow>=2.0.0"
]
`,
		"src/models/classifier.py": `import sklearn
from sklearn.ensemble import RandomForestClassifier

class Classifier:
    def __init__(self):
        self.model = RandomForestClassifier()
`,
		"src/training/train.py": `import mlflow
import tensorflow as tf

def train_model():
    with mlflow.start_run():
        pass
`,
		"notebooks/exploration.ipynb": `{
 "cells": [],
 "metadata": {},
 "nbformat": 4
}`,
		"data-pipeline/go.mod": `module ml-project/data-pipeline

go 1.24
`,
		"data-pipeline/main.go": `package main

func main() {}
`,
		"api/package.json": `{
  "name": "ml-api",
  "dependencies": {
    "express": "^4.18.0",
    "axios": "^1.0.0"
  }
}`,
		"api/src/server.js": `const express = require('express');
const app = express();
app.listen(3000);`,
	})

	result, err := suite.detector.DetectWorkspaceAt(mlProjectPath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Validate ML project detection
	assert.Equal(suite.T(), types.PROJECT_TYPE_MIXED, result.ProjectType)
	assert.Contains(suite.T(), result.Languages, types.PROJECT_TYPE_PYTHON)
	assert.Contains(suite.T(), result.Languages, types.PROJECT_TYPE_GO)
	assert.Contains(suite.T(), result.Languages, types.PROJECT_TYPE_NODEJS)

	// Should detect sub-projects
	subProjectNames := make([]string, len(result.SubProjects))
	for i, sp := range result.SubProjects {
		subProjectNames[i] = sp.Name
	}

	expectedProjects := []string{"ml-project", "data-pipeline", "api"}
	for _, expectedProject := range expectedProjects {
		found := false
		for _, name := range subProjectNames {
			if name == expectedProject {
				found = true
				break
			}
		}
		assert.True(suite.T(), found, "Expected sub-project %s not found", expectedProject)
	}
}

func (suite *WorkspaceDetectionSuite) TestDetectNestedProjects() {
	// Create nested project structure to test depth handling
	nestedPath := suite.helper.CreateNestedProject("nested-project", map[string]string{
		"go.mod": `module nested-project

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		"level1/go.mod": `module nested-project/level1

go 1.24
`,
		"level1/main.go": `package main

func main() {}
`,
		"level1/level2/go.mod": `module nested-project/level1/level2

go 1.24
`,
		"level1/level2/main.go": `package main

func main() {}
`,
		"level1/level2/level3/go.mod": `module nested-project/level1/level2/level3

go 1.24
`,
		"level1/level2/level3/main.go": `package main

func main() {}
`,
		"level1/level2/level3/level4/go.mod": `module nested-project/level1/level2/level3/level4

go 1.24
`,
		"level1/level2/level3/level4/main.go": `package main

func main() {}
`,
		"level1/level2/level3/level4/level5/go.mod": `module nested-project/level1/level2/level3/level4/level5

go 1.24
`,
		"level1/level2/level3/level4/level5/main.go": `package main

func main() {}
`,
		"level1/level2/level3/level4/level5/level6/go.mod": `module nested-project/level1/level2/level3/level4/level5/level6

go 1.24
`,
		"level1/level2/level3/level4/level5/level6/main.go": `package main

func main() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(nestedPath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Validate depth limiting (default maxDepth is 5)
	assert.Equal(suite.T(), types.PROJECT_TYPE_GO, result.ProjectType)
	
	// Should detect projects up to depth 5 (inclusive)
	expectedSubProjects := []string{"nested-project", "level1", "level2", "level3", "level4", "level5"}
	subProjectNames := make([]string, len(result.SubProjects))
	for i, sp := range result.SubProjects {
		subProjectNames[i] = sp.Name
	}

	// level6 should not be detected due to depth limit
	assert.NotContains(suite.T(), subProjectNames, "level6")
	
	// But projects within depth limit should be detected
	for _, expected := range expectedSubProjects {
		assert.Contains(suite.T(), subProjectNames, expected, "Expected sub-project %s not found", expected)
	}
}

func (suite *WorkspaceDetectionSuite) TestDetectEmptyWorkspace() {
	// Create empty workspace
	emptyPath := suite.helper.CreateEmptyWorkspace("empty-workspace")

	result, err := suite.detector.DetectWorkspaceAt(emptyPath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Should create a root sub-project for empty workspace
	assert.Equal(suite.T(), types.PROJECT_TYPE_UNKNOWN, result.ProjectType)
	assert.Equal(suite.T(), []string{types.PROJECT_TYPE_UNKNOWN}, result.Languages)
	assert.Len(suite.T(), result.SubProjects, 1)
	
	rootProject := result.SubProjects[0]
	assert.Equal(suite.T(), ".", rootProject.RelativePath)
	assert.Equal(suite.T(), types.PROJECT_TYPE_UNKNOWN, rootProject.ProjectType)
	assert.Equal(suite.T(), filepath.Base(emptyPath), rootProject.Name)
}

func (suite *WorkspaceDetectionSuite) TestDetectWithIgnoredDirectories() {
	// Create workspace with ignored directories
	workspacePath := suite.helper.CreateWorkspaceWithIgnoredDirs("workspace-with-ignored", map[string]string{
		"go.mod": `module workspace-with-ignored

go 1.24
`,
		"main.go": `package main

func main() {}
`,
		"node_modules/some-package/package.json": `{
  "name": "some-package"
}`,
		"vendor/github.com/some/pkg/go.mod": `module github.com/some/pkg

go 1.24
`,
		".git/config": `[core]
repositoryformatversion = 0
`,
		"build/output/binary": "binary content",
		"dist/bundle.js": "bundled javascript",
		"target/classes/Main.class": "compiled java",
		"venv/lib/python3.9/site-packages/package.py": "python package",
		"services/user-service/go.mod": `module workspace-with-ignored/user-service

go 1.24
`,
		"services/user-service/main.go": `package main

func main() {}
`,
	})

	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Should detect main project and services, but ignore system directories
	assert.Equal(suite.T(), types.PROJECT_TYPE_GO, result.ProjectType)
	assert.Len(suite.T(), result.SubProjects, 2) // root + user-service only

	subProjectNames := make([]string, len(result.SubProjects))
	for i, sp := range result.SubProjects {
		subProjectNames[i] = sp.Name
	}

	assert.Contains(suite.T(), subProjectNames, "workspace-with-ignored")
	assert.Contains(suite.T(), subProjectNames, "user-service")
}

func (suite *WorkspaceDetectionSuite) TestDetectWithContextCancellation() {
	// Create a large workspace to test context cancellation
	largeWorkspacePath := suite.helper.CreateLargeWorkspace("large-workspace", 100)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := suite.detector.DetectWorkspaceWithContext(ctx, largeWorkspacePath)
	
	// Should handle context cancellation gracefully
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "context")
}

func (suite *WorkspaceDetectionSuite) TestWorkspacePerformance() {
	// Create a moderately sized workspace
	workspacePath := suite.helper.CreateLargeWorkspace("performance-test", 50)

	start := time.Now()
	result, err := suite.detector.DetectWorkspaceAt(workspacePath)
	duration := time.Since(start)

	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), result)

	// Should complete within 5 seconds for 50 sub-projects
	assert.Less(suite.T(), duration, 5*time.Second, "Workspace detection took too long: %v", duration)
	
	// Should detect multiple sub-projects
	assert.Greater(suite.T(), len(result.SubProjects), 40, "Expected to detect most sub-projects")
	
	// Should have reasonable memory usage (implicit - no specific assertion for memory)
}

func (suite *WorkspaceDetectionSuite) TestWorkspaceIDGeneration() {
	// Create same workspace structure in different locations
	workspace1Path := suite.helper.CreateSimpleGoProject("workspace1", map[string]string{
		"go.mod": `module test-project

go 1.24
`,
		"main.go": `package main

func main() {}
`,
	})

	workspace2Path := suite.helper.CreateSimpleGoProject("workspace2", map[string]string{
		"go.mod": `module test-project

go 1.24
`,
		"main.go": `package main

func main() {}
`,
	})

	result1, err1 := suite.detector.DetectWorkspaceAt(workspace1Path)
	result2, err2 := suite.detector.DetectWorkspaceAt(workspace2Path)

	require.NoError(suite.T(), err1)
	require.NoError(suite.T(), err2)
	require.NotNil(suite.T(), result1)
	require.NotNil(suite.T(), result2)

	// IDs should be different for different paths
	assert.NotEqual(suite.T(), result1.ID, result2.ID)
	
	// But detection of same path should give same ID
	result1Again, err := suite.detector.DetectWorkspaceAt(workspace1Path)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), result1.ID, result1Again.ID)

	// Hashes should be different for different structures
	assert.NotEqual(suite.T(), result1.Hash, result2.Hash)
}

func TestWorkspaceDetectionSuite(t *testing.T) {
	suite.Run(t, new(WorkspaceDetectionSuite))
}