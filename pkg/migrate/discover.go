package migrate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
)

// filenameRegex matches migration file names: a six-digit zero-padded version,
// a snake_case name, and an up/down direction, e.g. 000001_initial.up.sql.
var filenameRegex = regexp.MustCompile(`^(\d{6})_([a-z0-9_]+)\.(up|down)\.sql$`)

// Discover scans a filesystem directory for migration files.
func Discover(dir string) ([]Migration, error) {
	return DiscoverFS(os.DirFS(dir), ".")
}

// DiscoverFS scans dir within fsys for migration files, validating that every
// version has an up file and returning them sorted by version ascending.
//
// Files that do not match the naming convention are ignored. The returned
// UpPath and DownPath are paths within fsys, suitable for fs.ReadFile.
func DiscoverFS(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: read dir %q: %w", dir, err)
	}

	type pending struct {
		version int64
		name    string
		up      string
		down    string
	}
	byVersion := make(map[int64]*pending)
	var order []int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := filenameRegex.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}

		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("migrate: parse version %q: %w", match[1], err)
		}
		name, direction := match[2], match[3]

		p := byVersion[version]
		if p == nil {
			p = &pending{version: version, name: name}
			byVersion[version] = p
			order = append(order, version)
		}
		if p.name != name {
			return nil, fmt.Errorf("migrate: version %s has conflicting names %q and %q", FormatVersion(version), p.name, name)
		}

		filePath := entry.Name()
		if dir != "" && dir != "." {
			filePath = path.Join(dir, entry.Name())
		}
		if direction == "up" {
			p.up = filePath
		} else {
			p.down = filePath
		}
	}

	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	migrations := make([]Migration, 0, len(order))
	for _, version := range order {
		p := byVersion[version]
		if p.up == "" {
			return nil, fmt.Errorf("migrate: version %s is missing an .up.sql file", FormatVersion(version))
		}
		migrations = append(migrations, Migration{
			Version:  p.version,
			Name:     p.name,
			UpPath:   p.up,
			DownPath: p.down,
		})
	}

	return migrations, nil
}
