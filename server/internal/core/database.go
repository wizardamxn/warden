package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenDB connects to Postgres and checks the connection works. TLS and the
// rest come from the URL itself, such as ?sslmode=require for Supabase.
func OpenDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		// Not wrapped: the URL holds the database password.
		return nil, errors.New("DATABASE_URL is not a valid Postgres URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, errors.New("DATABASE_URL could not be used")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		// pgx names the host and user here, never the password.
		return nil, fmt.Errorf("connect to the database: %w", err)
	}
	return pool, nil
}
