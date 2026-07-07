package validate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func TestValidate_ServerNameUniqueness(t *testing.T) {
	doc := &spec.Document{
		OpenAPI: spec.Version320,
		Info:    spec.Info{Title: "Test", Version: "1.0.0"},
		Servers: []spec.Server{
			{URL: "https://v1.example.com", Name: "prod"},
			{URL: "https://v2.example.com", Name: "prod"},
		},
		Paths: map[string]*spec.PathItem{},
	}
	errs := validate.ValidateDocument(doc, spec.Version320)
	assert.NotEmpty(t, errs)
	assertHasError(t, errs, "duplicates servers[0].name")
}

func TestValidate_ExclusiveBoundaryTypes(t *testing.T) {
	t.Run("OpenAPI 3.0.4 - boolean required", func(t *testing.T) {
		schema := &spec.Schema{
			ExclusiveMaximum: 100.0,
		}
		errs := validate.ValidateSchema("schema", schema, spec.Version304, map[*spec.Schema]bool{})
		assert.NotEmpty(t, errs)
		assertHasError(t, errs, "must be a boolean in OpenAPI 3.0.x")
	})

	t.Run("OpenAPI 3.1.0 - number required", func(t *testing.T) {
		schema := &spec.Schema{
			ExclusiveMaximum: true,
		}
		errs := validate.ValidateSchema("schema", schema, spec.Version310, map[*spec.Schema]bool{})
		assert.NotEmpty(t, errs)
		assertHasError(t, errs, "must be a number in OpenAPI 3.1.x or 3.2.0")
	})

	t.Run("OpenAPI 3.2.0 - number valid", func(t *testing.T) {
		schema := &spec.Schema{
			ExclusiveMaximum: 100.0,
		}
		errs := validate.ValidateSchema("schema", schema, spec.Version320, map[*spec.Schema]bool{})
		assertNoStrictErrors(t, errs)
	})
}

func TestValidate_320Fields_NewNodeTypeAndDefaultMapping(t *testing.T) {
	t.Run("Discriminator DefaultMapping", func(t *testing.T) {
		schema := &spec.Schema{
			Discriminator: &spec.Discriminator{
				PropertyName:   "type",
				DefaultMapping: "DefaultSchema",
			},
		}
		errs := validate.ValidateSchema("schema", schema, spec.Version312, map[*spec.Schema]bool{})
		assert.NotEmpty(t, errs)
		assertHasError(t, errs, "requires OpenAPI 3.2.0")
	})

	t.Run("XML NodeType", func(t *testing.T) {
		schema := &spec.Schema{
			XML: &spec.XML{
				NodeType: "element",
			},
		}
		errs := validate.ValidateSchema("schema", schema, spec.Version312, map[*spec.Schema]bool{})
		assert.NotEmpty(t, errs)
		assertHasError(t, errs, "requires OpenAPI 3.2.0")
	})
}

func TestValidate_ForbiddenHeaderNames(t *testing.T) {
	t.Run("Parameter headers", func(t *testing.T) {
		op := &spec.Operation{
			Responses: map[string]*spec.Response{"200": {Description: "OK"}},
			Parameters: []*spec.Parameter{
				{Name: "Authorization", In: "header", Schema: &spec.Schema{Type: "string"}},
			},
		}
		errs := validate.ValidateOperation("op", op, spec.Version320, map[string]string{}, nil, nil)
		assert.NotEmpty(t, errs)
		assertHasError(t, errs, "not allowed for header parameters")
	})

	t.Run("Response headers", func(t *testing.T) {
		resp := &spec.Response{
			Description: "OK",
			Headers: map[string]*spec.Header{
				"Content-Type": {Schema: &spec.Schema{Type: "string"}},
			},
		}
		errs := validate.ValidateResponse("resp", resp, spec.Version320)
		assert.NotEmpty(t, errs)
		assertHasWarning(t, errs, "is ignored by the OpenAPI spec")
	})
}

func TestValidate_DiscriminatorUsage(t *testing.T) {
	schema := &spec.Schema{
		Discriminator: &spec.Discriminator{PropertyName: "type"},
		Type:          "object",
	}
	errs := validate.ValidateSchema("schema", schema, spec.Version320, map[*spec.Schema]bool{})
	assert.NotEmpty(t, errs)
	assertHasError(t, errs, "only allowed with anyOf, oneOf, or allOf")
}

func TestValidate_EncodingContext(t *testing.T) {
	mediaType := &spec.MediaType{
		Schema: &spec.Schema{Type: "object"},
		Encoding: map[string]*spec.Encoding{
			"prop": {Style: "form"},
		},
	}
	errs := validate.ValidateMediaType("mt", "application/json", mediaType, spec.Version320)
	assert.NotEmpty(t, errs)
	assertHasError(t, errs, "ignored unless media type is multipart or application/x-www-form-urlencoded")
}

func TestOptions_320NewFields(t *testing.T) {
	// Using internal package because we can't import spec from validate_test (cyclic)
	// Actually we are in validate_test, so we can't import spec.
	// But we can check if the structs are correctly populated if we had a builder.
	// Since we are in internal/validate, let's just test the document structure.
	doc := &spec.Document{
		OpenAPI: spec.Version320,
		Info:    spec.Info{Title: "Test", Version: "1.0.0"},
		Servers: []spec.Server{
			{URL: "https://example.com", Name: "prod"},
		},
		Paths: map[string]*spec.PathItem{
			"/test": {
				Get: &spec.Operation{
					Summary: "Test Summary",
					RequestBody: &spec.RequestBody{
						Description: "Request Description",
						Content: map[string]spec.MediaType{
							"application/json": {
								Schema: &spec.Schema{Type: "string"},
							},
						},
					},
					Responses: map[string]*spec.Response{
						"200": {
							Summary:     "Response Summary",
							Description: "OK",
						},
					},
				},
			},
		},
	}
	errs := validate.ValidateDocument(doc, spec.Version320)
	assertNoStrictErrors(t, errs)
}
