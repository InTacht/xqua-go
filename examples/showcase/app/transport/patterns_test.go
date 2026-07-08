package transport_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShowcasePatternsOpenAPISpec(t *testing.T) {
	tr := newSmokeTransport(t)

	resp, err := tr.Fiber().Test(httptest.NewRequest(http.MethodGet, "/demo/openapi.json", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /demo/openapi.json, got %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode openapi: %v", err)
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object in demo openapi document")
	}

	triggerPath := "/demo/patterns/trigger"
	triggerItem, ok := paths[triggerPath].(map[string]any)
	if !ok {
		t.Fatalf("expected path %q in demo openapi", triggerPath)
	}
	post, ok := triggerItem["post"].(map[string]any)
	if !ok {
		t.Fatalf("expected POST on %q", triggerPath)
	}
	assertPatternsSecurity(t, post)
	assertResponseHasErrorCode(t, post, "401", "11012")
	assertResponseHasErrorCode(t, post, "401", "11013")
	assertResponseHasErrorCode(t, post, "401", "11014")
	assertResponseHasErrorCode(t, post, "429", "11015")
	if post["responses"].(map[string]any)["409"] != nil {
		t.Error("patterns trigger must not document 409 — replay is HTTP 200")
	}

	putPath := "/demo/patterns/subscribers/{id}/data"
	putItem, ok := paths[putPath].(map[string]any)
	if !ok {
		t.Fatalf("expected path %q in demo openapi", putPath)
	}
	put, ok := putItem["put"].(map[string]any)
	if !ok {
		t.Fatalf("expected PUT on %q", putPath)
	}
	assertPatternsSecurity(t, put)
	assertResponseHasErrorCode(t, put, "429", "11015")

	reqBody, ok := put["requestBody"].(map[string]any)
	if !ok {
		t.Fatal("expected requestBody on patterns PUT")
	}
	content, ok := reqBody["content"].(map[string]any)
	if !ok {
		t.Fatal("expected requestBody.content on patterns PUT")
	}
	jsonBody, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatal("expected application/json request body on patterns PUT")
	}
	schema, ok := jsonBody["schema"].(map[string]any)
	if !ok {
		t.Fatal("expected request body schema on patterns PUT")
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok || props["metadata"] == nil {
		t.Fatalf("expected metadata property on patterns PUT schema, got %+v", schema)
	}

	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatal("expected components in demo openapi")
	}
	schemes, ok := components["securitySchemes"].(map[string]any)
	if !ok || schemes["PatternsBearer"] == nil {
		t.Fatal("expected PatternsBearer in components.securitySchemes")
	}
}

func TestShowcasePatternsAuthDistinct401(t *testing.T) {
	tr := newSmokeTransport(t)

	cases := []struct {
		name     string
		auth     string
		wantCode string
	}{
		{name: "missing credential", auth: "", wantCode: "11012"},
		{name: "expired token", auth: "Bearer expired-token", wantCode: "11013"},
		{name: "invalid token", auth: "Bearer invalid-token", wantCode: "11014"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/demo/patterns/trigger", strings.NewReader(`{"payload":"x"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "auth-"+tc.name)
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
			out := readEnvelope(t, resp.Body)
			if len(out.Errors) != 1 || out.Errors[0].Code != tc.wantCode {
				t.Fatalf("expected code %s, got %+v", tc.wantCode, out.Errors)
			}
		})
	}
}

func TestShowcasePatternsIdempotencyReplay200(t *testing.T) {
	tr := newSmokeTransport(t)

	body := `{"payload":"notify"}`
	makeReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/demo/patterns/trigger", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer good-token")
		req.Header.Set("Idempotency-Key", "showcase-idem-1")
		return req
	}

	resp1, err := tr.Fiber().Test(makeReq())
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", resp1.StatusCode)
	}
	first := readPatternsTrigger(t, resp1.Body)
	if first.Data.Replayed {
		t.Fatal("first request must not set replayed")
	}
	if first.Data.RunID != "run_showcase-idem-1" {
		t.Fatalf("unexpected run_id: %q", first.Data.RunID)
	}

	resp2, err := tr.Fiber().Test(makeReq())
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("duplicate request: expected 200, got %d", resp2.StatusCode)
	}
	second := readPatternsTrigger(t, resp2.Body)
	if !second.Data.Replayed {
		t.Fatal("duplicate request must set replayed=true")
	}
	if second.Data.RunID != first.Data.RunID {
		t.Fatalf("expected same run_id, got %q vs %q", second.Data.RunID, first.Data.RunID)
	}
}

func TestShowcasePatternsWrappedMetadataPUT(t *testing.T) {
	tr := newSmokeTransport(t)

	req := httptest.NewRequest(http.MethodPut, "/demo/patterns/subscribers/sub-42/data",
		strings.NewReader(`{"metadata":{"tier":"gold","plan":"pro"}}`))
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

	var out struct {
		Status string `json:"status"`
		Data   struct {
			ID       string            `json:"id"`
			Metadata map[string]string `json:"metadata"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "success" {
		t.Fatalf("expected success envelope, got %+v", out)
	}
	if out.Data.ID != "sub-42" {
		t.Fatalf("expected id sub-42, got %q", out.Data.ID)
	}
	if out.Data.Metadata["tier"] != "gold" || out.Data.Metadata["plan"] != "pro" {
		t.Fatalf("unexpected metadata: %+v", out.Data.Metadata)
	}
}

func TestShowcasePatternsRateLimit429(t *testing.T) {
	tr := newSmokeTransport(t)

	req := httptest.NewRequest(http.MethodPut, "/demo/patterns/subscribers/rate-limit/data",
		strings.NewReader(`{"metadata":{"a":"b"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer good-token")

	var saw200, saw429 bool
	for range 15 {
		resp, err := tr.Fiber().Test(req)
		if err != nil {
			t.Fatal(err)
		}
		switch resp.StatusCode {
		case http.StatusOK:
			saw200 = true
		case http.StatusTooManyRequests:
			saw429 = true
			out := readEnvelope(t, resp.Body)
			if len(out.Errors) != 1 || out.Errors[0].Code != "11015" {
				t.Fatalf("expected 11015, got %+v", out.Errors)
			}
			resp.Body.Close()
			return
		}
		resp.Body.Close()
	}
	if !saw200 {
		t.Fatal("expected at least one successful request before rate limit")
	}
	if !saw429 {
		t.Fatal("expected rate limit 429 within 15 requests")
	}
}

func TestShowcasePatternsOpenAPISpecMatchesRuntimeAuthCode(t *testing.T) {
	tr := newSmokeTransport(t)

	specResp, err := tr.Fiber().Test(httptest.NewRequest(http.MethodGet, "/demo/openapi.json", nil))
	if err != nil {
		t.Fatal(err)
	}
	specDoc := decodeOpenAPI(t, specResp.Body)
	specResp.Body.Close()

	docCodes := collectOpenAPIErrorCodes(specDoc)

	req := httptest.NewRequest(http.MethodPost, "/demo/patterns/trigger", strings.NewReader(`{"payload":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer expired-token")
	req.Header.Set("Idempotency-Key", "spec-runtime-showcase")
	resp, err := tr.Fiber().Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	out := readEnvelope(t, resp.Body)
	if len(out.Errors) != 1 {
		t.Fatalf("expected one runtime error, got %+v", out.Errors)
	}
	if !docCodes[out.Errors[0].Code] {
		t.Fatalf("runtime auth code %q not documented in /demo/openapi.json", out.Errors[0].Code)
	}
}

type patternsTriggerEnvelope struct {
	Status string `json:"status"`
	Data   struct {
		RunID    string `json:"run_id"`
		Replayed bool   `json:"replayed"`
	} `json:"data"`
}

func readPatternsTrigger(t *testing.T, body io.Reader) patternsTriggerEnvelope {
	t.Helper()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	var out patternsTriggerEnvelope
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "success" {
		t.Fatalf("expected success status, got %+v", out)
	}
	return out
}

func decodeOpenAPI(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func assertPatternsSecurity(t *testing.T, op map[string]any) {
	t.Helper()
	security, ok := op["security"].([]any)
	if !ok || len(security) == 0 {
		t.Fatal("expected PatternsBearer security requirement on operation")
	}
	first, ok := security[0].(map[string]any)
	if !ok || first["PatternsBearer"] == nil {
		t.Fatalf("expected PatternsBearer security, got %+v", security)
	}
}

func assertResponseHasErrorCode(t *testing.T, op map[string]any, status, wantCode string) {
	t.Helper()
	responses, ok := op["responses"].(map[string]any)
	if !ok {
		t.Fatalf("expected responses on operation")
	}
	resp, ok := responses[status].(map[string]any)
	if !ok {
		t.Fatalf("expected %s response on operation", status)
	}
	ext, ok := resp["x-xqua-errors"].([]any)
	if !ok {
		t.Fatalf("expected x-xqua-errors on %s response", status)
	}
	for _, item := range ext {
		spec, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if code, _ := spec["code"].(string); code == wantCode {
			return
		}
	}
	t.Fatalf("expected error code %s on %s response, got %+v", wantCode, status, ext)
}

func collectOpenAPIErrorCodes(doc map[string]any) map[string]bool {
	codes := map[string]bool{}
	paths, _ := doc["paths"].(map[string]any)
	for _, item := range paths {
		pathItem, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, op := range pathItem {
			operation, ok := op.(map[string]any)
			if !ok {
				continue
			}
			responses, ok := operation["responses"].(map[string]any)
			if !ok {
				continue
			}
			for _, resp := range responses {
				response, ok := resp.(map[string]any)
				if !ok {
					continue
				}
				ext, ok := response["x-xqua-errors"].([]any)
				if !ok {
					continue
				}
				for _, e := range ext {
					spec, ok := e.(map[string]any)
					if !ok {
						continue
					}
					if code, ok := spec["code"].(string); ok {
						codes[code] = true
					}
				}
			}
		}
	}
	return codes
}
