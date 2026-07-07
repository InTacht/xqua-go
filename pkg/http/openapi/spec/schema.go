package spec

// Schema represents the OpenAPI Schema Object.
type Schema struct {
	Ref                   string              `json:"$ref,omitempty"                  yaml:"$ref,omitempty"`
	Schema                string              `json:"$schema,omitempty"               yaml:"$schema,omitempty"`
	ID                    string              `json:"$id,omitempty"                   yaml:"$id,omitempty"`
	Defs                  map[string]*Schema  `json:"$defs,omitempty"                 yaml:"$defs,omitempty"`
	Anchor                string              `json:"$anchor,omitempty"               yaml:"$anchor,omitempty"`
	DynamicAnchor         string              `json:"$dynamicAnchor,omitempty"        yaml:"$dynamicAnchor,omitempty"`
	DynamicRef            string              `json:"$dynamicRef,omitempty"           yaml:"$dynamicRef,omitempty"`
	Vocabulary            map[string]bool     `json:"$vocabulary,omitempty"           yaml:"$vocabulary,omitempty"`
	Comment               string              `json:"$comment,omitempty"              yaml:"$comment,omitempty"`
	Title                 string              `json:"title,omitempty"                 yaml:"title,omitempty"`
	Description           string              `json:"description,omitempty"           yaml:"description,omitempty"`
	Type                  any                 `json:"type,omitempty"                  yaml:"type,omitempty"`
	Format                string              `json:"format,omitempty"                yaml:"format,omitempty"`
	Nullable              bool                `json:"nullable,omitempty"              yaml:"nullable,omitempty"`
	Default               any                 `json:"default,omitempty"               yaml:"default,omitempty"`
	Example               any                 `json:"example,omitempty"               yaml:"example,omitempty"`
	Examples              []any               `json:"examples,omitempty"              yaml:"examples,omitempty"`
	Enum                  []any               `json:"enum,omitempty"                  yaml:"enum,omitempty"`
	Const                 any                 `json:"const,omitempty"                 yaml:"const,omitempty"`
	MultipleOf            *float64            `json:"multipleOf,omitempty"            yaml:"multipleOf,omitempty"`
	Maximum               *float64            `json:"maximum,omitempty"               yaml:"maximum,omitempty"`
	ExclusiveMaximum      any                 `json:"exclusiveMaximum,omitempty"      yaml:"exclusiveMaximum,omitempty"`
	Minimum               *float64            `json:"minimum,omitempty"               yaml:"minimum,omitempty"`
	ExclusiveMinimum      any                 `json:"exclusiveMinimum,omitempty"      yaml:"exclusiveMinimum,omitempty"`
	MaxLength             *int                `json:"maxLength,omitempty"             yaml:"maxLength,omitempty"`
	MinLength             *int                `json:"minLength,omitempty"             yaml:"minLength,omitempty"`
	Pattern               string              `json:"pattern,omitempty"               yaml:"pattern,omitempty"`
	MaxItems              *int                `json:"maxItems,omitempty"              yaml:"maxItems,omitempty"`
	MinItems              *int                `json:"minItems,omitempty"              yaml:"minItems,omitempty"`
	UniqueItems           *bool               `json:"uniqueItems,omitempty"           yaml:"uniqueItems,omitempty"`
	MaxProperties         *int                `json:"maxProperties,omitempty"         yaml:"maxProperties,omitempty"`
	MinProperties         *int                `json:"minProperties,omitempty"         yaml:"minProperties,omitempty"`
	Required              []string            `json:"required,omitempty"              yaml:"required,omitempty"`
	Properties            map[string]*Schema  `json:"properties,omitempty"            yaml:"properties,omitempty"`
	PatternProperties     map[string]*Schema  `json:"patternProperties,omitempty"     yaml:"patternProperties,omitempty"`
	Items                 *Schema             `json:"items,omitempty"                 yaml:"items,omitempty"`
	PrefixItems           []*Schema           `json:"prefixItems,omitempty"           yaml:"prefixItems,omitempty"`
	Contains              *Schema             `json:"contains,omitempty"              yaml:"contains,omitempty"`
	MaxContains           *int                `json:"maxContains,omitempty"           yaml:"maxContains,omitempty"`
	MinContains           *int                `json:"minContains,omitempty"           yaml:"minContains,omitempty"`
	AdditionalProperties  any                 `json:"additionalProperties,omitempty"  yaml:"additionalProperties,omitempty"`
	UnevaluatedProperties any                 `json:"unevaluatedProperties,omitempty" yaml:"unevaluatedProperties,omitempty"`
	PropertyNames         *Schema             `json:"propertyNames,omitempty"         yaml:"propertyNames,omitempty"`
	DependentRequired     map[string][]string `json:"dependentRequired,omitempty"     yaml:"dependentRequired,omitempty"`
	DependentSchemas      map[string]*Schema  `json:"dependentSchemas,omitempty"      yaml:"dependentSchemas,omitempty"`
	AllOf                 []*Schema           `json:"allOf,omitempty"                 yaml:"allOf,omitempty"`
	AnyOf                 []*Schema           `json:"anyOf,omitempty"                 yaml:"anyOf,omitempty"`
	OneOf                 []*Schema           `json:"oneOf,omitempty"                 yaml:"oneOf,omitempty"`
	Not                   *Schema             `json:"not,omitempty"                   yaml:"not,omitempty"`
	If                    *Schema             `json:"if,omitempty"                    yaml:"if,omitempty"`
	Then                  *Schema             `json:"then,omitempty"                  yaml:"then,omitempty"`
	Else                  *Schema             `json:"else,omitempty"                  yaml:"else,omitempty"`
	Deprecated            bool                `json:"deprecated,omitempty"            yaml:"deprecated,omitempty"`
	ReadOnly              bool                `json:"readOnly,omitempty"              yaml:"readOnly,omitempty"`
	WriteOnly             bool                `json:"writeOnly,omitempty"             yaml:"writeOnly,omitempty"`
	ContentEncoding       string              `json:"contentEncoding,omitempty"       yaml:"contentEncoding,omitempty"`
	ContentMediaType      string              `json:"contentMediaType,omitempty"      yaml:"contentMediaType,omitempty"`
	ContentSchema         *Schema             `json:"contentSchema,omitempty"         yaml:"contentSchema,omitempty"`
	Discriminator         *Discriminator      `json:"discriminator,omitempty"         yaml:"discriminator,omitempty"`
	XML                   *XML                `json:"xml,omitempty"                   yaml:"xml,omitempty"`
	ExternalDocs          *ExternalDocs       `json:"externalDocs,omitempty"          yaml:"externalDocs,omitempty"`
	Extensions            map[string]any      `json:"-"                               yaml:"-"`
	Extra                 map[string]any      `json:"-"                               yaml:"-"`
}

// Discriminator represents the OpenAPI Discriminator Object.
type Discriminator struct {
	PropertyName   string            `json:"propertyName"             yaml:"propertyName"`
	Mapping        map[string]string `json:"mapping,omitempty"        yaml:"mapping,omitempty"`
	DefaultMapping string            `json:"defaultMapping,omitempty" yaml:"defaultMapping,omitempty"`
	Extensions     map[string]any    `json:"-"                        yaml:"-"`
	Extra          map[string]any    `json:"-"                        yaml:"-"`
}

// XML represents the OpenAPI XML Object.
type XML struct {
	NodeType   string         `json:"nodeType,omitempty"  yaml:"nodeType,omitempty"`
	Name       string         `json:"name,omitempty"      yaml:"name,omitempty"`
	Namespace  string         `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Prefix     string         `json:"prefix,omitempty"    yaml:"prefix,omitempty"`
	Attribute  bool           `json:"attribute,omitempty" yaml:"attribute,omitempty"`
	Wrapped    bool           `json:"wrapped,omitempty"   yaml:"wrapped,omitempty"`
	Extensions map[string]any `json:"-"                   yaml:"-"`
	Extra      map[string]any `json:"-"                   yaml:"-"`
}
