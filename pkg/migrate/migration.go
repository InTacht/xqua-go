package migrate

import "fmt"

// Migration is a single versioned migration discovered from a Source.
//
// UpPath and DownPath are relative to the Source root and are used both for
// reading SQL and for logging. DownPath is empty when no .down.sql file exists
// for the version.
type Migration struct {
	Version  int64
	Name     string
	UpPath   string
	DownPath string
}

// Label renders a migration as "000001_name" for CLI output and logs.
func (m Migration) Label() string {
	return FormatVersion(m.Version) + "_" + m.Name
}

// FormatVersion renders a version as a zero-padded six-digit string.
func FormatVersion(v int64) string {
	return fmt.Sprintf("%06d", v)
}
