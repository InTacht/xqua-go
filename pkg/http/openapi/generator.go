package openapi

import (
	stderrors "errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/InTacht/xqua-go/pkg/errors"
	xhttp "github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/builder"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/parser"
	spec_reflect "github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
	"github.com/InTacht/xqua-go/pkg/runtime"

	"github.com/gofiber/fiber/v3"
)

// OpenAPIVersion is the OpenAPI specification version this engine emits.
const OpenAPIVersion = spec.Version320

// OpenAPIDialect is the default jsonSchemaDialect for generated documents
// (OpenAPI 3.2 dialect, 2025-09-17).
const OpenAPIDialect = "https://spec.openapis.org/oas/3.2/dialect/2025-09-17"

// defaultOpenAPIPath is the URL a document is served at when Config.Specs is nil
// (a single document) or when a Spec omits Path.
const defaultOpenAPIPath = "/openapi.json"

// envelopeVersionExtension names the vendor extension pinning the RES envelope
// wire contract on generated documents.
const envelopeVersionExtension = "x-xqua-envelope-version"

// Config configures the OpenAPI engine. Every field is optional: zero values
// inherit from the transport (DefaultStatus) or use canonical defaults
// (KindStatuses), and Specs follows the tri-state serving policy below.
type Config struct {
	// KindStatuses maps semantic error kinds to HTTP status codes for routes
	// registered through this engine. Nil uses DefaultKindStatuses().
	KindStatuses KindStatuses

	// DefaultStatus is the status used when a returned catalog error's kind is
	// not in KindStatuses. Zero inherits the transport's DefaultStatus.
	DefaultStatus int

	// Specs declares the OpenAPI documents this engine serves. Each is
	// auto-registered as a GET endpoint at its Path, generated lazily from the
	// engine manifest and filtered by prefix and membership tags.
	//
	//   - nil (default): a single document at /openapi.json covering all
	//     documented routes;
	//   - empty (non-nil) slice: no endpoints (explicit opt-out; use Document
	//     for programmatic generation);
	//   - non-empty: exactly those documents.
	Specs []Spec
}

// Generator is the declarative REST/OpenAPI engine. It attaches to a Transport,
// registers routes on the underlying Fiber app with catalog error mapping, and
// serves one or more OpenAPI 3.2 documents describing them. Create one with New.
type Generator struct {
	app     fiber.Router
	catalog *errors.Catalog
	log     runtime.Logger
	version string

	kindStatuses  KindStatuses
	defaultStatus int
	rec           *recorder
}

// New attaches an OpenAPI engine to a transport. It reads the transport's Fiber
// app, public catalog, logger, and default version/status through the
// transport's accessors, and auto-registers the configured document endpoints.
func New(t *xhttp.Transport, cfg Config) *Generator {
	ks := cfg.KindStatuses
	if ks == nil {
		ks = DefaultKindStatuses()
	}
	ds := cfg.DefaultStatus
	if ds <= 0 {
		ds = t.DefaultStatus()
	}
	g := &Generator{
		app:           t.Fiber(),
		catalog:       t.Catalog(),
		log:           t.Logger(),
		version:       t.Version(),
		kindStatuses:  ks,
		defaultStatus: ds,
		rec:           &recorder{},
	}
	g.install(cfg.Specs)
	return g
}

// Routes registers routes under a path prefix.
func (g *Generator) Routes(path string, register func(r *Router)) *Generator {
	register(newRouter(g.app.Group(path), g.catalog, g.kindStatuses, g.defaultStatus, path, g.rec))
	return g
}

// Manifest returns the engine's registration-time bookkeeping: every route
// registered through it (with resolved per-error HTTP status), the full public
// catalog, and the envelope version. Call it after all routes are registered.
func (g *Generator) Manifest() Manifest {
	return Manifest{
		EnvelopeVersion: xhttp.EnvelopeVersion,
		Routes:          g.rec.snapshot(),
		Catalog:         catalogSpecs(g.catalog, g.kindStatuses, g.defaultStatus),
	}
}

// Document renders one OpenAPI 3.2 document from the current manifest. It is
// pure (no server involved) and useful for CLI/tests. The served endpoints use
// the same path internally.
func (g *Generator) Document(s Spec) (*Document, error) {
	return buildDocument(g.Manifest(), s, g.version)
}

// install auto-registers a GET handler per configured document on Fiber. The
// endpoints attach directly, so they are never documented themselves.
func (g *Generator) install(specs []Spec) {
	if specs == nil {
		specs = []Spec{{Path: defaultOpenAPIPath}}
	}
	for _, s := range specs {
		if s.Path == "" {
			s.Path = defaultOpenAPIPath
		}
		g.app.Get(s.Path, g.handler(s))
	}
}

// Spec declares one OpenAPI document served by the engine. Several specs may
// coexist, each filtered to a subset of routes by Prefix (segment-aware path
// filter) and Specs (route membership tags).
type Spec struct {
	// Path is the URL the document is served at, e.g. "/mobile/openapi.json".
	// Empty defaults to "/openapi.json".
	Path string
	// Prefix restricts the document to routes under this path prefix
	// (segment-aware: "/mobile" matches "/mobile" and "/mobile/..." but not
	// "/mobilepay"). Empty includes all paths.
	Prefix string
	// Specs restricts the document to routes whose Route.Specs intersect this
	// list. Empty applies no tag filter.
	Specs []string

	// Self is the OpenAPI 3.2 $self URI identifying this document.
	Self string
	// JSONSchemaDialect is the default dialect for Schema Objects in this
	// document. Empty defaults to OpenAPIDialect.
	JSONSchemaDialect string

	Title string
	// Summary is a short info.summary (OpenAPI 3.2).
	Summary     string
	Description string
	// Version defaults to the transport's Version, then "0.0.0".
	Version string

	Contact      *Contact
	License      *License
	Servers      []Server
	Tags         []Tag
	ExternalDocs *ExternalDocs

	// Schemas are component schemas added to this document's components.schemas
	// alongside the envelope models, e.g. for {Ref: ...} body targets.
	Schemas map[string]*Schema
	// SecuritySchemes are named security schemes under components.securitySchemes
	// (apiKey, http, oauth2 including deviceAuthorization, openIdConnect).
	SecuritySchemes map[string]*SecurityScheme
	// Security is the document-level security requirement list. Each entry is an
	// alternative; keys are scheme names from SecuritySchemes, values are scopes.
	Security []SecurityRequirement

	// Webhooks declares OpenAPI 3.1+ webhook operations: outbound calls the
	// service makes to subscribers, keyed by webhook name.
	Webhooks map[string]Webhook

	// ReflectorConfig tunes how Go types are reflected into schemas.
	ReflectorConfig *ReflectorConfig
}

// Webhook declares one OpenAPI webhook operation (an outbound call). Method is
// the HTTP method of the callback; the embedded Route carries the operation
// metadata (summary, request/response schemas, ...). Handler is ignored.
type Webhook struct {
	Method string
	Route
}

// buildDocument renders an OpenAPI 3.2 document from a manifest for one spec. It
// is pure: manifest + spec in, document out. The manifest is first narrowed
// with Manifest.ForSpec(s.Prefix, s.Specs): only documented routes matching the
// spec's prefix and membership tags are included, and the catalog is subset to
// the errors those routes reference. Each route contributes one operation with
// reflect-derived request/response schemas, catalog error responses (carried
// under x-xqua-errors), and a default error-envelope response. The document is
// validated against OpenAPI 3.2 before it is returned.
func buildDocument(m Manifest, s Spec, defaultVersion string) (*Document, error) {
	m = m.ForSpec(s.Prefix, s.Specs)

	cfg := specConfig(s, defaultVersion)
	doc := newDocument(cfg, s, m)
	b := builder.NewBuilder(cfg, doc)
	pathParser := parser.NewColonParamParser()

	for i := range m.Routes {
		if err := addRoute(b, pathParser, m.Routes[i]); err != nil {
			return nil, err
		}
	}
	for name, wh := range s.Webhooks {
		if err := addWebhook(b, pathParser, name, wh); err != nil {
			return nil, err
		}
	}
	b.Finish()

	if err := validateDocument(doc, cfg.OpenAPIVersion); err != nil {
		return nil, err
	}
	return doc, nil
}

// specConfig maps a Spec onto the internal generator configuration, filling
// OpenAPI 3.2 defaults for version, dialect, title, and version.
func specConfig(s Spec, defaultVersion string) *spec.Config {
	dialect := s.JSONSchemaDialect
	if dialect == "" {
		dialect = OpenAPIDialect
	}
	title := s.Title
	if title == "" {
		title = "Service API"
	}
	version := s.Version
	if version == "" {
		version = defaultVersion
	}
	if version == "" {
		version = "0.0.0"
	}
	var description *string
	if s.Description != "" {
		d := s.Description
		description = &d
	}
	return &spec.Config{
		OpenAPIVersion:    OpenAPIVersion,
		Self:              s.Self,
		JSONSchemaDialect: dialect,
		Title:             title,
		InfoSummary:       s.Summary,
		Version:           version,
		Description:       description,
		Contact:           s.Contact,
		License:           s.License,
		Servers:           s.Servers,
		SecuritySchemes:   s.SecuritySchemes,
		Security:          s.Security,
		Tags:              s.Tags,
		ExternalDocs:      s.ExternalDocs,
		ReflectorConfig:   s.ReflectorConfig,
		PathParser:        parser.NewColonParamParser(),
	}
}

// newDocument seeds the OpenAPI document with document-level metadata, the RES
// envelope component schemas, and the spec's own component schemas.
func newDocument(cfg *spec.Config, s Spec, m Manifest) *spec.Document {
	doc := &spec.Document{
		OpenAPI:           cfg.OpenAPIVersion,
		Self:              cfg.Self,
		JSONSchemaDialect: cfg.JSONSchemaDialect,
		Info: spec.Info{
			Title:       cfg.Title,
			Summary:     cfg.InfoSummary,
			Description: cfg.Description,
			Version:     cfg.Version,
			Contact:     cfg.Contact,
			License:     cfg.License,
		},
		Servers:      cfg.Servers,
		Security:     cfg.Security,
		Tags:         cfg.Tags,
		ExternalDocs: cfg.ExternalDocs,
		Paths:        map[string]*spec.PathItem{},
		Components: &spec.Components{
			Schemas:         envelopeComponents(),
			SecuritySchemes: cfg.SecuritySchemes,
		},
		Extra: map[string]any{envelopeVersionExtension: m.EnvelopeVersion},
	}
	for name, schema := range s.Schemas {
		doc.Components.Schemas[name] = schema
	}
	return doc
}

// addRoute converts one manifest route into an OpenAPI operation via the builder
// and injects the catalog error responses and streaming item schemas.
func addRoute(b *builder.Builder, pathParser spec.PathParser, route RouteSpec) error {
	target, err := pathParser.Parse(route.Path)
	if err != nil {
		return err
	}

	opCfg := operationConfig(route.Method, route.OperationID, target, route.Summary, route.Description, route.Tags, route.op)

	var extra []ResponseDecl
	streaming, err := applyResponses(b, &opCfg, route.op.successBody, route.op.extra)
	if err != nil {
		return err
	}
	extra = streaming

	if err := b.AddOperation(route.Method, target, opCfg); err != nil {
		return err
	}

	op := operationFor(b.Doc.Paths[target], route.Method)
	if op == nil {
		return nil // hidden operation
	}
	for i := range route.op.requests {
		if err := addExplicitRequest(b, op, route.op.requests[i]); err != nil {
			return err
		}
	}
	for _, rd := range extra {
		if err := addStreamingResponse(b, op, rd); err != nil {
			return err
		}
	}
	injectCatalogErrors(op, route.Errors)
	return nil
}

// addWebhook renders a declared webhook into the document's webhooks section.
func addWebhook(b *builder.Builder, pathParser spec.PathParser, name string, wh Webhook) error {
	method := wh.Method
	if method == "" {
		method = http.MethodPost
	}
	decl := operationDecl{
		deprecated:   wh.Deprecated,
		hidden:       wh.Hidden,
		externalDocs: wh.ExternalDocs,
		security:     wh.Security,
		request:      wh.Request,
		requests:     wh.Requests,
		successBody:  successBodyFromWebhook(wh),
		extra:        wh.Extra,
	}
	opCfg := operationConfig(method, wh.OperationID, name, wh.Summary, wh.Description, wh.Tags, decl)
	if _, err := applyResponses(b, &opCfg, decl.successBody, decl.extra); err != nil {
		return err
	}
	return b.AddWebhookOperation(method, name, opCfg)
}

// operationConfig builds the builder operation config shared by routes and
// webhooks from the declarative operation data.
func operationConfig(method, operationID, target, summary, description string, tags []string, decl operationDecl) builder.OperationConfig {
	id := operationID
	if id == "" {
		id = operationIDFrom(method, target)
	}
	opCfg := builder.OperationConfig{
		OperationID:  id,
		Summary:      summary,
		Description:  description,
		Tags:         tags,
		Deprecated:   decl.deprecated,
		Hide:         decl.hidden,
		ExternalDocs: decl.externalDocs,
		Security:     securityConfigs(decl.security),
	}
	if decl.request != nil {
		opCfg.Requests = append(opCfg.Requests, &spec.ContentUnit{Structure: decl.request})
	}
	return opCfg
}

// applyResponses fills opCfg.Responses from success body and extra declarations.
func applyResponses(b *builder.Builder, opCfg *builder.OperationConfig, successBody any, extra []ResponseDecl) ([]ResponseDecl, error) {
	var streaming []ResponseDecl
	hasSuccess := false
	if successBody != nil && !extraHasStreamingSuccess(extra) {
		hasSuccess = true
		opCfg.Responses = append(opCfg.Responses, &spec.ContentUnit{
			HTTPStatus:  http.StatusOK,
			Structure:   successBody,
			Description: "Success",
		})
	}
	for _, rd := range extra {
		if isSuccessResponse(rd) {
			hasSuccess = true
		}
		if rd.ItemBody != nil {
			streaming = append(streaming, rd)
			continue
		}
		opCfg.Responses = append(opCfg.Responses, responseUnit(rd))
	}
	if !hasSuccess {
		opCfg.Responses = append(opCfg.Responses, &spec.ContentUnit{
			HTTPStatus:  http.StatusOK,
			Structure:   schemaRef("Envelope"),
			Description: "Success envelope",
		})
	}
	return streaming, nil
}

func extraHasStreamingSuccess(extra []ResponseDecl) bool {
	for _, rd := range extra {
		if rd.ItemBody != nil && rd.Status >= 200 && rd.Status < 300 {
			return true
		}
	}
	return false
}

func successBodyFromWebhook(wh Webhook) any {
	if wh.Responses != nil {
		if body := wh.Responses.SuccessBody(); body != nil {
			return body
		}
	}
	for _, rd := range wh.Extra {
		if isSuccessResponse(rd) {
			return rd.Body
		}
	}
	return nil
}

// operationIDFrom returns the explicit operationId or derives a deterministic
// one from method and converted path.
func operationIDFrom(method, target string) string {
	return operationID(method, target)
}

// securityConfigs maps declarative security requirements onto the builder's
// per-scheme configuration.
func securityConfigs(reqs []SecurityRequirement) []builder.OperationSecurityConfig {
	if len(reqs) == 0 {
		return nil
	}
	var out []builder.OperationSecurityConfig
	for _, req := range reqs {
		for name, scopes := range req {
			out = append(out, builder.OperationSecurityConfig{Name: name, Scopes: scopes})
		}
	}
	return out
}

// responseUnit maps a ResponseDecl onto a content unit consumed by the builder.
func responseUnit(rd ResponseDecl) *spec.ContentUnit {
	cu := &spec.ContentUnit{
		HTTPStatus:  rd.Status,
		IsDefault:   rd.Default,
		Structure:   rd.Body,
		ContentType: rd.ContentType,
		Description: rd.Description,
		Summary:     rd.Summary,
		Example:     rd.Example,
		Examples:    rd.Examples,
		Required:    rd.Required,
	}
	if len(rd.Encoding) > 0 {
		cu.Encoding = rd.Encoding
	}
	return cu
}

// addExplicitRequest attaches an explicit request content unit to the operation.
// A *spec.Schema structure is used verbatim; any other value is reflected.
func addExplicitRequest(b *builder.Builder, op *spec.Operation, cu ContentUnit) error {
	if op.RequestBody == nil {
		op.RequestBody = &spec.RequestBody{Content: map[string]spec.MediaType{}}
	}
	if cu.Description != "" {
		op.RequestBody.Description = cu.Description
	}
	if cu.Required {
		op.RequestBody.Required = true
	}
	ct := cu.ContentType
	if ct == "" {
		ct = "application/json"
	}
	schema, err := bodySchema(b, cu.Structure)
	if err != nil {
		return err
	}
	mt := spec.MediaType{Schema: schema, Example: cu.Example, Examples: cu.Examples}
	if len(cu.Encoding) > 0 {
		mt.Encoding = map[string]*spec.Encoding{}
		for prop, enc := range cu.Encoding {
			mt.Encoding[prop] = &spec.Encoding{ContentType: enc}
		}
	}
	op.RequestBody.Content[ct] = mt
	return nil
}

// addStreamingResponse sets an OpenAPI 3.2 itemSchema media type for a streaming
// response (text/event-stream by default); no schema and no envelope are used.
func addStreamingResponse(b *builder.Builder, op *spec.Operation, rd ResponseDecl) error {
	key := responseKey(rd)
	resp := op.Responses[key]
	if resp == nil {
		desc := rd.Description
		if desc == "" {
			desc = "Streamed response"
		}
		resp = &spec.Response{Description: desc}
		op.Responses[key] = resp
	}
	if resp.Content == nil {
		resp.Content = map[string]spec.MediaType{}
	}
	ct := rd.ContentType
	if ct == "" {
		ct = "text/event-stream"
	}
	item, err := bodySchema(b, rd.ItemBody)
	if err != nil {
		return err
	}
	resp.Content[ct] = spec.MediaType{ItemSchema: item, Example: rd.Example, Examples: rd.Examples}
	return nil
}

// bodySchema resolves a declarative body value to a schema: a *spec.Schema is
// used verbatim, anything else is reflected into a component schema.
func bodySchema(b *builder.Builder, value any) (*spec.Schema, error) {
	if value == nil {
		return nil, nil
	}
	if schema, ok := value.(*spec.Schema); ok {
		return schema, nil
	}
	return b.Reflector.SchemaForValue(value, spec_reflect.SchemaUseComponent)
}

// injectCatalogErrors adds one response per declared catalog status (carrying
// the entries under x-xqua-errors) plus a default error-envelope response.
func injectCatalogErrors(op *spec.Operation, specs []ErrorSpec) {
	for _, status := range errorStatuses(specs) {
		entries := errorsForStatus(specs, status)
		op.Responses[strconv.Itoa(status)] = &spec.Response{
			Description: errorDescription(entries),
			Content:     errorEnvelopeContent(),
			Extensions:  map[string]any{"x-xqua-errors": entries},
		}
	}
	op.Responses["default"] = &spec.Response{
		Description: "Any other public catalog error, rendered in the error envelope",
		Content:     errorEnvelopeContent(),
	}
}

func errorEnvelopeContent() map[string]spec.MediaType {
	return map[string]spec.MediaType{
		"application/json": {Schema: schemaRef("ErrorEnvelope")},
	}
}

// operationFor returns the operation stored on a path item for an HTTP method,
// including the OpenAPI 3.2 QUERY key and additional (custom) methods.
func operationFor(item *spec.PathItem, method string) *spec.Operation {
	if item == nil {
		return nil
	}
	switch strings.ToUpper(method) {
	case http.MethodGet:
		return item.Get
	case http.MethodPut:
		return item.Put
	case http.MethodPost:
		return item.Post
	case http.MethodDelete:
		return item.Delete
	case http.MethodOptions:
		return item.Options
	case http.MethodHead:
		return item.Head
	case http.MethodPatch:
		return item.Patch
	case http.MethodTrace:
		return item.Trace
	case xhttp.MethodQuery:
		return item.Query
	default:
		return item.AdditionalOperations[strings.ToUpper(method)]
	}
}

func isSuccessResponse(rd ResponseDecl) bool {
	return rd.Default || (rd.Status >= 200 && rd.Status < 300)
}

func responseKey(rd ResponseDecl) string {
	if rd.Default {
		return "default"
	}
	return strconv.Itoa(rd.Status)
}

func schemaRef(name string) *spec.Schema {
	return &spec.Schema{Ref: "#/components/schemas/" + name}
}

// validateDocument runs the OpenAPI validator and returns the first hard error,
// ignoring warnings and informational findings.
func validateDocument(doc *spec.Document, version string) error {
	for _, err := range validate.ValidateDocument(doc, version) {
		var ve *validate.Error
		if stderrors.As(err, &ve) && ve.Severity != validate.SeverityError {
			continue
		}
		return err
	}
	return nil
}

// errorStatuses returns the distinct declared statuses in ascending order.
func errorStatuses(specs []ErrorSpec) []int {
	seen := map[int]bool{}
	var out []int
	for _, s := range specs {
		if !seen[s.HTTPStatus] {
			seen[s.HTTPStatus] = true
			out = append(out, s.HTTPStatus)
		}
	}
	sort.Ints(out)
	return out
}

func errorsForStatus(specs []ErrorSpec, status int) []ErrorSpec {
	var out []ErrorSpec
	for _, s := range specs {
		if s.HTTPStatus == status {
			out = append(out, s)
		}
	}
	return out
}

// errorDescription renders a human summary of the catalog errors behind one
// status, e.g. "user not found (not_found/10002)".
func errorDescription(specs []ErrorSpec) string {
	parts := make([]string, 0, len(specs))
	for _, s := range specs {
		parts = append(parts, s.Message+" ("+s.Kind+"/"+s.Code+")")
	}
	return strings.Join(parts, "; ")
}

// operationID derives a deterministic snake_case id from method and an OpenAPI
// path: GET /api/v1/users/{id} → "get_api_v1_users_id".
func operationID(method, path string) string {
	id := strings.ToLower(method)
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return id + "_root"
	}
	for _, seg := range strings.Split(trimmed, "/") {
		seg = strings.NewReplacer("{", "", "}", "", ".", "_", "-", "_").Replace(seg)
		if seg == "" {
			continue
		}
		id += "_" + seg
	}
	return id
}
