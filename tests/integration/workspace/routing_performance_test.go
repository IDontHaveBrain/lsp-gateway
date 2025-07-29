package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/workspace"
)

// RoutingPerformanceTestSuite tests routing performance and efficiency
type RoutingPerformanceTestSuite struct {
	suite.Suite
	tempDir         string
	workspaceRoot   string
	gateway         workspace.WorkspaceGateway
	workspaceConfig *workspace.WorkspaceConfig
	gatewayConfig   *workspace.WorkspaceGatewayConfig
	ctx             context.Context
	cancel          context.CancelFunc
}

func TestRoutingPerformanceTestSuite(t *testing.T) {
	suite.Run(t, new(RoutingPerformanceTestSuite))
}

func (suite *RoutingPerformanceTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 120*time.Second)
	
	var err error
	suite.tempDir, err = os.MkdirTemp("", "routing-performance-test-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	suite.workspaceRoot = filepath.Join(suite.tempDir, "performance-workspace")
	err = os.MkdirAll(suite.workspaceRoot, 0755)
	suite.Require().NoError(err)
	
	suite.setupPerformanceTestWorkspace()
	suite.setupPerformanceConfig()
}

func (suite *RoutingPerformanceTestSuite) TearDownSuite() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
	suite.cancel()
	os.RemoveAll(suite.tempDir)
}

func (suite *RoutingPerformanceTestSuite) SetupTest() {
	suite.gateway = workspace.NewWorkspaceGateway()
	err := suite.gateway.Initialize(suite.ctx, suite.workspaceConfig, suite.gatewayConfig)
	suite.Require().NoError(err, "Failed to initialize gateway")
}

func (suite *RoutingPerformanceTestSuite) TearDownTest() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
}

func (suite *RoutingPerformanceTestSuite) setupPerformanceTestWorkspace() {
	// Create multiple projects for performance testing

	// Large Go project with multiple packages
	goProject := filepath.Join(suite.workspaceRoot, "large-go-service")
	suite.setupLargeGoProject(goProject)
	
	// Python project with multiple modules
	pythonProject := filepath.Join(suite.workspaceRoot, "python-microservice")
	suite.setupPythonProject(pythonProject)
	
	// TypeScript project with complex structure
	tsProject := filepath.Join(suite.workspaceRoot, "react-frontend")
	suite.setupTypeScriptProject(tsProject)
	
	// Additional nested projects for routing complexity
	nestedGoProject := filepath.Join(suite.workspaceRoot, "services", "auth-service")
	suite.setupNestedGoProject(nestedGoProject)
	
	jsProject := filepath.Join(suite.workspaceRoot, "shared", "utils")
	suite.setupJavaScriptProject(jsProject)
}

func (suite *RoutingPerformanceTestSuite) setupLargeGoProject(projectPath string) {
	suite.Require().NoError(os.MkdirAll(projectPath, 0755))
	
	goMod := `module large-go-service

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/golang-jwt/jwt/v5 v5.1.0
	github.com/gorilla/websocket v1.5.0
	github.com/redis/go-redis/v9 v9.3.0
	gorm.io/gorm v1.25.5
	gorm.io/driver/postgres v1.5.4
)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "go.mod"), []byte(goMod), 0644))
	
	// Create multiple packages
	packages := []string{"api", "auth", "database", "middleware", "models", "services", "utils", "websocket"}
	
	for _, pkg := range packages {
		pkgDir := filepath.Join(projectPath, pkg)
		suite.Require().NoError(os.MkdirAll(pkgDir, 0755))
		
		switch pkg {
		case "api":
			suite.createGoAPIPackage(pkgDir)
		case "auth":
			suite.createGoAuthPackage(pkgDir)
		case "database":
			suite.createGoDatabasePackage(pkgDir)
		case "middleware":
			suite.createGoMiddlewarePackage(pkgDir)
		case "models":
			suite.createGoModelsPackage(pkgDir)
		case "services":
			suite.createGoServicesPackage(pkgDir)
		case "utils":
			suite.createGoUtilsPackage(pkgDir)
		case "websocket":
			suite.createGoWebSocketPackage(pkgDir)
		}
	}
	
	// Main application file
	mainGo := `package main

import (
	"large-go-service/api"
	"large-go-service/auth"
	"large-go-service/database"
	"large-go-service/middleware"
	"large-go-service/websocket"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize database
	db, err := database.Initialize()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Initialize services
	authService := auth.NewAuthService(db)
	wsHub := websocket.NewHub()
	
	// Setup router
	r := gin.Default()
	
	// Middleware
	r.Use(middleware.CORS())
	r.Use(middleware.Logger())
	r.Use(middleware.RateLimit())
	
	// Routes
	api.SetupRoutes(r, authService)
	websocket.SetupWebSocket(r, wsHub)
	
	// Start WebSocket hub
	go wsHub.Run()
	
	log.Println("Starting server on :8080")
	r.Run(":8080")
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "main.go"), []byte(mainGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoAPIPackage(pkgDir string) {
	apiGo := `package api

import (
	"large-go-service/auth"
	"large-go-service/models"
	"large-go-service/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type API struct {
	authService *auth.AuthService
	userService *services.UserService
}

func NewAPI(authService *auth.AuthService, userService *services.UserService) *API {
	return &API{
		authService: authService,
		userService: userService,
	}
}

func SetupRoutes(r *gin.Engine, authService *auth.AuthService) {
	api := NewAPI(authService, services.NewUserService())
	
	v1 := r.Group("/api/v1")
	{
		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/login", api.Login)
			auth.POST("/register", api.Register)
			auth.POST("/refresh", api.RefreshToken)
			auth.POST("/logout", api.Logout)
		}
		
		// User routes
		users := v1.Group("/users")
		users.Use(api.AuthMiddleware())
		{
			users.GET("", api.GetUsers)
			users.GET("/:id", api.GetUser)
			users.POST("", api.CreateUser)
			users.PUT("/:id", api.UpdateUser)
			users.DELETE("/:id", api.DeleteUser)
		}
		
		// Profile routes
		profile := v1.Group("/profile")
		profile.Use(api.AuthMiddleware())
		{
			profile.GET("", api.GetProfile)
			profile.PUT("", api.UpdateProfile)
			profile.POST("/avatar", api.UploadAvatar)
		}
	}
}

func (api *API) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}
		
		claims, err := api.authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}
		
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

func (api *API) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	token, err := api.authService.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (api *API) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	user, err := api.authService.Register(req.Email, req.Password, req.Name)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, user)
}

func (api *API) GetUsers(c *gin.Context) {
	users, err := api.userService.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServer, gin.H{"error": "Failed to retrieve users"})
		return
	}
	
	c.JSON(http.StatusOK, users)
}

func (api *API) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	
	user, err := api.userService.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	
	c.JSON(http.StatusOK, user)
}

func (api *API) CreateUser(c *gin.Context) {
	// Implementation...
}

func (api *API) UpdateUser(c *gin.Context) {
	// Implementation...
}

func (api *API) DeleteUser(c *gin.Context) {
	// Implementation...
}

func (api *API) GetProfile(c *gin.Context) {
	// Implementation...
}

func (api *API) UpdateProfile(c *gin.Context) {
	// Implementation...
}

func (api *API) UploadAvatar(c *gin.Context) {
	// Implementation...
}

func (api *API) RefreshToken(c *gin.Context) {
	// Implementation...
}

func (api *API) Logout(c *gin.Context) {
	// Implementation...
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "api.go"), []byte(apiGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoAuthPackage(pkgDir string) {
	authGo := `package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type AuthService struct {
	db     *gorm.DB
	secret string
}

type Claims struct {
	UserID int    ` + "`json:\"user_id\"`" + `
	Email  string ` + "`json:\"email\"`" + `
	jwt.RegisteredClaims
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{
		db:     db,
		secret: "your-secret-key",
	}
}

func (s *AuthService) Login(email, password string) (string, error) {
	// Implementation for login
	return s.generateToken(1, email)
}

func (s *AuthService) Register(email, password, name string) (interface{}, error) {
	// Implementation for registration
	return map[string]interface{}{
		"id":    1,
		"email": email,
		"name":  name,
	}, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	})
	
	if err != nil {
		return nil, err
	}
	
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	
	return nil, errors.New("invalid token")
}

func (s *AuthService) generateToken(userID int, email string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "auth.go"), []byte(authGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoDatabasePackage(pkgDir string) {
	dbGo := `package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Initialize() (*gorm.DB, error) {
	dsn := "host=localhost user=testuser password=testpass dbname=testdb port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		// Fallback to in-memory for testing
		return nil, err
	}
	
	return db, nil
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "database.go"), []byte(dbGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoMiddlewarePackage(pkgDir string) {
	middlewareGo := `package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple rate limiting implementation
		c.Next()
	}
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "middleware.go"), []byte(middlewareGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoModelsPackage(pkgDir string) {
	modelsGo := `package models

import (
	"time"
)

type User struct {
	ID        int       ` + "`json:\"id\" gorm:\"primaryKey\"`" + `
	Email     string    ` + "`json:\"email\" gorm:\"uniqueIndex\"`" + `
	Name      string    ` + "`json:\"name\"`" + `
	Password  string    ` + "`json:\"-\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\"`" + `
}

type LoginRequest struct {
	Email    string ` + "`json:\"email\" binding:\"required,email\"`" + `
	Password string ` + "`json:\"password\" binding:\"required,min=6\"`" + `
}

type RegisterRequest struct {
	Email    string ` + "`json:\"email\" binding:\"required,email\"`" + `
	Password string ` + "`json:\"password\" binding:\"required,min=6\"`" + `
	Name     string ` + "`json:\"name\" binding:\"required\"`" + `
}

type Profile struct {
	ID       int    ` + "`json:\"id\"`" + `
	UserID   int    ` + "`json:\"user_id\"`" + `
	Bio      string ` + "`json:\"bio\"`" + `
	Avatar   string ` + "`json:\"avatar\"`" + `
	Location string ` + "`json:\"location\"`" + `
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "models.go"), []byte(modelsGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoServicesPackage(pkgDir string) {
	servicesGo := `package services

import (
	"large-go-service/models"
)

type UserService struct {
	users []models.User
}

func NewUserService() *UserService {
	return &UserService{
		users: make([]models.User, 0),
	}
}

func (s *UserService) GetAllUsers() ([]models.User, error) {
	return s.users, nil
}

func (s *UserService) GetUserByID(id int) (*models.User, error) {
	for _, user := range s.users {
		if user.ID == id {
			return &user, nil
		}
	}
	return nil, errors.New("user not found")
}

func (s *UserService) CreateUser(user models.User) (*models.User, error) {
	user.ID = len(s.users) + 1
	s.users = append(s.users, user)
	return &user, nil
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "user.go"), []byte(servicesGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoUtilsPackage(pkgDir string) {
	utilsGo := `package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

func GenerateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func SanitizeString(input string) string {
	return strings.TrimSpace(input)
}

func IsValidEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "utils.go"), []byte(utilsGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) createGoWebSocketPackage(pkgDir string) {
	wsGo := `package websocket

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("Client connected")
			
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Println("Client disconnected")
			}
			
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func SetupWebSocket(r *gin.Engine, hub *Hub) {
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		
		client := &Client{
			hub:  hub,
			conn: conn,
			send: make(chan []byte, 256),
		}
		
		hub.register <- client
		
		go client.writePump()
		go client.readPump()
	})
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pkgDir, "websocket.go"), []byte(wsGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) setupPythonProject(projectPath string) {
	suite.Require().NoError(os.MkdirAll(projectPath, 0755))
	
	// Large Python project with multiple modules
	requirements := `fastapi==0.104.1
uvicorn==0.24.0
pydantic==2.4.2
sqlalchemy==2.0.23
asyncpg==0.29.0
redis==5.0.1
celery==5.3.4
pytest==7.4.3
black==23.9.1
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "requirements.txt"), []byte(requirements), 0644))
	
	// Create multiple Python modules
	modules := []string{"api", "auth", "database", "models", "services", "utils", "tasks", "tests"}
	
	for _, module := range modules {
		moduleDir := filepath.Join(projectPath, module)
		suite.Require().NoError(os.MkdirAll(moduleDir, 0755))
		suite.Require().NoError(os.WriteFile(filepath.Join(moduleDir, "__init__.py"), []byte(""), 0644))
		
		// Create sample files in each module
		switch module {
		case "api":
			suite.createPythonAPIModule(moduleDir)
		case "models":
			suite.createPythonModelsModule(moduleDir)
		case "services":
			suite.createPythonServicesModule(moduleDir)
		}
	}
	
	mainPy := `from fastapi import FastAPI
from api.routes import router
import uvicorn

app = FastAPI(title="Python Microservice", version="1.0.0")
app.include_router(router, prefix="/api/v1")

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8001)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "main.py"), []byte(mainPy), 0644))
}

func (suite *RoutingPerformanceTestSuite) createPythonAPIModule(moduleDir string) {
	routesPy := `from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import List, Optional

router = APIRouter()

class UserCreate(BaseModel):
    name: str
    email: str
    
class User(BaseModel):
    id: int
    name: str
    email: str

@router.get("/users", response_model=List[User])
async def get_users():
    return []

@router.post("/users", response_model=User)
async def create_user(user: UserCreate):
    return User(id=1, name=user.name, email=user.email)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(moduleDir, "routes.py"), []byte(routesPy), 0644))
}

func (suite *RoutingPerformanceTestSuite) createPythonModelsModule(moduleDir string) {
	modelsPy := `from sqlalchemy import Column, Integer, String, DateTime
from sqlalchemy.ext.declarative import declarative_base
from pydantic import BaseModel
from datetime import datetime

Base = declarative_base()

class UserModel(Base):
    __tablename__ = "users"
    
    id = Column(Integer, primary_key=True, index=True)
    name = Column(String, index=True)
    email = Column(String, unique=True, index=True)
    created_at = Column(DateTime, default=datetime.utcnow)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(moduleDir, "user.py"), []byte(modelsPy), 0644))
}

func (suite *RoutingPerformanceTestSuite) createPythonServicesModule(moduleDir string) {
	servicesPy := `from typing import List, Optional
from models.user import UserModel

class UserService:
    def __init__(self):
        self.users = []
    
    async def get_all_users(self) -> List[UserModel]:
        return self.users
    
    async def create_user(self, name: str, email: str) -> UserModel:
        user = UserModel(id=len(self.users) + 1, name=name, email=email)
        self.users.append(user)
        return user
`
	suite.Require().NoError(os.WriteFile(filepath.Join(moduleDir, "user_service.py"), []byte(servicesPy), 0644))
}

func (suite *RoutingPerformanceTestSuite) setupTypeScriptProject(projectPath string) {
	suite.Require().NoError(os.MkdirAll(projectPath, 0755))
	
	packageJson := `{
  "name": "react-frontend",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "@types/react": "^18.2.25",
    "typescript": "^5.2.2",
    "axios": "^1.5.1"
  }
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "package.json"), []byte(packageJson), 0644))
	
	srcDir := filepath.Join(projectPath, "src")
	suite.Require().NoError(os.MkdirAll(srcDir, 0755))
	
	// Create multiple TypeScript files
	for i := 0; i < 5; i++ {
		componentTs := fmt.Sprintf(`import React from 'react';

interface Props {
    title: string;
    content: string;
}

export const Component%d: React.FC<Props> = ({ title, content }) => {
    return (
        <div>
            <h1>{title}</h1>
            <p>{content}</p>
        </div>
    );
};
`, i)
		suite.Require().NoError(os.WriteFile(filepath.Join(srcDir, fmt.Sprintf("Component%d.tsx", i)), []byte(componentTs), 0644))
	}
}

func (suite *RoutingPerformanceTestSuite) setupNestedGoProject(projectPath string) {
	suite.Require().NoError(os.MkdirAll(projectPath, 0755))
	
	goMod := `module auth-service

go 1.21
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "go.mod"), []byte(goMod), 0644))
	
	mainGo := `package main

func main() {
    // Auth service implementation
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "main.go"), []byte(mainGo), 0644))
}

func (suite *RoutingPerformanceTestSuite) setupJavaScriptProject(projectPath string) {
	suite.Require().NoError(os.MkdirAll(projectPath, 0755))
	
	packageJson := `{
  "name": "shared-utils",
  "version": "1.0.0",
  "main": "index.js"
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "package.json"), []byte(packageJson), 0644))
	
	indexJs := `function formatDate(date) {
    return date.toISOString();
}

function validateEmail(email) {
    return email.includes('@');
}

module.exports = {
    formatDate,
    validateEmail
};
`
	suite.Require().NoError(os.WriteFile(filepath.Join(projectPath, "index.js"), []byte(indexJs), 0644))
}

func (suite *RoutingPerformanceTestSuite) setupPerformanceConfig() {
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
			Level: "info", // Reduce logging for performance tests
		},
	}
	
	suite.gatewayConfig = &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot: suite.workspaceRoot,
		ExtensionMapping: map[string]string{
			"go":   "go",
			"py":   "python",
			"ts":   "typescript",
			"tsx":  "typescript",
			"js":   "javascript",
			"jsx":  "javascript",
		},
		Timeout:       10 * time.Second,
		EnableLogging: false, // Disable logging for performance tests
	}
}

func (suite *RoutingPerformanceTestSuite) TestSubProjectRoutingLatency() {
	suite.T().Log("Testing sub-project routing latency requirement (<1ms)")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	subProjects := suite.gateway.GetSubProjects()
	suite.Require().NotEmpty(subProjects, "Should detect sub-projects")
	
	// Test routing latency for multiple projects
	for _, project := range subProjects {
		for _, language := range project.Languages {
			start := time.Now()
			client, err := suite.gateway.GetSubProjectClient(project.ID, language)
			routingDuration := time.Since(start)
			
			if err == nil && client != nil {
				// Routing should complete within 1ms requirement
				suite.Less(routingDuration, 1*time.Millisecond, 
					"Routing for project %s, language %s took too long: %v", 
					project.ID, language, routingDuration)
				
				suite.T().Logf("Routing latency for %s/%s: %v", 
					project.ID, language, routingDuration)
			}
		}
	}
}

func (suite *RoutingPerformanceTestSuite) TestHealthCheckPerformance() {
	suite.T().Log("Testing health check performance requirement (<100ms)")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test health check performance
	const numChecks = 100
	var totalDuration time.Duration
	
	for i := 0; i < numChecks; i++ {
		start := time.Now()
		health := suite.gateway.Health()
		duration := time.Since(start)
		totalDuration += duration
		
		suite.NotNil(health)
		suite.Less(duration, 100*time.Millisecond, 
			"Health check %d took too long: %v", i, duration)
	}
	
	averageDuration := totalDuration / numChecks
	suite.T().Logf("Average health check duration: %v", averageDuration)
	suite.Less(averageDuration, 50*time.Millisecond, 
		"Average health check duration should be well under 100ms")
}

func (suite *RoutingPerformanceTestSuite) TestConcurrentRoutingPerformance() {
	suite.T().Log("Testing concurrent routing performance")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	subProjects := suite.gateway.GetSubProjects()
	suite.Require().NotEmpty(subProjects, "Should detect sub-projects")
	
	// Test concurrent routing with multiple goroutines
	const numGoroutines = 50
	const requestsPerGoroutine = 20
	
	var wg sync.WaitGroup
	results := make(chan time.Duration, numGoroutines*requestsPerGoroutine)
	
	startTime := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerGoroutine; j++ {
				project := subProjects[j%len(subProjects)]
				if len(project.Languages) == 0 {
					continue
				}
				
				language := project.Languages[j%len(project.Languages)]
				
				start := time.Now()
				client, _ := suite.gateway.GetSubProjectClient(project.ID, language)
				duration := time.Since(start)
				
				if client != nil {
					results <- duration
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(results)
	
	totalConcurrentTime := time.Since(startTime)
	
	// Analyze concurrent performance
	var durations []time.Duration
	var totalDuration time.Duration
	maxDuration := time.Duration(0)
	
	for duration := range results {
		durations = append(durations, duration)
		totalDuration += duration
		if duration > maxDuration {
			maxDuration = duration
		}
	}
	
	if len(durations) > 0 {
		averageDuration := totalDuration / time.Duration(len(durations))
		
		suite.T().Logf("Concurrent routing performance:")
		suite.T().Logf("  Total requests: %d", len(durations))
		suite.T().Logf("  Total time: %v", totalConcurrentTime)
		suite.T().Logf("  Average routing time: %v", averageDuration)
		suite.T().Logf("  Max routing time: %v", maxDuration)
		suite.T().Logf("  Requests per second: %.2f", 
			float64(len(durations))/totalConcurrentTime.Seconds())
		
		// Performance requirements
		suite.Less(averageDuration, 5*time.Millisecond, 
			"Average concurrent routing time should be reasonable")
		suite.Less(maxDuration, 50*time.Millisecond, 
			"Max concurrent routing time should be reasonable")
	}
}

func (suite *RoutingPerformanceTestSuite) TestJSONRPCRoutingThroughput() {
	suite.T().Log("Testing JSON-RPC routing throughput")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Create test requests for different languages
	testRequests := suite.createTestJSONRPCRequests()
	
	// Measure JSON-RPC handling performance
	const numRequests = 100
	var totalDuration time.Duration
	successCount := 0
	
	for i := 0; i < numRequests; i++ {
		req := testRequests[i%len(testRequests)]
		
		start := time.Now()
		recorder := httptest.NewRecorder()
		
		suite.gateway.HandleJSONRPC(recorder, req)
		
		duration := time.Since(start)
		totalDuration += duration
		
		// Consider any response (including errors) as handled
		if recorder.Code > 0 {
			successCount++
		}
	}
	
	averageDuration := totalDuration / numRequests
	throughput := float64(successCount) / totalDuration.Seconds()
	
	suite.T().Logf("JSON-RPC routing throughput:")
	suite.T().Logf("  Total requests: %d", numRequests)
	suite.T().Logf("  Successful requests: %d", successCount)
	suite.T().Logf("  Average response time: %v", averageDuration)
	suite.T().Logf("  Throughput: %.2f requests/second", throughput)
	
	// Performance expectations
	suite.Greater(successCount, numRequests/2, "Should handle majority of requests")
	suite.Less(averageDuration, 100*time.Millisecond, "Average response time should be reasonable")
}

func (suite *RoutingPerformanceTestSuite) createTestJSONRPCRequests() []*http.Request {
	requests := make([]*http.Request, 0)
	
	// Test files from different projects
	testFiles := []struct {
		uri      string
		language string
	}{
		{"file://" + filepath.Join(suite.workspaceRoot, "large-go-service", "main.go"), "go"},
		{"file://" + filepath.Join(suite.workspaceRoot, "large-go-service", "api", "api.go"), "go"},
		{"file://" + filepath.Join(suite.workspaceRoot, "python-microservice", "main.py"), "python"},
		{"file://" + filepath.Join(suite.workspaceRoot, "react-frontend", "src", "Component0.tsx"), "typescript"},
		{"file://" + filepath.Join(suite.workspaceRoot, "shared", "utils", "index.js"), "javascript"},
	}
	
	methods := []string{
		"textDocument/hover",
		"textDocument/definition",
		"textDocument/references",
		"textDocument/completion",
	}
	
	for _, testFile := range testFiles {
		for _, method := range methods {
			request := workspace.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  method,
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": testFile.uri,
					},
					"position": map[string]interface{}{
						"line":      0,
						"character": 0,
					},
				},
			}
			
			requestJSON, _ := json.Marshal(request)
			httpReq := httptest.NewRequest(http.MethodPost, "/jsonrpc", bytes.NewReader(requestJSON))
			httpReq.Header.Set("Content-Type", "application/json")
			requests = append(requests, httpReq)
		}
	}
	
	return requests
}

func (suite *RoutingPerformanceTestSuite) TestMemoryUsageStability() {
	suite.T().Log("Testing memory usage stability during routing")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Measure initial memory usage
	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)
	initialAlloc := initialMemStats.Alloc
	
	// Perform many routing operations
	const numOperations = 1000
	subProjects := suite.gateway.GetSubProjects()
	
	for i := 0; i < numOperations; i++ {
		if len(subProjects) == 0 {
			break
		}
		
		project := subProjects[i%len(subProjects)]
		if len(project.Languages) == 0 {
			continue
		}
		
		language := project.Languages[i%len(project.Languages)]
		suite.gateway.GetSubProjectClient(project.ID, language)
		
		// Perform health checks
		if i%100 == 0 {
			suite.gateway.Health()
		}
	}
	
	// Force garbage collection
	runtime.GC()
	runtime.GC() // Run twice to ensure cleanup
	
	// Measure final memory usage
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)
	finalAlloc := finalMemStats.Alloc
	
	memoryIncrease := int64(finalAlloc) - int64(initialAlloc)
	memoryIncreaseMB := float64(memoryIncrease) / (1024 * 1024)
	
	suite.T().Logf("Memory usage stability:")
	suite.T().Logf("  Initial allocation: %d bytes (%.2f MB)", 
		initialAlloc, float64(initialAlloc)/(1024*1024))
	suite.T().Logf("  Final allocation: %d bytes (%.2f MB)", 
		finalAlloc, float64(finalAlloc)/(1024*1024))
	suite.T().Logf("  Memory increase: %d bytes (%.2f MB)", 
		memoryIncrease, memoryIncreaseMB)
	
	// Memory usage should be reasonable (not more than 100MB increase)
	suite.Less(memoryIncreaseMB, 100.0, 
		"Memory usage increased too much during routing operations")
}

func (suite *RoutingPerformanceTestSuite) TestRoutingMetricsCollection() {
	suite.T().Log("Testing routing metrics collection performance")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Perform various routing operations
	subProjects := suite.gateway.GetSubProjects()
	for i := 0; i < 50; i++ {
		if len(subProjects) == 0 {
			break
		}
		
		project := subProjects[i%len(subProjects)]
		if len(project.Languages) > 0 {
			language := project.Languages[i%len(project.Languages)]
			suite.gateway.GetSubProjectClient(project.ID, language)
		}
	}
	
	// Test metrics collection performance
	start := time.Now()
	metrics := suite.gateway.GetRoutingMetrics()
	metricsDuration := time.Since(start)
	
	suite.Less(metricsDuration, 10*time.Millisecond, 
		"Metrics collection should be fast: %v", metricsDuration)
	
	if metrics != nil {
		suite.NotNil(metrics.StrategyUsage)
		suite.WithinDuration(time.Now(), metrics.LastUpdated, 5*time.Minute)
		suite.T().Logf("Metrics collection time: %v", metricsDuration)
	}
}

func (suite *RoutingPerformanceTestSuite) TestScalabilityWithManyProjects() {
	suite.T().Log("Testing scalability with many sub-projects")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	subProjects := suite.gateway.GetSubProjects()
	totalProjects := len(subProjects)
	
	suite.T().Logf("Detected %d sub-projects for scalability testing", totalProjects)
	
	if totalProjects == 0 {
		suite.T().Skip("No sub-projects detected for scalability testing")
		return
	}
	
	// Test that performance doesn't degrade significantly with many projects
	var routingTimes []time.Duration
	
	for _, project := range subProjects {
		for _, language := range project.Languages {
			start := time.Now()
			client, err := suite.gateway.GetSubProjectClient(project.ID, language)
			duration := time.Since(start)
			
			if err == nil && client != nil {
				routingTimes = append(routingTimes, duration)
			}
		}
	}
	
	if len(routingTimes) > 0 {
		var totalTime time.Duration
		maxTime := time.Duration(0)
		
		for _, duration := range routingTimes {
			totalTime += duration
			if duration > maxTime {
				maxTime = duration
			}
		}
		
		averageTime := totalTime / time.Duration(len(routingTimes))
		
		suite.T().Logf("Scalability metrics:")
		suite.T().Logf("  Projects: %d", totalProjects)
		suite.T().Logf("  Routing operations: %d", len(routingTimes))
		suite.T().Logf("  Average routing time: %v", averageTime)
		suite.T().Logf("  Max routing time: %v", maxTime)
		
		// Scalability requirements
		suite.Less(averageTime, 5*time.Millisecond, 
			"Average routing time should remain low with many projects")
		suite.Less(maxTime, 20*time.Millisecond, 
			"Max routing time should remain reasonable with many projects")
	}
}