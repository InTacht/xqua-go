package validate

import (
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func ValidateComponents(
	components *spec.Components,
	version string,
	operationIDs map[string]string,
	securitySchemes map[string]*spec.SecurityScheme,
	componentParameters map[string]*spec.Parameter,
) []error {
	var errs []error
	errs = append(errs, ValidateComponentKeys("schemas", components.Schemas)...)
	errs = append(errs, ValidateComponentKeys("responses", components.Responses)...)
	errs = append(errs, ValidateComponentKeys("parameters", components.Parameters)...)
	errs = append(errs, ValidateComponentKeys("examples", components.Examples)...)
	errs = append(errs, ValidateComponentKeys("requestBodies", components.RequestBodies)...)
	errs = append(errs, ValidateComponentKeys("headers", components.Headers)...)
	errs = append(errs, ValidateComponentKeys("securitySchemes", components.SecuritySchemes)...)
	errs = append(errs, ValidateComponentKeys("links", components.Links)...)
	errs = append(errs, ValidateComponentKeys("callbacks", components.Callbacks)...)
	errs = append(errs, ValidateComponentKeys("pathItems", components.PathItems)...)
	errs = append(errs, ValidateComponentKeys("mediaTypes", components.MediaTypes)...)
	if reflect.IsOpenAPI30(version) && components.PathItems != nil {
		errs = append(errs, Errorf("components.pathItems requires OpenAPI 3.1.x or 3.2.0"))
	}
	if version != spec.Version320 && components.MediaTypes != nil {
		errs = append(errs, Errorf("components.mediaTypes requires OpenAPI 3.2.0"))
	}
	for name, schema := range components.Schemas {
		errs = append(errs, ValidateSchema("components.schemas."+name, schema, version, map[*spec.Schema]bool{})...)
	}
	for name, response := range components.Responses {
		errs = append(errs, ValidateResponse("components.responses."+name, response, version)...)
	}
	for name, parameter := range components.Parameters {
		errs = append(
			errs,
			ValidateParameters(
				"components.parameters."+name,
				[]*spec.Parameter{parameter},
				version,
				componentParameters,
			)...)
	}
	for name, example := range components.Examples {
		errs = append(errs, ValidateExample("components.examples."+name, example, version)...)
	}
	for name, body := range components.RequestBodies {
		errs = append(errs, ValidateRequestBody("components.requestBodies."+name, body, version)...)
	}
	for name, header := range components.Headers {
		errs = append(errs, ValidateHeader("components.headers."+name, header, version)...)
	}
	for name, scheme := range components.SecuritySchemes {
		errs = append(errs, ValidateSecurityScheme("components.securitySchemes."+name, scheme, version)...)
	}
	for name, link := range components.Links {
		errs = append(errs, ValidateLink("components.links."+name, link, version)...)
	}
	for name, callback := range components.Callbacks {
		errs = append(
			errs,
			ValidateCallback(
				"components.callbacks."+name,
				callback,
				version,
				operationIDs,
				securitySchemes,
				componentParameters,
			)...)
	}
	for name, pathItem := range components.PathItems {
		errs = append(
			errs,
			ValidatePathItemOperations(
				"components.pathItems."+name,
				pathItem,
				version,
				operationIDs,
				securitySchemes,
				componentParameters,
			)...)
	}
	for name, mediaType := range components.MediaTypes {
		errs = append(errs, ValidateMediaType("components.mediaTypes."+name, "", mediaType, version)...)
	}
	return errs
}

func ValidateComponentKeys[T any](kind string, values map[string]T) []error {
	var errs []error
	for key := range values {
		if !componentRe.MatchString(key) {
			errs = append(errs, Errorf("components.%s key %q must match %s", kind, key, componentRe.String()))
		}
	}
	return errs
}
