package spec

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// MarshalJSON marshals an OpenAPI value while merging x-* extensions into objects.
func MarshalJSON(value any) ([]byte, error) {
	return json.Marshal(toSerializable(reflect.ValueOf(value), objectJSON))
}

const yamlIndent = 2

// MarshalYAML marshals an OpenAPI value while merging x-* extensions into objects.
func MarshalYAML(value any) ([]byte, error) {
	return yaml.MarshalWithOptions(
		toSerializable(reflect.ValueOf(value), objectYAML),
		yaml.Indent(yamlIndent),
		yaml.IndentSequence(true),
		yaml.UseSingleQuote(true),
	)
}

type objectMode int

const (
	objectJSON objectMode = iota
	objectYAML
)

type orderedField struct {
	Key   string
	Value any
}

type orderedObject []orderedField

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, field := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(field.Key)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(field.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func toSerializable(value reflect.Value, mode objectMode) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() { //nolint:exhaustive // only interested in complex types
	case reflect.Struct:
		return structToObject(value, mode)
	case reflect.Map:
		return mapToObject(value, mode)
	case reflect.Slice, reflect.Array:
		return sliceToSlice(value, mode)
	default:
		return value.Interface()
	}
}

func structToObject(value reflect.Value, mode objectMode) any {
	fields := []orderedField{}
	valueType := value.Type()
	for i := range value.NumField() {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Name == "Extensions" || field.Name == "Extra" {
			continue
		}
		name, omitempty := fieldTag(field, mode)
		if name == "" {
			continue
		}
		fieldValue := value.Field(i)
		if omitempty && isEmptyValue(fieldValue) {
			continue
		}
		fields = append(fields, orderedField{
			Key:   name,
			Value: toSerializable(fieldValue, mode),
		})
	}
	for i := range value.NumField() {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		switch field.Name {
		case "Extensions":
			fields = appendOrderedFields(fields, extensionFields(value.Field(i), mode)...)
		case "Expressions":
			fields = appendOrderedFields(fields, mapFields(value.Field(i), mode)...)
		case "Extra":
			fields = appendOrderedFields(fields, extraFields(value.Field(i), mode)...)
		}
	}
	return makeObject(fields, mode)
}

func mapToObject(value reflect.Value, mode objectMode) any {
	if value.IsNil() {
		return nil
	}
	return makeObject(mapFields(value, mode), mode)
}

func mapFields(value reflect.Value, mode objectMode) []orderedField {
	if value.Kind() != reflect.Map || value.IsNil() {
		return nil
	}
	fields := []orderedField{}
	iter := value.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() != reflect.String {
			continue
		}
		fields = append(fields, orderedField{
			Key:   key.String(),
			Value: mapValueToSerializable(iter.Value(), mode),
		})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})
	return fields
}

func mapValueToSerializable(value reflect.Value, mode objectMode) any {
	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Slice && value.IsNil() {
		return []any{}
	}
	return toSerializable(value, mode)
}

func sliceToSlice(value reflect.Value, mode objectMode) any {
	if value.Kind() == reflect.Slice && value.IsNil() {
		return nil
	}
	out := make([]any, 0, value.Len())
	for i := range value.Len() {
		out = append(out, toSerializable(value.Index(i), mode))
	}
	return out
}

func extensionFields(value reflect.Value, mode objectMode) []orderedField {
	fields := []orderedField{}
	if value.Kind() == reflect.Map && !value.IsNil() {
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() == reflect.String && strings.HasPrefix(key.String(), "x-") {
				fields = append(fields, orderedField{
					Key:   key.String(),
					Value: toSerializable(iter.Value(), mode),
				})
			}
		}
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})
	return fields
}

func extraFields(value reflect.Value, mode objectMode) []orderedField {
	fields := []orderedField{}
	if value.Kind() == reflect.Map && !value.IsNil() {
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() == reflect.String {
				fields = append(fields, orderedField{
					Key:   key.String(),
					Value: toSerializable(iter.Value(), mode),
				})
			}
		}
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})
	return fields
}

func makeObject(fields []orderedField, mode objectMode) any {
	if mode == objectYAML {
		out := yaml.MapSlice{}
		for _, field := range fields {
			out = append(out, yaml.MapItem{Key: field.Key, Value: field.Value})
		}
		return out
	}
	return orderedObject(fields)
}

func appendOrderedFields(fields []orderedField, next ...orderedField) []orderedField {
	for _, field := range next {
		replaced := false
		for i := range fields {
			if fields[i].Key == field.Key {
				fields[i] = field
				replaced = true
				break
			}
		}
		if !replaced {
			fields = append(fields, field)
		}
	}
	return fields
}

func fieldTag(field reflect.StructField, mode objectMode) (string, bool) {
	tag := field.Tag.Get("json")
	if mode == objectYAML {
		if yamlTag := field.Tag.Get("yaml"); yamlTag != "" {
			tag = yamlTag
		}
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false
	}
	if name == "" {
		name = field.Name
	}
	return name, strings.Contains(opts, "omitempty")
}

func isEmptyValue(value reflect.Value) bool {
	switch value.Kind() { //nolint:exhaustive // only interested in complex types
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	default:
		return value.IsZero()
	}
}
