package reflect

import (
	"mime/multipart"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

func TestSanitizeTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"Foo", "Foo"},
		{"*Foo", "Foo"},
		{"[]Foo", "FooList"},
		{"[][]Foo", "FooListList"},
		{"github.com/foo.User", "User"},
		{"BaseResponse[github.com/foo.User]", "BaseResponseUser"},
		{"Map[string, int]", "Mapstringint"},
		{"Complex[[]string, *int]", "ComplexstringListint"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeTypeName(tt.input))
		})
	}
}

func TestLowerCamel(t *testing.T) {
	assert.Equal(t, "fooBar", LowerCamel("FooBar"))
	assert.Equal(t, "foo", LowerCamel("Foo"))
	assert.Empty(t, LowerCamel(""))
}

func TestIndirectType(t *testing.T) {
	type Foo struct{}
	// f := Foo{}
	typ := reflect.TypeFor[Foo]()
	assert.Equal(t, typ, IndirectType(typ))
	assert.Equal(t, typ, IndirectType(reflect.TypeFor[*Foo]()))
	assert.Equal(t, typ, IndirectType(reflect.TypeFor[**Foo]()))
}

func TestForEachField(t *testing.T) {
	type Inner struct {
		InnerField string `json:"inner"`
	}
	type Outer struct {
		Inner `json:",inline"`

		OuterField string `json:"outer"`
		_          string
	}

	fields := []string{}
	ForEachField(reflect.TypeFor[Outer](), func(f reflect.StructField) {
		fields = append(fields, f.Name)
	})

	assert.Contains(t, fields, "OuterField")
	assert.Contains(t, fields, "InnerField")
	assert.NotContains(t, fields, "Inner") // Inner is inlined
}

func TestInternalHelpers(t *testing.T) {
	t.Run("prefixWithPkg", func(t *testing.T) {
		assert.Equal(t, "Model", prefixWithPkg(nil, "Model"))
		assert.Equal(t, "Model", prefixWithPkg(reflect.TypeFor[struct{}](), "Model"))
		assert.Equal(t, "TimeModel", prefixWithPkg(reflect.TypeFor[time.Time](), "Model"))
		assert.Empty(t, prefixWithPkg(reflect.TypeFor[time.Time](), ""))
	})

	t.Run("reflector helper accessors", func(t *testing.T) {
		r := &Reflector{}
		assert.Nil(t, r.interceptPropFn())
		assert.Nil(t, r.interceptSchemaFn())

		cfg := &spec.Config{
			ReflectorConfig: &spec.ReflectorConfig{
				InterceptProp:   func(spec.InterceptPropParams) error { return nil },
				InterceptSchema: func(spec.InterceptSchemaParams) (bool, error) { return false, nil },
			},
		}
		r = &Reflector{Config: cfg}
		assert.NotNil(t, r.interceptPropFn())
		assert.NotNil(t, r.interceptSchemaFn())
	})

	t.Run("shallowCopyMap", func(t *testing.T) {
		assert.Nil(t, shallowCopyMap(nil))

		in := map[string]any{"a": 1}
		out := shallowCopyMap(in)
		assert.Equal(t, map[string]any{"a": 1}, out)

		in["a"] = 2
		assert.Equal(t, 1, out["a"])
	})

	t.Run("uniqueStrings", func(t *testing.T) {
		assert.Nil(t, uniqueStrings(nil))
		assert.Equal(t, []string{}, uniqueStrings([]string{}))
		assert.Equal(t, []string{"a", "b", "c"}, uniqueStrings([]string{"a", "b", "a", "c", "b"}))
	})
}

func TestInferContentType(t *testing.T) {
	type noTags struct {
		Name string `json:"name"`
	}
	type formBody struct {
		Name  string `form:"name"`
		Email string `form:"email"`
	}
	type mixedBody struct {
		Query string `query:"q"`
		Name  string `form:"name"`
	}
	type fileBody struct {
		File *multipart.FileHeader `form:"file"`
	}
	type multiFileBody struct {
		Files []*multipart.FileHeader `form:"files"`
	}
	type fileInterfaceBody struct {
		File multipart.File `form:"file"`
	}

	assert.Empty(t, InferContentType(nil))
	assert.Empty(t, InferContentType(noTags{}))
	assert.Empty(t, InferContentType("string"))
	assert.Equal(t, "application/x-www-form-urlencoded", InferContentType(formBody{}))
	assert.Equal(t, "application/x-www-form-urlencoded", InferContentType(&formBody{}))
	assert.Equal(t, "application/x-www-form-urlencoded", InferContentType(mixedBody{}))
	assert.Equal(t, "multipart/form-data", InferContentType(fileBody{}))
	assert.Equal(t, "multipart/form-data", InferContentType(multiFileBody{}))
	assert.Equal(t, "multipart/form-data", InferContentType(fileInterfaceBody{}))
}
