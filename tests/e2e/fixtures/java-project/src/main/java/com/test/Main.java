package com.test;

import com.test.controller.UserController;
import com.test.model.User;
import com.test.service.UserService;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.ApplicationContext;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.ComponentScan;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

/**
 * Main application class for the LSP Gateway Test Project.
 * This class serves as the entry point and demonstrates usage of all components
 * for comprehensive LSP feature testing.
 */
@SpringBootApplication
@ComponentScan(basePackages = {"com.test"})
public class Main {
    
    /**
     * Main method - application entry point.
     * Demonstrates usage of all project components for LSP testing.
     * 
     * @param args command line arguments
     */
    public static void main(String[] args) {
        System.out.println("Starting LSP Gateway Test Application...");
        
        // Start Spring Boot application
        ApplicationContext context = SpringApplication.run(Main.class, args);
        
        // Demonstrate component usage for LSP testing
        demonstrateUserOperations(context);
        
        System.out.println("Application started successfully!");
        System.out.println("Server running on http://localhost:8080");
        System.out.println("API endpoints available at /api/users");
    }
    
    /**
     * Demonstrates comprehensive user operations for LSP feature testing.
     * This method provides extensive cross-references between all classes.
     * 
     * @param context the Spring application context
     */
    private static void demonstrateUserOperations(ApplicationContext context) {
        // Get beans from Spring context
        UserService userService = context.getBean(UserService.class);
        UserController userController = context.getBean(UserController.class);
        
        System.out.println("\n=== Demonstrating User Operations ===");
        
        // Test user creation
        demonstrateUserCreation(userService);
        
        // Test user retrieval
        demonstrateUserRetrieval(userService);
        
        // Test user search
        demonstrateUserSearch(userService);
        
        // Test user updates
        demonstrateUserUpdates(userService);
        
        // Test user statistics
        demonstrateUserStatistics(userService);
        
        // Test controller integration
        demonstrateControllerIntegration(userController);
    }
    
    /**
     * Demonstrates user creation operations.
     * 
     * @param userService the user service instance
     */
    private static void demonstrateUserCreation(UserService userService) {
        System.out.println("\n--- User Creation ---");
        
        try {
            // Create new users using UserService
            User testUser1 = userService.createUser("test_user1", "test1@example.com", "Test", "User1");
            System.out.println("Created user: " + testUser1.getFullName());
            
            User testUser2 = userService.createUser("test_user2", "test2@example.com", "Test", "User2");
            System.out.println("Created user: " + testUser2.getFullName());
            
            // Create user object manually
            User manualUser = new User("manual_user", "manual@example.com", "Manual", "User");
            User savedUser = userService.saveUser(manualUser);
            System.out.println("Saved manual user: " + savedUser.getFullName());
            
        } catch (IllegalArgumentException e) {
            System.err.println("User creation error: " + e.getMessage());
        }
    }
    
    /**
     * Demonstrates user retrieval operations.
     * 
     * @param userService the user service instance
     */
    private static void demonstrateUserRetrieval(UserService userService) {
        System.out.println("\n--- User Retrieval ---");
        
        // Get user by ID
        Optional<User> userById = userService.getUserById(1L);
        if (userById.isPresent()) {
            User user = userById.get();
            System.out.println("Found user by ID: " + user.getUsername());
            System.out.println("User details: " + user.toString());
        }
        
        // Get user by username
        Optional<User> userByUsername = userService.getUserByUsername("admin");
        userByUsername.ifPresent(user -> {
            System.out.println("Found user by username: " + user.getEmail());
            System.out.println("User is active: " + user.isActive());
            System.out.println("User created at: " + user.getCreatedAt());
        });
        
        // Get user by email
        Optional<User> userByEmail = userService.getUserByEmail("john@test.com");
        userByEmail.ifPresent(user -> {
            System.out.println("Found user by email: " + user.getFullName());
            user.updateLastLogin();
            System.out.println("Updated last login: " + user.getLastLoginAt());
        });
        
        // Get all users
        List<User> allUsers = userService.getAllUsers();
        System.out.println("Total users in system: " + allUsers.size());
        
        // Get active users
        List<User> activeUsers = userService.getActiveUsers();
        System.out.println("Active users: " + activeUsers.size());
    }
    
    /**
     * Demonstrates user search operations.
     * 
     * @param userService the user service instance
     */
    private static void demonstrateUserSearch(UserService userService) {
        System.out.println("\n--- User Search ---");
        
        // Search users by name
        List<User> searchResults = userService.searchUsersByName("John");
        System.out.println("Users with name 'John': " + searchResults.size());
        
        searchResults.forEach(user -> {
            System.out.println("  - " + user.getFullName() + " (" + user.getUsername() + ")");
        });
        
        // Search with different term
        List<User> adminUsers = userService.searchUsersByName("Admin");
        adminUsers.forEach(user -> {
            System.out.println("Admin user found: " + user.getUsername());
            System.out.println("  Email: " + user.getEmail());
            System.out.println("  Full name: " + user.getFullName());
        });
    }
    
    /**
     * Demonstrates user update operations.
     * 
     * @param userService the user service instance
     */
    private static void demonstrateUserUpdates(UserService userService) {
        System.out.println("\n--- User Updates ---");
        
        // Update user information
        Optional<User> userToUpdate = userService.getUserById(2L);
        if (userToUpdate.isPresent()) {
            User user = userToUpdate.get();
            User updatedData = new User();
            updatedData.setFirstName("Updated");
            updatedData.setLastName("Name");
            
            Optional<User> updatedUser = userService.updateUser(user.getId(), updatedData);
            updatedUser.ifPresent(u -> 
                System.out.println("Updated user: " + u.getFullName())
            );
        }
        
        // Test user deactivation
        boolean deactivated = userService.deactivateUser(3L);
        if (deactivated) {
            System.out.println("User deactivated successfully");
        }
        
        // Test user activation
        boolean activated = userService.activateUser(3L);
        if (activated) {
            System.out.println("User activated successfully");
        }
        
        // Record user login
        boolean loginRecorded = userService.recordUserLogin(1L);
        if (loginRecorded) {
            System.out.println("User login recorded");
        }
    }
    
    /**
     * Demonstrates user statistics operations.
     * 
     * @param userService the user service instance
     */
    private static void demonstrateUserStatistics(UserService userService) {
        System.out.println("\n--- User Statistics ---");
        
        long totalUsers = userService.getUserCount();
        long activeUsers = userService.getActiveUserCount();
        
        System.out.println("Total users: " + totalUsers);
        System.out.println("Active users: " + activeUsers);
        System.out.println("Inactive users: " + (totalUsers - activeUsers));
        
        // Calculate percentages
        if (totalUsers > 0) {
            double activePercentage = (double) activeUsers / totalUsers * 100;
            System.out.println("Active user percentage: " + String.format("%.1f%%", activePercentage));
        }
    }
    
    /**
     * Demonstrates controller integration for REST API testing.
     * 
     * @param userController the user controller instance
     */
    private static void demonstrateControllerIntegration(UserController userController) {
        System.out.println("\n--- Controller Integration ---");
        
        // Demonstrate controller usage (note: these would normally be HTTP requests)
        System.out.println("UserController initialized and ready for HTTP requests");
        System.out.println("Available endpoints:");
        System.out.println("  GET /api/users - Get all users");
        System.out.println("  GET /api/users/{id} - Get user by ID");
        System.out.println("  GET /api/users/username/{username} - Get user by username");
        System.out.println("  GET /api/users/email/{email} - Get user by email");
        System.out.println("  GET /api/users/search?searchTerm={term} - Search users");
        System.out.println("  POST /api/users - Create new user");
        System.out.println("  PUT /api/users/{id} - Update user");
        System.out.println("  DELETE /api/users/{id} - Deactivate user");
        System.out.println("  PATCH /api/users/{id}/activate - Activate user");
        System.out.println("  POST /api/users/{id}/login - Record user login");
        System.out.println("  GET /api/users/stats - Get user statistics");
    }
    
    /**
     * Creates a UserService bean for Spring dependency injection.
     * 
     * @return UserService instance
     */
    @Bean
    public UserService userService() {
        return new UserService();
    }
}