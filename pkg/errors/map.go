package errors

// Mapper converts a lower-level error into a canonical error when recognized.
type Mapper func(err error) (*Error, bool)

// Pair returns a Mapper that recognizes errors matching from (by Is, so wrap
// chains and clones are matched) and maps them to to. It makes the common
// one-to-one boundary translation read declaratively:
//
//	errors.MapOr(err, api.ErrInternal,
//	    errors.Pair(store.ErrNotFound, api.ErrUserNotFound),
//	    errors.Pair(store.ErrConflict, api.ErrConflict),
//	)
func Pair(from, to *Error) Mapper {
	return func(err error) (*Error, bool) {
		if from != nil && to != nil && Is(err, from) {
			return to, true
		}
		return nil, false
	}
}

// Mappers composes several mappers into one, consulting them in order and
// returning the first match. It lets a boundary keep a single reusable mapper
// value that can be passed to Map/MapOr.
func Mappers(mappers ...Mapper) Mapper {
	return func(err error) (*Error, bool) {
		for _, m := range mappers {
			if m == nil {
				continue
			}
			if e, ok := m(err); ok && e != nil {
				return e, true
			}
		}
		return nil, false
	}
}

// Map converts err into a canonical error using mappers. Mappers are always
// consulted first — including for already-structured errors — so module-level
// catalog entries can be re-mapped to public catalog entries at boundaries
// (match with Is inside the mapper). The matched entry wraps err as its cause.
//
// When no mapper matches, structured errors pass through unchanged and
// unstructured errors yield nil.
func Map(err error, mappers ...Mapper) error {
	if err == nil {
		return nil
	}
	for _, m := range mappers {
		if e, ok := m(err); ok && e != nil {
			return Wrap(err, e)
		}
	}
	if IsStructured(err) {
		return err
	}
	return nil
}

// Or wraps err with a catalog fallback entry when it is not already structured.
func Or(err error, fallback *Error) error {
	if err == nil {
		return nil
	}
	if IsStructured(err) {
		return err
	}
	validateFallback(fallback)
	return Wrap(err, clone(fallback))
}

// MapOr applies mappers first, then wraps err with fallback when none match.
// Unlike Map or Or alone, the fallback applies even when err is already
// structured — the typical boundary pattern where known internal entries are
// paired explicitly and everything else maps to one public fallback.
func MapOr(err error, fallback *Error, mappers ...Mapper) error {
	if err == nil {
		return nil
	}
	for _, m := range mappers {
		if e, ok := m(err); ok && e != nil {
			return Wrap(err, e)
		}
	}
	validateFallback(fallback)
	return Wrap(err, clone(fallback))
}
