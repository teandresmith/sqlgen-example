// Command migrate applies the ./migrations DDL to the configured database as an
// explicit step, for operators who prefer to run migrations out of band rather
// than at server startup (both paths call internal/migrate.Up, so they are
// equivalent). It reads only what it needs — DATABASE_URL and, optionally,
// MIGRATIONS_DIR — so migrating does not require the full server configuration.
package main

import (
	"log/slog"
	"os"

	"github.com/teandresmith/sqlgen-example/internal/migrate"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "./migrations"
	}

	logger.Info("applying migrations", "dir", dir)
	if err := migrate.Up(databaseURL, dir); err != nil {
		logger.Error("applying migrations", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")
}
