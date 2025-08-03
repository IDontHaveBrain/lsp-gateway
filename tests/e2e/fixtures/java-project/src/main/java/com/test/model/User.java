package com.test.model;

import com.fasterxml.jackson.annotation.JsonProperty;
import javax.validation.constraints.Email;
import javax.validation.constraints.NotBlank;
import javax.validation.constraints.NotNull;
import javax.validation.constraints.Size;
import java.time.LocalDateTime;
import java.util.Objects;

/**
 * User entity class representing a user in the system.
 * This class provides comprehensive user information and validation.
 */
public class User {
    
    @JsonProperty("id")
    private Long id;
    
    @NotBlank(message = "Username cannot be blank")
    @Size(min = 3, max = 50, message = "Username must be between 3 and 50 characters")
    @JsonProperty("username")
    private String username;
    
    @Email(message = "Email should be valid")
    @NotBlank(message = "Email cannot be blank")
    @JsonProperty("email")
    private String email;
    
    @NotBlank(message = "First name cannot be blank")
    @JsonProperty("firstName")
    private String firstName;
    
    @NotBlank(message = "Last name cannot be blank")
    @JsonProperty("lastName")
    private String lastName;
    
    @JsonProperty("isActive")
    private boolean isActive;
    
    @JsonProperty("createdAt")
    private LocalDateTime createdAt;
    
    @JsonProperty("lastLoginAt")
    private LocalDateTime lastLoginAt;
    
    /**
     * Default constructor for User.
     */
    public User() {
        this.isActive = true;
        this.createdAt = LocalDateTime.now();
    }
    
    /**
     * Constructor with required fields for User creation.
     * 
     * @param username the username for the user
     * @param email the email address for the user
     * @param firstName the first name of the user
     * @param lastName the last name of the user
     */
    public User(String username, String email, String firstName, String lastName) {
        this();
        this.username = username;
        this.email = email;
        this.firstName = firstName;
        this.lastName = lastName;
    }
    
    /**
     * Gets the user ID.
     * 
     * @return the user ID
     */
    public Long getId() {
        return id;
    }
    
    /**
     * Sets the user ID.
     * 
     * @param id the user ID to set
     */
    public void setId(Long id) {
        this.id = id;
    }
    
    /**
     * Gets the username.
     * 
     * @return the username
     */
    public String getUsername() {
        return username;
    }
    
    /**
     * Sets the username.
     * 
     * @param username the username to set
     */
    public void setUsername(String username) {
        this.username = username;
    }
    
    /**
     * Gets the email address.
     * 
     * @return the email address
     */
    public String getEmail() {
        return email;
    }
    
    /**
     * Sets the email address.
     * 
     * @param email the email address to set
     */
    public void setEmail(String email) {
        this.email = email;
    }
    
    /**
     * Gets the first name.
     * 
     * @return the first name
     */
    public String getFirstName() {
        return firstName;
    }
    
    /**
     * Sets the first name.
     * 
     * @param firstName the first name to set
     */
    public void setFirstName(String firstName) {
        this.firstName = firstName;
    }
    
    /**
     * Gets the last name.
     * 
     * @return the last name
     */
    public String getLastName() {
        return lastName;
    }
    
    /**
     * Sets the last name.
     * 
     * @param lastName the last name to set
     */
    public void setLastName(String lastName) {
        this.lastName = lastName;
    }
    
    /**
     * Gets the full name by combining first and last name.
     * 
     * @return the full name
     */
    public String getFullName() {
        return firstName + " " + lastName;
    }
    
    /**
     * Checks if the user is active.
     * 
     * @return true if the user is active, false otherwise
     */
    public boolean isActive() {
        return isActive;
    }
    
    /**
     * Sets the active status of the user.
     * 
     * @param active the active status to set
     */
    public void setActive(boolean active) {
        isActive = active;
    }
    
    /**
     * Gets the creation timestamp.
     * 
     * @return the creation timestamp
     */
    public LocalDateTime getCreatedAt() {
        return createdAt;
    }
    
    /**
     * Sets the creation timestamp.
     * 
     * @param createdAt the creation timestamp to set
     */
    public void setCreatedAt(LocalDateTime createdAt) {
        this.createdAt = createdAt;
    }
    
    /**
     * Gets the last login timestamp.
     * 
     * @return the last login timestamp
     */
    public LocalDateTime getLastLoginAt() {
        return lastLoginAt;
    }
    
    /**
     * Sets the last login timestamp.
     * 
     * @param lastLoginAt the last login timestamp to set
     */
    public void setLastLoginAt(LocalDateTime lastLoginAt) {
        this.lastLoginAt = lastLoginAt;
    }
    
    /**
     * Updates the last login timestamp to current time.
     */
    public void updateLastLogin() {
        this.lastLoginAt = LocalDateTime.now();
    }
    
    /**
     * Validates if the user has all required fields.
     * 
     * @return true if user is valid, false otherwise
     */
    public boolean isValid() {
        return username != null && !username.trim().isEmpty() &&
               email != null && !email.trim().isEmpty() &&
               firstName != null && !firstName.trim().isEmpty() &&
               lastName != null && !lastName.trim().isEmpty();
    }
    
    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        User user = (User) o;
        return Objects.equals(id, user.id) &&
               Objects.equals(username, user.username) &&
               Objects.equals(email, user.email);
    }
    
    @Override
    public int hashCode() {
        return Objects.hash(id, username, email);
    }
    
    @Override
    public String toString() {
        return "User{" +
                "id=" + id +
                ", username='" + username + '\'' +
                ", email='" + email + '\'' +
                ", firstName='" + firstName + '\'' +
                ", lastName='" + lastName + '\'' +
                ", isActive=" + isActive +
                ", createdAt=" + createdAt +
                ", lastLoginAt=" + lastLoginAt +
                '}';
    }
}