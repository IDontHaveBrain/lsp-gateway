package fixtures

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// FixtureManager manages test data and configuration templates
type FixtureManager struct {
	baseDir  string
	tempDirs []string
	mu       sync.RWMutex
	t        *testing.T
}

// FixtureType represents different types of test fixtures
type FixtureType string

const (
	FixtureTypeConfig       FixtureType = "config"
	FixtureTypeSourceCode   FixtureType = "source"
	FixtureTypeExpectedData FixtureType = "expected"
	FixtureTypeErrorData    FixtureType = "errors"
)

// NewFixtureManager creates a new fixture manager
func NewFixtureManager(t *testing.T, baseDir string) *FixtureManager {
	return &FixtureManager{
		baseDir:  baseDir,
		tempDirs: make([]string, 0),
		t:        t,
	}
}

// CreateTempWorkspace creates a temporary workspace with test files
func (fm *FixtureManager) CreateTempWorkspace() string {
	tempDir, err := os.MkdirTemp("", "lsp-gateway-workspace-*")
	if err != nil {
		fm.t.Fatalf("Failed to create temp workspace: %v", err)
	}

	fm.mu.Lock()
	fm.tempDirs = append(fm.tempDirs, tempDir)
	fm.mu.Unlock()

	// Add cleanup
	fm.t.Cleanup(func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fm.t.Logf("Failed to cleanup temp workspace %s: %v", tempDir, err)
		}
	})

	return tempDir
}

// CreateSourceFiles creates sample source files in the workspace
func (fm *FixtureManager) CreateSourceFiles(workspaceDir string) error {
	sourceFiles := map[string]string{
		"main.go": `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello, World!")
	os.Exit(0)
}

func greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

type Person struct {
	Name string
	Age  int
}

func (p Person) String() string {
	return fmt.Sprintf("%s (%d)", p.Name, p.Age)
}
`,
		"app.py": `#!/usr/bin/env python3

import sys
import json
from typing import List, Dict, Optional

def main():
    """Main function"""
    print("Hello, Python!")
    person = Person("Alice", 30)
    print(person.greet())

class Person:
    def __init__(self, name: str, age: int):
        self.name = name
        self.age = age
    
    def greet(self) -> str:
        return f"Hello, {self.name}!"
    
    def to_dict(self) -> Dict[str, str]:
        return {"name": self.name, "age": str(self.age)}

def process_data(data: List[Dict]) -> Optional[Dict]:
    """Process a list of dictionaries"""
    if not data:
        return None
    
    result = {}
    for item in data:
        if "id" in item:
            result[item["id"]] = item
    
    return result

if __name__ == "__main__":
    main()
`,
		"app.ts": `interface User {
    id: number;
    name: string;
    email: string;
    active: boolean;
}

class UserService {
    private users: User[] = [];

    constructor() {
        this.loadUsers();
    }

    async loadUsers(): Promise<void> {
        try {
            const response = await fetch('/api/users');
            this.users = await response.json();
        } catch (error) {
            console.error('Failed to load users:', error);
        }
    }

    findUserById(id: number): User | undefined {
        return this.users.find(user => user.id === id);
    }

    getActiveUsers(): User[] {
        return this.users.filter(user => user.active);
    }

    async createUser(userData: Omit<User, 'id'>): Promise<User> {
        const newUser: User = {
            id: Math.max(...this.users.map(u => u.id), 0) + 1,
            ...userData
        };
        
        this.users.push(newUser);
        return newUser;
    }
}

export default UserService;
`,
		"Application.java": `package com.example.app;

import java.util.*;
import java.util.stream.Collectors;

public class Application {
    private static final String VERSION = "1.0.0";
    private final List<User> users;
    
    public Application() {
        this.users = new ArrayList<>();
    }
    
    public static void main(String[] args) {
        Application app = new Application();
        app.run();
    }
    
    public void run() {
        System.out.println("Application started, version: " + VERSION);
        loadSampleData();
        processUsers();
    }
    
    private void loadSampleData() {
        users.add(new User(1, "Alice", "alice@example.com", true));
        users.add(new User(2, "Bob", "bob@example.com", false));
        users.add(new User(3, "Charlie", "charlie@example.com", true));
    }
    
    private void processUsers() {
        List<User> activeUsers = getActiveUsers();
        System.out.println("Active users: " + activeUsers.size());
        
        activeUsers.forEach(user -> 
            System.out.println("- " + user.getName() + " (" + user.getEmail() + ")")
        );
    }
    
    public List<User> getActiveUsers() {
        return users.stream()
                .filter(User::isActive)
                .collect(Collectors.toList());
    }
    
    public Optional<User> findUserById(int id) {
        return users.stream()
                .filter(user -> user.getId() == id)
                .findFirst();
    }
    
    public void addUser(User user) {
        if (user != null && user.getName() != null) {
            users.add(user);
        }
    }
}

class User {
    private final int id;
    private final String name;
    private final String email;
    private final boolean active;
    
    public User(int id, String name, String email, boolean active) {
        this.id = id;
        this.name = name;
        this.email = email;
        this.active = active;
    }
    
    public int getId() { return id; }
    public String getName() { return name; }
    public String getEmail() { return email; }
    public boolean isActive() { return active; }
    
    @Override
    public String toString() {
        return String.format("User{id=%d, name='%s', email='%s', active=%s}",
                id, name, email, active);
    }
}
`,
		"index.js": `const express = require('express');
const path = require('path');

class WebServer {
    constructor(port = 3000) {
        this.port = port;
        this.app = express();
        this.routes = [];
        this.setupMiddleware();
        this.setupRoutes();
    }

    setupMiddleware() {
        this.app.use(express.json());
        this.app.use(express.static('public'));
        this.app.use((req, res, next) => {
            console.log(req.method + ' ' + req.path);
            next();
        });
    }

    setupRoutes() {
        this.app.get('/', (req, res) => {
            res.json({ message: 'Hello, World!', version: '1.0.0' });
        });

        this.app.get('/api/users', (req, res) => {
            const users = this.getSampleUsers();
            res.json(users);
        });

        this.app.get('/api/users/:id', (req, res) => {
            const id = parseInt(req.params.id);
            const user = this.findUserById(id);
            
            if (user) {
                res.json(user);
            } else {
                res.status(404).json({ error: 'User not found' });
            }
        });

        this.app.post('/api/users', (req, res) => {
            const userData = req.body;
            const newUser = this.createUser(userData);
            res.status(201).json(newUser);
        });
    }

    getSampleUsers() {
        return [
            { id: 1, name: 'Alice', email: 'alice@example.com', active: true },
            { id: 2, name: 'Bob', email: 'bob@example.com', active: false },
            { id: 3, name: 'Charlie', email: 'charlie@example.com', active: true }
        ];
    }

    findUserById(id) {
        const users = this.getSampleUsers();
        return users.find(user => user.id === id);
    }

    createUser(userData) {
        const users = this.getSampleUsers();
        const maxId = Math.max(...users.map(u => u.id), 0);
        
        return {
            id: maxId + 1,
            name: userData.name,
            email: userData.email,
            active: userData.active !== false
        };
    }

    start() {
        return new Promise((resolve) => {
            this.server = this.app.listen(this.port, () => {
                console.log('Server running on port ' + this.port);
                resolve();
            });
        });
    }

    stop() {
        return new Promise((resolve) => {
            if (this.server) {
                this.server.close(resolve);
            } else {
                resolve();
            }
        });
    }
}

module.exports = WebServer;

if (require.main === module) {
    const server = new WebServer();
    server.start();
}
`,
	}

	for filename, content := range sourceFiles {
		filePath := filepath.Join(workspaceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create source file %s: %w", filename, err)
		}
	}

	return nil
}
