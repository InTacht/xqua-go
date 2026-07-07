package spec

// PathItem represents the OpenAPI Path Item Object.
type PathItem struct {
	Ref                  string                `json:"$ref,omitempty"                 yaml:"$ref,omitempty"`
	Summary              string                `json:"summary,omitempty"              yaml:"summary,omitempty"`
	Description          string                `json:"description,omitempty"          yaml:"description,omitempty"`
	Get                  *Operation            `json:"get,omitempty"                  yaml:"get,omitempty"`
	Put                  *Operation            `json:"put,omitempty"                  yaml:"put,omitempty"`
	Post                 *Operation            `json:"post,omitempty"                 yaml:"post,omitempty"`
	Delete               *Operation            `json:"delete,omitempty"               yaml:"delete,omitempty"`
	Options              *Operation            `json:"options,omitempty"              yaml:"options,omitempty"`
	Head                 *Operation            `json:"head,omitempty"                 yaml:"head,omitempty"`
	Patch                *Operation            `json:"patch,omitempty"                yaml:"patch,omitempty"`
	Trace                *Operation            `json:"trace,omitempty"                yaml:"trace,omitempty"`
	Query                *Operation            `json:"query,omitempty"                yaml:"query,omitempty"`
	AdditionalOperations map[string]*Operation `json:"additionalOperations,omitempty" yaml:"additionalOperations,omitempty"`
	Servers              []Server              `json:"servers,omitempty"              yaml:"servers,omitempty"`
	Parameters           []*Parameter          `json:"parameters,omitempty"           yaml:"parameters,omitempty"`
	Extensions           map[string]any        `json:"-"                              yaml:"-"`
	Extra                map[string]any        `json:"-"                              yaml:"-"`
}

// Parameter represents the OpenAPI Parameter Object.
type Parameter struct {
	Ref             string                `json:"$ref,omitempty"            yaml:"$ref,omitempty"`
	Summary         string                `json:"summary,omitempty"         yaml:"summary,omitempty"`
	Name            string                `json:"name,omitempty"            yaml:"name,omitempty"`
	In              string                `json:"in,omitempty"              yaml:"in,omitempty"`
	Description     string                `json:"description,omitempty"     yaml:"description,omitempty"`
	Required        bool                  `json:"required,omitempty"        yaml:"required,omitempty"`
	Deprecated      bool                  `json:"deprecated,omitempty"      yaml:"deprecated,omitempty"`
	AllowEmptyValue bool                  `json:"allowEmptyValue,omitempty" yaml:"allowEmptyValue,omitempty"`
	Style           string                `json:"style,omitempty"           yaml:"style,omitempty"`
	Explode         *bool                 `json:"explode,omitempty"         yaml:"explode,omitempty"`
	AllowReserved   bool                  `json:"allowReserved,omitempty"   yaml:"allowReserved,omitempty"`
	Schema          *Schema               `json:"schema,omitempty"          yaml:"schema,omitempty"`
	Content         map[string]*MediaType `json:"content,omitempty"         yaml:"content,omitempty"`
	Example         any                   `json:"example,omitempty"         yaml:"example,omitempty"`
	Examples        map[string]*Example   `json:"examples,omitempty"        yaml:"examples,omitempty"`
	Extensions      map[string]any        `json:"-"                         yaml:"-"`
	Extra           map[string]any        `json:"-"                         yaml:"-"`
}

// ParameterIn is the location of a parameter.
type ParameterIn string

const (
	// ParameterInPath indicates a path parameter.
	ParameterInPath ParameterIn = "path"
	// ParameterInQuery indicates a query parameter.
	ParameterInQuery ParameterIn = "query"
	// ParameterInQueryString indicates an OpenAPI 3.2 querystring parameter.
	ParameterInQueryString ParameterIn = "querystring"
	// ParameterInHeader indicates a header parameter.
	ParameterInHeader ParameterIn = "header"
	// ParameterInCookie indicates a cookie parameter.
	ParameterInCookie ParameterIn = "cookie"

	// ParameterInBody is used with ParameterTagMapping to override the struct tag used for JSON
	// request body field names. Defaults to "json".
	ParameterInBody ParameterIn = "body"
	// ParameterInForm is used with ParameterTagMapping to override the struct tag used for form
	// (application/x-www-form-urlencoded and multipart/form-data) request body field names. Defaults to "form".
	ParameterInForm ParameterIn = "form"
)
