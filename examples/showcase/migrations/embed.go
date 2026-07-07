package migrations

import "embed"

// FS holds the SQL migrations shipped with the api example.
//
//go:embed *.sql
var FS embed.FS
