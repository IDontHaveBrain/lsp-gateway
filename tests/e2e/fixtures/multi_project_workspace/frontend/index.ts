import React, { useState, useEffect, useCallback } from 'react';
import { createRoot } from 'react-dom/client';
import axios, { AxiosResponse } from 'axios';

interface User {
  id: number;
  name: string;
  email: string;
  age?: number;
  created_at: string;
  updated_at?: string;
  is_active: boolean;
}

interface CreateUserRequest {
  name: string;
  email: string;
  age?: number;
  password: string;
}

interface UpdateUserRequest {
  name?: string;
  email?: string;
  age?: number;
}

interface ApiResponse<T> {
  data: T;
  status: number;
  message?: string;
}

class UserApiClient {
  private baseUrl: string;

  constructor(baseUrl: string = 'http://localhost:8082') {
    this.baseUrl = baseUrl;
  }

  async getUsers(): Promise<User[]> {
    const response: AxiosResponse<User[]> = await axios.get(`${this.baseUrl}/users/`);
    return response.data;
  }

  async getUser(id: number): Promise<User> {
    const response: AxiosResponse<User> = await axios.get(`${this.baseUrl}/users/${id}`);
    return response.data;
  }

  async createUser(userData: CreateUserRequest): Promise<User> {
    const response: AxiosResponse<User> = await axios.post(`${this.baseUrl}/users/`, userData);
    return response.data;
  }

  async updateUser(id: number, userData: UpdateUserRequest): Promise<User> {
    const response: AxiosResponse<User> = await axios.put(`${this.baseUrl}/users/${id}`, userData);
    return response.data;
  }

  async deleteUser(id: number): Promise<void> {
    await axios.delete(`${this.baseUrl}/users/${id}`);
  }
}

interface UserListProps {
  users: User[];
  onUserSelect: (user: User) => void;
  onUserDelete: (id: number) => void;
}

const UserList: React.FC<UserListProps> = ({ users, onUserSelect, onUserDelete }) => {
  return (
    <div className="user-list">
      <h2>Users</h2>
      {users.length === 0 ? (
        <p>No users found</p>
      ) : (
        <ul>
          {users.map((user) => (
            <li key={user.id} className="user-item">
              <div onClick={() => onUserSelect(user)} className="user-info">
                <strong>{user.name}</strong> ({user.email})
                {user.age && <span> - Age: {user.age}</span>}
              </div>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onUserDelete(user.id);
                }}
                className="delete-btn"
              >
                Delete
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

interface UserFormProps {
  user?: User;
  onSubmit: (userData: CreateUserRequest | UpdateUserRequest) => void;
  onCancel: () => void;
}

const UserForm: React.FC<UserFormProps> = ({ user, onSubmit, onCancel }) => {
  const [formData, setFormData] = useState({
    name: user?.name || '',
    email: user?.email || '',
    age: user?.age?.toString() || '',
    password: ''
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const userData = {
      name: formData.name,
      email: formData.email,
      age: formData.age ? parseInt(formData.age) : undefined,
      ...(user ? {} : { password: formData.password })
    };
    onSubmit(userData);
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

  return (
    <form onSubmit={handleSubmit} className="user-form">
      <h2>{user ? 'Edit User' : 'Create User'}</h2>
      
      <div className="form-group">
        <label htmlFor="name">Name:</label>
        <input
          type="text"
          id="name"
          name="name"
          value={formData.name}
          onChange={handleInputChange}
          required
        />
      </div>

      <div className="form-group">
        <label htmlFor="email">Email:</label>
        <input
          type="email"
          id="email"
          name="email"
          value={formData.email}
          onChange={handleInputChange}
          required
        />
      </div>

      <div className="form-group">
        <label htmlFor="age">Age:</label>
        <input
          type="number"
          id="age"
          name="age"
          value={formData.age}
          onChange={handleInputChange}
          min="0"
          max="150"
        />
      </div>

      {!user && (
        <div className="form-group">
          <label htmlFor="password">Password:</label>
          <input
            type="password"
            id="password"
            name="password"
            value={formData.password}
            onChange={handleInputChange}
            required
            minLength={8}
          />
        </div>
      )}

      <div className="form-actions">
        <button type="submit">{user ? 'Update' : 'Create'}</button>
        <button type="button" onClick={onCancel}>Cancel</button>
      </div>
    </form>
  );
};

const App: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  const apiClient = new UserApiClient();

  const fetchUsers = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const usersData = await apiClient.getUsers();
      setUsers(usersData);
    } catch (err) {
      setError('Failed to fetch users');
      console.error('Error fetching users:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const handleCreateUser = async (userData: CreateUserRequest) => {
    try {
      const newUser = await apiClient.createUser(userData);
      setUsers(prev => [...prev, newUser]);
      setShowForm(false);
    } catch (err) {
      setError('Failed to create user');
      console.error('Error creating user:', err);
    }
  };

  const handleUpdateUser = async (userData: UpdateUserRequest) => {
    if (!selectedUser) return;
    
    try {
      const updatedUser = await apiClient.updateUser(selectedUser.id, userData);
      setUsers(prev => prev.map(user => 
        user.id === updatedUser.id ? updatedUser : user
      ));
      setSelectedUser(null);
      setShowForm(false);
    } catch (err) {
      setError('Failed to update user');
      console.error('Error updating user:', err);
    }
  };

  const handleDeleteUser = async (id: number) => {
    try {
      await apiClient.deleteUser(id);
      setUsers(prev => prev.filter(user => user.id !== id));
      if (selectedUser?.id === id) {
        setSelectedUser(null);
      }
    } catch (err) {
      setError('Failed to delete user');
      console.error('Error deleting user:', err);
    }
  };

  const handleUserSelect = (user: User) => {
    setSelectedUser(user);
    setShowForm(true);
  };

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  return (
    <div className="app">
      <header>
        <h1>Multi-Project Frontend</h1>
        <button onClick={() => setShowForm(true)}>Add User</button>
      </header>

      {error && <div className="error">{error}</div>}
      
      {isLoading ? (
        <div className="loading">Loading...</div>
      ) : (
        <>
          {showForm ? (
            <UserForm
              user={selectedUser}
              onSubmit={selectedUser ? handleUpdateUser : handleCreateUser}
              onCancel={() => {
                setShowForm(false);
                setSelectedUser(null);
              }}
            />
          ) : (
            <UserList
              users={users}
              onUserSelect={handleUserSelect}
              onUserDelete={handleDeleteUser}
            />
          )}
        </>
      )}
    </div>
  );
};

const container = document.getElementById('root');
if (!container) {
  throw new Error('Root element not found');
}

const root = createRoot(container);
root.render(React.createElement(App));