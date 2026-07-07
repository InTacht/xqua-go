package spec

// Operation represents the OpenAPI Operation Object.
type Operation struct {
	Tags         []string              `json:"tags,omitempty"         yaml:"tags,omitempty"`
	Summary      string                `json:"summary,omitempty"      yaml:"summary,omitempty"`
	Description  string                `json:"description,omitempty"  yaml:"description,omitempty"`
	ExternalDocs *ExternalDocs         `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	OperationID  string                `json:"operationId,omitempty"  yaml:"operationId,omitempty"`
	Parameters   []*Parameter          `json:"parameters,omitempty"   yaml:"parameters,omitempty"`
	RequestBody  *RequestBody          `json:"requestBody,omitempty"  yaml:"requestBody,omitempty"`
	Responses    map[string]*Response  `json:"responses"              yaml:"responses"`
	Callbacks    map[string]*Callback  `json:"callbacks,omitempty"    yaml:"callbacks,omitempty"`
	Deprecated   bool                  `json:"deprecated,omitempty"   yaml:"deprecated,omitempty"`
	Security     []SecurityRequirement `json:"security,omitempty"     yaml:"security,omitempty"`
	Servers      []Server              `json:"servers,omitempty"      yaml:"servers,omitempty"`
	Extensions   map[string]any        `json:"-"                      yaml:"-"`
	Extra        map[string]any        `json:"-"                      yaml:"-"`
}

// RequestBody represents the OpenAPI Request Body Object.
type RequestBody struct {
	Ref         string               `json:"$ref,omitempty"        yaml:"$ref,omitempty"`
	Summary     string               `json:"summary,omitempty"     yaml:"summary,omitempty"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"     yaml:"content,omitempty"`
	Required    bool                 `json:"required,omitempty"    yaml:"required,omitempty"`
	Extensions  map[string]any       `json:"-"                     yaml:"-"`
	Extra       map[string]any       `json:"-"                     yaml:"-"`
}

// Response represents the OpenAPI Response Object.
type Response struct {
	Ref         string               `json:"$ref,omitempty"        yaml:"$ref,omitempty"`
	Summary     string               `json:"summary,omitempty"     yaml:"summary,omitempty"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Headers     map[string]*Header   `json:"headers,omitempty"     yaml:"headers,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"     yaml:"content,omitempty"`
	Links       map[string]*Link     `json:"links,omitempty"       yaml:"links,omitempty"`
	Extensions  map[string]any       `json:"-"                     yaml:"-"`
	Extra       map[string]any       `json:"-"                     yaml:"-"`
}

// Header represents the OpenAPI Header Object.
type Header struct {
	Ref             string                `json:"$ref,omitempty"            yaml:"$ref,omitempty"`
	Summary         string                `json:"summary,omitempty"         yaml:"summary,omitempty"`
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

// Link represents the OpenAPI Link Object.
type Link struct {
	Ref          string         `json:"$ref,omitempty"         yaml:"$ref,omitempty"`
	Summary      string         `json:"summary,omitempty"      yaml:"summary,omitempty"`
	OperationRef string         `json:"operationRef,omitempty" yaml:"operationRef,omitempty"`
	OperationID  string         `json:"operationId,omitempty"  yaml:"operationId,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"   yaml:"parameters,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty"  yaml:"requestBody,omitempty"`
	Description  string         `json:"description,omitempty"  yaml:"description,omitempty"`
	Server       *Server        `json:"server,omitempty"       yaml:"server,omitempty"`
	Extensions   map[string]any `json:"-"                      yaml:"-"`
	Extra        map[string]any `json:"-"                      yaml:"-"`
}

// Callback represents the OpenAPI Callback Object.
type Callback struct {
	Ref         string               `json:"$ref,omitempty"        yaml:"$ref,omitempty"`
	Summary     string               `json:"summary,omitempty"     yaml:"summary,omitempty"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Expressions map[string]*PathItem `json:"-"                     yaml:"-"`
	Extensions  map[string]any       `json:"-"                     yaml:"-"`
	Extra       map[string]any       `json:"-"                     yaml:"-"`
}

// SecurityRequirement represents one OpenAPI Security Requirement Object.
type SecurityRequirement map[string][]string

// SecuritySchemeAPIKeyIn is the location of an API key security scheme.
type SecuritySchemeAPIKeyIn string

const (
	// SecuritySchemeAPIKeyInQuery indicates an API key in query.
	SecuritySchemeAPIKeyInQuery SecuritySchemeAPIKeyIn = "query"
	// SecuritySchemeAPIKeyInHeader indicates an API key in header.
	SecuritySchemeAPIKeyInHeader SecuritySchemeAPIKeyIn = "header"
	// SecuritySchemeAPIKeyInCookie indicates an API key in cookie.
	SecuritySchemeAPIKeyInCookie SecuritySchemeAPIKeyIn = "cookie"
)

// SecurityScheme represents the OpenAPI Security Scheme Object.
type SecurityScheme struct {
	Ref               string                 `json:"$ref,omitempty"              yaml:"$ref,omitempty"`
	Summary           string                 `json:"summary,omitempty"           yaml:"summary,omitempty"`
	Type              string                 `json:"type,omitempty"              yaml:"type,omitempty"`
	Description       *string                `json:"description,omitempty"       yaml:"description,omitempty"`
	Name              string                 `json:"name,omitempty"              yaml:"name,omitempty"`
	In                SecuritySchemeAPIKeyIn `json:"in,omitempty"                yaml:"in,omitempty"`
	Scheme            string                 `json:"scheme,omitempty"            yaml:"scheme,omitempty"`
	BearerFormat      *string                `json:"bearerFormat,omitempty"      yaml:"bearerFormat,omitempty"`
	Flows             *OAuthFlows            `json:"flows,omitempty"             yaml:"flows,omitempty"`
	OpenIDConnectURL  string                 `json:"openIdConnectUrl,omitempty"  yaml:"openIdConnectUrl,omitempty"`
	OAuth2MetadataURL string                 `json:"oauth2MetadataUrl,omitempty" yaml:"oauth2MetadataUrl,omitempty"`
	Deprecated        bool                   `json:"deprecated,omitempty"        yaml:"deprecated,omitempty"`
	Extensions        map[string]any         `json:"-"                           yaml:"-"`
	Extra             map[string]any         `json:"-"                           yaml:"-"`
}

// OAuthFlows represents the OpenAPI OAuth Flows Object.
type OAuthFlows struct {
	Implicit            *OAuthFlow     `json:"implicit,omitempty"            yaml:"implicit,omitempty"`
	Password            *OAuthFlow     `json:"password,omitempty"            yaml:"password,omitempty"`
	ClientCredentials   *OAuthFlow     `json:"clientCredentials,omitempty"   yaml:"clientCredentials,omitempty"`
	AuthorizationCode   *OAuthFlow     `json:"authorizationCode,omitempty"   yaml:"authorizationCode,omitempty"`
	DeviceAuthorization *OAuthFlow     `json:"deviceAuthorization,omitempty" yaml:"deviceAuthorization,omitempty"`
	Extensions          map[string]any `json:"-"                             yaml:"-"`
	Extra               map[string]any `json:"-"                             yaml:"-"`
}

// OAuthFlow represents one OpenAPI OAuth Flow Object.
type OAuthFlow struct {
	AuthorizationURL       string            `json:"authorizationUrl,omitempty"       yaml:"authorizationUrl,omitempty"`
	DeviceAuthorizationURL string            `json:"deviceAuthorizationUrl,omitempty" yaml:"deviceAuthorizationUrl,omitempty"`
	TokenURL               string            `json:"tokenUrl,omitempty"               yaml:"tokenUrl,omitempty"`
	RefreshURL             *string           `json:"refreshUrl,omitempty"             yaml:"refreshUrl,omitempty"`
	Scopes                 map[string]string `json:"scopes"                           yaml:"scopes"`
	Extensions             map[string]any    `json:"-"                                yaml:"-"`
	Extra                  map[string]any    `json:"-"                                yaml:"-"`
}
