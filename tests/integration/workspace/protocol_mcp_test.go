package workspace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/workspace"
	"lsp-gateway/mcp"
)

// MCPIntegrationTestSuite tests MCP integration with workspace context switching
type MCPIntegrationTestSuite struct {
	suite.Suite
	tempDir         string
	workspaceRoot   string
	mcpServer       *mcp.Server
	toolHandler     *mcp.ToolHandler
	workspaceConfig *workspace.WorkspaceConfig
	ctx             context.Context
	cancel          context.CancelFunc
}

func TestMCPIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(MCPIntegrationTestSuite))
}

func (suite *MCPIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	
	var err error
	suite.tempDir, err = os.MkdirTemp("", "mcp-integration-test-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	suite.workspaceRoot = filepath.Join(suite.tempDir, "workspace")
	err = os.MkdirAll(suite.workspaceRoot, 0755)
	suite.Require().NoError(err)
	
	suite.setupMultiProjectWorkspace()
	suite.setupMCPServer()
}

func (suite *MCPIntegrationTestSuite) TearDownSuite() {
	if suite.mcpServer != nil {
		suite.mcpServer.Stop()
	}
	suite.cancel()
	os.RemoveAll(suite.tempDir)
}

func (suite *MCPIntegrationTestSuite) SetupTest() {
	// Reset MCP server state for each test
	if suite.mcpServer != nil {
		suite.mcpServer.Stop()
	}
	suite.setupMCPServer()
}

func (suite *MCPIntegrationTestSuite) TearDownTest() {
	if suite.mcpServer != nil {
		suite.mcpServer.Stop()
	}
}

func (suite *MCPIntegrationTestSuite) setupMultiProjectWorkspace() {
	// Create Go sub-project
	goProject := filepath.Join(suite.workspaceRoot, "go-api")
	suite.Require().NoError(os.MkdirAll(goProject, 0755))
	
	goMod := `module go-api

go 1.21

require (
	github.com/gorilla/mux v1.8.1
	github.com/rs/cors v1.10.1
)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "go.mod"), []byte(goMod), 0644))
	
	mainGo := `package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// Product represents a product in the API
type Product struct {
	ID          int     ` + "`json:\"id\"`" + `
	Name        string  ` + "`json:\"name\"`" + `
	Description string  ` + "`json:\"description\"`" + `
	Price       float64 ` + "`json:\"price\"`" + `
	Category    string  ` + "`json:\"category\"`" + `
}

// ProductService handles product operations
type ProductService struct {
	products map[int]*Product
	nextID   int
}

// NewProductService creates a new product service
func NewProductService() *ProductService {
	return &ProductService{
		products: make(map[int]*Product),
		nextID:   1,
	}
}

// CreateProduct adds a new product
func (ps *ProductService) CreateProduct(product *Product) *Product {
	product.ID = ps.nextID
	ps.nextID++
	ps.products[product.ID] = product
	return product
}

// GetProduct retrieves a product by ID
func (ps *ProductService) GetProduct(id int) (*Product, bool) {
	product, exists := ps.products[id]
	return product, exists
}

// GetAllProducts returns all products
func (ps *ProductService) GetAllProducts() []*Product {
	products := make([]*Product, 0, len(ps.products))
	for _, product := range ps.products {
		products = append(products, product)
	}
	return products
}

// UpdateProduct updates an existing product
func (ps *ProductService) UpdateProduct(id int, updates *Product) (*Product, bool) {
	if product, exists := ps.products[id]; exists {
		if updates.Name != "" {
			product.Name = updates.Name
		}
		if updates.Description != "" {
			product.Description = updates.Description
		}
		if updates.Price > 0 {
			product.Price = updates.Price
		}
		if updates.Category != "" {
			product.Category = updates.Category
		}
		return product, true
	}
	return nil, false
}

// DeleteProduct removes a product
func (ps *ProductService) DeleteProduct(id int) bool {
	if _, exists := ps.products[id]; exists {
		delete(ps.products, id)
		return true
	}
	return false
}

// APIHandler handles HTTP requests
type APIHandler struct {
	productService *ProductService
}

// NewAPIHandler creates a new API handler
func NewAPIHandler() *APIHandler {
	return &APIHandler{
		productService: NewProductService(),
	}
}

func (h *APIHandler) createProductHandler(w http.ResponseWriter, r *http.Request) {
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	createdProduct := h.productService.CreateProduct(&product)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdProduct)
}

func (h *APIHandler) getProductHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	product, exists := h.productService.GetProduct(id)
	if !exists {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

func (h *APIHandler) getAllProductsHandler(w http.ResponseWriter, r *http.Request) {
	products := h.productService.GetAllProducts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func (h *APIHandler) updateProductHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var updates Product
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	product, exists := h.productService.UpdateProduct(id, &updates)
	if !exists {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

func (h *APIHandler) deleteProductHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	if !h.productService.DeleteProduct(id) {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	handler := NewAPIHandler()
	
	// Seed with sample data
	handler.productService.CreateProduct(&Product{
		Name:        "Laptop",
		Description: "High-performance laptop",
		Price:       999.99,
		Category:    "Electronics",
	})
	
	handler.productService.CreateProduct(&Product{
		Name:        "Coffee Mug",
		Description: "Ceramic coffee mug",
		Price:       12.99,
		Category:    "Kitchen",
	})

	router := mux.NewRouter()
	
	// Product routes
	router.HandleFunc("/api/products", handler.createProductHandler).Methods("POST")
	router.HandleFunc("/api/products", handler.getAllProductsHandler).Methods("GET")
	router.HandleFunc("/api/products/{id}", handler.getProductHandler).Methods("GET")
	router.HandleFunc("/api/products/{id}", handler.updateProductHandler).Methods("PUT")
	router.HandleFunc("/api/products/{id}", handler.deleteProductHandler).Methods("DELETE")
	
	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Enable CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})

	handler_with_cors := c.Handler(router)

	log.Println("Server starting on :8081")
	log.Fatal(http.ListenAndServe(":8081", handler_with_cors))
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "main.go"), []byte(mainGo), 0644))
	
	// Create Python sub-project
	pythonProject := filepath.Join(suite.workspaceRoot, "python-ml")
	suite.Require().NoError(os.MkdirAll(pythonProject, 0755))
	
	requirementsTxt := `numpy==1.24.3
pandas==2.0.3
scikit-learn==1.3.0
fastapi==0.104.1
uvicorn==0.24.0
pydantic==2.5.0
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "requirements.txt"), []byte(requirementsTxt), 0644))
	
	mlPy := `"""Machine Learning API for data processing and predictions."""

import numpy as np
import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import accuracy_score, classification_report
from sklearn.preprocessing import StandardScaler
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Dict, Any, Optional
import uvicorn
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="ML API", version="1.0.0", description="Machine Learning API")

class DataPoint(BaseModel):
    """Individual data point for prediction."""
    features: List[float]
    target: Optional[int] = None

class DataSet(BaseModel):
    """Dataset for training."""
    data: List[DataPoint]
    feature_names: List[str]
    target_names: List[str]

class PredictionRequest(BaseModel):
    """Request for making predictions."""
    features: List[List[float]]

class PredictionResponse(BaseModel):
    """Response containing predictions."""
    predictions: List[int]
    probabilities: List[List[float]]
    confidence_scores: List[float]

class ModelMetrics(BaseModel):
    """Model performance metrics."""
    accuracy: float
    classification_report: Dict[str, Any]
    feature_importance: Dict[str, float]

class MLService:
    """Machine learning service for model training and prediction."""
    
    def __init__(self):
        self.model: Optional[RandomForestClassifier] = None
        self.scaler: Optional[StandardScaler] = None
        self.feature_names: List[str] = []
        self.target_names: List[str] = []
        self.is_trained: bool = False
        
    def prepare_data(self, dataset: DataSet) -> tuple:
        """Prepare data for training."""
        logger.info("Preparing data for training")
        
        # Extract features and targets
        X = []
        y = []
        
        for point in dataset.data:
            if point.target is not None:
                X.append(point.features)
                y.append(point.target)
        
        if not X or not y:
            raise ValueError("Dataset must contain features and targets")
        
        X = np.array(X)
        y = np.array(y)
        
        # Store metadata
        self.feature_names = dataset.feature_names
        self.target_names = dataset.target_names
        
        # Scale features
        self.scaler = StandardScaler()
        X_scaled = self.scaler.fit_transform(X)
        
        return X_scaled, y
    
    def train_model(self, dataset: DataSet) -> ModelMetrics:
        """Train the machine learning model."""
        logger.info("Training machine learning model")
        
        X, y = self.prepare_data(dataset)
        
        # Split data
        X_train, X_test, y_train, y_test = train_test_split(
            X, y, test_size=0.2, random_state=42, stratify=y
        )
        
        # Train model
        self.model = RandomForestClassifier(
            n_estimators=100,
            random_state=42,
            max_depth=10,
            min_samples_split=5
        )
        self.model.fit(X_train, y_train)
        self.is_trained = True
        
        # Evaluate model
        y_pred = self.model.predict(X_test)
        accuracy = accuracy_score(y_test, y_pred)
        report = classification_report(y_test, y_pred, output_dict=True)
        
        # Feature importance
        importance_dict = {}
        if len(self.feature_names) == len(self.model.feature_importances_):
            importance_dict = dict(zip(
                self.feature_names, 
                self.model.feature_importances_.tolist()
            ))
        
        logger.info(f"Model trained with accuracy: {accuracy:.4f}")
        
        return ModelMetrics(
            accuracy=accuracy,
            classification_report=report,
            feature_importance=importance_dict
        )
    
    def predict(self, features: List[List[float]]) -> PredictionResponse:
        """Make predictions using the trained model."""
        if not self.is_trained or self.model is None or self.scaler is None:
            raise ValueError("Model must be trained before making predictions")
        
        logger.info(f"Making predictions for {len(features)} samples")
        
        # Scale features
        X = np.array(features)
        X_scaled = self.scaler.transform(X)
        
        # Make predictions
        predictions = self.model.predict(X_scaled).tolist()
        probabilities = self.model.predict_proba(X_scaled).tolist()
        
        # Calculate confidence scores (max probability for each prediction)
        confidence_scores = [max(probs) for probs in probabilities]
        
        return PredictionResponse(
            predictions=predictions,
            probabilities=probabilities,
            confidence_scores=confidence_scores
        )
    
    def get_model_info(self) -> Dict[str, Any]:
        """Get information about the current model."""
        return {
            "is_trained": self.is_trained,
            "feature_names": self.feature_names,
            "target_names": self.target_names,
            "n_features": len(self.feature_names),
            "n_classes": len(self.target_names) if self.target_names else 0,
            "model_type": "RandomForestClassifier" if self.model else None
        }

# Global ML service instance
ml_service = MLService()

@app.get("/")
async def root():
    """Root endpoint."""
    return {"message": "ML API", "version": "1.0.0"}

@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy", "model_trained": ml_service.is_trained}

@app.post("/train")
async def train_model(dataset: DataSet):
    """Train the machine learning model."""
    try:
        metrics = ml_service.train_model(dataset)
        return {
            "message": "Model trained successfully",
            "metrics": metrics
        }
    except Exception as e:
        logger.error(f"Training failed: {str(e)}")
        raise HTTPException(status_code=400, detail=str(e))

@app.post("/predict", response_model=PredictionResponse)
async def make_predictions(request: PredictionRequest):
    """Make predictions using the trained model."""
    try:
        return ml_service.predict(request.features)
    except Exception as e:
        logger.error(f"Prediction failed: {str(e)}")
        raise HTTPException(status_code=400, detail=str(e))

@app.get("/model/info")
async def get_model_info():
    """Get information about the current model."""
    return ml_service.get_model_info()

@app.post("/data/generate")
async def generate_sample_data(n_samples: int = 100, n_features: int = 4):
    """Generate sample data for testing."""
    try:
        # Generate synthetic data
        np.random.seed(42)
        X = np.random.randn(n_samples, n_features)
        
        # Create simple classification targets based on feature combinations
        y = ((X[:, 0] + X[:, 1]) > 0).astype(int)
        
        # Create dataset
        data_points = []
        for i in range(n_samples):
            data_points.append(DataPoint(
                features=X[i].tolist(),
                target=int(y[i])
            ))
        
        dataset = DataSet(
            data=data_points,
            feature_names=[f"feature_{i}" for i in range(n_features)],
            target_names=["class_0", "class_1"]
        )
        
        return {
            "message": f"Generated {n_samples} samples with {n_features} features",
            "dataset": dataset
        }
    except Exception as e:
        logger.error(f"Data generation failed: {str(e)}")
        raise HTTPException(status_code=400, detail=str(e))

def create_sample_dataset() -> DataSet:
    """Create a sample dataset for testing."""
    np.random.seed(42)
    n_samples = 200
    n_features = 4
    
    X = np.random.randn(n_samples, n_features)
    y = ((X[:, 0] + X[:, 1]) > 0).astype(int)
    
    data_points = []
    for i in range(n_samples):
        data_points.append(DataPoint(
            features=X[i].tolist(),
            target=int(y[i])
        ))
    
    return DataSet(
        data=data_points,
        feature_names=["feature_0", "feature_1", "feature_2", "feature_3"],
        target_names=["negative", "positive"]
    )

if __name__ == "__main__":
    # Train model with sample data on startup
    logger.info("Training model with sample data...")
    sample_data = create_sample_dataset()
    try:
        metrics = ml_service.train_model(sample_data)
        logger.info(f"Model trained with accuracy: {metrics.accuracy:.4f}")
    except Exception as e:
        logger.error(f"Failed to train initial model: {e}")
    
    uvicorn.run(app, host="0.0.0.0", port=8082)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "ml_api.py"), []byte(mlPy), 0644))
	
	// Create JavaScript sub-project
	jsProject := filepath.Join(suite.workspaceRoot, "js-client")
	suite.Require().NoError(os.MkdirAll(jsProject, 0755))
	
	packageJson := `{
  "name": "js-client",
  "version": "1.0.0",
  "description": "JavaScript client for API integration",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "test": "jest",
    "dev": "nodemon index.js"
  },
  "dependencies": {
    "axios": "^1.6.0",
    "express": "^4.18.2",
    "ws": "^8.14.0",
    "dotenv": "^16.3.0"
  },
  "devDependencies": {
    "jest": "^29.7.0",
    "nodemon": "^3.0.0"
  }
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(jsProject, "package.json"), []byte(packageJson), 0644))
	
	indexJs := `const express = require('express');
const axios = require('axios');
const WebSocket = require('ws');
require('dotenv').config();

/**
 * API Client for integrating with Go and Python services
 */
class APIClient {
    constructor(goApiUrl, pythonApiUrl) {
        this.goApiUrl = goApiUrl || 'http://localhost:8081';
        this.pythonApiUrl = pythonApiUrl || 'http://localhost:8082';
        this.httpClient = axios.create({
            timeout: 30000,
            headers: {
                'Content-Type': 'application/json'
            }
        });
    }

    /**
     * Get all products from Go API
     */
    async getAllProducts() {
        try {
            const response = await this.httpClient.get(` + "`${this.goApiUrl}/api/products`" + `);
            return response.data;
        } catch (error) {
            console.error('Error fetching products:', error.message);
            throw new Error(` + "`Failed to fetch products: ${error.message}`" + `);
        }
    }

    /**
     * Create a new product in Go API
     */
    async createProduct(productData) {
        try {
            const response = await this.httpClient.post(
                ` + "`${this.goApiUrl}/api/products`" + `,
                productData
            );
            return response.data;
        } catch (error) {
            console.error('Error creating product:', error.message);
            throw new Error(` + "`Failed to create product: ${error.message}`" + `);
        }
    }

    /**
     * Get product by ID from Go API
     */
    async getProduct(productId) {
        try {
            const response = await this.httpClient.get(
                ` + "`${this.goApiUrl}/api/products/${productId}`" + `
            );
            return response.data;
        } catch (error) {
            if (error.response && error.response.status === 404) {
                return null;
            }
            console.error('Error fetching product:', error.message);
            throw new Error(` + "`Failed to fetch product: ${error.message}`" + `);
        }
    }

    /**
     * Update product in Go API
     */
    async updateProduct(productId, updates) {
        try {
            const response = await this.httpClient.put(
                ` + "`${this.goApiUrl}/api/products/${productId}`" + `,
                updates
            );
            return response.data;
        } catch (error) {
            console.error('Error updating product:', error.message);
            throw new Error(` + "`Failed to update product: ${error.message}`" + `);
        }
    }

    /**
     * Delete product from Go API
     */
    async deleteProduct(productId) {
        try {
            await this.httpClient.delete(` + "`${this.goApiUrl}/api/products/${productId}`" + `);
            return true;
        } catch (error) {
            if (error.response && error.response.status === 404) {
                return false;
            }
            console.error('Error deleting product:', error.message);
            throw new Error(` + "`Failed to delete product: ${error.message}`" + `);
        }
    }

    /**
     * Train ML model using Python API
     */
    async trainModel(trainingData) {
        try {
            const response = await this.httpClient.post(
                ` + "`${this.pythonApiUrl}/train`" + `,
                trainingData
            );
            return response.data;
        } catch (error) {
            console.error('Error training model:', error.message);
            throw new Error(` + "`Failed to train model: ${error.message}`" + `);
        }
    }

    /**
     * Make predictions using Python ML API
     */
    async makePredictions(features) {
        try {
            const response = await this.httpClient.post(
                ` + "`${this.pythonApiUrl}/predict`" + `,
                { features }
            );
            return response.data;
        } catch (error) {
            console.error('Error making predictions:', error.message);
            throw new Error(` + "`Failed to make predictions: ${error.message}`" + `);
        }
    }

    /**
     * Get ML model information
     */
    async getModelInfo() {
        try {
            const response = await this.httpClient.get(` + "`${this.pythonApiUrl}/model/info`" + `);
            return response.data;
        } catch (error) {
            console.error('Error getting model info:', error.message);
            throw new Error(` + "`Failed to get model info: ${error.message}`" + `);
        }
    }

    /**
     * Generate sample data using Python API
     */
    async generateSampleData(nSamples = 100, nFeatures = 4) {
        try {
            const response = await this.httpClient.post(
                ` + "`${this.pythonApiUrl}/data/generate?n_samples=${nSamples}&n_features=${nFeatures}`" + `
            );
            return response.data;
        } catch (error) {
            console.error('Error generating sample data:', error.message);
            throw new Error(` + "`Failed to generate sample data: ${error.message}`" + `);
        }
    }

    /**
     * Check health of all services
     */
    async checkHealth() {
        const healthResults = {};

        try {
            const goHealth = await this.httpClient.get(` + "`${this.goApiUrl}/health`" + `);
            healthResults.goApi = {
                status: 'healthy',
                response: goHealth.data
            };
        } catch (error) {
            healthResults.goApi = {
                status: 'unhealthy',
                error: error.message
            };
        }

        try {
            const pythonHealth = await this.httpClient.get(` + "`${this.pythonApiUrl}/health`" + `);
            healthResults.pythonApi = {
                status: 'healthy',
                response: pythonHealth.data
            };
        } catch (error) {
            healthResults.pythonApi = {
                status: 'unhealthy',
                error: error.message
            };
        }

        return healthResults;
    }
}

/**
 * WebSocket Client for real-time communication
 */
class WebSocketClient {
    constructor(url) {
        this.url = url;
        this.ws = null;
        this.isConnected = false;
        this.messageHandlers = new Map();
    }

    connect() {
        return new Promise((resolve, reject) => {
            try {
                this.ws = new WebSocket(this.url);

                this.ws.on('open', () => {
                    console.log('WebSocket connected');
                    this.isConnected = true;
                    resolve();
                });

                this.ws.on('message', (data) => {
                    try {
                        const message = JSON.parse(data.toString());
                        this.handleMessage(message);
                    } catch (error) {
                        console.error('Error parsing WebSocket message:', error);
                    }
                });

                this.ws.on('close', () => {
                    console.log('WebSocket disconnected');
                    this.isConnected = false;
                });

                this.ws.on('error', (error) => {
                    console.error('WebSocket error:', error);
                    reject(error);
                });
            } catch (error) {
                reject(error);
            }
        });
    }

    handleMessage(message) {
        const handler = this.messageHandlers.get(message.type);
        if (handler) {
            handler(message.data);
        } else {
            console.log('Unhandled message type:', message.type);
        }
    }

    onMessage(type, handler) {
        this.messageHandlers.set(type, handler);
    }

    send(type, data) {
        if (this.isConnected && this.ws) {
            const message = { type, data, timestamp: new Date().toISOString() };
            this.ws.send(JSON.stringify(message));
        } else {
            throw new Error('WebSocket not connected');
        }
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
        }
    }
}

/**
 * Express server for the client application
 */
class ClientServer {
    constructor(port = 3000) {
        this.app = express();
        this.port = port;
        this.apiClient = new APIClient();
        this.setupMiddleware();
        this.setupRoutes();
    }

    setupMiddleware() {
        this.app.use(express.json());
        this.app.use(express.static('public'));
    }

    setupRoutes() {
        // Health check
        this.app.get('/health', (req, res) => {
            res.json({ status: 'healthy', timestamp: new Date().toISOString() });
        });

        // Proxy routes to Go API
        this.app.get('/api/products', async (req, res) => {
            try {
                const products = await this.apiClient.getAllProducts();
                res.json(products);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });

        this.app.post('/api/products', async (req, res) => {
            try {
                const product = await this.apiClient.createProduct(req.body);
                res.status(201).json(product);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });

        this.app.get('/api/products/:id', async (req, res) => {
            try {
                const product = await this.apiClient.getProduct(req.params.id);
                if (!product) {
                    return res.status(404).json({ error: 'Product not found' });
                }
                res.json(product);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });

        // ML API routes
        this.app.post('/api/ml/train', async (req, res) => {
            try {
                const result = await this.apiClient.trainModel(req.body);
                res.json(result);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });

        this.app.post('/api/ml/predict', async (req, res) => {
            try {
                const predictions = await this.apiClient.makePredictions(req.body.features);
                res.json(predictions);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });

        this.app.get('/api/ml/model/info', async (req, res) => {
            try {
                const info = await this.apiClient.getModelInfo();
                res.json(info);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });

        // Health check for all services
        this.app.get('/api/health/all', async (req, res) => {
            try {
                const health = await this.apiClient.checkHealth();
                res.json(health);
            } catch (error) {
                res.status(500).json({ error: error.message });
            }
        });

        // Dashboard route
        this.app.get('/', (req, res) => {
            res.send(` + "`" + `
                <html>
                <head><title>JS Client Dashboard</title></head>
                <body>
                    <h1>Multi-Service API Client</h1>
                    <p>JavaScript client for Go and Python APIs</p>
                    <ul>
                        <li><a href="/api/products">Products API</a></li>
                        <li><a href="/api/ml/model/info">ML Model Info</a></li>
                        <li><a href="/api/health/all">Health Check</a></li>
                    </ul>
                </body>
                </html>
            ` + "`" + `);
        });
    }

    start() {
        this.app.listen(this.port, () => {
            console.log(` + "`Client server running on port ${this.port}`" + `);
        });
    }
}

// Demo functions for testing
async function runDemo() {
    const client = new APIClient();
    
    console.log('Running API integration demo...');
    
    try {
        // Check health
        console.log('Checking service health...');
        const health = await client.checkHealth();
        console.log('Health status:', health);
        
        // Test Go API
        console.log('Testing Go API...');
        const products = await client.getAllProducts();
        console.log(` + "`Found ${products.length} products`" + `);
        
        // Test Python ML API
        console.log('Testing Python ML API...');
        const modelInfo = await client.getModelInfo();
        console.log('Model info:', modelInfo);
        
        if (modelInfo.is_trained) {
            // Make a prediction
            const predictions = await client.makePredictions([[1.0, 2.0, -1.0, 0.5]]);
            console.log('Predictions:', predictions);
        }
        
    } catch (error) {
        console.error('Demo failed:', error.message);
    }
}

// Export classes and run demo if called directly
module.exports = { APIClient, WebSocketClient, ClientServer };

if (require.main === module) {
    const server = new ClientServer(3000);
    server.start();
    
    // Run demo after a delay
    setTimeout(runDemo, 2000);
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(jsProject, "index.js"), []byte(indexJs), 0644))

	// Store workspace config
	suite.workspaceConfig = &workspace.WorkspaceConfig{
		Root: suite.workspaceRoot,
		Projects: []workspace.ProjectConfig{
			{
				Name: "go-api",
				Type: "go",
				Path: filepath.Join(suite.workspaceRoot, "go-api"),
				Languages: []string{"go"},
				RootMarkers: []string{"go.mod"},
			},
			{
				Name: "python-ml",
				Type: "python",
				Path: filepath.Join(suite.workspaceRoot, "python-ml"),
				Languages: []string{"python"},
				RootMarkers: []string{"requirements.txt"},
			},
			{
				Name: "js-client",
				Type: "javascript",
				Path: filepath.Join(suite.workspaceRoot, "js-client"),
				Languages: []string{"javascript"},
				RootMarkers: []string{"package.json"},
			},
		},
	}
}

func (suite *MCPIntegrationTestSuite) setupMCPServer() {
	// Create mock LSP client for testing
	mockClient := &MockLSPClient{}
	
	// Create tool handler
	suite.toolHandler = mcp.NewToolHandler(mockClient)
	
	// Initialize MCP server (would normally connect to gateway)
	suite.mcpServer = &mcp.Server{
		ToolHandler: suite.toolHandler,
	}
}

// MockLSPClient implements the LSPClient interface for testing
type MockLSPClient struct {
	responses map[string]json.RawMessage
}

func (m *MockLSPClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Return mock responses based on method
	switch method {
	case "textDocument/definition":
		return json.RawMessage(`[{"uri": "file:///test.go", "range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 15}}}]`), nil
	case "textDocument/references":
		return json.RawMessage(`[{"uri": "file:///test.go", "range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 15}}}]`), nil
	case "textDocument/hover":
		return json.RawMessage(`{"contents": [{"language": "go", "value": "func Test()"}]}`), nil
	case "textDocument/documentSymbol":
		return json.RawMessage(`[{"name": "TestFunction", "kind": 12, "range": {"start": {"line": 10, "character": 0}, "end": {"line": 15, "character": 1}}}]`), nil
	case "workspace/symbol":
		return json.RawMessage(`[{"name": "TestSymbol", "kind": 12, "location": {"uri": "file:///test.go", "range": {"start": {"line": 10, "character": 0}, "end": {"line": 10, "character": 10}}}}]`), nil
	case "textDocument/completion":
		return json.RawMessage(`{"items": [{"label": "TestCompletion", "kind": 3}]}`), nil
	default:
		return json.RawMessage(`null`), nil
	}
}

func (m *MockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

// Test MCP tool: goto_definition with workspace context
func (suite *MCPIntegrationTestSuite) TestMCPGotoDefinitionWithWorkspace() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	testCases := []struct {
		name        string
		toolCall    mcp.ToolCall
		projectPath string
		language    string
	}{
		{
			name: "Go project goto definition",
			toolCall: mcp.ToolCall{
				Name: "goto_definition",
				Arguments: map[string]interface{}{
					"uri":       fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
					"line":      25,
					"character": 10,
				},
			},
			projectPath: filepath.Join(suite.workspaceRoot, "go-api"),
			language:    "go",
		},
		{
			name: "Python project goto definition",
			toolCall: mcp.ToolCall{
				Name: "goto_definition",
				Arguments: map[string]interface{}{
					"uri":       fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-ml", "ml_api.py")),
					"line":      50,
					"character": 15,
				},
			},
			projectPath: filepath.Join(suite.workspaceRoot, "python-ml"),
			language:    "python",
		},
		{
			name: "JavaScript project goto definition",
			toolCall: mcp.ToolCall{
				Name: "goto_definition",
				Arguments: map[string]interface{}{
					"uri":       fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "js-client", "index.js")),
					"line":      35,
					"character": 20,
				},
			},
			projectPath: filepath.Join(suite.workspaceRoot, "js-client"),
			language:    "javascript",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := suite.toolHandler.CallTool(ctx, tc.toolCall)
			
			suite.NoError(err, "MCP tool call should succeed")
			suite.NotNil(result, "Result should not be nil")
			suite.False(result.IsError, "Result should not be an error")
			
			// Verify result structure
			suite.NotEmpty(result.Content, "Should have content")
			suite.NotNil(result.Meta, "Should have metadata")
			suite.Equal("textDocument/definition", result.Meta.LSPMethod, "Should have correct LSP method")
			
			// Verify timing
			duration, err := time.ParseDuration(result.Meta.Duration)
			suite.NoError(err, "Should parse duration")
			suite.True(duration < 10*time.Second, "Should complete within 10 seconds")
		})
	}
}

// Test MCP tool: find_references with workspace context
func (suite *MCPIntegrationTestSuite) TestMCPFindReferencesWithWorkspace() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		toolCall mcp.ToolCall
		project  string
	}{
		{
			name: "Go references with includeDeclaration",
			toolCall: mcp.ToolCall{
				Name: "find_references",
				Arguments: map[string]interface{}{
					"uri":                fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
					"line":               15,
					"character":          10,
					"includeDeclaration": true,
				},
			},
			project: "go-api",
		},
		{
			name: "Python references without includeDeclaration",
			toolCall: mcp.ToolCall{
				Name: "find_references",
				Arguments: map[string]interface{}{
					"uri":                fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-ml", "ml_api.py")),
					"line":               25,
					"character":          8,
					"includeDeclaration": false,
				},
			},
			project: "python-ml",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := suite.toolHandler.CallTool(ctx, tc.toolCall)
			
			suite.NoError(err, "MCP tool call should succeed")
			suite.NotNil(result, "Result should not be nil")
			suite.False(result.IsError, "Result should not be an error")
			
			// Verify result structure
			suite.NotEmpty(result.Content, "Should have content")
			suite.NotNil(result.Meta, "Should have metadata")
			suite.Equal("textDocument/references", result.Meta.LSPMethod, "Should have correct LSP method")
		})
	}
}

// Test MCP tool: get_hover_info with workspace context
func (suite *MCPIntegrationTestSuite) TestMCPGetHoverInfoWithWorkspace() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		fileUri  string
		line     int
		char     int
		project  string
	}{
		{
			name:    "Go hover info",
			fileUri: fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
			line:    20,
			char:    15,
			project: "go-api",
		},
		{
			name:    "Python hover info",
			fileUri: fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-ml", "ml_api.py")),
			line:    30,
			char:    10,
			project: "python-ml",
		},
		{
			name:    "JavaScript hover info",
			fileUri: fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "js-client", "index.js")),
			line:    40,
			char:    8,
			project: "js-client",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			toolCall := mcp.ToolCall{
				Name: "get_hover_info",
				Arguments: map[string]interface{}{
					"uri":       tc.fileUri,
					"line":      tc.line,
					"character": tc.char,
				},
			}

			result, err := suite.toolHandler.CallTool(ctx, toolCall)
			
			suite.NoError(err, "MCP tool call should succeed")
			suite.NotNil(result, "Result should not be nil")
			suite.False(result.IsError, "Result should not be an error")
			
			// Verify result structure
			suite.NotEmpty(result.Content, "Should have content")
			suite.NotNil(result.Meta, "Should have metadata")
			suite.Equal("textDocument/hover", result.Meta.LSPMethod, "Should have correct LSP method")
		})
	}
}

// Test MCP tool: get_document_symbols with workspace context
func (suite *MCPIntegrationTestSuite) TestMCPGetDocumentSymbolsWithWorkspace() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	testCases := []struct {
		name    string
		fileUri string
		project string
	}{
		{
			name:    "Go document symbols",
			fileUri: fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
			project: "go-api",
		},
		{
			name:    "Python document symbols",
			fileUri: fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-ml", "ml_api.py")),
			project: "python-ml",
		},
		{
			name:    "JavaScript document symbols",
			fileUri: fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "js-client", "index.js")),
			project: "js-client",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			toolCall := mcp.ToolCall{
				Name: "get_document_symbols",
				Arguments: map[string]interface{}{
					"uri": tc.fileUri,
				},
			}

			result, err := suite.toolHandler.CallTool(ctx, toolCall)
			
			suite.NoError(err, "MCP tool call should succeed")
			suite.NotNil(result, "Result should not be nil")
			suite.False(result.IsError, "Result should not be an error")
			
			// Verify result structure
			suite.NotEmpty(result.Content, "Should have content")
			suite.NotNil(result.Meta, "Should have metadata")
			suite.Equal("textDocument/documentSymbol", result.Meta.LSPMethod, "Should have correct LSP method")
			
			// Verify content is valid JSON array
			if len(result.Content) > 0 && result.Content[0].Data != nil {
				dataBytes, err := json.Marshal(result.Content[0].Data)
				suite.NoError(err, "Should marshal data")
				
				var symbols []interface{}
				err = json.Unmarshal(dataBytes, &symbols)
				suite.NoError(err, "Data should be valid symbol array")
			}
		})
	}
}

// Test MCP tool: search_workspace_symbols with workspace context
func (suite *MCPIntegrationTestSuite) TestMCPSearchWorkspaceSymbolsWithWorkspace() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	testCases := []struct {
		name  string
		query string
	}{
		{
			name:  "Search for 'Service'",
			query: "Service",
		},
		{
			name:  "Search for 'API'",
			query: "API",
		},
		{
			name:  "Search for 'Client'",
			query: "Client",
		},
		{
			name:  "Search for 'main'",
			query: "main",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			toolCall := mcp.ToolCall{
				Name: "search_workspace_symbols",
				Arguments: map[string]interface{}{
					"query": tc.query,
				},
			}

			result, err := suite.toolHandler.CallTool(ctx, toolCall)
			
			suite.NoError(err, "MCP tool call should succeed")
			suite.NotNil(result, "Result should not be nil")
			suite.False(result.IsError, "Result should not be an error")
			
			// Verify result structure
			suite.NotEmpty(result.Content, "Should have content")
			suite.NotNil(result.Meta, "Should have metadata")
			
			// Should use workspace/symbol or fallback method
			suite.True(
				result.Meta.LSPMethod == "workspace/symbol" || 
				strings.Contains(result.Meta.LSPMethod, "fallback"),
				"Should use workspace/symbol or fallback method")
		})
	}
}

// Test MCP error handling and validation
func (suite *MCPIntegrationTestSuite) TestMCPErrorHandling() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	testCases := []struct {
		name      string
		toolCall  mcp.ToolCall
		expectErr bool
	}{
		{
			name: "Invalid tool name",
			toolCall: mcp.ToolCall{
				Name:      "invalid_tool",
				Arguments: map[string]interface{}{},
			},
			expectErr: true,
		},
		{
			name: "Missing required URI parameter",
			toolCall: mcp.ToolCall{
				Name: "goto_definition",
				Arguments: map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid URI format",
			toolCall: mcp.ToolCall{
				Name: "get_hover_info",
				Arguments: map[string]interface{}{
					"uri":       "invalid-uri-format",
					"line":      10,
					"character": 5,
				},
			},
			expectErr: true,
		},
		{
			name: "Negative line number",
			toolCall: mcp.ToolCall{
				Name: "find_references",
				Arguments: map[string]interface{}{
					"uri":       fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
					"line":      -1,
					"character": 5,
				},
			},
			expectErr: true,
		},
		{
			name: "Negative character position",
			toolCall: mcp.ToolCall{
				Name: "get_document_symbols",
				Arguments: map[string]interface{}{
					"uri":       fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
					"line":      10,
					"character": -5,
				},
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := suite.toolHandler.CallTool(ctx, tc.toolCall)
			
			if tc.expectErr {
				// Should have either error or error result
				if err != nil {
					suite.Error(err, "Should return error")
				} else {
					suite.NotNil(result, "Should have result")
					suite.True(result.IsError, "Result should indicate error")
				}
			} else {
				suite.NoError(err, "Should not return error")
				suite.NotNil(result, "Should have result")
				suite.False(result.IsError, "Result should not indicate error")
			}
		})
	}
}

// Test MCP workspace context switching
func (suite *MCPIntegrationTestSuite) TestMCPWorkspaceContextSwitching() {
	ctx, cancel := context.WithTimeout(suite.ctx, 15*time.Second)
	defer cancel()

	// Test switching between different project contexts
	projects := []struct {
		name     string
		fileUri  string
		language string
	}{
		{
			name:     "go-api",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
			language: "go",
		},
		{
			name:     "python-ml",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-ml", "ml_api.py")),
			language: "python",
		},
		{
			name:     "js-client",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "js-client", "index.js")),
			language: "javascript",
		},
	}

	// Test rapid context switching
	for i := 0; i < 3; i++ {
		for _, project := range projects {
			suite.Run(fmt.Sprintf("Context switch to %s (iteration %d)", project.name, i+1), func() {
				toolCall := mcp.ToolCall{
					Name: "get_hover_info",
					Arguments: map[string]interface{}{
						"uri":       project.fileUri,
						"line":      10,
						"character": 5,
					},
				}

				start := time.Now()
				result, err := suite.toolHandler.CallTool(ctx, toolCall)
				duration := time.Since(start)

				suite.NoError(err, "Context switch should succeed")
				suite.NotNil(result, "Should have result")
				suite.False(result.IsError, "Should not have error")
				
				// Verify fast context switching (should complete within 1 second)
				suite.True(duration < 1*time.Second, 
					fmt.Sprintf("Context switch should be fast, took %v", duration))
			})
		}
	}
}

// Test MCP performance requirements
func (suite *MCPIntegrationTestSuite) TestMCPPerformanceRequirements() {
	ctx, cancel := context.WithTimeout(suite.ctx, 15*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		toolCall mcp.ToolCall
		timeout  time.Duration
	}{
		{
			name: "MCP tool call performance (10 second limit)",
			toolCall: mcp.ToolCall{
				Name: "goto_definition",
				Arguments: map[string]interface{}{
					"uri":       fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-api", "main.go")),
					"line":      15,
					"character": 10,
				},
			},
			timeout: 10 * time.Second,
		},
		{
			name: "Document symbols performance",
			toolCall: mcp.ToolCall{
				Name: "get_document_symbols",
				Arguments: map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-ml", "ml_api.py")),
				},
			},
			timeout: 10 * time.Second,
		},
		{
			name: "Workspace symbol search performance",
			toolCall: mcp.ToolCall{
				Name: "search_workspace_symbols",
				Arguments: map[string]interface{}{
					"query": "Service",
				},
			},
			timeout: 10 * time.Second,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			start := time.Now()
			
			perfCtx, perfCancel := context.WithTimeout(ctx, tc.timeout)
			defer perfCancel()

			result, err := suite.toolHandler.CallTool(perfCtx, tc.toolCall)
			duration := time.Since(start)

			suite.NoError(err, "Tool call should succeed within timeout")
			suite.NotNil(result, "Should have result")
			suite.False(result.IsError, "Should not have error")
			suite.True(duration < tc.timeout, 
				fmt.Sprintf("Tool call should complete within %v, took %v", tc.timeout, duration))
		})
	}
}

// Test MCP tool schema validation
func (suite *MCPIntegrationTestSuite) TestMCPToolSchemaValidation() {
	// Test that all MCP tools are properly registered
	tools := suite.toolHandler.ListTools()
	
	expectedTools := []string{
		"goto_definition",
		"find_references", 
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	suite.Len(tools, len(expectedTools), "Should have all expected tools")

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	for _, expectedTool := range expectedTools {
		suite.Contains(toolNames, expectedTool, fmt.Sprintf("Should contain tool: %s", expectedTool))
	}

	// Verify tool schemas
	for _, tool := range tools {
		suite.NotEmpty(tool.Name, "Tool should have name")
		suite.NotEmpty(tool.Description, "Tool should have description")
		suite.NotNil(tool.InputSchema, "Tool should have input schema")
		
		// Verify input schema structure
		inputSchema, ok := tool.InputSchema.(map[string]interface{})
		suite.True(ok, "Input schema should be object")
		
		suite.Equal("object", inputSchema["type"], "Input schema should be object type")
		suite.NotNil(inputSchema["properties"], "Should have properties")
		
		if tool.OutputSchema != nil {
			outputSchema := *tool.OutputSchema
			suite.Equal("object", outputSchema["type"], "Output schema should be object type")
		}
	}
}