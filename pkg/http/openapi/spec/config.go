package spec

import (
	"errors"
	"log/slog"
	"reflect"
)

// ErrSkipProperty can be returned from InterceptPropFunc to skip adding the property to the schema.
var ErrSkipProperty = errors.New("skip property")

// EmbedReferencer can be implemented by an embedded struct type to opt into $ref-based embedding.
// When a struct embeds a type that implements EmbedReferencer (or is tagged `refer:"true"`),
// the embedded type is registered as a component schema and referenced via allOf instead of
// having its fields inlined into the parent schema.
type EmbedReferencer interface {
	ReferEmbedded()
}

// InterceptPropParams defines parameters passed to InterceptPropFunc.
// Called twice per field: before schema generation (Processed=false) and after (Processed=true).
type InterceptPropParams struct {
	Name           string
	Field          reflect.StructField
	PropertySchema *Schema // nil when Processed=false
	ParentSchema   *Schema
	Processed      bool
	ParentType     reflect.Type // the struct type being reflected; same for all fields including embedded
}

// InterceptPropFunc intercepts field reflection to control or modify property schemas.
// Return ErrSkipProperty to skip adding the property.
type InterceptPropFunc func(params InterceptPropParams) error

// InterceptSchemaParams defines parameters passed to InterceptSchemaFunc.
// Called twice per type: before schema generation (Processed=false, empty schema) and after (Processed=true).
type InterceptSchemaParams struct {
	Type      reflect.Type
	Schema    *Schema
	Processed bool
}

// InterceptSchemaFunc intercepts type schema generation.
// On the pre-call (Processed=false), return stop=true to skip default processing and use Schema as-is.
type InterceptSchemaFunc func(params InterceptSchemaParams) (stop bool, err error)

const (
	// Version300 is OpenAPI 3.0.0.
	Version300 = "3.0.0"
	// Version301 is OpenAPI 3.0.1.
	Version301 = "3.0.1"
	// Version302 is OpenAPI 3.0.2.
	Version302 = "3.0.2"
	// Version303 is OpenAPI 3.0.3.
	Version303 = "3.0.3"
	// Version304 is OpenAPI 3.0.4.
	Version304 = "3.0.4"
	// Version310 is OpenAPI 3.1.0.
	Version310 = "3.1.0"
	// Version311 is OpenAPI 3.1.1.
	Version311 = "3.1.1"
	// Version312 is OpenAPI 3.1.2.
	Version312 = "3.1.2"
	// Version320 is OpenAPI 3.2.0.
	Version320 = "3.2.0"
)

// Config is the main configuration struct for generating an OpenAPI document.
// It contains all the necessary information and options to customize the generated document, including metadata,
// server information and security schemes.
type Config struct {
	Logger            *slog.Logger
	OpenAPIVersion    string
	Self              string
	Title             string
	InfoSummary       string
	Version           string
	Description       *string
	JSONSchemaDialect string
	Contact           *Contact
	License           *License
	TermsOfService    *string
	Servers           []Server
	SecuritySchemes   map[string]*SecurityScheme
	Security          []SecurityRequirement
	Tags              []Tag
	ExternalDocs      *ExternalDocs

	ReflectorConfig     *ReflectorConfig
	StripTrailingSlash  bool
	PathParser          PathParser
	DocumentCustomizers []func(*Document)

	SpecPath string
	CacheAge *int
}

// ReflectorConfig contains configuration options for the reflection process used to generate
// the OpenAPI document from Go types. It allows customization of how types are reflected, including inline references,
// stripping of definition name prefixes, and custom type mappings.
type ReflectorConfig struct {
	InlineRefs          bool
	StripDefNamePrefix  []string
	InterceptDefName    func(t reflect.Type, defaultDefName string) string
	InterceptProp       InterceptPropFunc
	InterceptSchema     InterceptSchemaFunc
	TypeMappings        []TypeMapping
	ParameterTagMapping map[ParameterIn]string
}

// TypeMapping represents a mapping between a source type and a destination type. It is used in the reflection process
// to specify how certain types should be mapped when generating the OpenAPI document. The Src field represents
// the original type, while the Dst field represents the type that should be used in the generated document.
type TypeMapping struct {
	Src any
	Dst any
}

// PathParser is an interface that defines a method for parsing a path string and returning a modified version of it.
// This can be used to customize how paths are represented in the generated OpenAPI document, allowing
// for transformations such as converting path parameters to a specific format or applying
// any other necessary modifications to the path strings.
type PathParser interface {
	Parse(path string) (string, error)
}
