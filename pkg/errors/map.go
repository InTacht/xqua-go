package errors

// Mapper converts a lower-level error into a canonical error when recognized.
type Mapper func(err error) (*Error, bool)

// Map converts err using mappers when it is not already structured.
// Returns nil when err is nil or no mapper matches.
func Map(err error, mappers ...Mapper) error {
	if err == nil {
		return nil
	}
	if IsStructured(err) {
		return err
	}
	for _, m := range mappers {
		if e, ok := m(err); ok && e != nil {
			return Wrap(err, clone(e))
		}
	}
	return nil
}

// Or wraps err with a fallback canonical error when it is not already structured.
func Or(err error, kind, code, message string) error {
	if err == nil {
		return nil
	}
	if IsStructured(err) {
		return err
	}
	msg := message
	if msg == "" {
		msg = err.Error()
	}
	return Wrap(err, New(kind, code, msg))
}

// MapOr applies mappers first, then falls back to Or.
func MapOr(err error, kind, code, message string, mappers ...Mapper) error {
	if err == nil {
		return nil
	}
	if mapped := Map(err, mappers...); mapped != nil {
		return mapped
	}
	return Or(err, kind, code, message)
}
