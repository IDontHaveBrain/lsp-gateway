package com.test.service;

import com.test.model.User;
import org.springframework.stereotype.Service;
import javax.validation.Valid;
import javax.validation.constraints.NotNull;
import java.time.LocalDateTime;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

/**
 * Service class for managing User operations.
 * Provides comprehensive user management functionality including CRUD operations,
 * user validation, and business logic.
 */
@Service
public class UserService {
    
    private final Map<Long, User> userRepository;
    private Long nextId;
    
    /**
     * Default constructor initializing the user service.
     */
    public UserService() {
        this.userRepository = new ConcurrentHashMap<>();
        this.nextId = 1L;
        initializeDefaultUsers();
    }
    
    /**
     * Initializes default users for testing purposes.
     */
    private void initializeDefaultUsers() {
        createUser("admin", "admin@test.com", "Admin", "User");
        createUser("john_doe", "john@test.com", "John", "Doe");
        createUser("jane_smith", "jane@test.com", "Jane", "Smith");
    }
    
    /**
     * Creates a new user with the provided information.
     * 
     * @param username the username for the new user
     * @param email the email address for the new user
     * @param firstName the first name of the new user
     * @param lastName the last name of the new user
     * @return the created User object
     * @throws IllegalArgumentException if user data is invalid
     */
    public User createUser(String username, String email, String firstName, String lastName) {
        validateUserInput(username, email, firstName, lastName);
        
        if (isUsernameExists(username)) {
            throw new IllegalArgumentException("Username already exists: " + username);
        }
        
        if (isEmailExists(email)) {
            throw new IllegalArgumentException("Email already exists: " + email);
        }
        
        User user = new User(username, email, firstName, lastName);
        user.setId(generateId());
        user.setCreatedAt(LocalDateTime.now());
        user.setActive(true);
        
        userRepository.put(user.getId(), user);
        return user;
    }
    
    /**
     * Saves or updates an existing user.
     * 
     * @param user the user to save or update
     * @return the saved User object
     */
    public User saveUser(@Valid @NotNull User user) {
        if (user.getId() == null) {
            user.setId(generateId());
            user.setCreatedAt(LocalDateTime.now());
        }
        
        if (!user.isValid()) {
            throw new IllegalArgumentException("User data is not valid");
        }
        
        userRepository.put(user.getId(), user);
        return user;
    }
    
    /**
     * Retrieves a user by their ID.
     * 
     * @param id the ID of the user to retrieve
     * @return Optional containing the user if found, empty otherwise
     */
    public Optional<User> getUserById(Long id) {
        if (id == null) {
            return Optional.empty();
        }
        return Optional.ofNullable(userRepository.get(id));
    }
    
    /**
     * Retrieves a user by their username.
     * 
     * @param username the username to search for
     * @return Optional containing the user if found, empty otherwise
     */
    public Optional<User> getUserByUsername(String username) {
        if (username == null || username.trim().isEmpty()) {
            return Optional.empty();
        }
        
        return userRepository.values().stream()
                .filter(user -> username.equals(user.getUsername()))
                .findFirst();
    }
    
    /**
     * Retrieves a user by their email address.
     * 
     * @param email the email address to search for
     * @return Optional containing the user if found, empty otherwise
     */
    public Optional<User> getUserByEmail(String email) {
        if (email == null || email.trim().isEmpty()) {
            return Optional.empty();
        }
        
        return userRepository.values().stream()
                .filter(user -> email.equals(user.getEmail()))
                .findFirst();
    }
    
    /**
     * Retrieves all users in the system.
     * 
     * @return List of all users
     */
    public List<User> getAllUsers() {
        return new ArrayList<>(userRepository.values());
    }
    
    /**
     * Retrieves all active users.
     * 
     * @return List of active users
     */
    public List<User> getActiveUsers() {
        return userRepository.values().stream()
                .filter(User::isActive)
                .collect(Collectors.toList());
    }
    
    /**
     * Searches for users by name (first name, last name, or full name).
     * 
     * @param searchTerm the search term to match against names
     * @return List of users matching the search criteria
     */
    public List<User> searchUsersByName(String searchTerm) {
        if (searchTerm == null || searchTerm.trim().isEmpty()) {
            return Collections.emptyList();
        }
        
        String lowerCaseSearch = searchTerm.toLowerCase();
        return userRepository.values().stream()
                .filter(user -> 
                    user.getFirstName().toLowerCase().contains(lowerCaseSearch) ||
                    user.getLastName().toLowerCase().contains(lowerCaseSearch) ||
                    user.getFullName().toLowerCase().contains(lowerCaseSearch))
                .collect(Collectors.toList());
    }
    
    /**
     * Updates an existing user.
     * 
     * @param id the ID of the user to update
     * @param updatedUser the updated user information
     * @return Optional containing the updated user if successful, empty otherwise
     */
    public Optional<User> updateUser(Long id, User updatedUser) {
        Optional<User> existingUserOpt = getUserById(id);
        if (!existingUserOpt.isPresent()) {
            return Optional.empty();
        }
        
        User existingUser = existingUserOpt.get();
        updateUserFields(existingUser, updatedUser);
        
        userRepository.put(id, existingUser);
        return Optional.of(existingUser);
    }
    
    /**
     * Deactivates a user instead of deleting them.
     * 
     * @param id the ID of the user to deactivate
     * @return true if user was deactivated, false if user not found
     */
    public boolean deactivateUser(Long id) {
        Optional<User> userOpt = getUserById(id);
        if (userOpt.isPresent()) {
            User user = userOpt.get();
            user.setActive(false);
            userRepository.put(id, user);
            return true;
        }
        return false;
    }
    
    /**
     * Activates a previously deactivated user.
     * 
     * @param id the ID of the user to activate
     * @return true if user was activated, false if user not found
     */
    public boolean activateUser(Long id) {
        Optional<User> userOpt = getUserById(id);
        if (userOpt.isPresent()) {
            User user = userOpt.get();
            user.setActive(true);
            userRepository.put(id, user);
            return true;
        }
        return false;
    }
    
    /**
     * Records a user login by updating their last login timestamp.
     * 
     * @param id the ID of the user who logged in
     * @return true if login was recorded, false if user not found
     */
    public boolean recordUserLogin(Long id) {
        Optional<User> userOpt = getUserById(id);
        if (userOpt.isPresent()) {
            User user = userOpt.get();
            user.updateLastLogin();
            userRepository.put(id, user);
            return true;
        }
        return false;
    }
    
    /**
     * Gets the total number of users.
     * 
     * @return total user count
     */
    public long getUserCount() {
        return userRepository.size();
    }
    
    /**
     * Gets the number of active users.
     * 
     * @return active user count
     */
    public long getActiveUserCount() {
        return userRepository.values().stream()
                .mapToLong(user -> user.isActive() ? 1 : 0)
                .sum();
    }
    
    /**
     * Validates user input parameters.
     * 
     * @param username the username to validate
     * @param email the email to validate
     * @param firstName the first name to validate
     * @param lastName the last name to validate
     * @throws IllegalArgumentException if any parameter is invalid
     */
    private void validateUserInput(String username, String email, String firstName, String lastName) {
        if (username == null || username.trim().isEmpty()) {
            throw new IllegalArgumentException("Username cannot be null or empty");
        }
        if (email == null || email.trim().isEmpty()) {
            throw new IllegalArgumentException("Email cannot be null or empty");
        }
        if (firstName == null || firstName.trim().isEmpty()) {
            throw new IllegalArgumentException("First name cannot be null or empty");
        }
        if (lastName == null || lastName.trim().isEmpty()) {
            throw new IllegalArgumentException("Last name cannot be null or empty");
        }
    }
    
    /**
     * Checks if a username already exists.
     * 
     * @param username the username to check
     * @return true if username exists, false otherwise
     */
    private boolean isUsernameExists(String username) {
        return userRepository.values().stream()
                .anyMatch(user -> username.equals(user.getUsername()));
    }
    
    /**
     * Checks if an email already exists.
     * 
     * @param email the email to check
     * @return true if email exists, false otherwise
     */
    private boolean isEmailExists(String email) {
        return userRepository.values().stream()
                .anyMatch(user -> email.equals(user.getEmail()));
    }
    
    /**
     * Generates the next available ID.
     * 
     * @return the next ID
     */
    private Long generateId() {
        return nextId++;
    }
    
    /**
     * Updates existing user fields with new values.
     * 
     * @param existingUser the existing user to update
     * @param updatedUser the new user data
     */
    private void updateUserFields(User existingUser, User updatedUser) {
        if (updatedUser.getUsername() != null) {
            existingUser.setUsername(updatedUser.getUsername());
        }
        if (updatedUser.getEmail() != null) {
            existingUser.setEmail(updatedUser.getEmail());
        }
        if (updatedUser.getFirstName() != null) {
            existingUser.setFirstName(updatedUser.getFirstName());
        }
        if (updatedUser.getLastName() != null) {
            existingUser.setLastName(updatedUser.getLastName());
        }
    }
}