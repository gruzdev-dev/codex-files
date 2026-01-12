package main

import (
	"log"
	"os"
	"strings"

	"codex-files/configs"
	"codex-files/migrations"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func main() {
	cfg, err := configs.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	databaseURL := cfg.DatabaseURL()
	if databaseURL == "" {
		log.Fatal("Database URL is required")
	}

	if strings.HasPrefix(databaseURL, "postgres://") {
		databaseURL = strings.Replace(databaseURL, "postgres://", "pgx5://", 1)
	} else if strings.HasPrefix(databaseURL, "postgresql://") {
		databaseURL = strings.Replace(databaseURL, "postgresql://", "pgx5://", 1)
	}

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatalf("Failed to create source driver: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}

	log.Println("Running database migrations...")
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("No migrations to apply")
			os.Exit(0)
		}
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migrations completed successfully")
}
