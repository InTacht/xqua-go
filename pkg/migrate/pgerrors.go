package migrate

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	// sqlstateUndefinedTable is returned when a referenced relation does not
	// exist (the schema exists but the table does not).
	sqlstateUndefinedTable = "42P01"
	// sqlstateInvalidSchemaName is returned when a referenced schema does not
	// exist, which happens on a fresh database before the bookkeeping schema is
	// created.
	sqlstateInvalidSchemaName = "3F000"
)

// isMissingObject reports whether err indicates that the bookkeeping schema or
// one of its tables does not exist yet. Such cases are treated as an empty/idle
// database before the library has bootstrapped its tables.
func isMissingObject(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == sqlstateUndefinedTable || pgErr.Code == sqlstateInvalidSchemaName
	}
	return false
}
