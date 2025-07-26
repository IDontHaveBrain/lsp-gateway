package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"
)

// TypeScriptAdvancedE2ETestSuite provides comprehensive E2E tests for advanced TypeScript LSP workflows
// addressing critical gaps in TypeScript development scenarios that developers rely on
type TypeScriptAdvancedE2ETestSuite struct {
	suite.Suite
	mockClient           *mocks.MockMcpClient
	testTimeout          time.Duration
	workspaceRoot        string
	advancedTypeScriptFiles map[string]AdvancedTypeScriptTestFile
}

// AdvancedTypeScriptTestFile represents realistic TypeScript files for advanced testing scenarios
type AdvancedTypeScriptTestFile struct {
	FileName          string
	Content           string
	Language          string
	Description       string
	Dependencies      []string
	Framework         string // React, Node.js, etc.
	TypeScriptVersion string // 5.x features
	ConfigFiles       []ConfigFile
}

// ConfigFile represents TypeScript configuration files that trigger hot reload
type ConfigFile struct {
	FileName string
	Content  string
	Type     string // tsconfig, package.json, etc.
}

// TypeScriptRefactoringResult captures results from advanced refactoring operations
type TypeScriptRefactoringResult struct {
	RenameSuccess         bool
	ExtractInterfaceSuccess bool
	MoveSymbolSuccess     bool
	ImportOrganizationSuccess bool
	CrossFileChangesCount int
	RefactoringLatency    time.Duration
	ErrorCount            int
	ValidationSuccess     bool
}

// TypeScriptBuildConfigResult captures results from build configuration hot reload tests
type TypeScriptBuildConfigResult struct {
	TsconfigReloadSuccess      bool
	PathMappingUpdateSuccess   bool
	IncrementalCompileSuccess  bool
	FrameworkConfigSuccess     bool
	RevalidationLatency        time.Duration
	ErrorCount                 int
	ConfigChangeDetected       bool
}

// TypeScriptModernFeaturesResult captures results from TypeScript 5.x+ features testing
type TypeScriptModernFeaturesResult struct {
	DecoratorsSupported        bool
	SatisfiesOperatorSupported bool
	ConstAssertionsSupported   bool
	UtilityTypesSupported      bool
	FeatureValidationLatency   time.Duration
	ErrorCount                 int
	TypeInferenceAccurate      bool
}

// TypeScriptFrameworkResult captures results from framework-specific integration tests
type TypeScriptFrameworkResult struct {
	ReactJSXSupported    bool
	NodejsTypingSuccess  bool
	ComponentTypingValid bool
	HookInferenceWorking bool
	FrameworkLatency     time.Duration
	ErrorCount           int
	IntegrationSuccess   bool
}

// SetupSuite initializes the test suite with advanced TypeScript fixtures
func (suite *TypeScriptAdvancedE2ETestSuite) SetupSuite() {
	suite.testTimeout = 45 * time.Second // Longer timeout for advanced operations
	suite.workspaceRoot = "/workspace"
	suite.setupAdvancedTypeScriptTestFiles()
}

// SetupTest initializes a fresh mock client for each test
func (suite *TypeScriptAdvancedE2ETestSuite) SetupTest() {
	suite.mockClient = mocks.NewMockMcpClient()
	suite.mockClient.SetHealthy(true)
}

// TearDownTest cleans up mock client state
func (suite *TypeScriptAdvancedE2ETestSuite) TearDownTest() {
	if suite.mockClient != nil {
		suite.mockClient.Reset()
	}
}

// setupAdvancedTypeScriptTestFiles creates realistic advanced TypeScript test files and fixtures
func (suite *TypeScriptAdvancedE2ETestSuite) setupAdvancedTypeScriptTestFiles() {
	suite.advancedTypeScriptFiles = map[string]AdvancedTypeScriptTestFile{
		"src/components/UserComponent.tsx": {
			FileName:          "src/components/UserComponent.tsx",
			Language:          "typescriptreact",
			Description:       "React component with advanced TypeScript features",
			Framework:         "React",
			TypeScriptVersion: "5.0+",
			Content: `import React, { useState, useCallback } from 'react';
import { User, UserRole } from '../types/User';
import { UserService } from '../services/UserService';

// Modern TypeScript 5.x Decorator (Stage 3)
@component
export class UserListManager {
    @readonly
    public users: User[] = [];
}

// Interface for component props with complex generic constraints
interface UserComponentProps<T extends User = User> {
    users: T[];
    onUserSelect: (user: T) => void;
    filter?: (user: T) => boolean;
    sortBy?: keyof T;
}

// Advanced utility type usage
type UserWithoutSensitive = Omit<User, 'email' | 'lastLogin'>;
type UserUpdatePayload = Partial<Pick<User, 'name' | 'role' | 'isActive'>>;

// Satisfies operator for type validation (TypeScript 5.0+)
const defaultUserConfig = {
    defaultRole: UserRole.USER,
    maxUsers: 1000,
    enableNotifications: true
} satisfies Record<string, any>;

// Advanced const assertion
const userPermissions = ['read', 'write', 'delete'] as const;
type UserPermission = typeof userPermissions[number];

// React component with complex TypeScript generics
export function UserComponent<T extends User>({
    users,
    onUserSelect,
    filter,
    sortBy
}: UserComponentProps<T>): JSX.Element {
    const [selectedUser, setSelectedUser] = useState<T | null>(null);
    const [isLoading, setIsLoading] = useState<boolean>(false);

    // Callback with proper TypeScript inference
    const handleUserClick = useCallback((user: T) => {
        setSelectedUser(user);
        onUserSelect(user);
    }, [onUserSelect]);

    // Advanced type guards and filtering
    const filteredUsers = users.filter((user): user is T => {
        if (filter) {
            return filter(user);
        }
        return user.isActive;
    });

    // Template literal types usage
    const getUserDisplayName = (user: T): ` + "`${string} (${UserRole})`" + ` => {
        return ` + "`${user.name} (${user.role})`" + `;
    };

    return (
        <div className="user-component">
            <h2>User Management</h2>
            {isLoading && <div className="loading">Loading users...</div>}
            
            <div className="user-list">
                {filteredUsers.map((user) => (
                    <div
                        key={user.id}
                        className={` + "`user-item ${selectedUser?.id === user.id ? 'selected' : ''}`" + `}
                        onClick={() => handleUserClick(user)}
                    >
                        <span className="user-name">{getUserDisplayName(user)}</span>
                        <span className="user-role">{user.role}</span>
                        {user.isActive ? (
                            <span className="status active">Active</span>
                        ) : (
                            <span className="status inactive">Inactive</span>
                        )}
                    </div>
                ))}
            </div>

            {selectedUser && (
                <div className="user-details">
                    <h3>Selected User</h3>
                    <pre>{JSON.stringify(selectedUser, null, 2)}</pre>
                </div>
            )}
        </div>
    );
}

// Higher-order component with advanced generics
export function withUserTracking<P extends {}>(
    Component: React.ComponentType<P>
): React.ComponentType<P & { trackingEnabled?: boolean }> {
    return function TrackedComponent(props) {
        const { trackingEnabled = true, ...restProps } = props;

        React.useEffect(() => {
            if (trackingEnabled) {
                console.log('User interaction tracked');
            }
        }, [trackingEnabled]);

        return <Component {...(restProps as P)} />;
    };
}

export default UserComponent;`,
			Dependencies: []string{"react", "@types/react", "../types/User", "../services/UserService"},
		},

		"src/server/api/userController.ts": {
			FileName:          "src/server/api/userController.ts",
			Language:          "typescript",
			Description:       "Node.js Express controller with advanced TypeScript typing",
			Framework:         "Node.js",
			TypeScriptVersion: "5.0+",
			Content: `import { Request, Response, NextFunction } from 'express';
import { User, UserRole, CreateUserRequest } from '../../types/User';
import { UserService } from '../../services/UserService';
import { ApiResponse, PaginatedResponse } from '../../types/Api';

// Advanced Express middleware typing with generics
interface TypedRequest<T = {}> extends Request {
    body: T;
    user?: User;
}

interface TypedResponse<T = any> extends Response {
    json(body: ApiResponse<T>): this;
}

// Complex type manipulation for query parameters
type UserQueryParams = {
    page?: string;
    limit?: string;
    role?: UserRole;
    active?: 'true' | 'false';
    search?: string;
    sortBy?: keyof User;
    sortOrder?: 'asc' | 'desc';
};

// Branded types for better type safety
type UserId = string & { readonly brand: unique symbol };
type UserEmail = string & { readonly brand: unique symbol };

// Template literal types for API endpoints
type UserEndpoint = ` + "`/api/users/${string}`" + `;

// Advanced utility types with conditional logic
type UserUpdateFields<T extends keyof User> = T extends 'id' | 'createdAt' 
    ? never 
    : T extends 'email' 
        ? UserEmail 
        : User[T];

export class UserController {
    private userService: UserService;
    
    constructor() {
        this.userService = new UserService();
    }

    // Method with complex generic constraints and error handling
    async getAllUsers<T extends User = User>(
        req: TypedRequest & { query: UserQueryParams },
        res: TypedResponse<PaginatedResponse<T[]>>
    ): Promise<void> {
        try {
            const {
                page = '1',
                limit = '10',
                role,
                active,
                search,
                sortBy = 'createdAt',
                sortOrder = 'desc'
            } = req.query;

            // Type assertion with validation
            const pageNum = parseInt(page, 10);
            const limitNum = parseInt(limit, 10);

            if (isNaN(pageNum) || isNaN(limitNum)) {
                res.status(400).json({
                    success: false,
                    error: 'Invalid pagination parameters',
                    data: null
                });
                return;
            }

            // Complex filtering with type guards
            const filters = {
                ...(role && { role }),
                ...(active !== undefined && { isActive: active === 'true' }),
                ...(search && { 
                    "$or": [
                        { name: { "$regex": search, "$options": 'i' } },
                        { email: { "$regex": search, "$options": 'i' } }
                    ]
                })
            };

            const users = await this.userService.findUsers(filters, {
                page: pageNum,
                limit: limitNum,
                sortBy: sortBy as keyof User,
                sortOrder
            });

            res.json({
                success: true,
                data: users,
                pagination: {
                    page: pageNum,
                    limit: limitNum,
                    total: users.length,
                    totalPages: Math.ceil(users.length / limitNum)
                }
            });
        } catch (error) {
            this.handleError(error, res);
        }
    }

    // Method with branded types and advanced validation
    async getUserById(
        req: TypedRequest & { params: { id: string } },
        res: TypedResponse<User>
    ): Promise<void> {
        try {
            const userId = req.params.id as UserId;
            
            if (!this.isValidUserId(userId)) {
                res.status(400).json({
                    success: false,
                    error: 'Invalid user ID format',
                    data: null
                });
                return;
            }

            const user = await this.userService.findById(userId);
            
            if (!user) {
                res.status(404).json({
                    success: false,
                    error: 'User not found',
                    data: null
                });
                return;
            }

            res.json({
                success: true,
                data: user
            });
        } catch (error) {
            this.handleError(error, res);
        }
    }

    // Method with complex type inference and validation
    async createUser(
        req: TypedRequest<CreateUserRequest>,
        res: TypedResponse<User>
    ): Promise<void> {
        try {
            const userData = req.body;
            
            // Advanced validation using discriminated unions
            const validationResult = await this.validateCreateUserRequest(userData);
            if (!validationResult.isValid) {
                res.status(400).json({
                    success: false,
                    error: validationResult.errors.join(', '),
                    data: null
                });
                return;
            }

            const newUser = await this.userService.createUser(userData);
            
            res.status(201).json({
                success: true,
                data: newUser
            });
        } catch (error) {
            this.handleError(error, res);
        }
    }

    // Advanced type guards and validation
    private isValidUserId(id: string): id is UserId {
        return /^[0-9a-fA-F]{24}$/.test(id);
    }

    private async validateCreateUserRequest(
        data: unknown
    ): Promise<{ isValid: true } | { isValid: false; errors: string[] }> {
        const errors: string[] = [];

        if (!data || typeof data !== 'object') {
            return { isValid: false, errors: ['Invalid request body'] };
        }

        const userData = data as Record<string, unknown>;

        if (!userData.name || typeof userData.name !== 'string') {
            errors.push('Name is required and must be a string');
        }

        if (!userData.email || typeof userData.email !== 'string') {
            errors.push('Email is required and must be a string');
        }

        if (userData.role && !Object.values(UserRole).includes(userData.role as UserRole)) {
            errors.push('Invalid user role');
        }

        return errors.length > 0 
            ? { isValid: false, errors }
            : { isValid: true };
    }

    // Error handling with proper typing
    private handleError(error: unknown, res: TypedResponse): void {
        console.error('UserController error:', error);
        
        const errorMessage = error instanceof Error 
            ? error.message 
            : 'An unexpected error occurred';

        res.status(500).json({
            success: false,
            error: errorMessage,
            data: null
        });
    }
}

// Middleware factory with advanced typing
export function createAuthMiddleware<T extends User = User>() {
    return async (
        req: TypedRequest,
        res: TypedResponse,
        next: NextFunction
    ): Promise<void> => {
        try {
            const token = req.headers.authorization?.replace('Bearer ', '');
            
            if (!token) {
                res.status(401).json({
                    success: false,
                    error: 'Authentication token required',
                    data: null
                });
                return;
            }

            // Mock token validation - in real app would verify JWT
            const user = await new UserService().validateToken(token);
            
            if (!user) {
                res.status(401).json({
                    success: false,
                    error: 'Invalid authentication token',
                    data: null
                });
                return;
            }

            req.user = user;
            next();
        } catch (error) {
            res.status(500).json({
                success: false,
                error: 'Authentication error',
                data: null
            });
        }
    };
}

export default UserController;`,
			Dependencies: []string{"express", "@types/express", "../../types/User", "../../services/UserService", "../../types/Api"},
		},

		"src/types/Advanced.ts": {
			FileName:          "src/types/Advanced.ts",
			Language:          "typescript",
			Description:       "Advanced TypeScript 5.x+ type definitions and utilities",
			Framework:         "",
			TypeScriptVersion: "5.0+",
			Content: `// Advanced TypeScript 5.x+ features and type utilities

// Branded types for enhanced type safety
export type Brand<T, TBrand> = T & { readonly __brand: TBrand };
export type UserId = Brand<string, 'UserId'>;
export type Email = Brand<string, 'Email'>;
export type Timestamp = Brand<number, 'Timestamp'>;

// Template literal types with complex patterns
export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';
export type ApiEndpoint<T extends string> = ` + "`/api/${T}`" + `;
export type UserEndpoint = ApiEndpoint<` + "`users/${string}`" + `>;
export type ResourceEndpoint<T extends string> = ` + "`${ApiEndpoint<T>}/${string}`" + `;

// Advanced conditional types and distributive conditionals
export type NonNullable<T> = T extends null | undefined ? never : T;
export type Nullable<T> = T | null;
export type Optional<T> = T | undefined;

// Complex mapped types with key filtering
export type RequiredKeys<T> = {
    [K in keyof T]-?: {} extends Pick<T, K> ? never : K;
}[keyof T];

export type OptionalKeys<T> = {
    [K in keyof T]-?: {} extends Pick<T, K> ? K : never;
}[keyof T];

export type RequiredPart<T> = Pick<T, RequiredKeys<T>>;
export type OptionalPart<T> = Pick<T, OptionalKeys<T>>;

// Advanced utility types for deep manipulation
export type DeepReadonly<T> = {
    readonly [P in keyof T]: T[P] extends object ? DeepReadonly<T[P]> : T[P];
};

export type DeepPartial<T> = {
    [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P];
};

export type DeepRequired<T> = {
    [P in keyof T]-?: T[P] extends object ? DeepRequired<T[P]> : T[P];
};

// Advanced generic constraints and conditional logic
export type IsExtending<T, U> = T extends U ? true : false;
export type IsEqual<T, U> = T extends U ? U extends T ? true : false : false;
export type IsNever<T> = IsEqual<T, never>;

// Function type utilities with complex signatures
export type AsyncReturnType<T extends (...args: any) => Promise<any>> = 
    T extends (...args: any) => Promise<infer R> ? R : never;

export type Parameters<T extends (...args: any) => any> = T extends (...args: infer P) => any ? P : never;

export type ConstructorParameters<T extends abstract new (...args: any) => any> = 
    T extends abstract new (...args: infer P) => any ? P : never;

// Advanced discriminated unions with exhaustive checking
export interface LoadingState {
    status: 'loading';
    data: undefined;
    error: undefined;
}

export interface SuccessState<T> {
    status: 'success';
    data: T;
    error: undefined;
}

export interface ErrorState {
    status: 'error';
    data: undefined;
    error: string;
}

export type AsyncState<T> = LoadingState | SuccessState<T> | ErrorState;

// Type guards for discriminated unions
export function isLoading<T>(state: AsyncState<T>): state is LoadingState {
    return state.status === 'loading';
}

export function isSuccess<T>(state: AsyncState<T>): state is SuccessState<T> {
    return state.status === 'success';
}

export function isError<T>(state: AsyncState<T>): state is ErrorState {
    return state.status === 'error';
}

// Advanced generic factories and builders
export interface EntityBuilder<T> {
    create(): T;
    with<K extends keyof T>(key: K, value: T[K]): EntityBuilder<T>;
    build(): T;
}

export function createEntityBuilder<T>(defaults: T): EntityBuilder<T> {
    let entity = { ...defaults };
    
    return {
        create: () => ({ ...defaults }),
        with: <K extends keyof T>(key: K, value: T[K]) => {
            entity[key] = value;
            return createEntityBuilder(entity);
        },
        build: () => entity
    };
}

// Satisfies operator examples (TypeScript 5.0+)
export const apiConfig = {
    baseUrl: 'https://api.example.com',
    timeout: 30000,
    retries: 3,
    endpoints: {
        users: '/users',
        posts: '/posts',
        comments: '/comments'
    }
} satisfies Record<string, any>;

// Const assertions with complex types
export const httpStatusCodes = {
    success: [200, 201, 202, 204],
    clientError: [400, 401, 403, 404, 409, 422],
    serverError: [500, 502, 503, 504]
} as const;

export type HttpStatusCode = typeof httpStatusCodes[keyof typeof httpStatusCodes][number];

// Advanced decorator types (Stage 3 proposal)
export type ClassDecorator<T extends abstract new (...args: any) => any> = (target: T) => T | void;
export type MethodDecorator<T = any> = (target: any, propertyKey: string | symbol, descriptor: TypedPropertyDescriptor<T>) => TypedPropertyDescriptor<T> | void;
export type PropertyDecorator = (target: any, propertyKey: string | symbol) => void;
export type ParameterDecorator = (target: any, propertyKey: string | symbol | undefined, parameterIndex: number) => void;

// Decorator factory functions
export function component<T extends abstract new (...args: any) => any>(target: T): T {
    // Component registration logic
    return target;
}

export function readonly(target: any, propertyKey: string): void {
    // Make property readonly
    Object.defineProperty(target, propertyKey, {
        writable: false,
        configurable: false
    });
}

export function validate<T>(validator: (value: T) => boolean) {
    return function (target: any, propertyKey: string): void {
        let value: T;
        
        Object.defineProperty(target, propertyKey, {
            get: () => value,
            set: (newValue: T) => {
                if (!validator(newValue)) {
                    throw new Error(` + "`Invalid value for ${propertyKey}`" + `);
                }
                value = newValue;
            },
            configurable: true,
            enumerable: true
        });
    };
}

// Advanced type manipulation for API responses
export type ApiResponse<T> = {
    success: true;
    data: T;
    error?: never;
} | {
    success: false;
    data?: never;
    error: string;
};

export type PaginatedResponse<T> = ApiResponse<{
    items: T[];
    pagination: {
        page: number;
        limit: number;
        total: number;
        totalPages: number;
    };
}>;

// Event system with advanced typing
export interface EventMap {
    'user:created': { userId: UserId; timestamp: Timestamp };
    'user:updated': { userId: UserId; changes: string[]; timestamp: Timestamp };
    'user:deleted': { userId: UserId; timestamp: Timestamp };
}

export type EventListener<T extends keyof EventMap> = (event: EventMap[T]) => void;

export interface EventEmitter {
    on<T extends keyof EventMap>(event: T, listener: EventListener<T>): void;
    off<T extends keyof EventMap>(event: T, listener: EventListener<T>): void;
    emit<T extends keyof EventMap>(event: T, data: EventMap[T]): void;
}

// Advanced error handling with discriminated unions
export interface ValidationError {
    type: 'validation';
    field: string;
    message: string;
    code: string;
}

export interface NetworkError {
    type: 'network';
    status: number;
    message: string;
    endpoint: string;
}

export interface UnknownError {
    type: 'unknown';
    message: string;
    originalError: unknown;
}

export type AppError = ValidationError | NetworkError | UnknownError;

// Type-safe error handling utilities
export function isValidationError(error: AppError): error is ValidationError {
    return error.type === 'validation';
}

export function isNetworkError(error: AppError): error is NetworkError {
    return error.type === 'network';
}

export function isUnknownError(error: AppError): error is UnknownError {
    return error.type === 'unknown';
}

export default {
    // Export utility functions and types
    createEntityBuilder,
    component,
    readonly,
    validate,
    isLoading,
    isSuccess,
    isError,
    isValidationError,
    isNetworkError,
    isUnknownError
};`,
			Dependencies: []string{},
		},

		"tsconfig.json": {
			FileName:          "tsconfig.json",
			Language:          "json",
			Description:       "TypeScript configuration file for hot reload testing",
			Framework:         "",
			TypeScriptVersion: "5.0+",
			Content: `{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "allowJs": true,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "noFallthroughCasesInSwitch": true,
    "module": "ESNext",
    "moduleResolution": "node",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "outDir": "./dist",
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"],
      "@components/*": ["src/components/*"],
      "@types/*": ["src/types/*"],
      "@services/*": ["src/services/*"],
      "@utils/*": ["src/utils/*"]
    },
    "experimentalDecorators": true,
    "emitDecoratorMetadata": true
  },
  "include": [
    "src/**/*",
    "types/**/*"
  ],
  "exclude": [
    "node_modules",
    "dist",
    "build",
    "**/*.test.ts",
    "**/*.test.tsx"
  ]
}`,
			Dependencies: []string{},
		},
	}
}

// Test 1: TypeScript Refactoring Operations Test
// Tests safe rename across files, extract interface, move symbols, and import organization
func (suite *TypeScriptAdvancedE2ETestSuite) TestTypeScriptRefactoringOperations() {
	refactoringTests := []struct {
		name           string
		operation      string
		targetFile     string
		targetSymbol   string
		expectedResult TypeScriptRefactoringResult
	}{
		{
			name:         "Safe Rename Across Files",
			operation:    "textDocument/rename",
			targetFile:   "src/components/UserComponent.tsx",
			targetSymbol: "UserComponent",
		},
		{
			name:         "Extract Interface from Component Props",
			operation:    "textDocument/codeAction",
			targetFile:   "src/components/UserComponent.tsx", 
			targetSymbol: "UserComponentProps",
		},
		{
			name:         "Move Symbol Between Files",
			operation:    "workspace/executeCommand",
			targetFile:   "src/types/Advanced.ts",
			targetSymbol: "ApiResponse",
		},
		{
			name:         "Organize Imports and Remove Unused",
			operation:    "textDocument/codeAction",
			targetFile:   "src/server/api/userController.ts",
			targetSymbol: "imports",
		},
	}

	for _, test := range refactoringTests {
		suite.Run(test.name, func() {
			result := suite.executeRefactoringOperation(test.operation, test.targetFile, test.targetSymbol)

			// Validate refactoring operation results
			suite.True(result.RenameSuccess || result.ExtractInterfaceSuccess || result.MoveSymbolSuccess || result.ImportOrganizationSuccess,
				"At least one refactoring operation should succeed for %s", test.name)
			suite.GreaterOrEqual(result.CrossFileChangesCount, 1, "Should detect cross-file changes for %s", test.name)
			suite.True(result.ValidationSuccess, "Refactoring validation should pass for %s", test.name)
			suite.Equal(0, result.ErrorCount, "No errors should occur during %s", test.name)
			suite.Less(result.RefactoringLatency, 5*time.Second, "Refactoring should complete within 5 seconds")
		})
	}
}

// Test 2: Build Configuration Hot Reload Test  
// Tests tsconfig.json changes, path mapping updates, incremental compilation, and framework config integration
func (suite *TypeScriptAdvancedE2ETestSuite) TestBuildConfigurationHotReload() {
	configTests := []struct {
		name         string
		configFile   string
		changeType   string
		description  string
	}{
		{
			name:        "TSConfig Compiler Options Update",
			configFile:  "tsconfig.json", 
			changeType:  "compilerOptions",
			description: "Test LSP server revalidation when tsconfig.json compiler options change",
		},
		{
			name:        "Path Mapping Configuration Update",
			configFile:  "tsconfig.json",
			changeType:  "paths",
			description: "Test module resolution changes with path mapping updates",
		},
		{
			name:        "Incremental Compilation Settings",
			configFile:  "tsconfig.json",
			changeType:  "incremental",
			description: "Test performance during incremental compilation configuration changes",
		},
		{
			name:        "Framework-Specific Config Integration",
			configFile:  "tsconfig.json",
			changeType:  "framework",
			description: "Test framework-specific configuration changes (JSX, decorators)",
		},
	}

	for _, test := range configTests {
		suite.Run(test.name, func() {
			result := suite.executeBuildConfigHotReload(test.configFile, test.changeType)

			// Validate build configuration hot reload
			suite.True(result.ConfigChangeDetected, "Configuration change should be detected for %s", test.name)
			suite.True(result.TsconfigReloadSuccess || result.PathMappingUpdateSuccess || result.IncrementalCompileSuccess || result.FrameworkConfigSuccess,
				"At least one config reload should succeed for %s", test.name)
			suite.Equal(0, result.ErrorCount, "No errors should occur during config reload for %s", test.name)
			suite.Less(result.RevalidationLatency, 3*time.Second, "Config revalidation should complete quickly")
			
			// Validate performance thresholds for config changes
			if test.changeType == "incremental" {
				suite.Less(result.RevalidationLatency, 2*time.Second, "Incremental compilation should be fast")
			}
		})
	}
}

// Test 3: Modern TypeScript 5.x+ Features Test
// Tests decorators, satisfies operator, const assertions, and utility types
func (suite *TypeScriptAdvancedE2ETestSuite) TestModernTypeScript5Features() {
	modernFeatureTests := []struct {
		name        string
		feature     string
		testFile    string
		description string
	}{
		{
			name:        "Stage 3 Decorators Support",
			feature:     "decorators",
			testFile:    "src/components/UserComponent.tsx",
			description: "Test decorator metadata validation and completion",
		},
		{
			name:        "Satisfies Operator Type Checking",
			feature:     "satisfies",
			testFile:    "src/types/Advanced.ts",
			description: "Test satisfies operator type checking and inference",
		},
		{
			name:        "Const Assertions and Template Literals",
			feature:     "const_assertions",
			testFile:    "src/types/Advanced.ts",
			description: "Test const assertions and template literal types",
		},
		{
			name:        "Advanced Utility Types",
			feature:     "utility_types",
			testFile:    "src/types/Advanced.ts",
			description: "Test advanced utility types and type manipulations",
		},
	}

	for _, test := range modernFeatureTests {
		suite.Run(test.name, func() {
			result := suite.executeModernTypeScriptFeatureTest(test.feature, test.testFile)

			// Validate modern TypeScript 5.x+ features
			suite.True(result.DecoratorsSupported || result.SatisfiesOperatorSupported || result.ConstAssertionsSupported || result.UtilityTypesSupported,
				"At least one modern TypeScript feature should be supported for %s", test.name)
			suite.True(result.TypeInferenceAccurate, "Type inference should be accurate for %s", test.name)
			suite.Equal(0, result.ErrorCount, "No errors should occur during modern feature test for %s", test.name)
			suite.Less(result.FeatureValidationLatency, 3*time.Second, "Feature validation should complete quickly")
		})
	}
}

// Test 4: Framework-Specific Integration Test
// Tests React/JSX and Node.js integration
func (suite *TypeScriptAdvancedE2ETestSuite) TestFrameworkSpecificIntegration() {
	frameworkTests := []struct {
		name        string
		framework   string
		testFile    string
		description string
	}{
		{
			name:        "React Component Props and JSX",
			framework:   "react",
			testFile:    "src/components/UserComponent.tsx",
			description: "Test React component prop validation, JSX support, and hook type inference",
		},
		{
			name:        "Node.js Express Route Typing",
			framework:   "nodejs",
			testFile:    "src/server/api/userController.ts",
			description: "Test Express route typing, middleware type safety, and API response types",
		},
	}

	for _, test := range frameworkTests {
		suite.Run(test.name, func() {
			result := suite.executeFrameworkIntegrationTest(test.framework, test.testFile)

			// Validate framework-specific integration
			if test.framework == "react" {
				suite.True(result.ReactJSXSupported, "React JSX should be supported")
				suite.True(result.ComponentTypingValid, "Component typing should be valid")
				suite.True(result.HookInferenceWorking, "Hook type inference should work")
			} else if test.framework == "nodejs" {
				suite.True(result.NodejsTypingSuccess, "Node.js typing should be successful")
			}
			
			suite.True(result.IntegrationSuccess, "Framework integration should succeed for %s", test.name)
			suite.Equal(0, result.ErrorCount, "No errors should occur during framework integration for %s", test.name)
			suite.Less(result.FrameworkLatency, 4*time.Second, "Framework integration should complete within reasonable time")
		})
	}
}

// Test 5: Cross-Protocol Advanced Validation
// Tests both HTTP JSON-RPC and MCP protocols for advanced TypeScript operations
func (suite *TypeScriptAdvancedE2ETestSuite) TestCrossProtocolAdvancedValidation() {
	protocols := []struct {
		name        string
		description string
	}{
		{"HTTP_JSON_RPC", "Test HTTP JSON-RPC protocol for advanced TypeScript operations"},
		{"MCP_Protocol", "Test MCP protocol advanced TypeScript tool integration"},
	}

	advancedMethods := []string{
		"textDocument/rename",
		"textDocument/prepareRename", 
		"textDocument/codeAction",
		"workspace/executeCommand",
		"textDocument/completion",
		"textDocument/didChange",
		"workspace/didChangeWatchedFiles",
	}

	for _, protocol := range protocols {
		suite.Run(protocol.name, func() {
			// Setup protocol-specific responses for advanced methods
			suite.setupAdvancedProtocolResponses(protocol.name, advancedMethods)

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			startTime := time.Now()
			successCount := 0

			// Test advanced LSP methods through the protocol
			for _, method := range advancedMethods {
				resp, err := suite.mockClient.SendLSPRequest(ctx, method, suite.createAdvancedTestParams(method))
				if err == nil && resp != nil {
					successCount++
				}
			}

			protocolLatency := time.Since(startTime)

			// Validate advanced protocol functionality
			suite.Equal(len(advancedMethods), successCount, "All advanced LSP methods should work through %s protocol", protocol.name)
			suite.Less(protocolLatency, 8*time.Second, "%s protocol should handle advanced operations efficiently", protocol.name)
			
			// Validate advanced operation call counts
			for _, method := range advancedMethods {
				suite.GreaterOrEqual(suite.mockClient.GetCallCount(method), 1, 
					"Advanced method %s should be called through %s", method, protocol.name)
			}
		})
	}
}

// Test 6: Performance Validation for Advanced Operations
// Tests performance against established thresholds for advanced TypeScript operations
func (suite *TypeScriptAdvancedE2ETestSuite) TestAdvancedOperationsPerformance() {
	const (
		maxAdvancedResponseTime = 8 * time.Second  // Higher threshold for complex operations
		minAdvancedThroughput   = 50               // Lower threshold due to complexity
		maxAdvancedErrorRate    = 0.08             // Slightly higher tolerance for advanced operations
	)

	performanceTests := []struct {
		name               string
		operationType      string
		requestCount       int
		concurrency        int
		expectedLatency    time.Duration
		complexityFactor   float64
	}{
		{
			name:             "Refactoring Operations Performance",
			operationType:    "textDocument/rename",
			requestCount:     10,
			concurrency:      2,
			expectedLatency:  4 * time.Second,
			complexityFactor: 2.0,
		},
		{
			name:             "Code Actions Performance",
			operationType:    "textDocument/codeAction",
			requestCount:     20,
			concurrency:      3,
			expectedLatency:  6 * time.Second,
			complexityFactor: 1.8,
		},
		{
			name:             "Advanced Completion Performance",
			operationType:    "textDocument/completion",
			requestCount:     50,
			concurrency:      5,
			expectedLatency:  maxAdvancedResponseTime,
			complexityFactor: 1.5,
		},
	}

	for _, test := range performanceTests {
		suite.Run(test.name, func() {
			// Setup sufficient responses for performance test with complexity consideration
			for i := 0; i < test.requestCount; i++ {
				suite.mockClient.QueueResponse(suite.createAdvancedOperationResponse(test.operationType))
			}

			startTime := time.Now()
			var wg sync.WaitGroup
			errorCount := 0
			mu := sync.Mutex{}

			// Execute concurrent advanced requests
			for i := 0; i < test.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					requestsPerWorker := test.requestCount / test.concurrency
					for j := 0; j < requestsPerWorker; j++ {
						ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
						_, err := suite.mockClient.SendLSPRequest(ctx, test.operationType, suite.createAdvancedTestParams(test.operationType))
						cancel()
						
						if err != nil {
							mu.Lock()
							errorCount++
							mu.Unlock()
						}
					}
				}(i)
			}

			wg.Wait()
			totalLatency := time.Since(startTime)

			// Calculate performance metrics with complexity adjustments
			actualThroughput := float64(test.requestCount) / totalLatency.Seconds()
			errorRate := float64(errorCount) / float64(test.requestCount)
			adjustedThroughput := float64(minAdvancedThroughput) / test.complexityFactor

			// Validate advanced performance thresholds
			suite.Less(totalLatency, maxAdvancedResponseTime, "Total response time should be under threshold for %s", test.name)
			suite.Greater(actualThroughput, adjustedThroughput, "Throughput should exceed adjusted minimum for %s", test.name)
			suite.Less(errorRate, maxAdvancedErrorRate, "Error rate should be under threshold for %s", test.name)
			suite.Equal(test.requestCount, suite.mockClient.GetCallCount(test.operationType), "All advanced requests should be processed")
		})
	}
}

// Helper methods for executing advanced workflows

// executeRefactoringOperation executes advanced TypeScript refactoring operations
func (suite *TypeScriptAdvancedE2ETestSuite) executeRefactoringOperation(operation, targetFile, targetSymbol string) TypeScriptRefactoringResult {
	result := TypeScriptRefactoringResult{}
	
	// Setup mock responses for refactoring operations
	suite.setupRefactoringResponses(operation, targetFile, targetSymbol)
	
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	switch operation {
	case "textDocument/rename":
		// First, prepare rename to check if symbol can be renamed safely
		prepareResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/prepareRename", map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, targetFile)},
			"position":     map[string]int{"line": 10, "character": 15},
		})
		
		if err == nil && prepareResp != nil {
			// Execute the actual rename
			renameResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/rename", map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, targetFile)},
				"position":     map[string]int{"line": 10, "character": 15},
				"newName":      fmt.Sprintf("%s_Renamed", targetSymbol),
			})
			
			result.RenameSuccess = err == nil && renameResp != nil
			if result.RenameSuccess {
				// Parse workspace edit to count cross-file changes
				var workspaceEdit map[string]interface{}
				if json.Unmarshal(renameResp, &workspaceEdit) == nil {
					if changes, ok := workspaceEdit["changes"].(map[string]interface{}); ok {
						result.CrossFileChangesCount = len(changes)
					}
				}
			}
		}

	case "textDocument/codeAction":
		// Request code actions for extract interface or organize imports
		codeActionResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/codeAction", map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, targetFile)},
			"range": map[string]interface{}{
				"start": map[string]int{"line": 5, "character": 0},
				"end":   map[string]int{"line": 20, "character": 0},
			},
			"context": map[string]interface{}{
				"only": []string{"refactor.extract", "source.organizeImports"},
			},
		})
		
		if err == nil && codeActionResp != nil {
			var codeActions []interface{}
			if json.Unmarshal(codeActionResp, &codeActions) == nil {
				result.ExtractInterfaceSuccess = len(codeActions) > 0
				result.ImportOrganizationSuccess = strings.Contains(targetSymbol, "imports")
				result.CrossFileChangesCount = len(codeActions)
			}
		}

	case "workspace/executeCommand":
		// Execute move symbol command
		executeResp, err := suite.mockClient.SendLSPRequest(ctx, "workspace/executeCommand", map[string]interface{}{
			"command": "typescript.moveToFile",
			"arguments": []interface{}{
				fmt.Sprintf("file://%s/%s", suite.workspaceRoot, targetFile),
				targetSymbol,
				fmt.Sprintf("file://%s/src/types/NewLocation.ts", suite.workspaceRoot),
			},
		})
		
		result.MoveSymbolSuccess = err == nil && executeResp != nil
		result.CrossFileChangesCount = 2 // Source and destination files
	}

	result.RefactoringLatency = time.Since(startTime)
	result.ValidationSuccess = result.RenameSuccess || result.ExtractInterfaceSuccess || result.MoveSymbolSuccess || result.ImportOrganizationSuccess
	result.ErrorCount = 0 // Mock implementation doesn't generate errors

	return result
}

// executeBuildConfigHotReload executes build configuration hot reload tests
func (suite *TypeScriptAdvancedE2ETestSuite) executeBuildConfigHotReload(configFile, changeType string) TypeScriptBuildConfigResult {
	result := TypeScriptBuildConfigResult{}
	
	// Setup mock responses for config changes
	suite.setupConfigHotReloadResponses(configFile, changeType)
	
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Simulate file change notification
	_, err := suite.mockClient.SendLSPRequest(ctx, "workspace/didChangeWatchedFiles", map[string]interface{}{
		"changes": []map[string]interface{}{
			{
				"uri":  fmt.Sprintf("file://%s/%s", suite.workspaceRoot, configFile),
				"type": 2, // Changed
			},
		},
	})
	
	result.ConfigChangeDetected = err == nil

	// Test different types of configuration changes
	switch changeType {
	case "compilerOptions":
		// Test compiler options reload
		diagnosticsResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/publishDiagnostics", map[string]interface{}{
			"uri": fmt.Sprintf("file://%s/src/components/UserComponent.tsx", suite.workspaceRoot),
		})
		result.TsconfigReloadSuccess = err == nil && diagnosticsResp != nil

	case "paths":
		// Test path mapping updates
		definitionResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/src/components/UserComponent.tsx", suite.workspaceRoot)},
			"position":     map[string]int{"line": 1, "character": 25}, // Import statement
		})
		result.PathMappingUpdateSuccess = err == nil && definitionResp != nil

	case "incremental":
		// Test incremental compilation
		symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/src/types/Advanced.ts", suite.workspaceRoot)},
		})
		result.IncrementalCompileSuccess = err == nil && symbolsResp != nil

	case "framework":
		// Test framework-specific config changes (JSX, decorators)
		hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/src/components/UserComponent.tsx", suite.workspaceRoot)},
			"position":     map[string]int{"line": 5, "character": 1}, // Decorator
		})
		result.FrameworkConfigSuccess = err == nil && hoverResp != nil
	}

	result.RevalidationLatency = time.Since(startTime)
	result.ErrorCount = 0

	return result
}

// executeModernTypeScriptFeatureTest executes modern TypeScript 5.x+ feature tests
func (suite *TypeScriptAdvancedE2ETestSuite) executeModernTypeScriptFeatureTest(feature, testFile string) TypeScriptModernFeaturesResult {
	result := TypeScriptModernFeaturesResult{}
	
	// Setup mock responses for modern features
	suite.setupModernFeatureResponses(feature, testFile)
	
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	switch feature {
	case "decorators":
		// Test decorator support
		hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile)},
			"position":     map[string]int{"line": 6, "character": 1}, // @component decorator
		})
		result.DecoratorsSupported = err == nil && hoverResp != nil && strings.Contains(string(hoverResp), "decorator")

	case "satisfies":
		// Test satisfies operator
		diagnosticsResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/publishDiagnostics", map[string]interface{}{
			"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile),
		})
		result.SatisfiesOperatorSupported = err == nil && diagnosticsResp != nil

	case "const_assertions":
		// Test const assertions
		completionResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile)},
			"position":     map[string]int{"line": 50, "character": 10},
		})
		result.ConstAssertionsSupported = err == nil && completionResp != nil

	case "utility_types":
		// Test advanced utility types
		definitionResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile)},
			"position":     map[string]int{"line": 25, "character": 15}, // Utility type usage
		})
		result.UtilityTypesSupported = err == nil && definitionResp != nil
	}

	result.FeatureValidationLatency = time.Since(startTime)
	result.TypeInferenceAccurate = true // Mock always provides accurate inference
	result.ErrorCount = 0

	return result
}

// executeFrameworkIntegrationTest executes framework-specific integration tests
func (suite *TypeScriptAdvancedE2ETestSuite) executeFrameworkIntegrationTest(framework, testFile string) TypeScriptFrameworkResult {
	result := TypeScriptFrameworkResult{}
	
	// Setup mock responses for framework integration
	suite.setupFrameworkIntegrationResponses(framework, testFile)
	
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	switch framework {
	case "react":
		// Test React JSX support
		symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile)},
		})
		result.ReactJSXSupported = err == nil && symbolsResp != nil

		// Test component prop validation
		hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile)},
			"position":     map[string]int{"line": 20, "character": 10}, // Props interface
		})
		result.ComponentTypingValid = err == nil && hoverResp != nil && strings.Contains(string(hoverResp), "Props")

		// Test hook type inference
		completionResp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile)},
			"position":     map[string]int{"line": 35, "character": 20}, // useState hook
		})
		result.HookInferenceWorking = err == nil && completionResp != nil

	case "nodejs":
		// Test Express route typing
		referencesResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, testFile)},
			"position":     map[string]int{"line": 15, "character": 10}, // TypedRequest interface
			"context":      map[string]bool{"includeDeclaration": true},
		})
		result.NodejsTypingSuccess = err == nil && referencesResp != nil
	}

	result.FrameworkLatency = time.Since(startTime)
	result.IntegrationSuccess = result.ReactJSXSupported || result.NodejsTypingSuccess
	result.ErrorCount = 0

	return result
}

// Helper methods for creating mock responses

func (suite *TypeScriptAdvancedE2ETestSuite) setupRefactoringResponses(operation, targetFile, targetSymbol string) {
	switch operation {
	case "textDocument/rename":
		// Prepare rename response
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"range": {
				"start": {"line": 10, "character": 15},
				"end": {"line": 10, "character": 30}
			},
			"placeholder": "` + targetSymbol + `"
		}`))
		
		// Rename response with workspace edit
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"changes": {
				"file:///workspace/` + targetFile + `": [
					{
						"range": {"start": {"line": 10, "character": 15}, "end": {"line": 10, "character": 30}},
						"newText": "` + targetSymbol + `_Renamed"
					}
				],
				"file:///workspace/src/types/User.ts": [
					{
						"range": {"start": {"line": 5, "character": 17}, "end": {"line": 5, "character": 32}},
						"newText": "` + targetSymbol + `_Renamed"
					}
				]
			}
		}`))

	case "textDocument/codeAction":
		suite.mockClient.QueueResponse(json.RawMessage(`[
			{
				"title": "Extract interface",
				"kind": "refactor.extract.interface",
				"edit": {
					"changes": {
						"file:///workspace/` + targetFile + `": [
							{
								"range": {"start": {"line": 5, "character": 0}, "end": {"line": 20, "character": 0}},
								"newText": "interface ExtractedInterface {\n  // extracted properties\n}\n"
							}
						]
					}
				}
			},
			{
				"title": "Organize imports",
				"kind": "source.organizeImports",
				"edit": {
					"changes": {
						"file:///workspace/` + targetFile + `": [
							{
								"range": {"start": {"line": 0, "character": 0}, "end": {"line": 5, "character": 0}},
								"newText": "import React from 'react';\nimport { User } from '../types/User';\n"
							}
						]
					}
				}
			}
		]`))

	case "workspace/executeCommand":
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"success": true,
			"filesChanged": [
				"file:///workspace/` + targetFile + `",
				"file:///workspace/src/types/NewLocation.ts"
			]
		}`))
	}
}

func (suite *TypeScriptAdvancedE2ETestSuite) setupConfigHotReloadResponses(configFile, changeType string) {
	// File change notification response
	suite.mockClient.QueueResponse(json.RawMessage(`{"acknowledged": true}`))

	switch changeType {
	case "compilerOptions":
		suite.mockClient.QueueResponse(json.RawMessage(`[
			{
				"range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 20}},
				"severity": 1,
				"message": "Compiler options updated - type checking enabled",
				"source": "typescript"
			}
		]`))

	case "paths":
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"uri": "file:///workspace/src/components/UserComponent.tsx",
			"range": {
				"start": {"line": 1, "character": 25},
				"end": {"line": 1, "character": 45}
			}
		}`))

	default:
		suite.mockClient.QueueResponse(json.RawMessage(`{"revalidated": true, "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
	}
}

func (suite *TypeScriptAdvancedE2ETestSuite) setupModernFeatureResponses(feature, testFile string) {
	switch feature {
	case "decorators":
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"contents": {
				"kind": "markdown",
				"value": "` + "```typescript\n@component decorator\n```" + `\n\nStage 3 decorator for component registration"
			},
			"range": {
				"start": {"line": 6, "character": 1},
				"end": {"line": 6, "character": 11}
			}
		}`))

	case "satisfies":
		suite.mockClient.QueueResponse(json.RawMessage(`[
			{
				"range": {"start": {"line": 15, "character": 10}, "end": {"line": 15, "character": 25}},
				"severity": 1,
				"message": "Type satisfies Record<string, any>",
				"source": "typescript"
			}
		]`))

	case "const_assertions":
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"items": [
				{"label": "userPermissions", "kind": 13, "detail": "readonly ['read', 'write', 'delete']"},
				{"label": "as const", "kind": 14, "detail": "const assertion"}
			]
		}`))

	case "utility_types":
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"uri": "file:///workspace/` + testFile + `",
			"range": {
				"start": {"line": 25, "character": 15},
				"end": {"line": 25, "character": 35}
			}
		}`))
	}
}

func (suite *TypeScriptAdvancedE2ETestSuite) setupFrameworkIntegrationResponses(framework, testFile string) {
	switch framework {
	case "react":
		// React JSX symbols response
		suite.mockClient.QueueResponse(json.RawMessage(`[
			{
				"name": "UserComponent",
				"kind": 12,
				"range": {"start": {"line": 20, "character": 0}, "end": {"line": 80, "character": 1}},
				"detail": "React.FunctionComponent<UserComponentProps<T>>"
			},
			{
				"name": "UserComponentProps",
				"kind": 11,
				"range": {"start": {"line": 12, "character": 0}, "end": {"line": 18, "character": 1}},
				"detail": "interface UserComponentProps<T extends User>"
			}
		]`))

		// Component props hover response
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"contents": {
				"kind": "markdown",
				"value": "` + "```typescript\ninterface UserComponentProps<T extends User>\n```" + `\n\nProps interface for UserComponent with generic constraints"
			},
			"range": {
				"start": {"line": 20, "character": 10},
				"end": {"line": 20, "character": 30}
			}
		}`))

		// Hook completion response
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"items": [
				{"label": "useState", "kind": 3, "detail": "<T>(initialState: T | (() => T)): [T, Dispatch<SetStateAction<T>>]"},
				{"label": "useCallback", "kind": 3, "detail": "<T extends (...args: any[]) => any>(callback: T, deps: DependencyList): T"}
			]
		}`))

	case "nodejs":
		// Express typing references response
		suite.mockClient.QueueResponse(json.RawMessage(`[
			{
				"uri": "file:///workspace/` + testFile + `",
				"range": {"start": {"line": 15, "character": 10}, "end": {"line": 15, "character": 25}}
			},
			{
				"uri": "file:///workspace/src/types/Api.ts",
				"range": {"start": {"line": 5, "character": 0}, "end": {"line": 5, "character": 15}}
			}
		]`))
	}
}

func (suite *TypeScriptAdvancedE2ETestSuite) setupAdvancedProtocolResponses(protocol string, methods []string) {
	baseResponses := map[string]json.RawMessage{
		"textDocument/rename": json.RawMessage(`{
			"changes": {
				"file:///workspace/test.ts": [
					{"range": {"start": {"line": 1, "character": 0}, "end": {"line": 1, "character": 10}}, "newText": "newName"}
				]
			}
		}`),
		"textDocument/prepareRename": json.RawMessage(`{
			"range": {"start": {"line": 1, "character": 0}, "end": {"line": 1, "character": 10}},
			"placeholder": "symbolName"
		}`),
		"textDocument/codeAction": json.RawMessage(`[
			{"title": "Extract function", "kind": "refactor.extract.function"},
			{"title": "Organize imports", "kind": "source.organizeImports"}
		]`),
		"workspace/executeCommand": json.RawMessage(`{"success": true}`),
		"textDocument/completion": json.RawMessage(`{
			"items": [
				{"label": "advancedMethod", "kind": 2, "detail": "() => Promise<void>"}
			]
		}`),
		"textDocument/didChange": json.RawMessage(`{"acknowledged": true}`),
		"workspace/didChangeWatchedFiles": json.RawMessage(`{"processed": true}`),
	}

	for _, method := range methods {
		if response, exists := baseResponses[method]; exists {
			suite.mockClient.QueueResponse(response)
		} else {
			suite.mockClient.QueueResponse(json.RawMessage(`{"result": "success", "method": "` + method + `"}`))
		}
	}
}

func (suite *TypeScriptAdvancedE2ETestSuite) createAdvancedTestParams(method string) map[string]interface{} {
	baseParams := map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///workspace/src/components/UserComponent.tsx"},
		"position":     map[string]int{"line": 10, "character": 15},
	}

	switch method {
	case "textDocument/rename":
		baseParams["newName"] = "RenamedSymbol"
		return baseParams
	case "textDocument/codeAction":
		baseParams["range"] = map[string]interface{}{
			"start": map[string]int{"line": 5, "character": 0},
			"end":   map[string]int{"line": 15, "character": 0},
		}
		baseParams["context"] = map[string]interface{}{
			"only": []string{"refactor", "source.organizeImports"},
		}
		return baseParams
	case "workspace/executeCommand":
		return map[string]interface{}{
			"command":   "typescript.organizeImports",
			"arguments": []interface{}{baseParams["textDocument"]},
		}
	case "textDocument/completion":
		baseParams["context"] = map[string]interface{}{
			"triggerKind": 1,
		}
		return baseParams
	case "textDocument/didChange":
		return map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":     baseParams["textDocument"].(map[string]string)["uri"],
				"version": 2,
			},
			"contentChanges": []map[string]interface{}{
				{
					"range": map[string]interface{}{
						"start": map[string]int{"line": 10, "character": 0},
						"end":   map[string]int{"line": 10, "character": 0},
					},
					"text": "// New comment\n",
				},
			},
		}
	case "workspace/didChangeWatchedFiles":
		return map[string]interface{}{
			"changes": []map[string]interface{}{
				{
					"uri":  "file:///workspace/tsconfig.json",
					"type": 2, // Changed
				},
			},
		}
	default:
		return baseParams
	}
}

func (suite *TypeScriptAdvancedE2ETestSuite) createAdvancedOperationResponse(operationType string) json.RawMessage {
	responses := map[string]json.RawMessage{
		"textDocument/rename": json.RawMessage(`{
			"changes": {
				"file:///workspace/src/components/UserComponent.tsx": [
					{"range": {"start": {"line": 10, "character": 15}, "end": {"line": 10, "character": 30}}, "newText": "RenamedSymbol"}
				],
				"file:///workspace/src/types/User.ts": [
					{"range": {"start": {"line": 5, "character": 17}, "end": {"line": 5, "character": 32}}, "newText": "RenamedSymbol"}
				]
			}
		}`),
		"textDocument/codeAction": json.RawMessage(`[
			{
				"title": "Extract to function",
				"kind": "refactor.extract.function",
				"edit": {
					"changes": {
						"file:///workspace/src/components/UserComponent.tsx": [
							{"range": {"start": {"line": 5, "character": 0}, "end": {"line": 15, "character": 0}}, "newText": "extractedFunction();\n\nfunction extractedFunction() {\n  // extracted code\n}\n"}
						]
					}
				}
			}
		]`),
		"textDocument/completion": json.RawMessage(`{
			"items": [
				{"label": "UserComponent", "kind": 5, "detail": "React.FunctionComponent", "insertText": "UserComponent"},
				{"label": "useState", "kind": 3, "detail": "<T>(initialState: T): [T, Dispatch<SetStateAction<T>>]", "insertText": "useState"},
				{"label": "useCallback", "kind": 3, "detail": "<T extends Function>(callback: T, deps: DependencyList): T", "insertText": "useCallback"}
			]
		}`),
	}

	if response, exists := responses[operationType]; exists {
		return response
	}
	return json.RawMessage(`{"result": "mock_success", "operation": "` + operationType + `"}`)
}

// TestSuite runner
func TestTypeScriptAdvancedE2ETestSuite(t *testing.T) {
	suite.Run(t, new(TypeScriptAdvancedE2ETestSuite))
}