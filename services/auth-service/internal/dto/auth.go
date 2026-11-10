// Package dto defines auth-service JSON shapes.
package dto

import (
	"time"

	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
)

// RegisterRequest is the body of POST /auth/register.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt max is 72 bytes
}

// LoginRequest is the body of POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse is a user as JSON. Never includes the password hash.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginResponse is returned by POST /auth/login.
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresAt   time.Time    `json:"expires_at"`
	User        UserResponse `json:"user"`
}

// FromUser never includes PasswordHash. That's why DTOs exist.
func FromUser(u *model.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}
