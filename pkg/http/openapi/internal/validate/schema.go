package validate

import (
	"fmt"
	"slices"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

//nolint:gocognit,funlen // recursive schema validation must cover many keyword branches.
func ValidateSchema(context string, schema *spec.Schema, version string, visited map[*spec.Schema]bool) []error {
	var errs []error
	if schema == nil {
		return nil
	}
	if visited[schema] {
		return nil
	}
	visited[schema] = true
	if schema.Deprecated {
		errs = append(errs, Warningf("%s is deprecated", context))
	}
	if (IsOpenAPI31(version) || IsOpenAPI32(version)) && schema.Example != nil {
		errs = append(errs,
			Warningf("%s.example is deprecated in OpenAPI 3.1.x and 3.2.0; use examples instead", context))
	}
	if reflect.IsOpenAPI30(version) {
		if schema.Ref != "" && HasSchemaRefSiblings(schema) {
			errs = append(errs, Errorf("%s must not define siblings with $ref in OpenAPI 3.0.x", context))
		}
		if schema.ReadOnly && schema.WriteOnly {
			errs = append(errs, Errorf("%s must not be both readOnly and writeOnly", context))
		}
		errs = append(errs, ValidateSchema304Fields(context, schema)...)
	}
	if (IsOpenAPI31(version) || IsOpenAPI32(version)) && schema.Nullable {
		errs = append(
			errs,
			Errorf(
				"%s.nullable is not supported in OpenAPI 3.1.x or 3.2.0; use type: [\"string\", \"null\"] instead",
				context,
			),
		)
	}
	if version != spec.Version320 {
		if schema.Discriminator != nil &&
			(schema.Discriminator.DefaultMapping != "" || ExtraHas(schema.Discriminator.Extra, "defaultMapping")) {
			errs = append(errs, Errorf("%s.discriminator.defaultMapping requires OpenAPI 3.2.0", context))
		}
		if schema.XML != nil && (schema.XML.NodeType != "" || ExtraHas(schema.XML.Extra, "nodeType")) {
			errs = append(errs, Errorf("%s.xml.nodeType requires OpenAPI 3.2.0", context))
		}
	}
	errs = append(errs, ValidateExclusiveBoundaries(context, schema, version)...)
	if schema.ExternalDocs != nil {
		if schema.ExternalDocs.URL == "" {
			errs = append(errs, Errorf("%s.externalDocs.url is required", context))
		} else if !IsURIReference(schema.ExternalDocs.URL) {
			errs = append(errs, Errorf("%s.externalDocs.url must be a URI", context))
		}
	}
	errs = append(errs, ValidateDiscriminator(context, schema, version)...)
	errs = append(errs, ValidateXML(context, schema)...)
	for name, child := range schema.Defs {
		errs = append(errs, ValidateSchema(context+".$defs."+name, child, version, visited)...)
	}
	for name, child := range schema.Properties {
		errs = append(errs, ValidateSchema(context+".properties."+name, child, version, visited)...)
	}
	for name, child := range schema.PatternProperties {
		errs = append(errs, ValidateSchema(context+".patternProperties."+name, child, version, visited)...)
	}
	errs = append(errs, ValidateSchema(context+".items", schema.Items, version, visited)...)
	for i, child := range schema.PrefixItems {
		errs = append(errs, ValidateSchema(fmt.Sprintf("%s.prefixItems[%d]", context, i), child, version, visited)...)
	}
	errs = append(errs, ValidateSchema(context+".contains", schema.Contains, version, visited)...)
	errs = append(
		errs,
		ValidateAnySchema(context+".additionalProperties", schema.AdditionalProperties, version, visited)...)
	errs = append(
		errs,
		ValidateAnySchema(context+".unevaluatedProperties", schema.UnevaluatedProperties, version, visited)...)
	errs = append(errs, ValidateSchema(context+".propertyNames", schema.PropertyNames, version, visited)...)
	for name, child := range schema.DependentSchemas {
		errs = append(errs, ValidateSchema(context+".dependentSchemas."+name, child, version, visited)...)
	}
	for i, child := range schema.AllOf {
		errs = append(errs, ValidateSchema(fmt.Sprintf("%s.allOf[%d]", context, i), child, version, visited)...)
	}
	for i, child := range schema.AnyOf {
		errs = append(errs, ValidateSchema(fmt.Sprintf("%s.anyOf[%d]", context, i), child, version, visited)...)
	}
	for i, child := range schema.OneOf {
		errs = append(errs, ValidateSchema(fmt.Sprintf("%s.oneOf[%d]", context, i), child, version, visited)...)
	}
	errs = append(errs, ValidateSchema(context+".not", schema.Not, version, visited)...)
	errs = append(errs, ValidateSchema(context+".if", schema.If, version, visited)...)
	errs = append(errs, ValidateSchema(context+".then", schema.Then, version, visited)...)
	errs = append(errs, ValidateSchema(context+".else", schema.Else, version, visited)...)
	errs = append(errs, ValidateSchema(context+".contentSchema", schema.ContentSchema, version, visited)...)
	return errs
}

func ValidateAnySchema(context string, value any, version string, visited map[*spec.Schema]bool) []error {
	switch typed := value.(type) {
	case *spec.Schema:
		return ValidateSchema(context, typed, version, visited)
	case spec.Schema:
		return ValidateSchema(context, &typed, version, visited)
	default:
		return nil
	}
}

//nolint:gocyclo,cyclop // OpenAPI 3.0.x checks enumerate many incompatible JSON Schema keywords.
func ValidateSchema304Fields(context string, schema *spec.Schema) []error {
	var errs []error
	if schema.Schema != "" || schema.ID != "" || len(schema.Defs) > 0 || schema.Anchor != "" ||
		schema.DynamicAnchor != "" ||
		schema.DynamicRef != "" ||
		len(schema.Vocabulary) > 0 ||
		schema.Comment != "" {
		errs = append(
			errs,
			Errorf("%s contains JSON Schema dialect fields that require OpenAPI 3.1.x or 3.2.0", context),
		)
	}
	if len(schema.Examples) > 0 || schema.Const != nil || len(schema.PatternProperties) > 0 ||
		len(schema.PrefixItems) > 0 ||
		schema.Contains != nil ||
		schema.MaxContains != nil ||
		schema.MinContains != nil ||
		schema.UnevaluatedProperties != nil ||
		schema.PropertyNames != nil ||
		len(schema.DependentRequired) > 0 ||
		len(schema.DependentSchemas) > 0 ||
		schema.If != nil ||
		schema.Then != nil ||
		schema.Else != nil ||
		schema.ContentEncoding != "" ||
		schema.ContentMediaType != "" ||
		schema.ContentSchema != nil {
		errs = append(
			errs,
			Errorf("%s contains JSON Schema 2020-12 keywords that require OpenAPI 3.1.x or 3.2.0", context),
		)
	}
	if _, ok := schema.Type.([]string); ok {
		errs = append(errs, Errorf("%s.type must be a string in OpenAPI 3.0.x", context))
	}
	if _, ok := schema.Type.([]any); ok {
		errs = append(errs, Errorf("%s.type must be a string in OpenAPI 3.0.x", context))
	}
	if schema.ExclusiveMaximum != nil {
		if _, ok := schema.ExclusiveMaximum.(bool); !ok {
			errs = append(errs, Errorf("%s.exclusiveMaximum must be a boolean in OpenAPI 3.0.x", context))
		}
	}
	if schema.ExclusiveMinimum != nil {
		if _, ok := schema.ExclusiveMinimum.(bool); !ok {
			errs = append(errs, Errorf("%s.exclusiveMinimum must be a boolean in OpenAPI 3.0.x", context))
		}
	}
	if ExtraHas(
		schema.Extra,
		"$schema",
		"$id",
		"$defs",
		"$anchor",
		"$dynamicAnchor",
		"$dynamicRef",
		"$vocabulary",
		"$comment",
		"examples",
		"const",
		"patternProperties",
		"prefixItems",
		"contains",
		"maxContains",
		"minContains",
		"unevaluatedProperties",
		"propertyNames",
		"dependentRequired",
		"dependentSchemas",
		"if",
		"then",
		"else",
		"contentEncoding",
		"contentMediaType",
		"contentSchema",
	) {
		errs = append(
			errs,
			Errorf("%s contains Extra JSON Schema keywords that require OpenAPI 3.1.x or 3.2.0", context),
		)
	}
	return errs
}

func ValidateExclusiveBoundaries(context string, schema *spec.Schema, version string) []error {
	var errs []error
	if !IsOpenAPI31(version) && !IsOpenAPI32(version) {
		return nil
	}
	if schema.ExclusiveMaximum != nil && !IsNumber(schema.ExclusiveMaximum) {
		errs = append(errs, Errorf("%s.exclusiveMaximum must be a number in OpenAPI 3.1.x or 3.2.0", context))
	}
	if schema.ExclusiveMinimum != nil && !IsNumber(schema.ExclusiveMinimum) {
		errs = append(errs, Errorf("%s.exclusiveMinimum must be a number in OpenAPI 3.1.x or 3.2.0", context))
	}
	return errs
}

func ValidateDiscriminator(context string, schema *spec.Schema, version string) []error {
	var errs []error
	if schema.Discriminator == nil {
		return nil
	}
	if schema.Discriminator.PropertyName == "" {
		errs = append(errs, Errorf("%s.discriminator.propertyName is required", context))
	}
	if len(schema.OneOf) == 0 && len(schema.AnyOf) == 0 && len(schema.AllOf) == 0 {
		errs = append(errs, Errorf("%s.discriminator is only allowed with anyOf, oneOf, or allOf", context))
	}
	if version == spec.Version320 && schema.Discriminator.PropertyName != "" &&
		!slices.Contains(schema.Required, schema.Discriminator.PropertyName) &&
		schema.Discriminator.DefaultMapping == "" && !ExtraHas(schema.Discriminator.Extra, "defaultMapping") {
		errs = append(
			errs,
			Errorf(
				"%s.discriminator.defaultMapping is required when discriminator property %q is optional",
				context,
				schema.Discriminator.PropertyName,
			),
		)
	}
	if (reflect.IsOpenAPI30(version) || IsOpenAPI31(version)) && schema.Discriminator.PropertyName != "" &&
		!slices.Contains(schema.Required, schema.Discriminator.PropertyName) {
		errs = append(
			errs,
			Warningf(
				"%s.discriminator.propertyName %q should be listed in the schema's required fields",
				context,
				schema.Discriminator.PropertyName,
			),
		)
	}
	return errs
}

func ValidateXML(context string, schema *spec.Schema) []error {
	var errs []error
	if schema.XML == nil {
		return nil
	}
	if schema.XML.Namespace != "" && !IsNonRelativeURI(schema.XML.Namespace) {
		errs = append(errs, Errorf("%s.xml.namespace must be a non-relative IRI", context))
	}
	nodeType := schema.XML.NodeType
	if extraNodeType, ok := schema.XML.Extra["nodeType"]; ok {
		nodeType, _ = extraNodeType.(string)
	}
	if nodeType != "" {
		if !slices.Contains([]string{"element", "attribute", "text", "cdata", "none"}, nodeType) {
			errs = append(errs,
				Errorf("%s.xml.nodeType must be one of element, attribute, text, cdata, or none", context))
		}
		if schema.XML.Attribute {
			errs = append(errs, Errorf("%s.xml.attribute must not be present when xml.nodeType is set", context))
		}
		if schema.XML.Wrapped {
			errs = append(errs, Errorf("%s.xml.wrapped must not be present when xml.nodeType is set", context))
		}
	}
	return errs
}
