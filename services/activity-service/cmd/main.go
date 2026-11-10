// Command activity-service consumes todo events and serves the activity feed.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"
	"github.com/PRASHMANE/Go-ToDo/pkg/database"
	pkgevents "github.com/PRASHMANE/Go-ToDo/pkg/events"
	"github.com/PRASHMANE/Go-ToDo/pkg/httpserver"
	"github.com/PRASHMANE/Go-ToDo/pkg/logger"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/consumer"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/handler"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/router"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/service"
)

func main() {
	// `app healthcheck` is used by Docker HEALTHCHECK (no curl in distroless images).
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(httpserver.Healthcheck(pkgconfig.String("PORT", "8083")))
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
	log := logger.New("activity-service", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	nc, js, err := pkgevents.Connect(cfg.NATSURL, "activity-service")
	if err != nil {
		return err
	}
	defer func() { _ = nc.Drain() }()
	if err := pkgevents.EnsureStream(ctx, js); err != nil { // either side may start first
		return err
	}

	repo := repository.NewActivityRepository(pool)
	svc := service.NewActivityService(repo)
	r := router.New(log, cfg.GinMode, handler.NewActivityHandler(svc, log))

	// Run the HTTP server AND the consumer concurrently. If either fails,
	// gctx is cancelled and the other one shuts down too.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return consumer.Run(gctx, js, svc, log) })
	g.Go(func() error { return httpserver.Run(gctx, log, ":"+cfg.Port, r) })
	return g.Wait()
}
