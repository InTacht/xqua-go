package reflect

import (
	"encoding"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/InTacht/xqua-go/pkg/http/openapi/spec"
)

var (
	typeOfTextMarshaler   = reflect.TypeFor[encoding.TextMarshaler]()
	typeOfTextUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
	typeOfJSONMarshaler   = reflect.TypeFor[json.Marshaler]()
)

func isTextMarshaler(t reflect.Type) bool {
	impl := func(iface reflect.Type) bool {
		return t.Implements(iface) || reflect.PointerTo(t).Implements(iface)
	}
	return impl(typeOfTextMarshaler) && impl(typeOfTextUnmarshaler) && !impl(typeOfJSONMarshaler)
}

//nolint:funlen,gocognit // covers full OpenAPI scalar/collection/struct mapping in one switch for readability.
func (r *Reflector) SchemaForType(
	t reflect.Type,
	mode SchemaMode,
	field *reflect.StructField,
) (*spec.Schema, error) {
	nullable := false
	for t != nil && t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}
	if t == nil {
		return &spec.Schema{}, nil
	}
	if mapped := r.TypeMapping[t]; mapped != nil {
		t = mapped
	}
	interceptSchema := r.interceptSchemaFn()
	//nolint:nestif // exposer path needs pre+post hook with nullable/tag application
	if schema := r.SchemaFromTypeExposer(t); schema != nil {
		// Pre-hook for exposer types: they bypass the standard pre-hook path below.
		if interceptSchema != nil {
			preSchema := &spec.Schema{}
			stop, err := interceptSchema(spec.InterceptSchemaParams{Type: t, Schema: preSchema})
			if err != nil {
				return nil, err
			}
			if stop {
				r.ApplyNullable(preSchema, nullable)
				if field != nil {
					r.ApplySchemaTags(preSchema, *field)
				}
				return preSchema, nil
			}
		}
		if interceptSchema != nil {
			params := spec.InterceptSchemaParams{Type: t, Schema: schema, Processed: true}
			if _, err := interceptSchema(params); err != nil {
				return nil, err
			}
		}
		r.ApplyNullable(schema, nullable)
		if field != nil {
			r.ApplySchemaTags(schema, *field)
		}
		return schema, nil
	}
	if mode == SchemaUseComponent && IsComponentType(t) && !r.InlineRefs() {
		schema, err := r.RefSchema(t)
		if err != nil {
			return nil, err
		}
		r.ApplyNullable(schema, nullable)
		if field != nil {
			r.ApplySchemaTags(schema, *field)
		}
		return schema, nil
	}

	// Pre-hook for inline and primitive types (component types are intercepted inside RefSchema).
	if interceptSchema != nil {
		preSchema := &spec.Schema{}
		stop, err := interceptSchema(spec.InterceptSchemaParams{Type: t, Schema: preSchema})
		if err != nil {
			return nil, err
		}
		if stop {
			r.ApplyNullable(preSchema, nullable)
			if field != nil {
				r.ApplySchemaTags(preSchema, *field)
			}
			return preSchema, nil
		}
	}

	if isTextMarshaler(t) {
		schema := &spec.Schema{Type: "string"}
		if interceptSchema != nil {
			if _, err := interceptSchema(
				spec.InterceptSchemaParams{Type: t, Schema: schema, Processed: true},
			); err != nil {
				return nil, err
			}
		}
		r.ApplyNullable(schema, nullable)
		if field != nil {
			r.ApplySchemaTags(schema, *field)
		}
		return schema, nil
	}

	var schema *spec.Schema
	switch t.Kind() { //nolint:exhaustive // only interested in types supported by OpenAPI
	case reflect.Bool:
		schema = &spec.Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		schema = &spec.Schema{Type: "integer", Format: "int32"}
	case reflect.Int64:
		schema = &spec.Schema{Type: "integer", Format: "int64"}
	case reflect.Uint8, reflect.Uint16:
		minVal := 0.0
		schema = &spec.Schema{Type: "integer", Format: "int32", Minimum: &minVal}
	case reflect.Uint, reflect.Uint32:
		minVal := 0.0
		schema = &spec.Schema{Type: "integer", Format: "int64", Minimum: &minVal}
	case reflect.Uint64, reflect.Uintptr:
		minVal := 0.0
		schema = &spec.Schema{Type: "integer", Format: "int64", Minimum: &minVal}
	case reflect.Float32:
		schema = &spec.Schema{Type: "number", Format: "float"}
	case reflect.Float64:
		schema = &spec.Schema{Type: "number", Format: "double"}
	case reflect.String:
		schema = &spec.Schema{Type: "string"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			if IsOpenAPI30(r.Config.OpenAPIVersion) {
				schema = &spec.Schema{Type: "string", Format: "byte"}
			} else {
				schema = &spec.Schema{Type: "string", ContentEncoding: "base64"}
			}
			break
		}
		items, err := r.SchemaForType(t.Elem(), SchemaUseComponent, nil)
		if err != nil {
			return nil, err
		}
		schema = &spec.Schema{Type: "array", Items: items}
	case reflect.Map:
		addlProps, err := r.SchemaForType(t.Elem(), SchemaUseComponent, nil)
		if err != nil {
			return nil, err
		}
		schema = &spec.Schema{
			Type:                 "object",
			AdditionalProperties: addlProps,
		}
	case reflect.Struct:
		if IsTime(t) {
			schema = &spec.Schema{Type: "string", Format: "date-time"}
		} else {
			var err error
			schema, err = r.StructSchema(t, "json", false, mode)
			if err != nil {
				return nil, err
			}
		}
	case reflect.Interface:
		schema = &spec.Schema{}
	default:
		schema = &spec.Schema{}
	}
	if interceptSchema != nil {
		postParams := spec.InterceptSchemaParams{Type: t, Schema: schema, Processed: true}
		if _, err := interceptSchema(postParams); err != nil {
			return nil, err
		}
	}
	r.ApplyNullable(schema, nullable)
	if field != nil {
		r.ApplySchemaTags(schema, *field)
	}
	return schema, nil
}

func (r *Reflector) ApplyNullable(schema *spec.Schema, nullable bool) {
	if !nullable || schema == nil {
		return
	}
	if IsOpenAPI30(r.Config.OpenAPIVersion) {
		if schema.Ref != "" {
			ref := schema.Ref
			*schema = spec.Schema{
				AllOf:    []*spec.Schema{{Ref: ref}},
				Nullable: true,
			}
			return
		}
		schema.Nullable = true
		return
	}
	if schema.Ref != "" {
		ref := schema.Ref
		*schema = spec.Schema{
			AnyOf: []*spec.Schema{
				{Ref: ref},
				{Type: "null"},
			},
		}
		return
	}
	switch typ := schema.Type.(type) {
	case string:
		if typ != "" {
			schema.Type = []string{typ, "null"}
		}
	case []string:
		if !slices.Contains(typ, "null") {
			typ = append(typ, "null")
			schema.Type = typ
		}
	}
}

func IsOpenAPI30(version string) bool {
	return version == spec.Version300 || version == spec.Version301 || version == spec.Version302 ||
		version == spec.Version303 || version == spec.Version304
}

func IsComponentType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.Name() != "" && !IsTime(t) && !isTextMarshaler(t)
}
