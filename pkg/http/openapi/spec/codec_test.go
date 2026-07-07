package spec

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStruct struct {
	Foo        string         `json:"foo"`
	Bar        int            `json:"bar,omitempty"`
	Baz        *string        `json:"baz,omitempty"`
	Extensions map[string]any `json:"-"`
}

func TestMarshalJSON(t *testing.T) {
	s := "baz"
	val := mockStruct{
		Foo:        "foo",
		Baz:        &s,
		Extensions: map[string]any{"x-extra": "val"},
	}

	data, err := MarshalJSON(val)
	require.NoError(t, err)

	var out map[string]any
	err = json.Unmarshal(data, &out)
	require.NoError(t, err)

	assert.Equal(t, "foo", out["foo"])
	assert.Equal(t, "baz", out["baz"])
	assert.Equal(t, "val", out["x-extra"])
	assert.NotContains(t, out, "bar")
}

type extraStruct struct {
	Foo   string         `json:"foo"`
	Extra map[string]any `json:"-"`
}

func TestMarshalExtra(t *testing.T) {
	val := extraStruct{
		Foo:   "foo",
		Extra: map[string]any{"bar": 123},
	}
	data, err := MarshalJSON(val)
	require.NoError(t, err)

	var out map[string]any
	err = json.Unmarshal(data, &out)
	require.NoError(t, err)

	assert.Equal(t, "foo", out["foo"])
	assert.InDelta(t, 123.0, out["bar"], 0.0001)
}

func TestToSerializable_EdgeCases(t *testing.T) {
	t.Run("MapValuePointer", func(t *testing.T) {
		s := "val"
		m := map[string]*string{"key": &s}
		res := toSerializable(reflect.ValueOf(m), objectJSON)
		// orderedObject is internal, so we check the marshaled JSON
		data, err := json.Marshal(res)
		require.NoError(t, err)
		assert.JSONEq(t, `{"key":"val"}`, string(data))
	})

	t.Run("EmbeddedStruct", func(t *testing.T) {
		type Inner struct {
			InnerField string `json:"inner"`
		}
		type Outer struct {
			Inner

			OuterField string `json:"outer"`
		}
		o := Outer{OuterField: "o", Inner: Inner{InnerField: "i"}}
		res := toSerializable(reflect.ValueOf(o), objectJSON)
		data, err := json.Marshal(res)
		require.NoError(t, err)
		// The codec currently doesn't flatten anonymous fields like standard encoding/json
		assert.JSONEq(t, `{"outer":"o", "Inner":{"inner":"i"}}`, string(data))
	})

	t.Run("OrderedFields", func(t *testing.T) {
		type Ordered struct {
			A string `json:"a"`
			B string `json:"b"`
		}
		o := Ordered{A: "1", B: "2"}
		res := toSerializable(reflect.ValueOf(o), objectYAML)
		if assert.IsType(t, yaml.MapSlice{}, res) {
			slice := res.(yaml.MapSlice)
			assert.Equal(t, "a", slice[0].Key)
			assert.Equal(t, "b", slice[1].Key)
		}
	})

	t.Run("Expressions", func(t *testing.T) {
		type ExprStruct struct {
			Expressions map[string]any `json:"-"`
		}
		e := ExprStruct{
			Expressions: map[string]any{"$method": "GET"},
		}
		res := toSerializable(reflect.ValueOf(e), objectJSON)
		data, err := json.Marshal(res)
		require.NoError(t, err)
		assert.JSONEq(t, `{"$method":"GET"}`, string(data))
	})

	t.Run("MapNonStringKey", func(t *testing.T) {
		m := map[int]string{1: "foo"}
		res := toSerializable(reflect.ValueOf(m), objectJSON)
		data, err := json.Marshal(res)
		require.NoError(t, err)
		assert.JSONEq(t, `{}`, string(data))
	})
}

func TestIsEmptyValue(t *testing.T) {
	cases := []struct {
		val   any
		empty bool
	}{
		{0, true},
		{1, false},
		{"", true},
		{"a", false},
		{false, true},
		{true, false},
		{0.0, true},
		{1.1, false},
		{[]int{}, true},
		{[]int{1}, false},
		{map[string]int{}, true},
		{map[string]int{"a": 1}, false},
		{uint(0), true},
		{uint(1), false},
		{float32(0), true},
		{float32(1.1), false},
		{[0]int{}, true},
		{[1]int{0}, false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.empty, isEmptyValue(reflect.ValueOf(tc.val)), "isEmptyValue(%v)", tc.val)
	}
}

func TestMarshalYAML(t *testing.T) {
	val := map[string]any{
		"foo":    "bar",
		"nested": []any{1, "two", map[string]any{"x": true}},
	}

	data, err := MarshalYAML(val)
	require.NoError(t, err)

	var out any
	err = yaml.Unmarshal(data, &out)
	require.NoError(t, err)
}

func TestToSerializable(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		assert.Nil(t, toSerializable(reflect.ValueOf(nil), objectJSON))
		var s *string
		assert.Nil(t, toSerializable(reflect.ValueOf(s), objectJSON))
	})

	t.Run("Map", func(t *testing.T) {
		m := map[string]int{"a": 1, "b": 2}
		res := toSerializable(reflect.ValueOf(m), objectYAML)
		if assert.IsType(t, yaml.MapSlice{}, res) {
			slice := res.(yaml.MapSlice)
			if assert.Len(t, slice, 2) {
				assert.Equal(t, "a", slice[0].Key)
				assert.Equal(t, "b", slice[1].Key)
			}
		}
	})

	t.Run("Slice", func(t *testing.T) {
		s := []string{"a", "b"}
		res := toSerializable(reflect.ValueOf(s), objectJSON)
		if assert.IsType(t, []any{}, res) {
			slice := res.([]any)
			assert.Equal(t, []any{"a", "b"}, slice)
		}
	})

	t.Run("Array", func(t *testing.T) {
		a := [2]string{"a", "b"}
		res := toSerializable(reflect.ValueOf(a), objectJSON)
		if assert.IsType(t, []any{}, res) {
			array := res.([]any)
			assert.Equal(t, []any{"a", "b"}, array)
		}
	})
}

func TestCodecInternalHelpers(t *testing.T) {
	t.Run("mapToObject returns nil for nil map", func(t *testing.T) {
		var m map[string]int
		assert.Nil(t, mapToObject(reflect.ValueOf(m), objectJSON))
	})

	t.Run("mapValueToSerializable handles nil interface", func(t *testing.T) {
		holder := struct{ V any }{}
		assert.Nil(t, mapValueToSerializable(reflect.ValueOf(holder).Field(0), objectJSON))
	})

	t.Run("mapValueToSerializable converts nil slice to empty array", func(t *testing.T) {
		var vals []string
		assert.Equal(t, []any{}, mapValueToSerializable(reflect.ValueOf(vals), objectJSON))
	})

	t.Run("sliceToSlice returns nil for nil slice", func(t *testing.T) {
		var vals []string
		assert.Nil(t, sliceToSlice(reflect.ValueOf(vals), objectJSON))
	})

	t.Run("appendOrderedFields replaces existing keys", func(t *testing.T) {
		fields := []orderedField{{Key: "x", Value: "old"}}
		out := appendOrderedFields(fields, orderedField{Key: "x", Value: "new"}, orderedField{Key: "y", Value: 2})
		require.Len(t, out, 2)
		assert.Equal(t, []orderedField{{Key: "x", Value: "new"}, {Key: "y", Value: 2}}, out)
	})
}
