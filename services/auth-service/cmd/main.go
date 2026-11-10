// Command auth-service runs the authentication HTTP API.
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
