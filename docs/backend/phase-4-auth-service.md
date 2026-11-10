# Backend Phase 4 — Auth Service (register, login, JWT)

**Goal:** a second microservice that owns users: register, log in, and get a
signed **JWT** that other services can trust.
**Branch:** `phase/b4-auth-service` · **Tag:** `b4`

## Concepts

- **Never store passwords.** Store a **bcrypt hash**. bcrypt is deliberately
  slow and salted, so leaked hashes are very expensive to crack.
- **JWT (JSON Web Token)** = `header.payload.signature` (each base64url).
  The payload ("claims") says *who* the user is (`sub` = user ID) and *until when*
  (`exp`). The signature (HMAC-SHA256 with a secret) proves nobody changed it.
  Anyone with the secret can **verify** without a database lookup, which is why
  the gateway can check tokens on its own.
- JWTs are **signed, not encrypted**: anyone can read the payload (try
  [jwt.io](https://jwt.io)). Never put secrets in claims.
- **Same error for "no such user" and "wrong password"** (`invalid credentials`),
  so attackers can't discover which emails are registered.
- **Same code is reused**: this service has the exact same layer structure
  as todo-service. That's the payoff of a consistent architecture.

## Files in this phase

```
pkg/auth/jwt.go                       # TokenManager (shared with gateway)
pkg/middleware/auth.go                # RequireJWT middleware
services/auth-service/
├── cmd/main.go
├── internal/
│   ├── config/config.go
│   ├── dto/auth.go
│   ├── handler/auth.go
│   ├── model/user.go
│   ├── repository/postgres.go
│   ├── router/router.go
│   └── service/auth.go
├── migrations/000001_create_users.{up,down}.sql
└── go.mod
```

## Step 1 — Create the module

```bash
git checkout -b phase/b4-auth-service
mkdir -p services/auth-service/{cmd,migrations} \
         services/auth-service/internal/{config,dto,handler,model,repository,router,service}
cd services/auth-service
go mod init github.com/PRASHMANE/Go-ToDo/services/auth-service
cd ../..
go work use ./services/auth-service
```

Add the same `require` + `replace` for `pkg` to its `go.mod` as in Phase B1.

## Step 2 — `pkg/auth/jwt.go`

```bash
cd pkg && go get github.com/golang-jwt/jwt/v5@latest && cd ..
```

```go
// Package auth issues and verifies JWT access tokens.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims are the fields stored inside our JWT.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims        // embedded: sub, exp, iat, iss, jti
}

// TokenManager signs and parses tokens with one HMAC secret.
type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewTokenManager creates a TokenManager. secret must be long and random.
func NewTokenManager(secret, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// Generate returns a signed token for the user and its expiry time.
func (m *TokenManager) Generate(userID, email string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(m.ttl)
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// Parse verifies signature, algorithm, issuer and expiry, and returns the claims.
func (m *TokenManager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // blocks "alg: none" attacks
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	return claims, nil
}
```

## Step 3 — `pkg/middleware/auth.go`

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/pkg/response"
)

// ContextUserEmail holds the email from the token.
const ContextUserEmail = "user_email"

// RequireJWT validates "Authorization: Bearer <token>" and stores the user in the context.
func RequireJWT(tm *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		claims, err := tm.Parse(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		c.Set(ContextUserID, claims.Subject)
		c.Set(ContextUserEmail, claims.Email)
		c.Next()
	}
}
```

## Step 4 — Migration

`migrations/000001_create_users.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    name          VARCHAR(100) NOT NULL,
    password_hash TEXT         NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

`000001_create_users.down.sql`: `DROP TABLE IF EXISTS users;`

```bash
export AUTH_DB_URL="postgres://auth_user:auth_pass@localhost:5432/auth_db?sslmode=disable"
migrate -path services/auth-service/migrations -database "$AUTH_DB_URL" up
```

## Step 5 — `internal/model/user.go`

```go
// Package model holds auth-service domain types.
package model

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// User is a registered account.
type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

## Step 6 — `internal/repository/postgres.go`

```go
// Package repository stores users in PostgreSQL.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
)

const uniqueViolation = "23505" // Postgres error code

const userColumns = `id, email, name, password_hash, created_at, updated_at`

// UserRepository is the Postgres implementation.
type UserRepository struct{ db *pgxpool.Pool }

// NewUserRepository wraps a pool.
func NewUserRepository(db *pgxpool.Pool) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (`+userColumns+`) VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.Email, u.Name, u.PasswordHash, u.CreatedAt, u.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return model.ErrEmailTaken // the DB's UNIQUE constraint is the source of truth
	}
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
}

func (r *UserRepository) getOne(ctx context.Context, query string, arg any) (*model.User, error) {
	var u model.User
	err := r.db.QueryRow(ctx, query, arg).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}
```

`errors.As` is like `errors.Is`, but for error **types**: "is there a
`*pgconn.PgError` somewhere in this chain? If so, put it in `pgErr`."

## Step 7 — `internal/service/auth.go`

```bash
cd services/auth-service
go get golang.org/x/crypto/bcrypt@latest github.com/jackc/pgx/v5@latest github.com/google/uuid@latest github.com/gin-gonic/gin@latest
```

```go
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
```

## Step 8 — `internal/dto/auth.go`

```go
// Package dto defines auth-service JSON shapes.
package dto

import (
	"time"

	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
)

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt max is 72 bytes
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

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
```

## Step 9 — `internal/handler/auth.go`

```go
// Package handler exposes auth over HTTP.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/pkg/response"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/dto"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
	log *slog.Logger
}

func NewAuthHandler(svc *service.AuthService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

// RegisterRoutes mounts /auth routes. requireAuth protects /me.
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	g := rg.Group("/auth")
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	g.GET("/me", requireAuth, h.me)
}

func (h *AuthHandler) register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req.Email, req.Name, req.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.FromUser(u))
}

func (h *AuthHandler) login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   res.ExpiresAt,
		User:        dto.FromUser(res.User),
	})
}

func (h *AuthHandler) me(c *gin.Context) {
	u, err := h.svc.Me(c.Request.Context(), c.GetString(middleware.ContextUserID))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromUser(u))
}

func (h *AuthHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrEmailTaken):
		response.Error(c, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, model.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, model.ErrUserNotFound):
		response.Error(c, http.StatusNotFound, "not_found", "user not found")
	default:
		h.log.Error("internal error", "err", err, "request_id", c.GetString(middleware.ContextRequestID))
		response.Error(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}
```

## Step 10 — config, router, main

`internal/config/config.go`:

```go
package config

import (
	"time"

	pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"
)

type Config struct {
	Port        string
	LogLevel    string
	GinMode     string
	DatabaseURL string
	JWTSecret   string
	JWTIssuer   string
	JWTTTL      time.Duration
}

func Load() (Config, error) {
	dbURL, err := pkgconfig.Required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	secret, err := pkgconfig.Required("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Port:        pkgconfig.String("PORT", "8081"),
		LogLevel:    pkgconfig.String("LOG_LEVEL", "info"),
		GinMode:     pkgconfig.String("GIN_MODE", "debug"),
		DatabaseURL: dbURL,
		JWTSecret:   secret,
		JWTIssuer:   pkgconfig.String("JWT_ISSUER", "go-todo-auth"),
		JWTTTL:      pkgconfig.Duration("JWT_TTL", 24*time.Hour),
	}, nil
}
```

`internal/router/router.go`:

```go
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/handler"
)

func New(log *slog.Logger, ginMode string, tm *auth.TokenManager, h *handler.AuthHandler) *gin.Engine {
	gin.SetMode(ginMode)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1, middleware.RequireJWT(tm))
	return r
}
```

`cmd/main.go`, the same shape as todo-service:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/pkg/database"
	"github.com/PRASHMANE/Go-ToDo/pkg/httpserver"
	"github.com/PRASHMANE/Go-ToDo/pkg/logger"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/handler"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/router"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New("auth-service", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	svc := service.NewAuthService(repository.NewUserRepository(pool), tm)
	h := handler.NewAuthHandler(svc, log)

	return httpserver.Run(ctx, log, ":"+cfg.Port, router.New(log, cfg.GinMode, tm, h))
}
```

## Step 11 — Secrets & Makefile

Generate a real secret and put it in `.env` (and a placeholder in `.env.example`):

```bash
openssl rand -base64 48
```

`.env.example` gains:

```bash
JWT_SECRET=change-me-to-a-long-random-string
JWT_ISSUER=go-todo-auth
```

Makefile:

```makefile
AUTH_DB_URL ?= postgres://auth_user:auth_pass@localhost:5432/auth_db?sslmode=disable
JWT_SECRET  ?= dev-only-secret-change-me-dev-only-secret

migrate-auth:    ## Apply auth-service migrations
	migrate -path services/auth-service/migrations -database "$(AUTH_DB_URL)" up

run-auth:        ## Run auth-service locally
	cd services/auth-service && DATABASE_URL="$(AUTH_DB_URL)" JWT_SECRET="$(JWT_SECRET)" go run ./cmd
```

## Verify

```bash
make migrate-auth run-auth
A=localhost:8081/api/v1/auth
curl -s -X POST $A/register -H 'Content-Type: application/json' \
  -d '{"email":"Me@Example.com","name":"Prashanth","password":"supersecret"}' | jq
curl -s -X POST $A/register -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","name":"x","password":"supersecret"}' | jq      # 409 email_taken
TOKEN=$(curl -s -X POST $A/login -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","password":"supersecret"}' | jq -r .access_token)
echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null; echo     # read the claims
curl -s $A/me -H "Authorization: Bearer $TOKEN" | jq
curl -s $A/me -H "Authorization: Bearer ${TOKEN}x" | jq     # 401
curl -s -X POST $A/login -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","password":"wrong"}' | jq    # 401 invalid_credentials
psql "$AUTH_DB_URL" -c 'select email, left(password_hash, 20) from users;'   # $2a$10$...
```

## Exercises

1. Set `JWT_TTL=10s`, log in, wait 11 seconds, call `/me`. What does `Parse` return?
2. Change the secret, restart, and use an old token. Why does it fail?
3. *(Stretch)* Add **refresh tokens**: a random opaque token stored **hashed** in a
   `refresh_tokens` table, `POST /auth/refresh` to swap it for a new access token,
   and a short (15m) access-token TTL.

## Commit

```bash
git add -A
git commit -m "feat(auth-service): user registration, login and JWT issuing (B4)"
```

**Next:** [Phase 5 — API gateway](phase-5-api-gateway.md)
