package errors

import (
	"fmt"
	"sort"
	"strings"
)

// Def describes one catalog entry. Code is required and must be unique within
// the catalog. Kind is an optional semantic category (for example "validation"
// or "not_found") that may be shared freely across catalogs; it defaults to
// the catalog name when omitted.
type Def struct {
	Kind    string
	Code    string
	Message string
	Source  string
}

// Catalog is a named group of canonical error definitions. Error identity is
// the defined entry itself, so catalogs are isolated by construction: two
// catalogs may use the same name, kinds, or codes without their entries ever
// matching each other via Is.
//
// Define all entries during package initialization; catalogs are not
// synchronized and must be treated as immutable once handlers run.
type Catalog struct {
	name    string
	entries map[string]*Error
}

// NewCatalog creates a catalog. The name labels the catalog (typically the
// module or service that owns it) and is the default Kind for entries that
// omit one. Names do not need to be unique across the process.
func NewCatalog(name string) *Catalog {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("errors: catalog name is required")
	}
	return &Catalog{
		name:    name,
		entries: make(map[string]*Error),
	}
}

// Name returns the catalog label.
func (c *Catalog) Name() string {
	return c.name
}

// Define registers an entry and returns a template *Error for handlers. Panics
// when Code is empty or already defined in this catalog.
func (c *Catalog) Define(def Def) *Error {
	code := strings.TrimSpace(def.Code)
	if code == "" {
		panic(fmt.Sprintf("errors: catalog %q: Code is required", c.name))
	}
	if c.entries[code] != nil {
		panic(fmt.Sprintf("errors: catalog %q: duplicate code %q", c.name, code))
	}

	kind := strings.TrimSpace(def.Kind)
	if kind == "" {
		kind = c.name
	}

	entry := newEntry(kind, code, def.Message, def.Source)
	c.entries[code] = entry
	return entry
}

// Lookup returns the entry defined with code. It is the decode table for
// re-binding wire errors (kind/code data) to catalog identity at client
// boundaries.
func (c *Catalog) Lookup(code string) (*Error, bool) {
	entry, ok := c.entries[code]
	return entry, ok
}

// Entries returns every defined entry sorted by code, for building manifests,
// OpenAPI schemas, or client code generation.
func (c *Catalog) Entries() []*Error {
	out := make([]*Error, 0, len(c.entries))
	for _, entry := range c.entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// Contains reports whether err (or the entry it was cloned from) was defined
// in this catalog.
func (c *Catalog) Contains(err *Error) bool {
	if err == nil || err.entry == nil {
		return false
	}
	return c.entries[err.entry.Code] == err.entry
}

func newEntry(kind, code, message, source string) *Error {
	e := &Error{
		Kind:    kind,
		Code:    code,
		Message: message,
		Source:  source,
	}
	e.entry = e
	return e
}

func validateFallback(fallback *Error) {
	if fallback == nil {
		panic("errors: fallback catalog entry is required")
	}
}
