// Package service implements registration and login.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
)

// UserRepository is what the service needs from storage.
type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

// LoginResult is returned on successful login.
type LoginResult struct {
	AccessToken string
	ExpiresAt   time.Time
	User        *model.User
}

// AuthService implements auth use cases.
type AuthService struct {
	users  UserRepository
	tokens *auth.TokenManager
	// dummyHash is compared when the user doesn't exist, so "unknown email"
	// takes as long as "wrong password" (prevents timing attacks).
	dummyHash []byte
}

// NewAuthService builds the service.
func NewAuthService(users UserRepository, tokens *auth.TokenManager) *AuthService {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)
	return &AuthService{users: users, tokens: tokens, dummyHash: dummy}
}

// Register creates a new user with a hashed password.
func (s *AuthService) Register(ctx context.Context, email, name, password string) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().UTC()
	u := &model.User{
		ID:           uuid.NewString(),
		Email:        normalizeEmail(email),
		Name:         strings.TrimSpace(name),
		PasswordHash: string(hash),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login checks credentials and issues an access token.
func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	u, err := s.users.GetByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, model.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password)) // equalise timing
		return nil, model.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, model.ErrInvalidCredentials
	}

	token, exp, err := s.tokens.Generate(u.ID, u.Email)
	if err != nil {
		return nil, err
	}
	return &LoginResult{AccessToken: token, ExpiresAt: exp, User: u}, nil
}

// Me returns the current user.
func (s *AuthService) Me(ctx context.Context, userID string) (*model.User, error) {
	return s.users.GetByID(ctx, userID)
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }
