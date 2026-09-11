package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Serve runs srv until it either fails to serve or ctx is cancelled, then
// gracefully shuts it down within drainTimeout (PRD story 58). Graceful
// shutdown stops accepting new connections and waits for in-flight requests to
// return before closing idle connections, so no request is cut off mid-flight.
//
// ctx is the shutdown trigger: cancel it (cmd/server derives it from
// SIGINT/SIGTERM via signal.NotifyContext) to begin draining. Serve returns
// nil on a clean drain, the listener error if the server failed to serve, or a
// wrapped error if the drain exceeded drainTimeout (in which case the remaining
// connections were force-closed). It blocks until the server has fully stopped,
// so callers can run post-serve cleanup (closing pools, flushing metrics)
// knowing no handler is still executing.
func Serve(ctx context.Context, srv *http.Server, drainTimeout time.Duration, logger *slog.Logger) error {
	serveErr := make(chan error, 1)
	go func() {
		// ErrServerClosed is the normal result of Shutdown; treat it as a clean
		// stop, not a failure.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		// The server stopped on its own (e.g. the port was already in use)
		// before any shutdown was requested.
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining in-flight requests")
	}

	// A fresh context (ctx is already cancelled) bounds how long we wait for
	// in-flight requests to drain before forcing connections closed.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	// Wait for ListenAndServe to actually return so the caller knows the
	// listener is fully closed before running its own cleanup.
	if err := <-serveErr; err != nil {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}
