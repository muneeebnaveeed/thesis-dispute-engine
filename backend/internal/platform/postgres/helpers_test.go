package postgres_test

import (
	"io/fs"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/migrations"
)

func migrationsFS(t *testing.T) fs.FS {
	t.Helper()
	return migrations.FS
}
