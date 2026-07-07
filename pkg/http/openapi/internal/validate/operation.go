package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func ValidateOperation(
	context string,
	op *spec.Operation,
	version string,
	operationIDs map[string]string,
	securitySchemes map[string]*spec.SecurityScheme,
	componentParameters map[string]*spec.Parameter,
) []error {
	var errs []error
	if op.OperationID != "" {
		if previous, exists := operationIDs[op.OperationID]; exists {
			errs = append(errs, Errorf("%s operationId %q duplicates %s", context, op.OperationID, previous))
		} else {
			operationIDs[op.OperationID] = context
		}
	} else {
		errs = append(errs, Warningf("%s.operationId is recommended", context))
	}
	if len(op.Tags) == 0 {
		errs = append(errs, Infof("%s.tags are missing", context))
	}
	if op.Summary == "" {
		errs = append(errs, Warningf("%s.summary is recommended", context))
	}
	if op.Description == "" {
		errs = append(errs, Warningf("%s.description is recommended", context))
	}
	if op.Deprecated {
		errs = append(errs, Warningf("%s is deprecated", context))
	}
	if len(op.Responses) == 0 {
		errs = append(errs, Errorf("%s responses is required", context))
	}
	for i := range op.Servers {
		errs = append(errs, ValidateServer(fmt.Sprintf("%s.servers[%d]", context, i), &op.Servers[i], version)...)
	}
	if op.ExternalDocs != nil && op.ExternalDocs.URL == "" {
		errs = append(errs, Errorf("%s.externalDocs.url is required", context))
	}
	if op.ExternalDocs != nil && op.ExternalDocs.URL != "" && !IsURIReference(op.ExternalDocs.URL) {
		errs = append(errs, Errorf("%s.externalDocs.url must be a URI", context))
	}
	errs = append(errs, ValidateParameters(context+".parameters", op.Parameters, version, componentParameters)...)
	if op.RequestBody != nil {
		errs = append(errs, ValidateRequestBody(context+".requestBody", op.RequestBody, version)...)
	}
	for code, response := range op.Responses {
		if code != "default" && !responseCodeRe.MatchString(code) {
			errs = append(
				errs,
				Errorf("%s.responses.%s must be default, a status code, or a status code range", context, code),
			)
		}
		errs = append(errs, ValidateResponse(context+".responses."+code, response, version)...)
	}
	for name, callback := range op.Callbacks {
		errs = append(
			errs,
			ValidateCallback(
				context+".callbacks."+name,
				callback,
				version,
				operationIDs,
				securitySchemes,
				componentParameters,
			)...)
	}
	errs = append(errs, ValidateSecurityRequirements(context+".security", op.Security, securitySchemes, version)...)
	return errs
}

//nolint:gocognit,gocyclo,cyclop,funlen // parameter validation intentionally aggregates many independent OpenAPI constraints.
func ValidateParameters(
	context string,
	params []*spec.Parameter,
	version string,
	componentParameters map[string]*spec.Parameter,
) []error {
	var errs []error
	seen := map[string]struct{}{}
	for i, param := range params {
		paramContext := fmt.Sprintf("%s[%d]", context, i)
		if param == nil {
			errs = append(errs, Errorf("%s is required", paramContext))
			continue
		}
		if param.Ref != "" {
			if HasParameterRefSiblings(param, version) {
				errs = append(errs, Errorf("%s must not define siblings with $ref", paramContext))
			}
			if resolved := ResolveParameterRef(param.Ref, componentParameters); resolved != nil {
				key := resolved.Name + "\x00" + resolved.In
				if _, exists := seen[key]; exists {
					errs = append(
						errs,
						Errorf("%s duplicates parameter %q in %q", paramContext, resolved.Name, resolved.In),
					)
				}
				seen[key] = struct{}{}
			}
			continue
		}
		if param.Deprecated {
			errs = append(errs, Warningf("%s is deprecated", paramContext))
		}
		if param.Summary != "" {
			errs = append(errs, Errorf("%s.summary is only allowed with $ref", paramContext))
		}
		if param.Name == "" {
			errs = append(errs, Errorf("%s.name is required", paramContext))
		}
		if param.In == "" {
			errs = append(errs, Errorf("%s.in is required", paramContext))
		} else if !IsValidParameterIn(param.In) {
			errs = append(
				errs,
				Errorf("%s.in must be one of query, querystring, header, path, or cookie", paramContext),
			)
		}
		if param.In == string(spec.ParameterInQueryString) && version != spec.Version320 {
			errs = append(errs, Errorf("%s querystring parameters require OpenAPI 3.2.0", paramContext))
		}
		if param.In == string(spec.ParameterInPath) && !param.Required {
			errs = append(errs, Errorf("%s path parameter must be required", paramContext))
		}
		if param.In == string(spec.ParameterInHeader) {
			lowerName := strings.ToLower(param.Name)
			if lowerName == "accept" || lowerName == "content-type" || lowerName == "authorization" {
				errs = append(errs,
					Errorf("%s.name %q is not allowed for header parameters", paramContext, param.Name))
			}
		}
		key := param.Name + "\x00" + param.In
		if _, exists := seen[key]; exists {
			errs = append(errs, Errorf("%s duplicates parameter %q in %q", paramContext, param.Name, param.In))
		}
		seen[key] = struct{}{}
		if param.Schema != nil && len(param.Content) > 0 {
			errs = append(errs, Errorf("%s schema and content are mutually exclusive", paramContext))
		}
		if param.Schema == nil && len(param.Content) == 0 {
			errs = append(errs, Errorf("%s must define schema or content", paramContext))
		}
		if len(param.Content) > 1 {
			errs = append(errs, Errorf("%s content must contain only one media type", paramContext))
		}
		if param.Example != nil && len(param.Examples) > 0 {
			errs = append(errs, Errorf("%s example and examples are mutually exclusive", paramContext))
		}
		if param.In == string(spec.ParameterInQueryString) {
			if param.Schema != nil {
				errs = append(errs, Errorf("%s querystring parameter must use content", paramContext))
			}
			if len(param.Content) == 0 {
				errs = append(errs, Errorf("%s querystring parameter content is required", paramContext))
			}
			if param.Style != "" || param.Explode != nil || param.AllowReserved || param.AllowEmptyValue {
				errs = append(
					errs,
					Errorf(
						"%s style, explode, allowReserved, and allowEmptyValue must not be used with querystring",
						paramContext,
					),
				)
			}
		}
		errs = append(errs, ValidateParameterSerialization(paramContext, param, version)...)
		errs = append(
			errs,
			ValidateSchema(paramContext+".schema", param.Schema, version, map[*spec.Schema]bool{})...)
		for mediaType, content := range param.Content {
			errs = append(errs, ValidateMediaType(paramContext+".content."+mediaType, mediaType, content, version)...)
		}
		for name, example := range param.Examples {
			errs = append(errs, ValidateExample(paramContext+".examples."+name, example, version)...)
		}
	}
	return errs
}

func ValidateQueryParameterMix(context string, params []*spec.Parameter) []error {
	var queryCount, querystringCount int
	for _, param := range params {
		if param == nil || param.Ref != "" {
			continue
		}
		switch param.In {
		case string(spec.ParameterInQuery):
			queryCount++
		case string(spec.ParameterInQueryString):
			querystringCount++
		}
	}
	if querystringCount > 1 {
		return []error{Errorf("%s must not define more than one querystring parameter", context)}
	}
	if querystringCount > 0 && queryCount > 0 {
		return []error{Errorf("%s must not mix query and querystring parameters", context)}
	}
	return nil
}

func ValidateRequestBody(context string, body *spec.RequestBody, version string) []error {
	var errs []error
	if body == nil {
		return nil
	}
	if body.Ref != "" {
		if BodyRefHasInvalidSiblings(body, version) {
			errs = append(errs, Errorf("%s must not define siblings with $ref", context))
		}
		return errs
	}
	if body.Summary != "" {
		errs = append(errs, Errorf("%s.summary is only allowed with $ref", context))
	}
	if len(body.Content) == 0 {
		errs = append(errs, Errorf("%s.content is required", context))
	}
	for mediaType, content := range body.Content {
		errs = append(errs, ValidateMediaType(context+".content."+mediaType, mediaType, &content, version)...)
	}
	return errs
}

func ValidateResponse(context string, response *spec.Response, version string) []error {
	var errs []error
	if response == nil {
		return []error{Errorf("%s is required", context)}
	}
	if response.Ref != "" {
		if ResponseRefHasInvalidSiblings(response, version) {
			errs = append(errs, Errorf("%s must not define siblings with $ref", context))
		}
		return errs
	}
	if version != spec.Version320 && response.Summary != "" {
		errs = append(errs, Errorf("%s.summary requires OpenAPI 3.2.0", context))
	}
	if version != spec.Version320 && response.Description == "" {
		errs = append(errs, Errorf("%s.description is required", context))
	}
	for name, header := range response.Headers {
		if strings.EqualFold(name, "Content-Type") {
			errs = append(errs, Warningf("%s.headers %q is ignored by the OpenAPI spec; omit it", context, name))
		}
		errs = append(errs, ValidateHeader(context+".headers."+name, header, version)...)
	}
	for mediaType, content := range response.Content {
		errs = append(errs, ValidateMediaType(context+".content."+mediaType, mediaType, &content, version)...)
	}
	for name, link := range response.Links {
		errs = append(errs, ValidateLink(context+".links."+name, link, version)...)
	}
	return errs
}

func ValidateHeader(context string, header *spec.Header, version string) []error {
	var errs []error
	if header == nil {
		return []error{Errorf("%s is required", context)}
	}
	if header.Ref != "" {
		if HeaderRefHasInvalidSiblings(header, version) {
			errs = append(errs, Errorf("%s must not define siblings with $ref", context))
		}
		return errs
	}
	if header.Deprecated {
		errs = append(errs, Warningf("%s is deprecated", context))
	}
	if header.Summary != "" {
		errs = append(errs, Errorf("%s.summary is only allowed with $ref", context))
	}
	if header.Schema != nil && len(header.Content) > 0 {
		errs = append(errs, Errorf("%s schema and content are mutually exclusive", context))
	}
	if header.Schema == nil && len(header.Content) == 0 {
		errs = append(errs, Errorf("%s must define schema or content", context))
	}
	if len(header.Content) > 1 {
		errs = append(errs, Errorf("%s content must contain only one media type", context))
	}
	if header.Example != nil && len(header.Examples) > 0 {
		errs = append(errs, Errorf("%s example and examples are mutually exclusive", context))
	}
	if header.AllowEmptyValue {
		errs = append(errs, Errorf("%s allowEmptyValue is not allowed for headers", context))
	}
	if header.AllowReserved && !IsOpenAPI32(version) {
		errs = append(errs, Errorf("%s allowReserved is not allowed for headers", context))
	}
	if header.Style != "" && header.Style != "simple" {
		errs = append(errs, Errorf("%s.style must be simple for headers", context))
	}
	errs = append(errs, ValidateSchema(context+".schema", header.Schema, version, map[*spec.Schema]bool{})...)
	for mediaType, content := range header.Content {
		errs = append(errs, ValidateMediaType(context+".content."+mediaType, mediaType, content, version)...)
	}
	for name, example := range header.Examples {
		errs = append(errs, ValidateExample(context+".examples."+name, example, version)...)
	}
	return errs
}

//nolint:gocognit // media type validation aggregates independent OpenAPI constraints.
func ValidateMediaType(context, mediaTypeName string, mediaType *spec.MediaType, version string) []error {
	var errs []error
	if mediaType == nil {
		return []error{Errorf("%s is required", context)}
	}
	if mediaType.Ref != "" {
		if MediaTypeRefHasInvalidSiblings(mediaType, version) {
			errs = append(errs, Errorf("%s must not define siblings with $ref", context))
		}
		return errs
	}
	if mediaType.Summary != "" || mediaType.Description != "" {
		errs = append(errs, Errorf("%s summary and description are only allowed with $ref", context))
	}
	if len(mediaType.Encoding) > 0 && !MediaTypeAllowsNamedEncoding(mediaTypeName) {
		errs = append(
			errs,
			Warningf(
				"%s.encoding is ignored unless media type is multipart or application/x-www-form-urlencoded",
				context,
			),
		)
	}
	if len(mediaType.PrefixEncoding) > 0 && !MediaTypeIsMultipart(mediaTypeName) {
		errs = append(errs, Errorf("%s.prefixEncoding requires multipart media type", context))
	}
	if mediaType.ItemEncoding != nil && !MediaTypeIsMultipart(mediaTypeName) {
		errs = append(errs, Errorf("%s.itemEncoding requires multipart media type", context))
	}
	if version != spec.Version320 {
		if mediaType.ItemSchema != nil {
			errs = append(errs, Errorf("%s.itemSchema requires OpenAPI 3.2.0", context))
		}
		if len(mediaType.PrefixEncoding) > 0 {
			errs = append(errs, Errorf("%s.prefixEncoding requires OpenAPI 3.2.0", context))
		}
		if mediaType.ItemEncoding != nil {
			errs = append(errs, Errorf("%s.itemEncoding requires OpenAPI 3.2.0", context))
		}
	}
	if mediaType.Example != nil && len(mediaType.Examples) > 0 {
		errs = append(errs, Errorf("%s example and examples are mutually exclusive", context))
	}
	if len(mediaType.Encoding) > 0 && (len(mediaType.PrefixEncoding) > 0 || mediaType.ItemEncoding != nil) {
		errs = append(errs, Errorf("%s encoding must not be used with prefixEncoding or itemEncoding", context))
	}
	if (len(mediaType.PrefixEncoding) > 0 || mediaType.ItemEncoding != nil) && mediaType.ItemSchema == nil &&
		!SchemaTypeIncludesArray(mediaType.Schema) {
		errs = append(
			errs,
			Errorf("%s prefixEncoding or itemEncoding requires itemSchema or an array schema", context),
		)
	}
	errs = append(errs, ValidateSchema(context+".schema", mediaType.Schema, version, map[*spec.Schema]bool{})...)
	errs = append(
		errs,
		ValidateSchema(context+".itemSchema", mediaType.ItemSchema, version, map[*spec.Schema]bool{})...)
	if mediaType.Schema != nil && mediaType.Schema.Ref == "" && len(mediaType.Encoding) > 0 {
		for name := range mediaType.Encoding {
			if _, exists := mediaType.Schema.Properties[name]; !exists {
				errs = append(errs, Errorf("%s.encoding.%s must correspond to a schema property", context, name))
			}
		}
	}
	for name, encoding := range mediaType.Encoding {
		errs = append(errs, ValidateEncoding(context+".encoding."+name, mediaTypeName, encoding, version)...)
	}
	for i, encoding := range mediaType.PrefixEncoding {
		errs = append(errs,
			ValidateEncoding(fmt.Sprintf("%s.prefixEncoding[%d]", context, i), mediaTypeName, encoding, version)...)
	}
	if mediaType.ItemEncoding != nil {
		errs = append(errs,
			ValidateEncoding(context+".itemEncoding", mediaTypeName, mediaType.ItemEncoding, version)...)
	}
	for name, example := range mediaType.Examples {
		errs = append(errs, ValidateExample(context+".examples."+name, example, version)...)
	}
	return errs
}

func ValidateEncoding(context, mediaTypeName string, encoding *spec.Encoding, version string) []error {
	var errs []error
	if encoding == nil {
		return []error{Errorf("%s is required", context)}
	}
	if (encoding.Style != "" || encoding.Explode != nil || encoding.AllowReserved) &&
		!MediaTypeAllowsNamedEncoding(mediaTypeName) {
		errs = append(
			errs,
			Warningf(
				"%s style, explode, and allowReserved are ignored unless media type is multipart or application/x-www-form-urlencoded",
				context,
			),
		)
	}
	if len(encoding.Headers) > 0 && !MediaTypeIsMultipart(mediaTypeName) {
		errs = append(errs, Errorf("%s.headers requires multipart media type", context))
	}
	if version != spec.Version320 {
		if len(encoding.PrefixEncoding) > 0 {
			errs = append(errs, Errorf("%s.prefixEncoding requires OpenAPI 3.2.0", context))
		}
		if encoding.ItemEncoding != nil {
			errs = append(errs, Errorf("%s.itemEncoding requires OpenAPI 3.2.0", context))
		}
	}
	for name, header := range encoding.Headers {
		if strings.EqualFold(name, "Content-Type") {
			errs = append(
				errs,
				Warningf(
					"%s.headers %q is described separately and is ignored; use encoding.contentType instead",
					context,
					name,
				),
			)
		}
		errs = append(errs, ValidateHeader(context+".headers."+name, header, version)...)
	}
	for name, nested := range encoding.Encoding {
		errs = append(errs, ValidateEncoding(context+".encoding."+name, mediaTypeName, nested, version)...)
	}
	for i, nested := range encoding.PrefixEncoding {
		errs = append(errs,
			ValidateEncoding(fmt.Sprintf("%s.prefixEncoding[%d]", context, i), mediaTypeName, nested, version)...)
	}
	if encoding.ItemEncoding != nil {
		errs = append(errs, ValidateEncoding(context+".itemEncoding", mediaTypeName, encoding.ItemEncoding, version)...)
	}
	return errs
}

func ValidateExample(context string, example *spec.Example, version string) []error {
	var errs []error
	if example == nil {
		return []error{Errorf("%s is required", context)}
	}
	if example.Ref != "" {
		if ExampleRefHasInvalidSiblings(example, version) {
			errs = append(errs, Errorf("%s must not define siblings with $ref", context))
		}
		return errs
	}
	if version != spec.Version320 && example.DataValue != nil {
		errs = append(errs, Errorf("%s.dataValue requires OpenAPI 3.2.0", context))
	}
	if version != spec.Version320 && example.SerializedValue != "" {
		errs = append(errs, Errorf("%s.serializedValue requires OpenAPI 3.2.0", context))
	}
	if example.Value != nil && example.ExternalValue != "" {
		errs = append(errs, Errorf("%s value and externalValue are mutually exclusive", context))
	}
	if example.ExternalValue != "" && !IsURIReference(example.ExternalValue) {
		errs = append(errs, Errorf("%s.externalValue must be a URI", context))
	}
	if example.DataValue != nil && example.Value != nil {
		errs = append(errs, Errorf("%s dataValue and value are mutually exclusive", context))
	}
	if example.SerializedValue != "" && (example.Value != nil || example.ExternalValue != "") {
		errs = append(
			errs,
			Errorf("%s serializedValue is mutually exclusive with value and externalValue", context),
		)
	}
	if HasSerializedExample(example) {
		errs = append(errs, Errorf("%s.serializedExample is not an OpenAPI field; use serializedValue", context))
	}
	return errs
}

func ValidateLink(context string, link *spec.Link, version string) []error {
	var errs []error
	if link == nil {
		return []error{Errorf("%s is required", context)}
	}
	if link.Ref != "" &&
		LinkRefHasInvalidSiblings(link, version) {
		return []error{Errorf("%s must not define siblings with $ref", context)}
	}
	if link.Ref == "" && link.Summary != "" {
		return []error{Errorf("%s.summary is only allowed with $ref", context)}
	}
	if link.OperationRef != "" && !IsURIReference(link.OperationRef) {
		errs = append(errs, Errorf("%s.operationRef must be a URI reference", context))
	}
	if link.OperationRef != "" && link.OperationID != "" {
		errs = append(errs, Errorf("%s operationRef and operationId are mutually exclusive", context))
	}
	if link.Ref == "" && link.OperationRef == "" && link.OperationID == "" {
		errs = append(errs, Errorf("%s must define operationRef or operationId", context))
	}
	return errs
}

func ValidateCallback(
	context string,
	callback *spec.Callback,
	version string,
	operationIDs map[string]string,
	securitySchemes map[string]*spec.SecurityScheme,
	componentParameters map[string]*spec.Parameter,
) []error {
	var errs []error
	if callback == nil {
		return []error{Errorf("%s is required", context)}
	}
	if callback.Ref != "" {
		if CallbackRefHasInvalidSiblings(callback, version) {
			errs = append(errs, Errorf("%s must not define siblings with $ref", context))
		}
		return errs
	}
	if len(callback.Expressions) == 0 {
		errs = append(errs, Errorf("%s must define at least one callback expression", context))
	}
	for expression, pathItem := range callback.Expressions {
		if pathItem == nil {
			errs = append(errs, Errorf("%s.%s is required", context, expression))
			continue
		}
		errs = append(
			errs,
			ValidatePathItemOperations(
				context+"."+expression,
				pathItem,
				version,
				operationIDs,
				securitySchemes,
				componentParameters,
			)...)
	}
	return errs
}

func ValidateSecurityRequirements(
	context string,
	requirements []spec.SecurityRequirement,
	schemes map[string]*spec.SecurityScheme,
	version string,
) []error {
	var errs []error
	for i, requirement := range requirements {
		for name, scopes := range requirement {
			scheme := schemes[name]
			if scheme == nil {
				if !SecurityRequirementMayUseURI(name, version) {
					errs = append(errs, Errorf("%s[%d] references undefined security scheme %q", context, i, name))
				}
				continue
			}
			if reflect.IsOpenAPI30(version) && scheme.Type != "oauth2" && scheme.Type != "openIdConnect" &&
				len(scopes) > 0 {
				errs = append(
					errs,
					Errorf("%s[%d].%s scopes are only allowed for oauth2 or openIdConnect", context, i, name),
				)
			}
		}
	}
	return errs
}

//nolint:gocognit // security scheme validation aggregates many independent spec constraints across multiple types.
func ValidateSecurityScheme(context string, scheme *spec.SecurityScheme, version string) []error {
	var errs []error
	if scheme == nil {
		return []error{Errorf("%s is required", context)}
	}
	if scheme.Ref != "" {
		if SecuritySchemeRefHasInvalidSiblings(scheme, version) {
			errs = append(errs, Errorf("%s must not define siblings with $ref", context))
		}
		return errs
	}
	if scheme.Summary != "" {
		errs = append(errs, Errorf("%s.summary is only allowed with $ref", context))
	}
	if !slices.Contains([]string{"apiKey", "http", "mutualTLS", "oauth2", "openIdConnect"}, scheme.Type) {
		errs = append(
			errs,
			Errorf("%s.type must be one of apiKey, http, mutualTLS, oauth2, or openIdConnect", context),
		)
	}
	if scheme.Type == "mutualTLS" && reflect.IsOpenAPI30(version) {
		errs = append(errs, Errorf("%s.type mutualTLS requires OpenAPI 3.1.x or 3.2.0", context))
	}
	if version != spec.Version320 &&
		(scheme.OAuth2MetadataURL != "" || scheme.Deprecated || ExtraHas(scheme.Extra, "oauth2MetadataUrl", "deprecated")) {
		errs = append(errs, Errorf("%s oauth2MetadataUrl and deprecated require OpenAPI 3.2.0", context))
	}
	if scheme.Deprecated && version == spec.Version320 {
		errs = append(errs, Warningf("%s is deprecated", context))
	}
	metadataURL, metadataURLPresent := SecuritySchemeOAuth2MetadataURL(scheme)
	if metadataURLPresent {
		if scheme.Type != "oauth2" {
			errs = append(errs, Errorf("%s.oauth2MetadataUrl is only allowed for oauth2 security schemes", context))
		}
		if !IsHTTPSURI(metadataURL) {
			errs = append(errs, Errorf("%s.oauth2MetadataUrl must be an HTTPS URI", context))
		}
	}
	switch scheme.Type {
	case "apiKey":
		if scheme.Name == "" {
			errs = append(errs, Errorf("%s.name is required for apiKey", context))
		}
		if !slices.Contains(
			[]string{
				string(spec.SecuritySchemeAPIKeyInQuery),
				string(spec.SecuritySchemeAPIKeyInHeader),
				string(spec.SecuritySchemeAPIKeyInCookie),
			},
			string(scheme.In),
		) {
			errs = append(errs, Errorf("%s.in must be query, header, or cookie for apiKey", context))
		}
	case "http":
		if scheme.Scheme == "" {
			errs = append(errs, Errorf("%s.scheme is required for http", context))
		}
	case "oauth2":
		if scheme.Flows == nil {
			errs = append(errs, Errorf("%s.flows is required for oauth2", context))
		} else {
			errs = append(errs, ValidateOAuthFlows(context+".flows", scheme.Flows, version)...)
		}
	case "openIdConnect":
		if scheme.OpenIDConnectURL == "" {
			errs = append(errs, Errorf("%s.openIdConnectUrl is required for openIdConnect", context))
		} else if !IsHTTPSURI(scheme.OpenIDConnectURL) {
			errs = append(errs, Errorf("%s.openIdConnectUrl must be an HTTPS URI without a fragment", context))
		}
	}
	return errs
}

func ValidateOAuthFlows(context string, flows *spec.OAuthFlows, version string) []error {
	var errs []error
	if version != spec.Version320 &&
		(flows.DeviceAuthorization != nil || ExtraHas(flows.Extra, "deviceAuthorization")) {
		errs = append(errs, Errorf("%s.deviceAuthorization requires OpenAPI 3.2.0", context))
	}
	if flows.Implicit != nil {
		errs = append(errs, ValidateOAuthFlow(context+".implicit", flows.Implicit, version, true, false)...)
	}
	if flows.Password != nil {
		errs = append(errs, ValidateOAuthFlow(context+".password", flows.Password, version, false, true)...)
	}
	if flows.ClientCredentials != nil {
		errs = append(
			errs,
			ValidateOAuthFlow(context+".clientCredentials", flows.ClientCredentials, version, false, true)...)
	}
	if flows.AuthorizationCode != nil {
		errs = append(
			errs,
			ValidateOAuthFlow(context+".authorizationCode", flows.AuthorizationCode, version, true, true)...)
	}
	if flows.DeviceAuthorization != nil {
		errs = append(
			errs,
			ValidateOAuthFlow(context+".deviceAuthorization", flows.DeviceAuthorization, version, false, true)...)
	}
	if flows.Implicit == nil && flows.Password == nil && flows.ClientCredentials == nil &&
		flows.AuthorizationCode == nil &&
		flows.DeviceAuthorization == nil &&
		!ExtraHas(flows.Extra, "deviceAuthorization") {
		errs = append(errs, Errorf("%s must define at least one OAuth flow", context))
	}
	return errs
}

func SecuritySchemeOAuth2MetadataURL(scheme *spec.SecurityScheme) (string, bool) {
	if scheme.OAuth2MetadataURL != "" {
		return scheme.OAuth2MetadataURL, true
	}
	if raw, ok := scheme.Extra["oauth2MetadataUrl"]; ok {
		value, _ := raw.(string)
		return value, true
	}
	return "", false
}

func ValidateOAuthFlow(
	context string,
	flow *spec.OAuthFlow,
	version string,
	needsAuthorizationURL, needsTokenURL bool,
) []error {
	var errs []error
	if needsAuthorizationURL && flow.AuthorizationURL == "" {
		errs = append(errs, Errorf("%s.authorizationUrl is required", context))
	} else if needsAuthorizationURL && flow.AuthorizationURL != "" && !IsURIReference(flow.AuthorizationURL) {
		errs = append(errs, Errorf("%s.authorizationUrl must be a URI", context))
	}
	if strings.HasSuffix(context, ".deviceAuthorization") && flow.DeviceAuthorizationURL == "" {
		errs = append(errs, Errorf("%s.deviceAuthorizationUrl is required", context))
	} else if strings.HasSuffix(context, ".deviceAuthorization") && flow.DeviceAuthorizationURL != "" &&
		!IsURIReference(flow.DeviceAuthorizationURL) {
		errs = append(errs, Errorf("%s.deviceAuthorizationUrl must be a URI", context))
	}
	if needsTokenURL && flow.TokenURL == "" {
		errs = append(errs, Errorf("%s.tokenUrl is required", context))
	} else if needsTokenURL && flow.TokenURL != "" && !IsURIReference(flow.TokenURL) {
		errs = append(errs, Errorf("%s.tokenUrl must be a URI", context))
	}
	if flow.Scopes == nil {
		errs = append(errs, Errorf("%s.scopes is required", context))
	}
	if version != spec.Version320 &&
		(flow.DeviceAuthorizationURL != "" || ExtraHas(flow.Extra, "deviceAuthorizationUrl")) {
		errs = append(errs, Errorf("%s.deviceAuthorizationUrl requires OpenAPI 3.2.0", context))
	}
	if flow.RefreshURL != nil && !IsURIReference(*flow.RefreshURL) {
		errs = append(errs, Errorf("%s.refreshUrl must be a URI", context))
	}
	return errs
}
