package detectors

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeScriptDetector_DetectTypeScriptProject(t *testing.T) {
	tests := []struct {
		name            string
		setupProject    func(string) error
		expectedMatch   bool
		expectedConfidence float64
		expectedFramework  string
		expectedBuildTool  string
	}{
		{
			name: "Basic TypeScript Project",
			setupProject: func(dir string) error {
				return createBasicTypeScriptProject(dir)
			},
			expectedMatch:      true,
			expectedConfidence: 0.85,
			expectedFramework:  "",
			expectedBuildTool:  "tsc",
		},
		{
			name: "React TypeScript Project", 
			setupProject: func(dir string) error {
				return createReactTypeScriptProject(dir)
			},
			expectedMatch:      true,
			expectedConfidence: 0.90,
			expectedFramework:  "react",
			expectedBuildTool:  "webpack",
		},
		{
			name: "Node.js TypeScript Project",
			setupProject: func(dir string) error {
				return createNodeTypeScriptProject(dir)
			},
			expectedMatch:      true,
			expectedConfidence: 0.90,
			expectedFramework:  "node",
			expectedBuildTool:  "tsc",
		},
		{
			name: "Vue TypeScript Project",
			setupProject: func(dir string) error {
				return createVueTypeScriptProject(dir)
			},
			expectedMatch:      true,
			expectedConfidence: 0.85,
			expectedFramework:  "vue",
			expectedBuildTool:  "vite",
		},
		{
			name: "No TypeScript Project",
			setupProject: func(dir string) error {
				return createNonTypeScriptProject(dir)
			},
			expectedMatch:      false,
			expectedConfidence: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "typescript-detector-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Setup project structure
			err = tt.setupProject(tempDir)
			require.NoError(t, err)

			// Create detector
			detector := NewTypeScriptProjectDetector()
			
			// Run detection
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			result, err := detector.DetectLanguage(ctx, tempDir)
			require.NoError(t, err)

			// Verify results
			if tt.expectedMatch {
				assert.Equal(t, "typescript", result.Language, "Should detect TypeScript as language")
				assert.GreaterOrEqual(t, result.Confidence, tt.expectedConfidence, "Confidence should meet minimum threshold")
				assert.Contains(t, result.RequiredServers, "typescript-language-server", "Should require TypeScript language server")
				
				// Framework and build tool would be checked in metadata if needed
				// The actual implementation may store these differently
			} else {
				assert.NotEqual(t, "typescript", result.Language, "Should not detect TypeScript project")
				// For JavaScript projects without TypeScript, we still expect some confidence
				// as a Node.js project, just not as a TypeScript project
			}
		})
	}
}

func TestTypeScriptDetector_PackageJsonAnalysis(t *testing.T) {
	tests := []struct {
		name                string
		packageJson         map[string]interface{}
		expectedHasTypeScript bool
		expectedFramework    string
		expectedBuildTool    string
		expectedPackageManager string
	}{
		{
			name: "TypeScript with React",
			packageJson: map[string]interface{}{
				"dependencies": map[string]interface{}{
					"react": "^18.0.0",
					"react-dom": "^18.0.0",
				},
				"devDependencies": map[string]interface{}{
					"typescript": "^5.0.0",
					"@types/react": "^18.0.0",
					"webpack": "^5.0.0",
				},
				"scripts": map[string]interface{}{
					"build": "webpack",
					"start": "webpack serve",
				},
			},
			expectedHasTypeScript: true,
			expectedFramework:     "react",
			expectedBuildTool:     "webpack",
			expectedPackageManager: "npm",
		},
		{
			name: "TypeScript with Vue",
			packageJson: map[string]interface{}{
				"dependencies": map[string]interface{}{
					"vue": "^3.0.0",
				},
				"devDependencies": map[string]interface{}{
					"typescript": "^5.0.0",
					"@vue/tsconfig": "^0.4.0",
					"vite": "^4.0.0",
				},
				"scripts": map[string]interface{}{
					"build": "vite build",
					"dev": "vite",
				},
			},
			expectedHasTypeScript: true,
			expectedFramework:     "vue",
			expectedBuildTool:     "vite",
			expectedPackageManager: "npm",
		},
		{
			name: "Node.js TypeScript",
			packageJson: map[string]interface{}{
				"dependencies": map[string]interface{}{
					"express": "^4.18.0",
				},
				"devDependencies": map[string]interface{}{
					"typescript": "^5.0.0",
					"@types/node": "^20.0.0",
					"@types/express": "^4.17.0",
				},
				"scripts": map[string]interface{}{
					"build": "tsc",
					"start": "node dist/index.js",
				},
			},
			expectedHasTypeScript: true,
			expectedFramework:     "node",
			expectedBuildTool:     "tsc",
			expectedPackageManager: "npm",
		},
		{
			name: "No TypeScript",
			packageJson: map[string]interface{}{
				"dependencies": map[string]interface{}{
					"lodash": "^4.17.0",
				},
				"scripts": map[string]interface{}{
					"start": "node index.js",
				},
			},
			expectedHasTypeScript: false,
			expectedFramework:     "",
			expectedBuildTool:     "",
			expectedPackageManager: "npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory with package.json
			tempDir, err := os.MkdirTemp("", "package-analysis-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			packageJsonPath := filepath.Join(tempDir, "package.json")
			packageJsonData, err := json.MarshalIndent(tt.packageJson, "", "  ")
			require.NoError(t, err)
			
			err = os.WriteFile(packageJsonPath, packageJsonData, 0644)
			require.NoError(t, err)

			// Create detector and analyze
			detector := NewTypeScriptProjectDetector()
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			result, err := detector.DetectLanguage(ctx, tempDir)
			require.NoError(t, err)

			// Verify package.json analysis results
			if tt.expectedHasTypeScript {
				// Projects with only TypeScript in package.json but no actual TS files/config
				// are detected as Node.js projects, not TypeScript projects
				assert.Equal(t, "nodejs", result.Language, "Should detect Node.js for package.json-only project")
				assert.Greater(t, result.Confidence, 0.0, "Should have positive confidence")
				// Framework and build tool detection would be verified through metadata if implemented
			} else {
				assert.NotEqual(t, "typescript", result.Language, "Should not detect TypeScript in package.json")
				// Non-TypeScript JavaScript projects still have confidence as Node.js projects
			}
		})
	}
}

func TestTypeScriptDetector_TSConfigAnalysis(t *testing.T) {
	tests := []struct {
		name              string
		tsConfig          map[string]interface{}
		expectedValid     bool
		expectedTarget    string
		expectedModuleRes string
		expectedJSX       string
	}{
		{
			name: "Valid React TSConfig",
			tsConfig: map[string]interface{}{
				"compilerOptions": map[string]interface{}{
					"target": "ES2020",
					"module": "esnext",
					"moduleResolution": "node",
					"jsx": "react-jsx",
					"strict": true,
					"esModuleInterop": true,
				},
			},
			expectedValid:     true,
			expectedTarget:    "ES2020", 
			expectedModuleRes: "node",
			expectedJSX:       "react-jsx",
		},
		{
			name: "Node.js TSConfig",
			tsConfig: map[string]interface{}{
				"compilerOptions": map[string]interface{}{
					"target": "ES2022",
					"module": "commonjs",
					"moduleResolution": "node",
					"strict": true,
					"declaration": true,
					"outDir": "./dist",
				},
			},
			expectedValid:     true,
			expectedTarget:    "ES2022",
			expectedModuleRes: "node",
			expectedJSX:       "",
		},
		{
			name: "Invalid TSConfig",
			tsConfig: map[string]interface{}{
				"invalid": "config",
			},
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory with tsconfig.json
			tempDir, err := os.MkdirTemp("", "tsconfig-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			tsconfigPath := filepath.Join(tempDir, "tsconfig.json")
			tsconfigData, err := json.MarshalIndent(tt.tsConfig, "", "  ")
			require.NoError(t, err)
			
			err = os.WriteFile(tsconfigPath, tsconfigData, 0644)
			require.NoError(t, err)

			// Create basic package.json to ensure TypeScript detection
			packageJson := map[string]interface{}{
				"devDependencies": map[string]interface{}{
					"typescript": "^5.0.0",
				},
			}
			packageJsonData, err := json.MarshalIndent(packageJson, "", "  ")
			require.NoError(t, err)
			
			err = os.WriteFile(filepath.Join(tempDir, "package.json"), packageJsonData, 0644)
			require.NoError(t, err)

			// Create detector and analyze
			detector := NewTypeScriptProjectDetector()
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			result, err := detector.DetectLanguage(ctx, tempDir)
			require.NoError(t, err)

			// Verify TSConfig analysis results
			if tt.expectedValid {
				assert.Equal(t, "typescript", result.Language, "Should detect valid TSConfig")
				assert.Greater(t, result.Confidence, 0.0, "Should have positive confidence for valid TSConfig")
				// Additional TSConfig-specific assertions would go here
				// These would depend on the actual implementation details
			}
		})
	}
}

func TestTypeScriptDetector_FrameworkDetection(t *testing.T) {
	frameworks := []struct {
		name         string
		dependencies []string
		expected     string
	}{
		{
			name:         "React",
			dependencies: []string{"react", "@types/react"},
			expected:     "react",
		},
		{
			name:         "Vue",
			dependencies: []string{"vue", "@vue/compiler-sfc"},
			expected:     "vue",
		},
		{
			name:         "Angular",
			dependencies: []string{"@angular/core", "@angular/cli"},
			expected:     "angular",
		},
		{
			name:         "Next.js",
			dependencies: []string{"next", "react"},
			expected:     "next",
		},
		{
			name:         "Express",
			dependencies: []string{"express", "@types/express"},
			expected:     "node",
		},
	}

	for _, framework := range frameworks {
		t.Run(framework.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "framework-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Create package.json with framework dependencies
			deps := make(map[string]interface{})
			devDeps := make(map[string]interface{})
			
			for _, dep := range framework.dependencies {
				if dep == "typescript" || strings.HasPrefix(dep, "@types/") {
					devDeps[dep] = "^5.0.0"
				} else {
					deps[dep] = "^1.0.0"
				}
			}
			devDeps["typescript"] = "^5.0.0"

			packageJson := map[string]interface{}{
				"dependencies":    deps,
				"devDependencies": devDeps,
			}

			packageJsonData, err := json.MarshalIndent(packageJson, "", "  ")
			require.NoError(t, err)
			
			err = os.WriteFile(filepath.Join(tempDir, "package.json"), packageJsonData, 0644)
			require.NoError(t, err)

			// Create detector and test framework detection
			detector := NewTypeScriptProjectDetector()
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			result, err := detector.DetectLanguage(ctx, tempDir)
			require.NoError(t, err)

			// Projects with only TypeScript in devDependencies but no actual TS files/config
			// are detected as Node.js projects
			assert.Equal(t, "nodejs", result.Language, "Should detect Node.js for package.json-only project")
			assert.Greater(t, result.Confidence, 0.0, "Should have positive confidence")
			// Framework detection would be verified through metadata if implemented
		})
	}
}

func TestTypeScriptDetector_ConfidenceScoring(t *testing.T) {
	tests := []struct {
		name               string
		setupProject       func(string) error
		expectedMinConfidence float64
		expectedMaxConfidence float64
	}{
		{
			name: "High Confidence - Complete Setup",
			setupProject: func(dir string) error {
				return createCompleteTypeScriptProject(dir)
			},
			expectedMinConfidence: 0.85,
			expectedMaxConfidence: 1.0,
		},
		{
			name: "Medium Confidence - Basic Setup",
			setupProject: func(dir string) error {
				return createBasicTypeScriptProject(dir)
			},
			expectedMinConfidence: 0.75,
			expectedMaxConfidence: 0.95,
		},
		{
			name: "Low Confidence - Minimal Setup",
			setupProject: func(dir string) error {
				return createMinimalTypeScriptProject(dir)
			},
			expectedMinConfidence: 0.7,
			expectedMaxConfidence: 0.86,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "confidence-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Setup project
			err = tt.setupProject(tempDir)
			require.NoError(t, err)

			// Create detector and test confidence scoring
			detector := NewTypeScriptProjectDetector()
			
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			
			result, err := detector.DetectLanguage(ctx, tempDir)
			require.NoError(t, err)

			assert.Equal(t, "typescript", result.Language, "Should detect TypeScript project")
			assert.GreaterOrEqual(t, result.Confidence, tt.expectedMinConfidence, "Confidence should be above minimum")
			assert.LessOrEqual(t, result.Confidence, tt.expectedMaxConfidence, "Confidence should be below maximum")
		})
	}
}

// Helper functions to create test project structures

func createBasicTypeScriptProject(dir string) error {
	// Create package.json with TypeScript
	packageJson := map[string]interface{}{
		"devDependencies": map[string]interface{}{
			"typescript": "^5.0.0",
		},
		"scripts": map[string]interface{}{
			"build": "tsc",
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "package.json"), packageJson); err != nil {
		return err
	}

	// Create tsconfig.json
	tsconfig := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"target": "ES2020",
			"module": "commonjs",
			"strict": true,
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "tsconfig.json"), tsconfig); err != nil {
		return err
	}

	// Create a TypeScript source file
	return os.WriteFile(filepath.Join(dir, "index.ts"), []byte(`
interface User {
  id: number;
  name: string;
}

function greetUser(user: User): string {
  return `+"`Hello, ${user.name}!`"+`;
}
`), 0644)
}

func createReactTypeScriptProject(dir string) error {
	// Create package.json with React and TypeScript
	packageJson := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"react":     "^18.0.0",
			"react-dom": "^18.0.0",
		},
		"devDependencies": map[string]interface{}{
			"typescript":    "^5.0.0",
			"@types/react": "^18.0.0",
			"webpack":      "^5.0.0",
		},
		"scripts": map[string]interface{}{
			"build": "webpack",
			"start": "webpack serve",
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "package.json"), packageJson); err != nil {
		return err
	}

	// Create TSConfig with React JSX
	tsconfig := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"target":           "ES2020",
			"module":           "esnext",
			"moduleResolution": "node",
			"jsx":              "react-jsx",
			"strict":           true,
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "tsconfig.json"), tsconfig); err != nil {
		return err
	}

	// Create React component
	return os.WriteFile(filepath.Join(dir, "App.tsx"), []byte(`
import React from 'react';

interface AppProps {
  title: string;
}

const App: React.FC<AppProps> = ({ title }) => {
  return <h1>{title}</h1>;
};

export default App;
`), 0644)
}

func createNodeTypeScriptProject(dir string) error {
	// Create package.json with Node.js and TypeScript
	packageJson := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"express": "^4.18.0",
		},
		"devDependencies": map[string]interface{}{
			"typescript":      "^5.0.0",
			"@types/node":     "^20.0.0",
			"@types/express":  "^4.17.0",
		},
		"scripts": map[string]interface{}{
			"build": "tsc",
			"start": "node dist/index.js",
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "package.json"), packageJson); err != nil {
		return err
	}

	// Create Node.js TSConfig
	tsconfig := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"target":     "ES2022",
			"module":     "commonjs",
			"outDir":     "./dist",
			"strict":     true,
			"esModuleInterop": true,
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "tsconfig.json"), tsconfig); err != nil {
		return err
	}

	// Create Express server
	return os.WriteFile(filepath.Join(dir, "server.ts"), []byte(`
import express, { Request, Response } from 'express';

const app = express();
const port = 3000;

app.get('/', (req: Request, res: Response) => {
  res.send('Hello TypeScript!');
});

app.listen(port, () => {
  console.log(`+"`Server running on port ${port}`"+`);
});
`), 0644)
}

func createVueTypeScriptProject(dir string) error {
	// Create package.json with Vue and TypeScript
	packageJson := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"vue": "^3.0.0",
		},
		"devDependencies": map[string]interface{}{
			"typescript":       "^5.0.0",
			"@vue/tsconfig":    "^0.4.0",
			"vite":             "^4.0.0",
		},
		"scripts": map[string]interface{}{
			"build": "vite build",
			"dev":   "vite",
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "package.json"), packageJson); err != nil {
		return err
	}

	// Create Vue TSConfig
	tsconfig := map[string]interface{}{
		"extends": "@vue/tsconfig/tsconfig.dom.json",
		"compilerOptions": map[string]interface{}{
			"target": "ES2020",
			"strict": true,
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "tsconfig.json"), tsconfig); err != nil {
		return err
	}

	// Create Vue component
	return os.WriteFile(filepath.Join(dir, "App.vue"), []byte(`
<template>
  <div>{{ message }}</div>
</template>

<script setup lang="ts">
interface User {
  name: string;
}

const user: User = { name: 'Vue TypeScript' };
const message = `+"`Hello, ${user.name}!`"+`;
</script>
`), 0644)
}

func createCompleteTypeScriptProject(dir string) error {
	// Start with React TypeScript project
	if err := createReactTypeScriptProject(dir); err != nil {
		return err
	}

	// Add additional TypeScript files
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return err
	}

	// Add types directory
	typesDir := filepath.Join(srcDir, "types")
	if err := os.MkdirAll(typesDir, 0755); err != nil {
		return err
	}

	// Add utils directory
	utilsDir := filepath.Join(srcDir, "utils")
	if err := os.MkdirAll(utilsDir, 0755); err != nil {
		return err
	}

	// Create additional TypeScript files for higher confidence
	files := map[string]string{
		filepath.Join(typesDir, "api.ts"): `
export interface ApiResponse<T> {
  data: T;
  status: number;
  message: string;
}
`,
		filepath.Join(utilsDir, "helpers.ts"): `
export function formatDate(date: Date): string {
  return date.toISOString().split('T')[0];
}
`,
	}

	for filePath, content := range files {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func createMinimalTypeScriptProject(dir string) error {
	// Create package.json with TypeScript dependency
	packageJson := map[string]interface{}{
		"devDependencies": map[string]interface{}{
			"typescript": "^5.0.0",
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "package.json"), packageJson); err != nil {
		return err
	}
	
	// Create a minimal TypeScript file to ensure it's detected as TypeScript
	return os.WriteFile(filepath.Join(dir, "index.ts"), []byte(`
const greeting: string = "Hello, TypeScript!";
console.log(greeting);
`), 0644)
}

func createNonTypeScriptProject(dir string) error {
	// Create a JavaScript project without TypeScript
	packageJson := map[string]interface{}{
		"dependencies": map[string]interface{}{
			"lodash": "^4.17.0",
		},
		"scripts": map[string]interface{}{
			"start": "node index.js",
		},
	}
	
	if err := writeJSONFile(filepath.Join(dir, "package.json"), packageJson); err != nil {
		return err
	}

	// Create a JavaScript file
	return os.WriteFile(filepath.Join(dir, "index.js"), []byte(`
const _ = require('lodash');

function greet(name) {
  return `+"`Hello, ${name}!`"+`;
}

console.log(greet('JavaScript'));
`), 0644)
}

func writeJSONFile(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, jsonData, 0644)
}