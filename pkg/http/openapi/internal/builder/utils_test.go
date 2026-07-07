package builder

import (
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func TestSetOperation(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		version string
		wantErr bool
	}{
		{"GET", http.MethodGet, spec.Version304, false},
		{"POST", http.MethodPost, spec.Version304, false},
		{"PUT", http.MethodPut, spec.Version304, false},
		{"DELETE", http.MethodDelete, spec.Version304, false},
		{"PATCH", http.MethodPatch, spec.Version304, false},
		{"HEAD", http.MethodHead, spec.Version304, false},
		{"OPTIONS", http.MethodOptions, spec.Version304, false},
		{"TRACE", http.MethodTrace, spec.Version304, false},
		{"QUERY 3.2.0", "QUERY", spec.Version320, false},
		{
			"QUERY 3.1.2",
			"QUERY",
			spec.Version312,
			false,
		}, // QUERY is handled as a standard method in SetOperation but it's not in net/http.
		{"Custom method 3.2.0", "CUSTOM", spec.Version320, false},
		{"Custom method 3.0.4", "CUSTOM", spec.Version304, true},
		{"Duplicate GET", http.MethodGet, spec.Version304, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &spec.PathItem{}
			op := &spec.Operation{}
			if tt.name == "Duplicate GET" {
				item.Get = &spec.Operation{}
			}

			err := SetOperation(item, tt.method, op, tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContentType(t *testing.T) {
	type formBody struct {
		Name string `form:"name"`
	}
	type fileBody struct {
		File *multipart.FileHeader `form:"file"`
	}

	assert.Equal(t, "application/json", ContentType(nil))
	assert.Equal(t, "application/json", ContentType(&spec.ContentUnit{}))
	assert.Equal(t, "application/xml", ContentType(&spec.ContentUnit{ContentType: "application/xml"}))
	assert.Equal(t, "application/x-www-form-urlencoded", ContentType(&spec.ContentUnit{Structure: formBody{}}))
	assert.Equal(t, "multipart/form-data", ContentType(&spec.ContentUnit{Structure: fileBody{}}))
	// explicit ContentType always wins over struct tag inference
	assert.Equal(t, "application/json", ContentType(&spec.ContentUnit{
		ContentType: "application/json",
		Structure:   formBody{},
	}))
}

func TestResponseDescription(t *testing.T) {
	assert.Equal(t, "OK", ResponseDescription(&spec.ContentUnit{HTTPStatus: http.StatusOK}))
	assert.Equal(t, "Custom", ResponseDescription(&spec.ContentUnit{Description: "Custom"}))
	assert.Equal(t, "Default response", ResponseDescription(&spec.ContentUnit{IsDefault: true}))
	assert.Equal(t, "HTTP 999 response", ResponseDescription(&spec.ContentUnit{HTTPStatus: 999}))
}

func TestOneOf(t *testing.T) {
	v := OneOf(1, "two")
	ov, ok := v.(oneOfValue)
	assert.True(t, ok)
	assert.Equal(t, []any{1, "two"}, ov.GetValues())
}

func TestMergeResponses(t *testing.T) {
	responses := []*spec.ContentUnit{
		{HTTPStatus: 200, ContentType: "application/json", Structure: 1},
		{HTTPStatus: 200, ContentType: "application/json", Structure: "two"},
		{HTTPStatus: 400, ContentType: "application/json", Structure: "err"},
	}

	merged := MergeResponses(responses)
	assert.Len(t, merged, 2)
	assert.Equal(t, 200, merged[0].HTTPStatus)
	assert.Equal(t, 400, merged[1].HTTPStatus)

	ov, ok := merged[0].Structure.(oneOfValue)
	assert.True(t, ok)
	assert.Equal(t, []any{1, "two"}, ov.GetValues())
}
