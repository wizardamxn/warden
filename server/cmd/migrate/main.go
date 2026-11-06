// The migrate program brings the database up to date. Run it before the API and
// the worker, from server/: go run ./cmd/migrate
package main

import (
	"context"
	"fmt"
	"os"

	"warden/internal/core"
	"warden/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	settings, err := core.Load(".env")
	if err != nil {
		return err
	}
	if err := settings.Require("DATABASE_URL"); err != nil {
		return err
	}

	ctx := context.Background()
	db, err := core.OpenDB(ctx, settings.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	applied, err := migrations.Apply(ctx, db)
	if err != nil {
		return err
	}
	fmt.Printf("%d migrations applied %v\n", len(applied), applied)
	return nil
}
