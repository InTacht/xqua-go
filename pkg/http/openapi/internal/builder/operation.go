package builder

import (
	"strconv"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/validate"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func (b *Builder) AddRequest(op *spec.Operation, cu *spec.ContentUnit) error {
	ct := ContentType(cu)
	params, body, err := b.Reflector.RequestParts(cu.Structure, ct)
	if err != nil {
		return err
	}
	op.Parameters = append(op.Parameters, params...)

	if body == nil {
		isDefaultJSON := ct == "application/json" || cu.ContentType == ""
		if isDefaultJSON && cu.Format == "" && cu.Example == nil && len(cu.Examples) == 0 {
			return nil
		}
	}
	b.Config.Logger.Debug("building request body", "contentType", ct)

	if op.RequestBody == nil {
		op.RequestBody = &spec.RequestBody{Content: map[string]spec.MediaType{}}
	}
	if cu.Description != "" {
		op.RequestBody.Description = cu.Description
	}
	if cu.Required {
		op.RequestBody.Required = true
	}
	if body == nil && cu.Structure == nil {
		body = &spec.Schema{Type: "string"}
	}
	if body != nil && cu.Format != "" {
		body.Format = cu.Format
	}
	mt := spec.MediaType{Schema: body}
	ApplyContentExamples(&mt, cu)
	if len(cu.Encoding) > 0 {
		mt.Encoding = map[string]*spec.Encoding{}
		for prop, enc := range cu.Encoding {
			mt.Encoding[prop] = &spec.Encoding{ContentType: enc}
		}
	}
	op.RequestBody.Content[ct] = mt
	return nil
}

func (b *Builder) AddResponse(op *spec.Operation, cu *spec.ContentUnit) error {
	key := strconv.Itoa(cu.HTTPStatus)
	if cu.IsDefault {
		key = "default"
	} else if cu.HTTPStatus == 0 {
		return validate.Errorf("HTTP status is required unless ContentDefault is set")
	}
	b.Config.Logger.Debug("building response body", "status", key)

	response := op.Responses[key]
	if response == nil {
		response = &spec.Response{Description: ResponseDescription(cu)}
		op.Responses[key] = response
	}
	if cu.Summary != "" && b.Config.OpenAPIVersion == spec.Version320 {
		response.Summary = cu.Summary
	}
	if response.Content == nil {
		response.Content = map[string]spec.MediaType{}
	}

	ct := ContentType(cu)
	if cu.Structure != nil || cu.ContentType != "" || cu.Example != nil || len(cu.Examples) > 0 {
		schema, err := b.Reflector.SchemaForValue(cu.Structure, reflect.SchemaUseComponent)
		if err != nil {
			return err
		}
		if schema == nil && cu.ContentType != "" {
			schema = &spec.Schema{Type: "string"}
		}
		if schema != nil && cu.Format != "" {
			schema.Format = cu.Format
		}
		mt := spec.MediaType{
			Schema: schema,
		}
		ApplyContentExamples(&mt, cu)
		response.Content[ct] = mt
	}
	return nil
}

func ApplyContentExamples(mediaType *spec.MediaType, cu *spec.ContentUnit) {
	if cu.Example != nil {
		mediaType.Example = cu.Example
	}
	if len(cu.Examples) > 0 {
		mediaType.Examples = cu.Examples
	}
}
