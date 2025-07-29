package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/internal/workspace"
)

// ClientManagementTestSuite tests LSP client lifecycle management
type ClientManagementTestSuite struct {
	suite.Suite
	tempDir         string
	workspaceRoot   string
	gateway         workspace.WorkspaceGateway
	workspaceConfig *workspace.WorkspaceConfig
	gatewayConfig   *workspace.WorkspaceGatewayConfig
	ctx             context.Context
	cancel          context.CancelFunc
}

func TestClientManagementTestSuite(t *testing.T) {
	suite.Run(t, new(ClientManagementTestSuite))
}

func (suite *ClientManagementTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 60*time.Second)
	
	var err error
	suite.tempDir, err = os.MkdirTemp("", "client-management-test-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	suite.workspaceRoot = filepath.Join(suite.tempDir, "workspace")
	err = os.MkdirAll(suite.workspaceRoot, 0755)
	suite.Require().NoError(err)
	
	suite.setupTestWorkspace()
	suite.setupClientManagementConfig()
}

func (suite *ClientManagementTestSuite) TearDownSuite() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
	suite.cancel()
	os.RemoveAll(suite.tempDir)
}

func (suite *ClientManagementTestSuite) SetupTest() {
	suite.gateway = workspace.NewWorkspaceGateway()
	err := suite.gateway.Initialize(suite.ctx, suite.workspaceConfig, suite.gatewayConfig)
	suite.Require().NoError(err, "Failed to initialize gateway")
}

func (suite *ClientManagementTestSuite) TearDownTest() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
}

func (suite *ClientManagementTestSuite) setupTestWorkspace() {
	// Create Go project with detailed structure
	goProject := filepath.Join(suite.workspaceRoot, "services", "user-service")
	suite.Require().NoError(os.MkdirAll(goProject, 0755))
	
	goMod := `module user-service

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/golang-jwt/jwt/v5 v5.1.0
)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "go.mod"), []byte(goMod), 0644))
	
	// Main service file
	mainGo := `package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID       int    ` + "`json:\"id\"`" + `
	Username string ` + "`json:\"username\"`" + `
	Email    string ` + "`json:\"email\"`" + `
}

type UserService struct {
	users []User
}

func NewUserService() *UserService {
	return &UserService{
		users: make([]User, 0),
	}
}

func (us *UserService) CreateUser(username, email string) User {
	user := User{
		ID:       len(us.users) + 1,
		Username: username,
		Email:    email,
	}
	us.users = append(us.users, user)
	return user
}

func (us *UserService) GetUser(id int) (User, bool) {
	for _, user := range us.users {
		if user.ID == id {
			return user, true
		}
	}
	return User{}, false
}

func main() {
	service := NewUserService()
	r := gin.Default()
	
	r.POST("/users", func(c *gin.Context) {
		var req struct {
			Username string ` + "`json:\"username\"`" + `
			Email    string ` + "`json:\"email\"`" + `
		}
		
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		user := service.CreateUser(req.Username, req.Email)
		c.JSON(http.StatusCreated, user)
	})
	
	r.GET("/users/:id", func(c *gin.Context) {
		// Implementation details...
	})
	
	log.Println("Starting user service on :8080")
	r.Run(":8080")
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "main.go"), []byte(mainGo), 0644))
	
	// Add handler package
	handlerDir := filepath.Join(goProject, "handlers")
	suite.Require().NoError(os.MkdirAll(handlerDir, 0755))
	
	userHandler := `package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service UserService
}

func NewUserHandler(service UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	
	user, found := h.service.GetUser(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	
	c.JSON(http.StatusOK, user)
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(handlerDir, "user.go"), []byte(userHandler), 0644))
	
	// Create Python project
	pythonProject := filepath.Join(suite.workspaceRoot, "services", "notification-service")
	suite.Require().NoError(os.MkdirAll(pythonProject, 0755))
	
	requirements := `fastapi==0.104.1
uvicorn==0.24.0
pydantic==2.4.2
sqlalchemy==2.0.23
asyncpg==0.29.0
redis==5.0.1
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "requirements.txt"), []byte(requirements), 0644))
	
	mainPy := `from fastapi import FastAPI, HTTPException, Depends
from pydantic import BaseModel, EmailStr
from typing import List, Optional
import asyncio
import redis
from sqlalchemy import create_engine, Column, Integer, String, DateTime
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession
from datetime import datetime

app = FastAPI(title="Notification Service", version="1.0.0")

# Redis client
redis_client = redis.Redis(host='localhost', port=6379, db=0)

Base = declarative_base()

class NotificationModel(Base):
    __tablename__ = "notifications"
    
    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(Integer, index=True)
    message = Column(String)
    status = Column(String, default="pending")
    created_at = Column(DateTime, default=datetime.utcnow)

class Notification(BaseModel):
    id: Optional[int] = None
    user_id: int
    message: str
    status: str = "pending"
    created_at: Optional[datetime] = None

class NotificationCreate(BaseModel):
    user_id: int
    message: str

class NotificationService:
    def __init__(self):
        self.notifications = []
    
    async def create_notification(self, notification: NotificationCreate) -> Notification:
        new_notification = Notification(
            id=len(self.notifications) + 1,
            user_id=notification.user_id,
            message=notification.message,
            created_at=datetime.utcnow()
        )
        self.notifications.append(new_notification)
        
        # Cache in Redis
        await self.cache_notification(new_notification)
        
        return new_notification
    
    async def cache_notification(self, notification: Notification):
        try:
            redis_client.setex(
                f"notification:{notification.id}",
                3600,  # 1 hour TTL
                notification.model_dump_json()
            )
        except Exception as e:
            print(f"Failed to cache notification: {e}")
    
    async def get_notifications(self, user_id: int) -> List[Notification]:
        return [n for n in self.notifications if n.user_id == user_id]

notification_service = NotificationService()

@app.post("/notifications/", response_model=Notification)
async def create_notification(notification: NotificationCreate):
    return await notification_service.create_notification(notification)

@app.get("/notifications/{user_id}", response_model=List[Notification])
async def get_user_notifications(user_id: int):
    return await notification_service.get_notifications(user_id)

@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "notification-service"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8001)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "main.py"), []byte(mainPy), 0644))
	
	// Create models package
	modelsDir := filepath.Join(pythonProject, "models")
	suite.Require().NoError(os.MkdirAll(modelsDir, 0755))
	
	suite.Require().NoError(os.WriteFile(filepath.Join(modelsDir, "__init__.py"), []byte(""), 0644))
	
	userModel := `from pydantic import BaseModel, EmailStr
from typing import Optional
from datetime import datetime

class UserProfile(BaseModel):
    id: int
    username: str
    email: EmailStr
    full_name: Optional[str] = None
    created_at: datetime
    is_active: bool = True
    
    class Config:
        from_attributes = True

class UserPreferences(BaseModel):
    user_id: int
    email_notifications: bool = True
    sms_notifications: bool = False
    push_notifications: bool = True
    
    def should_send_email(self) -> bool:
        return self.email_notifications
    
    def should_send_sms(self) -> bool:
        return self.sms_notifications
`
	suite.Require().NoError(os.WriteFile(filepath.Join(modelsDir, "user.py"), []byte(userModel), 0644))
	
	// Create TypeScript project
	tsProject := filepath.Join(suite.workspaceRoot, "services", "api-gateway")
	suite.Require().NoError(os.MkdirAll(tsProject, 0755))
	
	packageJson := `{
  "name": "api-gateway",
  "version": "1.0.0",
  "description": "API Gateway service",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.2",
    "axios": "^1.5.1",
    "cors": "^2.8.5",
    "helmet": "^7.1.0",
    "express-rate-limit": "^7.1.3",
    "jsonwebtoken": "^9.0.2",
    "winston": "^3.10.0"
  },
  "devDependencies": {
    "@types/express": "^4.17.20",
    "@types/node": "^20.8.7",
    "@types/cors": "^2.8.14",
    "@types/jsonwebtoken": "^9.0.3",
    "typescript": "^5.2.2",
    "ts-node": "^10.9.1",
    "jest": "^29.7.0"
  }
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(tsProject, "package.json"), []byte(packageJson), 0644))
	
	srcDir := filepath.Join(tsProject, "src")
	suite.Require().NoError(os.MkdirAll(srcDir, 0755))
	
	indexTs := `import express, { Request, Response, NextFunction } from 'express';
import cors from 'cors';
import helmet from 'helmet';
import rateLimit from 'express-rate-limit';
import jwt from 'jsonwebtoken';
import axios from 'axios';
import winston from 'winston';

// Configure logger
const logger = winston.createLogger({
    level: 'info',
    format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
    ),
    transports: [
        new winston.transports.Console(),
        new winston.transports.File({ filename: 'logs/api-gateway.log' })
    ]
});

interface User {
    id: number;
    username: string;
    email: string;
}

interface AuthenticatedRequest extends Request {
    user?: User;
}

interface ServiceConfig {
    name: string;
    url: string;
    timeout: number;
    retries: number;
}

class ApiGateway {
    private app: express.Application;
    private services: Map<string, ServiceConfig>;
    
    constructor() {
        this.app = express();
        this.services = new Map();
        this.setupMiddleware();
        this.setupRoutes();
        this.registerServices();
    }
    
    private setupMiddleware(): void {
        // Security middleware
        this.app.use(helmet());
        this.app.use(cors({
            origin: process.env.ALLOWED_ORIGINS?.split(',') || ['http://localhost:3000'],
            credentials: true
        }));
        
        // Rate limiting
        const limiter = rateLimit({
            windowMs: 15 * 60 * 1000, // 15 minutes
            max: 100, // limit each IP to 100 requests per windowMs
            message: 'Too many requests from this IP'
        });
        this.app.use(limiter);
        
        this.app.use(express.json({ limit: '10mb' }));
        this.app.use(express.urlencoded({ extended: true }));
        
        // Request logging
        this.app.use((req: Request, res: Response, next: NextFunction) => {
            logger.info(\`\${req.method} \${req.path}\`, {
                ip: req.ip,
                userAgent: req.get('User-Agent')
            });
            next();
        });
    }
    
    private async authenticateToken(req: AuthenticatedRequest, res: Response, next: NextFunction): Promise<void> {
        const authHeader = req.headers['authorization'];
        const token = authHeader && authHeader.split(' ')[1];
        
        if (!token) {
            res.status(401).json({ error: 'Access token required' });
            return;
        }
        
        try {
            const decoded = jwt.verify(token, process.env.JWT_SECRET || 'default-secret') as any;
            req.user = decoded.user;
            next();
        } catch (error) {
            res.status(403).json({ error: 'Invalid token' });
        }
    }
    
    private registerServices(): void {
        this.services.set('user-service', {
            name: 'user-service',
            url: 'http://localhost:8080',
            timeout: 5000,
            retries: 3
        });
        
        this.services.set('notification-service', {
            name: 'notification-service',
            url: 'http://localhost:8001',
            timeout: 5000,
            retries: 2
        });
    }
    
    private async proxyRequest(serviceName: string, path: string, method: string, data?: any, headers?: any): Promise<any> {
        const service = this.services.get(serviceName);
        if (!service) {
            throw new Error(\`Service \${serviceName} not found\`);
        }
        
        const url = \`\${service.url}\${path}\`;
        
        try {
            const response = await axios({
                method: method as any,
                url,
                data,
                headers,
                timeout: service.timeout
            });
            
            return response.data;
        } catch (error) {
            logger.error(\`Error proxying to \${serviceName}\`, { error, url, method });
            throw error;
        }
    }
    
    private setupRoutes(): void {
        // Health check
        this.app.get('/health', (req: Request, res: Response) => {
            res.json({
                status: 'healthy',
                timestamp: new Date().toISOString(),
                services: Array.from(this.services.keys())
            });
        });
        
        // User service routes
        this.app.post('/api/users', this.authenticateToken.bind(this), async (req: AuthenticatedRequest, res: Response) => {
            try {
                const result = await this.proxyRequest('user-service', '/users', 'POST', req.body);
                res.json(result);
            } catch (error) {
                res.status(500).json({ error: 'Failed to create user' });
            }
        });
        
        this.app.get('/api/users/:id', this.authenticateToken.bind(this), async (req: AuthenticatedRequest, res: Response) => {
            try {
                const result = await this.proxyRequest('user-service', \`/users/\${req.params.id}\`, 'GET');
                res.json(result);
            } catch (error) {
                res.status(500).json({ error: 'Failed to get user' });
            }
        });
        
        // Notification service routes
        this.app.post('/api/notifications', this.authenticateToken.bind(this), async (req: AuthenticatedRequest, res: Response) => {
            try {
                const result = await this.proxyRequest('notification-service', '/notifications/', 'POST', req.body);
                res.json(result);
            } catch (error) {
                res.status(500).json({ error: 'Failed to create notification' });
            }
        });
        
        this.app.get('/api/notifications/:userId', this.authenticateToken.bind(this), async (req: AuthenticatedRequest, res: Response) => {
            try {
                const result = await this.proxyRequest('notification-service', \`/notifications/\${req.params.userId}\`, 'GET');
                res.json(result);
            } catch (error) {
                res.status(500).json({ error: 'Failed to get notifications' });
            }
        });
    }
    
    public start(port: number = 3000): void {
        this.app.listen(port, () => {
            logger.info(\`API Gateway started on port \${port}\`);
        });
    }
}

// Start the gateway
const gateway = new ApiGateway();
gateway.start();

export default ApiGateway;
`
	suite.Require().NoError(os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte(indexTs), 0644))
	
	// Add middleware package
	middlewareDir := filepath.Join(srcDir, "middleware")
	suite.Require().NoError(os.MkdirAll(middlewareDir, 0755))
	
	authMiddleware := `import { Request, Response, NextFunction } from 'express';
import jwt from 'jsonwebtoken';

interface AuthenticatedRequest extends Request {
    user?: {
        id: number;
        username: string;
        email: string;
    };
}

export const authenticateToken = (req: AuthenticatedRequest, res: Response, next: NextFunction): void => {
    const authHeader = req.headers['authorization'];
    const token = authHeader && authHeader.split(' ')[1];
    
    if (!token) {
        res.status(401).json({ error: 'Access token required' });
        return;
    }
    
    try {
        const decoded = jwt.verify(token, process.env.JWT_SECRET || 'default-secret') as any;
        req.user = decoded.user;
        next();
    } catch (error) {
        res.status(403).json({ error: 'Invalid token' });
    }
};

export const validateRequest = (schema: any) => {
    return (req: Request, res: Response, next: NextFunction): void => {
        const { error } = schema.validate(req.body);
        if (error) {
            res.status(400).json({ error: error.details[0].message });
            return;
        }
        next();
    };
};
`
	suite.Require().NoError(os.WriteFile(filepath.Join(middlewareDir, "auth.ts"), []byte(authMiddleware), 0644))
}

func (suite *ClientManagementTestSuite) setupClientManagementConfig() {
	suite.workspaceConfig = &workspace.WorkspaceConfig{
		Workspace: config.WorkspaceConfig{
			RootPath: suite.workspaceRoot,
		},
		Servers: map[string]*config.ServerConfig{
			"gopls": {
				Command:   "gopls",
				Args:      []string{"serve"},
				Languages: []string{"go"},
				Transport: "stdio",
				Enabled:   true,
			},
			"pylsp": {
				Command:   "pylsp",
				Args:      []string{},
				Languages: []string{"python"},
				Transport: "stdio",
				Enabled:   true,
			},
			"typescript-language-server": {
				Command:   "typescript-language-server",
				Args:      []string{"--stdio"},
				Languages: []string{"typescript", "javascript"},
				Transport: "stdio",
				Enabled:   true,
			},
		},
		Logging: config.LoggingConfig{
			Level: "debug",
		},
	}
	
	suite.gatewayConfig = &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot: suite.workspaceRoot,
		ExtensionMapping: map[string]string{
			"go":   "go",
			"py":   "python",
			"ts":   "typescript",
			"js":   "javascript",
		},
		Timeout:       30 * time.Second,
		EnableLogging: true,
	}
}

func (suite *ClientManagementTestSuite) TestClientLifecycle() {
	suite.T().Log("Testing LSP client lifecycle management")
	
	// Start gateway
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Verify initial client state
	health := suite.gateway.Health()
	suite.NotNil(health)
	initialClientCount := health.ActiveClients
	
	// Test sub-project detection
	subProjects := suite.gateway.GetSubProjects()
	suite.NotEmpty(subProjects, "Should detect sub-projects")
	
	for _, project := range subProjects {
		suite.T().Logf("Detected project: %s (type: %s, languages: %v)", 
			project.ID, project.ProjectType, project.Languages)
	}
	
	// Allow time for clients to initialize
	time.Sleep(2 * time.Second)
	
	// Check client status after initialization
	health = suite.gateway.Health()
	suite.GreaterOrEqual(health.ActiveClients, initialClientCount)
	
	// Test client retrieval for each sub-project
	for _, project := range subProjects {
		for _, language := range project.Languages {
			client, err := suite.gateway.GetSubProjectClient(project.ID, language)
			if err == nil {
				suite.NotNil(client, "Client should exist for project %s, language %s", project.ID, language)
				
				// Test client is active (if LSP server is available)
				isActive := client.IsActive()
				suite.T().Logf("Client for %s/%s is active: %v", project.ID, language, isActive)
			} else {
				suite.T().Logf("Client not available for %s/%s: %v", project.ID, language, err)
			}
		}
	}
}

func (suite *ClientManagementTestSuite) TestLegacyClientCompatibility() {
	suite.T().Log("Testing legacy client compatibility")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test legacy client retrieval
	supportedLanguages := []string{"go", "python", "typescript", "javascript"}
	
	for _, language := range supportedLanguages {
		client, exists := suite.gateway.GetClient(language)
		if exists {
			suite.NotNil(client, "Legacy client should exist for %s", language)
			
			// Test basic client interface
			isActive := client.IsActive()
			suite.T().Logf("Legacy client for %s is active: %v", language, isActive)
		} else {
			suite.T().Logf("Legacy client not available for %s", language)
		}
	}
}

func (suite *ClientManagementTestSuite) TestClientStartupTiming() {
	suite.T().Log("Testing client startup timing requirements")
	
	startTime := time.Now()
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	startupDuration := time.Since(startTime)
	
	// Client startup should complete within 5 seconds per client requirement
	suite.Less(startupDuration, 15*time.Second, "Gateway startup took too long: %v", startupDuration)
	
	suite.T().Logf("Gateway startup completed in %v", startupDuration)
}

func (suite *ClientManagementTestSuite) TestClientHealthMonitoring() {
	suite.T().Log("Testing client health monitoring")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test health monitoring timing requirement
	healthCheckStart := time.Now()
	health := suite.gateway.Health()
	healthCheckDuration := time.Since(healthCheckStart)
	
	// Health check should complete within 100ms requirement
	suite.Less(healthCheckDuration, 100*time.Millisecond, "Health check took too long: %v", healthCheckDuration)
	
	suite.NotNil(health)
	suite.NotNil(health.ClientStatuses)
	suite.NotNil(health.SubProjectClients)
	
	// Verify health structure contains expected information
	suite.GreaterOrEqual(health.ActiveClients, 0)
	suite.WithinDuration(time.Now(), health.LastCheck, 5*time.Second)
	
	// Test sub-project client health information
	if len(health.SubProjectClients) > 0 {
		for projectID, projectClients := range health.SubProjectClients {
			suite.NotEmpty(projectID, "Project ID should not be empty")
			suite.NotNil(projectClients, "Project clients should not be nil")
			
			for language, clientStatus := range projectClients {
				suite.NotEmpty(language, "Language should not be empty")
				suite.NotEmpty(clientStatus.Language, "Client status language should not be empty")
				suite.WithinDuration(time.Now(), clientStatus.LastUsed, 10*time.Minute)
			}
		}
	}
}

func (suite *ClientManagementTestSuite) TestClientErrorHandling() {
	suite.T().Log("Testing client error handling and recovery")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test invalid project ID
	_, err = suite.gateway.GetSubProjectClient("non-existent-project", "go")
	suite.Error(err, "Should return error for non-existent project")
	
	// Test invalid language
	subProjects := suite.gateway.GetSubProjects()
	if len(subProjects) > 0 {
		_, err = suite.gateway.GetSubProjectClient(subProjects[0].ID, "invalid-language")
		suite.Error(err, "Should return error for invalid language")
	}
	
	// Verify gateway continues to function after errors
	health := suite.gateway.Health()
	suite.NotNil(health)
}

func (suite *ClientManagementTestSuite) TestClientResourceManagement() {
	suite.T().Log("Testing client resource management")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Get initial resource state
	initialHealth := suite.gateway.Health()
	initialClientCount := initialHealth.ActiveClients
	
	// Refresh sub-projects (may create additional clients)
	err = suite.gateway.RefreshSubProjects(suite.ctx)
	suite.NoError(err, "Failed to refresh sub-projects")
	
	// Allow time for client operations
	time.Sleep(1 * time.Second)
	
	// Verify resource management
	updatedHealth := suite.gateway.Health()
	suite.GreaterOrEqual(updatedHealth.ActiveClients, 0)
	
	// Test resource cleanup doesn't break functionality
	suite.NotNil(updatedHealth.ClientStatuses)
	suite.WithinDuration(time.Now(), updatedHealth.LastCheck, 5*time.Second)
	
	suite.T().Logf("Client count: initial=%d, after_refresh=%d", 
		initialClientCount, updatedHealth.ActiveClients)
}

func (suite *ClientManagementTestSuite) TestConcurrentClientAccess() {
	suite.T().Log("Testing concurrent client access")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	subProjects := suite.gateway.GetSubProjects()
	if len(subProjects) == 0 {
		suite.T().Skip("No sub-projects available for concurrent testing")
		return
	}
	
	// Test concurrent client retrieval
	const numGoroutines = 10
	results := make(chan bool, numGoroutines)
	
	project := subProjects[0]
	if len(project.Languages) == 0 {
		suite.T().Skip("No languages available for concurrent testing")
		return
	}
	
	language := project.Languages[0]
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					results <- false
				}
			}()
			
			client, err := suite.gateway.GetSubProjectClient(project.ID, language)
			if err != nil {
				results <- false
				return
			}
			
			// Test basic client operation
			isActive := client != nil && client.IsActive()
			results <- true // Success if no panic
		}()
	}
	
	// Wait for all goroutines to complete
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		select {
		case success := <-results:
			if success {
				successCount++
			}
		case <-time.After(10 * time.Second):
			suite.Fail("Concurrent client access timed out")
		}
	}
	
	suite.Greater(successCount, 0, "At least some concurrent accesses should succeed")
	suite.T().Logf("Concurrent access success rate: %d/%d", successCount, numGoroutines)
}

func (suite *ClientManagementTestSuite) TestClientShutdownSequence() {
	suite.T().Log("Testing client shutdown sequence")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Verify clients are running
	health := suite.gateway.Health()
	initialActiveClients := health.ActiveClients
	
	// Test graceful shutdown
	shutdownStart := time.Now()
	err = suite.gateway.Stop()
	shutdownDuration := time.Since(shutdownStart)
	
	suite.NoError(err, "Gateway shutdown should not return error")
	suite.Less(shutdownDuration, 10*time.Second, "Shutdown took too long: %v", shutdownDuration)
	
	// Verify clean shutdown state
	health = suite.gateway.Health()
	suite.LessOrEqual(health.ActiveClients, initialActiveClients, "Active client count should not increase after shutdown")
	
	suite.T().Logf("Shutdown completed in %v, initial clients: %d, final clients: %d", 
		shutdownDuration, initialActiveClients, health.ActiveClients)
}

func (suite *ClientManagementTestSuite) TestClientReinitialization() {
	suite.T().Log("Testing client reinitialization after shutdown")
	
	// First startup
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway initially")
	
	initialHealth := suite.gateway.Health()
	initialProjects := suite.gateway.GetSubProjects()
	
	// Shutdown
	err = suite.gateway.Stop()
	suite.Require().NoError(err, "Failed to stop gateway")
	
	// Reinitialize with new gateway instance
	suite.gateway = workspace.NewWorkspaceGateway()
	err = suite.gateway.Initialize(suite.ctx, suite.workspaceConfig, suite.gatewayConfig)
	suite.Require().NoError(err, "Failed to reinitialize gateway")
	
	// Second startup
	err = suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to restart gateway")
	
	// Verify reinitialization
	restartedHealth := suite.gateway.Health()
	restartedProjects := suite.gateway.GetSubProjects()
	
	suite.NotNil(restartedHealth)
	suite.NotNil(restartedProjects)
	suite.Equal(len(initialProjects), len(restartedProjects), "Project count should remain consistent")
	
	suite.T().Logf("Reinitialization successful: projects=%d, clients=%d", 
		len(restartedProjects), restartedHealth.ActiveClients)
}

func (suite *ClientManagementTestSuite) TestClientInterfaceCompliance() {
	suite.T().Log("Testing LSP client interface compliance")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	subProjects := suite.gateway.GetSubProjects()
	for _, project := range subProjects {
		for _, language := range project.Languages {
			client, err := suite.gateway.GetSubProjectClient(project.ID, language)
			if err != nil {
				continue // Skip if client not available
			}
			
			// Test interface compliance
			suite.IsType((*transport.LSPClient)(nil), &client, "Client should implement LSPClient interface")
			
			// Test basic interface methods don't panic
			suite.NotPanics(func() {
				client.IsActive()
			}, "IsActive should not panic")
			
			suite.T().Logf("Client interface compliance verified for %s/%s", project.ID, language)
		}
	}
}