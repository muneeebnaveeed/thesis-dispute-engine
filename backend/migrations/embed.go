// Package migrations embeds the SQL files so the binary migrates itself.
package migrations

import "embed"

// FS holds every NNNN_name.sql in this directory.
//
//go:embed *.sql
var FS embed.FS
