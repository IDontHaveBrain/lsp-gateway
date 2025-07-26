package com.test.controller;

import com.test.model.User;
import com.test.service.UserService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import javax.validation.Valid;
import java.util.List;
import java.util.Optional;

/**
 * REST Controller for managing User-related HTTP endpoints.
 * Provides comprehensive RESTful API for user management operations.
 */
@RestController
@RequestMapping("/api/users")
@CrossOrigin(origins = "*")
public class UserController {
    
    private final UserService userService;
    
    /**
     * Constructor with dependency injection for UserService.
     * 
     * @param userService the user service to inject
     */
    @Autowired
    public UserController(UserService userService) {
        this.userService = userService;
    }
    
    /**
     * Creates a new user.
     * 
     * @param user the user data to create
     * @return ResponseEntity containing the created user or error message
     */
    @PostMapping
    public ResponseEntity<?> createUser(@Valid @RequestBody User user) {
        try {
            if (user.getUsername() == null || user.getEmail() == null || 
                user.getFirstName() == null || user.getLastName() == null) {
                return ResponseEntity.badRequest()
                    .body("Missing required fields: username, email, firstName, lastName");
            }
            
            User createdUser = userService.createUser(
                user.getUsername(), 
                user.getEmail(), 
                user.getFirstName(), 
                user.getLastName()
            );
            
            return ResponseEntity.status(HttpStatus.CREATED).body(createdUser);
            
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(e.getMessage());
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body("Failed to create user: " + e.getMessage());
        }
    }
    
    /**
     * Retrieves all users.
     * 
     * @return ResponseEntity containing list of all users
     */
    @GetMapping
    public ResponseEntity<List<User>> getAllUsers() {
        try {
            List<User> users = userService.getAllUsers();
            return ResponseEntity.ok(users);
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).build();
        }
    }
    
    /**
     * Retrieves all active users.
     * 
     * @return ResponseEntity containing list of active users
     */
    @GetMapping("/active")
    public ResponseEntity<List<User>> getActiveUsers() {
        try {
            List<User> activeUsers = userService.getActiveUsers();
            return ResponseEntity.ok(activeUsers);
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).build();
        }
    }
    
    /**
     * Retrieves a user by their ID.
     * 
     * @param id the user ID
     * @return ResponseEntity containing the user or not found status
     */
    @GetMapping("/{id}")
    public ResponseEntity<User> getUserById(@PathVariable Long id) {
        try {
            Optional<User> userOpt = userService.getUserById(id);
            if (userOpt.isPresent()) {
                return ResponseEntity.ok(userOpt.get());
            } else {
                return ResponseEntity.notFound().build();
            }
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).build();
        }
    }
    
    /**
     * Retrieves a user by their username.
     * 
     * @param username the username to search for
     * @return ResponseEntity containing the user or not found status
     */
    @GetMapping("/username/{username}")
    public ResponseEntity<User> getUserByUsername(@PathVariable String username) {
        try {
            Optional<User> userOpt = userService.getUserByUsername(username);
            if (userOpt.isPresent()) {
                return ResponseEntity.ok(userOpt.get());
            } else {
                return ResponseEntity.notFound().build();
            }
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).build();
        }
    }
    
    /**
     * Retrieves a user by their email address.
     * 
     * @param email the email address to search for
     * @return ResponseEntity containing the user or not found status
     */
    @GetMapping("/email/{email}")
    public ResponseEntity<User> getUserByEmail(@PathVariable String email) {
        try {
            Optional<User> userOpt = userService.getUserByEmail(email);
            if (userOpt.isPresent()) {
                return ResponseEntity.ok(userOpt.get());
            } else {
                return ResponseEntity.notFound().build();
            }
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).build();
        }
    }
    
    /**
     * Searches for users by name.
     * 
     * @param searchTerm the search term for user names
     * @return ResponseEntity containing list of matching users
     */
    @GetMapping("/search")
    public ResponseEntity<List<User>> searchUsers(@RequestParam String searchTerm) {
        try {
            if (searchTerm == null || searchTerm.trim().isEmpty()) {
                return ResponseEntity.badRequest().build();
            }
            
            List<User> matchingUsers = userService.searchUsersByName(searchTerm);
            return ResponseEntity.ok(matchingUsers);
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).build();
        }
    }
    
    /**
     * Updates an existing user.
     * 
     * @param id the ID of the user to update
     * @param user the updated user data
     * @return ResponseEntity containing the updated user or error status
     */
    @PutMapping("/{id}")
    public ResponseEntity<?> updateUser(@PathVariable Long id, @Valid @RequestBody User user) {
        try {
            Optional<User> updatedUserOpt = userService.updateUser(id, user);
            if (updatedUserOpt.isPresent()) {
                return ResponseEntity.ok(updatedUserOpt.get());
            } else {
                return ResponseEntity.notFound().build();
            }
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(e.getMessage());
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body("Failed to update user: " + e.getMessage());
        }
    }
    
    /**
     * Deactivates a user.
     * 
     * @param id the ID of the user to deactivate
     * @return ResponseEntity with success or error status
     */
    @DeleteMapping("/{id}")
    public ResponseEntity<?> deactivateUser(@PathVariable Long id) {
        try {
            boolean success = userService.deactivateUser(id);
            if (success) {
                return ResponseEntity.ok().body("User deactivated successfully");
            } else {
                return ResponseEntity.notFound().build();
            }
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body("Failed to deactivate user: " + e.getMessage());
        }
    }
    
    /**
     * Activates a previously deactivated user.
     * 
     * @param id the ID of the user to activate
     * @return ResponseEntity with success or error status
     */
    @PatchMapping("/{id}/activate")
    public ResponseEntity<?> activateUser(@PathVariable Long id) {
        try {
            boolean success = userService.activateUser(id);
            if (success) {
                return ResponseEntity.ok().body("User activated successfully");
            } else {
                return ResponseEntity.notFound().build();
            }
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body("Failed to activate user: " + e.getMessage());
        }
    }
    
    /**
     * Records a user login.
     * 
     * @param id the ID of the user who logged in
     * @return ResponseEntity with success or error status
     */
    @PostMapping("/{id}/login")
    public ResponseEntity<?> recordLogin(@PathVariable Long id) {
        try {
            boolean success = userService.recordUserLogin(id);
            if (success) {
                return ResponseEntity.ok().body("Login recorded successfully");
            } else {
                return ResponseEntity.notFound().build();
            }
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR)
                .body("Failed to record login: " + e.getMessage());
        }
    }
    
    /**
     * Gets user statistics.
     * 
     * @return ResponseEntity containing user statistics
     */
    @GetMapping("/stats")
    public ResponseEntity<UserStats> getUserStats() {
        try {
            long totalUsers = userService.getUserCount();
            long activeUsers = userService.getActiveUserCount();
            long inactiveUsers = totalUsers - activeUsers;
            
            UserStats stats = new UserStats(totalUsers, activeUsers, inactiveUsers);
            return ResponseEntity.ok(stats);
        } catch (Exception e) {
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).build();
        }
    }
    
    /**
     * Inner class for user statistics.
     */
    public static class UserStats {
        private final long totalUsers;
        private final long activeUsers;
        private final long inactiveUsers;
        
        public UserStats(long totalUsers, long activeUsers, long inactiveUsers) {
            this.totalUsers = totalUsers;
            this.activeUsers = activeUsers;
            this.inactiveUsers = inactiveUsers;
        }
        
        public long getTotalUsers() {
            return totalUsers;
        }
        
        public long getActiveUsers() {
            return activeUsers;
        }
        
        public long getInactiveUsers() {
            return inactiveUsers;
        }
    }
}