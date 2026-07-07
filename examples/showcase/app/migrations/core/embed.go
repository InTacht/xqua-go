package migrations

import "embed"

// FS holds SQL migrations for the core Postgres database.
//
//go:embed *.sql
var FS embed.FS
