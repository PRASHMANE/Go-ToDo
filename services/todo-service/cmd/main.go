// Command todo-service runs the todo HTTP API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/PRASHMANE/Go-ToDo/pkg/httpserver"
	"github.com/PRASHMANE/Go-ToDo/pkg/logger"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/router"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
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
	log := logger.New("todo-service", cfg.LogLevel)

	// ctx is cancelled when we receive Ctrl+C (SIGINT) or docker stop (SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repo := repository.NewMemoryTodoRepository()
	svc := service.NewTodoService(repo)
	todoHandler := handler.NewTodoHandler(svc, log)

	r := router.New(log, cfg.GinMode, todoHandler)
	return httpserver.Run(ctx, log, ":"+cfg.Port, r)
}
