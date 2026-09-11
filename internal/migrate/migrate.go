// Package migrate applies the ./migrations/*.up.sql files to Postgres at
// startup. These are the same DDL files sqlgen parses to generate the client
// (ADR-0002), so applying them here keeps the running schema and the generated
// code from drifting. golang-migrate is the applier; the up/down file naming
// matches sqlgen's parser, which skips *.down.sql.
package migrate

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	// pgx/v5 database driver (registers the "pgx5" URL scheme).
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	// file source driver (registers the "file" URL scheme).
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Up applies every pending up-migration in dir to the database at databaseURL,
// then closes the migrator. It is a no-op when the database is already at the
// latest version. dir may be relative; it is resolved to an absolute path so
// the file source works regardless of the caller's working directory (the
// server runs from the module root, tests from their package directory).
func Up(databaseURL, dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("migrate: resolving migrations dir %q: %w", dir, err)
	}
	sourceURL := "file://" + filepath.ToSlash(absDir)

	m, err := migrate.New(sourceURL, pgxScheme(databaseURL))
	if err != nil {
		return fmt.Errorf("migrate: opening migrator: %w", err)
	}
	// Close releases both the source and database handles; the database
	// connection here is migrate's own, separate from the app's pgx pool.
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: applying migrations: %w", err)
	}
	return nil
}

// pgxScheme rewrites a postgres:// (or postgresql://) connection string to the
// pgx5:// scheme golang-migrate's pgx/v5 driver registers, leaving any string
// that already uses a migrate scheme untouched.
func pgxScheme(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(databaseURL, prefix); ok {
			return "pgx5://" + rest
		}
	}
	return databaseURL
}
