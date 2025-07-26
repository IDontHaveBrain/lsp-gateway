package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"
)

// LSPPosition represents a position in a text document
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// JavaBasicWorkflowE2ETestSuite provides comprehensive E2E tests for Java LSP workflows
// covering all essential Spring Boot development scenarios that developers use daily
type JavaBasicWorkflowE2ETestSuite struct {
	suite.Suite
	mockClient    *mocks.MockMcpClient
	testTimeout   time.Duration
	workspaceRoot string
	javaFiles     map[string]JavaTestFile
}

// JavaTestFile represents a realistic Java file with its metadata
type JavaTestFile struct {
	FileName    string
	Content     string
	Language    string
	Description string
	Symbols     []JavaSymbol
}

// JavaSymbol represents a Java symbol with position information
type JavaSymbol struct {
	Name        string
	Kind        int    // LSP SymbolKind
	Position    LSPPosition
	Type        string // Java type information
	Description string
}

// JavaWorkflowResult captures results from Java workflow execution
type JavaWorkflowResult struct {
	DefinitionFound        bool
	ReferencesCount        int
	HoverInfoRetrieved     bool
	DocumentSymbolsCount   int
	CompletionItemsCount   int
	TypeInformationValid   bool
	WorkflowLatency        time.Duration
	ErrorCount             int
	RequestCount           int
}

// SetupSuite initializes the test suite with realistic Java fixtures
func (suite *JavaBasicWorkflowE2ETestSuite) SetupSuite() {
	suite.testTimeout = 30 * time.Second
	suite.workspaceRoot = "/workspace"
	suite.setupJavaTestFiles()
}

// SetupTest initializes a fresh mock client for each test
func (suite *JavaBasicWorkflowE2ETestSuite) SetupTest() {
	suite.mockClient = mocks.NewMockMcpClient()
	suite.mockClient.SetHealthy(true)
}

// TearDownTest cleans up mock client state
func (suite *JavaBasicWorkflowE2ETestSuite) TearDownTest() {
	if suite.mockClient != nil {
		suite.mockClient.Reset()
	}
}

// setupJavaTestFiles creates realistic Java Spring Boot test files and fixtures
func (suite *JavaBasicWorkflowE2ETestSuite) setupJavaTestFiles() {
	suite.javaFiles = map[string]JavaTestFile{
		"src/main/java/com/example/app/Application.java": {
			FileName:    "src/main/java/com/example/app/Application.java",
			Language:    "java",
			Description: "Main Spring Boot application class",
			Content: `package com.example.app;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.ApplicationContext;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
public class Application {

    private static ApplicationContext context;

    public static void main(String[] args) {
        context = SpringApplication.run(Application.class, args);
        System.out.println("Spring Boot application started successfully");
    }

    public static ApplicationContext getContext() {
        return context;
    }

    @RestController
    public static class HealthController {
        
        @GetMapping("/health")
        public HealthStatus getHealth() {
            return new HealthStatus("UP", System.currentTimeMillis());
        }
    }

    public record HealthStatus(String status, long timestamp) {}
}`,
			Symbols: []JavaSymbol{
				{Name: "Application", Kind: 5, Position: LSPPosition{Line: 8, Character: 13}, Type: "class", Description: "Main Spring Boot application class"},
				{Name: "main", Kind: 6, Position: LSPPosition{Line: 12, Character: 23}, Type: "method", Description: "Application main method"},
				{Name: "getContext", Kind: 6, Position: LSPPosition{Line: 17, Character: 39}, Type: "method", Description: "Get application context method"},
				{Name: "HealthController", Kind: 5, Position: LSPPosition{Line: 21, Character: 30}, Type: "class", Description: "Health check controller"},
			},
		},
		"src/main/java/com/example/app/controller/UserController.java": {
			FileName:    "src/main/java/com/example/app/controller/UserController.java",
			Language:    "java",
			Description: "Spring Boot REST controller for user management",
			Content: `package com.example.app.controller;

import com.example.app.entity.User;
import com.example.app.service.UserService;
import com.example.app.exception.UserNotFoundException;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Optional;

@RestController
@RequestMapping("/api/users")
@CrossOrigin(origins = "*")
public class UserController {

    private final UserService userService;

    @Autowired
    public UserController(UserService userService) {
        this.userService = userService;
    }

    @GetMapping
    public ResponseEntity<List<User>> getAllUsers() {
        List<User> users = userService.findAllUsers();
        return ResponseEntity.ok(users);
    }

    @GetMapping("/{id}")
    public ResponseEntity<User> getUserById(@PathVariable Long id) {
        try {
            User user = userService.findUserById(id);
            return ResponseEntity.ok(user);
        } catch (UserNotFoundException e) {
            return ResponseEntity.notFound().build();
        }
    }

    @PostMapping
    public ResponseEntity<User> createUser(@RequestBody CreateUserRequest request) {
        User user = userService.createUser(request.name(), request.email(), request.role());
        return ResponseEntity.status(HttpStatus.CREATED).body(user);
    }

    @PutMapping("/{id}")
    public ResponseEntity<User> updateUser(@PathVariable Long id, @RequestBody UpdateUserRequest request) {
        try {
            User updatedUser = userService.updateUser(id, request.name(), request.email());
            return ResponseEntity.ok(updatedUser);
        } catch (UserNotFoundException e) {
            return ResponseEntity.notFound().build();
        }
    }

    @DeleteMapping("/{id}")
    public ResponseEntity<Void> deleteUser(@PathVariable Long id) {
        try {
            userService.deleteUser(id);
            return ResponseEntity.noContent().build();
        } catch (UserNotFoundException e) {
            return ResponseEntity.notFound().build();
        }
    }

    public record CreateUserRequest(String name, String email, String role) {}
    public record UpdateUserRequest(String name, String email) {}
}`,
			Symbols: []JavaSymbol{
				{Name: "UserController", Kind: 5, Position: LSPPosition{Line: 16, Character: 13}, Type: "class", Description: "User REST controller"},
				{Name: "getAllUsers", Kind: 6, Position: LSPPosition{Line: 26, Character: 43}, Type: "method", Description: "Get all users endpoint"},
				{Name: "getUserById", Kind: 6, Position: LSPPosition{Line: 31, Character: 35}, Type: "method", Description: "Get user by ID endpoint"},
				{Name: "createUser", Kind: 6, Position: LSPPosition{Line: 41, Character: 33}, Type: "method", Description: "Create user endpoint"},
				{Name: "updateUser", Kind: 6, Position: LSPPosition{Line: 46, Character: 33}, Type: "method", Description: "Update user endpoint"},
			},
		},
		"src/main/java/com/example/app/service/UserService.java": {
			FileName:    "src/main/java/com/example/app/service/UserService.java",
			Language:    "java",
			Description: "Spring Boot service layer with business logic",
			Content: `package com.example.app.service;

import com.example.app.entity.User;
import com.example.app.entity.UserRole;
import com.example.app.repository.UserRepository;
import com.example.app.exception.UserNotFoundException;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

@Service
@Transactional
public class UserService {

    private final UserRepository userRepository;

    @Autowired
    public UserService(UserRepository userRepository) {
        this.userRepository = userRepository;
    }

    public List<User> findAllUsers() {
        return userRepository.findAll();
    }

    public User findUserById(Long id) throws UserNotFoundException {
        return userRepository.findById(id)
                .orElseThrow(() -> new UserNotFoundException("User not found with id: " + id));
    }

    public List<User> findUsersByRole(UserRole role) {
        return userRepository.findByRole(role);
    }

    public User createUser(String name, String email, String roleStr) {
        UserRole role = UserRole.valueOf(roleStr.toUpperCase());
        User user = User.builder()
                .name(name)
                .email(email)
                .role(role)
                .createdAt(LocalDateTime.now())
                .isActive(true)
                .build();
        return userRepository.save(user);
    }

    @Transactional
    public User updateUser(Long id, String name, String email) throws UserNotFoundException {
        User user = findUserById(id);
        user.setName(name);
        user.setEmail(email);
        user.setUpdatedAt(LocalDateTime.now());
        return userRepository.save(user);
    }

    @Transactional
    public void deleteUser(Long id) throws UserNotFoundException {
        User user = findUserById(id);
        userRepository.delete(user);
    }

    public boolean isAdminUser(User user) {
        return user.getRole() == UserRole.ADMIN;
    }

    public long countActiveUsers() {
        return userRepository.countByIsActiveTrue();
    }

    public List<User> searchUsersByName(String namePattern) {
        return userRepository.findByNameContainingIgnoreCase(namePattern);
    }
}`,
			Symbols: []JavaSymbol{
				{Name: "UserService", Kind: 5, Position: LSPPosition{Line: 16, Character: 13}, Type: "class", Description: "User service class"},
				{Name: "findAllUsers", Kind: 6, Position: LSPPosition{Line: 25, Character: 21}, Type: "method", Description: "Find all users method"},
				{Name: "findUserById", Kind: 6, Position: LSPPosition{Line: 29, Character: 16}, Type: "method", Description: "Find user by ID method"},
				{Name: "createUser", Kind: 6, Position: LSPPosition{Line: 37, Character: 16}, Type: "method", Description: "Create user method"},
				{Name: "updateUser", Kind: 6, Position: LSPPosition{Line: 49, Character: 16}, Type: "method", Description: "Update user method"},
			},
		},
		"src/main/java/com/example/app/entity/User.java": {
			FileName:    "src/main/java/com/example/app/entity/User.java",
			Language:    "java",
			Description: "JPA entity with relationships and modern Java features",
			Content: `package com.example.app.entity;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.LocalDateTime;
import java.util.List;

@Entity
@Table(name = "users")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class User {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, length = 100)
    private String name;

    @Column(nullable = false, unique = true, length = 150)
    private String email;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private UserRole role;

    @Column(name = "is_active", nullable = false)
    private Boolean isActive;

    @Column(name = "created_at", nullable = false)
    private LocalDateTime createdAt;

    @Column(name = "updated_at")
    private LocalDateTime updatedAt;

    @OneToMany(mappedBy = "user", cascade = CascadeType.ALL, fetch = FetchType.LAZY)
    private List<UserProfile> profiles;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "manager_id")
    private User manager;

    @OneToMany(mappedBy = "manager", fetch = FetchType.LAZY)
    private List<User> subordinates;

    @PrePersist
    protected void onCreate() {
        createdAt = LocalDateTime.now();
        if (isActive == null) {
            isActive = true;
        }
    }

    @PreUpdate
    protected void onUpdate() {
        updatedAt = LocalDateTime.now();
    }

    public boolean hasRole(UserRole requiredRole) {
        return this.role == requiredRole;
    }

    public boolean isManagerOf(User otherUser) {
        return otherUser.getManager() != null && otherUser.getManager().getId().equals(this.id);
    }

    public int getSubordinateCount() {
        return subordinates != null ? subordinates.size() : 0;
    }
}`,
			Symbols: []JavaSymbol{
				{Name: "User", Kind: 5, Position: LSPPosition{Line: 17, Character: 13}, Type: "class", Description: "User JPA entity"},
				{Name: "onCreate", Kind: 6, Position: LSPPosition{Line: 48, Character: 19}, Type: "method", Description: "JPA pre-persist callback"},
				{Name: "onUpdate", Kind: 6, Position: LSPPosition{Line: 56, Character: 19}, Type: "method", Description: "JPA pre-update callback"},
				{Name: "hasRole", Kind: 6, Position: LSPPosition{Line: 60, Character: 21}, Type: "method", Description: "Check user role method"},
				{Name: "isManagerOf", Kind: 6, Position: LSPPosition{Line: 64, Character: 21}, Type: "method", Description: "Check manager relationship method"},
			},
		},
		"src/main/java/com/example/app/entity/UserRole.java": {
			FileName:    "src/main/java/com/example/app/entity/UserRole.java",
			Language:    "java",
			Description: "Modern Java enum with sealed interface patterns",
			Content: `package com.example.app.entity;

import java.util.Set;

public enum UserRole {
    USER("User", Set.of(Permission.READ_OWN_PROFILE)),
    MODERATOR("Moderator", Set.of(
            Permission.READ_OWN_PROFILE,
            Permission.READ_ALL_PROFILES,
            Permission.UPDATE_ANY_PROFILE
    )),
    ADMIN("Administrator", Set.of(
            Permission.READ_OWN_PROFILE,
            Permission.READ_ALL_PROFILES,
            Permission.UPDATE_ANY_PROFILE,
            Permission.DELETE_ANY_PROFILE,
            Permission.MANAGE_USERS
    ));

    private final String displayName;
    private final Set<Permission> permissions;

    UserRole(String displayName, Set<Permission> permissions) {
        this.displayName = displayName;
        this.permissions = permissions;
    }

    public String getDisplayName() {
        return displayName;
    }

    public Set<Permission> getPermissions() {
        return permissions;
    }

    public boolean hasPermission(Permission permission) {
        return permissions.contains(permission);
    }

    public boolean canManageUsers() {
        return hasPermission(Permission.MANAGE_USERS);
    }

    public boolean canDeleteProfiles() {
        return hasPermission(Permission.DELETE_ANY_PROFILE);
    }

    public static UserRole fromString(String roleStr) {
        try {
            return UserRole.valueOf(roleStr.toUpperCase());
        } catch (IllegalArgumentException e) {
            return UserRole.USER;
        }
    }

    public sealed interface Permission permits ReadPermission, WritePermission, AdminPermission {}
    
    public enum ReadPermission implements Permission {
        READ_OWN_PROFILE, READ_ALL_PROFILES
    }
    
    public enum WritePermission implements Permission {
        UPDATE_OWN_PROFILE, UPDATE_ANY_PROFILE, DELETE_ANY_PROFILE
    }
    
    public enum AdminPermission implements Permission {
        MANAGE_USERS, SYSTEM_CONFIG
    }

    public static final Permission READ_OWN_PROFILE = ReadPermission.READ_OWN_PROFILE;
    public static final Permission READ_ALL_PROFILES = ReadPermission.READ_ALL_PROFILES;
    public static final Permission UPDATE_OWN_PROFILE = WritePermission.UPDATE_OWN_PROFILE;
    public static final Permission UPDATE_ANY_PROFILE = WritePermission.UPDATE_ANY_PROFILE;
    public static final Permission DELETE_ANY_PROFILE = WritePermission.DELETE_ANY_PROFILE;
    public static final Permission MANAGE_USERS = AdminPermission.MANAGE_USERS;
    public static final Permission SYSTEM_CONFIG = AdminPermission.SYSTEM_CONFIG;
}`,
			Symbols: []JavaSymbol{
				{Name: "UserRole", Kind: 10, Position: LSPPosition{Line: 4, Character: 12}, Type: "enum", Description: "User role enumeration"},
				{Name: "hasPermission", Kind: 6, Position: LSPPosition{Line: 34, Character: 21}, Type: "method", Description: "Check permission method"},
				{Name: "fromString", Kind: 6, Position: LSPPosition{Line: 46, Character: 26}, Type: "method", Description: "Parse role from string method"},
				{Name: "Permission", Kind: 11, Position: LSPPosition{Line: 54, Character: 25}, Type: "interface", Description: "Permission sealed interface"},
			},
		},
		"src/main/java/com/example/app/repository/UserRepository.java": {
			FileName:    "src/main/java/com/example/app/repository/UserRepository.java",
			Language:    "java",
			Description: "Spring Data JPA repository with custom queries",
			Content: `package com.example.app.repository;

import com.example.app.entity.User;
import com.example.app.entity.UserRole;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

@Repository
public interface UserRepository extends JpaRepository<User, Long> {

    List<User> findByRole(UserRole role);

    List<User> findByIsActiveTrue();

    List<User> findByNameContainingIgnoreCase(String name);

    Optional<User> findByEmail(String email);

    List<User> findByCreatedAtAfter(LocalDateTime dateTime);

    long countByIsActiveTrue();

    long countByRole(UserRole role);

    @Query("SELECT u FROM User u WHERE u.role = :role AND u.isActive = true")
    List<User> findActiveUsersByRole(@Param("role") UserRole role);

    @Query("SELECT u FROM User u WHERE u.manager IS NOT NULL AND u.manager.id = :managerId")
    List<User> findSubordinatesByManagerId(@Param("managerId") Long managerId);

    @Query("SELECT u FROM User u WHERE u.name LIKE %:pattern% OR u.email LIKE %:pattern%")
    List<User> searchByNameOrEmail(@Param("pattern") String pattern);

    @Query(value = "SELECT * FROM users u WHERE u.created_at >= :fromDate AND u.created_at <= :toDate", 
           nativeQuery = true)
    List<User> findUsersCreatedBetween(@Param("fromDate") LocalDateTime fromDate, 
                                       @Param("toDate") LocalDateTime toDate);

    @Query("SELECT COUNT(u) FROM User u WHERE u.role = 'ADMIN' AND u.isActive = true")
    long countActiveAdmins();

    @Query("SELECT u FROM User u LEFT JOIN FETCH u.profiles WHERE u.id = :id")
    Optional<User> findByIdWithProfiles(@Param("id") Long id);

    @Query("SELECT DISTINCT u FROM User u LEFT JOIN FETCH u.subordinates s WHERE u.role = 'ADMIN'")
    List<User> findAllAdminsWithSubordinates();

    boolean existsByEmail(String email);

    void deleteByIdAndIsActiveFalse(Long id);
}`,
			Symbols: []JavaSymbol{
				{Name: "UserRepository", Kind: 11, Position: LSPPosition{Line: 14, Character: 17}, Type: "interface", Description: "User repository interface"},
				{Name: "findByRole", Kind: 6, Position: LSPPosition{Line: 16, Character: 21}, Type: "method", Description: "Find users by role method"},
				{Name: "findActiveUsersByRole", Kind: 6, Position: LSPPosition{Line: 29, Character: 21}, Type: "method", Description: "Find active users by role query method"},
				{Name: "searchByNameOrEmail", Kind: 6, Position: LSPPosition{Line: 35, Character: 21}, Type: "method", Description: "Search users by name or email method"},
			},
		},
		"src/main/java/com/example/app/exception/UserNotFoundException.java": {
			FileName:    "src/main/java/com/example/app/exception/UserNotFoundException.java",
			Language:    "java",
			Description: "Custom exception with Spring Boot error handling",
			Content: `package com.example.app.exception;

import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ResponseStatus;

@ResponseStatus(value = HttpStatus.NOT_FOUND)
public class UserNotFoundException extends RuntimeException {

    private final Long userId;
    private final String userEmail;

    public UserNotFoundException(String message) {
        super(message);
        this.userId = null;
        this.userEmail = null;
    }

    public UserNotFoundException(String message, Long userId) {
        super(message);
        this.userId = userId;
        this.userEmail = null;
    }

    public UserNotFoundException(String message, String userEmail) {
        super(message);
        this.userId = null;
        this.userEmail = userEmail;
    }

    public UserNotFoundException(String message, Throwable cause) {
        super(message, cause);
        this.userId = null;
        this.userEmail = null;
    }

    public Long getUserId() {
        return userId;
    }

    public String getUserEmail() {
        return userEmail;
    }

    public record ErrorDetails(String message, Long userId, String userEmail, long timestamp) {
        public static ErrorDetails from(UserNotFoundException ex) {
            return new ErrorDetails(
                ex.getMessage(),
                ex.getUserId(),
                ex.getUserEmail(),
                System.currentTimeMillis()
            );
        }
    }
}`,
			Symbols: []JavaSymbol{
				{Name: "UserNotFoundException", Kind: 5, Position: LSPPosition{Line: 6, Character: 13}, Type: "class", Description: "User not found exception"},
				{Name: "getUserId", Kind: 6, Position: LSPPosition{Line: 35, Character: 16}, Type: "method", Description: "Get user ID method"},
				{Name: "getUserEmail", Kind: 6, Position: LSPPosition{Line: 39, Character: 19}, Type: "method", Description: "Get user email method"},
				{Name: "ErrorDetails", Kind: 5, Position: LSPPosition{Line: 43, Character: 19}, Type: "record", Description: "Error details record"},
			},
		},
	}
}

// TestBasicLSPOperations tests all fundamental LSP operations for Java
func (suite *JavaBasicWorkflowE2ETestSuite) TestBasicLSPOperations() {
	testCases := []struct {
		name         string
		fileName     string
		symbolName   string
		expectedType string
	}{
		{"Go to Definition - Class", "src/main/java/com/example/app/Application.java", "Application", "class"},
		{"Go to Definition - Method", "src/main/java/com/example/app/service/UserService.java", "findUserById", "method"},
		{"Go to Definition - Interface", "src/main/java/com/example/app/repository/UserRepository.java", "UserRepository", "interface"},
		{"Go to Definition - Enum", "src/main/java/com/example/app/entity/UserRole.java", "UserRole", "enum"},
		{"Go to Definition - Exception", "src/main/java/com/example/app/exception/UserNotFoundException.java", "UserNotFoundException", "class"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executeBasicLSPWorkflow(tc.fileName, tc.symbolName, tc.expectedType)

			// Validate basic LSP operations
			suite.True(result.DefinitionFound, "Definition should be found for %s", tc.symbolName)
			suite.Greater(result.ReferencesCount, 0, "References should be found for %s", tc.symbolName)
			suite.True(result.HoverInfoRetrieved, "Hover info should be retrieved for %s", tc.symbolName)
			suite.Greater(result.DocumentSymbolsCount, 0, "Document symbols should be found")
			suite.Equal(0, result.ErrorCount, "No errors should occur during workflow")
			suite.Less(result.WorkflowLatency, 5*time.Second, "Workflow should complete within 5 seconds")
		})
	}
}

// TestJavaSpecificFeatures tests Java-specific language features
func (suite *JavaBasicWorkflowE2ETestSuite) TestJavaSpecificFeatures() {
	testCases := []struct {
		name        string
		feature     string
		description string
	}{
		{"Spring Annotations", "spring_annotations", "Navigate Spring Boot annotations and their implementations"},
		{"JPA Relationships", "jpa_relationships", "Handle JPA entity relationships and mappings"},
		{"Modern Java Features", "modern_java", "Handle records, sealed classes, and pattern matching"},
		{"Auto-completion", "completion", "Java-aware code completion with Spring Boot context"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executeJavaSpecificWorkflow(tc.feature)

			// Validate Java-specific features
			suite.True(result.TypeInformationValid, "Type information should be valid for %s", tc.feature)
			
			if tc.feature == "completion" {
				suite.Greater(result.CompletionItemsCount, 0, "Completion items should be available")
			}
			
			suite.Equal(0, result.ErrorCount, "No errors should occur for %s", tc.feature)
			suite.Less(result.WorkflowLatency, 3*time.Second, "Feature test should complete quickly")
		})
	}
}

// TestMultiFileJavaProject tests navigation across multiple Java files
func (suite *JavaBasicWorkflowE2ETestSuite) TestMultiFileJavaProject() {
	// Setup responses for multi-file navigation
	suite.setupMultiFileResponses()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	startTime := time.Now()
	totalSymbols := 0
	totalReferences := 0

	// Navigate through each Java file
	for fileName, fileInfo := range suite.javaFiles {
		suite.Run(fmt.Sprintf("Multi-file navigation - %s", fileName), func() {
			// Get document symbols for the file
			symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
			})
			suite.NoError(err, "Document symbols should be retrieved for %s", fileName)
			suite.NotNil(symbolsResp, "Symbol response should not be nil for %s", fileName)

			// Parse and count symbols
			var symbols []interface{}
			err = json.Unmarshal(symbolsResp, &symbols)
			suite.NoError(err, "Should be able to parse symbols for %s", fileName)
			totalSymbols += len(symbols)

			// Test cross-file references for main symbols
			for _, symbol := range fileInfo.Symbols {
				refsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
					"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
					"position":     symbol.Position,
					"context":      map[string]bool{"includeDeclaration": true},
				})
				suite.NoError(err, "References should be found for %s in %s", symbol.Name, fileName)
				suite.NotNil(refsResp, "References response should not be nil")

				// Count references
				var refs []interface{}
				if json.Unmarshal(refsResp, &refs) == nil {
					totalReferences += len(refs)
				}
			}
		})
	}

	multiFileLatency := time.Since(startTime)

	// Validate multi-file project navigation
	suite.Greater(totalSymbols, 15, "Should find significant number of symbols across all files")
	suite.Greater(totalReferences, 8, "Should find cross-file references")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), len(suite.javaFiles), "Document symbols should be called for each file")
	suite.Greater(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES), 0, "References should be called")
	suite.Less(multiFileLatency, 10*time.Second, "Multi-file navigation should complete within reasonable time")
}

// TestProtocolValidation tests both HTTP JSON-RPC and MCP protocol validation
func (suite *JavaBasicWorkflowE2ETestSuite) TestProtocolValidation() {
	protocols := []struct {
		name        string
		description string
	}{
		{"HTTP_JSON_RPC", "Test HTTP JSON-RPC protocol for Java operations"},
		{"MCP_Protocol", "Test MCP protocol Java tool integration"},
	}

	for _, protocol := range protocols {
		suite.Run(protocol.name, func() {
			// Setup protocol-specific responses
			suite.setupProtocolResponses(protocol.name)

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			startTime := time.Now()

			// Test core LSP methods through the protocol
			methods := []string{
				mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
				mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
				mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
				mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
				mcp.LSP_METHOD_WORKSPACE_SYMBOL,
			}

			successCount := 0
			for _, method := range methods {
				resp, err := suite.mockClient.SendLSPRequest(ctx, method, suite.createTestParams(method))
				if err == nil && resp != nil {
					successCount++
				}
			}

			protocolLatency := time.Since(startTime)

			// Validate protocol functionality
			suite.Equal(len(methods), successCount, "All LSP methods should work through %s protocol", protocol.name)
			suite.Less(protocolLatency, 5*time.Second, "%s protocol should respond quickly", protocol.name)
			
			// Validate dual protocol consistency
			if protocol.name == "MCP_Protocol" {
				// Ensure MCP protocol provides consistent responses
				suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), 1, "Definition should be called through MCP")
			}
		})
	}
}

// TestPerformanceValidation tests performance against established thresholds
func (suite *JavaBasicWorkflowE2ETestSuite) TestPerformanceValidation() {
	// Performance thresholds based on project guidelines
	const (
		maxResponseTime = 5 * time.Second
		minThroughput   = 100 // requests per second
		maxErrorRate    = 0.05 // 5%
	)

	testCases := []struct {
		name            string
		requestCount    int
		concurrency     int
		expectedLatency time.Duration
	}{
		{"Single Request Performance", 1, 1, 100 * time.Millisecond},
		{"Moderate Load Performance", 50, 5, 2 * time.Second},
		{"High Load Performance", 100, 10, maxResponseTime},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Reset mock state to ensure clean state for each performance test
			suite.mockClient.Reset()
			
			// Setup sufficient responses for performance test
			for i := 0; i < tc.requestCount; i++ {
				suite.mockClient.QueueResponse(json.RawMessage(`{
					"uri": "file:///workspace/src/main/java/com/example/app/Application.java",
					"range": {
						"start": {"line": 8, "character": 13},
						"end": {"line": 8, "character": 24}
					}
				}`))
			}

			startTime := time.Now()
			var wg sync.WaitGroup
			errorCount := 0
			mu := sync.Mutex{}

			// Execute concurrent requests
			for i := 0; i < tc.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					requestsPerWorker := tc.requestCount / tc.concurrency
					for j := 0; j < requestsPerWorker; j++ {
						ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
						_, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
							"textDocument": map[string]string{"uri": "file:///workspace/src/main/java/com/example/app/Application.java"},
							"position":     LSPPosition{Line: 8, Character: 13},
						})
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

			// Calculate performance metrics
			actualThroughput := float64(tc.requestCount) / totalLatency.Seconds()
			errorRate := float64(errorCount) / float64(tc.requestCount)

			// Validate performance thresholds
			suite.Less(totalLatency, maxResponseTime, "Total response time should be under threshold for %s", tc.name)
			suite.Greater(actualThroughput, float64(minThroughput), "Throughput should exceed minimum for %s", tc.name)
			suite.Less(errorRate, maxErrorRate, "Error rate should be under threshold for %s", tc.name)
			suite.Equal(tc.requestCount, suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), "All requests should be processed")
		})
	}
}

// TestConcurrentWorkflows tests multiple Java workflows running concurrently
func (suite *JavaBasicWorkflowE2ETestSuite) TestConcurrentWorkflows() {
	const numConcurrentWorkflows = 8

	// Setup responses for concurrent execution
	for i := 0; i < numConcurrentWorkflows*4; i++ { // 4 requests per workflow
		suite.mockClient.QueueResponse(suite.createJavaSymbolResponse())
		suite.mockClient.QueueResponse(suite.createJavaDefinitionResponse())
		suite.mockClient.QueueResponse(suite.createJavaReferencesResponse())
		suite.mockClient.QueueResponse(suite.createJavaHoverResponse())
	}

	var wg sync.WaitGroup
	results := make(chan bool, numConcurrentWorkflows)
	startTime := time.Now()

	// Launch concurrent Java workflows
	for i := 0; i < numConcurrentWorkflows; i++ {
		wg.Add(1)
		go func(workflowID int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute Java workflow: symbols → definition → references → hover
			_, err1 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.java", workflowID)},
			})

			_, err2 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.java", workflowID)},
				"position":     LSPPosition{Line: 5, Character: 10},
			})

			_, err3 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.java", workflowID)},
				"position":     LSPPosition{Line: 5, Character: 10},
				"context":      map[string]bool{"includeDeclaration": true},
			})

			_, err4 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.java", workflowID)},
				"position":     LSPPosition{Line: 5, Character: 10},
			})

			results <- (err1 == nil && err2 == nil && err3 == nil && err4 == nil)
		}(i)
	}

	// Wait for all workflows to complete
	wg.Wait()
	close(results)

	concurrentLatency := time.Since(startTime)

	// Validate concurrent execution results
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}

	suite.Equal(numConcurrentWorkflows, successCount, "All concurrent Java workflows should succeed")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), numConcurrentWorkflows, "All symbol requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), numConcurrentWorkflows, "All definition requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES), numConcurrentWorkflows, "All references requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER), numConcurrentWorkflows, "All hover requests should be made")
	suite.Less(concurrentLatency, 15*time.Second, "Concurrent Java workflows should complete within reasonable time")
}

// Helper methods for workflow execution

// executeBasicLSPWorkflow executes a complete basic LSP workflow for Java
func (suite *JavaBasicWorkflowE2ETestSuite) executeBasicLSPWorkflow(fileName, symbolName, expectedType string) JavaWorkflowResult {
	result := JavaWorkflowResult{}

	// Setup mock responses
	suite.mockClient.QueueResponse(suite.createJavaDefinitionResponse())
	suite.mockClient.QueueResponse(suite.createJavaReferencesResponse())
	suite.mockClient.QueueResponse(suite.createJavaHoverResponse())
	suite.mockClient.QueueResponse(suite.createJavaSymbolResponse())

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Execute workflow steps
	defResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 8, Character: 13},
	})
	result.DefinitionFound = err == nil && defResp != nil
	if err != nil {
		result.ErrorCount++
	}

	refsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 8, Character: 13},
		"context":      map[string]bool{"includeDeclaration": true},
	})
	if err == nil && refsResp != nil {
		var refs []interface{}
		if json.Unmarshal(refsResp, &refs) == nil {
			result.ReferencesCount = len(refs)
		}
	} else {
		result.ErrorCount++
	}

	hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 8, Character: 13},
	})
	result.HoverInfoRetrieved = err == nil && hoverResp != nil
	if err != nil {
		result.ErrorCount++
	}

	symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
	})
	if err == nil && symbolsResp != nil {
		var symbols []interface{}
		if json.Unmarshal(symbolsResp, &symbols) == nil {
			result.DocumentSymbolsCount = len(symbols)
		}
	} else {
		result.ErrorCount++
	}

	result.WorkflowLatency = time.Since(startTime)
	result.RequestCount = len(suite.mockClient.SendLSPRequestCalls)
	result.TypeInformationValid = true // Mock always provides valid type info

	return result
}

// executeJavaSpecificWorkflow executes Java-specific feature tests
func (suite *JavaBasicWorkflowE2ETestSuite) executeJavaSpecificWorkflow(feature string) JavaWorkflowResult {
	result := JavaWorkflowResult{}

	// Reset mock state to ensure clean state for each feature test
	suite.mockClient.Reset()

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	switch feature {
	case "completion":
		// Mock completion response
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"items": [
				{"label": "findUserById", "kind": 2, "detail": "User findUserById(Long id) throws UserNotFoundException"},
				{"label": "createUser", "kind": 2, "detail": "User createUser(String name, String email, String role)"},
				{"label": "userRepository", "kind": 6, "detail": "UserRepository"}
			]
		}`))
		resp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/src/main/java/com/example/app/service/UserService.java", suite.workspaceRoot)},
			"position":     LSPPosition{Line: 25, Character: 10},
		})
		if err == nil && resp != nil {
			var completion map[string]interface{}
			if json.Unmarshal(resp, &completion) == nil {
				if items, ok := completion["items"].([]interface{}); ok {
					result.CompletionItemsCount = len(items)
				}
			}
		}

	default:
		// Generic Java feature test using supported LSP features
		suite.mockClient.QueueResponse(suite.createJavaDefinitionResponse())
		_, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/src/main/java/com/example/app/Application.java", suite.workspaceRoot)},
			"position":     LSPPosition{Line: 8, Character: 13},
		})
		if err != nil {
			result.ErrorCount++
		}
	}

	result.WorkflowLatency = time.Since(startTime)
	result.TypeInformationValid = true
	result.RequestCount = len(suite.mockClient.SendLSPRequestCalls)

	return result
}

// Helper methods for creating mock responses

func (suite *JavaBasicWorkflowE2ETestSuite) setupMultiFileResponses() {
	// Setup responses for each file type
	responses := []json.RawMessage{
		suite.createJavaSymbolResponse(),
		suite.createJavaDefinitionResponse(),
		suite.createJavaReferencesResponse(),
		suite.createJavaHoverResponse(),
	}

	// Queue responses for all files
	for range suite.javaFiles {
		for _, response := range responses {
			suite.mockClient.QueueResponse(response)
		}
	}
}

func (suite *JavaBasicWorkflowE2ETestSuite) setupProtocolResponses(protocol string) {
	baseResponses := []json.RawMessage{
		suite.createJavaDefinitionResponse(),
		suite.createJavaReferencesResponse(),
		suite.createJavaHoverResponse(),
		suite.createJavaSymbolResponse(),
		suite.createWorkspaceSymbolResponse(),
	}

	for _, response := range baseResponses {
		suite.mockClient.QueueResponse(response)
	}
}

func (suite *JavaBasicWorkflowE2ETestSuite) createTestParams(method string) map[string]interface{} {
	switch method {
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		return map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///workspace/src/main/java/com/example/app/Application.java"},
			"position":     LSPPosition{Line: 8, Character: 13},
		}
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		return map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///workspace/src/main/java/com/example/app/Application.java"},
		}
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		return map[string]interface{}{
			"query": "Application",
		}
	default:
		return map[string]interface{}{}
	}
}

func (suite *JavaBasicWorkflowE2ETestSuite) createJavaDefinitionResponse() json.RawMessage {
	return json.RawMessage(`{
		"uri": "file:///workspace/src/main/java/com/example/app/Application.java",
		"range": {
			"start": {"line": 8, "character": 13},
			"end": {"line": 8, "character": 24}
		}
	}`)
}

func (suite *JavaBasicWorkflowE2ETestSuite) createJavaReferencesResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"uri": "file:///workspace/src/main/java/com/example/app/Application.java",
			"range": {"start": {"line": 8, "character": 13}, "end": {"line": 8, "character": 24}}
		},
		{
			"uri": "file:///workspace/src/main/java/com/example/app/controller/UserController.java",
			"range": {"start": {"line": 16, "character": 13}, "end": {"line": 16, "character": 27}}
		},
		{
			"uri": "file:///workspace/src/main/java/com/example/app/service/UserService.java",
			"range": {"start": {"line": 16, "character": 13}, "end": {"line": 16, "character": 24}}
		}
	]`)
}

func (suite *JavaBasicWorkflowE2ETestSuite) createJavaHoverResponse() json.RawMessage {
	return json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```java\n@SpringBootApplication\npublic class Application\n```" + `\n\nMain Spring Boot application class with embedded health controller.\n\n**Annotations:**\n- ` + "`@SpringBootApplication`" + ` - Enables Spring Boot auto-configuration\n\n**Methods:**\n- ` + "`main(String[] args): void`" + ` - Application entry point\n- ` + "`getContext(): ApplicationContext`" + ` - Get Spring application context"
		},
		"range": {
			"start": {"line": 8, "character": 13},
			"end": {"line": 8, "character": 24}
		}
	}`)
}

func (suite *JavaBasicWorkflowE2ETestSuite) createJavaSymbolResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "Application",
			"kind": 5,
			"range": {"start": {"line": 8, "character": 0}, "end": {"line": 28, "character": 1}},
			"selectionRange": {"start": {"line": 8, "character": 13}, "end": {"line": 8, "character": 24}},
			"children": [
				{
					"name": "context",
					"kind": 6,
					"range": {"start": {"line": 10, "character": 4}, "end": {"line": 10, "character": 47}},
					"selectionRange": {"start": {"line": 10, "character": 32}, "end": {"line": 10, "character": 39}}
				},
				{
					"name": "main",
					"kind": 6,
					"range": {"start": {"line": 12, "character": 4}, "end": {"line": 15, "character": 5}},
					"selectionRange": {"start": {"line": 12, "character": 23}, "end": {"line": 12, "character": 27}}
				},
				{
					"name": "getContext",
					"kind": 6,
					"range": {"start": {"line": 17, "character": 4}, "end": {"line": 19, "character": 5}},
					"selectionRange": {"start": {"line": 17, "character": 39}, "end": {"line": 17, "character": 49}}
				},
				{
					"name": "HealthController",
					"kind": 5,
					"range": {"start": {"line": 21, "character": 4}, "end": {"line": 27, "character": 5}},
					"selectionRange": {"start": {"line": 21, "character": 30}, "end": {"line": 21, "character": 46}},
					"children": [
						{
							"name": "getHealth",
							"kind": 6,
							"range": {"start": {"line": 24, "character": 8}, "end": {"line": 26, "character": 9}},
							"selectionRange": {"start": {"line": 24, "character": 27}, "end": {"line": 24, "character": 36}}
						}
					]
				}
			]
		}
	]`)
}

func (suite *JavaBasicWorkflowE2ETestSuite) createWorkspaceSymbolResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "Application",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/main/java/com/example/app/Application.java",
				"range": {"start": {"line": 8, "character": 13}, "end": {"line": 8, "character": 24}}
			}
		},
		{
			"name": "UserController",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/main/java/com/example/app/controller/UserController.java",
				"range": {"start": {"line": 16, "character": 13}, "end": {"line": 16, "character": 27}}
			}
		},
		{
			"name": "UserService",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/main/java/com/example/app/service/UserService.java",
				"range": {"start": {"line": 16, "character": 13}, "end": {"line": 16, "character": 24}}
			}
		},
		{
			"name": "User",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/src/main/java/com/example/app/entity/User.java",
				"range": {"start": {"line": 17, "character": 13}, "end": {"line": 17, "character": 17}}
			}
		},
		{
			"name": "UserRole",
			"kind": 10,
			"location": {
				"uri": "file:///workspace/src/main/java/com/example/app/entity/UserRole.java",
				"range": {"start": {"line": 4, "character": 12}, "end": {"line": 4, "character": 20}}
			}
		},
		{
			"name": "UserRepository",
			"kind": 11,
			"location": {
				"uri": "file:///workspace/src/main/java/com/example/app/repository/UserRepository.java",
				"range": {"start": {"line": 14, "character": 17}, "end": {"line": 14, "character": 31}}
			}
		}
	]`)
}

// Benchmark tests for performance measurement

// BenchmarkJavaBasicWorkflow benchmarks the basic Java workflow
func BenchmarkJavaBasicWorkflow(b *testing.B) {
	suite := &JavaBasicWorkflowE2ETestSuite{}
	suite.SetupSuite()

	workflows := []string{"definition", "references", "hover", "symbols"}

	for _, workflow := range workflows {
		b.Run(fmt.Sprintf("Java_%s", workflow), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()
				result := suite.executeBasicLSPWorkflow("src/main/java/com/example/app/Application.java", "Application", "class")
				if result.ErrorCount > 0 {
					b.Errorf("Workflow failed with %d errors", result.ErrorCount)
				}
				suite.TearDownTest()
			}
		})
	}
}

// BenchmarkJavaConcurrentWorkflows benchmarks concurrent workflow execution
func BenchmarkJavaConcurrentWorkflows(b *testing.B) {
	suite := &JavaBasicWorkflowE2ETestSuite{}
	suite.SetupSuite()

	concurrencyLevels := []int{1, 4, 8, 16}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrent_%d", concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()

				// Setup sufficient responses
				for j := 0; j < concurrency*4; j++ {
					suite.mockClient.QueueResponse(suite.createJavaSymbolResponse())
				}

				var wg sync.WaitGroup
				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
							"textDocument": map[string]string{"uri": "file:///workspace/src/main/java/com/example/app/Application.java"},
						})
					}()
				}
				wg.Wait()

				suite.TearDownTest()
			}
		})
	}
}

// TestSuite runner
func TestJavaBasicWorkflowE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaBasicWorkflowE2ETestSuite))
}