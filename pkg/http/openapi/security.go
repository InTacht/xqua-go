package openapi

import (
	"context"
	"fmt"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
	"github.com/gofiber/fiber/v3"
)

// Credential carries the extracted secret and operation metadata passed to Verify.
type Credential struct {
	Scheme string
	Raw    string
	Scopes []string
	Meta   map[string]string
}

// VerifyFunc validates a credential and returns the caller-defined identity.
type VerifyFunc func(ctx context.Context, cred Credential) (Identity, error)

// ExtractFunc reads a raw credential from an HTTP request for one scheme.
type ExtractFunc func(c fiber.Ctx) (raw string, ok bool)

// Scheme binds OpenAPI security metadata to runtime extraction and verification.
type Scheme struct {
	Spec    SecurityScheme
	Verify  VerifyFunc
	Extract ExtractFunc
}

// SecuritySpec declares route, group, or engine security requirements.
type SecuritySpec struct {
	explicit     bool
	public       bool
	requirements []SecurityRequirement
}

// InheritSecurity leaves security to parent groups or the engine default.
func InheritSecurity() SecuritySpec { return SecuritySpec{} }

// PublicSecurity marks a route or group as explicitly unsecured.
func PublicSecurity() SecuritySpec {
	return SecuritySpec{explicit: true, public: true}
}

// RequireSecurity requires one scheme, optionally with OAuth/OIDC scopes.
func RequireSecurity(scheme string, scopes ...string) SecuritySpec {
	return SecuritySpec{
		explicit:     true,
		requirements: []SecurityRequirement{{scheme: scopes}},
	}
}

// RequireAnySecurity requires any one of the named schemes (OpenAPI OR).
func RequireAnySecurity(schemes ...string) SecuritySpec {
	reqs := make([]SecurityRequirement, len(schemes))
	for i, name := range schemes {
		reqs[i] = SecurityRequirement{name: {}}
	}
	return SecuritySpec{explicit: true, requirements: reqs}
}

func (s SecuritySpec) isExplicit() bool { return s.explicit }

func (s SecuritySpec) isPublic() bool { return s.explicit && s.public }

func (s SecuritySpec) requirementsOrNil() []SecurityRequirement {
	if !s.explicit || s.public {
		return nil
	}
	return cloneRequirements(s.requirements)
}

// ResolveSecurity picks the effective requirement list walking route → groups → default.
// public is true when the resolved policy is explicitly unsecured.
func ResolveSecurity(route SecuritySpec, groups []SecuritySpec, defaultSpec SecuritySpec) (requirements []SecurityRequirement, public bool) {
	if route.isExplicit() {
		if route.isPublic() {
			return nil, true
		}
		return cloneRequirements(route.requirements), false
	}
	for i := len(groups) - 1; i >= 0; i-- {
		if groups[i].isExplicit() {
			if groups[i].isPublic() {
				return nil, true
			}
			return cloneRequirements(groups[i].requirements), false
		}
	}
	if defaultSpec.isExplicit() {
		if defaultSpec.isPublic() {
			return nil, true
		}
		return cloneRequirements(defaultSpec.requirements), false
	}
	return nil, false
}

func cloneRequirements(in []SecurityRequirement) []SecurityRequirement {
	if len(in) == 0 {
		return nil
	}
	out := make([]SecurityRequirement, len(in))
	copy(out, in)
	return out
}

// BearerOptions configures an http bearer security scheme.
type BearerOptions struct {
	Format  string
	Verify  VerifyFunc
	Extract ExtractFunc
	Summary string
	Desc    string
}

// BearerScheme registers an http bearer scheme.
func BearerScheme(opts BearerOptions) Scheme {
	spec := SecurityScheme{
		Type:   "http",
		Scheme: "bearer",
	}
	if opts.Format != "" {
		spec.BearerFormat = &opts.Format
	}
	if opts.Summary != "" {
		spec.Summary = opts.Summary
	}
	if opts.Desc != "" {
		spec.Description = &opts.Desc
	}
	extract := opts.Extract
	if extract == nil {
		extract = extractBearer
	}
	return Scheme{Spec: spec, Verify: opts.Verify, Extract: extract}
}

// APIKeyIn is the location of an API key.
type APIKeyIn = spec.SecuritySchemeAPIKeyIn

const (
	InHeader = spec.SecuritySchemeAPIKeyInHeader
	InQuery  = spec.SecuritySchemeAPIKeyInQuery
	InCookie = spec.SecuritySchemeAPIKeyInCookie
)

// APIKeyOptions configures an apiKey security scheme.
type APIKeyOptions struct {
	Name    string
	In      APIKeyIn
	Verify  VerifyFunc
	Extract ExtractFunc
	Summary string
	Desc    string
}

// APIKeyScheme registers an apiKey scheme.
func APIKeyScheme(opts APIKeyOptions) Scheme {
	if opts.In == "" {
		opts.In = InHeader
	}
	spec := SecurityScheme{
		Type: "apiKey",
		Name: opts.Name,
		In:   opts.In,
	}
	if opts.Summary != "" {
		spec.Summary = opts.Summary
	}
	if opts.Desc != "" {
		spec.Description = &opts.Desc
	}
	extract := opts.Extract
	if extract == nil {
		extract = apiKeyExtractor(opts.Name, opts.In)
	}
	return Scheme{Spec: spec, Verify: opts.Verify, Extract: extract}
}

// HTTPOptions configures a generic http security scheme (basic, digest, etc.).
type HTTPOptions struct {
	Scheme  string
	Verify  VerifyFunc
	Extract ExtractFunc
	Summary string
	Desc    string
}

// HTTPScheme registers an http scheme with a custom Authorization scheme name.
func HTTPScheme(opts HTTPOptions) Scheme {
	if opts.Scheme == "" {
		opts.Scheme = "basic"
	}
	spec := SecurityScheme{
		Type:   "http",
		Scheme: opts.Scheme,
	}
	if opts.Summary != "" {
		spec.Summary = opts.Summary
	}
	if opts.Desc != "" {
		spec.Description = &opts.Desc
	}
	extract := opts.Extract
	if extract == nil {
		switch opts.Scheme {
		case "bearer":
			extract = extractBearer
		case "basic":
			extract = extractBasic
		default:
			extract = extractHTTPScheme(opts.Scheme)
		}
	}
	return Scheme{Spec: spec, Verify: opts.Verify, Extract: extract}
}

// OAuth2Options configures an oauth2 security scheme (flows are documentation).
type OAuth2Options struct {
	Flows             *OAuthFlows
	OAuth2MetadataURL string
	Verify            VerifyFunc
	Extract           ExtractFunc
	Summary           string
	Desc              string
}

// OAuth2Scheme registers an oauth2 scheme; runtime validates via Bearer-style extraction.
func OAuth2Scheme(opts OAuth2Options) Scheme {
	spec := SecurityScheme{
		Type:              "oauth2",
		Flows:             opts.Flows,
		OAuth2MetadataURL: opts.OAuth2MetadataURL,
	}
	if opts.Summary != "" {
		spec.Summary = opts.Summary
	}
	if opts.Desc != "" {
		spec.Description = &opts.Desc
	}
	extract := opts.Extract
	if extract == nil {
		extract = extractBearer
	}
	return Scheme{Spec: spec, Verify: opts.Verify, Extract: extract}
}

// OIDCOptions configures an openIdConnect security scheme.
type OIDCOptions struct {
	OpenIDConnectURL string
	Verify           VerifyFunc
	Extract          ExtractFunc
	Summary          string
	Desc             string
}

// OpenIDConnectScheme registers an openIdConnect scheme; runtime validates via Bearer extraction.
func OpenIDConnectScheme(opts OIDCOptions) Scheme {
	spec := SecurityScheme{
		Type:             "openIdConnect",
		OpenIDConnectURL: opts.OpenIDConnectURL,
	}
	if opts.Summary != "" {
		spec.Summary = opts.Summary
	}
	if opts.Desc != "" {
		spec.Description = &opts.Desc
	}
	extract := opts.Extract
	if extract == nil {
		extract = extractBearer
	}
	return Scheme{Spec: spec, Verify: opts.Verify, Extract: extract}
}

// MTLSOptions configures a mutualTLS security scheme.
type MTLSOptions struct {
	Verify  VerifyFunc
	Extract ExtractFunc
	Summary string
	Desc    string
}

// MutualTLSScheme registers a mutualTLS scheme.
func MutualTLSScheme(opts MTLSOptions) Scheme {
	spec := SecurityScheme{Type: "mutualTLS"}
	if opts.Summary != "" {
		spec.Summary = opts.Summary
	}
	if opts.Desc != "" {
		spec.Description = &opts.Desc
	}
	extract := opts.Extract
	if extract == nil {
		extract = extractMutualTLS
	}
	return Scheme{Spec: spec, Verify: opts.Verify, Extract: extract}
}

// ValidateRequirements panics when a requirement references multiple schemes in one object.
func ValidateRequirements(prefix string, reqs []SecurityRequirement) {
	for i, req := range reqs {
		if len(req) != 1 {
			panic(fmt.Sprintf("%s: security requirement[%d] must name exactly one scheme (got %d)", prefix, i, len(req)))
		}
	}
}

func extractBearer(c fiber.Ctx) (string, bool) {
	auth := c.Get("Authorization")
	if len(auth) < 8 || !equalFoldPrefix(auth, "Bearer ") {
		return "", false
	}
	token := trimSpace(auth[7:])
	if token == "" {
		return "", false
	}
	return token, true
}

func extractBasic(c fiber.Ctx) (string, bool) {
	auth := c.Get("Authorization")
	if len(auth) < 7 || !equalFoldPrefix(auth, "Basic ") {
		return "", false
	}
	token := trimSpace(auth[6:])
	if token == "" {
		return "", false
	}
	return token, true
}

func extractHTTPScheme(name string) ExtractFunc {
	prefix := name + " "
	return func(c fiber.Ctx) (string, bool) {
		auth := c.Get("Authorization")
		if len(auth) <= len(prefix) || !equalFoldPrefix(auth, prefix) {
			return "", false
		}
		token := trimSpace(auth[len(prefix):])
		if token == "" {
			return "", false
		}
		return token, true
	}
}

func apiKeyExtractor(name string, in APIKeyIn) ExtractFunc {
	return func(c fiber.Ctx) (string, bool) {
		switch in {
		case InQuery:
			v := c.Get(name)
			if v == "" {
				return "", false
			}
			return v, true
		case InCookie:
			v := c.Cookies(name)
			if v == "" {
				return "", false
			}
			return v, true
		default:
			v := c.Get(name)
			if v == "" {
				return "", false
			}
			return v, true
		}
	}
}

func extractMutualTLS(c fiber.Ctx) (string, bool) {
	cert := c.Get("X-Forwarded-Tls-Client-Cert")
	if cert == "" {
		cert = c.Get("X-SSL-Client-Cert")
	}
	if cert == "" {
		return "", false
	}
	return cert, true
}

func equalFoldPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := range prefix {
		a, b := s[i], prefix[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// DocumentSecurity resolves document-level security for OpenAPI generation.
func DocumentSecurity(spec []SecurityRequirement, defaultSpec SecuritySpec) []SecurityRequirement {
	if len(spec) > 0 {
		return cloneRequirements(spec)
	}
	if defaultSpec.isExplicit() && !defaultSpec.isPublic() {
		return cloneRequirements(defaultSpec.requirements)
	}
	return nil
}

// MergeSecuritySchemes merges registered schemes into a components map.
func MergeSecuritySchemes(base map[string]*SecurityScheme, registered map[string]Scheme) map[string]*SecurityScheme {
	if len(registered) == 0 {
		return base
	}
	out := base
	if out == nil {
		out = map[string]*SecurityScheme{}
	} else {
		clone := make(map[string]*SecurityScheme, len(base)+len(registered))
		for k, v := range base {
			clone[k] = v
		}
		out = clone
	}
	for name, scheme := range registered {
		spec := scheme.Spec
		out[name] = &spec
	}
	return out
}

// RegisteredSchemeNames returns scheme names from a registry map.
func RegisteredSchemeNames(schemes map[string]Scheme) []string {
	names := make([]string, 0, len(schemes))
	for name := range schemes {
		names = append(names, name)
	}
	return names
}
