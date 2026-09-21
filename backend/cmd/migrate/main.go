// Command migrate applies pending migrations as the schema owner and exits; the API itself never migrates.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/config"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/postgres"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, cfg.MigrateDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	files, err := postgres.Load(migrations.FS)
	if err != nil {
		return err
	}
	ran, err := postgres.Migrate(ctx, pool, files)
	if err != nil {
		return err
	}
	fmt.Printf("migrate: applied %d of %d\n", ran, len(files))
	return nil
}
