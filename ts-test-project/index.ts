interface User {
  id: number;
  name: string;
  email: string;
  isActive: boolean;
}

class UserService {
  private users: User[] = [];

  constructor() {
    this.initializeUsers();
  }

  private initializeUsers(): void {
    this.users = [
      { id: 1, name: "John Doe", email: "john@example.com", isActive: true },
      { id: 2, name: "Jane Smith", email: "jane@example.com", isActive: false }
    ];
  }

  public findUserById(id: number): User | undefined {
    return this.users.find(user => user.id === id);
  }

  public getAllActiveUsers(): User[] {
    return this.users.filter(user => user.isActive);
  }

  public createUser(userData: Omit<User, 'id'>): User {
    const newUser: User = {
      id: this.users.length + 1,
      ...userData
    };
    this.users.push(newUser);
    return newUser;
  }
}

const userService = new UserService();
const activeUsers = userService.getAllActiveUsers();
const specificUser = userService.findUserById(1);

export { UserService, User };
export default userService;