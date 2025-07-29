# Multi-Project Workspace

This workspace contains 4 different language projects for comprehensive LSP testing:

## Projects

### Go Service (go-service/)
- **Port**: 8081  
- **Features**: REST API with user management
- **Technologies**: Gorilla Mux, JSON handling, HTTP middleware
- **LSP Testing**: Struct definitions, interface implementations, method receivers

### Python API (python-api/)  
- **Port**: 8082
- **Features**: FastAPI with async/await patterns
- **Technologies**: FastAPI, Pydantic, type hints, dependency injection
- **LSP Testing**: Class hierarchies, async functions, type annotations

### TypeScript Frontend (frontend/)
- **Port**: 8080 (dev server)
- **Features**: React application with user interface
- **Technologies**: React, TypeScript, Axios, generic types
- **LSP Testing**: Interface definitions, generic components, module imports

### Java Backend (java-backend/)
- **Port**: 8083
- **Features**: Spring Boot with JPA repository pattern  
- **Technologies**: Spring Boot, JPA, Maven, REST controllers
- **LSP Testing**: Annotations, inheritance, package structure

## Development

Each project can be run independently:

```bash
# Go Service
cd go-service && go run main.go

# Python API  
cd python-api && pip install -r requirements.txt && python app.py

# TypeScript Frontend
cd frontend && npm install && npm run dev

# Java Backend
cd java-backend && mvn spring-boot:run
```

This workspace is designed for testing LSP Gateway functionality across multiple programming languages simultaneously.