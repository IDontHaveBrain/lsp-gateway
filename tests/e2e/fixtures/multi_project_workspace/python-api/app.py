from datetime import datetime
from typing import List, Optional, Dict, Any
from fastapi import FastAPI, HTTPException, Depends, status
from pydantic import BaseModel, EmailStr, Field
import uvicorn


class UserBase(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    email: EmailStr
    age: Optional[int] = Field(None, ge=0, le=150)


class UserCreate(UserBase):
    password: str = Field(..., min_length=8)


class UserUpdate(BaseModel):
    name: Optional[str] = Field(None, min_length=1, max_length=100)
    email: Optional[EmailStr] = None
    age: Optional[int] = Field(None, ge=0, le=150)


class UserResponse(UserBase):
    id: int
    created_at: datetime
    updated_at: Optional[datetime] = None
    is_active: bool = True

    class Config:
        from_attributes = True


class UserDatabase:
    def __init__(self):
        self._users: Dict[int, Dict[str, Any]] = {}
        self._next_id: int = 1

    async def create_user(self, user_data: UserCreate) -> UserResponse:
        user_dict = {
            "id": self._next_id,
            "name": user_data.name,
            "email": user_data.email,
            "age": user_data.age,
            "created_at": datetime.now(),
            "updated_at": None,
            "is_active": True,
        }
        self._users[self._next_id] = user_dict
        self._next_id += 1
        return UserResponse(**user_dict)

    async def get_user(self, user_id: int) -> Optional[UserResponse]:
        user_dict = self._users.get(user_id)
        return UserResponse(**user_dict) if user_dict else None

    async def get_users(self, skip: int = 0, limit: int = 100) -> List[UserResponse]:
        users = list(self._users.values())[skip:skip + limit]
        return [UserResponse(**user) for user in users]

    async def update_user(self, user_id: int, user_update: UserUpdate) -> Optional[UserResponse]:
        user_dict = self._users.get(user_id)
        if not user_dict:
            return None

        update_data = user_update.model_dump(exclude_unset=True)
        if update_data:
            user_dict.update(update_data)
            user_dict["updated_at"] = datetime.now()

        return UserResponse(**user_dict)

    async def delete_user(self, user_id: int) -> bool:
        return self._users.pop(user_id, None) is not None


class UserService:
    def __init__(self, database: UserDatabase):
        self.database = database

    async def create_user(self, user_data: UserCreate) -> UserResponse:
        return await self.database.create_user(user_data)

    async def get_user_by_id(self, user_id: int) -> UserResponse:
        user = await self.database.get_user(user_id)
        if not user:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"User with id {user_id} not found"
            )
        return user

    async def get_all_users(self, skip: int = 0, limit: int = 100) -> List[UserResponse]:
        return await self.database.get_users(skip=skip, limit=limit)

    async def update_user(self, user_id: int, user_update: UserUpdate) -> UserResponse:
        user = await self.database.update_user(user_id, user_update)
        if not user:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"User with id {user_id} not found"
            )
        return user

    async def delete_user(self, user_id: int) -> Dict[str, str]:
        success = await self.database.delete_user(user_id)
        if not success:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"User with id {user_id} not found"
            )
        return {"message": "User deleted successfully"}


app = FastAPI(title="Python API", description="User management API", version="1.0.0")
db = UserDatabase()
user_service = UserService(db)


def get_user_service() -> UserService:
    return user_service


@app.post("/users/", response_model=UserResponse, status_code=status.HTTP_201_CREATED)
async def create_user(
    user_data: UserCreate,
    service: UserService = Depends(get_user_service)
) -> UserResponse:
    return await service.create_user(user_data)


@app.get("/users/{user_id}", response_model=UserResponse)
async def get_user(
    user_id: int,
    service: UserService = Depends(get_user_service)
) -> UserResponse:
    return await service.get_user_by_id(user_id)


@app.get("/users/", response_model=List[UserResponse])
async def get_users(
    skip: int = 0,
    limit: int = 100,
    service: UserService = Depends(get_user_service)
) -> List[UserResponse]:
    return await service.get_all_users(skip=skip, limit=limit)


@app.put("/users/{user_id}", response_model=UserResponse)
async def update_user(
    user_id: int,
    user_update: UserUpdate,
    service: UserService = Depends(get_user_service)
) -> UserResponse:
    return await service.update_user(user_id, user_update)


@app.delete("/users/{user_id}")
async def delete_user(
    user_id: int,
    service: UserService = Depends(get_user_service)
) -> Dict[str, str]:
    return await service.delete_user(user_id)


@app.get("/health")
async def health_check() -> Dict[str, str]:
    return {"status": "healthy", "timestamp": datetime.now().isoformat()}


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8082)