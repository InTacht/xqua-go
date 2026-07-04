package env

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Bind populates the fields of the struct pointed to by ptr from environment
// variables, using struct tags to declare each binding:
//
//	type Config struct {
//	    Host        string        `env:"APP_HOST" default:"0.0.0.0"`
//	    Port        int           `env:"APP_PORT" default:"8080"`
//	    Debug       bool          `env:"DEBUG" default:"false"`
//	    Shutdown    time.Duration `env:"SHUTDOWN_TIMEOUT" default:"30s"`
//	    DatabaseURL string        `env:"DATABASE_URL,required"`
//	}
//
//	var cfg Config
//	if err := env.Bind(&cfg); err != nil {
//	    log.Fatal(err)
//	}
//
// The env tag names the variable; add ",required" to fail when it is unset or
// empty. A default tag supplies a fallback when the variable is unset or empty;
// without one the field keeps its zero value. Fields with no env tag (or
// `env:"-"`) and unexported fields are skipped.
//
// Supported field types:
//
//   - any type implementing encoding.TextUnmarshaler (for example time.Time,
//     net.IP, netip.Addr, and your own types) — parsed via UnmarshalText
//   - string, bool
//   - the signed and unsigned integer types, and the float types
//   - time.Duration, parsed with time.ParseDuration
//   - slices of any supported element type, from a comma-separated value
//
// Named types built on a supported kind (for example `type Level string`) work
// as that kind. Bind reports every missing or invalid variable in a single
// joined error rather than failing on the first.
func Bind(ptr any) error {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("env: Bind requires a non-nil pointer to a struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("env: Bind requires a pointer to a struct")
	}
	rt := rv.Type()

	var errs []error
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("env")
		if tag == "" || tag == "-" {
			continue
		}

		name, required := parseEnvTag(tag)
		raw, ok := os.LookupEnv(name)
		if !ok || raw == "" {
			if required {
				errs = append(errs, fmt.Errorf("env: %s is required", name))
				continue
			}
			def, hasDef := field.Tag.Lookup("default")
			if !hasDef {
				continue
			}
			raw = def
		}

		if err := setField(rv.Field(i), raw); err != nil {
			errs = append(errs, fmt.Errorf("env: %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

func parseEnvTag(tag string) (name string, required bool) {
	parts := strings.Split(tag, ",")
	name = strings.TrimSpace(parts[0])
	for _, opt := range parts[1:] {
		if strings.TrimSpace(opt) == "required" {
			required = true
		}
	}
	return name, required
}

var (
	durationType        = reflect.TypeFor[time.Duration]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

func setField(field reflect.Value, raw string) error {
	// A custom parser (encoding.TextUnmarshaler) wins over kind-based parsing,
	// so types like time.Time, net.IP, and user types decode themselves.
	if field.CanAddr() && reflect.PointerTo(field.Type()).Implements(textUnmarshalerType) {
		u := field.Addr().Interface().(encoding.TextUnmarshaler)
		if err := u.UnmarshalText([]byte(raw)); err != nil {
			return fmt.Errorf("invalid value %q: %w", raw, err)
		}
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid bool %q", raw)
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == durationType {
			d, err := time.ParseDuration(raw)
			if err != nil {
				return fmt.Errorf("invalid duration %q", raw)
			}
			field.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid int %q", raw)
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid unsigned int %q", raw)
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid float %q", raw)
		}
		field.SetFloat(f)
	case reflect.Slice:
		return setSlice(field, raw)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}

// setSlice parses a comma-separated value into a slice, delegating each element
// to setField so slices inherit every supported element type. Surrounding
// whitespace on elements is trimmed; an empty value yields an empty slice.
func setSlice(field reflect.Value, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		field.Set(reflect.MakeSlice(field.Type(), 0, 0))
		return nil
	}
	parts := strings.Split(raw, ",")
	out := reflect.MakeSlice(field.Type(), len(parts), len(parts))
	for i, part := range parts {
		if err := setField(out.Index(i), strings.TrimSpace(part)); err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
	}
	field.Set(out)
	return nil
}
