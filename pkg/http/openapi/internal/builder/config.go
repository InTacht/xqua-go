package builder

import "github.com/InTacht/xqua-go/pkg/http/openapi/spec"

// OperationConfig is the resolved, declarative description of a single OpenAPI
// operation. It replaces the former functional-option configuration: callers
// populate the struct directly (the engine does this from openapi.Route).
type OperationConfig struct {
	Hide         bool
	OperationID  string
	Description  string
	Summary      string
	ExternalDocs *spec.ExternalDocs
	Deprecated   bool
	Tags         []string
	Security     []OperationSecurityConfig
	Requests     []*spec.ContentUnit
	Responses    []*spec.ContentUnit
	Customizers  []func(*spec.Operation)
}

// OperationSecurityConfig describes one operation security requirement entry.
type OperationSecurityConfig struct {
	Name   string
	Scopes []string
}
