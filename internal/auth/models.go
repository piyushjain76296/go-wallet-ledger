package auth

import (
	"time"
)

type Role string

const (
	RoleUser        Role = "USER"
	RoleMerchant    Role = "MERCHANT"
	RoleDistributor Role = "DISTRIBUTOR"
	RolePlatform    Role = "PLATFORM"
	RoleSystem      Role = "SYSTEM"
	RoleAdmin       Role = "ADMIN"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}
