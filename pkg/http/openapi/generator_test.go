package openapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/InTacht/xqua-go/pkg/logger"

	"gopkg.in/yaml.v3"
)

var update = flag.Bool("update", false, "rewrite golden files")

var specCatalog = errors.NewCatalog("spec")

// Defined once: New would otherwise re-Define the standard fallbacks on the
// shared catalog for every transport this test file builds.
var specFallbacks = http.StandardErrors(specCatalog)

var (
	specIDRequired = specCatalog.Define(errors.Def{
		Kind: errors.KindValidation, Code: "10001", Message: "id is required", Source: "params.id",
	})
	specNotFound = specCatalog.Define(errors.Def{
		Kind: errors.KindNotFound, Code: "10002", Message: "user not found",
	})
	specConflict = specCatalog.Define(errors.Def{
		Kind: errors.KindConflict, Code: "10003", Message: "stale user version",
	})
)

type specAckIn struct{}

type specAckOut struct {
	openapi.Response
}

type specIDIn struct {
	ID int64 `path:"id"`
}

func specNop(_ context.Context, _ specAckIn) (specAckOut, error) { return specAckOut{}, nil }

func specNopID(_ context.Context, _ specIDIn) (specAckOut, error) { return specAckOut{}, nil }

func specLogger() *logger.Logger {
	return logger.New(&logger.Config{Name: "spec-test", ID: "spec-test-1"})
}

// newSpecAPI builds a transport (Version 1.2.3) and attaches an OpenAPI engine
// serving the given specs. Pass an empty slice to opt out of served endpoints
// and drive generation directly via Generator.Document.
func newSpecAPI(specs []openapi.Spec) (*http.Transport, *openapi.Generator) {
	tr := http.New(http.Config{
		Logger:    specLogger(),
		Catalog:   specCatalog,
		Fallbacks: specFallbacks,
		Version:   "1.2.3",
	})
	return tr, openapi.New(tr, openapi.Config{Specs: specs})
}

// goldenAPI builds a representative single-surface engine for golden tests.
func goldenAPI() *openapi.Generator {
	_, api := newSpecAPI([]openapi.Spec{})

	type getUserIn struct {
		ID int64 `path:"id"`
	}
	type userOut struct {
		openapi.Response
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	type createUserIn struct {
		Name string `json:"name"`
	}

	nopGet := func(_ context.Context, _ getUserIn) (userOut, error) { return userOut{}, nil }
	nopPost := func(_ context.Context, _ createUserIn) (userOut, error) { return userOut{}, nil }

	api.Routes("/api/v1", func(r *openapi.Router) {
		users := r.Group(openapi.GroupConfig{
			Prefix:    "/users",
			Responses: openapi.Returns().Err(422, specIDRequired),
		})
		users.Route("/:id").Get(openapi.Route{
			Handler:   nopGet,
			Summary:   "Fetch one user",
			Responses: openapi.Returns().Err(404, specNotFound),
		})
		users.Route("/:id").Put(openapi.Route{
			Handler:   nopGet,
			Summary:   "Update a user",
			Responses: openapi.Returns().Err(404, specNotFound).Err(409, specConflict),
		})
		users.Route("/").Post(openapi.Route{
			Handler:     nopPost,
			Summary:     "Create a user",
			Description: "Creates a user from the request body.",
			Requests: []openapi.ContentUnit{{
				Description: "New user",
				Required:    true,
				Structure:   &openapi.Schema{Ref: "#/components/schemas/NewUser"},
			}},
			Responses: openapi.Returns(userOut{}),
		})
		r.Route("/internal").Get(openapi.Route{
			Handler:   specNop,
			Responses: openapi.Returns(),
		})
	})
	return api
}

func goldenSpec() openapi.Spec {
	return openapi.Spec{
		Self:    "https://api.example.com/openapi.json",
		Title:   "Spec Test API",
		Summary: "Contract for the spec-test transport",
		Version: "1.2.3",
		Servers: []openapi.Server{{URL: "https://api.example.com"}},
		Schemas: map[string]*openapi.Schema{
			"NewUser": {
				Type:     "object",
				Required: []string{"name"},
				Properties: map[string]*openapi.Schema{
					"name":  {Type: "string"},
					"email": {Type: "string"},
				},
			},
			"User": {
				Type: "object",
				Properties: map[string]*openapi.Schema{
					"id":   {Type: "integer"},
					"name": {Type: "string"},
				},
			},
		},
	}
}

func generate(t *testing.T) *openapi.Document {
	t.Helper()
	doc, err := goldenAPI().Document(goldenSpec())
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return doc
}

func TestGenerateStructure(t *testing.T) {
	doc := generate(t)

	if doc.OpenAPI != openapi.OpenAPIVersion {
		t.Errorf("expected openapi %q, got %q", openapi.OpenAPIVersion, doc.OpenAPI)
	}
	if doc.JSONSchemaDialect != openapi.OpenAPIDialect {
		t.Errorf("expected dialect %q, got %q", openapi.OpenAPIDialect, doc.JSONSchemaDialect)
	}
	if doc.Self != "https://api.example.com/openapi.json" {
		t.Errorf("expected $self, got %q", doc.Self)
	}
	if doc.Extra["x-xqua-envelope-version"] != http.EnvelopeVersion {
		t.Errorf("expected envelope version %q, got %v", http.EnvelopeVersion, doc.Extra["x-xqua-envelope-version"])
	}
	if doc.Info.Title != "Spec Test API" || doc.Info.Version != "1.2.3" || doc.Info.Summary == "" {
		t.Errorf("unexpected info: %+v", doc.Info)
	}

	// Fiber ":id" converts to OpenAPI "{id}".
	item, ok := doc.Paths["/api/v1/users/{id}"]
	if !ok {
		t.Fatalf("expected converted path, got paths: %v", keys(doc.Paths))
	}
	get := item.Get
	if get == nil {
		t.Fatal("expected GET operation")
	}
	if get.Summary != "Fetch one user" {
		t.Errorf("expected summary, got %q", get.Summary)
	}
	if get.OperationID != "get_api_v1_users_id" {
		t.Errorf("unexpected operationId %q", get.OperationID)
	}
	if len(get.Parameters) != 1 || get.Parameters[0].Name != "id" || get.Parameters[0].In != "path" || !get.Parameters[0].Required {
		t.Errorf("unexpected parameters: %+v", get.Parameters)
	}

	// 200 success + kind-resolved 404/422 (group-inherited) + default.
	for _, status := range []string{"200", "404", "422", "default"} {
		if _, ok := get.Responses[status]; !ok {
			t.Errorf("expected %s response, got %v", status, keys(get.Responses))
		}
	}
	notFound := get.Responses["404"]
	nfErrors := xquaErrors(t, notFound)
	if len(nfErrors) != 1 || nfErrors[0].Code != specNotFound.Code {
		t.Errorf("expected x-xqua-errors carrying the catalog entry, got %+v", nfErrors)
	}
	if notFound.Description == "" {
		t.Error("expected error response description from the catalog entry")
	}

	// Explicit Status override lands under its status on PUT.
	put := item.Put
	if put == nil {
		t.Fatal("expected PUT operation")
	}
	conflict, ok := put.Responses["409"]
	if !ok {
		t.Fatalf("expected 409 response on PUT, got %v", keys(put.Responses))
	}
	if cErrors := xquaErrors(t, conflict); len(cErrors) != 1 || cErrors[0].Code != specConflict.Code {
		t.Errorf("unexpected 409 errors: %+v", cErrors)
	}

	// POST carries request/response body schemas (reflect model: the payload
	// type is the media schema, not the RES envelope).
	post := doc.Paths["/api/v1/users"].Post
	if post == nil {
		t.Fatal("expected POST operation")
	}
	if post.RequestBody == nil || !post.RequestBody.Required {
		t.Fatalf("expected required request body, got %+v", post.RequestBody)
	}
	if got := post.RequestBody.Content["application/json"].Schema.Ref; got != "#/components/schemas/NewUser" {
		t.Errorf("expected NewUser request ref, got %v", got)
	}
	if got := post.Responses["200"].Content["application/json"].Schema.Ref; got != "#/components/schemas/OpenapiTestUserOut" {
		t.Errorf("expected reflected userOut response ref, got %v", got)
	}

	// Ops and bare handlers are not Documented — omitted from the spec.
	for _, path := range []string{"/health", "/version", "/api/v1/internal"} {
		if _, ok := doc.Paths[path]; ok {
			t.Errorf("unexpected non-documented path %q in spec", path)
		}
	}

	// Envelope contract modeled once as components, plus the spec's own schemas.
	for _, name := range []string{"Envelope", "ErrorEnvelope", "ErrorDetail", "Pagination", "Cursor", "NewUser", "User"} {
		if _, ok := doc.Components.Schemas[name]; !ok {
			t.Errorf("expected component schema %q", name)
		}
	}
}

func TestGenerateGolden(t *testing.T) {
	got, err := openapi.MarshalJSON(generate(t))
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "spec.golden.json", got)
}

// TestGenerateGoldenYAML pins the YAML serialization byte-for-byte, guarding the
// YAML codec (goccy) against drift the same way the JSON golden does.
func TestGenerateGoldenYAML(t *testing.T) {
	got, err := openapi.MarshalYAML(generate(t))
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "spec.golden.yaml", got)
}

// assertGolden compares got against testdata/<name>, rewriting it under -update.
// Output is normalized to exactly one trailing newline so JSON and YAML codecs
// (which differ on trailing whitespace) share the same comparison.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	got = append(bytes.TrimRight(got, "\n"), '\n')

	golden := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden %s (run `go test ./pkg/http/openapi -update` to regenerate): %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("generated %s differs from golden file; run `go test ./pkg/http/openapi -update` and review the diff\n--- got ---\n%s", name, got)
	}
}

func TestYAMLOutput(t *testing.T) {
	raw, err := openapi.MarshalYAML(generate(t))
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		t.Fatalf("YAML output does not parse: %v", err)
	}
	if v["openapi"] != openapi.OpenAPIVersion {
		t.Errorf("expected openapi %q in YAML, got %v", openapi.OpenAPIVersion, v["openapi"])
	}
	if _, ok := v["paths"].(map[string]any); !ok {
		t.Errorf("expected paths mapping in YAML, got %T", v["paths"])
	}
}

// TestOpenAPI32Defaults verifies the document claims OpenAPI 3.2.0 and applies
// the default dialect when Spec.JSONSchemaDialect is empty.
func TestOpenAPI32Defaults(t *testing.T) {
	_, api := newSpecAPI([]openapi.Spec{})
	api.Routes("/api", func(r *openapi.Router) {
		r.Route("/a").Get(openapi.Route{Handler: specNop, Summary: "a", Responses: openapi.Returns()})
	})

	doc, err := api.Document(openapi.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	if doc.OpenAPI != "3.2.0" {
		t.Errorf("expected openapi 3.2.0, got %q", doc.OpenAPI)
	}
	if doc.JSONSchemaDialect != openapi.OpenAPIDialect {
		t.Errorf("expected default dialect, got %q", doc.JSONSchemaDialect)
	}
	if doc.Self != "" {
		t.Errorf("expected empty $self when unset, got %q", doc.Self)
	}

	custom, err := api.Document(openapi.Spec{
		JSONSchemaDialect: "https://example.com/dialect",
		Self:              "https://example.com/spec.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if custom.JSONSchemaDialect != "https://example.com/dialect" {
		t.Errorf("expected custom dialect, got %q", custom.JSONSchemaDialect)
	}
	if custom.Self != "https://example.com/spec.json" {
		t.Errorf("expected custom $self, got %q", custom.Self)
	}
}

// TestItemSchemaSSE verifies OpenAPI 3.2 itemSchema on a streaming response
// (text/event-stream) and that the RES envelope is not applied.
func TestItemSchemaSSE(t *testing.T) {
	_, api := newSpecAPI([]openapi.Spec{})
	api.Routes("/chat", func(r *openapi.Router) {
		r.Route("/stream").Post(openapi.Route{
			Handler: specNop,
			Summary: "Stream model tokens using SSE",
			Requests: []openapi.ContentUnit{{
				Required: true,
				Structure: &openapi.Schema{
					Type: "object",
					Properties: map[string]*openapi.Schema{
						"prompt": {Type: "string"},
					},
				},
			}},
			Extra: []openapi.ResponseDecl{{
				Status:      200,
				Description: "Token feed",
				ItemBody:    &openapi.Schema{Ref: "#/components/schemas/TokenChunk"},
			}},
		})
	})

	doc, err := api.Document(openapi.Spec{
		Schemas: map[string]*openapi.Schema{
			"TokenChunk": {
				Type:     "object",
				Required: []string{"id", "delta"},
				Properties: map[string]*openapi.Schema{
					"id":    {Type: "string", Format: "uuid"},
					"delta": {Type: "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	op := doc.Paths["/chat/stream"].Post
	if op == nil {
		t.Fatal("expected POST /chat/stream")
	}
	sse := op.Responses["200"].Content["text/event-stream"]
	if sse.ItemSchema == nil || sse.ItemSchema.Ref != "#/components/schemas/TokenChunk" {
		t.Errorf("expected itemSchema $ref to TokenChunk, got %+v", sse.ItemSchema)
	}
	if sse.Schema != nil {
		t.Errorf("streaming response must not set schema, got %+v", sse.Schema)
	}
	// Envelope must not wrap streaming responses.
	if _, ok := op.Responses["200"].Content["application/json"]; ok {
		t.Error("streaming response must not use application/json envelope")
	}
	if _, ok := doc.Components.Schemas["TokenChunk"]; !ok {
		t.Error("expected TokenChunk component schema")
	}
}

// TestQueryMethod verifies Router.Query registers the HTTP QUERY method and
// OpenAPI emits it under the "query" operation key.
func TestQueryMethod(t *testing.T) {
	tr, api := newSpecAPI([]openapi.Spec{})
	api.Routes("/analytics", func(r *openapi.Router) {
		r.Route("/search").Query(openapi.Route{
			Handler:     specNop,
			Summary:     "Query telemetry database",
			OperationID: "fetchTelemetry",
			Responses:   openapi.Returns(),
			Requests: []openapi.ContentUnit{{
				Required: true,
				Structure: &openapi.Schema{
					Type: "object",
					Properties: map[string]*openapi.Schema{
						"aggregations": {
							Type:  "array",
							Items: &openapi.Schema{Type: "string"},
						},
					},
				},
			}},
		})
	})

	route := findRoute(t, api.Manifest(), http.MethodQuery, "/analytics/search")
	if !route.Documented {
		t.Error("expected QUERY route to be Documented")
	}

	doc, err := api.Document(openapi.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/analytics/search"].Query
	if op == nil {
		t.Fatal("expected query operation")
	}
	if op.OperationID != "fetchTelemetry" {
		t.Errorf("unexpected operationId %q", op.OperationID)
	}
	if op.RequestBody == nil || !op.RequestBody.Required {
		t.Errorf("expected required request body on QUERY, got %+v", op.RequestBody)
	}

	// Fiber must accept QUERY at runtime (RequestMethods includes it by default).
	req := httptest.NewRequest(http.MethodQuery, "/analytics/search", nil)
	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("QUERY handler status = %d, want 200", resp.StatusCode)
	}
}

// TestDeviceOAuth verifies document-level security and OAuth2 deviceAuthorization
// (OpenAPI 3.2) security schemes land under components.securitySchemes.
func TestDeviceOAuth(t *testing.T) {
	_, api := newSpecAPI([]openapi.Spec{})
	api.Routes("/api", func(r *openapi.Router) {
		r.Route("/metrics").Get(openapi.Route{Handler: specNop, Summary: "Metrics", Responses: openapi.Returns()})
	})

	doc, err := api.Document(openapi.Spec{
		Security: []openapi.SecurityRequirement{
			{"OAuth2DeviceFlow": {"analytics:write"}, "ApiKeyHeader": {}},
		},
		SecuritySchemes: map[string]*openapi.SecurityScheme{
			"ApiKeyHeader": {
				Type: "apiKey",
				In:   "header",
				Name: "X-API-Token",
			},
			"OAuth2DeviceFlow": {
				Type:              "oauth2",
				OAuth2MetadataURL: "https://example.com/.well-known/oauth-authorization-server",
				Flows: &openapi.OAuthFlows{
					DeviceAuthorization: &openapi.OAuthFlow{
						AuthorizationURL:       "https://example.com/authorize",
						DeviceAuthorizationURL: "https://example.com/device",
						TokenURL:               "https://example.com/token",
						Scopes: map[string]string{
							"analytics:write": "Permission to post metrics data directly.",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(doc.Security) != 1 {
		t.Fatalf("expected document security, got %+v", doc.Security)
	}
	if scopes := doc.Security[0]["OAuth2DeviceFlow"]; len(scopes) != 1 || scopes[0] != "analytics:write" {
		t.Errorf("unexpected OAuth2 scopes: %v", scopes)
	}

	oauth := doc.Components.SecuritySchemes["OAuth2DeviceFlow"]
	if oauth.Type != "oauth2" || oauth.OAuth2MetadataURL == "" {
		t.Errorf("unexpected oauth scheme: %+v", oauth)
	}
	flow := oauth.Flows.DeviceAuthorization
	if flow == nil || flow.DeviceAuthorizationURL == "" || flow.TokenURL == "" {
		t.Errorf("expected deviceAuthorization flow, got %+v", oauth.Flows)
	}
	if flow.Scopes["analytics:write"] == "" {
		t.Error("expected analytics:write scope description")
	}

	apiKey := doc.Components.SecuritySchemes["ApiKeyHeader"]
	if apiKey.Type != "apiKey" || apiKey.Name != "X-API-Token" || apiKey.In != "header" {
		t.Errorf("unexpected apiKey scheme: %+v", apiKey)
	}
}

func TestConfigSchemesMergedIntoDocument(t *testing.T) {
	tr := http.New(http.Config{Logger: specLogger(), Catalog: specCatalog, Fallbacks: specFallbacks, Version: "1.2.3"})
	api := openapi.New(tr, openapi.Config{
		Specs: []openapi.Spec{},
		Schemes: map[string]openapi.Scheme{
			"BearerAuth": openapi.BearerScheme(openapi.BearerOptions{Format: "JWT"}),
		},
	})
	api.Routes("/api", func(r *openapi.Router) {
		r.Describe("/secure").Get(openapi.Route{
			Summary:   "Secure route",
			Security:  openapi.RequireSecurity("BearerAuth"),
			Responses: openapi.Returns(specAckOut{}),
		})
	})

	doc, err := api.Document(openapi.Spec{Title: "API"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Components.SecuritySchemes["BearerAuth"] == nil {
		t.Fatal("expected BearerAuth in components.securitySchemes")
	}
	item := doc.Paths["/api/secure"]
	if item == nil || item.Get == nil {
		t.Fatal("expected GET /api/secure operation")
	}
	if len(item.Get.Security) != 1 || item.Get.Security[0]["BearerAuth"] == nil {
		t.Fatalf("expected operation security, got %+v", item.Get.Security)
	}
}

// TestMultipartBodyEncoding verifies ContentType and Encoding on request bodies
// (o2-style multipart uploads).
func TestMultipartBodyEncoding(t *testing.T) {
	_, api := newSpecAPI([]openapi.Spec{})
	api.Routes("/assets", func(r *openapi.Router) {
		r.Route("/upload").Post(openapi.Route{
			Handler:   specNop,
			Summary:   "Upload asset",
			Responses: openapi.Returns(),
			Requests: []openapi.ContentUnit{{
				Required:    true,
				ContentType: "multipart/form-data",
				Structure:   &openapi.Schema{Ref: "#/components/schemas/MultipartUploadPayload"},
				Encoding: map[string]string{
					"file_binary": "image/png, image/jpeg, application/pdf",
				},
			}},
		})
	})

	doc, err := api.Document(openapi.Spec{
		Schemas: map[string]*openapi.Schema{
			"MultipartUploadPayload": {Type: "object"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rb := doc.Paths["/assets/upload"].Post.RequestBody
	mt := rb.Content["multipart/form-data"]
	if mt.Schema == nil || mt.Schema.Ref != "#/components/schemas/MultipartUploadPayload" {
		t.Errorf("expected multipart schema ref, got %+v", mt.Schema)
	}
	if mt.Encoding["file_binary"] == nil || mt.Encoding["file_binary"].ContentType == "" {
		t.Errorf("expected file_binary encoding, got %+v", mt.Encoding)
	}
}

// TestOperationIDExplicitVsAuto verifies an explicit OperationID wins and an
// empty one derives from method + path.
func TestOperationIDExplicitVsAuto(t *testing.T) {
	_, api := newSpecAPI([]openapi.Spec{})
	api.Routes("/api", func(r *openapi.Router) {
		r.Route("/a").Get(openapi.Route{Handler: specNop, Summary: "auto", Responses: openapi.Returns()})
		r.Route("/b").Get(openapi.Route{Handler: specNop, OperationID: "customId", Responses: openapi.Returns()})
	})

	doc, err := api.Document(openapi.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Paths["/api/a"].Get.OperationID; got != "get_api_a" {
		t.Errorf("expected auto operationId, got %q", got)
	}
	if got := doc.Paths["/api/b"].Get.OperationID; got != "customId" {
		t.Errorf("expected explicit operationId, got %q", got)
	}
}

// TestDescribeDocumentsWithoutRouting verifies Describe records a
// documentation-only operation (rendered into documents) without registering a
// Fiber route.
func TestDescribeDocumentsWithoutRouting(t *testing.T) {
	tr, api := newSpecAPI([]openapi.Spec{})
	api.Routes("", func(r *openapi.Router) {
		r.Describe("/chat/ws").Get(openapi.Route{
			Summary: "WebSocket handshake",
			Extra: []openapi.ResponseDecl{
				{Status: 101, Description: "Switching Protocols"},
			},
		})
	})

	route := findRoute(t, api.Manifest(), "GET", "/chat/ws")
	if !route.Documented {
		t.Error("expected described route to be Documented")
	}

	doc, err := api.Document(openapi.Spec{})
	if err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/chat/ws"].Get
	if op == nil {
		t.Fatal("expected described GET /chat/ws operation")
	}
	if _, ok := op.Responses["101"]; !ok {
		t.Errorf("expected 101 response, got %v", keys(op.Responses))
	}

	// Describe never routes traffic: no Fiber handler was registered.
	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/chat/ws", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected no live /chat/ws route (404), got %d", resp.StatusCode)
	}
}

// TestDescribeRejectsHandler verifies Describe panics when a Handler is set,
// since it must not register live traffic.
func TestDescribeRejectsHandler(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when Describe route sets Handler")
		}
	}()
	_, api := newSpecAPI([]openapi.Spec{})
	api.Routes("", func(r *openapi.Router) {
		r.Describe("/x").Get(openapi.Route{Handler: specNop, Summary: "x"})
	})
}

// TestWebhookRendered verifies declarative webhooks land in the document's
// webhooks section via the builder.
func TestWebhookRendered(t *testing.T) {
	_, api := newSpecAPI([]openapi.Spec{})
	doc, err := api.Document(openapi.Spec{
		Webhooks: map[string]openapi.Webhook{
			"newInvoice": {
				Method: "POST",
				Route: openapi.Route{
					Summary: "An invoice was created",
					Requests: []openapi.ContentUnit{{
						Required:  true,
						Structure: &openapi.Schema{Ref: "#/components/schemas/Invoice"},
					}},
				},
			},
		},
		Schemas: map[string]*openapi.Schema{
			"Invoice": {Type: "object"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	item, ok := doc.Webhooks["newInvoice"]
	if !ok || item == nil {
		t.Fatalf("expected newInvoice webhook, got %v", doc.Webhooks)
	}
	if item.Post == nil {
		t.Fatalf("expected POST webhook operation, got %+v", item)
	}
	if item.Post.Summary != "An invoice was created" {
		t.Errorf("unexpected webhook summary %q", item.Post.Summary)
	}
}

// TestForSpecFiltering covers prefix boundaries, tag intersection, membership in
// two documents, and the catalog subset.
func TestForSpecFiltering(t *testing.T) {
	_, api := multiSurface()

	mobile := mustGenerate(t, api, openapi.Spec{Prefix: "/mobile"})
	assertPaths(t, "mobile", mobile, []string{
		"/mobile/v1/investments",
		"/mobile/v1/investments/manage",
		"/mobile/ping",
	})
	// Prefix is segment-aware: "/mobile" must not match "/mobilepay".
	if _, ok := mobile.Paths["/mobilepay/charge"]; ok {
		t.Error("segment-aware prefix leaked /mobilepay under /mobile")
	}

	console := mustGenerate(t, api, openapi.Spec{Prefix: "/console"})
	assertPaths(t, "console", console, []string{"/console/v1/users", "/console/ping"})

	// Tag intersection: the "shared" tag selects routes across both surfaces.
	shared := mustGenerate(t, api, openapi.Spec{Specs: []string{"shared"}})
	assertPaths(t, "shared", shared, []string{"/mobile/ping", "/console/ping"})

	// Tighter nested prefix selects only the manage subtree.
	manage := mustGenerate(t, api, openapi.Spec{Prefix: "/mobile/v1/investments/manage"})
	assertPaths(t, "manage", manage, []string{"/mobile/v1/investments/manage"})

	// Catalog subset: the mobile doc only references errors used by its routes.
	if collectErrorCodes(mobile)[specConflict.Code] {
		t.Error("console-only conflict error leaked into the mobile document")
	}
}

func TestForSpecCatalogSubset(t *testing.T) {
	_, api := multiSurface()
	console := api.Manifest().ForSpec("/console", nil)
	// Only errors referenced by console routes should remain in the catalog.
	for _, e := range console.Catalog {
		if e.Code == specIDRequired.Code {
			t.Errorf("mobile-only error %q leaked into console catalog subset", e.Code)
		}
	}
}

// TestAutoRegisterDefault verifies nil Config.Specs serves a single document at
// /openapi.json.
func TestAutoRegisterDefault(t *testing.T) {
	tr := http.New(http.Config{
		Logger: specLogger(), Catalog: specCatalog, Fallbacks: specFallbacks,
	})
	openapi.New(tr, openapi.Config{}).Routes("/api/v1", func(r *openapi.Router) {
		users := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(422, specIDRequired),
		})
		users.Route("/users/:id").Get(openapi.Route{
			Handler: specNopID, Summary: "Fetch", Responses: openapi.Returns(),
		})
	})

	doc := fetchSpec(t, tr, "/openapi.json")
	if _, ok := doc.Paths["/api/v1/users/{id}"]; !ok {
		t.Errorf("expected documented route in default spec, got %v", keys(doc.Paths))
	}
}

// TestAutoRegisterEmptyOptOut verifies an empty (non-nil) slice registers no
// OpenAPI endpoints.
func TestAutoRegisterEmptyOptOut(t *testing.T) {
	tr, api := newSpecAPI([]openapi.Spec{})
	api.Routes("/api/v1", func(r *openapi.Router) {
		users := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(422, specIDRequired),
		})
		users.Route("/users/:id").Get(openapi.Route{
			Handler: specNopID, Summary: "Fetch", Responses: openapi.Returns(),
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", "/openapi.json", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected no /openapi.json (404), got %d", resp.StatusCode)
	}
}

// TestAutoRegisterMultiSurface verifies three documents are served at three
// paths, each carrying its own route set, and none contains the spec routes.
func TestAutoRegisterMultiSurface(t *testing.T) {
	tr, _ := multiSurface()

	mobile := fetchSpec(t, tr, "/mobile/openapi.json")
	assertPaths(t, "mobile(served)", mobile, []string{
		"/mobile/v1/investments",
		"/mobile/v1/investments/manage",
		"/mobile/ping",
	})
	if mobile.Info.Title != "Mobile API" {
		t.Errorf("expected per-spec title, got %q", mobile.Info.Title)
	}

	console := fetchSpec(t, tr, "/console/openapi.json")
	assertPaths(t, "console(served)", console, []string{"/console/v1/users", "/console/ping"})

	manage := fetchSpec(t, tr, "/mobile/v1/investments/manage/openapi.json")
	assertPaths(t, "manage(served)", manage, []string{"/mobile/v1/investments/manage"})

	// Spec endpoints attach on Fiber directly, so they never document themselves.
	for _, p := range []string{"/mobile/openapi.json", "/console/openapi.json"} {
		if _, ok := mobile.Paths[p]; ok {
			t.Errorf("spec endpoint %q appeared in a generated document", p)
		}
	}
}

// multiSurface mirrors a finkhoz-style deployment: distinct /mobile and
// /console surfaces, a shared-tag route on each, and a nested manage subtree.
func multiSurface() (*http.Transport, *openapi.Generator) {
	tr, api := func() (*http.Transport, *openapi.Generator) {
		tr := http.New(http.Config{
			Logger:    specLogger(),
			Catalog:   specCatalog,
			Fallbacks: specFallbacks,
		})
		return tr, openapi.New(tr, openapi.Config{
			Specs: []openapi.Spec{
				{Path: "/mobile/openapi.json", Prefix: "/mobile", Title: "Mobile API"},
				{Path: "/console/openapi.json", Prefix: "/console", Title: "Console API"},
				{
					Path:   "/mobile/v1/investments/manage/openapi.json",
					Prefix: "/mobile/v1/investments/manage",
					Title:  "Investments Manage",
				},
			},
		})
	}()

	api.Routes("/mobile", func(r *openapi.Router) {
		v1 := r.Group(openapi.GroupConfig{Prefix: "/v1", Specs: []string{"mobile"}})
		v1.Route("/investments").Get(openapi.Route{
			Handler:   specNop,
			Summary:   "List investments",
			Responses: openapi.Returns().Err(422, specIDRequired),
		})
		v1.Route("/investments/manage").Post(openapi.Route{
			Handler:   specNop,
			Summary:   "Manage an investment",
			Responses: openapi.Returns(),
			Requests: []openapi.ContentUnit{{
				Required:  true,
				Structure: &openapi.Schema{Type: "object"},
			}},
		})
		r.Route("/ping").Get(openapi.Route{
			Handler:   specNop,
			Summary:   "Mobile ping",
			Specs:     []string{"shared"},
			Responses: openapi.Returns(),
		})
	})

	api.Routes("/console", func(r *openapi.Router) {
		r.Route("/v1/users").Get(openapi.Route{
			Handler:   specNop,
			Summary:   "List users",
			Responses: openapi.Returns().Err(409, specConflict),
		})
		r.Route("/ping").Get(openapi.Route{
			Handler:   specNop,
			Summary:   "Console ping",
			Specs:     []string{"shared"},
			Responses: openapi.Returns(),
		})
	})

	api.Routes("/mobilepay", func(r *openapi.Router) {
		r.Route("/charge").Get(openapi.Route{
			Handler: specNop, Summary: "Charge", Responses: openapi.Returns(),
		})
	})

	return tr, api
}

func mustGenerate(t *testing.T, api *openapi.Generator, s openapi.Spec) *openapi.Document {
	t.Helper()
	doc, err := api.Document(s)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return doc
}

func fetchSpec(t *testing.T, tr *http.Transport, path string) *openapi.Document {
	t.Helper()
	resp, err := tr.Fiber().Test(httptest.NewRequest("GET", path, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s: expected 200, got %d", path, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("GET %s: unexpected content type %q", path, ct)
	}
	raw, _ := io.ReadAll(resp.Body)
	var doc openapi.Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("GET %s: served spec does not parse: %v", path, err)
	}
	return &doc
}

func assertPaths(t *testing.T, label string, doc *openapi.Document, want []string) {
	t.Helper()
	got := map[string]bool{}
	for p := range doc.Paths {
		got[p] = true
	}
	if len(got) != len(want) {
		t.Errorf("%s: expected %d paths %v, got %v", label, len(want), want, keys(doc.Paths))
	}
	for _, p := range want {
		if !got[p] {
			t.Errorf("%s: expected path %q, got %v", label, p, keys(doc.Paths))
		}
	}
}

// operationsOf returns every operation set on a path item, keyed by lowercase
// method.
func operationsOf(item *openapi.PathItem) map[string]*openapi.Operation {
	ops := map[string]*openapi.Operation{}
	add := func(name string, op *openapi.Operation) {
		if op != nil {
			ops[name] = op
		}
	}
	add("get", item.Get)
	add("put", item.Put)
	add("post", item.Post)
	add("delete", item.Delete)
	add("options", item.Options)
	add("head", item.Head)
	add("patch", item.Patch)
	add("trace", item.Trace)
	add("query", item.Query)
	for method, op := range item.AdditionalOperations {
		add(method, op)
	}
	return ops
}

// xquaErrors reads the x-xqua-errors extension from an in-memory response.
func xquaErrors(t *testing.T, resp *openapi.SpecResponse) []openapi.ErrorSpec {
	t.Helper()
	return responseErrors(resp)
}

// collectErrorCodes gathers every catalog error code surfaced via x-xqua-errors
// across an in-memory document.
func collectErrorCodes(doc *openapi.Document) map[string]bool {
	codes := map[string]bool{}
	for _, item := range doc.Paths {
		for _, op := range operationsOf(item) {
			for _, resp := range op.Responses {
				for _, e := range responseErrors(resp) {
					codes[e.Code] = true
				}
			}
		}
	}
	return codes
}

func responseErrors(resp *openapi.SpecResponse) []openapi.ErrorSpec {
	if resp == nil || resp.Extensions == nil {
		return nil
	}
	raw, ok := resp.Extensions["x-xqua-errors"]
	if !ok {
		return nil
	}
	specs, _ := raw.([]openapi.ErrorSpec)
	return specs
}

func findRoute(t *testing.T, m openapi.Manifest, method, path string) openapi.RouteSpec {
	t.Helper()
	for _, r := range m.Routes {
		if r.Method == method && r.Path == path {
			return r
		}
	}
	t.Fatalf("route %s %s not found", method, path)
	return openapi.RouteSpec{}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
