// Command seed populates a realistic multi-workspace dataset so an operator can
// explore the API immediately (issue 16, PRD story 54). It reuses the server's
// exact wiring: it reads the same configuration, applies the same migrations,
// and builds the same *database.Client (internal/appdb) — so every row it writes
// goes through the real tenant scoping, authz, cache, and event publishing and
// is queryable through the GraphQL API under the appropriate Active Workspace.
//
// Seeding assumes a fresh database. If any Workspace already exists the command
// refuses to run rather than colliding with existing data on a unique
// constraint, so re-invoking it is a safe no-op instead of a confusing failure.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/teandresmith/sqlgen-example/internal/appdb"
	"github.com/teandresmith/sqlgen-example/internal/auth"
	"github.com/teandresmith/sqlgen-example/internal/config"
	"github.com/teandresmith/sqlgen-example/internal/migrate"
	"github.com/teandresmith/sqlgen-example/internal/seed"
	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("seed exited with error", "err", err)
		os.Exit(1)
	}
}

// run performs the full seed sequence and returns an error rather than calling
// os.Exit, so deferred cleanup runs and the failure is logged in one place.
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Migrations are the single source of truth (ADR-0002); apply them first so
	// the schema is present, exactly as the server does at startup.
	logger.Info("applying migrations", "dir", cfg.MigrationsDir)
	if err := migrate.Up(cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		return err
	}

	ctx := context.Background()

	// The same client construction the server uses (issue 16 acceptance). The
	// seeder has no metrics pipeline, so it passes a nil cache recorder.
	stack, err := appdb.New(ctx, cfg, nil, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := stack.Close(); err != nil {
			logger.Warn("closing data-access stack", "err", err)
		}
	}()

	// Refuse to seed a non-empty database: the seeder creates rows with unique
	// emails and slugs and does not upsert, so a second run would fail on a
	// constraint violation partway through. Counting Workspaces (untenanted)
	// needs no Active Workspace.
	count, err := stack.Client.Workspaces().Count(ctx, nil)
	if err != nil {
		return fmt.Errorf("checking for existing data: %w", err)
	}
	if count > 0 {
		logger.Warn("database already contains workspaces; skipping seed", "workspaces", count)
		return nil
	}

	authSvc := auth.NewService(cfg.JWTSecret, auth.DefaultTokenTTL)
	summary, err := seed.Run(ctx, stack.Client, authSvc.HashPassword, logger)
	if err != nil {
		return err
	}

	printSummary(summary)
	return nil
}

// printSummary writes a human-readable report to stdout so the operator can see
// what was created and how to reach it — the Workspace ids to put in the
// X-Workspace-Id header and the shared login to authenticate with.
func printSummary(s *seed.Summary) {
	fmt.Println("Seeded database:")
	// Sort the entity names so the summary prints in a stable order regardless
	// of Go's randomized map iteration.
	names := make([]string, 0, len(s.Totals))
	for name := range s.Totals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("  %-12s %d\n", name, s.Totals[name])
	}
	fmt.Println()
	fmt.Println("Workspaces (set the", tenancy.WorkspaceHeader, "header to one of these ids):")
	for _, ws := range s.Workspaces {
		fmt.Printf("  %-10s %s  owner=%s\n", ws.Slug, ws.ID, ws.OwnerEmail)
	}
	fmt.Println()
	fmt.Printf("Every seeded user's password is %q.\n", s.Password)
	fmt.Println("Log in via the `login` mutation, then send the JWT as `Authorization: Bearer <token>`")
	fmt.Printf("with `%s: <workspace-id>` to explore that workspace's data.\n", tenancy.WorkspaceHeader)
}
