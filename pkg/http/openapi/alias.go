package openapi

import (
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/builder"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

// The following aliases expose the typed OpenAPI model (pkg/http/openapi) under
// the http package so callers declare documents, routes, and schemas without a
// second import. They are the wire objects the generator emits.

// Document is an alias of spec.Document.
type Document = spec.Document

// Info is an alias of spec.Info.
type Info = spec.Info

// Contact is an alias of spec.Contact.
type Contact = spec.Contact

// License is an alias of spec.License.
type License = spec.License

// Tag is an alias of spec.Tag.
type Tag = spec.Tag

// ExternalDocs is an alias of spec.ExternalDocs.
type ExternalDocs = spec.ExternalDocs

// Server is an alias of spec.Server.
type Server = spec.Server

// ServerVariable is an alias of spec.ServerVariable.
type ServerVariable = spec.ServerVariable

// Components is an alias of spec.Components.
type Components = spec.Components

// PathItem is an alias of spec.PathItem.
type PathItem = spec.PathItem

// Operation is an alias of spec.Operation.
type Operation = spec.Operation

// SpecResponse is an alias of spec.Response (OpenAPI operation response object).
type SpecResponse = spec.Response

// RequestBody is an alias of spec.RequestBody.
type RequestBody = spec.RequestBody

// Parameter is an alias of spec.Parameter.
type Parameter = spec.Parameter

// MediaType is an alias of spec.MediaType.
type MediaType = spec.MediaType

// Encoding is an alias of spec.Encoding.
type Encoding = spec.Encoding

// Example is an alias of spec.Example.
type Example = spec.Example

// Schema is an alias of spec.Schema (the typed JSON Schema object).
type Schema = spec.Schema

// ContentUnit is an alias of spec.ContentUnit: an explicit request/response
// content descriptor used when reflection of a Go type is not wanted.
type ContentUnit = spec.ContentUnit

// SecurityScheme is an alias of spec.SecurityScheme.
type SecurityScheme = spec.SecurityScheme

// SecurityRequirement is an alias of spec.SecurityRequirement: a map of
// scheme name to required scopes. Each entry in a list is an alternative.
type SecurityRequirement = spec.SecurityRequirement

// OAuthFlows is an alias of spec.OAuthFlows.
type OAuthFlows = spec.OAuthFlows

// OAuthFlow is an alias of spec.OAuthFlow.
type OAuthFlow = spec.OAuthFlow

// ReflectorConfig is an alias of spec.ReflectorConfig: reflection tuning for
// a document (inline refs, definition name prefixes, type mappings, intercepts).
type ReflectorConfig = spec.ReflectorConfig

// OneOf returns a value that represents multiple possible schemas. Pass it as a
// response Body to emit a oneOf union of the reflected member types.
func OneOf(values ...any) any {
	return builder.OneOf(values...)
}

// SchemaExposer lets custom Go types provide their own OpenAPI Schema Object.
// The selected OpenAPI version is passed so implementations can return
// version-specific schema keywords.
type SchemaExposer interface {
	OpenAPISchema(version string) *spec.Schema
}

// StaticSchemaExposer lets custom Go types provide an OpenAPI Schema Object
// when they do not need version-specific output.
type StaticSchemaExposer interface {
	OpenAPISchema() *spec.Schema
}

// MarshalJSON serializes an OpenAPI document (or fragment) as JSON, merging
// x-* extensions into their objects. Output is deterministic.
func MarshalJSON(value any) ([]byte, error) {
	return spec.MarshalJSON(value)
}

// MarshalYAML serializes an OpenAPI document (or fragment) as YAML, merging
// x-* extensions into their objects.
func MarshalYAML(value any) ([]byte, error) {
	return spec.MarshalYAML(value)
}
