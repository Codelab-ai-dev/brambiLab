// Package db builds the PostgreSQL configuration shared by the API and migrations.
package db

import (
	"errors"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config returns the pool configuration. The preferred source is the standard libpq
// variables (PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE, PGSSLMODE): pgx reads each
// value verbatim, so passwords with URI-reserved characters (@ / # : ? %) need no escaping.
// A non-empty databaseURL (DATABASE_URL) is still accepted when it is already correctly encoded.
func Config(databaseURL string) (*pgxpool.Config, error) {
	if databaseURL == "" && os.Getenv("PGHOST") == "" {
		return nil, errors.New("database not configured: set PGHOST and related PG* variables, or DATABASE_URL")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// Do not wrap: parse errors may echo the connection string, including the password.
		if databaseURL != "" {
			return nil, errors.New("DATABASE_URL is not a valid PostgreSQL connection string")
		}
		return nil, errors.New("PG* environment variables do not form a valid PostgreSQL configuration")
	}
	return cfg, nil
}
