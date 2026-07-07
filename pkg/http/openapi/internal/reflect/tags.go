package reflect

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

//nolint:gocyclo,cyclop,funlen // struct tags map to many independent schema fields.
func (r *Reflector) ApplySchemaTags(schema *spec.Schema, field reflect.StructField) {
	tag := field.Tag
	if v := tag.Get("type"); v != "" {
		schema.Type = ParseTypeTag(v, r.Config.OpenAPIVersion)
	}
	if v := tag.Get("title"); v != "" {
		schema.Title = v
	}
	if v := tag.Get("description"); v != "" {
		schema.Description = v
	}
	if v := tag.Get("format"); v != "" {
		schema.Format = v
	}
	if v := tag.Get("pattern"); v != "" {
		schema.Pattern = v
	}
	if v := tag.Get("default"); v != "" {
		schema.Default = ParseTagValue(v)
	}
	if v := tag.Get("example"); v != "" {
		schema.Example = ParseTagValue(v)
	}
	if v := tag.Get("examples"); v != "" && !IsOpenAPI30(r.Config.OpenAPIVersion) {
		schema.Examples = ParseTagValues(v)
	}
	if v := tag.Get("enum"); v != "" {
		schema.Enum = ParseTagValues(v)
	}
	if v := tag.Get("const"); v != "" && !IsOpenAPI30(r.Config.OpenAPIVersion) {
		schema.Const = ParseTagValue(v)
	}
	if v := FloatTag(tag.Get("multipleOf")); v != nil {
		schema.MultipleOf = v
	}
	if v := FloatTag(tag.Get("maximum")); v != nil {
		schema.Maximum = v
	}
	if v := FloatTag(tag.Get("minimum")); v != nil {
		schema.Minimum = v
	}
	r.ApplyExclusiveLimit(schema, tag, "exclusiveMaximum")
	r.ApplyExclusiveLimit(schema, tag, "exclusiveMinimum")
	if v := IntTag(tag.Get("maxLength")); v != nil {
		schema.MaxLength = v
	}
	if v := IntTag(tag.Get("minLength")); v != nil {
		schema.MinLength = v
	}
	if v := IntTag(tag.Get("maxItems")); v != nil {
		schema.MaxItems = v
	}
	if v := IntTag(tag.Get("minItems")); v != nil {
		schema.MinItems = v
	}
	if v := IntTag(tag.Get("maxProperties")); v != nil {
		schema.MaxProperties = v
	}
	if v := IntTag(tag.Get("minProperties")); v != nil {
		schema.MinProperties = v
	}
	if v := tag.Get("uniqueItems"); v != "" {
		b := BoolTag(v)
		schema.UniqueItems = &b
	}
	if BoolTag(tag.Get("nullable")) {
		r.ApplyNullable(schema, true)
	}
	if BoolTag(tag.Get("deprecated")) {
		schema.Deprecated = true
	}
	if BoolTag(tag.Get("readOnly")) {
		schema.ReadOnly = true
	}
	if BoolTag(tag.Get("writeOnly")) {
		schema.WriteOnly = true
	}
	if v := tag.Get("contentEncoding"); v != "" && !IsOpenAPI30(r.Config.OpenAPIVersion) {
		schema.ContentEncoding = v
	}
	if v := tag.Get("contentMediaType"); v != "" && !IsOpenAPI30(r.Config.OpenAPIVersion) {
		schema.ContentMediaType = v
	}
	r.ApplyXMLTags(schema, tag)
}

func (r *Reflector) ApplyExclusiveLimit(schema *spec.Schema, tag reflect.StructTag, key string) {
	value := tag.Get(key)
	if value == "" {
		return
	}
	if IsOpenAPI30(r.Config.OpenAPIVersion) {
		schemaValue := BoolTag(value)
		if key == "exclusiveMaximum" {
			schema.ExclusiveMaximum = schemaValue
		} else {
			schema.ExclusiveMinimum = schemaValue
		}
		return
	}
	if schemaValue := FloatTag(value); schemaValue != nil {
		if key == "exclusiveMaximum" {
			schema.ExclusiveMaximum = *schemaValue
		} else {
			schema.ExclusiveMinimum = *schemaValue
		}
	}
}

func (r *Reflector) ApplyXMLTags(schema *spec.Schema, tag reflect.StructTag) {
	xmlName := tag.Get("xmlName")
	xmlNamespace := tag.Get("xmlNamespace")
	xmlPrefix := tag.Get("xmlPrefix")
	xmlAttribute := tag.Get("xmlAttribute")
	xmlWrapped := tag.Get("xmlWrapped")
	xmlNodeType := tag.Get("xmlNodeType")
	if xmlName == "" && xmlNamespace == "" && xmlPrefix == "" && xmlAttribute == "" && xmlWrapped == "" &&
		xmlNodeType == "" {
		return
	}
	if schema.XML == nil {
		schema.XML = &spec.XML{}
	}
	schema.XML.Name = xmlName
	schema.XML.Namespace = xmlNamespace
	schema.XML.Prefix = xmlPrefix

	if r.Config.OpenAPIVersion == spec.Version320 {
		switch {
		case xmlNodeType != "":
			schema.XML.NodeType = xmlNodeType
		case xmlAttribute != "" && BoolTag(xmlAttribute):
			schema.XML.NodeType = "attribute"
		case xmlWrapped != "" && BoolTag(xmlWrapped):
			schema.XML.NodeType = "element"
		}
	} else {
		if xmlAttribute != "" {
			schema.XML.Attribute = BoolTag(xmlAttribute)
		}
		if xmlWrapped != "" {
			schema.XML.Wrapped = BoolTag(xmlWrapped)
		}
	}

	if schema.XML.Name == "" && schema.XML.Namespace == "" && schema.XML.Prefix == "" && !schema.XML.Attribute &&
		!schema.XML.Wrapped &&
		schema.XML.NodeType == "" &&
		len(schema.XML.Extra) == 0 {
		schema.XML = nil
	}
}

func ParseTagValue(value string) any {
	var decoded any
	if json.Unmarshal([]byte(value), &decoded) == nil {
		return NormalizeTagValue(decoded)
	}
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return value
}

func NormalizeTagValue(value any) any {
	switch typed := value.(type) {
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, NormalizeTagValue(item))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = NormalizeTagValue(item)
		}
		return out
	case float64:
		if math.Trunc(typed) == typed && typed >= math.MinInt64 && typed <= math.MaxInt64 {
			return int64(typed)
		}
		return typed
	default:
		return value
	}
}

func ParseTagValues(value string) []any {
	var decoded []any
	if json.Unmarshal([]byte(value), &decoded) == nil {
		return decoded
	}
	parts := strings.Split(value, ",")
	out := make([]any, 0, len(parts))
	for _, part := range parts {
		out = append(out, ParseTagValue(strings.TrimSpace(part)))
	}
	return out
}

func ParseTypeTag(value string, version string) any {
	parts := strings.Split(value, ",")
	if IsOpenAPI30(version) || len(parts) == 1 {
		return strings.TrimSpace(parts[0])
	}
	types := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			types = append(types, trimmed)
		}
	}
	if len(types) == 1 {
		return types[0]
	}
	return types
}

func TagName(field reflect.StructField, key string) string {
	raw := field.Tag.Get(key)
	if raw == "-" {
		return ""
	}
	if raw == "" {
		return ""
	}
	name, _, _ := strings.Cut(raw, ",")
	if name == "" || name == "-" {
		return ""
	}
	return name
}

func IgnoredField(field reflect.StructField, key string) bool {
	if field.Tag.Get(key) == "-" {
		return true
	}
	return key != "json" && field.Tag.Get("json") == "-"
}

func BoolTag(value string) bool {
	b, _ := strconv.ParseBool(value)
	return b
}

func IntTag(value string) *int {
	if value == "" {
		return nil
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &i
}

func FloatTag(value string) *float64 {
	if value == "" {
		return nil
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &f
}

func BodyNameTag(contentType string) string {
	switch {
	case strings.Contains(contentType, "x-www-form-urlencoded"), strings.Contains(contentType, "multipart/form-data"):
		return "form"
	default:
		return "json"
	}
}
