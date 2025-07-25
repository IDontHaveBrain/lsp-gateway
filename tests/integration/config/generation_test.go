package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/integration/config/helpers"
)

// AutoGenerationTestSuite provides comprehensive tests for automatic configuration generation
type AutoGenerationTestSuite struct {
	suite.Suite
	testHelper *helpers.ConfigTestHelper
	tempDir    string
	cleanup    []func()
}

// SetupTest initializes the test environment
func (suite *AutoGenerationTestSuite) SetupTest() {
	tempDir, err := os.MkdirTemp("", "generation-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir

	suite.testHelper = helpers.NewConfigTestHelper(tempDir)
	suite.cleanup = []func(){}

	// Register cleanup for temp directory
	suite.cleanup = append(suite.cleanup, func() {
		_ = os.RemoveAll(tempDir)
	})
}

// TearDownTest cleans up test resources
func (suite *AutoGenerationTestSuite) TearDownTest() {
	for _, cleanupFunc := range suite.cleanup {
		cleanupFunc()
	}
}

// TestProjectDetectionAndAnalysis tests comprehensive project detection and analysis
func (suite *AutoGenerationTestSuite) TestProjectDetectionAndAnalysis() {
	suite.Run("DetectGoMonorepoProject", func() {
		// Create Go monorepo structure
		goMonorepoStructure := map[string]string{
			// Root workspace
			"go.work": `go 1.21

use (
	./services/auth
	./services/api
	./services/worker
	./libs/shared
	./libs/database
)`,

			// Auth service
			"services/auth/go.mod":     "module github.com/company/monorepo/services/auth\n\ngo 1.21\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.1\n\tgithub.com/company/monorepo/libs/shared v0.0.0\n)",
			"services/auth/main.go":    "package main\n\nimport (\n\t\"github.com/gin-gonic/gin\"\n\t\"github.com/company/monorepo/libs/shared\"\n)\n\nfunc main() {\n\tr := gin.Default()\n\tr.Run(\":8080\")\n}",
			"services/auth/handler.go": "package main\n\nimport \"github.com/gin-gonic/gin\"\n\ntype AuthHandler struct{}\n\nfunc (h *AuthHandler) Login(c *gin.Context) {}",
			"services/auth/model.go":   "package main\n\ntype User struct {\n\tID    int    `json:\"id\"`\n\tEmail string `json:\"email\"`\n}",

			// API service
			"services/api/go.mod":        "module github.com/company/monorepo/services/api\n\ngo 1.21\n\nrequire (\n\tgithub.com/labstack/echo/v4 v4.11.1\n\tgithub.com/company/monorepo/libs/database v0.0.0\n)",
			"services/api/main.go":       "package main\n\nimport (\n\t\"github.com/labstack/echo/v4\"\n\t\"github.com/company/monorepo/libs/database\"\n)\n\nfunc main() {\n\te := echo.New()\n\te.Logger.Fatal(e.Start(\":8081\"))\n}",
			"services/api/controller.go": "package main\n\nimport \"github.com/labstack/echo/v4\"\n\ntype APIController struct{}\n\nfunc (c *APIController) GetUsers(ctx echo.Context) error { return nil }",

			// Worker service
			"services/worker/go.mod":  "module github.com/company/monorepo/services/worker\n\ngo 1.21\n\nrequire (\n\tgithub.com/company/monorepo/libs/shared v0.0.0\n\tgithub.com/company/monorepo/libs/database v0.0.0\n)",
			"services/worker/main.go": "package main\n\nimport (\n\t\"time\"\n\t\"github.com/company/monorepo/libs/shared\"\n)\n\nfunc main() {\n\tfor {\n\t\tprocessJobs()\n\t\ttime.Sleep(5 * time.Second)\n\t}\n}",
			"services/worker/job.go":  "package main\n\nimport \"context\"\n\ntype Job struct {\n\tID      string\n\tPayload map[string]interface{}\n}\n\nfunc processJobs() {\n\t// Process jobs\n}",

			// Shared library
			"libs/shared/go.mod":    "module github.com/company/monorepo/libs/shared\n\ngo 1.21",
			"libs/shared/config.go": "package shared\n\nimport \"os\"\n\ntype Config struct {\n\tDatabaseURL string\n\tRedisURL    string\n}\n\nfunc LoadConfig() *Config {\n\treturn &Config{\n\t\tDatabaseURL: os.Getenv(\"DATABASE_URL\"),\n\t\tRedisURL:    os.Getenv(\"REDIS_URL\"),\n\t}\n}",
			"libs/shared/logger.go": "package shared\n\nimport \"log\"\n\ntype Logger struct{}\n\nfunc NewLogger() *Logger {\n\treturn &Logger{}\n}\n\nfunc (l *Logger) Info(msg string) {\n\tlog.Println(\"INFO:\", msg)\n}",

			// Database library
			"libs/database/go.mod":        "module github.com/company/monorepo/libs/database\n\ngo 1.21\n\nrequire (\n\tgorm.io/gorm v1.25.5\n\tgorm.io/driver/postgres v1.5.4\n)",
			"libs/database/connection.go": "package database\n\nimport (\n\t\"gorm.io/gorm\"\n\t\"gorm.io/driver/postgres\"\n)\n\ntype DB struct {\n\t*gorm.DB\n}\n\nfunc Connect(dsn string) (*DB, error) {\n\tdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})\n\treturn &DB{db}, err\n}",
			"libs/database/models.go":     "package database\n\nimport \"gorm.io/gorm\"\n\ntype BaseModel struct {\n\tID        uint           `gorm:\"primarykey\"`\n\tCreatedAt time.Time\n\tUpdatedAt time.Time\n\tDeletedAt gorm.DeletedAt `gorm:\"index\"`\n}",

			// Root files
			"Makefile": `# Go Monorepo Makefile
.PHONY: build test lint clean

build:
	go work sync
	cd services/auth && go build -o ../../bin/auth
	cd services/api && go build -o ../../bin/api  
	cd services/worker && go build -o ../../bin/worker

test:
	go work sync
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
	go clean ./...`,

			"README.md": `# Go Monorepo

A comprehensive Go monorepo with microservices architecture:

- **services/auth**: Authentication service (Gin)
- **services/api**: REST API service (Echo)  
- **services/worker**: Background worker service
- **libs/shared**: Shared utilities and configuration
- **libs/database**: Database connection and models

Uses Go workspaces for dependency management.`,
		}

		projectPath := suite.testHelper.CreateTestProject("go-monorepo", goMonorepoStructure)

		// Detect and analyze project
		analysis, err := suite.testHelper.DetectAndAnalyzeProject(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(analysis)

		// Validate project detection
		suite.Equal("monorepo", analysis.ProjectType, "Should detect as Go monorepo")
		suite.Equal("go", analysis.DominantLanguage, "Go should be dominant language")
		suite.Contains(analysis.Languages, "go", "Should detect Go language")

		// Validate Go workspace detection
		suite.True(analysis.HasGoWorkspace, "Should detect Go workspace")
		suite.NotEmpty(analysis.GoWorkspaceModules, "Should identify workspace modules")
		suite.GreaterOrEqual(len(analysis.GoWorkspaceModules), 5, "Should find all workspace modules")

		// Validate build system detection
		suite.Contains(analysis.BuildSystems, "go", "Should detect Go build system")
		suite.Contains(analysis.BuildSystems, "make", "Should detect Makefile")

		// Validate service architecture detection
		suite.GreaterOrEqual(len(analysis.Services), 3, "Should detect multiple services")
		suite.Contains(analysis.Services, "auth", "Should detect auth service")
		suite.Contains(analysis.Services, "api", "Should detect api service")
		suite.Contains(analysis.Services, "worker", "Should detect worker service")

		// Validate library detection
		suite.GreaterOrEqual(len(analysis.Libraries), 2, "Should detect shared libraries")
		suite.Contains(analysis.Libraries, "shared", "Should detect shared library")
		suite.Contains(analysis.Libraries, "database", "Should detect database library")
	})

	suite.Run("DetectPythonMLProject", func() {
		// Create Python ML project structure
		pythonMLStructure := map[string]string{
			// Main project configuration
			"pyproject.toml": `[build-system]
requires = ["setuptools>=65.0", "wheel", "setuptools-scm"]
build-backend = "setuptools.build_meta"

[project]
name = "ml-platform"
version = "1.0.0"
description = "Machine Learning Platform"
authors = [{name = "ML Team", email = "ml-team@company.com"}]
dependencies = [
    "fastapi>=0.100.0",
    "uvicorn>=0.23.0",
    "pandas>=2.0.0",
    "numpy>=1.24.0",
    "scikit-learn>=1.3.0",
    "tensorflow>=2.13.0",
    "torch>=2.0.0",
    "transformers>=4.30.0",
    "mlflow>=2.5.0",
    "pydantic>=2.0.0",
    "sqlalchemy>=2.0.0",
    "alembic>=1.11.0"
]

[project.optional-dependencies]
dev = [
    "pytest>=7.0.0",
    "pytest-asyncio>=0.21.0",
    "black>=23.0.0",
    "isort>=5.12.0",
    "flake8>=6.0.0",
    "mypy>=1.0.0"
]

[tool.setuptools.packages.find]
where = ["src"]

[tool.black]
line-length = 88
target-version = ['py311']

[tool.isort]
profile = "black"
multi_line_output = 3`,

			// Source code structure
			"src/ml_platform/__init__.py": "from .main import app\n\n__version__ = \"1.0.0\"",

			"src/ml_platform/main.py": `from fastapi import FastAPI, HTTPException
from contextlib import asynccontextmanager
from .models import PredictionRequest, PredictionResponse
from .ml_service import MLService
from .database import init_db

@asynccontextmanager
async def lifespan(app: FastAPI):
    # Initialize database
    await init_db()
    # Load ML models
    await MLService.load_models()
    yield
    # Cleanup
    await MLService.cleanup()

app = FastAPI(
    title="ML Platform API",
    description="Production ML inference platform",
    version="1.0.0",
    lifespan=lifespan
)

@app.post("/predict", response_model=PredictionResponse)
async def predict(request: PredictionRequest):
    try:
        result = await MLService.predict(request.data, request.model_name)
        return PredictionResponse(prediction=result, model_name=request.model_name)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
async def health_check():
    return {"status": "healthy", "models": await MLService.get_model_status()}`,

			"src/ml_platform/models.py": `from pydantic import BaseModel, Field
from typing import List, Dict, Any, Optional
from enum import Enum

class ModelType(str, Enum):
    CLASSIFICATION = "classification"
    REGRESSION = "regression"
    NLP = "nlp"
    COMPUTER_VISION = "computer_vision"

class PredictionRequest(BaseModel):
    data: Dict[str, Any] = Field(..., description="Input data for prediction")
    model_name: str = Field(..., description="Name of the model to use")
    preprocessing: Optional[Dict[str, Any]] = Field(None, description="Preprocessing parameters")

class PredictionResponse(BaseModel):
    prediction: Any = Field(..., description="Model prediction result")
    model_name: str = Field(..., description="Name of the model used")
    confidence: Optional[float] = Field(None, description="Prediction confidence score")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Additional metadata")

class ModelInfo(BaseModel):
    name: str
    type: ModelType
    version: str
    accuracy: Optional[float] = None
    created_at: str
    is_loaded: bool = False`,

			"src/ml_platform/ml_service.py": `import asyncio
import pickle
import numpy as np
import pandas as pd
from typing import Dict, Any, List, Optional
from pathlib import Path
import mlflow
import logging

logger = logging.getLogger(__name__)

class MLService:
    _models: Dict[str, Any] = {}
    _model_metadata: Dict[str, Dict] = {}
    
    @classmethod
    async def load_models(cls):
        """Load all available ML models"""
        models_dir = Path("models")
        if not models_dir.exists():
            logger.warning("Models directory not found")
            return
            
        for model_path in models_dir.glob("*.pkl"):
            try:
                with open(model_path, 'rb') as f:
                    model = pickle.load(f)
                cls._models[model_path.stem] = model
                logger.info(f"Loaded model: {model_path.stem}")
            except Exception as e:
                logger.error(f"Failed to load model {model_path}: {e}")
    
    @classmethod
    async def predict(cls, data: Dict[str, Any], model_name: str) -> Any:
        """Make prediction using specified model"""
        if model_name not in cls._models:
            raise ValueError(f"Model {model_name} not found")
            
        model = cls._models[model_name]
        
        # Convert data to appropriate format
        if isinstance(data.get('features'), list):
            features = np.array(data['features']).reshape(1, -1)
        else:
            features = pd.DataFrame([data])
            
        # Make prediction
        prediction = model.predict(features)
        
        # Calculate confidence if supported
        confidence = None
        if hasattr(model, 'predict_proba'):
            proba = model.predict_proba(features)
            confidence = float(np.max(proba))
            
        return {
            "prediction": prediction.tolist() if hasattr(prediction, 'tolist') else prediction,
            "confidence": confidence
        }
    
    @classmethod
    async def get_model_status(cls) -> Dict[str, bool]:
        """Get status of all loaded models"""
        return {name: True for name in cls._models.keys()}
    
    @classmethod
    async def cleanup(cls):
        """Cleanup resources"""
        cls._models.clear()
        cls._model_metadata.clear()`,

			"src/ml_platform/database.py": `from sqlalchemy.ext.asyncio import AsyncSession, create_async_engine
from sqlalchemy.orm import sessionmaker, declarative_base
from sqlalchemy import Column, Integer, String, DateTime, Text, Float
import os
from datetime import datetime

DATABASE_URL = os.getenv("DATABASE_URL", "sqlite+aiosqlite:///./ml_platform.db")

engine = create_async_engine(DATABASE_URL, echo=True)
AsyncSessionLocal = sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)

Base = declarative_base()

class PredictionLog(Base):
    __tablename__ = "prediction_logs"
    
    id = Column(Integer, primary_key=True, index=True)
    model_name = Column(String, index=True)
    input_data = Column(Text)
    prediction = Column(Text)
    confidence = Column(Float, nullable=True)
    created_at = Column(DateTime, default=datetime.utcnow)

class ModelMetrics(Base):
    __tablename__ = "model_metrics"
    
    id = Column(Integer, primary_key=True, index=True)
    model_name = Column(String, index=True)
    metric_name = Column(String)
    metric_value = Column(Float)
    created_at = Column(DateTime, default=datetime.utcnow)

async def init_db():
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

async def get_db():
    async with AsyncSessionLocal() as session:
        yield session`,

			// Training scripts
			"scripts/train_model.py": `#!/usr/bin/env python3
import pandas as pd
import numpy as np
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split
from sklearn.metrics import accuracy_score, classification_report
import pickle
import mlflow
import mlflow.sklearn
from pathlib import Path

def train_classification_model(data_path: str, model_name: str):
    """Train a classification model"""
    # Load data
    data = pd.read_csv(data_path)
    X = data.drop('target', axis=1)
    y = data['target']
    
    # Split data
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)
    
    # Train model
    with mlflow.start_run():
        model = RandomForestClassifier(n_estimators=100, random_state=42)
        model.fit(X_train, y_train)
        
        # Evaluate
        y_pred = model.predict(X_test)
        accuracy = accuracy_score(y_test, y_pred)
        
        # Log metrics
        mlflow.log_metric("accuracy", accuracy)
        mlflow.sklearn.log_model(model, "model")
        
        # Save model
        models_dir = Path("models")
        models_dir.mkdir(exist_ok=True)
        
        with open(models_dir / f"{model_name}.pkl", 'wb') as f:
            pickle.dump(model, f)
            
        print(f"Model {model_name} trained with accuracy: {accuracy:.4f}")
        print(classification_report(y_test, y_pred))

if __name__ == "__main__":
    import sys
    if len(sys.argv) != 3:
        print("Usage: python train_model.py <data_path> <model_name>")
        sys.exit(1)
        
    train_classification_model(sys.argv[1], sys.argv[2])`,

			"scripts/evaluate_model.py": `#!/usr/bin/env python3
import pickle
import pandas as pd
import numpy as np
from sklearn.metrics import accuracy_score, precision_score, recall_score, f1_score
from pathlib import Path
import json

def evaluate_model(model_path: str, test_data_path: str):
    """Evaluate a trained model"""
    # Load model
    with open(model_path, 'rb') as f:
        model = pickle.load(f)
    
    # Load test data
    test_data = pd.read_csv(test_data_path)
    X_test = test_data.drop('target', axis=1)
    y_test = test_data['target']
    
    # Make predictions
    y_pred = model.predict(X_test)
    
    # Calculate metrics
    metrics = {
        "accuracy": accuracy_score(y_test, y_pred),
        "precision": precision_score(y_test, y_pred, average='weighted'),
        "recall": recall_score(y_test, y_pred, average='weighted'),
        "f1_score": f1_score(y_test, y_pred, average='weighted')
    }
    
    print("Model Evaluation Results:")
    for metric, value in metrics.items():
        print(f"{metric}: {value:.4f}")
    
    return metrics

if __name__ == "__main__":
    import sys
    if len(sys.argv) != 3:
        print("Usage: python evaluate_model.py <model_path> <test_data_path>")
        sys.exit(1)
        
    evaluate_model(sys.argv[1], sys.argv[2])`,

			// Tests
			"tests/__init__.py": "",
			"tests/test_api.py": `import pytest
from fastapi.testclient import TestClient
from src.ml_platform.main import app

client = TestClient(app)

def test_health_endpoint():
    response = client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert "status" in data
    assert data["status"] == "healthy"

def test_predict_endpoint():
    # This would require a trained model to be loaded
    request_data = {
        "data": {"features": [1.0, 2.0, 3.0, 4.0]},
        "model_name": "test_model"
    }
    response = client.post("/predict", json=request_data)
    # Test would depend on whether test model is available
    # assert response.status_code in [200, 500]`,

			"tests/test_ml_service.py": `import pytest
import asyncio
from src.ml_platform.ml_service import MLService

@pytest.mark.asyncio
async def test_load_models():
    await MLService.load_models()
    # Test depends on available models

@pytest.mark.asyncio 
async def test_get_model_status():
    status = await MLService.get_model_status()
    assert isinstance(status, dict)`,

			// Configuration files
			"Dockerfile": `FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \\
    gcc \\
    g++ \\
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY pyproject.toml ./
RUN pip install -e .

# Copy source code
COPY src/ ./src/
COPY models/ ./models/

# Expose port
EXPOSE 8000

# Run application
CMD ["uvicorn", "src.ml_platform.main:app", "--host", "0.0.0.0", "--port", "8000"]`,

			"docker-compose.yml": `version: '3.8'
services:
  ml-platform:
    build: .
    ports:
      - "8000:8000"
    environment:
      - DATABASE_URL=postgresql://user:password@db:5432/ml_platform
      - MLFLOW_TRACKING_URI=http://mlflow:5000
    volumes:
      - ./models:/app/models
    depends_on:
      - db
      - mlflow

  db:
    image: postgres:15
    environment:
      - POSTGRES_DB=ml_platform
      - POSTGRES_USER=user  
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  mlflow:
    image: python:3.11-slim
    command: >
      bash -c "pip install mlflow psycopg2-binary &&
               mlflow server --backend-store-uri postgresql://user:password@db:5432/mlflow 
               --default-artifact-root ./artifacts --host 0.0.0.0 --port 5000"
    ports:
      - "5000:5000"
    depends_on:
      - db

volumes:
  postgres_data:`,

			"Makefile": `# ML Platform Makefile
.PHONY: install test lint format train evaluate serve

install:
	pip install -e .
	pip install -e ".[dev]"

test:
	pytest tests/ -v

lint:
	flake8 src/ tests/
	mypy src/

format:
	black src/ tests/
	isort src/ tests/

train:
	python scripts/train_model.py data/train.csv model_v1

evaluate:
	python scripts/evaluate_model.py models/model_v1.pkl data/test.csv

serve:
	uvicorn src.ml_platform.main:app --reload --host 0.0.0.0 --port 8000

docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down`,

			"README.md": `# ML Platform

A production-ready machine learning platform built with FastAPI, featuring:

## Features

- **RESTful API**: FastAPI-based REST API for model inference
- **Model Management**: MLflow integration for model versioning and tracking
- **Database Integration**: PostgreSQL with SQLAlchemy for logging and metrics
- **Async Support**: Full async/await support for high performance
- **Type Safety**: Pydantic models for request/response validation
- **Testing**: Comprehensive test suite with pytest
- **Docker Support**: Multi-container setup with Docker Compose

## Architecture

- **API Layer**: FastAPI application with automatic OpenAPI documentation
- **ML Service**: Model loading, prediction, and management
- **Database Layer**: Async SQLAlchemy for data persistence
- **Training Pipeline**: Scikit-learn based training with MLflow tracking
- **Evaluation**: Model evaluation and metrics calculation

## Usage

See Makefile for development commands.`,
		}

		projectPath := suite.testHelper.CreateTestProject("python-ml", pythonMLStructure)

		// Detect and analyze project
		analysis, err := suite.testHelper.DetectAndAnalyzeProject(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(analysis)

		// Validate project detection
		suite.Equal("single-language", analysis.ProjectType, "Should detect as single-language Python project")
		suite.Equal("python", analysis.DominantLanguage, "Python should be dominant language")
		suite.Contains(analysis.Languages, "python", "Should detect Python language")

		// Validate Python-specific detection
		suite.True(analysis.HasPyProjectToml, "Should detect pyproject.toml")
		suite.Contains(analysis.BuildSystems, "setuptools", "Should detect setuptools build system")
		suite.Contains(analysis.Frameworks, "fastapi", "Should detect FastAPI framework")

		// Validate ML-specific detection
		suite.Contains(analysis.Dependencies, "scikit-learn", "Should detect scikit-learn")
		suite.Contains(analysis.Dependencies, "tensorflow", "Should detect TensorFlow")
		suite.Contains(analysis.Dependencies, "torch", "Should detect PyTorch")
		suite.Contains(analysis.Dependencies, "mlflow", "Should detect MLflow")

		// Validate project structure
		suite.Contains(analysis.SourceDirs, "src", "Should identify src directory")
		suite.Contains(analysis.TestDirs, "tests", "Should identify tests directory")
		suite.Contains(analysis.ScriptDirs, "scripts", "Should identify scripts directory")

		// Validate containerization detection
		suite.True(analysis.HasDockerfile, "Should detect Dockerfile")
		suite.True(analysis.HasDockerCompose, "Should detect docker-compose.yml")
	})

	suite.Run("DetectFullStackJavaScriptProject", func() {
		// Create full-stack JavaScript/TypeScript project
		fullStackStructure := map[string]string{
			// Root package.json for monorepo
			"package.json": `{
  "name": "fullstack-app",
  "private": true,
  "workspaces": [
    "apps/*",
    "packages/*"
  ],
  "scripts": {
    "build": "turbo run build",
    "dev": "turbo run dev --parallel",
    "lint": "turbo run lint",
    "test": "turbo run test"
  },
  "devDependencies": {
    "turbo": "^1.10.0",
    "eslint": "^8.48.0",
    "prettier": "^3.0.0",
    "typescript": "^5.0.0"
  }
}`,

			"turbo.json": `{
  "$schema": "https://turbo.build/schema.json",
  "pipeline": {
    "build": {
      "dependsOn": ["^build"],
      "outputs": ["dist/**", ".next/**", "build/**"]
    },
    "dev": {
      "cache": false,
      "persistent": true
    },
    "lint": {},
    "test": {
      "dependsOn": ["^build"]
    }
  }
}`,

			// Frontend React app
			"apps/web/package.json": `{
  "name": "web",
  "version": "1.0.0",
  "scripts": {
    "build": "next build",
    "dev": "next dev",
    "start": "next start",
    "lint": "next lint",
    "test": "jest"
  },
  "dependencies": {
    "next": "^13.4.0",
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "typescript": "^5.0.0",
    "@shared/ui": "*",
    "@shared/utils": "*"
  },
  "devDependencies": {
    "eslint": "^8.48.0",
    "eslint-config-next": "^13.4.0",
    "@types/node": "^20.0.0",
    "jest": "^29.0.0",
    "@testing-library/react": "^13.4.0"
  }
}`,

			"apps/web/next.config.js": `/** @type {import('next').NextConfig} */
const nextConfig = {
  experimental: {
    appDir: true,
  },
  transpilePackages: ['@shared/ui', '@shared/utils'],
}

module.exports = nextConfig`,

			"apps/web/tsconfig.json": `{
  "extends": "../../packages/tsconfig/nextjs.json",
  "compilerOptions": {
    "plugins": [{ "name": "next" }]
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx", ".next/types/**/*.ts"],
  "exclude": ["node_modules"]
}`,

			"apps/web/src/app/page.tsx": `'use client'

import { useState, useEffect } from 'react'
import { Button } from '@shared/ui'
import { formatDate } from '@shared/utils'

interface User {
  id: number
  name: string
  email: string
  createdAt: string
}

export default function HomePage() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchUsers()
  }, [])

  const fetchUsers = async () => {
    try {
      const response = await fetch('/api/users')
      const data = await response.json()
      setUsers(data)
    } catch (error) {
      console.error('Failed to fetch users:', error)
    } finally {
      setLoading(false)
    }
  }

  if (loading) return <div>Loading...</div>

  return (
    <div className="container mx-auto p-4">
      <h1 className="text-3xl font-bold mb-6">Full Stack App</h1>
      <div className="grid gap-4">
        {users.map(user => (
          <div key={user.id} className="border p-4 rounded">
            <h2 className="text-xl font-semibold">{user.name}</h2>
            <p className="text-gray-600">{user.email}</p>
            <p className="text-sm text-gray-500">
              Created: {formatDate(user.createdAt)}
            </p>
          </div>
        ))}
      </div>
      <Button onClick={fetchUsers} className="mt-4">
        Refresh Users
      </Button>
    </div>
  )
}`,

			"apps/web/src/app/api/users/route.ts": `import { NextResponse } from 'next/server'

// Mock user data - in real app would come from database
const users = [
  { id: 1, name: 'John Doe', email: 'john@example.com', createdAt: '2023-01-01' },
  { id: 2, name: 'Jane Smith', email: 'jane@example.com', createdAt: '2023-01-02' },
  { id: 3, name: 'Bob Johnson', email: 'bob@example.com', createdAt: '2023-01-03' },
]

export async function GET() {
  // Simulate API delay
  await new Promise(resolve => setTimeout(resolve, 500))
  
  return NextResponse.json(users)
}

export async function POST(request: Request) {
  const body = await request.json()
  
  const newUser = {
    id: users.length + 1,
    name: body.name,
    email: body.email,
    createdAt: new Date().toISOString()
  }
  
  users.push(newUser)
  
  return NextResponse.json(newUser, { status: 201 })
}`,

			// Backend Express API
			"apps/api/package.json": `{
  "name": "api",
  "version": "1.0.0",
  "scripts": {
    "build": "tsc",
    "dev": "nodemon src/index.ts",
    "start": "node dist/index.js",
    "lint": "eslint src/**/*.ts",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.0",
    "cors": "^2.8.5",
    "helmet": "^7.0.0",
    "morgan": "^1.10.0",
    "prisma": "^5.0.0",
    "@prisma/client": "^5.0.0",
    "@shared/utils": "*"
  },
  "devDependencies": {
    "@types/express": "^4.17.0",
    "@types/cors": "^2.8.0",
    "@types/morgan": "^1.9.0",
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0",
    "nodemon": "^3.0.0",
    "ts-node": "^10.9.0",
    "jest": "^29.0.0",
    "@types/jest": "^29.0.0",
    "supertest": "^6.3.0",
    "@types/supertest": "^2.0.0"
  }
}`,

			"apps/api/tsconfig.json": `{
  "extends": "../../packages/tsconfig/base.json",
  "compilerOptions": {
    "outDir": "dist",
    "rootDir": "src"
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}`,

			"apps/api/src/index.ts": `import express from 'express'
import cors from 'cors'
import helmet from 'helmet'
import morgan from 'morgan'
import { userRouter } from './routes/users'
import { errorHandler } from './middleware/errorHandler'
import { validateRequest } from './middleware/validation'

const app = express()
const PORT = process.env.PORT || 3001

// Middleware
app.use(helmet())
app.use(cors())
app.use(morgan('combined'))
app.use(express.json())
app.use(express.urlencoded({ extended: true }))

// Routes
app.use('/api/users', userRouter)

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'healthy', timestamp: new Date().toISOString() })
})

// Error handling
app.use(errorHandler)

app.listen(PORT, () => {
  console.log('API server running on port ' + PORT)
})

export default app`,

			"apps/api/src/routes/users.ts": `import { Router } from 'express'
import { PrismaClient } from '@prisma/client'
import { validateUser } from '../middleware/validation'
import { formatDate } from '@shared/utils'

const router = Router()
const prisma = new PrismaClient()

// GET /api/users
router.get('/', async (req, res, next) => {
  try {
    const users = await prisma.user.findMany({
      select: {
        id: true,
        name: true,
        email: true,
        createdAt: true
      }
    })
    
    const formattedUsers = users.map(user => ({
      ...user,
      createdAt: formatDate(user.createdAt.toISOString())
    }))
    
    res.json(formattedUsers)
  } catch (error) {
    next(error)
  }
})

// POST /api/users
router.post('/', validateUser, async (req, res, next) => {
  try {
    const { name, email } = req.body
    
    const user = await prisma.user.create({
      data: { name, email },
      select: {
        id: true,
        name: true,
        email: true,
        createdAt: true
      }
    })
    
    res.status(201).json({
      ...user,
      createdAt: formatDate(user.createdAt.toISOString())
    })
  } catch (error) {
    next(error)
  }
})

// GET /api/users/:id
router.get('/:id', async (req, res, next) => {
  try {
    const { id } = req.params
    
    const user = await prisma.user.findUnique({
      where: { id: parseInt(id) },
      select: {
        id: true,
        name: true,
        email: true,
        createdAt: true
      }
    })
    
    if (!user) {
      return res.status(404).json({ error: 'User not found' })
    }
    
    res.json({
      ...user,
      createdAt: formatDate(user.createdAt.toISOString())
    })
  } catch (error) {
    next(error)
  }
})

export { router as userRouter }`,

			"apps/api/src/middleware/validation.ts": `import { Request, Response, NextFunction } from 'express'

export const validateUser = (req: Request, res: Response, next: NextFunction) => {
  const { name, email } = req.body
  
  if (!name || typeof name !== 'string' || name.trim().length === 0) {
    return res.status(400).json({ error: 'Name is required and must be a non-empty string' })
  }
  
  if (!email || typeof email !== 'string' || !isValidEmail(email)) {
    return res.status(400).json({ error: 'Valid email is required' })
  }
  
  next()
}

export const validateRequest = (schema: any) => {
  return (req: Request, res: Response, next: NextFunction) => {
    const { error } = schema.validate(req.body)
    if (error) {
      return res.status(400).json({ error: error.details[0].message })
    }
    next()
  }
}

function isValidEmail(email: string): boolean {
  const emailRegex = /^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$/
  return emailRegex.test(email)
}`,

			"apps/api/src/middleware/errorHandler.ts": `import { Request, Response, NextFunction } from 'express'

export const errorHandler = (
  error: Error,
  req: Request,
  res: Response,
  next: NextFunction
) => {
  console.error('Error:', error)
  
  // Prisma errors
  if (error.name === 'PrismaClientKnownRequestError') {
    return res.status(400).json({ error: 'Database error' })
  }
  
  // Validation errors
  if (error.name === 'ValidationError') {
    return res.status(400).json({ error: error.message })
  }
  
  // Default error
  res.status(500).json({ 
    error: 'Internal server error',
    message: process.env.NODE_ENV === 'development' ? error.message : undefined
  })
}`,

			"apps/api/prisma/schema.prisma": `generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

model User {
  id        Int      @id @default(autoincrement())
  name      String
  email     String   @unique
  createdAt DateTime @default(now())
  updatedAt DateTime @updatedAt

  @@map("users")
}`,

			// Shared UI package
			"packages/ui/package.json": `{
  "name": "@shared/ui",
  "version": "1.0.0",
  "main": "index.ts",
  "types": "index.ts",
  "dependencies": {
    "react": "^18.2.0",
    "@types/react": "^18.2.0"
  },
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`,

			"packages/ui/index.ts": `export { Button } from './Button'
export { Input } from './Input'
export { Card } from './Card'`,

			"packages/ui/Button.tsx": `import React from 'react'

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger'
  size?: 'sm' | 'md' | 'lg'
  children: React.ReactNode
}

export const Button: React.FC<ButtonProps> = ({
  variant = 'primary',
  size = 'md',
  className = '',
  children,
  ...props
}) => {
  const baseClasses = 'inline-flex items-center justify-center font-medium rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2'
  
  const variants = {
    primary: 'bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500',
    secondary: 'bg-gray-200 text-gray-900 hover:bg-gray-300 focus:ring-gray-500',
    danger: 'bg-red-600 text-white hover:bg-red-700 focus:ring-red-500'
  }
  
  const sizes = {
    sm: 'px-3 py-2 text-sm',
    md: 'px-4 py-2 text-base',
    lg: 'px-6 py-3 text-lg'
  }
  
  const classes = baseClasses + ' ' + variants[variant] + ' ' + sizes[size] + ' ' + className
  
  return (
    <button className={classes} {...props}>
      {children}
    </button>
  )
}`,

			// Shared utils package
			"packages/utils/package.json": `{
  "name": "@shared/utils",
  "version": "1.0.0",
  "main": "index.ts",
  "types": "index.ts",
  "devDependencies": {
    "typescript": "^5.0.0"
  }
}`,

			"packages/utils/index.ts": `export { formatDate } from './date'
export { validateEmail } from './validation'
export { apiClient } from './api'`,

			"packages/utils/date.ts": `export const formatDate = (dateString: string): string => {
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  })
}

export const formatDateTime = (dateString: string): string => {
  const date = new Date(dateString)
  return date.toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}`,

			// TypeScript config packages
			"packages/tsconfig/package.json": `{
  "name": "@shared/tsconfig",
  "version": "1.0.0",
  "files": ["*.json"]
}`,

			"packages/tsconfig/base.json": `{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020"],
    "module": "CommonJS",
    "moduleResolution": "node",
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  }
}`,

			"packages/tsconfig/nextjs.json": `{
  "extends": "./base.json",
  "compilerOptions": {
    "target": "ES5",
    "lib": ["DOM", "DOM.Iterable", "ES6"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "preserve",
    "incremental": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "allowJs": true,
    "plugins": [{ "name": "next" }]
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx"],
  "exclude": ["node_modules"]
}`,

			// Root configuration files
			"docker-compose.yml": `version: '3.8'
services:
  web:
    build:
      context: .
      dockerfile: apps/web/Dockerfile
    ports:
      - "3000:3000"
    environment:
      - NODE_ENV=development
    depends_on:
      - api

  api:
    build:
      context: .
      dockerfile: apps/api/Dockerfile
    ports:
      - "3001:3001"
    environment:
      - NODE_ENV=development
      - DATABASE_URL=postgresql://user:password@db:5432/fullstackapp
    depends_on:
      - db

  db:
    image: postgres:15
    environment:
      - POSTGRES_DB=fullstackapp
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

volumes:
  postgres_data:`,

			"README.md": `# Full Stack TypeScript Application

A modern full-stack application built with:

## Frontend (Next.js 13+)
- **Framework**: Next.js with App Router
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **Testing**: Jest + React Testing Library

## Backend (Express)
- **Framework**: Express.js
- **Language**: TypeScript  
- **Database**: PostgreSQL with Prisma ORM
- **Testing**: Jest + Supertest

## Shared Packages
- **@shared/ui**: React component library
- **@shared/utils**: Common utilities
- **@shared/tsconfig**: Shared TypeScript configurations

## Architecture
- **Monorepo**: Turborepo for build orchestration
- **Package Management**: npm workspaces
- **Database**: PostgreSQL with Prisma
- **Containerization**: Docker Compose for local development

## Development
` + "```bash" + `
npm install
npm run dev
` + "```" + ``,
		}

		projectPath := suite.testHelper.CreateTestProject("fullstack-js", fullStackStructure)

		// Detect and analyze project
		analysis, err := suite.testHelper.DetectAndAnalyzeProject(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(analysis)

		// Validate project detection
		suite.Equal("monorepo", analysis.ProjectType, "Should detect as monorepo")
		suite.Equal("typescript", analysis.DominantLanguage, "TypeScript should be dominant language")

		// Validate languages
		suite.Contains(analysis.Languages, "typescript", "Should detect TypeScript")
		suite.Contains(analysis.Languages, "javascript", "Should detect JavaScript")

		// Validate monorepo detection
		suite.True(analysis.IsMonorepo, "Should detect as monorepo")
		suite.True(analysis.HasWorkspaces, "Should detect npm workspaces")
		suite.Contains(analysis.BuildSystems, "turbo", "Should detect Turborepo")

		// Validate framework detection
		suite.Contains(analysis.Frameworks, "next.js", "Should detect Next.js")
		suite.Contains(analysis.Frameworks, "express", "Should detect Express")
		suite.Contains(analysis.Frameworks, "prisma", "Should detect Prisma ORM")

		// Validate project structure
		suite.GreaterOrEqual(len(analysis.Apps), 2, "Should detect multiple apps")
		suite.Contains(analysis.Apps, "web", "Should detect web app")
		suite.Contains(analysis.Apps, "api", "Should detect API app")

		suite.GreaterOrEqual(len(analysis.Packages), 3, "Should detect shared packages")
		suite.Contains(analysis.Packages, "ui", "Should detect UI package")
		suite.Contains(analysis.Packages, "utils", "Should detect utils package")
	})
}

// TestTemplateSelectionAndApplication tests template selection and application logic
func (suite *AutoGenerationTestSuite) TestTemplateSelectionAndApplication() {
	suite.Run("SelectMonorepoTemplate", func() {
		// Create monorepo project structure
		monorepoStructure := map[string]string{
			"go.work":        "go 1.21\n\nuse (\n\t./auth\n\t./api\n\t./worker\n)",
			"auth/go.mod":    "module auth\n\ngo 1.21",
			"auth/main.go":   "package main\n\nfunc main() {}",
			"api/go.mod":     "module api\n\ngo 1.21",
			"api/main.go":    "package main\n\nfunc main() {}",
			"worker/go.mod":  "module worker\n\ngo 1.21",
			"worker/main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("template-monorepo", monorepoStructure)

		// Test template selection
		selectedTemplate, err := suite.testHelper.SelectTemplate(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(selectedTemplate)

		// Validate monorepo template selected
		suite.Equal("monorepo", selectedTemplate.Type, "Should select monorepo template")
		suite.Contains(selectedTemplate.Languages, "go", "Template should support Go")
		suite.Contains(selectedTemplate.ProjectTypes, "monorepo", "Template should support monorepo type")

		// Apply template
		generatedConfig, err := suite.testHelper.ApplyTemplate(selectedTemplate, projectPath)
		suite.Require().NoError(err)
		suite.NotNil(generatedConfig)

		// Validate generated configuration
		suite.Equal("monorepo", generatedConfig.ProjectInfo.ProjectType, "Generated config should be monorepo type")
		suite.GreaterOrEqual(len(generatedConfig.ServerConfigs), 1, "Should have LSP servers configured")

		// Validate Go-specific configuration
		hasGoServer := false
		for _, server := range generatedConfig.ServerConfigs {
			if exactMatchString(server.Languages, "go") {
				hasGoServer = true
				suite.Equal("gopls", server.Command, "Should configure gopls for Go")
				break
			}
		}
		suite.True(hasGoServer, "Should have Go LSP server configured")
	})

	suite.Run("SelectMicroservicesTemplate", func() {
		// Create microservices project structure
		microservicesStructure := map[string]string{
			"docker-compose.yml": "version: '3.8'\nservices:\n  auth:\n    build: ./auth\n  user:\n    build: ./user\n  api:\n    build: ./api-gateway",

			"auth/go.mod":     "module auth-service\n\ngo 1.21",
			"auth/main.go":    "package main\n\nfunc main() {}",
			"auth/Dockerfile": "FROM golang:1.21\nWORKDIR /app\nCOPY . .\nRUN go build -o auth .\nCMD [\"./auth\"]",

			"user/pom.xml":                 `<?xml version="1.0"?><project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>user-service</artifactId><version>1.0.0</version></project>`,
			"user/src/main/java/Main.java": "public class Main { public static void main(String[] args) {} }",
			"user/Dockerfile":              "FROM openjdk:17\nWORKDIR /app\nCOPY target/*.jar app.jar\nCMD [\"java\", \"-jar\", \"app.jar\"]",

			"api-gateway/package.json": `{"name": "api-gateway", "dependencies": {"express": "^4.18.0"}}`,
			"api-gateway/server.js":    "const express = require('express'); const app = express(); app.listen(3000);",
			"api-gateway/Dockerfile":   "FROM node:18\nWORKDIR /app\nCOPY package*.json ./\nRUN npm install\nCOPY . .\nCMD [\"node\", \"server.js\"]",
		}

		projectPath := suite.testHelper.CreateTestProject("template-microservices", microservicesStructure)

		// Test template selection
		selectedTemplate, err := suite.testHelper.SelectTemplate(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(selectedTemplate)

		// Validate microservices template selected
		suite.Equal("microservices", selectedTemplate.Type, "Should select microservices template")
		suite.Contains(selectedTemplate.Languages, "go", "Template should support Go")
		suite.Contains(selectedTemplate.Languages, "java", "Template should support Java")
		suite.Contains(selectedTemplate.Languages, "javascript", "Template should support JavaScript")

		// Apply template
		generatedConfig, err := suite.testHelper.ApplyTemplate(selectedTemplate, projectPath)
		suite.Require().NoError(err)
		suite.NotNil(generatedConfig)

		// Validate multi-language configuration
		suite.Equal("microservices", generatedConfig.ProjectInfo.ProjectType, "Should be microservices type")
		suite.GreaterOrEqual(len(generatedConfig.ServerConfigs), 3, "Should have servers for multiple languages")

		// Validate each language has appropriate server
		languageServers := make(map[string]bool)
		for _, server := range generatedConfig.ServerConfigs {
			for _, lang := range server.Languages {
				languageServers[lang] = true
			}
		}

		suite.True(languageServers["go"], "Should have Go server")
		suite.True(languageServers["java"], "Should have Java server")
		suite.True(languageServers["javascript"], "Should have JavaScript server")
	})

	suite.Run("SelectFullStackTemplate", func() {
		// Create full-stack project structure
		fullStackStructure := map[string]string{
			"frontend/package.json":  `{"name": "frontend", "dependencies": {"react": "^18.0.0", "typescript": "^5.0.0"}}`,
			"frontend/tsconfig.json": `{"compilerOptions": {"target": "ES2020", "jsx": "react-jsx"}}`,
			"frontend/src/App.tsx":   "import React from 'react';\n\nconst App = () => <div>App</div>;\n\nexport default App;",

			"backend/go.mod":  "module backend\n\ngo 1.21",
			"backend/main.go": "package main\n\nfunc main() {}",

			"shared/types.ts": "export interface User { id: number; name: string; }",
		}

		projectPath := suite.testHelper.CreateTestProject("template-fullstack", fullStackStructure)

		// Test template selection
		selectedTemplate, err := suite.testHelper.SelectTemplate(projectPath)
		suite.Require().NoError(err)
		suite.NotNil(selectedTemplate)

		// Validate full-stack template selected
		suite.Equal("full-stack", selectedTemplate.Type, "Should select full-stack template")
		suite.Contains(selectedTemplate.Languages, "typescript", "Template should support TypeScript")
		suite.Contains(selectedTemplate.Languages, "go", "Template should support Go")

		// Apply template
		generatedConfig, err := suite.testHelper.ApplyTemplate(selectedTemplate, projectPath)
		suite.Require().NoError(err)
		suite.NotNil(generatedConfig)

		// Validate frontend-backend configuration
		suite.Equal("frontend-backend", generatedConfig.ProjectInfo.ProjectType, "Should be frontend-backend type")

		// Should have optimizations for full-stack development
		suite.NotNil(generatedConfig.WorkspaceConfig, "Should have workspace config")
		suite.NotEmpty(generatedConfig.ServerConfigs, "Should have multi-language server configs")
	})
}

// TestOptimizationModeApplication tests application of different optimization modes
func (suite *AutoGenerationTestSuite) TestOptimizationModeApplication() {
	suite.Run("ApplyDevelopmentOptimization", func() {
		// Create simple project
		projectStructure := map[string]string{
			"go.mod":  "module test-project\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("dev-optimization", projectStructure)

		// Generate config with development optimization
		config, err := suite.testHelper.GenerateConfigWithOptimization(projectPath, "development")
		suite.Require().NoError(err)
		suite.NotNil(config)

		// Validate development optimizations
		suite.Equal("development", config.OptimizedFor)
		suite.NotNil(config.ProjectInfo, "Should have project info")
		suite.Equal("3.0", config.Version, "Should have correct version")
		suite.NotEmpty(config.ServerConfigs, "Should have server configurations")
	})

	suite.Run("ApplyProductionOptimization", func() {
		// Create project for production optimization
		projectStructure := map[string]string{
			"go.mod":  "module prod-project\n\ngo 1.21",
			"main.go": "package main\n\nfunc main() {}",
		}

		projectPath := suite.testHelper.CreateTestProject("prod-optimization", projectStructure)

		// Generate config with production optimization
		config, err := suite.testHelper.GenerateConfigWithOptimization(projectPath, "production")
		suite.Require().NoError(err)
		suite.NotNil(config)

		// Validate production optimizations
		suite.Equal("production", config.OptimizedFor)
		suite.NotNil(config.ProjectInfo, "Should have project info")
		suite.Equal("3.0", config.Version, "Should have correct version")
		suite.NotEmpty(config.ServerConfigs, "Should have server configurations")

		// Should have workspace configuration for production
		suite.NotNil(config.WorkspaceConfig, "Should have workspace config for production")
	})

	suite.Run("ApplyLargeProjectOptimization", func() {
		// Create large project structure
		largeProjectStructure := make(map[string]string)

		// Generate many files to simulate large project
		largeProjectStructure["go.mod"] = "module large-project\n\ngo 1.21"
		for i := 0; i < 100; i++ {
			largeProjectStructure[fmt.Sprintf("service%d/main.go", i)] = fmt.Sprintf("package main\n\nfunc main%d() {}", i)
			largeProjectStructure[fmt.Sprintf("service%d/handler.go", i)] = fmt.Sprintf("package main\n\ntype Handler%d struct{}", i)
		}

		projectPath := suite.testHelper.CreateTestProject("large-optimization", largeProjectStructure)

		// Generate config with large-project optimization
		config, err := suite.testHelper.GenerateConfigWithOptimization(projectPath, "large-project")
		suite.Require().NoError(err)
		suite.NotNil(config)

		// Validate large project optimizations
		suite.Equal("large-project", config.OptimizedFor)
		suite.NotNil(config.ProjectInfo, "Should have project info")
		suite.NotEmpty(config.ServerConfigs, "Should have server configurations")
		suite.NotNil(config.WorkspaceConfig, "Should have workspace config")
		suite.NotNil(config.Metadata, "Should have metadata for large project settings")
		suite.GreaterOrEqual(len(config.ServerConfigs), 1, "Should have at least one server configured")
	})
}

// Helper function to check if slice contains exact string match
func exactMatchString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Run the test suite
func TestAutoGenerationTestSuite(t *testing.T) {
	suite.Run(t, new(AutoGenerationTestSuite))
}
