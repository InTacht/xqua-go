package binder

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"

	"github.com/InTacht/xqua-go/pkg/errors"
	oreflect "github.com/InTacht/xqua-go/pkg/http/openapi/internal/reflect"

	"github.com/gofiber/fiber/v3"
)

var (
	typeFileHeader = reflect.TypeFor[multipart.FileHeader]()
	typeFile       = reflect.TypeFor[multipart.File]()
)

// Binder binds a Fiber request into a typed input struct.
type Binder struct {
	inType              reflect.Type
	validationTemplates []*errors.Error
}

// New creates a binder for the given input type. validationTemplates are the
// catalog entries declared under Responses.Err(422, ...) for this route (merged
// from group + route). Bind failures are returned as clones of these entries.
func New(inType reflect.Type, validationTemplates []*errors.Error) *Binder {
	if inType == nil {
		return nil
	}
	return &Binder{
		inType:              inType,
		validationTemplates: append([]*errors.Error(nil), validationTemplates...),
	}
}

func (b *Binder) bindStruct(c fiber.Ctx, v reflect.Value, prefix string) error {
	t := v.Type()
	var valErrs errors.Errors
	for i := range t.NumField() {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		fv := v.Field(i)
		if field.Anonymous && oreflect.IndirectType(field.Type).Kind() == reflect.Struct && oreflect.TagName(field, "json") == "" {
			if err := b.bindStruct(c, fv, prefix); err != nil {
				if es, ok := err.(errors.Errors); ok {
					valErrs = valErrs.Append(es...)
				}
			}
			continue
		}
		if err := b.bindField(c, field, fv, prefix); err != nil {
			if es, ok := err.(errors.Errors); ok {
				valErrs = valErrs.Append(es...)
			}
		}
	}
	if len(valErrs) > 0 {
		return valErrs
	}
	return nil
}

func (b *Binder) bindField(c fiber.Ctx, field reflect.StructField, fv reflect.Value, prefix string) error {
	if !fv.CanSet() {
		return nil
	}
	if name := oreflect.TagName(field, "path"); name != "" {
		return b.setFromString(c.Params(name), field, fv, source(prefix, "params", name))
	}
	if name := oreflect.TagName(field, "query"); name != "" {
		return b.setFromString(c.Query(name), field, fv, source(prefix, "query", name))
	}
	if name := oreflect.TagName(field, "header"); name != "" {
		return b.setFromString(c.Get(name), field, fv, source(prefix, "header", name))
	}
	if name := oreflect.TagName(field, "cookie"); name != "" {
		return b.setFromString(c.Cookies(name), field, fv, source(prefix, "cookie", name))
	}
	if name := oreflect.TagName(field, "form"); name != "" {
		if isFormFileType(field.Type) {
			return b.bindFormFile(c, name, field, fv, prefix)
		}
		return b.setFromString(c.FormValue(name), field, fv, source(prefix, "form", name))
	}
	return nil
}

func (b *Binder) bindFormFile(c fiber.Ctx, name string, field reflect.StructField, fv reflect.Value, prefix string) error {
	src := source(prefix, "form", name)
	ft := field.Type

	switch ft.Kind() {
	case reflect.Pointer:
		if ft.Elem() != typeFileHeader {
			return fmt.Errorf("unsupported form file type %s", ft)
		}
		fh, err := c.FormFile(name)
		if err != nil {
			return errors.Errors{b.validationErr(src, err.Error())}
		}
		if fh == nil {
			if oreflect.BoolTag(field.Tag.Get("required")) {
				return errors.Errors{b.validationErr(src, "required")}
			}
			return nil
		}
		fv.Set(reflect.ValueOf(fh))
		return nil
	case reflect.Slice:
		elem := ft.Elem()
		for elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		if elem != typeFileHeader {
			return fmt.Errorf("unsupported form file type %s", ft)
		}
		form, err := c.MultipartForm()
		if err != nil {
			return errors.Errors{b.validationErr(src, err.Error())}
		}
		files := form.File[name]
		if len(files) == 0 {
			if oreflect.BoolTag(field.Tag.Get("required")) {
				return errors.Errors{b.validationErr(src, "required")}
			}
			return nil
		}
		slice := reflect.MakeSlice(ft, len(files), len(files))
		for i, fh := range files {
			slice.Index(i).Set(reflect.ValueOf(fh))
		}
		fv.Set(slice)
		return nil
	default:
		if ft == typeFile || ft.Implements(typeFile) {
			fh, err := c.FormFile(name)
			if err != nil {
				return errors.Errors{b.validationErr(src, err.Error())}
			}
			if fh == nil {
				if oreflect.BoolTag(field.Tag.Get("required")) {
					return errors.Errors{b.validationErr(src, "required")}
				}
				return nil
			}
			f, err := fh.Open()
			if err != nil {
				return errors.Errors{b.validationErr(src, err.Error())}
			}
			fv.Set(reflect.ValueOf(f))
			return nil
		}
		return fmt.Errorf("unsupported form file type %s", ft)
	}
}

func isFormFileType(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		t = t.Elem()
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
	}
	return t == typeFileHeader || t == typeFile || t.Implements(typeFile)
}

func (b *Binder) BindWithBody(c fiber.Ctx) (any, error) {
	if b == nil || b.inType == nil {
		return nil, nil
	}
	v := reflect.New(b.inType)
	if err := b.bindStruct(c, v.Elem(), ""); err != nil {
		return nil, err
	}
	if hasFormFields(b.inType) {
		// Form and multipart fields are bound in bindStruct via FormValue/FormFile.
	} else if hasJSONFields(b.inType) || len(c.Body()) > 0 {
		if err := b.bindBodyFields(c, v.Elem()); err != nil {
			return nil, err
		}
	}
	if err := b.validateStruct(v.Elem(), ""); err != nil {
		return nil, err
	}
	return v.Elem().Interface(), nil
}

func hasJSONFields(t reflect.Type) bool {
	found := false
	oreflect.ForEachField(t, func(field reflect.StructField) {
		if oreflect.TagName(field, "json") != "" {
			found = true
		}
	})
	return found
}

func hasFormFields(t reflect.Type) bool {
	found := false
	oreflect.ForEachField(t, func(field reflect.StructField) {
		if oreflect.TagName(field, "form") != "" {
			found = true
		}
	})
	return found
}

func (b *Binder) bindBodyFields(c fiber.Ctx, v reflect.Value) error {
	body := c.Body()
	if len(body) == 0 {
		return nil
	}
	if allJSONBody(b.inType) {
		if err := json.Unmarshal(body, v.Addr().Interface()); err != nil {
			return errors.Errors{b.validationErr("body", err.Error())}
		}
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return errors.Errors{b.validationErr("body", err.Error())}
	}
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		name := oreflect.TagName(field, "json")
		if name == "" {
			continue
		}
		rawVal, ok := raw[name]
		if !ok {
			continue
		}
		fv := v.Field(i)
		if err := json.Unmarshal(rawVal, fv.Addr().Interface()); err != nil {
			return errors.Errors{b.validationErr(source("", "body", name), err.Error())}
		}
	}
	return nil
}

func allJSONBody(t reflect.Type) bool {
	hasJSON := false
	hasOther := false
	oreflect.ForEachField(t, func(field reflect.StructField) {
		if oreflect.TagName(field, "path") != "" || oreflect.TagName(field, "query") != "" ||
			oreflect.TagName(field, "header") != "" || oreflect.TagName(field, "cookie") != "" {
			hasOther = true
		}
		if oreflect.TagName(field, "json") != "" {
			hasJSON = true
		}
	})
	return hasJSON && !hasOther
}

func (b *Binder) setFromString(raw string, field reflect.StructField, fv reflect.Value, src string) error {
	if raw == "" {
		if def := field.Tag.Get("default"); def != "" {
			raw = def
		}
	}
	if raw == "" {
		if oreflect.BoolTag(field.Tag.Get("required")) {
			return errors.Errors{b.validationErr(src, "required")}
		}
		return nil
	}
	if err := setScalar(raw, fv); err != nil {
		return errors.Errors{b.validationErr(src, err.Error())}
	}
	return b.validateField(field, fv, src)
}

func setScalar(raw string, fv reflect.Value) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	case reflect.Bool:
		n, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(n)
	default:
		return fmt.Errorf("unsupported type %s", fv.Type())
	}
	return nil
}

func (b *Binder) validateStruct(v reflect.Value, prefix string) error {
	var valErrs errors.Errors
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		fv := v.Field(i)
		if field.Anonymous && oreflect.IndirectType(field.Type).Kind() == reflect.Struct {
			if err := b.validateStruct(fv, prefix); err != nil {
				if es, ok := err.(errors.Errors); ok {
					valErrs = valErrs.Append(es...)
				}
			}
			continue
		}
		src := fieldSource(field, prefix)
		if err := b.validateField(field, fv, src); err != nil {
			if es, ok := err.(errors.Errors); ok {
				valErrs = valErrs.Append(es...)
			}
		}
	}
	if len(valErrs) > 0 {
		return valErrs
	}
	return nil
}

func (b *Binder) validateField(field reflect.StructField, fv reflect.Value, src string) error {
	if tag := field.Tag.Get("minimum"); tag != "" {
		min, _ := strconv.ParseFloat(tag, 64)
		if fv.Kind() >= reflect.Int && fv.Kind() <= reflect.Int64 {
			if float64(fv.Int()) < min {
				return errors.Errors{b.validationErr(src, fmt.Sprintf("must be >= %s", tag))}
			}
		}
	}
	if tag := field.Tag.Get("minLength"); tag != "" {
		min, _ := strconv.Atoi(tag)
		if fv.Kind() == reflect.String && len(fv.String()) < min {
			return errors.Errors{b.validationErr(src, fmt.Sprintf("minLength %d", min))}
		}
	}
	return nil
}

func fieldSource(field reflect.StructField, prefix string) string {
	for _, tag := range []string{"path", "query", "header", "cookie", "json", "form"} {
		if name := oreflect.TagName(field, tag); name != "" {
			loc := tag
			if tag == "path" {
				loc = "params"
			}
			return source(prefix, loc, name)
		}
	}
	return prefix
}

func source(prefix, loc, name string) string {
	if prefix != "" {
		return prefix + "." + name
	}
	return loc + "." + name
}

func (b *Binder) validationErr(source, message string) *errors.Error {
	tmpl := matchValidationTemplate(b.validationTemplates, source)
	if tmpl == nil {
		return &errors.Error{
			Kind:    errors.KindValidation,
			Code:    "validation",
			Message: message,
			Source:  source,
		}
	}
	if tmpl.Source == source {
		return tmpl.WithSource(source)
	}
	return tmpl.WithSource(source).WithMessage(message)
}

func matchValidationTemplate(templates []*errors.Error, source string) *errors.Error {
	for _, e := range templates {
		if e.Source == source {
			return e
		}
	}
	for _, e := range templates {
		if e.Source == "" {
			return e
		}
	}
	if len(templates) > 0 {
		return templates[0]
	}
	return nil
}

// BindRequest binds path/query/header/cookie and JSON body when appropriate.
func (b *Binder) BindRequest(c fiber.Ctx) (any, error) {
	if b == nil {
		return struct{}{}, nil
	}
	method := strings.ToUpper(c.Method())
	if method == fiber.MethodGet || method == fiber.MethodDelete || method == fiber.MethodHead {
		v := reflect.New(b.inType)
		if err := b.bindStruct(c, v.Elem(), ""); err != nil {
			return nil, err
		}
		if err := b.validateStruct(v.Elem(), ""); err != nil {
			return nil, err
		}
		return v.Elem().Interface(), nil
	}
	return b.BindWithBody(c)
}
