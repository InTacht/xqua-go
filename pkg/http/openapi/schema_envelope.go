package openapi

import "github.com/InTacht/xqua-go/pkg/http/openapi/spec"

// envelopeComponents models the RES envelope wire contract once, as OpenAPI
// component schemas. The shapes mirror this package's envelope rendering
// (EnvelopeVersion): a success envelope with free-form data plus optional
// pagination/cursor metadata, and an error envelope carrying catalog error
// details. Correlation IDs (request_id / client_request_id) appear on both.
func envelopeComponents() map[string]*spec.Schema {
	correlation := func(props map[string]*spec.Schema) map[string]*spec.Schema {
		props["request_id"] = &spec.Schema{
			Type:        "string",
			Description: "Server request ID (X-Request-Id).",
		}
		props["client_request_id"] = &spec.Schema{
			Type:        "string",
			Description: "Caller-supplied correlation ID (X-Client-Request-Id), echoed when valid.",
		}
		return props
	}

	return map[string]*spec.Schema{
		"Envelope": {
			Type:        "object",
			Description: "Success envelope.",
			Required:    []string{"status"},
			Properties: correlation(map[string]*spec.Schema{
				"status":  {Type: "string", Const: "success"},
				"message": {Type: "string"},
				"data": {
					Type:                 "object",
					AdditionalProperties: true,
					Description: "Route-specific payload. Routes may document it via " +
						"Route.Responses; otherwise it is an open object.",
				},
				"cursor":     schemaRef("Cursor"),
				"pagination": schemaRef("Pagination"),
			}),
		},
		"ErrorEnvelope": {
			Type:        "object",
			Description: "Error envelope. Errors are public catalog entries only.",
			Required:    []string{"status", "errors"},
			Properties: correlation(map[string]*spec.Schema{
				"status":  {Type: "string", Const: "error"},
				"message": {Type: "string"},
				"errors": {
					Type:  "array",
					Items: schemaRef("ErrorDetail"),
				},
			}),
		},
		"ErrorDetail": {
			Type:        "object",
			Description: "One public catalog error as rendered on the wire.",
			Required:    []string{"kind", "code", "message"},
			Properties: map[string]*spec.Schema{
				"kind":    {Type: "string", Description: "Semantic category (validation, not_found, …)."},
				"code":    {Type: "string", Description: "Catalog code, unique within the service catalog."},
				"message": {Type: "string"},
				"source":  {Type: "string", Description: "Field or input that caused the error."},
				"cause":   {Type: "string", Description: "Underlying cause, when exposed."},
			},
		},
		"Pagination": {
			Type:        "object",
			Description: "Page-based list metadata.",
			Required:    []string{"first", "last"},
			Properties: map[string]*spec.Schema{
				"total_count": {Type: "integer"},
				"total_pages": {Type: "integer"},
				"max_page":    {Type: "integer"},
				"page":        {Type: "integer"},
				"size":        {Type: "integer"},
				"first":       {Type: "boolean"},
				"last":        {Type: "boolean"},
			},
		},
		"Cursor": {
			Type:        "object",
			Description: "Cursor-based list metadata.",
			Properties: map[string]*spec.Schema{
				"next":     {Type: "string"},
				"previous": {Type: "string"},
			},
		},
	}
}
