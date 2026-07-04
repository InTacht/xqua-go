package migrate

import (
	"fmt"
	"regexp"
	"time"
)

const (
	// defaultAdvisoryLockKey is the global Postgres advisory lock key used to
	// serialize migrations across replicas. Apps sharing a database should set a
	// distinct AdvisoryLockKey.
	defaultAdvisoryLockKey int64 = 839274928374
	defaultPollInterval          = 500 * time.Millisecond

	// defaultSchema is the Postgres schema that holds the library's own
	// bookkeeping tables (kept separate from application tables).
	defaultSchema = "_xqua_"
)

// schemaNameRegex restricts the bookkeeping schema to a safe, unquoted-style
// identifier so it can be interpolated into DDL without injection risk.
var schemaNameRegex = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// Config configures a Migrator.
//
// Source and InstanceID are required. AdvisoryLockKey, PollInterval, and Schema
// fall back to defaults when left at their zero value.
type Config struct {
	Source          Source
	InstanceID      string
	AdvisoryLockKey int64
	PollInterval    time.Duration

	// Schema is the Postgres schema for the library's tracking tables
	// (schema_migrations, migration_state, and the internal meta table).
	// Defaults to "_xqua_". Application migrations are unaffected and
	// continue to target whatever schema their SQL specifies.
	Schema string
}

func (cfg Config) validate() error {
	if cfg.InstanceID == "" {
		return fmt.Errorf("migrate: InstanceID is required")
	}
	if cfg.Source.FS == nil && cfg.Source.Dir == "" {
		return fmt.Errorf("migrate: Source.Dir is required when Source.FS is nil")
	}
	return nil
}

func (cfg *Config) applyDefaults() {
	if cfg.AdvisoryLockKey == 0 {
		cfg.AdvisoryLockKey = defaultAdvisoryLockKey
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.Schema == "" {
		cfg.Schema = defaultSchema
	}
}

func validateSchemaName(schema string) error {
	if len(schema) > 63 {
		return fmt.Errorf("migrate: Schema %q exceeds 63 characters", schema)
	}
	if !schemaNameRegex.MatchString(schema) {
		return fmt.Errorf("migrate: Schema %q must match %s", schema, schemaNameRegex.String())
	}
	return nil
}
