// Package httpserver runs an http.Server with sane timeouts and graceful shutdown.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Run starts the server and blocks until ctx is cancelled (SIGINT/SIGTERM)
// or the server fails to start. On cancellation it shuts down gracefully.
func Run(ctx context.Context, log *slog.Logger, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second, // protects against slowloris
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", addr)
		// ListenAndServe blocks. It always returns a non-nil error;
		// ErrServerClosed just means "Shutdown was called", which is normal.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh: // failed to start (e.g. port already in use)
		return err
	case <-ctx.Done(): // got a shutdown signal
	}

	log.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("http server stopped")
	return nil
}

// Healthcheck GETs http://127.0.0.1:<port>/healthz and returns an exit code.
// Used by Docker: `CMD ["/app", "healthcheck"]` (distroless images have no curl).
func Healthcheck(port string) int {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
