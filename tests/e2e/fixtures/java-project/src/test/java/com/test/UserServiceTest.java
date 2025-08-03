package com.test;

import com.test.model.User;
import com.test.service.UserService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;
import static org.junit.jupiter.api.Assertions.*;
import java.util.List;
import java.util.Optional;

/**
 * Test class for UserService functionality.
 * Provides comprehensive test coverage for user operations.
 */
class UserServiceTest {
    
    private UserService userService;
    
    @BeforeEach
    void setUp() {
        userService = new UserService();
    }
    
    @Test
    @DisplayName("Should create user with valid data")
    void testCreateUserWithValidData() {
        String username = "testuser";
        String email = "test@example.com";
        String firstName = "Test";
        String lastName = "User";
        
        User createdUser = userService.createUser(username, email, firstName, lastName);
        
        assertNotNull(createdUser);
        assertEquals(username, createdUser.getUsername());
        assertEquals(email, createdUser.getEmail());
        assertEquals(firstName, createdUser.getFirstName());
        assertEquals(lastName, createdUser.getLastName());
        assertTrue(createdUser.isActive());
        assertNotNull(createdUser.getId());
        assertNotNull(createdUser.getCreatedAt());
    }
    
    @Test
    @DisplayName("Should retrieve user by ID")
    void testGetUserById() {
        Optional<User> userOpt = userService.getUserById(1L);
        
        assertTrue(userOpt.isPresent());
        User user = userOpt.get();
        assertEquals(1L, user.getId());
        assertEquals("admin", user.getUsername());
    }
    
    @Test
    @DisplayName("Should retrieve user by username")
    void testGetUserByUsername() {
        Optional<User> userOpt = userService.getUserByUsername("admin");
        
        assertTrue(userOpt.isPresent());
        User user = userOpt.get();
        assertEquals("admin", user.getUsername());
        assertEquals("admin@test.com", user.getEmail());
    }
    
    @Test
    @DisplayName("Should get all users")
    void testGetAllUsers() {
        List<User> users = userService.getAllUsers();
        
        assertNotNull(users);
        assertTrue(users.size() >= 3); // Default users initialized
    }
    
    @Test
    @DisplayName("Should search users by name")
    void testSearchUsersByName() {
        List<User> results = userService.searchUsersByName("John");
        
        assertNotNull(results);
        assertFalse(results.isEmpty());
        assertTrue(results.stream().anyMatch(user -> 
            user.getFirstName().contains("John") || user.getLastName().contains("John")
        ));
    }
    
    @Test
    @DisplayName("Should deactivate user")
    void testDeactivateUser() {
        User newUser = userService.createUser("deactivate_test", "deactivate@test.com", "Test", "User");
        Long userId = newUser.getId();
        
        boolean result = userService.deactivateUser(userId);
        
        assertTrue(result);
        Optional<User> updatedUser = userService.getUserById(userId);
        assertTrue(updatedUser.isPresent());
        assertFalse(updatedUser.get().isActive());
    }
    
    @Test
    @DisplayName("Should record user login")
    void testRecordUserLogin() {
        boolean result = userService.recordUserLogin(1L);
        
        assertTrue(result);
        Optional<User> user = userService.getUserById(1L);
        assertTrue(user.isPresent());
        assertNotNull(user.get().getLastLoginAt());
    }
    
    @Test
    @DisplayName("Should get user count")
    void testGetUserCount() {
        long count = userService.getUserCount();
        
        assertTrue(count >= 3); // At least the default users
    }
    
    @Test
    @DisplayName("Should throw exception for invalid user creation")
    void testCreateUserWithInvalidData() {
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser(null, "test@example.com", "Test", "User");
        });
        
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser("testuser", null, "Test", "User");
        });
        
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser("testuser", "test@example.com", null, "User");
        });
        
        assertThrows(IllegalArgumentException.class, () -> {
            userService.createUser("testuser", "test@example.com", "Test", null);
        });
    }
}