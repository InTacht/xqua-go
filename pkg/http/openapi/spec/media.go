package spec

// ContentUnit is an internal content descriptor used by option builders.
type ContentUnit struct {
	Structure   any
	HTTPStatus  int
	ContentType string
	IsDefault   bool
	Summary     string
	Description string
	Encoding    map[string]string
	Example     any
	Examples    map[string]*Example
	Required    bool
	Format      string
}

// MediaType represents the OpenAPI Media Type Object.
type MediaType struct {
	Ref            string               `json:"$ref,omitempty"           yaml:"$ref,omitempty"`
	Summary        string               `json:"summary,omitempty"        yaml:"summary,omitempty"`
	Description    string               `json:"description,omitempty"    yaml:"description,omitempty"`
	Schema         *Schema              `json:"schema,omitempty"         yaml:"schema,omitempty"`
	ItemSchema     *Schema              `json:"itemSchema,omitempty"     yaml:"itemSchema,omitempty"`
	Example        any                  `json:"example,omitempty"        yaml:"example,omitempty"`
	Examples       map[string]*Example  `json:"examples,omitempty"       yaml:"examples,omitempty"`
	Encoding       map[string]*Encoding `json:"encoding,omitempty"       yaml:"encoding,omitempty"`
	PrefixEncoding []*Encoding          `json:"prefixEncoding,omitempty" yaml:"prefixEncoding,omitempty"`
	ItemEncoding   *Encoding            `json:"itemEncoding,omitempty"   yaml:"itemEncoding,omitempty"`
	Extensions     map[string]any       `json:"-"                        yaml:"-"`
	Extra          map[string]any       `json:"-"                        yaml:"-"`
}

// Encoding represents the OpenAPI Encoding Object.
type Encoding struct {
	ContentType    string               `json:"contentType,omitempty"    yaml:"contentType,omitempty"`
	Headers        map[string]*Header   `json:"headers,omitempty"        yaml:"headers,omitempty"`
	Style          string               `json:"style,omitempty"          yaml:"style,omitempty"`
	Explode        *bool                `json:"explode,omitempty"        yaml:"explode,omitempty"`
	AllowReserved  bool                 `json:"allowReserved,omitempty"  yaml:"allowReserved,omitempty"`
	Encoding       map[string]*Encoding `json:"encoding,omitempty"       yaml:"encoding,omitempty"`
	PrefixEncoding []*Encoding          `json:"prefixEncoding,omitempty" yaml:"prefixEncoding,omitempty"`
	ItemEncoding   *Encoding            `json:"itemEncoding,omitempty"   yaml:"itemEncoding,omitempty"`
	Extensions     map[string]any       `json:"-"                        yaml:"-"`
	Extra          map[string]any       `json:"-"                        yaml:"-"`
}

// Example represents the OpenAPI Example Object.
type Example struct {
	Ref             string `json:"$ref,omitempty"            yaml:"$ref,omitempty"`
	Summary         string `json:"summary,omitempty"         yaml:"summary,omitempty"`
	Description     string `json:"description,omitempty"     yaml:"description,omitempty"`
	DataValue       any    `json:"dataValue,omitempty"       yaml:"dataValue,omitempty"`
	Value           any    `json:"value,omitempty"           yaml:"value,omitempty"`
	ExternalValue   string `json:"externalValue,omitempty"   yaml:"externalValue,omitempty"`
	SerializedValue string `json:"serializedValue,omitempty" yaml:"serializedValue,omitempty"`
	// Deprecated: OpenAPI 3.2 uses serializedValue. This field is accepted for
	// source compatibility but is not serialized.
	SerializedExample any            `json:"-" yaml:"-"`
	Extensions        map[string]any `json:"-" yaml:"-"`
	Extra             map[string]any `json:"-" yaml:"-"`
}
