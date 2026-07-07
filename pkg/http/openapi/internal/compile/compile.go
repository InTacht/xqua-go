package compile

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/InTacht/xqua-go/pkg/errors"
	"github.com/InTacht/xqua-go/pkg/http/openapi/internal/binder"
)

// ResponseDecl is a minimal extra-response descriptor for compile (mirrors openapi.ResponseDecl).
type ResponseDecl struct {
	Status   int
	ItemBody any
}

type ErrCase struct {
	Status int
	Errors []*errors.Error
}

// Input is everything needed to compile one route at registration time.
type Input struct {
	Method string
	Path   string

	Describe bool
	Handler  any
	Request  any

	InferSuccess bool
	SuccessType  reflect.Type
	ErrCases     []ErrCase

	Enveloped bool

	Extra []ResponseDecl

	Catalog       *errors.Catalog
	KindStatuses  map[string]int
	DefaultStatus int
}

// Route is a compiled, registration-time route ready for the adapter.
type Route struct {
	Method      string
	Path        string
	Binder      *binder.Binder
	Call        func(ctx context.Context, in any) (any, error)
	Enveloped   bool
	SuccessType reflect.Type
	InType      reflect.Type
	ErrCases    []ErrCase
	ErrIndex    map[*errors.Error]int
}

// Build validates and compiles a route. It panics on contract violations.
func Build(in Input) *Route {
	prefix := fmt.Sprintf("openapi: %s %s", in.Method, in.Path)

	var inType reflect.Type
	var call func(context.Context, any) (any, error)

	if in.Describe {
		if in.Handler != nil {
			panic(prefix + ": Describe route must not set Handler")
		}
		if in.SuccessType == nil && len(in.Extra) == 0 {
			panic(prefix + ": Describe route requires Returns(T{}) or Extra response")
		}
		if in.Request != nil {
			inType = indirect(reflect.TypeOf(in.Request))
		}
	} else {
		if in.Handler == nil {
			panic(prefix + ": Handler is required")
		}
		var handlerOut reflect.Type
		inType, handlerOut, call = parseHandler(prefix, in.Handler)
		if in.InferSuccess || in.SuccessType == nil {
			in.SuccessType = handlerOut
		} else if in.SuccessType != handlerOut {
			panic(fmt.Sprintf("%s: Returns(%s) does not match Handler return type %s",
				prefix, in.SuccessType.Name(), handlerOut.Name()))
		}
	}

	validateErrCases(prefix, in)

	if inType != nil && !in.Describe && hasBindableFields(inType) && len(validationTemplates(in.ErrCases)) == 0 {
		panic(prefix + ": route with bindable input requires Responses.Err(422, ...) or group inheritance")
	}

	valTemplates := validationTemplates(in.ErrCases)
	var b *binder.Binder
	if inType != nil {
		b = binder.New(inType, valTemplates)
	}

	errIndex := make(map[*errors.Error]int)
	for _, c := range in.ErrCases {
		for _, e := range c.Errors {
			errIndex[e] = c.Status
		}
	}

	return &Route{
		Method:      in.Method,
		Path:        in.Path,
		Binder:      b,
		Call:        call,
		Enveloped:   in.Enveloped,
		SuccessType: in.SuccessType,
		InType:      inType,
		ErrCases:    append([]ErrCase(nil), in.ErrCases...),
		ErrIndex:    errIndex,
	}
}

func parseHandler(prefix string, handler any) (inType, outType reflect.Type, call func(context.Context, any) (any, error)) {
	t := reflect.TypeOf(handler)
	if t == nil || t.Kind() != reflect.Func {
		panic(prefix + ": Handler must be func(context.Context, In) (Out, error)")
	}
	if t.NumIn() != 2 || t.NumOut() != 2 {
		panic(prefix + ": Handler must be func(context.Context, In) (Out, error)")
	}
	ctxType := reflect.TypeFor[context.Context]()
	if !t.In(0).Implements(ctxType) {
		panic(prefix + ": Handler must be func(context.Context, In) (Out, error)")
	}
	if !t.Out(1).Implements(reflect.TypeFor[error]()) {
		panic(prefix + ": Handler must be func(context.Context, In) (Out, error)")
	}
	inType = indirect(t.In(1))
	outType = indirect(t.Out(0))

	hv := reflect.ValueOf(handler)
	call = func(ctx context.Context, in any) (any, error) {
		var inVal reflect.Value
		if in == nil {
			inVal = reflect.Zero(t.In(1))
		} else {
			inVal = reflect.ValueOf(in)
		}
		out := hv.Call([]reflect.Value{reflect.ValueOf(ctx), inVal})
		if !out[1].IsNil() {
			return nil, out[1].Interface().(error)
		}
		return out[0].Interface(), nil
	}
	return inType, outType, call
}

func validateErrCases(prefix string, in Input) {
	for _, c := range in.ErrCases {
		for _, e := range c.Errors {
			if e == nil {
				panic(fmt.Sprintf("%s: nil catalog entry in Responses.Err(%d, ...)", prefix, c.Status))
			}
			if !in.Catalog.Contains(e) {
				panic(fmt.Sprintf(
					"%s: Responses.Err(%d, ...) entry %s<%s> is not in the public catalog %q",
					prefix, c.Status, e.Kind, e.Code, in.Catalog.Name(),
				))
			}
			resolved := kindStatus(in.KindStatuses, e.Kind, in.DefaultStatus)
			if resolved != c.Status {
				panic(fmt.Sprintf(
					"%s: %s<%s> resolves to HTTP %d but listed under Err(%d, ...)",
					prefix, e.Kind, e.Code, resolved, c.Status,
				))
			}
		}
	}
}

func kindStatus(kindStatuses map[string]int, kind string, defaultStatus int) int {
	if code, ok := kindStatuses[kind]; ok {
		return code
	}
	return defaultStatus
}

func validationTemplates(errCases []ErrCase) []*errors.Error {
	var out []*errors.Error
	for _, c := range errCases {
		if c.Status != 422 {
			continue
		}
		out = append(out, c.Errors...)
	}
	return out
}

func hasBindableFields(t reflect.Type) bool {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return false
	}
	for i := range t.NumField() {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		ft := field.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if field.Anonymous && ft.Kind() == reflect.Struct {
			if hasBindableFields(ft) {
				return true
			}
			continue
		}
		for _, tag := range []string{"path", "query", "header", "cookie", "json", "form"} {
			if tagName := fieldTag(field, tag); tagName != "" {
				return true
			}
		}
	}
	return false
}

func fieldTag(field reflect.StructField, key string) string {
	v, ok := field.Tag.Lookup(key)
	if !ok || v == "-" {
		return ""
	}
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

func indirect(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
