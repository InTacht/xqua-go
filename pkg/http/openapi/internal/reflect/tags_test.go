package reflect_test

import (
	std_reflect "reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"
	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func TestIgnoredField(t *testing.T) {
	type TestStruct struct {
		IgnoredJSON        string `json:"-"`
		IgnoredCustom      string `custom:"-"`
		NotIgnored         string `json:"foo"`
		JSONDashNotIgnored string `json:"'-',omitempty"`
	}

	typ := std_reflect.TypeFor[TestStruct]()

	f, _ := typ.FieldByName("IgnoredJSON")
	assert.True(t, reflect.IgnoredField(f, "json"))
	assert.True(t, reflect.IgnoredField(f, "custom"))

	f, _ = typ.FieldByName("IgnoredCustom")
	assert.False(t, reflect.IgnoredField(f, "json"))
	assert.True(t, reflect.IgnoredField(f, "custom"))

	f, _ = typ.FieldByName("NotIgnored")
	assert.False(t, reflect.IgnoredField(f, "json"))

	f, _ = typ.FieldByName("JSONDashNotIgnored")
	assert.False(t, reflect.IgnoredField(f, "json"))
}

func TestParseTypeTag(t *testing.T) {
	assert.Equal(t, "string", reflect.ParseTypeTag("string", "3.0.4"))
	assert.Equal(t, "string", reflect.ParseTypeTag("string,null", "3.0.4"))
	assert.Equal(t, []string{"string", "null"}, reflect.ParseTypeTag("string,null", "3.1.0"))
	assert.Equal(t, "string", reflect.ParseTypeTag("string", "3.1.0"))
}

func TestNormalizeTagValue(t *testing.T) {
	assert.Equal(t, int64(123), reflect.NormalizeTagValue(123.0))
	assert.InDelta(t, 123.45, reflect.NormalizeTagValue(123.45), 0.0001)
	assert.Equal(t, "foo", reflect.NormalizeTagValue("foo"))
	assert.Equal(t, []any{int64(1)}, reflect.NormalizeTagValue([]any{1.0}))
	assert.Equal(t, map[string]any{"k": int64(1)}, reflect.NormalizeTagValue(map[string]any{"k": 1.0}))
}

func TestIntTag(t *testing.T) {
	assert.Nil(t, reflect.IntTag(""))
	assert.Nil(t, reflect.IntTag("abc"))
	i := reflect.IntTag("123")
	require.NotNil(t, i)
	assert.Equal(t, 123, *i)
}

func TestApplyXMLTags(t *testing.T) {
	r304 := reflect.NewReflector(&spec.Config{OpenAPIVersion: spec.Version304})
	r320 := reflect.NewReflector(&spec.Config{OpenAPIVersion: spec.Version320})

	t.Run("OpenAPI 3.0.4", func(t *testing.T) {
		s := &spec.Schema{}
		tag := std_reflect.StructTag(`xmlName:"foo" xmlAttribute:"true" xmlWrapped:"true"`)
		r304.ApplyXMLTags(s, tag)
		require.NotNil(t, s.XML)
		assert.Equal(t, "foo", s.XML.Name)
		assert.True(t, s.XML.Attribute)
		assert.True(t, s.XML.Wrapped)
	})

	t.Run("OpenAPI 3.2.0 NodeType", func(t *testing.T) {
		s := &spec.Schema{}
		tag := std_reflect.StructTag(`xmlNodeType:"attribute"`)
		r320.ApplyXMLTags(s, tag)
		require.NotNil(t, s.XML)
		assert.Equal(t, "attribute", s.XML.NodeType)
	})

	t.Run("OpenAPI 3.2.0 Attribute Fallback", func(t *testing.T) {
		s := &spec.Schema{}
		tag := std_reflect.StructTag(`xmlAttribute:"true"`)
		r320.ApplyXMLTags(s, tag)
		assert.Equal(t, "attribute", s.XML.NodeType)
	})

	t.Run("OpenAPI 3.2.0 Wrapped Fallback", func(t *testing.T) {
		s := &spec.Schema{}
		tag := std_reflect.StructTag(`xmlWrapped:"true"`)
		r320.ApplyXMLTags(s, tag)
		assert.Equal(t, "element", s.XML.NodeType)
	})

	t.Run("Empty", func(t *testing.T) {
		s := &spec.Schema{}
		r304.ApplyXMLTags(s, "")
		assert.Nil(t, s.XML)
	})
}

func TestParseTagValueAndValues(t *testing.T) {
	t.Run("ParseTagValue", func(t *testing.T) {
		assert.Equal(t, true, reflect.ParseTagValue("true"))
		assert.Equal(t, int64(42), reflect.ParseTagValue("42"))
		assert.InDelta(t, 3.14, reflect.ParseTagValue("3.14"), 0.0001)
		assert.Equal(
			t,
			map[string]any{"a": int64(1), "b": []any{int64(2), int64(3)}},
			reflect.ParseTagValue(`{"a":1,"b":[2,3]}`),
		)
		assert.Equal(t, "plain-text", reflect.ParseTagValue("plain-text"))
	})

	t.Run("ParseTagValues", func(t *testing.T) {
		assert.Equal(t, []any{float64(1), float64(2), float64(3)}, reflect.ParseTagValues("[1,2,3]"))
		assert.Equal(t, []any{int64(1), int64(2), "three"}, reflect.ParseTagValues("1, 2, three"))
	})
}

func TestApplyExclusiveLimit(t *testing.T) {
	t.Run("OpenAPI30UsesBooleans", func(t *testing.T) {
		r := reflect.NewReflector(&spec.Config{OpenAPIVersion: spec.Version304})
		s := &spec.Schema{}
		tag := std_reflect.StructTag(`exclusiveMaximum:"true" exclusiveMinimum:"false"`)

		r.ApplyExclusiveLimit(s, tag, "exclusiveMaximum")
		r.ApplyExclusiveLimit(s, tag, "exclusiveMinimum")

		assert.Equal(t, true, s.ExclusiveMaximum)
		assert.Equal(t, false, s.ExclusiveMinimum)
	})

	t.Run("OpenAPI31UsesNumbers", func(t *testing.T) {
		r := reflect.NewReflector(&spec.Config{OpenAPIVersion: spec.Version312})
		s := &spec.Schema{}
		tag := std_reflect.StructTag(`exclusiveMaximum:"9.5" exclusiveMinimum:"-1.25"`)

		r.ApplyExclusiveLimit(s, tag, "exclusiveMaximum")
		r.ApplyExclusiveLimit(s, tag, "exclusiveMinimum")

		assert.InDelta(t, 9.5, s.ExclusiveMaximum, 0.0001)
		assert.InDelta(t, -1.25, s.ExclusiveMinimum, 0.0001)
	})
}

func TestApplySchemaTags(t *testing.T) {
	type Payload struct {
		Value string `json:"value" type:"string,null" title:"User value" description:"desc" format:"uuid" pattern:"^u_" default:"42" example:"7" examples:"[1,2]" enum:"a,b" const:"fixed" multipleOf:"2" maximum:"10" minimum:"1" exclusiveMaximum:"9.5" exclusiveMinimum:"0.5" maxLength:"64" minLength:"1" maxItems:"3" minItems:"1" maxProperties:"5" minProperties:"1" uniqueItems:"true" nullable:"true" deprecated:"true" readOnly:"true" writeOnly:"true" contentEncoding:"gzip" contentMediaType:"application/json"`
	}

	field, ok := std_reflect.TypeFor[Payload]().FieldByName("Value")
	require.True(t, ok)

	r := reflect.NewReflector(&spec.Config{OpenAPIVersion: spec.Version312})
	schema := &spec.Schema{}
	r.ApplySchemaTags(schema, field)

	assert.Equal(t, []string{"string", "null"}, schema.Type)
	assert.Equal(t, "User value", schema.Title)
	assert.Equal(t, "desc", schema.Description)
	assert.Equal(t, "uuid", schema.Format)
	assert.Equal(t, "^u_", schema.Pattern)
	assert.Equal(t, int64(42), schema.Default)
	assert.Equal(t, int64(7), schema.Example)
	assert.Equal(t, []any{float64(1), float64(2)}, schema.Examples)
	assert.Equal(t, []any{"a", "b"}, schema.Enum)
	assert.Equal(t, "fixed", schema.Const)
	assert.True(t, schema.Deprecated)
	assert.True(t, schema.ReadOnly)
	assert.True(t, schema.WriteOnly)
	assert.Equal(t, "gzip", schema.ContentEncoding)
	assert.Equal(t, "application/json", schema.ContentMediaType)
}
