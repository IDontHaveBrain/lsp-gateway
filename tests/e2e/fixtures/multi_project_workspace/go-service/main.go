package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Created  time.Time `json:"created"`
}

type UserService interface {
	GetUser(id int) (*User, error)
	CreateUser(name, email string) (*User, error)
	UpdateUser(user *User) error
	DeleteUser(id int) error
}

type InMemoryUserService struct {
	users  map[int]*User
	nextID int
}

func NewInMemoryUserService() UserService {
	return &InMemoryUserService{
		users:  make(map[int]*User),
		nextID: 1,
	}
}

func (s *InMemoryUserService) GetUser(id int) (*User, error) {
	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}
	return user, nil
}

func (s *InMemoryUserService) CreateUser(name, email string) (*User, error) {
	user := &User{
		ID:      s.nextID,
		Name:    name,
		Email:   email,
		Created: time.Now(),
	}
	s.users[s.nextID] = user
	s.nextID++
	return user, nil
}

func (s *InMemoryUserService) UpdateUser(user *User) error {
	if _, exists := s.users[user.ID]; !exists {
		return fmt.Errorf("user with ID %d not found", user.ID)
	}
	s.users[user.ID] = user
	return nil
}

func (s *InMemoryUserService) DeleteUser(id int) error {
	if _, exists := s.users[id]; !exists {
		return fmt.Errorf("user with ID %d not found", id)
	}
	delete(s.users, id)
	return nil
}

type UserHandler struct {
	service UserService
}

func NewUserHandler(service UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.service.GetUser(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	user, err := h.service.CreateUser(req.Name, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func main() {
	service := NewInMemoryUserService()
	handler := NewUserHandler(service)

	router := mux.NewRouter()
	
	router.HandleFunc("/users/{id:[0-9]+}", handler.GetUserHandler).Methods("GET")
	router.HandleFunc("/users", handler.CreateUserHandler).Methods("POST")
	
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE"}),
		handlers.AllowedHeaders([]string{"Content-Type"}),
	)

	server := &http.Server{
		Addr:         ":8081",
		Handler:      corsHandler(router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	log.Printf("Go service starting on port 8081")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}