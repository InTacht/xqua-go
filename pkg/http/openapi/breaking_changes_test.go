package openapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/InTacht/xqua-go/pkg/errors"
	xhttp "github.com/InTacht/xqua-go/pkg/http"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
	"github.com/gofiber/fiber/v3"
)

var bcCatalog = errors.NewCatalog("breaking")

var bcFallbacks = xhttp.StandardErrors(bcCatalog)

var (
	bcMissingBearer = bcCatalog.Define(errors.Def{Kind: errors.KindUnauthorized, Code: "401001", Message: "missing bearer"})
	bcExpired       = bcCatalog.Define(errors.Def{Kind: errors.KindUnauthorized, Code: "401002", Message: "token expired"})
	bcInvalid       = bcCatalog.Define(errors.Def{Kind: errors.KindUnauthorized, Code: "401003", Message: "invalid token"})
	bcForbidden     = bcCatalog.Define(errors.Def{Kind: errors.KindForbidden, Code: "403001", Message: "forbidden"})
	bcRateLimited   = bcCatalog.Define(errors.Def{Kind: errors.KindRateLimit, Code: "429001", Message: "rate limited"})
	bcValidation    = bcCatalog.Define(errors.Def{Kind: errors.KindValidation, Code: "422001", Message: "validation failed"})
)

type bcTriggerIn struct {
	IdempotencyKey string `header:"Idempotency-Key" required:"true"`
	Payload        string `json:"payload" required:"true"`
}

type bcTriggerOut struct {
	openapi.Response
	Data struct {
		RunID    string `json:"run_id"`
		Replayed bool   `json:"replayed,omitempty"`
	} `json:"data"`
}

type bcReplaceDataIn struct {
	ID       string            `path:"id"`
	Metadata map[string]string `json:"metadata" required:"true"`
}

type bcReplaceDataOut struct {
	openapi.Response
	Data struct {
		ID       string            `json:"id"`
		Metadata map[string]string `json:"metadata"`
	} `json:"data"`
}

func newBreakingAPI() (*xhttp.Transport, *openapi.Generator) {
	tr := xhttp.New(xhttp.Config{
		Logger:        specLogger(),
		Catalog:       bcCatalog,
		Fallbacks:     bcFallbacks,
		DefaultStatus: 500,
		Version:       "1.0.0",
	})
	api := openapi.New(tr, openapi.Config{
		Specs: []openapi.Spec{},
		Schemes: map[string]openapi.Scheme{
			"PatternsBearer": openapi.BearerScheme(openapi.BearerOptions{
				Verify: func(_ context.Context, cred openapi.Credential) (openapi.Identity, error) {
					switch cred.Raw {
					case "expired-token":
						return nil, bcExpired
					case "invalid-token":
						return nil, bcInvalid
					case "good-token":
						return "bc-user", nil
					default:
						return nil, bcInvalid
					}
				},
			}),
		},
	})
	return tr, api
}

func registerBreakingRoutes(api *openapi.Generator, rateLimit func(fiber.Ctx, openapi.RouteContext) error) {
	var idempotency = map[string]string{}

	api.Routes("/v1", func(r *openapi.Router) {
		public := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().
				Err(401, bcMissingBearer, bcExpired, bcInvalid).
				Err(403, bcForbidden).
				Err(422, bcValidation).
				Err(429, bcRateLimited),
			Security: openapi.RequireSecurity("PatternsBearer"),
			AfterAuth: []openapi.Middleware{
				rateLimit,
			},
		})

		public.Route("/trigger").Post(openapi.Route{
			Handler: func(_ context.Context, in bcTriggerIn) (bcTriggerOut, error) {
				if runID, ok := idempotency[in.IdempotencyKey]; ok {
					var out bcTriggerOut
					out.Message = "duplicate idempotency key; returning existing run"
					out.Data.RunID = runID
					out.Data.Replayed = true
					return out, nil
				}
				runID := "run_" + in.IdempotencyKey
				idempotency[in.IdempotencyKey] = runID
				var out bcTriggerOut
				out.Message = "trigger accepted"
				out.Data.RunID = runID
				return out, nil
			},
			Summary:   "Trigger run (idempotent replay as 200)",
			Responses: openapi.Returns(),
		})

		public.Route("/subscribers/:id/data").Put(openapi.Route{
			Handler: func(_ context.Context, in bcReplaceDataIn) (bcReplaceDataOut, error) {
				var out bcReplaceDataOut
				out.Message = "subscriber data replaced"
				out.Data.ID = in.ID
				out.Data.Metadata = in.Metadata
				return out, nil
			},
			Summary:   "Replace subscriber data (wrapped metadata map)",
			Responses: openapi.Returns(),
		})
	})
}

func TestBreakingChangesOpenAPISpec(t *testing.T) {
	_, api := newBreakingAPI()
	registerBreakingRoutes(api, func(_ fiber.Ctx, _ openapi.RouteContext) error { return nil })

	doc, err := api.Document(openapi.Spec{Title: "Breaking API", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}

	trigger := doc.Paths["/v1/trigger"].Post
	if trigger == nil {
		t.Fatal("expected POST /v1/trigger")
	}
	if len(trigger.Security) != 1 || trigger.Security[0]["PatternsBearer"] == nil {
		t.Fatalf("expected PatternsBearer security on trigger, got %+v", trigger.Security)
	}
	if doc.Components.SecuritySchemes["PatternsBearer"] == nil {
		t.Fatal("expected PatternsBearer in components.securitySchemes")
	}

	// Group-declared 401/429 errors surface on operations via x-xqua-errors.
	unauthorized := trigger.Responses["401"]
	if unauthorized == nil {
		t.Fatal("expected 401 response on trigger")
	}
	codes401 := errorCodes(xquaErrors(t, unauthorized))
	for _, want := range []string{"401001", "401002", "401003"} {
		if !codes401[want] {
			t.Errorf("expected 401 response to document code %s, got %+v", want, codes401)
		}
	}
	rateLimited := trigger.Responses["429"]
	if rateLimited == nil {
		t.Fatal("expected 429 response inherited from group")
	}
	codes429 := errorCodes(xquaErrors(t, rateLimited))
	if !codes429["429001"] {
		t.Errorf("expected 429001 on trigger, got %+v", codes429)
	}
	if trigger.Responses["409"] != nil {
		t.Error("idempotent trigger must not declare 409 — replay is HTTP 200")
	}

	// Success response schema includes replayed + run_id under data.
	success := trigger.Responses["200"]
	if success == nil || success.Content["application/json"].Schema == nil {
		t.Fatal("expected 200 JSON response schema on trigger")
	}
	triggerOutSchema := resolveSchemaRef(t, doc, success.Content["application/json"].Schema.Ref)
	dataProp := triggerOutSchema.Properties["data"]
	if dataProp == nil {
		t.Fatal("expected data property on trigger success schema")
	}
	dataSchema := schemaOrRef(t, doc, dataProp)
	if dataSchema.Properties["run_id"] == nil {
		t.Error("expected run_id on trigger data schema")
	}
	if dataSchema.Properties["replayed"] == nil {
		t.Error("expected replayed on trigger data schema")
	}

	// Wrapped metadata request body on PUT.
	put := doc.Paths["/v1/subscribers/{id}/data"].Put
	if put == nil {
		t.Fatal("expected PUT /v1/subscribers/{id}/data")
	}
	if put.RequestBody == nil {
		t.Fatalf("expected request body on PUT, got params=%d", len(put.Parameters))
	}
	putInSchema := schemaOrRef(t, doc, put.RequestBody.Content["application/json"].Schema)
	if !slices.Contains(putInSchema.Required, "metadata") {
		t.Errorf("expected metadata in PUT request required fields, got %+v", putInSchema.Required)
	}
	metadata := putInSchema.Properties["metadata"]
	if metadata == nil {
		t.Fatal("expected metadata property on PUT request schema")
	}
	metadataSchema := schemaOrRef(t, doc, metadata)
	if metadataSchema.Type != "object" {
		t.Errorf("expected metadata object type, got %+v", metadataSchema.Type)
	}
	if metadataSchema.AdditionalProperties == nil {
		t.Error("expected metadata map to expose additionalProperties for string values")
	}

	// Manifest error specs match declared group contract.
	m := api.Manifest()
	route := findRoute(t, m, "POST", "/v1/trigger")
	codeSet := map[string]bool{}
	for _, e := range route.Errors {
		codeSet[e.Code] = true
	}
	for _, want := range []string{"401001", "401002", "401003", "429001", "422001", "403001"} {
		if !codeSet[want] {
			t.Errorf("manifest missing error code %s for POST /v1/trigger", want)
		}
	}
}

func TestBreakingChangesGuardDistinct401Runtime(t *testing.T) {
	tr, api := newBreakingAPI()
	registerBreakingRoutes(api, func(_ fiber.Ctx, _ openapi.RouteContext) error { return nil })

	cases := []struct {
		name     string
		auth     string
		wantCode string
	}{
		{name: "missing credential", auth: "", wantCode: "401001"},
		{name: "expired token", auth: "Bearer expired-token", wantCode: "401002"},
		{name: "invalid token", auth: "Bearer invalid-token", wantCode: "401003"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/trigger", strings.NewReader(`{"payload":"x"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "k1")
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			resp, err := tr.Fiber().Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", resp.StatusCode)
			}
			assertEnvelopeCode(t, resp.Body, tc.wantCode)
		})
	}
}

func TestBreakingChangesIdempotencyReplay200(t *testing.T) {
	tr, api := newBreakingAPI()
	registerBreakingRoutes(api, func(_ fiber.Ctx, _ openapi.RouteContext) error { return nil })

	body := `{"payload":"notify"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer good-token")
	req.Header.Set("Idempotency-Key", "idem-1")

	resp1, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", resp1.StatusCode)
	}
	first := readSuccessTrigger(t, resp1.Body)
	if first.Data.Replayed {
		t.Fatal("first request must not set replayed")
	}
	if first.Data.RunID != "run_idem-1" {
		t.Fatalf("unexpected run_id: %q", first.Data.RunID)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/trigger", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer good-token")
	req2.Header.Set("Idempotency-Key", "idem-1")
	resp2, err := tr.Fiber().Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("duplicate request: expected 200, got %d", resp2.StatusCode)
	}
	second := readSuccessTrigger(t, resp2.Body)
	if !second.Data.Replayed {
		t.Fatal("duplicate request must set replayed=true")
	}
	if second.Data.RunID != first.Data.RunID {
		t.Fatalf("expected same run_id, got %q vs %q", second.Data.RunID, first.Data.RunID)
	}
	if len(second.Errors) > 0 {
		t.Fatalf("replay must not return error envelope, got %+v", second.Errors)
	}
}

func TestBreakingChangesWrappedMetadataPUT(t *testing.T) {
	tr, api := newBreakingAPI()
	registerBreakingRoutes(api, func(_ fiber.Ctx, _ openapi.RouteContext) error { return nil })

	req := httptest.NewRequest(http.MethodPut, "/v1/subscribers/sub-1/data", strings.NewReader(`{"metadata":{"tier":"gold","plan":"pro"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer good-token")

	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out bcReplaceDataOut
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "success" {
		t.Fatalf("expected success envelope, got %+v", out)
	}
	if out.Data.ID != "sub-1" {
		t.Fatalf("expected id sub-1, got %q", out.Data.ID)
	}
	if out.Data.Metadata["tier"] != "gold" || out.Data.Metadata["plan"] != "pro" {
		t.Fatalf("unexpected metadata: %+v", out.Data.Metadata)
	}
}

func TestBreakingChangesAfterAuth429InSpecAndRuntime(t *testing.T) {
	tr, api := newBreakingAPI()
	var hits int
	registerBreakingRoutes(api, func(_ fiber.Ctx, _ openapi.RouteContext) error {
		hits++
		if hits > 1 {
			return bcRateLimited
		}
		return nil
	})

	doc, err := api.Document(openapi.Spec{Title: "Breaking API", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	put := doc.Paths["/v1/subscribers/{id}/data"].Put
	if put.Responses["429"] == nil {
		t.Fatal("expected 429 documented on PUT from group inheritance")
	}

	req := httptest.NewRequest(http.MethodPut, "/v1/subscribers/sub-1/data", strings.NewReader(`{"metadata":{"a":"b"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer good-token")

	for i := range 2 {
		resp, err := tr.Fiber().Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("first PUT: expected 200, got %d", resp.StatusCode)
			}
		} else {
			if resp.StatusCode != http.StatusTooManyRequests {
				t.Fatalf("second PUT: expected 429, got %d", resp.StatusCode)
			}
			assertEnvelopeCode(t, resp.Body, "429001")
		}
		resp.Body.Close()
	}
}

func TestBreakingChangesAfterAuthInheritedFromNestedGroup(t *testing.T) {
	tr := xhttp.New(xhttp.Config{
		Logger:        specLogger(),
		Catalog:       bcCatalog,
		Fallbacks:     bcFallbacks,
		DefaultStatus: 500,
	})
	api := openapi.New(tr, openapi.Config{Specs: []openapi.Spec{}})

	var parentRan, childRan bool
	api.Routes("/v1", func(r *openapi.Router) {
		parent := r.Group(openapi.GroupConfig{
			Responses: openapi.Returns().Err(429, bcRateLimited),
			AfterAuth: []openapi.Middleware{
				func(_ fiber.Ctx, _ openapi.RouteContext) error {
					parentRan = true
					return nil
				},
			},
		})
		child := parent.Group(openapi.GroupConfig{
			AfterAuth: []openapi.Middleware{
				func(_ fiber.Ctx, _ openapi.RouteContext) error {
					childRan = true
					return nil
				},
			},
		})
		child.Route("/ping").Get(openapi.Route{
			Handler: func(_ context.Context, _ struct{}) (ackOut, error) {
				return ackOut{Response: openapi.Response{Message: "pong"}}, nil
			},
			Summary: "Nested AfterAuth inheritance",
		})
	})

	resp, err := tr.Fiber().Test(httptest.NewRequest(http.MethodGet, "/v1/ping", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !parentRan || !childRan {
		t.Fatalf("expected both middleware stacks to run: parent=%v child=%v", parentRan, childRan)
	}
}

func errorCodes(specs []openapi.ErrorSpec) map[string]bool {
	out := map[string]bool{}
	for _, s := range specs {
		out[s.Code] = true
	}
	return out
}

func resolveSchemaRef(t *testing.T, doc *openapi.Document, ref string) *openapi.Schema {
	t.Helper()
	if ref == "" {
		t.Fatal("empty schema ref")
	}
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		t.Fatalf("unexpected ref %q", ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	schema := doc.Components.Schemas[name]
	if schema == nil {
		t.Fatalf("schema %q not found in components", name)
	}
	return schema
}

func schemaOrRef(t *testing.T, doc *openapi.Document, s *openapi.Schema) *openapi.Schema {
	t.Helper()
	if s == nil {
		t.Fatal("nil schema")
	}
	if s.Ref != "" {
		return resolveSchemaRef(t, doc, s.Ref)
	}
	return s
}

type successTriggerEnvelope struct {
	Status string `json:"status"`
	Data   struct {
		RunID    string `json:"run_id"`
		Replayed bool   `json:"replayed"`
	} `json:"data"`
	Errors []xhttp.ErrorDetail `json:"errors"`
}

func readSuccessTrigger(t *testing.T, body io.Reader) successTriggerEnvelope {
	t.Helper()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	var out successTriggerEnvelope
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "success" {
		t.Fatalf("expected success status, got %+v", out)
	}
	return out
}

func assertEnvelopeCode(t *testing.T, body io.Reader, wantCode string) {
	t.Helper()
	out := readEnvelope(t, body)
	if len(out.Errors) != 1 {
		t.Fatalf("expected one error, got %+v", out)
	}
	if out.Errors[0].Code != wantCode {
		t.Fatalf("expected code %s, got %s", wantCode, out.Errors[0].Code)
	}
}

// readEnvelope is defined in router_test.go (same package).

func TestBreakingChangesOpenAPISpecMatchesRuntimeAuthCodes(t *testing.T) {
	tr, api := newBreakingAPI()
	registerBreakingRoutes(api, func(_ fiber.Ctx, _ openapi.RouteContext) error { return nil })

	doc, err := api.Document(openapi.Spec{Title: "Breaking API", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	docCodes := collectErrorCodes(doc)

	req := httptest.NewRequest(http.MethodPost, "/v1/trigger", bytes.NewReader([]byte(`{"payload":"x"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer expired-token")
	req.Header.Set("Idempotency-Key", "spec-runtime")
	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var runtimeEnv envelopeOut
	if err := json.NewDecoder(resp.Body).Decode(&runtimeEnv); err != nil {
		t.Fatal(err)
	}
	if len(runtimeEnv.Errors) != 1 {
		t.Fatalf("expected one runtime error, got %+v", runtimeEnv)
	}
	runtimeCode := runtimeEnv.Errors[0].Code
	if !docCodes[runtimeCode] {
		t.Fatalf("runtime auth code %q not documented in OpenAPI spec: documented=%v", runtimeCode, docCodes)
	}
}

func generateBreakingChangesDoc(t *testing.T) *openapi.Document {
	t.Helper()
	_, api := newBreakingAPI()
	registerBreakingRoutes(api, func(_ fiber.Ctx, _ openapi.RouteContext) error { return nil })
	doc, err := api.Document(openapi.Spec{Title: "Breaking API", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestBreakingChangesGolden(t *testing.T) {
	doc := generateBreakingChangesDoc(t)
	got, err := openapi.MarshalJSON(doc)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "breaking-changes.golden.json", got)
}
