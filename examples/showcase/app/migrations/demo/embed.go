package migrations

import "embed"

// FS holds SQL migrations for the demo Postgres database.
//
//go:embed *.sql
var FS embed.FS
