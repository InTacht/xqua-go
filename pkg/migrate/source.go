package migrate

import (
	"fmt"
	"io/fs"
	"os"
)

// Source locates and reads migration SQL files.
//
// When FS is nil, migrations are read from the operating system filesystem
// rooted at Dir. When FS is set (for example an embed.FS), Dir is the path
// prefix within FS that contains the migration files. An empty Dir with a set FS
// reads from the FS root.
type Source struct {
	Dir string
	FS  fs.FS
}

func (s Source) resolve() (fs.FS, string) {
	if s.FS != nil {
		dir := s.Dir
		if dir == "" {
			dir = "."
		}
		return s.FS, dir
	}
	return os.DirFS(s.Dir), "."
}

// Discover scans the source for migration files, sorted by version ascending.
func (s Source) Discover() ([]Migration, error) {
	fsys, dir := s.resolve()
	return DiscoverFS(fsys, dir)
}

// ReadUp returns the contents of the migration's up file.
func (s Source) ReadUp(m Migration) (string, error) {
	return s.read(m.UpPath)
}

// ReadDown returns the contents of the migration's down file. It is an error to
// call ReadDown for a migration without a down file.
func (s Source) ReadDown(m Migration) (string, error) {
	if m.DownPath == "" {
		return "", fmt.Errorf("migrate: version %s has no .down.sql file", FormatVersion(m.Version))
	}
	return s.read(m.DownPath)
}

func (s Source) read(name string) (string, error) {
	fsys, _ := s.resolve()
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", fmt.Errorf("migrate: read %q: %w", name, err)
	}
	return string(data), nil
}
