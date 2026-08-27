package database

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// AutoMigrate applies the database schema if not already present
func AutoMigrate(ctx context.Context, pool *pgxpool.Pool) error {
	log.Printf("[DATABASE] Running automatic schema migration check...")
	_, err := pool.Exec(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}
	log.Printf("[DATABASE] Schema migrations verified and applied successfully.")
	return nil
}
