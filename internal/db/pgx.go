// Package db provides PostgreSQL database connectivity via pgx.
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool creates a new connection pool to the given PostgreSQL DSN.
func Pool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}
