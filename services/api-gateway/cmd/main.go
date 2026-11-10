// Command api-gateway is the single public entry point to the backend.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"
	"github.com/PRASHMANE/Go-ToDo/pkg/httpserver"
	"github.com/PRASHMANE/Go-ToDo/pkg/logger"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/proxy"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/router"
)

func main() {
	// `app healthcheck` is used by Docker HEALTHCHECK (no curl in distroless images).
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(httpserver.Healthcheck(pkgconfig.String("PORT", "8080")))
	}
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
	log := logger.New("api-gateway", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authProxy, err := proxy.New(cfg.AuthServiceURL, log)
	if err != nil {
		return err
	}
	todoProxy, err := proxy.New(cfg.TodoServiceURL, log)
	if err != nil {
		return err
	}

	// The gateway only VERIFIES tokens, so the TTL value here is irrelevant.
	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, time.Hour)

	r, err := router.New(cfg, log, tm, router.Upstreams{Auth: authProxy, Todo: todoProxy})
	if err != nil {
		return err
	}
	return httpserver.Run(ctx, log, ":"+cfg.Port, r)
}
