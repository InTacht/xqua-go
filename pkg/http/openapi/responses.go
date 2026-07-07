package openapi

import (
	"reflect"

	"github.com/InTacht/xqua-go/pkg/errors"
)

// ErrCase declares catalog errors returned at one HTTP status.
type ErrCase struct {
	Status int
	Errors []*errors.Error
}

// ResponseSet is the success + error contract for a route or group.
type ResponseSet struct {
	inferSuccess bool
	successBody  any
	cases        []ErrCase
}

// Returns starts a response contract. With no arguments success is inferred from
// the route Handler return type. With one argument success is the given type
// (required for Describe routes).
func Returns(body ...any) *ResponseSet {
	rs := &ResponseSet{}
	if len(body) == 0 {
		rs.inferSuccess = true
		return rs
	}
	rs.successBody = body[0]
	return rs
}

// Err declares public catalog errors for one HTTP status.
func (r *ResponseSet) Err(status int, errs ...*errors.Error) *ResponseSet {
	if r == nil {
		r = &ResponseSet{}
	}
	r.cases = append(r.cases, ErrCase{Status: status, Errors: append([]*errors.Error(nil), errs...)})
	return r
}

// Cases returns the declared error cases.
func (r *ResponseSet) Cases() []ErrCase {
	if r == nil {
		return nil
	}
	return append([]ErrCase(nil), r.cases...)
}

// InferSuccess reports whether success type should be taken from the handler.
func (r *ResponseSet) InferSuccess() bool {
	return r == nil || r.inferSuccess
}

// SuccessBody returns the explicit success sample value, if any.
func (r *ResponseSet) SuccessBody() any {
	if r == nil {
		return nil
	}
	return r.successBody
}

// SuccessType returns the reflect.Type of the explicit success body, if set.
func (r *ResponseSet) SuccessType() reflect.Type {
	if r == nil || r.successBody == nil {
		return nil
	}
	t := reflect.TypeOf(r.successBody)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// MergeResponseSets layers route on top of group error cases. Success comes from
// route only; group contributes error cases only.
func MergeResponseSets(group, route *ResponseSet) *ResponseSet {
	out := &ResponseSet{
		inferSuccess: true,
	}
	if route != nil {
		out.inferSuccess = route.inferSuccess
		out.successBody = route.successBody
		out.cases = append(out.cases, route.cases...)
	}
	if group != nil {
		out.cases = append(group.cases, out.cases...)
	}
	return out
}
