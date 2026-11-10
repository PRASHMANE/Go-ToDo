// Command todo-service runs the todo HTTP API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"
	"github.com/PRASHMANE/Go-ToDo/pkg/database"
	"github.com/PRASHMANE/Go-ToDo/pkg/httpserver"
	"github.com/PRASHMANE/Go-ToDo/pkg/logger"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/router"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

func main() {
	// `app healthcheck` is used by Docker HEALTHCHECK (no curl in distroless images).
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(httpserver.Healthcheck(pkgconfig.String("PORT", "8082")))
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
	log := logger.New("todo-service", cfg.LogLevel)

	// ctx is cancelled when we receive Ctrl+C (SIGINT) or docker stop (SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var repo service.TodoRepository
	if cfg.DatabaseURL != "" {
		pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		repo = repository.NewPostgresTodoRepository(pool)
		log.Info("using postgres repository")
	} else {
		repo = repository.NewMemoryTodoRepository()
		log.Warn("DATABASE_URL not set, using in-memory repository")
	}

	svc := service.NewTodoService(repo)
	todoHandler := handler.NewTodoHandler(svc, log)

	r := router.New(log, cfg.GinMode, todoHandler)
	return httpserver.Run(ctx, log, ":"+cfg.Port, r)
}
